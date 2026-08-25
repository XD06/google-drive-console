package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/dsk/drive-backup-console/internal/apikey"
	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/config"
	"github.com/dsk/drive-backup-console/internal/download"
	"github.com/dsk/drive-backup-console/internal/upload"
)

// Deps holds optional services wired by main.
type Deps struct {
	Config   config.Config
	Auth     *auth.Service
	Uploads  *upload.Service            // Store required; Drive may be filled per-request
	Keys     *apikey.Store              // programmatic API keys for /api/v1 (nil disables key auth)
	Downloads *download.PersistentStore // nil disables download feature
}

// NewRouter wires HTTP routes.
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()
	serveSPA(mux, d)
	mux.HandleFunc("GET /api/health", HealthHandler(d.Config.DevMode))

	ah := &AuthHandlers{Auth: d.Auth, FrontendOrigin: d.Config.FrontendOrigin}
	mux.HandleFunc("GET /oauth2/login", ah.Login)
	mux.HandleFunc("GET /oauth2/callback", ah.Callback)
	mux.HandleFunc("GET /api/auth/me", ah.Me)
	mux.HandleFunc("POST /api/auth/logout", ah.Logout)

	fh := &FilesHandlers{
		Auth:          d.Auth,
		DefaultFolder: d.Config.RootFolderID,
	}
	mux.Handle("GET /api/files", RequireSession(d.Auth, http.HandlerFunc(fh.List)))
	mux.Handle("GET /api/files/search", RequireSession(d.Auth, http.HandlerFunc(fh.Search)))
	mux.Handle("GET /api/files/{id}/content", RequireSession(d.Auth, http.HandlerFunc(fh.GetContent)))
	mux.Handle("PUT /api/files/{id}/content", RequireSession(d.Auth, BodyLimit(3<<20)(http.HandlerFunc(fh.PutContent))))
	mux.Handle("GET /api/files/{id}/download", RequireSession(d.Auth, http.HandlerFunc(fh.Download)))
	mux.Handle("POST /api/files", RequireSession(d.Auth, BodyLimit(3<<20)(http.HandlerFunc(fh.Create))))
	mux.Handle("POST /api/files/mkdir", RequireSession(d.Auth, BodyLimit(64<<10)(http.HandlerFunc(fh.Mkdir))))
	mux.Handle("POST /api/files/simple", RequireSession(d.Auth, http.HandlerFunc(fh.SimpleUpload)))
	mux.Handle("PATCH /api/files/{id}", RequireSession(d.Auth, BodyLimit(64<<10)(http.HandlerFunc(fh.Rename))))
	mux.Handle("POST /api/files/{id}/move", RequireSession(d.Auth, BodyLimit(64<<10)(http.HandlerFunc(fh.Move))))
	mux.Handle("POST /api/files/{id}/copy", RequireSession(d.Auth, BodyLimit(64<<10)(http.HandlerFunc(fh.Copy))))
	mux.Handle("DELETE /api/files/{id}", RequireSession(d.Auth, http.HandlerFunc(fh.Trash)))
	mux.Handle("POST /api/files/{id}/share", RequireSession(d.Auth, BodyLimit(64<<10)(http.HandlerFunc(fh.Share))))
	mux.Handle("DELETE /api/files/{id}/share", RequireSession(d.Auth, http.HandlerFunc(fh.Unshare)))
	mux.Handle("GET /api/files/{id}/permissions", RequireSession(d.Auth, http.HandlerFunc(fh.ListPermissions)))
	mux.Handle("GET /api/files/{id}/revisions", RequireSession(d.Auth, http.HandlerFunc(fh.Revisions)))
	mux.Handle("POST /api/files/{id}/revisions/{rev}/restore", RequireSession(d.Auth, http.HandlerFunc(fh.RestoreRevision)))
	mux.Handle("GET /api/files/{id}/zip", RequireSession(d.Auth, http.HandlerFunc(fh.DownloadZip)))
	mux.Handle("GET /api/files/{id}/thumbnail", RequireSession(d.Auth, http.HandlerFunc(fh.Thumbnail)))
	mux.Handle("POST /api/files/batch", RequireSession(d.Auth, BodyLimit(1<<20)(http.HandlerFunc(fh.Batch))))
	mux.Handle("POST /api/files/zip", RequireSession(d.Auth, BodyLimit(1<<20)(http.HandlerFunc(fh.MultiZip))))

	uh := &UploadHandlers{
		Auth:          d.Auth,
		Uploads:       d.Uploads,
		DefaultFolder: d.Config.RootFolderID,
	}
	mux.Handle("POST /api/uploads", RequireSession(d.Auth, http.HandlerFunc(uh.Create)))
	mux.Handle("GET /api/uploads/{id}", RequireSession(d.Auth, http.HandlerFunc(uh.Status)))
	mux.Handle("PUT /api/uploads/{id}/chunk", RequireSession(d.Auth, http.HandlerFunc(uh.Chunk)))
	mux.Handle("POST /api/uploads/{id}/cancel", RequireSession(d.Auth, http.HandlerFunc(uh.Cancel)))

	// Overview
	oh := &OverviewHandlers{Auth: d.Auth}
	mux.Handle("GET /api/overview", RequireSession(d.Auth, http.HandlerFunc(oh.Get)))

	// Downloads (yt-dlp → Google Drive)
	if d.Downloads != nil {
		dh := &DownloadHandlers{
			Auth:          d.Auth,
			Store:         d.Downloads,
			YtDlp: download.YtDlpConfig{
				BinPath:    d.Config.YtDlpPath,
				Proxy:       d.Config.DownloadProxy,
				CookiePath: d.Config.DownloadCookiePath,
				TmpDir:      d.Config.DownloadTmpDir,
			},
			DefaultFolder: d.Config.RootFolderID,
		}

		mux.Handle("POST /api/downloads", RequireSession(d.Auth, http.HandlerFunc(dh.Create)))
		mux.Handle("GET /api/downloads", RequireSession(d.Auth, http.HandlerFunc(dh.List)))
		mux.Handle("DELETE /api/downloads", RequireSession(d.Auth, http.HandlerFunc(dh.ClearFinished)))
		mux.Handle("GET /api/downloads/{id}", RequireSession(d.Auth, http.HandlerFunc(dh.Status)))
		mux.Handle("POST /api/downloads/{id}/cancel", RequireSession(d.Auth, http.HandlerFunc(dh.Cancel)))
		mux.Handle("POST /api/downloads/{id}/retry-upload", RequireSession(d.Auth, http.HandlerFunc(dh.RetryUpload)))
		mux.Handle("DELETE /api/downloads/{id}", RequireSession(d.Auth, http.HandlerFunc(dh.Delete)))
	}

	// --- /api/v1: programmatic API for AI agents / scripts --------------------
	// Reuses the same handlers as the UI API, but authenticated by cookie OR API
	// key and gated by scope. This namespace is the stable external contract.
	guard := func(scope apikey.Scope, h http.Handler) http.Handler {
		return guardScope(d.Auth, d.Keys, scope, h)
	}
	rd, rw := apikey.ScopeRead, apikey.ScopeReadWrite

	// Discovery (no auth so tools can inspect the contract first).
	mux.HandleFunc("GET /api/v1/openapi.json", OpenAPIHandler())

	// API key management (session-only; a key can never manage keys).
	kh := &APIKeyHandlers{Keys: d.Keys}
	mux.Handle("POST /api/v1/keys", RequireSession(d.Auth, BodyLimit(64<<10)(http.HandlerFunc(kh.Create))))
	mux.Handle("GET /api/v1/keys", RequireSession(d.Auth, http.HandlerFunc(kh.List)))
	mux.Handle("DELETE /api/v1/keys/{id}", RequireSession(d.Auth, http.HandlerFunc(kh.Revoke)))

	// Files (read).
	mux.Handle("GET /api/v1/files", guard(rd, http.HandlerFunc(fh.List)))
	mux.Handle("GET /api/v1/files/search", guard(rd, http.HandlerFunc(fh.Search)))
	mux.Handle("GET /api/v1/files/{id}/content", guard(rd, http.HandlerFunc(fh.GetContent)))
	mux.Handle("GET /api/v1/files/{id}/download", guard(rd, http.HandlerFunc(fh.Download)))
	mux.Handle("GET /api/v1/files/{id}/permissions", guard(rd, http.HandlerFunc(fh.ListPermissions)))
	mux.Handle("GET /api/v1/files/{id}/revisions", guard(rd, http.HandlerFunc(fh.Revisions)))
	mux.Handle("GET /api/v1/files/{id}/zip", guard(rd, http.HandlerFunc(fh.DownloadZip)))
	mux.Handle("GET /api/v1/files/{id}/thumbnail", guard(rd, http.HandlerFunc(fh.Thumbnail)))
	mux.Handle("POST /api/v1/files/zip", guard(rd, BodyLimit(1<<20)(http.HandlerFunc(fh.MultiZip))))

	// Files (read-write).
	mux.Handle("POST /api/v1/files", guard(rw, BodyLimit(3<<20)(http.HandlerFunc(fh.Create))))
	mux.Handle("PUT /api/v1/files/{id}/content", guard(rw, BodyLimit(3<<20)(http.HandlerFunc(fh.PutContent))))
	mux.Handle("POST /api/v1/files/mkdir", guard(rw, BodyLimit(64<<10)(http.HandlerFunc(fh.Mkdir))))
	mux.Handle("POST /api/v1/files/simple", guard(rw, http.HandlerFunc(fh.SimpleUpload)))
	mux.Handle("PATCH /api/v1/files/{id}", guard(rw, BodyLimit(64<<10)(http.HandlerFunc(fh.Rename))))
	mux.Handle("POST /api/v1/files/{id}/move", guard(rw, BodyLimit(64<<10)(http.HandlerFunc(fh.Move))))
	mux.Handle("POST /api/v1/files/{id}/copy", guard(rw, BodyLimit(64<<10)(http.HandlerFunc(fh.Copy))))
	mux.Handle("DELETE /api/v1/files/{id}", guard(rw, http.HandlerFunc(fh.Trash)))
	mux.Handle("POST /api/v1/files/{id}/share", guard(rw, BodyLimit(64<<10)(http.HandlerFunc(fh.Share))))
	mux.Handle("DELETE /api/v1/files/{id}/share", guard(rw, http.HandlerFunc(fh.Unshare)))
	mux.Handle("POST /api/v1/files/{id}/revisions/{rev}/restore", guard(rw, http.HandlerFunc(fh.RestoreRevision)))
	mux.Handle("POST /api/v1/files/batch", guard(rw, BodyLimit(1<<20)(http.HandlerFunc(fh.Batch))))

	// Resumable uploads.
	mux.Handle("POST /api/v1/uploads", guard(rw, http.HandlerFunc(uh.Create)))
	mux.Handle("GET /api/v1/uploads/{id}", guard(rd, http.HandlerFunc(uh.Status)))
	mux.Handle("PUT /api/v1/uploads/{id}/chunk", guard(rw, http.HandlerFunc(uh.Chunk)))
	mux.Handle("POST /api/v1/uploads/{id}/cancel", guard(rw, http.HandlerFunc(uh.Cancel)))

	// Overview.
	mux.Handle("GET /api/v1/overview", guard(rd, http.HandlerFunc(oh.Get)))

	// Downloads (programmatic API).
	if d.Downloads != nil {
		dh := &DownloadHandlers{
			Auth:          d.Auth,
			Store:         d.Downloads,
			YtDlp: download.YtDlpConfig{
				BinPath:    d.Config.YtDlpPath,
				Proxy:       d.Config.DownloadProxy,
				CookiePath: d.Config.DownloadCookiePath,
				TmpDir:      d.Config.DownloadTmpDir,
			},
			DefaultFolder: d.Config.RootFolderID,
		}
		mux.Handle("POST /api/v1/downloads", guard(rw, http.HandlerFunc(dh.Create)))
		mux.Handle("GET /api/v1/downloads", guard(rd, http.HandlerFunc(dh.List)))
		mux.Handle("DELETE /api/v1/downloads", guard(rw, http.HandlerFunc(dh.ClearFinished)))
		mux.Handle("GET /api/v1/downloads/{id}", guard(rd, http.HandlerFunc(dh.Status)))
		mux.Handle("POST /api/v1/downloads/{id}/cancel", guard(rw, http.HandlerFunc(dh.Cancel)))
		mux.Handle("POST /api/v1/downloads/{id}/retry-upload", guard(rw, http.HandlerFunc(dh.RetryUpload)))
		mux.Handle("DELETE /api/v1/downloads/{id}", guard(rw, http.HandlerFunc(dh.Delete)))
	}

	return mux
}

// serveSPA registers the root handler. When a built SPA is present in
// Config.WebDistDir it is served (with immutable caching for hashed assets and
// a client-routing fallback); otherwise the minimal placeholder landing page
// is served for "/" only.
func serveSPA(mux *http.ServeMux, d Deps) {
	if dir := d.Config.WebDistDir; dir != "" {
		if fi, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && !fi.IsDir() {
			mux.Handle("GET /", SPAHandler(dir))
			return
		}
	}
	mux.HandleFunc("GET /{$}", HomeHandler(d.Auth))
}
