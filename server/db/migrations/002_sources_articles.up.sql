CREATE TABLE sources (
  id                bigserial PRIMARY KEY,
  user_id           bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind              text NOT NULL DEFAULT 'rss',
  url               text NOT NULL,
  normalized_url    text NOT NULL,
  title             text NOT NULL,
  icon_url          text,
  language_hint     text,
  last_fetched_at   timestamptz,
  last_success_at   timestamptz,
  consecutive_fails int NOT NULL DEFAULT 0,
  health            text NOT NULL DEFAULT 'unknown',
  created_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  UNIQUE (user_id, normalized_url)
);

CREATE TABLE articles (
  id                bigserial PRIMARY KEY,
  source_id         bigint NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  external_id       text NOT NULL,
  link              text NOT NULL,
  normalized_link   text NOT NULL,
  title             text NOT NULL,
  language          text NOT NULL,
  content_html      text NOT NULL,
  content_text      text NOT NULL,
  author            text,
  published_at      timestamptz NOT NULL,
  fetched_at        timestamptz NOT NULL DEFAULT now(),
  UNIQUE (source_id, normalized_link)
);

CREATE INDEX idx_articles_fts ON articles USING GIN (
  to_tsvector('simple', title || ' ' || coalesce(content_text, ''))
);
