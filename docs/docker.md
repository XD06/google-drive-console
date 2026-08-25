# Docker Deployment Guide

## Overview

Drive Backup Console uses a multi-stage Docker build:

1. **Frontend** — Node 20 Alpine builds the Vite/React SPA
2. **Backend** — Go 1.23 Alpine compiles a static binary (+ cookieconvert utility)
3. **Runtime** — Alpine 3.20 with yt-dlp + ffmpeg pre-installed (~80 MB final size)

The resulting container serves both the API and the SPA on a single port, with the download feature (yt-dlp) ready out of the box.

## Quick Deploy

```bash
# 1. Create .env from template
cp .env.example .env
# Edit .env — fill GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, SESSION_SECRET

# 2. Build & start
docker compose up --build -d

# 3. Verify
curl http://localhost:3000/api/health
# → {"status":"ok"}

# 4. Verify yt-dlp is available
docker compose exec app yt-dlp --version
# → 2024.x.x
```

## Environment Variables

All variables are passed via `.env` (loaded by `env_file` in Compose):

### Core (required)

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GOOGLE_CLIENT_ID` | Yes | — | Google OAuth 2.0 client ID |
| `GOOGLE_CLIENT_SECRET` | Yes | — | Google OAuth 2.0 client secret |
| `SESSION_SECRET` | Yes | — | Random string (32+ chars) for signing cookies |
| `OAUTH_REDIRECT_URL` | Yes | `http://localhost:3000/oauth2/callback` | Must match Google Console |
| `FRONTEND_ORIGIN` | No | `http://localhost:3000/` | Post-login redirect target |

### General

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | Container listen port |
| `DATA_DIR` | `/data` | Token & state storage (Docker volume) |
| `ROOT_FOLDER_ID` | — | Restrict to a specific Drive folder |
| `HTTP_PROXY` | — | Proxy for Google API (e.g. `socks5://host:port`) |
| `SECURE_COOKIE` | `true` | Set `false` if not behind HTTPS |
| `WEB_DIST_DIR` | `/web/dist` | Path to built SPA assets |

### Download feature (yt-dlp)

| Variable | Default (Docker) | Description |
|----------|-------------------|-------------|
| `YTDLP_PATH` | `/usr/bin/yt-dlp` | Path to yt-dlp binary. Set empty to disable. |
| `DOWNLOAD_PROXY` | — | Proxy for yt-dlp only (e.g. `socks5://host:port`) |
| `DOWNLOAD_COOKIE_PATH` | — | Netscape cookie file for auth-required sites |
| `DOWNLOAD_TMP_DIR` | `/data/downloads` | Temp directory for downloaded files |

> **Note:** In Docker, `YTDLP_PATH` and `DOWNLOAD_TMP_DIR` are pre-set in the
> Dockerfile. Override them only if you know what you're doing.

## Download Feature

### How it works

The download feature uses [yt-dlp](https://github.com/yt-dlp/yt-dlp) to download
media from supported sites (YouTube, Bilibili, Douyin, direct links, etc.) and
automatically uploads the result to Google Drive.

The Docker image comes with yt-dlp and ffmpeg pre-installed — no extra setup needed.

### Using cookies (for Bilibili, Douyin, etc.)

Some sites require authentication. To provide cookies:

1. **Export cookies from your browser** using the [Cookie Editor](https://cookie-editor.com)
   extension (JSON format).

2. **Convert to Netscape format** — run the converter inside the container:

```bash
# Copy JSON files into the container
docker compose cp bilibili.com.json app:/tmp/bilibili.com.json

# Convert
docker compose exec app /cookieconvert /tmp/bilibili.com.json > /tmp/cookies.txt

# Move to data volume (persists across restarts)
docker compose exec app sh -c "cat /tmp/cookies.txt > /data/cookies.txt"
```

3. **Set the cookie path** in `.env`:

```env
DOWNLOAD_COOKIE_PATH=/data/cookies.txt
```

4. **Restart** the container:

```bash
docker compose restart
```

Alternatively, you can convert cookies on your host machine if you have Go installed:

```bash
go run ./cmd/cookieconvert bilibili.com.json > data/cookies.txt
```

### Proxy configuration

If yt-dlp needs a proxy to reach certain sites (e.g. YouTube behind a firewall):

```env
# In .env
DOWNLOAD_PROXY=socks5://host.docker.internal:10808
```

> `host.docker.internal` resolves to the Docker host IP on Docker Desktop.
> On Linux, add `extra_hosts: ["host.docker.internal:host-gateway"]` to
> `docker-compose.yml`.

Domestic sites (Bilibili, Douyin, etc.) automatically bypass the proxy.

## Volumes

| Mount | Purpose |
|-------|---------|
| `app-data:/data` | OAuth token, upload state, API keys, download temp files, cookies |

To backup:

```bash
docker compose exec app cat /data/token.json > backup-token.json
```

## Building Manually

```bash
# Build image
docker build -t drive-backup-console .

# Run with explicit env
docker run -d \
  --name dbc \
  -p 3000:3000 \
  -v dbc-data:/data \
  --env-file .env \
  -e DATA_DIR=/data \
  -e WEB_DIST_DIR=/web/dist \
  -e YTDLP_PATH=/usr/bin/yt-dlp \
  -e SECURE_COOKIE=true \
  drive-backup-console
```

## Reverse Proxy (Nginx)

Example for production with HTTPS:

```nginx
server {
    listen 443 ssl http2;
    server_name drive.example.com;

    ssl_certificate     /etc/ssl/drive.crt;
    ssl_certificate_key /etc/ssl/drive.key;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket / streaming support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Large upload support
        client_max_body_size 0;
        proxy_request_buffering off;
    }
}
```

## Health Check

The `/api/health` endpoint returns `200 {"status":"ok"}` when the server is ready.

```bash
docker compose ps         # check health status
docker compose logs -f app  # stream logs
```

## Updating

```bash
docker compose down
docker compose up --build -d
```

Data in the `app-data` volume persists across rebuilds.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `oauth_not_configured` | Check `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` are set |
| Cookie not set after login | Ensure `OAUTH_REDIRECT_URL` domain matches browser URL |
| Can't reach Google APIs | Set `HTTP_PROXY` if behind a firewall |
| Permission denied on `/data` | Volume must be writable by UID 100 (app user) |
| Download feature disabled | Check `YTDLP_PATH` is set (default: `/usr/bin/yt-dlp`) |
| yt-dlp version too old | `docker compose exec app apk upgrade yt-dlp` |
| `unknown_video` extension | Expected for direct links — server auto-infers correct extension |
| Cookies not working | Ensure `DOWNLOAD_COOKIE_PATH` points to `/data/cookies.txt` |

## Image Size

| Stage | ~Size |
|-------|-------|
| Frontend build | ~200 MB (discarded) |
| Backend build | ~500 MB (discarded) |
| **Final image** | **~80 MB** (Alpine + yt-dlp + ffmpeg) |

> The image is larger than the previous distroless build (~12 MB) because
> yt-dlp and ffmpeg are bundled. This is a deliberate trade-off for
> out-of-the-box download functionality.
