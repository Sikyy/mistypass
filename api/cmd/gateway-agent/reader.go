//go:build cgo

package main

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ebfe/scard"
)

// ReaderAdapter is the common interface for all credential reader types.
type ReaderAdapter interface {
	Name() string
	Type() string // "ble" | "nfc" | "tcp_simulator"
	// Authenticate sends a challenge and waits for a signed response.
	Authenticate(challenge []byte) (*BLEAuthResponse, error)
	Close() error
}

// PCSCReader reads NFC UIDs from a PC/SC compatible reader (e.g. ACS WalletMate II).
// It polls for card presence and calls the callback with the UID when a card is detected.
type PCSCReader struct {
	logger   *slog.Logger
	lockID   string        // which door this reader controls
	pollInterval time.Duration
	onCredential func(credentialType, credentialData, lockID string)
	stopCh   chan struct{}
}

func NewPCSCReader(logger *slog.Logger, lockID string, pollInterval time.Duration, onCredential func(string, string, string)) *PCSCReader {
	if pollInterval <= 0 {
		pollInterval = 300 * time.Millisecond
	}
	return &PCSCReader{
		logger:       logger,
		lockID:       lockID,
		pollInterval: pollInterval,
		onCredential: onCredential,
		stopCh:       make(chan struct{}),
	}
}

func (r *PCSCReader) Start() error {
	// Verify PC/SC subsystem is available
	ctx, err := scard.EstablishContext()
	if err != nil {
		return fmt.Errorf("pcsc: establish context: %w", err)
	}
	ctx.Release()

	r.logger.Info("PC/SC reader starting", "lock_id", r.lockID, "poll_interval", r.pollInterval)
	go r.pollLoop()
	return nil
}

func (r *PCSCReader) Stop() {
	close(r.stopCh)
}

func (r *PCSCReader) pollLoop() {
	var lastUID string
	var lastSeenAt time.Time
	// Debounce: ignore same UID within 3 seconds to prevent repeated triggers
	const debounceDuration = 3 * time.Second

	for {
		select {
		case <-r.stopCh:
			return
		default:
		}

		uid, err := r.readCardUID()
		if err != nil {
			errStr := err.Error()
			// Log non-trivial errors at debug level to avoid spam
			if !strings.Contains(errStr, "no card") &&
				!strings.Contains(errStr, "no reader") &&
				!strings.Contains(errStr, "SCARD_E_NO_SMARTCARD") &&
				!strings.Contains(errStr, "SCARD_W_REMOVED") &&
				!strings.Contains(errStr, "SW=6E00") {
				r.logger.Warn("pcsc read error", "error", err)
			}
			time.Sleep(r.pollInterval)
			continue
		}

		if uid == "" {
			time.Sleep(r.pollInterval)
			continue
		}

		r.logger.Info("raw UID read", "uid", uid)

		now := time.Now()
		if uid == lastUID && now.Sub(lastSeenAt) < debounceDuration {
			// Same card still on reader, skip
			time.Sleep(r.pollInterval)
			continue
		}

		lastUID = uid
		lastSeenAt = now

		r.logger.Info("NFC card detected", "uid", uid, "lock_id", r.lockID)
		r.onCredential("nfc_uid", uid, r.lockID)

		time.Sleep(r.pollInterval)
	}
}

