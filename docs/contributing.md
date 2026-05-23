**English** | [简体中文](contributing.zh-CN.md)

# Contributing to xReader

Thank you for your interest in contributing to xReader! This guide covers everything you need to set up a local development environment, run tests, and submit pull requests.

> This project is developed with AI assistance. Conventions for agents and contributors alike live in [AGENTS.md](../AGENTS.md) at the repo root. Read it — it is the canonical reference for architecture details, command cheatsheet, and what to avoid.

---

## 1. Architecture Overview

xReader is intentionally simple: **one Go binary** does everything.

- **API server** — HTTP handlers for the frontend and Fever API clients
- **Embedded static frontend** — the compiled Next.js output is baked into the binary at build time
- **Background worker** — RSS fetch loop + AI pipeline run in a goroutine inside the same process
- **Auto-migration** — the binary applies pending Postgres migrations on startup; no manual step needed

AI settings (model, base URL, API key) are stored **encrypted in the database** and configured at runtime through the Setup Wizard UI. There is no config file. Environment variables override database values via `platform.ConfigResolver`.

For full package-level detail and a complete command reference, see [AGENTS.md](../AGENTS.md).

---

## 2. Prerequisites

| Tool | Minimum version | Notes |
|------|----------------|-------|
| Go | 1.25+ | `go version` |
| Node.js | 20+ | `node --version` |
| pnpm | any recent | `npm i -g pnpm` if missing |
| Docker | any recent | Required for backend tests (testcontainers-go spins up a real Postgres container per test) |

---

## 3. Local Development

### Start the database

```bash
make up          # starts a Postgres 16 container
```

### Run the backend

```bash
cd server
go run ./cmd/xreader
```

On first run the server prints a **SETUP TOKEN** to the console. Open `http://localhost:3000/setup` and enter it to configure GitHub OAuth and AI settings.

### Run the frontend dev server

In a separate terminal:

```bash
cd web
pnpm install
pnpm dev         # Turbopack dev server on :3000
```

> During frontend development, point your browser at the Next.js dev server (`:3000`). The Go server runs on the same port in production; use a different port for one of them locally if both are running simultaneously.

### Useful Make targets

```bash
make up          # Start Postgres 16 container
make down        # Stop containers
make build       # Build web then compile Go binary
make lint        # Run all linters (Go + TypeScript)
make test        # Run all backend + frontend tests
```

---

## 4. Testing

### Run everything

```bash
make test
```

### Backend tests (Go)

```bash
cd server
go test ./...           # all packages — requires Docker
go test ./... -race     # with race detector
```

Backend tests use **testcontainers-go**: each test suite spins up a real Postgres 16 container via `testutil.SetupTestDB()`. There is **no database mocking**. Docker must be running.

To run a single test:

```bash
cd server
go test ./internal/source/... -run TestNormalize -v -count=1
```

### Frontend tests

```bash
cd web
pnpm vitest run                                        # all tests
pnpm vitest run src/lib/api-client.test.ts             # single file
```

### Lint

```bash
make lint
# or individually:
cd server && go vet ./...
cd web && pnpm lint
```

Both must be clean before a PR is merged.

---

## 5. Database Changes

xReader uses **sqlc** as its database access layer. The workflow is:

1. Write or edit SQL in `server/db/queries/*.sql`
2. Run `make sqlc-generate` to regenerate the Go code in `server/db/gen/`
3. **Never hand-edit** files under `server/db/gen/` — they are overwritten on every generate run

Migrations live in `server/db/migrations/`. Adding a new migration is a **safety gate** — see Section 7.

```bash
make sqlc-generate     # regenerate Go code from SQL queries
make migrate-up        # run all pending migrations (also runs automatically on startup)
make migrate-down      # rollback one migration
```

---

## 6. Commit & Pull Request Guidelines

### Commit style

Follow [Conventional Commits](https://www.conventionalcommits.org/):

| Prefix | Use for |
|--------|---------|
| `feat:` | new user-visible feature |
| `fix:` | bug fix |
| `docs:` | documentation only |
| `refactor:` | code restructuring, no behavior change |
| `test:` | adding or fixing tests |
| `chore:` | build scripts, deps, config |

Scope is encouraged: `feat(reader):`, `fix(auth):`, `docs(contributing):`.

### Pull request checklist

- [ ] One feature or fix per PR — do not bundle unrelated changes
- [ ] New features come with tests; bug fixes include a regression test
- [ ] `make lint` passes (Go vet + ESLint + TypeScript)
- [ ] `make test` passes (requires Docker for backend)
- [ ] PR description explains *why*, not just *what*

---

## 7. Safety Gates

Always **ask the maintainer first** before:

- **Adding a new database migration** (`server/db/migrations/`) — schema changes affect all deployments
- **Adding a new Go or npm dependency** — review for license compatibility and supply-chain risk
- **Destructive git operations** — `push --force`, `reset --hard`, or `commit --amend` on already-pushed commits

---

## License

By contributing, you agree that your contributions are licensed under **AGPL-3.0**.
