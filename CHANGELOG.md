# Changelog

All notable changes to Drive Backup Console are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); entries are written for humans, grouped by category.

The project has no tagged releases yet — everything below ships on the `main` branch.

## [Unreleased]

### Added

- **Drive Backup Console (initial release, 2026-07-26)** — full-stack Google Drive file manager: folder browsing with breadcrumbs, Spotlight-style search modal (Ctrl/Cmd+K), type filters, multi-select batch trash/move, ZIP download of folders or selected items, media previews (images with pinch-zoom, video, PDF, in-place text editing), share links, revision history and restore, and a storage-overview dashboard.
- **Resumable chunked uploads (2026-07-26)** — pause/resume, drag-and-drop, progress tracking, and automatic resume from the server's byte offset when a chunk request fails.
- **Parallel downloads (2026-07-26)** — large files download over multiple streams using Range requests.
- **`/api/v1` programmatic API (2026-07-26)** — stable external contract for AI agents and scripts: `dbc_`-prefixed API keys with `read` / `readwrite` scopes, key management UI in Settings, OpenAPI 3 spec at `/api/v1/openapi.json`, and the `cmd/cookieconvert` utility (Cookie Editor JSON → Netscape format).
- **yt-dlp download feature (2026-08-25)** — paste a URL (YouTube, Bilibili, Douyin, direct links, and other supported sites) and the server downloads the media, then uploads it to Drive automatically. Includes a job list with progress/speed/ETA, cancel, retry-upload (re-uploads without re-downloading), delete and clear-finished, per-domain proxy routing (domestic sites bypass the proxy), and job persistence across restarts.
- **API key management in the settings panel (2026-07-26)** — create/revoke keys from the browser session.
- **Wallpaper themes (2026-07-26/27)** — Aurora / Sakura / Minimal backdrops behind the glass panels, plus a photo-wallpaper mode with a floating glass content card; mobile top chrome floats as a glass card in photo mode.
- **Lightbox improvements (2026-07-26)** — previous/next navigation between media and a real-time video speed badge.
- **Docker (2026-08-25)** — multi-stage build on an Alpine runtime with yt-dlp and ffmpeg pre-installed (~80 MB final image), so the download feature works out of the box; compose adds a `/api/health` health check.
- **Download pause/resume + dedupe cache (2026-10-03)** — downloads can be paused and resumed (new `paused` job state; yt-dlp now keeps `.part` files, so resume fetches only the missing tail). Transient download failures auto-retry up to 3×, and jobs that were running when the server restarted come back as *paused* (resumable) instead of failed. Re-submitting a URL that is already active returns the existing job, and a URL completed within `DOWNLOAD_CACHE_TTL` (new env var, default 24 h) returns its Drive file without re-downloading.
- **Direct file links (2026-10-03)** — `POST /api/files/{id}/link` mints a revocable, unguessable `…/d/{token}` URL that streams the file through the server, so it can be shared and fetched anywhere without a Google login. Supports HTTP Range (video seeking, resumable/partial fetches) and md5 ETags; `?dl=1` forces a download. Managed in the Share dialog (create/copy/remove) and mirrored on `/api/v1` with scope guards; links persist in `DATA_DIR/links.json`.
- **Agent API surface completed (2026-10-03)** — `openapi.json` now documents the previously missing uploads, downloads (incl. pause/resume and dedupe semantics) and direct-link endpoints with a `DownloadJob` schema; `docs/SKILL.md` gained matching URL-download and direct-link recipes.
- **Drive-style file selection & batch move (2026-10-03)** — Ctrl/Shift-click selection on desktop, long-press selection on mobile, a toolbar that transforms into a bulk-action bar with a three-state select-all checkbox, multi-item context-menu actions, and a move dialog rewritten as a folder browser (paginated subfolder navigation, batch move, self-drop prevention).
- **UI a11y & polish (2026-10-03)** — static wallpapers and glass transparency survive `prefers-reduced-transparency`/reduced-motion settings; no more bare-gradient flash on reload (wallpaper/theme painted by an inline head script); paused downloads stay visible on the main view.

### Security

- **Thumbnail SSRF fixed (2026-08-12)** — the thumbnail endpoint's `?link=` parameter is now restricted to an allowlist of Google thumbnail hosts (with redirect re-checks), closing an SSRF vector that could have leaked the server's Google OAuth token or reached internal hosts.

### Fixed

- Upload progress bar jumped to 100% immediately; it now reflects the real Drive upload phase with a monotonic high-water mark (2026-07-26, refined 2026-08-12).
- Video/PDF previews were slow because files were fully downloaded as blobs; they now stream (2026-07-26).
- Video speed badge races — badge never appearing, killed at mount by a stale reset effect, or anchored wrong (2026-07-26).
- Concurrent chunk uploads could double-flush to Drive (flush-generation guard added); upload jobs are now persisted to disk and terminal jobs reaped after 1 h (2026-08-12).
- A stale "Load more" response could merge another folder's files into the current listing; list state moved into an external store with generation guards (2026-08-12).
- ZIP walks are bounded (visited set, max depth, max file count) against deep or cyclic folder structures (2026-08-12).
- Direct-link downloads produced `.unknown_video` files; the server now infers the extension from the URL (2026-08-25).
- Retry after a failed upload restarted the whole pipeline; it now re-uploads only, using the already-downloaded temp file (2026-08-25).
- The service worker cached Vite dev modules, so frontend edits appeared to have no effect until the SW was manually unregistered; dev-only paths (`/src/`, `/@…`) now bypass the cache and the shell cache version was bumped (2026-10-03).

### Performance

- Content-aware Gzip: only compressible types ≥1 KiB are compressed; media, Range responses, and streaming paths bypass it entirely (2026-07-26).
- HTTP connection pooling (HTTP/2, keep-alive) for Google API calls; list `pageSize` passthrough; upload flush alignment (2026-07-26).
- React runtime split into its own long-cacheable chunk; modern-browser build target (2026-07-26).

### Docs

- `SKILL.md` agent skill so AI agents can operate the drive through `/api/v1` (2026-07-26); README, API contract, and Docker guide updated alongside each feature.
