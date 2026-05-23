-- name: GetUserByFeverAPIKey :one
SELECT id, github_id, github_username, avatar_url, native_language, role, density_pref, theme_pref, created_at, fever_api_key
FROM users WHERE fever_api_key = $1;

-- name: SetFeverAPIKey :exec
UPDATE users SET fever_api_key = $2 WHERE id = $1;

-- name: FeverListSources :many
SELECT id, user_id, url, title, icon_url, category
FROM sources
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY id;

-- name: FeverListItems :many
SELECT a.id, a.source_id, a.title, a.author, a.content_html, a.link, a.published_at,
       COALESCE(st.is_read, false) AS is_read,
       COALESCE(st.is_starred, false) AS is_starred
FROM articles a
JOIN sources s ON a.source_id = s.id
LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
WHERE s.user_id = $1 AND s.deleted_at IS NULL
  AND ($2::bigint = 0 OR a.id > $2)
  AND ($3::bigint = 0 OR a.id < $3)
ORDER BY a.id DESC
LIMIT 50;

-- name: FeverListItemsByIDs :many
SELECT a.id, a.source_id, a.title, a.author, a.content_html, a.link, a.published_at,
       COALESCE(st.is_read, false) AS is_read,
       COALESCE(st.is_starred, false) AS is_starred
FROM articles a
JOIN sources s ON a.source_id = s.id
LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
WHERE s.user_id = $1 AND s.deleted_at IS NULL
  AND a.id = ANY($2::bigint[])
ORDER BY a.id DESC;

-- name: FeverUnreadItemIDs :many
SELECT a.id
FROM articles a
JOIN sources s ON a.source_id = s.id
LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
WHERE s.user_id = $1 AND s.deleted_at IS NULL
  AND (st.is_read IS NULL OR st.is_read = false)
ORDER BY a.id;

-- name: FeverSavedItemIDs :many
SELECT a.id
FROM articles a
JOIN sources s ON a.source_id = s.id
JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
WHERE s.user_id = $1 AND s.deleted_at IS NULL
  AND st.is_starred = true
ORDER BY a.id;

-- name: FeverBulkMarkFeedRead :exec
INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
SELECT $1, a.id, true, now()
FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE s.id = $2 AND s.user_id = $1 AND s.deleted_at IS NULL
  AND a.published_at < $3
ON CONFLICT (user_id, article_id) DO UPDATE SET
  is_read = true,
  last_read_at = now();

-- name: FeverBulkMarkAllRead :exec
INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
SELECT $1, a.id, true, now()
FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE s.user_id = $1 AND s.deleted_at IS NULL
  AND a.published_at < $2
ON CONFLICT (user_id, article_id) DO UPDATE SET
  is_read = true,
  last_read_at = now();
