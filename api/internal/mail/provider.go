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

const (
	ResendEndpointDefault          = "https://api.resend.com/emails"
	CloudflareEmailEndpointDefault = "https://api.cloudflare.com/client/v4/accounts/{account_id}/email/sending/send"
)

type Provider interface {
	Provider() string
	Send(ctx context.Context, msg Message) (Receipt, error)
}

type Attachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	Type        string `json:"type,omitempty"`
	Disposition string `json:"disposition,omitempty"`
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

type CloudflareOptions struct {
	Endpoint  string
	AccountID string
	APIToken  string
	From      string
	Timeout   time.Duration
	Client    *http.Client
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
		timeout = 15 * time.Second
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

type CloudflareProvider struct {
	endpoint string
	apiToken string
	from     string
	client   *http.Client
}

func NewCloudflareProvider(options CloudflareOptions) (*CloudflareProvider, error) {
	endpoint, err := resolveCloudflareEmailEndpoint(options.Endpoint, options.AccountID)
	if err != nil {
		return nil, err
	}
	apiToken := strings.TrimSpace(options.APIToken)
	from := strings.TrimSpace(options.From)
	if apiToken == "" || from == "" {
		return nil, errors.New("cloudflare provider requires api token/from")
	}
	timeout := options.Timeout
	if timeout < time.Second {
		timeout = 5 * time.Second
	}
	client := options.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	return &CloudflareProvider{
		endpoint: endpoint,
		apiToken: apiToken,
		from:     from,
		client:   client,
	}, nil
}

func (p *CloudflareProvider) Provider() string {
	return "cloudflare"
}

func (p *CloudflareProvider) Send(ctx context.Context, msg Message) (Receipt, error) {
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
		payload["attachments"] = cloudflareAttachments(msg.Attachments)
	}
	if headers := cloudflareHeaders(msg); len(headers) > 0 {
		payload["headers"] = headers
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Receipt{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return Receipt{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
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

	readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var response struct {
		Success *bool `json:"success"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
		Result struct {
			Delivered        []string `json:"delivered"`
			PermanentBounces []string `json:"permanent_bounces"`
			Queued           []string `json:"queued"`
		} `json:"result"`
	}
	_ = json.Unmarshal(readBody, &response)
	if response.Success != nil && !*response.Success {
		return Receipt{}, HTTPError{
			StatusCode: resp.StatusCode,
			Body:       cloudflareErrorBody(response.Errors, string(readBody)),
		}
	}
	return Receipt{
		Provider:               p.Provider(),
		ProviderDeliveryID:     strings.TrimSpace(msg.IdempotencyKey),
		ProviderDeliveryStatus: cloudflareDeliveryStatus(response.Result.Delivered, response.Result.Queued, response.Result.PermanentBounces),
	}, nil
}

func (p *CloudflareProvider) Confirm(ctx context.Context, providerDeliveryID string) (Confirmation, error) {
	deliveryID := strings.TrimSpace(providerDeliveryID)
	if deliveryID == "" {
		return Confirmation{}, nil
	}
	return Confirmation{
		Receipt: Receipt{
			Provider:               p.Provider(),
			ProviderDeliveryID:     deliveryID,
			ProviderDeliveryStatus: "unknown",
		},
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

func resolveCloudflareEmailEndpoint(endpoint, accountID string) (string, error) {
	resolved := strings.TrimSpace(endpoint)
	if resolved == "" {
		resolved = CloudflareEmailEndpointDefault
	}
	if strings.Contains(resolved, "{account_id}") {
		accountID = strings.TrimSpace(accountID)
		if accountID == "" {
			return "", errors.New("cloudflare provider requires account id")
		}
		resolved = strings.ReplaceAll(resolved, "{account_id}", url.PathEscape(accountID))
	}
	return resolved, nil
}

func cloudflareAttachments(attachments []Attachment) []Attachment {
	if len(attachments) == 0 {
		return nil
	}
	out := make([]Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		next := Attachment{
			Filename:    strings.TrimSpace(attachment.Filename),
			Content:     attachment.Content,
			Type:        strings.TrimSpace(attachment.Type),
			Disposition: strings.TrimSpace(attachment.Disposition),
		}
		if next.Type == "" && strings.HasSuffix(strings.ToLower(next.Filename), ".pdf") {
			next.Type = "application/pdf"
		}
		if next.Disposition == "" {
			next.Disposition = "attachment"
		}
		out = append(out, next)
	}
	return out
}

func cloudflareHeaders(msg Message) map[string]string {
	headers := make(map[string]string)
	if idempotencyKey := strings.TrimSpace(msg.IdempotencyKey); idempotencyKey != "" {
		headers["X-MistyPass-Idempotency-Key"] = idempotencyKey
	}
	if tenantID := strings.TrimSpace(msg.TenantID); tenantID != "" {
		headers["X-MistyPass-Tenant-ID"] = tenantID
	}
	for key, value := range cloneStringMap(msg.Metadata) {
		headerKey := cloudflareMetadataHeaderKey(key)
		if headerKey == "" {
			continue
		}
		headers[headerKey] = value
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}

func cloudflareMetadataHeaderKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune('-')
		}
	}
	clean := strings.Trim(builder.String(), "-")
	if clean == "" {
		return ""
	}
	return "X-MistyPass-Meta-" + clean
}

func cloudflareDeliveryStatus(delivered, queued, bounced []string) string {
	switch {
	case len(bounced) > 0 && len(delivered)+len(queued) > 0:
		return "partial_bounce"
	case len(bounced) > 0:
		return "bounced"
	case len(queued) > 0:
		return "queued"
	case len(delivered) > 0:
		return "delivered"
	default:
		return "accepted"
	}
}

func cloudflareErrorBody(errors []struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}, fallback string) string {
	parts := make([]string, 0, len(errors))
	for _, item := range errors {
		message := strings.TrimSpace(item.Message)
		if message == "" {
			continue
		}
		if item.Code != 0 {
			message = fmt.Sprintf("%d: %s", item.Code, message)
		}
		parts = append(parts, message)
	}
	if len(parts) > 0 {
		return strings.Join(parts, "; ")
	}
	return strings.TrimSpace(fallback)
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
