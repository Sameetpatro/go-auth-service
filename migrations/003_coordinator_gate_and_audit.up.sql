-- Coordinator assigned gate (e.g. Gate 1, Gate 2)
ALTER TABLE users ADD COLUMN IF NOT EXISTS assigned_gate VARCHAR(100);

-- Extended audit actions
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'DELETE_LEADER';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'MANUAL_CHECK_IN';
ALTER TYPE audit_action ADD VALUE IF NOT EXISTS 'BULK_DELETE_GUESTS';
