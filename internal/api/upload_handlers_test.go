package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/drive"
	"github.com/dsk/drive-backup-console/internal/upload"
)

type stubDrive struct {
	session string
}

func (s *stubDrive) ResumableStart(ctx context.Context, name, mimeType, parentID string, size int64) (string, error) {
	return "http://session.test/1", nil
}
func (s *stubDrive) UploadRange(ctx context.Context, sessionURL string, start, end, total int64, data []byte) (drive.UploadChunkResult, error) {
	if end+1 >= total {
		return drive.UploadChunkResult{Complete: true, FileID: "f1", NextByte: total}, nil
	}
	return drive.UploadChunkResult{Complete: false, NextByte: end + 1}, nil
}
func (s *stubDrive) UploadEmpty(ctx context.Context, sessionURL string) (drive.UploadChunkResult, error) {
	return drive.UploadChunkResult{Complete: true, FileID: "e1"}, nil
}
func (s *stubDrive) QueryUploadStatus(ctx context.Context, sessionURL string, total int64) (drive.UploadChunkResult, error) {
	return drive.UploadChunkResult{Complete: true, FileID: "f1", NextByte: total}, nil
}

func TestUploads_Unauthorized(t *testing.T) {
	svc := testFilesAuthService(t)
	store := upload.NewStore()
	up := &upload.Service{Store: store, Drive: &stubDrive{}}
	h := NewRouter(Deps{
		Auth:    svc,
		Uploads: up,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/uploads", bytes.NewReader([]byte(`{"name":"a","size":1}`)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestUploads_CreateChunkStatus(t *testing.T) {
	svc := testFilesAuthService(t)
	up := &upload.Service{Store: upload.NewStore(), Drive: &stubDrive{}}
	h := NewRouter(Deps{Auth: svc, Uploads: up})

	// Create
	body := []byte(`{"name":"note.txt","size":5,"mimeType":"text/plain"}`)
	req := withSession(t, svc, httptest.NewRequest(http.MethodPost, "/api/uploads", bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rr.Code, rr.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id, _ := created["uploadId"].(string)
	if id == "" {
		t.Fatal("no id")
	}

	// Chunk
	req2 := withSession(t, svc, httptest.NewRequest(http.MethodPut, "/api/uploads/"+id+"/chunk", bytes.NewReader([]byte("hello"))))
	req2.Header.Set("X-Upload-Offset", "0")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("chunk %d %s", rr2.Code, rr2.Body.String())
	}
	var st map[string]any
	_ = json.Unmarshal(rr2.Body.Bytes(), &st)
	if st["status"] != "completed" {
		t.Fatalf("%v", st)
	}

	// Status
	req3 := withSession(t, svc, httptest.NewRequest(http.MethodGet, "/api/uploads/"+id, nil))
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Fatalf("status %d", rr3.Code)
	}
}

// ensure auth.Service type used
var _ = auth.Session{}