package upload

import (
	"sync"
	"time"
)

// Status of an upload job.
type Status string

const (
	StatusPending   Status = "pending"
	StatusUploading Status = "uploading"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Job is a resumable upload tracked by the server.
type Job struct {
	mu            sync.RWMutex // protects all mutable fields below
	ID            string       `json:"uploadId"`
	Name          string       `json:"name"`
	MimeType      string       `json:"mimeType"`
	ParentID      string       `json:"parentId,omitempty"`
	Total         int64        `json:"total"`
	BytesSent     int64        `json:"bytesSent"`     // confirmed on Drive
	BytesReceived int64        `json:"bytesReceived"` // accepted from client
	Status        Status       `json:"status"`
	FileID        string       `json:"fileId,omitempty"`
	Error         string       `json:"error,omitempty"`
	SessionURL    string       `json:"-"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`

	// FlushSize is the adaptive flush threshold for this job (U2).
	// 0 means use drive.DefaultFlushSize.
	FlushSize int64 `json:"-"`

	// buffer holds unaligned remainder not yet sent to Drive.
	buffer   []byte
	flushing bool   // true while a flush loop is actively uploading to Drive
	flushGen uint64 // bumped each time a goroutine becomes the flusher; clear only if gen matches
}

// PublicView is the API JSON for job status (no session URL).
type PublicView struct {
	UploadID      string `json:"uploadId"`
	BytesSent     int64  `json:"bytesSent"`
	BytesReceived int64  `json:"bytesReceived"`
	Total         int64  `json:"total"`
	Status        Status `json:"status"`
	FileID        string `json:"fileId"`
	Name          string `json:"name,omitempty"`
	Error         string `json:"error,omitempty"`
}

// View returns a JSON-safe snapshot. Thread-safe.
func (j *Job) View() PublicView {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return PublicView{
		UploadID:      j.ID,
		BytesSent:     j.BytesSent,
		BytesReceived: j.BytesReceived,
		Total:         j.Total,
		Status:        j.Status,
		FileID:        j.FileID,
		Name:          j.Name,
		Error:         j.Error,
	}
}

// Snapshot returns a value copy of the job for safe access outside locks.
func (j *Job) Snapshot() Job {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return Job{
		ID:            j.ID,
		Name:          j.Name,
		MimeType:      j.MimeType,
		ParentID:      j.ParentID,
		Total:         j.Total,
		BytesSent:     j.BytesSent,
		BytesReceived: j.BytesReceived,
		Status:        j.Status,
		FileID:        j.FileID,
		Error:         j.Error,
		SessionURL:    j.SessionURL,
		CreatedAt:     j.CreatedAt,
		UpdatedAt:     j.UpdatedAt,
	}
}
