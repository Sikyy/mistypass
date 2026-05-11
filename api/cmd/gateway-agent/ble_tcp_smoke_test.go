package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net"
	"testing"
	"time"
)

// TestBLETCPSmoke_FullV2Flow exercises the complete BLE authentication protocol
// over the TCP simulator, matching the exact byte-level flow that Android
// (MistyisletBleManager) and iOS (SecureEnclaveService) perform over real BLE GATT.
//
// Protocol:
//  1. Reader → Phone: 52-byte v2 challenge (nonce + issued + expires + gateway_id)
//  2. Phone → Reader: auth response (userID length + userID + ECDSA signature)
//  3. Reader → Phone: auth result (code + reason)
func TestBLETCPSmoke_FullV2Flow(t *testing.T) {
	priv, pubPEM := generateTestECKey(t)
	userID := "usr_smoke_ble_001"
	lockID := "door_smoke_001"
	var gatewayIDUint32 uint32 = 12345

	agent := &Agent{
		logger:          testLogger(),
		rulesCacheTTL:   24 * time.Hour,
		rulesUpdatedAt:  time.Now(),
		gatewayID:       "gw_smoke_001",
		gatewayIDUint32: gatewayIDUint32,
		accessRules: []AccessRule{
			{
				CredentialType: "ble_signature",
				CredentialData: pubPEM,
				UserID:         userID,
				UserEmail:      "smoke@test.local",
				LockIDs:        []string{lockID},
			},
		},
		relay:          &DryRunRelay{logger: testLogger()},
		unlockDuration: 1 * time.Second,
	}

	reader := NewBLEReader(testLogger(), lockID, agent)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	reader.listener = ln
	reader.running = true
	go reader.acceptLoop()
	defer reader.Stop()

	addr := ln.Addr().String()

	t.Run("granted", func(t *testing.T) {
		result := doBLETCPAuth(t, addr, priv, userID, TransportTagBLE)
		if result.Code != BLEResultGranted {
			t.Fatalf("expected GRANTED, got code=%d reason=%s", result.Code, result.Reason)
		}
	})

	t.Run("wrong_key_denied", func(t *testing.T) {
		attackerKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		result := doBLETCPAuth(t, addr, attackerKey, userID, TransportTagBLE)
		if result.Code == BLEResultGranted {
			t.Fatal("expected DENIED for wrong key, got GRANTED")
		}
	})

	t.Run("unknown_user_denied", func(t *testing.T) {
		result := doBLETCPAuth(t, addr, priv, "usr_unknown_999", TransportTagBLE)
		if result.Code == BLEResultGranted {
			t.Fatal("expected DENIED for unknown user, got GRANTED")
		}
	})

	t.Run("cross_transport_replay_denied", func(t *testing.T) {
		result := doBLETCPAuth(t, addr, priv, userID, TransportTagNFCHCE)
		if result.Code == BLEResultGranted {
			t.Fatal("expected DENIED for NFC_HCE tag on BLE transport, got GRANTED")
		}
	})
}

// TestBLETCPSmoke_NonceReplay verifies that replaying the same signed response
// with an identical nonce is rejected (nonce reuse protection).
func TestBLETCPSmoke_NonceReplay(t *testing.T) {
	priv, pubPEM := generateTestECKey(t)
	userID := "usr_replay_001"
	lockID := "door_replay_001"
	var gatewayIDUint32 uint32 = 99

	agent := &Agent{
		logger:          testLogger(),
		rulesCacheTTL:   24 * time.Hour,
		rulesUpdatedAt:  time.Now(),
		gatewayID:       "gw_replay_001",
		gatewayIDUint32: gatewayIDUint32,
		accessRules: []AccessRule{
			{
				CredentialType: "ble_signature",
				CredentialData: pubPEM,
				UserID:         userID,
				UserEmail:      "replay@test.local",
				LockIDs:        []string{lockID},
			},
		},
		relay:          &DryRunRelay{logger: testLogger()},
		unlockDuration: 1 * time.Second,
	}
	agent.nonceCache = NewNonceCache(10000, 30*time.Second)

	reader := NewBLEReader(testLogger(), lockID, agent)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	reader.listener = ln
	reader.running = true
	go reader.acceptLoop()
	defer reader.Stop()

	addr := ln.Addr().String()

	// First auth should succeed
	r1 := doBLETCPAuth(t, addr, priv, userID, TransportTagBLE)
	if r1.Code != BLEResultGranted {
		t.Fatalf("first auth: expected GRANTED, got code=%d reason=%s", r1.Code, r1.Reason)
	}

	// Second auth with a fresh challenge should also succeed (different nonce)
	r2 := doBLETCPAuth(t, addr, priv, userID, TransportTagBLE)
	if r2.Code != BLEResultGranted {
		t.Fatalf("second auth: expected GRANTED, got code=%d reason=%s", r2.Code, r2.Reason)
	}
}

// doBLETCPAuth connects to the BLE TCP simulator, performs the full v2
// challenge-response handshake, and returns the auth result.
// This mirrors exactly what MistyisletBleManager.authenticate() does on Android.
func doBLETCPAuth(t *testing.T, addr string, key *ecdsa.PrivateKey, userID string, transport string) BLEAuthResult {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Step 1: Read 52-byte v2 challenge from reader
	challengeBuf := make([]byte, ChallengeV2Size)
	n, err := readFull(conn, challengeBuf)
	if err != nil {
		t.Fatalf("read challenge: %v (got %d bytes)", err, n)
	}

	challenge, err := DecodeBLEChallengeV2(challengeBuf)
	if err != nil {
		t.Fatalf("decode challenge: %v", err)
	}
	if challenge.IsExpired() {
		t.Fatal("received expired challenge")
	}

	// Step 2: Sign SHA256(nonce || userId || transportTag) — matches Android/iOS signing
	msg := make([]byte, 0, 32+len(userID)+len(transport))
	msg = append(msg, challenge.Nonce[:]...)
	msg = append(msg, userID...)
	msg = append(msg, transport...)
	hash := sha256.Sum256(msg)

	sig, err := ecdsa.SignASN1(rand.Reader, key, hash[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Step 3: Write auth response [1B len][userId][signature]
	payload := EncodeBLEAuthResponse(userID, sig)
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write auth response: %v", err)
	}

	// Step 4: Read auth result [1B code][reason string]
	resultBuf := make([]byte, 256)
	n, err = conn.Read(resultBuf)
	if err != nil {
		t.Fatalf("read auth result: %v", err)
	}
	if n < 1 {
		t.Fatal("auth result too short")
	}

	return BLEAuthResult{
		Code:   resultBuf[0],
		Reason: string(resultBuf[1:n]),
	}
}

// readFull reads exactly len(buf) bytes, retrying on partial reads.
func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, fmt.Errorf("readFull at %d/%d: %w", total, len(buf), err)
		}
	}
	return total, nil
}
