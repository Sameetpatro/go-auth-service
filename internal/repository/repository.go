package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/sameetpatro/go-qr-auth/internal/models"
)

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.db.GetContext(ctx, &user, `SELECT * FROM users WHERE email = $1`, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*models.User, error) {
	var user models.User
	err := r.db.GetContext(ctx, &user, `SELECT * FROM users WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) NextCoordinatorNumber(ctx context.Context) (int, error) {
	var num int
	err := r.db.GetContext(ctx, &num, `SELECT nextval('coordinator_email_seq')`)
	return num, err
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (email, password_hash, role, is_active, coordinator_number, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRowxContext(ctx, query,
		user.Email, user.PasswordHash, user.Role, user.IsActive,
		user.CoordinatorNumber, user.CreatedBy,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id int64, hash string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`,
		hash, id,
	)
	return err
}

func (r *UserRepository) SetActive(ctx context.Context, id int64, active bool) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET is_active = $1, updated_at = NOW() WHERE id = $2 AND role = 'coordinator'`,
		active, id,
	)
	return err
}

func (r *UserRepository) ListCoordinators(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.db.SelectContext(ctx, &users,
		`SELECT * FROM users WHERE role = 'coordinator' ORDER BY coordinator_number ASC`,
	)
	return users, err
}

type GuestRepository struct {
	db *sqlx.DB
}

func NewGuestRepository(db *sqlx.DB) *GuestRepository {
	return &GuestRepository{db: db}
}

func (r *GuestRepository) Create(ctx context.Context, guest *models.Guest) error {
	if guest.Metadata == nil {
		guest.Metadata = json.RawMessage(`{}`)
	}
	query := `
		INSERT INTO guests (uuid, name, phone_number, email, qr_token, qr_image_url, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRowxContext(ctx, query,
		guest.UUID, guest.Name, guest.PhoneNumber, guest.Email,
		guest.QRToken, guest.QRImageURL, guest.Metadata,
	).Scan(&guest.ID, &guest.CreatedAt, &guest.UpdatedAt)
}

func (r *GuestRepository) FindByID(ctx context.Context, id int64) (*models.GuestWithChecker, error) {
	var guest models.GuestWithChecker
	query := `
		SELECT g.*, u.email AS checked_in_by_email
		FROM guests g
		LEFT JOIN users u ON g.checked_in_by = u.id
		WHERE g.id = $1`
	err := r.db.GetContext(ctx, &guest, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &guest, nil
}

func (r *GuestRepository) FindByQRToken(ctx context.Context, token string) (*models.GuestWithChecker, error) {
	var guest models.GuestWithChecker
	query := `
		SELECT g.*, u.email AS checked_in_by_email
		FROM guests g
		LEFT JOIN users u ON g.checked_in_by = u.id
		WHERE g.qr_token = $1`
	err := r.db.GetContext(ctx, &guest, query, token)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &guest, nil
}

func (r *GuestRepository) Update(ctx context.Context, guest *models.Guest) error {
	if guest.Metadata == nil {
		guest.Metadata = json.RawMessage(`{}`)
	}
	query := `
		UPDATE guests SET name = $1, phone_number = $2, email = $3, metadata = $4, updated_at = NOW()
		WHERE id = $5 RETURNING updated_at`
	return r.db.QueryRowxContext(ctx, query,
		guest.Name, guest.PhoneNumber, guest.Email, guest.Metadata, guest.ID,
	).Scan(&guest.UpdatedAt)
}

func (r *GuestRepository) Delete(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM scan_attempts WHERE guest_id = $1`, id); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM guests WHERE id = $1`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *GuestRepository) Search(ctx context.Context, query string, limit, offset int) ([]models.GuestWithChecker, int64, error) {
	pattern := "%" + strings.TrimSpace(query) + "%"
	countQuery := `
		SELECT COUNT(*) FROM guests
		WHERE name ILIKE $1 OR phone_number ILIKE $1 OR email ILIKE $1`
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, pattern); err != nil {
		return nil, 0, err
	}

	selectQuery := `
		SELECT g.*, u.email AS checked_in_by_email
		FROM guests g
		LEFT JOIN users u ON g.checked_in_by = u.id
		WHERE g.name ILIKE $1 OR g.phone_number ILIKE $1 OR g.email ILIKE $1
		ORDER BY g.name ASC
		LIMIT $2 OFFSET $3`
	var guests []models.GuestWithChecker
	if err := r.db.SelectContext(ctx, &guests, selectQuery, pattern, limit, offset); err != nil {
		return nil, 0, err
	}
	return guests, total, nil
}

