package upload

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PersistentStore wraps Store with disk persistence.
// Jobs are saved to a JSON file so they survive server restarts.
// On restart, jobs in "uploading" status are marked as "failed"
// because the resumable upload session may have expired.
type PersistentStore struct {
	*Store
	path     string
	mu       sync.Mutex
	timer    *time.Timer
	dirty    bool
	reaperCh chan struct{}
	reaperWg sync.WaitGroup
}

// NewPersistentStore creates a store that persists to the given file path.
func NewPersistentStore(path string) *PersistentStore {
	ps := &PersistentStore{
		Store: NewStore(),
		path:  path,
	}
	ps.loadFromDisk()
	return ps
}

// Put inserts or replaces a job and schedules a disk save.
func (ps *PersistentStore) Put(j *Job) {
	ps.Store.Put(j)
	ps.scheduleSave()
}

// Delete removes a job and schedules a disk save.
func (ps *PersistentStore) Delete(id string) {
	ps.Store.Delete(id)
	ps.scheduleSave()
}

// DeleteOlderThan removes terminal jobs older than maxAge and schedules a save if any were removed.
func (ps *PersistentStore) DeleteOlderThan(maxAge time.Duration) int {
	n := ps.Store.DeleteOlderThan(maxAge)
	if n > 0 {
		ps.scheduleSave()
	}
	return n
}

// StartReaper periodically deletes terminal jobs older than maxAge.
// Safe to call once; subsequent calls are no-ops while a reaper is running.
func (ps *PersistentStore) StartReaper(interval, maxAge time.Duration) {
	if interval <= 0 || maxAge <= 0 {
		return
	}
	ps.mu.Lock()
	if ps.reaperCh != nil {
		ps.mu.Unlock()
		return
	}
	ps.reaperCh = make(chan struct{})
	ch := ps.reaperCh
	ps.mu.Unlock()

	ps.reaperWg.Add(1)
	go func() {
		defer ps.reaperWg.Done()
		// Run once immediately so long-lived processes don't wait a full interval
		// after a crash-free restart that still has old jobs in memory.
		if n := ps.DeleteOlderThan(maxAge); n > 0 {
			log.Printf("upload reaper: removed %d terminal jobs older than %s", n, maxAge)
		}
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if n := ps.DeleteOlderThan(maxAge); n > 0 {
					log.Printf("upload reaper: removed %d terminal jobs older than %s", n, maxAge)
				}
			case <-ch:
				return
			}
		}
	}()
}

// StopReaper stops the background reaper and waits for it to exit.
func (ps *PersistentStore) StopReaper() {
	ps.mu.Lock()
	ch := ps.reaperCh
	ps.reaperCh = nil
	ps.mu.Unlock()
	if ch == nil {
		return
	}
	close(ch)
	ps.reaperWg.Wait()
}

// Update applies fn under the Store map lock and schedules a disk save.
func (ps *PersistentStore) Update(id string, fn func(j *Job) error) (*Job, error) {
	j, err := ps.Store.Update(id, fn)
	if err == nil {
		ps.scheduleSave()
	}
	return j, err
}

// scheduleSave debounces disk writes to at most once per second.
func (ps *PersistentStore) scheduleSave() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.dirty = true
	if ps.timer != nil {
		return // already scheduled
	}
	ps.timer = time.AfterFunc(time.Second, ps.flush)
}

// flush writes dirty state to disk.
func (ps *PersistentStore) flush() {
	ps.mu.Lock()
	if !ps.dirty {
		ps.mu.Unlock()
		return
	}
	ps.dirty = false
	ps.timer = nil
	ps.mu.Unlock()

	if err := ps.saveToDisk(); err != nil {
		log.Printf("persist uploads: %v", err)
	}
}

// SaveNow forces an immediate disk save (e.g., on shutdown).
func (ps *PersistentStore) SaveNow() {
	ps.mu.Lock()
	if ps.timer != nil {
		ps.timer.Stop()
		ps.timer = nil
	}
	ps.dirty = false
	ps.mu.Unlock()
	if err := ps.saveToDisk(); err != nil {
		log.Printf("persist uploads (shutdown): %v", err)
	}
}

// persistJob is the on-disk representation of a Job.
// It excludes the mutex and buffer (which are transient).
type persistJob struct {
	ID            string    `json:"uploadId"`
	Name          string    `json:"name"`
	MimeType      string    `json:"mimeType"`
	ParentID      string    `json:"parentId,omitempty"`
	Total         int64     `json:"total"`
	BytesSent     int64     `json:"bytesSent"`
	BytesReceived int64     `json:"bytesReceived"`
	Status        Status    `json:"status"`
	FileID        string    `json:"fileId,omitempty"`
	Error         string    `json:"error,omitempty"`
	SessionURL    string    `json:"sessionUrl,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (ps *PersistentStore) saveToDisk() error {
	ps.Store.mu.Lock()
	jobs := make([]persistJob, 0, len(ps.Store.jobs))
	for _, j := range ps.Store.jobs {
		j.mu.RLock()
		pj := persistJob{
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
		j.mu.RUnlock()
		// Skip transient jobs that haven't started
		if pj.Status == StatusPending && pj.BytesReceived == 0 {
			continue
		}
		jobs = append(jobs, pj)
	}
	ps.Store.mu.Unlock()

	dir := filepath.Dir(ps.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return err
	}

	tmp := ps.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, ps.path)
}

func (ps *PersistentStore) loadFromDisk() {
	data, err := os.ReadFile(ps.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("load persisted uploads: %v", err)
		}
		return
	}

	var jobs []persistJob
	if err := json.Unmarshal(data, &jobs); err != nil {
		log.Printf("parse persisted uploads: %v", err)
		return
	}

	now := time.Now().UTC()
	for _, pj := range jobs {
		j := &Job{
			ID:            pj.ID,
			Name:          pj.Name,
			MimeType:      pj.MimeType,
			ParentID:      pj.ParentID,
			Total:         pj.Total,
			BytesSent:     pj.BytesSent,
			BytesReceived: pj.BytesReceived,
			Status:        pj.Status,
			FileID:        pj.FileID,
			Error:         pj.Error,
			SessionURL:    pj.SessionURL,
			CreatedAt:     pj.CreatedAt,
			UpdatedAt:     pj.UpdatedAt,
		}

		// Jobs that were "uploading" when the server stopped are now failed
		// because the resumable session may have expired.
		if j.Status == StatusUploading || j.Status == StatusPending {
			j.Status = StatusFailed
			j.Error = "server restarted during upload"
			j.UpdatedAt = now
		}

		// Clean up old completed/failed jobs (> 24 hours old)
		if j.Status == StatusCompleted || j.Status == StatusFailed || j.Status == StatusCancelled {
			if now.Sub(j.UpdatedAt) > 24*time.Hour {
				continue // skip old jobs
			}
		}

		ps.Store.Put(j)
	}
	log.Printf("loaded %d persisted upload jobs from %s", len(ps.Store.jobs), ps.path)
}
