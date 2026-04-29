// gateway-agent is the program that runs on the physical Gateway device (e.g. Orange Pi).
// It connects to the Mistyislet Cloud via HTTPS :443, pulls configuration and access rules,
// performs local access decisions, drives relay/lock hardware, and pushes events back.
//
// Usage:
//
//	go run ./cmd/gateway-agent \
//	  -api http://192.168.1.100:8081 \
//	  -gateway gw_demo_001 \
//	  -tenant tenant_demo_jakarta \
//	  -token mistypass-dev-bootstrap-local-only-20260424
//
// For cross-compiling to Orange Pi (ARM64 Linux):
//
//	GOOS=linux GOARCH=arm64 go build -o gateway-agent ./cmd/gateway-agent
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	apiURL := flag.String("api", "http://localhost:8081", "Cloud API base URL")
	gatewayID := flag.String("gateway", "gw_demo_001", "Gateway ID")
	tenantID := flag.String("tenant", "tenant_demo_jakarta", "Tenant ID")
	bootstrapToken := flag.String("token", "", "Bootstrap token for initial registration")
	configPollInterval := flag.Duration("poll", 30*time.Second, "Config pull interval")
	heartbeatInterval := flag.Duration("heartbeat", 30*time.Second, "Heartbeat interval")
	relayGPIO := flag.Int("relay-gpio", -1, "GPIO pin number for relay (e.g. 73 for PC9 on OPi Zero3). -1 disables GPIO.")
	relayRS485 := flag.String("relay-rs485", "", "RS485 serial device for Modbus relay (e.g. /dev/ttyUSB0). Empty disables RS485.")
	unlockDuration := flag.Duration("unlock-duration", 5*time.Second, "How long to hold relay open for unlock")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	agent := &Agent{
		logger:             logger,
		apiURL:             *apiURL,
		gatewayID:          *gatewayID,
		tenantID:           *tenantID,
		bootstrapToken:     *bootstrapToken,
		configPollInterval: *configPollInterval,
		heartbeatInterval:  *heartbeatInterval,
		unlockDuration:     *unlockDuration,
		relayGPIOPin:       *relayGPIO,
		relayRS485Device:   *relayRS485,
	}

	logger.Info("gateway agent starting",
		"api", *apiURL,
		"gateway_id", *gatewayID,
		"tenant_id", *tenantID,
		"relay_gpio", *relayGPIO,
		"relay_rs485", *relayRS485,
	)

	if err := agent.Start(); err != nil {
		logger.Error("agent failed to start", "error", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("=== Mistyislet Gateway Agent Running ===")
	fmt.Printf("Gateway:  %s\n", *gatewayID)
	fmt.Printf("Cloud:    %s\n", *apiURL)
	fmt.Printf("Tenant:   %s\n", *tenantID)
	fmt.Printf("Poll:     %s\n", *configPollInterval)
	if *relayGPIO >= 0 {
		fmt.Printf("Relay:    GPIO %d\n", *relayGPIO)
	} else if *relayRS485 != "" {
		fmt.Printf("Relay:    RS485 %s\n", *relayRS485)
	} else {
		fmt.Println("Relay:    disabled (dry-run mode)")
	}
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Start stdin credential input for testing
	startStdinInput(agent, logger)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down gateway agent")
	agent.Stop()
}
