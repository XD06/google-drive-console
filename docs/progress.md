# Progress Log

## 2026-07-27 → 08-12 — Bug audit fixes + frontend store split

**Status:** complete (unit/build green; browser E2E still manual) — ready to push

### Theme

Full-stack bug audit (`BUG_AUDIT_2026-07-27.md`) + architecture notes (`docs/ARCHITECTURE_RECOMMENDATIONS_2026-07-27.md`), then ship the high-priority correctness/security fixes and start the App.tsx de-godification.

### Backend (security / correctness / durability)

- [x] **C1** Thumbnail `?link=` allowlist + redirect re-check (block SSRF / OAuth token exfil) + `thumbnail_url_test.go`
- [x] **H3** Upload flush generation (`flushGen`) so concurrent chunk PUTs cannot double-flush / flip completed→failed
- [x] **H4** Terminal-job reaper (10m tick, 1h maxAge); nil buffer on fail path
- [x] **H5** `Service` takes `JobStore`; wire `*PersistentStore` so debounced `uploads.json` saves actually run
- [x] **M2** Zip walk: visited set, max depth 20, max 5000 files; multi-zip item cap 200
- [x] Download: log mid-stream copy failures; honor `If-None-Match` from list hints **before** opening media
- [x] Overview about-cache keyed by session email (no cross-user leak)
- [x] Drive query escape: backslash before quote; retry body drain capped at 1 MiB
- [x] Dev helpers: `start.sh` / `start.bat` / `stop.sh` / `stop.bat`; ignore `.dev-pids`

### Frontend (races / re-render / structure)

- [x] **H1 / M1** `web/src/lib/fileStore.ts` — list/pagination/cache via `useSyncExternalStore` (stale load-more guard, LRU cache)
- [x] **M1 / H7** `web/src/lib/uploadSession.ts` — upload jobs outside App; logout aborts all controllers
- [x] **H6** image/editor open generation refs (no stale blob / editor clobber)
- [x] Split pages: `FilesPage.tsx`, `OverviewPage.tsx`; top-level `ErrorBoundary`
- [x] Upload progress: real Drive-phase polling (drop fake 50–95% estimate); monotonic hi-water
- [x] Delete unused `ImageLightbox.tsx`; CSS/hooks polish for phase-3 layout
- [x] Smoke screenshots: `ph3-files.png`, `ph3-overview.png`, `web/phase2-smoke-gate.png`

### Verification (2026-08-12)

| Suite | Result |
|-------|--------|
| `go test -count=1 ./...` | pass (api, apikey, auth, config, drive, upload) |
| `cd web && npm test` | 60/60 pass (incl. new `fileStore.test.ts`) |
| `cd web && npm run build` (`tsc -b` + vite) | pass |

### Still open / next

- Architecture recs not fully done: `drive.Backend` interface, further handler split, more App shell thinning
- Remaining audit items (medium/low): e.g. passive wheel on lightbox, apikey disk-under-mutex, some dead prefs
- No automated browser E2E — manual smoke still recommended after deploy

---

## 2026-07-25 — Docs refresh + repo tidy

**Status:** complete

- `api-contract.md` rewritten to match current router: search, copy, share/permissions, revisions, zip/multi-zip, thumbnail, batch, simple upload, `/api/v1` + API keys + OpenAPI; chunk max corrected to 32 MiB
- README / DEV_WORKFLOW brought current (GitHub repo, agent API, commands)
- Archived to `docs/archive/`: `google-services-dev-guide.md`, `docs/superpowers/`
- Removed frozen mock demos: `web/demo/`, `web/demo-directions/`, `web/preview.html`

---

## 2026-07-23 → 07-25 — Post-v1 feature & perf rounds (summary)

**Status:** complete — pushed to GitHub `XD06/drive-backup-console` (master)

