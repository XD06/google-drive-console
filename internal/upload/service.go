package upload

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dsk/drive-backup-console/internal/drive"
)

// MaxClientChunk is the max body size accepted per PUT from the browser.
const MaxClientChunk = 32 << 20 // 32 MiB (supports U5: 5-32MB files as single chunk)

// DriveUploader is the Drive surface used by upload jobs.
type DriveUploader interface {
	ResumableStart(ctx context.Context, name, mimeType, parentID string, size int64) (sessionURL string, err error)
	UploadRange(ctx context.Context, sessionURL string, start, end, total int64, data []byte) (drive.UploadChunkResult, error)
	UploadEmpty(ctx context.Context, sessionURL string) (drive.UploadChunkResult, error)
	QueryUploadStatus(ctx context.Context, sessionURL string, total int64) (drive.UploadChunkResult, error)
}

// JobStore is the persistence surface used by Service.
// *Store and *PersistentStore both satisfy it.
type JobStore interface {
	Put(j *Job)
	Get(id string) (*Job, error)
	Update(id string, fn func(j *Job) error) (*Job, error)
	Delete(id string)
}

// Service orchestrates job store + Drive resumable upload.
type Service struct {
	Store JobStore
	Drive DriveUploader
}

// CreateInput is POST /api/uploads body.
type CreateInput struct {
	Name     string
	Size     int64
	ParentID string
	MimeType string
}

// Create starts a Drive session and registers a job.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Job, error) {
	if s == nil || s.Store == nil || s.Drive == nil {
		return nil, errors.New("upload service not configured")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, &ValidationError{Message: "name required"}
	}
	if in.Size < 0 {
		return nil, &ValidationError{Message: "size must be >= 0"}
	}
	mime := in.MimeType
	if mime == "" {
		mime = "application/octet-stream"
	}
	parent := strings.TrimSpace(in.ParentID)
	if parent == "" {
		parent = "root"
	}

	sessionURL, err := s.Drive.ResumableStart(ctx, name, mime, parent, in.Size)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	j := &Job{
		ID:            newID(),
		Name:          name,
		MimeType:      mime,
		ParentID:      parent,
		Total:         in.Size,
		BytesSent:     0,
		BytesReceived: 0,
		Status:        StatusPending,
		SessionURL:    sessionURL,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if in.Size == 0 {
		res, err := s.Drive.UploadEmpty(ctx, sessionURL)
		if err != nil {
			j.Status = StatusFailed
			j.Error = err.Error()
			s.Store.Put(j)
			return j, err
		}
		j.Status = StatusCompleted
		j.FileID = res.FileID
	}

	// Calculate adaptive flush size based on total file size (U2).
	// Larger files use larger flush sizes to reduce HTTP round-trips to Drive.
	j.FlushSize = adaptiveFlushSize(in.Size)

	s.Store.Put(j)
	return j, nil
}

// adaptiveFlushSize returns the optimal Drive flush size for a given file size.
// < 200MB: 16 MiB (default), >200MB: 32 MiB.
// 32 MiB is the ceiling on purpose: it matches MaxClientChunk, so every
// client PUT fills exactly one flush and streams to Drive immediately.
// A larger flush (e.g. 64 MiB) would buffer the first client chunk without
// sending anything — a pipeline bubble — and double peak memory per job.
// All values are multiples of ChunkMultiple (256 KiB).
func adaptiveFlushSize(total int64) int64 {
	const (
		miB        = 1 << 20
		threshold1 = 200 * miB
	)
	if total > threshold1 {
		return 32 * miB
	}
	return drive.DefaultFlushSize
}

// Get returns job by id.
func (s *Service) Get(id string) (*Job, error) {
	if s == nil || s.Store == nil {
		return nil, errors.New("upload service not configured")
	}
	return s.Store.Get(id)
}

