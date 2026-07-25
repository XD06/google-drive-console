package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/dsk/drive-backup-console/internal/auth"
)

// AuthHandlers serves OAuth and session endpoints.
type AuthHandlers struct {
	Auth           *auth.Service
	FrontendOrigin string // post-login redirect; empty falls back to /
}

func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	if h.Auth == nil || !h.Auth.Enabled {
		writeJSONError(w, http.StatusServiceUnavailable, "oauth_not_configured", "Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET")
		return
	}
	url, err := h.Auth.LoginURL()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "oauth_error", err.Error())
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *AuthHandlers) Callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	if h.Auth == nil || !h.Auth.Enabled {
		writeJSONError(w, http.StatusServiceUnavailable, "oauth_not_configured", "OAuth not configured")
		return
	}
	q := r.URL.Query()
	if errStr := q.Get("error"); errStr != "" {
		writeJSONError(w, http.StatusBadRequest, "oauth_denied", errStr)
		return
	}
	sess, err := h.Auth.CompleteLogin(r.Context(), q.Get("code"), q.Get("state"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "oauth_callback_failed", err.Error())
		return
	}
	if err := h.Auth.Sessions.SetCookie(w, sess); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	dest := strings.TrimSpace(h.FrontendOrigin)
	if dest == "" {
		dest = "/"
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

func (h *AuthHandlers) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	if h.Auth == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "auth_unavailable", "auth not ready")
		return
	}
	sess, err := h.Auth.Sessions.FromRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
		return
	}
	email, connected, _ := h.Auth.MeFromSession(sess)
	writeJSON(w, http.StatusOK, map[string]any{
		"email":     email,
		"connected": connected,
	})
}

func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	if h.Auth == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "auth_unavailable", "auth not ready")
		return
	}
	clearToken := r.URL.Query().Get("clearToken") == "1"
	_ = h.Auth.Logout(clearToken)
	h.Auth.Sessions.ClearCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"loggedOut": true,
		"at":        time.Now().UTC().Format(time.RFC3339),
	})
}
