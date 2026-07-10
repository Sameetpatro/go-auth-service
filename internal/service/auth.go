package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/sameetpatro/go-qr-auth/internal/audit"
	jwtsvc "github.com/sameetpatro/go-qr-auth/internal/auth"
	"github.com/sameetpatro/go-qr-auth/internal/config"
	"github.com/sameetpatro/go-qr-auth/internal/dto"
	"github.com/sameetpatro/go-qr-auth/internal/models"
	"github.com/sameetpatro/go-qr-auth/internal/qr"
	"github.com/sameetpatro/go-qr-auth/internal/repository"
	"github.com/sameetpatro/go-qr-auth/pkg/password"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserInactive       = errors.New("account is disabled")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrDuplicateGuest     = errors.New("guest already exists")
	ErrForbiddenAction    = errors.New("forbidden action")
)

type AuthService struct {
	users    *repository.UserRepository
	tokens   *repository.RefreshTokenRepository
	jwt      *jwtsvc.Service
	audit    *audit.Service
}

func NewAuthService(
	users *repository.UserRepository,
	tokens *repository.RefreshTokenRepository,
	jwt *jwtsvc.Service,
	audit *audit.Service,
) *AuthService {
	return &AuthService{users: users, tokens: tokens, jwt: jwt, audit: audit}
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest, ip string) (*dto.AuthResponse, error) {
	user, err := s.users.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if user == nil || !password.Verify(req.Password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}
	if !user.IsActive {
		return nil, ErrUserInactive
	}

	accessToken, _, err := s.jwt.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}
	refreshToken, hash, expiresAt, err := s.jwt.GenerateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	rt := &models.RefreshToken{UserID: user.ID, TokenHash: hash, ExpiresAt: expiresAt}
	if err := s.tokens.Create(ctx, rt); err != nil {
		return nil, err
	}

	role := user.Role
	s.audit.Log(ctx, &user.ID, &role, models.AuditLogin, fmt.Sprintf("User %s logged in", user.Email), ip)

	return &dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.jwt.AccessExpiry().Seconds()),
		User:         userToResponse(user),
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*dto.AuthResponse, error) {
	claims, err := s.jwt.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, ErrUnauthorized
	}

	hash := jwtsvc.HashToken(refreshToken)
	stored, err := s.tokens.FindByHash(ctx, hash)
	if err != nil || stored == nil {
		return nil, ErrUnauthorized
	}

	user, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil || user == nil || !user.IsActive {
		return nil, ErrUnauthorized
	}

	_ = s.tokens.DeleteByHash(ctx, hash)

	accessToken, _, err := s.jwt.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}
	newRefresh, newHash, expiresAt, err := s.jwt.GenerateRefreshToken(user)
	if err != nil {
		return nil, err
	}
	rt := &models.RefreshToken{UserID: user.ID, TokenHash: newHash, ExpiresAt: expiresAt}
	if err := s.tokens.Create(ctx, rt); err != nil {
		return nil, err
	}

	return &dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		ExpiresIn:    int64(s.jwt.AccessExpiry().Seconds()),
		User:         userToResponse(user),
	}, nil
}

func userToResponse(user *models.User) dto.UserResponse {
	return dto.UserResponse{
		ID:           user.ID,
		Email:        user.Email,
		Role:         string(user.Role),
		AssignedGate: user.AssignedGate,
	}
}

type CoordinatorService struct {
	users *repository.UserRepository
	audit *audit.Service
}

func NewCoordinatorService(users *repository.UserRepository, audit *audit.Service) *CoordinatorService {
	return &CoordinatorService{users: users, audit: audit}
}

