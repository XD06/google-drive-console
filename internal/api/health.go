package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dsk/drive-backup-console/internal/auth"
)

// HealthResponse is the JSON body for GET /api/health.
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	TimeUTC string `json:"timeUtc"`
	DevMode bool   `json:"devMode,omitempty"`
}

// HealthHandler returns a simple liveness payload.
func HealthHandler(devMode bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
			return
		}
		writeJSON(w, http.StatusOK, HealthResponse{
			Status:  "ok",
			Service: "drive-backup-console",
			TimeUTC: time.Now().UTC().Format(time.RFC3339),
			DevMode: devMode,
		})
	}
}

// HomeHandler serves a minimal post-OAuth landing page (SPA comes later).
func HomeHandler(authSvc *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
			return
		}
		email := ""
		connected := false
		if authSvc != nil {
			if sess, err := authSvc.Sessions.FromRequest(r); err == nil {
				email, connected, _ = authSvc.MeFromSession(sess)
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Drive Backup Console</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 40rem; margin: 2rem auto; padding: 0 1rem; color: #1a1d24; }
    a { color: #0f766e; }
    .card { border: 1px solid #e4e6eb; border-radius: 12px; padding: 1.25rem 1.5rem; }
    .ok { color: #0f766e; font-weight: 600; }
    .muted { color: #6b7280; font-size: 0.9rem; }
    ul { line-height: 1.8; }
  </style>
</head>
<body>
  <h1>Drive Backup Console</h1>
  <div class="card">
`)
		if connected && email != "" {
			_, _ = fmt.Fprintf(w, `    <p class="ok">Signed in as %s</p>
    <p class="muted">OAuth token is stored. SPA UI is not embedded yet — use API links below.</p>
    <ul>
      <li><a href="/api/auth/me">/api/auth/me</a></li>
      <li><a href="/api/files">/api/files</a></li>
      <li><a href="/api/health">/api/health</a></li>
    </ul>
`, email)
		} else {
			_, _ = fmt.Fprint(w, `    <p>Not signed in (or session cookie missing).</p>
    <p><a href="/oauth2/login">Sign in with Google</a></p>
    <ul>
      <li><a href="/api/health">/api/health</a></li>
    </ul>
`)
		}
		_, _ = fmt.Fprint(w, `  </div>
</body>
</html>
`)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
