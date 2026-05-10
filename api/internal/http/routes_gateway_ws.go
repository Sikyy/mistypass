package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mistypass/cloud/api/internal/modules/event"
)

// --- Gateway WebSocket Handler ---
//
// Maintains persistent TLS connections from gateway devices.
// Enables real-time credential pushes and low-latency event uploads.

var wsUpgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   4096,
	WriteBufferSize:  4096,
	CheckOrigin:      func(r *http.Request) bool { return true }, // gateways are trusted devices
}

// gwWSConn tracks a single connected gateway WebSocket.
type gwWSConn struct {
	conn        *websocket.Conn
	gatewayID   string
	tenantID    string
	ruleVer     string // last-known rule version on gateway
	connectedAt time.Time
}

// gwWSRegistry manages active gateway WebSocket connections.
type gwWSRegistry struct {
	mu    sync.RWMutex
	conns map[string]*gwWSConn // keyed by gatewayID
}

func newGWWSRegistry() *gwWSRegistry {
	return &gwWSRegistry{
		conns: make(map[string]*gwWSConn),
	}
}

func (reg *gwWSRegistry) add(gw *gwWSConn) {
	reg.mu.Lock()
	// Close existing connection for this gateway (replaced by new one)
	if existing, ok := reg.conns[gw.gatewayID]; ok {
		existing.conn.Close()
	}
	reg.conns[gw.gatewayID] = gw
	reg.mu.Unlock()
}

func (reg *gwWSRegistry) remove(gatewayID string) {
	reg.mu.Lock()
	delete(reg.conns, gatewayID)
	reg.mu.Unlock()
}

func (reg *gwWSRegistry) get(gatewayID string) (*gwWSConn, bool) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	c, ok := reg.conns[gatewayID]
	return c, ok
}

// PushConfig sends a config update to a specific gateway via WebSocket.
// Returns false if gateway is not connected via WS.
func (reg *gwWSRegistry) PushConfig(gatewayID string, configData json.RawMessage) bool {
	reg.mu.RLock()
	gw, ok := reg.conns[gatewayID]
	reg.mu.RUnlock()
	if !ok {
		return false
	}

	msg := gwWSMessage{Type: "config_push", Data: configData}
	payload, _ := json.Marshal(msg)
	if err := gw.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return false
	}
	return true
}

// PushCredentials sends a credential update to a specific gateway via WebSocket.
func (reg *gwWSRegistry) PushCredentials(gatewayID string, credData json.RawMessage) bool {
	reg.mu.RLock()
	gw, ok := reg.conns[gatewayID]
	reg.mu.RUnlock()
	if !ok {
		return false
	}

	msg := gwWSMessage{Type: "credential_push", Data: credData}
	payload, _ := json.Marshal(msg)
	if err := gw.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return false
	}
	return true
}

// gwWSMessage is the framing protocol for gateway WebSocket messages.
type gwWSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// gatewayWebSocket handles the WebSocket upgrade and ongoing communication.
// GET /api/v1/gateway/ws
func (s *server) gatewayWebSocket(w http.ResponseWriter, r *http.Request) {
	gatewayID := strings.TrimSpace(r.Header.Get("X-Gateway-ID"))
	tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))

	if gatewayID == "" || tenantID == "" {
		http.Error(w, "missing gateway_id or tenant_id", http.StatusUnauthorized)
		return
	}

	// Verify gateway exists and token is valid
	record, ok := s.findGatewayByTenant(tenantID, gatewayID)
	if !ok {
		http.Error(w, "gateway not found", http.StatusNotFound)
		return
	}

	if !s.authorizeGatewayWebSocketDeviceToken(w, r, record.ID) {
		return
	}

	// Upgrade to WebSocket
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("ws: upgrade failed", "error", err, "gateway_id", gatewayID)
		return
	}

	s.logger.Info("ws: gateway connected", "gateway_id", gatewayID, "tenant_id", tenantID)

	// Wait for auth message (first frame)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, authPayload, err := conn.ReadMessage()
	if err != nil {
		s.logger.Warn("ws: auth read timeout", "error", err, "gateway_id", gatewayID)
		conn.Close()
		return
	}

	var authMsg gwWSMessage
	if err := json.Unmarshal(authPayload, &authMsg); err != nil || authMsg.Type != "auth" {
		s.logger.Warn("ws: invalid auth message", "gateway_id", gatewayID)
		conn.Close()
		return
	}

	var authData struct {
		GatewayID   string `json:"gateway_id"`
		TenantID    string `json:"tenant_id"`
		RuleVersion string `json:"rule_version"`
	}
	if err := json.Unmarshal(authMsg.Data, &authData); err != nil {
		conn.Close()
		return
	}
	if authData.GatewayID != "" && authData.GatewayID != gatewayID {
		conn.Close()
		return
	}
	if authData.TenantID != "" && authData.TenantID != tenantID {
		conn.Close()
		return
	}

	// Register connection
	gwConn := &gwWSConn{
		conn:        conn,
		gatewayID:   gatewayID,
		tenantID:    tenantID,
		ruleVer:     authData.RuleVersion,
		connectedAt: time.Now(),
	}

	if s.gwWSRegistry == nil {
		s.gwWSRegistry = newGWWSRegistry()
	}
	s.gwWSRegistry.add(gwConn)
	defer s.gwWSRegistry.remove(gatewayID)

	// Send initial config push if gateway's rule version is outdated
	go s.wsInitialConfigPush(gwConn)

	// Start ping ticker (server → gateway keepalive)
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Read loop
	conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	// Spawn ping goroutine
	stopPing := make(chan struct{})
	go func() {
		for {
			select {
			case <-pingTicker.C:
				pingMsg := gwWSMessage{Type: "ping"}
				payload, _ := json.Marshal(pingMsg)
				if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
					return
				}
			case <-stopPing:
				return
			}
		}
	}()
	defer close(stopPing)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				s.logger.Warn("ws: read error", "error", err, "gateway_id", gatewayID)
			}
			break
		}
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		s.handleGWWSMessage(gwConn, message)
	}

	s.logger.Info("ws: gateway disconnected", "gateway_id", gatewayID,
		"session_duration", time.Since(gwConn.connectedAt).Round(time.Second))
}

