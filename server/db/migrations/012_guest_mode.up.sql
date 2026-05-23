-- Allow guest users to have NULL github_id
ALTER TABLE users ALTER COLUMN github_id DROP NOT NULL;

-- Replace absolute unique constraint with a regular unique index.
-- Postgres allows multiple NULL values in a regular unique index (NULLs are not
-- considered equal), so guests can have github_id = NULL while OAuth login's
-- ON CONFLICT (github_id) still works (partial indexes cannot be inferred by
-- ON CONFLICT without an explicit WHERE clause).
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_github_id_key;
DROP INDEX IF EXISTS users_github_id_key;
CREATE UNIQUE INDEX users_github_id_key ON users (github_id);

-- Track when guest users expire
ALTER TABLE users ADD COLUMN expires_at TIMESTAMPTZ;

-- Fast lookup for cleanup job
CREATE INDEX idx_users_guest_expires ON users (role, expires_at)
  WHERE role = 'guest';

-- Fast session deletion by user_id during cleanup
CREATE INDEX idx_auth_sessions_user_id ON auth_sessions (user_id);
