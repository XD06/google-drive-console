package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/drive"
)

// DriveFactory builds a Drive client for the current request (injectable for tests).
type DriveFactory func(r *http.Request, svc *auth.Service) (*drive.Client, error)

// FilesHandlers serves file list/download.
type FilesHandlers struct {
	Auth  *auth.Service
	Drive DriveFactory
	// DefaultFolder used when folderId query empty and ROOT_FOLDER_ID set; empty → "root".
	DefaultFolder string
}

func defaultDriveFactory(r *http.Request, svc *auth.Service) (*drive.Client, error) {
	if svc == nil {
		return nil, errors.New("auth not ready")
	}
	hc, err := svc.HTTPClient(r.Context())
	if err != nil {
		return nil, err
	}
	return drive.NewClient(hc), nil
}

func (h *FilesHandlers) factory() DriveFactory {
	if h.Drive != nil {
		return h.Drive
	}
	return defaultDriveFactory
}

// List handles GET /api/files?folderId=&pageToken=&pageSize=
func (h *FilesHandlers) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	folderID := strings.TrimSpace(r.URL.Query().Get("folderId"))
	if folderID == "" {
		folderID = h.DefaultFolder
	}
	// Optional pageSize (clamped in client.List). Larger pages cut the number
	// of Drive round-trips needed to fully load big folders.
	pageSize := 0
	if ps := strings.TrimSpace(r.URL.Query().Get("pageSize")); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil {
			pageSize = n
		}
	}
	res, err := client.List(r.Context(), drive.ListOptions{
		FolderID:  folderID,
		PageToken: r.URL.Query().Get("pageToken"),
		PageSize:  pageSize,
	})
	if err != nil {
		writeDriveError(w, err)
		return
	}
	// Ensure non-null items array
	if res.Items == nil {
		res.Items = []drive.FileItem{}
	}
	writeJSON(w, http.StatusOK, res)
}

// Search handles GET /api/files/search?q=&scope=folder|drive&folderId=&pageToken=&pageSize=
func (h *FilesHandlers) Search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}

	q := drive.NormalizeSearchQuery(r.URL.Query().Get("q"))
	scopeRaw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	if scopeRaw == "" {
		scopeRaw = string(drive.SearchScopeDrive)
	}
	scope := drive.SearchScope(scopeRaw)

	folderID := strings.TrimSpace(r.URL.Query().Get("folderId"))
	if scope == drive.SearchScopeFolder && folderID == "" {
		folderID = h.DefaultFolder
	}

	pageSize := 0
	if ps := strings.TrimSpace(r.URL.Query().Get("pageSize")); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil {
			pageSize = n
		}
	}

	res, err := client.Search(r.Context(), drive.SearchOptions{
		Query:     q,
		Scope:     scope,
		FolderID:  folderID,
		PageToken: r.URL.Query().Get("pageToken"),
		PageSize:  pageSize,
	})
	if err != nil {
		writeDriveError(w, err)
		return
	}
	if res.Items == nil {
		res.Items = []drive.FileItem{}
	}
	writeJSON(w, http.StatusOK, res)
}

