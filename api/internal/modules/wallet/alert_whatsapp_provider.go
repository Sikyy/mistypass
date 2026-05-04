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
	Send(ctx context.Context, input AlertWhatsAppSendInput) (AlertWhatsAppSendResult, error)
}

type AlertWhatsAppSendInput struct {
	TenantID       string
	To             []string
	IdempotencyKey string
	Text           string
	// Template-based messaging (optional, overrides Text if set)
	TemplateName   string
	TemplateParams []string // {{1}}, {{2}}, etc.
	TemplateLang   string   // e.g. "en_US", "id"
}

type AlertWhatsAppSendResult struct {
	ProviderDeliveryID     string
	ProviderDeliveryStatus string
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

func (s *whatsAppMockSender) Send(_ context.Context, _ AlertWhatsAppSendInput) (AlertWhatsAppSendResult, error) {
	return AlertWhatsAppSendResult{
		ProviderDeliveryStatus: "accepted",
	}, nil
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

func (s *metaWhatsAppSender) Send(ctx context.Context, input AlertWhatsAppSendInput) (AlertWhatsAppSendResult, error) {
	nextTo := dedupStrings(input.To)
	if len(nextTo) == 0 {
		return AlertWhatsAppSendResult{}, errors.New("meta whatsapp requires at least one receiver")
	}
	nextText := strings.TrimSpace(input.Text)
	if nextText == "" {
		return AlertWhatsAppSendResult{}, errors.New("meta whatsapp text is required")
	}

	var providerDeliveryID string
	for i := range nextTo {
		var payload map[string]any
		if input.TemplateName != "" {
			// Template-based message (approved by Meta, supports variables)
			lang := input.TemplateLang
			if lang == "" {
				lang = "en_US"
			}
			templatePayload := map[string]any{
				"name":     input.TemplateName,
				"language": map[string]string{"code": lang},
			}
			if len(input.TemplateParams) > 0 {
				params := make([]map[string]any, len(input.TemplateParams))
				for j, p := range input.TemplateParams {
					params[j] = map[string]any{"type": "text", "text": p}
				}
				templatePayload["components"] = []map[string]any{
					{"type": "body", "parameters": params},
				}
			}
			payload = map[string]any{
				"messaging_product": "whatsapp",
				"to":                nextTo[i],
				"type":              "template",
				"template":          templatePayload,
			}
		} else {
			// Plain text message (for testing or user-initiated conversations)
			payload = map[string]any{
				"messaging_product": "whatsapp",
				"to":                nextTo[i],
				"type":              "text",
				"text": map[string]any{
					"body":        nextText,
					"preview_url": false,
				},
			}
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return AlertWhatsAppSendResult{}, err
		}

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			fmt.Sprintf("%s/%s/messages", s.endpointBase, s.phoneNumberID),
			bytes.NewReader(body),
		)
		if err != nil {
			return AlertWhatsAppSendResult{}, err
		}
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey := strings.TrimSpace(input.IdempotencyKey); idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			return AlertWhatsAppSendResult{}, err
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			resp.Body.Close()
			return AlertWhatsAppSendResult{}, AlertEmailHTTPError{
				StatusCode: resp.StatusCode,
				Body:       strings.TrimSpace(string(readBody)),
			}
		}
		readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		var response struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(readBody, &response)
		if providerDeliveryID == "" && len(response.Messages) > 0 {
			providerDeliveryID = strings.TrimSpace(response.Messages[0].ID)
		}
	}
	return AlertWhatsAppSendResult{
		ProviderDeliveryID:     providerDeliveryID,
		ProviderDeliveryStatus: "accepted",
	}, nil
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
