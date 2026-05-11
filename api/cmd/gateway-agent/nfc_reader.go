package main

import (
	"context"
	"fmt"
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

	// 1. Wait for card/phone to be presented (WaitForCard already sends SELECT AID
	//    to probe for the correct interface, so we skip sending it again here to
	//    minimize APDU round-trips and reduce the chance of the phone leaving the field)
	if err := r.driver.WaitForCard(ctx); err != nil {
		return nil, fmt.Errorf("wait for card: %w", err)
	}
	defer r.driver.Disconnect()

	// 2. AUTHENTICATE (SELECT AID already confirmed by WaitForCard)
	authCmd, err := BuildAuthenticate(challenge)
	if err != nil {
		return nil, fmt.Errorf("build AUTHENTICATE APDU: %w", err)
	}
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
