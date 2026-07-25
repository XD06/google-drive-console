package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dsk/drive-backup-console/internal/config"
)

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSPAHandler_ImmutableAssets(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "index.html"), "<!doctype html><title>app</title>")
	mustWrite(t, filepath.Join(dir, "assets", "index-abc123.js"), "console.log(1)")
	mustWrite(t, filepath.Join(dir, "manifest.webmanifest"), "{}")

	h := SPAHandler(dir)

	// Hashed asset → immutable, 1 year.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/assets/index-abc123.js", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("asset status = %d", rr.Code)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Fatalf("asset Cache-Control = %q", cc)
	}

	// Non-hashed static file → no-cache (always revalidate).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("manifest status = %d", rr.Code)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("manifest Cache-Control = %q", cc)
	}
}

func TestSPAHandler_ClientRouteFallback(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "index.html"), "<!doctype html><title>app</title>")
	h := SPAHandler(dir)

	// Root and unknown deep links both fall back to index.html.
	for _, p := range []string{"/", "/files/deep/route"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, p, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("path %s status = %d", p, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "<title>app</title>") {
			t.Fatalf("path %s did not serve index.html: %s", p, rr.Body.String())
		}
		if cc := rr.Header().Get("Cache-Control"); cc != "no-cache" {
			t.Fatalf("path %s Cache-Control = %q", p, cc)
		}
	}
}

func TestSPAHandler_UnknownAPIReturns404JSON(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "index.html"), "<!doctype html><title>app</title>")
	h := SPAHandler(dir)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("unknown /api should return JSON, got Content-Type %q", ct)
	}
	if strings.Contains(rr.Body.String(), "<title>") {
		t.Fatalf("unknown /api must not serve the HTML shell")
	}
}

func TestNewRouter_ServesSPAWhenDistPresent(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "index.html"), "<!doctype html><title>spa</title>")

	h := NewRouter(Deps{Config: config.Config{WebDistDir: dir}})

	// "/" serves the SPA shell.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "spa") {
		t.Fatalf("SPA not served at /: status=%d body=%s", rr.Code, rr.Body.String())
	}
	// API routes still take precedence over the catch-all SPA handler.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "\"status\"") {
		t.Fatalf("health route broken by SPA handler: status=%d body=%s", rr.Code, rr.Body.String())
	}
}
