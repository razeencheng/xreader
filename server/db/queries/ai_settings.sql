-- name: GetAIProviderSettings :one
SELECT * FROM ai_provider_settings WHERE id = 1;

-- name: UpsertAIProviderSettings :one
INSERT INTO ai_provider_settings (
  id,
  endpoint,
  model,
  api_key_ciphertext,
  api_key_nonce,
  api_key_hint,
  updated_by_user_id
)
VALUES (1, $1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
  endpoint = EXCLUDED.endpoint,
  model = EXCLUDED.model,
  api_key_ciphertext = COALESCE(EXCLUDED.api_key_ciphertext, ai_provider_settings.api_key_ciphertext),
  api_key_nonce = COALESCE(EXCLUDED.api_key_nonce, ai_provider_settings.api_key_nonce),
  api_key_hint = CASE
    WHEN EXCLUDED.api_key_ciphertext IS NULL THEN ai_provider_settings.api_key_hint
    ELSE EXCLUDED.api_key_hint
  END,
  updated_by_user_id = EXCLUDED.updated_by_user_id,
  updated_at = now()
RETURNING *;
