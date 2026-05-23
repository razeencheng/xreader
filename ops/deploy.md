# Production deployment

xReader is a single Go binary that serves both the API and static frontend. It connects to a Postgres 16 database and auto-migrates on startup.

## 1) Prerequisites

- Docker and Docker Compose installed on the target host
- A GitHub OAuth App (create at https://github.com/settings/developers)
  - Set the callback URL to `https://your-domain.com/api/auth/callback`

## 2) Deploy with Docker Compose

The `deploy/` directory contains a ready-to-use compose file.

```bash
cd deploy
cp .env.example .env
# Edit .env with your values (see below)
docker compose up -d
```

The stack includes:

- `xreader` — single binary serving HTTP on port 3000
- `postgres` — Postgres 16 with a named volume for data persistence
- `cloudflared` (optional) — Cloudflare Tunnel for exposing the service

## 3) Required environment variables

| Variable | Description |
|---|---|
| `SESSION_SECRET` | Random secret for cookie signing (generate with `openssl rand -hex 32`) |
| `POSTGRES_PASSWORD` | Password for the Postgres `xreader` user |

`DATABASE_URL` is auto-configured in the Docker Compose file using `POSTGRES_PASSWORD`.

### Optional environment variables

| Variable | Default | Description |
|---|---|---|
| `GITHUB_CLIENT_ID` | (none) | GitHub OAuth App client ID (can also be set via Setup Wizard) |
| `GITHUB_CLIENT_SECRET` | (none) | GitHub OAuth App client secret (can also be set via Setup Wizard) |
| `GITHUB_CALLBACK_URL` | (none) | OAuth callback URL (can also be set via Setup Wizard) |
| `PORT` | `3000` | HTTP listen port |
| `SETUP_TOKEN` | (auto-generated) | Token for the Setup Wizard; if unset, a random one is printed to logs |
| `COOKIE_SECURE` | (unset) | Set to `true` to mark session cookies as Secure (use behind HTTPS) |
| `XREADER_AI_ENCRYPTION_KEY` | (derived from SESSION_SECRET) | Separate key for encrypting AI API keys stored in DB |
| `TZ` | `Asia/Shanghai` | Container timezone |

## 4) Initial setup via Setup Wizard

On first launch with no admin users configured, xReader prints a setup token to the container logs:

```bash
docker compose logs xreader | grep "SETUP TOKEN"
```

Open `https://your-domain.com/setup` in a browser and enter the token. The wizard guides you through:

1. GitHub OAuth configuration (client ID, secret, callback URL)
2. AI provider settings (base URL, model, API key)
3. First admin user (GitHub username)

All settings are stored in the database and take effect without restarting.

## 5) Verify health

```bash
curl -fsS http://localhost:3000/health
```

Expected response:

```json
{"status":"ok"}
```

## 6) Useful operational commands

Check container status:

```bash
docker compose ps
```

View logs:

```bash
docker compose logs -f xreader
```

Restart the service:

```bash
docker compose restart xreader
```

Seed an admin manually (alternative to the Setup Wizard):

```bash
docker compose exec xreader /xreader seed-admin --github-username=<username>
```

## 7) Updating

Pull the latest image and recreate:

```bash
docker compose pull && docker compose up -d
```

Migrations run automatically on startup. No manual migration step is needed.
