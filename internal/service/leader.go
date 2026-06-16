package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"database/sql"

	"github.com/sameetpatro/go-qr-auth/internal/audit"
	"github.com/sameetpatro/go-qr-auth/internal/dto"
	"github.com/sameetpatro/go-qr-auth/internal/models"
	"github.com/sameetpatro/go-qr-auth/internal/repository"
	"github.com/sameetpatro/go-qr-auth/pkg/password"
)

var (
	ErrLeaderNotFound    = errors.New("leader not found")
	ErrLeaderEmailExists = errors.New("leader email already exists")
	ErrInvalidUsername   = errors.New("invalid username")
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{1,48}[a-zA-Z0-9]$`)

const leaderEmailDomain = "leader.jms"

type LeaderService struct {
	users *repository.UserRepository
	audit *audit.Service
}

func NewLeaderService(users *repository.UserRepository, audit *audit.Service) *LeaderService {
	return &LeaderService{users: users, audit: audit}
}

func (s *LeaderService) Create(ctx context.Context, masterID int64, req dto.CreateLeaderRequest, ip string) (*dto.CreateLeaderResponse, error) {
	username := strings.ToLower(strings.TrimSpace(req.Username))
	if !usernamePattern.MatchString(username) {
		return nil, ErrInvalidUsername
	}

	email := fmt.Sprintf("%s@%s", username, leaderEmailDomain)
	exists, err := s.users.EmailExists(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrLeaderEmailExists
	}

	var plainPassword string
	if req.GeneratePassword || req.Password == nil || strings.TrimSpace(*req.Password) == "" {
		plainPassword, err = password.Generate(10)
		if err != nil {
			return nil, err
		}
	} else {
		plainPassword = *req.Password
	}

	hash, err := password.Hash(plainPassword)
	if err != nil {
		return nil, err
	}

	displayName := username
	user := &models.User{
		Email:        email,
		PasswordHash: hash,
		Role:         models.RoleLeader,
		IsActive:     true,
		DisplayName:  &displayName,
		CreatedBy:    &masterID,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}

	role := models.RoleMaster
	s.audit.Log(ctx, &masterID, &role, models.AuditCreateLeader,
		fmt.Sprintf("Created leader %s", email), ip)

	return &dto.CreateLeaderResponse{
		ID:       user.ID,
		Email:    email,
		Username: username,
		Password: plainPassword,
		Role:     string(models.RoleLeader),
	}, nil
}

func (s *LeaderService) List(ctx context.Context) ([]dto.LeaderResponse, error) {
	users, err := s.users.ListLeaders(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]dto.LeaderResponse, len(users))
	for i, u := range users {
		total, checkedIn, err := s.users.GuestCountsForLeader(ctx, u.ID)
		if err != nil {
			return nil, err
		}
		username := leaderUsername(u.Email)
		result[i] = dto.LeaderResponse{
			ID:         u.ID,
			Email:      u.Email,
			Username:   username,
			IsActive:   u.IsActive,
			GuestCount: total,
			CheckedIn:  checkedIn,
			CreatedAt:  u.CreatedAt,
		}
	}
	return result, nil
}

func (s *LeaderService) Disable(ctx context.Context, masterID, leaderID int64, ip string) error {
	user, err := s.users.FindByID(ctx, leaderID)
	if err != nil || user == nil || user.Role != models.RoleLeader {
		return ErrLeaderNotFound
	}
	if err := s.users.SetActive(ctx, leaderID, models.RoleLeader, false); err != nil {
		return err
	}
	role := models.RoleMaster
	s.audit.Log(ctx, &masterID, &role, models.AuditDisableLeader,
		fmt.Sprintf("Disabled leader ID %d", leaderID), ip)
	return nil
}

func (s *LeaderService) ResetPassword(ctx context.Context, masterID, leaderID int64, ip string) (*dto.ResetPasswordResponse, error) {
	user, err := s.users.FindByID(ctx, leaderID)
	if err != nil || user == nil || user.Role != models.RoleLeader {
		return nil, ErrLeaderNotFound
	}

	plainPassword, err := password.Generate(10)
	if err != nil {
		return nil, err
	}
	hash, err := password.Hash(plainPassword)
	if err != nil {
		return nil, err
	}
	if err := s.users.UpdatePassword(ctx, leaderID, hash); err != nil {
		return nil, err
	}

	role := models.RoleMaster
	s.audit.Log(ctx, &masterID, &role, models.AuditResetPassword,
		fmt.Sprintf("Reset password for leader %s", user.Email), ip)

	return &dto.ResetPasswordResponse{Email: user.Email, NewPassword: plainPassword}, nil
}

func (s *LeaderService) Delete(ctx context.Context, masterID, leaderID int64, ip string) (*dto.DeleteLeaderResult, error) {
	guestsDeleted, err := s.users.DeleteLeader(ctx, leaderID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLeaderNotFound
	}
	if err != nil {
		return nil, err
	}
	role := models.RoleMaster
	s.audit.Log(ctx, &masterID, &role, models.AuditDeleteLeader,
		fmt.Sprintf("Deleted leader ID %d and %d guests", leaderID, guestsDeleted), ip)
	return &dto.DeleteLeaderResult{LeaderID: leaderID, GuestsDeleted: guestsDeleted}, nil
}

func leaderUsername(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) > 0 {
		return parts[0]
	}
	return email
}
