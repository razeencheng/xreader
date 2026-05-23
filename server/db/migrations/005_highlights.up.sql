CREATE TABLE highlights (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  article_id BIGINT NOT NULL REFERENCES articles(id),
  layer TEXT NOT NULL DEFAULT 'original' CHECK (layer IN ('original', 'translation')),
  paragraph_index INT NOT NULL,
  text_start_offset INT NOT NULL,
  text_end_offset INT NOT NULL,
  quoted_text TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  color TEXT NOT NULL DEFAULT 'yellow',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_highlights_user_article ON highlights (user_id, article_id);
