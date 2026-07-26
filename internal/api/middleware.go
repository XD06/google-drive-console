package api

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strconv"
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
	gz          *gzip.Writer // non-nil once compression is engaged
	wroteHeader bool
	skip        bool
}

// isCompressible reports whether a Content-Type benefits from gzip.
// Already-compressed formats (images, fonts, media, archives) are excluded.
func isCompressible(ct string) bool {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.TrimSpace(strings.ToLower(ct))
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	switch ct {
	case "application/json", "application/javascript", "application/x-javascript",
		"application/xml", "application/manifest+json", "image/svg+xml":
		return true
	}
	return false
}

// WriteHeader decides — based on the response headers the handler set — whether
// this response is worth compressing: skip non-compressible types, responses
// that already carry an encoding, byte-range replies (gzip would corrupt
// Content-Range semantics), and tiny payloads where the gzip header overhead
// outweighs the savings.
func (g *gzipResponseWriter) WriteHeader(code int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true
	h := g.ResponseWriter.Header()
	switch {
	case !isCompressible(h.Get("Content-Type")),
		h.Get("Content-Encoding") != "",
		h.Get("Content-Range") != "":
		g.skip = true
	default:
		if cl := h.Get("Content-Length"); cl != "" {
			if n, err := strconv.Atoi(cl); err == nil && n < 1024 {
				g.skip = true
			}
		}
	}
	if !g.skip {
		g.gz = gzipPool.Get().(*gzip.Writer)
		g.gz.Reset(g.ResponseWriter)
		h.Set("Content-Encoding", "gzip")
		h.Del("Content-Length") // length changes after compression
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		// Mirror net/http: sniff the type before the implicit 200 so the
		// compressibility decision sees a real Content-Type.
		if g.Header().Get("Content-Type") == "" {
			g.Header().Set("Content-Type", http.DetectContentType(b))
		}
		g.WriteHeader(http.StatusOK)
	}
	if g.skip {
		return g.ResponseWriter.Write(b)
	}
	return g.gz.Write(b)
}

func (g *gzipResponseWriter) Flush() {
	if g.gz != nil {
		g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// close finalizes the gzip stream (if engaged) and returns the writer to the pool.
func (g *gzipResponseWriter) close() {
	if g.gz != nil {
		g.gz.Close()
		gzipPool.Put(g.gz)
		g.gz = nil
	}
}

// GzipMiddleware compresses JSON and text responses for clients that accept it.
// Binary stream endpoints are skipped by path up front; everything else is
// decided per-response by Content-Type/size at header-write time, so
// already-compressed assets (images, fonts) are never double-compressed.
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

		grw := &gzipResponseWriter{ResponseWriter: w}
		defer grw.close()
		next.ServeHTTP(grw, r)
	})
}
