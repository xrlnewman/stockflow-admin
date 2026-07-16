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

func TestStockFlowStoresEndpointUsesEnvelope(t *testing.T) {
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret"}, store.NewMemoryStore())
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"phone":"13800000000","password":"demo123456"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	r.ServeHTTP(loginRes, loginReq)
	var login struct{ Data struct{ AccessToken string `json:"accessToken"` } `json:"data"` }
	if err := json.Unmarshal(loginRes.Body.Bytes(), &login); err != nil || login.Data.AccessToken == "" {
		t.Fatalf("expected login token, body=%s", loginRes.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stores", nil)
	req.Header.Set("Authorization", "Bearer "+login.Data.AccessToken)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected stores 200, got %d: %s", res.Code, res.Body.String())
	}
	var envelope struct{ Code int `json:"code"`; Data struct{ List []map[string]any `json:"list"` } `json:"data"` }
	if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil || envelope.Code != 0 || len(envelope.Data.List) != 2 {
		t.Fatalf("expected two StockFlow stores, body=%s", res.Body.String())
	}
}
