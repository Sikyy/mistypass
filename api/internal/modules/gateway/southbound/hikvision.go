package southbound

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HikvisionDoorProvider implements DoorProvider for Hikvision access control devices
// using the ISAPI HTTP REST protocol with Digest Authentication.
//
// Supported devices: DS-K1T series (face terminals), DS-K2602 (controllers),
// DS-K1A802 (fingerprint), and other ISAPI-compatible access control devices.
type HikvisionDoorProvider struct {
	client *http.Client
}

func NewHikvisionDoorProvider() *HikvisionDoorProvider {
	return &HikvisionDoorProvider{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *HikvisionDoorProvider) ProviderName() string { return "hikvision" }

// Unlock sends a remote open command.
// PUT /ISAPI/AccessControl/RemoteControl/door/{doorNo}
func (p *HikvisionDoorProvider) Unlock(ctx context.Context, cfg DeviceConfig, doorIndex int) error {
	if doorIndex <= 0 {
		doorIndex = 1
	}
	url := fmt.Sprintf("%s/ISAPI/AccessControl/RemoteControl/door/%d", buildBaseURL(cfg), doorIndex)
	body := `<RemoteControlDoor><cmd>open</cmd></RemoteControlDoor>`
	return p.doCommand(ctx, cfg, http.MethodPut, url, body, "unlock")
}

// Lock sends a remote lock command.
func (p *HikvisionDoorProvider) Lock(ctx context.Context, cfg DeviceConfig, doorIndex int) error {
	if doorIndex <= 0 {
		doorIndex = 1
	}
	url := fmt.Sprintf("%s/ISAPI/AccessControl/RemoteControl/door/%d", buildBaseURL(cfg), doorIndex)
	body := `<RemoteControlDoor><cmd>close</cmd></RemoteControlDoor>`
	return p.doCommand(ctx, cfg, http.MethodPut, url, body, "lock")
}

// SyncUsers pushes users + cards to the device for offline verification.
// POST /ISAPI/AccessControl/UserInfo/Record?format=json
func (p *HikvisionDoorProvider) SyncUsers(ctx context.Context, cfg DeviceConfig, users []DeviceUser) error {
	base := buildBaseURL(cfg)
	userURL := fmt.Sprintf("%s/ISAPI/AccessControl/UserInfo/Record?format=json", base)
	cardURL := fmt.Sprintf("%s/ISAPI/AccessControl/CardInfo/Record?format=json", base)

	for _, user := range users {
		userPayload := map[string]any{
			"UserInfo": map[string]any{
				"employeeNo": user.UserID,
				"name":       user.Name,
				"Valid": map[string]any{
					"enable":    true,
					"beginTime": nvl(user.StartTime, "2020-01-01T00:00:00"),
					"endTime":   nvl(user.EndTime, "2037-12-31T23:59:59"),
				},
			},
		}
		userBody, _ := json.Marshal(userPayload)
		if err := p.doCommand(ctx, cfg, http.MethodPost, userURL, string(userBody), "sync user"); err != nil {
			return fmt.Errorf("user %s: %w", user.UserID, err)
		}

		if user.CardNumber != "" {
			cardPayload := map[string]any{
				"CardInfo": map[string]any{
					"employeeNo": user.UserID,
					"cardNo":     user.CardNumber,
					"cardType":   "normalCard",
				},
			}
			cardBody, _ := json.Marshal(cardPayload)
			// Ignore 409 (card already exists)
			_ = p.doCommand(ctx, cfg, http.MethodPost, cardURL, string(cardBody), "sync card")
		}
	}
	return nil
}

// DeleteUser removes a user from the device.
// PUT /ISAPI/AccessControl/UserInfo/Delete?format=json
func (p *HikvisionDoorProvider) DeleteUser(ctx context.Context, cfg DeviceConfig, userID string) error {
	url := fmt.Sprintf("%s/ISAPI/AccessControl/UserInfo/Delete?format=json", buildBaseURL(cfg))
	payload := map[string]any{
		"UserInfoDelCond": map[string]any{
			"EmployeeNoList": []map[string]string{{"employeeNo": userID}},
		},
	}
	body, _ := json.Marshal(payload)
	return p.doCommand(ctx, cfg, http.MethodPut, url, string(body), "delete user")
}

// GetDoorStatus retrieves the current door state.
func (p *HikvisionDoorProvider) GetDoorStatus(ctx context.Context, cfg DeviceConfig, doorIndex int) (*DoorStatus, error) {
	if doorIndex <= 0 {
		doorIndex = 1
	}
	url := fmt.Sprintf("%s/ISAPI/AccessControl/Door/param/%d", buildBaseURL(cfg), doorIndex)

	resp, err := p.doDigestRequest(ctx, cfg, http.MethodGet, url, "")
	if err != nil {
		return nil, fmt.Errorf("hikvision door status: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	state := "unknown"
	lower := strings.ToLower(string(bodyBytes))
	if strings.Contains(lower, "open") {
		state = "open"
	} else if strings.Contains(lower, "close") {
		state = "closed"
	}

	return &DoorStatus{DoorIndex: doorIndex, State: state, Locked: state == "closed"}, nil
}

// SubscribeEvents opens a long-polling stream for real-time access events.
// GET /ISAPI/Event/notification/alertStream
func (p *HikvisionDoorProvider) SubscribeEvents(ctx context.Context, cfg DeviceConfig, handler func(DoorEvent)) error {
	url := fmt.Sprintf("%s/ISAPI/Event/notification/alertStream", buildBaseURL(cfg))
	streamClient := &http.Client{Timeout: 0}

	resp, err := doDigestAuth(ctx, streamClient, http.MethodGet, url, cfg.Username, cfg.Password, "")
	if err != nil {
		return fmt.Errorf("hikvision events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hikvision events: status %d", resp.StatusCode)
	}

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if event, ok := parseHikEvent(string(buf[:n])); ok {
				handler(event)
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// TestConnection verifies reachability and credentials.
func (p *HikvisionDoorProvider) TestConnection(ctx context.Context, cfg DeviceConfig) error {
	url := fmt.Sprintf("%s/ISAPI/System/deviceInfo", buildBaseURL(cfg))
	resp, err := p.doDigestRequest(ctx, cfg, http.MethodGet, url, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("hikvision: invalid credentials")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hikvision: status %d", resp.StatusCode)
	}
	return nil
}

// --- internal helpers ---

func (p *HikvisionDoorProvider) doCommand(ctx context.Context, cfg DeviceConfig, method, url, body, op string) error {
	resp, err := p.doDigestRequest(ctx, cfg, method, url, body)
	if err != nil {
		return fmt.Errorf("hikvision %s: %w", op, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hikvision %s: status %d body=%s", op, resp.StatusCode, string(respBody))
	}
	return nil
}

func (p *HikvisionDoorProvider) doDigestRequest(ctx context.Context, cfg DeviceConfig, method, url, body string) (*http.Response, error) {
	return doDigestAuth(ctx, p.client, method, url, cfg.Username, cfg.Password, body)
}

func parseHikEvent(chunk string) (DoorEvent, bool) {
	if !strings.Contains(chunk, "AccessControllerEvent") && !strings.Contains(chunk, "EventNotificationAlert") {
		return DoorEvent{}, false
	}
	event := DoorEvent{Timestamp: time.Now().UTC().Format(time.RFC3339), DoorIndex: 1}
	if strings.Contains(chunk, "verified") || strings.Contains(chunk, "Access") {
		event.EventType = "access_granted"
	} else if strings.Contains(chunk, "denied") {
		event.EventType = "access_denied"
	} else {
		event.EventType = "door_opened"
	}
	return event, true
}

// --- Digest Auth implementation ---

func doDigestAuth(ctx context.Context, client *http.Client, method, url, username, password, body string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	setContentType(req, body)

	// First request to get WWW-Authenticate challenge
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	resp.Body.Close()

	authHeader := resp.Header.Get("WWW-Authenticate")
	if authHeader == "" {
		return nil, fmt.Errorf("no WWW-Authenticate header")
	}

	// Parse and compute digest
	realm := extractDigestParam(authHeader, "realm")
	nonce := extractDigestParam(authHeader, "nonce")
	qop := extractDigestParam(authHeader, "qop")

	uriPath := extractPath(url)
	ha1 := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", username, realm, password))))
	ha2 := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s", method, uriPath))))

	nc := "00000001"
	cnonce := fmt.Sprintf("%08x", time.Now().UnixNano()&0xFFFFFFFF)
	var response string
	if qop != "" {
		response = fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s:%s:%s:%s", ha1, nonce, nc, cnonce, qop, ha2))))
	} else {
		response = fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2))))
	}

	authValue := fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
		username, realm, nonce, uriPath, response)
	if qop != "" {
		authValue += fmt.Sprintf(`, qop=%s, nc=%s, cnonce="%s"`, qop, nc, cnonce)
	}

	// Retry with auth
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req2, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	setContentType(req2, body)
	req2.Header.Set("Authorization", authValue)
	return client.Do(req2)
}

func setContentType(req *http.Request, body string) {
	if body == "" {
		return
	}
	if strings.HasPrefix(strings.TrimSpace(body), "<") {
		req.Header.Set("Content-Type", "application/xml")
	} else {
		req.Header.Set("Content-Type", "application/json")
	}
}

func extractDigestParam(header, param string) string {
	lower := strings.ToLower(header)
	key := strings.ToLower(param) + "="
	idx := strings.Index(lower, key)
	if idx < 0 {
		return ""
	}
	rest := header[idx+len(key):]
	rest = strings.Trim(rest, `"`)
	if end := strings.IndexAny(rest, `",`); end >= 0 {
		return strings.Trim(rest[:end], `"`)
	}
	return strings.Trim(rest, `"`)
}

func extractPath(rawURL string) string {
	if idx := strings.Index(rawURL, "://"); idx >= 0 {
		rest := rawURL[idx+3:]
		if pathIdx := strings.Index(rest, "/"); pathIdx >= 0 {
			return rest[pathIdx:]
		}
	}
	return rawURL
}

func buildBaseURL(cfg DeviceConfig) string {
	scheme := "http"
	if cfg.UseTLS {
		scheme = "https"
	}
	port := cfg.Port
	if port == 0 {
		port = 80
	}
	return fmt.Sprintf("%s://%s:%d", scheme, cfg.Host, port)
}

func nvl(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// Ensure xml import is used (for future XML parsing)
var _ = xml.Header
