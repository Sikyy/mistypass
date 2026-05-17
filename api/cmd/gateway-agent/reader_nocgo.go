//go:build !cgo

package main

import (
	"fmt"
	"log/slog"
	"time"
)

// PCSCReader is a stub when PC/SC (CGO) is not available.
type PCSCReader struct{}

func NewPCSCReader(_ *slog.Logger, _ string, _ time.Duration, _ func(string, string, string)) *PCSCReader {
	return &PCSCReader{}
}

func (r *PCSCReader) Start() error {
	return fmt.Errorf("pcsc: not available (built without CGO)")
}

func (r *PCSCReader) Stop() {}

func formatPCSCReaderInfo() string {
	return "PC/SC: not available (built without CGO)"
}
