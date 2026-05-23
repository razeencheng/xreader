DROP INDEX IF EXISTS idx_auth_sessions_user_id;
DROP INDEX IF EXISTS idx_users_guest_expires;
ALTER TABLE users DROP COLUMN IF EXISTS expires_at;

-- Remove guest users before restoring constraint
DELETE FROM highlights WHERE user_id IN (SELECT id FROM users WHERE role = 'guest');
DELETE FROM article_state_changes WHERE user_id IN (SELECT id FROM users WHERE role = 'guest');
DELETE FROM users WHERE role = 'guest';

DROP INDEX IF EXISTS users_github_id_key;
ALTER TABLE users ALTER COLUMN github_id SET NOT NULL;
CREATE UNIQUE INDEX users_github_id_key ON users (github_id);
