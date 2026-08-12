package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// HTTPDoer abstracts *http.Client for tests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client talks to Google Drive API v3 over HTTP.
type Client struct {
	HTTP    HTTPDoer
	BaseURL string // without trailing slash; default DefaultAPIBase
}

// NewClient builds a client. httpClient must attach OAuth credentials.
// Wraps with gzip transport for compressed responses and retry for transient errors.
func NewClient(httpClient HTTPDoer) *Client {
	return &Client{HTTP: wrapWithRetry(wrapWithGzip(httpClient)), BaseURL: DefaultAPIBase}
}

func (c *Client) base() string {
	if c.BaseURL == "" {
		return DefaultAPIBase
	}
	return strings.TrimRight(c.BaseURL, "/")
}

// ListOptions controls file listing.
type ListOptions struct {
	FolderID  string
	PageToken string
	PageSize  int
}

// List returns non-trashed children of folderID (default "root").
func (c *Client) List(ctx context.Context, opt ListOptions) (ListResult, error) {
	folderID := strings.TrimSpace(opt.FolderID)
	if folderID == "" {
		folderID = "root"
	}
	pageSize := opt.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	q := fmt.Sprintf("'%s' in parents and trashed = false", escapeDriveQuery(folderID))
	params := url.Values{}
	params.Set("q", q)
	params.Set("pageSize", strconv.Itoa(pageSize))
	params.Set("fields", "nextPageToken,files(id,name,mimeType,size,modifiedTime,thumbnailLink,description,md5Checksum,version)")
	params.Set("orderBy", "folder,name_natural")
	params.Set("supportsAllDrives", "true")
	params.Set("includeItemsFromAllDrives", "true")
	if opt.PageToken != "" {
		params.Set("pageToken", opt.PageToken)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/files?"+params.Encode(), nil)
	if err != nil {
		return ListResult{}, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return ListResult{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return ListResult{}, err
	}
	if res.StatusCode != http.StatusOK {
		return ListResult{}, mapDriveError(res.StatusCode, body)
	}

	var raw struct {
		NextPageToken string `json:"nextPageToken"`
		Files         []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			MimeType      string `json:"mimeType"`
			Size          string `json:"size"`
			ModifiedTime  string `json:"modifiedTime"`
			ThumbnailLink string `json:"thumbnailLink"`
			Description   string `json:"description"`
			Md5Checksum   string `json:"md5Checksum"`
			Version       string `json:"version"`
		} `json:"files"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ListResult{}, err
	}

	items := make([]FileItem, 0, len(raw.Files))
	for _, f := range raw.Files {
		item := FileItem{
			ID:           f.ID,
			Name:         f.Name,
			MimeType:     f.MimeType,
			ModifiedTime: f.ModifiedTime,
			IsFolder:     f.MimeType == FolderMIME,
			Description:  f.Description,
			ThumbnailURL: f.ThumbnailLink,
			Md5Checksum:  f.Md5Checksum,
			Version:      f.Version,
		}
		if f.Size != "" {
			if n, err := strconv.ParseInt(f.Size, 10, 64); err == nil {
				item.Size = &n
			}
		}
		items = append(items, item)
	}

	return ListResult{
		FolderID:      folderID,
		Items:         items,
		NextPageToken: raw.NextPageToken,
	}, nil
}

// FileMeta is metadata needed for download headers.
type FileMeta struct {
	ID            string
	Name          string
	MimeType      string
	Size          *int64
	Md5Checksum   string
	Version       string
	ThumbnailLink string
}

// GetMeta fetches file metadata by id.
func (c *Client) GetMeta(ctx context.Context, fileID string) (FileMeta, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return FileMeta{}, fmt.Errorf("file id required")
	}
	params := url.Values{}
	params.Set("fields", "id,name,mimeType,size,md5Checksum,version,thumbnailLink")
	params.Set("supportsAllDrives", "true")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/files/"+url.PathEscape(fileID)+"?"+params.Encode(), nil)
	if err != nil {
		return FileMeta{}, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return FileMeta{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return FileMeta{}, err
	}
	if res.StatusCode != http.StatusOK {
		return FileMeta{}, mapDriveError(res.StatusCode, body)
	}
	var raw struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		MimeType      string `json:"mimeType"`
		Size          string `json:"size"`
		Md5Checksum   string `json:"md5Checksum"`
		Version       string `json:"version"`
		ThumbnailLink string `json:"thumbnailLink"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return FileMeta{}, err
	}
	meta := FileMeta{ID: raw.ID, Name: raw.Name, MimeType: raw.MimeType, Md5Checksum: raw.Md5Checksum, Version: raw.Version, ThumbnailLink: raw.ThumbnailLink}
	if raw.Size != "" {
		if n, err := strconv.ParseInt(raw.Size, 10, 64); err == nil {
			meta.Size = &n
		}
	}
	return meta, nil
}

// Download opens a media stream for a binary file (not Google Docs native types).
// Caller must close the body.
func (c *Client) Download(ctx context.Context, fileID string) (meta FileMeta, body io.ReadCloser, contentType string, err error) {
	meta, err = c.GetMeta(ctx, fileID)
	if err != nil {
		return FileMeta{}, nil, "", err
	}
	m, b, ct, _, _, err := c.downloadWithMeta(ctx, meta, "")
	return m, b, ct, err
}

// DownloadMedia is Download with caller-supplied metadata (from a prior List),
// skipping the GetMeta round-trip. meta.ID must be set; Name/MimeType/Size/
// Md5Checksum/Version are used for response headers and guards.
func (c *Client) DownloadMedia(ctx context.Context, meta FileMeta) (FileMeta, io.ReadCloser, string, error) {
	m, body, ct, _, _, err := c.downloadWithMeta(ctx, meta, "")
	return m, body, ct, err
}

// downloadWithMeta performs the alt=media GET. rangeHeader is optional
// (e.g. "bytes=0-1023"); when set, both 200 and 206 are accepted.
// Also returns the upstream Content-Range header and status code.
func (c *Client) downloadWithMeta(ctx context.Context, meta FileMeta, rangeHeader string) (FileMeta, io.ReadCloser, string, string, int, error) {
	fileID := strings.TrimSpace(meta.ID)
	if fileID == "" {
		return FileMeta{}, nil, "", "", 0, fmt.Errorf("file id required")
	}
	if meta.MimeType == FolderMIME {
		return FileMeta{}, nil, "", "", 0, &APIError{Status: http.StatusBadRequest, Code: "is_folder", Message: "Cannot download a folder"}
	}
	if strings.HasPrefix(meta.MimeType, "application/vnd.google-apps.") {
		return FileMeta{}, nil, "", "", 0, &APIError{Status: http.StatusBadRequest, Code: "export_required", Message: "Google Docs native files need export (not supported in M2)"}
	}

	params := url.Values{}
	params.Set("alt", "media")
	params.Set("supportsAllDrives", "true")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/files/"+url.PathEscape(fileID)+"?"+params.Encode(), nil)
	if err != nil {
		return FileMeta{}, nil, "", "", 0, err
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return FileMeta{}, nil, "", "", 0, err
	}
	if res.StatusCode != http.StatusOK && (rangeHeader == "" || res.StatusCode != http.StatusPartialContent) {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		return FileMeta{}, nil, "", "", 0, mapDriveError(res.StatusCode, b)
	}
	ct := res.Header.Get("Content-Type")
	if ct == "" {
		ct = meta.MimeType
	}
	return meta, res.Body, ct, res.Header.Get("Content-Range"), res.StatusCode, nil
}

// DownloadRange downloads a byte range of a file (supports HTTP Range requests).
// rangeHeader should be like "bytes=0-1023" or "bytes=1024-".
// Returns 206 status for partial content, 200 for full content.
func (c *Client) DownloadRange(ctx context.Context, fileID, rangeHeader string) (meta FileMeta, body io.ReadCloser, contentType string, contentRange string, statusCode int, err error) {
	m, err := c.GetMeta(ctx, fileID)
	if err != nil {
		return FileMeta{}, nil, "", "", 0, err
	}
	return c.DownloadRangeMedia(ctx, m, rangeHeader)
}

// DownloadRangeMedia is DownloadRange with caller-supplied metadata (from a
// prior List), skipping the GetMeta round-trip.
func (c *Client) DownloadRangeMedia(ctx context.Context, meta FileMeta, rangeHeader string) (FileMeta, io.ReadCloser, string, string, int, error) {
	return c.downloadWithMeta(ctx, meta, rangeHeader)
}

// APIError is a Drive or mapping error with HTTP semantics for handlers.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e == nil {
		return "drive error"
	}
	return e.Message
}

func mapDriveError(status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = http.StatusText(status)
	}
	// Try Google error envelope
	var ge struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &ge) == nil && ge.Error.Message != "" {
		msg = ge.Error.Message
	}
	code := "drive_error"
	switch status {
	case http.StatusUnauthorized:
		code = "drive_unauthorized"
	case http.StatusForbidden:
		code = "drive_forbidden"
	case http.StatusNotFound:
		code = "not_found"
	case http.StatusTooManyRequests:
		code = "rate_limited"
	case http.StatusBadRequest:
		code = "bad_request"
	}
	return &APIError{Status: status, Code: code, Message: msg}
}

// escapeDriveQuery escapes single quotes in Drive query strings.
func escapeDriveQuery(s string) string {
	// Escape backslashes first, then single quotes, per Google Drive query
	// string rules. Escaping quotes alone is insufficient: a trailing (or
	// odd) backslash in the input would escape the closing quote we add,
	// breaking out of the quoted literal (query injection / syntax errors).
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "'", "\\'")
}
