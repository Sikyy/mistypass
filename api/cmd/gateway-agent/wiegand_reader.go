package main

import (
	"fmt"
	"log/slog"
	"sync"
)

// WiegandReader receives card credentials from a Wiegand-output reader
// (e.g. ZKTeco ProID10) via GPIO edge detection on D0/D1 pins.
// Decodes 26-bit and 34-bit Wiegand frames and passes card data
// to the onCredential callback for access control.
type WiegandReader struct {
	d0Pin, d1Pin int
	lockID       string
	onCredential func(credType, credData, lockID string)
	logger       *slog.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewWiegandReader creates a reader bound to two GPIO pins and a lock ID.
func NewWiegandReader(d0Pin, d1Pin int, lockID string, onCredential func(string, string, string), logger *slog.Logger) *WiegandReader {
	return &WiegandReader{
		d0Pin:        d0Pin,
		d1Pin:        d1Pin,
		lockID:       lockID,
		onCredential: onCredential,
		logger:       logger.With("component", "wiegand_reader"),
	}
}

// --- Parity helpers ---

// checkEvenParity returns true if the number of 1-bits in bits is even.
func checkEvenParity(bits []byte) bool {
	count := 0
	for _, b := range bits {
		if b != 0 {
			count++
		}
	}
	return count%2 == 0
}

// checkOddParity returns true if the number of 1-bits in bits is odd.
func checkOddParity(bits []byte) bool {
	return !checkEvenParity(bits)
}

// --- Wiegand frame decoding ---

// bitsToUint converts a slice of 0/1 bytes to an unsigned integer (MSB first).
func bitsToUint(bits []byte) uint32 {
	var val uint32
	for _, b := range bits {
		val = (val << 1) | uint32(b&1)
	}
	return val
}

// decodeWiegand26 decodes a 26-bit Wiegand frame.
// Format: PE(1) | FC(8) | Card(16) | PO(1)
// PE = even parity over bits 0-12 (includes PE bit itself),
// PO = odd parity over bits 13-25 (includes PO bit itself).
func decodeWiegand26(bits []byte) (facilityCode uint16, cardNumber uint32, err error) {
	if len(bits) != 26 {
		return 0, 0, fmt.Errorf("wiegand26: expected 26 bits, got %d", len(bits))
	}
	// Even parity: bit 0 covers bits 1-12 (the parity bit + first 12 data bits)
	if !checkEvenParity(bits[0:13]) {
		return 0, 0, fmt.Errorf("wiegand26: even parity check failed")
	}
	// Odd parity: bit 25 covers bits 13-25 (last 12 data bits + parity bit)
	if !checkOddParity(bits[13:26]) {
		return 0, 0, fmt.Errorf("wiegand26: odd parity check failed")
	}
	facilityCode = uint16(bitsToUint(bits[1:9]))
	cardNumber = bitsToUint(bits[9:25])
	return facilityCode, cardNumber, nil
}

// decodeWiegand34 decodes a 34-bit Wiegand frame.
// Format: PE(1) | FC(16) | Card(16) | PO(1)
// PE = even parity over bits 0-16, PO = odd parity over bits 17-33.
func decodeWiegand34(bits []byte) (facilityCode uint16, cardNumber uint32, err error) {
	if len(bits) != 34 {
		return 0, 0, fmt.Errorf("wiegand34: expected 34 bits, got %d", len(bits))
	}
	// Even parity: bit 0 covers bits 1-16
	if !checkEvenParity(bits[0:17]) {
		return 0, 0, fmt.Errorf("wiegand34: even parity check failed")
	}
	// Odd parity: bit 33 covers bits 17-33
	if !checkOddParity(bits[17:34]) {
		return 0, 0, fmt.Errorf("wiegand34: odd parity check failed")
	}
	facilityCode = uint16(bitsToUint(bits[1:17]))
	cardNumber = bitsToUint(bits[17:33])
	return facilityCode, cardNumber, nil
}

// decodeWiegandFrame auto-detects 26/34-bit format and decodes.
func decodeWiegandFrame(bits []byte) (credType string, facilityCode uint16, cardNumber uint32, err error) {
	switch len(bits) {
	case 26:
		fc, cn, err := decodeWiegand26(bits)
		return "wiegand_26", fc, cn, err
	case 34:
		fc, cn, err := decodeWiegand34(bits)
		return "wiegand_34", fc, cn, err
	default:
		return "", 0, 0, fmt.Errorf("wiegand: unsupported frame length %d bits (expected 26 or 34)", len(bits))
	}
}
