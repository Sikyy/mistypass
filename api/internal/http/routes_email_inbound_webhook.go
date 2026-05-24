package httpx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	stateKeyEmailInbound                 = "module_email_inbound"
	emailInboundWebhookSecretHeader      = "X-MistyPass-Email-Webhook-Secret"
	emailInboundWebhookSignatureHeader   = "X-MistyPass-Email-Signature"
	emailInboundWebhookTimestampHeader   = "X-MistyPass-Email-Timestamp"
	emailInboundWebhookSignaturePrefix   = "sha256="
	emailInboundWebhookMaxClockSkew      = 5 * time.Minute
	emailInboundWebhookMaxPayloadBytes   = 1 << 20
	emailInboundEventsMaxPersisted       = 500
	emailInboundProviderDefault          = "cloudflare_email_worker"
	emailInboundEventTypeDefault         = "inbound_message"
	emailInboundWebhookAuditAction       = "email_inbound_event_received"
	emailInboundWebhookAuditSource       = "email_inbound_webhook"
	emailInboundWebhookReceiptTimeLayout = time.RFC3339
)

type emailInboundEvent struct {
	ID                 string                   `json:"id"`
	TenantID           string                   `json:"tenant_id"`
	Provider           string                   `json:"provider"`
	EventType          string                   `json:"event_type"`
	MessageID          string                   `json:"message_id"`
	ProviderDeliveryID string                   `json:"provider_delivery_id,omitempty"`
	From               string                   `json:"from,omitempty"`
	To                 []string                 `json:"to,omitempty"`
	Subject            string                   `json:"subject,omitempty"`
	ReceivedAt         string                   `json:"received_at"`
	Metadata           map[string]string        `json:"metadata,omitempty"`
	Attachments        []emailInboundAttachment `json:"attachments,omitempty"`
	RawPayload         json.RawMessage          `json:"raw_payload,omitempty"`
	CreatedAt          string                   `json:"created_at"`
}

type emailInboundAttachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
}

type emailInboundEventRequest struct {
	TenantID           string                   `json:"tenant_id"`
	Provider           string                   `json:"provider"`
	EventType          string                   `json:"event_type"`
	MessageID          string                   `json:"message_id"`
	ProviderDeliveryID string                   `json:"provider_delivery_id"`
	From               string                   `json:"from"`
	To                 []string                 `json:"to"`
	Subject            string                   `json:"subject"`
	ReceivedAt         string                   `json:"received_at"`
	Metadata           map[string]string        `json:"metadata"`
	Attachments        []emailInboundAttachment `json:"attachments"`
	RawPayload         json.RawMessage          `json:"raw_payload"`
}

type emailInboundStateSnapshot struct {
	Events []emailInboundEvent `json:"events"`
	Seq    int                 `json:"seq"`
}

