package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// DefaultScopes for Drive backup + account email.
var DefaultScopes = []string{
	"https://www.googleapis.com/auth/drive",
	"https://www.googleapis.com/auth/userinfo.email",
}

// CodeExchanger abstracts token exchange for tests.
type CodeExchanger interface {
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
}

// UserInfoClient fetches the Google account email.
type UserInfoClient interface {
	Email(ctx context.Context, tok *oauth2.Token) (string, error)
}

// OAuthConfig wraps golang.org/x/oauth2 for Google.
type OAuthConfig struct {
	Config *oauth2.Config
}

// NewOAuthConfig builds a Google OAuth2 config.
func NewOAuthConfig(clientID, clientSecret, redirectURL string, scopes []string) *OAuthConfig {
	if len(scopes) == 0 {
		scopes = DefaultScopes
	}
	return &OAuthConfig{
		Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       scopes,
			Endpoint:     google.Endpoint,
		},
	}
}

// AuthCodeURL returns the Google consent URL for state.
func (o *OAuthConfig) AuthCodeURL(state string) string {
	return o.Config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// Exchange implements CodeExchanger.
func (o *OAuthConfig) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return o.Config.Exchange(ctx, code)
}

// GoogleUserInfo fetches email via OAuth2 userinfo endpoint.
type GoogleUserInfo struct {
	HTTPClient func(ctx context.Context, t *oauth2.Token) *http.Client
}

// Email implements UserInfoClient.
func (g *GoogleUserInfo) Email(ctx context.Context, tok *oauth2.Token) (string, error) {
	clientFn := g.HTTPClient
	if clientFn == nil {
		clientFn = func(ctx context.Context, t *oauth2.Token) *http.Client {
			return oauth2.NewClient(ctx, oauth2.StaticTokenSource(t))
		}
	}
	client := clientFn(ctx, tok)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return "", err
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo status %d: %s", res.StatusCode, string(body))
	}
	var payload struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Email == "" {
		return "", fmt.Errorf("userinfo missing email")
	}
	return payload.Email, nil
}
