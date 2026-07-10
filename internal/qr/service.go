package qr

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/sameetpatro/go-qr-auth/internal/config"
)

// Uploader stores a rendered QR card permanently (e.g. Cloudinary) and
// returns its public URL. When nil, the Service falls back to local disk.
type Uploader interface {
	UploadPNG(ctx context.Context, data []byte, publicID string) (string, error)
	DeletePNG(ctx context.Context, publicID string) error
}

type GuestQRInput struct {
	UUID     uuid.UUID
	GuestID  int64
	Name     string
	Phone    *string
	Email    *string
	Address  *string
	College  *string
	Metadata map[string]interface{}
}

type Service struct {
	secret    string
	imagePath string
	imageURL  string
	event     config.EventConfig
	uploader  Uploader
}

func NewService(cfg config.StorageConfig, event config.EventConfig, jwtSecret string, uploader Uploader) *Service {
	return &Service{
		secret:    jwtSecret,
		imagePath: cfg.QRImagePath,
		imageURL:  cfg.QRImageURL,
		event:     event,
		uploader:  uploader,
	}
}

func (s *Service) SignToken(guestUUID uuid.UUID) string {
	return s.signToken(guestUUID.String())
}

func (s *Service) RenderGuestQRPNG(input GuestQRInput, token string) ([]byte, error) {
	info := s.buildCardInfo(input)
	return renderInvitationCard(token, info)
}

func (s *Service) GenerateGuestQR(ctx context.Context, input GuestQRInput) (token string, imageURL string, err error) {
	token = s.signToken(input.UUID.String())

	if input.GuestID <= 0 {
		return "", "", fmt.Errorf("guest id required for qr image")
	}

	imageURL, err = s.storeCard(ctx, input, token)
	if err != nil {
		// Guest creation must not fail because image storage is down;
		// the qr-image endpoint self-heals the URL on next fetch.
		log.Printf("qr: store card for guest %d failed: %v", input.GuestID, err)
		return token, "", nil
	}
	return token, imageURL, nil
}

func (s *Service) RegenerateCard(ctx context.Context, input GuestQRInput, token string) (imageURL string, err error) {
	return s.storeCard(ctx, input, token)
}

// storeCard renders the invitation card and persists it: to Cloudinary when an
// uploader is configured (permanent CDN URL), otherwise to the local disk.
func (s *Service) storeCard(ctx context.Context, input GuestQRInput, token string) (string, error) {
	info := s.buildCardInfo(input)
	data, err := renderInvitationCard(token, info)
	if err != nil {
		return "", err
	}

	if s.uploader != nil {
		return s.uploader.UploadPNG(ctx, data, fmt.Sprintf("qr/guest_%d", input.GuestID))
	}

	if err := os.MkdirAll(s.imagePath, 0o755); err != nil {
		return "", fmt.Errorf("create qr directory: %w", err)
	}
	filename := fmt.Sprintf("%s_%d.png", SanitizeFilename(input.Name), input.GuestID)
	if err := os.WriteFile(filepath.Join(s.imagePath, filename), data, 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", s.imageURL, filename), nil
}

// IsPermanentURL reports whether the stored qr_image_url already points at
// permanent storage and does not need regeneration.
func (s *Service) IsPermanentURL(url *string) bool {
	return s.uploader != nil && url != nil && strings.Contains(*url, "res.cloudinary.com")
}

// DeleteStoredCard removes a guest's stored QR card from Cloudinary (or the
// local disk fallback) so deleted guests don't leave orphaned images behind.
func (s *Service) DeleteStoredCard(ctx context.Context, guestID int64, name string) error {
	if s.uploader != nil {
		return s.uploader.DeletePNG(ctx, fmt.Sprintf("qr/guest_%d", guestID))
	}
	filename := fmt.Sprintf("%s_%d.png", SanitizeFilename(name), guestID)
	if err := os.Remove(filepath.Join(s.imagePath, filename)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Service) buildCardInfo(input GuestQRInput) GuestCardInfo {
	info := GuestCardInfo{
		Name:          input.Name,
		EventName:     s.event.Name,
		EventDate:     s.event.Date,
		EventLocation: s.event.Location,
	}
	if input.Phone != nil {
		info.Phone = *input.Phone
	}
	if input.Email != nil {
		info.Email = *input.Email
	}
	if input.Address != nil {
		info.Address = *input.Address
	} else if input.Metadata != nil {
		if v, ok := input.Metadata["address"].(string); ok {
			info.Address = v
		}
	}
	if input.College != nil {
		info.College = *input.College
	} else if input.Metadata != nil {
		if v, ok := input.Metadata["college"].(string); ok {
			info.College = v
		}
	}
	return info
}

func MetadataAddressCollege(meta map[string]interface{}) (address, college *string) {
	if meta == nil {
		return nil, nil
	}
	if v, ok := meta["address"].(string); ok && v != "" {
		address = &v
	}
	if v, ok := meta["college"].(string); ok && v != "" {
		college = &v
	}
	return address, college
}

func MergeMetadata(reqMeta map[string]interface{}, address, college *string) map[string]interface{} {
	meta := make(map[string]interface{})
	for k, v := range reqMeta {
		meta[k] = v
	}
	if address != nil && *address != "" {
		meta["address"] = *address
	}
	if college != nil && *college != "" {
		meta["college"] = *college
	}
	if len(meta) == 0 {
		return map[string]interface{}{}
	}
	return meta
}

func (s *Service) signToken(payload string) string {
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", payload, signature)
}

func (s *Service) ValidateToken(token string) (uuid.UUID, bool) {
	parts := splitLastDot(token)
	if len(parts) != 2 {
		return uuid.Nil, false
	}
	payload, signature := parts[0], parts[1]
	expected := s.signToken(payload)
	expectedSig := splitLastDot(expected)
	if len(expectedSig) != 2 || !hmac.Equal([]byte(signature), []byte(expectedSig[1])) {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(payload)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func splitLastDot(s string) []string {
	idx := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}

func RawMetadata(meta json.RawMessage) map[string]interface{} {
	var m map[string]interface{}
	if len(meta) > 0 {
		_ = json.Unmarshal(meta, &m)
	}
	return m
}
