package drive

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "'root' in parents") {
			t.Fatalf("q = %q", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"files": [
				{"id":"f1","name":"Backups","mimeType":"application/vnd.google-apps.folder","modifiedTime":"2026-07-20T10:00:00Z"},
				{"id":"f2","name":"a.zip","mimeType":"application/zip","size":"1048576","modifiedTime":"2026-07-21T01:00:00Z"}
			]
		}`))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	res, err := c.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.FolderID != "root" || len(res.Items) != 2 {
		t.Fatalf("%+v", res)
	}
	if !res.Items[0].IsFolder || res.Items[0].Size != nil {
		t.Fatalf("folder item: %+v", res.Items[0])
	}
	if res.Items[1].IsFolder || res.Items[1].Size == nil || *res.Items[1].Size != 1048576 {
		t.Fatalf("file item: %+v", res.Items[1])
	}
}

func TestClient_Download(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/files/f2") && r.URL.Query().Get("alt") != "media":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"f2","name":"a.zip","mimeType":"application/zip","size":"3"}`))
		case strings.HasPrefix(r.URL.Path, "/files/f2") && r.URL.Query().Get("alt") == "media":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write([]byte("abc"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	meta, body, ct, err := c.Download(context.Background(), "f2")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	if meta.Name != "a.zip" || ct != "application/zip" {
		t.Fatalf("meta=%+v ct=%s", meta, ct)
	}
	b, _ := io.ReadAll(body)
	if string(b) != "abc" {
		t.Fatalf("body = %q", b)
	}
}

func TestClient_DownloadFolderRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"d1","name":"Dir","mimeType":"application/vnd.google-apps.folder"}`))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	_, _, _, err := c.Download(context.Background(), "d1")
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*APIError)
	if !ok || ae.Code != "is_folder" {
		t.Fatalf("err = %v", err)
	}
}

func TestClient_ListErrorMapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"insufficient permissions"}}`))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	_, err := c.List(context.Background(), ListOptions{FolderID: "x"})
	ae, ok := err.(*APIError)
	if !ok || ae.Status != 403 || ae.Code != "drive_forbidden" {
		t.Fatalf("err = %#v", err)
	}
}