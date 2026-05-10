package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mistypass/cloud/api/internal/bus"
)

func TestGatewayWebSocketFanoutDeliversLocalConfigPush(t *testing.T) {
	registry := newGWWSRegistry()
	s := &server{gwWSRegistry: registry, instanceID: "api-target"}
	serverConnCh := make(chan *websocket.Conn, 1)

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		registry.add(&gwWSConn{
			conn:        conn,
			gatewayID:   "gw_demo_001",
			tenantID:    "tenant_demo_jakarta",
			connectedAt: time.Now().UTC(),
		})
		serverConnCh <- conn
	}))
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer clientConn.Close()

	var serverConn *websocket.Conn
	select {
	case serverConn = <-serverConnCh:
	case <-time.After(time.Second):
		t.Fatal("server websocket connection not registered")
	}
	defer serverConn.Close()

	push := bus.GatewayWebSocketPush{
		RequestID: "push_001",
		TenantID:  "tenant_demo_jakarta",
		GatewayID: "gw_demo_001",
		Type:      "config_push",
		Data:      json.RawMessage(`{"authz_cache":{"version":"cache-v1"}}`),
		IssuedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.Marshal(push)
	if err != nil {
		t.Fatalf("marshal push: %v", err)
	}

	s.handleGatewayWebSocketPush(raw)

	if err := clientConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, message, err := clientConn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message: %v", err)
	}
	var frame gwWSMessage
	if err := json.Unmarshal(message, &frame); err != nil {
		t.Fatalf("decode websocket frame: %v", err)
	}
	if frame.Type != "config_push" {
		t.Fatalf("expected config_push frame, got %s", frame.Type)
	}
	if !strings.Contains(string(frame.Data), "cache-v1") {
		t.Fatalf("expected pushed config payload, got %s", string(frame.Data))
	}
}

func TestGatewayWebSocketFanoutIgnoresNonLocalGateway(t *testing.T) {
	s := &server{gwWSRegistry: newGWWSRegistry(), instanceID: "api-target"}
	push := bus.GatewayWebSocketPush{
		RequestID: "push_002",
		TenantID:  "tenant_demo_jakarta",
		GatewayID: "gw_not_connected",
		Type:      "config_push",
		Data:      json.RawMessage(`{"authz_cache":{"version":"cache-v1"}}`),
		IssuedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.Marshal(push)
	if err != nil {
		t.Fatalf("marshal push: %v", err)
	}

	s.handleGatewayWebSocketPush(raw)
}

func TestGatewayWebSocketFanoutSkipsOriginInstance(t *testing.T) {
	s := &server{gwWSRegistry: newGWWSRegistry(), instanceID: "api-origin"}
	push := bus.GatewayWebSocketPush{
		RequestID:        "push_003",
		OriginInstanceID: "api-origin",
		TenantID:         "tenant_demo_jakarta",
		GatewayID:        "gw_demo_001",
		Type:             "config_push",
		Data:             json.RawMessage(`{"authz_cache":{"version":"cache-v1"}}`),
		IssuedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.Marshal(push)
	if err != nil {
		t.Fatalf("marshal push: %v", err)
	}

	s.handleGatewayWebSocketPush(raw)
}
