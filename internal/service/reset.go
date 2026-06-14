package service

import (
	"context"
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"
)

type ResetService struct {
	db        *sqlx.DB
	qrPath    string
}

func NewResetService(db *sqlx.DB, qrImagePath string) *ResetService {
	return &ResetService{db: db, qrPath: qrImagePath}
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

	return nil
}
