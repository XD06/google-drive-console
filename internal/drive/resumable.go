package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ChunkMultiple is the required intermediate chunk size for Drive resumable uploads (256 KiB).
const ChunkMultiple = 256 * 1024

// DefaultFlushSize is the recommended minimum bytes to flush per UploadRange call.
// Larger chunks reduce HTTP round-trips at the cost of memory.
// Must be a multiple of ChunkMultiple. 16 MiB balances throughput and memory.
const DefaultFlushSize = 16 * 1024 * 1024

// ResumableStart starts a Drive resumable upload session and returns the session URI.
func (c *Client) ResumableStart(ctx context.Context, name, mimeType, parentID string, size int64) (sessionURL string, err error) {
	if strings.TrimSpace(name) == "" {
		return "", &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "name required"}
	}
	if size < 0 {
		return "", &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "size must be >= 0"}
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	meta := map[string]any{
		"name":     name,
		"mimeType": mimeType,
	}
	if p := strings.TrimSpace(parentID); p != "" && p != "root" {
		meta["parents"] = []string{p}
	}

	raw, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	u := c.uploadBase() + "/files?uploadType=resumable&supportsAllDrives=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("X-Upload-Content-Type", mimeType)
	req.Header.Set("X-Upload-Content-Length", strconv.FormatInt(size, 10))

	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return "", mapDriveError(res.StatusCode, body)
	}
	loc := res.Header.Get("Location")
	if loc == "" {
		return "", &APIError{Status: http.StatusBadGateway, Code: "drive_error", Message: "missing Location for resumable session"}
	}
	return loc, nil
}

// UploadChunkResult is the outcome of putting a content range to a resumable session.
type UploadChunkResult struct {
	Complete bool
	FileID   string
	NextByte int64
}

// maxRetries is the maximum number of retry attempts for transient errors.
const maxRetries = 5

// UploadRange PUTs bytes for [start, end] inclusive against total size to sessionURL.
// It implements exponential backoff retry for transient errors (5xx, 429, network).
// On retry, it queries Drive's upload status to resume from the correct offset.
func (c *Client) UploadRange(ctx context.Context, sessionURL string, start, end, total int64, data []byte) (UploadChunkResult, error) {
	if sessionURL == "" {
		return UploadChunkResult{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "session URL required"}
	}
	if end < start || int64(len(data)) != (end-start+1) {
		return UploadChunkResult{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "data length must match Content-Range"}
	}

	curStart := start
	curData := data
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s, 8s, 16s (with jitter)
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			jitter := time.Duration(rand.Int63n(int64(backoff) / 2))
			select {
			case <-time.After(backoff + jitter):
			case <-ctx.Done():
				return UploadChunkResult{}, ctx.Err()
			}

			// Query Drive's upload status to find the correct resume offset.
			status, qErr := c.QueryUploadStatus(ctx, sessionURL, total)
			if qErr == nil && !status.Complete && status.NextByte > curStart {
				// Drive received partial bytes; adjust offset.
			offset := status.NextByte - curStart
				if offset < int64(len(curData)) {
					curStart = status.NextByte
					curData = curData[offset:]
				}
			}
			// If query failed or NextByte <= curStart, resend entire chunk.
		}

		curEnd := curStart + int64(len(curData)) - 1
		res, err := c.uploadRangeOnce(ctx, sessionURL, curStart, curEnd, total, curData)
		if err == nil {
			return res, nil
		}
		if !isRetryableError(err) {
			return res, err
		}
		lastErr = err
	}
	return UploadChunkResult{}, fmt.Errorf("upload failed after %d retries: %w", maxRetries, lastErr)
}

// uploadRangeOnce performs a single PUT without retry logic.
func (c *Client) uploadRangeOnce(ctx context.Context, sessionURL string, start, end, total int64, data []byte) (UploadChunkResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, sessionURL, bytes.NewReader(data))
	if err != nil {
		return UploadChunkResult{}, err
	}
	req.Header.Set("Content-Length", strconv.Itoa(len(data)))
	req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
	req.ContentLength = int64(len(data))

	res, err := c.HTTP.Do(req)
	if err != nil {
		return UploadChunkResult{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))

	switch res.StatusCode {
	case http.StatusOK, http.StatusCreated:
		var meta struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(body, &meta)
		return UploadChunkResult{Complete: true, FileID: meta.ID, NextByte: total}, nil
	case 308: // Resume Incomplete (Drive)
		next := parseRangeNext(res.Header.Get("Range"))
		if next < 0 {
			next = start // No Range header = 0 bytes received, must resend entire chunk
		}
		return UploadChunkResult{Complete: false, NextByte: next}, nil
	default:
		return UploadChunkResult{}, mapDriveError(res.StatusCode, body)
	}
}

