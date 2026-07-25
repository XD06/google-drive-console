package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dsk/drive-backup-console/internal/apikey"
	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/config"
)

// v1Router builds a real router with a session-capable auth service (backed by
// an empty token store) and an api-key store.
func v1Router(t *testing.T) (*auth.Service, *apikey.Store, http.Handler) {
	t.Helper()
	svc, err := auth.NewService(config.Config{
		DevMode:       true,
		SessionSecret: "v1-test-secret",
		TokenPath:     filepath.Join(t.TempDir(), "token.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	store := apikey.NewStore(filepath.Join(t.TempDir(), "apikeys.json"))
	h := NewRouter(Deps{Config: config.Config{DevMode: true}, Auth: svc, Keys: store})
	return svc, store, h
}

func doV1(h http.Handler, method, path, authz string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestV1_OpenAPIIsPublic(t *testing.T) {
	_, _, h := v1Router(t)
	rec := doV1(h, http.MethodGet, "/api/v1/openapi.json", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("openapi: want 200, got %d", rec.Code)
	}
}

func TestV1_RequiresAuth(t *testing.T) {
	_, _, h := v1Router(t)
	rec := doV1(h, http.MethodGet, "/api/v1/files", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no creds: want 401, got %d", rec.Code)
	}
}

func TestV1_ScopeGate(t *testing.T) {
	_, store, h := v1Router(t)
	_, readTok, _ := store.Create("reader", apikey.ScopeRead)
	_, rwTok, _ := store.Create("writer", apikey.ScopeReadWrite)

	// read key on a write route → 403, decided before the handler runs.
	rec := doV1(h, http.MethodPost, "/api/v1/files", "Bearer "+readTok, strings.NewReader(`{"name":"x"}`))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("read key on write route: want 403, got %d", rec.Code)
	}
	// readwrite key passes the scope gate (then fails on the missing token → not 403).
	rec2 := doV1(h, http.MethodPost, "/api/v1/files", "Bearer "+rwTok, strings.NewReader(`{"name":"x"}`))
	if rec2.Code == http.StatusForbidden {
		t.Fatal("readwrite key should pass the scope gate, got 403")
	}
}

func TestV1_KeysAreSessionOnly(t *testing.T) {
	svc, store, h := v1Router(t)
	_, rwTok, _ := store.Create("writer", apikey.ScopeReadWrite)

	// A key (even readwrite) cannot manage keys → 401.
	rec := doV1(h, http.MethodGet, "/api/v1/keys", "Bearer "+rwTok, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("key on keys endpoint: want 401, got %d", rec.Code)
	}

	// The owner (session cookie) can create a key → 201.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", strings.NewReader(`{"name":"ci","scope":"read"}`))
	req.AddCookie(sessionCookie(t, svc))
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("session create key: want 201, got %d body=%s", rec2.Code, rec2.Body.String())
	}
}
