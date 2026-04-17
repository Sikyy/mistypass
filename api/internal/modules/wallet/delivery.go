package wallet

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type PassDeliveryChannelResult struct {
	Channel       string   `json:"channel"`
	Status        string   `json:"status"`
	Reason        string   `json:"reason,omitempty"`
	Provider      string   `json:"provider,omitempty"`
	ProviderError string   `json:"provider_error,omitempty"`
	Retryable     bool     `json:"retryable"`
	Receivers     []string `json:"receivers,omitempty"`
}

type PassDeliveryNotification struct {
	ID                   string                      `json:"id"`
	TenantID             string                      `json:"tenant_id"`
	PassID               string                      `json:"pass_id"`
	TemplateID           string                      `json:"template_id"`
	TargetType           string                      `json:"target_type"`
	TargetID             string                      `json:"target_id"`
	Channels             []string                    `json:"channels,omitempty"`
	Status               string                      `json:"status"`
	Reason               string                      `json:"reason,omitempty"`
	Attempt              int                         `json:"attempt,omitempty"`
	Retryable            bool                        `json:"retryable"`
	Provider             string                      `json:"provider,omitempty"`
	ProviderError        string                      `json:"provider_error,omitempty"`
	ChannelResults       []PassDeliveryChannelResult `json:"channel_results,omitempty"`
	SourceNotificationID string                      `json:"source_notification_id,omitempty"`
	TriggeredAt          time.Time                   `json:"triggered_at"`
}

func clonePassDeliveryChannelResults(items []PassDeliveryChannelResult) []PassDeliveryChannelResult {
	if len(items) == 0 {
		return nil
	}
	output := make([]PassDeliveryChannelResult, 0, len(items))
	for i := range items {
		record := items[i]
		record.Receivers = append([]string(nil), items[i].Receivers...)
		output = append(output, record)
	}
	return output
}

func clonePassDeliveryNotification(input PassDeliveryNotification) PassDeliveryNotification {
	output := input
	output.Channels = append([]string(nil), input.Channels...)
	output.ChannelResults = clonePassDeliveryChannelResults(input.ChannelResults)
	return output
}

func clonePassDeliveryNotifications(items []PassDeliveryNotification) []PassDeliveryNotification {
	output := make([]PassDeliveryNotification, 0, len(items))
	for i := range items {
		output = append(output, clonePassDeliveryNotification(items[i]))
	}
	return output
}

func normalizePassDeliveryChannels(channels []string) ([]string, error) {
	normalized := normalizeDispatchChannels(channels)
	if len(normalized) == 0 {
		return nil, ErrPassDeliveryChannelRequired
	}
	for i := range normalized {
		switch normalized[i] {
		case "email", "whatsapp":
		default:
			return nil, ErrInvalidPassDeliveryChannel
		}
	}
	return normalized, nil
}

func (s *Service) ListPassDeliveryNotifications(tenantID, passID string) []PassDeliveryNotification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	nextPassID := strings.TrimSpace(passID)
	items := make([]PassDeliveryNotification, 0, len(s.passDeliveryNotifications))
	for i := range s.passDeliveryNotifications {
		if s.passDeliveryNotifications[i].TenantID != nextTenantID {
			continue
		}
		if nextPassID != "" && s.passDeliveryNotifications[i].PassID != nextPassID {
			continue
		}
		items = append(items, clonePassDeliveryNotification(s.passDeliveryNotifications[i]))
	}
	return items
}

