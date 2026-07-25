package apikey

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrNotFound is returned when revoking a key id that does not exist.
var ErrNotFound = errors.New("api key not found")

// touchInterval throttles LastUsedAt disk writes on verify.
const touchInterval = time.Minute

// Store persists API keys as JSON and verifies presented tokens.
// It keeps an in-memory copy guarded by a mutex; all mutations persist
// atomically (temp file + rename), mirroring the OAuth token store.
type Store struct {
	path   string
	mu     sync.Mutex
	keys   []Key
	loaded bool
}

// NewStore creates a store backed by path (parent dir created on first write).
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Create mints a new key, persists its hash, and returns the sanitized key
// metadata plus the plaintext token (shown to the caller exactly once).
func (s *Store) Create(name string, scope Scope) (Key, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Key{}, "", errors.New("name required")
	}
	if !scope.Valid() {
		return Key{}, "", errors.New("invalid scope")
	}
	token, err := generateToken()
	if err != nil {
		return Key{}, "", err
	}
	id, err := generateID()
	if err != nil {
		return Key{}, "", err
	}
	k := Key{
		ID:        id,
		Name:      name,
		Hint:      hintFor(token),
		Hash:      hashToken(token),
		Scope:     scope,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return Key{}, "", err
	}
	s.keys = append(s.keys, k)
	if err := s.persist(); err != nil {
		return Key{}, "", err
	}
	return sanitize(k), token, nil
}

// List returns all keys (newest last) with secrets stripped.
func (s *Store) List() ([]Key, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	out := make([]Key, 0, len(s.keys))
	for _, k := range s.keys {
		out = append(out, sanitize(k))
	}
	return out, nil
}

// Verify resolves a plaintext token to its key. It returns the sanitized key
// and true when the token matches an existing, non-revoked key. LastUsedAt is
// updated (throttled) as a side effect.
func (s *Store) Verify(token string) (Key, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Key{}, false
	}
	h := hashToken(token)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return Key{}, false
	}
	for i := range s.keys {
		k := &s.keys[i]
		if k.Revoked {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(k.Hash), []byte(h)) == 1 {
			s.touchLocked(k)
			return sanitize(*k), true
		}
	}
	return Key{}, false
}

// Revoke marks a key revoked by id. Returns ErrNotFound if absent.
func (s *Store) Revoke(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	for i := range s.keys {
		if s.keys[i].ID == id {
			if s.keys[i].Revoked {
				return nil
			}
			s.keys[i].Revoked = true
			return s.persist()
		}
	}
	return ErrNotFound
}

// touchLocked updates LastUsedAt at most once per touchInterval and best-effort
// persists it. Caller must hold s.mu.
func (s *Store) touchLocked(k *Key) {
	now := time.Now().UTC()
	if k.LastUsedAt != "" {
		if prev, err := time.Parse(time.RFC3339, k.LastUsedAt); err == nil && now.Sub(prev) < touchInterval {
			return
		}
	}
	k.LastUsedAt = now.Format(time.RFC3339)
	_ = s.persist() // best-effort; verification succeeds regardless
}

// ensureLoaded lazily loads keys from disk once. Caller must hold s.mu.
func (s *Store) ensureLoaded() error {
	if s.loaded {
		return nil
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.keys = nil
			s.loaded = true
			return nil
		}
		return err
	}
	var ks []Key
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &ks); err != nil {
			return err
		}
	}
	s.keys = ks
	s.loaded = true
	return nil
}

// persist writes the in-memory keys atomically. Caller must hold s.mu.
func (s *Store) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.keys, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
