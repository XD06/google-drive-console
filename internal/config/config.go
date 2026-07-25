package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

// Config holds process configuration loaded from the environment.
type Config struct {
	Port               int
	GoogleClientID     string
	GoogleClientSecret string
	OAuthRedirectURL   string
	// FrontendOrigin is where OAuth callback redirects after SetCookie (SPA URL).
	FrontendOrigin string
	DataDir        string
	TokenPath      string
	// APIKeysPath is where programmatic API keys are persisted (JSON).
	APIKeysPath   string
	SessionSecret string
	RootFolderID  string
	// DevMode allows running without full OAuth secrets (health-only smoke).
	DevMode bool
	// SecureCookie controls the Secure flag on session cookies.
	// Defaults to !DevMode (true in production, false in dev).
	SecureCookie bool
	// HTTPProxy is the proxy URL for reaching Google APIs (e.g. socks5://127.0.0.1:10808).
	HTTPProxy string
	// WebDistDir is the directory holding the built SPA (index.html + assets/).
	// When present, the server serves the SPA with immutable caching for hashed
	// assets and a client-routing fallback to index.html. Empty/missing → the
	// minimal placeholder landing page is served instead.
	WebDistDir string
}

// Load reads configuration from environment variables.
// Required for full server mode: GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, SESSION_SECRET.
// When DEV_MODE=1, OAuth secrets may be empty so health checks work during scaffold.
func Load() (Config, error) {
	cfg := Config{
		Port:               envInt("PORT", 3000),
		GoogleClientID:     strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		GoogleClientSecret: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")),
		OAuthRedirectURL:   envString("OAUTH_REDIRECT_URL", "http://localhost:3000/oauth2/callback"),
		FrontendOrigin:     envString("FRONTEND_ORIGIN", "http://localhost:5174/"),
		DataDir:            envString("DATA_DIR", "./data"),
		SessionSecret:      strings.TrimSpace(os.Getenv("SESSION_SECRET")),
		RootFolderID:       strings.TrimSpace(os.Getenv("ROOT_FOLDER_ID")),
		DevMode:            envBool("DEV_MODE", false),
		HTTPProxy:          strings.TrimSpace(os.Getenv("HTTP_PROXY")),
		WebDistDir:         envString("WEB_DIST_DIR", "./web/dist"),
	}
	cfg.SecureCookie = envBool("SECURE_COOKIE", !cfg.DevMode)
	cfg.FrontendOrigin = normalizeFrontendOrigin(cfg.FrontendOrigin)

	tokenPath := strings.TrimSpace(os.Getenv("TOKEN_PATH"))
	if tokenPath == "" {
		tokenPath = cfg.DataDir + string(os.PathSeparator) + "token.json"
	}
	cfg.TokenPath = tokenPath

	apiKeysPath := strings.TrimSpace(os.Getenv("APIKEYS_PATH"))
	if apiKeysPath == "" {
		apiKeysPath = cfg.DataDir + string(os.PathSeparator) + "apikeys.json"
	}
	cfg.APIKeysPath = apiKeysPath

	if cfg.Port < 1 || cfg.Port > 65535 {
		return Config{}, fmt.Errorf("PORT must be between 1 and 65535, got %d", cfg.Port)
	}

	if cfg.DevMode && cfg.SessionSecret == "" {
		// Generate a random ephemeral secret for each startup.
		// This is NOT a public known key — it changes every restart.
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return Config{}, fmt.Errorf("generate dev session secret: %w", err)
		}
		cfg.SessionSecret = hex.EncodeToString(b)
		log.Printf("WARN: DEV_MODE using ephemeral session secret (restart will invalidate sessions)")
	}

	if !cfg.DevMode {
		if cfg.GoogleClientID == "" {
			return Config{}, fmt.Errorf("GOOGLE_CLIENT_ID is required (or set DEV_MODE=1 for scaffold)")
		}
		if cfg.GoogleClientSecret == "" {
			return Config{}, fmt.Errorf("GOOGLE_CLIENT_SECRET is required (or set DEV_MODE=1 for scaffold)")
		}
		if cfg.SessionSecret == "" {
			return Config{}, fmt.Errorf("SESSION_SECRET is required (or set DEV_MODE=1 for scaffold)")
		}
	}

	return cfg, nil
}

func envString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

// normalizeFrontendOrigin ensures a usable absolute URL ending with /.
func normalizeFrontendOrigin(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "http://localhost:5174/"
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "http://" + s
	}
	if !strings.HasSuffix(s, "/") {
		s += "/"
	}
	return s
}
