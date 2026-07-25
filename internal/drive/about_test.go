package drive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_About(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/about") {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("fields") != "user,storageQuota" {
			t.Fatalf("fields = %q", r.URL.Query().Get("fields"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"user": {"emailAddress":"a@example.com","displayName":"Ada"},
			"storageQuota": {"limit":"16106127360","usage":"1234567890","usageInDrive":"1000"}
		}`))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	info, err := c.About(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.User.Email != "a@example.com" || info.User.DisplayName != "Ada" {
		t.Fatalf("user = %+v", info.User)
	}
	if info.Storage.Limit != 16106127360 || info.Storage.Usage != 1234567890 || info.Storage.UsageInDrive != 1000 {
		t.Fatalf("storage = %+v", info.Storage)
	}
}
