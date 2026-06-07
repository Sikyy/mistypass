package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
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
	// Matter relay configuration
	relayMatterNodeID  uint64              // Matter node ID for target lock (0 = disabled)
	matterEndpoint     int                 // Door Lock cluster endpoint (default: 1)
	matterStorageDir   string              // chip-tool fabric credential storage
	matterSetupCode    string              // setup code for commissioning
	matterChipToolPath string              // path to chip-tool binary
	matterTimedTimeout int                 // --timedInteractionTimeoutMs value
	tlsPinSHA256       string              // hex-encoded SHA256 of Cloud API's TLS certificate public key (SPKI)
	wsURL              string              // WebSocket URL for persistent TLS connection (e.g. wss://api.example.com/api/v1/gateway/ws)
	mtlsCertDir        string              // directory for mTLS client cert + key (e.g. /var/lib/mistypass/mtls/)
	agentVersion       string              // build-time version, used for OTA anti-downgrade
	otaPublicKeys      []ed25519.PublicKey // pinned Ed25519 keys for OTA verification (empty = OTA disabled)

	mu              sync.RWMutex
	deviceToken     string // device-specific token obtained from registration
	accessRules     []AccessRule
	ruleVersion     string
	rulesUpdatedAt  time.Time     // when access rules were last successfully pulled
	rulesCacheTTL   time.Duration // MaxOfflineDuration: max age of cached rules before deny_all (0 = no TTL)
	eventQueue      []AccessEvent
	stopCh          chan struct{}
	relay           RelayDriver
	nonceCache      *NonceCache // replay-protection cache for v2 challenge nonces
	gatewayIDUint32 uint32      // numeric gateway ID embedded in v2 challenges

	// Offline state tracking
	offlineSince      time.Time // when connectivity was lost (zero = online)
	eventPushBackoff  time.Duration
	eventQueueDropped int64 // count of events dropped due to queue overflow

	// WebSocket persistent connection
	wsClient *WSClient

	// mTLS client authentication
	mtls *DeviceMTLS

	// Cached HTTP client (rebuilt on TLS config change)
	cachedHTTPClient *http.Client
}

// AccessRule is a local credential → lock mapping for offline decision.
//
// Security constraint: ValidUntil MUST be ≤ rulesCacheTTL (MaxOfflineDuration).
// The Cloud issues credentials with ValidUntil ≤ 72h. When the Gateway is online,
// WebSocket pushes refresh credentials continuously. When offline, both ValidUntil
// and rulesCacheTTL bound the revocation delay window.
type AccessRule struct {
	CredentialType string   `json:"credential_type"`
	CredentialData string   `json:"credential_data"`
	UserID         string   `json:"user_id"`
	UserEmail      string   `json:"user_email"`
	LockIDs        []string `json:"lock_ids"`
	ValidUntil     string   `json:"valid_until,omitempty"` // RFC3339; empty = valid until cache expires
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

	// OTA rollback watchdog: if a pending self-update is not confirmed in time,
	// restore the previous binary. No-op when there is no pending marker.
	go a.otaWatchdog(90 * time.Second)

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
	} else if a.relayMatterNodeID > 0 {
		ctrl, err := NewMatterController(a.matterChipToolPath, a.matterStorageDir, a.matterTimedTimeout, a.logger)
		if err != nil {
			return fmt.Errorf("matter controller init: %w", err)
		}
		mr, err := NewMatterRelay(ctrl, a.relayMatterNodeID, a.matterEndpoint, a.logger)
		if err != nil {
			return fmt.Errorf("matter relay init: %w", err)
		}
		// Auto-commission if no existing fabric
		if !mr.HasFabric() {
			if a.matterSetupCode == "" {
				return fmt.Errorf("matter: no fabric data and no -matter-setup-code provided")
			}
			if err := mr.Commission(a.matterSetupCode); err != nil {
				return fmt.Errorf("matter commission failed: %w", err)
			}
		}
		a.relay = mr
	} else {
		a.relay = &DryRunRelay{logger: a.logger}
	}

	// Start Matter lock state subscription if applicable
	if mr, ok := a.relay.(*MatterRelay); ok {
		go mr.SubscribeLockState(context.Background(), func(state DoorLockState) {
			a.mu.Lock()
			a.eventQueue = append(a.eventQueue, AccessEvent{
				GatewayID:  a.gatewayID,
				EventType:  "lock_state_changed",
				Result:     state.String(),
				OccurredAt: time.Now().UTC().Format(time.RFC3339),
			})
			a.mu.Unlock()
		})
	}

	// Initialize mTLS client certificate
	if a.mtlsCertDir != "" {
		a.mtls = NewDeviceMTLS(a.mtlsCertDir)
		if a.mtls.Load() {
			cn, notAfter, _ := a.mtls.CertInfo()
			a.logger.Info("mTLS: loaded client certificate",
				"cn", cn,
				"expires", notAfter.Format(time.RFC3339),
				"remaining", time.Until(notAfter).Round(time.Minute),
			)
		} else {
			a.logger.Info("mTLS: no valid client certificate, will obtain during registration")
		}
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

	// Start persistent WebSocket connection.
	// Auto-derive WS URL from API URL if not explicitly set.
	if a.wsURL == "" && a.apiURL != "" {
		a.wsURL = deriveWSURL(a.apiURL)
	}
	if a.wsURL != "" {
		a.wsClient = NewWSClient(a.logger, a, a.wsURL)
		a.wsClient.Start()
		a.logger.Info("ws: client started", "url", a.wsURL)
	}

	return nil
}

