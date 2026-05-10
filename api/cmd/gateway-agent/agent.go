package main

import (
	"bytes"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Agent is the main Gateway agent that runs on the edge device.
type Agent struct {
	logger             *slog.Logger
	apiURL             string
	gatewayID          string
	tenantID           string
	bootstrapToken     string
	deviceTokenFile    string // path to persist device token across restarts
	configPollInterval time.Duration
	heartbeatInterval  time.Duration
	unlockDuration     time.Duration
	relayGPIOPin       int
	relayRS485Device   string
	relayOSDPDevice    string // RS485 serial device for OSDP v2 reader control
	osdpAddress        byte   // OSDP peripheral device address (0-126)
	tlsPinSHA256       string // hex-encoded SHA256 of Cloud API's TLS certificate public key (SPKI)

	mu              sync.RWMutex
	deviceToken     string // device-specific token obtained from registration
	accessRules     []AccessRule
	ruleVersion     string
	rulesUpdatedAt  time.Time     // when access rules were last successfully pulled
	rulesCacheTTL   time.Duration // max age of cached rules before denying access (0 = no TTL)
	eventQueue      []AccessEvent
	stopCh          chan struct{}
	relay           RelayDriver
	nonceCache      *NonceCache // replay-protection cache for v2 challenge nonces
	gatewayIDUint32 uint32      // numeric gateway ID embedded in v2 challenges
}

// AccessRule is a local credential → lock mapping for offline decision.
type AccessRule struct {
	CredentialType string   `json:"credential_type"`
	CredentialData string   `json:"credential_data"`
	UserID         string   `json:"user_id"`
	UserEmail      string   `json:"user_email"`
	LockIDs        []string `json:"lock_ids"`
}

// AccessEvent is queued for upload to Cloud.
type AccessEvent struct {
	GatewayID  string `json:"gateway_id"`
	EventType  string `json:"event_type"`
	LockID     string `json:"lock_id,omitempty"`
	Actor      string `json:"actor,omitempty"`
	Result     string `json:"result"`
	OccurredAt string `json:"occurred_at"`
}

func (a *Agent) Start() error {
	a.stopCh = make(chan struct{})

	// Derive numeric gateway ID from string ID (used in v2 challenges for binding)
	if a.gatewayID != "" {
		h := sha256.Sum256([]byte(a.gatewayID))
		a.gatewayIDUint32 = binary.BigEndian.Uint32(h[:4])
	}

	// Initialize nonce replay-protection cache for v2 challenge verification
	a.nonceCache = NewNonceCache(10000, 30*time.Second)

	// Initialize relay driver (priority: GPIO > OSDP > RS485 Modbus > DryRun)
	if a.relayGPIOPin >= 0 {
		driver, err := NewGPIORelay(a.relayGPIOPin, a.logger)
		if err != nil {
			return fmt.Errorf("gpio relay init: %w", err)
		}
		a.relay = driver
	} else if a.relayOSDPDevice != "" {
		driver, err := NewOSDPRelay(a.relayOSDPDevice, a.osdpAddress, a.logger)
		if err != nil {
			return fmt.Errorf("osdp relay init: %w", err)
		}
		a.relay = driver
	} else if a.relayRS485Device != "" {
		driver, err := NewRS485Relay(a.relayRS485Device, a.logger)
		if err != nil {
			return fmt.Errorf("rs485 relay init: %w", err)
		}
		a.relay = driver
	} else {
		a.relay = &DryRunRelay{logger: a.logger}
	}

	// Load persisted device token if available
	if err := a.loadDeviceToken(); err != nil {
		a.logger.Debug("no persisted device token found, will register", "error", err)
	}

	// If no device token persisted, attempt registration with bootstrap token.
	// Check deviceToken directly (not activeToken) to avoid false triggers when
	// the device token file contains the same value as the bootstrap token.
	a.mu.RLock()
	hasDeviceToken := a.deviceToken != ""
	a.mu.RUnlock()
	if !hasDeviceToken && a.bootstrapToken != "" {
		a.logger.Info("no device token, attempting registration with bootstrap token")
		if err := a.registerDevice(); err != nil {
			a.logger.Warn("device registration failed, falling back to bootstrap token", "error", err)
		}
	}

	// Initial config pull
	if err := a.pullConfig(); err != nil {
		a.logger.Warn("initial config pull failed, will retry", "error", err)
	}

	// Background loops
	go a.configPollLoop()
	go a.heartbeatLoop()
	go a.eventPushLoop()

	return nil
}

// registerDevice calls the bootstrap register endpoint and stores the returned device token.
func (a *Agent) registerDevice() error {
	body, _ := json.Marshal(map[string]any{
		"serial_number":   a.gatewayID,
		"tenant_id":       a.tenantID,
		"building_id":     "",
		"device_capacity": 4,
	})

	url := strings.TrimRight(a.apiURL, "/") + "/api/v1/gateway/register"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bootstrap-Token", a.bootstrapToken)

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("register call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		// Already registered — this is OK, keep using bootstrap token until
		// we implement a token re-issue flow
		a.logger.Info("device already registered, using bootstrap token as fallback")
		return nil
	}
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var result struct {
		GatewayID   string `json:"gateway_id"`
		DeviceToken string `json:"device_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("register decode: %w", err)
	}

	if result.DeviceToken == "" {
		return fmt.Errorf("register returned empty device token")
	}

	a.mu.Lock()
	a.deviceToken = result.DeviceToken
	if result.GatewayID != "" {
		a.gatewayID = result.GatewayID
	}
	a.mu.Unlock()

	if err := a.saveDeviceToken(); err != nil {
		a.logger.Error("failed to persist device token", "error", err)
	}

	a.logger.Info("device registered, device token obtained", "gateway_id", a.gatewayID)
	return nil
}

// activeToken returns the best available token: device token if available, otherwise bootstrap.
func (a *Agent) activeToken() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.deviceToken != "" {
		return a.deviceToken
	}
	return a.bootstrapToken
}

// loadDeviceToken reads the device token from the persisted file.
func (a *Agent) loadDeviceToken() error {
	if a.deviceTokenFile == "" {
		return fmt.Errorf("no device token file configured")
	}
	data, err := os.ReadFile(a.deviceTokenFile)
	if err != nil {
		return err
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return fmt.Errorf("device token file is empty")
	}
	a.mu.Lock()
	a.deviceToken = token
	a.mu.Unlock()
	a.logger.Info("loaded persisted device token")
	return nil
}

// saveDeviceToken writes the device token to a local file for persistence across restarts.
func (a *Agent) saveDeviceToken() error {
	if a.deviceTokenFile == "" {
		return nil
	}
	a.mu.RLock()
	token := a.deviceToken
	a.mu.RUnlock()
	if token == "" {
		return nil
	}
	// Write with restrictive permissions (owner read/write only)
	return os.WriteFile(a.deviceTokenFile, []byte(token), 0600)
}

func (a *Agent) Stop() {
	close(a.stopCh)
	if a.relay != nil {
		a.relay.Close()
	}
}

// --- Config Pull ---

func (a *Agent) pullConfig() error {
	body, _ := json.Marshal(map[string]string{
		"gateway_id":         a.gatewayID,
		"tenant_id":          a.tenantID,
		"current_version":    "",
		"authz_cache_version": a.ruleVersion,
	})

	resp, err := a.apiRequest("POST", "/api/v1/gateway/config/pull", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("config/pull status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var result struct {
		AuthzCache struct {
			Version     string       `json:"version"`
			AccessRules []AccessRule `json:"access_rules"`
		} `json:"authz_cache"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("config/pull decode: %w", err)
	}

	a.mu.Lock()
	a.accessRules = result.AuthzCache.AccessRules
	a.ruleVersion = result.AuthzCache.Version
	a.rulesUpdatedAt = time.Now().UTC()
	a.mu.Unlock()

	a.logger.Info("config pulled",
		"version", result.AuthzCache.Version,
		"access_rules", len(result.AuthzCache.AccessRules),
	)
	return nil
}

func (a *Agent) configPollLoop() {
	ticker := time.NewTicker(a.configPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := a.pullConfig(); err != nil {
				a.logger.Warn("config poll failed", "error", err)
			}
		case <-a.stopCh:
			return
		}
	}
}

