package config

import (
	"os"
	"testing"
)

func withEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	prev := make(map[string]*string, len(kv))
	for k, v := range kv {
		if old, ok := os.LookupEnv(k); ok {
			cp := old
			prev[k] = &cp
		} else {
			prev[k] = nil
		}
		if err := os.Setenv(k, v); err != nil {
			t.Fatalf("setenv %s: %v", k, err)
		}
	}
	t.Cleanup(func() {
		for k, old := range prev {
			if old == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *old)
			}
		}
	})
}

func clearOAuthEnv(t *testing.T) {
	t.Helper()
	withEnv(t, map[string]string{
		"GOOGLE_CLIENT_ID":     "",
		"GOOGLE_CLIENT_SECRET": "",
		"SESSION_SECRET":       "",
		"PORT":                 "",
		"DATA_DIR":             "",
		"TOKEN_PATH":           "",
		"OAUTH_REDIRECT_URL":   "",
		"FRONTEND_ORIGIN":      "",
		"ROOT_FOLDER_ID":       "",
		"DEV_MODE":             "",
	})
}

func TestLoad_DevModeDefaults(t *testing.T) {
	clearOAuthEnv(t)
	withEnv(t, map[string]string{"DEV_MODE": "1"})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 3000 {
		t.Errorf("Port = %d, want 3000", cfg.Port)
	}
	if cfg.OAuthRedirectURL != "http://localhost:3000/oauth2/callback" {
		t.Errorf("OAuthRedirectURL = %q", cfg.OAuthRedirectURL)
	}
	if cfg.FrontendOrigin != "http://localhost:5174/" {
		t.Errorf("FrontendOrigin = %q", cfg.FrontendOrigin)
	}
	if cfg.DataDir != "./data" {
		t.Errorf("DataDir = %q", cfg.DataDir)
	}
	want := "./data" + string(os.PathSeparator) + "token.json"
	if cfg.TokenPath != want {
		t.Errorf("TokenPath = %q, want %q", cfg.TokenPath, want)
	}
	if !cfg.DevMode {
		t.Error("DevMode should be true")
	}
}

func TestLoad_RequiresSecretsWithoutDevMode(t *testing.T) {
	clearOAuthEnv(t)
	withEnv(t, map[string]string{"DEV_MODE": "0"})

	_, err := Load()
	if err == nil {
		t.Fatal("expected error without secrets")
	}
}

func TestLoad_FullConfig(t *testing.T) {
	clearOAuthEnv(t)
	withEnv(t, map[string]string{
		"PORT":                 "3001",
		"GOOGLE_CLIENT_ID":     "id",
		"GOOGLE_CLIENT_SECRET": "secret",
		"SESSION_SECRET":       "sess",
		"DATA_DIR":             `C:\tmp\drive-data`,
		"TOKEN_PATH":           `C:\tmp\drive-data\tok.json`,
		"OAUTH_REDIRECT_URL":   "http://localhost:3001/oauth2/callback",
		"ROOT_FOLDER_ID":       "root123",
		"DEV_MODE":             "0",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 3001 {
		t.Errorf("Port = %d", cfg.Port)
	}
	if cfg.GoogleClientID != "id" || cfg.GoogleClientSecret != "secret" {
		t.Error("client credentials mismatch")
	}
	if cfg.SessionSecret != "sess" {
		t.Error("session secret mismatch")
	}
	if cfg.TokenPath != `C:\tmp\drive-data\tok.json` {
		t.Errorf("TokenPath = %q", cfg.TokenPath)
	}
	if cfg.RootFolderID != "root123" {
		t.Errorf("RootFolderID = %q", cfg.RootFolderID)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	clearOAuthEnv(t)
	withEnv(t, map[string]string{
		"DEV_MODE": "1",
		"PORT":     "99999",
	})
	_, err := Load()
	if err == nil {
		t.Fatal("expected invalid port error")
	}
}
