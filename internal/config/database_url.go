package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// loadDatabaseConfig reads DB settings from DATABASE_URL (Render) or individual DB_* vars.
func loadDatabaseConfig(connMaxLifetime time.Duration) (DatabaseConfig, error) {
	dbURL := osGetenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = osGetenv("DATABASE_INTERNAL_URL")
	}

	if dbURL != "" {
		cfg, err := parseDatabaseURL(dbURL)
		if err != nil {
			return DatabaseConfig{}, err
		}
		cfg.MaxOpenConns = getEnvInt("DB_MAX_OPEN_CONNS", 25)
		cfg.MaxIdleConns = getEnvInt("DB_MAX_IDLE_CONNS", 10)
		cfg.ConnMaxLifetime = connMaxLifetime
		return cfg, nil
	}

	return DatabaseConfig{
		Host:            getEnv("DB_HOST", "localhost"),
		Port:            getEnv("DB_PORT", "5432"),
		User:            getEnv("DB_USER", "postgres"),
		Password:        getEnv("DB_PASSWORD", "postgres"),
		Name:            getEnv("DB_NAME", "event_entry"),
		SSLMode:         getEnv("DB_SSLMODE", "disable"),
		MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime: connMaxLifetime,
	}, nil
}

func parseDatabaseURL(raw string) (DatabaseConfig, error) {
	// Render sometimes uses postgres:// — normalize for net/url
	normalized := raw
	if strings.HasPrefix(normalized, "postgres://") {
		normalized = "postgresql://" + strings.TrimPrefix(normalized, "postgres://")
	}

	u, err := url.Parse(normalized)
	if err != nil {
		return DatabaseConfig{}, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "5432"
	}

	user := u.User.Username()
	password, _ := u.User.Password()

	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return DatabaseConfig{}, fmt.Errorf("DATABASE_URL missing database name")
	}

	sslMode := u.Query().Get("sslmode")
	if sslMode == "" {
		// Render internal URLs often omit sslmode; require is safe for hosted Postgres.
		sslMode = "require"
	}

	return DatabaseConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Name:     dbName,
		SSLMode:  sslMode,
	}, nil
}

// osGetenv wraps os.Getenv for testability.
func osGetenv(key string) string {
	return getEnv(key, "")
}
