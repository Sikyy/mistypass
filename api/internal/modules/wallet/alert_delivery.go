package wallet

import (
	"context"
	"strings"
)

type AlertDeliveryConfirmationInput struct {
	TenantID       string
	IdempotencyKey string
	ChannelResults []JobAlertChannelResult
}

type AlertDeliveryConfirmationResult struct {
	Confirmed      bool
	Retryable      bool
	Provider       string
	ProviderError  string
	ChannelResults []JobAlertChannelResult
}

type AlertDeliveryInput struct {
	TenantID       string
	Channels       []string
	ReceiverGroups []string
	IdempotencyKey string
	EmailSubject   string
	EmailText      string
	WhatsAppText   string
}

type AlertDeliveryResult struct {
	Status         string
	Reason         string
	Provider       string
	ProviderError  string
	Retryable      bool
	ChannelResults []JobAlertChannelResult
}

func (s *Service) DispatchAlert(input AlertDeliveryInput) AlertDeliveryResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	channels := normalizeDispatchChannels(input.Channels)
	if len(channels) == 0 {
		return AlertDeliveryResult{
			Status: "skipped",
			Reason: "channel_disabled",
		}
	}

	results := make([]JobAlertChannelResult, 0, len(channels))
	for i := range channels {
		switch channels[i] {
		case "email":
			results = append(results, s.dispatchAlertEmailMessageLocked(
				strings.TrimSpace(input.TenantID),
				input.ReceiverGroups,
				input.IdempotencyKey,
				input.EmailSubject,
				input.EmailText,
			))
		case "whatsapp":
			results = append(results, s.dispatchAlertWhatsAppMessageLocked(
				strings.TrimSpace(input.TenantID),
				input.ReceiverGroups,
				input.IdempotencyKey,
				input.WhatsAppText,
			))
		default:
			results = append(results, JobAlertChannelResult{
				Channel: channels[i],
				Status:  "skipped",
				Reason:  "channel_disabled",
			})
		}
	}

	status, reason, provider, providerError, retryable, channelResults := summarizeJobAlertChannelResults(results)
	return AlertDeliveryResult{
		Status:         status,
		Reason:         reason,
		Provider:       provider,
		ProviderError:  providerError,
		Retryable:      retryable,
		ChannelResults: cloneJobAlertChannelResults(channelResults),
	}
}

func (s *Service) ConfirmAlertDelivery(input AlertDeliveryConfirmationInput) AlertDeliveryConfirmationResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := cloneJobAlertChannelResults(input.ChannelResults)
	sentCount := 0
	confirmableCount := 0
	confirmedCount := 0
	retryable := false
	firstProvider := ""
	firstProviderError := ""

	for i := range results {
		if results[i].Status != "sent" {
			continue
		}
		sentCount++
		if firstProvider == "" {
			firstProvider = strings.TrimSpace(results[i].Provider)
		}

		switch strings.ToLower(strings.TrimSpace(results[i].Channel)) {
		case "email":
			confirmSender, ok := s.jobAlertEmailSender.(alertEmailConfirmationSender)
			if !ok {
				continue
			}
			deliveryID := strings.TrimSpace(results[i].ProviderDeliveryID)
			if deliveryID == "" {
				continue
			}
			confirmableCount++
			confirmed, err := confirmSender.Confirm(context.Background(), AlertEmailConfirmInput{
				ProviderDeliveryID: deliveryID,
			})
			if err != nil {
				retryable = isJobAlertProviderRetryable(err)
				if firstProviderError == "" {
					firstProviderError = strings.TrimSpace(err.Error())
				}
				continue
			}
			results[i].ProviderDeliveryID = strings.TrimSpace(confirmed.ProviderDeliveryID)
			results[i].ProviderDeliveryStatus = strings.TrimSpace(confirmed.ProviderDeliveryStatus)
			switch {
			case confirmed.Confirmed || isPositiveAlertProviderDeliveryStatus(results[i].ProviderDeliveryStatus):
				confirmedCount++
			case isNegativeAlertProviderDeliveryStatus(results[i].ProviderDeliveryStatus):
				results[i].Status = "failed"
				results[i].Reason = "provider_delivery_failed"
				results[i].Retryable = false
				if firstProviderError == "" {
					firstProviderError = alertProviderDeliveryStatusError(results[i].ProviderDeliveryStatus)
				}
			}
		}
	}

	confirmed := sentCount > 0 && confirmableCount == sentCount && confirmedCount == sentCount
	if confirmed {
		firstProviderError = ""
		retryable = false
	}
	return AlertDeliveryConfirmationResult{
		Confirmed:      confirmed,
		Retryable:      retryable,
		Provider:       firstProvider,
		ProviderError:  firstProviderError,
		ChannelResults: results,
	}
}

func normalizeAlertProviderDeliveryStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func isPositiveAlertProviderDeliveryStatus(status string) bool {
	switch normalizeAlertProviderDeliveryStatus(status) {
	case "delivered", "sent", "success", "succeeded", "opened", "clicked":
		return true
	default:
		return false
	}
}

func isNegativeAlertProviderDeliveryStatus(status string) bool {
	switch normalizeAlertProviderDeliveryStatus(status) {
	case "bounced", "failed", "rejected", "cancelled", "canceled", "complained", "complaint", "undelivered", "delivery_failed", "blocked":
		return true
	default:
		return false
	}
}

