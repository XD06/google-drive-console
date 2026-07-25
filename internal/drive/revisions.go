package drive

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Revision represents a file version in Drive.
type Revision struct {
	ID               string `json:"id"`
	ModifiedTime     string `json:"modifiedTime"`
	Size             *int64 `json:"size"`
	LastModifyingUser string `json:"lastModifyingUser,omitempty"`
}

// ListRevisions returns all revisions for a file.
func (c *Client) ListRevisions(ctx context.Context, fileID string) ([]Revision, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "file id required"}
	}
	params := url.Values{}
	params.Set("supportsAllDrives", "true")
	params.Set("fields", "revisions(id,modifiedTime,size,lastModifyingUser)")
	urlStr := c.base() + "/files/" + url.PathEscape(fileID) + "/revisions?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, mapDriveError(res.StatusCode, body)
	}
	var raw struct {
		Revisions []struct {
			ID           string `json:"id"`
			ModifiedTime string `json:"modifiedTime"`
			Size         string `json:"size"`
			LastModifyingUser struct {
				DisplayName  string `json:"displayName"`
				EmailAddress string `json:"emailAddress"`
			} `json:"lastModifyingUser"`
		} `json:"revisions"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	items := make([]Revision, 0, len(raw.Revisions))
	for _, r := range raw.Revisions {
		rv := Revision{ID: r.ID, ModifiedTime: r.ModifiedTime}
		if r.Size != "" {
			if n, err := strconv.ParseInt(r.Size, 10, 64); err == nil {
				rv.Size = &n
			}
		}
		if r.LastModifyingUser.DisplayName != "" {
			rv.LastModifyingUser = r.LastModifyingUser.DisplayName
		} else if r.LastModifyingUser.EmailAddress != "" {
			rv.LastModifyingUser = r.LastModifyingUser.EmailAddress
		}
		items = append(items, rv)
	}
	return items, nil
}

// RestoreRevision promotes a revision to be the current version by downloading
// the revision media and re-uploading it as the file's current content.
// Google Docs native files (application/vnd.google-apps.*) are not supported.
func (c *Client) RestoreRevision(ctx context.Context, fileID, revisionID string) error {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "file id required"}
	}
	revisionID = strings.TrimSpace(revisionID)
	if revisionID == "" {
		return &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "revision id required"}
	}

	// 1. Get file metadata (check type)
	meta, err := c.GetMeta(ctx, fileID)
	if err != nil {
		return err
	}
	if strings.HasPrefix(meta.MimeType, "application/vnd.google-apps.") {
		return &APIError{Status: http.StatusBadRequest, Code: "not_supported",
			Message: "Cannot restore Google Docs native files"}
	}

	// 2. Download the revision media content
	dlParams := url.Values{}
	dlParams.Set("alt", "media")
	dlParams.Set("supportsAllDrives", "true")
	dlURL := c.base() + "/files/" + url.PathEscape(fileID) +
		"/revisions/" + url.PathEscape(revisionID) + "?" + dlParams.Encode()
	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		return err
	}
	dlRes, err := c.HTTP.Do(dlReq)
	if err != nil {
		return err
	}
	defer dlRes.Body.Close()
	if dlRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(dlRes.Body, 1<<20))
		return mapDriveError(dlRes.StatusCode, b)
	}

	// 3. Stream-upload the revision content to overwrite the current file
	uploadURL := c.uploadBase() + "/files/" + url.PathEscape(fileID) +
		"?uploadType=media&supportsAllDrives=true"
	upReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, uploadURL, dlRes.Body)
	if err != nil {
		return err
	}
	upReq.Header.Set("Content-Type", meta.MimeType)
	if cl := dlRes.Header.Get("Content-Length"); cl != "" {
		upReq.Header.Set("Content-Length", cl)
	}
	upRes, err := c.HTTP.Do(upReq)
	if err != nil {
		return err
	}
	defer upRes.Body.Close()
	if upRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(upRes.Body, 1<<20))
		return mapDriveError(upRes.StatusCode, b)
	}
	return nil
}
