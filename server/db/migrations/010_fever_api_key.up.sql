-- Stores SHA-256(MD5(username:password)), a 64-char hex string.
ALTER TABLE users ADD COLUMN fever_api_key CHAR(64);
CREATE INDEX idx_users_fever_api_key ON users (fever_api_key) WHERE fever_api_key IS NOT NULL;