// isRetryableError reports whether the error is transient and worth retrying.
func isRetryableError(err error) bool {
	var ae *APIError
	if errors.As(err, &ae) {
		switch ae.Status {
		case http.StatusRequestTimeout, // 408
				http.StatusTooManyRequests,       // 429
				http.StatusInternalServerError,    // 500
				http.StatusBadGateway,             // 502
				http.StatusServiceUnavailable,     // 503
				http.StatusGatewayTimeout:         // 504
			return true
		}
		return false
	}
	// Network errors (no HTTP response) are always retryable.
	return true
}

func parseRangeNext(rangeHeader string) int64 {
	// Range: bytes=0-524287 -> next is 524288
	rangeHeader = strings.TrimSpace(rangeHeader)
	if rangeHeader == "" {
		return -1
	}
	j := strings.LastIndex(rangeHeader, "-")
	if j < 0 {
		return -1
	}
	endStr := strings.TrimSpace(rangeHeader[j+1:])
	n, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		return -1
	}
	return n + 1
}

func (c *Client) uploadBase() string {
	base := c.base()
	if strings.Contains(base, "/upload/drive/") {
		return base
	}
	if u, err := url.Parse(base); err == nil {
		if strings.HasPrefix(u.Path, "/drive/") {
			u.Path = "/upload" + u.Path
			return strings.TrimRight(u.String(), "/")
		}
	}
	return "https://www.googleapis.com/upload/drive/v3"
}

// UploadEmpty completes a zero-byte resumable session (Content-Range: bytes */0).
func (c *Client) UploadEmpty(ctx context.Context, sessionURL string) (UploadChunkResult, error) {
	if sessionURL == "" {
		return UploadChunkResult{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "session URL required"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, sessionURL, http.NoBody)
	if err != nil {
		return UploadChunkResult{}, err
	}
	req.Header.Set("Content-Length", "0")
	req.Header.Set("Content-Range", "bytes */0")
	req.ContentLength = 0
	res, err := c.HTTP.Do(req)
	if err != nil {
		return UploadChunkResult{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	switch res.StatusCode {
	case http.StatusOK, http.StatusCreated:
		var meta struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(body, &meta)
		return UploadChunkResult{Complete: true, FileID: meta.ID, NextByte: 0}, nil
	default:
		return UploadChunkResult{}, mapDriveError(res.StatusCode, body)
	}
}

// QueryUploadStatus sends an empty PUT with Content-Range: bytes */{total}
// to check the resumable upload session status. This is the standard Drive
// resumable upload protocol way to query how many bytes Drive has received.
// Returns (complete, fileID, nextByte, error).
func (c *Client) QueryUploadStatus(ctx context.Context, sessionURL string, total int64) (UploadChunkResult, error) {
	if sessionURL == "" {
		return UploadChunkResult{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "session URL required"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, sessionURL, http.NoBody)
	if err != nil {
		return UploadChunkResult{}, err
	}
	req.Header.Set("Content-Length", "0")
	req.Header.Set("Content-Range", fmt.Sprintf("bytes */%d", total))
	req.ContentLength = 0
	res, err := c.HTTP.Do(req)
	if err != nil {
		return UploadChunkResult{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	switch res.StatusCode {
	case http.StatusOK, http.StatusCreated:
		var meta struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(body, &meta)
		return UploadChunkResult{Complete: true, FileID: meta.ID, NextByte: total}, nil
	case 308:
		next := parseRangeNext(res.Header.Get("Range"))
		if next < 0 {
			next = 0 // 308 without Range means 0 bytes received
		}
		return UploadChunkResult{Complete: false, NextByte: next}, nil
	default:
		return UploadChunkResult{}, mapDriveError(res.StatusCode, body)
	}
}