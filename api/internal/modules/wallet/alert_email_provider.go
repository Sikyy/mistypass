package wallet

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

type alertEmailSender interface {
	Provider() string
	Send(ctx context.Context, input AlertEmailSendInput) (AlertEmailSendResult, error)
}

type alertEmailConfirmationSender interface {
	Confirm(ctx context.Context, input AlertEmailConfirmInput) (AlertEmailConfirmResult, error)
}

type AlertEmailSendInput struct {
	TenantID       string
	To             []string
	IdempotencyKey string
	Subject        string
	Text           string
}

type AlertEmailSendResult struct {
	ProviderDeliveryID     string
	ProviderDeliveryStatus string
}

type AlertEmailConfirmInput struct {
	ProviderDeliveryID string
}

type AlertEmailConfirmResult struct {
	Confirmed              bool
	ProviderDeliveryID     string
	ProviderDeliveryStatus string
}

type AlertEmailHTTPError struct {
	StatusCode int
	Body       string
}

func (e AlertEmailHTTPError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("email provider http status %d", e.StatusCode)
	}
	return fmt.Sprintf("email provider http status %d: %s", e.StatusCode, body)
}

func (e AlertEmailHTTPError) Retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= http.StatusInternalServerError
}

const resendEndpointDefault = "https://api.resend.com/emails"

type resendSender struct {
	endpoint string
	apiKey   string
	from     string
	client   *http.Client
}

func newResendSender(endpoint, apiKey, from string, timeout time.Duration) (*resendSender, error) {
	nextEndpoint := strings.TrimSpace(endpoint)
	if nextEndpoint == "" {
		nextEndpoint = resendEndpointDefault
	}
	nextAPIKey := strings.TrimSpace(apiKey)
	nextFrom := strings.TrimSpace(from)
	if nextAPIKey == "" || nextFrom == "" {
		return nil, errors.New("resend provider requires api key/from")
	}
	nextTimeout := timeout
	if nextTimeout < time.Second {
		nextTimeout = 5 * time.Second
	}

	return &resendSender{
		endpoint: nextEndpoint,
		apiKey:   nextAPIKey,
		from:     nextFrom,
		client: &http.Client{
			Timeout: nextTimeout,
		},
	}, nil
}

func (s *resendSender) Provider() string {
	return "resend"
}

func (s *resendSender) Send(ctx context.Context, input AlertEmailSendInput) (AlertEmailSendResult, error) {
	payload := map[string]any{
		"from":    s.from,
		"to":      append([]string(nil), input.To...),
		"subject": input.Subject,
		"text":    input.Text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return AlertEmailSendResult{}, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		s.endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return AlertEmailSendResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey := strings.TrimSpace(input.IdempotencyKey); idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return AlertEmailSendResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return AlertEmailSendResult{}, AlertEmailHTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(readBody)),
		}
	}
	readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	var response struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(readBody, &response)
	return AlertEmailSendResult{
		ProviderDeliveryID:     strings.TrimSpace(response.ID),
		ProviderDeliveryStatus: "accepted",
	}, nil
}

func (s *resendSender) Confirm(ctx context.Context, input AlertEmailConfirmInput) (AlertEmailConfirmResult, error) {
	deliveryID := strings.TrimSpace(input.ProviderDeliveryID)
	if deliveryID == "" {
		return AlertEmailConfirmResult{}, nil
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		strings.TrimRight(s.endpoint, "/")+"/"+url.PathEscape(deliveryID),
		nil,
	)
	if err != nil {
		return AlertEmailConfirmResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return AlertEmailConfirmResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return AlertEmailConfirmResult{
			ProviderDeliveryID: deliveryID,
		}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return AlertEmailConfirmResult{}, AlertEmailHTTPError{
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
	providerDeliveryID := strings.TrimSpace(response.ID)
	if providerDeliveryID == "" {
		providerDeliveryID = deliveryID
	}
	status := strings.TrimSpace(response.LastEvent)
	if status == "" {
		status = "accepted"
	}
	return AlertEmailConfirmResult{
		Confirmed:              isPositiveAlertProviderDeliveryStatus(status),
		ProviderDeliveryID:     providerDeliveryID,
		ProviderDeliveryStatus: status,
	}, nil
}
