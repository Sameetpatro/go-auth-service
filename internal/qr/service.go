package qr

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/sameetpatro/go-qr-auth/internal/config"
)

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
}

func NewService(cfg config.StorageConfig, event config.EventConfig, jwtSecret string) *Service {
	return &Service{
		secret:    jwtSecret,
		imagePath: cfg.QRImagePath,
		imageURL:  cfg.QRImageURL,
		event:     event,
	}
}

func (s *Service) SignToken(guestUUID uuid.UUID) string {
	return s.signToken(guestUUID.String())
}

func (s *Service) GenerateGuestQR(input GuestQRInput) (token string, imageURL string, err error) {
	token = s.signToken(input.UUID.String())

	if err := os.MkdirAll(s.imagePath, 0o755); err != nil {
		return "", "", fmt.Errorf("create qr directory: %w", err)
	}

	guestID := input.GuestID
	if guestID <= 0 {
		return "", "", fmt.Errorf("guest id required for qr image")
	}

	filename := fmt.Sprintf("%s_%d.png", SanitizeFilename(input.Name), guestID)
	filePath := filepath.Join(s.imagePath, filename)

	info := s.buildCardInfo(input)
	if err := writeInvitationCard(filePath, token, info); err != nil {
		return token, "", nil
	}

	imageURL = fmt.Sprintf("%s/%s", s.imageURL, filename)
	return token, imageURL, nil
}

func (s *Service) RegenerateCard(input GuestQRInput, token string) (imageURL string, err error) {
	if err := os.MkdirAll(s.imagePath, 0o755); err != nil {
		return "", err
	}
	guestID := input.GuestID
	filename := fmt.Sprintf("%s_%d.png", SanitizeFilename(input.Name), guestID)
	filePath := filepath.Join(s.imagePath, filename)
	info := s.buildCardInfo(input)
	if err := writeInvitationCard(filePath, token, info); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", s.imageURL, filename), nil
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
