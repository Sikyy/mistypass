package main

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

// BLEReader manages the BLE GATT server for mobile credential authentication.
// In MVP mode, it uses a TCP listener to simulate BLE connections (for testing
// without hardware). The protocol is identical — only the transport layer differs.
//
// Real BLE hardware support (via tinygo.org/x/bluetooth or BlueZ D-Bus) will be
// added in Phase 4 when ESP32 hardware is ready. The authentication protocol
// remains the same regardless of transport.
type BLEReader struct {
	logger   *slog.Logger
	lockID   string
	agent    *Agent
	listener net.Listener // TCP listener for simulator mode

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

// NewBLEReader creates a BLE reader bound to a specific lock/door.
func NewBLEReader(logger *slog.Logger, lockID string, agent *Agent) *BLEReader {
	return &BLEReader{
		logger: logger,
		lockID: lockID,
		agent:  agent,
		stopCh: make(chan struct{}),
	}
}

// Start starts the BLE reader in TCP simulator mode on the given port.
// This allows the Android app (or test client) to connect via TCP and
// perform the same challenge-response protocol as real BLE.
func (b *BLEReader) Start(listenAddr string) error {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("ble reader listen: %w", err)
	}
	b.listener = ln
	b.running = true

	b.logger.Info("BLE reader started (TCP simulator mode)",
		"lock_id", b.lockID,
		"listen", listenAddr,
	)

	go b.acceptLoop()
	return nil
}

// Stop shuts down the BLE reader.
func (b *BLEReader) Stop() {
	close(b.stopCh)
	if b.listener != nil {
		b.listener.Close()
	}
	b.running = false
}

// acceptLoop handles incoming connections (each = one BLE session).
func (b *BLEReader) acceptLoop() {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			select {
			case <-b.stopCh:
				return
			default:
				b.logger.Warn("BLE accept error", "error", err)
				continue
			}
		}
		go b.handleSession(conn)
	}
}

// handleSession implements the full BLE authentication handshake over a single connection.
// Protocol v2 (same as BLE GATT, but over TCP for testability):
//
//  1. Reader sends CHALLENGE: [52 bytes] (32 nonce + 8 issued + 8 expires + 4 gateway_id)
//  2. Phone sends AUTH_RESPONSE: [1 byte userID_len][userID][signature]
//  3. Reader sends AUTH_RESULT: [1 byte code][reason string]
//  4. Connection closes
func (b *BLEReader) handleSession(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	remoteAddr := conn.RemoteAddr().String()
	b.logger.Info("BLE session started", "remote", remoteAddr, "lock_id", b.lockID)

	// Step 1: Generate and send v2 challenge (52 bytes)
	challenge, err := NewBLEChallengeV2(b.agent.gatewayIDUint32)
	if err != nil {
		b.logger.Error("BLE generate challenge failed", "error", err)
		return
	}

	challengeBytes := challenge.Encode()
	if _, err := conn.Write(challengeBytes); err != nil {
		b.logger.Warn("BLE send challenge failed", "error", err)
		return
	}

	// Step 2: Read auth response
	// Max size: 1 + 255 (userID) + 128 (signature) = 384 bytes
	buf := make([]byte, 384)
	n, err := conn.Read(buf)
	if err != nil {
		b.logger.Warn("BLE read auth response failed", "error", err)
		return
	}

	response, err := DecodeBLEAuthResponse(buf[:n])
	if err != nil {
		b.logger.Warn("BLE decode auth response failed", "error", err)
		result := BLEAuthResult{Code: BLEResultDenied, Reason: "invalid_response"}
		conn.Write(result.Encode())
		return
	}

	b.logger.Info("BLE auth response received",
		"user_id", response.UserID,
		"signature_len", len(response.Signature),
		"lock_id", b.lockID,
	)

	// Step 3: Verify with v2 (transport tag + nonce replay + gateway_id check)
	result := b.agent.VerifyAuthResponseV2(response, challengeBytes, TransportTagBLE)
	conn.Write(result.Encode())

	if result.Code == BLEResultGranted {
		b.logger.Info("BLE ACCESS GRANTED — unlocking door", "user_id", response.UserID, "lock_id", b.lockID)
		if err := b.agent.relay.Unlock(b.agent.unlockDuration); err != nil {
			b.logger.Error("relay unlock failed", "error", err)
		}
		b.agent.queueEvent(AccessEvent{
			GatewayID:  b.agent.gatewayID,
			EventType:  "access_granted",
			LockID:     b.lockID,
			Actor:      response.UserID,
			Result:     "allow",
			OccurredAt: time.Now().UTC().Format(time.RFC3339),
		})
	} else {
		b.logger.Warn("BLE ACCESS DENIED", "user_id", response.UserID, "lock_id", b.lockID, "reason", result.Reason)
		b.agent.queueEvent(AccessEvent{
			GatewayID:  b.agent.gatewayID,
			EventType:  "access_denied",
			LockID:     b.lockID,
			Actor:      response.UserID,
			Result:     result.Reason,
			OccurredAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	b.logger.Info("BLE session completed",
		"remote", remoteAddr,
		"user_id", response.UserID,
		"result", result.Reason,
	)
}

// GenerateChallengeV2 creates a v2 challenge (used by real BLE GATT read handler).
func (b *BLEReader) GenerateChallengeV2() (*BLEChallengeV2, error) {
	return NewBLEChallengeV2(b.agent.gatewayIDUint32)
}

// formatBLEReaderInfo returns diagnostic info about the BLE reader.
func formatBLEReaderInfo(lockID, listenAddr string) string {
	return fmt.Sprintf("BLE Reader (TCP Simulator):\n  Lock ID: %s\n  Listen:  %s\n  Protocol: MistyPass BLE Auth v2\n  Service UUID: %s\n", lockID, listenAddr, BLEServiceUUID)
}

// --- stdin BLE simulation command ---

// handleBLESimCommand processes a "ble <user_id> <nonce_hex> <sig_hex> <lock_id>" stdin command.
// This lets you test BLE authentication from the gateway agent console.
func handleBLESimCommand(agent *Agent, logger *slog.Logger, parts []string) {
	if len(parts) < 4 {
		fmt.Println("Usage: ble <user_id> <nonce_hex> <signature_hex> [lock_id]")
		fmt.Println("  nonce_hex: 64-char hex string (32 bytes)")
		fmt.Println("  signature_hex: hex-encoded ECDSA signature")
		return
	}

	userID := parts[1]
	nonceHex := parts[2]
	sigHex := parts[3]
	lockID := ""
	if len(parts) >= 5 {
		lockID = parts[4]
	}

	nonceBytes, err := hex.DecodeString(nonceHex)
	if err != nil || len(nonceBytes) != 32 {
		fmt.Println("Error: nonce must be exactly 32 bytes (64 hex chars)")
		return
	}

	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		fmt.Println("Error: invalid signature hex")
		return
	}

	var nonce [32]byte
	copy(nonce[:], nonceBytes)

	challenge := &BLEChallenge{
		Nonce:     nonce,
		IssuedAt:  time.Now().Add(-5 * time.Second),
		ExpiresAt: time.Now().Add(25 * time.Second),
	}
	response := &BLEAuthResponse{
		UserID:    userID,
		Signature: sigBytes,
	}

	result := agent.HandleBLEAuth(challenge, response, lockID)
	if result.Code == BLEResultGranted {
		fmt.Printf("✓ BLE AUTH GRANTED (user=%s, lock=%s)\n", userID, lockID)
	} else {
		fmt.Printf("✗ BLE AUTH DENIED (user=%s, lock=%s, reason=%s)\n", userID, lockID, result.Reason)
	}
}
