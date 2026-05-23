-- Generate bigrams from CJK text for indexing.
-- CJK characters have Unicode code points > 0x2E7F (11903 decimal).
CREATE OR REPLACE FUNCTION cjk_bigrams(input text) RETURNS tsvector AS $$
DECLARE
    result tsvector := ''::tsvector;
    clean text;
    i integer;
    bigram text;
BEGIN
    clean := regexp_replace(input, '[[:space:][:punct:]]+', ' ', 'g');
    FOR i IN 1..length(clean)-1 LOOP
        IF ascii(substr(clean, i, 1)) > 11903 AND ascii(substr(clean, i+1, 1)) > 11903 THEN
            bigram := substr(clean, i, 2);
            result := result || to_tsvector('simple', bigram);
        END IF;
    END LOOP;
    RETURN result;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Generate a tsquery from CJK text using bigrams (matching the index).
-- Falls back to plainto_tsquery for non-CJK input.
CREATE OR REPLACE FUNCTION cjk_tsquery(input text) RETURNS tsquery AS $$
DECLARE
    result tsquery;
    clean text;
    i integer;
    bigram text;
    bq tsquery;
BEGIN
    clean := regexp_replace(input, '[[:space:][:punct:]]+', ' ', 'g');
    result := NULL;
    FOR i IN 1..length(clean)-1 LOOP
        IF ascii(substr(clean, i, 1)) > 11903 AND ascii(substr(clean, i+1, 1)) > 11903 THEN
            bigram := substr(clean, i, 2);
            bq := to_tsquery('simple', bigram);
            IF result IS NULL THEN
                result := bq;
            ELSE
                result := result && bq;
            END IF;
        END IF;
    END LOOP;
    RETURN COALESCE(result, plainto_tsquery('simple', input));
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Update the search_vec trigger to include CJK bigrams alongside simple tokens.
CREATE OR REPLACE FUNCTION articles_search_vec_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vec :=
        setweight(to_tsvector('simple', COALESCE(NEW.title, '')), 'A') ||
        setweight(cjk_bigrams(COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('simple', LEFT(COALESCE(NEW.content_text, ''), 10000)), 'B') ||
        setweight(cjk_bigrams(LEFT(COALESCE(NEW.content_text, ''), 5000)), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Rebuild existing search vectors with the new bigram-aware function.
UPDATE articles SET search_vec =
    setweight(to_tsvector('simple', COALESCE(title, '')), 'A') ||
    setweight(cjk_bigrams(COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('simple', LEFT(COALESCE(content_text, ''), 10000)), 'B') ||
    setweight(cjk_bigrams(LEFT(COALESCE(content_text, ''), 5000)), 'B');
