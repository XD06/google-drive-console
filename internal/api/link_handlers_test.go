package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/drive"
	"github.com/dsk/drive-backup-console/internal/share"
)

// newLinkTestHarness builds a LinkHandlers against a fake Drive upstream.
// The upstream serves metadata for f1 and echoes fixed media bytes.
func newLinkTestHarness(t *testing.T) (*auth.Service, *LinkHandlers, string) {
	t.Helper()
	svc := testFilesAuthService(t)
	media := "hello-bytes"
	drvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("alt") == "media" {
			if r.Header.Get("Range") != "" {
				w.Header().Set("Content-Type", "video/mp4")
				w.Header().Set("Content-Range", "bytes 0-4/11")
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write([]byte(media[:5]))
				return
			}
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte(media))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"f1","name":"clip.mp4","mimeType":"video/mp4","size":"11","md5Checksum":"md5-1","version":"3"}`))
	}))
	t.Cleanup(drvSrv.Close)

	links := share.New(filepath.Join(t.TempDir(), "links.json"))
	lh := &LinkHandlers{
		Auth:  svc,
		Links: links,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: drvSrv.Client(), BaseURL: drvSrv.URL}, nil
		},
	}
	return svc, lh, media
}

func TestLinkCreate_List_Revoke(t *testing.T) {
	svc, lh, _ := newLinkTestHarness(t)

	mux := http.NewServeMux()
	mux.Handle("POST /api/files/{id}/link", RequireSession(svc, http.HandlerFunc(lh.Create)))
	mux.Handle("GET /api/files/{id}/links", RequireSession(svc, http.HandlerFunc(lh.ListByFile)))
	mux.Handle("DELETE /api/links/{token}", RequireSession(svc, http.HandlerFunc(lh.Revoke)))

	// Create (session required).
	req := withSession(t, svc, httptest.NewRequest(http.MethodPost, "/api/files/f1/link", nil))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rr.Code, rr.Body.String())
	}
	var created linkJSON
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Token == "" || !strings.Contains(created.URL, "/d/"+created.Token) {
		t.Fatalf("unexpected link payload: %+v", created)
	}
	if created.URL[:4] != "http" {
		t.Fatalf("url not absolute: %s", created.URL)
	}

	// Idempotent create returns the same token.
	req2 := withSession(t, svc, httptest.NewRequest(http.MethodPost, "/api/files/f1/link", nil))
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)
	var again linkJSON
	_ = json.Unmarshal(rr2.Body.Bytes(), &again)
	if again.Token != created.Token {
		t.Fatalf("second create returned different token %s want %s", again.Token, created.Token)
	}

	// ListByFile finds it.
	req3 := withSession(t, svc, httptest.NewRequest(http.MethodGet, "/api/files/f1/links", nil))
	rr3 := httptest.NewRecorder()
	mux.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusOK {
		t.Fatalf("list status = %d", rr3.Code)
	}
	var listed struct {
		Links []linkJSON `json:"links"`
	}
	if err := json.Unmarshal(rr3.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Links) != 1 || listed.Links[0].Token != created.Token {
		t.Fatalf("list = %+v", listed)
	}

	// Revoke (idempotent), then the store must be empty.
	req4 := withSession(t, svc, httptest.NewRequest(http.MethodDelete, "/api/links/"+created.Token, nil))
	rr4 := httptest.NewRecorder()
	mux.ServeHTTP(rr4, req4)
	if rr4.Code != http.StatusOK {
		t.Fatalf("revoke status = %d", rr4.Code)
	}
	if links := lh.Links.ListByFile("f1"); len(links) != 0 {
		t.Fatalf("link survived revoke")
	}
}

func TestLinkStream_Public(t *testing.T) {
	_, lh, media := newLinkTestHarness(t)

	link, err := lh.Links.Create("f1", "clip.mp4")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /d/{token}", http.HandlerFunc(lh.Stream))

	// No session cookie — the token is the credential.
	req := httptest.NewRequest(http.MethodGet, "/d/"+link.Token, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("stream status = %d body=%s", rr.Code, rr.Body.String())
	}
	b, _ := io.ReadAll(rr.Body)
	if string(b) != media {
		t.Fatalf("stream body = %q", string(b))
	}
	if got := rr.Header().Get("Content-Type"); got != "video/mp4" {
		t.Fatalf("content type = %q", got)
	}
	if cd := rr.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "inline") {
		t.Fatalf("content disposition = %q, want inline", cd)
	}
	if ar := rr.Header().Get("Accept-Ranges"); ar != "bytes" {
		t.Fatalf("accept-ranges = %q", ar)
	}

	// Range request → 206 with upstream Content-Range passed through.
	reqR := httptest.NewRequest(http.MethodGet, "/d/"+link.Token, nil)
	reqR.Header.Set("Range", "bytes=0-4")
	rrR := httptest.NewRecorder()
	mux.ServeHTTP(rrR, reqR)
	if rrR.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d", rrR.Code)
	}
	if cr := rrR.Header().Get("Content-Range"); cr != "bytes 0-4/11" {
		t.Fatalf("content-range = %q", cr)
	}
	rb, _ := io.ReadAll(rrR.Body)
	if string(rb) != media[:5] {
		t.Fatalf("range body = %q", string(rb))
	}

	// Unknown token → 404 JSON error.
	reqBad := httptest.NewRequest(http.MethodGet, "/d/deadbeef", nil)
	rrBad := httptest.NewRecorder()
	mux.ServeHTTP(rrBad, reqBad)
	if rrBad.Code != http.StatusNotFound {
		t.Fatalf("unknown token status = %d", rrBad.Code)
	}
}

func TestLinkStream_RevokedAfterCreate(t *testing.T) {
	_, lh, _ := newLinkTestHarness(t)
	link, err := lh.Links.Create("f1", "clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lh.Links.Revoke(link.Token); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /d/{token}", http.HandlerFunc(lh.Stream))
	req := httptest.NewRequest(http.MethodGet, "/d/"+link.Token, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("revoked link status = %d, want 404", rr.Code)
	}
}

func TestAbsoluteBaseURL_UsesForwardedProto(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Host = "files.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	got := absoluteBaseURL(req)
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "https" || u.Host != "files.example.com" {
		t.Fatalf("absoluteBaseURL = %q", got)
	}
}
