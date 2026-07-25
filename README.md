# Drive Backup Console

Self-hosted web console for managing personal Google Drive — browse, upload, download, share, and organise files through a beautiful Apple-inspired glass UI.

## Features

- **Full Drive browser** — folder navigation, breadcrumbs, search, type filters
- **Resumable chunked uploads** — pause/resume, progress tracking, drag-and-drop
- **Parallel downloads** — multi-stream Range requests for large files
- **Batch operations** — multi-select trash, move, ZIP download
- **Media preview** — images (pinch-zoom), video, PDF, text editing
- **Sharing & revisions** — generate share links, browse file history
- **Overview dashboard** — storage usage, upload history, type breakdown
- **Responsive** — works on desktop and mobile with touch-optimised UI
- **Dark / Light / System** theme with Apple Liquid Glass design
- **Performance** — Gzip compression, connection pooling, thumbnail caching, context-aware retries

## Tech Stack

| Layer | Tech |
|-------|------|
| Backend | Go 1.23, `net/http`, Google Drive API v3 |
| Frontend | Vite 5 + React 18 + TypeScript + Tailwind CSS |
| Auth | Google OAuth 2.0 (server-side token storage) |
| Deploy | Docker multi-stage build |

## Prerequisites

- Go 1.23+
- Node 20+ / npm 10+
- Google Cloud project with OAuth 2.0 Web credentials
- (Optional) Docker & Docker Compose for containerised deployment

## Quick Start

### 1. Configure environment

```bash
cp .env.example .env
# Edit .env with your Google OAuth credentials
```

Required variables:

| Variable | Description |
|----------|-------------|
| `GOOGLE_CLIENT_ID` | OAuth 2.0 client ID |
| `GOOGLE_CLIENT_SECRET` | OAuth 2.0 client secret |
| `SESSION_SECRET` | Random string for cookie signing |
| `OAUTH_REDIRECT_URL` | Callback URL (default: `http://localhost:3000/oauth2/callback`) |
| `FRONTEND_ORIGIN` | SPA URL for post-login redirect (default: `http://localhost:5174/`) |

Optional:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | Server listen port |
| `DEV_MODE` | `0` | `1` = skip OAuth secret validation |
| `DATA_DIR` | `./data` | Token & upload data directory |
| `ROOT_FOLDER_ID` | _(empty)_ | Limit browsing to a specific Drive folder |
| `HTTP_PROXY` | _(empty)_ | Proxy for Google API calls (e.g. `socks5://...`) |
| `WEB_DIST_DIR` | `./web/dist` | Path to built SPA assets |

### 2. Run in development

```bash
# Backend (terminal 1)
go run ./cmd/server

# Frontend (terminal 2)
cd web && npm install && npm run dev
```

- API: http://localhost:3000/api/health
- SPA: http://localhost:5174 (proxies `/api` → `:3000`)

### 3. Run with Docker

```bash
docker compose up --build
```

Open http://localhost:3000 — the Go server serves both API and SPA.

See `docs/docker.md` for full deployment guide.

## Project Layout

```
cmd/server/              Server entrypoint
internal/
  api/                   HTTP handlers, middleware, router
  auth/                  OAuth 2.0 + session management
  config/                Environment configuration
  drive/                 Google Drive API client
  upload/                Resumable upload job store
web/
  src/                   React SPA source
  dist/                  Production build output
docs/                    Design docs, API contract, progress
data/                    Runtime data (gitignored)
```

## Testing

```bash
# Go unit tests
go test ./... -count=1

# Frontend tests
cd web && npm test -- --run

# Type check
cd web && npx tsc --noEmit
```

## Documentation

- [API Contract](docs/api-contract.md)
- [Design Spec](docs/spark/2026-07-21-drive-backup-console-design.md)
- [Docker Deployment](docs/docker.md)
- [Development Workflow](docs/DEV_WORKFLOW.md)
- [Progress Log](docs/progress.md)

## License

Private — personal use only.
