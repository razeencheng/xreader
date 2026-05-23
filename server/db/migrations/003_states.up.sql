CREATE TABLE article_states (
  user_id           bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  article_id        bigint NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  is_read           boolean NOT NULL DEFAULT false,
  is_starred        boolean NOT NULL DEFAULT false,
  reading_progress  jsonb,
  last_read_at      timestamptz,
  PRIMARY KEY (user_id, article_id)
);

CREATE TABLE article_state_changes (
  user_id    bigint NOT NULL,
  article_id bigint NOT NULL,
  changed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, article_id, changed_at)
);

-- Add a materialized tsvector column to articles for faster FTS
ALTER TABLE articles ADD COLUMN search_vec tsvector;

-- Populate existing rows
UPDATE articles SET search_vec = to_tsvector('simple', title || ' ' || coalesce(content_text, ''));

-- Trigger to auto-maintain search_vec on INSERT/UPDATE
CREATE OR REPLACE FUNCTION articles_search_vec_update() RETURNS trigger AS $$
BEGIN
  NEW.search_vec := to_tsvector('simple', NEW.title || ' ' || coalesce(NEW.content_text, ''));
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_articles_search_vec
  BEFORE INSERT OR UPDATE OF title, content_text ON articles
  FOR EACH ROW EXECUTE FUNCTION articles_search_vec_update();

-- Replace the expression-based GIN index with one on the materialized column
DROP INDEX IF EXISTS idx_articles_fts;
CREATE INDEX idx_articles_fts ON articles USING GIN (search_vec);
