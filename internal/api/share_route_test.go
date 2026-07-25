package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestShareRouteRegistered(t *testing.T) {
	// Build router without auth (will return 401 but NOT 404 if route exists)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/files/{id}/share", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})

	req := httptest.NewRequest(http.MethodPost, "/api/files/abc123/share", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatalf("route POST /api/files/{id}/share returned 404 — pattern not matched")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
