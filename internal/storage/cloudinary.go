// Package storage provides permanent object storage for QR invitation cards.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api"
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
