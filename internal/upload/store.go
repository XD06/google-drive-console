package upload

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("upload not found")
	ErrConflict = errors.New("upload state conflict")
)

// Store is an in-memory job registry.
type Store struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

// NewStore creates an empty store.
func NewStore() *Store {
	return &Store{jobs: make(map[string]*Job)}
}

// Put inserts or replaces a job.
func (s *Store) Put(j *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.ID] = j
}

// Get returns a job by id.
func (s *Store) Get(id string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return j, nil
}

// Delete removes a job.
func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
}

// Update applies fn under the Store map lock. fn must NOT perform network I/O —
// use the Job's own mutex (j.mu) for field-level synchronization and do I/O
// outside any Store lock. This method is retained only for simple atomic
// state transitions (e.g. Cancel).
func (s *Store) Update(id string, fn func(j *Job) error) (*Job, error) {
	s.mu.Lock()
	j, ok := s.jobs[id]
	s.mu.Unlock()
	if !ok {
		return nil, ErrNotFound
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if err := fn(j); err != nil {
		return j, err
	}
	j.UpdatedAt = time.Now().UTC()
	return j, nil
}

// DeleteOlderThan removes terminal jobs whose UpdatedAt is older than maxAge.
// Returns the number of jobs deleted. Pending/uploading jobs are never removed.
func (s *Store) DeleteOlderThan(maxAge time.Duration) int {
	if maxAge <= 0 {
		return 0
	}
	cutoff := time.Now().UTC().Add(-maxAge)
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for id, j := range s.jobs {
		j.mu.RLock()
		terminal := j.Status == StatusCompleted || j.Status == StatusFailed || j.Status == StatusCancelled
		old := !j.UpdatedAt.After(cutoff)
		j.mu.RUnlock()
		if terminal && old {
			delete(s.jobs, id)
			n++
		}
	}
	return n
}