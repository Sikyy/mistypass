package main

import (
	"log/slog"
	"strings"
	"time"
)

// NFCHCEReader manages the NFC HCE authentication loop.
// It polls for phone presence on a PC/SC reader, generates v2 challenges,
// and verifies signed responses using ECDSA P-256.
type NFCHCEReader struct {
	logger  *slog.Logger
	lockID  string
	agent   *Agent
	reader  *NFCReader
	stopCh  chan struct{}
	running bool
}

func NewNFCHCEReader(logger *slog.Logger, lockID string, agent *Agent, driver NFCDriver) *NFCHCEReader {
	return &NFCHCEReader{
		logger: logger,
		lockID: lockID,
		agent:  agent,
		reader: NewNFCReader(driver, "NFC-HCE"),
		stopCh: make(chan struct{}),
	}
}

func (r *NFCHCEReader) Start() error {
	r.running = true
	r.logger.Info("NFC HCE reader starting", "lock_id", r.lockID)
	go r.pollLoop()
	return nil
}

func (r *NFCHCEReader) Stop() {
	close(r.stopCh)
	r.running = false
	r.reader.Close()
}

func (r *NFCHCEReader) pollLoop() {
	for {
		select {
		case <-r.stopCh:
			return
		default:
		}

		r.handleOnce()
		// Brief pause between authentication attempts to avoid CPU spin
		time.Sleep(500 * time.Millisecond)
	}
}

func (r *NFCHCEReader) handleOnce() {
	// Generate v2 challenge (52 bytes: nonce + issued_at + expires_at + gateway_id)
	challenge, err := NewBLEChallengeV2(r.agent.gatewayIDUint32)
	if err != nil {
		r.logger.Error("NFC HCE: generate challenge failed", "error", err)
		return
	}

	challengeBytes := challenge.Encode()

	// Authenticate: waits for card → SELECT AID → AUTHENTICATE APDU
	response, err := r.reader.Authenticate(challengeBytes)
	if err != nil {
		// Only log non-trivial errors (timeout = no card present, expected)
		errStr := err.Error()
		if !strings.Contains(errStr, "context deadline") &&
			!strings.Contains(errStr, "context canceled") {
			r.logger.Debug("NFC HCE: auth cycle failed", "error", err)
		}
		return
	}

	r.logger.Info("NFC HCE: auth response received",
		"user_id", response.UserID,
		"signature_len", len(response.Signature),
	)

	// Verify using v2 protocol with NFC_HCE transport tag
	result := r.agent.VerifyAuthResponseV2(response, challengeBytes, TransportTagNFCHCE)

	if result.Code == BLEResultGranted {
		r.logger.Info("NFC HCE ACCESS GRANTED",
			"user_id", response.UserID,
			"lock_id", r.lockID,
		)
		if err := r.agent.relay.Unlock(r.agent.unlockDuration); err != nil {
			r.logger.Error("relay unlock failed", "error", err)
		}
		r.agent.queueEvent(AccessEvent{
			GatewayID:  r.agent.gatewayID,
			EventType:  "access_granted",
			LockID:     r.lockID,
			Actor:      response.UserID,
			Result:     "allow",
			OccurredAt: time.Now().UTC().Format(time.RFC3339),
		})
	} else {
		r.logger.Warn("NFC HCE ACCESS DENIED",
			"user_id", response.UserID,
			"lock_id", r.lockID,
			"code", result.Code,
			"reason", result.Reason,
		)
		r.agent.queueEvent(AccessEvent{
			GatewayID:  r.agent.gatewayID,
			EventType:  "access_denied",
			LockID:     r.lockID,
			Actor:      response.UserID,
			Result:     result.Reason,
			OccurredAt: time.Now().UTC().Format(time.RFC3339),
		})
	}
}