func (r *GuestRepository) List(ctx context.Context, limit, offset int) ([]models.GuestWithChecker, int64, error) {
	var total int64
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM guests`); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT g.*, u.email AS checked_in_by_email
		FROM guests g
		LEFT JOIN users u ON g.checked_in_by = u.id
		ORDER BY g.created_at DESC
		LIMIT $1 OFFSET $2`
	var guests []models.GuestWithChecker
	if err := r.db.SelectContext(ctx, &guests, query, limit, offset); err != nil {
		return nil, 0, err
	}
	return guests, total, nil
}

// CheckIn performs atomic check-in with SELECT FOR UPDATE to prevent race conditions.
func (r *GuestRepository) CheckIn(ctx context.Context, qrToken string, userID int64) (*models.GuestWithChecker, models.ScanResult, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback() //nolint:errcheck

	var guest models.Guest
	lockQuery := `SELECT * FROM guests WHERE qr_token = $1 FOR UPDATE`
	err = tx.GetContext(ctx, &guest, lockQuery, qrToken)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, models.ScanResultEntryDenied, nil
	}
	if err != nil {
		return nil, "", err
	}

	if guest.IsCheckedIn {
		var result models.GuestWithChecker
		fetchQuery := `
			SELECT g.*, u.email AS checked_in_by_email
			FROM guests g
			LEFT JOIN users u ON g.checked_in_by = u.id
			WHERE g.id = $1`
		if err := tx.GetContext(ctx, &result, fetchQuery, guest.ID); err != nil {
			return nil, "", err
		}
		if err := tx.Commit(); err != nil {
			return nil, "", err
		}
		return &result, models.ScanResultAlreadyEntered, nil
	}

	now := time.Now().UTC()
	updateQuery := `
		UPDATE guests SET is_checked_in = TRUE, checked_in_at = $1, checked_in_by = $2, updated_at = NOW()
		WHERE id = $3 AND is_checked_in = FALSE`
	result, err := tx.ExecContext(ctx, updateQuery, now, userID, guest.ID)
	if err != nil {
		return nil, "", err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, "", err
	}
	if rows == 0 {
		var checked models.GuestWithChecker
		fetchQuery := `
			SELECT g.*, u.email AS checked_in_by_email
			FROM guests g
			LEFT JOIN users u ON g.checked_in_by = u.id
			WHERE g.id = $1`
		if err := tx.GetContext(ctx, &checked, fetchQuery, guest.ID); err != nil {
			return nil, "", err
		}
		if err := tx.Commit(); err != nil {
			return nil, "", err
		}
		return &checked, models.ScanResultAlreadyEntered, nil
	}

	var updated models.GuestWithChecker
	fetchQuery := `
		SELECT g.*, u.email AS checked_in_by_email
		FROM guests g
		LEFT JOIN users u ON g.checked_in_by = u.id
		WHERE g.id = $1`
	if err := tx.GetContext(ctx, &updated, fetchQuery, guest.ID); err != nil {
		return nil, "", err
	}

	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	return &updated, models.ScanResultEntryAllowed, nil
}

type ScanRepository struct {
	db *sqlx.DB
}

func NewScanRepository(db *sqlx.DB) *ScanRepository {
	return &ScanRepository{db: db}
}

func (r *ScanRepository) Create(ctx context.Context, attempt *models.ScanAttempt) error {
	query := `
		INSERT INTO scan_attempts (guest_id, qr_token, user_id, result, gate_name, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`
	return r.db.QueryRowxContext(ctx, query,
		attempt.GuestID, attempt.QRToken, attempt.UserID, attempt.Result,
		attempt.GateName, attempt.IPAddress,
	).Scan(&attempt.ID, &attempt.CreatedAt)
}

type RefreshTokenRepository struct {
	db *sqlx.DB
}

func NewRefreshTokenRepository(db *sqlx.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3) RETURNING id, created_at`
	return r.db.QueryRowxContext(ctx, query,
		token.UserID, token.TokenHash, token.ExpiresAt,
	).Scan(&token.ID, &token.CreatedAt)
}

func (r *RefreshTokenRepository) FindByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	err := r.db.GetContext(ctx, &token,
		`SELECT * FROM refresh_tokens WHERE token_hash = $1 AND expires_at > NOW()`, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *RefreshTokenRepository) DeleteByHash(ctx context.Context, hash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token_hash = $1`, hash)
	return err
}

func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE expires_at <= NOW()`)
	return err
}

type AuditRepository struct {
	db *sqlx.DB
}

func NewAuditRepository(db *sqlx.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(ctx context.Context, log *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (user_id, role, action, description, ip_address)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`
	return r.db.QueryRowxContext(ctx, query,
		log.UserID, log.Role, log.Action, log.Description, log.IPAddress,
	).Scan(&log.ID, &log.CreatedAt)
}

type AnalyticsRepository struct {
	db *sqlx.DB
}

func NewAnalyticsRepository(db *sqlx.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

func (r *AnalyticsRepository) TotalGuests(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM guests`)
	return count, err
}

func (r *AnalyticsRepository) TotalCheckedIn(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM guests WHERE is_checked_in = TRUE`)
	return count, err
}

