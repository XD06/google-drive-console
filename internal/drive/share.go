package drive

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Permission represents a Drive file permission (share link).
type Permission struct {
	ID        string `json:"id"`
	Type      string `json:"type"`     // "anyone", "user", "group"
	Role      string `json:"role"`     // "reader", "commenter", "writer"
	LinkShare bool   `json:"linkShare,omitempty"`
}

// ShareInfo contains share link and permission details.
type ShareInfo struct {
	ShareURL    string `json:"shareUrl,omitempty"`
	Permission  Permission `json:"permission"`
	AnyoneCanRead bool   `json:"anyoneCanRead"`
}

// ShareLink creates an anyone-with-link reader permission and returns the share URL.
func (c *Client) ShareLink(ctx context.Context, fileID string, role string) (ShareInfo, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return ShareInfo{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "file id required"}
	}
	if role == "" {
		role = "reader"
	}

	// Create permission
	params := url.Values{}
	params.Set("supportsAllDrives", "true")
	params.Set("sendNotificationEmail", "false")
	urlStr := c.base() + "/files/" + url.PathEscape(fileID) + "/permissions?" + params.Encode()

	bodyObj := map[string]string{"type": "anyone", "role": role}
	b, err := json.Marshal(bodyObj)
	if err != nil {
		return ShareInfo{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, strings.NewReader(string(b)))
	if err != nil {
		return ShareInfo{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return ShareInfo{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return ShareInfo{}, err
	}
	if res.StatusCode != http.StatusOK {
		return ShareInfo{}, mapDriveError(res.StatusCode, body)
	}

	var perm struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(body, &perm); err != nil {
		return ShareInfo{}, err
	}

	// Fetch file metadata for share link
	meta, err := c.GetMeta(ctx, fileID)
	if err != nil {
		return ShareInfo{Permission: Permission{ID: perm.ID, Type: perm.Type, Role: perm.Role, LinkShare: true}, AnyoneCanRead: true}, nil
	}

	shareURL := "https://drive.google.com/file/d/" + fileID + "/view"
	_ = meta // avoid unused

	return ShareInfo{
		ShareURL:      shareURL,
		Permission:    Permission{ID: perm.ID, Type: perm.Type, Role: perm.Role, LinkShare: true},
		AnyoneCanRead: true,
	}, nil
}

// Unshare removes an anyone permission (by permission id or type=anyone).
func (c *Client) Unshare(ctx context.Context, fileID, permissionID string) error {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "file id required"}
	}
	permissionID = strings.TrimSpace(permissionID)
	if permissionID == "" {
		return &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "permission id required"}
	}

	params := url.Values{}
	params.Set("supportsAllDrives", "true")
	urlStr := c.base() + "/files/" + url.PathEscape(fileID) + "/permissions/" + url.PathEscape(permissionID) + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, urlStr, nil)
	if err != nil {
		return err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		return mapDriveError(res.StatusCode, body)
	}
	return nil
}

// ListPermissions returns all permissions for a file.
func (c *Client) ListPermissions(ctx context.Context, fileID string) ([]Permission, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "file id required"}
	}
	params := url.Values{}
	params.Set("supportsAllDrives", "true")
	params.Set("fields", "permissions(id,type,role)")
	urlStr := c.base() + "/files/" + url.PathEscape(fileID) + "/permissions?" + params.Encode()
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
		Permissions []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Role string `json:"role"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	items := make([]Permission, 0, len(raw.Permissions))
	for _, p := range raw.Permissions {
		items = append(items, Permission{ID: p.ID, Type: p.Type, Role: p.Role, LinkShare: p.Type == "anyone"})
	}
	return items, nil
}