- feat: Spotlight-style search modal (Ctrl/Cmd+K, scope pills, 250ms suggest) — `84ea925`
- perf: list `pageSize` passthrough, content-aware gzip, 32 MiB upload flush alignment — `469ba74`
- feat: agent-ready `/api/v1` (API keys, scopes, OpenAPI), Docker multi-stage deploy
- fix series: upload progress bar, streaming video/PDF preview, chunk retry with offset resume, lightbox navigation + video speed badge — `bec88f3`…`4ba26c9`
- Verification: `go test ./...` pass · vitest 40/40 · `tsc --noEmit` clean

---

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

## 2026-07-21 — React live files + upload UI

**Status:** complete (client unit tests)

### Done

- [x] API client: me, logout, listFiles, createUpload, putUploadChunk, uploadFile
- [x] App: auth gate, file table, crumbs, download, Upload + progress bar
- [x] `npm test` — 9 tests pass; `tsc -b` clean

### How to try

1. API on `:3000` with `.env` + token
2. `cd web && npm run dev` → `:5174`
3. Connect Google (proxy `/oauth2`) then reopen Vite if needed
4. Browse Drive; Upload sends chunks ≤5 MiB

### Next

- Embed SPA in Go; polish demo layout later if needed

---

## 2026-07-21 — React file browser (live API)

**Status:** complete (unit-tested client; browser needs session cookie)

### Done

- [x] `web/src/lib/api.ts`: `fetchMe`, `logout`, `listFiles`, `ApiError`, format helpers
- [x] `App.tsx`: auth gate, Connect Google, file table, folder crumbs, download link, disconnect
- [x] Vite proxy already maps `/api` + `/oauth2` → `:3000`
- [x] `npm test` — 7 tests pass

### Notes

- After OAuth, callback lands on API `:3000/`; reopen Vite (`:5174`) — cookie is host-only on `localhost` (shared across ports).
- Upload UI still pending.

---

## 2026-07-21 — Live upload API smoke

**Status:** complete

### Done

- [x] `POST /api/uploads` with session → 201 pending job
- [x] `PUT .../chunk` 66-byte text file → `status=completed` + Drive `fileId`
- [x] File visible in `GET /api/files` (`dbc-smoke-*.txt`)

### Verification

| Step | Result |
|------|--------|
| create | 201, uploadId issued |
| chunk | 200, completed, bytesSent=total |
| list | smoke filename present |

### Notes

- End-to-end path works: OAuth token → Drive resumable upload → list.
- Next: React file table + upload UI.

---

## 2026-07-21 — Live OAuth smoke + home route

**Status:** complete

### Done

- [x] `.env` from Google client secret (gitignored) + `config.LoadDotEnv`
- [x] Server `oauth=true` on `:3000`
- [x] Browser Google consent → `data/token.json` for test user
- [x] Fix post-login **404**: callback redirected to `/` with no route → added `GET /{$}` HTML home
- [x] Live smoke: `/api/auth/me` + `/api/files` with session cookie → 200 (Drive list OK)

### Verification

| Check | Result |
|-------|--------|
| OAuth exchange + token file | pass |
| `GET /` after login | HTML “Signed in as …”(after fix) |
| `GET /api/auth/me` | `connected:true` |
| `GET /api/files` | folder items from Drive |
| unit tests api/auth/config | pass |

### Notes

- Browser session cookie is set on callback; open `http://localhost:3000/` in the **same browser** that finished OAuth.
- Temporary helper `cmd/mintcookie` exists for API smoke without browser cookie jar (dev only; do not ship).
- Next: React file table + upload UI; optional small upload smoke via API.

---

## 2026-07-21 — Milestone 3: Resumable upload + progress

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

## 2026-07-21 — Milestone 2: Drive list + download

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

## 2026-07-21 — Milestone 1: OAuth + session

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
| Live OAuth | pending — set Google secrets |

### Notes

- Without Google secrets: login/callback → `503 oauth_not_configured`.
- Session cookie: `dbc_session`. Go get: SOCKS5 + goproxy.cn if needed.

---

## 2026-07-21 — Milestone 0: Process + scaffold

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
