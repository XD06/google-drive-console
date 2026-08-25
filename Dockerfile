# ============================================================
# Multi-stage build for Drive Backup Console
# Stage 1: Build frontend (Node)
# Stage 2: Build backend  (Go) — server + cookieconvert
# Stage 3: Runtime         (Alpine — supports yt-dlp + ffmpeg)
# ============================================================

# --- Stage 1: Frontend build ---
FROM node:20-alpine AS frontend
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts
COPY web/ ./
RUN npm run build

# --- Stage 2: Backend build ---
FROM golang:1.23-alpine AS backend
RUN apk add --no-cache ca-certificates
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
# Embed the frontend dist into the binary's serving path
COPY --from=frontend /build/web/dist web/dist/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /cookieconvert ./cmd/cookieconvert

# --- Stage 3: Runtime (Alpine with yt-dlp + ffmpeg) ---
FROM alpine:3.20
RUN apk add --no-cache \
    ca-certificates \
    yt-dlp \
    ffmpeg \
    tzdata \
    && addgroup -S app && adduser -S app -G app

# Copy binaries
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend /server /server
COPY --from=backend /cookieconvert /cookieconvert

# Runtime configuration
ENV PORT=3000
ENV DATA_DIR=/data
ENV WEB_DIST_DIR=/web/dist
ENV YTDLP_PATH=/usr/bin/yt-dlp
ENV DOWNLOAD_TMP_DIR=/data/downloads
EXPOSE 3000

# Data volume for tokens, uploads, downloads
VOLUME ["/data"]

# Copy web dist for SPA serving
COPY --from=frontend /build/web/dist /web/dist/

USER app
ENTRYPOINT ["/server"]
