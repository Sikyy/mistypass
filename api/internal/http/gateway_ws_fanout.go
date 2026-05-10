package httpx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/mistypass/cloud/api/internal/bus"
)

const gatewayWebSocketPushSubject = "gateway.ws.push"

// startGatewayWebSocketPushSubscriber receives cross-replica WebSocket push
// messages. Only the API instance that owns the target gateway connection will
// deliver the payload locally.
func (s *server) startGatewayWebSocketPushSubscriber(natsURL, subjectPrefix string) error {
	if strings.TrimSpace(natsURL) == "" {
		s.loggerOrDefault().Info("gateway websocket push subscriber disabled: no NATS URL configured")
		return nil
	}

	conn, err := nats.Connect(
		natsURL,
		nats.Name("mistypass-api-gateway-ws-push-subscriber"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(30),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return fmt.Errorf("gateway websocket push subscriber: nats connect: %w", err)
	}

	subject := qualifiedInternalSubject(subjectPrefix, gatewayWebSocketPushSubject)
	_, err = conn.Subscribe(subject, func(msg *nats.Msg) {
		s.handleGatewayWebSocketPush(msg.Data)
	})
	if err != nil {
		conn.Close()
		return fmt.Errorf("gateway websocket push subscriber: subscribe %s: %w", subject, err)
	}

	s.loggerOrDefault().Info("gateway websocket push subscriber started", "subject", subject, "nats_url", natsURL)
	return nil
}

func (s *server) publishGatewayWebSocketPush(tenantID, gatewayID, messageType string, data json.RawMessage) {
	if s == nil || s.messageBus == nil || !s.messageBus.Enabled() {
		return
	}
	nextGatewayID := strings.TrimSpace(gatewayID)
	nextTenantID := strings.TrimSpace(tenantID)
	nextType := strings.TrimSpace(messageType)
	if nextGatewayID == "" || nextTenantID == "" || nextType == "" || len(data) == 0 {
		return
	}

	requestID, err := randomHexID(8)
	if err != nil {
		requestID = fmt.Sprintf("gw-ws-%d", time.Now().UnixNano())
	}
	payload := bus.GatewayWebSocketPush{
		RequestID:        requestID,
		OriginInstanceID: strings.TrimSpace(s.instanceID),
		TenantID:         nextTenantID,
		GatewayID:        nextGatewayID,
		Type:             nextType,
		Data:             append(json.RawMessage(nil), data...),
		IssuedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.messageBus.PublishJSON(ctx, gatewayWebSocketPushSubject, payload, nil); err != nil {
		s.loggerOrDefault().Warn("gateway websocket push fanout publish failed", "gateway_id", nextGatewayID, "type", nextType, "err", err)
		recordGatewayWebSocketPushFanout("publish_error")
		return
	}
	recordGatewayWebSocketPushFanout("published")
}

func (s *server) handleGatewayWebSocketPush(raw []byte) {
	var payload bus.GatewayWebSocketPush
	if err := json.Unmarshal(raw, &payload); err != nil {
		s.loggerOrDefault().Warn("gateway websocket push fanout decode failed", "err", err)
		recordGatewayWebSocketPushFanout("decode_error")
		return
	}
	if strings.TrimSpace(payload.GatewayID) == "" || len(payload.Data) == 0 {
		recordGatewayWebSocketPushFanout("invalid")
		return
	}
	if strings.TrimSpace(payload.OriginInstanceID) != "" && strings.TrimSpace(payload.OriginInstanceID) == strings.TrimSpace(s.instanceID) {
		recordGatewayWebSocketPushFanout("origin_skipped")
		return
	}

	switch strings.TrimSpace(payload.Type) {
	case "config_push":
		if s.gwWSRegistry != nil && s.gwWSRegistry.PushConfig(payload.GatewayID, payload.Data) {
			recordGatewayWebSocketPushFanout("delivered")
			return
		}
	case "credential_push":
		if s.gwWSRegistry != nil && s.gwWSRegistry.PushCredentials(payload.GatewayID, payload.Data) {
			recordGatewayWebSocketPushFanout("delivered")
			return
		}
	default:
		recordGatewayWebSocketPushFanout("unknown_type")
		return
	}

	recordGatewayWebSocketPushFanout("not_local")
}

func qualifiedInternalSubject(subjectPrefix, subject string) string {
	prefix := strings.Trim(strings.TrimSpace(subjectPrefix), ".")
	if prefix == "" {
		prefix = "mistypass"
	}
	nextSubject := strings.Trim(strings.TrimSpace(subject), ".")
	if nextSubject == "" {
		return prefix
	}
	return prefix + "." + nextSubject
}
