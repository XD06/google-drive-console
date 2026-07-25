package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/drive"
)

func TestFilesSearch_OK(t *testing.T) {
	svc := testFilesAuthService(t)
	var gotQ string
	drvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQ = r.URL.Query().Get("q")
		_, _ = w.Write([]byte(`{"files":[{"id":"1","name":"backup.zip","mimeType":"application/zip","size":"10","modifiedTime":"2026-07-21T00:00:00Z"},{"id":"2","name":"backup","mimeType":"application/vnd.google-apps.folder","modifiedTime":"2026-07-20T00:00:00Z"}]}`))
	}))
	defer drvSrv.Close()

	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, s *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: drvSrv.Client(), BaseURL: drvSrv.URL}, nil
		},
	}
	h := RequireSession(svc, http.HandlerFunc(fh.Search))
	req := withSession(t, svc, httptest.NewRequest(http.MethodGet, "/api/files/search?q=backup&scope=drive", nil))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(gotQ, "name contains 'backup'") {
		t.Fatalf("drive q = %q", gotQ)
	}
	var body drive.SearchResult
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Query != "backup" || body.Scope != "drive" || len(body.Items) != 2 {
		t.Fatalf("%+v", body)
	}
	if body.Items[0].ID != "2" {
		t.Fatalf("rank: %+v", body.Items)
	}
}

func TestFilesSearch_FolderScope(t *testing.T) {
	svc := testFilesAuthService(t)
	var gotQ string
	drvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQ = r.URL.Query().Get("q")
		_, _ = w.Write([]byte(`{"files":[]}`))
	}))
	defer drvSrv.Close()

	fh := &FilesHandlers{
		Auth:          svc,
		DefaultFolder: "root",
		Drive: func(r *http.Request, s *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: drvSrv.Client(), BaseURL: drvSrv.URL}, nil
		},
	}
	h := RequireSession(svc, http.HandlerFunc(fh.Search))
	req := withSession(t, svc, httptest.NewRequest(http.MethodGet, "/api/files/search?q=ab&scope=folder&folderId=fld1", nil))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(gotQ, "'fld1' in parents") {
		t.Fatalf("drive q = %q", gotQ)
	}
}

func TestFilesSearch_ShortQuery(t *testing.T) {
	svc := testFilesAuthService(t)
	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, s *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: http.DefaultClient, BaseURL: "http://example.invalid"}, nil
		},
	}
	h := RequireSession(svc, http.HandlerFunc(fh.Search))
	req := withSession(t, svc, httptest.NewRequest(http.MethodGet, "/api/files/search?q=a", nil))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestFilesSearch_Unauthorized(t *testing.T) {
	svc := testFilesAuthService(t)
	h := RequireSession(svc, http.HandlerFunc((&FilesHandlers{Auth: svc}).Search))
	req := httptest.NewRequest(http.MethodGet, "/api/files/search?q=ab", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rr.Code)
	}
}

