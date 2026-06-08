package audit

import (
	"context"
	"fmt"

	"github.com/sameetpatro/go-qr-auth/internal/models"
	"github.com/sameetpatro/go-qr-auth/internal/repository"
)

type Service struct {
	repo *repository.AuditRepository
}

func NewService(repo *repository.AuditRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Log(ctx context.Context, userID *int64, role *models.UserRole, action models.AuditAction, description string, ip string) {
	log := &models.AuditLog{
		UserID:      userID,
		Role:        role,
		Action:      action,
		Description: strPtr(description),
		IPAddress:   strPtr(ip),
	}
	if err := s.repo.Create(ctx, log); err != nil {
		fmt.Printf("audit log error: %v\n", err)
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
