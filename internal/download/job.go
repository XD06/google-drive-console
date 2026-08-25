package download

import (
	"sync"
	"time"
)

// Status of a download job.
type Status string

const (
	StatusPending    Status = "pending"     // created, not yet started
	StatusResolving  Status = "resolving"   // yt-dlp is extracting metadata
	StatusDownloading Status = "downloading" // yt-dlp is downloading the file
	StatusUploading  Status = "uploading"   // uploading to Google Drive
	StatusCompleted  Status = "completed"    // file uploaded to Drive
	StatusFailed     Status = "failed"       // error occurred
	StatusCancelled  Status = "cancelled"    // user cancelled
)

// Job is a download task tracked by the server.
type Job struct {
	mu sync.RWMutex // protects all mutable fields below

	ID         string    `json:"id"`
	URL        string    `json:"url"`
	ParentID   string    `json:"parentId,omitempty"` // Google Drive destination folder
	Status     Status    `json:"status"`
	Progress   float64   `json:"progress"`   // 0-100
	Speed      string    `json:"speed"`      // e.g. "1.23MiB/s"
	ETA        string    `json:"eta"`        // e.g. "00:30"
	Downloaded int64     `json:"downloaded"` // bytes downloaded
	Total      int64     `json:"total"`     // total bytes (0 if unknown)
	Title      string    `json:"title"`     // video/file title from yt-dlp
	Thumbnail  string    `json:"thumbnail,omitempty"`
	Extractor  string    `json:"extractor"`  // e.g. "youtube", "douyin", "generic"
	Uploader   string    `json:"uploader,omitempty"`
	Duration   int       `json:"duration,omitempty"` // seconds
	Ext        string    `json:"ext,omitempty"`      // file extension
	FileName   string    `json:"fileName,omitempty"` // local temp file path
	DriveFileID string   `json:"driveFileId,omitempty"` // Google Drive file ID after upload
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`

	// internal fields (not serialized)
	cancelCh chan struct{}
}

// JobSnapshot is a lock-free, JSON-safe copy of a Job's public fields.
// It is returned by View() and used everywhere we need to pass job data
// by value (JSON serialization, API responses, persistence).
type JobSnapshot struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	ParentID    string    `json:"parentId,omitempty"`
	Status      Status    `json:"status"`
	Progress    float64   `json:"progress"`
	Speed       string    `json:"speed"`
	ETA         string    `json:"eta"`
	Downloaded  int64     `json:"downloaded"`
	Total       int64     `json:"total"`
	Title       string    `json:"title"`
	Thumbnail   string    `json:"thumbnail,omitempty"`
	Extractor   string    `json:"extractor"`
	Uploader    string    `json:"uploader,omitempty"`
	Duration    int       `json:"duration,omitempty"`
	Ext         string    `json:"ext,omitempty"`
	FileName    string    `json:"fileName,omitempty"`
	DriveFileID string    `json:"driveFileId,omitempty"`
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// View returns a JSON-safe snapshot. Thread-safe.
func (j *Job) View() JobSnapshot {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return JobSnapshot{
		ID:          j.ID,
		URL:         j.URL,
		ParentID:    j.ParentID,
		Status:      j.Status,
		Progress:    j.Progress,
		Speed:       j.Speed,
		ETA:         j.ETA,
		Downloaded:  j.Downloaded,
		Total:       j.Total,
		Title:       j.Title,
		Thumbnail:   j.Thumbnail,
		Extractor:   j.Extractor,
		Uploader:    j.Uploader,
		Duration:    j.Duration,
		Ext:         j.Ext,
		FileName:    j.FileName,
		DriveFileID: j.DriveFileID,
		Error:       j.Error,
		CreatedAt:   j.CreatedAt,
		UpdatedAt:   j.UpdatedAt,
	}
}

// setStatus updates status and timestamp.
func (j *Job) setStatus(s Status) {
	j.mu.Lock()
	j.Status = s
	j.UpdatedAt = time.Now().UTC()
	j.mu.Unlock()
}

// setProgress updates download progress fields.
func (j *Job) setProgress(pct float64, speed, eta string, downloaded, total int64) {
	j.mu.Lock()
	j.Progress = pct
	j.Speed = speed
	j.ETA = eta
	j.Downloaded = downloaded
	if total > 0 {
		j.Total = total
	}
	j.UpdatedAt = time.Now().UTC()
	j.mu.Unlock()
}

// setError marks the job as failed with an error message.
func (j *Job) setError(err string) {
	j.mu.Lock()
	j.Status = StatusFailed
	j.Error = err
	j.UpdatedAt = time.Now().UTC()
	j.mu.Unlock()
}

// CancelChannel returns a channel that is closed when the job is cancelled.
func (j *Job) CancelChannel() <-chan struct{} {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.cancelCh
}
