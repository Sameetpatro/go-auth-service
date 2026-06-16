package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleMaster      UserRole = "master"
	RoleLeader      UserRole = "leader"
	RoleCoordinator UserRole = "coordinator"
)

type User struct {
	ID                int64      `db:"id" json:"id"`
	Email             string     `db:"email" json:"email"`
	PasswordHash      string     `db:"password_hash" json:"-"`
	Role              UserRole   `db:"role" json:"role"`
	IsActive          bool       `db:"is_active" json:"is_active"`
	DisplayName       *string    `db:"display_name" json:"display_name,omitempty"`
	CoordinatorNumber *int       `db:"coordinator_number" json:"coordinator_number,omitempty"`
	AssignedGate      *string    `db:"assigned_gate" json:"assigned_gate,omitempty"`
	CreatedBy         *int64     `db:"created_by" json:"created_by,omitempty"`
	CreatedAt         time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at" json:"updated_at"`
}

type Guest struct {
	ID           int64           `db:"id" json:"id"`
	UUID         uuid.UUID       `db:"uuid" json:"uuid"`
	Name         string          `db:"name" json:"name"`
	PhoneNumber  *string         `db:"phone_number" json:"phone_number,omitempty"`
	Email        *string         `db:"email" json:"email,omitempty"`
	QRToken      string          `db:"qr_token" json:"-"`
	QRImageURL   *string         `db:"qr_image_url" json:"qr_image_url,omitempty"`
	IsCheckedIn  bool            `db:"is_checked_in" json:"is_checked_in"`
	CheckedInAt  *time.Time      `db:"checked_in_at" json:"checked_in_at,omitempty"`
	CheckedInBy  *int64          `db:"checked_in_by" json:"checked_in_by,omitempty"`
	CreatedBy    *int64          `db:"created_by" json:"created_by,omitempty"`
	Metadata     json.RawMessage `db:"metadata" json:"metadata,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
}

type GuestWithChecker struct {
	Guest
	CheckedInByEmail  *string `db:"checked_in_by_email" json:"checked_in_by_email,omitempty"`
	CreatedByEmail    *string `db:"created_by_email" json:"created_by_email,omitempty"`
}

type ScanAttempt struct {
	ID        int64     `db:"id" json:"id"`
	GuestID   *int64    `db:"guest_id" json:"guest_id,omitempty"`
	QRToken   *string   `db:"qr_token" json:"qr_token,omitempty"`
	UserID    int64     `db:"user_id" json:"user_id"`
	Result    string    `db:"result" json:"result"`
	GateName  *string   `db:"gate_name" json:"gate_name,omitempty"`
	IPAddress *string   `db:"ip_address" json:"ip_address,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type RefreshToken struct {
	ID        int64     `db:"id" json:"id"`
	UserID    int64     `db:"user_id" json:"user_id"`
	TokenHash string    `db:"token_hash" json:"-"`
	ExpiresAt time.Time `db:"expires_at" json:"expires_at"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type AuditAction string

const (
	AuditLogin              AuditAction = "LOGIN"
	AuditScan               AuditAction = "SCAN"
	AuditCreateGuest        AuditAction = "CREATE_GUEST"
	AuditUpdateGuest        AuditAction = "UPDATE_GUEST"
	AuditDeleteGuest        AuditAction = "DELETE_GUEST"
	AuditCreateCoordinator  AuditAction = "CREATE_COORDINATOR"
	AuditCreateLeader       AuditAction = "CREATE_LEADER"
	AuditDisableCoordinator AuditAction = "DISABLE_COORDINATOR"
	AuditDisableLeader      AuditAction = "DISABLE_LEADER"
	AuditResetPassword      AuditAction = "RESET_PASSWORD"
	AuditExportReport       AuditAction = "EXPORT_REPORT"
	AuditImportGuests       AuditAction = "IMPORT_GUESTS"
	AuditInviteGuests       AuditAction = "INVITE_GUESTS"
	AuditDeleteLeader       AuditAction = "DELETE_LEADER"
	AuditManualCheckIn      AuditAction = "MANUAL_CHECK_IN"
	AuditBulkDeleteGuests   AuditAction = "BULK_DELETE_GUESTS"
)

type AuditLog struct {
	ID          int64       `db:"id" json:"id"`
	UserID      *int64      `db:"user_id" json:"user_id,omitempty"`
	Role        *UserRole   `db:"role" json:"role,omitempty"`
	Action      AuditAction `db:"action" json:"action"`
	Description *string     `db:"description" json:"description,omitempty"`
	IPAddress   *string     `db:"ip_address" json:"ip_address,omitempty"`
	CreatedAt   time.Time   `db:"created_at" json:"created_at"`
}

type ScanResult string

const (
	ScanResultEntryAllowed   ScanResult = "ENTRY_ALLOWED"
	ScanResultAlreadyEntered ScanResult = "ALREADY_ENTERED"
	ScanResultEntryDenied    ScanResult = "ENTRY_DENIED"
)
