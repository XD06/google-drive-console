package drive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_Search_DriveScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "name contains 'report'") {
			t.Fatalf("q = %q", q)
		}
		if strings.Contains(q, "in parents") {
			t.Fatalf("drive scope should not constrain parents: %q", q)
		}
		if !strings.Contains(q, "trashed = false") {
			t.Fatalf("q = %q", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"files": [
				{"id":"1","name":"x report y","mimeType":"text/plain","size":"1","modifiedTime":"2026-07-21T00:00:00Z"},
				{"id":"2","name":"report","mimeType":"application/vnd.google-apps.folder","modifiedTime":"2026-07-20T00:00:00Z"},
				{"id":"3","name":"report-final.pdf","mimeType":"application/pdf","size":"2","modifiedTime":"2026-07-22T00:00:00Z"}
			]
		}`))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	res, err := c.Search(context.Background(), SearchOptions{Query: "report", Scope: SearchScopeDrive})
	if err != nil {
		t.Fatal(err)
	}
	if res.Query != "report" || res.Scope != "drive" || len(res.Items) != 3 {
		t.Fatalf("%+v", res)
	}
	// exact folder "report" should rank first
	if res.Items[0].ID != "2" || !res.Items[0].IsFolder {
		t.Fatalf("expected exact folder first, got %+v", res.Items[0])
	}
	// prefix before contains
	if res.Items[1].ID != "3" {
		t.Fatalf("expected prefix next, got %+v", res.Items[1])
	}
	if res.Items[2].ID != "1" {
		t.Fatalf("expected contains last, got %+v", res.Items[2])
	}
}

func TestClient_Search_FolderScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "'abc' in parents") {
			t.Fatalf("q = %q", q)
		}
		if !strings.Contains(q, "name contains 'ab'") {
			t.Fatalf("q = %q", q)
		}
		_, _ = w.Write([]byte(`{"files":[{"id":"f","name":"ab","mimeType":"text/plain","size":"1","modifiedTime":"2026-07-21T00:00:00Z"}]}`))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	res, err := c.Search(context.Background(), SearchOptions{
		Query:    "ab",
		Scope:    SearchScopeFolder,
		FolderID: "abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.FolderID != "abc" || res.Scope != "folder" || len(res.Items) != 1 {
		t.Fatalf("%+v", res)
	}
}

func TestClient_Search_EscapesQuote(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, `name contains 'o\'reilly'`) {
			t.Fatalf("q = %q", q)
		}
		_, _ = w.Write([]byte(`{"files":[]}`))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	_, err := c.Search(context.Background(), SearchOptions{Query: "o'reilly", Scope: SearchScopeDrive})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClient_Search_ShortQuery(t *testing.T) {
	c := &Client{HTTP: http.DefaultClient, BaseURL: "http://example.invalid"}
	_, err := c.Search(context.Background(), SearchOptions{Query: "a"})
	ae, ok := err.(*APIError)
	if !ok || ae.Code != "bad_request" {
		t.Fatalf("err = %#v", err)
	}
}

func TestRankSearchItems(t *testing.T) {
	items := []FileItem{
		{ID: "c", Name: "zz note", IsFolder: false},
		{ID: "b", Name: "note.txt", IsFolder: false},
		{ID: "a", Name: "note", IsFolder: true},
	}
	rankSearchItems(items, "note")
	if items[0].ID != "a" || items[1].ID != "b" || items[2].ID != "c" {
		t.Fatalf("%+v", items)
	}
}