func (s *server) authorizeGatewayWebSocketDeviceToken(w http.ResponseWriter, r *http.Request, gatewayID string) bool {
	if strings.TrimSpace(r.Header.Get("X-Device-Token")) != "" {
		return s.authorizeGatewayDeviceToken(w, r, gatewayID)
	}
	if token, err := bearerToken(r.Header.Get("Authorization")); err == nil && strings.TrimSpace(token) != "" {
		return s.authorizeGatewayDeviceToken(w, r, gatewayID)
	}

	queryToken := strings.TrimSpace(r.URL.Query().Get("token"))
	if queryToken == "" {
		return s.authorizeGatewayDeviceToken(w, r, gatewayID)
	}

	// Backwards compatibility for older gateway-agent builds. New agents send
	// Authorization/X-Device-Token headers so device tokens do not appear in URLs.

	clone := r.Clone(r.Context())
	clone.Header = r.Header.Clone()
	clone.Header.Set("X-Device-Token", queryToken)
	return s.authorizeGatewayDeviceToken(w, clone, gatewayID)
}

func (s *server) handleGWWSMessage(gw *gwWSConn, raw []byte) {
	var msg gwWSMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "event_batch":
		s.handleGWWSEventBatch(gw, msg.Data)
	case "pong":
		// Keepalive response — already handled by SetReadDeadline
	default:
		s.logger.Debug("ws: unknown message type", "type", msg.Type, "gateway_id", gw.gatewayID)
	}
}

func (s *server) handleGWWSEventBatch(gw *gwWSConn, data json.RawMessage) {
	var batch struct {
		GatewayID string `json:"gateway_id"`
		TenantID  string `json:"tenant_id"`
		Events    []struct {
			EventType  string `json:"event_type"`
			LockID     string `json:"lock_id"`
			Actor      string `json:"actor"`
			Result     string `json:"result"`
			OccurredAt string `json:"occurred_at"`
		} `json:"events"`
	}
	if err := json.Unmarshal(data, &batch); err != nil {
		s.logger.Debug("ws: event batch decode failed", "error", err)
		return
	}

	// Process each event using the same ingest pipeline as HTTP
	for _, ev := range batch.Events {
		occurredAt, _ := time.Parse(time.RFC3339, ev.OccurredAt)
		if occurredAt.IsZero() {
			occurredAt = time.Now().UTC()
		}

		if s.eventSvc != nil {
			s.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
				TenantID:  gw.tenantID,
				GatewayID: gw.gatewayID,
				DoorID:    ev.LockID,
				Actor:     ev.Actor,
				Type:      ev.EventType,
				Result:    ev.Result,
				At:        occurredAt,
			})
		}
	}

	s.logger.Debug("ws: event batch processed",
		"gateway_id", gw.gatewayID,
		"count", len(batch.Events),
	)
}

// wsInitialConfigPush sends a fresh config to the gateway if its rule version is stale.
func (s *server) wsInitialConfigPush(gw *gwWSConn) {
	snapshot, err := s.gatewaySvc.PullConfig(gw.tenantID, gw.gatewayID)
	if err != nil {
		s.logger.Debug("ws: initial config push failed", "error", err, "gateway_id", gw.gatewayID)
		return
	}

	fetchedAt := time.Now().UTC()
	authzCache := s.buildGatewayConfigAuthzCache(gw.tenantID, snapshot.GatewayID, snapshot.BoundDoorIDs, fetchedAt)

	// Only push if version differs from what gateway reported
	if authzCache.Version == gw.ruleVer {
		return
	}

	configPayload, _ := json.Marshal(map[string]any{
		"authz_cache": authzCache,
	})

	if s.gwWSRegistry != nil {
		s.gwWSRegistry.PushConfig(gw.gatewayID, configPayload)
		s.logger.Info("ws: initial config pushed",
			"gateway_id", gw.gatewayID,
			"version", authzCache.Version,
		)
	}
}

func (s *server) pushAuthzCacheToConnectedGateways(tenantID string) int {
	if s.gwWSRegistry == nil || s.gatewaySvc == nil {
		return 0
	}

	gateways := s.gatewaySvc.List(tenantID)
	pushed := 0
	for _, gw := range gateways {
		if s.pushGatewayAuthzCache(gw.TenantID, gw.ID, gw.BoundDoorIDs) {
			pushed++
		}
	}
	return pushed
}

func (s *server) pushGatewayAuthzCache(tenantID, gatewayID string, boundDoorIDs []string) bool {
	if s.gwWSRegistry == nil {
		return false
	}
	if _, ok := s.gwWSRegistry.get(gatewayID); !ok {
		return false
	}

	authzCache := s.buildGatewayConfigAuthzCache(tenantID, gatewayID, boundDoorIDs, time.Now().UTC())
	configPayload, _ := json.Marshal(map[string]any{
		"authz_cache": authzCache,
	})
	return s.gwWSRegistry.PushConfig(gatewayID, configPayload)
}
