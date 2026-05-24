package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const ResendEndpointDefault = "https://api.resend.com/emails"

type Provider interface {
	Provider() string
	Send(ctx context.Context, msg Message) (Receipt, error)
}

type Attachment struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

type Message struct {
	TenantID       string
	To             []string
	IdempotencyKey string
	Subject        string
	Text           string
	HTML           string
	Attachments    []Attachment
	Metadata       map[string]string
}

type Receipt struct {
	Provider               string
	ProviderDeliveryID     string
	ProviderDeliveryStatus string
}

type Confirmation struct {
	Confirmed bool
	Receipt
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e HTTPError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("email provider http status %d", e.StatusCode)
	}
	return fmt.Sprintf("email provider http status %d: %s", e.StatusCode, body)
}

func (e HTTPError) Retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= http.StatusInternalServerError
}

type ResendOptions struct {
	Endpoint string
	APIKey   string
	From     string
	Timeout  time.Duration
	Client   *http.Client
}

type ResendProvider struct {
	endpoint string
	apiKey   string
	from     string
	client   *http.Client
}

func NewResendProvider(options ResendOptions) (*ResendProvider, error) {
	endpoint := strings.TrimSpace(options.Endpoint)
	if endpoint == "" {
		endpoint = ResendEndpointDefault
	}
	apiKey := strings.TrimSpace(options.APIKey)
	from := strings.TrimSpace(options.From)
	if apiKey == "" || from == "" {
		return nil, errors.New("resend provider requires api key/from")
	}
	timeout := options.Timeout
	if timeout < time.Second {
		timeout = 5 * time.Second
	}
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &ResendProvider{
		endpoint: endpoint,
		apiKey:   apiKey,
		from:     from,
		client:   client,
	}, nil
}

func (p *ResendProvider) Provider() string {
	return "resend"
}

func (p *ResendProvider) Send(ctx context.Context, msg Message) (Receipt, error) {
	payload := map[string]any{
		"from":    p.from,
		"to":      append([]string(nil), msg.To...),
		"subject": strings.TrimSpace(msg.Subject),
	}
	if text := strings.TrimSpace(msg.Text); text != "" {
		payload["text"] = text
	}
	if html := strings.TrimSpace(msg.HTML); html != "" {
		payload["html"] = html
	}
	if len(msg.Attachments) > 0 {
		payload["attachments"] = append([]Attachment(nil), msg.Attachments...)
	}
	if len(msg.Metadata) > 0 {
		payload["metadata"] = cloneStringMap(msg.Metadata)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Receipt{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return Receipt{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey := strings.TrimSpace(msg.IdempotencyKey); idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return Receipt{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return Receipt{}, HTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(readBody)),
		}
	}

	readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	var response struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(readBody, &response)
	return Receipt{
		Provider:               p.Provider(),
		ProviderDeliveryID:     strings.TrimSpace(response.ID),
		ProviderDeliveryStatus: "accepted",
	}, nil
}

func (p *ResendProvider) Confirm(ctx context.Context, providerDeliveryID string) (Confirmation, error) {
	deliveryID := strings.TrimSpace(providerDeliveryID)
	if deliveryID == "" {
		return Confirmation{}, nil
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		strings.TrimRight(p.endpoint, "/")+"/"+url.PathEscape(deliveryID),
		nil,
	)
	if err != nil {
		return Confirmation{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return Confirmation{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Confirmation{
			Receipt: Receipt{
				Provider:           p.Provider(),
				ProviderDeliveryID: deliveryID,
			},
		}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return Confirmation{}, HTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(readBody)),
		}
	}

	readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var response struct {
		ID        string `json:"id"`
		LastEvent string `json:"last_event"`
	}
	_ = json.Unmarshal(readBody, &response)
	deliveryID = strings.TrimSpace(response.ID)
	if deliveryID == "" {
		deliveryID = providerDeliveryID
	}
	status := strings.TrimSpace(response.LastEvent)
	if status == "" {
		status = "accepted"
	}
	return Confirmation{
		Confirmed: isPositiveDeliveryStatus(status),
		Receipt: Receipt{
			Provider:               p.Provider(),
			ProviderDeliveryID:     deliveryID,
			ProviderDeliveryStatus: status,
		},
	}, nil
}

func isPositiveDeliveryStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "delivered", "sent", "success", "succeeded", "opened", "clicked":
		return true
	default:
		return false
	}
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		cloned[key] = value
	}
	return cloned
}
