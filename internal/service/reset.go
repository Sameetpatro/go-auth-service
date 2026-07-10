package service

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
)

// QRPurger bulk-deletes stored QR images (e.g. everything under the "qr/"
// prefix on Cloudinary). Nil when only local-disk storage is in use.
type QRPurger interface {
	DeleteAllPNGs(ctx context.Context, prefix string) error
}

type ResetService struct {
	db      *sqlx.DB
	qrPath  string
	qrPurge QRPurger
}

func NewResetService(db *sqlx.DB, qrImagePath string, qrPurge QRPurger) *ResetService {
	return &ResetService{db: db, qrPath: qrImagePath, qrPurge: qrPurge}
}

func (s *ResetService) ResetAllData(ctx context.Context) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmts := []string{
		`DELETE FROM scan_attempts`,
		`DELETE FROM guests`,
		`DELETE FROM audit_logs`,
		`DELETE FROM refresh_tokens`,
		`DELETE FROM users WHERE role IN ('coordinator', 'leader')`,
		`ALTER SEQUENCE coordinator_email_seq RESTART WITH 1`,
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("reset query failed (%s): %w", stmt, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if s.qrPath != "" {
		_ = os.RemoveAll(s.qrPath)
		_ = os.MkdirAll(s.qrPath, 0o755)
	}

	if s.qrPurge != nil {
		if err := s.qrPurge.DeleteAllPNGs(context.Background(), "qr/"); err != nil {
			// DB reset already succeeded; leftover images are harmless.
			log.Printf("reset: cloudinary purge failed: %v", err)
		}
	}

	return nil
}
