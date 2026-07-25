package drive

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateFolder_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/files" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("supportsAllDrives") != "true" {
			t.Fatalf("supportsAllDrives missing")
		}
		b, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		_ = json.Unmarshal(b, &reqBody)
		if reqBody["name"] != "New Folder" {
			t.Fatalf("name = %v", reqBody["name"])
		}
		if reqBody["mimeType"] != FolderMIME {
			t.Fatalf("mimeType = %v", reqBody["mimeType"])
		}
		parents, ok := reqBody["parents"].([]interface{})
		if !ok || len(parents) != 1 || parents[0] != "p1" {
			t.Fatalf("parents = %v", reqBody["parents"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"f123","name":"New Folder","mimeType":"application/vnd.google-apps.folder","modifiedTime":"2026-07-21T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	item, err := c.CreateFolder(context.Background(), "New Folder", "p1")
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != "f123" || !item.IsFolder {
		t.Fatalf("item = %+v", item)
	}
}

func TestTrash_Success(t *testing.T) {
	targetID := "x123"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/files/"+targetID {
			t.Fatalf("path = %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		var reqBody map[string]bool
		_ = json.Unmarshal(b, &reqBody)
		if val, ok := reqBody["trashed"]; !ok || !val {
			t.Fatalf("trashed body = %v", reqBody)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	if err := c.Trash(context.Background(), targetID); err != nil {
		t.Fatal(err)
	}
}

func TestCreateFolder_EmptyName(t *testing.T) {
	c := &Client{HTTP: http.DefaultClient, BaseURL: "https://example.com"}
	_, err := c.CreateFolder(context.Background(), "   ", "root")
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*APIError)
	if !ok || ae.Status != http.StatusBadRequest || ae.Code != "bad_request" {
		t.Fatalf("err = %v", err)
	}
}

func TestRename_Success(t *testing.T) {
	targetID := "r1"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/files/"+targetID {
			t.Fatalf("path = %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		var reqBody map[string]string
		_ = json.Unmarshal(b, &reqBody)
		if reqBody["name"] != "renamed.txt" {
			t.Fatalf("name = %v", reqBody["name"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"r1","name":"renamed.txt","mimeType":"text/plain","size":"3","modifiedTime":"2026-07-21T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	item, err := c.Rename(context.Background(), targetID, "renamed.txt")
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != "r1" || item.Name != "renamed.txt" || item.IsFolder {
		t.Fatalf("item = %+v", item)
	}
}

func TestRename_EmptyName(t *testing.T) {
	c := &Client{HTTP: http.DefaultClient, BaseURL: "https://example.com"}
	_, err := c.Rename(context.Background(), "x", "  ")
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*APIError)
	if !ok || ae.Code != "bad_request" {
		t.Fatalf("err = %v", err)
	}
}

func TestMove_Success(t *testing.T) {
	targetID := "m1"
	var sawGet, sawPatch bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/"+targetID {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Method == http.MethodGet {
			sawGet = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"parents":["oldParent"]}`))
			return
		}
		if r.Method == http.MethodPatch {
			sawPatch = true
			q := r.URL.Query()
			if q.Get("addParents") != "newParent" {
				t.Fatalf("addParents = %s", q.Get("addParents"))
			}
			if q.Get("removeParents") != "oldParent" {
				t.Fatalf("removeParents = %s", q.Get("removeParents"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"m1","name":"doc.txt","mimeType":"text/plain","modifiedTime":"2026-07-21T00:00:00Z"}`))
			return
		}
		t.Fatalf("unexpected method %s", r.Method)
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	item, err := c.Move(context.Background(), targetID, "newParent", "")
	if err != nil {
		t.Fatal(err)
	}
	if !sawGet || !sawPatch {
		t.Fatalf("sawGet=%v sawPatch=%v", sawGet, sawPatch)
	}
	if item.ID != "m1" || item.Name != "doc.txt" {
		t.Fatalf("item = %+v", item)
	}
}

func TestMove_EmptyParent(t *testing.T) {
	c := &Client{HTTP: http.DefaultClient, BaseURL: "https://example.com"}
	_, err := c.Move(context.Background(), "x", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
}
