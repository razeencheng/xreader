CREATE TABLE users (
    id              bigserial PRIMARY KEY,
    github_id       bigint UNIQUE NOT NULL,
    github_username text UNIQUE NOT NULL,
    avatar_url      text,
    native_language text NOT NULL DEFAULT 'zh-CN',
    role            text NOT NULL DEFAULT 'user',
    density_pref    text NOT NULL DEFAULT 'comfortable',
    theme_pref      text NOT NULL DEFAULT 'system',
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auth_sessions (
    id           text PRIMARY KEY,
    user_id      bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    user_agent   text
);

CREATE TABLE auth_allowlist (
    github_username  text PRIMARY KEY,
    added_by_user_id bigint REFERENCES users(id),
    added_at         timestamptz NOT NULL DEFAULT now(),
    note             text
);
