// gateway-simulator is a development tool that simulates a physical gateway device.
// It subscribes to NATS command topics and responds with events, enabling end-to-end
// testing of the unlock flow without real hardware.
//
// Usage:
//
//	go run ./cmd/gateway-simulator -gateway gw_demo_001 -nats nats://localhost:4222
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/mistypass/cloud/api/internal/bus"
)

func main() {
	gatewayID := flag.String("gateway", "gw_demo_001", "Gateway ID to simulate")
	natsURL := flag.String("nats", "nats://localhost:4222", "NATS server URL")
	prefix := flag.String("prefix", "mistypass", "NATS subject prefix")
	unlockDuration := flag.Duration("unlock-duration", 5*time.Second, "How long to hold unlock (simulated)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("gateway simulator starting",
		"gateway_id", *gatewayID,
		"nats_url", *natsURL,
		"prefix", *prefix,
	)

	conn, err := nats.Connect(
		*natsURL,
		nats.Name(fmt.Sprintf("gateway-simulator-%s", *gatewayID)),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(30),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		logger.Error("failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer conn.Close()
	logger.Info("connected to NATS", "url", conn.ConnectedUrl())

	commandSubject := fmt.Sprintf("%s.gateway.%s.command", *prefix, *gatewayID)
	eventSubject := fmt.Sprintf("%s.gateway.%s.event", *prefix, *gatewayID)

	sub, err := conn.Subscribe(commandSubject, func(msg *nats.Msg) {
		var cmd bus.GatewayCommand
		if err := json.Unmarshal(msg.Data, &cmd); err != nil {
			logger.Warn("failed to decode command", "error", err, "data", string(msg.Data))
			return
		}

		logger.Info("COMMAND RECEIVED",
			"command", cmd.Command,
			"lock_id", cmd.LockID,
			"place_id", cmd.PlaceID,
			"issued_by", cmd.IssuedBy,
			"request_id", cmd.RequestID,
		)

		// Simulate command execution
		status := "success"
		errMsg := ""

		switch cmd.Command {
		case "unlock":
			logger.Info(fmt.Sprintf(">>> UNLOCKING door %s (hold for %s)", cmd.LockID, *unlockDuration))
			go func() {
				time.Sleep(*unlockDuration)
				logger.Info(fmt.Sprintf(">>> RELOCKED door %s", cmd.LockID))
			}()
		case "lock_down":
			logger.Info(fmt.Sprintf(">>> LOCKDOWN door %s — access denied until cancel", cmd.LockID))
		case "cancel_lockdown":
			logger.Info(fmt.Sprintf(">>> LOCKDOWN CANCELLED for door %s — normal access resumed", cmd.LockID))
		default:
			status = "failed"
			errMsg = "unknown command: " + cmd.Command
			logger.Warn("unknown command", "command", cmd.Command)
		}

		// Publish event response
		evt := bus.GatewayEvent{
			RequestID:  cmd.RequestID,
			GatewayID:  cmd.GatewayID,
			EventType:  "command_ack",
			Command:    cmd.Command,
			LockID:     cmd.LockID,
			PlaceID:    cmd.PlaceID,
			TenantID:   cmd.TenantID,
			Status:     status,
			Error:      errMsg,
			OccurredAt: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
		body, _ := json.Marshal(evt)
		if err := conn.Publish(eventSubject, body); err != nil {
			logger.Warn("failed to publish event", "error", err)
		} else {
			logger.Info("event published",
				"event_type", evt.EventType,
				"status", evt.Status,
				"lock_id", evt.LockID,
			)
		}
	})
	if err != nil {
		logger.Error("failed to subscribe", "subject", commandSubject, "error", err)
		os.Exit(1)
	}
	defer sub.Unsubscribe()

	logger.Info("subscribed to commands", "subject", commandSubject)
	logger.Info("publishing events to", "subject", eventSubject)

	// Also subscribe to wildcard for all gateways (useful when simulating multiple)
	wildcardSubject := fmt.Sprintf("%s.gateway.*.command", *prefix)
	if strings.Contains(*gatewayID, "*") || *gatewayID == "all" {
		sub2, err := conn.Subscribe(wildcardSubject, func(msg *nats.Msg) {
			var cmd bus.GatewayCommand
			if err := json.Unmarshal(msg.Data, &cmd); err != nil {
				return
			}
			logger.Info("WILDCARD COMMAND",
				"gateway_id", cmd.GatewayID,
				"command", cmd.Command,
				"lock_id", cmd.LockID,
			)
		})
		if err == nil {
			defer sub2.Unsubscribe()
			logger.Info("also subscribed to wildcard", "subject", wildcardSubject)
		}
	}

	// Send heartbeat periodically
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			heartbeat := bus.GatewayEvent{
				GatewayID:  *gatewayID,
				EventType:  "heartbeat",
				TenantID:   "",
				Status:     "online",
				OccurredAt: time.Now().UTC().Format("2006-01-02T15:04:05Z07:00"),
			}
			body, _ := json.Marshal(heartbeat)
			conn.Publish(eventSubject, body)
		}
	}()

	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Println()
	fmt.Println("=== Gateway Simulator Running ===")
	fmt.Printf("Gateway ID: %s\n", *gatewayID)
	fmt.Printf("Listening:  %s\n", commandSubject)
	fmt.Printf("Events:     %s\n", eventSubject)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	<-ctx.Done()
	logger.Info("shutting down gateway simulator")
}
