package main

import (
	"bytes"
	"testing"
)

func TestBuildSelectAID(t *testing.T) {
	cmd := BuildSelectAID()
	// Expected: 00 A4 04 00 08 [8-byte AID] 00
	if len(cmd) != 5+len(NFCAID)+1 {
		t.Fatalf("expected %d bytes, got %d", 5+len(NFCAID)+1, len(cmd))
	}
	// CLA=00, INS=A4, P1=04, P2=00
	if cmd[0] != 0x00 || cmd[1] != 0xA4 || cmd[2] != 0x04 || cmd[3] != 0x00 {
		t.Fatalf("unexpected header: %02X %02X %02X %02X", cmd[0], cmd[1], cmd[2], cmd[3])
	}
	// Lc = AID length
	if cmd[4] != byte(len(NFCAID)) {
		t.Fatalf("expected Lc=%d, got %d", len(NFCAID), cmd[4])
	}
	// AID bytes
	if !bytes.Equal(cmd[5:5+len(NFCAID)], NFCAID) {
		t.Fatal("AID mismatch in SELECT command")
	}
	// Le=00
	if cmd[len(cmd)-1] != 0x00 {
		t.Fatalf("expected Le=00, got %02X", cmd[len(cmd)-1])
	}
}

func TestBuildAuthenticate(t *testing.T) {
	challenge := make([]byte, ChallengeV2Size)
	for i := range challenge {
		challenge[i] = byte(i)
	}

	cmd := BuildAuthenticate(challenge)
	// Expected: 80 88 00 00 34 [52-byte challenge] 00
	expectedLen := 5 + ChallengeV2Size + 1
	if len(cmd) != expectedLen {
		t.Fatalf("expected %d bytes, got %d", expectedLen, len(cmd))
	}
	if cmd[0] != 0x80 || cmd[1] != 0x88 || cmd[2] != 0x00 || cmd[3] != 0x00 {
		t.Fatalf("unexpected header: %02X %02X %02X %02X", cmd[0], cmd[1], cmd[2], cmd[3])
	}
	if cmd[4] != byte(ChallengeV2Size) {
		t.Fatalf("expected Lc=%d, got %d", ChallengeV2Size, cmd[4])
	}
	if !bytes.Equal(cmd[5:5+ChallengeV2Size], challenge) {
		t.Fatal("challenge data mismatch in AUTHENTICATE command")
	}
	if cmd[len(cmd)-1] != 0x00 {
		t.Fatalf("expected Le=00, got %02X", cmd[len(cmd)-1])
	}
}

func TestBuildAuthenticate_WrongSize(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for wrong challenge size")
		}
	}()
	BuildAuthenticate([]byte{1, 2, 3})
}

func TestParseAPDUResponse_OK(t *testing.T) {
	// Simulate: 3 bytes data + 90 00
	resp := []byte{0x01, 0x02, 0x03, 0x90, 0x00}
	data, sw, err := ParseAPDUResponse(resp)
	if err != nil {
		t.Fatalf("ParseAPDUResponse: %v", err)
	}
	if sw != SW_OK {
		t.Fatalf("expected SW=%04X, got %04X", SW_OK, sw)
	}
	if !bytes.Equal(data, []byte{0x01, 0x02, 0x03}) {
		t.Fatalf("unexpected data: %v", data)
	}
}

func TestParseAPDUResponse_ErrorSW(t *testing.T) {
	// Simulate: no data + 6A 82 (app not found)
	resp := []byte{0x6A, 0x82}
	data, sw, err := ParseAPDUResponse(resp)
	if err != nil {
		t.Fatalf("ParseAPDUResponse: %v", err)
	}
	if sw != SW_APP_NOT_FOUND {
		t.Fatalf("expected SW=%04X, got %04X", SW_APP_NOT_FOUND, sw)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty data, got %d bytes", len(data))
	}
}

func TestParseAPDUResponse_TooShort(t *testing.T) {
	_, _, err := ParseAPDUResponse([]byte{0x90})
	if err == nil {
		t.Fatal("expected error for 1-byte response")
	}

	_, _, err = ParseAPDUResponse(nil)
	if err == nil {
		t.Fatal("expected error for nil response")
	}
}

