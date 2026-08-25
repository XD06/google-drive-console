package download

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dsk/drive-backup-console/internal/drive"
)

// SimpleUploadThreshold: files smaller than this use SimpleUpload (one request).
// Larger files use resumable upload (chunked).
const SimpleUploadThreshold int64 = 5 << 20 // 5 MiB

// DriveUploader is the Drive surface used by the download service.
// *drive.Client satisfies this interface.
// FolderLister is optionally implemented to find or create a "Downloads" folder.
type DriveUploader interface {
	SimpleUpload(ctx context.Context, name, mimeType, parentID string, content io.Reader) (drive.FileItem, error)
	ResumableStart(ctx context.Context, name, mimeType, parentID string, size int64) (string, error)
	UploadRange(ctx context.Context, sessionURL string, start, end, total int64, data []byte) (drive.UploadChunkResult, error)
	UploadEmpty(ctx context.Context, sessionURL string) (drive.UploadChunkResult, error)
	CreateFolder(ctx context.Context, name, parentID string) (drive.FileItem, error)
	List(ctx context.Context, opt drive.ListOptions) (drive.ListResult, error)
}

// JobStore is the persistence surface (both *Store and *PersistentStore satisfy it).
type JobStore interface {
	Put(j *Job)
	Get(id string) (*Job, error)
	Update(id string, fn func(j *Job) error) (*Job, error)
	Delete(id string)
	List() []JobSnapshot
}

// Service orchestrates download jobs: yt-dlp downloads → upload to Google Drive.
type Service struct {
	Store    JobStore
	Drive    DriveUploader
	YtDlp    YtDlpConfig
	DefParent string // default Drive folder ID when parentId omitted
}

// CreateInput is POST /api/downloads body.
type CreateInput struct {
	URL      string `json:"url"`
	ParentID string `json:"parentId"`
}

// Create validates input, creates a job, and starts the download goroutine.
// The job is returned immediately with status "pending".
func (s *Service) Create(ctx context.Context, in CreateInput) (*Job, error) {
	if s == nil || s.Store == nil {
		return nil, errors.New("download service not configured")
	}
	rawURL := strings.TrimSpace(in.URL)
	if rawURL == "" {
		return nil, &ValidationError{Message: "url required"}
	}
	parent := strings.TrimSpace(in.ParentID)
	if parent == "" {
		parent = s.DefParent
	}
	if parent == "" {
		parent = "root"
	}

	now := time.Now().UTC()
	j := &Job{
		ID:        newID(),
		URL:       rawURL,
		ParentID:  parent,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
		cancelCh:  make(chan struct{}),
	}
	s.Store.Put(j)

	// Launch the download goroutine. It uses a detached context so it survives
	// the HTTP request — downloads run in the background even if the user
	// navigates away.
	go s.run(j.ID)

	return j, nil
}

// Get returns a job by id.
func (s *Service) Get(id string) (*Job, error) {
	if s == nil || s.Store == nil {
		return nil, errors.New("download service not configured")
	}
	return s.Store.Get(id)
}

// List returns all jobs.
func (s *Service) List() []JobSnapshot {
	if s == nil || s.Store == nil {
		return nil
	}
	return s.Store.List()
}

// Cancel marks a job as cancelled and signals the goroutine to stop.
func (s *Service) Cancel(id string) (*Job, error) {
	if s == nil || s.Store == nil {
		return nil, errors.New("download service not configured")
	}
	j, err := s.Store.Get(id)
	if err != nil {
		return nil, err
	}
	j.mu.Lock()
	if j.Status == StatusCompleted {
		j.mu.Unlock()
		return nil, &ValidationError{Message: "already completed"}
	}
	if j.Status == StatusCancelled || j.Status == StatusFailed {
		j.mu.Unlock()
		return j, nil
	}
	j.Status = StatusCancelled
	j.UpdatedAt = time.Now().UTC()
	close(j.cancelCh)
	j.mu.Unlock()

	s.Store.Put(j)
	return j, nil
}

// Delete removes a terminal job from the store and cleans up its local file.
// Non-terminal (active) jobs must be cancelled first.
func (s *Service) Delete(id string) error {
	if s == nil || s.Store == nil {
		return errors.New("download service not configured")
	}
	j, err := s.Store.Get(id)
	if err != nil {
		return err
	}
	j.mu.RLock()
	status := j.Status
	filePath := j.FileName
	j.mu.RUnlock()

	if !isTerminal(status) {
		return &ValidationError{Message: "cannot delete active job — cancel first"}
	}

	// Clean up local file if it still exists.
	if filePath != "" {
		os.Remove(filePath)
	}

	s.Store.Delete(id)
	return nil
}

