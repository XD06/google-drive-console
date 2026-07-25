# API Contract (v1)

**Status:** draft aligned with design; implemented routes marked ✅  
**Base URL (local):** `http://localhost:3000`  
**Content-Type:** `application/json` unless noted  

## Error envelope

```json
{
  "error": {
    "code": "not_authenticated",
    "message": "Sign in with Google required"
  }
}
```

Common HTTP status: `400` validation, `401` auth, `403` Drive/permission, `404` missing, `405` method, `429` quota, `502` upstream Google.

---

## ✅ Health

### `GET /api/health`

No auth. Liveness for scaffold and ops.

**200**

```json
{
  "status": "ok",
  "service": "drive-backup-console",
  "timeUtc": "2026-07-21T03:00:00Z",
  "devMode": true
}
```

`devMode` omitted or false in production config.

---

## ✅ OAuth + session

Requires `GOOGLE_CLIENT_ID` + `GOOGLE_CLIENT_SECRET`. Without them, login/callback return `503 oauth_not_configured` (DevMode may still run health/me unauthenticated).

### `GET /oauth2/login`

Redirects browser to Google OAuth (`access_type=offline`, `prompt=consent` via `ApprovalForce`). Issues short-lived CSRF `state` (in-memory).

**302** → Google consent URL  
**503** if OAuth not configured

### `GET /oauth2/callback?code=&state=`

Validates `state`, exchanges code, persists tokens under `TOKEN_PATH` (JSON), sets HttpOnly session cookie `dbc_session` (HMAC-SHA256), redirects to `/`.

**302** → `/` on success  
**400** `oauth_callback_failed` / `oauth_denied`  
**503** if OAuth not configured

### `GET /api/auth/me`

**401** `not_authenticated` if no/invalid session cookie.

**200**

```json
{
  "email": "user@gmail.com",
  "connected": true
}
```

`connected` is true when session email matches a stored token file.

### `POST /api/auth/logout`

Clears session cookie. Optional `?clearToken=1` also deletes local token file.

**200**

```json
{
  "ok": true,
  "loggedOut": true,
  "at": "2026-07-21T03:00:00Z"
}
```

---

## ✅ Files

### `GET /api/files?folderId=&pageToken=`

Requires session cookie. `folderId` defaults to `ROOT_FOLDER_ID` env or `root`.

**200**

```json
{
  "folderId": "root",
  "items": [
    {
      "id": "1abc",
      "name": "Backups",
      "mimeType": "application/vnd.google-apps.folder",
      "size": null,
      "modifiedTime": "2026-07-20T10:00:00Z",
      "isFolder": true
    },
    {
      "id": "2def",
      "name": "archive.zip",
      "mimeType": "application/zip",
      "size": 1048576,
      "modifiedTime": "2026-07-21T01:00:00Z",
      "isFolder": false
    }
  ],
  "nextPageToken": null
}
```

### `GET /api/files/{id}/download`

Requires session. Streams binary media; `Content-Disposition: attachment`. Folders and Google Docs native types return `400` (`is_folder` / `export_required`).

### `POST /api/files/mkdir` ✅

Requires session. Creates a folder under `parentId` (default `ROOT_FOLDER_ID` or `root`).

```json
{ "name": "Backups", "parentId": "root" }
```

**201** — `FileItem` (`isFolder: true`)  
**400** empty name / invalid JSON

### `POST /api/files` ✅

Requires session. Creates a file (optional initial text `content`).

```json
{
  "name": "note.md",
  "parentId": "root",
  "mimeType": "text/markdown",
  "content": "# hello"
}
```

`mimeType` optional (server/Drive default when empty). `content` optional (empty file).

**201** — `FileItem`  
**400** empty name / invalid JSON

### `DELETE /api/files/{id}` ✅

Requires session. Moves the file/folder to Drive trash (`trashed: true`).

**204** empty body  
**400** missing id

### `PATCH /api/files/{id}` ✅

Requires session. Renames a file or folder.

```json
{ "name": "renamed.md" }
```

**200** — updated `FileItem`  
**400** empty name / missing id / invalid JSON

### `POST /api/files/{id}/move` ✅

Requires session. Moves a file or folder under a new parent (Drive `addParents` / `removeParents`).

```json
{ "parentId": "folderIdOrRoot" }
```

**200** — updated `FileItem`  
**400** missing `parentId` / missing id / invalid JSON

### `GET /api/files/{id}/content` ✅

Requires session. Returns UTF-8 text for previewable text files (not folders / binary / native Docs export).

**200**

```json
{
  "content": "file body…",
  "mimeType": "text/plain",
  "size": 42,
  "name": "note.txt"
}
```

### `PUT /api/files/{id}/content` ✅

Requires session. Replaces text media body.

```json
{ "content": "updated body" }
```

**200** `{ "ok": true }`

---

## Overview ✅

### `GET /api/overview`

Requires session. Proxies Drive `about` (`user` + `storageQuota`). Quota numbers are integers (bytes).

**200**

```json
{
  "storage": {
    "limit": 16106127360,
    "usage": 1234567890,
    "usageInDrive": 1000
  },
  "user": {
    "email": "user@gmail.com",
    "displayName": "Ada"
  }
}
```

**401** without session. SPA also shows client-only upload history (`localStorage` key `dbc_upload_history`) and type counts from the **current folder listing** (not a full-drive scan).

---

## Uploads (M3) ✅

Requires session. Jobs live in memory for the process lifetime (restart loses in-flight jobs).

### `POST /api/uploads`

```json
{
  "name": "backup.zip",
  "size": 10485760,
  "parentId": "1abc",
  "mimeType": "application/zip"
}
```

Starts a Google Drive **resumable** session. Empty `parentId` → `ROOT_FOLDER_ID` or `root`.

**201**

```json
{
  "uploadId": "up_xxx",
  "status": "pending",
  "total": 10485760,
  "name": "backup.zip",
  "fileId": null
}
```

Zero-byte files may return `status: "completed"` with `fileId` set immediately.

### `PUT /api/uploads/{uploadId}/chunk`

Binary body (max **8 MiB** per request). Sequential append.

Optional headers:

- `X-Upload-Offset: <start>` — must match server `bytesReceived`
- `Content-Range: bytes start-end/total` — start used as offset check

Server buffers and flushes to Drive in **256 KiB** multiples; the **final** request may be any size.

**200**

```json
{
  "uploadId": "up_xxx",
  "bytesSent": 5242880,
  "bytesReceived": 5242880,
  "total": 10485760,
  "status": "uploading",
  "fileId": null,
  "error": null
}
```

### `GET /api/uploads/{uploadId}`

Same JSON shape as chunk response (includes `name` / `error` when set).

### `POST /api/uploads/{uploadId}/cancel`

Best-effort cancel (`status: cancelled`). Does not delete partial Drive objects.

Statuses: `pending` | `uploading` | `completed` | `failed` | `cancelled`.

---

## Auth gate

- Public: `GET /api/health`, `GET /oauth2/login`, `GET /oauth2/callback`, `GET /api/auth/me`, `POST /api/auth/logout`
- Drive list/download/mutate/content/overview/uploads: `RequireSession` (session cookie). Token file used when calling Google via OAuth client.

## Frontend notes

- Production: same origin as Go (embed).
- Dev: Vite proxy `/api` and `/oauth2` → `http://localhost:3000`.
- Connect Google → navigate to `/oauth2/login`.