// --- Heartbeat ---

func (a *Agent) heartbeatLoop() {
	ticker := time.NewTicker(a.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.sendHeartbeat()
		case <-a.stopCh:
			return
		}
	}
}

func (a *Agent) sendHeartbeat() {
	body, _ := json.Marshal(map[string]string{
		"gateway_id": a.gatewayID,
		"tenant_id":  a.tenantID,
	})
	resp, err := a.apiRequest("POST", "/api/v1/gateway/heartbeat", body)
	if err != nil {
		a.logger.Debug("heartbeat failed", "error", err)
		return
	}
	resp.Body.Close()
}

// --- Event Push ---

func (a *Agent) queueEvent(evt AccessEvent) {
	a.mu.Lock()
	a.eventQueue = append(a.eventQueue, evt)
	a.mu.Unlock()
}

func (a *Agent) eventPushLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.pushEvents()
		case <-a.stopCh:
			a.pushEvents() // flush on shutdown
			return
		}
	}
}

func (a *Agent) pushEvents() {
	a.mu.Lock()
	if len(a.eventQueue) == 0 {
		a.mu.Unlock()
		return
	}
	batch := make([]AccessEvent, len(a.eventQueue))
	copy(batch, a.eventQueue)
	a.eventQueue = a.eventQueue[:0]
	a.mu.Unlock()

	body, _ := json.Marshal(map[string]any{
		"gateway_id": a.gatewayID,
		"tenant_id":  a.tenantID,
		"events":     batch,
	})
	resp, err := a.apiRequest("POST", "/api/v1/gateway/events/batch", body)
	if err != nil {
		a.logger.Warn("event push failed, requeueing", "error", err, "count", len(batch))
		a.mu.Lock()
		a.eventQueue = append(batch, a.eventQueue...)
		a.mu.Unlock()
		return
	}
	resp.Body.Close()
	a.logger.Info("events pushed", "count", len(batch))
}

