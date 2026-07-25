package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var openapiSpec []byte

// OpenAPIHandler serves the static OpenAPI 3.1 contract for the /api/v1 surface.
// It requires no authentication so tools can discover the API before holding a key.
func OpenAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write(openapiSpec)
	}
}
