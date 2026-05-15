package main

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/host/v3"
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

// --- GPIO via periph.io ---

// lookupPin resolves a GPIO number to a periph.io pin.
// Tries numeric name first ("73"), then GPIO prefix ("GPIO73").
func lookupPin(num int) (gpio.PinIO, error) {
	name := strconv.Itoa(num)
	if p := gpioreg.ByName(name); p != nil {
		return p, nil
	}
	prefixed := fmt.Sprintf("GPIO%d", num)
	if p := gpioreg.ByName(prefixed); p != nil {
		return p, nil
	}
	return nil, fmt.Errorf("GPIO pin %d not found in registry (tried %q and %q)", num, name, prefixed)
}

// Start initializes periph.io host drivers, resolves GPIO pins,
// configures falling edge detection, and launches the read loop.
func (w *WiegandReader) Start() error {
	if _, err := host.Init(); err != nil {
		return fmt.Errorf("periph host init: %w", err)
	}

	d0, err := lookupPin(w.d0Pin)
	if err != nil {
		return fmt.Errorf("wiegand D0: %w", err)
	}
	d1, err := lookupPin(w.d1Pin)
	if err != nil {
		return fmt.Errorf("wiegand D1: %w", err)
	}

	// Configure pins: input, pull-up, falling edge interrupt.
	// External 10kΩ pull-ups to 3.3V are still recommended for reliable
	// idle-HIGH, but periph.io will also configure internal pull-ups
	// if the hardware supports it.
	if err := d0.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		return fmt.Errorf("wiegand D0 configure: %w", err)
	}
	if err := d1.In(gpio.PullUp, gpio.FallingEdge); err != nil {
		return fmt.Errorf("wiegand D1 configure: %w", err)
	}

	w.stopCh = make(chan struct{})
	w.wg.Add(1)
	go w.readLoop(d0, d1)

	w.logger.Info("wiegand reader started",
		"d0_pin", w.d0Pin, "d1_pin", w.d1Pin, "lock_id", w.lockID)
	return nil
}

// Stop signals the read loop and pin watchers to exit.
func (w *WiegandReader) Stop() {
	if w.stopCh != nil {
		close(w.stopCh)
		w.wg.Wait()
	}
	w.logger.Info("wiegand reader stopped")
}

// pinWatcher monitors a single GPIO pin for falling edges and sends
// the corresponding bit value (0 for D0, 1 for D1) to bitCh.
func (w *WiegandReader) pinWatcher(pin gpio.PinIO, bit byte, bitCh chan<- byte) {
	defer w.wg.Done()
	for {
		select {
		case <-w.stopCh:
			return
		default:
		}
		// WaitForEdge blocks until a falling edge or 100ms timeout.
		// The 100ms timeout allows periodic stop-signal checks.
		if pin.WaitForEdge(100 * time.Millisecond) {
			select {
			case bitCh <- bit:
			case <-w.stopCh:
				return
			}
		}
	}
}

// readLoop collects bits from D0/D1 pin watchers, assembles frames,
// and decodes after 50ms of silence (Wiegand frame end).
func (w *WiegandReader) readLoop(d0, d1 gpio.PinIO) {
	defer w.wg.Done()

	bitCh := make(chan byte, 64)

	// Launch pin watchers for D0 (0-bit) and D1 (1-bit)
	w.wg.Add(2)
	go w.pinWatcher(d0, 0, bitCh)
	go w.pinWatcher(d1, 1, bitCh)

	var bits []byte
	frameTimer := time.NewTimer(50 * time.Millisecond)
	frameTimer.Stop() // inactive until first bit arrives
	debounceUntil := time.Time{}

	w.logger.Info("wiegand read loop started")

	for {
		select {
		case <-w.stopCh:
			frameTimer.Stop()
			return

		case bit := <-bitCh:
			// Skip bits during debounce window (2s after last successful read)
			if time.Now().Before(debounceUntil) {
				continue
			}
			bits = append(bits, bit)
			frameTimer.Reset(50 * time.Millisecond)

		case <-frameTimer.C:
			if len(bits) == 0 {
				continue
			}
			if time.Now().Before(debounceUntil) {
				bits = bits[:0]
				continue
			}

			credType, fc, card, err := decodeWiegandFrame(bits)
			if err != nil {
				w.logger.Warn("wiegand frame decode error",
					"error", err, "bit_count", len(bits))
			} else {
				credData := fmt.Sprintf("%d:%d", fc, card)
				w.logger.Info("wiegand card detected",
					"type", credType, "fc", fc, "card", card, "lock_id", w.lockID)
				w.onCredential(credType, credData, w.lockID)

				// Debounce: ignore edges for 2 seconds to prevent double-reads
				debounceUntil = time.Now().Add(2 * time.Second)
			}
			bits = bits[:0]
		}
	}
}
