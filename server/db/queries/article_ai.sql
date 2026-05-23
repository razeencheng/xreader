-- name: UpsertTitleTranslation :exec
INSERT INTO article_ai (article_id, target_language, title_translated, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (article_id, target_language) DO UPDATE SET
  title_translated = $3,
  updated_at = now();

-- name: UpsertSummary :exec
INSERT INTO article_ai (article_id, target_language, summary, summary_status, summary_skip_reason, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (article_id, target_language) DO UPDATE SET
  summary = EXCLUDED.summary,
  summary_status = EXCLUDED.summary_status,
  summary_skip_reason = EXCLUDED.summary_skip_reason,
  updated_at = now();

-- name: GetArticleAI :one
SELECT * FROM article_ai
WHERE article_id = $1 AND target_language = $2;

-- name: SetBodyStatus :exec
UPDATE article_ai
SET body_translation_status = $2, updated_at = now()
WHERE article_id = $1 AND target_language = $3;

-- name: SetBodyTranslation :exec
UPDATE article_ai
SET body_translation_content = $3, body_translation_status = $4, updated_at = now()
WHERE article_id = $1 AND target_language = $2;

-- name: SetBodyTranslationStatus :exec
UPDATE article_ai
SET body_translation_status = $3, updated_at = now()
WHERE article_id = $1 AND target_language = $2;

-- name: SetBodyTranslationContent :exec
UPDATE article_ai
SET body_translation_content = $3, body_translation_status = 'done', updated_at = now()
WHERE article_id = $1 AND target_language = $2;

-- name: ResetBodyTranslation :exec
UPDATE article_ai
SET body_translation_content = NULL, body_translation_status = 'none', updated_at = now()
WHERE article_id = $1 AND target_language = $2;

-- name: EnsureArticleAI :exec
INSERT INTO article_ai (article_id, target_language)
VALUES ($1, $2)
ON CONFLICT (article_id, target_language) DO NOTHING;

-- name: ListArticlesMissingAI :many
SELECT a.id
FROM articles a
WHERE NOT EXISTS (
  SELECT 1 FROM article_ai ai
  WHERE ai.article_id = a.id AND ai.target_language = $1
    AND ai.title_translated IS NOT NULL AND ai.title_translated != ''
    AND ai.summary_status IN ('done', 'skipped')
)
ORDER BY a.published_at DESC
LIMIT $2;
