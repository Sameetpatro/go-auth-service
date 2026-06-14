package dto

import (
	"time"

	"github.com/google/uuid"
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
	User         UserResponse `json:"user"`
}

type UserResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type CreateCoordinatorResponse struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type CreateLeaderRequest struct {
	Username         string  `json:"username" binding:"required,min=2,max=50"`
	Password         *string `json:"password,omitempty" binding:"omitempty,min=8"`
	GeneratePassword bool    `json:"generate_password"`
}

type CreateLeaderResponse struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type LeaderResponse struct {
	ID          int64     `json:"id"`
	Email       string    `json:"email"`
	Username    string    `json:"username"`
	IsActive    bool      `json:"is_active"`
	GuestCount  int64     `json:"guest_count"`
	CheckedIn   int64     `json:"checked_in_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type CoordinatorResponse struct {
	ID                int64     `json:"id"`
	Email             string    `json:"email"`
	IsActive          bool      `json:"is_active"`
	CoordinatorNumber *int      `json:"coordinator_number,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type ResetPasswordResponse struct {
	Email       string `json:"email"`
	NewPassword string `json:"new_password"`
}

type CreateGuestRequest struct {
	Name        string                 `json:"name" binding:"required,min=1,max=255"`
	PhoneNumber *string                `json:"phone_number,omitempty"`
	Email       *string                `json:"email,omitempty" binding:"omitempty,email"`
	Address     *string                `json:"address,omitempty"`
	College     *string                `json:"college,omitempty"`
	LeaderID    *int64                 `json:"leader_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type UpdateGuestRequest struct {
	Name        *string                `json:"name,omitempty" binding:"omitempty,min=1,max=255"`
	PhoneNumber *string                `json:"phone_number,omitempty"`
	Email       *string                `json:"email,omitempty" binding:"omitempty,email"`
	Address     *string                `json:"address,omitempty"`
	College     *string                `json:"college,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type GuestResponse struct {
	ID              int64                  `json:"id"`
	UUID            uuid.UUID              `json:"uuid"`
	Name            string                 `json:"name"`
	PhoneNumber     *string                `json:"phone_number,omitempty"`
	Email           *string                `json:"email,omitempty"`
	QRImageURL      *string                `json:"qr_image_url,omitempty"`
	IsCheckedIn     bool                   `json:"is_checked_in"`
	CheckedInAt     *time.Time             `json:"checked_in_at,omitempty"`
	CheckedInBy     *int64                 `json:"checked_in_by,omitempty"`
	CheckedInByEmail *string               `json:"checked_in_by_email,omitempty"`
	CreatedBy        *int64                `json:"created_by,omitempty"`
	CreatedByEmail   *string               `json:"created_by_email,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

type GuestSearchResponse struct {
	Guest     GuestResponse `json:"guest"`
	QRStatus  string        `json:"qr_status"`
	EntryStatus string      `json:"entry_status"`
}

type ScanRequest struct {
	QRToken  string  `json:"qr_token" binding:"required"`
	GateName *string `json:"gate_name,omitempty"`
}

type ScanResponse struct {
	Result            string     `json:"result"`
	Guest             *GuestResponse `json:"guest,omitempty"`
	CheckedInAt       *time.Time `json:"checked_in_at,omitempty"`
	CheckedInBy       *int64     `json:"checked_in_by,omitempty"`
	CheckedInByEmail  *string    `json:"checked_in_by_email,omitempty"`
	Message           string     `json:"message"`
}

type ImportResult struct {
	TotalRows   int      `json:"total_rows"`
	Imported    int      `json:"imported"`
	Failed      int      `json:"failed"`
	Errors      []string `json:"errors,omitempty"`
}

type GuestInvitation struct {
	GuestID     int64  `json:"guest_id"`
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
	Message     string `json:"message"`
	QRImageURL  string `json:"qr_image_url,omitempty"`
}

type InviteAllResult struct {
	Total         int               `json:"total"`
	Sent          int               `json:"sent"`
	Skipped       int               `json:"skipped"`
	Failed        int               `json:"failed"`
	Invitations   []GuestInvitation `json:"invitations"`
	SkippedGuests []string          `json:"skipped_guests,omitempty"`
	Errors        []string          `json:"errors,omitempty"`
}

type AnalyticsOverview struct {
	TotalGuests        int64   `json:"total_guests"`
	TotalCheckedIn     int64   `json:"total_checked_in"`
	TotalPending       int64   `json:"total_pending"`
	CheckInPercentage  float64 `json:"check_in_percentage"`
	TodayEntries       int64   `json:"today_entries"`
	VIPEntries         int64   `json:"vip_entries"`
}

type HourlyEntryCount struct {
	Hour  int   `json:"hour"`
	Count int64 `json:"count"`
}

type CoordinatorEntryCount struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Count  int64  `json:"count"`
}

type AnalyticsResponse struct {
	Overview            AnalyticsOverview       `json:"overview"`
	HourlyEntryCount    []HourlyEntryCount      `json:"hourly_entry_count"`
	CoordinatorEntries  []CoordinatorEntryCount `json:"coordinator_entries"`
	LeaderEntries       []CoordinatorEntryCount `json:"leader_entries"`
	MasterEntries       []CoordinatorEntryCount `json:"master_entries"`
	LeaderGuestStats    []LeaderGuestStats      `json:"leader_guest_stats,omitempty"`
}

type LeaderGuestStats struct {
	UserID        int64  `json:"user_id"`
	Email         string `json:"email"`
	Username      string `json:"username"`
	TotalGuests   int64  `json:"total_guests"`
	CheckedIn     int64  `json:"checked_in"`
	PendingGuests int64  `json:"pending_guests"`
}

type InsightsResponse struct {
	GuestsAddedPerDay       []DailyCount            `json:"guests_added_per_day"`
	EntriesPerHour          []HourlyEntryCount      `json:"entries_per_hour"`
	MostActiveCoordinator   *CoordinatorEntryCount  `json:"most_active_coordinator"`
	PeakEntryTime           *string                 `json:"peak_entry_time"`
	PendingGuests           int64                   `json:"pending_guests"`
	DuplicateScanAttempts   int64                   `json:"duplicate_scan_attempts"`
	FailedScanAttempts      int64                   `json:"failed_scan_attempts"`
	TopScanningGates        []GateCount             `json:"top_scanning_gates"`
	AverageEntryRate        float64                 `json:"average_entry_rate"`
}

type DailyCount struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type GateCount struct {
	GateName string `json:"gate_name"`
	Count    int64  `json:"count"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type PaginationMeta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type PaginatedGuestsResponse struct {
	Data []GuestResponse `json:"data"`
	Meta PaginationMeta  `json:"meta"`
}
