# AGENTS.md

This file provides guidance to Codex and other AI agents when working with code in this repository.

## Project overview

xReader is a design-first information aggregation platform built as a single Go binary with an embedded Next.js frontend. It features GitHub OAuth authentication (allowlist-based), RSS feed aggregation with AI-powered title translation and summary, and an immersive bilingual reader.

The architecture is intentionally simple: one binary (`xreader`) serves both the API and static frontend, connects to a single Postgres 16 database, and auto-migrates on startup. AI settings are stored in the database and configured via a Setup Wizard UI.

## Commands

### Backend (Go)

```bash
cd server && go test ./...                     # All tests (requires Docker for testcontainers)
cd server && go test ./... -race               # Tests with race detection
cd server && go test ./internal/source/... -run TestNormalize -v -count=1  # Single test
cd server && go build -o bin/xreader ./cmd/xreader  # Build binary
cd server && go vet ./...                      # Lint
```

### Frontend (Next.js)

```bash
cd web && pnpm install                         # Install dependencies
cd web && pnpm dev                             # Dev server (Turbopack) on :3000
cd web && pnpm vitest run                      # Unit tests
cd web && pnpm vitest run src/lib/api-client.test.ts  # Single test
cd web && pnpm build                           # Production build (outputs to web/out/)
cd web && pnpm lint                            # ESLint + TypeScript check
```

### Infrastructure

```bash
make up                                        # Start Postgres 16 container
make down                                      # Stop containers
make build                                     # Build web + server binary
make test                                      # Run all backend + frontend tests
make lint                                      # Lint all
make sqlc-generate                             # Regenerate Go code from SQL queries
make migrate-up                                # Run all migrations (manual; auto on startup)
make migrate-down                              # Rollback one migration
make seed-admin GH_USER=razeencheng            # Bootstrap the first admin
```

## Architecture

### Single binary (`server/cmd/xreader/`)

The `xreader` binary does everything:
- Serves the API on the configured port (default `:3000`)
- Serves the embedded static frontend (built Next.js output)
- Runs the RSS fetch + AI pipeline worker in a background goroutine
- Auto-migrates the database on startup
- Supports a `seed-admin` subcommand for bootstrapping

### Backend (`server/`)

- **`cmd/xreader/`** — Single entry point (API + worker + static files)
- **`cmd/backfill-ai/`** — One-off utility to backfill AI processing
- **`internal/`** — Domain packages, each with service + handler + tests:
  - `admin/` — Allowlist management
  - `ai/` — OpenAI-compatible client, dynamic settings from DB, eager/lazy pipeline
  - `article/` — Article listing, state, FTS search, SSE for lazy body translation
  - `auth/` — GitHub OAuth, Postgres-backed sessions, cookie state
  - `crypto/` — Encryption for secrets stored in DB
  - `fever/` — Fever API compatibility
  - `highlight/` — Highlight CRUD with offset-based anchoring
  - `middleware/` — Auth + admin guard, security headers
  - `platform/` — Router, health endpoint, config resolver (env -> DB fallback)
  - `safenet/` — Content safety checks
  - `setup/` — Setup Wizard API (initial configuration)
  - `source/` — Source adapter interface, RSS adapter, URL normalization, OPML
  - `sync/` — Fetch worker loop
  - `testutil/` — `SetupTestDB()` via testcontainers-go (real Postgres per test)
  - `user/` — Profile settings (native language, density, theme)
- **`db/`** — Migrations (`migrations/`), sqlc queries (`queries/`), generated code (`gen/`)

### Frontend (`web/`)

- **Next.js 15** App Router + TypeScript + Tailwind CSS 4
- **Output:** Static export (`next export` -> `web/out/`), embedded into Go binary
- **State:** Zustand (client: auth, UI prefs) + TanStack React Query (server data)
- **API client:** `src/lib/api-client.ts` — typed fetch with credentials, auto-401 redirect
- **Path alias:** `@/` maps to `src/`
- **Testing:** Vitest + Testing Library (unit)

### AI pipeline

- **OpenAI-compatible**: all AI calls go through a `DynamicClient` that reads config from the database at runtime (set via Setup Wizard UI).
- **Eager (on fetch):** title translation + summary for every new article
- **Lazy (on read):** body translation streams paragraph-by-paragraph via SSE
- **No config file** — AI settings (base URL, model, API key) are stored encrypted in the `settings` table.

### Configuration resolution

The `platform.ConfigResolver` checks environment variables first, then falls back to the `settings` database table. This allows env-var overrides while supporting runtime configuration via the Setup Wizard.

## Environment

Required vars for the binary:

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | Yes | Postgres connection string |
| `SESSION_SECRET` | Yes | Random secret for cookie signing |

Optional vars (can also be configured via Setup Wizard):

| Variable | Description |
|---|---|
| `PORT` | HTTP listen port (default `3000`) |
| `GITHUB_CLIENT_ID` | GitHub OAuth client ID |
| `GITHUB_CLIENT_SECRET` | GitHub OAuth client secret |
| `GITHUB_CALLBACK_URL` | GitHub OAuth callback URL |
| `COOKIE_SECURE` | Set to `true` behind HTTPS |
| `SETUP_TOKEN` | Fixed setup token (otherwise auto-generated) |
| `XREADER_AI_ENCRYPTION_KEY` | Separate encryption key for AI secrets |
| `XREADER_DEV_MODE` | Set to `true` to allow weak SESSION_SECRET locally |

## Key patterns

- **Backend tests require Docker** — `testutil.SetupTestDB()` uses testcontainers-go. No database mocking.
- **sqlc is the database access layer** — write SQL in `server/db/queries/*.sql`, then `make sqlc-generate`. Never hand-write Go query code in `db/gen/`.
- **Auto-migration** — the binary runs migrations on startup. No manual step needed in production.
- **Config in DB** — OAuth and AI settings are stored in the `settings` table, editable via Setup Wizard without restart.
- **One task = one commit** — follow conventional commits (`feat(scope): ...`).

## Safety gates (always ask the owner before)

- Adding new database migrations (`server/db/migrations/`)
- Adding new Go or npm dependencies
- Destructive git operations (`push --force`, `reset --hard`, `commit --amend` on pushed commits)

## What NOT to do

- Don't start multiple tasks at once. Finish one, log it, commit, then start the next.
- Don't refactor code unrelated to the current task.
- Don't add features not in the spec.
- Don't ignore test failures — they are the signal.
- Don't hardcode AI provider endpoints or model names — they're config-driven via the DB.
- Don't weaken test assertions to make them pass.
