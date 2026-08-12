package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/drive"
)

// aboutCacheTTL is how long the About response is cached.
const aboutCacheTTL = 5 * time.Minute

// OverviewHandlers serves the /api/overview endpoint.
type OverviewHandlers struct {
	Auth  *auth.Service
	Drive DriveFactory

	// Process-level cache for the About API response.
	// Drive's storage quota and user info change infrequently;
	// caching eliminates a redundant API call on every page navigation.
	// Keyed by the authenticated identity so one session can never be served
	// another user's cached email / storage quota.
	aboutMu        sync.Mutex
	aboutCache     drive.AboutInfo
	aboutCacheUser string
	aboutExp       time.Time
}

func (h *OverviewHandlers) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}

	// Identify the caller so the cache is never shared across identities.
	// Session principals carry an email; API-key callers do not and simply
	// bypass the cache (a rare, cheap path).
	var who string
	if sess, ok := SessionFromContext(r.Context()); ok {
		who = sess.Email
	}

	// Check cache first (only for an identified caller, and only its own entry).
	if who != "" {
		h.aboutMu.Lock()
		if time.Now().Before(h.aboutExp) && h.aboutCacheUser == who && h.aboutCache.User.Email != "" {
			cached := h.aboutCache
			h.aboutMu.Unlock()
			writeJSON(w, http.StatusOK, cached)
			return
		}
		h.aboutMu.Unlock()
	}

	client, err := h.factory()(r, h.Auth)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	info, err := client.About(r.Context())
	if err != nil {
		writeDriveError(w, err)
		return
	}

	// Update cache for this identity only.
	if who != "" {
		h.aboutMu.Lock()
		h.aboutCache = info
		h.aboutCacheUser = who
		h.aboutExp = time.Now().Add(aboutCacheTTL)
		h.aboutMu.Unlock()
	}

	writeJSON(w, http.StatusOK, info)
}

// factory proxies to files handlers pattern
func (h *OverviewHandlers) factory() DriveFactory {
	if h.Drive != nil {
		return h.Drive
	}
	// reuse files_handlers defaultDriveFactory
	return defaultDriveFactory
}
