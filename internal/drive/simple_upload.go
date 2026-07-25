package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

// SimpleUpload uploads a file using uploadType=multipart (metadata + media in one request).
// Best for files < 5MB — saves 2 round-trips vs resumable (init + chunk + complete).
func (c *Client) SimpleUpload(ctx context.Context, name, mimeType, parentID string, content io.Reader) (FileItem, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return FileItem{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "name required"}
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Build metadata JSON
	meta := map[string]interface{}{"name": name, "mimeType": mimeType}
	if parentID != "" {
		meta["parents"] = []string{parentID}
	}
	metaJSON, _ := json.Marshal(meta)

	// Build multipart/related body
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Part 1: metadata (application/json)
	hdr1 := map[string][]string{
		"Content-Type": {"application/json; charset=UTF-8"},
	}
	pw1, err := mw.CreatePart(hdr1)
	if err != nil {
		return FileItem{}, err
	}
	pw1.Write(metaJSON)

	// Part 2: media content
	hdr2 := map[string][]string{
		"Content-Type": {mimeType},
	}
	pw2, err := mw.CreatePart(hdr2)
	if err != nil {
		return FileItem{}, err
	}
	if _, err := io.Copy(pw2, content); err != nil {
		return FileItem{}, err
	}
	mw.Close()

	// Send request
	params := url.Values{}
	params.Set("uploadType", "multipart")
	params.Set("supportsAllDrives", "true")
	params.Set("fields", "id,name,mimeType,size,modifiedTime")
	urlStr := c.uploadBase() + "/files?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, &buf)
	if err != nil {
		return FileItem{}, err
	}
	req.Header.Set("Content-Type", "multipart/related; boundary="+mw.Boundary())

	res, err := c.HTTP.Do(req)
	if err != nil {
		return FileItem{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return FileItem{}, err
	}
	if res.StatusCode != http.StatusOK {
		return FileItem{}, mapDriveError(res.StatusCode, body)
	}
	return parseFileItem(body)
}
