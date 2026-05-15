package main

import "testing"

func TestCheckEvenParity(t *testing.T) {
	tests := []struct {
		name string
		bits []byte
		want bool
	}{
		{"all zeros", []byte{0, 0, 0, 0}, true},   // 0 ones = even
		{"one 1", []byte{1, 0, 0, 0}, false},       // 1 one = odd
		{"two 1s", []byte{1, 1, 0, 0}, true},       // 2 ones = even
		{"three 1s", []byte{1, 1, 1, 0}, false},    // 3 ones = odd
		{"all ones", []byte{1, 1, 1, 1}, true},     // 4 ones = even
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkEvenParity(tt.bits); got != tt.want {
				t.Errorf("checkEvenParity(%v) = %v, want %v", tt.bits, got, tt.want)
			}
		})
	}
}

func TestCheckOddParity(t *testing.T) {
	tests := []struct {
		name string
		bits []byte
		want bool
	}{
		{"all zeros", []byte{0, 0, 0, 0}, false}, // 0 ones = even, not odd
		{"one 1", []byte{1, 0, 0, 0}, true},      // 1 one = odd
		{"two 1s", []byte{1, 1, 0, 0}, false},    // 2 ones = even
		{"three 1s", []byte{1, 1, 1, 0}, true},   // 3 ones = odd
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkOddParity(tt.bits); got != tt.want {
				t.Errorf("checkOddParity(%v) = %v, want %v", tt.bits, got, tt.want)
			}
		})
	}
}

func TestDecodeWiegand26(t *testing.T) {
	// Build a valid 26-bit frame: PE(even over 1-12) | FC 8bit | Card 16bit | PO(odd over 13-24)
	// FC=100 (01100100), Card=12345 (0011000000111001)
	// Bits 1-12:  0 1 1 0 0 1 0 0 | 0 0 1 1  -> 5 ones -> odd -> PE=1
	// Bits 13-24: 0 0 0 0 0 0 1 1 1 0 0 1    -> 4 ones -> even -> PO=1
	frame := []byte{
		1,                                                     // bit 0: even parity
		0, 1, 1, 0, 0, 1, 0, 0,                               // bits 1-8: FC=100
		0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 1, 1, 1, 0, 0, 1,     // bits 9-24: Card=12345
		1,                                                     // bit 25: odd parity
	}
	fc, card, err := decodeWiegand26(frame)
	if err != nil {
		t.Fatalf("decodeWiegand26() error: %v", err)
	}
	if fc != 100 {
		t.Errorf("facility code = %d, want 100", fc)
	}
	if card != 12345 {
		t.Errorf("card number = %d, want 12345", card)
	}
}

func TestDecodeWiegand26_ParityError(t *testing.T) {
	// Same frame but flip the even parity bit
	frame := []byte{
		0,                                                     // bit 0: WRONG parity (should be 1)
		0, 1, 1, 0, 0, 1, 0, 0,
		0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 1, 1, 1, 0, 0, 1,
		1,
	}
	_, _, err := decodeWiegand26(frame)
	if err == nil {
		t.Fatal("decodeWiegand26() expected parity error")
	}
}

func TestDecodeWiegand26_WrongLength(t *testing.T) {
	frame := make([]byte, 20) // not 26
	_, _, err := decodeWiegand26(frame)
	if err == nil {
		t.Fatal("decodeWiegand26() expected length error")
	}
}

func TestDecodeWiegand34(t *testing.T) {
	// Build a valid 34-bit frame: PE(even over 1-16) | FC 16bit | Card 16bit | PO(odd over 17-32)
	// FC=200 (0000000011001000), Card=54321 (1101010000110001)
	// Bits 1-16: 0 0 0 0 0 0 0 0 1 1 0 0 1 0 0 0 -> 3 ones -> odd -> PE=1 (to make bits[0:17] even)
	// Bits 17-32: 1 1 0 1 0 1 0 0 0 0 1 1 0 0 0 1 -> 7 ones -> odd -> PO=0 (bits[17:34] already odd)
	frame := []byte{
		1,                                                     // bit 0: even parity (PE=1 so bits[0:17] has 4 ones = even)
		0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 0, 0, 1, 0, 0, 0,     // bits 1-16: FC=200
		1, 1, 0, 1, 0, 1, 0, 0, 0, 0, 1, 1, 0, 0, 0, 1,     // bits 17-32: Card=54321
		0,                                                     // bit 33: odd parity (PO=0 so bits[17:34] has 7 ones = odd)
	}
	fc, card, err := decodeWiegand34(frame)
	if err != nil {
		t.Fatalf("decodeWiegand34() error: %v", err)
	}
	if fc != 200 {
		t.Errorf("facility code = %d, want 200", fc)
	}
	if card != 54321 {
		t.Errorf("card number = %d, want 54321", card)
	}
}

func TestDecodeWiegand34_ParityError(t *testing.T) {
	frame := []byte{
		0,                                                     // bit 0: WRONG parity (should be 1)
		0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 0, 0, 1, 0, 0, 0,
		1, 1, 0, 1, 0, 1, 0, 0, 0, 0, 1, 1, 0, 0, 0, 1,
		0,
	}
	_, _, err := decodeWiegand34(frame)
	if err == nil {
		t.Fatal("decodeWiegand34() expected parity error")
	}
}

// validFrame returns a parity-valid Wiegand frame of the given bit length
// with FC=0 and Card=0 (all data bits zero). Only valid for 26 or 34 bits.
// For 26 bits: PE=0 (0 ones in data bits 1-12), PO=1 (odd parity over bits 13-25).
// For 34 bits: PE=0 (0 ones in data bits 1-16), PO=1 (odd parity over bits 17-33).
func validFrame(bitCount int) []byte {
	frame := make([]byte, bitCount)
	if bitCount == 26 || bitCount == 34 {
		frame[bitCount-1] = 1 // set trailing odd parity bit so bits[mid:end] has 1 one = odd
	}
	return frame
}

func TestDecodeWiegandFrame(t *testing.T) {
	tests := []struct {
		name     string
		frame    []byte
		wantType string
		wantErr  bool
	}{
		{"26 bits", validFrame(26), "wiegand_26", false},
		{"34 bits", validFrame(34), "wiegand_34", false},
		{"20 bits", make([]byte, 20), "", true},
		{"0 bits", make([]byte, 0), "", true},
		{"40 bits", make([]byte, 40), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, _, _, err := decodeWiegandFrame(tt.frame)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeWiegandFrame(%d bits) err = %v, wantErr %v", len(tt.frame), err, tt.wantErr)
			}
			if err == nil && gotType != tt.wantType {
				t.Errorf("decodeWiegandFrame(%d bits) type = %q, want %q", len(tt.frame), gotType, tt.wantType)
			}
		})
	}
}
