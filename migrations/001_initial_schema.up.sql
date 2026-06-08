-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users (Master and Coordinator)
CREATE TYPE user_role AS ENUM ('master', 'coordinator');

CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    email           VARCHAR(255) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    role            user_role NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    coordinator_number INT,
    created_by      BIGINT REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);

-- Guests
CREATE TABLE guests (
    id              BIGSERIAL PRIMARY KEY,
    uuid            UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    phone_number    VARCHAR(50),
    email           VARCHAR(255),
    qr_token        VARCHAR(512) NOT NULL UNIQUE,
    qr_image_url    VARCHAR(1024),
    is_checked_in   BOOLEAN NOT NULL DEFAULT FALSE,
    checked_in_at   TIMESTAMPTZ,
    checked_in_by   BIGINT REFERENCES users(id),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_guests_phone_number ON guests(phone_number);
CREATE INDEX idx_guests_email ON guests(email);
CREATE INDEX idx_guests_qr_token ON guests(qr_token);
CREATE INDEX idx_guests_name ON guests(name);
CREATE INDEX idx_guests_is_checked_in ON guests(is_checked_in);
CREATE INDEX idx_guests_metadata ON guests USING GIN(metadata);

-- Scan attempts (for duplicate/failed scan tracking)
CREATE TABLE scan_attempts (
    id              BIGSERIAL PRIMARY KEY,
    guest_id        BIGINT REFERENCES guests(id),
    qr_token        VARCHAR(512),
    user_id         BIGINT NOT NULL REFERENCES users(id),
    result          VARCHAR(50) NOT NULL,
    gate_name       VARCHAR(100),
    ip_address      VARCHAR(45),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scan_attempts_guest_id ON scan_attempts(guest_id);
CREATE INDEX idx_scan_attempts_user_id ON scan_attempts(user_id);
CREATE INDEX idx_scan_attempts_result ON scan_attempts(result);
CREATE INDEX idx_scan_attempts_created_at ON scan_attempts(created_at);

-- Refresh tokens
CREATE TABLE refresh_tokens (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      VARCHAR(255) NOT NULL UNIQUE,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);

-- Audit logs
CREATE TYPE audit_action AS ENUM (
    'LOGIN',
    'SCAN',
    'CREATE_GUEST',
    'UPDATE_GUEST',
    'DELETE_GUEST',
    'CREATE_COORDINATOR',
    'DISABLE_COORDINATOR',
    'RESET_PASSWORD',
    'EXPORT_REPORT',
    'IMPORT_GUESTS'
);

CREATE TABLE audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT REFERENCES users(id),
    role            user_role,
    action          audit_action NOT NULL,
    description     TEXT,
    ip_address      VARCHAR(45),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);

-- Coordinator counter for auto email generation
CREATE SEQUENCE coordinator_email_seq START 1;

-- Seed default master user (password: Master@123)
-- bcrypt hash of Master@123
INSERT INTO users (email, password_hash, role, is_active)
VALUES (
    'master@event.app',
    '$2a$10$K2AbVJtWXC4dCWLvWWIu0OjEJ1xTUTX1j/yeeuWEhvcvoBqE3U392',
    'master',
    TRUE
);
