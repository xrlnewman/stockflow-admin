package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xrlnewman/stockflow-admin/server/internal/config"
	"github.com/xrlnewman/stockflow-admin/server/internal/domain"
	"github.com/xrlnewman/stockflow-admin/server/internal/platform/store"
	"github.com/xrlnewman/stockflow-admin/server/internal/transport/httpapi"
)

func TestHealthEndpointReportsDependencies(t *testing.T) {
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret"}, store.NewMemoryStore())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
}

func TestProtectedEndpointRequiresBearerToken(t *testing.T) {
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret"}, store.NewMemoryStore())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}

func TestLoginReturnsBearerTokenAndMe(t *testing.T) {
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret"}, store.NewMemoryStore())
	body, _ := json.Marshal(map[string]string{"phone": "13800000000", "password": "demo123456"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	r.ServeHTTP(loginRes, loginReq)
	if loginRes.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", loginRes.Code, loginRes.Body.String())
	}
	var envelope struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginRes.Body.Bytes(), &envelope); err != nil || envelope.Data.AccessToken == "" {
		t.Fatalf("expected access token, body=%s", loginRes.Body.String())
	}
	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+envelope.Data.AccessToken)
	meRes := httptest.NewRecorder()
	r.ServeHTTP(meRes, meReq)
	if meRes.Code != http.StatusOK {
		t.Fatalf("expected me 200, got %d", meRes.Code)
	}
}

func TestCustomerCannotAssignOrder(t *testing.T) {
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret"}, store.NewMemoryStore())
	body, _ := json.Marshal(map[string]string{"phone": "13800000000", "password": "demo123456"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	r.ServeHTTP(loginRes, loginReq)
	var envelope struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(loginRes.Body.Bytes(), &envelope)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/orders/order-1/assign", bytes.NewBufferString(`{"technicianId":"tech-demo"}`))
	req.Header.Set("Authorization", "Bearer "+envelope.Data.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", res.Code)
	}
}

func TestTechnicianCannotOperateAnotherTechniciansOrder(t *testing.T) {
	st := store.NewMemoryStore()
	st.SaveOrder(domain.Order{ID: "order-owned-by-other", TechnicianID: "tech-other", State: domain.OrderAssigned}, "")
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret"}, st)
	body, _ := json.Marshal(map[string]string{"phone": "13700000000", "password": "demo123456"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	r.ServeHTTP(loginRes, loginReq)
	var envelope struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(loginRes.Body.Bytes(), &envelope)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workbench/orders/order-owned-by-other/accept", nil)
	req.Header.Set("Authorization", "Bearer "+envelope.Data.AccessToken)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", res.Code)
	}
}

func TestCustomerCanCreateDemoOrderWithIdempotencyKey(t *testing.T) {
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret"}, store.NewMemoryStore())
	body, _ := json.Marshal(map[string]string{"phone": "13800000000", "password": "demo123456"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	r.ServeHTTP(loginRes, loginReq)
	var loginEnvelope struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(loginRes.Body.Bytes(), &loginEnvelope)
	date := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")
	orderBody := []byte(fmt.Sprintf(`{"serviceId":"svc-clean","addressId":"addr-demo","date":"%s","slotId":"slot-demo-am","remark":"请提前联系"}`, date))
	orderReq := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(orderBody))
	orderReq.Header.Set("Content-Type", "application/json")
	orderReq.Header.Set("Authorization", "Bearer "+loginEnvelope.Data.AccessToken)
	orderReq.Header.Set("Idempotency-Key", "order-idempotency-demo")
	orderRes := httptest.NewRecorder()
	r.ServeHTTP(orderRes, orderReq)
	if orderRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", orderRes.Code, orderRes.Body.String())
	}
	retryReq := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(orderBody))
	retryReq.Header.Set("Content-Type", "application/json")
	retryReq.Header.Set("Authorization", "Bearer "+loginEnvelope.Data.AccessToken)
	retryReq.Header.Set("Idempotency-Key", "order-idempotency-demo")
	retryRes := httptest.NewRecorder()
	r.ServeHTTP(retryRes, retryReq)
	if retryRes.Code != http.StatusCreated {
		t.Fatalf("expected idempotent retry 201, got %d", retryRes.Code)
	}
}

func TestCustomerCanListOwnOrders(t *testing.T) {
	st := store.NewMemoryStore()
	st.SaveOrder(domain.Order{ID: "customer-order", UserID: "user-demo", State: domain.OrderPendingDispatch}, "")
	st.SaveOrder(domain.Order{ID: "other-order", UserID: "another-user", State: domain.OrderCompleted}, "")
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret"}, st)
	body, _ := json.Marshal(map[string]string{"phone": "13800000000", "password": "demo123456"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	r.ServeHTTP(loginRes, loginReq)
	var loginEnvelope struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(loginRes.Body.Bytes(), &loginEnvelope)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders?page=1&pageSize=20", nil)
	req.Header.Set("Authorization", "Bearer "+loginEnvelope.Data.AccessToken)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var envelope struct {
		Data struct {
			List []domain.Order `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(envelope.Data.List) != 1 || envelope.Data.List[0].ID != "customer-order" {
		t.Fatalf("expected only customer's order, got %+v", envelope.Data.List)
	}
}

func TestCustomerConfirmCompletesOwnOrder(t *testing.T) {
	st := store.NewMemoryStore()
	st.SaveOrder(domain.Order{ID: "order-awaiting-confirm", UserID: "user-demo", State: domain.OrderPendingCustomerConfirmation}, "")
	r := httpapi.NewRouter(config.Config{JWTSecret: "test-secret"}, st)
	body, _ := json.Marshal(map[string]string{"phone": "13800000000", "password": "demo123456"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	r.ServeHTTP(loginRes, loginReq)
	var loginEnvelope struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(loginRes.Body.Bytes(), &loginEnvelope)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/order-awaiting-confirm/confirm", nil)
	req.Header.Set("Authorization", "Bearer "+loginEnvelope.Data.AccessToken)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var envelope struct {
		Data struct {
			State domain.OrderState `json:"state"`
		} `json:"data"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &envelope)
	if envelope.Data.State != domain.OrderCompleted {
		t.Fatalf("expected completed, got %s: %s", envelope.Data.State, res.Body.String())
	}
}
