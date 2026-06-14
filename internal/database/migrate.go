package database

import (
	"fmt"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
)

// RunMigrations applies the initial schema when AUTO_MIGRATE=true (useful on Render).
func RunMigrations(db *sqlx.DB) error {
	if os.Getenv("AUTO_MIGRATE") != "true" {
		return nil
	}

	migrations := []string{
		"migrations/001_initial_schema.up.sql",
		"migrations/002_roles_and_guest_ownership.up.sql",
	}

	for _, path := range migrations {
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", path, err)
		}

		if _, err := db.Exec(string(sqlBytes)); err != nil {
			msg := err.Error()
			if strings.Contains(msg, "already exists") ||
				strings.Contains(msg, "duplicate key") ||
				strings.Contains(msg, "enum label") {
				continue
			}
			return fmt.Errorf("run migration %s: %w", path, err)
		}
	}
	return nil
}
