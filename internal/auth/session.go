package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	sessionCookieName = "dbc_session"
	sessionTTL        = 30 * 24 * time.Hour
)

// Session holds authenticated browser session data.
type Session struct {
	Email   string    `json:"email"`
	Expires time.Time `json:"exp"`
}

// SessionManager signs and verifies HttpOnly session cookies.
type SessionManager struct {
	secret []byte
	secure bool
}

// NewSessionManager creates a manager. secret must be non-empty.
func NewSessionManager(secret string, secureCookie bool) (*SessionManager, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("session secret is empty")
	}
	return &SessionManager{secret: []byte(secret), secure: secureCookie}, nil
}

// Encode produces a signed cookie value for the session.
func (m *SessionManager) Encode(s Session) (string, error) {
	if s.Email == "" {
		return "", errors.New("session email required")
	}
	if s.Expires.IsZero() {
		s.Expires = time.Now().UTC().Add(sessionTTL)
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	sig := m.sign(payload)
	return payload + "." + sig, nil
}

// Decode verifies signature and expiry.
func (m *SessionManager) Decode(value string) (Session, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return Session{}, errors.New("invalid session format")
	}
	payload, sig := parts[0], parts[1]
	if !hmac.Equal([]byte(sig), []byte(m.sign(payload))) {
		return Session{}, errors.New("invalid session signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return Session{}, err
	}
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return Session{}, err
	}
	if s.Email == "" {
		return Session{}, errors.New("empty email in session")
	}
	if time.Now().UTC().After(s.Expires) {
		return Session{}, errors.New("session expired")
	}
	return s, nil
}

func (m *SessionManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// SetCookie writes the session cookie on the response.
func (m *SessionManager) SetCookie(w http.ResponseWriter, s Session) error {
	val, err := m.Encode(s)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   m.secure,
		Expires:  s.Expires,
		MaxAge:   int(time.Until(s.Expires).Seconds()),
	})
	return nil
}

// ClearCookie removes the session cookie.
func (m *SessionManager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   m.secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// FromRequest reads and validates the session cookie.
func (m *SessionManager) FromRequest(r *http.Request) (Session, error) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return Session{}, fmt.Errorf("no session: %w", err)
	}
	return m.Decode(c.Value)
}
