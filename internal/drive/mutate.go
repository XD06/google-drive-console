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

// CreateFolder creates a folder with the given name under parentID (default "root").
func (c *Client) CreateFolder(ctx context.Context, name, parentID string) (FileItem, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return FileItem{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "name required"}
	}
	if strings.TrimSpace(parentID) == "" {
		parentID = "root"
	}
	params := url.Values{}
	params.Set("supportsAllDrives", "true")
	params.Set("fields", "id,name,mimeType,size,modifiedTime")
	urlStr := c.base() + "/files?" + params.Encode()
	bodyObj := map[string]interface{}{
		"name":     name,
		"mimeType": FolderMIME,
		"parents":  []string{parentID},
	}
	b, err := json.Marshal(bodyObj)
	if err != nil {
		return FileItem{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, strings.NewReader(string(b)))
	if err != nil {
		return FileItem{}, err
	}
	req.Header.Set("Content-Type", "application/json")
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
	var raw struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		MimeType     string `json:"mimeType"`
		Size         string `json:"size"`
		ModifiedTime string `json:"modifiedTime"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return FileItem{}, err
	}
	item := FileItem{
		ID:           raw.ID,
		Name:         raw.Name,
		MimeType:     raw.MimeType,
		ModifiedTime: raw.ModifiedTime,
		IsFolder:     raw.MimeType == FolderMIME,
	}
	if raw.Size != "" {
		if n, err := strconv.ParseInt(raw.Size, 10, 64); err == nil {
			item.Size = &n
		}
	}
	return item, nil
}

// Trash moves a file to Drive trash (sets trashed = true).
func (c *Client) Trash(ctx context.Context, fileID string) error {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "file id required"}
	}
	params := url.Values{}
	params.Set("supportsAllDrives", "true")
	urlStr := c.base() + "/files/" + url.PathEscape(fileID) + "?" + params.Encode()
	bodyObj := map[string]bool{"trashed": true}
	b, err := json.Marshal(bodyObj)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, urlStr, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return mapDriveError(res.StatusCode, body)
	}
	return nil
}

// Rename updates a file or folder display name.
func (c *Client) Rename(ctx context.Context, fileID, name string) (FileItem, error) {
	return c.UpdateMetadata(ctx, fileID, &name, nil)
}

// UpdateMetadata updates a file's name and/or description. Only non-nil fields
// are sent to Drive; a non-nil empty description clears it. At least one of
// name/description must be provided, and a non-nil name must be non-empty.
func (c *Client) UpdateMetadata(ctx context.Context, fileID string, name, description *string) (FileItem, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return FileItem{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "file id required"}
	}
	if name == nil && description == nil {
		return FileItem{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "name or description required"}
	}
	bodyObj := map[string]interface{}{}
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return FileItem{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "name required"}
		}
		bodyObj["name"] = n
	}
	if description != nil {
		// Non-nil: set the description (empty string clears it).
		bodyObj["description"] = *description
	}
	params := url.Values{}
	params.Set("supportsAllDrives", "true")
	params.Set("fields", "id,name,mimeType,size,modifiedTime,description")
	urlStr := c.base() + "/files/" + url.PathEscape(fileID) + "?" + params.Encode()
	b, err := json.Marshal(bodyObj)
	if err != nil {
		return FileItem{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, urlStr, strings.NewReader(string(b)))
	if err != nil {
		return FileItem{}, err
	}
	req.Header.Set("Content-Type", "application/json")
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

// Move relocates a file/folder under newParentID (single-parent move).
// If fromParentID is non-empty, skips the GET parents lookup (saves 1 RTT).
func (c *Client) Move(ctx context.Context, fileID, newParentID, fromParentID string) (FileItem, error) {
	fileID = strings.TrimSpace(fileID)
	newParentID = strings.TrimSpace(newParentID)
	if fileID == "" {
		return FileItem{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "file id required"}
	}
	if newParentID == "" {
		return FileItem{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "parentId required"}
	}

	var oldParents []string
	if fromParentID = strings.TrimSpace(fromParentID); fromParentID != "" {
		// Caller provided the current parent — skip GET
		oldParents = []string{fromParentID}
	} else {
		// Fallback: GET current parents
		getParams := url.Values{}
		getParams.Set("supportsAllDrives", "true")
		getParams.Set("fields", "parents")
		getURL := c.base() + "/files/" + url.PathEscape(fileID) + "?" + getParams.Encode()
		getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
		if err != nil {
			return FileItem{}, err
		}
		getRes, err := c.HTTP.Do(getReq)
		if err != nil {
			return FileItem{}, err
		}
		defer getRes.Body.Close()
		getBody, err := io.ReadAll(io.LimitReader(getRes.Body, 1<<20))
		if err != nil {
			return FileItem{}, err
		}
		if getRes.StatusCode != http.StatusOK {
			return FileItem{}, mapDriveError(getRes.StatusCode, getBody)
		}
		var meta struct {
			Parents []string `json:"parents"`
		}
		if err := json.Unmarshal(getBody, &meta); err != nil {
			return FileItem{}, err
		}
		oldParents = meta.Parents
	}

	// Already only under the target parent
	if len(oldParents) == 1 && oldParents[0] == newParentID {
		return c.getFileItem(ctx, fileID)
	}

	params := url.Values{}
	params.Set("supportsAllDrives", "true")
	params.Set("addParents", newParentID)
	if len(oldParents) > 0 {
		params.Set("removeParents", strings.Join(oldParents, ","))
	}
	params.Set("fields", "id,name,mimeType,size,modifiedTime")
	urlStr := c.base() + "/files/" + url.PathEscape(fileID) + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, urlStr, strings.NewReader("{}"))
	if err != nil {
		return FileItem{}, err
	}
	req.Header.Set("Content-Type", "application/json")
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

// getFileItem fetches FileItem fields (id/name/mime/size/modifiedTime).
func (c *Client) getFileItem(ctx context.Context, fileID string) (FileItem, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return FileItem{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "file id required"}
	}
	params := url.Values{}
	params.Set("supportsAllDrives", "true")
	params.Set("fields", "id,name,mimeType,size,modifiedTime")
	urlStr := c.base() + "/files/" + url.PathEscape(fileID) + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return FileItem{}, err
	}
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

func parseFileItem(body []byte) (FileItem, error) {
	var raw struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		MimeType     string `json:"mimeType"`
		Size         string `json:"size"`
		ModifiedTime string `json:"modifiedTime"`
		Description  string `json:"description"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return FileItem{}, err
	}
	item := FileItem{
		ID:           raw.ID,
		Name:         raw.Name,
		MimeType:     raw.MimeType,
		ModifiedTime: raw.ModifiedTime,
		IsFolder:     raw.MimeType == FolderMIME,
		Description:  raw.Description,
	}
	if raw.Size != "" {
		if n, err := strconv.ParseInt(raw.Size, 10, 64); err == nil {
			item.Size = &n
		}
	}
	return item, nil
}