// Download handles GET /api/files/{id}/download
func (h *FilesHandlers) Download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		// Fallback for patterns without PathValue
		id = extractFileID(r.URL.Path)
	}
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}

	// Optional caller-supplied metadata (from the already-loaded file list).
	// When `mime` is present we can skip the GetMeta round-trip to Drive
	// entirely, cutting download/preview latency from 2 RTT to 1 RTT.
	hint := downloadMetaHint(r, id)
	useHint := hint.MimeType != ""

	// etagOf derives the cache validator shared by the full and Range branches.
	etagOf := func(meta drive.FileMeta) string {
		if meta.Md5Checksum != "" {
			return `"` + meta.Md5Checksum + `"`
		}
		if meta.Version != "" {
			return `"` + meta.Version + `"`
		}
		return ""
	}

	// Check for Range header → use DownloadRange for partial content
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		var (
			meta         drive.FileMeta
			body         io.ReadCloser
			contentType  string
			contentRange string
			statusCode   int
			err          error
		)
		if useHint {
			meta, body, contentType, contentRange, statusCode, err = client.DownloadRangeMedia(r.Context(), hint, rangeHeader)
		} else {
			meta, body, contentType, contentRange, statusCode, err = client.DownloadRange(r.Context(), id, rangeHeader)
		}
		if err != nil {
			writeDriveError(w, err)
			return
		}
		defer body.Close()
		filename := meta.Name
		if filename == "" {
			filename = id
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", contentDisposition(filename))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Cache-Control", "private, max-age=300")
		if etag := etagOf(meta); etag != "" {
			w.Header().Set("ETag", etag)
		}
		if contentRange != "" {
			w.Header().Set("Content-Range", contentRange)
		}
		w.WriteHeader(statusCode) // 206 Partial Content or 200 OK
		buf := make([]byte, 256*1024)
		if _, err := io.CopyBuffer(w, body, buf); err != nil {
			// Headers (and often a Content-Length) are already sent, so we can no
			// longer signal failure to the client via status code. Log it so a
			// truncated (silently incomplete) download is at least observable.
			log.Printf("download: range stream for %q interrupted: %v", id, err)
		}
		return
	}

	// Standard full download.
	// When the client already sent md5/ver hints, honor If-None-Match before
	// opening the Drive media stream — otherwise every conditional revalidation
	// wastes a full upstream GET that is discarded after headers.
	if match := r.Header.Get("If-None-Match"); match != "" {
		if etag := etagOf(hint); etag != "" && match == etag {
			w.Header().Set("ETag", etag)
			w.Header().Set("Cache-Control", "private, max-age=300")
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	var (
		meta        drive.FileMeta
		body        io.ReadCloser
		contentType string
	)
	if useHint {
		meta, body, contentType, err = client.DownloadMedia(r.Context(), hint)
	} else {
		meta, body, contentType, err = client.Download(r.Context(), id)
	}
	if err != nil {
		writeDriveError(w, err)
		return
	}
	defer body.Close()

	filename := meta.Name
	if filename == "" {
		filename = id
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", contentDisposition(filename))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, max-age=300")

	// ETag support: use md5Checksum if available, fall back to version.
	if etag := etagOf(meta); etag != "" {
		w.Header().Set("ETag", etag)
		if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	if meta.Size != nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", *meta.Size))
	}
	w.WriteHeader(http.StatusOK)
	// Use 256 KiB buffer instead of the default 32 KiB for higher throughput.
	buf := make([]byte, 256*1024)
	if _, err := io.CopyBuffer(w, body, buf); err != nil {
		// 200 + Content-Length is already committed; a mid-stream failure
		// yields a truncated file the client may treat as complete. Log it
		// so the data-integrity issue is not entirely silent.
		log.Printf("download: stream for %q interrupted: %v", id, err)
	}
}

// downloadMetaHint builds a FileMeta from the download query parameters the
// frontend passes through from its already-loaded list state. All values are
// hints only — they affect response headers and guards, never the bytes
// streamed from Drive. Returns meta with MimeType empty when no usable hint
// was supplied (caller falls back to a GetMeta round-trip).
func downloadMetaHint(r *http.Request, id string) drive.FileMeta {
	q := r.URL.Query()
	meta := drive.FileMeta{
		ID:          id,
		Name:        strings.TrimSpace(q.Get("name")),
		MimeType:    strings.TrimSpace(q.Get("mime")),
		Md5Checksum: strings.TrimSpace(q.Get("md5")),
		Version:     strings.TrimSpace(q.Get("ver")),
	}
	if s := strings.TrimSpace(q.Get("size")); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n >= 0 {
			meta.Size = &n
		}
	}
	return meta
}

// GetContent handles GET /api/files/{id}/content
func (h *FilesHandlers) GetContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 && parts[0] == "api" && parts[1] == "files" {
			id = parts[2]
		}
	}
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	txt, err := client.GetTextContent(r.Context(), id)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	resp := map[string]interface{}{
		"content":  txt.Content,
		"mimeType": txt.MimeType,
		"size":     txt.Size,
		"name":     txt.Name,
	}
	writeJSON(w, http.StatusOK, resp)
}

