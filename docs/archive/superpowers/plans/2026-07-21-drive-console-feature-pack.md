# Drive Console Feature Pack — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use executing-plans (or implement task-by-task). Steps use checkbox (`- [ ]`) syntax.  
> **Spec:** `docs/superpowers/specs/2026-07-21-drive-console-feature-pack-design.md`

**Goal:** Ship create folder/file, trash, text modal edit, icon actions, and Overview on the live Go + React console.

**Architecture:** Extend `internal/drive.Client` + `FilesHandlers` / new handlers; SPA `api.ts` + `App.tsx` (split components if App exceeds ~1k lines). Phased delivery: UX+mkdir+trash → content modal → new file + Overview.

**Tech stack:** Go 1.22+ `http.ServeMux`, existing Drive HTTP client, Vite/React/TS, vitest, existing toast/settings patterns.

**Error envelope (existing):** `{ "error": { "code", "message" } }` — keep for all new routes.

---

## File map

| Area | Create / modify |
|------|-----------------|
| Drive client | `internal/drive/client.go`, new tests `client_mutate_test.go` |
| Types | `internal/drive/types.go` if needed (or keep in client.go) |
| API | `internal/api/files_handlers.go`, `router.go`, `files_handlers_test.go` |
| Overview | `internal/api/overview_handlers.go` (+ test) |
| Contract | `docs/api-contract.md` |
| Frontend client | `web/src/lib/api.ts`, `api.test.ts` |
| UI | `web/src/App.tsx`, `web/src/demo.css` |
| Optional split | `web/src/components/TextFileModal.tsx`, `ContextMenu.tsx`, `Overview.tsx` |
| Progress | `docs/progress.md` |

---

## Phase 1 — Row UX, trash, mkdir

### Task 1: Drive client — CreateFolder + Trash

**Files:**
- Modify: `internal/drive/client.go`
- Create: `internal/drive/mutate_test.go` (or extend existing drive tests)

- [ ] **Step 1: Add `CreateFolder`**

```go
// CreateFolder creates a folder under parentID (default "root").
func (c *Client) CreateFolder(ctx context.Context, name, parentID string) (FileItem, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return FileItem{}, &APIError{Status: 400, Code: "bad_request", Message: "name required"}
	}
	if strings.TrimSpace(parentID) == "" {
		parentID = "root"
	}
	body := map[string]any{
		"name":     name,
		"mimeType": FolderMIME,
		"parents":  []string{parentID},
	}
	// POST /files?supportsAllDrives=true&fields=id,name,mimeType,size,modifiedTime
	// return FileItem with IsFolder true
}
```

- [ ] **Step 2: Add `Trash`**

```go
// Trash moves a file/folder to the Drive trash (PATCH files/{id} {trashed:true}).
func (c *Client) Trash(ctx context.Context, fileID string) error {
	// PATCH with {"trashed": true}, supportsAllDrives=true
}
```

- [ ] **Step 3: Unit test with httptest mock**

- CreateFolder: assert POST body mime + parents; 200 maps to FileItem  
- Trash: assert PATCH path + trashed true  
- Empty name → 400 APIError  

- [ ] **Step 4: Run tests**

```powershell
go test ./internal/drive/ -count=1
```

Expected: PASS

---

### Task 2: API — mkdir + trash handlers

**Files:**
- Modify: `internal/api/files_handlers.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/files_handlers_test.go`

- [ ] **Step 1: `Mkdir` handler** `POST /api/files/mkdir`

Request JSON: `{ "name", "parentId" }`  
`parentId` empty → `DefaultFolder` or `"root"`.  
201 → `drive.FileItem` JSON.

- [ ] **Step 2: `Trash` handler** `DELETE /api/files/{id}`

204 empty body (or `{ "ok": true }` — pick **204** and document).  
Map drive errors via existing `writeDriveError`.

- [ ] **Step 3: Wire router**

```go
mux.Handle("POST /api/files/mkdir", RequireSession(d.Auth, http.HandlerFunc(fh.Mkdir)))
mux.Handle("DELETE /api/files/{id}", RequireSession(d.Auth, http.HandlerFunc(fh.Trash)))
```

Note: `GET /api/files/{id}/download` already exists — keep pattern consistent with PathValue.

- [ ] **Step 4: Handler tests** (mock Drive factory like List tests)

- Unauthorized without cookie  
- Mkdir OK  
- Trash OK → 204  

- [ ] **Step 5: Run**

```powershell
go test ./internal/api/ -count=1
```

Expected: PASS

---

