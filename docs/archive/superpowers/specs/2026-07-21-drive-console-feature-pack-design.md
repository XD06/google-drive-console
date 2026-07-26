# Drive Backup Console — Feature Pack Design

**Date:** 2026-07-21  
**Status:** Ready for user review  
**Approach:** B — phased UI + minimum Drive API surface  
**Related:** `docs/spark/2026-07-21-drive-backup-console-design.md` (v1 browse/upload)

## 1. Goal

Extend the live self-hosted Drive console so operators can manage files beyond list/upload/download:

1. Create folders and files in the current folder  
2. Trash files/folders to Google Drive recycle bin  
3. Double-click folders without name text-selection flash  
4. Double-click text files to preview/edit in a modal and save back  
5. Icon row actions (download, delete) and context menus  
6. Overview: account storage usage + this console’s upload history  

Non-goals for this pack: permanent delete, full-tree analytics, collaborative editing, move/rename drag-drop, sharing, multi-select bulk ops.

## 2. Decisions (locked)

| Topic | Choice |
|-------|--------|
| Implementation path | Phased min-API (B) |
| Create UI | Toolbar primary + context menu |
| Create target | Always current open folder |
| New file flow | Open editor first → save picks name + type |
| Delete | Trash only (recoverable in Drive) |
| Text modal | Editable; explicit Save |
| Overview scope | Account Drive quota + local upload history (not recursive folder stats) |
| Light type chart | Yes — from currently loaded list only, labeled “Current folder listing” |

## 3. Phases

### Phase 1 — Create folder, trash, row UX

- `user-select: none` on name cells; double-click open folder without blue selection flash  
- Icon actions on row hover: download (files), trash (files + folders)  
- Toolbar + context menu: **New folder** (wired in Phase 1). **New file** control may be present but disabled or no-ops until Phase 2/3 editor path is ready.  
- Context menu on list blank / row: New folder, New file, Download, Move to trash  

- Confirm before trash (filename + “Move to trash”)  
- Toast on success/error  

### Phase 2 — Text content modal

- Double-click text-like files opens modal  
- Load content; edit; explicit Save; dirty close confirm  
- Size cap for edit (2MB): over cap → read-only or block with download hint  

### Phase 3 — New file end-to-end + Overview

- New file = empty draft modal → Save → name + extension → create  
- Overview nav: Files | Overview  
- Storage bar from Drive about  
- Upload history from session/localStorage counters already used by upload UI  

## 4. UX detail

### 4.1 Navigation

- Keep existing file browser as default  
- Add Overview as peer view (rail or top segment control)  

### 4.2 Create folder

- Modal: name field, Cancel / Create  
- Submit → API → refresh list → toast  
- Empty name disabled; invalid chars: surface Drive error  

### 4.3 Create file

- Opens text editor modal with empty body, title “Untitled”  
- Save opens secondary step: filename + type/extension (txt/md/json/…)  
- Creates under current `folderId`  
- After success: close or keep open on new id; refresh list  

### 4.4 Text open / edit

**Preview types (extension, case-insensitive):**  
`.txt`, `.md`, `.markdown`, `.json`, `.csv`, `.log`, `.yaml`, `.yml`, `.xml`, `.html`, `.css`, `.js`, `.ts`, `.env` (text), and files with `mimeType` starting `text/` or `application/json`.

Others: no modal; existing download behavior (or toast “Preview not supported”).

**Modal chrome:** filename title, close, Save (disabled when clean), monospace editor.

**Dirty close:** confirm discard.

### 4.5 Delete

- Trash only via Drive API  
- Confirm dialog with name  
- Folders: same trash (Drive handles folder trash)  
- No permanent delete in this pack  

### 4.6 Row interactions

- Single click: highlight selected row (visual only; no multi-select bulk ops in this pack)  
- Double-click folder: navigate in  
- Double-click text file: modal  
- Icons: 16–20px, hover-visible on desktop; always available via context menu  
- Name column: no text selection on double-click (`user-select: none` + preventDefault on selectstart if needed)  

