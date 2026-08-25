# Drive Backup Console

Self-hosted web console for managing personal Google Drive — browse, upload, download, share, and organise files through a beautiful Apple-inspired glass UI.

## Features

- **Full Drive browser** — folder navigation, breadcrumbs, Spotlight-style search (Ctrl/Cmd+K), type filters
- **Resumable chunked uploads** — pause/resume, progress tracking, drag-and-drop
- **Parallel downloads** — multi-stream Range requests for large files
- **Batch operations** — multi-select trash, move, ZIP download
- **Media preview** — images (pinch-zoom), video, PDF, text editing
- **Sharing & revisions** — generate share links, browse file history
- **Media download** — yt-dlp powered: YouTube, Bilibili, direct links → auto-upload to Drive
- **Overview dashboard** — storage usage, upload history, type breakdown
- **Responsive** — works on desktop and mobile with touch-optimised UI
- **Dark / Light / System** theme with Apple Liquid Glass design
- **Agent-ready API** — stable `/api/v1` with API keys (read / readwrite scopes) and OpenAPI spec at `/api/v1/openapi.json`
- **Performance** — content-aware Gzip, connection pooling, thumbnail caching, adaptive chunk pipeline, context-aware retries

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
- **yt-dlp** (for download feature) — [install](https://github.com/yt-dlp/yt-dlp#installation)
- **ffmpeg** (for download feature — yt-dlp uses it for merging formats)
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
| `YTDLP_PATH` | _(empty)_ | Path to yt-dlp binary; enables download feature |
| `DOWNLOAD_PROXY` | _(empty)_ | Proxy for yt-dlp (e.g. `socks5://127.0.0.1:10808`) |
| `DOWNLOAD_COOKIE_PATH` | _(empty)_ | Netscape cookie file for auth-required sites |
| `DOWNLOAD_TMP_DIR` | `DATA_DIR/downloads` | Temp directory for downloaded files |

### 2. Install yt-dlp (for download feature)

The download feature requires [yt-dlp](https://github.com/yt-dlp/yt-dlp) and [ffmpeg](https://ffmpeg.org/).

**Windows (winget):**
```bash
winget install yt-dlp.yt-dlp
winget install Gyan.FFmpeg
```

**macOS (Homebrew):**
```bash
brew install yt-dlp ffmpeg
```

**Linux (pip):**
```bash
pip install yt-dlp
sudo apt install ffmpeg  # or: brew install ffmpeg
```

Find the binary path and set it in `.env`:
```bash
# Windows: typically C:\Users\<you>\AppData\Local\Programs\yt-dlp\yt-dlp.exe
# macOS/Linux: /opt/homebrew/bin/yt-dlp or /usr/local/bin/yt-dlp or ~/.local/bin/yt-dlp
which yt-dlp  # macOS/Linux
where yt-dlp  # Windows
```

Set in `.env`:
```env
YTDLP_PATH=/path/to/yt-dlp
```

> **Docker users:** yt-dlp and ffmpeg are pre-installed in the Docker image. Skip this step.

### 3. Run in development

```bash
# Backend (terminal 1)
go run ./cmd/server

# Frontend (terminal 2)
cd web && npm install && npm run dev
```

- API: http://localhost:3000/api/health
- SPA: http://localhost:5174 (proxies `/api` → `:3000`)

### 4. Run with Docker

```bash
docker compose up --build -d
```

Open http://localhost:3000 — the Go server serves both API and SPA, with yt-dlp and ffmpeg pre-installed.

See `docs/docker.md` for full deployment guide.

## Project Layout

```
cmd/server/              Server entrypoint
internal/
  api/                   HTTP handlers, middleware, router
  apikey/                API key store for /api/v1 (agents & scripts)
  auth/                  OAuth 2.0 + session management
  config/                Environment configuration
  drive/                 Google Drive API client
  download/              yt-dlp download service & job store
  upload/                Resumable upload job store
web/
  src/                   React SPA source
  dist/                  Production build output
cmd/
  server/                Server entrypoint
  cookieconvert/         Cookie Editor JSON → Netscape format converter
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
- [Agent Skill — operate the drive via API](SKILL.md)
- [Design Spec](docs/spark/2026-07-21-drive-backup-console-design.md)
- [Agent File API Design](docs/spark/2026-07-23-agent-file-api-design.md)
- [Docker Deployment](docs/docker.md)
- [Development Workflow](docs/DEV_WORKFLOW.md)
- [Progress Log](docs/progress.md)

## Download Feature

The download feature uses [yt-dlp](https://github.com/yt-dlp/yt-dlp) to download media from YouTube, Bilibili, Douyin, direct links, and other supported sites, then automatically uploads the result to Google Drive.

> **Docker:** yt-dlp + ffmpeg are pre-installed. Just `docker compose up --build` and the feature is ready.
> **Local dev:** see the [Install yt-dlp](#2-install-yt-dlp-for-download-feature) step above.

### Cookies (for auth-required sites)

Sites like Bilibili and Douyin require authentication cookies. To set up:

1. Install the [Cookie Editor](https://cookie-editor.com) browser extension.
2. Navigate to the site and log in.
3. Open Cookie Editor → Export → JSON format → save as `bilibili.com.json`.
4. Convert to Netscape format:

```bash
# Using the built-in converter tool
go run ./cmd/cookieconvert bilibili.com.json > data/cookies.txt

# Or inside Docker
docker compose exec app /cookieconvert /tmp/bilibili.com.json > /data/cookies.txt
```

5. Set in `.env`:
```env
DOWNLOAD_COOKIE_PATH=./data/cookies.txt   # local dev
# DOWNLOAD_COOKIE_PATH=/data/cookies.txt   # Docker
```

6. Restart the server.

### How it works

- **Two-phase pipeline**: metadata resolution → download → upload to Drive (resumable for large files).
- **Retry upload**: if upload fails but the download succeeded, retry only re-uploads — no re-download.
- **Non-video files**: when yt-dlp returns `unknown_video` extension (common for direct links), the server infers the correct extension from the URL.
- **Folder routing**: downloaded files go to `ROOT_FOLDER_ID` or a specified Drive folder. Virtual folder names (e.g. `Videos`) are auto-created.
- **Proxy routing**: domestic sites (Bilibili, Douyin) automatically bypass the download proxy; foreign sites (YouTube) use `DOWNLOAD_PROXY` if set.
- **Job persistence**: download jobs survive server restarts via `DATA_DIR/downloads.json`.
- **Cleanup**: a background reaper removes terminal jobs older than 1 hour; temp files are deleted after upload.

## License

Private — personal use only.
