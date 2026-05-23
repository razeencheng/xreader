-- name: SetArticleRead :one
INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
SELECT $1, a.id, $3, now()
FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE a.id = $2 AND s.user_id = $1 AND s.deleted_at IS NULL
ON CONFLICT (user_id, article_id) DO UPDATE SET
  is_read = $3,
  last_read_at = now()
RETURNING article_id;

-- name: SetArticleStarred :one
INSERT INTO article_states (user_id, article_id, is_starred)
SELECT $1, a.id, $3
FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE a.id = $2 AND s.user_id = $1 AND s.deleted_at IS NULL
ON CONFLICT (user_id, article_id) DO UPDATE SET
  is_starred = $3
RETURNING article_id;

-- name: GetArticleState :one
SELECT * FROM article_states
WHERE user_id = $1 AND article_id = $2;

-- name: UpdateReadingProgress :one
INSERT INTO article_states (user_id, article_id, reading_progress)
SELECT $1, a.id, $3
FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE a.id = $2 AND s.user_id = $1 AND s.deleted_at IS NULL
ON CONFLICT (user_id, article_id) DO UPDATE SET
  reading_progress = $3
RETURNING article_id;

-- name: RecordStateChange :exec
INSERT INTO article_state_changes (user_id, article_id)
VALUES ($1, $2);

-- name: ListStateChangesSince :many
SELECT sc.article_id,
       sc.changed_at,
       COALESCE(st.is_read, false)    AS is_read,
       COALESCE(st.is_starred, false) AS is_starred
FROM article_state_changes sc
LEFT JOIN article_states st
  ON st.user_id = sc.user_id AND st.article_id = sc.article_id
WHERE sc.user_id = $1 AND sc.changed_at > $2
ORDER BY sc.changed_at ASC;

-- name: BatchSetReadBySource :many
WITH upserted AS (
  INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
  SELECT $1, a.id, $3, CASE WHEN $3 THEN now() ELSE NULL END
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
  WHERE s.id = $2 AND s.user_id = $1
    AND s.deleted_at IS NULL
    AND COALESCE(st.is_read, false) <> $3
  ON CONFLICT (user_id, article_id) DO UPDATE SET
    is_read = $3,
    last_read_at = CASE WHEN $3 THEN now() ELSE NULL END
  RETURNING article_id
), changes AS (
  INSERT INTO article_state_changes (user_id, article_id)
  SELECT $1, article_id FROM upserted
)
SELECT article_id FROM upserted;

-- name: BatchSetReadToday :many
WITH upserted AS (
  INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
  SELECT $1, a.id, $2, CASE WHEN $2 THEN now() ELSE NULL END
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
  WHERE s.user_id = $1 AND a.published_at >= now() - interval '24 hours'
    AND s.deleted_at IS NULL
    AND COALESCE(st.is_read, false) <> $2
  ON CONFLICT (user_id, article_id) DO UPDATE SET
    is_read = $2,
    last_read_at = CASE WHEN $2 THEN now() ELSE NULL END
  RETURNING article_id
), changes AS (
  INSERT INTO article_state_changes (user_id, article_id)
  SELECT $1, article_id FROM upserted
)
SELECT article_id FROM upserted;

-- name: BatchSetReadStream :many
WITH upserted AS (
  INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
  SELECT $1, a.id, $2, CASE WHEN $2 THEN now() ELSE NULL END
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
  WHERE s.user_id = $1
    AND s.deleted_at IS NULL
    AND COALESCE(st.is_read, false) <> $2
  ON CONFLICT (user_id, article_id) DO UPDATE SET
    is_read = $2,
    last_read_at = CASE WHEN $2 THEN now() ELSE NULL END
  RETURNING article_id
), changes AS (
  INSERT INTO article_state_changes (user_id, article_id)
  SELECT $1, article_id FROM upserted
)
SELECT article_id FROM upserted;

-- name: MarkInitialSourceBacklogRead :many
WITH ranked AS (
  SELECT a.id,
         a.published_at,
         row_number() OVER (ORDER BY a.published_at DESC NULLS LAST, a.id DESC) AS rn
  FROM articles a
  WHERE a.source_id = $1
), upserted AS (
  INSERT INTO article_states (user_id, article_id, is_read, last_read_at)
  SELECT s.user_id, ranked.id, true, now()
  FROM ranked
  JOIN sources s ON s.id = $1 AND s.deleted_at IS NULL
  WHERE NOT (
    ranked.published_at >= now() - interval '7 days'
    OR ranked.rn <= 20
  )
  ON CONFLICT (user_id, article_id) DO UPDATE SET
    is_read = true,
    last_read_at = now()
  RETURNING article_id
), changes AS (
  INSERT INTO article_state_changes (user_id, article_id)
  SELECT s.user_id, article_id FROM upserted
  JOIN sources s ON s.id = $1 AND s.deleted_at IS NULL
)
SELECT article_id FROM upserted;