// registerDevice calls the bootstrap register endpoint and stores the returned device token.
// If mTLS is configured, it includes a CSR in the request and stores the signed certificate.
func (a *Agent) registerDevice() error {
	regPayload := map[string]any{
		"serial_number":   a.gatewayID,
		"tenant_id":       a.tenantID,
		"building_id":     "",
		"device_capacity": 4,
	}

	// Generate CSR for mTLS if configured and no valid cert exists
	if a.mtls != nil && !a.mtls.HasValidCert() {
		csrPEM, err := a.mtls.GenerateCSR(a.gatewayID, a.tenantID)
		if err != nil {
			a.logger.Warn("mTLS: CSR generation failed, registering without mTLS", "error", err)
		} else {
			regPayload["csr_pem"] = string(csrPEM)
			a.logger.Info("mTLS: CSR generated, including in registration")
		}
	}

	body, _ := json.Marshal(regPayload)

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
		CertPEM     string `json:"cert_pem,omitempty"`    // signed client certificate (if CSR was provided)
		CACertPEM   string `json:"ca_cert_pem,omitempty"` // CA certificate for verification
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("register decode: %w", err)
	}

	if result.DeviceToken == "" {
		return fmt.Errorf("register returned empty device token")
	}

	// Store mTLS certificate if returned
	if a.mtls != nil && result.CertPEM != "" {
		if err := a.mtls.StoreCert([]byte(result.CertPEM), []byte(result.CACertPEM)); err != nil {
			a.logger.Warn("mTLS: failed to store certificate", "error", err)
		} else {
			a.rebuildHTTPClient()
			cn, notAfter, _ := a.mtls.CertInfo()
			a.logger.Info("mTLS: client certificate obtained",
				"cn", cn,
				"expires", notAfter.Format(time.RFC3339),
			)
		}
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
	if a.wsClient != nil {
		a.wsClient.Stop()
	}
	if a.relay != nil {
		a.relay.Close()
	}
}

// --- Config Pull ---

