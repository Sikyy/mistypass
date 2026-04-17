package wallet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type alertEmailSender interface {
	Provider() string
	Send(ctx context.Context, input AlertEmailSendInput) error
}

type AlertEmailSendInput struct {
	TenantID string
	To       []string
	Subject  string
	Text     string
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

func (s *resendSender) Send(ctx context.Context, input AlertEmailSendInput) error {
	payload := map[string]any{
		"from":    s.from,
		"to":      append([]string(nil), input.To...),
		"subject": input.Subject,
		"text":    input.Text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		s.endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return AlertEmailHTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(readBody)),
		}
	}
	return nil
}
