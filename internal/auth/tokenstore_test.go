package auth

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestTokenStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")
	store := NewTokenStore(path)

	tok := &oauth2.Token{
		AccessToken:  "access",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	if err := store.Save(StoredToken{Email: "u@g.com", Token: tok}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "u@g.com" || got.Token.AccessToken != "access" || got.Token.RefreshToken != "refresh" {
		t.Fatalf("unexpected %+v", got)
	}
	if !store.Exists() {
		t.Fatal("expected exists")
	}
	if err := store.Delete(); err != nil {
		t.Fatal(err)
	}
	if store.Exists() {
		t.Fatal("expected deleted")
	}
}

func TestTokenStoreRejectsEmpty(t *testing.T) {
	store := NewTokenStore(filepath.Join(t.TempDir(), "t.json"))
	if err := store.Save(StoredToken{}); err == nil {
		t.Fatal("expected error")
	}
}
