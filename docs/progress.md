# Progress Log

## 2026-10-03 — Round 5b: deploy-ready Docker defaults

**Status:** complete (go test all green; vitest 60 pass; tsc clean; deployed to VPS)

- [x] **Loopback port binding:** `docker-compose.yml` now maps `127.0.0.1:${PORT:-3000}:3000` instead of publishing on all interfaces — the intended setup is a host reverse proxy in front, and the API should not be directly reachable. `PORT` in `.env` picks the host port. Documented in `docs/docker.md` (new "Port binding" note).
- [x] **Arch-generic build:** the backend build stages no longer hardcode `GOARCH=amd64`; they use BuildKit's auto-injected `TARGETARCH` (amd64 fallback for legacy builders), so the same Dockerfile builds on ARM VPS.
- [x] `.dockerignore` excludes `*.com.json` (yt-dlp cookie exports must never ride in the build context).
- [x] Verified: three local suites green; image built and container healthy on the production Ubuntu 24.04 VPS (`/api/health` 200 via loopback port).

---

## 2026-10-03 — Round 4: background download control + direct links

**Status:** complete (go test all green; tsc + vitest 60 pass; Chrome MCP smoke below)

- [x] **Download pause/resume (断点接续):** removed yt-dlp `--no-part` in favor of `--continue` (+`--retries 10 --fragment-retries 10`), so partial downloads persist as `.part` files. New `paused` job status; `POST /api/downloads/{id}/pause` kills the subprocess and keeps the part file, `/resume` continues from it (skips re-resolve when metadata is known). Transient download failures auto-retry up to 3× resuming the part file. A job active at server restart becomes `paused` on load (was: failed) and is resumable.
- [x] **URL dedupe + cache:** `POST /api/downloads` with an already-active URL returns the existing job (200, `dedupe:"active"`); a URL completed within `DOWNLOAD_CACHE_TTL` (new env, default `24h`) returns it with `dedupe:"completed"` + `driveFileId` — no re-download. Completed jobs are now kept by the reaper for the cache TTL (failed/cancelled stay at 1h); `Store.DeleteExpired(doneTTL, otherTTL)` replaces the flat TTL.
- [x] **Direct links (直链):** new `internal/share` store (`DATA_DIR/links.json`, idempotent one-link-per-file, revocable). `GET /d/{token}` is public — streams file bytes from Drive through the backend with Range/206 passthrough, md5 ETag, `inline` Content-Disposition (`?dl=1` forces download), works without any Google login. Management: `POST /api/files/{id}/link`, `GET /api/files/{id}/links`, `GET /api/links`, `DELETE /api/links/{token}` (also mirrored on `/api/v1` with read/readwrite scopes). ShareDialog now has a "Direct link" section (create/copy/remove).
- [x] **Agent API docs:** openapi.json previously documented only files/keys/meta — added the full uploads + downloads (incl. pause/resume + dedupe semantics) + links surface with a `DownloadJob` schema and scope annotations.
- [x] Frontend: pause/resume buttons on download cards (icon-paused state colors), store actions + `paused` status, `IconPlay` added.
- [x] Verified: new Go tests (dedupe find/pause-resume guards/expiry TTLs; share store CRUD/persistence; link create/stream/Range/revoke via fake Drive upstream) + full suites green; Chrome MCP smoke: direct-link create → fetch `/d/{token}` 200 (bytes + headers) and Range → 206; download job pause → `paused` status shown → resume → progress continues.

---

## 2026-10-03 — Round 3: no flash on reload, select-all checkbox, batch move

**Status:** complete (tsc + vitest 60 pass; Chrome MCP hands-on incl. real batch move + revert)

- [x] Reload flash fixed: `<html>` had no `data-wallpaper` until React's first effect, so every reload flashed the bare body gradient. An inline script in `index.html` now paints `data-wallpaper`/`data-theme` from localStorage before React mounts (hooks remain the source of truth).
- [x] Selection toolbar: the big round ✕ button read as "select all" but cleared the selection. Replaced with a square three-state select-all checkbox (checked / mixed / unchecked, `role="checkbox" aria-checked`); compact ✕ at the far right clears.
- [x] Batch move: Move now accepts the whole selection (toolbar "Move" button + multi-aware context menu showing "Move N items… / Trash N items / Download N items / ZIP N items" when right-clicking inside the selection; single-item actions hidden). `doMove` moves items one-by-one with per-item failure reporting.
- [x] Move dialog rewritten as a folder browser: breadcrumb + subfolder list loaded via `listFiles` (was a `<select>` that only knew root + breadcrumbs + sibling folders). Destination defaults to the current folder; moved items are excluded from the browse list so a folder can't be dropped into itself. Fixed trail bug: the main breadcrumb omits root, so the browser path was missing "My Drive" when opening from a subfolder.
- [x] Verified end-to-end with a real move of 2 folders root → Downloads → root (data restored).

---

## 2026-10-03 — Round 2: glass fix + toolbar selection bar

**Status:** complete (tsc + vitest 60 pass; Chrome MCP verified desktop + mobile)

- [x] Wallpaper glass restored: the `prefers-reduced-transparency` "authoritative" downgrade also forced `--blur: none` + opaque fills in wallpaper modes — panels rendered solid even though the wallpaper was now visible. The solid downgrade now applies only to `minimal`; wallpaper modes keep the glass the user picked.
- [x] Selection toolbar moved into the top toolbar (replaces Upload/tabs while items are selected, Google-Drive style): the file list no longer shifts vertically on desktop or mobile (mobile buttons keep their 36px touch min-height; container padding compensates).
- [x] Toolbar button polish: capsule shape, gradient primary (Upload), borderless text-chip secondaries, ghost-danger Trash; mobile selection toolbar scrolls horizontally when narrow, "Select all" stays on one line.

