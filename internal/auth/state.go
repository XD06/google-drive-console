package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// StateStore holds short-lived OAuth CSRF states in memory.
type StateStore struct {
	mu   sync.Mutex
	ttl  time.Duration
	data map[string]time.Time
}

// NewStateStore creates an in-memory state map (process-local).
func NewStateStore(ttl time.Duration) *StateStore {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &StateStore{ttl: ttl, data: make(map[string]time.Time)}
}

// Issue creates a new state value.
func (s *StateStore) Issue() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := hex.EncodeToString(b)
	s.mu.Lock()
	s.purgeLocked(time.Now())
	s.data[state] = time.Now().Add(s.ttl)
	s.mu.Unlock()
	return state, nil
}

// Consume validates and removes state (one-time use).
func (s *StateStore) Consume(state string) error {
	if state == "" {
		return errors.New("empty state")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.purgeLocked(time.Now())
	exp, ok := s.data[state]
	if !ok {
		return errors.New("unknown or reused state")
	}
	delete(s.data, state)
	if time.Now().After(exp) {
		return errors.New("state expired")
	}
	return nil
}

func (s *StateStore) purgeLocked(now time.Time) {
	for k, exp := range s.data {
		if now.After(exp) {
			delete(s.data, k)
		}
	}
}
