package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	m, err := NewSessionManager("test-secret-key", false)
	if err != nil {
		t.Fatal(err)
	}
	s := Session{Email: "user@example.com", Expires: time.Now().UTC().Add(time.Hour)}
	val, err := m.Encode(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Decode(val)
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != s.Email {
		t.Fatalf("email %q", got.Email)
	}
}

func TestSessionTamperRejected(t *testing.T) {
	m, _ := NewSessionManager("test-secret-key", false)
	val, _ := m.Encode(Session{Email: "a@b.com", Expires: time.Now().UTC().Add(time.Hour)})
	// flip last char of signature
	bad := val[:len(val)-1] + "x"
	if _, err := m.Decode(bad); err == nil {
		t.Fatal("expected signature error")
	}
}

func TestSessionExpired(t *testing.T) {
	m, _ := NewSessionManager("test-secret-key", false)
	val, _ := m.Encode(Session{Email: "a@b.com", Expires: time.Now().UTC().Add(-time.Minute)})
	if _, err := m.Decode(val); err == nil {
		t.Fatal("expected expiry error")
	}
}

func TestSetAndReadCookie(t *testing.T) {
	m, _ := NewSessionManager("test-secret-key", false)
	rr := httptest.NewRecorder()
	sess := Session{Email: "u@g.com", Expires: time.Now().UTC().Add(time.Hour)}
	if err := m.SetCookie(rr, sess); err != nil {
		t.Fatal(err)
	}
	res := rr.Result()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range res.Cookies() {
		req.AddCookie(c)
	}
	got, err := m.FromRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "u@g.com" {
		t.Fatalf("got %q", got.Email)
	}
}
