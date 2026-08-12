package drive

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Zip walk limits — Drive folders can have multiple parents / shortcuts, so
// recursion needs a visited set plus hard caps to avoid infinite walks and
// multi-GB in-memory inventories before streaming starts.
const (
	maxZipDepth = 20
	maxZipFiles = 5000
)

// ListFolderRecursive returns all non-trashed descendants of folderID.
// Used for zip streaming.
func (c *Client) listFolderRecursive(ctx context.Context, folderID string) ([]FileItem, error) {
	return c.listFolderRecursiveLimited(ctx, folderID, 0, map[string]bool{})
}

func (c *Client) listFolderRecursiveLimited(ctx context.Context, folderID string, depth int, visited map[string]bool) ([]FileItem, error) {
	if depth > maxZipDepth {
		return nil, fmt.Errorf("zip: folder tree deeper than %d levels", maxZipDepth)
	}
	if visited[folderID] {
		return nil, nil // cycle / multi-parent revisit — skip quietly
	}
	visited[folderID] = true

	var all []FileItem
	pageToken := ""
	for {
		q := fmt.Sprintf("'%s' in parents and trashed = false", escapeDriveQuery(folderID))
		params := url.Values{}
		params.Set("q", q)
		params.Set("pageSize", "200")
		params.Set("fields", "nextPageToken,files(id,name,mimeType,size,modifiedTime)")
		params.Set("supportsAllDrives", "true")
		params.Set("includeItemsFromAllDrives", "true")
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/files?"+params.Encode(), nil)
		if err != nil {
			return nil, err
		}
		res, err := c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		if err != nil {
			return nil, err
		}
		if res.StatusCode != http.StatusOK {
			return nil, mapDriveError(res.StatusCode, body)
		}
		var raw struct {
			NextPageToken string `json:"nextPageToken"`
			Files         []struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				MimeType     string `json:"mimeType"`
				Size         string `json:"size"`
				ModifiedTime string `json:"modifiedTime"`
			} `json:"files"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}
		for _, f := range raw.Files {
			item := FileItem{
				ID:           f.ID,
				Name:         f.Name,
				MimeType:     f.MimeType,
				ModifiedTime: f.ModifiedTime,
				IsFolder:     f.MimeType == FolderMIME,
			}
			if f.Size != "" {
				if n, err := strconv.ParseInt(f.Size, 10, 64); err == nil {
					item.Size = &n
				}
			}
			all = append(all, item)
		}
		pageToken = raw.NextPageToken
		if pageToken == "" {
			break
		}
	}

	// Recurse into subfolders
	var result []FileItem
	for _, item := range all {
		if len(result) >= maxZipFiles {
			return nil, fmt.Errorf("zip: tree exceeds %d items", maxZipFiles)
		}
		if item.IsFolder {
			children, err := c.listFolderRecursiveLimited(ctx, item.ID, depth+1, visited)
			if err != nil {
				return nil, err
			}
			result = append(result, item)
			for _, ch := range children {
				if len(result) >= maxZipFiles {
					return nil, fmt.Errorf("zip: tree exceeds %d items", maxZipFiles)
				}
				ch.Name = item.Name + "/" + ch.Name
				result = append(result, ch)
			}
		} else {
			result = append(result, item)
		}
	}
	return result, nil
}

// StreamZip writes a zip archive of all files under folderID to w.
// Folders are represented as directory entries; files are streamed via Download.
func (c *Client) StreamZip(ctx context.Context, folderID, rootName string, w io.Writer) error {
	if rootName == "" {
		rootName = "drive-archive"
	}

	items, err := c.listFolderRecursive(ctx, folderID)
	if err != nil {
		return err
	}

	zw := zip.NewWriter(w)
	// No defer zw.Close() — only close once at the end to avoid double-close.

	for _, item := range items {
		path := rootName + "/" + item.Name

		if item.IsFolder {
			path += "/"
			hdr := zip.FileHeader{Name: path, Method: zip.Deflate}
			if _, err := zw.CreateHeader(&hdr); err != nil {
				return err
			}
			continue // directory entry only
		}

		// Download file media first — only create zip entry if download succeeds
		_, body, _, err := c.Download(ctx, item.ID)
		if err != nil {
			// Skip files that can't be downloaded (e.g. Google Docs native)
			continue
		}

		// Use Store (no compression) for already-compressed formats to save CPU
		method := zip.Deflate
		if isAlreadyCompressed(item.MimeType) {
			method = zip.Store
		}
		hdr := zip.FileHeader{Name: path, Method: method}
		writer, err := zw.CreateHeader(&hdr)
		if err != nil {
			body.Close()
			return err
		}

		// Check io.Copy errors — propagate to abort the zip on transfer failure
		if _, err := io.Copy(writer, body); err != nil {
			body.Close()
			return fmt.Errorf("zip: copy %s: %w", path, err)
		}
		body.Close()
	}

	return zw.Close()
}

// isAlreadyCompressed reports whether the mimeType is already a compressed format
// where applying Deflate would waste CPU without meaningful size reduction.
func isAlreadyCompressed(mimeType string) bool {
	mt := strings.ToLower(strings.TrimSpace(mimeType))
	compressedPrefixes := []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
		"image/heic",
		"image/avif",
		"video/",
		"audio/",
		"application/zip",
		"application/x-zip",
		"application/gzip",
		"application/x-gzip",
		"application/x-bzip",
		"application/x-bzip2",
		"application/x-7z",
		"application/x-rar",
		"application/x-tar",
		"application/pdf",
	}
	for _, p := range compressedPrefixes {
		if strings.HasPrefix(mt, p) {
			return true
		}
	}
	return false
}

// ZipItem is a file to include in a multi-file zip archive.
type ZipItem struct {
	ID   string
	Name string
}

// StreamZipMulti writes a zip archive of the given files to w.
// Each file is downloaded individually and added to the zip.
// Files that fail to download are skipped (same behavior as StreamZip).
// Uses DownloadMedia with pre-fetched metadata to avoid redundant GetMeta calls
// (previously 3 upstream round-trips per file, now 1: meta + media in one call).
func (c *Client) StreamZipMulti(ctx context.Context, items []ZipItem, w io.Writer) error {
	zw := zip.NewWriter(w)

	for _, item := range items {
		// Get metadata to determine mimeType for compression method
		meta, err := c.GetMeta(ctx, item.ID)
		if err != nil {
			continue // skip files that can't be accessed
		}

		// Use DownloadMedia with the metadata we already have (skips the
		// redundant GetMeta inside Download — saves 1 RTT per file).
		_, body, _, err := c.DownloadMedia(ctx, meta)
		if err != nil {
			continue // skip files that can't be downloaded
		}

		method := zip.Deflate
		if isAlreadyCompressed(meta.MimeType) {
			method = zip.Store
		}

		name := item.Name
		if name == "" {
			name = meta.Name
		}
		hdr := zip.FileHeader{Name: name, Method: method}
		writer, err := zw.CreateHeader(&hdr)
		if err != nil {
			body.Close()
			return err
		}

		if _, err := io.Copy(writer, body); err != nil {
			body.Close()
			return fmt.Errorf("zip: copy %s: %w", name, err)
		}
		body.Close()
	}

	return zw.Close()
}
