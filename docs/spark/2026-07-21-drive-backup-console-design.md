# Drive Backup Console — Design Spec

**Date:** 2026-07-21  
**Status:** Approved for implementation planning (pending user final review of this file)  
**Workspace:** `C:\Users\dsk\Desktop\google-drive` (folder-backed; not a git repo)

## 1. Problem & goals

Build a **self-hosted, high-performance web console** that uses the user’s personal Google Drive (AI Pro / large quota) as a backup “NAS-like” store.

**v1 success criteria**

- User connects Google account via OAuth (test user under Publishing status **Testing**).
- Browse folders and list files (breadcrumbs / parent navigation).
- Upload files with **progress** (large files via Drive **resumable** upload).
- Download files through the backend (stream proxy).
- Modern, clean UI: simple layout, not sparse; visual enough for directory + progress.

**Out of scope for v1**

- Multi-user accounts / team admin
- Scheduled / cron backups, exclude rules, version diff
- Service accounts for personal My Drive
- Publishing the OAuth app to production verification
- Apps Script–based upload path

## 2. Constraints (from product & GCP)

| Item | Decision / fact |
|------|------------------|
| Drive access | User OAuth only (`https://www.googleapis.com/auth/drive`) |
| GCP project | `gen-lang-client-0283045431` (“Gemini API”) |
| OAuth client | Web application “Drive Backup Console” |
| Client ID | `637837932628-prfqskog23eklqj7dreak44brp9bfhuo.apps.googleusercontent.com` |
| Redirect URI | `http://localhost:3000/oauth2/callback` (must match route exactly) |
| Consent | External, **Testing**; test user `xxt221673@gmail.com` |
| Deploy | Local Windows first; same binary later on a server (env + redirect update) |
| Drive limits (design awareness) | File up to 5TB; ~750GB/day upload; resumable for >5MB; chunks multiples of 256KB |

Secrets (`client_secret`, refresh tokens) never commit to source control.

## 3. Architecture

```
Browser (SPA UI)
    │  same origin http://localhost:3000
    ▼
Go single process
    ├── embed frontend dist
    ├── OAuth  /oauth2/login, /oauth2/callback
    ├── Session cookie (HttpOnly) — browser never holds Google tokens
    ├── REST API  list / upload sessions / download
    └── Token store (local file under DATA_DIR / TOKEN_PATH)
            │
            ▼
    Google Drive API v3 (resumable uploads)
```

**Why this shape**

- One process for local use and later server deploy.
- Tokens stay on the server process disk, not in localStorage.
- Redirect URI already registered for port 3000.

## 4. Tech stack

### Backend

| Layer | Choice |
|-------|--------|
| Language | Go |
| Entry | `cmd/server` |
| HTTP | `net/http` with optional lightweight router (e.g. chi) if routing grows |
| OAuth | `golang.org/x/oauth2`, Google endpoint |
| Drive | `google.golang.org/api/drive/v3` |
| Frontend serve | `//go:embed` of built `web/dist` |
| Config | Environment variables (+ optional `.env` loader for local only) |

**Internal packages**

- `internal/config` — env, paths, port
- `internal/auth` — OAuth login/callback, state, token load/save/refresh
- `internal/drive` — list, download stream, resumable session + chunk send
- `internal/api` — HTTP handlers, session middleware
- `internal/upload` — in-memory or disk-backed upload job state + progress
- `web/` — Vite app source; build output embedded from `web/dist`

### Frontend

| Layer | Choice |
|-------|--------|
| Tooling | Vite |
| UI | React + TypeScript |
| Style | Tailwind CSS |
| UX | Directory table, breadcrumbs, drag-and-drop upload, progress list |

Production: assets embedded and served by Go on the same origin (no CORS for API in prod).

### Configuration (v1)

| Variable | Purpose | Local default |
|----------|---------|----------------|
| `PORT` | Listen port | `3000` |
| `GOOGLE_CLIENT_ID` | OAuth client ID | from downloaded JSON |
| `GOOGLE_CLIENT_SECRET` | OAuth client secret | from downloaded JSON |
| `OAUTH_REDIRECT_URL` | Must match GCP | `http://localhost:3000/oauth2/callback` |
| `DATA_DIR` | App data root | e.g. `./data` |
| `TOKEN_PATH` | Refresh/access token JSON path | under `DATA_DIR` |
| `SESSION_SECRET` | Cookie signing | random local secret |
| `ROOT_FOLDER_ID` | Optional Drive folder root | empty = Drive root or auto-create `Backup` |

## 5. Modules & responsibilities

