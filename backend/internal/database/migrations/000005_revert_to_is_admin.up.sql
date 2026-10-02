-- 000005_revert_to_is_admin.up.sql
-- Revert from is_superadmin back to is_admin as the single authoritative column

ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;

-- Migrate any is_superadmin flags over to is_admin
UPDATE users SET is_admin = true WHERE is_superadmin = true;

-- Drop is_superadmin index and column
DROP INDEX IF EXISTS idx_users_is_superadmin;
ALTER TABLE users DROP COLUMN IF EXISTS is_superadmin;

-- Ensure partial index on is_admin exists for fast permission lookups
CREATE INDEX IF NOT EXISTS idx_users_is_admin ON users(is_admin) WHERE is_admin = true;
E