# API Contract

**Status:** current — matches `internal/api/router.go`  
**Base URL (local):** `http://localhost:3000`  
**Content-Type:** `application/json` unless noted  
**Machine-readable spec:** `GET /api/v1/openapi.json` (no auth) is the canonical OpenAPI document.

## Authentication

Two auth modes:

| Namespace | Auth | Audience |
|-----------|------|----------|
| `/api/*` | Session cookie `dbc_session` (HMAC-SHA256, set by OAuth callback) | Browser SPA |
| `/api/v1/*` | Session cookie **or** API key `Authorization: Bearer dbk_…` | AI agents / scripts |

API keys carry a scope: `read` or `readwrite`. Key management routes are session-only (a key can never manage keys).

## Error envelope

```json
{
  "error": {
    "code": "not_authenticated",
    "message": "Sign in with Google required"
  }
}
```

Common HTTP status: `400` validation, `401` auth, `403` Drive/permission/scope, `404` missing, `405` method, `429` quota, `502` upstream Google.

---

## Health

### `GET /api/health`

No auth. `200 {"status":"ok","service":"drive-backup-console","timeUtc":"…","devMode":true}` (`devMode` omitted in production).

---

## OAuth + session

Requires `GOOGLE_CLIENT_ID` + `GOOGLE_CLIENT_SECRET`; without them login/callback return `503 oauth_not_configured`.

- `GET /oauth2/login` — 302 to Google consent (`access_type=offline`, forced approval). Issues short-lived CSRF `state`.
- `GET /oauth2/callback?code=&state=` — validates state, exchanges code, persists token under `DATA_DIR`, sets `dbc_session` cookie, 302 to `FRONTEND_ORIGIN`.
- `GET /api/auth/me` — `200 {"email":"…","connected":true}` or `401 not_authenticated`.
- `POST /api/auth/logout` — clears cookie; `?clearToken=1` also deletes the stored token file.

---

## Files

All routes below require a session (or, under `/api/v1`, an API key with sufficient scope).

### `GET /api/files?folderId=&pageToken=&pageSize=`

Lists a folder. `folderId` defaults to `ROOT_FOLDER_ID` env or `root`. `pageSize` optional (server clamps; SPA sends 100).

**200**

```json
{
  "folderId": "root",
  "items": [
    { "id": "1abc", "name": "Backups", "mimeType": "application/vnd.google-apps.folder",
      "size": null, "modifiedTime": "2026-07-20T10:00:00Z", "isFolder": true }
  ],
  "nextPageToken": null
}
```

### `GET /api/files/search?q=&scope=folder|drive&folderId=&pageToken=&pageSize=`

Full-text name search. `scope=folder` restricts to `folderId`; default searches the whole Drive. Response shape matches list.

### `GET /api/files/{id}/download`

Streams binary media, `Content-Disposition: attachment`. Supports **Range** requests (passthrough to Google — used for video/PDF streaming and parallel downloads). Optional metadata-hint query params from list data let the server skip the `GetMeta` round-trip. Folders / native Google Docs types → `400` (`is_folder` / `export_required`).

### `POST /api/files/mkdir`

`{ "name": "Backups", "parentId": "root" }` → **201** `FileItem` (`isFolder: true`).

### `POST /api/files`

Creates a file with optional initial text: `{ "name": "note.md", "parentId": "root", "mimeType": "text/markdown", "content": "# hello" }` → **201** `FileItem`. Body limit 3 MiB.

### `POST /api/files/simple?name=&parentId=&mimeType=`

Raw binary body (max **5 MiB**) uploaded in one request — used by the SPA for small files. → **201** `FileItem`.

### `PATCH /api/files/{id}`

Rename: `{ "name": "renamed.md" }` → **200** updated `FileItem`.

### `POST /api/files/{id}/move`

`{ "parentId": "folderIdOrRoot" }` → **200** updated `FileItem`.

### `POST /api/files/{id}/copy`

`{ "parentId": "…", "name": "optional new name" }` → **201** `FileItem` of the copy.

### `DELETE /api/files/{id}`

Moves to Drive trash → **204**.

### `POST /api/files/batch`

Bulk trash/move (max 100 ids, 6-way concurrent server-side):

```json
{ "action": "trash", "ids": ["id1", "id2"], "parentId": "required for move" }
```

**200** `{ "succeeded": N, "errors": [{ "id": "…", "error": "…" }] }`.

### `GET /api/files/{id}/content` / `PUT /api/files/{id}/content`

Text read/write for previewable text files. GET → `{ "content", "mimeType", "size", "name" }`; PUT `{ "content": "…" }` → `{ "ok": true }`. PUT body limit 3 MiB.