func (r *AnalyticsRepository) TodayEntries(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM guests WHERE is_checked_in = TRUE AND checked_in_at::date = CURRENT_DATE`)
	return count, err
}

func (r *AnalyticsRepository) VIPEntries(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM guests WHERE is_checked_in = TRUE AND metadata->>'vip_status' = 'true'`)
	return count, err
}

func (r *AnalyticsRepository) HourlyEntryCount(ctx context.Context) ([]struct {
	Hour  int   `db:"hour"`
	Count int64 `db:"count"`
}, error) {
	var results []struct {
		Hour  int   `db:"hour"`
		Count int64 `db:"count"`
	}
	query := `
		SELECT EXTRACT(HOUR FROM checked_in_at)::int AS hour, COUNT(*) AS count
		FROM guests
		WHERE is_checked_in = TRUE AND checked_in_at IS NOT NULL
		GROUP BY hour ORDER BY hour`
	err := r.db.SelectContext(ctx, &results, query)
	return results, err
}

func (r *AnalyticsRepository) EntriesByRole(ctx context.Context, role models.UserRole) ([]struct {
	UserID int64  `db:"user_id"`
	Email  string `db:"email"`
	Count  int64  `db:"count"`
}, error) {
	var results []struct {
		UserID int64  `db:"user_id"`
		Email  string `db:"email"`
		Count  int64  `db:"count"`
	}
	query := `
		SELECT u.id AS user_id, u.email, COUNT(g.id) AS count
		FROM guests g
		JOIN users u ON g.checked_in_by = u.id
		WHERE u.role = $1 AND g.is_checked_in = TRUE
		GROUP BY u.id, u.email
		ORDER BY count DESC`
	err := r.db.SelectContext(ctx, &results, query, role)
	return results, err
}

func (r *AnalyticsRepository) GuestsAddedPerDay(ctx context.Context, days int) ([]struct {
	Date  string `db:"date"`
	Count int64  `db:"count"`
}, error) {
	var results []struct {
		Date  string `db:"date"`
		Count int64  `db:"count"`
	}
	query := fmt.Sprintf(`
		SELECT TO_CHAR(created_at::date, 'YYYY-MM-DD') AS date, COUNT(*) AS count
		FROM guests
		WHERE created_at >= CURRENT_DATE - INTERVAL '%d days'
		GROUP BY created_at::date ORDER BY date`, days)
	err := r.db.SelectContext(ctx, &results, query)
	return results, err
}

func (r *AnalyticsRepository) DuplicateScanAttempts(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM scan_attempts WHERE result = 'ALREADY_ENTERED'`)
	return count, err
}

func (r *AnalyticsRepository) FailedScanAttempts(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM scan_attempts WHERE result = 'ENTRY_DENIED'`)
	return count, err
}

func (r *AnalyticsRepository) TopScanningGates(ctx context.Context, limit int) ([]struct {
	GateName string `db:"gate_name"`
	Count    int64  `db:"count"`
}, error) {
	var results []struct {
		GateName string `db:"gate_name"`
		Count    int64  `db:"count"`
	}
	query := `
		SELECT COALESCE(gate_name, 'Unknown') AS gate_name, COUNT(*) AS count
		FROM scan_attempts
		WHERE result = 'ENTRY_ALLOWED'
		GROUP BY gate_name ORDER BY count DESC LIMIT $1`
	err := r.db.SelectContext(ctx, &results, query, limit)
	return results, err
}

func (r *AnalyticsRepository) PeakEntryHour(ctx context.Context) (*int, error) {
	var hour sql.NullInt64
	query := `
		SELECT EXTRACT(HOUR FROM checked_in_at)::int AS hour
		FROM guests WHERE is_checked_in = TRUE AND checked_in_at IS NOT NULL
		GROUP BY hour ORDER BY COUNT(*) DESC LIMIT 1`
	err := r.db.GetContext(ctx, &hour, query)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !hour.Valid {
		return nil, nil
	}
	h := int(hour.Int64)
	return &h, nil
}

func (r *AnalyticsRepository) AverageEntryRate(ctx context.Context) (float64, error) {
	var rate sql.NullFloat64
	query := `
		SELECT CASE
			WHEN COUNT(DISTINCT DATE(checked_in_at)) = 0 THEN 0
			ELSE COUNT(*)::float / COUNT(DISTINCT DATE(checked_in_at))
		END AS rate
		FROM guests WHERE is_checked_in = TRUE AND checked_in_at IS NOT NULL`
	err := r.db.GetContext(ctx, &rate, query)
	if err != nil {
		return 0, err
	}
	if !rate.Valid {
		return 0, nil
	}
	return rate.Float64, nil
}
