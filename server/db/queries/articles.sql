-- name: CreateArticle :one
INSERT INTO articles (
    source_id, external_id, link, normalized_link, title, language,
    content_html, content_text, author, published_at, fetched_at
)
VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11
)
RETURNING *;

-- name: GetArticleByID :one
SELECT * FROM articles
WHERE id = $1;

-- name: UpdateArticleContent :one
UPDATE articles
SET content_html = $2,
    content_text = $3
WHERE id = $1
RETURNING *;

-- name: ListArticlesBySource :many
SELECT a.* FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE a.source_id = $1
  AND s.deleted_at IS NULL
ORDER BY a.published_at DESC;

-- name: ArticleExistsByNormalizedLink :one
SELECT EXISTS(
    SELECT 1 FROM articles
    WHERE source_id = $1 AND normalized_link = $2
) AS exists;

-- name: UpsertArticle :one
INSERT INTO articles (
    source_id, external_id, link, normalized_link, title, language,
    content_html, content_text, author, published_at, fetched_at
)
VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11
)
ON CONFLICT (source_id, normalized_link) DO NOTHING
RETURNING *;

-- name: SearchArticles :many
SELECT a.id, a.source_id, a.title, a.link, a.language, a.published_at,
       ts_headline('simple', a.title || ' ' || COALESCE(a.content_text, ''),
                   plainto_tsquery('simple', @q) || cjk_tsquery(@q),
                   'MaxWords=20, MinWords=6') AS headline
FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE s.user_id = @user_id
  AND s.deleted_at IS NULL
  AND a.search_vec @@ (plainto_tsquery('simple', @q) || cjk_tsquery(@q))
ORDER BY ts_rank(a.search_vec, plainto_tsquery('simple', @q) || cjk_tsquery(@q)) DESC
LIMIT 100;

-- name: ListArticlesToday :many
SELECT a.* FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE s.user_id = $1
  AND s.deleted_at IS NULL
  AND a.published_at >= now() - interval '24 hours'
ORDER BY a.published_at DESC
LIMIT 100;

-- name: ListArticlesStream :many
SELECT a.* FROM articles a
JOIN sources s ON a.source_id = s.id
WHERE s.user_id = $1
  AND s.deleted_at IS NULL
  AND ($2::timestamptz IS NULL OR a.published_at < $2)
ORDER BY a.published_at DESC, a.id DESC
LIMIT $3;

-- name: ListArticlesStarred :many
SELECT a.* FROM articles a
JOIN sources s ON a.source_id = s.id
JOIN article_states st ON a.id = st.article_id AND st.user_id = $1
WHERE s.user_id = $1
  AND s.deleted_at IS NULL
  AND st.is_starred = true
ORDER BY a.published_at DESC
LIMIT 100;

-- name: ListArticlesTodayEnriched :many
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
  LEFT JOIN article_ai ai ON ai.article_id = a.id AND ai.target_language = $2
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
  WHERE s.user_id = $1
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

-- name: ListArticlesStreamEnriched :many
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
  LEFT JOIN article_ai ai ON ai.article_id = a.id AND ai.target_language = $3
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
  WHERE s.user_id = $1
    AND s.deleted_at IS NULL
    AND ($2::timestamptz IS NULL OR a.published_at < $2)
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
LIMIT $4;

-- name: ListArticlesStarredEnriched :many
WITH ranked AS (
  SELECT a.id, a.source_id, a.title, a.link, a.language, a.author, a.published_at, a.content_text,
         COALESCE(ai.title_translated, '') AS title_translated,
         COALESCE(ai.summary, '') AS summary,
         s.title AS source_title,
         st.is_read,
         st.is_starred,
         row_number() OVER (PARTITION BY a.normalized_link ORDER BY a.published_at DESC, a.id DESC) AS rn
  FROM articles a
  JOIN article_states st ON a.id = st.article_id AND st.user_id = $1
  LEFT JOIN article_ai ai ON ai.article_id = a.id AND ai.target_language = $2
  JOIN sources s ON a.source_id = s.id
  WHERE s.user_id = $1
    AND s.deleted_at IS NULL
    AND st.is_starred = true
)
SELECT id, source_id, title, link, language, author, published_at, content_text,
       title_translated, summary, source_title, is_read, is_starred
FROM ranked
WHERE rn = 1
ORDER BY published_at DESC, id DESC
LIMIT 100;

-- name: ListArticlesBySourceEnriched :many
SELECT a.id, a.source_id, a.title, a.link, a.language, a.author, a.published_at, a.content_text,
       COALESCE(ai.title_translated, '') AS title_translated,
       COALESCE(ai.summary, '') AS summary,
       s.title AS source_title,
       COALESCE(st.is_read, false) AS is_read,
       COALESCE(st.is_starred, false) AS is_starred
FROM articles a
JOIN sources s ON a.source_id = s.id AND s.user_id = @user_id AND s.deleted_at IS NULL
LEFT JOIN article_ai ai ON ai.article_id = a.id AND ai.target_language = @target_language
LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = @user_id
WHERE a.source_id = @source_id
  AND (
    @read_filter::text = 'all'
    OR (@read_filter::text = 'unread' AND COALESCE(st.is_read, false) = false)
    OR (@read_filter::text = 'read' AND COALESCE(st.is_read, false) = true)
  )
ORDER BY a.published_at DESC;

-- name: CountArticlesTodayByReadState :one
WITH grouped AS (
  SELECT a.normalized_link,
         bool_or(COALESCE(st.is_read, false) = false) AS has_unread
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
  WHERE s.user_id = $1
    AND s.deleted_at IS NULL
    AND a.published_at >= now() - interval '24 hours'
  GROUP BY a.normalized_link
)
SELECT
  COUNT(*) AS all_count,
  COUNT(*) FILTER (WHERE has_unread) AS unread_count,
  COUNT(*) FILTER (WHERE NOT has_unread) AS read_count
FROM grouped;

-- name: CountArticlesStreamByReadState :one
WITH grouped AS (
  SELECT a.normalized_link,
         bool_or(COALESCE(st.is_read, false) = false) AS has_unread
  FROM articles a
  JOIN sources s ON a.source_id = s.id
  LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = $1
  WHERE s.user_id = $1
    AND s.deleted_at IS NULL
  GROUP BY a.normalized_link
)
SELECT
  COUNT(*) AS all_count,
  COUNT(*) FILTER (WHERE has_unread) AS unread_count,
  COUNT(*) FILTER (WHERE NOT has_unread) AS read_count
FROM grouped;

-- name: CountArticlesBySourceReadState :one
SELECT
  COUNT(*) AS all_count,
  COUNT(*) FILTER (WHERE COALESCE(st.is_read, false) = false) AS unread_count,
  COUNT(*) FILTER (WHERE COALESCE(st.is_read, false) = true) AS read_count
FROM articles a
JOIN sources s ON a.source_id = s.id AND s.user_id = @user_id AND s.deleted_at IS NULL
LEFT JOIN article_states st ON st.article_id = a.id AND st.user_id = @user_id
WHERE a.source_id = @source_id;
