package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/config"
	"github.com/dsk/drive-backup-console/internal/drive"
)

func testFilesAuthService(t *testing.T) *auth.Service {
	t.Helper()
	svc, err := auth.NewService(config.Config{
		DevMode:       true,
		SessionSecret: "test-secret-for-files",
		TokenPath:     t.TempDir() + "/token.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func withSession(t *testing.T, svc *auth.Service, req *http.Request) *http.Request {
	t.Helper()
	sess := auth.Session{Email: "u@example.com", Expires: time.Now().UTC().Add(time.Hour)}
	val, err := svc.Sessions.Encode(sess)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: "dbc_session", Value: val})
	return req
}

func TestFilesList_Unauthorized(t *testing.T) {
	svc := testFilesAuthService(t)
	h := RequireSession(svc, http.HandlerFunc((&FilesHandlers{Auth: svc}).List))
	req := httptest.NewRequest(http.MethodGet, "/api/files", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestFilesList_OK(t *testing.T) {
	svc := testFilesAuthService(t)
	drvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"files":[{"id":"1","name":"a.zip","mimeType":"application/zip","size":"10","modifiedTime":"2026-07-21T00:00:00Z"}]}`))
	}))
	defer drvSrv.Close()

	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: drvSrv.Client(), BaseURL: drvSrv.URL}, nil
		},
	}
	h := RequireSession(svc, http.HandlerFunc(fh.List))
	req := withSession(t, svc, httptest.NewRequest(http.MethodGet, "/api/files?folderId=root", nil))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body drive.ListResult
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.FolderID != "root" || len(body.Items) != 1 || body.Items[0].Name != "a.zip" {
		t.Fatalf("%+v", body)
	}
}

func TestFilesDownload_OK(t *testing.T) {
	svc := testFilesAuthService(t)
	drvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "alt=media") {
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write([]byte("xyz"))
			return
		}
		_, _ = w.Write([]byte(`{"id":"f2","name":"pack.zip","mimeType":"application/zip","size":"3"}`))
	}))
	defer drvSrv.Close()

	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: drvSrv.Client(), BaseURL: drvSrv.URL}, nil
		},
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api/files/{id}/download", RequireSession(svc, http.HandlerFunc(fh.Download)))
	req := withSession(t, svc, httptest.NewRequest(http.MethodGet, "/api/files/f2/download", nil))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "xyz" {
		t.Fatalf("body = %q", rr.Body.String())
	}
	cd := rr.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "pack.zip") {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	_ = context.Background()
	_ = io.EOF
}

func TestFilesList_DriveForbidden(t *testing.T) {
	svc := testFilesAuthService(t)
	drvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"no access"}}`))
	}))
	defer drvSrv.Close()
	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: drvSrv.Client(), BaseURL: drvSrv.URL}, nil
		},
	}
	h := RequireSession(svc, http.HandlerFunc(fh.List))
	req := withSession(t, svc, httptest.NewRequest(http.MethodGet, "/api/files", nil))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}