### Task 3: Frontend api + Phase 1 UI

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/api.test.ts`
- Modify: `web/src/App.tsx`
- Modify: `web/src/demo.css`

- [ ] **Step 1: Client methods**

```ts
export async function createFolder(name: string, parentId?: string): Promise<FileItem>
export async function trashFile(id: string): Promise<void>
```

- [ ] **Step 2: Vitest** fetch mocks for createFolder / trashFile

- [ ] **Step 3: CSS**

```css
.name-cell, .name-cell span {
  user-select: none;
  -webkit-user-select: none;
}
.file-table tbody tr {
  user-select: none;
}
.row-actions .btn-icon {
  /* icon-only 32px hit target */
}
```

Optional: `onMouseDown` preventDefault on double-click path if selection still flashes.

- [ ] **Step 4: Toolbar**

- Enable **New Folder** → small modal (name input) → `createFolder(name, folderId)` → `loadFiles` + toast  
- Keep **New File** disabled with title “Phase 2/3” OR hide until Task 6  

- [ ] **Step 5: Icon row actions**

Replace text Open/Download with icon buttons:

- Folder: open (folder icon) + trash  
- File: download (anchor with icon) + trash  

Trash → confirm dialog (`window.confirm` OK for v1, or lightweight modal matching settings sheet) → `trashFile` → refresh + toast.

- [ ] **Step 6: Context menu (minimal)**

Right-click on table body / row:

- New folder  
- Download (files)  
- Move to trash  

`preventDefault` on contextmenu; position fixed menu; click-away closes.

- [ ] **Step 7: Verify**

```powershell
cd web; npx tsc --noEmit; npm test
```

Manual: double-click folder name → no blue select; mkdir appears; trash removes from list.

---

## Phase 2 — Text content modal

### Task 4: Drive + API content get/put

**Files:**
- Modify: `internal/drive/client.go`
- Modify: `internal/api/files_handlers.go`, `router.go`, tests

- [ ] **Step 1: Client**

```go
const MaxTextContent = 2 << 20 // 2 MiB

func (c *Client) GetTextContent(ctx context.Context, fileID string) (name, mime string, content string, size int64, err error)
func (c *Client) UpdateTextContent(ctx context.Context, fileID, content string) error
```

GetTextContent: GetMeta first; reject folder / google-apps native / size > MaxTextContent / non-text MIME (allow `text/*`, `application/json`, `application/xml`, empty+known extensions handled at handler). Download alt=media, read LimitReader MaxTextContent+1.

UpdateTextContent: media upload PATCH or multipart — simplest:  
`PATCH https://www.googleapis.com/upload/drive/v3/files/{id}?uploadType=media` with body bytes and Content-Type text/plain (Drive stores content; metadata mime may stay).

- [ ] **Step 2: Handlers**

`GET /api/files/{id}/content` → JSON:

```json
{ "content": "...", "mimeType": "...", "size": 123, "name": "a.md" }
```

`PUT /api/files/{id}/content` body `{ "content": "..." }` → 200 `{ "ok": true }` or metadata.

- [ ] **Step 3: Router + tests**

- [ ] **Step 4:** `go test ./internal/drive/ ./internal/api/ -count=1`

---

### Task 5: Text modal UI

**Files:**
- Modify: `web/src/lib/api.ts` — `fetchFileContent`, `saveFileContent`
- Modify: `web/src/App.tsx` (+ optional `TextFileModal.tsx`)
- Modify: `web/src/demo.css`

- [ ] **Step 1: Helper `isTextPreviewable(item: FileItem): boolean`**

Extensions from spec; mime `text/*` or `application/json`.

- [ ] **Step 2: Double-click file**

If previewable → open modal + load content; else download or toast.

- [ ] **Step 3: Modal**

- Title = name  
- textarea monospace, value controlled  
- Save enabled when dirty  
- Close with dirty confirm  
- Loading / error states  

- [ ] **Step 4: Vitest** for `isTextPreviewable`

- [ ] **Step 5:** `npx tsc --noEmit; npm test` + manual open md → edit → save → reopen

---

## Phase 3 — New file + Overview

### Task 6: Create file API + new-file flow

**Files:**
- Drive: `CreateFile(ctx, name, parentID, mimeType, content string) (FileItem, error)`  
  multipart or two-step: metadata POST then media (for empty content, metadata-only POST with mime).

- API: `POST /api/files` JSON `{ name, parentId, mimeType, content? }` → 201 FileItem

- Frontend: toolbar **New File** opens editor draft (`id: null`); Save → name+extension dialog → `createFile` → refresh; optional stay open with new id.

- [ ] Implement client + handler + tests  
- [ ] Wire UI end-to-end  
- [ ] Extension → mime map: `.md`→text/markdown, `.json`→application/json, `.txt`→text/plain, default text/plain  

---

### Task 7: Overview

**Files:**
- Drive: `About(ctx) (StorageQuota, error)` → `GET /about?fields=storageQuota,user`  
- API: `GET /api/overview` → storage + user email/displayName  
- Frontend: view switch Files | Overview in rail or topbar segment  
- Cards: storage bar; upload history from existing `jobs` + optional `localStorage` key `dbc_upload_history`; type bars from current `items`  

- [x] Backend + test  
- [x] Frontend page + CSS  
- [x] tsc + vitest  

---

### Task 8: Docs + smoke

- [x] Update `docs/api-contract.md` with all new routes (✅)  
- [x] Update `docs/progress.md` with phase notes  
- [x] Manual smoke checklist:

  1. Login localhost:5174  
  2. New folder → visible  
  3. New file → save as `note.md` → open → edit → save  
  4. Trash file → gone  
  5. Double-click name no selection  
  6. Overview storage numbers  

```powershell
go test ./... -count=1
cd web; npm test; npx tsc --noEmit
```

---

## Dependency graph

```
Task1 (drive mutate) → Task2 (API mkdir/trash) → Task3 (UI Phase1)
Task1/2 patterns → Task4 (content API) → Task5 (modal)
Task4/5 → Task6 (create file)
Task1 patterns → Task7 (overview)
All → Task8 (docs/smoke)
```

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-21-drive-console-feature-pack.md`.

**Two execution options:**

1. **Subagent-driven (this session)** — dispatch agent per task, review between tasks  
2. **Inline** — implement Task 1→… in this chat, small chunks  

Which do you prefer?
