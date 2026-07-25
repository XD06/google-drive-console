package drive

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// countingDoer is a mock HTTPDoer that counts calls and delegates to a handler.
type countingDoer struct {
	calls   int
	handler func(req *http.Request, call int) *http.Response
}

func (c *countingDoer) Do(req *http.Request) (*http.Response, error) {
	c.calls++
	return c.handler(req, c.calls), nil
}

func mkResp(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}
}

// GET is body-less and idempotent: transport should retry transient 5xx.
func TestRetryTransport_RetriesBodylessGET(t *testing.T) {
	doer := &countingDoer{handler: func(_ *http.Request, call int) *http.Response {
		if call < 3 {
			return mkResp(http.StatusServiceUnavailable)
		}
		return mkResp(http.StatusOK)
	}}
	rt := &retryTransport{wrapped: doer, maxRetries: 3, baseBackoff: time.Millisecond}

	req, _ := http.NewRequest(http.MethodGet, "http://example.test/files", nil)
	res, err := rt.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if doer.calls != 3 {
		t.Fatalf("calls = %d, want 3 (2 retries then success)", doer.calls)
	}
}

// PUT carries a body: transport must NOT retry it (that is UploadRange's job).
// Re-sending would transmit an already-consumed (empty) body.
func TestRetryTransport_DoesNotRetryBodyPUT(t *testing.T) {
	doer := &countingDoer{handler: func(_ *http.Request, _ int) *http.Response {
		return mkResp(http.StatusServiceUnavailable)
	}}
	rt := &retryTransport{wrapped: doer, maxRetries: 3, baseBackoff: time.Millisecond}

	req, _ := http.NewRequest(http.MethodPut, "http://example.test/session", strings.NewReader("chunk-bytes"))
	res, err := rt.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected the 503 response to be returned to caller for its own handling")
	}
	if doer.calls != 1 {
		t.Fatalf("calls = %d, want 1 (PUT with body must not be retried at transport level)", doer.calls)
	}
}

// PATCH with a body (e.g. text content update, revision restore) must not be retried either.
func TestRetryTransport_DoesNotRetryBodyPATCH(t *testing.T) {
	doer := &countingDoer{handler: func(_ *http.Request, _ int) *http.Response {
		return mkResp(http.StatusInternalServerError)
	}}
	rt := &retryTransport{wrapped: doer, maxRetries: 3, baseBackoff: time.Millisecond}

	req, _ := http.NewRequest(http.MethodPatch, "http://example.test/files/x", strings.NewReader("content"))
	_, err := rt.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doer.calls != 1 {
		t.Fatalf("calls = %d, want 1 (PATCH with body must not be retried)", doer.calls)
	}
}
