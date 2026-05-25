package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
)

const (
	FCMProviderName            = "fcm"
	FCMEndpointDefaultTemplate = "https://fcm.googleapis.com/v1/projects/{project_id}/messages:send"
	FCMMessagingScope          = "https://www.googleapis.com/auth/firebase.messaging"
)

type Provider interface {
	Provider() string
	Send(ctx context.Context, msg Message) (Receipt, error)
}

type Message struct {
	Token            string
	Title            string
	Body             string
	Type             string
	Data             map[string]string
	AndroidChannelID string
	TTL              time.Duration
}

type Receipt struct {
	Provider           string `json:"provider"`
	ProviderMessageID  string `json:"provider_message_id,omitempty"`
	ProviderStatus     string `json:"provider_status"`
	ProviderStatusCode int    `json:"provider_status_code,omitempty"`
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e HTTPError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("push provider http status %d", e.StatusCode)
	}
	return fmt.Sprintf("push provider http status %d: %s", e.StatusCode, body)
}

func (e HTTPError) Retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= http.StatusInternalServerError
}

type FCMOptions struct {
	ProjectID          string
	Endpoint           string
	ServiceAccountFile string
	ServiceAccountJSON string
	Timeout            time.Duration
	Client             *http.Client
}

type FCMProvider struct {
	projectID string
	endpoint  string
	client    *http.Client
}

func NewFCMProvider(options FCMOptions) (*FCMProvider, error) {
	projectID := strings.TrimSpace(options.ProjectID)
	serviceAccountJSON, err := serviceAccountPayload(options.ServiceAccountFile, options.ServiceAccountJSON)
	if err != nil {
		return nil, err
	}
	if projectID == "" && len(serviceAccountJSON) > 0 {
		projectID = serviceAccountProjectID(serviceAccountJSON)
	}
	if projectID == "" {
		return nil, errors.New("fcm provider requires project id")
	}

	endpoint := resolveFCMEndpoint(options.Endpoint, projectID)
	timeout := options.Timeout
	if timeout < time.Second {
		timeout = 10 * time.Second
	}

	client := options.Client
	if client == nil {
		if len(serviceAccountJSON) == 0 {
			return nil, errors.New("fcm provider requires service account json")
		}
		jwtConfig, err := google.JWTConfigFromJSON(serviceAccountJSON, FCMMessagingScope)
		if err != nil {
			return nil, fmt.Errorf("fcm provider parse service account: %w", err)
		}
		client = jwtConfig.Client(context.Background())
		client.Timeout = timeout
	}

	return &FCMProvider{
		projectID: projectID,
		endpoint:  endpoint,
		client:    client,
	}, nil
}

func (p *FCMProvider) Provider() string {
	return FCMProviderName
}

func (p *FCMProvider) Send(ctx context.Context, msg Message) (Receipt, error) {
	token := strings.TrimSpace(msg.Token)
	if token == "" {
		return Receipt{}, errors.New("fcm message token is required")
	}

	payload := map[string]any{
		"message": fcmMessagePayload(msg, token),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Receipt{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return Receipt{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return Receipt{}, err
	}
	defer resp.Body.Close()

	readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Receipt{}, HTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(readBody)),
		}
	}

	var response struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(readBody, &response)
	return Receipt{
		Provider:           p.Provider(),
		ProviderMessageID:  strings.TrimSpace(response.Name),
		ProviderStatus:     "sent",
		ProviderStatusCode: resp.StatusCode,
	}, nil
}

func fcmMessagePayload(msg Message, token string) map[string]any {
	data := map[string]string{}
	for key, value := range msg.Data {
		nextKey := strings.TrimSpace(key)
		if nextKey == "" {
			continue
		}
		data[nextKey] = strings.TrimSpace(value)
	}
	if msg.Type != "" {
		data["type"] = strings.TrimSpace(msg.Type)
	}
	if msg.Title != "" {
		data["title"] = strings.TrimSpace(msg.Title)
	}
	if msg.Body != "" {
		data["body"] = strings.TrimSpace(msg.Body)
	}

	message := map[string]any{
		"token": token,
	}
	if len(data) > 0 {
		message["data"] = data
	}
	title := strings.TrimSpace(msg.Title)
	body := strings.TrimSpace(msg.Body)
	if title != "" || body != "" {
		message["notification"] = map[string]string{
			"title": title,
			"body":  body,
		}
	}

	android := map[string]any{}
	if msg.TTL > 0 {
		android["ttl"] = fmt.Sprintf("%ds", int(msg.TTL.Seconds()))
	}
	notification := map[string]string{}
	if channelID := strings.TrimSpace(msg.AndroidChannelID); channelID != "" {
		notification["channel_id"] = channelID
	}
	if len(notification) > 0 {
		android["notification"] = notification
	}
	if len(android) > 0 {
		message["android"] = android
	}
	return message
}

func serviceAccountPayload(filePath, raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		return []byte(raw), nil
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, nil
	}
	payload, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read fcm service account file: %w", err)
	}
	return payload, nil
}

func serviceAccountProjectID(payload []byte) string {
	var parsed struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.ProjectID)
}

func resolveFCMEndpoint(endpoint, projectID string) string {
	next := strings.TrimSpace(endpoint)
	if next == "" {
		next = FCMEndpointDefaultTemplate
	}
	return strings.ReplaceAll(next, "{project_id}", strings.TrimSpace(projectID))
}
