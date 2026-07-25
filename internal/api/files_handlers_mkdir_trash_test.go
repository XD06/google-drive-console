package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/drive"
	"github.com/dsk/drive-backup-console/internal/config"
)

func TestFilesMkdir_Unauthorized(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		DevMode:       true,
		SessionSecret: "test-secret-for-files",
		TokenPath:     t.TempDir() + "/token.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	fh := &FilesHandlers{Auth: svc}
	h := RequireSession(svc, http.HandlerFunc(fh.Mkdir))
	req := httptest.NewRequest(http.MethodPost, "/api/files/mkdir", strings.NewReader(`{"name":"x"}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestFilesMkdir_OK(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		DevMode:       true,
		SessionSecret: "test-secret-for-files",
		TokenPath:     t.TempDir() + "/token.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	drvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"f3","name":"new folder","mimeType":"application/vnd.google-apps.folder","modifiedTime":"2026-07-21T00:00:00Z"}`))
	}))
	defer drvSrv.Close()

	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: drvSrv.Client(), BaseURL: drvSrv.URL}, nil
		},
	}
	h := RequireSession(svc, http.HandlerFunc(fh.Mkdir))
	req := withSession(t, svc, httptest.NewRequest(http.MethodPost, "/api/files/mkdir", strings.NewReader(`{"name":"new folder","parentId":"root"}`)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var fi drive.FileItem
	if err := json.Unmarshal(rr.Body.Bytes(), &fi); err != nil {
		t.Fatal(err)
	}
	if fi.ID != "f3" || fi.Name != "new folder" || !fi.IsFolder {
		t.Fatalf("unexpected file %+v", fi)
	}
}

func TestFilesTrash_OK(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		DevMode:       true,
		SessionSecret: "test-secret-for-files",
		TokenPath:     t.TempDir() + "/token.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	drvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer drvSrv.Close()

	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: drvSrv.Client(), BaseURL: drvSrv.URL}, nil
		},
	}
	mux := http.NewServeMux()
	mux.Handle("DELETE /api/files/{id}", RequireSession(svc, http.HandlerFunc(fh.Trash)))
	req := withSession(t, svc, httptest.NewRequest(http.MethodDelete, "/api/files/f4", nil))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestFilesTrash_Unauthorized(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		DevMode:       true,
		SessionSecret: "test-secret-for-files",
		TokenPath:     t.TempDir() + "/token.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	fh := &FilesHandlers{Auth: svc}
	h := RequireSession(svc, http.HandlerFunc(fh.Trash))
	req := httptest.NewRequest(http.MethodDelete, "/api/files/x", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestFilesRename_OK(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		DevMode:       true,
		SessionSecret: "test-secret-for-files",
		TokenPath:     t.TempDir() + "/token.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	drvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"f5","name":"renamed.md","mimeType":"text/markdown","size":"4","modifiedTime":"2026-07-21T00:00:00Z"}`))
	}))
	defer drvSrv.Close()

	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: drvSrv.Client(), BaseURL: drvSrv.URL}, nil
		},
	}
	mux := http.NewServeMux()
	mux.Handle("PATCH /api/files/{id}", RequireSession(svc, http.HandlerFunc(fh.Rename)))
	req := withSession(t, svc, httptest.NewRequest(http.MethodPatch, "/api/files/f5", strings.NewReader(`{"name":"renamed.md"}`)))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var fi drive.FileItem
	if err := json.Unmarshal(rr.Body.Bytes(), &fi); err != nil {
		t.Fatal(err)
	}
	if fi.ID != "f5" || fi.Name != "renamed.md" {
		t.Fatalf("unexpected file %+v", fi)
	}
}

func TestFilesRename_BadName(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		DevMode:       true,
		SessionSecret: "test-secret-for-files",
		TokenPath:     t.TempDir() + "/token.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: http.DefaultClient, BaseURL: "https://example.com"}, nil
		},
	}
	mux := http.NewServeMux()
	mux.Handle("PATCH /api/files/{id}", RequireSession(svc, http.HandlerFunc(fh.Rename)))
	req := withSession(t, svc, httptest.NewRequest(http.MethodPatch, "/api/files/f5", strings.NewReader(`{"name":"  "}`)))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestFilesMove_OK(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		DevMode:       true,
		SessionSecret: "test-secret-for-files",
		TokenPath:     t.TempDir() + "/token.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	drvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"parents":["old"]}`))
			return
		}
		if r.Method == http.MethodPatch {
			_, _ = w.Write([]byte(`{"id":"f6","name":"moved.txt","mimeType":"text/plain","modifiedTime":"2026-07-21T00:00:00Z"}`))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer drvSrv.Close()

	fh := &FilesHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: drvSrv.Client(), BaseURL: drvSrv.URL}, nil
		},
	}
	mux := http.NewServeMux()
	mux.Handle("POST /api/files/{id}/move", RequireSession(svc, http.HandlerFunc(fh.Move)))
	req := withSession(t, svc, httptest.NewRequest(http.MethodPost, "/api/files/f6/move", strings.NewReader(`{"parentId":"newParent"}`)))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var fi drive.FileItem
	if err := json.Unmarshal(rr.Body.Bytes(), &fi); err != nil {
		t.Fatal(err)
	}
	if fi.ID != "f6" || fi.Name != "moved.txt" {
		t.Fatalf("unexpected file %+v", fi)
	}
}

func TestFilesMove_Unauthorized(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		DevMode:       true,
		SessionSecret: "test-secret-for-files",
		TokenPath:     t.TempDir() + "/token.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	fh := &FilesHandlers{Auth: svc}
	h := RequireSession(svc, http.HandlerFunc(fh.Move))
	req := httptest.NewRequest(http.MethodPost, "/api/files/x/move", strings.NewReader(`{"parentId":"p"}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rr.Code)
	}
}
