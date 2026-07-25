package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/drive"
	"github.com/dsk/drive-backup-console/internal/upload"
)

// UploadHandlers serves resumable upload endpoints.
type UploadHandlers struct {
	Auth    *auth.Service
	Uploads *upload.Service
	// Drive optional factory for lazy binding when Uploads.Drive is nil
	Drive DriveFactory
	// DefaultFolder when parentId omitted
	DefaultFolder string
}

type createUploadBody struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	ParentID string `json:"parentId"`
	MimeType string `json:"mimeType"`
}

// Create handles POST /api/uploads
func (h *UploadHandlers) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	svc, err := h.service(r)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "upload_unavailable", err.Error())
		return
	}
	var body createUploadBody
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	parent := strings.TrimSpace(body.ParentID)
	if parent == "" {
		parent = h.DefaultFolder
	}
	if parent == "" {
		parent = "root"
	}
	job, err := svc.Create(r.Context(), upload.CreateInput{
		Name:     body.Name,
		Size:     body.Size,
		ParentID: parent,
		MimeType: body.MimeType,
	})
	if err != nil {
		h.writeUploadErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"uploadId": job.ID,
		"status":   job.Status,
		"total":    job.Total,
		"name":     job.Name,
		"fileId":   nullIfEmpty(job.FileID),
	})
}

// Status handles GET /api/uploads/{id}
func (h *UploadHandlers) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	svc, err := h.service(r)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "upload_unavailable", err.Error())
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "missing upload id")
		return
	}
	job, err := svc.Get(id)
	if err != nil {
		if errors.Is(err, upload.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "not_found", "upload not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	v := job.View()
	writeJSON(w, http.StatusOK, map[string]any{
		"uploadId":      v.UploadID,
		"bytesSent":     v.BytesSent,
		"bytesReceived": v.BytesReceived,
		"total":         v.Total,
		"status":        v.Status,
		"fileId":        nullIfEmpty(v.FileID),
		"name":          v.Name,
		"error":         nullIfEmpty(v.Error),
	})
}

// Chunk handles PUT /api/uploads/{id}/chunk
func (h *UploadHandlers) Chunk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "PUT only")
		return
	}
	svc, err := h.service(r)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "upload_unavailable", err.Error())
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "missing upload id")
		return
	}
	// Optional Content-Range: bytes start-end/total
	offset := int64(-1)
	if cr := r.Header.Get("Content-Range"); cr != "" {
		start, ok := parseClientContentRangeStart(cr)
		if !ok {
			writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid Content-Range")
			return
		}
		offset = start
	} else if o := r.Header.Get("X-Upload-Offset"); o != "" {
		n, err := strconv.ParseInt(o, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid X-Upload-Offset")
			return
		}
		offset = n
	}

	data, err := io.ReadAll(io.LimitReader(r.Body, upload.MaxClientChunk+1))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "failed to read body")
		return
	}
	if len(data) == 0 {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "empty chunk")
		return
	}
	if len(data) > upload.MaxClientChunk {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "chunk_too_large", "chunk exceeds max size")
		return
	}

	job, err := svc.AppendChunk(r.Context(), id, offset, data)
	if err != nil {
		if job != nil && job.Status == upload.StatusFailed {
			// still return status body with error
			h.writeUploadErr(w, err)
			return
		}
		h.writeUploadErr(w, err)
		return
	}
	v := job.View()
	writeJSON(w, http.StatusOK, map[string]any{
		"uploadId":      v.UploadID,
		"bytesSent":     v.BytesSent,
		"bytesReceived": v.BytesReceived,
		"total":         v.Total,
		"status":        v.Status,
		"fileId":        nullIfEmpty(v.FileID),
		"error":         nullIfEmpty(v.Error),
	})
}

// Cancel handles POST /api/uploads/{id}/cancel
func (h *UploadHandlers) Cancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	svc, err := h.service(r)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "upload_unavailable", err.Error())
		return
	}
	id := r.PathValue("id")
	job, err := svc.Cancel(id)
	if err != nil {
		h.writeUploadErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"uploadId": job.ID,
		"status":   job.Status,
	})
}

func (h *UploadHandlers) service(r *http.Request) (*upload.Service, error) {
	if h.Uploads != nil && h.Uploads.Drive != nil {
		return h.Uploads, nil
	}
	// Build per-request service with OAuth HTTP client when only store is shared
	if h.Uploads == nil || h.Uploads.Store == nil {
		return nil, errors.New("upload store not configured")
	}
	factory := h.Drive
	if factory == nil {
		factory = defaultDriveFactory
	}
	dc, err := factory(r, h.Auth)
	if err != nil {
		return nil, err
	}
	return &upload.Service{Store: h.Uploads.Store, Drive: dc}, nil
}

func (h *UploadHandlers) writeUploadErr(w http.ResponseWriter, err error) {
	var ve *upload.ValidationError
	if errors.As(err, &ve) {
		writeJSONError(w, http.StatusBadRequest, "bad_request", ve.Message)
		return
	}
	if errors.Is(err, upload.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "not_found", "upload not found")
		return
	}
	var ae *drive.APIError
	if errors.As(err, &ae) {
		writeDriveError(w, err)
		return
	}
	writeJSONError(w, http.StatusBadGateway, "upload_error", err.Error())
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func parseClientContentRangeStart(cr string) (int64, bool) {
	// bytes start-end/total
	cr = strings.TrimSpace(cr)
	if !strings.HasPrefix(cr, "bytes ") {
		return 0, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(cr, "bytes "))
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return 0, false
	}
	se := strings.Split(parts[0], "-")
	if len(se) != 2 {
		return 0, false
	}
	start, err := strconv.ParseInt(se[0], 10, 64)
	if err != nil {
		return 0, false
	}
	return start, true
}