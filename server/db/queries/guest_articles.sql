-- name: GuestListArticlesTodayEnriched :many
WITH ranked AS (
  SELECT a.id, a.source_id, a.title, a.link, a.language, a.author, a.published_at, a.content_text,
         COALESCE(ai.title_translated, '') AS title_translated,
         COALESCE(ai.summary, '') AS summary,
         s.title AS source_title,
         COALESCE(st.is_read, false) AS is_read,
         COALESCE(st.is_starred, false) AS is_starred,
         row_number() OVER (PARTITION BY a.normalized_link ORDER BY a.published_at DESC, a.id DESC) AS rn
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_ai ai ON ai.article_id = a.id AND ai.target_language = @target_language
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = @state_owner_id
  WHERE s.user_id = @content_owner_id
    AND s.deleted_at IS NULL
    AND a.published_at >= now() - interval '24 hours'
    AND (
      @read_filter::text = 'all'
      OR (@read_filter::text = 'unread' AND COALESCE(st.is_read, false) = false)
      OR (@read_filter::text = 'read' AND COALESCE(st.is_read, false) = true)
    )
)
SELECT id, source_id, title, link, language, author, published_at, content_text,
       title_translated, summary, source_title, is_read, is_starred
FROM ranked
WHERE rn = 1
ORDER BY published_at DESC, id DESC
LIMIT 100;

-- name: GuestListArticlesStreamEnriched :many
WITH ranked AS (
  SELECT a.id, a.source_id, a.title, a.link, a.language, a.author, a.published_at, a.content_text,
         COALESCE(ai.title_translated, '') AS title_translated,
         COALESCE(ai.summary, '') AS summary,
         s.title AS source_title,
         COALESCE(st.is_read, false) AS is_read,
         COALESCE(st.is_starred, false) AS is_starred,
         row_number() OVER (PARTITION BY a.normalized_link ORDER BY a.published_at DESC, a.id DESC) AS rn
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_ai ai ON ai.article_id = a.id AND ai.target_language = @target_language
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = @state_owner_id
  WHERE s.user_id = @content_owner_id
    AND s.deleted_at IS NULL
    AND (@cursor::timestamptz IS NULL OR a.published_at < @cursor)
    AND (
      @read_filter::text = 'all'
      OR (@read_filter::text = 'unread' AND COALESCE(st.is_read, false) = false)
      OR (@read_filter::text = 'read' AND COALESCE(st.is_read, false) = true)
    )
)
SELECT id, source_id, title, link, language, author, published_at, content_text,
       title_translated, summary, source_title, is_read, is_starred
FROM ranked
WHERE rn = 1
ORDER BY published_at DESC, id DESC
LIMIT @lim;

-- name: GuestListArticlesStarredEnriched :many
WITH ranked AS (
  SELECT a.id, a.source_id, a.title, a.link, a.language, a.author, a.published_at, a.content_text,
         COALESCE(ai.title_translated, '') AS title_translated,
         COALESCE(ai.summary, '') AS summary,
         s.title AS source_title,
         COALESCE(st.is_read, false) AS is_read,
         COALESCE(st.is_starred, false) AS is_starred,
         row_number() OVER (PARTITION BY a.normalized_link ORDER BY a.published_at DESC, a.id DESC) AS rn
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_ai ai ON ai.article_id = a.id AND ai.target_language = @target_language
  JOIN article_states st ON st.article_id = a.id AND st.user_id = @state_owner_id
  WHERE s.user_id = @content_owner_id
    AND s.deleted_at IS NULL
    AND st.is_starred = true
)
SELECT id, source_id, title, link, language, author, published_at, content_text,
       title_translated, summary, source_title, is_read, is_starred
FROM ranked
WHERE rn = 1
ORDER BY published_at DESC, id DESC
LIMIT 100;

-- name: GuestListArticlesBySourceEnriched :many
WITH ranked AS (
  SELECT a.id, a.source_id, a.title, a.link, a.language, a.author, a.published_at, a.content_text,
         COALESCE(ai.title_translated, '') AS title_translated,
         COALESCE(ai.summary, '') AS summary,
         s.title AS source_title,
         COALESCE(st.is_read, false) AS is_read,
         COALESCE(st.is_starred, false) AS is_starred,
         row_number() OVER (PARTITION BY a.normalized_link ORDER BY a.published_at DESC, a.id DESC) AS rn
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_ai ai ON ai.article_id = a.id AND ai.target_language = @target_language
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = @state_owner_id
  WHERE s.user_id = @content_owner_id
    AND a.source_id = @source_id
    AND s.deleted_at IS NULL
    AND (
      @read_filter::text = 'all'
      OR (@read_filter::text = 'unread' AND COALESCE(st.is_read, false) = false)
      OR (@read_filter::text = 'read' AND COALESCE(st.is_read, false) = true)
    )
)
SELECT id, source_id, title, link, language, author, published_at, content_text,
       title_translated, summary, source_title, is_read, is_starred
FROM ranked
WHERE rn = 1
ORDER BY published_at DESC, id DESC;

-- name: GuestSearchArticles :many
SELECT a.id, a.source_id, a.title, a.link, a.language, a.published_at,
       ts_headline('simple', a.title || ' ' || COALESCE(a.content_text, ''),
                   plainto_tsquery('simple', @q) || cjk_tsquery(@q),
                   'MaxWords=20, MinWords=6') AS headline
FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE s.user_id = @content_owner_id
  AND s.deleted_at IS NULL
  AND a.search_vec @@ (plainto_tsquery('simple', @q) || cjk_tsquery(@q))
ORDER BY ts_rank(a.search_vec, plainto_tsquery('simple', @q) || cjk_tsquery(@q)) DESC
LIMIT 100;

-- name: GuestCountTodayByReadState :one
WITH grouped AS (
  SELECT a.normalized_link,
         bool_or(COALESCE(st.is_read, false) = false) AS has_unread
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = @state_owner_id
  WHERE s.user_id = @content_owner_id
    AND s.deleted_at IS NULL
    AND a.published_at >= now() - interval '24 hours'
  GROUP BY a.normalized_link
)
SELECT
  COUNT(*) AS all_count,
  COUNT(*) FILTER (WHERE has_unread) AS unread_count,
  COUNT(*) FILTER (WHERE NOT has_unread) AS read_count
FROM grouped;

-- name: GuestCountStreamByReadState :one
WITH grouped AS (
  SELECT a.normalized_link,
         bool_or(COALESCE(st.is_read, false) = false) AS has_unread
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = @state_owner_id
  WHERE s.user_id = @content_owner_id
    AND s.deleted_at IS NULL
  GROUP BY a.normalized_link
)
SELECT
  COUNT(*) AS all_count,
  COUNT(*) FILTER (WHERE has_unread) AS unread_count,
  COUNT(*) FILTER (WHERE NOT has_unread) AS read_count
FROM grouped;

-- name: GuestCountBySourceReadState :one
SELECT
  COUNT(*) AS all_count,
  COUNT(*) FILTER (WHERE COALESCE(st.is_read, false) = false) AS unread_count,
  COUNT(*) FILTER (WHERE COALESCE(st.is_read, false) = true) AS read_count
FROM articles a
JOIN sources s ON a.source_id = s.id AND s.user_id = @content_owner_id AND s.deleted_at IS NULL
LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = @state_owner_id
WHERE a.source_id = @source_id;
