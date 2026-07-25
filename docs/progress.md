# Progress Log

## 2026-07-22 — Feature pack Tasks 1–8

**Status:** complete (unit tests green; browser checklist manual)

### Done

- [x] Drive: CreateFolder, Trash, CreateFile, Get/Update text content, About
- [x] API: `POST /api/files/mkdir`, `POST /api/files`, `DELETE /api/files/{id}`, content GET/PUT, `GET /api/overview`
- [x] SPA: mkdir/new file/trash, text modal, icon actions, type-specific list icons, Overview cards
- [x] Docs: `api-contract.md` routes; this entry; plan checkboxes

### Verification

| Suite | Result |
|-------|--------|
| `go test ./... -count=1` | pass (api, auth, config, drive, upload) |
| `npx tsc --noEmit` + `npm test -- --run` | pass (20 tests) |
| API `:3000` health | 200 |
| SPA `:5174` | 200 |

### Manual smoke checklist

1. Open **http://localhost:5174/** (cookie host `localhost`)
2. New folder → visible in list
3. New file → save as `note.md` → open modal → edit → save
4. Trash file → removed from list
5. Double-click name: open folder / text without selecting name text
6. List icons differ by type (pdf / txt / font / folder / …)
7. Overview: storage bar, upload history, type breakdown

### Next

- Optional: embed SPA in Go; durable upload jobs; full-drive type stats

---

## 2026-07-21 — OAuth returns to live SPA

**Status:** complete (unit tests); browser re-login manual

### Problem

- Login finished on API `:3000` home (black/plain page) instead of React SPA
- Host mismatch risk: `localhost` vs `127.0.0.1` breaks `dbc_session`

### Fix

- [x] `FRONTEND_ORIGIN` in config (default `http://localhost:5174/`)
- [x] `AuthHandlers.Callback` redirects to SPA after `SetCookie`
- [x] `.env` / `.env.example` updated
- [x] `go test ./internal/config ./internal/api` pass
- [x] API restarted with SOCKS proxy

### How to verify

1. Open **http://localhost:5174/** (not 5173, not 127.0.0.1, not :3000)
2. Connect Google → Google → should land on SPA with Drive list

---

## 2026-07-21 — Formal full-stack smoke (API + Vite proxy)


**Status:** complete (API e2e + proxy; browser click-path still manual)

### Matrix

