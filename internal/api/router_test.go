package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsk/drive-backup-console/internal/config"
)

func TestNewRouter_Health(t *testing.T) {
	h := NewRouter(Deps{Config: config.Config{DevMode: true}})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestNewRouter_Home(t *testing.T) {
	h := NewRouter(Deps{Config: config.Config{DevMode: true}})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct == "" || rr.Body.Len() < 20 {
		t.Fatalf("expected HTML home, ct=%q len=%d", ct, rr.Body.Len())
	}
}