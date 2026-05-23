-- Restore original trigger from migration 003 (simple tsvector, no CJK bigrams).
CREATE OR REPLACE FUNCTION articles_search_vec_update() RETURNS trigger AS $$
BEGIN
  NEW.search_vec := to_tsvector('simple', NEW.title || ' ' || coalesce(NEW.content_text, ''));
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Rebuild search vectors with original function.
UPDATE articles SET search_vec = to_tsvector('simple', title || ' ' || coalesce(content_text, ''));

-- Drop CJK helper functions.
DROP FUNCTION IF EXISTS cjk_tsquery(text);
DROP FUNCTION IF EXISTS cjk_bigrams(text);
