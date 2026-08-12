package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/dsk/drive-backup-console/internal/drive"
)

// Copy handles POST /api/files/{id}/copy
func (h *FilesHandlers) Copy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	var req struct {
		ParentID string `json:"parentId"`
		Name     string `json:"name"`
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
	item, err := client.Copy(r.Context(), id, req.ParentID, req.Name)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Share handles POST /api/files/{id}/share
func (h *FilesHandlers) Share(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	// Always attempt to decode; BodyLimit middleware already caps size.
	_ = json.NewDecoder(r.Body).Decode(&req)
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "reader"
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	info, err := client.ShareLink(r.Context(), id, role)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// Unshare handles DELETE /api/files/{id}/share
func (h *FilesHandlers) Unshare(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	permID := strings.TrimSpace(r.URL.Query().Get("permissionId"))
	if permID == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "permissionId required")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	if err := client.Unshare(r.Context(), id, permID); err != nil {
		writeDriveError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListPermissions handles GET /api/files/{id}/permissions
func (h *FilesHandlers) ListPermissions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	perms, err := client.ListPermissions(r.Context(), id)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"permissions": perms})
}

// Revisions handles GET /api/files/{id}/revisions
func (h *FilesHandlers) Revisions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	revs, err := client.ListRevisions(r.Context(), id)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"revisions": revs})
}

// RestoreRevision handles POST /api/files/{id}/revisions/{rev}/restore
func (h *FilesHandlers) RestoreRevision(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	revID := r.PathValue("rev")
	if id == "" || revID == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id and revision id required")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	if err := client.RestoreRevision(r.Context(), id, revID); err != nil {
		writeDriveError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// DownloadZip handles GET /api/files/{id}/download?zip=1
func (h *FilesHandlers) DownloadZip(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}

	meta, err := client.GetMeta(r.Context(), id)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	if meta.MimeType != drive.FolderMIME {
		writeJSONError(w, http.StatusBadRequest, "not_folder", "zip download is only available for folders")
		return
	}

	rootName := meta.Name
	if rootName == "" {
		rootName = "drive-archive"
	}
	safeName := url.PathEscape(rootName)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"; filename*=UTF-8''%s.zip`, rootName, safeName))

	if err := client.StreamZip(r.Context(), id, rootName, w); err != nil {
		log.Printf("zip stream failed for folder %s: %v", id, err)
		return
	}
}

// SimpleUpload handles POST /api/files/simple — multipart upload for small files (<5MB)
func (h *FilesHandlers) SimpleUpload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "name query param required")
		return
	}
	parentID := strings.TrimSpace(r.URL.Query().Get("parentId"))
	if parentID == "" {
		parentID = h.DefaultFolder
	}
	if parentID == "" {
		parentID = "root"
	}
	mimeType := strings.TrimSpace(r.URL.Query().Get("mimeType"))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Limit body to 5MB
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)

	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	item, err := client.SimpleUpload(r.Context(), name, mimeType, parentID, r.Body)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// Thumbnail handles GET /api/files/{id}/thumbnail — proxies Google's thumbnailLink.
// This avoids the browser 401 problem: Google's thumbnail URLs require OAuth,
// which <img> tags cannot provide. The server proxies with its credentials.
// Accepts optional ?link= query param with the known thumbnailLink from list
// data to skip the GetMeta round-trip (N+1 → 0 upstream calls for browsing).
//
// Security: link is allowlisted to Google thumbnail hosts only. client.HTTP is an
// oauth2 client that attaches the Drive access token to every request; fetching an
// attacker-controlled URL would exfiltrate that token and enable SSRF.
func (h *FilesHandlers) Thumbnail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}

	// Fast path: caller already knows the thumbnailLink from list response
	thumbURL := strings.TrimSpace(r.URL.Query().Get("link"))
	if thumbURL != "" && !isAllowedThumbnailURL(thumbURL) {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "link must be a Google thumbnail HTTPS URL")
		return
	}
	if thumbURL == "" {
		// Slow path: fetch metadata to get thumbnailLink
		meta, err := client.GetMeta(r.Context(), id)
		if err != nil {
			writeDriveError(w, err)
			return
		}
		thumbURL = meta.ThumbnailLink
	}

	if thumbURL == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	// Defense in depth: GetMeta-sourced links are validated too.
	if !isAllowedThumbnailURL(thumbURL) {
		writeJSONError(w, http.StatusBadGateway, "thumbnail_error", "refusing non-Google thumbnail URL")
		return
	}

	// Proxy the thumbnail from Google
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, thumbURL, nil)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "thumbnail_error", "failed to create request")
		return
	}
	res, err := h.thumbnailDoer(r.Context(), client.HTTP).Do(req)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "thumbnail_error", "failed to fetch thumbnail")
		return
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		writeJSONError(w, http.StatusBadGateway, "thumbnail_error", "Google returned non-200 for thumbnail")
		return
	}

	w.Header().Set("Content-Type", res.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, res.Body)
}

