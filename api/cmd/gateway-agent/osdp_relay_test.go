package main

import "testing"

func TestOSDPCRC16(t *testing.T) {
	// Verify CRC is deterministic and non-zero for real data
	pkt := []byte{0x53, 0x00, 0x08, 0x00, 0x04, 0x60}
	crc1 := osdpCRC16(pkt)
	crc2 := osdpCRC16(pkt)
	if crc1 == 0 {
		t.Fatal("CRC should not be zero for non-empty packet")
	}
	if crc1 != crc2 {
		t.Fatalf("CRC should be deterministic: got 0x%04X and 0x%04X", crc1, crc2)
	}

	// Different data should produce different CRC
	pkt2 := []byte{0x53, 0x01, 0x08, 0x00, 0x04, 0x60}
	crc3 := osdpCRC16(pkt2)
	if crc1 == crc3 {
		t.Fatal("different data should produce different CRC")
	}
}

func TestBuildPacket(t *testing.T) {
	r := &OSDPRelay{address: 0x00, seq: 0}

	// Build a POLL command
	pkt := r.buildPacket(osdpCmdPoll, nil)

	// Minimum OSDP packet: SOM(1) + ADDR(1) + LEN(2) + CTRL(1) + CMD(1) + CRC(2) = 8 bytes
	if len(pkt) != 8 {
		t.Fatalf("expected 8-byte poll packet, got %d bytes", len(pkt))
	}
	if pkt[0] != osdpSOM {
		t.Fatalf("expected SOM 0x53, got 0x%02X", pkt[0])
	}
	if pkt[1] != 0x00 {
		t.Fatalf("expected address 0x00, got 0x%02X", pkt[1])
	}
	if pkt[5] != osdpCmdPoll {
		t.Fatalf("expected cmd 0x60 (POLL), got 0x%02X", pkt[5])
	}

	// Verify CRC bytes are correctly appended
	dataWithoutCRC := pkt[:len(pkt)-2]
	expectedCRC := osdpCRC16(dataWithoutCRC)
	actualCRC := uint16(pkt[len(pkt)-2]) | uint16(pkt[len(pkt)-1])<<8
	if expectedCRC != actualCRC {
		t.Fatalf("CRC mismatch: expected 0x%04X, got 0x%04X", expectedCRC, actualCRC)
	}
}

func TestBuildOutCommand(t *testing.T) {
	r := &OSDPRelay{address: 0x01, seq: 2}

	// Build an OUT command: temporarily unlock for 5 seconds (50 × 100ms)
	outData := []byte{0x00, osdpOutTemporaryOn, 0x32, 0x00}
	pkt := r.buildPacket(osdpCmdOut, outData)

	// SOM(1) + ADDR(1) + LEN(2) + CTRL(1) + CMD(1) + DATA(4) + CRC(2) = 12 bytes
	if len(pkt) != 12 {
		t.Fatalf("expected 12-byte OUT packet, got %d bytes", len(pkt))
	}
	if pkt[1] != 0x01 {
		t.Fatalf("expected address 0x01, got 0x%02X", pkt[1])
	}
	if pkt[5] != osdpCmdOut {
		t.Fatalf("expected cmd 0x68 (OUT), got 0x%02X", pkt[5])
	}

	// Verify CRC bytes are correctly appended
	dataWithoutCRC := pkt[:len(pkt)-2]
	expectedCRC := osdpCRC16(dataWithoutCRC)
	actualCRC := uint16(pkt[len(pkt)-2]) | uint16(pkt[len(pkt)-1])<<8
	if expectedCRC != actualCRC {
		t.Fatalf("CRC mismatch: expected 0x%04X, got 0x%04X", expectedCRC, actualCRC)
	}
}

func TestSequenceRotation(t *testing.T) {
	r := &OSDPRelay{address: 0x00, seq: 0}

	for i := 0; i < 8; i++ {
		expected := byte(i % 4)
		if r.seq != expected {
			t.Fatalf("iteration %d: expected seq %d, got %d", i, expected, r.seq)
		}
		r.buildPacket(osdpCmdPoll, nil)
		r.seq = (r.seq + 1) & 0x03
	}
}
