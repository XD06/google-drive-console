package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsk/drive-backup-console/internal/auth"
	"github.com/dsk/drive-backup-console/internal/config"
	"github.com/dsk/drive-backup-console/internal/drive"
)

func TestOverview_Unauthorized(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		DevMode:       true,
		SessionSecret: "test-secret-for-overview",
		TokenPath:     t.TempDir() + "/token.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	oh := &OverviewHandlers{Auth: svc}
	h := RequireSession(svc, http.HandlerFunc(oh.Get))
	req := httptest.NewRequest(http.MethodGet, "/api/overview", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestOverview_OK(t *testing.T) {
	svc, err := auth.NewService(config.Config{
		DevMode:       true,
		SessionSecret: "test-secret-for-overview",
		TokenPath:     t.TempDir() + "/token.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	drvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"user": {"emailAddress":"u@example.com","displayName":"User"},
			"storageQuota": {"limit":"100","usage":"40","usageInDrive":"30"}
		}`))
	}))
	defer drvSrv.Close()

	oh := &OverviewHandlers{
		Auth: svc,
		Drive: func(r *http.Request, svc *auth.Service) (*drive.Client, error) {
			return &drive.Client{HTTP: drvSrv.Client(), BaseURL: drvSrv.URL}, nil
		},
	}
	h := RequireSession(svc, http.HandlerFunc(oh.Get))
	req := withSession(t, svc, httptest.NewRequest(http.MethodGet, "/api/overview", nil))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var body drive.AboutInfo
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.User.Email != "u@example.com" || body.Storage.Usage != 40 || body.Storage.Limit != 100 {
		t.Fatalf("body = %+v", body)
	}
}
