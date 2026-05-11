package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSClient maintains a persistent WebSocket connection to the Cloud API.
// It receives real-time credential pushes and sends events, replacing HTTP polling.
// On disconnect it reconnects with exponential backoff and falls back to HTTP.
type WSClient struct {
	logger *slog.Logger
	agent  *Agent
	url    string // wss://host/api/v1/gateway/ws

	mu        sync.Mutex
	conn      *websocket.Conn
	connected bool
	stopCh    chan struct{}
	done      chan struct{}
}

// WSMessage is the framing protocol for WebSocket messages.
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Inbound message types (cloud → gateway)
const (
	WSTypeCredentialPush = "credential_push" // full or incremental credential update
	WSTypePing           = "ping"
	WSTypeConfigPush     = "config_push" // full config push (same as config/pull response)
)

// Outbound message types (gateway → cloud)
const (
	WSTypeAuth       = "auth"        // first message: authenticate
	WSTypeEventBatch = "event_batch" // push access events
	WSTypePong       = "pong"
)

// WSCredentialPush is the payload for credential_push messages.
type WSCredentialPush struct {
	Version     string       `json:"version"`
	AccessRules []AccessRule `json:"access_rules"`
	Incremental bool         `json:"incremental"` // false = full replace, true = merge
}

func NewWSClient(logger *slog.Logger, agent *Agent, wsURL string) *WSClient {
	return &WSClient{
		logger: logger,
		agent:  agent,
		url:    wsURL,
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (ws *WSClient) Start() {
	go ws.connectLoop()
}

func (ws *WSClient) Stop() {
	close(ws.stopCh)
	ws.mu.Lock()
	if ws.conn != nil {
		ws.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shutdown"))
		ws.conn.Close()
	}
	ws.mu.Unlock()
	<-ws.done
}

func (ws *WSClient) IsConnected() bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.connected
}

// SendEvents sends an event batch over the WebSocket. Returns false if not connected.
func (ws *WSClient) SendEvents(events []AccessEvent) bool {
	data, _ := json.Marshal(map[string]any{
		"gateway_id": ws.agent.gatewayID,
		"tenant_id":  ws.agent.tenantID,
		"events":     events,
	})
	msg := WSMessage{Type: WSTypeEventBatch, Data: data}
	payload, _ := json.Marshal(msg)

	return ws.writeMessage(payload)
}

// writeMessage sends a text frame while holding the write lock.
// gorilla/websocket does not support concurrent writers, so all writes
// (SendEvents, pong replies, auth frame) must go through this method.
func (ws *WSClient) writeMessage(payload []byte) bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if !ws.connected || ws.conn == nil {
		return false
	}
	if err := ws.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		ws.logger.Debug("ws: write failed", "error", err)
		return false
	}
	return true
}

func (ws *WSClient) connectLoop() {
	defer close(ws.done)

	backoff := time.Second
	const maxBackoff = 2 * time.Minute

	for {
		select {
		case <-ws.stopCh:
			return
		default:
		}

		if err := ws.connect(); err != nil {
			ws.logger.Debug("ws: connect failed", "error", err, "retry_in", backoff)
			select {
			case <-time.After(backoff):
			case <-ws.stopCh:
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Connected — reset backoff and run read loop
		backoff = time.Second
		ws.readLoop()

		// readLoop exited = disconnected
		ws.mu.Lock()
		ws.connected = false
		if ws.conn != nil {
			ws.conn.Close()
			ws.conn = nil
		}
		ws.mu.Unlock()
		ws.logger.Info("ws: disconnected, reconnecting...")
	}
}

func (ws *WSClient) connect() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  ws.tlsConfig(),
	}

	token := ws.agent.activeToken()
	url := ws.url

	header := http.Header{}
	header.Set("X-Gateway-ID", ws.agent.gatewayID)
	header.Set("X-Tenant-ID", ws.agent.tenantID)
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
		header.Set("X-Device-Token", token)
	}

	conn, resp, err := dialer.Dial(url, header)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("ws dial: %w (status=%d)", err, resp.StatusCode)
		}
		return fmt.Errorf("ws dial: %w", err)
	}

	// Send auth message as first frame
	authMsg := WSMessage{Type: WSTypeAuth}
	authData, _ := json.Marshal(map[string]string{
		"gateway_id":   ws.agent.gatewayID,
		"tenant_id":    ws.agent.tenantID,
		"rule_version": ws.agent.ruleVersion,
	})
	authMsg.Data = authData
	payload, _ := json.Marshal(authMsg)
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		conn.Close()
		return fmt.Errorf("ws auth: %w", err)
	}

	ws.mu.Lock()
	ws.conn = conn
	ws.connected = true
	ws.mu.Unlock()

	ws.logger.Info("ws: connected", "url", ws.url)
	return nil
}