| Unit | Does | Depends on | Interface |
|------|------|------------|-----------|
| `auth` | Start OAuth, handle callback, persist tokens, refresh access | config, oauth2 | Token source for Drive client |
| `drive` | files.list, media download, resumable init + chunk | Drive API + token | Pure Drive operations |
| `upload` | Job id, size, progress, session URL | drive | Create job, accept chunks, query progress |
| `api` | Routes, session cookie gate, JSON errors | auth, drive, upload | HTTP only |
| `web` UI | Auth button, file browser, upload/download UX | `/api/*`, `/oauth2/*` | Browser only |

Each unit is testable with mocks at the Drive/token boundary.

## 6. Data flows

### 6.1 Login

1. User opens UI → if no session, show “Connect Google”.
2. `GET /oauth2/login` → redirect to Google with `access_type=offline` and `prompt=consent` (so a **refresh_token** is issued on first link).
3. User approves as test user.
4. `GET /oauth2/callback?code=&state=` → validate `state` → exchange code → write tokens to `TOKEN_PATH` (file permissions restricted on OS) → set HttpOnly session cookie → redirect to UI `/`.

### 6.2 List / navigate

- `GET /api/files?folderId=&pageToken=`  
  - Calls Drive `files.list` with parent query, fields needed for name/mime/size/modified.  
  - Returns items + optional `nextPageToken`.  
- UI breadcrumbs built from stack of folder ids (v1: client-side stack or path segments resolved by id).

### 6.3 Upload

1. `POST /api/uploads` body: `{ name, size, parentId, mimeType }` → backend creates Drive resumable session → returns `{ uploadId }`.
2. Client sends file bytes as chunks: `PUT /api/uploads/{uploadId}/chunk` with range headers or sequential chunks (implementation detail: backend re-packs to 256KB multiples toward Drive).
3. Progress: `GET /api/uploads/{uploadId}` returns `{ bytesSent, total, status }` **or** SSE `GET /api/uploads/{uploadId}/events` if easy in same stack.
4. On complete, Drive file id returned; UI refreshes list.

**v1 preference:** browser → backend → Drive (backend holds token). Optional later: client-direct resumable with short-lived session URL only if security review allows.

### 6.4 Download

- `GET /api/files/{id}/download`  
  - Backend streams Drive media to response with `Content-Disposition` filename.  
  - No Google access token in browser.

## 7. Security

- Keep OAuth app in **Testing**; only listed test users.
- Google client secret and refresh token **only** on server filesystem / env.
- OAuth `state` CSRF check required.
- Session cookie: `HttpOnly`, `SameSite=Lax` (local HTTP); on server deploy switch to HTTPS + `Secure`.
- Same-origin API in production embed mode.
- Clear UX when refresh fails: force re-login.
- Surface Drive quota / daily upload errors as readable messages (no stack traces to UI).

## 8. Error handling

| Case | Behavior |
|------|----------|
| Not logged in | `401` + UI login CTA |
| Invalid/expired Google token and refresh fails | Clear session, re-auth |
| Upload interrupted | Job remains resumable if Drive session URL still valid; else new job |
| File too large / day quota | `429`/`403` mapped to Chinese/English user message |
| Missing folder id | `400` with hint |

## 9. Testing strategy (v1)

- Unit: OAuth state validation; token file round-trip; upload progress math.
- Drive client: interface + mock for list/upload/download handlers.
- Manual checklist: login → list → small upload → multi-chunk large upload → download → re-login after deleting token file.

No requirement to add heavy e2e frameworks in v1.

## 10. Repository layout (target)

```
google-drive/
  docs/spark/2026-07-21-drive-backup-console-design.md  # this file
  cmd/server/main.go
  internal/config/
  internal/auth/
  internal/drive/
  internal/upload/
  internal/api/
  web/                 # Vite React TS
  data/                # gitignored local tokens
  .env.example
  go.mod
  README.md
  google-services-dev-guide.md  # existing reference; not the app
```

## 11. Implementation milestones (for later planning only)

1. Scaffold Go module + config + embed stub + health route on `:3000`.
2. OAuth login/callback + token persist + session gate.
3. Drive list + download.
4. Resumable upload + progress API + UI progress.
5. Polish UI (drag-drop, empty states) + README runbook (env, GCP checklist).

## 12. Non-goals reaffirmation

- No service account for My Drive.
- No publish/verification flow unless product scope expands to third parties.
- No multi-tenant or public internet auth beyond “later server + HTTPS + optional app password”.

## 13. Decisions log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Backend runtime | Go | User preference; concurrent/stream-friendly |
| Packaging | Single binary + embedded SPA | Local-first, simple later deploy |
| Frontend | React + TS + Tailwind + Vite | Modern visual console without overbuilding |
| v1 features | Auth + browse + upload/download + progress | Enough for backup console MVP |
| Token storage | Server-side file | Keeps secrets off the browser |
| Upload path | Via backend resumable | Simpler security; progress under our control |

---

**End of design.** Implementation must not start until the user explicitly approves this written spec and requests an implementation plan or coding.