// thumbnailDoer returns an HTTP doer for proxying a Google thumbnail. It reuses
// the OAuth transport (the access token must be attached for Google to serve
// the image), but installs a CheckRedirect that re-validates every hop against
// the Google host allowlist. Without this, the transport re-attaches the Drive
// token on each redirect, so an off-Google redirect could exfiltrate it and
// enable SSRF. Falls back to the provided doer when the OAuth client is
// unavailable (e.g. tests with an injected Drive factory and no Auth service).
func (h *FilesHandlers) thumbnailDoer(ctx context.Context, fallback drive.HTTPDoer) drive.HTTPDoer {
	if h.Auth == nil {
		return fallback
	}
	hc, err := h.Auth.HTTPClient(ctx)
	if err != nil {
		return fallback
	}
	safe := *hc // copy so we never mutate the shared, cached OAuth client
	safe.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if !isAllowedThumbnailURL(req.URL.String()) {
			return fmt.Errorf("thumbnail redirect to disallowed host %q blocked", req.URL.Host)
		}
		return nil
	}
	return &safe
}

// isAllowedThumbnailURL reports whether raw is an https URL on a Google
// thumbnail host. Host matching is suffix-based on the hostname only (not the
// full authority), so userinfo / spoofed paths cannot bypass the check.
func isAllowedThumbnailURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || strings.Contains(host, "..") {
		return false
	}
	for _, suffix := range []string{
		"googleusercontent.com",
		"ggpht.com",
		"googleapis.com",
		"google.com",
	} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// Batch handles POST /api/files/batch — performs trash or move on multiple files.
// Body: {"action":"trash"|"move", "ids":["id1","id2"], "parentId":"..."}
// This replaces N frontend round-trips with a single request.
func (h *FilesHandlers) Batch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	var req struct {
		Action   string   `json:"action"`
		IDs      []string `json:"ids"`
		ParentID string   `json:"parentId"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if len(req.IDs) == 0 {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "ids required")
		return
	}
	if len(req.IDs) > 100 {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "max 100 items per batch")
		return
	}
	action := strings.TrimSpace(req.Action)
	if action != "trash" && action != "move" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "action must be 'trash' or 'move'")
		return
	}
	if action == "move" && strings.TrimSpace(req.ParentID) == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "parentId required for move")
		return
	}

	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}

	type itemErr struct {
		ID    string `json:"id"`
		Error string `json:"error"`
	}

	// Bounded concurrency: process up to 6 items in parallel
	const workers = 6
	type result struct {
		id  string
		err error
	}
	results := make(chan result, len(req.IDs))
	sem := make(chan struct{}, workers)

	for _, id := range req.IDs {
		sem <- struct{}{}
		go func(id string) {
			defer func() { <-sem }()
			var opErr error
			switch action {
			case "trash":
				opErr = client.Trash(r.Context(), id)
			case "move":
				_, opErr = client.Move(r.Context(), id, req.ParentID, "")
			}
			results <- result{id: id, err: opErr}
		}(id)
	}

	var errors []itemErr
	succeeded := 0
	for range req.IDs {
		r := <-results
		if r.err != nil {
			errors = append(errors, itemErr{ID: r.id, Error: r.err.Error()})
		} else {
			succeeded++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"succeeded": succeeded,
		"failed":    len(errors),
		"errors":    errors,
	})
}

// MultiZip handles POST /api/files/zip — streams multiple files as a single ZIP.
// Body: {"items":[{"id":"...","name":"..."}]}
func (h *FilesHandlers) MultiZip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	var req struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if len(req.Items) == 0 {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "items required")
		return
	}
	const maxMultiZipItems = 200
	if len(req.Items) > maxMultiZipItems {
		writeJSONError(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("max %d items per zip", maxMultiZipItems))
		return
	}

	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}

	rootName := "selected-files"
	safeName := url.PathEscape(rootName)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"; filename*=UTF-8''%s.zip`, rootName, safeName))

	// Build a simple file list and use StreamZip with a virtual root.
	// We create a temporary folder-like structure by downloading each file.
	items := make([]drive.ZipItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, drive.ZipItem{ID: it.ID, Name: it.Name})
	}

	if err := client.StreamZipMulti(r.Context(), items, w); err != nil {
		log.Printf("multi-zip stream failed: %v", err)
		return
	}
}
