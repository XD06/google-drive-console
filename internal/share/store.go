// Package share stores revocable direct links to Drive files. A link is an
// unguessable token served publicly at /d/{token}; the backend streams the
// file bytes from Google Drive using the stored OAuth token, so the link
// works from anywhere without a Google session.
package share

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Link is a public direct link to a Drive file.
type Link struct {
	Token     string    `json:"token"`
	FileID    string    `json:"fileId"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

// ErrNotFound is returned when a link does not exist.
var ErrNotFound = errors.New("link not found")

// Store persists links to a JSON file. One link per file: creating a second
// link for the same file returns the existing one (idempotent, like
// Google's anyone-with-link sharing).
type Store struct {
	mu    sync.Mutex
	path  string
	links map[string]Link // token → link
}

// New opens (or creates) the link store at path.
func New(path string) *Store {
	s := &Store{path: path, links: make(map[string]Link)}
	s.load()
	return s
}

// Create returns the existing link for fileID, or mints a new one.
func (s *Store) Create(fileID, name string) (Link, error) {
	if fileID == "" {
		return Link{}, errors.New("file id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.links {
		if l.FileID == fileID {
			return l, nil
		}
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return Link{}, err
	}
	l := Link{
		Token:     hex.EncodeToString(b),
		FileID:    fileID,
		Name:      name,
		CreatedAt: time.Now().UTC(),
	}
	s.links[l.Token] = l
	if err := s.save(); err != nil {
		delete(s.links, l.Token)
		return Link{}, err
	}
	return l, nil
}

// Get returns the link for a token.
func (s *Store) Get(token string) (Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.links[token]
	if !ok {
		return Link{}, ErrNotFound
	}
	return l, nil
}

// List returns all links, newest first.
func (s *Store) List() []Link {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Link, 0, len(s.links))
	for _, l := range s.links {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// ListByFile returns the links pointing at fileID (0 or 1 entries).
func (s *Store) ListByFile(fileID string) []Link {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Link, 0, 1)
	for _, l := range s.links {
		if l.FileID == fileID {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// Revoke deletes a link. Returns false when the token was unknown.
func (s *Store) Revoke(token string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.links[token]; !ok {
		return false, nil
	}
	delete(s.links, token)
	if err := s.save(); err != nil {
		return true, err
	}
	return true, nil
}

func (s *Store) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("load share links: %v", err)
		}
		return
	}
	var links []Link
	if err := json.Unmarshal(data, &links); err != nil {
		log.Printf("parse share links: %v", err)
		return
	}
	for _, l := range links {
		if l.Token != "" && l.FileID != "" {
			s.links[l.Token] = l
		}
	}
}

// save writes the store to disk atomically. Callers must hold s.mu.
func (s *Store) save() error {
	links := make([]Link, 0, len(s.links))
	for _, l := range s.links {
		links = append(links, l)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
