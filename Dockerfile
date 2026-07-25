# ============================================================
# Multi-stage build for Drive Backup Console
# Stage 1: Build frontend (Node)
# Stage 2: Build backend  (Go)
# Stage 3: Runtime         (distroless/static)
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

# --- Stage 3: Minimal runtime ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend /server /server

# Runtime configuration
ENV PORT=3000
ENV DATA_DIR=/data
ENV WEB_DIST_DIR=/web/dist
EXPOSE 3000

# Data volume for tokens & uploads
VOLUME ["/data"]

# Copy web dist for SPA serving
COPY --from=frontend /build/web/dist /web/dist/

ENTRYPOINT ["/server"]
