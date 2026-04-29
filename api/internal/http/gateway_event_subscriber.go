package httpx

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/mistypass/cloud/api/internal/bus"
	"github.com/mistypass/cloud/api/internal/modules/event"
)

// startGatewayEventSubscriber subscribes to gateway event topics and processes
// command acknowledgments as access events.
func (s *server) startGatewayEventSubscriber(natsURL, subjectPrefix string) error {
	if strings.TrimSpace(natsURL) == "" {
		s.logger.Info("gateway event subscriber disabled: no NATS URL configured")
		return nil
	}

	conn, err := nats.Connect(
		natsURL,
		nats.Name("mistypass-api-gateway-event-subscriber"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(30),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return fmt.Errorf("gateway event subscriber: nats connect: %w", err)
	}

	prefix := strings.Trim(strings.TrimSpace(subjectPrefix), ".")
	if prefix == "" {
		prefix = "mistypass"
	}

	subject := fmt.Sprintf("%s.gateway.*.event", prefix)
	_, err = conn.Subscribe(subject, func(msg *nats.Msg) {
		s.handleGatewayEvent(msg.Data)
	})
	if err != nil {
		conn.Close()
		return fmt.Errorf("gateway event subscriber: subscribe %s: %w", subject, err)
	}

	s.logger.Info("gateway event subscriber started", "subject", subject, "nats_url", natsURL)
	return nil
}

func (s *server) handleGatewayEvent(data []byte) {
	var evt bus.GatewayEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		s.logger.Warn("gateway event: invalid JSON", "error", err)
		return
	}

	switch evt.EventType {
	case "command_ack":
		s.handleCommandAck(evt)
	case "heartbeat":
		s.logger.Debug("gateway heartbeat", "gateway_id", evt.GatewayID, "status", evt.Status)
	case "access_granted", "access_denied":
		s.handleAccessEvent(evt)
	default:
		s.logger.Debug("gateway event", "event_type", evt.EventType, "gateway_id", evt.GatewayID)
	}
}

func (s *server) handleCommandAck(evt bus.GatewayEvent) {
	logger := s.logger.With(
		"event_type", "command_ack",
		"gateway_id", evt.GatewayID,
		"command", evt.Command,
		"lock_id", evt.LockID,
		"status", evt.Status,
		"request_id", evt.RequestID,
	)

	if evt.Status == "success" {
		logger.Info("gateway command executed successfully")
	} else {
		logger.Warn("gateway command failed", "error", evt.Error)
	}

	if evt.GatewayID != "" && evt.TenantID != "" {
		at, _ := time.Parse("2006-01-02T15:04:05Z07:00", evt.OccurredAt)
		if at.IsZero() {
			at = time.Now().UTC()
		}
		_, _, err := s.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
			TenantID:  evt.TenantID,
			Type:      evt.Command + "_" + evt.Status,
			GatewayID: evt.GatewayID,
			DoorID:    evt.LockID,
			Result:    evt.Status,
			At:        at,
		})
		if err != nil {
			logger.Warn("failed to record command ack as access event", "error", err)
		}
	}
}

func (s *server) handleAccessEvent(evt bus.GatewayEvent) {
	s.logger.Info("access event from gateway",
		"event_type", evt.EventType,
		"gateway_id", evt.GatewayID,
		"lock_id", evt.LockID,
	)

	if evt.GatewayID != "" && evt.TenantID != "" {
		at, _ := time.Parse("2006-01-02T15:04:05Z07:00", evt.OccurredAt)
		if at.IsZero() {
			at = time.Now().UTC()
		}
		s.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
			TenantID:  evt.TenantID,
			Type:      evt.EventType,
			GatewayID: evt.GatewayID,
			DoorID:    evt.LockID,
			Result:    evt.Status,
			At:        at,
		})
	}
}
