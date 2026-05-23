CREATE TABLE article_ai (
  article_id bigint NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  target_language text NOT NULL,
  title_translated text NOT NULL DEFAULT '',
  summary text NOT NULL DEFAULT '',
  summary_status text NOT NULL DEFAULT 'none',
  summary_skip_reason text NOT NULL DEFAULT '',
  body_translation_status text NOT NULL DEFAULT 'none',
  body_translation_content jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (article_id, target_language)
);