func (s *Service) DispatchPassDelivery(
	tenantID,
	passID string,
	channels,
	emailRecipients,
	whatsAppRecipients []string,
	actor string,
) (PassDeliveryNotification, error) {
	nextPassID := strings.TrimSpace(passID)
	if nextPassID == "" {
		return PassDeliveryNotification{}, ErrPassIDRequired
	}

	nextChannels, err := normalizePassDeliveryChannels(channels)
	if err != nil {
		return PassDeliveryNotification{}, err
	}

	nextTenantID := normalizeTenantID(tenantID)
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	passIndex := -1
	for i := range s.passes {
		if s.passes[i].ID == nextPassID {
			passIndex = i
			break
		}
	}
	if passIndex < 0 || s.passes[passIndex].TenantID != nextTenantID {
		return PassDeliveryNotification{}, ErrPassNotFound
	}

	template, found := findTemplateByID(s.templates, s.passes[passIndex].TemplateID)
	if !found || template.TenantID != nextTenantID {
		return PassDeliveryNotification{}, ErrTemplateNotFound
	}

	id, err := walletID("wdn_")
	if err != nil {
		return PassDeliveryNotification{}, err
	}

	record := PassDeliveryNotification{
		ID:          id,
		TenantID:    nextTenantID,
		PassID:      s.passes[passIndex].ID,
		TemplateID:  s.passes[passIndex].TemplateID,
		TargetType:  s.passes[passIndex].TargetType,
		TargetID:    s.passes[passIndex].TargetID,
		Channels:    append([]string(nil), nextChannels...),
		Attempt:     s.nextPassDeliveryAttemptLocked(nextTenantID, s.passes[passIndex].ID),
		TriggeredAt: now,
	}

	status, reason, provider, providerError, retryable, channelResults := s.dispatchPassDeliveryLocked(
		s.passes[passIndex],
		template,
		nextChannels,
		emailRecipients,
		whatsAppRecipients,
	)
	record.Status = status
	record.Reason = reason
	record.Provider = provider
	record.ProviderError = providerError
	record.Retryable = retryable
	record.ChannelResults = clonePassDeliveryChannelResults(channelResults)

	s.passDeliveryNotifications = append([]PassDeliveryNotification{clonePassDeliveryNotification(record)}, s.passDeliveryNotifications...)
	if len(s.passDeliveryNotifications) > 5000 {
		s.passDeliveryNotifications = s.passDeliveryNotifications[:5000]
	}

	auditResult := record.Status
	if record.Reason != "" {
		auditResult += ":" + record.Reason
	}
	s.appendAuditLocked(nextTenantID, "wallet.pass.delivery.dispatch", nextActor, record.PassID, auditResult)
	if err := s.persistLocked(); err != nil {
		return PassDeliveryNotification{}, err
	}
	return clonePassDeliveryNotification(record), nil
}

func (s *Service) RetryPassDeliveryNotification(tenantID, notificationID, actor string) (PassDeliveryNotification, error) {
	nextTenantID := normalizeTenantID(tenantID)
	nextNotificationID := strings.TrimSpace(notificationID)
	if nextNotificationID == "" {
		return PassDeliveryNotification{}, ErrPassDeliveryNotificationNotFound
	}
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	var source PassDeliveryNotification
	found := false
	for i := range s.passDeliveryNotifications {
		if s.passDeliveryNotifications[i].TenantID != nextTenantID {
			continue
		}
		if s.passDeliveryNotifications[i].ID != nextNotificationID {
			continue
		}
		source = clonePassDeliveryNotification(s.passDeliveryNotifications[i])
		found = true
		break
	}
	if !found {
		return PassDeliveryNotification{}, ErrPassDeliveryNotificationNotFound
	}

	retryChannels := make([]string, 0, len(source.ChannelResults))
	emailRecipients := make([]string, 0)
	whatsAppRecipients := make([]string, 0)
	for i := range source.ChannelResults {
		if source.ChannelResults[i].Status != "failed" || !source.ChannelResults[i].Retryable {
			continue
		}
		retryChannels = append(retryChannels, source.ChannelResults[i].Channel)
		switch source.ChannelResults[i].Channel {
		case "email":
			emailRecipients = append(emailRecipients, source.ChannelResults[i].Receivers...)
		case "whatsapp":
			whatsAppRecipients = append(whatsAppRecipients, source.ChannelResults[i].Receivers...)
		}
	}
	if len(retryChannels) == 0 {
		return PassDeliveryNotification{}, ErrPassDeliveryRetryNotAllowed
	}

	passIndex := -1
	for i := range s.passes {
		if s.passes[i].ID == source.PassID {
			passIndex = i
			break
		}
	}
	if passIndex < 0 || s.passes[passIndex].TenantID != nextTenantID {
		return PassDeliveryNotification{}, ErrPassNotFound
	}

	template, found := findTemplateByID(s.templates, s.passes[passIndex].TemplateID)
	if !found || template.TenantID != nextTenantID {
		return PassDeliveryNotification{}, ErrTemplateNotFound
	}

	id, err := walletID("wdn_")
	if err != nil {
		return PassDeliveryNotification{}, err
	}

	record := PassDeliveryNotification{
		ID:                   id,
		TenantID:             nextTenantID,
		PassID:               source.PassID,
		TemplateID:           source.TemplateID,
		TargetType:           source.TargetType,
		TargetID:             source.TargetID,
		Channels:             append([]string(nil), retryChannels...),
		Attempt:              s.nextPassDeliveryAttemptLocked(nextTenantID, source.PassID),
		SourceNotificationID: source.ID,
		TriggeredAt:          now,
	}

	status, reason, provider, providerError, retryable, channelResults := s.dispatchPassDeliveryLocked(
		s.passes[passIndex],
		template,
		retryChannels,
		emailRecipients,
		whatsAppRecipients,
	)
	record.Status = status
	record.Reason = reason
	record.Provider = provider
	record.ProviderError = providerError
	record.Retryable = retryable
	record.ChannelResults = clonePassDeliveryChannelResults(channelResults)

	s.passDeliveryNotifications = append([]PassDeliveryNotification{clonePassDeliveryNotification(record)}, s.passDeliveryNotifications...)
	if len(s.passDeliveryNotifications) > 5000 {
		s.passDeliveryNotifications = s.passDeliveryNotifications[:5000]
	}

	auditResult := record.Status
	if record.Reason != "" {
		auditResult += ":" + record.Reason
	}
	s.appendAuditLocked(nextTenantID, "wallet.pass.delivery.retry", nextActor, source.ID, auditResult)
	if err := s.persistLocked(); err != nil {
		return PassDeliveryNotification{}, err
	}
	return clonePassDeliveryNotification(record), nil
}