// ClearFinished removes all terminal jobs (completed/failed/cancelled) and
// cleans up their local files. Returns the number of jobs removed.
func (s *Service) ClearFinished() int {
	if s == nil || s.Store == nil {
		return 0
	}
	jobs := s.Store.List()
	count := 0
	for _, snap := range jobs {
		if !isTerminal(snap.Status) {
			continue
		}
		if snap.FileName != "" {
			os.Remove(snap.FileName)
		}
		s.Store.Delete(snap.ID)
		count++
	}
	return count
}

// RetryUpload re-attempts the upload phase for a job whose download completed
// but upload failed. The local file must still exist (j.FileName). If the file
// is gone, returns an error so the caller can fall back to full re-download.
func (s *Service) RetryUpload(jobID string) (*Job, error) {
	if s == nil || s.Store == nil {
		return nil, errors.New("download service not configured")
	}
	j, err := s.Store.Get(jobID)
	if err != nil {
		return nil, err
	}
	j.mu.RLock()
	status := j.Status
	filePath := j.FileName
	j.mu.RUnlock()

	if status != StatusFailed {
		return nil, &ValidationError{Message: "job is not in failed state"}
	}
	if filePath == "" {
		return nil, &ValidationError{Message: "no local file to retry upload — re-download required"}
	}
	if !fileExists(filePath) {
		// Local file was cleaned up — can't retry upload.
		j.mu.Lock()
		j.FileName = ""
		j.mu.Unlock()
		return nil, &ValidationError{Message: "local file no longer exists — re-download required"}
	}

	go s.runUpload(jobID)
	return j, nil
}

// runUpload re-runs only the upload phase (Phase 3) using the existing file.
func (s *Service) runUpload(jobID string) {
	j, err := s.Store.Get(jobID)
	if err != nil {
		return
	}

	j.mu.RLock()
	filePath := j.FileName
	j.mu.RUnlock()

	if filePath == "" || !fileExists(filePath) {
		j.setError("local file no longer exists")
		s.Store.Put(j)
		return
	}

	// Re-parse minimal metadata from job fields for uploadToDrive.
	meta := metaInfo{
		Title: j.Title,
		Ext:   j.Ext,
	}

	j.setStatus(StatusUploading)
	s.Store.Put(j)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fileID, err := s.uploadToDrive(ctx, j, filePath, meta)
	if err != nil {
		j.setError("retry upload failed: " + err.Error())
		s.Store.Put(j)
		return
	}

	j.mu.Lock()
	j.DriveFileID = fileID
	j.Status = StatusCompleted
	j.Progress = 100
	j.Error = ""
	j.UpdatedAt = time.Now().UTC()
	j.mu.Unlock()
	s.Store.Put(j)

	// Clean up temp file on successful upload.
	os.Remove(filePath)
	log.Printf("retry upload job %s completed: %s → drive file %s", jobID, j.Title, fileID)
}

