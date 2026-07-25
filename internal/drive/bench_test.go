package drive

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// Benchmark: List / GetMeta metadata parsing latency
// ---------------------------------------------------------------------------

func makeFakeListJSON(n int) []byte {
	type file struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		MimeType     string `json:"mimeType"`
		Size         string `json:"size"`
		ModifiedTime string `json:"modifiedTime"`
	}
	files := make([]file, n)
	for i := range files {
		files[i] = file{
			ID:           fmt.Sprintf("id_%04d", i),
			Name:         fmt.Sprintf("file_%04d.zip", i),
			MimeType:     "application/zip",
			Size:         "1048576",
			ModifiedTime: "2026-01-01T00:00:00Z",
		}
	}
	out := struct {
		Files         []file `json:"files"`
		NextPageToken string `json:"nextPageToken"`
	}{Files: files, NextPageToken: "tok_next"}
	b, _ := json.Marshal(out)
	return b
}

func gzipBytes(data []byte) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write(data)
	gw.Close()
	return buf.Bytes()
}

// BenchmarkList_50files measures end-to-end latency for List with 50 files.
func BenchmarkList_50files(b *testing.B) {
	payload := makeFakeListJSON(50)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.List(ctx, ListOptions{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkList_100files measures List with 100 files.
func BenchmarkList_100files(b *testing.B) {
	payload := makeFakeListJSON(100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.List(ctx, ListOptions{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGetMeta measures per-file metadata fetch latency.
func BenchmarkGetMeta(b *testing.B) {
	payload := []byte(`{"id":"f1","name":"test.zip","mimeType":"application/zip","size":"1048576"}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.GetMeta(ctx, "f1")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Benchmark: Download throughput (io.Copy from body to writer)
// ---------------------------------------------------------------------------

// BenchmarkDownload_1MB measures download throughput for 1 MB file.
func BenchmarkDownload_1MB(b *testing.B) {
	benchmarkDownload(b, 1<<20)
}

// BenchmarkDownload_10MB measures download throughput for 10 MB file.
func BenchmarkDownload_10MB(b *testing.B) {
	benchmarkDownload(b, 10<<20)
}

// BenchmarkDownload_50MB measures download throughput for 50 MB file.
func BenchmarkDownload_50MB(b *testing.B) {
	benchmarkDownload(b, 50<<20)
}

func benchmarkDownload(b *testing.B, size int) {
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte(i & 0xff)
	}
	metaJSON := fmt.Sprintf(`{"id":"f1","name":"big.bin","mimeType":"application/octet-stream","size":"%d"}`, size)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("alt") == "media" {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(payload)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(metaJSON))
		}
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	ctx := context.Background()

	b.SetBytes(int64(size))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, body, _, err := c.Download(ctx, "f1")
		if err != nil {
			b.Fatal(err)
		}
		// Simulate reading body to a discard writer (as Download handler does)
		n, _ := io.Copy(io.Discard, body)
		body.Close()
		if n != int64(size) {
			b.Fatalf("read %d, want %d", n, size)
		}
	}
}

// ---------------------------------------------------------------------------
// Benchmark: UploadRange throughput (single chunk PUT)
// ---------------------------------------------------------------------------

// BenchmarkUploadRange_256KB benchmarks upload with current 256KB chunk size.
func BenchmarkUploadRange_256KB(b *testing.B) {
	benchmarkUploadRange(b, ChunkMultiple)
}

// BenchmarkUploadRange_1MB benchmarks upload with 1MB chunk size.
func BenchmarkUploadRange_1MB(b *testing.B) {
	benchmarkUploadRange(b, 1<<20)
}

// BenchmarkUploadRange_8MB benchmarks upload with 8MB chunk size.
func BenchmarkUploadRange_8MB(b *testing.B) {
	benchmarkUploadRange(b, 8<<20)
}

func benchmarkUploadRange(b *testing.B, chunkSize int) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain body to simulate real network transfer
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(308) // Resume Incomplete
		w.Header().Set("Range", fmt.Sprintf("bytes=0-%d", chunkSize-1))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL + "/drive/v3"}
	ctx := context.Background()
	data := make([]byte, chunkSize)
	totalSize := int64(100 << 20) // pretend 100 MB file

	b.SetBytes(int64(chunkSize))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.UploadRange(ctx, srv.URL+"/session/1", 0, int64(chunkSize-1), totalSize, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// Benchmark: gzip compression impact on List response
// ---------------------------------------------------------------------------

// BenchmarkList_100files_gzip measures List with gzip-compressed response.
func BenchmarkList_100files_gzip(b *testing.B) {
	payload := makeFakeListJSON(100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Encoding") == "gzip" || r.Header.Get("User-Agent") != "" {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			w.WriteHeader(200)
			gz.Write(payload)
			gz.Close()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	}))
	defer srv.Close()

	c := NewClient(srv.Client())
	c.BaseURL = srv.URL
	ctx := context.Background()

	// Measure compressed size vs uncompressed
	compressed := gzipBytes(payload)
	b.Logf("Uncompressed: %d bytes, Gzip: %d bytes (%.1f%% reduction)", len(payload), len(compressed), float64(len(payload)-len(compressed))/float64(len(payload))*100)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := c.List(ctx, ListOptions{})
		if err != nil {
			b.Fatal(err)
		}
	}
}
