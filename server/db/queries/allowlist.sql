-- name: ListAllowlist :many
SELECT * FROM auth_allowlist ORDER BY added_at DESC;

-- name: GetAllowlistEntry :one
SELECT * FROM auth_allowlist WHERE github_username = $1;

-- name: AddToAllowlist :exec
INSERT INTO auth_allowlist (github_username, added_by_user_id, note)
VALUES ($1, $2, $3)
ON CONFLICT (github_username) DO NOTHING;

-- name: RemoveFromAllowlist :exec
DELETE FROM auth_allowlist WHERE github_username = $1;

-- name: IsAllowlisted :one
SELECT EXISTS(SELECT 1 FROM auth_allowlist WHERE github_username = $1) AS is_allowed;
