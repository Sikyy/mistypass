//go:build !cgo

package main

import (
	"context"
	"fmt"
	"log/slog"
)

// PCSCNFCDriver is a stub when PC/SC (CGO) is not available.
type PCSCNFCDriver struct{}

func NewPCSCNFCDriver(_ *slog.Logger) *PCSCNFCDriver {
	return &PCSCNFCDriver{}
}

func (d *PCSCNFCDriver) WaitForCard(_ context.Context) error {
	return fmt.Errorf("pcsc: not available (built without CGO)")
}

func (d *PCSCNFCDriver) Transmit(_ []byte) ([]byte, error) {
	return nil, fmt.Errorf("pcsc: not available (built without CGO)")
}

func (d *PCSCNFCDriver) Disconnect() error {
	return nil
}
