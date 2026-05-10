package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
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
	certSerial  string
	ruleVer     string // last-known rule version on gateway
	connectedAt time.Time
	writeMu     sync.Mutex
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
		existing.closeWithPolicy("replaced by a newer connection")
	}
	reg.conns[gw.gatewayID] = gw
	reg.mu.Unlock()
}

func (reg *gwWSRegistry) remove(gw *gwWSConn) {
	if gw == nil {
		return
	}
	reg.mu.Lock()
	if current, ok := reg.conns[gw.gatewayID]; ok && current == gw {
		delete(reg.conns, gw.gatewayID)
	}
	reg.mu.Unlock()
}

func (reg *gwWSRegistry) get(gatewayID string) (*gwWSConn, bool) {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	c, ok := reg.conns[gatewayID]
	return c, ok
}

func (reg *gwWSRegistry) CloseGateway(gatewayID, reason string) bool {
	reg.mu.RLock()
	gw, ok := reg.conns[gatewayID]
	reg.mu.RUnlock()
	if !ok {
		return false
	}
	gw.closeWithPolicy(reason)
	return true
}

func (reg *gwWSRegistry) CloseCertificateSerial(serialNumber, reason string) int {
	serial := gateway.NormalizeCertificateSerial(serialNumber)
	if serial == "" {
		return 0
	}

	reg.mu.RLock()
	targets := make([]*gwWSConn, 0)
	for _, gw := range reg.conns {
		if gateway.NormalizeCertificateSerial(gw.certSerial) == serial {
			targets = append(targets, gw)
		}
	}
	reg.mu.RUnlock()

	for _, gw := range targets {
		gw.closeWithPolicy(reason)
	}
	return len(targets)
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
	return gw.writeJSON(msg)
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
	return gw.writeJSON(msg)
}

func (gw *gwWSConn) writeJSON(message gwWSMessage) bool {
	if gw == nil || gw.conn == nil {
		return false
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return false
	}
	gw.writeMu.Lock()
	defer gw.writeMu.Unlock()
	return gw.conn.WriteMessage(websocket.TextMessage, payload) == nil
}

func (gw *gwWSConn) closeWithPolicy(reason string) {
	if gw == nil || gw.conn == nil {
		return
	}
	nextReason := strings.TrimSpace(reason)
	if nextReason == "" {
		nextReason = "gateway session closed"
	}
	gw.writeMu.Lock()
	defer gw.writeMu.Unlock()
	_ = gw.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.ClosePolicyViolation, nextReason),
		time.Now().Add(5*time.Second),
	)
	_ = gw.conn.Close()
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
	maxSessionTTL := s.gatewayWebSocketMaxSessionTTL()

	// Upgrade to WebSocket
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Warn("ws: upgrade failed", "error", err, "gateway_id", gatewayID)
		return
	}
	defer conn.Close()

	s.logger.Info("ws: gateway connected", "gateway_id", gatewayID, "tenant_id", tenantID)
	recordGatewayWebSocketSession("connected")

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
		certSerial:  gatewayCertificateSerialFromRequest(r),
		ruleVer:     authData.RuleVersion,
		connectedAt: time.Now(),
	}

	if s.gwWSRegistry == nil {
		s.gwWSRegistry = newGWWSRegistry()
	}
	s.gwWSRegistry.add(gwConn)
	defer s.gwWSRegistry.remove(gwConn)
	defer func() {
		s.logger.Info("ws: gateway disconnected", "gateway_id", gatewayID,
			"session_duration", time.Since(gwConn.connectedAt).Round(time.Second))
		recordGatewayWebSocketSession("disconnected")
	}()

	// Send initial config push if gateway's rule version is outdated
	go s.wsInitialConfigPush(gwConn)

	// Start ping ticker (server → gateway keepalive)
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()
	sessionExpiresAt := time.Now().Add(maxSessionTTL)

	// Read loop
	conn.SetReadDeadline(gatewayWebSocketReadDeadline(sessionExpiresAt))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(gatewayWebSocketReadDeadline(sessionExpiresAt))
		return nil
	})

	// Spawn ping goroutine
	stopPing := make(chan struct{})
	go func() {
		for {
			select {
			case <-pingTicker.C:
				if !gwConn.writeJSON(gwWSMessage{Type: "ping"}) {
					return
				}
			case <-stopPing:
				return
			}
		}
	}()
	defer close(stopPing)

	for {
		if !time.Now().Before(sessionExpiresAt) {
			s.closeExpiredGatewayWebSocket(gwConn, maxSessionTTL)
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if !time.Now().Before(sessionExpiresAt) {
				s.closeExpiredGatewayWebSocket(gwConn, maxSessionTTL)
				return
			}
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				s.logger.Warn("ws: read error", "error", err, "gateway_id", gatewayID)
			}
			break
		}
		conn.SetReadDeadline(gatewayWebSocketReadDeadline(sessionExpiresAt))
		s.handleGWWSMessage(gwConn, message)
	}

}

