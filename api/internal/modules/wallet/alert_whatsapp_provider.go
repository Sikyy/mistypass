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

type alertWhatsAppSender interface {
	Provider() string
	Send(ctx context.Context, input AlertWhatsAppSendInput) error
}

type AlertWhatsAppSendInput struct {
	TenantID string
	To       []string
	Text     string
}

const (
	whatsAppEndpointDefault = "https://graph.facebook.com/v22.0"
)

type whatsAppMockSender struct{}

func newWhatsAppMockSender() *whatsAppMockSender {
	return &whatsAppMockSender{}
}

func (s *whatsAppMockSender) Provider() string {
	return "mock_whatsapp"
}

func (s *whatsAppMockSender) Send(_ context.Context, _ AlertWhatsAppSendInput) error {
	return nil
}

type metaWhatsAppSender struct {
	endpointBase  string
	apiKey        string
	phoneNumberID string
	client        *http.Client
}

func newMetaWhatsAppSender(
	endpointBase,
	apiKey,
	phoneNumberID string,
	timeout time.Duration,
) (*metaWhatsAppSender, error) {
	nextEndpointBase := strings.TrimRight(strings.TrimSpace(endpointBase), "/")
	if nextEndpointBase == "" {
		nextEndpointBase = whatsAppEndpointDefault
	}
	nextAPIKey := strings.TrimSpace(apiKey)
	nextPhoneNumberID := strings.TrimSpace(phoneNumberID)
	if nextAPIKey == "" || nextPhoneNumberID == "" {
		return nil, errors.New("meta whatsapp provider requires api key/phone number id")
	}
	nextTimeout := timeout
	if nextTimeout < time.Second {
		nextTimeout = 5 * time.Second
	}
	return &metaWhatsAppSender{
		endpointBase:  nextEndpointBase,
		apiKey:        nextAPIKey,
		phoneNumberID: nextPhoneNumberID,
		client: &http.Client{
			Timeout: nextTimeout,
		},
	}, nil
}

func (s *metaWhatsAppSender) Provider() string {
	return "meta"
}

func (s *metaWhatsAppSender) Send(ctx context.Context, input AlertWhatsAppSendInput) error {
	nextTo := dedupStrings(input.To)
	if len(nextTo) == 0 {
		return errors.New("meta whatsapp requires at least one receiver")
	}
	nextText := strings.TrimSpace(input.Text)
	if nextText == "" {
		return errors.New("meta whatsapp text is required")
	}

	for i := range nextTo {
		payload := map[string]any{
			"messaging_product": "whatsapp",
			"to":                nextTo[i],
			"type":              "text",
			"text": map[string]any{
				"body":        nextText,
				"preview_url": false,
			},
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			fmt.Sprintf("%s/%s/messages", s.endpointBase, s.phoneNumberID),
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
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			resp.Body.Close()
			return AlertEmailHTTPError{
				StatusCode: resp.StatusCode,
				Body:       strings.TrimSpace(string(readBody)),
			}
		}
		resp.Body.Close()
	}
	return nil
}

func dedupStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	output := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for i := range items {
		next := strings.TrimSpace(items[i])
		if next == "" {
			continue
		}
		key := strings.ToLower(next)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		output = append(output, next)
	}
	if len(output) == 0 {
		return nil
	}
	return output
}
