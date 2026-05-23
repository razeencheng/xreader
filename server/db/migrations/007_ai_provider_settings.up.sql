CREATE TABLE ai_provider_settings (
  id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  endpoint TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  api_key_ciphertext BYTEA,
  api_key_nonce BYTEA,
  api_key_hint TEXT NOT NULL DEFAULT '',
  updated_by_user_id BIGINT REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
