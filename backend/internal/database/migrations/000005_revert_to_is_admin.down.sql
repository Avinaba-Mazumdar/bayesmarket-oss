-- 000005_revert_to_is_admin.down.sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_superadmin BOOLEAN NOT NULL DEFAULT false;
UPDATE users SET is_superadmin = true WHERE is_admin = true;
DROP INDEX IF EXISTS idx_users_is_admin;
ALTER TABLE users DROP COLUMN IF EXISTS is_admin;
CREATE INDEX IF NOT EXISTS idx_users_is_superadmin ON users(is_superadmin) WHERE is_superadmin = true;
