package drive

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResumableStart(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/files") {
			t.Fatalf("path %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.RawQuery, "uploadType=resumable") {
			t.Fatalf("query %s", r.URL.RawQuery)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Location", "http://example.test/session/1")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL + "/drive/v3"}
	loc, err := c.ResumableStart(context.Background(), "a.zip", "application/zip", "folder1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if loc != "http://example.test/session/1" {
		t.Fatalf("loc=%s", loc)
	}
	if !strings.Contains(string(gotBody), `"name":"a.zip"`) || !strings.Contains(string(gotBody), `"folder1"`) {
		t.Fatalf("body %s", gotBody)
	}
}

func TestUploadRangeComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Range") != "bytes 0-2/3" {
			t.Fatalf("range %s", r.Header.Get("Content-Range"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"file123"}`))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL + "/drive/v3"}
	res, err := c.UploadRange(context.Background(), srv.URL, 0, 2, 3, []byte("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Complete || res.FileID != "file123" {
		t.Fatalf("%+v", res)
	}
}

func TestUploadRangeIncomplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Range", "bytes=0-262143")
		w.WriteHeader(308)
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client()}
	data := make([]byte, ChunkMultiple)
	res, err := c.UploadRange(context.Background(), srv.URL, 0, int64(ChunkMultiple-1), int64(ChunkMultiple*2), data)
	if err != nil {
		t.Fatal(err)
	}
	if res.Complete || res.NextByte != ChunkMultiple {
		t.Fatalf("%+v", res)
	}
}

func TestParseRangeNext(t *testing.T) {
	if n := parseRangeNext("bytes=0-524287"); n != 524288 {
		t.Fatalf("got %d", n)
	}
}