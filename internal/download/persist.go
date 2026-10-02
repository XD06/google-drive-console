package download

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
type PersistentStore struct {
	*Store
	path     string
	mu       sync.Mutex
	timer    *time.Timer
	dirty    bool
	reaperCh chan struct{}
	reaperWg sync.WaitGroup

	// CacheTTL keeps completed jobs (URL→Drive dedupe cache) this long.
	// Zero means DefaultCacheTTL.
	CacheTTL time.Duration
	// TerminalTTL keeps failed/cancelled jobs this long. Zero means 1h.
	TerminalTTL time.Duration
}

func (ps *PersistentStore) cacheTTL() time.Duration {
	if ps.CacheTTL > 0 {
		return ps.CacheTTL
	}
	return DefaultCacheTTL
}

func (ps *PersistentStore) terminalTTL() time.Duration {
	if ps.TerminalTTL > 0 {
		return ps.TerminalTTL
	}
	return time.Hour
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

// DeleteExpired removes terminal jobs past their per-status TTL: completed
// jobs live for the cache TTL (URL dedupe cache), failed/cancelled jobs for
// the shorter terminal TTL.
func (ps *PersistentStore) DeleteExpired() int {
	n := ps.Store.DeleteExpired(ps.cacheTTL(), ps.terminalTTL())
	if n > 0 {
		ps.scheduleSave()
	}
	return n
}

// StartReaper periodically deletes terminal jobs older than maxAge.
// Deprecated: prefer StartReaperTTLs, which applies the per-status TTLs.
func (ps *PersistentStore) StartReaper(interval, maxAge time.Duration) {
	ps.StartReaperTTLs(interval, maxAge, maxAge)
}

// StartReaperTTLs periodically deletes expired terminal jobs: completed older
// than doneTTL, failed/cancelled older than otherTTL.
func (ps *PersistentStore) StartReaperTTLs(interval, doneTTL, otherTTL time.Duration) {
	if interval <= 0 || doneTTL <= 0 || otherTTL <= 0 {
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

	reap := func() {
		if n := ps.Store.DeleteExpired(doneTTL, otherTTL); n > 0 {
			ps.scheduleSave()
			log.Printf("download reaper: removed %d terminal jobs (done>%s, other>%s)", n, doneTTL, otherTTL)
		}
	}

	ps.reaperWg.Add(1)
	go func() {
		defer ps.reaperWg.Done()
		reap()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				reap()
			case <-ch:
				return
			}
		}
	}()
}

// StopReaper stops the background reaper.
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

// Update applies fn and schedules a disk save.
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
		return
	}
	ps.timer = time.AfterFunc(time.Second, ps.flush)
}

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
		log.Printf("persist downloads: %v", err)
	}
}

// SaveNow forces an immediate disk save (e.g. on shutdown).
func (ps *PersistentStore) SaveNow() {
	ps.mu.Lock()
	if ps.timer != nil {
		ps.timer.Stop()
		ps.timer = nil
	}
	ps.dirty = false
	ps.mu.Unlock()
	if err := ps.saveToDisk(); err != nil {
		log.Printf("persist downloads (shutdown): %v", err)
	}
}

// persistJob is the on-disk representation (excludes mutex and cancel channel).
type persistJob struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	ParentID    string    `json:"parentId,omitempty"`
	Status      Status    `json:"status"`
	Progress    float64   `json:"progress"`
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

func (ps *PersistentStore) saveToDisk() error {
	ps.Store.mu.Lock()
	jobs := make([]persistJob, 0, len(ps.Store.jobs))
	for _, j := range ps.Store.jobs {
		j.mu.RLock()
		jobs = append(jobs, persistJob{
			ID:          j.ID,
			URL:         j.URL,
			ParentID:    j.ParentID,
			Status:      j.Status,
			Progress:    j.Progress,
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
		})
		j.mu.RUnlock()
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
			log.Printf("load persisted downloads: %v", err)
		}
		return
	}

	var jobs []persistJob
	if err := json.Unmarshal(data, &jobs); err != nil {
		log.Printf("parse persisted downloads: %v", err)
		return
	}

	now := time.Now().UTC()
	for _, pj := range jobs {
		j := &Job{
			ID:          pj.ID,
			URL:         pj.URL,
			ParentID:    pj.ParentID,
			Status:      pj.Status,
			Progress:    pj.Progress,
			Downloaded:  pj.Downloaded,
			Total:       pj.Total,
			Title:       pj.Title,
			Thumbnail:   pj.Thumbnail,
			Extractor:   pj.Extractor,
			Uploader:    pj.Uploader,
			Duration:    pj.Duration,
			Ext:         pj.Ext,
			FileName:    pj.FileName,
			DriveFileID: pj.DriveFileID,
			Error:       pj.Error,
			CreatedAt:   pj.CreatedAt,
			UpdatedAt:   pj.UpdatedAt,
			cancelCh:    make(chan struct{}),
		}

		// Jobs that were active when the server stopped become paused: the
		// yt-dlp .part file (when there was one) survives on disk, so Resume
		// can pick up where the restart interrupted them.
		if j.Status == StatusPending || j.Status == StatusResolving ||
			j.Status == StatusDownloading || j.Status == StatusUploading {
			j.Status = StatusPaused
			j.Speed = ""
			j.ETA = ""
			j.UpdatedAt = now
		}

		// Drop stale terminal jobs: completed past the cache TTL, failed/
		// cancelled past the terminal TTL.
		if isTerminal(j.Status) {
			ttl := ps.terminalTTL()
			if j.Status == StatusCompleted {
				ttl = ps.cacheTTL()
			}
			if now.Sub(j.UpdatedAt) > ttl {
				continue
			}
		}

		ps.Store.Put(j)
	}
	log.Printf("loaded %d persisted download jobs from %s", len(ps.Store.jobs), ps.path)
}

func isTerminal(s Status) bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled
}
