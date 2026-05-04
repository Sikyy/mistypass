package main

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// OSDPRelay sends OSDP v2 commands over RS-485 to control a door reader/lock.
// Implements the RelayDriver interface for use as an alternative to GPIO or Modbus.
//
// OSDP (Open Supervised Device Protocol) is the SIA standard for reader-to-controller
// communication. This implementation supports the basic osdp_OUT command for relay control.
type OSDPRelay struct {
	device  string
	address byte // OSDP peripheral device address (0-126)
	seq     byte // rolling sequence number (0-3)
	logger  *slog.Logger
}

// OSDP packet constants
const (
	osdpSOM = 0x53 // Start of Message marker

	// Command IDs
	osdpCmdPoll = 0x60 // Poll (keep-alive)
	osdpCmdOut  = 0x68 // Output control
	osdpCmdLED  = 0x69 // LED control
	osdpCmdBuz  = 0x6A // Buzzer control

	// Output control codes for osdp_OUT
	osdpOutNop             = 0x00 // No operation
	osdpOutPermanentOff    = 0x01 // Set permanently inactive
	osdpOutPermanentOn     = 0x02 // Set permanently active
	osdpOutTemporaryOn     = 0x03 // Set active for timer period, then inactive
	osdpOutTemporaryOff    = 0x04 // Set inactive for timer period, then active
)

func NewOSDPRelay(device string, address byte, logger *slog.Logger) (*OSDPRelay, error) {
	if _, err := os.Stat(device); err != nil {
		return nil, fmt.Errorf("osdp serial device not found: %s: %w", device, err)
	}
	if address > 126 {
		return nil, fmt.Errorf("osdp address must be 0-126, got %d", address)
	}
	logger.Info("OSDP v2 relay initialized", "device", device, "address", address)
	return &OSDPRelay{device: device, address: address, logger: logger}, nil
}

func (r *OSDPRelay) Unlock(duration time.Duration) error {
	// Convert duration to OSDP timer units (100ms per unit, max 65535)
	timerUnits := int(duration.Milliseconds() / 100)
	if timerUnits > 65535 {
		timerUnits = 65535
	}
	if timerUnits < 1 {
		timerUnits = 50 // default 5 seconds
	}

	// Build osdp_OUT command: temporarily activate output 0
	outData := []byte{
		0x00,                          // Output number (0 = first relay)
		osdpOutTemporaryOn,            // Control code: active for timer, then off
		byte(timerUnits & 0xFF),       // Timer LSB (100ms units)
		byte((timerUnits >> 8) & 0xFF), // Timer MSB
	}

	r.logger.Info(">>> OSDP OUT: unlock", "address", r.address, "duration", duration)
	return r.sendCommand(osdpCmdOut, outData)
}

func (r *OSDPRelay) Close() error {
	// Send a final output-off command
	offData := []byte{0x00, osdpOutPermanentOff, 0x00, 0x00}
	return r.sendCommand(osdpCmdOut, offData)
}

// sendCommand builds and transmits a complete OSDP packet.
func (r *OSDPRelay) sendCommand(cmdID byte, data []byte) error {
	packet := r.buildPacket(cmdID, data)

	f, err := os.OpenFile(r.device, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("osdp open serial: %w", err)
	}
	defer f.Close()

	_, err = f.Write(packet)
	if err != nil {
		return fmt.Errorf("osdp write: %w", err)
	}

	r.seq = (r.seq + 1) & 0x03 // rotate 0→1→2→3→0
	return nil
}

// buildPacket constructs an OSDP v2 packet with CRC-16.
//
// Packet format:
//
//	[SOM] [ADDR] [LEN_LSB] [LEN_MSB] [CTRL] [CMD_ID] [CMD_DATA...] [CRC_LSB] [CRC_MSB]
func (r *OSDPRelay) buildPacket(cmdID byte, data []byte) []byte {
	// Length = header(5) + cmdID(1) + data + CRC(2)
	totalLen := 5 + 1 + len(data) + 2

	pkt := make([]byte, 0, totalLen)
	pkt = append(pkt, osdpSOM)
	pkt = append(pkt, r.address)
	pkt = append(pkt, byte(totalLen&0xFF))        // length LSB
	pkt = append(pkt, byte((totalLen>>8)&0xFF))    // length MSB
	pkt = append(pkt, r.controlByte())
	pkt = append(pkt, cmdID)
	pkt = append(pkt, data...)

	// Append CRC-16 (CRC-16/AUG-CCITT, used by OSDP)
	crc := osdpCRC16(pkt)
	pkt = append(pkt, byte(crc&0xFF), byte((crc>>8)&0xFF))

	return pkt
}

// controlByte returns the OSDP control byte.
// Bits: [7:unused] [6:unused] [5:unused] [4:unused] [3:SCS] [2:CRC] [1:SEQ1] [0:SEQ0]
func (r *OSDPRelay) controlByte() byte {
	// Bit 2 = 1: CRC mode (not checksum)
	// Bits 0-1: sequence number
	return 0x04 | (r.seq & 0x03)
}

// osdpCRC16 computes the CRC-16/AUG-CCITT used by OSDP.
// Polynomial: 0x1021, Init: 0x1D0F
func osdpCRC16(data []byte) uint16 {
	crc := uint16(0x1D0F)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// Poll sends an osdp_POLL command to check reader status (keep-alive).
func (r *OSDPRelay) Poll() error {
	return r.sendCommand(osdpCmdPoll, nil)
}

// SetLED sends an osdp_LED command to control the reader LED.
func (r *OSDPRelay) SetLED(readerNum, ledNum, tempColor, permColor byte, timerUnits uint16) error {
	data := []byte{
		readerNum,
		ledNum,
		0x01,                                // temporary control: set
		0x01,                                // on time (100ms)
		0x01,                                // off time (100ms)
		tempColor,                           // temp on color
		0x00,                                // temp off color
		byte(timerUnits & 0xFF),             // timer LSB
		byte((timerUnits >> 8) & 0xFF),      // timer MSB
		0x01,                                // permanent control: set
		0x01,                                // perm on time
		0x00,                                // perm off time
		permColor,                           // perm on color
		0x00,                                // perm off color
	}
	return r.sendCommand(osdpCmdLED, data)
}

// Buzzer sends an osdp_BUZ command for audible feedback.
func (r *OSDPRelay) Buzzer(readerNum, toneCode, onTime, offTime, count byte) error {
	data := []byte{readerNum, toneCode, onTime, offTime, count}
	return r.sendCommand(osdpCmdBuz, data)
}

// ensure binary is imported
var _ = binary.LittleEndian
