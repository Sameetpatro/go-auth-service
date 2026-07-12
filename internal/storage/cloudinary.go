// Package storage provides permanent object storage for QR invitation cards.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/admin"
	"github.com/cloudinary/cloudinary-go/v2/api/admin/search"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

// CloudinaryStorage uploads QR card PNGs to Cloudinary and returns permanent
// CDN URLs. Unlike the local filesystem (ephemeral on Render), these URLs
// survive service restarts and are served from Cloudinary's edge network.
type CloudinaryStorage struct {
	cld *cloudinary.Cloudinary
}

// NewCloudinary creates a client from a CLOUDINARY_URL-style credential
// (cloudinary://api_key:api_secret@cloud_name).
func NewCloudinary(cloudinaryURL string) (*CloudinaryStorage, error) {
	cld, err := cloudinary.NewFromURL(cloudinaryURL)
	if err != nil {
		return nil, fmt.Errorf("cloudinary init: %w", err)
	}
	cld.Config.URL.Secure = true
	return &CloudinaryStorage{cld: cld}, nil
}

// UploadPNG uploads PNG bytes under a stable public ID (e.g. "qr/guest_42")
// and returns the permanent HTTPS URL. Re-uploading the same public ID
// overwrites the previous asset, so regenerating a card keeps one URL per guest.
func (s *CloudinaryStorage) UploadPNG(ctx context.Context, data []byte, publicID string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := s.cld.Upload.Upload(ctx, bytes.NewReader(data), uploader.UploadParams{
		PublicID:     publicID,
		Overwrite:    api.Bool(true),
		Invalidate:   api.Bool(true),
		ResourceType: "image",
		Format:       "png",
	})
	if err != nil {
		return "", fmt.Errorf("cloudinary upload: %w", err)
	}
	if resp.Error.Message != "" {
		return "", fmt.Errorf("cloudinary upload: %s", resp.Error.Message)
	}
	if resp.SecureURL == "" {
		return "", fmt.Errorf("cloudinary upload: empty secure_url in response")
	}
	return resp.SecureURL, nil
}

// DeletePNG permanently removes a single asset (e.g. "qr/guest_42") and
// invalidates CDN caches. Deleting a non-existent asset is not an error.
func (s *CloudinaryStorage) DeletePNG(ctx context.Context, publicID string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := s.cld.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID:   publicID,
		Invalidate: api.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("cloudinary destroy %s: %w", publicID, err)
	}
	// "ok" on success, "not found" when already gone — both acceptable.
	if resp.Result != "ok" && resp.Result != "not found" {
		return fmt.Errorf("cloudinary destroy %s: %s", publicID, resp.Result)
	}
	return nil
}

// DeleteAllPNGs removes every asset under the given prefix (e.g. "qr/").
// Used by the admin full-reset flow.
func (s *CloudinaryStorage) DeleteAllPNGs(ctx context.Context, prefix string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	_, err := s.cld.Admin.DeleteAssetsByPrefix(ctx, admin.DeleteAssetsByPrefixParams{
		Prefix:     api.CldAPIArray{prefix},
		Invalidate: api.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("cloudinary delete by prefix %s: %w", prefix, err)
	}
	return nil
}

// PurgeAll deletes every image asset under prefix, looping because Cloudinary's
// delete-by-prefix endpoint only removes up to 100 assets per call. Returns the
// total number of assets deleted.
func (s *CloudinaryStorage) PurgeAll(ctx context.Context, prefix string) (int, error) {
	total := 0
	for iteration := 0; iteration < 1000; iteration++ {
		callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		resp, err := s.cld.Admin.DeleteAssetsByPrefix(callCtx, admin.DeleteAssetsByPrefixParams{
			Prefix:     api.CldAPIArray{prefix},
			Invalidate: api.Bool(true),
		})
		cancel()
		if err != nil {
			return total, fmt.Errorf("cloudinary delete by prefix %s: %w", prefix, err)
		}
		if resp.Error.Message != "" {
			return total, fmt.Errorf("cloudinary delete by prefix %s: %s", prefix, resp.Error.Message)
		}
		total += len(resp.Deleted)
		// Done once the API stops reporting a partial result and this batch was empty.
		if !resp.Partial && len(resp.Deleted) == 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	return total, nil
}

// PurgeEverything deletes every image asset in the account (Cloudinary's
// delete-by-prefix rejects an empty prefix, so we search for all images and
// delete them in batches of 100). Returns the total number of assets deleted.
func (s *CloudinaryStorage) PurgeEverything(ctx context.Context) (int, error) {
	total := 0
	for iteration := 0; iteration < 10000; iteration++ {
		listCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		res, err := s.cld.Admin.Search(listCtx, search.Query{
			Expression: "resource_type:image",
			MaxResults: 100,
		})
		cancel()
		if err != nil {
			return total, fmt.Errorf("cloudinary search: %w", err)
		}
		if res.Error.Message != "" {
			return total, fmt.Errorf("cloudinary search: %s", res.Error.Message)
		}
		if len(res.Assets) == 0 {
			break
		}

		ids := make([]string, 0, len(res.Assets))
		for _, a := range res.Assets {
			ids = append(ids, a.PublicID)
		}

		delCtx, cancelDel := context.WithTimeout(ctx, 60*time.Second)
		resp, err := s.cld.Admin.DeleteAssets(delCtx, admin.DeleteAssetsParams{
			PublicIDs:  api.CldAPIArray(ids),
			Invalidate: api.Bool(true),
		})
		cancelDel()
		if err != nil {
			return total, fmt.Errorf("cloudinary delete assets: %w", err)
		}
		if resp.Error.Message != "" {
			return total, fmt.Errorf("cloudinary delete assets: %s", resp.Error.Message)
		}
		total += len(resp.Deleted)
		if len(resp.Deleted) == 0 {
			// Nothing deleted this round; avoid an infinite loop.
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	return total, nil
}