| Layer | Result |
|-------|--------|
| `npm test` (9) + `tsc -b` | pass |
| API `GET /api/health` | 200 ok |
| Session `GET /api/auth/me` | connected xxt221673@gmail.com |
| Drive `GET /api/files` | 200 list (root) |
| Upload create + `PUT .../chunk` | completed, fileId `17z5gNJiXFO_XPalN9wn84Ga_sPIXHrYU` (`e2e-smoke-3.txt`) |
| Vite `:5174` proxy `/api/*` | health/me/files/create OK |
| React unit wiring | yes |
| Browser manual E2E on SPA | not automated (open http://127.0.0.1:5174 with cookie) |

### Runtime notes

- API must use SOCKS for Google: `HTTPS_PROXY=socks5://127.0.0.1:10808` + `NO_PROXY=localhost,127.0.0.1` (else IPv6 dial timeout).
- Live SPA is **:5174**; frozen demo on **:5173** is not wired.
- Chunk path is `PUT /api/uploads/{id}/chunk`; body size must match create `size`.
- Cookie mint needs real `SESSION_SECRET` from `.env` (not only dev default).

### Servers left running

- API `:3000`, Vite `:5174`

### Next

- Optional: open SPA and click Connect/Upload once; embed SPA in Go; job durability

---

## 2026-07-21 鈥?React live files + upload UI

**Status:** complete (client unit tests)

### Done

- [x] API client: me, logout, listFiles, createUpload, putUploadChunk, uploadFile
- [x] App: auth gate, file table, crumbs, download, Upload + progress bar
- [x] `npm test` 鈥?9 tests pass; `tsc -b` clean

### How to try

1. API on `:3000` with `.env` + token
2. `cd web && npm run dev` 鈫?`:5174`
3. Connect Google (proxy `/oauth2`) then reopen Vite if needed
4. Browse Drive; Upload sends chunks 鈮? MiB

### Next

- Embed SPA in Go; polish demo layout later if needed

---

## 2026-07-21 鈥?React file browser (live API)

**Status:** complete (unit-tested client; browser needs session cookie)

### Done

- [x] `web/src/lib/api.ts`: `fetchMe`, `logout`, `listFiles`, `ApiError`, format helpers
- [x] `App.tsx`: auth gate, Connect Google, file table, folder crumbs, download link, disconnect
- [x] Vite proxy already maps `/api` + `/oauth2` 鈫?`:3000`
- [x] `npm test` 鈥?7 tests pass

### Notes

- After OAuth, callback lands on API `:3000/`; reopen Vite (`:5174`) 鈥?cookie is host-only on `localhost` (shared across ports).
- Upload UI still pending.

---

## 2026-07-21 鈥?Live upload API smoke

**Status:** complete

### Done

- [x] `POST /api/uploads` with session 鈫?201 pending job
- [x] `PUT .../chunk` 66-byte text file 鈫?`status=completed` + Drive `fileId`
- [x] File visible in `GET /api/files` (`dbc-smoke-*.txt`)

### Verification

| Step | Result |
|------|--------|
| create | 201, uploadId issued |
| chunk | 200, completed, bytesSent=total |
| list | smoke filename present |

### Notes

- End-to-end path works: OAuth token 鈫?Drive resumable upload 鈫?list.
- Next: React file table + upload UI.

---

## 2026-07-21 鈥?Live OAuth smoke + home route

**Status:** complete

### Done

- [x] `.env` from Google client secret (gitignored) + `config.LoadDotEnv`
- [x] Server `oauth=true` on `:3000`
- [x] Browser Google consent 鈫?`data/token.json` for test user
- [x] Fix post-login **404**: callback redirected to `/` with no route 鈫?added `GET /{$}` HTML home
- [x] Live smoke: `/api/auth/me` + `/api/files` with session cookie 鈫?200 (Drive list OK)

### Verification

| Check | Result |
|-------|--------|
| OAuth exchange + token file | pass |
| `GET /` after login | HTML 鈥淪igned in as 鈥︹€?(after fix) |
| `GET /api/auth/me` | `connected:true` |
| `GET /api/files` | folder items from Drive |
| unit tests api/auth/config | pass |

### Notes

- Browser session cookie is set on callback; open `http://localhost:3000/` in the **same browser** that finished OAuth.
- Temporary helper `cmd/mintcookie` exists for API smoke without browser cookie jar (dev only; do not ship).
- Next: React file table + upload UI; optional small upload smoke via API.

---

## 2026-07-21 鈥?Milestone 3: Resumable upload + progress

**Status:** complete (unit-tested with mocked Drive; UI progress still optional)

### Done

- [x] `internal/drive`: ResumableStart, UploadRange, UploadEmpty (256 KiB multiples)
- [x] `internal/upload`: job store, Service.Create/AppendChunk/Cancel/Get
- [x] `POST /api/uploads`, `PUT .../chunk`, `GET .../{id}`, `POST .../cancel` (+ RequireSession)
- [x] Per-request Drive client from OAuth HTTP client
- [x] Unit tests: drive resumable, upload service, upload handlers
- [x] Docs: api-contract + progress + plan

### Verification

| Suite | Result |
|-------|--------|
| `go test ./...` | pass (api, auth, config, drive, upload) |

### Notes

- Upload jobs are **in-memory** only (lost on process restart).
- Client chunks need not be 256 KiB; server re-packs.
- Live upload needs OAuth token + Drive API enabled.
- UI progress rail / React upload widget still pending (frontend).

### Next

- Optional: React file table + upload UI against API
- Live OAuth smoke with `.env` secrets
- M4+: embed SPA, durability for jobs if needed

---

## 2026-07-21 鈥?Milestone 2: Drive list + download

**Status:** complete (unit-tested against mocked Drive HTTP)

### Done

- [x] `internal/drive`: Client.List / GetMeta / Download, APIError mapping
- [x] `GET /api/files?folderId=&pageToken=` (RequireSession)
- [x] `GET /api/files/{id}/download` stream + Content-Disposition
- [x] `auth.Service.HTTPClient` for token refresh + persist
- [x] Unit tests: drive client + files handlers
- [x] Docs: contract + progress

### Verification

| Suite | Result |
|-------|--------|
| `go test ./...` | pass (api, auth, config, drive) |

### Notes

- Native Google Docs types not exportable in M2.
- Live Drive needs OAuth token file from M1 browser login.
- Optional: minimal React file table still thin (web shell health-only); demo remains polish6.

---

## 2026-07-21 鈥?Milestone 1: OAuth + session

**Status:** complete (unit-tested; live Google login needs env secrets)

### Done

- [x] `internal/auth`: OAuth config, CSRF state, token JSON store, session cookie, service
- [x] Routes: `GET /oauth2/login`, `GET /oauth2/callback`, `GET /api/auth/me`, `POST /api/auth/logout`
- [x] `api.Deps` wiring in `router.go` + `cmd/server/main.go`
- [x] Unit tests with mocks / httptest
- [x] Dep: `golang.org/x/oauth2 v0.30.0`
- [x] Docs: `api-contract.md` + this progress entry

### Verification

| Suite | Result |
|-------|--------|
| `go test ./...` | pass (api, auth, config) |
| Live OAuth | pending 鈥?set Google secrets |

### Notes

- Without Google secrets: login/callback 鈫?`503 oauth_not_configured`.
- Session cookie: `dbc_session`. Go get: SOCKS5 + goproxy.cn if needed.

---

## 2026-07-21 鈥?Milestone 0: Process + scaffold

**Status:** complete

- DEV_WORKFLOW, Go health, Vite shell, docs, demo freeze polish6.
## 2026-07-21 — UI align (toast + drop)

**Status:** complete (tsc + vitest; browser smoke via DevTools)

### Done
- [x] Toast host: enter → is-in (setTimeout, not rAF) → out
- [x] Settings Banner toggle feedback toast
- [x] Upload complete / error / logout toasts (prefs-aware)
- [x] Drop overlay on `#main-drop` (dragDepth)
- [x] `npx tsc --noEmit` + `npm test` (9) pass

### Try
1. http://localhost:5174/ → Settings → Banner alerts → toast slides in
2. Drag files onto main panel → teal dashed overlay
3. Upload → Activity rail + optional toast

### Next
- Optional: embed SPA in Go; job durability; README port note