// run is the main goroutine: resolve → download → upload to Drive → cleanup.
func (s *Service) run(jobID string) {
	j, err := s.Store.Get(jobID)
	if err != nil {
		return
	}

	// Check cancellation before starting.
	if isCancelled(j) {
		return
	}

	// Phase 1: Resolve metadata.
	j.setStatus(StatusResolving)
	s.Store.Put(j)

	dlCtx, dlCancel := context.WithCancel(context.Background())
	defer dlCancel()

	// Wire job cancellation to context cancellation.
	go func() {
		select {
		case <-j.CancelChannel():
			dlCancel()
		case <-dlCtx.Done():
		}
	}()

	meta, err := ResolveMetadata(dlCtx, s.YtDlp, j.URL)
	if err != nil {
		j.setError(err.Error())
		s.Store.Put(j)
		return
	}

	// Save metadata to job.
	j.mu.Lock()
	j.Title = meta.Title
	j.Thumbnail = meta.Thumbnail
	j.Extractor = meta.Extractor
	j.Uploader = meta.Uploader
	if meta.Duration > 0 {
		j.Duration = int(meta.Duration)
	}
	j.Ext = meta.Ext
	if meta.Filesize > 0 {
		j.Total = meta.Filesize
	} else if meta.FilesizeApprox > 0 {
		j.Total = meta.FilesizeApprox
	}
	j.UpdatedAt = time.Now().UTC()
	j.mu.Unlock()
	s.Store.Put(j)

	if isCancelled(j) {
		return
	}

	// Phase 2: Download via yt-dlp.
	j.setStatus(StatusDownloading)
	s.Store.Put(j)

	result, err := Download(dlCtx, s.YtDlp, j.URL, func(pct float64, speed, eta string, downloaded, total int64) {
		j.setProgress(pct, speed, eta, downloaded, total)
		s.Store.Put(j)
	})
	if err != nil {
		if isCancelled(j) {
			j.setStatus(StatusCancelled)
		} else {
			j.setError(err.Error())
		}
		s.Store.Put(j)
		return
	}

	if isCancelled(j) {
		// Clean up downloaded file.
		os.Remove(result.FilePath)
		j.setStatus(StatusCancelled)
		s.Store.Put(j)
		return
	}

	// Save file path to job.
	j.mu.Lock()
	j.FileName = result.FilePath
	j.UpdatedAt = time.Now().UTC()
	j.mu.Unlock()
	s.Store.Put(j)

	// Phase 3: Upload to Google Drive.
	if s.Drive == nil {
		j.setError("drive uploader not configured")
		s.Store.Put(j)
		return
	}

	j.setStatus(StatusUploading)
	s.Store.Put(j)

	fileID, err := s.uploadToDrive(dlCtx, j, result.FilePath, meta)
	if err != nil {
		if isCancelled(j) {
			j.setStatus(StatusCancelled)
			s.Store.Put(j)
			os.Remove(result.FilePath)
			return
		}
		// Upload failed but file is still on disk — mark as failed but keep
		// the local file so RetryUpload can reuse it without re-downloading.
		j.setError("upload to drive failed: " + err.Error())
		s.Store.Put(j)
		return
	}

	// Success!
	j.mu.Lock()
	j.DriveFileID = fileID
	j.Status = StatusCompleted
	j.Progress = 100
	j.UpdatedAt = time.Now().UTC()
	j.mu.Unlock()
	s.Store.Put(j)

	// Clean up temp file.
	os.Remove(result.FilePath)
	log.Printf("download job %s completed: %s → drive file %s", jobID, j.Title, fileID)
}

// uploadToDrive uploads the downloaded file to Google Drive.
// Uses SimpleUpload for small files, resumable for large ones.
func (s *Service) uploadToDrive(ctx context.Context, j *Job, filePath string, meta metaInfo) (string, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}
	size := fi.Size()
	name := meta.Title
	if name == "" {
		name = filepath.Base(filePath)
	}
	// Ensure the filename has the right extension.
	ext := meta.Ext
	if ext == "" || ext == "unknown_video" {
		// yt-dlp returned unknown_video — try to infer from the original URL.
		ext = guessExt(j.URL)
		// If the file on disk has a real extension (not .unknown_video), prefer that.
		diskExt := strings.TrimPrefix(filepath.Ext(filePath), ".")
		if diskExt != "" && diskExt != "unknown_video" {
			ext = diskExt
		}
	}
	if ext != "" && !strings.HasSuffix(name, "."+ext) {
		name = name + "." + ext
	}
	mimeType := guessMimeType(ext)
	parent := j.ParentID
	if parent == "" {
		parent = "root"
	}

	// Resolve folder names (e.g. "Videos") to real Drive folder IDs.
	// If parent is already a Drive ID or "root", it's used as-is.
	resolvedParent, err := s.ensureDownloadsFolder(ctx, parent)
	if err == nil && resolvedParent != "" {
		parent = resolvedParent
	}

	// Small file: one-shot upload.
	if size < SimpleUploadThreshold {
		f, err := os.Open(filePath)
		if err != nil {
			return "", fmt.Errorf("open file: %w", err)
		}
		defer f.Close()
		item, err := s.Drive.SimpleUpload(ctx, name, mimeType, parent, f)
		if err != nil {
			return "", err
		}
		return item.ID, nil
	}

	// Large file: resumable upload in chunks.
	sessionURL, err := s.Drive.ResumableStart(ctx, name, mimeType, parent, size)
	if err != nil {
		return "", err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	const chunkSize = 16 * 1024 * 1024 // 16 MiB
	buf := make([]byte, chunkSize)
	var sent int64
	for sent < size {
		n, err := io.ReadFull(f, buf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return "", fmt.Errorf("read file: %w", err)
		}
		if n == 0 {
			break
		}
		chunk := buf[:n]
		start := sent
		end := sent + int64(n) - 1

		res, err := s.Drive.UploadRange(ctx, sessionURL, start, end, size, chunk)
		if err != nil {
			return "", err
		}
		sent = res.NextByte
		if sent <= start {
			sent = end + 1
		}

		// Update progress (upload phase).
		pct := float64(sent) / float64(size) * 100
		j.mu.Lock()
		j.Progress = pct
		j.Downloaded = sent
		j.UpdatedAt = time.Now().UTC()
		j.mu.Unlock()
		s.Store.Put(j)

		if res.Complete {
			return res.FileID, nil
		}
	}

	// All bytes sent but Drive didn't signal completion — query status.
	// This mirrors the logic in upload.Service.AppendChunk.
	if sent >= size {
		// Try one more empty upload to finalize.
		res, err := s.Drive.UploadRange(ctx, sessionURL, size-1, size-1, size, []byte{})
		if err == nil && res.Complete {
			return res.FileID, nil
		}
	}

	return "", fmt.Errorf("upload completed but Drive did not confirm")
}

