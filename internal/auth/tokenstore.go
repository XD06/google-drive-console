package auth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// StoredToken is the on-disk OAuth token plus account email.
type StoredToken struct {
	Email string        `json:"email"`
	Token *oauth2.Token `json:"token"`
}

// TokenStore persists tokens as JSON under a fixed path.
type TokenStore struct {
	path string
	mu   sync.Mutex
	// mtime-based cache to avoid reading disk on every request
	cached    StoredToken
	cachedMt time.Time
	hasCache bool
}

// NewTokenStore creates a store at path (parent dir must be writable).
func NewTokenStore(path string) *TokenStore {
	return &TokenStore{path: path}
}

// Save writes token atomically (temp file + rename).
func (s *TokenStore) Save(st StoredToken) error {
	if st.Email == "" || st.Token == nil {
		return errors.New("email and token required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return err
	}
	// Update cache after successful save
	if fi, err := os.Stat(s.path); err == nil {
		s.cached = st
		s.cachedMt = fi.ModTime()
		s.hasCache = true
	}
	return nil
}

// Load reads the token file. Returns os.ErrNotExist if missing.
// Uses mtime-based caching: if the file hasn't changed since last read,
// returns the cached token without disk I/O.
func (s *TokenStore) Load() (StoredToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fi, err := os.Stat(s.path)
	if err != nil {
		return StoredToken{}, err
	}
	// Return cached token if file hasn't changed
	if s.hasCache && fi.ModTime().Equal(s.cachedMt) {
		return s.cached, nil
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return StoredToken{}, err
	}
	var st StoredToken
	if err := json.Unmarshal(raw, &st); err != nil {
		return StoredToken{}, err
	}
	if st.Email == "" || st.Token == nil {
		return StoredToken{}, errors.New("invalid token file")
	}
	// Update cache
	s.cached = st
	s.cachedMt = fi.ModTime()
	s.hasCache = true
	return st, nil
}

// Delete removes the token file if present.
func (s *TokenStore) Delete() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasCache = false // Invalidate cache
	err := os.Remove(s.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Exists reports whether a token file is present.
func (s *TokenStore) Exists() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := os.Stat(s.path)
	return err == nil
}

// TokenSource returns an oauth2.TokenSource that refreshes using conf and persists updates.
func (s *TokenStore) TokenSource(conf *oauth2.Config, base StoredToken) oauth2.TokenSource {
	return oauth2.ReuseTokenSource(base.Token, &persistingSource{
		store: s,
		email: base.Email,
		inner: conf.TokenSource(context.Background(), base.Token),
	})
}

type persistingSource struct {
	store *TokenStore
	email string
	inner oauth2.TokenSource
}

func (p *persistingSource) Token() (*oauth2.Token, error) {
	tok, err := p.inner.Token()
	if err != nil {
		return nil, err
	}
	// Best-effort persist on refresh (expiry moved forward).
	_ = p.store.Save(StoredToken{Email: p.email, Token: tok})
	return tok, nil
}

// Ensure token still valid helper for tests.
func tokenValid(t *oauth2.Token) bool {
	if t == nil {
		return false
	}
	if t.Valid() {
		return true
	}
	// Valid() requires AccessToken; allow refresh-only presence check separately.
	return t.RefreshToken != "" && !t.Expiry.IsZero() && time.Until(t.Expiry) > 0
}
