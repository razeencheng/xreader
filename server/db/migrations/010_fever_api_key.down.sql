DROP INDEX IF EXISTS idx_users_fever_api_key;
ALTER TABLE users DROP COLUMN IF EXISTS fever_api_key;
