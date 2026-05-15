//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// --- GPIO sysfs helpers ---

func gpioExport(pin int) error {
	return os.WriteFile("/sys/class/gpio/export", []byte(fmt.Sprintf("%d", pin)), 0o600)
}

func gpioUnexport(pin int) error {
	return os.WriteFile("/sys/class/gpio/unexport", []byte(fmt.Sprintf("%d", pin)), 0o600)
}

func gpioSetDirection(pin int, direction string) error {
	return os.WriteFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin), []byte(direction), 0o600)
}

func gpioSetEdge(pin int, edge string) error {
	return os.WriteFile(fmt.Sprintf("/sys/class/gpio/gpio%d/edge", pin), []byte(edge), 0o600)
}

func gpioOpenValue(pin int) (*os.File, error) {
	return os.Open(fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin))
}

// initGPIOPin exports a pin, sets it as input with falling edge detection.
func initGPIOPin(pin int) error {
	if err := gpioExport(pin); err != nil {
		// May already be exported, try to continue
	}
	if err := gpioSetDirection(pin, "in"); err != nil {
		return fmt.Errorf("gpio%d set direction: %w", pin, err)
	}
	if err := gpioSetEdge(pin, "falling"); err != nil {
		return fmt.Errorf("gpio%d set edge: %w", pin, err)
	}
	return nil
}

// Start initializes GPIO pins and launches the epoll read loop.
func (w *WiegandReader) Start() error {
	// Initialize D0
	if err := initGPIOPin(w.d0Pin); err != nil {
		gpioUnexport(w.d0Pin)
		return fmt.Errorf("wiegand D0 init: %w", err)
	}

	// Initialize D1 (rollback D0 on failure)
	if err := initGPIOPin(w.d1Pin); err != nil {
		gpioUnexport(w.d0Pin)
		gpioUnexport(w.d1Pin)
		return fmt.Errorf("wiegand D1 init: %w", err)
	}

	// Open value files for epoll
	d0File, err := gpioOpenValue(w.d0Pin)
	if err != nil {
		w.cleanup()
		return fmt.Errorf("wiegand D0 open: %w", err)
	}
	d1File, err := gpioOpenValue(w.d1Pin)
	if err != nil {
		d0File.Close()
		w.cleanup()
		return fmt.Errorf("wiegand D1 open: %w", err)
	}

	w.stopCh = make(chan struct{})
	w.wg.Add(1)
	go w.readLoop(d0File, d1File)

	w.logger.Info("wiegand reader started",
		"d0_pin", w.d0Pin, "d1_pin", w.d1Pin, "lock_id", w.lockID)
	return nil
}

// Stop signals the read loop to exit and unexports GPIO pins.
func (w *WiegandReader) Stop() {
	if w.stopCh != nil {
		close(w.stopCh)
		w.wg.Wait()
	}
	w.cleanup()
	w.logger.Info("wiegand reader stopped")
}

func (w *WiegandReader) cleanup() {
	gpioUnexport(w.d0Pin)
	gpioUnexport(w.d1Pin)
}

// readLoop is the epoll-based GPIO edge detection loop.
// It monitors D0/D1 for falling edges, collects bits into a frame buffer,
// and decodes the frame after 50ms of silence.
func (w *WiegandReader) readLoop(d0File, d1File *os.File) {
	defer w.wg.Done()
	defer d0File.Close()
	defer d1File.Close()

	d0Fd := int(d0File.Fd())
	d1Fd := int(d1File.Fd())

	// Create epoll instance
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		w.logger.Error("wiegand epoll create failed", "error", err)
		return
	}
	defer syscall.Close(epfd)

	// Register D0 and D1 for priority events (EPOLLPRI = sysfs GPIO edge)
	for _, fd := range []int{d0Fd, d1Fd} {
		event := syscall.EpollEvent{
			Events: syscall.EPOLLPRI | syscall.EPOLLERR,
			Fd:     int32(fd),
		}
		if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, fd, &event); err != nil {
			w.logger.Error("wiegand epoll add failed", "fd", fd, "error", err)
			return
		}
		// Initial read to clear any pending event
		buf := make([]byte, 1)
		syscall.Read(fd, buf)   //nolint:errcheck
		syscall.Seek(fd, 0, 0) //nolint:errcheck
	}

	var bits []byte
	events := make([]syscall.EpollEvent, 2)
	debounceUntil := time.Time{} // zero = no debounce active

	w.logger.Info("wiegand epoll loop started")

	for {
		// Check for stop signal (non-blocking)
		select {
		case <-w.stopCh:
			return
		default:
		}

		// Wait for edge events with 50ms timeout
		n, err := syscall.EpollWait(epfd, events, 50)
		if err != nil {
			if err == syscall.EINTR {
				continue // interrupted by signal, retry
			}
			w.logger.Error("wiegand epoll wait error", "error", err)
			time.Sleep(1 * time.Second) // backoff before retry
			continue
		}

		now := time.Now()

		// Process edge events
		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)
			// Consume the event: read + seek back to 0
			buf := make([]byte, 1)
			syscall.Read(fd, buf)   //nolint:errcheck
			syscall.Seek(fd, 0, 0) //nolint:errcheck

			// Skip if debouncing
			if now.Before(debounceUntil) {
				continue
			}

			switch fd {
			case d0Fd:
				bits = append(bits, 0)
			case d1Fd:
				bits = append(bits, 1)
			}
		}

		// Frame timeout: 50ms with no new bits and buffer non-empty
		if n == 0 && len(bits) > 0 {
			// Skip if debouncing
			if now.Before(debounceUntil) {
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

				// Debounce: ignore edges for 2 seconds
				debounceUntil = time.Now().Add(2 * time.Second)
			}
			bits = bits[:0]
		}
	}
}