// --- Local Access Decision ---

// VerifyCredential checks the local access rule cache and returns allow/deny.
// If rulesCacheTTL is set and the cached rules are older than the TTL, all access is denied.
func (a *Agent) VerifyCredential(credentialType, credentialData, lockID string) (decision string, userID string, userEmail string) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Deny all access if rules cache has expired
	if a.rulesCacheTTL > 0 && !a.rulesUpdatedAt.IsZero() {
		if time.Since(a.rulesUpdatedAt) > a.rulesCacheTTL {
			return "deny", "", ""
		}
	}

	normalizedData := strings.ToUpper(strings.TrimSpace(credentialData))
	normalizedType := strings.ToLower(strings.TrimSpace(credentialType))

	for _, rule := range a.accessRules {
		if strings.ToLower(rule.CredentialType) != normalizedType {
			continue
		}
		if strings.ToUpper(rule.CredentialData) != normalizedData {
			continue
		}
		for _, id := range rule.LockIDs {
			if id == lockID {
				return "allow", rule.UserID, rule.UserEmail
			}
		}
		// Credential found but no access to this lock
		return "deny", rule.UserID, rule.UserEmail
	}
	return "deny", "", ""
}

// VerifyBLEAuth performs BLE challenge-response verification using ECDSA.
// It looks up the user's public key from cached access rules (type=ble_signature)
// and verifies the signature over SHA256(nonce || userID).
// Returns allow/deny with user info.
func (a *Agent) VerifyBLEAuth(challenge *BLEChallenge, response *BLEAuthResponse, lockID string) (decision string, userID string, userEmail string) {
	if challenge == nil || response == nil {
		return "deny", "", ""
	}

	// Check challenge hasn't expired
	if challenge.IsExpired() {
		return "deny", response.UserID, ""
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	// Deny all access if rules cache has expired
	if a.rulesCacheTTL > 0 && !a.rulesUpdatedAt.IsZero() {
		if time.Since(a.rulesUpdatedAt) > a.rulesCacheTTL {
			return "deny", response.UserID, ""
		}
	}

	// Find the BLE credential rule for this user
	for _, rule := range a.accessRules {
		if strings.ToLower(rule.CredentialType) != "ble_signature" {
			continue
		}
		if rule.UserID != response.UserID {
			continue
		}

		// Verify signature using stored public key (rule.CredentialData = PEM public key)
		err := VerifyBLESignature(rule.CredentialData, challenge.Nonce, response.UserID, response.Signature)
		if err != nil {
			a.logger.Warn("BLE signature verification failed",
				"user_id", response.UserID,
				"error", err,
			)
			return "deny", rule.UserID, rule.UserEmail
		}

		// Signature valid — check if user has access to this lock
		for _, id := range rule.LockIDs {
			if id == lockID {
				return "allow", rule.UserID, rule.UserEmail
			}
		}
		// Valid credential but no access to this lock
		return "deny", rule.UserID, rule.UserEmail
	}

	return "deny", response.UserID, ""
}

// VerifyAuthResponseV2 performs the full v2 verification chain in fast-to-slow order:
// nonce cache -> gateway_id -> credential lookup -> ECDSA verify.
// transport is "BLE" or "NFC_HCE" (use TransportTagBLE / TransportTagNFCHCE).
func (a *Agent) VerifyAuthResponseV2(resp *BLEAuthResponse, challenge []byte, transport string) BLEAuthResult {
	if len(challenge) < 32 {
		return BLEAuthResult{Code: BLEResultDenied, Reason: "challenge_too_short"}
	}
	nonce := challenge[:32]

	// 1. Nonce reuse check (fastest — in-memory LRU)
	if a.nonceCache != nil && a.nonceCache.Contains(nonce) {
		return BLEAuthResult{Code: BLEResultDenied, Reason: "nonce_reuse"}
	}

	// 2. Gateway ID check (fast — compare uint32)
	if len(challenge) >= ChallengeV2Size {
		gatewayID := binary.BigEndian.Uint32(challenge[48:52])
		if gatewayID != a.gatewayIDUint32 {
			return BLEAuthResult{Code: BLEResultDenied, Reason: "gateway_id_mismatch"}
		}
	}

	// 3. Credential lookup from cached access rules
	// A user may have multiple active credentials (e.g. phone + tablet),
	// so we collect all matching public keys and try each one.
	type candidateKey struct {
		publicKeyPEM string
		userEmail    string
	}
	a.mu.RLock()
	var candidates []candidateKey
	for _, rule := range a.accessRules {
		if rule.CredentialType == "ble_signature" && rule.UserID == resp.UserID {
			candidates = append(candidates, candidateKey{
				publicKeyPEM: rule.CredentialData,
				userEmail:    rule.UserEmail,
			})
		}
	}
	a.mu.RUnlock()

	if len(candidates) == 0 {
		return BLEAuthResult{Code: BLEResultUnknownUser, Reason: "unknown_user"}
	}

	// 4. ECDSA signature verification — try each registered key
	var nonceArr [32]byte
	copy(nonceArr[:], nonce)
	var lastErr error
	var matchedEmail string
	for _, cand := range candidates {
		err := VerifyBLESignatureV2(cand.publicKeyPEM, nonceArr, resp.UserID, transport, resp.Signature)
		if err == nil {
			matchedEmail = cand.userEmail
			lastErr = nil
			break
		}
		lastErr = err
	}
	if lastErr != nil {
		return BLEAuthResult{Code: BLEResultInvalidSignature, Reason: lastErr.Error()}
	}

	// 5. Mark nonce as used (only after successful verification)
	if a.nonceCache != nil {
		a.nonceCache.Add(nonce)
	}

	_ = matchedEmail // available for audit logging in caller
	return BLEAuthResult{Code: BLEResultGranted, Reason: "access_granted"}
}

// HandleBLEAuth is the high-level handler for a complete BLE authentication.
// Called by the BLE reader after receiving an auth response from a phone.
func (a *Agent) HandleBLEAuth(challenge *BLEChallenge, response *BLEAuthResponse, lockID string) BLEAuthResult {
	decision, userID, userEmail := a.VerifyBLEAuth(challenge, response, lockID)

	logger := a.logger.With(
		"credential_type", "ble_signature",
		"lock_id", lockID,
		"user_id", userID,
		"decision", decision,
	)

	if decision == "allow" {
		logger.Info("BLE ACCESS GRANTED — unlocking door")
		if err := a.relay.Unlock(a.unlockDuration); err != nil {
			logger.Error("relay unlock failed", "error", err)
		}
		a.queueEvent(AccessEvent{
			GatewayID:  a.gatewayID,
			EventType:  "access_granted",
			LockID:     lockID,
			Actor:      userEmail,
			Result:     "allow",
			OccurredAt: time.Now().UTC().Format(time.RFC3339),
		})
		return BLEAuthResult{Code: BLEResultGranted, Reason: "access_granted"}
	}

	reason := "no_access"
	code := BLEResultDenied
	if challenge.IsExpired() {
		reason = "challenge_expired"
		code = BLEResultExpiredChallenge
	} else if userID == "" {
		reason = "unknown_user"
		code = BLEResultUnknownUser
	}

	logger.Warn("BLE ACCESS DENIED", "reason", reason)
	a.queueEvent(AccessEvent{
		GatewayID:  a.gatewayID,
		EventType:  "access_denied",
		LockID:     lockID,
		Actor:      userEmail,
		Result:     reason,
		OccurredAt: time.Now().UTC().Format(time.RFC3339),
	})
	return BLEAuthResult{Code: code, Reason: reason}
}

// HandleCredentialPresented is called when a reader detects a credential.
// It performs local decision, drives the relay if allowed, and queues an event.
func (a *Agent) HandleCredentialPresented(credentialType, credentialData, lockID string) {
	now := time.Now().UTC()
	decision, userID, userEmail := a.VerifyCredential(credentialType, credentialData, lockID)

	logger := a.logger.With(
		"credential_type", credentialType,
		"lock_id", lockID,
		"user_id", userID,
		"decision", decision,
	)

	if decision == "allow" {
		logger.Info("ACCESS GRANTED — unlocking door")
		if err := a.relay.Unlock(a.unlockDuration); err != nil {
			logger.Error("relay unlock failed", "error", err)
		}
	} else {
		// Check if denial is due to stale cache
		a.mu.RLock()
		cacheStale := a.rulesCacheTTL > 0 && !a.rulesUpdatedAt.IsZero() && time.Since(a.rulesUpdatedAt) > a.rulesCacheTTL
		a.mu.RUnlock()
		if cacheStale {
			logger.Error("ACCESS DENIED — rules cache expired, all access blocked until Cloud resync")
		} else {
			logger.Warn("ACCESS DENIED")
		}
	}

	eventType := "access_denied"
	if decision == "allow" {
		eventType = "access_granted"
	}
	a.queueEvent(AccessEvent{
		GatewayID:  a.gatewayID,
		EventType:  eventType,
		LockID:     lockID,
		Actor:      userEmail,
		Result:     decision,
		OccurredAt: now.Format(time.RFC3339),
	})

	_ = userID
}

// --- HTTP Client ---

func (a *Agent) httpClient() *http.Client {
	if a.tlsPinSHA256 == "" {
		// Use standard TLS verification (system CA bundle) when no pin is configured.
		return &http.Client{Timeout: 30 * time.Second}
	}
	pinBytes, err := hex.DecodeString(a.tlsPinSHA256)
	if err != nil || len(pinBytes) != sha256.Size {
		a.logger.Warn("invalid tls-pin-sha256, falling back to standard TLS verification", "error", err)
		return &http.Client{Timeout: 30 * time.Second}
	}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				VerifyConnection: func(cs tls.ConnectionState) error {
					for _, cert := range cs.PeerCertificates {
						spkiHash := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
						if hex.EncodeToString(spkiHash[:]) == a.tlsPinSHA256 {
							return nil
						}
					}
					return fmt.Errorf("TLS certificate pinning failed: no certificate matched pin %s", a.tlsPinSHA256)
				},
			},
		},
	}
}

func (a *Agent) apiRequest(method, path string, body []byte) (*http.Response, error) {
	url := strings.TrimRight(a.apiURL, "/") + path
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	token := a.activeToken()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-Device-Token", token)
	}
	// Per-request nonce + timestamp for replay protection
	nonceBytes := make([]byte, 16)
	crand.Read(nonceBytes)
	req.Header.Set("X-Request-Nonce", hex.EncodeToString(nonceBytes))
	req.Header.Set("X-Request-Timestamp", time.Now().UTC().Format(time.RFC3339))
	return a.httpClient().Do(req)
}
