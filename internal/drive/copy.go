package drive

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Copy creates a copy of a file under a new parent (optional new name).
func (c *Client) Copy(ctx context.Context, fileID, parentID, newName string) (FileItem, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return FileItem{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "file id required"}
	}
	parentID = strings.TrimSpace(parentID)

	params := url.Values{}
	params.Set("supportsAllDrives", "true")
	params.Set("fields", "id,name,mimeType,size,modifiedTime")
	urlStr := c.base() + "/files/" + url.PathEscape(fileID) + "/copy?" + params.Encode()

	bodyObj := map[string]interface{}{}
	if newName != "" {
		bodyObj["name"] = newName
	}
	if parentID != "" {
		bodyObj["parents"] = []string{parentID}
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
	return parseFileItem(body)
}
