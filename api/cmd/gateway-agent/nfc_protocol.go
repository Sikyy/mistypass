package main

import (
	"encoding/binary"
	"fmt"
)

// NFC HCE Application ID: F0 4D 49 53 54 59 01 00 (8 bytes)
var NFCAID = []byte{0xF0, 0x4D, 0x49, 0x53, 0x54, 0x59, 0x01, 0x00}

// APDU command builders

// BuildSelectAID constructs: 00 A4 04 00 08 [AID] 00
func BuildSelectAID() []byte {
	cmd := []byte{0x00, 0xA4, 0x04, 0x00, byte(len(NFCAID))}
	cmd = append(cmd, NFCAID...)
	cmd = append(cmd, 0x00) // Le
	return cmd
}

// BuildAuthenticate constructs: 80 88 00 00 34 [52-byte challenge] 00
func BuildAuthenticate(challenge []byte) ([]byte, error) {
	if len(challenge) != ChallengeV2Size {
		return nil, fmt.Errorf("challenge must be %d bytes, got %d", ChallengeV2Size, len(challenge))
	}
	cmd := []byte{0x80, 0x88, 0x00, 0x00, byte(ChallengeV2Size)}
	cmd = append(cmd, challenge...)
	cmd = append(cmd, 0x00) // Le
	return cmd, nil
}

// APDU response status words
const (
	SW_OK                     = 0x9000
	SW_SECURITY_NOT_SATISFIED = 0x6982
	SW_CONDITIONS_NOT_MET     = 0x6985
	SW_APP_NOT_FOUND          = 0x6A82
	SW_INTERNAL_ERROR         = 0x6F00
)

// ParseAPDUResponse extracts data and status word from a response.
func ParseAPDUResponse(resp []byte) (data []byte, sw uint16, err error) {
	if len(resp) < 2 {
		return nil, 0, fmt.Errorf("APDU response too short: %d bytes", len(resp))
	}
	sw = binary.BigEndian.Uint16(resp[len(resp)-2:])
	data = resp[:len(resp)-2]
	return data, sw, nil
}

// ParseNFCAuthResponse extracts userId and signature from AUTHENTICATE response data.
// Format: [1B userId_len][userId bytes][ECDSA signature]
func ParseNFCAuthResponse(data []byte) (*BLEAuthResponse, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("auth response too short")
	}
	userIDLen := int(data[0])
	if len(data) < 1+userIDLen+1 {
		return nil, fmt.Errorf("auth response truncated: need %d bytes for userId, have %d", userIDLen, len(data)-1)
	}
	if userIDLen > 180 {
		return nil, fmt.Errorf("userId too long: %d bytes, max 180", userIDLen)
	}
	userID := string(data[1 : 1+userIDLen])
	signature := data[1+userIDLen:]
	// ECDSA P-256 signatures are 64 bytes (raw r||s) or ~70-72 bytes (ASN.1 DER)
	if len(signature) < 64 {
		return nil, fmt.Errorf("signature too short: %d bytes, minimum 64", len(signature))
	}
	return &BLEAuthResponse{UserID: userID, Signature: signature}, nil
}