func (s *Service) nextPassDeliveryAttemptLocked(tenantID, passID string) int {
	nextAttempt := 1
	for i := range s.passDeliveryNotifications {
		if s.passDeliveryNotifications[i].TenantID != tenantID {
			continue
		}
		if s.passDeliveryNotifications[i].PassID != passID {
			continue
		}
		if s.passDeliveryNotifications[i].Attempt >= nextAttempt {
			nextAttempt = s.passDeliveryNotifications[i].Attempt + 1
		}
	}
	return nextAttempt
}

func (s *Service) dispatchPassDeliveryLocked(
	pass PassInstance,
	template PassTemplate,
	channels,
	emailRecipients,
	whatsAppRecipients []string,
) (string, string, string, string, bool, []PassDeliveryChannelResult) {
	results := make([]PassDeliveryChannelResult, 0, len(channels))
	for i := range channels {
		switch channels[i] {
		case "email":
			results = append(results, s.dispatchPassDeliveryEmailChannelLocked(pass, template, emailRecipients))
		case "whatsapp":
			results = append(results, s.dispatchPassDeliveryWhatsAppChannelLocked(pass, template, whatsAppRecipients))
		default:
			results = append(results, PassDeliveryChannelResult{
				Channel: channels[i],
				Status:  "failed",
				Reason:  "channel_not_supported",
			})
		}
	}
	return summarizePassDeliveryChannelResults(results)
}

func (s *Service) dispatchPassDeliveryEmailChannelLocked(
	pass PassInstance,
	template PassTemplate,
	recipients []string,
) PassDeliveryChannelResult {
	nextProvider := s.jobAlertEmailProvider
	if nextProvider == "" {
		nextProvider = "mock"
	}
	nextRecipients := dedupStrings(recipients)
	if len(nextRecipients) == 0 {
		return PassDeliveryChannelResult{
			Channel:       "email",
			Status:        "failed",
			Reason:        "email_receivers_not_configured",
			Provider:      nextProvider,
			ProviderError: "email receivers not configured",
			Retryable:     false,
		}
	}

	if nextProvider == "resend" {
		if s.jobAlertEmailSender == nil {
			return PassDeliveryChannelResult{
				Channel:       "email",
				Status:        "failed",
				Reason:        "provider_not_configured",
				Provider:      nextProvider,
				ProviderError: "resend sender is not configured",
				Retryable:     false,
				Receivers:     nextRecipients,
			}
		}
		subject, text := buildPassDeliveryEmailMessage(pass, template)
		err := s.jobAlertEmailSender.Send(
			context.Background(),
			AlertEmailSendInput{
				TenantID: pass.TenantID,
				To:       nextRecipients,
				Subject:  subject,
				Text:     text,
			},
		)
		if err != nil {
			channelRetryable := isJobAlertProviderRetryable(err)
			reason := "provider_error"
			if channelRetryable {
				reason = "provider_transient_error"
			}
			return PassDeliveryChannelResult{
				Channel:       "email",
				Status:        "failed",
				Reason:        reason,
				Provider:      nextProvider,
				ProviderError: strings.TrimSpace(err.Error()),
				Retryable:     channelRetryable,
				Receivers:     nextRecipients,
			}
		}
	}

	return PassDeliveryChannelResult{
		Channel:   "email",
		Status:    "sent",
		Provider:  nextProvider,
		Retryable: false,
		Receivers: nextRecipients,
	}
}

