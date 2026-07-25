package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dsk/drive-backup-console/internal/apikey"
)

func newKeyHandlers(t *testing.T) *APIKeyHandlers {
	t.Helper()
	return &APIKeyHandlers{Keys: apikey.NewStore(filepath.Join(t.TempDir(), "apikeys.json"))}
}

func TestAPIKeyCreate_ReturnsTokenOnce(t *testing.T) {
	h := newKeyHandlers(t)
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest(http.MethodPost, "/api/v1/keys", strings.NewReader(`{"name":"bot","scope":"readwrite"}`)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if tok, _ := resp["token"].(string); !strings.HasPrefix(tok, "dbc_") {
		t.Fatalf("expected token, got %v", resp["token"])
	}
	if _, ok := resp["hash"]; ok {
		t.Fatal("response must not include hash")
	}
	if id, _ := resp["id"].(string); id == "" {
		t.Fatal("expected id")
	}
	if resp["scope"] != "readwrite" {
		t.Fatalf("scope = %v", resp["scope"])
	}
}

func TestAPIKeyCreate_Validation(t *testing.T) {
	h := newKeyHandlers(t)
	// Missing name.
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest(http.MethodPost, "/api/v1/keys", strings.NewReader(`{"scope":"read"}`)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("no name: want 400, got %d", rr.Code)
	}
	// Bad scope.
	rr2 := httptest.NewRecorder()
	h.Create(rr2, httptest.NewRequest(http.MethodPost, "/api/v1/keys", strings.NewReader(`{"name":"x","scope":"admin"}`)))
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("bad scope: want 400, got %d", rr2.Code)
	}
}

func TestAPIKeyList_NoSecrets(t *testing.T) {
	h := newKeyHandlers(t)
	if _, _, err := h.Keys.Create("a", apikey.ScopeRead); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.Keys.Create("b", apikey.ScopeReadWrite); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	h.List(rr, httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	var resp struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Keys) != 2 {
		t.Fatalf("want 2 keys, got %d", len(resp.Keys))
	}
	for _, k := range resp.Keys {
		if _, ok := k["hash"]; ok {
			t.Fatal("list must not include hash")
		}
		if _, ok := k["token"]; ok {
			t.Fatal("list must not include token")
		}
	}
}

func TestAPIKeyRevoke(t *testing.T) {
	h := newKeyHandlers(t)
	k, _, err := h.Keys.Create("bot", apikey.ScopeReadWrite)
	if err != nil {
		t.Fatal(err)
	}
	// Route through a mux so PathValue("id") is populated.
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/v1/keys/{id}", h.Revoke)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/v1/keys/"+k.ID, nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("revoke: want 204, got %d", rr.Code)
	}
	// Unknown id → 404.
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodDelete, "/api/v1/keys/deadbeef", nil))
	if rr2.Code != http.StatusNotFound {
		t.Fatalf("unknown id: want 404, got %d", rr2.Code)
	}
}
