package drive

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

const MaxTextContent = 2 << 20 // 2 MiB

// TextContent is returned by GetTextContent.
type TextContent struct {
	Name     string
	MimeType string
	Content  string
	Size     int64
}

func IsTextEditableMIME(mime, name string) bool {
	mime = strings.ToLower(strings.TrimSpace(mime))
	if strings.HasPrefix(mime, "text/") {
		return true
	}
	switch mime {
	case "application/json", "application/xml", "application/javascript", "application/typescript":
		return true
	}
	// Extension fallback (Drive often uses application/octet-stream)
	if name != "" {
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".txt", ".md", ".markdown", ".json", ".csv", ".log", ".yaml", ".yml", ".xml", ".html", ".htm", ".css", ".js", ".ts", ".env":
			return true
		}
	}
	return false
}

func (c *Client) GetTextContent(ctx context.Context, fileID string) (TextContent, error) {
	meta, err := c.GetMeta(ctx, fileID)
	if err != nil {
		return TextContent{}, err
	}
	if meta.MimeType == FolderMIME {
		return TextContent{}, &APIError{Status: http.StatusBadRequest, Code: "is_folder", Message: "Cannot read a folder"}
	}
	if strings.HasPrefix(meta.MimeType, "application/vnd.google-apps.") {
		return TextContent{}, &APIError{Status: http.StatusBadRequest, Code: "export_required", Message: "Google Docs native files need export (not supported in M2)"}
	}
	if meta.Size != nil && *meta.Size > MaxTextContent {
		return TextContent{}, &APIError{Status: http.StatusRequestEntityTooLarge, Code: "content_too_large", Message: "File too large"}
	}
	if !IsTextEditableMIME(meta.MimeType, meta.Name) {
		return TextContent{}, &APIError{Status: http.StatusBadRequest, Code: "not_text", Message: "Not a text-editable file"}
	}

	params := url.Values{}
	params.Set("alt", "media")
	params.Set("supportsAllDrives", "true")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/files/"+url.PathEscape(fileID)+"?"+params.Encode(), nil)
	if err != nil {
		return TextContent{}, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return TextContent{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		return TextContent{}, mapDriveError(res.StatusCode, b)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, MaxTextContent+1))
	if err != nil {
		return TextContent{}, err
	}
	if int64(len(b)) > MaxTextContent {
		return TextContent{}, &APIError{Status: http.StatusRequestEntityTooLarge, Code: "content_too_large", Message: "File too large"}
	}
	return TextContent{Name: meta.Name, MimeType: meta.MimeType, Content: string(b), Size: int64(len(b))}, nil
}

func (c *Client) UpdateTextContent(ctx context.Context, fileID, content string) error {
	if int64(len(content)) > MaxTextContent {
		return &APIError{Status: http.StatusRequestEntityTooLarge, Code: "content_too_large", Message: "Content too large"}
	}
	meta, err := c.GetMeta(ctx, fileID)
	if err != nil {
		return err
	}
	if meta.MimeType == FolderMIME {
		return &APIError{Status: http.StatusBadRequest, Code: "is_folder", Message: "Cannot write a folder"}
	}
	if strings.HasPrefix(meta.MimeType, "application/vnd.google-apps.") {
		return &APIError{Status: http.StatusBadRequest, Code: "export_required", Message: "Google Docs native files need export (not supported in M2)"}
	}
	if !IsTextEditableMIME(meta.MimeType, meta.Name) {
		return &APIError{Status: http.StatusBadRequest, Code: "not_text", Message: "Not a text-editable file"}
	}

	var uploadURL string
	base := c.base()
	if base != "" && base != DefaultAPIBase {
		uploadURL = base + "/files/" + url.PathEscape(fileID) + "?uploadType=media&supportsAllDrives=true"
	} else {
		uploadURL = c.uploadBase() + "/files/" + url.PathEscape(fileID) + "?uploadType=media&supportsAllDrives=true"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, uploadURL, strings.NewReader(content))
	if err != nil {
		return err
	}
	ct := meta.MimeType
	if ct == "" {
		ct = "text/plain; charset=utf-8"
	}
	req.Header.Set("Content-Type", ct)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		return mapDriveError(res.StatusCode, b)
	}
	return nil
}
