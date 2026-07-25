package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/drive"
)

// Download with a full metadata hint (mime/name/size/md5 from the list
// response) must skip the GetMeta round-trip: exactly one alt=media call.
func TestFiles_Download_WithHintSkipsGetMeta(t *testing.T) {
	svc := testFilesAuthService(t)
	var metaCalls, mediaCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("alt") == "media" {
			atomic.AddInt32(&mediaCalls, 1)
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("PDFBYTES"))
			return
		}
		atomic.AddInt32(&metaCalls, 1)
		_, _ = w.Write([]byte(`{"id":"f1","name":"a.pdf","mimeType":"application/pdf","size":"8","md5Checksum":"abc123"}`))
	}))
	defer srv.Close()

	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: srv.Client(), BaseURL: srv.URL}, nil
		},
	}
	h := RequireSession(svc, http.HandlerFunc(fh.Download))
	url := "/api/files/f1/download?mime=application%2Fpdf&name=a.pdf&size=8&md5=abc123"
	req := withSession(t, svc, httptest.NewRequest(http.MethodGet, url, nil))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if got := atomic.LoadInt32(&mediaCalls); got != 1 {
		t.Fatalf("media calls = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&metaCalls); got != 0 {
		t.Fatalf("GetMeta calls = %d, want 0 (hint should skip GetMeta)", got)
	}
	if rr.Body.String() != "PDFBYTES" {
		t.Fatalf("body = %q", rr.Body.String())
	}
	if etag := rr.Header().Get("ETag"); etag != `"abc123"` {
		t.Fatalf("ETag = %q, want %q", etag, `"abc123"`)
	}
	if cc := rr.Header().Get("Cache-Control"); cc == "" {
		t.Fatal("Cache-Control missing")
	}
	if cl := rr.Header().Get("Content-Length"); cl != "8" {
		t.Fatalf("Content-Length = %q, want 8 (from hint)", cl)
	}
	cd := rr.Header().Get("Content-Disposition")
	if cd == "" || !strings.Contains(cd, "a.pdf") {
		t.Fatalf("Content-Disposition = %q, want filename a.pdf", cd)
	}
}

// Without hint params the handler must keep the legacy path: GetMeta + media.
func TestFiles_Download_NoHintStillFetchesMeta(t *testing.T) {
	svc := testFilesAuthService(t)
	var metaCalls, mediaCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("alt") == "media" {
			atomic.AddInt32(&mediaCalls, 1)
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("hi"))
			return
		}
		atomic.AddInt32(&metaCalls, 1)
		_, _ = w.Write([]byte(`{"id":"f1","name":"a.txt","mimeType":"text/plain","size":"2","md5Checksum":"m1"}`))
	}))
	defer srv.Close()

	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: srv.Client(), BaseURL: srv.URL}, nil
		},
	}
	h := RequireSession(svc, http.HandlerFunc(fh.Download))
	req := withSession(t, svc, httptest.NewRequest(http.MethodGet, "/api/files/f1/download", nil))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if got := atomic.LoadInt32(&metaCalls); got != 1 {
		t.Fatalf("GetMeta calls = %d, want 1 (no hint fallback)", got)
	}
	if got := atomic.LoadInt32(&mediaCalls); got != 1 {
		t.Fatalf("media calls = %d, want 1", got)
	}
}

// Range branch now emits cache headers too, and honours the hint path.
func TestFiles_DownloadRange_WithHintAndCacheHeaders(t *testing.T) {
	svc := testFilesAuthService(t)
	var metaCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("alt") == "media" {
			if r.Header.Get("Range") != "" {
				w.Header().Set("Content-Type", "video/mp4")
				w.Header().Set("Content-Range", "bytes 0-3/10")
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write([]byte("0123"))
				return
			}
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("0123456789"))
			return
		}
		atomic.AddInt32(&metaCalls, 1)
		_, _ = w.Write([]byte(`{"id":"v1","name":"v.mp4","mimeType":"video/mp4","size":"10"}`))
	}))
	defer srv.Close()

	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: srv.Client(), BaseURL: srv.URL}, nil
		},
	}
	h := RequireSession(svc, http.HandlerFunc(fh.Download))
	req := withSession(t, svc, httptest.NewRequest(http.MethodGet, "/api/files/v1/download?mime=video%2Fmp4&name=v.mp4&size=10&ver=7", nil))
	req.Header.Set("Range", "bytes=0-3")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", rr.Code)
	}
	if got := atomic.LoadInt32(&metaCalls); got != 0 {
		t.Fatalf("GetMeta calls = %d, want 0 (hint should skip GetMeta)", got)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "private, max-age=300" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	if etag := rr.Header().Get("ETag"); etag != `"7"` {
		t.Fatalf("ETag = %q, want version fallback %q", etag, `"7"`)
	}
	if cr := rr.Header().Get("Content-Range"); cr != "bytes 0-3/10" {
		t.Fatalf("Content-Range = %q", cr)
	}
	if rr.Body.String() != "0123" {
		t.Fatalf("body = %q", rr.Body.String())
	}
}