func (s *server) receiveEmailInboundWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, emailInboundWebhookMaxPayloadBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.authorizeEmailInboundWebhook(w, r, body) {
		return
	}

	var request emailInboundEventRequest
	if err := json.Unmarshal(body, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	event, err := s.recordEmailInboundEvent(request, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.appendAuditLog(
		nil,
		event.TenantID,
		emailInboundWebhookAuditAction,
		emailInboundAuditTarget(event),
		emailInboundWebhookAuditSource,
	)
	writeJSON(w, http.StatusAccepted, event)
}

func (s *server) listEmailInboundEvents(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
			return
		}
		limit = parsed
	}
	eventType := strings.TrimSpace(r.URL.Query().Get("event_type"))
	provider := strings.TrimSpace(r.URL.Query().Get("provider"))
	messageID := strings.TrimSpace(r.URL.Query().Get("message_id"))
	providerDeliveryID := strings.TrimSpace(r.URL.Query().Get("provider_delivery_id"))

	s.emailInboundMu.RLock()
	defer s.emailInboundMu.RUnlock()

	items := make([]emailInboundEvent, 0, len(s.emailInboundEvents))
	for i := range s.emailInboundEvents {
		event := s.emailInboundEvents[i]
		if event.TenantID != tenantID {
			continue
		}
		if eventType != "" && event.EventType != eventType {
			continue
		}
		if provider != "" && event.Provider != provider {
			continue
		}
		if messageID != "" && event.MessageID != messageID {
			continue
		}
		if providerDeliveryID != "" && event.ProviderDeliveryID != providerDeliveryID {
			continue
		}
		items = append(items, cloneEmailInboundEvent(event))
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *server) authorizeEmailInboundWebhook(w http.ResponseWriter, r *http.Request, body []byte) bool {
	secret := strings.TrimSpace(s.cfg.EmailInboundWebhookSecret)
	if secret == "" {
		writeError(w, http.StatusServiceUnavailable, "email inbound webhook secret is not configured")
		return false
	}

	if signature := strings.TrimSpace(r.Header.Get(emailInboundWebhookSignatureHeader)); signature != "" {
		timestamp := strings.TrimSpace(r.Header.Get(emailInboundWebhookTimestampHeader))
		if timestamp == "" || !validEmailInboundWebhookTimestamp(timestamp, time.Now().UTC()) {
			writeError(w, http.StatusUnauthorized, "invalid email inbound webhook timestamp")
			return false
		}
		expected := signEmailInboundWebhookPayload(secret, timestamp, body)
		if !constantTimeStringEqual(signature, expected) {
			writeError(w, http.StatusUnauthorized, "invalid email inbound webhook signature")
			return false
		}
		return true
	}

	provided := strings.TrimSpace(r.Header.Get(emailInboundWebhookSecretHeader))
	if provided == "" {
		if token, err := bearerToken(r.Header.Get("Authorization")); err == nil {
			provided = token
		}
	}
	if provided == "" || !constantTimeStringEqual(provided, secret) {
		writeError(w, http.StatusUnauthorized, "invalid email inbound webhook secret")
		return false
	}
	return true
}

func (s *server) recordEmailInboundEvent(request emailInboundEventRequest, rawBody []byte) (emailInboundEvent, error) {
	tenantID := strings.TrimSpace(request.TenantID)
	if tenantID == "" {
		return emailInboundEvent{}, fmt.Errorf("tenant_id is required")
	}

	receivedAt := time.Now().UTC().Truncate(time.Second)
	if raw := strings.TrimSpace(request.ReceivedAt); raw != "" {
		parsed, err := time.Parse(emailInboundWebhookReceiptTimeLayout, raw)
		if err != nil {
			return emailInboundEvent{}, fmt.Errorf("received_at must be RFC3339")
		}
		receivedAt = parsed.UTC().Truncate(time.Second)
	}
	now := time.Now().UTC().Truncate(time.Second)

	s.emailInboundMu.Lock()
	defer s.emailInboundMu.Unlock()

	s.emailInboundSeq++
	id := fmt.Sprintf("email_evt_%06d", s.emailInboundSeq)
	messageID := strings.TrimSpace(request.MessageID)
	if messageID == "" {
		messageID = "msg_" + id
	}
	event := emailInboundEvent{
		ID:                 id,
		TenantID:           tenantID,
		Provider:           firstNonEmptyString(strings.TrimSpace(request.Provider), emailInboundProviderDefault),
		EventType:          firstNonEmptyString(strings.TrimSpace(request.EventType), emailInboundEventTypeDefault),
		MessageID:          messageID,
		ProviderDeliveryID: strings.TrimSpace(request.ProviderDeliveryID),
		From:               strings.TrimSpace(request.From),
		To:                 uniqueStrings(request.To),
		Subject:            strings.TrimSpace(request.Subject),
		ReceivedAt:         receivedAt.Format(time.RFC3339),
		Metadata:           cloneEmailInboundMetadata(request.Metadata),
		Attachments:        cloneEmailInboundAttachments(request.Attachments),
		RawPayload:         cloneEmailInboundRawPayload(firstNonEmptyRawMessage(request.RawPayload, rawBody)),
		CreatedAt:          now.Format(time.RFC3339),
	}

	s.emailInboundEvents = append([]emailInboundEvent{event}, s.emailInboundEvents...)
	if len(s.emailInboundEvents) > emailInboundEventsMaxPersisted {
		s.emailInboundEvents = s.emailInboundEvents[:emailInboundEventsMaxPersisted]
	}
	s.persistEmailInboundEventsLocked()
	return cloneEmailInboundEvent(event), nil
}