// PutContent handles PUT /api/files/{id}/content
func (h *FilesHandlers) PutContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "PUT only")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 && parts[0] == "api" && parts[1] == "files" {
			id = parts[2]
		}
	}
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	if err := client.UpdateTextContent(r.Context(), id, req.Content); err != nil {
		writeDriveError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Create handles POST /api/files
func (h *FilesHandlers) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	var req struct {
		Name     string `json:"name"`
		ParentID string `json:"parentId"`
		MimeType string `json:"mimeType"`
		Content  string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "name required")
		return
	}
	parent := strings.TrimSpace(req.ParentID)
	if parent == "" {
		if h.DefaultFolder != "" {
			parent = h.DefaultFolder
		} else {
			parent = "root"
		}
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	item, err := client.CreateFile(r.Context(), name, parent, req.MimeType, req.Content)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Mkdir handles POST /api/files/mkdir
func (h *FilesHandlers) Mkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	var req struct {
		Name     string `json:"name"`
		ParentID string `json:"parentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "name required")
		return
	}
	parent := strings.TrimSpace(req.ParentID)
	if parent == "" {
		if h.DefaultFolder != "" {
			parent = h.DefaultFolder
		} else {
			parent = "root"
		}
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	item, err := client.CreateFolder(r.Context(), name, parent)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Trash handles DELETE /api/files/{id}
func (h *FilesHandlers) Trash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "DELETE only")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		// no download-like fallback needed for simple id path
		// but keep attempt to extract from URL
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// /api/files/{id}
		if len(parts) >= 3 && parts[0] == "api" && parts[1] == "files" {
			id = parts[2]
		}
	}
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	if err := client.Trash(r.Context(), id); err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Rename handles PATCH /api/files/{id} with { "name"?: string, "description"?: string }.
// At least one field must be present; an empty "description" clears it. (Named
// Rename for backward compatibility; it now updates name and/or description.)
func (h *FilesHandlers) Rename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "PATCH only")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 3 && parts[0] == "api" && parts[1] == "files" {
			id = parts[2]
		}
	}
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if req.Name == nil && req.Description == nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "name or description required")
		return
	}
	var namePtr *string
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" {
			writeJSONError(w, http.StatusBadRequest, "bad_request", "name required")
			return
		}
		namePtr = &n
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	item, err := client.UpdateMetadata(r.Context(), id, namePtr, req.Description)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// Move handles POST /api/files/{id}/move with { "parentId": "..." }.
func (h *FilesHandlers) Move(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// /api/files/{id}/move
		if len(parts) >= 4 && parts[0] == "api" && parts[1] == "files" && parts[3] == "move" {
			id = parts[2]
		}
	}
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	var req struct {
		ParentID     string `json:"parentId"`
		FromParentID string `json:"fromParentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	parentID := strings.TrimSpace(req.ParentID)
	if parentID == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "parentId required")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	item, err := client.Move(r.Context(), id, parentID, req.FromParentID)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func writeDriveError(w http.ResponseWriter, err error) {
	var ae *drive.APIError
	if errors.As(err, &ae) {
		status := ae.Status
		if status < 400 {
			status = http.StatusBadGateway
		}
		// Map Google 401 to our 401 for token issues
		if status == http.StatusUnauthorized {
			writeJSONError(w, http.StatusUnauthorized, ae.Code, ae.Message)
			return
		}
		if status >= 500 {
			writeJSONError(w, http.StatusBadGateway, ae.Code, ae.Message)
			return
		}
		writeJSONError(w, status, ae.Code, ae.Message)
		return
	}
	writeJSONError(w, http.StatusBadGateway, "drive_error", err.Error())
}

func contentDisposition(filename string) string {
	// RFC 5987 filename*
	escaped := url.PathEscape(filename)
	// simple fallback without quotes issues
	safe := strings.ReplaceAll(path.Base(filename), `"`, "")
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, safe, escaped)
}

func extractFileID(p string) string {
	// /api/files/{id}/download
	parts := strings.Split(strings.Trim(p, "/"), "/")
	// api files id download
	if len(parts) >= 4 && parts[0] == "api" && parts[1] == "files" && parts[3] == "download" {
		return parts[2]
	}
	return ""
}
