package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
	configPollInterval time.Duration
	heartbeatInterval  time.Duration
	unlockDuration     time.Duration
	relayGPIOPin       int
	relayRS485Device   string

	mu          sync.RWMutex
	accessRules []AccessRule
	ruleVersion string
	eventQueue  []AccessEvent
	stopCh      chan struct{}
	relay       RelayDriver
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

	// Initialize relay driver
	if a.relayGPIOPin >= 0 {
		driver, err := NewGPIORelay(a.relayGPIOPin, a.logger)
		if err != nil {
			return fmt.Errorf("gpio relay init: %w", err)
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
func (a *Agent) VerifyCredential(credentialType, credentialData, lockID string) (decision string, userID string, userEmail string) {
	a.mu.RLock()
	defer a.mu.RUnlock()

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
		logger.Warn("ACCESS DENIED")
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
	if a.bootstrapToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.bootstrapToken)
		req.Header.Set("X-Device-Token", a.bootstrapToken)
	}
	return http.DefaultClient.Do(req)
}
