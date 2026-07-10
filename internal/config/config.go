package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Event    EventConfig
	Storage  StorageConfig
	RateLimit RateLimitConfig
	CORS     CORSConfig
}

type ServerConfig struct {
	Port        string
	Environment string
	BaseURL     string
}

type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

type EventConfig struct {
	Name     string
	Date     string
	Location string
}

type StorageConfig struct {
	QRImagePath string
	QRImageURL  string
	// CloudinaryURL enables permanent QR image storage on Cloudinary
	// (format: cloudinary://api_key:api_secret@cloud_name). When empty,
	// images are written to the local filesystem instead.
	CloudinaryURL string
}

type RateLimitConfig struct {
	RequestsPerMinute int
}

type CORSConfig struct {
	AllowedOrigins []string
}

func Load() (*Config, error) {
	accessExpiry, err := time.ParseDuration(getEnv("JWT_ACCESS_EXPIRY", "24h"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_EXPIRY: %w", err)
	}
	refreshExpiry, err := time.ParseDuration(getEnv("JWT_REFRESH_EXPIRY", "720h"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_EXPIRY: %w", err)
	}
	connMaxLifetime, err := time.ParseDuration(getEnv("DB_CONN_MAX_LIFETIME", "5m"))
	if err != nil {
		return nil, fmt.Errorf("invalid DB_CONN_MAX_LIFETIME: %w", err)
	}

	dbCfg, err := loadDatabaseConfig(connMaxLifetime)
	if err != nil {
		return nil, err
	}

	// Render sets PORT; fall back to SERVER_PORT for local dev.
	port := getEnv("PORT", getEnv("SERVER_PORT", "8080"))

	cfg := &Config{
		Server: ServerConfig{
			Port:        port,
			Environment: getEnv("ENVIRONMENT", "development"),
			BaseURL:     getEnv("SERVER_BASE_URL", "http://localhost:8080"),
		},
		Database: dbCfg,
		JWT: JWTConfig{
			AccessSecret:  getEnv("JWT_ACCESS_SECRET", "change-me-access-secret-min-32-chars!!"),
			RefreshSecret: getEnv("JWT_REFRESH_SECRET", "change-me-refresh-secret-min-32-chars!"),
			AccessExpiry:  accessExpiry,
			RefreshExpiry: refreshExpiry,
		},
		Event: EventConfig{
			Name:     getEnv("EVENT_NAME", "FELICITATIONS & VEER GOKULA MANCHAN"),
			Date:     getEnv("EVENT_DATE", "19 July 2026 | 2:00 PM"),
			Location: getEnv("EVENT_LOCATION", "Bharat Mandapam, New Delhi"),
		},
		Storage: StorageConfig{
			QRImagePath:   getEnv("QR_IMAGE_PATH", "./storage/qr"),
			QRImageURL:    resolveQRImageURL(getEnv("QR_IMAGE_URL", ""), getEnv("SERVER_BASE_URL", "http://localhost:8080")),
			CloudinaryURL: getEnv("CLOUDINARY_URL", ""),
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: getEnvInt("RATE_LIMIT_RPM", 100),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "*")),
		},
	}

	return cfg, nil
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			part := s[start:i]
			if part != "" {
				result = append(result, part)
			}
			start = i + 1
		}
	}
	return result
}

func resolveQRImageURL(qrImageURL, serverBaseURL string) string {
	qrImageURL = strings.TrimSpace(qrImageURL)
	if qrImageURL != "" && !strings.Contains(qrImageURL, "localhost") {
		return strings.TrimRight(qrImageURL, "/")
	}
	base := strings.TrimRight(strings.TrimSpace(serverBaseURL), "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return base + "/storage/qr"
}
