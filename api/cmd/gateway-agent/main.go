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
	relayOSDP := flag.String("relay-osdp", "", "RS485 serial device for OSDP v2 reader (e.g. /dev/ttyUSB0). Empty disables OSDP.")
	osdpAddress := flag.Int("osdp-address", 0, "OSDP peripheral device address (0-126)")
	unlockDuration := flag.Duration("unlock-duration", 5*time.Second, "How long to hold relay open for unlock")
	deviceTokenFile := flag.String("token-file", "/var/lib/mistypass/device-token", "File to persist device token across restarts")
	tlsPin := flag.String("tls-pin-sha256", "", "SHA256 hash of Cloud API TLS certificate SPKI for certificate pinning (hex-encoded)")
	rulesCacheTTL := flag.Duration("rules-cache-ttl", 24*time.Hour, "Max age of cached access rules before denying all access (0 = no TTL)")
	readerLockID := flag.String("reader-lock-id", "", "Lock ID that this reader controls (enables PC/SC NFC UID reader, e.g. door_jkt_001)")
	readerPoll := flag.Duration("reader-poll", 300*time.Millisecond, "NFC reader polling interval")
	nfcHCELockID := flag.String("nfc-hce-lock-id", "", "Lock ID for NFC HCE reader (enables APDU-based phone auth, e.g. door_lobby_001)")
	bleLockID := flag.String("ble-lock-id", "", "Lock ID for BLE reader (enables BLE auth, e.g. door_factory_001)")
	bleListenAddr := flag.String("ble-listen", ":9900", "TCP address for BLE simulator (used until real BLE hardware is ready)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	agent := &Agent{
		logger:             logger,
		apiURL:             *apiURL,
		gatewayID:          *gatewayID,
		tenantID:           *tenantID,
		bootstrapToken:     *bootstrapToken,
		deviceTokenFile:    *deviceTokenFile,
		configPollInterval: *configPollInterval,
		heartbeatInterval:  *heartbeatInterval,
		unlockDuration:     *unlockDuration,
		relayGPIOPin:       *relayGPIO,
		relayRS485Device:   *relayRS485,
		relayOSDPDevice:    *relayOSDP,
		osdpAddress:        byte(*osdpAddress),
		tlsPinSHA256:       *tlsPin,
		rulesCacheTTL:      *rulesCacheTTL,
	}

	logger.Info("gateway agent starting",
		"api", *apiURL,
		"gateway_id", *gatewayID,
		"tenant_id", *tenantID,
		"relay_gpio", *relayGPIO,
		"relay_rs485", *relayRS485,
		"relay_osdp", *relayOSDP,
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
		fmt.Printf("Relay:    GPIO %d (Wiegand)\n", *relayGPIO)
	} else if *relayOSDP != "" {
		fmt.Printf("Relay:    OSDP v2 %s addr=%d\n", *relayOSDP, *osdpAddress)
	} else if *relayRS485 != "" {
		fmt.Printf("Relay:    RS485 Modbus %s\n", *relayRS485)
	} else {
		fmt.Println("Relay:    disabled (dry-run mode)")
	}

	// Start PC/SC NFC reader if lock ID is configured
	var nfcReader *PCSCReader
	if *readerLockID != "" {
		fmt.Println(formatPCSCReaderInfo())
		nfcReader = NewPCSCReader(logger, *readerLockID, *readerPoll, agent.HandleCredentialPresented)
		if err := nfcReader.Start(); err != nil {
			logger.Warn("NFC reader failed to start, falling back to stdin", "error", err)
		} else {
			fmt.Printf("Reader:   PC/SC NFC → %s\n", *readerLockID)
		}
	}

	// Start NFC HCE reader (APDU-based phone auth via PC/SC) if lock ID is configured
	var nfcHCEReader *NFCHCEReader
	if *nfcHCELockID != "" {
		fmt.Println(formatPCSCReaderInfo())
		driver := NewPCSCNFCDriver(logger)
		nfcHCEReader = NewNFCHCEReader(logger, *nfcHCELockID, agent, driver)
		if err := nfcHCEReader.Start(); err != nil {
			logger.Warn("NFC HCE reader failed to start", "error", err)
		} else {
			fmt.Printf("NFC HCE: PC/SC APDU → %s (AID: F04D495354590100)\n", *nfcHCELockID)
		}
	}

	// Start BLE reader (TCP simulator mode) if lock ID is configured
	var bleReader *BLEReader
	if *bleLockID != "" {
		fmt.Println(formatBLEReaderInfo(*bleLockID, *bleListenAddr))
		bleReader = NewBLEReader(logger, *bleLockID, agent)
		if err := bleReader.Start(*bleListenAddr); err != nil {
			logger.Warn("BLE reader failed to start", "error", err)
		} else {
			fmt.Printf("BLE:      TCP %s → %s\n", *bleListenAddr, *bleLockID)
		}
	}

	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Start stdin credential input for testing (always available)
	startStdinInput(agent, logger)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down gateway agent")
	if nfcReader != nil {
		nfcReader.Stop()
	}
	if nfcHCEReader != nil {
		nfcHCEReader.Stop()
	}
	if bleReader != nil {
		bleReader.Stop()
	}
	agent.Stop()
}
