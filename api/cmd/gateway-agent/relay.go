package main

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// RelayDriver abstracts the hardware relay control.
type RelayDriver interface {
	Unlock(duration time.Duration) error
	Close() error
}

// --- DryRunRelay (no hardware, just logs) ---

type DryRunRelay struct {
	logger      *slog.Logger
	mu          sync.Mutex
	relockTimer *time.Timer
}

func (r *DryRunRelay) Unlock(duration time.Duration) error {
	r.logger.Info(fmt.Sprintf(">>> DRY RUN: relay ON for %s", duration))
	r.mu.Lock()
	if r.relockTimer != nil {
		r.relockTimer.Stop()
	}
	r.relockTimer = time.AfterFunc(duration, func() {
		r.logger.Info(">>> DRY RUN: relay OFF (door relocked)")
	})
	r.mu.Unlock()
	return nil
}

func (r *DryRunRelay) Close() error {
	r.mu.Lock()
	if r.relockTimer != nil {
		r.relockTimer.Stop()
	}
	r.mu.Unlock()
	return nil
}

// --- GPIO Relay (Linux sysfs, works on Orange Pi / Raspberry Pi) ---

type GPIORelay struct {
	pin         int
	logger      *slog.Logger
	mu          sync.Mutex
	relockTimer *time.Timer
}

func NewGPIORelay(pin int, logger *slog.Logger) (*GPIORelay, error) {
	r := &GPIORelay{pin: pin, logger: logger}

	// Export GPIO pin via sysfs
	if err := r.writeFile("/sys/class/gpio/export", fmt.Sprintf("%d", pin)); err != nil {
		// May already be exported, continue
		logger.Debug("gpio export (may already exist)", "pin", pin, "error", err)
	}

	// Set direction to output
	if err := r.writeFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin), "out"); err != nil {
		return nil, fmt.Errorf("gpio direction: %w", err)
	}

	// Set initial state to HIGH (relay off, assuming active-low trigger)
	if err := r.writeFile(fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin), "1"); err != nil {
		return nil, fmt.Errorf("gpio initial value: %w", err)
	}

	logger.Info("GPIO relay initialized", "pin", pin)
	return r, nil
}

func (r *GPIORelay) Unlock(duration time.Duration) error {
	valuePath := fmt.Sprintf("/sys/class/gpio/gpio%d/value", r.pin)

	// Pull LOW to activate relay (active-low trigger)
	r.logger.Info(">>> GPIO relay ON (unlocking)", "pin", r.pin)
	if err := r.writeFile(valuePath, "0"); err != nil {
		return fmt.Errorf("gpio relay on: %w", err)
	}

	r.mu.Lock()
	if r.relockTimer != nil {
		r.relockTimer.Stop()
	}
	r.relockTimer = time.AfterFunc(duration, func() {
		r.logger.Info(">>> GPIO relay OFF (relocked)", "pin", r.pin)
		if err := r.writeFile(valuePath, "1"); err != nil {
			r.logger.Error("gpio relay off failed", "error", err)
		}
	})
	r.mu.Unlock()

	return nil
}

func (r *GPIORelay) Close() error {
	r.mu.Lock()
	if r.relockTimer != nil {
		r.relockTimer.Stop()
	}
	r.mu.Unlock()
	// Set HIGH (relay off) and unexport
	valuePath := fmt.Sprintf("/sys/class/gpio/gpio%d/value", r.pin)
	r.writeFile(valuePath, "1")
	r.writeFile("/sys/class/gpio/unexport", fmt.Sprintf("%d", r.pin))
	return nil
}

func (r *GPIORelay) writeFile(path, value string) error {
	return os.WriteFile(path, []byte(value), 0o600)
}

// --- RS485 Modbus Relay (via serial port) ---

type RS485Relay struct {
	device      string
	logger      *slog.Logger
	mu          sync.Mutex
	relockTimer *time.Timer
}

func NewRS485Relay(device string, logger *slog.Logger) (*RS485Relay, error) {
	// Verify serial device exists
	if _, err := os.Stat(device); err != nil {
		return nil, fmt.Errorf("rs485 device not found: %s: %w", device, err)
	}
	logger.Info("RS485 relay initialized", "device", device)
	return &RS485Relay{device: device, logger: logger}, nil
}

func (r *RS485Relay) Unlock(duration time.Duration) error {
	// Modbus RTU command to turn on relay 1
	// Standard LC-tech RS485 relay: address=0x01, function=0x05, register=0x0000, value=0xFF00
	onCmd := []byte{0x01, 0x05, 0x00, 0x00, 0xFF, 0x00, 0x8C, 0x3A}
	offCmd := []byte{0x01, 0x05, 0x00, 0x00, 0x00, 0x00, 0xCD, 0xCA}

	r.logger.Info(">>> RS485 relay ON (unlocking)", "device", r.device)
	if err := r.sendCommand(onCmd); err != nil {
		return fmt.Errorf("rs485 relay on: %w", err)
	}

	r.mu.Lock()
	if r.relockTimer != nil {
		r.relockTimer.Stop()
	}
	r.relockTimer = time.AfterFunc(duration, func() {
		r.logger.Info(">>> RS485 relay OFF (relocked)", "device", r.device)
		if err := r.sendCommand(offCmd); err != nil {
			r.logger.Error("rs485 relay off failed", "error", err)
		}
	})
	r.mu.Unlock()

	return nil
}

func (r *RS485Relay) sendCommand(cmd []byte) error {
	f, err := os.OpenFile(r.device, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(cmd)
	return err
}

func (r *RS485Relay) Close() error {
	r.mu.Lock()
	if r.relockTimer != nil {
		r.relockTimer.Stop()
	}
	r.mu.Unlock()
	return nil
}
