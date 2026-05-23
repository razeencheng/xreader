-- name: GuestSetArticleRead :one
INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
SELECT @state_owner_id, a.id, @is_read, now()
FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE a.id = @article_id AND s.user_id = @content_owner_id AND s.deleted_at IS NULL
ON CONFLICT (user_id, article_id) DO UPDATE SET
  is_read = @is_read,
  last_read_at = now()
RETURNING article_id;

-- name: GuestSetArticleStarred :one
INSERT INTO article_states (user_id, article_id, is_starred)
SELECT @state_owner_id, a.id, @is_starred
FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE a.id = @article_id AND s.user_id = @content_owner_id AND s.deleted_at IS NULL
ON CONFLICT (user_id, article_id) DO UPDATE SET
  is_starred = @is_starred
RETURNING article_id;

-- name: GuestUpdateReadingProgress :one
INSERT INTO article_states (user_id, article_id, reading_progress)
SELECT @state_owner_id, a.id, @reading_progress
FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE a.id = @article_id AND s.user_id = @content_owner_id AND s.deleted_at IS NULL
ON CONFLICT (user_id, article_id) DO UPDATE SET
  reading_progress = @reading_progress
RETURNING article_id;

-- name: GuestBatchSetReadToday :many
WITH upserted AS (
  INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
  SELECT @state_owner_id, a.id, @is_read, CASE WHEN @is_read THEN now() ELSE NULL END
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = @state_owner_id
  WHERE s.user_id = @content_owner_id AND a.published_at >= now() - interval '24 hours'
    AND s.deleted_at IS NULL
    AND COALESCE(st.is_read, false) <> @is_read
  ON CONFLICT (user_id, article_id) DO UPDATE SET
    is_read = @is_read,
    last_read_at = CASE WHEN @is_read THEN now() ELSE NULL END
  RETURNING article_id
), changes AS (
  INSERT INTO article_state_changes (user_id, article_id)
  SELECT @state_owner_id, article_id FROM upserted
)
SELECT article_id FROM upserted;

-- name: GuestBatchSetReadStream :many
WITH upserted AS (
  INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
  SELECT @state_owner_id, a.id, @is_read, CASE WHEN @is_read THEN now() ELSE NULL END
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = @state_owner_id
  WHERE s.user_id = @content_owner_id
    AND s.deleted_at IS NULL
    AND COALESCE(st.is_read, false) <> @is_read
  ON CONFLICT (user_id, article_id) DO UPDATE SET
    is_read = @is_read,
    last_read_at = CASE WHEN @is_read THEN now() ELSE NULL END
  RETURNING article_id
), changes AS (
  INSERT INTO article_state_changes (user_id, article_id)
  SELECT @state_owner_id, article_id FROM upserted
)
SELECT article_id FROM upserted;

-- name: GuestBatchSetReadBySource :many
WITH upserted AS (
  INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
  SELECT @state_owner_id, a.id, @is_read, CASE WHEN @is_read THEN now() ELSE NULL END
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = @state_owner_id
  WHERE s.id = @source_id AND s.user_id = @content_owner_id
    AND s.deleted_at IS NULL
    AND COALESCE(st.is_read, false) <> @is_read
  ON CONFLICT (user_id, article_id) DO UPDATE SET
    is_read = @is_read,
    last_read_at = CASE WHEN @is_read THEN now() ELSE NULL END
  RETURNING article_id
), changes AS (
  INSERT INTO article_state_changes (user_id, article_id)
  SELECT @state_owner_id, article_id FROM upserted
)
SELECT article_id FROM upserted;
