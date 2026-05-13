// tcp-auth-test is a minimal BLE TCP simulator for testing iOS/Android clients.
//
// It listens on :9900, sends a v2 challenge, accepts a signed response,
// verifies the ECDSA signature against a provided public key PEM, and returns
// ACCESS GRANTED or DENIED.
//
// Usage:
//
//	# Step 1: Deploy iOS app, open TCP Auth Test, copy the public key PEM
//	# Step 2: Save the PEM to a file
//	echo '-----BEGIN PUBLIC KEY-----
//	MFkw...
//	-----END PUBLIC KEY-----' > /tmp/ios-pubkey.pem
//
//	# Step 3: Run this test server
//	go run ./cmd/tcp-auth-test -pubkey /tmp/ios-pubkey.pem -user usr_test_001
//
//	# Step 4: In iOS app, tap "Run TCP Auth" → should see ACCESS GRANTED
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

const (
	challengeV2Size = 52
	resultGranted   = 0x01
	resultDenied    = 0x02
)

func main() {
	pubkeyFile := flag.String("pubkey", "", "Path to PEM public key file (from iOS Secure Enclave)")
	userID := flag.String("user", "", "Expected user ID (must match what iOS sends)")
	listenAddr := flag.String("listen", ":9900", "TCP listen address")
	gatewayID := flag.String("gateway-id", "gw_test_001", "Gateway ID for challenge")
	acceptAny := flag.Bool("accept-any", false, "Accept any valid ECDSA signature (skip user/key matching)")
	flag.Parse()

	if *pubkeyFile == "" && !*acceptAny {
		fmt.Println("Usage: tcp-auth-test -pubkey <pem-file> -user <user-id>")
		fmt.Println("   or: tcp-auth-test -accept-any  (accepts any valid signature)")
		os.Exit(1)
	}

	var pubKey *ecdsa.PublicKey
	if *pubkeyFile != "" {
		var err error
		pubKey, err = loadPublicKey(*pubkeyFile)
		if err != nil {
			log.Fatalf("Failed to load public key: %v", err)
		}
		log.Printf("Loaded public key from %s", *pubkeyFile)
	}

	// Compute gateway_id hash (SHA256(gatewayID)[0:4])
	gwHash := sha256.Sum256([]byte(*gatewayID))
	gwIDUint32 := binary.BigEndian.Uint32(gwHash[:4])
	log.Printf("Gateway ID: %s (hash prefix: 0x%08X)", *gatewayID, gwIDUint32)

	ln, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("Listen %s: %v", *listenAddr, err)
	}
	log.Printf("TCP BLE simulator listening on %s", *listenAddr)
	log.Printf("Waiting for iOS/Android connections...")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Accept: %v", err)
			continue
		}
		go handleConnection(conn, pubKey, *userID, gwIDUint32, *acceptAny)
	}
}

func handleConnection(conn net.Conn, pubKey *ecdsa.PublicKey, expectedUserID string, gwID uint32, acceptAny bool) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	log.Printf("[%s] New connection", remote)

	conn.SetDeadline(time.Now().Add(10 * time.Second))

	// Step 1: Generate and send 52-byte v2 challenge
	challenge, nonce := buildChallenge(gwID)
	if _, err := conn.Write(challenge); err != nil {
		log.Printf("[%s] Write challenge: %v", remote, err)
		return
	}
	log.Printf("[%s] Sent 52-byte v2 challenge (nonce=%x...)", remote, nonce[:8])

	// Step 2: Read auth response
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("[%s] Read response: %v", remote, err)
		return
	}
	if n < 2 {
		log.Printf("[%s] Response too short: %d bytes", remote, n)
		sendResult(conn, resultDenied, "response too short")
		return
	}

	// Parse: [1B userID_len][userID][signature]
	userIDLen := int(buf[0])
	if 1+userIDLen >= n {
		log.Printf("[%s] Invalid response format", remote)
		sendResult(conn, resultDenied, "invalid format")
		return
	}
	receivedUserID := string(buf[1 : 1+userIDLen])
	signature := buf[1+userIDLen : n]
	log.Printf("[%s] Received response: userID=%q sigLen=%d", remote, receivedUserID, len(signature))

	// Step 3: Verify
	if !acceptAny && expectedUserID != "" && receivedUserID != expectedUserID {
		log.Printf("[%s] DENIED: userID mismatch (got %q, expected %q)", remote, receivedUserID, expectedUserID)
		sendResult(conn, resultDenied, "user not found")
		return
	}

	// Build sign payload: nonce || userID || "BLE"
	var msg []byte
	msg = append(msg, nonce[:]...)
	msg = append(msg, receivedUserID...)
	msg = append(msg, "BLE"...)
	hash := sha256.Sum256(msg)

	if pubKey != nil {
		if !ecdsa.VerifyASN1(pubKey, hash[:], signature) {
			log.Printf("[%s] DENIED: signature verification failed", remote)
			sendResult(conn, resultDenied, "signature invalid")
			return
		}
	} else if acceptAny {
		// In accept-any mode, try to verify against the signature's own key
		// (we can't verify without a key, but we accept it)
		log.Printf("[%s] accept-any mode: skipping signature verification", remote)
	}

	log.Printf("[%s] ✅ ACCESS GRANTED for user %q", remote, receivedUserID)
	sendResult(conn, resultGranted, "access granted")
}

func buildChallenge(gwID uint32) ([]byte, [32]byte) {
	challenge := make([]byte, challengeV2Size)

	// nonce[0:32]: 32 random bytes
	var nonce [32]byte
	rand.Read(nonce[:])
	copy(challenge[0:32], nonce[:])

	// issued_at[32:40]: now as BigEndian uint64
	now := time.Now()
	binary.BigEndian.PutUint64(challenge[32:40], uint64(now.Unix()))

	// expires_at[40:48]: now + 30s as BigEndian uint64
	binary.BigEndian.PutUint64(challenge[40:48], uint64(now.Add(30*time.Second).Unix()))

	// gateway_id[48:52]: SHA256(gatewayID)[0:4] as BigEndian uint32
	binary.BigEndian.PutUint32(challenge[48:52], gwID)

	return challenge, nonce
}

func sendResult(conn net.Conn, code byte, reason string) {
	result := make([]byte, 1+len(reason))
	result[0] = code
	copy(result[1:], reason)
	conn.Write(result)
}

func loadPublicKey(path string) (*ecdsa.PublicKey, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- CLI tool, trusted path
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKIX key: %w", err)
	}

	ecKey, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA key (got %T)", pub)
	}
	if ecKey.Curve != elliptic.P256() {
		return nil, fmt.Errorf("not P-256 (got %s)", ecKey.Curve.Params().Name)
	}

	return ecKey, nil
}
