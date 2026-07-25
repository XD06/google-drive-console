# Docker Deployment Guide

## Overview

Drive Backup Console uses a multi-stage Docker build:

1. **Frontend** — Node 20 Alpine builds the Vite/React SPA
2. **Backend** — Go 1.23 Alpine compiles a static binary
3. **Runtime** — Distroless (nonroot) image, ~12 MB final size

The resulting container serves both the API and the SPA on a single port.

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
```

## Environment Variables

All variables are passed via `.env` (loaded by `env_file` in Compose):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GOOGLE_CLIENT_ID` | Yes | — | Google OAuth 2.0 client ID |
| `GOOGLE_CLIENT_SECRET` | Yes | — | Google OAuth 2.0 client secret |
| `SESSION_SECRET` | Yes | — | Random string (32+ chars) for signing cookies |
| `OAUTH_REDIRECT_URL` | Yes | `http://localhost:3000/oauth2/callback` | Must match Google Console |
| `FRONTEND_ORIGIN` | No | `http://localhost:3000/` | Post-login redirect target |
| `PORT` | No | `3000` | Container listen port |
| `DATA_DIR` | No | `/data` | Token & state storage (Docker volume) |
| `ROOT_FOLDER_ID` | No | — | Restrict to a specific Drive folder |
| `HTTP_PROXY` | No | — | Proxy for Google API (e.g. `socks5://host:port`) |
| `SECURE_COOKIE` | No | `true` | Set `false` if not behind HTTPS |

> **Production note:** When running behind HTTPS (recommended), set
> `OAUTH_REDIRECT_URL=https://yourdomain.com/oauth2/callback` and
> `FRONTEND_ORIGIN=https://yourdomain.com/`.

## Volumes

| Mount | Purpose |
|-------|---------|
| `app-data:/data` | Persists OAuth token (`token.json`), upload state, API keys |

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
docker compose ps   # check health status
docker compose logs -f app   # stream logs
```

## Updating

```bash
docker compose down
docker compose pull  # if using registry image
docker compose up --build -d
```

Data in the `app-data` volume persists across rebuilds.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `oauth_not_configured` | Check `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` are set |
| Cookie not set after login | Ensure `OAUTH_REDIRECT_URL` domain matches browser URL |
| Can't reach Google APIs | Set `HTTP_PROXY` if behind a firewall |
| Permission denied on `/data` | Volume must be writable by UID 65534 (nonroot) |

## Image Size

| Stage | ~Size |
|-------|-------|
| Frontend build | ~200 MB (discarded) |
| Backend build | ~500 MB (discarded) |
| **Final image** | **~12 MB** |
