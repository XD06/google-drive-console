package apikey

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestScopeValidAndAllows(t *testing.T) {
	if !ScopeRead.Valid() || !ScopeReadWrite.Valid() {
		t.Fatal("read/readwrite should be valid")
	}
	if Scope("bogus").Valid() {
		t.Fatal("bogus scope should be invalid")
	}
	// readwrite satisfies everything; read only satisfies read.
	if !ScopeReadWrite.Allows(ScopeRead) || !ScopeReadWrite.Allows(ScopeReadWrite) {
		t.Fatal("readwrite should allow read and readwrite")
	}
	if ScopeRead.Allows(ScopeReadWrite) {
		t.Fatal("read must not allow readwrite")
	}
	if !ScopeRead.Allows(ScopeRead) {
		t.Fatal("read should allow read")
	}
}

func TestTokenAndHintFormat(t *testing.T) {
	tok, err := generateToken()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tok, tokenPrefix) {
		t.Fatalf("token missing prefix: %q", tok)
	}
	if tok != strings.ToLower(tok) {
		t.Fatalf("token should be lowercase: %q", tok)
	}
	hint := hintFor(tok)
	if !strings.HasPrefix(hint, tokenPrefix) || !strings.HasSuffix(hint, "…") {
		t.Fatalf("unexpected hint: %q", hint)
	}
	// Hint must not reveal the full token.
	if strings.TrimSuffix(hint, "…") == tok {
		t.Fatal("hint leaks full token")
	}
	// Two tokens differ.
	tok2, _ := generateToken()
	if tok == tok2 {
		t.Fatal("tokens should be unique")
	}
}

func TestStoreCreateListVerify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "apikeys.json")
	s := NewStore(path)

	k, token, err := s.Create("research-bot", ScopeRead)
	if err != nil {
		t.Fatal(err)
	}
	if k.Hash != "" {
		t.Fatal("returned key must not carry hash")
	}
	if k.ID == "" || k.Scope != ScopeRead || k.Name != "research-bot" {
		t.Fatalf("unexpected key metadata: %+v", k)
	}

	// List is sanitized.
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Hash != "" {
		t.Fatalf("list should have 1 sanitized key, got %+v", list)
	}

	// Verify correct token.
	got, ok := s.Verify(token)
	if !ok || got.ID != k.ID || got.Scope != ScopeRead {
		t.Fatalf("verify failed for valid token: ok=%v got=%+v", ok, got)
	}
	// Verify sets LastUsedAt.
	if got.LastUsedAt == "" {
		t.Fatal("verify should set LastUsedAt")
	}
	// Wrong token rejected.
	if _, ok := s.Verify("dbc_wrongtoken"); ok {
		t.Fatal("verify should reject wrong token")
	}
	if _, ok := s.Verify(""); ok {
		t.Fatal("verify should reject empty token")
	}
}

func TestStoreRevoke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "apikeys.json")
	s := NewStore(path)
	k, token, err := s.Create("bot", ScopeReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Verify(token); !ok {
		t.Fatal("token should verify before revoke")
	}
	if err := s.Revoke(k.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Verify(token); ok {
		t.Fatal("revoked token must not verify")
	}
	// Revoking again is a no-op.
	if err := s.Revoke(k.ID); err != nil {
		t.Fatalf("re-revoke should be nil, got %v", err)
	}
	// Unknown id.
	if err := s.Revoke("deadbeef"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStorePersistenceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "apikeys.json")
	s1 := NewStore(path)
	_, token, err := s1.Create("bot", ScopeReadWrite)
	if err != nil {
		t.Fatal(err)
	}

	// A fresh store over the same file sees the persisted key.
	s2 := NewStore(path)
	got, ok := s2.Verify(token)
	if !ok || got.Scope != ScopeReadWrite {
		t.Fatalf("reloaded store failed to verify: ok=%v got=%+v", ok, got)
	}
	list, err := s2.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("reloaded store should have 1 key, got %d", len(list))
	}
}

func TestCreateValidation(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "apikeys.json"))
	if _, _, err := s.Create("  ", ScopeRead); err == nil {
		t.Fatal("empty name should error")
	}
	if _, _, err := s.Create("ok", Scope("bogus")); err == nil {
		t.Fatal("invalid scope should error")
	}
}