func TestParseNFCAuthResponse_Valid(t *testing.T) {
	userID := "usr_nfc_001"
	sig := []byte{0x30, 0x44, 0x02, 0x20, 0xAA, 0xBB}

	// Build: [1B len][userId][signature]
	data := make([]byte, 1+len(userID)+len(sig))
	data[0] = byte(len(userID))
	copy(data[1:1+len(userID)], userID)
	copy(data[1+len(userID):], sig)

	resp, err := ParseNFCAuthResponse(data)
	if err != nil {
		t.Fatalf("ParseNFCAuthResponse: %v", err)
	}
	if resp.UserID != userID {
		t.Fatalf("expected userID %q, got %q", userID, resp.UserID)
	}
	if !bytes.Equal(resp.Signature, sig) {
		t.Fatalf("signature mismatch: got %v", resp.Signature)
	}
}

func TestParseNFCAuthResponse_TooShort(t *testing.T) {
	_, err := ParseNFCAuthResponse([]byte{0x05})
	if err == nil {
		t.Fatal("expected error for 1-byte data")
	}

	_, err = ParseNFCAuthResponse(nil)
	if err == nil {
		t.Fatal("expected error for nil data")
	}
}

func TestParseNFCAuthResponse_Truncated(t *testing.T) {
	// Claims userId is 10 bytes, but only 3 bytes follow (no room for sig)
	data := []byte{0x0A, 0x41, 0x42, 0x43}
	_, err := ParseNFCAuthResponse(data)
	if err == nil {
		t.Fatal("expected error for truncated userId")
	}
}

func TestParseNFCAuthResponse_UserIDTooLong(t *testing.T) {
	// userId length byte = 200, exceeds max 180
	data := make([]byte, 202)
	data[0] = 200
	_, err := ParseNFCAuthResponse(data)
	if err == nil {
		t.Fatal("expected error for userId > 180 bytes")
	}
}

func TestParseNFCAuthResponse_CompatibleWithBLEDecode(t *testing.T) {
	// Both ParseNFCAuthResponse and DecodeBLEAuthResponse use the same wire format.
	// Verify they produce equivalent results.
	userID := "usr_compat_001"
	sig := []byte{0x30, 0x46, 0x02, 0x21, 0x00, 0xCC}

	encoded := EncodeBLEAuthResponse(userID, sig)

	nfcResp, err := ParseNFCAuthResponse(encoded)
	if err != nil {
		t.Fatalf("ParseNFCAuthResponse: %v", err)
	}
	bleResp, err := DecodeBLEAuthResponse(encoded)
	if err != nil {
		t.Fatalf("DecodeBLEAuthResponse: %v", err)
	}

	if nfcResp.UserID != bleResp.UserID {
		t.Fatalf("userId mismatch: NFC=%q, BLE=%q", nfcResp.UserID, bleResp.UserID)
	}
	if !bytes.Equal(nfcResp.Signature, bleResp.Signature) {
		t.Fatal("signature mismatch between NFC and BLE parse")
	}
}

func TestNFCAID(t *testing.T) {
	if len(NFCAID) != 8 {
		t.Fatalf("expected AID length 8, got %d", len(NFCAID))
	}
	// First byte should be F0 (proprietary range)
	if NFCAID[0] != 0xF0 {
		t.Fatalf("AID should start with F0, got %02X", NFCAID[0])
	}
}

func TestStatusWordConstants(t *testing.T) {
	// Verify ISO 7816-4 status word values
	tests := []struct {
		name     string
		sw       uint16
		expected uint16
	}{
		{"SW_OK", SW_OK, 0x9000},
		{"SW_SECURITY_NOT_SATISFIED", SW_SECURITY_NOT_SATISFIED, 0x6982},
		{"SW_CONDITIONS_NOT_MET", SW_CONDITIONS_NOT_MET, 0x6985},
		{"SW_APP_NOT_FOUND", SW_APP_NOT_FOUND, 0x6A82},
		{"SW_INTERNAL_ERROR", SW_INTERNAL_ERROR, 0x6F00},
	}
	for _, tt := range tests {
		if tt.sw != tt.expected {
			t.Errorf("%s: expected %04X, got %04X", tt.name, tt.expected, tt.sw)
		}
	}
}
