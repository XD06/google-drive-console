package auth

import (
	"context"
	"net/http"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/dsk/drive-backup-console/internal/config"
)

// Service coordinates OAuth, tokens, and sessions.
type Service struct {
	OAuth    *OAuthConfig
	Tokens   *TokenStore
	Sessions *SessionManager
	States   *StateStore
	Exchange CodeExchanger
	UserInfo UserInfoClient
	Enabled  bool // false when OAuth credentials missing

	// Process-level cache for the OAuth HTTP client.
	// Rebuilt only when the token file changes (detected via mtime).
	clientMu       sync.RWMutex
	cachedClient   *http.Client
	clientTokenMt  time.Time
}

// NewService wires auth components from config.
func NewService(cfg config.Config) (*Service, error) {
	secret := cfg.SessionSecret
	if secret == "" {
		return nil, errors.New("SESSION_SECRET is required (empty in non-dev mode)")
	}
	sessions, err := NewSessionManager(secret, cfg.SecureCookie)
	if err != nil {
		return nil, err
	}

	svc := &Service{
		Tokens:   NewTokenStore(cfg.TokenPath),
		Sessions: sessions,
		States:   NewStateStore(10 * time.Minute),
		Enabled:  cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "",
	}

	if svc.Enabled {
		oauth := NewOAuthConfig(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.OAuthRedirectURL, DefaultScopes)
		svc.OAuth = oauth
		svc.Exchange = oauth
		svc.UserInfo = &GoogleUserInfo{}
	}
	return svc, nil
}

// LoginURL issues CSRF state and returns Google redirect URL.
func (s *Service) LoginURL() (string, error) {
	if !s.Enabled || s.OAuth == nil {
		return "", errors.New("oauth not configured")
	}
	state, err := s.States.Issue()
	if err != nil {
		return "", err
	}
	return s.OAuth.AuthCodeURL(state), nil
}

// CompleteLogin validates state, exchanges code, stores token, returns session.
func (s *Service) CompleteLogin(ctx context.Context, code, state string) (Session, error) {
	if !s.Enabled {
		return Session{}, errors.New("oauth not configured")
	}
	if err := s.States.Consume(state); err != nil {
		return Session{}, fmt.Errorf("state: %w", err)
	}
	if code == "" {
		return Session{}, errors.New("missing code")
	}
	ex := s.Exchange
	if ex == nil {
		return Session{}, errors.New("no code exchanger")
	}
	tok, err := ex.Exchange(ctx, code)
	if err != nil {
		return Session{}, fmt.Errorf("exchange: %w", err)
	}
	email, err := s.UserInfo.Email(ctx, tok)
	if err != nil {
		return Session{}, fmt.Errorf("userinfo: %w", err)
	}
	if err := s.Tokens.Save(StoredToken{Email: email, Token: tok}); err != nil {
		return Session{}, fmt.Errorf("save token: %w", err)
	}
	sess := Session{Email: email, Expires: time.Now().UTC().Add(sessionTTL)}
	return sess, nil
}

// CurrentUser returns session email if cookie valid and token file present.
func (s *Service) MeFromSession(sess Session) (email string, connected bool, err error) {
	if sess.Email == "" {
		return "", false, errors.New("empty session")
	}
	st, err := s.Tokens.Load()
	if err != nil {
		return sess.Email, false, nil
	}
	if st.Email != "" && st.Email != sess.Email {
		// Prefer token file email if mismatch.
		return st.Email, true, nil
	}
	return sess.Email, st.Token != nil, nil
}

// Logout clears session cookie responsibility of caller; deletes token if requested.
func (s *Service) Logout(clearToken bool) error {
	if clearToken {
		s.clientMu.Lock()
		s.cachedClient = nil
		s.clientMu.Unlock()
		return s.Tokens.Delete()
	}
	return nil
}

// Token returns stored oauth token for Drive clients.
func (s *Service) Token() (*oauth2.Token, string, error) {
	st, err := s.Tokens.Load()
	if err != nil {
		return nil, "", err
	}
	return st.Token, st.Email, nil
}

// HTTPClient returns an OAuth2 HTTP client that refreshes and persists tokens.
// Returns error if no stored token or OAuth not configured.
// The client is cached at process level and only rebuilt when the token file changes.
func (s *Service) HTTPClient(ctx context.Context) (*http.Client, error) {
	st, err := s.Tokens.Load()
	if err != nil {
		return nil, err
	}
	if st.Token == nil {
		return nil, errors.New("no token")
	}

	// Check file mtime to decide if cached client is still valid
	fi, statErr := os.Stat(s.Tokens.path)
	var mt time.Time
	if statErr == nil {
		mt = fi.ModTime()
	}

	// Fast path: return cached client if token file hasn't changed
	s.clientMu.RLock()
	if s.cachedClient != nil && mt.Equal(s.clientTokenMt) {
		c := s.cachedClient
		s.clientMu.RUnlock()
		return c, nil
	}
	s.clientMu.RUnlock()

	// Slow path: build new client
	var client *http.Client
	if s.OAuth == nil || s.OAuth.Config == nil {
		// Still allow static token for tests without full OAuth config.
		client = oauth2.NewClient(ctx, oauth2.StaticTokenSource(st.Token))
	} else {
		src := s.Tokens.TokenSource(s.OAuth.Config, st)
		client = oauth2.NewClient(ctx, src)
	}

	// Cache the new client
	s.clientMu.Lock()
	s.cachedClient = client
	s.clientTokenMt = mt
	s.clientMu.Unlock()

	return client, nil
}
