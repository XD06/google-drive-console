---
name: drive-backup-console-api
description: Operate a self-hosted Drive Backup Console (Google Drive gateway) through its /api/v1 REST API — browse folders, search, read/write text files, upload files of any size, download, share, trash/move/copy, ZIP folders. Use when an agent needs programmatic file storage on the user's Google Drive via an API key, or when the user mentions the Drive Backup Console, dbc_ API keys, or /api/v1 file operations.
---

# Drive Backup Console API

Self-hosted gateway to the user's Google Drive. All operations go through
`{BASE_URL}/api/v1/*` (default local: `http://localhost:3000`).

## Setup

1. **Get an API key** — keys are minted with a browser session (cookie auth):
   the user creates one in the Web UI under **Settings → API Keys**, or via
   `POST /api/v1/keys` with `{"name":"agent","scope":"readwrite"}`. The token
   (`dbc_…`) is shown **once** at creation. Ask the user for it if no key exists yet.
2. **Authenticate every request**:

```bash
curl -H "Authorization: Bearer dbc_xxx" {BASE_URL}/api/v1/files
```

Scopes: `read` (list/search/download/status) or `readwrite` (everything).
A `403` means the key's scope is insufficient; a `401` means missing/revoked key.

**Discovery**: `GET /api/v1/openapi.json` (no auth) returns the full OpenAPI 3 spec.
Fetch it when an endpoint's exact schema is needed.

## Endpoint quick reference

| Action | Endpoint | Scope |
|--------|----------|-------|
| List folder | `GET /api/v1/files?folderId=&pageToken=&pageSize=100` | read |
| Search | `GET /api/v1/files/search?q=&scope=folder\|drive&folderId=` | read |
| Download | `GET /api/v1/files/{id}/download` (supports `Range`) | read |
| Read text | `GET /api/v1/files/{id}/content` | read |
| Write text | `PUT /api/v1/files/{id}/content` `{"content":"…"}` | readwrite |
| Create text file | `POST /api/v1/files` `{"name","parentId","mimeType","content"}` | readwrite |
| Upload ≤5 MiB | `POST /api/v1/files/simple?name=&parentId=&mimeType=` (raw body) | readwrite |
| Upload large | `POST /api/v1/uploads` then `PUT /api/v1/uploads/{id}/chunk` | readwrite |
| New folder | `POST /api/v1/files/mkdir` `{"name","parentId"}` | readwrite |
| Rename | `PATCH /api/v1/files/{id}` `{"name"}` | readwrite |
| Move | `POST /api/v1/files/{id}/move` `{"parentId"}` | readwrite |
| Copy | `POST /api/v1/files/{id}/copy` `{"parentId","name"}` | readwrite |
| Trash | `DELETE /api/v1/files/{id}` → 204 | readwrite |
| Batch trash/move | `POST /api/v1/files/batch` `{"action","ids",["parentId"]}` (≤100 ids) | readwrite |
| Share link | `POST /api/v1/files/{id}/share` `{"role":"reader"}` | readwrite |
| Unshare | `DELETE /api/v1/files/{id}/share?permissionId=` | readwrite |
| Permissions | `GET /api/v1/files/{id}/permissions` | read |
| Revisions | `GET /api/v1/files/{id}/revisions` · `POST …/revisions/{rev}/restore` | read / readwrite |
| Folder as ZIP | `GET /api/v1/files/{id}/zip` | read |
| Multi ZIP | `POST /api/v1/files/zip` `{"ids":[…]}` | read |
| Thumbnail | `GET /api/v1/files/{id}/thumbnail` | read |
| Storage quota | `GET /api/v1/overview` | read |

## Core concepts

- **IDs, not paths.** Every file/folder is addressed by its Drive ID. Resolve paths by
  walking: list `folderId=root`, find the child folder's `id`, list again, etc.
  `parentId` empty/omitted defaults to the configured root.
- **List response**: `{ "folderId", "items": [FileItem…], "nextPageToken" }`.
  `FileItem`: `{ "id", "name", "mimeType", "size", "modifiedTime", "isFolder" }`.
  Keep requesting with `pageToken` until `nextPageToken` is null.
- **Native Google Docs** (`application/vnd.google-apps.*`) cannot be downloaded raw —
  the API returns `400 export_required`. Folders return `400 is_folder` (use `/zip`).

## Uploading files

**≤5 MiB — one request:**

```bash
curl -X POST -H "Authorization: Bearer dbc_xxx" \
  --data-binary @photo.jpg \
  "{BASE_URL}/api/v1/files/simple?name=photo.jpg&parentId=root&mimeType=image/jpeg"
# → 201 FileItem
```

**>5 MiB — resumable protocol:**

```bash
# 1. Create job (size in bytes is required and must be exact)
curl -X POST -H "Authorization: Bearer dbc_xxx" -H "Content-Type: application/json" \
  -d '{"name":"backup.zip","size":104857600,"parentId":"root","mimeType":"application/zip"}' \
  {BASE_URL}/api/v1/uploads
# → 201 { "uploadId": "up_xxx", "status": "pending", … }

# 2. Send sequential chunks (max 32 MiB each; last one may be smaller)
curl -X PUT -H "Authorization: Bearer dbc_xxx" \
  -H "X-Upload-Offset: 0" --data-binary @chunk0 \
  {BASE_URL}/api/v1/uploads/up_xxx/chunk
# → 200 { "bytesReceived": 33554432, "status": "uploading", … }
# Repeat with X-Upload-Offset = previous bytesReceived until status = "completed"
```

Rules:
- Chunks are strictly sequential; `X-Upload-Offset` must equal the server's current
  `bytesReceived`. On mismatch the server returns `400` with message
  `expected offset N, got M` — recover by reading `bytesReceived` from
  `GET /api/v1/uploads/{id}` and re-sending from there (retries are safe).
- Final response carries `"status":"completed"` and the new `fileId`.
- Jobs survive server restarts; `POST …/cancel` abandons one.

## Downloading

```bash
curl -H "Authorization: Bearer dbc_xxx" -o out.bin \
  {BASE_URL}/api/v1/files/{id}/download
```

`Range: bytes=0-…` is honored (partial/parallel downloads). Response streams; do not
buffer large files in memory.

## Error handling

All errors: `{ "error": { "code": "…", "message": "…" } }`.

| Status | Meaning | Action |
|--------|---------|--------|
| 400 | validation (incl. upload offset mismatch) | for offset errors: re-sync via `GET /api/v1/uploads/{id}` |
| 401 | bad/missing key or session | stop; ask user for a valid key |
| 403 | insufficient scope / Drive permission | stop; needs `readwrite` key |
| 429 | Google quota | back off (exponential, respect `Retry-After`) |
| 502 | upstream Google error | retry once, then surface |

## Constraints

- Simple upload cap 5 MiB; chunk cap 32 MiB; JSON bodies ≤3 MiB (`content` write ≤3 MiB).
- Batch operations cap at 100 ids, actions `trash` | `move` only.
- Share default role is `reader`; the response includes the public link.
- Google-side daily limits apply (750 GB upload/day per account).
