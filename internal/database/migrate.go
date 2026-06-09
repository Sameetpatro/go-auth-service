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

	sqlBytes, err := os.ReadFile("migrations/001_initial_schema.up.sql")
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}

	if _, err := db.Exec(string(sqlBytes)); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate key") {
			return nil
		}
		return fmt.Errorf("run migration: %w", err)
	}
	return nil
}
