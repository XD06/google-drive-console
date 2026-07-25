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

// StorageQuota mirrors parsed numeric values from Google About.storageQuota.
type StorageQuota struct {
	Limit        int64 `json:"limit"`
	Usage        int64 `json:"usage"`
	UsageInDrive int64 `json:"usageInDrive"`
}

type AboutUser struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

type AboutInfo struct {
	Storage StorageQuota `json:"storage"`
	User    AboutUser    `json:"user"`
}

// About fetches the "about" resource from Drive and parses numeric quota fields.
func (c *Client) About(ctx context.Context) (AboutInfo, error) {
	params := url.Values{}
	// Request only the fields we need
	params.Set("fields", "user,storageQuota")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/about?"+params.Encode(), nil)
	if err != nil {
		return AboutInfo{}, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return AboutInfo{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return AboutInfo{}, err
	}
	if res.StatusCode != http.StatusOK {
		return AboutInfo{}, mapDriveError(res.StatusCode, body)
	}
	var raw struct {
		User struct {
			EmailAddress string `json:"emailAddress"`
			DisplayName  string `json:"displayName"`
		} `json:"user"`
		StorageQuota struct {
			Limit        string `json:"limit"`
			Usage        string `json:"usage"`
			UsageInDrive string `json:"usageInDrive"`
		} `json:"storageQuota"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return AboutInfo{}, err
	}
	ai := AboutInfo{}
	ai.User.Email = raw.User.EmailAddress
	ai.User.DisplayName = raw.User.DisplayName
	// Parse numeric quota strings; leaving zero on parse failure
	if raw.StorageQuota.Limit != "" {
		if v, err := strconv.ParseInt(strings.TrimSpace(raw.StorageQuota.Limit), 10, 64); err == nil {
			ai.Storage.Limit = v
		}
	}
	if raw.StorageQuota.Usage != "" {
		if v, err := strconv.ParseInt(strings.TrimSpace(raw.StorageQuota.Usage), 10, 64); err == nil {
			ai.Storage.Usage = v
		}
	}
	if raw.StorageQuota.UsageInDrive != "" {
		if v, err := strconv.ParseInt(strings.TrimSpace(raw.StorageQuota.UsageInDrive), 10, 64); err == nil {
			ai.Storage.UsageInDrive = v
		}
	}
	return ai, nil
}
