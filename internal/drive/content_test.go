package drive

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestIsTextEditableMIME(t *testing.T) {
	cases := []struct{m, n string; ok bool}{
		{"text/plain", "a.txt", true},
		{"application/json", "a.json", true},
		{"", "README.md", true},
		{"application/octet-stream", "a.bin", false},
		{"application/octet-stream", "notes.md", true},
		{"image/png", "a.png", false},
	}
	for _, c := range cases {
		if IsTextEditableMIME(c.m, c.n) != c.ok {
			t.Fatalf("IsTextEditableMIME(%q,%q) = %v, want %v", c.m, c.n, !c.ok, c.ok)
		}
	}
}

func TestClient_GetTextContent_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("alt") == "media" {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("hello world"))
			return
		}
		// metadata
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"f1","name":"a.txt","mimeType":"text/plain","size":"11"}`))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	txt, err := c.GetTextContent(context.Background(), "f1")
	if err != nil {
		t.Fatal(err)
	}
	if txt.Content != "hello world" || txt.MimeType != "text/plain" || txt.Size != 11 {
		t.Fatalf("got %+v", txt)
	}
}

func TestClient_GetTextContent_TooLarge(t *testing.T) {
	// metadata with huge size
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("alt") == "media" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("x"))
			return
		}
		// metadata with huge size
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"f2","name":"big.txt","mimeType":"text/plain","size":"` + strconv.FormatInt(MaxTextContent+1, 10) + `"}`))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	_, err := c.GetTextContent(context.Background(), "f2")
	if err == nil {
		t.Fatal("expected error")
	}
	if ae, ok := err.(*APIError); !ok || ae.Code != "content_too_large" {
		t.Fatalf("err = %v", err)
	}
}

func TestClient_UpdateTextContent_OK(t *testing.T) {
	received := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("alt") == "media" {
			// not used here
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodGet {
			// metadata
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"f3","name":"a.txt","mimeType":"text/plain","size":"3"}`))
			return
		}
		if r.Method == http.MethodPatch {
			b, _ := io.ReadAll(r.Body)
			received = string(b)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	err := c.UpdateTextContent(context.Background(), "f3", "new content")
	if err != nil {
		t.Fatal(err)
	}
	if received != "new content" {
		t.Fatalf("received = %q", received)
	}
}