func (a *Agent) pullConfig() error {
	body, _ := json.Marshal(map[string]string{
		"gateway_id":          a.gatewayID,
		"tenant_id":           a.tenantID,
		"current_version":     "",
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
		PendingOTATasks []otaTask `json:"pending_ota_tasks"`
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
	a.confirmPendingOTA()                   // finalize a prior self-update on a healthy pull
	a.maybeApplyOTA(result.PendingOTATasks) // may os.Exit to restart into new binary
	return nil
}

func (a *Agent) configPollLoop() {
	ticker := time.NewTicker(a.configPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// When WebSocket is connected, configs are pushed in real-time.
			// Reduce poll to 10x interval as a safety net (fallback only).
			if a.wsClient != nil && a.wsClient.IsConnected() {
				ticker.Reset(a.configPollInterval * 10)
			} else {
				ticker.Reset(a.configPollInterval)
			}
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
	// Check mTLS cert renewal (renew when < 24h remaining, cert lifetime = 72h)
	if a.mtls != nil && a.mtls.NeedsRenewal(24*time.Hour) {
		a.renewMTLSCert()
	}

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

// renewMTLSCert requests a new client certificate from the Cloud CA.
// POST /api/v1/gateway/cert/renew with a fresh CSR.
func (a *Agent) renewMTLSCert() {
	csrPEM, err := a.mtls.GenerateCSR(a.gatewayID, a.tenantID)
	if err != nil {
		a.logger.Warn("mTLS: CSR generation for renewal failed", "error", err)
		return
	}

	body, _ := json.Marshal(map[string]string{
		"gateway_id": a.gatewayID,
		"tenant_id":  a.tenantID,
		"csr_pem":    string(csrPEM),
	})
	resp, err := a.apiRequest("POST", "/api/v1/gateway/cert/renew", body)
	if err != nil {
		a.logger.Warn("mTLS: cert renewal request failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logger.Warn("mTLS: cert renewal rejected", "status", resp.StatusCode)
		return
	}

	var result struct {
		CertPEM   string `json:"cert_pem"`
		CACertPEM string `json:"ca_cert_pem"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		a.logger.Warn("mTLS: cert renewal decode failed", "error", err)
		return
	}

	if err := a.mtls.StoreCert([]byte(result.CertPEM), []byte(result.CACertPEM)); err != nil {
		a.logger.Warn("mTLS: cert renewal store failed", "error", err)
		return
	}
	a.rebuildHTTPClient()

	cn, notAfter, _ := a.mtls.CertInfo()
	a.logger.Info("mTLS: certificate renewed",
		"cn", cn,
		"expires", notAfter.Format(time.RFC3339),
	)
}

// --- Event Push (Offline-Resilient Queue) ---

const (
	eventQueueMaxSize = 10000           // max events retained in memory during offline
	eventPushInterval = 5 * time.Second // base push interval
	eventBackoffMax   = 5 * time.Minute // max backoff between retries
	eventBatchMaxSize = 200             // max events per HTTP batch
)

func (a *Agent) queueEvent(evt AccessEvent) {
	a.mu.Lock()
	if len(a.eventQueue) >= eventQueueMaxSize {
		// Drop oldest events to make room (ring-buffer behavior)
		drop := len(a.eventQueue) - eventQueueMaxSize + 1
		a.eventQueue = a.eventQueue[drop:]
		a.eventQueueDropped += int64(drop)
	}
	a.eventQueue = append(a.eventQueue, evt)
	a.mu.Unlock()
}

func (a *Agent) eventPushLoop() {
	ticker := time.NewTicker(eventPushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.pushEvents()
		case <-a.stopCh:
			a.pushEvents() // best-effort flush on shutdown
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

	// Respect backoff
	if a.eventPushBackoff > 0 {
		a.eventPushBackoff -= eventPushInterval
		if a.eventPushBackoff > 0 {
			a.mu.Unlock()
			return
		}
		a.eventPushBackoff = 0
	}

	// Take a batch (up to max batch size)
	batchSize := len(a.eventQueue)
	if batchSize > eventBatchMaxSize {
		batchSize = eventBatchMaxSize
	}
	batch := make([]AccessEvent, batchSize)
	copy(batch, a.eventQueue[:batchSize])
	a.eventQueue = a.eventQueue[batchSize:]
	remaining := len(a.eventQueue)
	a.mu.Unlock()

	// Try WebSocket first (lower latency, persistent connection)
	if a.wsClient != nil && a.wsClient.IsConnected() {
		if a.wsClient.SendEvents(batch) {
			// Success via WebSocket — reset offline state
			a.mu.Lock()
			wasOffline := !a.offlineSince.IsZero()
			offlineDuration := time.Duration(0)
			if wasOffline {
				offlineDuration = time.Since(a.offlineSince)
				a.offlineSince = time.Time{}
			}
			a.eventPushBackoff = 0
			dropped := a.eventQueueDropped
			a.eventQueueDropped = 0
			a.mu.Unlock()

			if wasOffline {
				a.logger.Info("connectivity restored via ws, events synced",
					"batch", len(batch),
					"remaining", remaining,
					"offline_duration", offlineDuration.Round(time.Second),
					"dropped_during_offline", dropped,
				)
			} else {
				a.logger.Debug("events pushed via ws", "count", len(batch))
			}
			return
		}
		// WS send failed, fall through to HTTP
		a.logger.Debug("ws event send failed, falling back to HTTP")
	}

	// Fallback: HTTP POST
	body, _ := json.Marshal(map[string]any{
		"gateway_id": a.gatewayID,
		"tenant_id":  a.tenantID,
		"events":     batch,
	})
	resp, err := a.apiRequest("POST", "/api/v1/gateway/events/batch", body)
	if err != nil {
		// Push failed — requeue and apply exponential backoff
		a.mu.Lock()
		a.eventQueue = append(batch, a.eventQueue...)
		if a.offlineSince.IsZero() {
			a.offlineSince = time.Now()
		}
		// Exponential backoff: 10s → 20s → 40s → ... → 5min cap
		if a.eventPushBackoff == 0 {
			a.eventPushBackoff = 10 * time.Second
		} else {
			a.eventPushBackoff *= 2
			if a.eventPushBackoff > eventBackoffMax {
				a.eventPushBackoff = eventBackoffMax
			}
		}
		queueLen := len(a.eventQueue)
		a.mu.Unlock()
		a.logger.Warn("event push failed, backoff",
			"error", err,
			"queued", queueLen,
			"backoff", a.eventPushBackoff,
			"offline_since", a.offlineSince.Format(time.RFC3339),
		)
		return
	}
	resp.Body.Close()

	// Success — reset offline state and backoff
	a.mu.Lock()
	wasOffline := !a.offlineSince.IsZero()
	offlineDuration := time.Duration(0)
	if wasOffline {
		offlineDuration = time.Since(a.offlineSince)
		a.offlineSince = time.Time{}
	}
	a.eventPushBackoff = 0
	dropped := a.eventQueueDropped
	a.eventQueueDropped = 0
	a.mu.Unlock()

	if wasOffline {
		a.logger.Info("connectivity restored, events synced",
			"batch", len(batch),
			"remaining", remaining,
			"offline_duration", offlineDuration.Round(time.Second),
			"dropped_during_offline", dropped,
		)
	} else {
		a.logger.Info("events pushed", "count", len(batch))
	}
}

// --- Local Access Decision ---
//
// Security invariant — MaxOfflineDuration (rulesCacheTTL):
//
//   When the Gateway loses connectivity, cached credentials remain usable for at
//   most rulesCacheTTL (default 72h). After that, deny_all kicks in. This bounds
//   the worst-case revocation delay: a user revoked while the Gateway is offline
//   can still open doors for at most this window.
//
//   Corollary: any per-credential valid_until MUST be ≤ rulesCacheTTL. Setting
//   valid_until=30d with rulesCacheTTL=72h would create a false sense of security
//   — the cache TTL is the binding constraint, not individual credential expiry.
//
//   When online, revocations propagate via WebSocket config_push.
//   The 72h window is the offline worst case, not the normal case.

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

	now := time.Now().UTC()
	for _, rule := range a.accessRules {
		if strings.ToLower(rule.CredentialType) != normalizedType {
			continue
		}
		if strings.ToUpper(rule.CredentialData) != normalizedData {
			continue
		}
		// Per-credential expiry check
		if rule.ValidUntil != "" {
			if expiry, err := time.Parse(time.RFC3339, rule.ValidUntil); err == nil && now.After(expiry) {
				return "deny", rule.UserID, rule.UserEmail
			}
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
// nonce cache -> cache freshness -> gateway_id -> credential lookup -> ECDSA verify.
// transport is "BLE" or "NFC_HCE" (use TransportTagBLE / TransportTagNFCHCE).
func (a *Agent) VerifyAuthResponseV2(resp *BLEAuthResponse, challenge []byte, transport string) BLEAuthResult {
	if len(challenge) < 32 {
		return BLEAuthResult{Code: BLEResultDenied, Reason: "challenge_too_short"}
	}
	nonce := challenge[:32]

	// 0. Cache freshness check — deny all if offline beyond MaxOfflineDuration
	a.mu.RLock()
	cacheExpired := a.rulesCacheTTL > 0 && !a.rulesUpdatedAt.IsZero() &&
		time.Since(a.rulesUpdatedAt) > a.rulesCacheTTL
	a.mu.RUnlock()
	if cacheExpired {
		return BLEAuthResult{Code: BLEResultDenied, Reason: "cache_expired_offline_too_long"}
	}

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
	// Skip expired credentials (ValidUntil check).
	type candidateKey struct {
		publicKeyPEM string
		userEmail    string
	}
	now := time.Now().UTC()
	a.mu.RLock()
	var candidates []candidateKey
	for _, rule := range a.accessRules {
		if rule.CredentialType == "ble_signature" && rule.UserID == resp.UserID {
			// Per-credential expiry: skip if valid_until has passed
			if rule.ValidUntil != "" {
				if expiry, err := time.Parse(time.RFC3339, rule.ValidUntil); err == nil && now.After(expiry) {
					continue
				}
			}
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
	a.mu.RLock()
	c := a.cachedHTTPClient
	a.mu.RUnlock()
	if c != nil {
		return c
	}
	return a.rebuildHTTPClient()
}

func (a *Agent) rebuildHTTPClient() *http.Client {
	tlsCfg := a.buildTLSConfig()
	var c *http.Client
	if tlsCfg == nil {
		c = &http.Client{Timeout: 30 * time.Second}
	} else {
		c = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: tlsCfg,
			},
		}
	}
	a.mu.Lock()
	a.cachedHTTPClient = c
	a.mu.Unlock()
	return c
}

// pinnedTLSConfig returns a TLS config for WebSocket connections.
// Includes mTLS client cert + optional certificate pinning.
func (a *Agent) pinnedTLSConfig() *tls.Config {
	return a.buildTLSConfig()
}

// buildTLSConfig creates a unified TLS config with:
//   - mTLS client certificate (if available) for mutual authentication
//   - Certificate pinning (if configured) for server verification
//   - TLS 1.3 preferred (1.2 minimum) for ECDHE forward secrecy
//
// Returns nil if neither mTLS nor pinning is configured.
func (a *Agent) buildTLSConfig() *tls.Config {
	hasMTLS := a.mtls != nil && a.mtls.HasValidCert()
	hasPin := a.tlsPinSHA256 != ""

	if !hasMTLS && !hasPin {
		return nil
	}

	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// mTLS: present client certificate for mutual authentication
	if hasMTLS {
		mtlsCfg := a.mtls.TLSConfig(a.tlsPinSHA256)
		cfg.Certificates = mtlsCfg.Certificates
		cfg.MinVersion = tls.VersionTLS13 // enforce TLS 1.3 when using mTLS
		if mtlsCfg.RootCAs != nil {
			cfg.RootCAs = mtlsCfg.RootCAs
		}
	}

	// Certificate pinning: verify server's SPKI hash
	if hasPin {
		pinBytes, err := hex.DecodeString(a.tlsPinSHA256)
		if err != nil || len(pinBytes) != sha256.Size {
			a.logger.Warn("invalid tls-pin-sha256, skipping pin verification", "error", err)
		} else {
			cfg.VerifyConnection = func(cs tls.ConnectionState) error {
				for _, cert := range cs.PeerCertificates {
					spkiHash := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
					if hex.EncodeToString(spkiHash[:]) == a.tlsPinSHA256 {
						return nil
					}
				}
				return fmt.Errorf("TLS certificate pinning failed: no certificate matched pin %s", a.tlsPinSHA256)
			}
		}
	}

	return cfg
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
	if a.gatewayID != "" {
		req.Header.Set("X-Gateway-ID", a.gatewayID)
	}
	if a.tenantID != "" {
		req.Header.Set("X-Tenant-ID", a.tenantID)
	}
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

// deriveWSURL converts an HTTP API base URL to a WebSocket URL.
// e.g. "https://api.example.com" → "wss://api.example.com/api/v1/gateway/ws"
//
//	"http://localhost:8081"    → "ws://localhost:8081/api/v1/gateway/ws"
func deriveWSURL(apiURL string) string {
	u := strings.TrimRight(apiURL, "/")
	if strings.HasPrefix(u, "https://") {
		return "wss://" + strings.TrimPrefix(u, "https://") + "/api/v1/gateway/ws"
	}
	if strings.HasPrefix(u, "http://") {
		return "ws://" + strings.TrimPrefix(u, "http://") + "/api/v1/gateway/ws"
	}
	return "wss://" + u + "/api/v1/gateway/ws"
}
