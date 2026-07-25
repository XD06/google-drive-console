package auth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/dsk/drive-backup-console/internal/config"
)

type mockExchanger struct {
	tok *oauth2.Token
	err error
}

func (m mockExchanger) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tok, nil
}

type mockUserInfo struct {
	email string
	err   error
}

func (m mockUserInfo) Email(ctx context.Context, tok *oauth2.Token) (string, error) {
	return m.email, m.err
}

func TestCompleteLogin(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		GoogleClientID:     "id",
		GoogleClientSecret: "sec",
		OAuthRedirectURL:   "http://localhost:3000/oauth2/callback",
		TokenPath:          filepath.Join(dir, "token.json"),
		SessionSecret:      "sess-secret",
		DevMode:            true,
	}
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	svc.Exchange = mockExchanger{tok: &oauth2.Token{
		AccessToken:  "a",
		RefreshToken: "r",
		Expiry:       time.Now().Add(time.Hour),
	}}
	svc.UserInfo = mockUserInfo{email: "xxt@gmail.com"}

	state, err := svc.States.Issue()
	if err != nil {
		t.Fatal(err)
	}
	sess, err := svc.CompleteLogin(context.Background(), "code", state)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Email != "xxt@gmail.com" {
		t.Fatalf("email %q", sess.Email)
	}
	st, err := svc.Tokens.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Token.RefreshToken != "r" {
		t.Fatal("refresh not saved")
	}
}

func TestCompleteLoginBadState(t *testing.T) {
	cfg := config.Config{
		GoogleClientID: "id", GoogleClientSecret: "s",
		TokenPath: filepath.Join(t.TempDir(), "t.json"), SessionSecret: "x",
	}
	svc, _ := NewService(cfg)
	svc.Exchange = mockExchanger{tok: &oauth2.Token{AccessToken: "a"}}
	svc.UserInfo = mockUserInfo{email: "a@b.com"}
	_, err := svc.CompleteLogin(context.Background(), "c", "bad")
	if err == nil {
		t.Fatal("expected state error")
	}
}

func TestLoginURLRequiresConfig(t *testing.T) {
	svc, err := NewService(config.Config{SessionSecret: "x", DevMode: true, TokenPath: filepath.Join(t.TempDir(), "t.json")})
	if err != nil {
		t.Fatal(err)
	}
	if svc.Enabled {
		t.Fatal("should be disabled without client id")
	}
	if _, err := svc.LoginURL(); err == nil {
		t.Fatal("expected error")
	}
}
