package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/drive"
)

func TestFiles_GetContent_OK(t *testing.T) {
	svc := testFilesAuthService(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("alt") == "media" {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("hi"))
			return
		}
		_, _ = w.Write([]byte(`{"id":"f1","name":"a.txt","mimeType":"text/plain","size":"2"}`))
	}))
	defer srv.Close()

	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: srv.Client(), BaseURL: srv.URL}, nil
		},
	}
	h := RequireSession(svc, http.HandlerFunc(fh.GetContent))
	req := withSession(t, svc, httptest.NewRequest(http.MethodGet, "/api/files/f1/content", nil))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["content"] != "hi" || body["mimeType"] != "text/plain" {
		t.Fatalf("body = %+v", body)
	}
}

func TestFiles_PutContent_OK(t *testing.T) {
	svc := testFilesAuthService(t)
	received := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("uploadType") == "media" {
			b, _ := io.ReadAll(r.Body)
			received = string(b)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		// meta request
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"f2","name":"a.txt","mimeType":"text/plain","size":"2"}`))
	}))
	defer srv.Close()

	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: srv.Client(), BaseURL: srv.URL}, nil
		},
	}
	h := RequireSession(svc, http.HandlerFunc(fh.PutContent))
	body := strings.NewReader(`{"content":"hello from api"}`)
	req := withSession(t, svc, httptest.NewRequest(http.MethodPut, "/api/files/f2/content", body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if received != "hello from api" {
		t.Fatalf("received = %q", received)
	}
}

func TestFilesContent_Unauthorized(t *testing.T) {
	svc := testFilesAuthService(t)
	h := RequireSession(svc, http.HandlerFunc((&FilesHandlers{Auth: svc}).GetContent))
	req := httptest.NewRequest(http.MethodGet, "/api/files/f1/content", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rr.Code)
	}
}
