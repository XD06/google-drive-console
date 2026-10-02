package share

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestStoreCreateGetRevoke(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "links.json"))

	l, err := s.Create("f1", "clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Token) != 32 || l.FileID != "f1" || l.Name != "clip.mp4" {
		t.Fatalf("unexpected link: %+v", l)
	}

	got, err := s.Get(l.Token)
	if err != nil || got.FileID != "f1" {
		t.Fatalf("get: %+v err=%v", got, err)
	}

	// Same file → same token (idempotent).
	again, err := s.Create("f1", "clip.mp4")
	if err != nil || again.Token != l.Token {
		t.Fatalf("second create: %+v err=%v", again, err)
	}
	// Different file → different token.
	other, err := s.Create("f2", "b.mp4")
	if err != nil || other.Token == l.Token {
		t.Fatalf("other file link collides: %+v", other)
	}

	if ok, err := s.Revoke(l.Token); err != nil || !ok {
		t.Fatalf("revoke: ok=%v err=%v", ok, err)
	}
	if _, err := s.Get(l.Token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after revoke: err=%v", err)
	}
	if ok, _ := s.Revoke(l.Token); ok {
		t.Fatal("second revoke should report false")
	}
}

func TestStorePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "links.json")
	s := New(path)
	l, err := s.Create("f1", "a.bin")
	if err != nil {
		t.Fatal(err)
	}

	s2 := New(path)
	got, err := s2.Get(l.Token)
	if err != nil || got.FileID != "f1" || got.Name != "a.bin" {
		t.Fatalf("reopened store lost link: %+v err=%v", got, err)
	}
}

func TestStoreCreateEmptyFileID(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "links.json"))
	if _, err := s.Create("", "x"); err == nil {
		t.Fatal("empty file id should be rejected")
	}
}