// ensureDownloadsFolder finds or creates a "Downloads" folder under root,
// then returns its ID. If the parentId looks like a folder name (not a Drive ID),
// it tries to find/create it under the Downloads folder instead.
// If parentId is empty or "root", returns the Downloads folder ID.
func (s *Service) ensureDownloadsFolder(ctx context.Context, parentID string) (string, error) {
	if s.Drive == nil {
		return parentID, nil
	}

	// If parentID looks like a real Drive ID (long alphanumeric), use as-is.
	if isDriveFileID(parentID) {
		return parentID, nil
	}

	// Otherwise, find or create a "Downloads" folder under root.
	dlFolderID, err := s.findOrCreateFolder(ctx, "Downloads", "root")
	if err != nil {
		// Fallback: just use root.
		return "root", nil
	}

	// If parentID is a name like "Videos", "Music", etc., find/create it
	// under the Downloads folder.
	if parentID != "" && parentID != "root" {
		subID, err := s.findOrCreateFolder(ctx, parentID, dlFolderID)
		if err != nil {
			return dlFolderID, nil // fallback to Downloads folder itself
		}
		return subID, nil
	}

	return dlFolderID, nil
}

// findOrCreateFolder searches for a folder by name under parentID. If not
// found, creates it. Returns the folder ID.
func (s *Service) findOrCreateFolder(ctx context.Context, name, parentID string) (string, error) {
	if parentID == "" {
		parentID = "root"
	}

	// Search for existing folder with this name under parent.
	res, err := s.Drive.List(ctx, drive.ListOptions{FolderID: parentID, PageSize: 200})
	if err == nil {
		for _, item := range res.Items {
			if item.IsFolder && item.Name == name {
				return item.ID, nil
			}
		}
	}

	// Not found — create it.
	folder, err := s.Drive.CreateFolder(ctx, name, parentID)
	if err != nil {
		return "", err
	}
	return folder.ID, nil
}

// isDriveFileID returns true if s looks like a Google Drive file ID
// (typically 20-60 chars, alphanumeric with hyphens/underscores).
func isDriveFileID(s string) bool {
	if len(s) < 10 || len(s) > 100 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// fileExists checks if a file exists on disk.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ValidationError is a 400-class input problem.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func newID() string {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return fmt.Sprintf("dl_%d", time.Now().UnixNano())
	}
	return "dl_" + hex.EncodeToString(b[:])
}

func isCancelled(j *Job) bool {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.Status == StatusCancelled
}

// guessMimeType returns a MIME type for common file extensions.
func guessMimeType(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	switch ext {
	case "mp4":
		return "video/mp4"
	case "webm":
		return "video/webm"
	case "mkv":
		return "video/x-matroska"
	case "mp3":
		return "audio/mpeg"
	case "m4a":
		return "audio/mp4"
	case "opus":
		return "audio/opus"
	case "wav":
		return "audio/wav"
	case "ogg":
		return "audio/ogg"
	case "epub":
		return "application/epub+zip"
	case "pdf":
		return "application/pdf"
	case "zip":
		return "application/zip"
	case "webp":
		return "image/webp"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "txt":
		return "text/plain"
	case "srt":
		return "application/x-subrip"
	default:
		return "application/octet-stream"
	}
}
