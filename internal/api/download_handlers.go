package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/download"
	"github.com/dsk/drive-backup-console/internal/drive"
)

// DownloadHandlers serves download-related endpoints.
type DownloadHandlers struct {
	Auth    *auth.Service
	Store   *download.PersistentStore
	YtDlp   download.YtDlpConfig
	Drive   DriveFactory
	DefaultFolder string
}

type createDownloadBody struct {
	URL      string `json:"url"`
	ParentID string `json:"parentId"`
}

// Create handles POST /api/downloads
func (h *DownloadHandlers) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	svc, err := h.service(r)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "download_unavailable", err.Error())
		return
	}
	var body createDownloadBody
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	job, err := svc.Create(r.Context(), download.CreateInput{
		URL:      body.URL,
		ParentID: body.ParentID,
	})
	if err != nil {
		h.writeErr(w, err)
		return
	}
	v := job.View()
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     v.ID,
		"url":    v.URL,
		"status": v.Status,
		"title":  nullIfEmpty(v.Title),
	})
}

// Status handles GET /api/downloads/{id}
func (h *DownloadHandlers) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	svc, err := h.service(r)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "download_unavailable", err.Error())
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "missing download id")
		return
	}
	j, err := svc.Get(id)
	if err != nil {
		if errors.Is(err, download.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "not_found", "download not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, jobToJSON(j.View()))
}

// List handles GET /api/downloads
func (h *DownloadHandlers) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	svc, err := h.service(r)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "download_unavailable", err.Error())
		return
	}
	jobs := svc.List()
	result := make([]map[string]any, 0, len(jobs))
	for _, j := range jobs {
		result = append(result, jobToJSON(j))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"jobs": result,
	})
}

// Cancel handles POST /api/downloads/{id}/cancel
func (h *DownloadHandlers) Cancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	svc, err := h.service(r)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "download_unavailable", err.Error())
		return
	}
	id := r.PathValue("id")
	job, err := svc.Cancel(id)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":     job.ID,
		"status": job.Status,
	})
}

// RetryUpload handles POST /api/downloads/{id}/retry-upload
func (h *DownloadHandlers) RetryUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	svc, err := h.service(r)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "download_unavailable", err.Error())
		return
	}
	id := r.PathValue("id")
	job, err := svc.RetryUpload(id)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":     job.ID,
		"status": "uploading",
	})
}

// Delete handles DELETE /api/downloads/{id}
func (h *DownloadHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "DELETE only")
		return
	}
	svc, err := h.service(r)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "download_unavailable", err.Error())
		return
	}
	id := r.PathValue("id")
	if err := svc.Delete(id); err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// ClearFinished handles DELETE /api/downloads
func (h *DownloadHandlers) ClearFinished(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "DELETE only")
		return
	}
	svc, err := h.service(r)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "download_unavailable", err.Error())
		return
	}
	n := svc.ClearFinished()
	writeJSON(w, http.StatusOK, map[string]any{"cleared": n})
}

// service builds a per-request download.Service with a Drive client backed by
// the user's OAuth session.
func (h *DownloadHandlers) service(r *http.Request) (*download.Service, error) {
	if h == nil || h.Auth == nil {
		return nil, errors.New("auth not configured")
	}
	factory := h.Drive
	if factory == nil {
		factory = defaultDriveFactory
	}
	dc, err := factory(r, h.Auth)
	if err != nil {
		return nil, err
	}
	parent := h.DefaultFolder
	if parent == "" {
		parent = "root"
	}
	return &download.Service{
		Store:     h.Store,
		Drive:     dc,
		YtDlp:     h.YtDlp,
		DefParent: parent,
	}, nil
}

func (h *DownloadHandlers) writeErr(w http.ResponseWriter, err error) {
	var ve *download.ValidationError
	if errors.As(err, &ve) {
		writeJSONError(w, http.StatusBadRequest, "bad_request", ve.Message)
		return
	}
	if errors.Is(err, download.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "not_found", "download not found")
		return
	}
	var ae *drive.APIError
	if errors.As(err, &ae) {
		writeDriveError(w, err)
		return
	}
	writeJSONError(w, http.StatusBadGateway, "download_error", err.Error())
}

// jobToJSON converts a Job snapshot to a JSON-friendly map.
func jobToJSON(j download.JobSnapshot) map[string]any {
	return map[string]any{
		"id":          j.ID,
		"url":         j.URL,
		"parentId":    nullIfEmpty(j.ParentID),
		"status":      j.Status,
		"progress":    j.Progress,
		"speed":       nullIfEmpty(j.Speed),
		"eta":         nullIfEmpty(j.ETA),
		"downloaded":  j.Downloaded,
		"total":       j.Total,
		"title":       nullIfEmpty(j.Title),
		"thumbnail":   nullIfEmpty(j.Thumbnail),
		"extractor":   nullIfEmpty(j.Extractor),
		"uploader":    nullIfEmpty(j.Uploader),
		"duration":    j.Duration,
		"ext":         nullIfEmpty(j.Ext),
		"fileName":    nullIfEmpty(j.FileName),
		"driveFileId": nullIfEmpty(j.DriveFileID),
		"error":       nullIfEmpty(j.Error),
		"createdAt":   j.CreatedAt,
		"updatedAt":   j.UpdatedAt,
	}
}