---

## 2026-10-03 — Wallpaper a11y fix + Google-Drive-style selection

**Status:** complete (go test + vitest 60 + tsc pass; Chrome MCP hands-on verification on desktop & mobile viewports)

### Done
- [x] Wallpaper fix: `prefers-reduced-transparency: reduce` was `display:none`-ing every `body::before/::after`, hiding the static photo/aurora/sakura wallpapers entirely (Windows "transparency effects off"). Static user-chosen wallpapers now survive the downgrade (`display: block` re-set in the wallpaper layer); the minimal animated field stays hidden.
- [x] Desktop selection model aligned with Google Drive: plain click selects a row (was a no-op), Ctrl/Cmd+click toggles, Shift+click selects the range from the click anchor, double-click still opens, right-click re-selects a row outside the current selection, click on empty space below rows and Escape clear the selection.
- [x] Selection bar: `position: absolute` overlay → `sticky` (no longer covers the first row, stays visible while scrolling); mobile-only "Select all" button (thead is hidden on phones); FAB hidden while selecting.
- [x] Touch behavior unchanged: tap opens, long-press enters selection, taps toggle while selecting.

### Verify
1. Windows with transparency effects off → photo wallpaper now visible in Settings → Appearance.
2. Desktop: click / Ctrl+click / Shift+click rows; right-click an unselected row; Esc; click empty area.
3. Mobile viewport (≤900px): long-press row → selection bar with Select all, FAB hidden, first row not covered.

### Checks
`go test ./... -count=1` green · `cd web && npm test` 60/60 green · `npx tsc --noEmit` clean

---

## 2026-08-25 — Download feature (yt-dlp → Google Drive)

**Status:** complete (go build pass; manual browser testing done)

### Feature

Full-stack download pipeline: paste a URL (YouTube, Bilibili, Douyin, direct links, etc.) → yt-dlp downloads → auto-upload to Google Drive. Integrated into the main SPA with consistent dark/glass theme.

### Backend

- `internal/download/` — complete yt-dlp service layer:
  - `yt_dlp.go`: metadata resolution, download with progress parsing, output template management, multi-strategy file lookup (exact match → glob → directory scan)
  - `service.go`: two-phase pipeline (download → upload), retry-upload (no re-download), folder auto-creation, `unknown_video` extension handling
  - `persist.go`: job persistence to `downloads.json`, background reaper (10m tick, 1h maxAge)
  - `store.go` / `job.go`: in-memory + persistent job store with mutex-protected state
  - `proxy.go`: per-domain proxy routing (domestic sites bypass proxy)
- `internal/api/download_handlers.go` — 7 REST handlers: create, list, status, cancel, retry-upload, delete, clear-finished
- `internal/api/router.go` — routes wired into both `/api/downloads` (session) and `/api/v1/downloads` (API key)
- `internal/config/config.go` — new env vars: `YTDLP_PATH`, `DOWNLOAD_PROXY`, `DOWNLOAD_COOKIE_PATH`, `DOWNLOAD_TMP_DIR`
- `cmd/cookieconvert/` — utility to convert Cookie Editor JSON → Netscape format for yt-dlp

### Frontend

- `web/src/components/DownloadPage.tsx` — full download UI: hero input zone, job history list, progress bars, retry/cancel/delete actions
- `web/src/lib/downloadStore.ts` — external store with optimistic UI (local delete + `deletedIds` set to prevent stale records on refresh)
- `web/src/App.tsx` — integrated `downloads` view into nav, toolbar, and page routing
- `web/src/components/Sidebar.tsx` / `MobileNav.tsx` — download nav item in sidebar and mobile bottom bar
- `web/src/features.css` — download page styles matching global dark/glass theme
- `web/src/lib/api.ts` — exported `parseError` for reuse by download store

### Bug fixes during integration

- **Grid collapse**: toolbar `null` caused `main-row` to collapse → render placeholder toolbar div
- **405 Method Not Allowed**: stale server binary → recompile and restart
- **File not found for direct links**: yt-dlp ignores `-o` template for direct links → added `scanDirForLargestMedia` fallback
- **`unknown_video` extension**: yt-dlp returns this for non-video direct links → output template uses inferred extension instead of `%(ext)s`; `uploadToDrive` infers from URL; `findDownloadedFile` includes `unknown_video` in known extensions
- **Upload failed (folder-books)**: virtual folder names not resolved → auto-create Drive folders by name
- **Retry re-downloads**: retry was starting full pipeline → split `RetryUpload` to only re-upload using existing local file
- **Records reappear after clear**: `refreshDownloads` overwrote local deletes → `deletedIds` Set filters stale records

### Verification

| Check | Result |
|-------|--------|
| `go build ./...` | pass |
| `go test ./...` | pass |
| Manual: YouTube download → Drive upload | pass |
| Manual: Bilibili (with cookies) | pass |
| Manual: direct link (non-video, `.unknown_video`) | pass |
| Manual: retry-upload after simulated failure | pass |
| Manual: delete/clear-finished | pass |

---

## 2026-07-27 → 08-12 — Bug audit fixes + frontend store split

**Status:** complete (unit/build green; browser E2E still manual) — ready to push

### Theme

Full-stack bug audit (`docs/archive/2026-07-27-BUG_AUDIT.md`) + architecture notes (`docs/archive/2026-07-27-ARCHITECTURE_RECOMMENDATIONS.md`), then ship the high-priority correctness/security fixes and start the App.tsx de-godification.

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
