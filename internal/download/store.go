package download

import (
	"errors"
	"sync"
	"time"
)

// ErrNotFound is returned when a job does not exist.
var ErrNotFound = errors.New("download job not found")

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

// List returns all jobs as snapshots sorted by CreatedAt descending (newest first).
func (s *Store) List() []JobSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]JobSnapshot, 0, len(s.jobs))
	for _, j := range s.jobs {
		result = append(result, j.View())
	}
	return result
}

// Update applies fn under the Store map lock.
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
		terminal := isTerminal(j.Status)
		old := !j.UpdatedAt.After(cutoff)
		j.mu.RUnlock()
		if terminal && old {
			delete(s.jobs, id)
			n++
		}
	}
	return n
}

// DeleteExpired removes terminal jobs past their per-status TTL: completed
// jobs (the URL→Drive dedupe cache) after doneTTL, failed/cancelled after
// otherTTL. Non-terminal jobs — including paused ones — are never touched.
func (s *Store) DeleteExpired(doneTTL, otherTTL time.Duration) int {
	if doneTTL <= 0 || otherTTL <= 0 {
		return 0
	}
	doneCutoff := time.Now().UTC().Add(-doneTTL)
	otherCutoff := time.Now().UTC().Add(-otherTTL)
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for id, j := range s.jobs {
		j.mu.RLock()
		status := j.Status
		updated := j.UpdatedAt
		j.mu.RUnlock()
		var cutoff time.Time
		switch status {
		case StatusCompleted:
			cutoff = doneCutoff
		case StatusFailed, StatusCancelled:
			cutoff = otherCutoff
		default:
			continue
		}
		if !updated.After(cutoff) {
			delete(s.jobs, id)
			n++
		}
	}
	return n
}
