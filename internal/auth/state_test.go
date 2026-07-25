package auth

import (
	"testing"
	"time"
)

func TestStateIssueConsume(t *testing.T) {
	s := NewStateStore(time.Minute)
	st, err := s.Issue()
	if err != nil || st == "" {
		t.Fatalf("issue: %v %q", err, st)
	}
	if err := s.Consume(st); err != nil {
		t.Fatal(err)
	}
	if err := s.Consume(st); err == nil {
		t.Fatal("reuse should fail")
	}
}

func TestStateUnknown(t *testing.T) {
	s := NewStateStore(time.Minute)
	if err := s.Consume("nope"); err == nil {
		t.Fatal("expected error")
	}
}
