//go:build !linux

package main

import "errors"

// Start is a stub for non-Linux platforms. GPIO/epoll is Linux-only.
func (w *WiegandReader) Start() error {
	return errors.New("wiegand reader: GPIO/epoll not supported on this platform (Linux only)")
}

// Stop is a stub for non-Linux platforms.
func (w *WiegandReader) Stop() {
	w.logger.Warn("wiegand reader: Stop called on non-Linux platform (no-op)")
}

func (w *WiegandReader) cleanup() {}
