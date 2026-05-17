//go:build cgo

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"time"

	"github.com/ebfe/scard"
)

// PCSCNFCDriver implements NFCDriver using the PC/SC subsystem.
// It communicates with NFC phones via ISO-DEP (ISO 14443-4) through
// a contactless reader like the ACS Wallet Mate II.
//
// The Wallet Mate II exposes multiple logical interfaces (e.g. Apple VAS,
// Google Smart Tap, generic contactless). Only the generic interface routes
// custom AIDs to the phone's HCE service. This driver probes each interface
// with SELECT AID to find the correct one.
type PCSCNFCDriver struct {
	logger *slog.Logger
	ctx    *scard.Context
	card   *scard.Card
}

func NewPCSCNFCDriver(logger *slog.Logger) *PCSCNFCDriver {
	return &PCSCNFCDriver{logger: logger}
}

func (d *PCSCNFCDriver) WaitForCard(ctx context.Context) error {
	scardCtx, err := scard.EstablishContext()
	if err != nil {
		return fmt.Errorf("establish context: %w", err)
	}
	d.ctx = scardCtx

	selectAIDCmd := BuildSelectAID()

	for {
		select {
		case <-ctx.Done():
			scardCtx.Release()
			d.ctx = nil
			return ctx.Err()
		default:
		}

		readers, err := scardCtx.ListReaders()
		if err != nil || len(readers) == 0 {
			time.Sleep(200 * time.Millisecond)
			continue
		}

		// Try each reader interface; probe with SELECT AID to find the one
		// that routes our custom AID to the phone's HCE service.
		for _, readerName := range readers {
			card, err := scardCtx.Connect(readerName, scard.ShareShared, scard.ProtocolAny)
			if err != nil {
				d.logger.Debug("NFC HCE: connect failed", "reader", readerName, "error", err)
				continue // no card on this interface
			}

			// Probe: send SELECT AID and check if the phone's HCE service responds
			resp, err := card.Transmit(selectAIDCmd)
			if err != nil {
				card.Disconnect(scard.LeaveCard)
				continue
			}

			// Check status word (last 2 bytes)
			if len(resp) >= 2 {
				sw := binary.BigEndian.Uint16(resp[len(resp)-2:])
				if sw == SW_OK {
					d.card = card
					d.logger.Debug("NFC HCE: SELECT AID OK", "reader", readerName)
					return nil
				}
				d.logger.Debug("NFC HCE: SELECT AID rejected", "reader", readerName, "sw", fmt.Sprintf("%04X", sw))
			}

			card.Disconnect(scard.LeaveCard)
		}

		time.Sleep(200 * time.Millisecond)
	}
}

func (d *PCSCNFCDriver) Transmit(command []byte) ([]byte, error) {
	if d.card == nil {
		return nil, fmt.Errorf("no card connected")
	}
	return d.card.Transmit(command)
}

func (d *PCSCNFCDriver) Disconnect() error {
	if d.card != nil {
		d.card.Disconnect(scard.LeaveCard)
		d.card = nil
	}
	if d.ctx != nil {
		d.ctx.Release()
		d.ctx = nil
	}
	return nil
}
