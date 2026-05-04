package southbound

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ZKTecoDoorProvider implements DoorProvider for ZKTeco access control devices.
//
// ZKTeco devices use a Push Protocol where the device connects to the platform,
// and a REST API for user/card management.
//
// Supported devices: ZK InBio series (controllers), SpeedFace (terminals),
// ProFace (face recognition), F18/F22 (fingerprint readers).
//
// Key API endpoints (device-hosted):
//   - POST /cgi-bin/accessControl.cgi?action=openDoor&channel={n}   — remote unlock
//   - POST /cgi-bin/recordUpdater.cgi?action=insert&name=user       — add user
//   - POST /cgi-bin/recordUpdater.cgi?action=remove&name=user       — delete user
//   - POST /cgi-bin/recordFinder.cgi?action=find&name=AccessControlCard — query cards
type ZKTecoDoorProvider struct {
	client *http.Client
}

func NewZKTecoDoorProvider() *ZKTecoDoorProvider {
	return &ZKTecoDoorProvider{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *ZKTecoDoorProvider) ProviderName() string { return "zkteco" }

// Unlock sends a remote open command.
// POST /cgi-bin/accessControl.cgi?action=openDoor&channel={doorIndex}&UserID=&Type=Remote
func (p *ZKTecoDoorProvider) Unlock(ctx context.Context, cfg DeviceConfig, doorIndex int) error {
	if doorIndex <= 0 {
		doorIndex = 1
	}
	url := fmt.Sprintf("%s/cgi-bin/accessControl.cgi?action=openDoor&channel=%d&UserID=&Type=Remote",
		buildBaseURL(cfg), doorIndex)

	resp, err := p.doBasicRequest(ctx, cfg, http.MethodPost, url, "")
	if err != nil {
		return fmt.Errorf("zkteco unlock: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "OK") {
		return fmt.Errorf("zkteco unlock: status %d, body=%s", resp.StatusCode, string(body))
	}
	return nil
}

// Lock is not directly supported by most ZKTeco devices (they auto-lock after timeout).
func (p *ZKTecoDoorProvider) Lock(_ context.Context, _ DeviceConfig, _ int) error {
	return fmt.Errorf("zkteco: explicit lock not supported (device auto-locks after configured timeout)")
}

// SyncUsers pushes users to the ZKTeco device.
// POST /cgi-bin/recordUpdater.cgi?action=insert&name=user
func (p *ZKTecoDoorProvider) SyncUsers(ctx context.Context, cfg DeviceConfig, users []DeviceUser) error {
	for _, user := range users {
		if err := p.addUser(ctx, cfg, user); err != nil {
			return fmt.Errorf("zkteco sync user %s: %w", user.UserID, err)
		}
		if user.CardNumber != "" {
			if err := p.addCard(ctx, cfg, user.UserID, user.CardNumber); err != nil {
				return fmt.Errorf("zkteco sync card for %s: %w", user.UserID, err)
			}
		}
	}
	return nil
}

func (p *ZKTecoDoorProvider) addUser(ctx context.Context, cfg DeviceConfig, user DeviceUser) error {
	reqURL := fmt.Sprintf("%s/cgi-bin/recordUpdater.cgi?action=insert&name=user", buildBaseURL(cfg))
	body := url.Values{
		"UserID":    {user.UserID},
		"UserName":  {user.Name},
		"Privilege": {"0"},
		"Enable":    {"true"},
	}.Encode()

	resp, err := p.doBasicRequest(ctx, cfg, http.MethodPost, reqURL, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && !strings.Contains(string(respBody), "OK") {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (p *ZKTecoDoorProvider) addCard(ctx context.Context, cfg DeviceConfig, userID, cardNumber string) error {
	url := fmt.Sprintf("%s/cgi-bin/recordUpdater.cgi?action=insert&name=card", buildBaseURL(cfg))
	body := fmt.Sprintf("UserID=%s&CardNo=%s&CardStatus=0&CardType=0",
		userID, cardNumber)

	resp, err := p.doBasicRequest(ctx, cfg, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// DeleteUser removes a user from the ZKTeco device.
func (p *ZKTecoDoorProvider) DeleteUser(ctx context.Context, cfg DeviceConfig, userID string) error {
	url := fmt.Sprintf("%s/cgi-bin/recordUpdater.cgi?action=remove&name=user", buildBaseURL(cfg))
	body := fmt.Sprintf("UserID=%s", userID)

	resp, err := p.doBasicRequest(ctx, cfg, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("zkteco delete user: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// GetDoorStatus queries the current door state.
func (p *ZKTecoDoorProvider) GetDoorStatus(ctx context.Context, cfg DeviceConfig, doorIndex int) (*DoorStatus, error) {
	if doorIndex <= 0 {
		doorIndex = 1
	}
	url := fmt.Sprintf("%s/cgi-bin/accessControl.cgi?action=getDoorStatus&channel=%d",
		buildBaseURL(cfg), doorIndex)

	resp, err := p.doBasicRequest(ctx, cfg, http.MethodGet, url, "")
	if err != nil {
		return nil, fmt.Errorf("zkteco door status: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	state := "unknown"
	lower := strings.ToLower(string(body))
	if strings.Contains(lower, "open") {
		state = "open"
	} else if strings.Contains(lower, "close") || strings.Contains(lower, "lock") {
		state = "closed"
	}

	return &DoorStatus{DoorIndex: doorIndex, State: state, Locked: state == "closed"}, nil
}

// SubscribeEvents uses ZKTeco Push Protocol.
// The device pushes events to a configured server endpoint.
// This method sets up a local HTTP listener for receiving push events.
func (p *ZKTecoDoorProvider) SubscribeEvents(ctx context.Context, cfg DeviceConfig, handler func(DoorEvent)) error {
	// ZKTeco Push Protocol: device POSTs events to a configured URL.
	// In production, the cloud platform registers as the push target.
	// For now, return not-implemented since it requires the device to be configured
	// to push to our server (done during device setup, not runtime).
	return fmt.Errorf("zkteco: event subscription requires device-side push configuration (see docs)")
}

// TestConnection verifies the ZKTeco device is reachable.
func (p *ZKTecoDoorProvider) TestConnection(ctx context.Context, cfg DeviceConfig) error {
	// Try ZKTeco-specific endpoint
	url := fmt.Sprintf("%s/cgi-bin/magicBox.cgi?action=getDeviceType", buildBaseURL(cfg))
	resp, err := p.doBasicRequest(ctx, cfg, http.MethodGet, url, "")
	if err != nil {
		return fmt.Errorf("zkteco test: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("zkteco: invalid credentials")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("zkteco: status %d", resp.StatusCode)
	}
	return nil
}

// doBasicRequest performs an HTTP request with Basic Auth (most ZKTeco devices).
func (p *ZKTecoDoorProvider) doBasicRequest(ctx context.Context, cfg DeviceConfig, method, url, body string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return p.client.Do(req)
}

// --- ZKTeco Push Event Handler ---

// ZKPushEvent represents an event received from a ZKTeco device via Push Protocol.
type ZKPushEvent struct {
	SerialNumber string `json:"sn"`
	EventType    int    `json:"type"`
	Time         string `json:"time"`
	Pin          string `json:"pin"` // user ID
	CardNo       string `json:"cardno"`
	Door         int    `json:"door"`
	Status       int    `json:"status"` // 0=denied, 1=granted
}

// ParseZKPushEvent parses a ZKTeco push event from the HTTP request body.
// Used by the cloud endpoint that receives device push notifications.
func ParseZKPushEvent(body []byte) (*DoorEvent, error) {
	var zk ZKPushEvent
	if err := json.Unmarshal(body, &zk); err != nil {
		return nil, fmt.Errorf("parse zkteco push event: %w", err)
	}

	eventType := "access_denied"
	if zk.Status == 1 {
		eventType = "access_granted"
	}

	return &DoorEvent{
		EventType:  eventType,
		UserID:     zk.Pin,
		CardNumber: zk.CardNo,
		DoorIndex:  zk.Door,
		Timestamp:  zk.Time,
		Extra: map[string]string{
			"serial_number": zk.SerialNumber,
		},
	}, nil
}
