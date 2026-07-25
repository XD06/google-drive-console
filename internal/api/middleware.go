package api

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/dsk/drive-backup-console/internal/auth"
)

type ctxKey int

const sessionKey ctxKey = 1

// SessionFromContext returns the authenticated session if RequireSession ran.
func SessionFromContext(ctx context.Context) (auth.Session, bool) {
	s, ok := ctx.Value(sessionKey).(auth.Session)
	return s, ok
}

// RequireSession rejects unauthenticated API requests.
func RequireSession(svc *auth.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "auth_unavailable", "auth not ready")
			return
		}
		sess, err := svc.Sessions.FromRequest(r)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
			return
		}
		ctx := context.WithValue(r.Context(), sessionKey, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// BodyLimit wraps a handler with a request body size limit.
// Returns 413 Request Entity Too Large when the body exceeds maxBytes.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// ---------- Gzip compression ----------

var gzipPool = sync.Pool{
	New: func() any {
		gz, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return gz
	},
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) { return g.gz.Write(b) }
func (g *gzipResponseWriter) Flush() {
	g.gz.Flush()
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// GzipMiddleware compresses JSON and text responses for clients that accept it.
// It skips binary streams (downloads, uploads, thumbnails) to avoid double-
// compressing or buffering large payloads.
func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if client doesn't accept gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		// Skip binary stream endpoints (download, thumbnail, upload chunks, zip)
		p := r.URL.Path
		if strings.Contains(p, "/download") || strings.Contains(p, "/thumbnail") ||
			strings.Contains(p, "/chunk") || strings.HasSuffix(p, "/zip") {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(w)
		defer func() {
			gz.Close()
			gzipPool.Put(gz)
		}()

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length") // length changes after compression
		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, gz: gz}, r)
	})
}
