package qr

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"

	"github.com/sameetpatro/go-qr-auth/internal/config"
)

type Service struct {
	secret    string
	imagePath string
	imageURL  string
}

func NewService(cfg config.StorageConfig, jwtSecret string) *Service {
	return &Service{
		secret:    jwtSecret,
		imagePath: cfg.QRImagePath,
		imageURL:  cfg.QRImageURL,
	}
}

func (s *Service) GenerateGuestQR(guestUUID uuid.UUID) (token string, imageURL string, err error) {
	token = s.signToken(guestUUID.String())

	if err := os.MkdirAll(s.imagePath, 0o755); err != nil {
		return "", "", fmt.Errorf("create qr directory: %w", err)
	}

	filename := fmt.Sprintf("%s.png", guestUUID.String())
	filePath := filepath.Join(s.imagePath, filename)

	if err := qrcode.WriteFile(token, qrcode.Medium, 256, filePath); err != nil {
		return "", "", fmt.Errorf("generate qr image: %w", err)
	}

	imageURL = fmt.Sprintf("%s/%s", s.imageURL, filename)
	return token, imageURL, nil
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