func (s *server) persistEmailInboundEventsLocked() {
	if s.stateStore == nil {
		return
	}
	events := make([]emailInboundEvent, 0, len(s.emailInboundEvents))
	for i := range s.emailInboundEvents {
		events = append(events, cloneEmailInboundEvent(s.emailInboundEvents[i]))
	}
	_ = s.stateStore.Save(stateKeyEmailInbound, emailInboundStateSnapshot{
		Events: events,
		Seq:    s.emailInboundSeq,
	})
}

func (s *server) restoreEmailInboundEventsFromState() {
	if s.stateStore == nil {
		return
	}
	var snapshot emailInboundStateSnapshot
	found, err := s.stateStore.Load(stateKeyEmailInbound, &snapshot)
	if err != nil || !found {
		return
	}
	s.emailInboundMu.Lock()
	defer s.emailInboundMu.Unlock()
	s.emailInboundEvents = make([]emailInboundEvent, 0, len(snapshot.Events))
	for i := range snapshot.Events {
		s.emailInboundEvents = append(s.emailInboundEvents, cloneEmailInboundEvent(snapshot.Events[i]))
	}
	if snapshot.Seq > s.emailInboundSeq {
		s.emailInboundSeq = snapshot.Seq
	}
}

func signEmailInboundWebhookPayload(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	mac.Write([]byte(strings.TrimSpace(timestamp)))
	mac.Write([]byte("."))
	mac.Write(body)
	return emailInboundWebhookSignaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

func validEmailInboundWebhookTimestamp(value string, now time.Time) bool {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return false
	}
	at := time.Unix(seconds, 0).UTC()
	if at.After(now.Add(emailInboundWebhookMaxClockSkew)) {
		return false
	}
	return !at.Before(now.Add(-emailInboundWebhookMaxClockSkew))
}

func emailInboundAuditTarget(event emailInboundEvent) string {
	parts := []string{
		"event_id=" + event.ID,
		"event_type=" + event.EventType,
		"message_id=" + event.MessageID,
		"provider=" + event.Provider,
	}
	if event.ProviderDeliveryID != "" {
		parts = append(parts, "provider_delivery_id="+event.ProviderDeliveryID)
	}
	if related := emailInboundRelatedRef(event.Metadata); related != "" {
		parts = append(parts, "related="+related)
	}
	return strings.Join(parts, ",")
}

func emailInboundRelatedRef(metadata map[string]string) string {
	for _, key := range []string{"report_schedule_id", "wallet_delivery_id", "enterprise_alert_id", "provider_delivery_id"} {
		if value := strings.TrimSpace(metadata[key]); value != "" {
			return key + ":" + value
		}
	}
	return ""
}

func cloneEmailInboundEvent(event emailInboundEvent) emailInboundEvent {
	event.To = append([]string(nil), event.To...)
	event.Metadata = cloneEmailInboundMetadata(event.Metadata)
	event.Attachments = cloneEmailInboundAttachments(event.Attachments)
	event.RawPayload = cloneEmailInboundRawPayload(event.RawPayload)
	return event
}

func cloneEmailInboundMetadata(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		nextKey := strings.TrimSpace(key)
		if nextKey == "" {
			continue
		}
		output[nextKey] = strings.TrimSpace(value)
	}
	if len(output) == 0 {
		return nil
	}
	return output
}

func cloneEmailInboundAttachments(input []emailInboundAttachment) []emailInboundAttachment {
	if len(input) == 0 {
		return nil
	}
	output := make([]emailInboundAttachment, 0, len(input))
	for i := range input {
		item := emailInboundAttachment{
			Filename:    strings.TrimSpace(input[i].Filename),
			ContentType: strings.TrimSpace(input[i].ContentType),
			SizeBytes:   input[i].SizeBytes,
			Checksum:    strings.TrimSpace(input[i].Checksum),
		}
		if item.Filename == "" && item.ContentType == "" && item.SizeBytes <= 0 && item.Checksum == "" {
			continue
		}
		output = append(output, item)
	}
	if len(output) == 0 {
		return nil
	}
	return output
}

func firstNonEmptyRawMessage(primary json.RawMessage, fallback []byte) json.RawMessage {
	if len(primary) > 0 && strings.TrimSpace(string(primary)) != "" {
		return primary
	}
	return fallback
}

func cloneEmailInboundRawPayload(input []byte) json.RawMessage {
	if len(input) == 0 {
		return nil
	}
	output := make([]byte, len(input))
	copy(output, input)
	return json.RawMessage(output)
}
