package wallet

import (
	"context"
	"github.com/mistypass/cloud/api/internal/mail"
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

type AlertEmailHTTPError = mail.HTTPError

type resendSender struct {
	provider *mail.ResendProvider
}

type cloudflareSender struct {
	provider *mail.CloudflareProvider
}

func newResendSender(endpoint, apiKey, from string, timeout time.Duration) (*resendSender, error) {
	provider, err := mail.NewResendProvider(mail.ResendOptions{
		Endpoint: endpoint,
		APIKey:   apiKey,
		From:     from,
		Timeout:  timeout,
	})
	if err != nil {
		return nil, err
	}
	return &resendSender{provider: provider}, nil
}

func newCloudflareSender(endpoint, accountID, apiToken, from string, timeout time.Duration) (*cloudflareSender, error) {
	provider, err := mail.NewCloudflareProvider(mail.CloudflareOptions{
		Endpoint:  endpoint,
		AccountID: accountID,
		APIToken:  apiToken,
		From:      from,
		Timeout:   timeout,
	})
	if err != nil {
		return nil, err
	}
	return &cloudflareSender{provider: provider}, nil
}

func (s *resendSender) Provider() string {
	return s.provider.Provider()
}

func (s *cloudflareSender) Provider() string {
	return s.provider.Provider()
}

func (s *resendSender) Send(ctx context.Context, input AlertEmailSendInput) (AlertEmailSendResult, error) {
	receipt, err := s.provider.Send(ctx, mail.Message{
		TenantID:       input.TenantID,
		To:             input.To,
		IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
		Subject:        strings.TrimSpace(input.Subject),
		Text:           strings.TrimSpace(input.Text),
	})
	if err != nil {
		return AlertEmailSendResult{}, err
	}
	return AlertEmailSendResult{
		ProviderDeliveryID:     strings.TrimSpace(receipt.ProviderDeliveryID),
		ProviderDeliveryStatus: strings.TrimSpace(receipt.ProviderDeliveryStatus),
	}, nil
}

func (s *cloudflareSender) Send(ctx context.Context, input AlertEmailSendInput) (AlertEmailSendResult, error) {
	receipt, err := s.provider.Send(ctx, mail.Message{
		TenantID:       input.TenantID,
		To:             input.To,
		IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
		Subject:        strings.TrimSpace(input.Subject),
		Text:           strings.TrimSpace(input.Text),
	})
	if err != nil {
		return AlertEmailSendResult{}, err
	}
	return AlertEmailSendResult{
		ProviderDeliveryID:     strings.TrimSpace(receipt.ProviderDeliveryID),
		ProviderDeliveryStatus: strings.TrimSpace(receipt.ProviderDeliveryStatus),
	}, nil
}

func (s *resendSender) Confirm(ctx context.Context, input AlertEmailConfirmInput) (AlertEmailConfirmResult, error) {
	deliveryID := strings.TrimSpace(input.ProviderDeliveryID)
	if deliveryID == "" {
		return AlertEmailConfirmResult{}, nil
	}
	confirmed, err := s.provider.Confirm(ctx, deliveryID)
	if err != nil {
		return AlertEmailConfirmResult{}, err
	}
	return AlertEmailConfirmResult{
		Confirmed:              confirmed.Confirmed,
		ProviderDeliveryID:     strings.TrimSpace(confirmed.ProviderDeliveryID),
		ProviderDeliveryStatus: strings.TrimSpace(confirmed.ProviderDeliveryStatus),
	}, nil
}

func (s *cloudflareSender) Confirm(ctx context.Context, input AlertEmailConfirmInput) (AlertEmailConfirmResult, error) {
	deliveryID := strings.TrimSpace(input.ProviderDeliveryID)
	if deliveryID == "" {
		return AlertEmailConfirmResult{}, nil
	}
	confirmed, err := s.provider.Confirm(ctx, deliveryID)
	if err != nil {
		return AlertEmailConfirmResult{}, err
	}
	return AlertEmailConfirmResult{
		Confirmed:              confirmed.Confirmed,
		ProviderDeliveryID:     strings.TrimSpace(confirmed.ProviderDeliveryID),
		ProviderDeliveryStatus: strings.TrimSpace(confirmed.ProviderDeliveryStatus),
	}, nil
}