func (s *CoordinatorService) Create(ctx context.Context, creatorID int64, creatorRole models.UserRole, req dto.CreateCoordinatorRequest, ip string) (*dto.CreateCoordinatorResponse, error) {
	if creatorRole != models.RoleMaster {
		return nil, ErrForbiddenAction
	}

	num, err := s.users.NextCoordinatorNumber(ctx)
	if err != nil {
		return nil, err
	}

	plainPassword, err := password.Generate(8)
	if err != nil {
		return nil, err
	}
	hash, err := password.Hash(plainPassword)
	if err != nil {
		return nil, err
	}

	email := fmt.Sprintf("coordinator%d@event.app", num)
	coordNum := num
	var assignedGate *string
	if req.GateName != nil && strings.TrimSpace(*req.GateName) != "" {
		gate := strings.TrimSpace(*req.GateName)
		assignedGate = &gate
	}
	user := &models.User{
		Email:             email,
		PasswordHash:      hash,
		Role:              models.RoleCoordinator,
		IsActive:          true,
		CoordinatorNumber: &coordNum,
		AssignedGate:      assignedGate,
		CreatedBy:         &creatorID,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}

	s.audit.Log(ctx, &creatorID, &creatorRole, models.AuditCreateCoordinator,
		fmt.Sprintf("Created coordinator %s", email), ip)

	return &dto.CreateCoordinatorResponse{
		ID:           user.ID,
		Email:        email,
		Password:     plainPassword,
		Role:         string(models.RoleCoordinator),
		AssignedGate: assignedGate,
	}, nil
}

func (s *CoordinatorService) List(ctx context.Context) ([]dto.CoordinatorResponse, error) {
	users, err := s.users.ListCoordinators(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.CoordinatorResponse, len(users))
	for i, u := range users {
		result[i] = dto.CoordinatorResponse{
			ID:                u.ID,
			Email:             u.Email,
			IsActive:          u.IsActive,
			CoordinatorNumber: u.CoordinatorNumber,
			AssignedGate:      u.AssignedGate,
			CreatedAt:         u.CreatedAt,
		}
	}
	return result, nil
}

func (s *CoordinatorService) Disable(ctx context.Context, actorID int64, actorRole models.UserRole, coordinatorID int64, ip string) error {
	if actorRole != models.RoleMaster {
		return ErrForbiddenAction
	}
	if err := s.users.SetActive(ctx, coordinatorID, models.RoleCoordinator, false); err != nil {
		return err
	}
	s.audit.Log(ctx, &actorID, &actorRole, models.AuditDisableCoordinator,
		fmt.Sprintf("Disabled coordinator ID %d", coordinatorID), ip)
	return nil
}

func (s *CoordinatorService) ResetPassword(ctx context.Context, actorID int64, actorRole models.UserRole, coordinatorID int64, ip string) (*dto.ResetPasswordResponse, error) {
	if actorRole != models.RoleMaster {
		return nil, ErrForbiddenAction
	}
	user, err := s.users.FindByID(ctx, coordinatorID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.Role != models.RoleCoordinator {
		return nil, fmt.Errorf("coordinator not found")
	}

	plainPassword, err := password.Generate(8)
	if err != nil {
		return nil, err
	}
	hash, err := password.Hash(plainPassword)
	if err != nil {
		return nil, err
	}
	if err := s.users.UpdatePassword(ctx, coordinatorID, hash); err != nil {
		return nil, err
	}

	role := actorRole
	s.audit.Log(ctx, &actorID, &role, models.AuditResetPassword,
		fmt.Sprintf("Reset password for coordinator %s", user.Email), ip)

	return &dto.ResetPasswordResponse{Email: user.Email, NewPassword: plainPassword}, nil
}

type GuestService struct {
	guests        *repository.GuestRepository
	users         *repository.UserRepository
	qr            QRGenerator
	notifications NotificationSender
	event         config.EventConfig
	audit         *audit.Service
	ws            WebSocketBroadcaster
}

type QRGenerator interface {
	SignToken(guestUUID uuid.UUID) string
	GenerateGuestQR(ctx context.Context, input qr.GuestQRInput) (token, imageURL string, err error)
	RenderGuestQRPNG(input qr.GuestQRInput, token string) ([]byte, error)
	RegenerateCard(ctx context.Context, input qr.GuestQRInput, token string) (imageURL string, err error)
	IsPermanentURL(url *string) bool
}

type NotificationSender interface {
	SendGuestInvitation(ctx context.Context, guestName, phone, eventName, eventDate, eventLocation, qrImageURL string) error
}

type WebSocketBroadcaster interface {
	BroadcastGuestAdded(data interface{})
	BroadcastDashboardUpdated()
}

func NewGuestService(
	guests *repository.GuestRepository,
	users *repository.UserRepository,
	qr QRGenerator,
	notifications NotificationSender,
	event config.EventConfig,
	audit *audit.Service,
	ws WebSocketBroadcaster,
) *GuestService {
	return &GuestService{
		guests: guests, users: users, qr: qr, notifications: notifications,
		event: event, audit: audit, ws: ws,
	}
}
