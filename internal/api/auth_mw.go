package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/dsk/drive-backup-console/internal/apikey"
	"github.com/dsk/drive-backup-console/internal/auth"
)

// PrincipalKind identifies how a request was authenticated.
type PrincipalKind int

const (
	// PrincipalSession is a browser session (cookie); full read-write.
	PrincipalSession PrincipalKind = iota
	// PrincipalAPIKey is a programmatic caller authenticated by API key.
	PrincipalAPIKey
)

// Principal is the authenticated caller resolved by Authenticate. API keys
// authorize calling the API as the site owner (server-side Google token); the
// scope limits what the caller may do, not whose Drive is accessed.
type Principal struct {
	Kind    PrincipalKind
	KeyID   string       // set for API-key principals
	KeyHint string       // non-secret key identifier, for audit logging
	Scope   apikey.Scope // session principals are ScopeReadWrite
}

const principalKey ctxKey = 2

// PrincipalFromContext returns the principal set by Authenticate.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// Authenticate resolves the caller from an API key (Authorization: Bearer) or,
// failing that, a session cookie, and stores a Principal in the request
// context. Unauthenticated requests are rejected with 401. Scope is enforced
// separately by RequireScope. Handlers build the Drive client from the
// server-side Google token regardless of which principal authenticated.
func Authenticate(svc *auth.Service, keys *apikey.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if svc == nil || svc.Sessions == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "auth_unavailable", "auth not ready")
			return
		}

		// Programmatic path: API key via bearer token.
		if tok := bearerToken(r); tok != "" {
			if keys == nil {
				writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "API keys not enabled")
				return
			}
			k, ok := keys.Verify(tok)
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Invalid or revoked API key")
				return
			}
			p := Principal{Kind: PrincipalAPIKey, KeyID: k.ID, KeyHint: k.Hint, Scope: k.Scope}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, p)))
			return
		}

		// UI path: signed session cookie grants full read-write.
		sess, err := svc.Sessions.FromRequest(r)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "Sign in with Google required")
			return
		}
		p := Principal{Kind: PrincipalSession, Scope: apikey.ScopeReadWrite}
		ctx := context.WithValue(r.Context(), principalKey, p)
		ctx = context.WithValue(ctx, sessionKey, sess) // keep session available to handlers
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireScope enforces that the resolved principal satisfies the minimum
// scope. It must run after Authenticate. Session principals hold ScopeReadWrite
// and thus always pass; API-key principals with an insufficient scope get 403.
func RequireScope(min apikey.Scope, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "not_authenticated", "authentication required")
			return
		}
		if !p.Scope.Allows(min) {
			writeJSONError(w, http.StatusForbidden, "insufficient_scope", "API key lacks the required scope for this operation")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// guardScope composes Authenticate + RequireScope for a v1 handler.
func guardScope(svc *auth.Service, keys *apikey.Store, min apikey.Scope, h http.Handler) http.Handler {
	return Authenticate(svc, keys, RequireScope(min, h))
}
