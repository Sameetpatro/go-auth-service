-- Add leader role to user_role enum
ALTER TYPE user_role ADD VALUE IF NOT EXISTS 'leader';

-- Track which leader created each guest
ALTER TABLE guests ADD COLUMN IF NOT EXISTS created_by BIGINT REFERENCES users(id);
CREATE INDEX IF NOT EXISTS idx_guests_created_by ON guests(created_by);

-- Optional display name for leaders
ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name VARCHAR(100);

-- Extend audit actions
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'CREATE_LEADER';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'DISABLE_LEADER';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'INVITE_GUESTS';