// readCardUID tries all available PC/SC reader interfaces, powers the card,
// and sends GET UID APDU to retrieve the NFC UID.
// WalletMate II exposes multiple logical interfaces (e.g. Apple VAS, Google Smart Tap, generic NFC).
func (r *PCSCReader) readCardUID() (string, error) {
	ctx, err := scard.EstablishContext()
	if err != nil {
		return "", fmt.Errorf("establish context: %w", err)
	}
	defer ctx.Release()

	readers, err := ctx.ListReaders()
	if err != nil || len(readers) == 0 {
		return "", fmt.Errorf("no reader found")
	}

	// Try each reader interface — WalletMate II has multiple logical interfaces
	var lastErr error
	for _, readerName := range readers {
		uid, err := r.tryReadFromReader(ctx, readerName)
		if err != nil {
			lastErr = err
			continue // no card on this interface, try next
		}
		if uid != "" {
			return uid, nil
		}
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("no card on any interface")
}

func (r *PCSCReader) tryReadFromReader(ctx *scard.Context, readerName string) (string, error) {
	card, err := ctx.Connect(readerName, scard.ShareShared, scard.ProtocolAny)
	if err != nil {
		return "", err
	}
	defer card.Disconnect(scard.LeaveCard)

	// Verify chip fingerprint before reading UID
	if !r.verifyChipFingerprint(card, readerName) {
		return "", fmt.Errorf("chip fingerprint rejected")
	}

	getUIDCmd := []byte{0xFF, 0xCA, 0x00, 0x00, 0x00}
	resp, err := card.Transmit(getUIDCmd)
	if err != nil {
		return "", err
	}

	if len(resp) < 2 {
		return "", fmt.Errorf("response too short: %d bytes", len(resp))
	}

	sw1 := resp[len(resp)-2]
	sw2 := resp[len(resp)-1]
	if sw1 != 0x90 || sw2 != 0x00 {
		return "", fmt.Errorf("apdu error: SW=%02X%02X", sw1, sw2)
	}

	uidBytes := resp[:len(resp)-2]
	if len(uidBytes) == 0 {
		return "", fmt.Errorf("empty uid")
	}

	uid := strings.ToUpper(hex.EncodeToString(uidBytes))
	return uid, nil
}

// verifyChipFingerprint checks ATR and GET VERSION to reject clone cards.
// Real DESFire EV2/EV3: Vendor=0x04(NXP), 7-byte UID, ATR starts with 3B81.
// Magic cards (MIFARE Classic clones) fail these checks.
func (r *PCSCReader) verifyChipFingerprint(card *scard.Card, readerName string) bool {
	status, err := card.Status()
	if err != nil {
		return false
	}

	atr := status.Atr
	if len(atr) < 4 {
		r.logger.Warn("card rejected: ATR too short", "reader", readerName, "atr_len", len(atr))
		return false
	}

	// DESFire ATR starts with 3B 81 (or 3B 8x for contactless)
	if atr[0] != 0x3B {
		r.logger.Warn("card rejected: invalid TS byte", "reader", readerName, "ts", fmt.Sprintf("%02X", atr[0]))
		return false
	}

	// GET VERSION: DESFire native command wrapped in PC/SC
	resp, err := card.Transmit([]byte{0x90, 0x60, 0x00, 0x00, 0x00})
	if err != nil || len(resp) < 9 {
		r.logger.Warn("card rejected: GET VERSION failed", "reader", readerName)
		return false
	}

	sw := resp[len(resp)-1]
	if sw != 0xAF && sw != 0x00 {
		r.logger.Warn("card rejected: GET VERSION bad status", "reader", readerName, "sw", fmt.Sprintf("%02X", sw))
		return false
	}

	data := resp[:len(resp)-2]
	vendorID := data[0]
	majorVer := data[3]

	if vendorID != 0x04 {
		r.logger.Warn("card rejected: not NXP chip", "reader", readerName, "vendor", fmt.Sprintf("%02X", vendorID))
		return false
	}

	// Major version: 0x30=EV1, 0x31=EV2, 0x33=EV3
	if majorVer < 0x30 {
		r.logger.Warn("card rejected: not DESFire EV series", "reader", readerName, "major", fmt.Sprintf("%02X", majorVer))
		return false
	}

	return true
}

// listPCSCReaders returns available PC/SC reader names (for diagnostics).
func listPCSCReaders() ([]string, error) {
	ctx, err := scard.EstablishContext()
	if err != nil {
		return nil, err
	}
	defer ctx.Release()
	return ctx.ListReaders()
}

// formatPCSCReaderInfo returns a human-readable string of available readers.
func formatPCSCReaderInfo() string {
	readers, err := listPCSCReaders()
	if err != nil {
		return fmt.Sprintf("PC/SC error: %v", err)
	}
	if len(readers) == 0 {
		return "No PC/SC readers detected"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("PC/SC readers detected (%d):\n", len(readers)))
	for i, name := range readers {
		sb.WriteString(fmt.Sprintf("  [%d] %s\n", i, name))
	}
	_ = hex.EncodeToString // ensure hex import used
	return sb.String()
}