### Sharing

- `POST /api/files/{id}/share` — `{ "role": "reader" }` (default `reader`) → **200** share-link info.
- `DELETE /api/files/{id}/share?permissionId=` — revoke → **204**.
- `GET /api/files/{id}/permissions` — **200** `{ "permissions": [...] }`.

### Revisions

- `GET /api/files/{id}/revisions` — revision list.
- `POST /api/files/{id}/revisions/{rev}/restore` — restore a revision.

### ZIP download

- `GET /api/files/{id}/zip` — streams a folder as ZIP.
- `POST /api/files/zip` — `{ "ids": [...] }` streams multiple items as one ZIP.

### `GET /api/files/{id}/thumbnail?link=`

Proxies Google's OAuth-protected `thumbnailLink` so `<img>` tags work. `link=` (from list data) skips the metadata round-trip. `Cache-Control: private, max-age=3600`. **404** when no thumbnail exists.

---

## Resumable uploads

Jobs are persisted under `DATA_DIR` and survive restarts; chunk retry resumes from the server's `bytesReceived`.

### `POST /api/uploads`

```json
{ "name": "backup.zip", "size": 10485760, "parentId": "1abc", "mimeType": "application/zip" }
```

Starts a Google Drive **resumable** session → **201** `{ "uploadId", "status": "pending", "total", "name", "fileId": null }`. Zero-byte files may complete immediately.

### `PUT /api/uploads/{uploadId}/chunk`

Binary body, max **32 MiB** per request (`upload.MaxClientChunk`). Sequential append. Optional headers:

- `X-Upload-Offset: <start>` — must equal server `bytesReceived` (mismatch → `409` with current offset, client resumes from there)
- `Content-Range: bytes start-end/total`

Server buffers and flushes to Drive in 256 KiB multiples (adaptive flush: 16 MiB default, 32 MiB for files >200 MB, pipelined with the incoming chunk). The final request may be any size.

**200** `{ "uploadId", "bytesSent", "bytesReceived", "total", "status", "fileId", "error" }`

### `GET /api/uploads/{uploadId}` / `POST /api/uploads/{uploadId}/cancel`

Status polling (same shape as chunk response) / best-effort cancel.

Statuses: `pending` | `uploading` | `completed` | `failed` | `cancelled`.

Client strategy (SPA): ≤5 MB → `POST /api/files/simple`; 5–32 MB → single chunk; larger → 16/32 MiB adaptive chunks with pipeline depth 2.

---

## Overview

### `GET /api/overview`

Proxies Drive `about` (user + storage quota, bytes as integers); server caches for 5 minutes.

```json
{
  "storage": { "limit": 16106127360, "usage": 1234567890, "usageInDrive": 1000 },
  "user": { "email": "user@gmail.com", "displayName": "Ada" }
}
```

SPA additionally shows client-only upload history (`localStorage` `dbc_upload_history`) and type counts from the current folder listing.

---

## `/api/v1` — programmatic API

Stable external contract for AI agents and scripts. Same handlers as the UI API, but auth accepts **cookie or API key** and every route is scope-gated (`read` / `readwrite`).

- `GET /api/v1/openapi.json` — OpenAPI 3 document, no auth (discovery).
- Key management (session cookie only):
  - `POST /api/v1/keys` — `{ "name": "…", "scope": "read"|"readwrite" }` → key shown once (`dbk_…`).
  - `GET /api/v1/keys` — list (no secrets).
  - `DELETE /api/v1/keys/{id}` — revoke.
- Mirrored file routes: list/search/content/download/permissions/revisions/zip/thumbnail/multi-zip require `read`; create/content-write/mkdir/simple/rename/move/copy/trash/share/unshare/restore/batch require `readwrite`.
- Uploads: create/chunk/cancel require `readwrite`; status requires `read`.
- `GET /api/v1/overview` requires `read`.

---

## Middleware / transport notes

- **Gzip** — content-aware: only compressible types (text/JSON/JS/XML/SVG) ≥1 KiB; skips media, Range responses, and pre-encoded bodies. Download/thumbnail/chunk/zip paths bypass entirely.
- **Body limits** — JSON mutation routes 64 KiB–3 MiB (see router); chunk 32 MiB; simple upload 5 MiB.
- **Static SPA** — served from `WEB_DIST_DIR` with immutable caching for hashed assets and client-routing fallback.

## Frontend notes

- Production: same origin as Go (embedded `web/dist`).
- Dev: Vite proxy `/api` + `/oauth2` → `http://localhost:3000`; SPA on `:5174`.
