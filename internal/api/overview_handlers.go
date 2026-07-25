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
	aboutMu    sync.Mutex
	aboutCache drive.AboutInfo
	aboutExp   time.Time
}

func (h *OverviewHandlers) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}

	// Check cache first
	h.aboutMu.Lock()
	if time.Now().Before(h.aboutExp) && h.aboutCache.User.Email != "" {
		cached := h.aboutCache
		h.aboutMu.Unlock()
		writeJSON(w, http.StatusOK, cached)
		return
	}
	h.aboutMu.Unlock()

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

	// Update cache
	h.aboutMu.Lock()
	h.aboutCache = info
	h.aboutExp = time.Now().Add(aboutCacheTTL)
	h.aboutMu.Unlock()

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
