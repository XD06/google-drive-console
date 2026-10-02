package api

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/drive"
	"github.com/dsk/drive-backup-console/internal/share"
)

// LinkHandlers serves direct links: revocable public URLs (/d/{token}) that
// stream file bytes from Google Drive through this backend, plus the
// session/api-key endpoints that create, list and revoke them.
type LinkHandlers struct {
	Auth  *auth.Service
	Drive DriveFactory
	Links *share.Store
}

type linkJSON struct {
	Token     string `json:"token"`
	URL       string `json:"url"`
	FileID    string `json:"fileId"`
	Name      string `json:"name,omitempty"`
	CreatedAt string `json:"createdAt"`
}

func (h *LinkHandlers) linkToJSON(r *http.Request, l share.Link) linkJSON {
	return linkJSON{
		Token:     l.Token,
		URL:       absoluteBaseURL(r) + "/d/" + l.Token,
		FileID:    l.FileID,
		Name:      l.Name,
		CreatedAt: l.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// Create handles POST /api/files/{id}/link — creates (or returns the existing)
// direct link for a file.
func (h *LinkHandlers) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "file id required")
		return
	}
	client, err := h.factory(r)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "drive_unavailable", err.Error())
		return
	}
	meta, err := client.GetMeta(r.Context(), id)
	if err != nil {
		writeDriveError(w, err)
		return
	}
	if meta.MimeType == drive.FolderMIME {
		writeJSONError(w, http.StatusBadRequest, "is_folder", "Cannot link a folder (zip it first)")
		return
	}
	link, err := h.Links.Create(id, meta.Name)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, h.linkToJSON(r, link))
}

// ListByFile handles GET /api/files/{id}/links.
func (h *LinkHandlers) ListByFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	id := r.PathValue("id")
	links := h.Links.ListByFile(id)
	out := make([]linkJSON, 0, len(links))
	for _, l := range links {
		out = append(out, h.linkToJSON(r, l))
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": out})
}

// ListAll handles GET /api/links.
func (h *LinkHandlers) ListAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	links := h.Links.List()
	out := make([]linkJSON, 0, len(links))
	for _, l := range links {
		out = append(out, h.linkToJSON(r, l))
	}
	writeJSON(w, http.StatusOK, map[string]any{"links": out})
}

// Revoke handles DELETE /api/links/{token}.
func (h *LinkHandlers) Revoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "DELETE only")
		return
	}
	token := r.PathValue("token")
	revoked, err := h.Links.Revoke(token)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"revoked": revoked})
}

// Stream handles GET /d/{token} — the public direct link. Streams the file
// from Drive with Range passthrough (video seeking, resumable fetches).
// No session required; the unguessable token is the credential.
func (h *LinkHandlers) Stream(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		writeJSONError(w, http.StatusNotFound, "not_found", "link not found")
		return
	}
	link, err := h.Links.Get(token)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "not_found", "link not found")
		return
	}
	client, err := h.factory(r)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "drive_unavailable", err.Error())
		return
	}
	meta, err := client.GetMeta(r.Context(), link.FileID)
	if err != nil {
		writeDriveError(w, err)
		return
	}

	_, body, contentType, contentRange, statusCode, err := client.DownloadRangeMedia(r.Context(), meta, r.Header.Get("Range"))
	if err != nil {
		writeDriveError(w, err)
		return
	}
	defer body.Close()

	filename := meta.Name
	if filename == "" {
		filename = link.Name
	}
	if filename == "" {
		filename = link.FileID
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", inlineDisposition(filename, r.URL.Query().Get("dl") == "1"))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, max-age=300")
	if meta.Md5Checksum != "" {
		w.Header().Set("ETag", `"`+meta.Md5Checksum+`"`)
	} else if meta.Version != "" {
		w.Header().Set("ETag", `"`+meta.Version+`"`)
	}
	if contentRange != "" {
		w.Header().Set("Content-Range", contentRange)
	}
	w.WriteHeader(statusCode) // 200 or 206
	if r.Method == http.MethodHead {
		return
	}
	buf := make([]byte, 256*1024)
	if _, err := io.CopyBuffer(w, body, buf); err != nil {
		log.Printf("direct link %s: stream for %q interrupted: %v", token, link.FileID, err)
	}
}

func (h *LinkHandlers) factory(r *http.Request) (*drive.Client, error) {
	if h.Drive != nil {
		return h.Drive(r, h.Auth)
	}
	return defaultDriveFactory(r, h.Auth)
}

// inlineDisposition builds a Content-Disposition header that previews inline
// (so browsers play video/images directly) unless forceDownload (?dl=1) is set.
func inlineDisposition(filename string, forceDownload bool) string {
	disposition := "inline"
	if forceDownload {
		disposition = "attachment"
	}
	escaped := strings.ReplaceAll(path.Base(filename), `"`, "")
	return disposition + `; filename="` + escaped + `"; filename*=UTF-8''` + url.PathEscape(filename)
}

// absoluteBaseURL rebuilds the externally visible origin from the request so
// link URLs stay valid behind a reverse proxy that sets X-Forwarded-Proto.
func absoluteBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	}
	return scheme + "://" + r.Host
}
