package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// NFCDriver abstracts the hardware communication layer.
type NFCDriver interface {
	WaitForCard(ctx context.Context) error
	Transmit(command []byte) ([]byte, error)
	Disconnect() error
}

// NFCReader implements ReaderAdapter for NFC ISO-DEP via PC/SC.
type NFCReader struct {
	driver NFCDriver
	name   string
}

func NewNFCReader(driver NFCDriver, name string) *NFCReader {
	return &NFCReader{driver: driver, name: name}
}

func (r *NFCReader) Name() string { return r.name }
func (r *NFCReader) Type() string { return "nfc" }

func (r *NFCReader) Authenticate(challenge []byte) (*BLEAuthResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Wait for card/phone to be presented
	if err := r.driver.WaitForCard(ctx); err != nil {
		return nil, fmt.Errorf("wait for card: %w", err)
	}
	defer r.driver.Disconnect()

	// 2. SELECT AID
	selectCmd := BuildSelectAID()
	selectResp, err := r.driver.Transmit(selectCmd)
	if err != nil {
		return nil, fmt.Errorf("SELECT AID transmit: %w", err)
	}
	_, sw, err := ParseAPDUResponse(selectResp)
	if err != nil {
		return nil, fmt.Errorf("parse SELECT response: %w", err)
	}
	if sw != SW_OK {
		return nil, fmt.Errorf("SELECT AID failed: SW=%04X", sw)
	}
	slog.Debug("NFC: SELECT AID success")

	// 3. AUTHENTICATE
	authCmd := BuildAuthenticate(challenge)
	authResp, err := r.driver.Transmit(authCmd)
	if err != nil {
		return nil, fmt.Errorf("AUTHENTICATE transmit: %w", err)
	}
	data, sw, err := ParseAPDUResponse(authResp)
	if err != nil {
		return nil, fmt.Errorf("parse AUTH response: %w", err)
	}

	switch sw {
	case SW_OK:
		return ParseNFCAuthResponse(data)
	case SW_SECURITY_NOT_SATISFIED:
		return nil, fmt.Errorf("device locked (2FA required)")
	case SW_CONDITIONS_NOT_MET:
		return nil, fmt.Errorf("credential expired or suspended")
	default:
		return nil, fmt.Errorf("AUTHENTICATE failed: SW=%04X", sw)
	}
}

func (r *NFCReader) Close() error {
	return r.driver.Disconnect()
}

// TCPNFCSimDriver simulates NFC over TCP for development testing.
type TCPNFCSimDriver struct {
	addr string
}

func NewTCPNFCSimDriver(addr string) *TCPNFCSimDriver {
	return &TCPNFCSimDriver{addr: addr}
}

func (d *TCPNFCSimDriver) WaitForCard(ctx context.Context) error {
	return nil // simulator always "ready"
}

func (d *TCPNFCSimDriver) Transmit(command []byte) ([]byte, error) {
	return nil, fmt.Errorf("TCP NFC simulator not yet connected")
}

func (d *TCPNFCSimDriver) Disconnect() error {
	return nil
}
