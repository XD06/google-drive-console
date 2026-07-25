package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// SPAHandler serves a built single-page app from distDir with cache headers
// tuned for speed:
//
//   - /assets/* (content-hashed filenames from the bundler) → immutable, 1 year:
//     repeat visits reuse them from disk cache with zero network.
//   - other files (index.html, sw.js, manifest, icons) → no-cache: always
//     revalidate so a new deploy is picked up immediately.
//   - unknown non-API routes → index.html, so client-side routing works on
//     deep links / reloads.
//
// API and OAuth paths are never served here. They are registered as more
// specific mux patterns; if one nonetheless reaches this handler (unknown
// /api/* GET), we return 404 JSON rather than the HTML shell.
func SPAHandler(distDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(distDir))
	indexPath := filepath.Join(distDir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.URL.Path
		if strings.HasPrefix(reqPath, "/api/") || strings.HasPrefix(reqPath, "/oauth2") {
			writeJSONError(w, http.StatusNotFound, "not_found", "not found")
			return
		}

		// Anchor at "/" so path.Clean cannot escape distDir via "..".
		rel := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(reqPath, "/")), "/")
		if rel != "" {
			full := filepath.Join(distDir, filepath.FromSlash(rel))
			if fi, err := os.Stat(full); err == nil && !fi.IsDir() {
				if strings.HasPrefix(rel, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "no-cache")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// SPA fallback: serve index.html for "/" and unknown client routes.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, indexPath)
	})
}
