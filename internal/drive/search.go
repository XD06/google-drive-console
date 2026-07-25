package drive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// SearchScope controls where name search runs.
type SearchScope string

const (
	SearchScopeFolder SearchScope = "folder"
	SearchScopeDrive  SearchScope = "drive"
)

// SearchOptions controls name-only file/folder search.
type SearchOptions struct {
	Query     string
	Scope     SearchScope
	FolderID  string // required when Scope is folder
	PageToken string
	PageSize  int
}

// SearchResult is a page of name-matched files.
type SearchResult struct {
	Query         string     `json:"query"`
	Scope         string     `json:"scope"`
	FolderID      string     `json:"folderId,omitempty"`
	Items         []FileItem `json:"items"`
	NextPageToken string     `json:"nextPageToken"`
}

// Search finds non-trashed files/folders whose name contains Query.
// Ranking is applied to the current page only (Drive pagination is opaque).
func (c *Client) Search(ctx context.Context, opt SearchOptions) (SearchResult, error) {
	q := strings.TrimSpace(opt.Query)
	if q == "" {
		return SearchResult{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "query required"}
	}
	if utf8RuneCount(q) < 2 {
		return SearchResult{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "query must be at least 2 characters"}
	}

	scope := opt.Scope
	if scope == "" {
		scope = SearchScopeDrive
	}
	if scope != SearchScopeFolder && scope != SearchScopeDrive {
		return SearchResult{}, &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: "scope must be folder or drive"}
	}

	folderID := strings.TrimSpace(opt.FolderID)
	if scope == SearchScopeFolder {
		if folderID == "" {
			folderID = "root"
		}
	}

	pageSize := opt.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	// name contains is case-insensitive on Drive; escape for query DSL safety.
	clauses := []string{
		fmt.Sprintf("name contains '%s'", escapeDriveQuery(q)),
		"trashed = false",
	}
	if scope == SearchScopeFolder {
		clauses = append(clauses, fmt.Sprintf("'%s' in parents", escapeDriveQuery(folderID)))
	}
	driveQ := strings.Join(clauses, " and ")

	params := url.Values{}
	params.Set("q", driveQ)
	params.Set("pageSize", strconv.Itoa(pageSize))
	params.Set("fields", "nextPageToken,files(id,name,mimeType,size,modifiedTime,thumbnailLink,description,md5Checksum,version)")
	// orderBy helps a bit; we re-rank locally for exact/prefix preference.
	params.Set("orderBy", "folder,name_natural")
	params.Set("supportsAllDrives", "true")
	params.Set("includeItemsFromAllDrives", "true")
	if opt.PageToken != "" {
		params.Set("pageToken", opt.PageToken)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/files?"+params.Encode(), nil)
	if err != nil {
		return SearchResult{}, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return SearchResult{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return SearchResult{}, err
	}
	if res.StatusCode != http.StatusOK {
		return SearchResult{}, mapDriveError(res.StatusCode, body)
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
		return SearchResult{}, err
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

	rankSearchItems(items, q)

	out := SearchResult{
		Query:         q,
		Scope:         string(scope),
		Items:         items,
		NextPageToken: raw.NextPageToken,
	}
	if scope == SearchScopeFolder {
		out.FolderID = folderID
	}
	return out, nil
}

func utf8RuneCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// rankSearchItems sorts by exact > prefix > contains, then folders, then name.
func rankSearchItems(items []FileItem, query string) {
	q := strings.ToLower(strings.TrimSpace(query))
	sort.SliceStable(items, func(i, j int) bool {
		si, sj := searchTier(items[i].Name, q), searchTier(items[j].Name, q)
		if si != sj {
			return si < sj // lower tier is better
		}
		if items[i].IsFolder != items[j].IsFolder {
			return items[i].IsFolder
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
}

// searchTier: 0 exact, 1 prefix, 2 contains, 3 other.
func searchTier(name, qLower string) int {
	n := strings.ToLower(name)
	if n == qLower {
		return 0
	}
	if strings.HasPrefix(n, qLower) {
		return 1
	}
	if strings.Contains(n, qLower) {
		return 2
	}
	// Drive may match differently (e.g. punctuation); keep last.
	return 3
}

// NormalizeSearchQuery trims and collapses internal whitespace for display/cache keys.
func NormalizeSearchQuery(q string) string {
	return strings.Join(strings.FieldsFunc(strings.TrimSpace(q), unicode.IsSpace), " ")
}
