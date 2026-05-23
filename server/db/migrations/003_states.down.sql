DROP TRIGGER IF EXISTS trg_articles_search_vec ON articles;
DROP FUNCTION IF EXISTS articles_search_vec_update();
DROP INDEX IF EXISTS idx_articles_fts;
ALTER TABLE articles DROP COLUMN IF EXISTS search_vec;
DROP TABLE IF EXISTS article_state_changes;
DROP TABLE IF EXISTS article_states;

-- Restore original expression-based index
CREATE INDEX idx_articles_fts ON articles USING GIN (
  to_tsvector('simple', title || ' ' || coalesce(content_text, ''))
);
