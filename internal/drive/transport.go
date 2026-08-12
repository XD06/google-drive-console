package drive

import (
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const userAgent = "drive-backup-console/1.0 (gzip)"

// gzipTransport wraps an HTTPDoer, injecting User-Agent header with "gzip"
// so Google Drive API returns gzip-compressed responses.
// Go's http.Transport automatically adds Accept-Encoding: gzip and
// transparently decompresses gzip when DisableCompression is false (default).
type gzipTransport struct {
	wrapped HTTPDoer
}

func (g *gzipTransport) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", userAgent)
	return g.wrapped.Do(req)
}

// wrapWithGzip wraps an HTTPDoer to add gzip-enabling headers.
func wrapWithGzip(hc HTTPDoer) HTTPDoer {
	return &gzipTransport{wrapped: hc}
}

// retryTransport wraps an HTTPDoer with automatic retry for transient errors.
// Only body-less idempotent methods (GET, DELETE, HEAD) are retried on 429/5xx.
// Requests carrying a body (PUT/PATCH/POST) are NOT retried here: the body
// reader is already consumed after the first attempt, so a blind re-send would
// transmit an empty body. Resumable chunk uploads (PUT) have their own
// resume-aware retry in UploadRange (see resumable.go), which queries the
// upload offset before resending — so transport-level retry must not double up.
type retryTransport struct {
	wrapped     HTTPDoer
	maxRetries  int
	baseBackoff time.Duration
}

func (t *retryTransport) Do(req *http.Request) (*http.Response, error) {
	// Only retry body-less idempotent methods. A non-nil Body means the request
	// carries payload whose reader cannot be safely rewound here, so we must not
	// retry it (that is delegated to UploadRange for resumable PUTs).
	retryable := req.Body == nil &&
		(req.Method == http.MethodGet ||
			req.Method == http.MethodDelete ||
			req.Method == http.MethodHead)

	var lastErr error
	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := t.baseBackoff * time.Duration(1<<uint(attempt-1))
			// Add jitter ±25%
			jitter := time.Duration(rand.Int63n(int64(backoff) / 4))
			select {
			case <-time.After(backoff + jitter):
			case <-req.Context().Done():
				if lastErr != nil {
					return nil, lastErr
				}
				return nil, req.Context().Err()
			}
		}

		resp, err := t.wrapped.Do(req)
		if err != nil {
			lastErr = err
			if !retryable {
				return nil, err
			}
			continue
		}

		// Retry on 429 and 5xx for idempotent methods
		if retryable && (resp.StatusCode == 429 || resp.StatusCode >= 500) {
			// Honor Retry-After header if present (capped at 60s)
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil {
					wait := time.Duration(secs) * time.Second
					if wait > 60*time.Second {
						wait = 60 * time.Second
					}
					select {
					case <-time.After(wait):
					case <-req.Context().Done():
						io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
						resp.Body.Close()
						return nil, req.Context().Err()
					}
				}
			}
			// Drain body to allow connection reuse (capped — misbehaving
			// upstreams must not pin unbounded memory on a discard).
			io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			lastErr = &APIError{Status: resp.StatusCode, Code: "transient_error", Message: "transient server error"}
			continue
		}
		return resp, nil
	}
	if lastErr == nil {
		lastErr = &APIError{Status: 503, Code: "max_retries_exceeded", Message: "max retries exceeded"}
	}
	log.Printf("retry exhausted after %d attempts: %v", t.maxRetries+1, lastErr)
	return nil, lastErr
}

// wrapWithRetry wraps an HTTPDoer with retry logic.
func wrapWithRetry(hc HTTPDoer) HTTPDoer {
	return &retryTransport{
		wrapped:     hc,
		maxRetries:  3,
		baseBackoff: time.Second,
	}
}