func gatewayWebSocketReadDeadline(sessionExpiresAt time.Time) time.Time {
	keepaliveDeadline := time.Now().Add(90 * time.Second)
	if sessionExpiresAt.IsZero() || keepaliveDeadline.Before(sessionExpiresAt) {
		return keepaliveDeadline
	}
	return sessionExpiresAt
}

func (s *server) closeExpiredGatewayWebSocket(gw *gwWSConn, maxSessionTTL time.Duration) {
	gatewayID := ""
	if gw != nil {
		gatewayID = gw.gatewayID
		gw.closeWithPolicy("session expired; reconnect")
	}
	recordGatewayWebSocketSession("expired")
	s.loggerOrDefault().Info("ws: gateway session expired", "gateway_id", gatewayID, "max_session_ttl", maxSessionTTL)
}

func (s *server) gatewayWebSocketMaxSessionTTL() time.Duration {
	if s == nil || s.cfg.GatewayWebSocketMaxSessionTTL < time.Minute {
		return 6 * time.Hour
	}
	return s.cfg.GatewayWebSocketMaxSessionTTL
}

func (s *server) authorizeGatewayWebSocketDeviceToken(w http.ResponseWriter, r *http.Request, gatewayID string) bool {
	if s.authorizeGatewayClientCertificate(r, gatewayID) {
		recordGatewayWebSocketAuth("mtls")
		return true
	}
	if gatewayMTLSRequired(r) {
		recordGatewayWebSocketAuth("mtls_required_missing")
		writeError(w, http.StatusUnauthorized, "valid gateway client certificate required")
		return false
	}

	if strings.TrimSpace(r.URL.Query().Get("token")) != "" {
		recordGatewayWebSocketAuth("query_token_rejected")
		writeError(w, http.StatusUnauthorized, "WebSocket token query parameter is no longer supported")
		return false
	}

	if strings.TrimSpace(r.Header.Get("X-Device-Token")) != "" {
		recordGatewayWebSocketAuth("x_device_token")
		return s.authorizeGatewayDeviceToken(w, r, gatewayID)
	}
	if token, err := bearerToken(r.Header.Get("Authorization")); err == nil && strings.TrimSpace(token) != "" {
		recordGatewayWebSocketAuth("authorization_bearer")
		return s.authorizeGatewayDeviceToken(w, r, gatewayID)
	}

	recordGatewayWebSocketAuth("missing")
	return s.authorizeGatewayDeviceToken(w, r, gatewayID)
}

func (s *server) handleGWWSMessage(gw *gwWSConn, raw []byte) {
	if s.gatewayConnectionRevoked(gw) {
		if gw != nil {
			gw.closeWithPolicy("gateway revoked; reconnect denied")
		}
		recordGatewayWebSocketSession("revoked_closed")
		return
	}

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
		if s.gwWSRegistry.PushConfig(gw.gatewayID, configPayload) {
			s.logger.Info("ws: initial config pushed",
				"gateway_id", gw.gatewayID,
				"version", authzCache.Version,
			)
		}
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
	authzCache := s.buildGatewayConfigAuthzCache(tenantID, gatewayID, boundDoorIDs, time.Now().UTC())
	configPayload, _ := json.Marshal(map[string]any{
		"authz_cache": authzCache,
	})
	localPushed := false
	if s.gwWSRegistry != nil {
		if _, ok := s.gwWSRegistry.get(gatewayID); ok {
			localPushed = s.gwWSRegistry.PushConfig(gatewayID, configPayload)
		}
	}
	s.publishGatewayWebSocketPush(tenantID, gatewayID, "config_push", configPayload)
	return localPushed
}

func (s *server) closeGatewayWebSocketSessions(gatewayID, reason string) bool {
	if s == nil || s.gwWSRegistry == nil {
		return false
	}
	return s.gwWSRegistry.CloseGateway(gatewayID, reason)
}

func (s *server) closeGatewayWebSocketCertificateSerial(serialNumber, reason string) int {
	if s == nil || s.gwWSRegistry == nil {
		return 0
	}
	return s.gwWSRegistry.CloseCertificateSerial(serialNumber, reason)
}

func (s *server) gatewayConnectionRevoked(gw *gwWSConn) bool {
	if s == nil || gw == nil {
		return false
	}
	if s.gatewayIdentityStatusRevoked(gw.gatewayID) {
		return true
	}
	if s.gatewayClientSerialRevoked(gw.certSerial) {
		return true
	}
	if s.gatewaySvc == nil {
		return false
	}
	record, ok := s.gatewaySvc.FindGatewayByID(gw.tenantID, gw.gatewayID)
	return ok && gatewayCertificateStatusRevoked(record.Status)
}
