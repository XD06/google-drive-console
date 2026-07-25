package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/dsk/drive-backup-console/internal/apikey"
)

// APIKeyHandlers manages API keys. Every endpoint here is session-only (the
// site owner via cookie); an API key can never create or revoke keys.
type APIKeyHandlers struct {
	Keys *apikey.Store
}

// Create handles POST /api/v1/keys with { "name": "...", "scope": "read"|"readwrite" }.
// The plaintext token is returned exactly once, in the response.
func (h *APIKeyHandlers) Create(w http.ResponseWriter, r *http.Request) {
	if h.Keys == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "keys_unavailable", "API keys not enabled")
		return
	}
	var req struct {
		Name  string `json:"name"`
		Scope string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "name required")
		return
	}
	scope := apikey.Scope(strings.TrimSpace(req.Scope))
	if scope == "" {
		scope = apikey.ScopeRead // safe default: least privilege
	}
	if !scope.Valid() {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "scope must be 'read' or 'readwrite'")
		return
	}
	key, token, err := h.Keys.Create(name, scope)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "server_error", "could not create key")
		return
	}
	// Embed sanitized key metadata plus the one-time token.
	writeJSON(w, http.StatusCreated, struct {
		apikey.Key
		Token string `json:"token"`
	}{Key: key, Token: token})
}

// List handles GET /api/v1/keys, returning key metadata without secrets.
func (h *APIKeyHandlers) List(w http.ResponseWriter, r *http.Request) {
	if h.Keys == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "keys_unavailable", "API keys not enabled")
		return
	}
	keys, err := h.Keys.List()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "server_error", "could not list keys")
		return
	}
	if keys == nil {
		keys = []apikey.Key{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

// Revoke handles DELETE /api/v1/keys/{id}.
func (h *APIKeyHandlers) Revoke(w http.ResponseWriter, r *http.Request) {
	if h.Keys == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "keys_unavailable", "API keys not enabled")
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "key id required")
		return
	}
	if err := h.Keys.Revoke(id); err != nil {
		if errors.Is(err, apikey.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "not_found", "key not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "server_error", "could not revoke key")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
