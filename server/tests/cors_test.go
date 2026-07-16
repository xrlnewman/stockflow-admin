package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xrlnewman/stockflow-admin/server/internal/config"
	"github.com/xrlnewman/stockflow-admin/server/internal/platform/store"
	"github.com/xrlnewman/stockflow-admin/server/internal/transport/httpapi"
)

func TestConfiguredOriginCanCallAPI(t *testing.T) {
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret", CORSOrigins: "http://localhost:4330"}, store.NewMemoryStore())
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/orders", nil)
	req.Header.Set("Origin", "http://localhost:4330")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", res.Code)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:4330" {
		t.Fatalf("expected allowed origin, got %q", got)
	}
}

func TestUnconfiguredOriginIsRejected(t *testing.T) {
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret", CORSOrigins: "http://localhost:4330"}, store.NewMemoryStore())
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/orders", nil)
	req.Header.Set("Origin", "https://untrusted.example")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unexpected CORS permission for unconfigured origin")
	}
}