func (s *Service) dispatchPassDeliveryWhatsAppChannelLocked(
	pass PassInstance,
	template PassTemplate,
	receivers []string,
) PassDeliveryChannelResult {
	nextProvider := s.jobAlertWhatsAppProvider
	if nextProvider == "" {
		nextProvider = "mock"
	}
	nextReceivers := dedupStrings(receivers)
	if len(nextReceivers) == 0 {
		return PassDeliveryChannelResult{
			Channel:       "whatsapp",
			Status:        "failed",
			Reason:        "whatsapp_receivers_not_configured",
			Provider:      nextProvider,
			ProviderError: "whatsapp receivers not configured",
			Retryable:     false,
		}
	}

	if s.jobAlertWhatsAppSender == nil {
		return PassDeliveryChannelResult{
			Channel:       "whatsapp",
			Status:        "failed",
			Reason:        "provider_not_configured",
			Provider:      nextProvider,
			ProviderError: nextProvider + " whatsapp sender is not configured",
			Retryable:     false,
			Receivers:     nextReceivers,
		}
	}

	err := s.jobAlertWhatsAppSender.Send(
		context.Background(),
		AlertWhatsAppSendInput{
			TenantID: pass.TenantID,
			To:       nextReceivers,
			Text:     buildPassDeliveryWhatsAppMessage(pass, template),
		},
	)
	if err != nil {
		channelRetryable := isJobAlertProviderRetryable(err)
		reason := "provider_error"
		if channelRetryable {
			reason = "provider_transient_error"
		}
		return PassDeliveryChannelResult{
			Channel:       "whatsapp",
			Status:        "failed",
			Reason:        reason,
			Provider:      nextProvider,
			ProviderError: strings.TrimSpace(err.Error()),
			Retryable:     channelRetryable,
			Receivers:     nextReceivers,
		}
	}
	return PassDeliveryChannelResult{
		Channel:   "whatsapp",
		Status:    "sent",
		Provider:  nextProvider,
		Retryable: false,
		Receivers: nextReceivers,
	}
}

func summarizePassDeliveryChannelResults(
	channelResults []PassDeliveryChannelResult,
) (string, string, string, string, bool, []PassDeliveryChannelResult) {
	if len(channelResults) == 0 {
		return "failed", "channel_disabled", "", "", false, channelResults
	}

	anySent := false
	anyFailed := false
	retryable := false
	firstFailed := PassDeliveryChannelResult{}
	firstSkipped := PassDeliveryChannelResult{}
	for i := range channelResults {
		switch channelResults[i].Status {
		case "sent":
			anySent = true
		case "failed":
			if !anyFailed {
				firstFailed = channelResults[i]
			}
			anyFailed = true
			if channelResults[i].Retryable {
				retryable = true
			}
		case "skipped":
			if firstSkipped.Channel == "" {
				firstSkipped = channelResults[i]
			}
		}
	}

	if anySent {
		provider := summarizePassDeliveryProvider(channelResults)
		if anyFailed {
			return "sent", "partial_channel_failure", provider, firstFailed.ProviderError, retryable, channelResults
		}
		return "sent", "", provider, "", false, channelResults
	}
	if anyFailed {
		return "failed", firstFailed.Reason, firstFailed.Provider, firstFailed.ProviderError, retryable, channelResults
	}
	if firstSkipped.Channel != "" {
		return "skipped", firstSkipped.Reason, firstSkipped.Provider, firstSkipped.ProviderError, false, channelResults
	}
	return "failed", "delivery_not_attempted", "", "", false, channelResults
}

func summarizePassDeliveryProvider(channelResults []PassDeliveryChannelResult) string {
	for i := range channelResults {
		if strings.TrimSpace(channelResults[i].Provider) != "" {
			return channelResults[i].Provider
		}
	}
	return ""
}

func buildPassDeliveryEmailMessage(pass PassInstance, template PassTemplate) (string, string) {
	subject := fmt.Sprintf("MistyPass 已为 %s 准备就绪", pass.TargetID)
	lines := []string{
		fmt.Sprintf("对象：%s (%s)", pass.TargetID, pass.TargetType),
		fmt.Sprintf("模板：%s", template.Name),
		fmt.Sprintf("状态：%s", pass.Status),
	}
	if strings.TrimSpace(pass.SaveLink) != "" {
		lines = append(lines, fmt.Sprintf("保存链接：%s", pass.SaveLink))
	}
	if strings.TrimSpace(pass.ExpiresAt) != "" {
		lines = append(lines, fmt.Sprintf("失效时间：%s", pass.ExpiresAt))
	}
	lines = append(lines, "请通过 MistyPass 保存链接完成领取。")
	return subject, strings.Join(lines, "\n")
}

func buildPassDeliveryWhatsAppMessage(pass PassInstance, template PassTemplate) string {
	lines := []string{
		fmt.Sprintf("MistyPass 已为 %s 准备就绪。", pass.TargetID),
		fmt.Sprintf("模板：%s", template.Name),
	}
	if strings.TrimSpace(pass.SaveLink) != "" {
		lines = append(lines, fmt.Sprintf("保存链接：%s", pass.SaveLink))
	}
	if strings.TrimSpace(pass.ExpiresAt) != "" {
		lines = append(lines, fmt.Sprintf("失效时间：%s", pass.ExpiresAt))
	}
	return strings.Join(lines, "\n")
}