func (ws *WSClient) readLoop() {
	// Snapshot conn under lock so Stop() closing ws.conn can't race with reads.
	ws.mu.Lock()
	conn := ws.conn
	ws.mu.Unlock()
	if conn == nil {
		return
	}

	// Set read deadline for keepalive (server should ping every 30s)
	conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		select {
		case <-ws.stopCh:
			return
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if closeErr, ok := err.(*websocket.CloseError); ok && strings.Contains(strings.ToLower(closeErr.Text), "session expired") {
				ws.logger.Info("ws: session expired by server, reconnecting")
				return
			}
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway) {
				ws.logger.Warn("ws: read error", "error", err)
			}
			return
		}

		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		ws.handleMessage(message)
	}
}

func (ws *WSClient) handleMessage(raw []byte) {
	var msg WSMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		ws.logger.Debug("ws: invalid message", "error", err)
		return
	}

	switch msg.Type {
	case WSTypeCredentialPush:
		ws.handleCredentialPush(msg.Data)
	case WSTypeConfigPush:
		ws.handleConfigPush(msg.Data)
	case WSTypePing:
		pong := WSMessage{Type: WSTypePong}
		payload, _ := json.Marshal(pong)
		ws.writeMessage(payload)
	default:
		ws.logger.Debug("ws: unknown message type", "type", msg.Type)
	}
}

func (ws *WSClient) handleCredentialPush(data json.RawMessage) {
	var push WSCredentialPush
	if err := json.Unmarshal(data, &push); err != nil {
		ws.logger.Warn("ws: credential push decode failed", "error", err)
		return
	}

	ws.agent.mu.Lock()
	if push.Incremental {
		// Merge: update or append rules from the push, keyed by (CredentialType, CredentialData, UserID)
		for _, incoming := range push.AccessRules {
			found := false
			for i, existing := range ws.agent.accessRules {
				if existing.CredentialType == incoming.CredentialType &&
					existing.CredentialData == incoming.CredentialData &&
					existing.UserID == incoming.UserID {
					ws.agent.accessRules[i] = incoming
					found = true
					break
				}
			}
			if !found {
				ws.agent.accessRules = append(ws.agent.accessRules, incoming)
			}
		}
	} else {
		ws.agent.accessRules = push.AccessRules
	}
	ws.agent.ruleVersion = push.Version
	ws.agent.rulesUpdatedAt = time.Now().UTC()
	ws.agent.mu.Unlock()

	ws.logger.Info("ws: credentials updated",
		"version", push.Version,
		"rules", len(push.AccessRules),
		"incremental", push.Incremental,
	)
}

func (ws *WSClient) handleConfigPush(data json.RawMessage) {
	// Same structure as config/pull response
	var result struct {
		AuthzCache struct {
			Version     string       `json:"version"`
			AccessRules []AccessRule `json:"access_rules"`
		} `json:"authz_cache"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		ws.logger.Warn("ws: config push decode failed", "error", err)
		return
	}

	ws.agent.mu.Lock()
	ws.agent.accessRules = result.AuthzCache.AccessRules
	ws.agent.ruleVersion = result.AuthzCache.Version
	ws.agent.rulesUpdatedAt = time.Now().UTC()
	ws.agent.mu.Unlock()

	ws.logger.Info("ws: config updated via push",
		"version", result.AuthzCache.Version,
		"rules", len(result.AuthzCache.AccessRules),
	)
}

func (ws *WSClient) tlsConfig() *tls.Config {
	if ws.agent.tlsPinSHA256 == "" {
		return nil
	}
	// Certificate pinning — reuse the same logic as HTTP client
	return ws.agent.pinnedTLSConfig()
}
