package drive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

// CreateFile creates a new file with optional initial text content.
func (c *Client) CreateFile(ctx context.Context, name, parentID, mimeType, content string) (FileItem, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return FileItem{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "name required"}
	}
	if parentID == "" {
		parentID = "root"
	}
	if int64(len(content)) > MaxTextContent {
		return FileItem{}, &APIError{Status: http.StatusRequestEntityTooLarge, Code: "content_too_large", Message: "Content too large"}
	}

	mt := strings.TrimSpace(mimeType)
	if mt == "" {
		mt = mimeFromFilename(name)
	}
	if mt == "" {
		mt = "text/plain"
	}

	// When content is provided, use multipart upload (single request: metadata + media)
	// This avoids the previous 2-step approach (POST metadata + PATCH media) which
	// had an inconsistent intermediate state (file created but empty if step 2 failed).
	if content != "" {
		pID := parentID
		if pID == "root" {
			pID = "" // SimpleUpload sets parents only when parentID != ""; omitting parents defaults to root
		}
		return c.SimpleUpload(ctx, name, mt, pID, strings.NewReader(content))
	}

	// No content: create file with metadata only (single POST)
	meta := map[string]any{
		"name":     name,
		"mimeType": mt,
	}
	if p := strings.TrimSpace(parentID); p != "" && p != "root" {
		meta["parents"] = []string{p}
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return FileItem{}, err
	}

	u := c.base() + "/files?supportsAllDrives=true&fields=id,name,mimeType,size,modifiedTime"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return FileItem{}, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return FileItem{}, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return FileItem{}, mapDriveError(res.StatusCode, b)
	}
	return parseFileItem(b)
}

func parseSizeString(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	var n int64
	_, err := fmt.Sscan(s, &n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// mimeFromFilename derives a text-friendly mime type from filename extension.
func mimeFromFilename(name string) string {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(name)))
	switch ext {
	case ".md", ".markdown":
		return "text/markdown"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "text/javascript"
	case ".ts":
		return "text/plain"
	case ".csv":
		return "text/csv"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".log", ".env", ".txt":
		return "text/plain"
	default:
		return ""
	}
}