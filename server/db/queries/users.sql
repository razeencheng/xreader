-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByGithubID :one
SELECT * FROM users WHERE github_id = $1;

-- name: GetUserByGithubUsername :one
SELECT * FROM users WHERE github_username = $1;

-- name: UpsertUser :one
INSERT INTO users (github_id, github_username, avatar_url)
VALUES ($1, $2, $3)
ON CONFLICT (github_id) DO UPDATE SET
    github_username = EXCLUDED.github_username,
    avatar_url = EXCLUDED.avatar_url
RETURNING *;

-- name: UpdateUserSettings :one
UPDATE users
SET native_language = COALESCE(NULLIF($2, ''), native_language),
    density_pref = COALESCE(NULLIF($3, ''), density_pref),
    theme_pref = COALESCE(NULLIF($4, ''), theme_pref)
WHERE id = $1
RETURNING *;

-- name: ListDistinctNativeLanguages :many
SELECT DISTINCT native_language
FROM users
WHERE native_language <> ''
ORDER BY native_language;

-- name: UpdateUserRole :exec
UPDATE users SET role = $2 WHERE id = $1;

-- name: CreateSession :exec
INSERT INTO auth_sessions (id, user_id, user_agent)
VALUES ($1, $2, $3);

-- name: GetSession :one
SELECT * FROM auth_sessions WHERE id = $1;

-- name: TouchSession :exec
UPDATE auth_sessions SET last_seen_at = now() WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM auth_sessions WHERE id = $1;

-- name: DeleteUserSessions :exec
DELETE FROM auth_sessions WHERE user_id = $1;
