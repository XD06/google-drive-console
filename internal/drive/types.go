package drive

// FileItem is a Drive file or folder for the console API.
type FileItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MimeType     string `json:"mimeType"`
	Size         *int64 `json:"size"` // null for folders / unknown
	ModifiedTime string `json:"modifiedTime"`
	IsFolder     bool   `json:"isFolder"`
	Description  string `json:"description,omitempty"`  // Drive native description (folder/file blurb)
	ThumbnailURL string `json:"thumbnailUrl,omitempty"` // proxy URL for thumbnail
	Md5Checksum  string `json:"md5Checksum,omitempty"`  // lets clients skip GetMeta on download
	Version      string `json:"version,omitempty"`      // ETag fallback when no md5 (Docs files)
}

// ListResult is a page of files in a folder.
type ListResult struct {
	FolderID      string     `json:"folderId"`
	Items         []FileItem `json:"items"`
	NextPageToken string     `json:"nextPageToken"`
}

// FolderMIME is Google Drive's folder mime type.
const FolderMIME = "application/vnd.google-apps.folder"

// DefaultAPIBase is the Drive API v3 base URL.
const DefaultAPIBase = "https://www.googleapis.com/drive/v3"
