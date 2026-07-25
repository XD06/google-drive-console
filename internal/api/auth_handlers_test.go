package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/config"
)

type fakeEx struct {
	tok *oauth2.Token
}

func (f fakeEx) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return f.tok, nil
}

type fakeUI struct{ email string }

func (f fakeUI) Email(ctx context.Context, tok *oauth2.Token) (string, error) {
	return f.email, nil
}

func testAuthService(t *testing.T) *auth.Service {
	t.Helper()
	cfg := config.Config{
		GoogleClientID:     "id",
		GoogleClientSecret: "sec",
		OAuthRedirectURL:   "http://localhost:3000/oauth2/callback",
		TokenPath:          filepath.Join(t.TempDir(), "token.json"),
		SessionSecret:      "test-session-secret",
		DevMode:            true,
	}
	svc, err := auth.NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	svc.Exchange = fakeEx{tok: &oauth2.Token{AccessToken: "a", RefreshToken: "r", Expiry: time.Now().Add(time.Hour)}}
	svc.UserInfo = fakeUI{email: "tester@gmail.com"}
	return svc
}

func TestMeUnauthorized(t *testing.T) {
	h := &AuthHandlers{Auth: testAuthService(t)}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rr := httptest.NewRecorder()
	h.Me(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rr.Code)
	}
}

func TestLoginRedirect(t *testing.T) {
	h := &AuthHandlers{Auth: testAuthService(t)}
	req := httptest.NewRequest(http.MethodGet, "/oauth2/login", nil)
	rr := httptest.NewRecorder()
	h.Login(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if loc == "" || len(loc) < 20 {
		t.Fatalf("location %q", loc)
	}
}

func TestCallbackAndMe(t *testing.T) {
	svc := testAuthService(t)
	h := &AuthHandlers{Auth: svc}
	state, err := svc.States.Issue()
	if err != nil {
		t.Fatal(err)
	}
	h.FrontendOrigin = "http://localhost:5174/"
	req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?code=abc&state="+state, nil)
	rr := httptest.NewRecorder()
	h.Callback(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("callback status %d %s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "http://localhost:5174/" {
		t.Fatalf("callback Location = %q, want SPA origin", loc)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	for _, c := range cookies {
		meReq.AddCookie(c)
	}
	meRR := httptest.NewRecorder()
	h.Me(meRR, meReq)
	if meRR.Code != http.StatusOK {
		t.Fatalf("me status %d %s", meRR.Code, meRR.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(meRR.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["email"] != "tester@gmail.com" {
		t.Fatalf("body %+v", body)
	}
	if body["connected"] != true {
		t.Fatalf("connected %+v", body["connected"])
	}
}

func TestLogout(t *testing.T) {
	svc := testAuthService(t)
	h := &AuthHandlers{Auth: svc}
	_ = svc.Tokens.Save(auth.StoredToken{
		Email: "tester@gmail.com",
		Token: &oauth2.Token{AccessToken: "a", RefreshToken: "r"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout?clearToken=1", nil)
	rr := httptest.NewRecorder()
	h.Logout(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}
	if svc.Tokens.Exists() {
		t.Fatal("token should be cleared")
	}
}

func TestRequireSession(t *testing.T) {
	svc := testAuthService(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := SessionFromContext(r.Context())
		if !ok || s.Email == "" {
			t.Error("missing session in context")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	h := RequireSession(svc, inner)

	// unauthorized
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/x", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth %d", rr.Code)
	}

	// authorized
	rr2 := httptest.NewRecorder()
	_ = svc.Sessions.SetCookie(rr2, auth.Session{Email: "a@b.com", Expires: time.Now().Add(time.Hour)})
	req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	for _, c := range rr2.Result().Cookies() {
		req.AddCookie(c)
	}
	rr3 := httptest.NewRecorder()
	h.ServeHTTP(rr3, req)
	if rr3.Code != http.StatusNoContent {
		t.Fatalf("authz %d", rr3.Code)
	}
}
