-- name: CreateHighlight :one
INSERT INTO highlights (user_id, article_id, layer, paragraph_index, text_start_offset, text_end_offset, quoted_text, note)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetHighlight :one
SELECT * FROM highlights WHERE id = $1 AND user_id = $2;

-- name: ListHighlightsByArticle :many
SELECT * FROM highlights
WHERE user_id = $1 AND article_id = $2
ORDER BY paragraph_index, text_start_offset;

-- name: UpdateHighlightNote :exec
UPDATE highlights
SET note = $3,
    updated_at = now()
WHERE id = $1 AND user_id = $2;

-- name: DeleteHighlight :exec
DELETE FROM highlights
WHERE id = $1 AND user_id = $2;

-- name: ListHighlightsByUser :many
SELECT h.*, a.title as article_title, a.link as article_link
FROM highlights h
JOIN articles a ON h.article_id = a.id
WHERE h.user_id = $1
ORDER BY h.created_at DESC
LIMIT $2 OFFSET $3;

-- name: SearchHighlights :many
SELECT h.*, a.title as article_title, a.link as article_link
FROM highlights h
JOIN articles a ON h.article_id = a.id
WHERE h.user_id = $1
  AND (h.quoted_text ILIKE '%' || $2 || '%' OR h.note ILIKE '%' || $2 || '%')
ORDER BY h.created_at DESC
LIMIT $3 OFFSET $4;
