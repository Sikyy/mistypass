package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func TestGetGatewayMQTTBootstrapDisabled(t *testing.T) {
	s := &server{
		cfg:        config.Config{MQTTEnabled: false},
		gatewaySvc: gateway.NewService(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateways/gw_demo_001/mqtt/bootstrap", nil)
	req = withGatewayMQTTURLParam(req, "gatewayID", "gw_demo_001")
	req = withGatewayMQTTUser(req, auth.User{ID: "u1", Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})

	res := httptest.NewRecorder()
	s.getGatewayMQTTBootstrap(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusServiceUnavailable, res.Code, res.Body.String())
	}
}

func TestGetGatewayMQTTBootstrapSuccess(t *testing.T) {
	s := &server{
		cfg: config.Config{
			MQTTEnabled:     true,
			MQTTBrokerURL:   "tcp://emqx:1883",
			MQTTTopicPrefix: "mistypass",
		},
		gatewaySvc: gateway.NewService(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gateways/gw_demo_001/mqtt/bootstrap", nil)
	req = withGatewayMQTTURLParam(req, "gatewayID", "gw_demo_001")
	req = withGatewayMQTTUser(req, auth.User{ID: "u1", Role: "tenant_admin", TenantID: "tenant_demo_jakarta"})

	res := httptest.NewRecorder()
	s.getGatewayMQTTBootstrap(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body=%s", http.StatusOK, res.Code, res.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if enabled, _ := payload["enabled"].(bool); !enabled {
		t.Fatalf("expected enabled=true, got payload=%v", payload)
	}
	if brokerURL, _ := payload["broker_url"].(string); brokerURL != "tcp://emqx:1883" {
		t.Fatalf("unexpected broker_url: %q", brokerURL)
	}
	topics, ok := payload["topics"].(map[string]any)
	if !ok {
		t.Fatalf("expected topics object, got payload=%v", payload)
	}
	publish, ok := topics["publish"].(map[string]any)
	if !ok {
		t.Fatalf("expected publish topics object, got payload=%v", payload)
	}
	accessTopic, _ := publish["access"].(string)
	wantAccess := "mistypass/tenants/tenant_demo_jakarta/gateways/gw_demo_001/events/access"
	if accessTopic != wantAccess {
		t.Fatalf("unexpected access topic: got=%q want=%q", accessTopic, wantAccess)
	}
}

func TestMQTTTopicSegment(t *testing.T) {
	if got := mqttTopicSegment("tenant_demo_jakarta"); got != "tenant_demo_jakarta" {
		t.Fatalf("unexpected unchanged segment: %q", got)
	}
	if got := mqttTopicSegment(" Tenant Demo / Jakarta "); got != "tenant-demo-jakarta" {
		t.Fatalf("unexpected normalized segment: %q", got)
	}
	if got := mqttTopicSegment("***"); got != "unknown" {
		t.Fatalf("unexpected unknown segment fallback: %q", got)
	}
}

func withGatewayMQTTURLParam(request *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	ctx := context.WithValue(request.Context(), chi.RouteCtxKey, routeCtx)
	return request.WithContext(ctx)
}

func withGatewayMQTTUser(request *http.Request, user auth.User) *http.Request {
	ctx := context.WithValue(request.Context(), authUserContextKey, user)
	return request.WithContext(ctx)
}
