# Setup Guide

Everything needed to get Drive Backup Console running locally or in Docker. Docker-specific deployment topics (volumes, reverse proxy, image layout) live in [docker.md](docker.md).

## Prerequisites

- **Go 1.23+** — backend build/run
- **Node 20+ / npm 10+** — frontend build/dev
- **Google Cloud project** with OAuth 2.0 Web credentials (see below)
- **yt-dlp + ffmpeg** — only for the media-download feature ([install](#yt-dlp-install))
- (Optional) Docker & Docker Compose for containerised deployment

## 1. Google OAuth credentials

1. In [Google Cloud Console](https://console.cloud.google.com/), create (or pick) a project.
2. Configure the **OAuth consent screen** (External is fine for a personal account; add yourself as a test user if the app stays in testing).
3. Create credentials → **OAuth client ID** → *Web application*.
4. Add the authorized redirect URI: `http://localhost:3000/oauth2/callback` (adjust host/port only if you changed them).
5. Copy the client ID and client secret into `.env`.

The app requests the full `auth/drive` scope because it browses, uploads, and organises the whole Drive — `drive.file` scope is not sufficient (see [design.md](design.md)).

## 2. Configure environment

```bash
cp .env.example .env
# Edit .env with your credentials
```

Required (unless `DEV_MODE=1`, which skips OAuth validation for health-only smoke):

| Variable | Description |
|----------|-------------|
| `GOOGLE_CLIENT_ID` | OAuth 2.0 client ID |
| `GOOGLE_CLIENT_SECRET` | OAuth 2.0 client secret |
| `SESSION_SECRET` | Random string (32+ chars) for signing the session cookie |

Optional (defaults verified in `internal/config/config.go`):

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | Server listen port |
| `OAUTH_REDIRECT_URL` | `http://localhost:3000/oauth2/callback` | Must match Google Console |
| `FRONTEND_ORIGIN` | `http://localhost:5174/` | Post-login redirect (SPA URL) |
| `DEV_MODE` | `0` | `1` = skip OAuth secret validation, ephemeral session secret |
| `DATA_DIR` | `./data` | Token & state directory (Docker: `/data`) |
| `TOKEN_PATH` | `$DATA_DIR/token.json` | OAuth token file location |
| `APIKEYS_PATH` | `$DATA_DIR/apikeys.json` | API key store location |
| `ROOT_FOLDER_ID` | _(empty)_ | Limit browsing to a specific Drive folder |
| `HTTP_PROXY` | _(empty)_ | Proxy for Google API calls (e.g. `socks5://127.0.0.1:10808`) |
| `SECURE_COOKIE` | `!DEV_MODE` | `Secure` flag on the session cookie; set `0` for plain-HTTP LAN use |
| `WEB_DIST_DIR` | `./web/dist` | Built SPA assets (Docker: `/web/dist`) |
| `YTDLP_PATH` | _(empty)_ | Path to yt-dlp binary; empty = download feature disabled |
| `DOWNLOAD_PROXY` | _(empty)_ | Proxy for yt-dlp only (domestic sites bypass it) |
| `DOWNLOAD_COOKIE_PATH` | _(empty)_ | Netscape cookie file for auth-required sites |
| `DOWNLOAD_TMP_DIR` | `$DATA_DIR/downloads` | Temp directory for downloaded media |

## 3. Run in development

```bash
# Backend (terminal 1) — run from the repo root so .env is found
go run ./cmd/server

# Frontend (terminal 2)
cd web && npm install && npm run dev
```

- API: http://localhost:3000/api/health
- SPA: http://localhost:5174 (Vite proxies `/api` and `/oauth2` → `:3000`)

Or start/stop both with `bash start.sh` / `bash stop.sh` (or `start.bat` / `stop.bat` on cmd).

Note: `web/dist` is gitignored — the SPA is served by the Go server only after `cd web && npm run build` (or via the Vite dev server, which doesn't need a build).

## 4. Run with Docker

```bash
cp .env.example .env   # fill in credentials
docker compose up --build -d
```

Open http://localhost:3000 — the Go server serves both API and SPA, with yt-dlp and ffmpeg pre-installed. Full deployment guide: [docker.md](docker.md).

## yt-dlp install

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
sudo apt install ffmpeg
```

Then point `.env` at the binary:

```bash
which yt-dlp   # macOS/Linux
where yt-dlp   # Windows (typically C:\Users\<you>\AppData\Local\Programs\yt-dlp\yt-dlp.exe)
```

```env
YTDLP_PATH=/path/to/yt-dlp
```

> **Docker users:** yt-dlp and ffmpeg are pre-installed in the image (`YTDLP_PATH=/usr/bin/yt-dlp`). Skip this section.

## Cookies (for auth-required sites)

Bilibili, Douyin, and similar sites need login cookies:

1. Install the [Cookie Editor](https://cookie-editor.com) browser extension, log in to the site, and export cookies as **JSON** (e.g. `bilibili.com.json`).
2. Convert to Netscape format:

```bash
# On the host (requires Go)
go run ./cmd/cookieconvert bilibili.com.json > data/cookies.txt

# Or inside Docker
docker compose cp bilibili.com.json app:/tmp/bilibili.com.json
docker compose exec app /cookieconvert /tmp/bilibili.com.json > /data/cookies.txt
```

3. Set in `.env` and restart the server:

```env
DOWNLOAD_COOKIE_PATH=./data/cookies.txt   # local dev
# DOWNLOAD_COOKIE_PATH=/data/cookies.txt  # Docker
```

> Cookie exports contain live session tokens — never commit them (the repo's `.gitignore` already excludes `*.com.json`).

## Proxy configuration

- `HTTP_PROXY` routes **Google API** traffic (set it if Google is unreachable directly).
- `DOWNLOAD_PROXY` routes **yt-dlp only**; foreign sites (YouTube) use it while domestic sites (Bilibili, Douyin) automatically bypass it. In Docker use `socks5://host.docker.internal:10808` style addresses.

## Data directory contents

All under `DATA_DIR` (created automatically, `0700`): `token.json` (OAuth token), `apikeys.json` (API keys), `uploads.json` / `downloads.json` (job state, survives restarts), `downloads/` (temp media, cleaned after upload). Deleting the directory logs you out and orphans running jobs.
