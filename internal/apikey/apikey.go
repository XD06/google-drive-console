// Package apikey provides named, revocable API keys with read / read-write
// scopes for programmatic (AI agent / script) access to the console API.
//
// Tokens are high-entropy secrets shown to the user exactly once at creation;
// only a SHA-256 hash is persisted. Verification compares hashes in constant
// time. Keys authorize calling the API as the site owner (the server-side
// Google token) — the scope limits what a key may do, not whose Drive it sees.
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"strings"
)

// Scope controls what actions an API key may perform.
type Scope string

const (
	// ScopeRead permits read-only operations (list, search, read, download).
	ScopeRead Scope = "read"
	// ScopeReadWrite permits reads plus mutations (write, create, move, delete…).
	ScopeReadWrite Scope = "readwrite"
)

// Valid reports whether s is a recognized scope.
func (s Scope) Valid() bool {
	return s == ScopeRead || s == ScopeReadWrite
}

// Allows reports whether a key with scope s may perform an action that
// requires at least min. ScopeReadWrite satisfies any requirement.
func (s Scope) Allows(min Scope) bool {
	if s == ScopeReadWrite {
		return true
	}
	return s == ScopeRead && min == ScopeRead
}

// Key is a persisted API key. It never carries the plaintext token; Hash is
// stripped from any copy returned to callers (see Store.List / Create).
type Key struct {
	ID         string `json:"id"`             // short random id, used to reference/revoke
	Name       string `json:"name"`           // human label, e.g. "research-bot"
	Hint       string `json:"hint"`           // visible prefix, e.g. "dbc_a1b2c3…"
	Hash       string `json:"hash,omitempty"` // SHA-256(token) hex; never exposed via API
	Scope      Scope  `json:"scope"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
	Revoked    bool   `json:"revoked"`
}

const (
	tokenPrefix = "dbc_"
	tokenBytes  = 32 // 256-bit secret
	idBytes     = 8
	hintLen     = 6 // visible characters after the prefix
)

// b32 is lowercase, unpadded base32 for compact, URL-safe tokens.
var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// hashToken returns the hex-encoded SHA-256 of a token.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// generateToken returns a new random token like "dbc_<base32>".
func generateToken() (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return tokenPrefix + strings.ToLower(b32.EncodeToString(raw)), nil
}

// generateID returns a short random hex identifier for a key.
func generateID() (string, error) {
	raw := make([]byte, idBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// hintFor returns the non-secret identifier prefix of a token, e.g. "dbc_a1b2c3…".
func hintFor(token string) string {
	body := strings.TrimPrefix(token, tokenPrefix)
	if len(body) > hintLen {
		body = body[:hintLen]
	}
	return tokenPrefix + body + "…"
}

// sanitize returns a copy of k with the secret hash removed.
func sanitize(k Key) Key {
	k.Hash = ""
	return k
}
