<div align="center">

# xReader

**Self-hosted RSS reader with AI-powered translation & key points**

**English** | [简体中文](README.zh-CN.md)

[Features](#features) · [Quick Start](#quick-start) · [Deployment](docs/deployment.md) · [Contributing](docs/contributing.md)

</div>

![Feed List](docs/screenshots/feed-list.png)

![Article Reader](docs/screenshots/reader.png)

---

## Features

- **AI Bilingual Reading** — Titles auto-translated, paragraphs rendered side-by-side, 3-5 bullet key points per article
- **Fever API** — Connect Reeder, NetNewsWire, Unread, and other native clients
- **Full-Text Search** — CJK-aware search across all articles, no plugins needed
- **Highlights & Notes** — Select text, highlight, annotate
- **Dark Mode + Themes** — Light/dark/system with 4 accent colors
- **OpenAI-Compatible AI** — Works with DeepSeek, Moonshot, one-api, OpenRouter, or any relay
- **Simple Deployment** — Single binary + Postgres, 2 containers
- **Keyboard-First** — `L`/`H` next/previous article, `J`/`K` scroll down/up, `S` star, `R` mark read, `F` focus mode

## Quick Start

```bash
git clone https://github.com/razeencheng/xreader.git
cd xreader
echo "SESSION_SECRET=$(openssl rand -hex 32)" > .env

docker compose up -d

# Check logs for the Setup Token
docker compose logs xreader | grep "SETUP TOKEN"

# Open http://localhost:3000/setup → enter token → configure
```

## Configuration

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | Yes (auto in compose) | Postgres connection string |
| `SESSION_SECRET` | Yes | Random string for session signing |
| `GITHUB_CLIENT_ID` | No* | GitHub OAuth App Client ID |
| `GITHUB_CLIENT_SECRET` | No* | GitHub OAuth App Client Secret |
| `GITHUB_CALLBACK_URL` | No | OAuth callback URL, e.g. `https://your-domain/api/auth/github/callback` |
| `COOKIE_SECURE` | No | Set `true` when served over HTTPS |
| `SETUP_TOKEN` | No | Fixed setup token (auto-generated if unset) |
| `XREADER_AI_ENCRYPTION_KEY` | No | Custom encryption key for stored secrets |
| `XREADER_GA_ID` | No | Google Analytics measurement ID; injected at runtime when set |

*Configured via Setup Wizard on first run, or set as env vars.

## Fever API

Connect third-party RSS clients:

1. Go to **Settings → Third-party Clients**
2. Set a Fever password
3. In your client, add a Fever account:
   - **Server:** `https://your-domain/fever/`
   - **Username:** your GitHub username
   - **Password:** the password you set

Tested with: Reeder, NetNewsWire, Unread.

## Development

```bash
# Prerequisites: Go 1.25+, Node.js 20+, pnpm, Docker

# Start database
make up

# Backend
cd server && go run ./cmd/xreader

# Frontend (separate terminal)
cd web && pnpm install && pnpm dev

# Tests
make test
```

## Roadmap

- [ ] Local username/password auth
- [ ] Auto-cleanup retention policies
- [ ] PWA support
- [ ] Health monitoring dashboard
- [ ] More adapters (HackerNews, Reddit, Newsletter)
- [ ] Google Reader API

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and the [contributing guide](docs/contributing.md). Deployment: [docs/deployment.md](docs/deployment.md).

## License

[AGPL-3.0](LICENSE)