### 4.7 Overview

Cards:

1. **Storage** — used / limit, progress bar; retry on failure  
2. **Uploads (this console)** — success/fail counts, recent entries from client-side history  
3. **Types** — distribution of items in the **currently loaded list** only; labeled as “Current folder listing”  

## 5. API contract

All routes require existing session cookie auth. JSON errors: `{ "error": "message" }`. `401` → frontend re-auth.

### 5.1 Create folder

`POST /api/files/mkdir`

```json
{ "name": "Backups", "parentId": "root-or-folder-id" }
```

Response `201`: file metadata object consistent with list items (`id`, `name`, `mimeType`, …).

### 5.2 Create file

`POST /api/files`

```json
{
  "name": "notes.md",
  "parentId": "…",
  "mimeType": "text/markdown",
  "content": "optional initial body"
}
```

Response `201`: metadata. If `content` omitted, empty file.

### 5.3 Trash

`DELETE /api/files/:id`

Response `204` or `{ "ok": true }`.

### 5.4 Get content

`GET /api/files/:id/content`

- `200` JSON only: `{ "content": "…", "mimeType": "…", "size": n, "name": "…" }`  
- `413`/`400` if not text or too large for preview

### 5.5 Put content

`PUT /api/files/:id/content`

```json
{ "content": "full file text" }
```

Response `200` metadata or `{ "ok": true }`. Reject binary / over size with `400`/`413`.

### 5.6 Overview

`GET /api/overview`

```json
{
  "storage": {
    "limit": 16106127360,
    "usage": 1234567890,
    "usageInDrive": 1234567890
  },
  "user": { "email": "…", "displayName": "…" }
}
```

Upload history stays client-side; not returned by this endpoint.

## 6. Backend design

- Extend `internal/drive` client: CreateFolder, CreateFile, Trash, GetContent, UpdateContent, About  
- Handlers in `internal/api` next to existing files/upload routes  
- Content size limit: 2 MiB for get/put text paths  
- MIME detection for create from extension map + override field  
- No new DB tables  

## 7. Frontend design

- `web/src/lib/api.ts` — new methods  
- Components (can live in `App.tsx` initially; split if file grows):  
  - `ContextMenu`  
  - `TextFileModal`  
  - `ConfirmDialog` / mkdir modal  
  - `OverviewPage`  
- Reuse toast, settings prefs, list refresh patterns  
- Icon set: inline SVG or existing icon style in demo CSS; download + trash minimum  

## 8. Error handling

| Case | Behavior |
|------|----------|
| Network / 5xx | Toast error; keep UI state |
| 409 name conflict | Toast Drive message |
| 403 | Toast permission |
| 401 | Clear session → login |
| Oversized text | Toast + suggest download |
| Trash cancel | No API call |

## 9. Testing

- **Go:** mock Drive service; table tests for mkdir, trash, content get/put, overview  
- **Frontend:** unit tests for extension→preview eligibility, api helpers, dirty-state logic  
- **Manual smoke:** mkdir → appear; trash → gone; open md → edit → save; Overview storage; double-click name no select  

## 10. Rollout order (implementation)

1. CSS user-select + icon buttons (download wired)  
2. Trash API + confirm + icon  
3. Mkdir API + toolbar + context menu  
4. Content GET/PUT + text modal  
5. New file flow  
6. Overview endpoint + page  
7. Docs: `api-contract.md`, `progress.md`  

## 11. Open risks

- Drive API rate limits on content rewrite  
- Large text pasting UX (cap at 2MB)  
- Context menu vs browser default (preventDefault carefully)  
- `App.tsx` size — split when Phase 2+ lands if unreadable  

## 12. Approval

Design sections §1–§4 approved in brainstorming (2026-07-21). This document is the written spec for review before implementation planning.
