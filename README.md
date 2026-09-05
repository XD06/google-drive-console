# Drive Backup Console

Self-hosted web console for managing personal Google Drive — browse, upload, download, share, and organise files through an Apple-inspired glass UI, with a stable `/api/v1` for AI agents and scripts.

## Features

- **Full Drive browser** — folder navigation, Spotlight-style search (Ctrl/Cmd+K), type filters, batch trash/move, ZIP download
- **Resumable chunked uploads** — pause/resume, progress tracking, drag-and-drop
- **Parallel downloads** — multi-stream Range requests; media previews (images, video, PDF, text editing)
- **Media download** — yt-dlp powered: YouTube, Bilibili, direct links → auto-upload to Drive
- **Overview dashboard** — storage usage, upload history, type breakdown
- **Agent-ready API** — `/api/v1` with `dbc_` API keys (read / readwrite) and OpenAPI spec at `/api/v1/openapi.json`
- Dark / Light / System theme, wallpaper backdrops, responsive desktop & mobile UI

## Quick Start

```bash
# 1. Configure (fill GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / SESSION_SECRET)
cp .env.example .env

# 2a. Development — backend (terminal 1) + frontend (terminal 2)
go run ./cmd/server                 # API on :3000
cd web && npm install && npm run dev # SPA on :5174 (proxies /api → :3000)

# 2b. Or production-style container (yt-dlp + ffmpeg included)
docker compose up --build -d        # http://localhost:3000
```

Google OAuth credential setup, the full environment-variable reference, yt-dlp install, and cookie setup for auth-required sites are in **[docs/SETUP.md](docs/SETUP.md)**. Docker deployment details are in **[docs/docker.md](docs/docker.md)**.

## Tech Stack

| Layer | Tech |
|-------|------|
| Backend | Go 1.23, stdlib `net/http`, Google Drive API v3 (raw REST) |
| Frontend | React 19 + TypeScript + Vite 6 + Tailwind CSS |
| Auth | Google OAuth 2.0 (server-side token storage, session cookie) |
| Tests | `go test` + Vitest (jsdom) |
| Deploy | Docker multi-stage build (Alpine + yt-dlp + ffmpeg) |

## Project Structure

```
cmd/server/          Server entrypoint (wiring only)
cmd/cookieconvert/   Cookie Editor JSON → Netscape converter (yt-dlp cookies)
internal/
  api/               HTTP handlers, middleware, router, OpenAPI
  auth/              OAuth 2.0 flow, token store, session cookie
  apikey/            dbc_ API keys for /api/v1
  drive/             Google Drive API client (the only Google caller)
  upload/            Resumable upload pipeline
  download/          yt-dlp download pipeline
web/                 React SPA (src/ + dist build output)
docs/                Setup, API contract, architecture archive, design notes
data/                Runtime state (gitignored; created at startup)
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for module boundaries, dependency rules, and core data flows.

## Testing

```bash
go test ./... -count=1       # backend unit tests
cd web && npm test           # frontend tests (Vitest)
cd web && npx tsc --noEmit   # type check
```

## Documentation

- Setup guide (OAuth, env vars, yt-dlp, cookies): [docs/SETUP.md](docs/SETUP.md)
- Architecture overview: [ARCHITECTURE.md](ARCHITECTURE.md)
- API contract (all routes, payloads, limits): [docs/api-contract.md](docs/api-contract.md)
- Agent skill — operate the drive via `/api/v1`: [docs/SKILL.md](docs/SKILL.md)
- Docker deployment: [docs/docker.md](docs/docker.md)
- Security model & design decisions: [docs/design.md](docs/design.md)
- Feature/design specs: [docs/spark/](docs/spark/)
- Update log: [CHANGELOG.md](CHANGELOG.md)
- Progress log: [docs/progress.md](docs/progress.md)
- AI agent rules (local-only): AGENTS.md

## License

Private — personal use only.