func alertProviderDeliveryStatusError(status string) string {
	normalized := normalizeAlertProviderDeliveryStatus(status)
	if normalized == "" {
		return "provider delivery confirmation failed"
	}
	return "provider delivery status " + normalized
}

func (s *Service) dispatchAlertEmailMessageLocked(
	tenantID string,
	receiverGroups []string,
	idempotencyKey string,
	subject string,
	text string,
) JobAlertChannelResult {
	nextProvider := s.jobAlertEmailProvider
	if nextProvider == "" {
		nextProvider = "mock"
	}

	recipients := s.resolveAlertEmailReceiversLocked(receiverGroups)
	if len(recipients) == 0 {
		return JobAlertChannelResult{
			Channel:       "email",
			Status:        "failed",
			Reason:        "email_receivers_not_configured",
			Provider:      nextProvider,
			ProviderError: "email receivers not configured",
			Retryable:     false,
		}
	}

	if nextProvider == "resend" || nextProvider == "spaceemail" {
		nextProvider = "resend"
		if s.jobAlertEmailSender == nil {
			return JobAlertChannelResult{
				Channel:       "email",
				Status:        "failed",
				Reason:        "provider_not_configured",
				Provider:      nextProvider,
				ProviderError: "resend sender is not configured",
				Retryable:     false,
				Receivers:     recipients,
			}
		}
		sendResult, err := s.jobAlertEmailSender.Send(
			context.Background(),
			AlertEmailSendInput{
				TenantID:       tenantID,
				To:             recipients,
				IdempotencyKey: strings.TrimSpace(idempotencyKey),
				Subject:        strings.TrimSpace(subject),
				Text:           strings.TrimSpace(text),
			},
		)
		if err != nil {
			retryable := isJobAlertProviderRetryable(err)
			reason := "provider_error"
			if retryable {
				reason = "provider_transient_error"
			}
			return JobAlertChannelResult{
				Channel:       "email",
				Status:        "failed",
				Reason:        reason,
				Provider:      nextProvider,
				ProviderError: strings.TrimSpace(err.Error()),
				Retryable:     retryable,
				Receivers:     recipients,
			}
		}
		return JobAlertChannelResult{
			Channel:                "email",
			Status:                 "sent",
			Provider:               nextProvider,
			ProviderDeliveryID:     strings.TrimSpace(sendResult.ProviderDeliveryID),
			ProviderDeliveryStatus: strings.TrimSpace(sendResult.ProviderDeliveryStatus),
			Retryable:              false,
			Receivers:              recipients,
		}
	}

	return JobAlertChannelResult{
		Channel:   "email",
		Status:    "sent",
		Provider:  "mock",
		Retryable: false,
		Receivers: recipients,
	}
}

func (s *Service) dispatchAlertWhatsAppMessageLocked(
	tenantID string,
	receiverGroups []string,
	idempotencyKey string,
	text string,
) JobAlertChannelResult {
	nextProvider := s.jobAlertWhatsAppProvider
	if nextProvider == "" {
		nextProvider = "mock"
	}
	receivers := s.resolveAlertWhatsAppReceiversLocked(receiverGroups)
	if len(receivers) == 0 {
		return JobAlertChannelResult{
			Channel:       "whatsapp",
			Status:        "failed",
			Reason:        "whatsapp_receivers_not_configured",
			Provider:      nextProvider,
			ProviderError: "whatsapp receivers not configured",
			Retryable:     false,
		}
	}

	if nextProvider == "mock" || nextProvider == "meta" {
		if s.jobAlertWhatsAppSender == nil {
			return JobAlertChannelResult{
				Channel:       "whatsapp",
				Status:        "failed",
				Reason:        "provider_not_configured",
				Provider:      nextProvider,
				ProviderError: nextProvider + " whatsapp sender is not configured",
				Retryable:     false,
				Receivers:     receivers,
			}
		}
		sendResult, err := s.jobAlertWhatsAppSender.Send(
			context.Background(),
			AlertWhatsAppSendInput{
				TenantID:       tenantID,
				To:             receivers,
				IdempotencyKey: strings.TrimSpace(idempotencyKey),
				Text:           strings.TrimSpace(text),
			},
		)
		if err != nil {
			retryable := isJobAlertProviderRetryable(err)
			reason := "provider_error"
			if retryable {
				reason = "provider_transient_error"
			}
			return JobAlertChannelResult{
				Channel:       "whatsapp",
				Status:        "failed",
				Reason:        reason,
				Provider:      nextProvider,
				ProviderError: strings.TrimSpace(err.Error()),
				Retryable:     retryable,
				Receivers:     receivers,
			}
		}
		return JobAlertChannelResult{
			Channel:                "whatsapp",
			Status:                 "sent",
			Provider:               nextProvider,
			ProviderDeliveryID:     strings.TrimSpace(sendResult.ProviderDeliveryID),
			ProviderDeliveryStatus: strings.TrimSpace(sendResult.ProviderDeliveryStatus),
			Retryable:              false,
			Receivers:              receivers,
		}
	}

	return JobAlertChannelResult{
		Channel:   "whatsapp",
		Status:    "failed",
		Reason:    "provider_not_supported",
		Provider:  nextProvider,
		Retryable: false,
		Receivers: receivers,
	}
}
