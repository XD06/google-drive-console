package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/dsk/drive-backup-console/internal/apikey"
	"github.com/dsk/drive-backup-console/internal/auth"
)

// testAuth builds a Service with a real session manager plus an apikey store.
func testAuth(t *testing.T) (*auth.Service, *apikey.Store) {
	t.Helper()
	sm, err := auth.NewSessionManager("test-secret", false)
	if err != nil {
		t.Fatal(err)
	}
	store := apikey.NewStore(filepath.Join(t.TempDir(), "apikeys.json"))
	return &auth.Service{Sessions: sm}, store
}

// sessionCookie returns a valid signed session cookie via exported methods.
func sessionCookie(t *testing.T, svc *auth.Service) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	if err := svc.Sessions.SetCookie(rec, auth.Session{Email: "owner@example.com", Expires: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie set")
	}
	return cookies[0]
}

// okHandler records that it ran and returns 200.
func okHandler(ran *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*ran = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuthenticate_NoCredential(t *testing.T) {
	svc, store := testAuth(t)
	var ran bool
	h := Authenticate(svc, store, okHandler(&ran))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/files", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if ran {
		t.Fatal("handler should not run without credentials")
	}
}

func TestAuthenticate_SessionCookie(t *testing.T) {
	svc, store := testAuth(t)
	var ran bool
	h := guardScope(svc, store, apikey.ScopeReadWrite, okHandler(&ran))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files", nil)
	req.AddCookie(sessionCookie(t, svc))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !ran {
		t.Fatalf("session should pass readwrite: code=%d ran=%v", rec.Code, ran)
	}
}

func TestAuthenticate_ReadKeyScopes(t *testing.T) {
	svc, store := testAuth(t)
	_, token, err := store.Create("reader", apikey.ScopeRead)
	if err != nil {
		t.Fatal(err)
	}

	// read scope passes a read-guarded route.
	var ranRead bool
	readRoute := guardScope(svc, store, apikey.ScopeRead, okHandler(&ranRead))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	readRoute.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !ranRead {
		t.Fatalf("read key on read route: code=%d ran=%v", rec.Code, ranRead)
	}

	// read scope is rejected on a readwrite-guarded route with 403.
	var ranWrite bool
	writeRoute := guardScope(svc, store, apikey.ScopeReadWrite, okHandler(&ranWrite))
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/files", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	writeRoute.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("read key on write route: want 403, got %d", rec2.Code)
	}
	if ranWrite {
		t.Fatal("handler must not run for insufficient scope")
	}
}

func TestAuthenticate_ReadWriteKey(t *testing.T) {
	svc, store := testAuth(t)
	_, token, err := store.Create("bot", apikey.ScopeReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	var ran bool
	route := guardScope(svc, store, apikey.ScopeReadWrite, okHandler(&ran))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	route.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !ran {
		t.Fatalf("readwrite key should pass: code=%d ran=%v", rec.Code, ran)
	}
}

func TestAuthenticate_InvalidAndRevoked(t *testing.T) {
	svc, store := testAuth(t)
	k, token, err := store.Create("bot", apikey.ScopeReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	var ran bool
	route := guardScope(svc, store, apikey.ScopeRead, okHandler(&ran))

	// Bogus token → 401.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	req.Header.Set("Authorization", "Bearer dbc_bogus")
	rec := httptest.NewRecorder()
	route.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bogus token: want 401, got %d", rec.Code)
	}

	// Revoked token → 401.
	if err := store.Revoke(k.ID); err != nil {
		t.Fatal(err)
	}
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	route.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token: want 401, got %d", rec2.Code)
	}
	if ran {
		t.Fatal("handler must not run for invalid/revoked keys")
	}
}

func TestAuthenticate_BearerWithoutKeyStore(t *testing.T) {
	sm, err := auth.NewSessionManager("test-secret", false)
	if err != nil {
		t.Fatal(err)
	}
	svc := &auth.Service{Sessions: sm}
	var ran bool
	h := Authenticate(svc, nil, okHandler(&ran))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	req.Header.Set("Authorization", "Bearer dbc_whatever")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || ran {
		t.Fatalf("bearer with nil store: want 401 no-run, got code=%d ran=%v", rec.Code, ran)
	}
}
