package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xrlnewman/stockflow-admin/server/internal/config"
	"github.com/xrlnewman/stockflow-admin/server/internal/platform/store"
	"github.com/xrlnewman/stockflow-admin/server/internal/transport/httpapi"
)

func TestStockFlowDashboardUsesEnvelope(t *testing.T) {
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret", CORSOrigins: "*"}, store.NewMemoryStore())
	login := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"phone":"13900000000","password":"demo123456"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(login, req)
	if login.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", login.Code, login.Body.String())
	}
	var tokenBody struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &tokenBody); err != nil {
		t.Fatal(err)
	}
	if tokenBody.Data.AccessToken == "" {
		t.Fatalf("missing token: %s", login.Body.String())
	}
	for _, path := range []string{"/api/v1/dashboard", "/api/v1/warehouses", "/api/v1/products", "/api/v1/stocks/alerts", "/api/v1/purchase-orders", "/api/v1/sales-orders", "/api/v1/stock-movements"} {
		res := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+tokenBody.Data.AccessToken)
		r.ServeHTTP(res, request)
		if res.Code != http.StatusOK {
			t.Fatalf("%s returned %d: %s", path, res.Code, res.Body.String())
		}
	}
}

func TestStockFlowHealthEnvelope(t *testing.T) {
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret", CORSOrigins: "*"}, store.NewMemoryStore())
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	var body struct {
		Code any            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != float64(0) || body.Data["status"] != "ok" {
		t.Fatalf("unexpected envelope: %s", res.Body.String())
	}
}
