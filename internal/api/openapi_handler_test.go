package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAPISpecIsValidJSON(t *testing.T) {
	if len(openapiSpec) == 0 {
		t.Fatal("embedded openapi spec is empty")
	}
	var doc map[string]any
	if err := json.Unmarshal(openapiSpec, &doc); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}
	if doc["openapi"] == nil || doc["paths"] == nil {
		t.Fatal("openapi doc missing openapi/paths")
	}
}

func TestOpenAPIHandlerServes(t *testing.T) {
	rr := httptest.NewRecorder()
	OpenAPIHandler()(rr, httptest.NewRequest(http.MethodGet, "/api/v1/openapi.json", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct == "" {
		t.Fatal("missing content-type")
	}
	var doc map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &doc); err != nil {
		t.Fatalf("served body not valid JSON: %v", err)
	}
}