// AppendChunk accepts the next sequential bytes from the client and flushes aligned blocks to Drive.
// offset < 0 means "append at BytesReceived" (no Content-Range from client).
//
// Network I/O (UploadRange) is performed OUTSIDE any lock so that concurrent
// jobs, status polls, and cancel requests are never blocked by in-flight uploads.
func (s *Service) AppendChunk(ctx context.Context, id string, offset int64, data []byte) (*Job, error) {
	if s == nil || s.Store == nil || s.Drive == nil {
		return nil, errors.New("upload service not configured")
	}
	if len(data) == 0 {
		return nil, &ValidationError{Message: "empty chunk"}
	}
	if len(data) > MaxClientChunk {
		return nil, &ValidationError{Message: fmt.Sprintf("chunk exceeds %d bytes", MaxClientChunk)}
	}

	j, err := s.Store.Get(id)
	if err != nil {
		return nil, err
	}

	// Phase 1: Lock job — validate, append to buffer, set status
	j.mu.Lock()
	if j.Status == StatusCompleted || j.Status == StatusCancelled || j.Status == StatusFailed {
		j.mu.Unlock()
		return nil, &ValidationError{Message: "upload not accepting chunks: " + string(j.Status)}
	}
	if offset >= 0 && offset != j.BytesReceived {
		j.mu.Unlock()
		return nil, &ValidationError{Message: fmt.Sprintf("expected offset %d, got %d", j.BytesReceived, offset)}
	}
	if j.BytesReceived+int64(len(data)) > j.Total {
		j.mu.Unlock()
		return nil, &ValidationError{Message: "chunk exceeds total size"}
	}

	j.buffer = append(j.buffer, data...)
	j.BytesReceived += int64(len(data))
	j.Status = StatusUploading
	sessionURL := j.SessionURL
	total := j.Total
	j.mu.Unlock()

	// Phase 2: Flush loop — lock only to extract chunk slice, unlock for UploadRange
	// If another caller is already flushing, skip to avoid concurrent uploads.
	j.mu.Lock()
	if j.flushing {
		j.mu.Unlock()
		return j, nil
	}
	j.flushing = true
	j.flushGen++
	myGen := j.flushGen
	j.mu.Unlock()

	releaseFlush := func() {
		if j.flushGen == myGen {
			j.flushing = false
		}
	}

	for {
		j.mu.Lock()
		if j.Status == StatusCancelled {
			releaseFlush()
			j.mu.Unlock()
			return nil, &ValidationError{Message: "upload cancelled"}
		}
		if j.BytesSent >= total {
			releaseFlush()
			j.mu.Unlock()
			break
		}
		isFinalClient := j.BytesReceived >= total
		bufLen := int64(len(j.buffer))
		var take int64
		if isFinalClient {
			need := total - j.BytesSent
			if bufLen < need {
				releaseFlush()
				j.mu.Unlock()
				break
			}
			take = need
		} else {
			aligned := (bufLen / drive.ChunkMultiple) * drive.ChunkMultiple
			flushThreshold := j.FlushSize
			if flushThreshold == 0 {
				flushThreshold = drive.DefaultFlushSize
			}
			if aligned < flushThreshold {
				releaseFlush()
				j.mu.Unlock()
				break
			}
			take = aligned
		}

		// Copy chunk data so UploadRange can read it safely outside the lock.
		chunk := make([]byte, take)
		copy(chunk, j.buffer[:take])
		j.buffer = j.buffer[take:]

		start := j.BytesSent
		end := start + take - 1
		j.mu.Unlock()

		// Network I/O — NO lock held
		res, err := s.Drive.UploadRange(ctx, sessionURL, start, end, total, chunk)
		if err != nil {
			j.mu.Lock()
			// Only the active flusher may fail the job — a concurrent completer
			// must not be flipped back to failed.
			if j.flushGen == myGen && j.Status != StatusCompleted && j.Status != StatusCancelled {
				j.Status = StatusFailed
				j.Error = err.Error()
				j.buffer = nil
				j.UpdatedAt = time.Now().UTC()
			}
			releaseFlush()
			j.mu.Unlock()
			return nil, err
		}

		j.mu.Lock()
		if res.NextByte > 0 {
			// If Drive received fewer bytes than we sent, re-prepend unsent bytes to buffer.
			if res.NextByte < end+1 {
				// chunk[unsentOffset:] contains the unsent bytes.
				unsentOffset := res.NextByte - start
				if unsentOffset >= 0 && int64(unsentOffset) < int64(len(chunk)) {
					j.buffer = append(chunk[unsentOffset:], j.buffer...)
				}
			}
			j.BytesSent = res.NextByte
		} else {
			j.BytesSent = end + 1
		}
		if res.Complete {
			j.Status = StatusCompleted
			j.FileID = res.FileID
			j.buffer = nil
			releaseFlush()
			j.UpdatedAt = time.Now().UTC()
			j.mu.Unlock()
			return j, nil
		}
		j.UpdatedAt = time.Now().UTC()
		j.mu.Unlock()
	}

	// Phase 3: Check if we expected completion but didn't get it.
	// Only clear flushing if we are still the owning flusher — another
	// goroutine may have started after we released the flag on break.
	j.mu.Lock()
	releaseFlush()
	if j.BytesReceived >= total && j.BytesSent >= total && j.Status != StatusCompleted {
		// All bytes sent but Drive didn't signal completion (e.g. last chunk got 308).
		// Query the upload session status before declaring failure.
		j.mu.Unlock()
		status, qErr := s.Drive.QueryUploadStatus(ctx, sessionURL, total)
		j.mu.Lock()
		if qErr == nil && status.Complete {
			j.Status = StatusCompleted
			j.FileID = status.FileID
			j.buffer = nil
			j.UpdatedAt = time.Now().UTC()
			j.mu.Unlock()
			return j, nil
		}
		if qErr == nil && status.NextByte >= total {
			// Drive confirmed all bytes received but hasn't finalized.
			// Mark as completed — data is intact on Drive's side.
			j.Status = StatusCompleted
			j.buffer = nil
			j.UpdatedAt = time.Now().UTC()
			j.mu.Unlock()
			return j, nil
		}
		if j.Status != StatusCompleted && j.Status != StatusCancelled {
			j.Status = StatusFailed
			j.buffer = nil
			if qErr != nil {
				j.Error = "upload finished but Drive did not complete (status query failed: " + qErr.Error() + ")"
			} else {
				j.Error = "upload finished but Drive did not complete"
			}
			j.UpdatedAt = time.Now().UTC()
			errMsg := j.Error
			j.mu.Unlock()
			return nil, errors.New(errMsg)
		}
		j.mu.Unlock()
		return j, nil
	}
	j.mu.Unlock()

	return j, nil
}

// Cancel marks a job cancelled.
func (s *Service) Cancel(id string) (*Job, error) {
	if s == nil || s.Store == nil {
		return nil, errors.New("upload service not configured")
	}
	return s.Store.Update(id, func(j *Job) error {
		if j.Status == StatusCompleted {
			return &ValidationError{Message: "already completed"}
		}
		j.Status = StatusCancelled
		j.buffer = nil
		return nil
	})
}

// ValidationError is a 400-class input problem.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func newID() string {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return fmt.Sprintf("up_%d", time.Now().UnixNano())
	}
	return "up_" + hex.EncodeToString(b[:])
}
