package httpx

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/mail"
	"github.com/mistypass/cloud/api/internal/modules/access"
)

const (
	userInvitationResendEndpointDefault          = "https://api.resend.com/emails"
	userInvitationProviderSecretHeader           = "X-MistyPass-Invitation-Webhook-Secret"
	userInvitationProviderSignatureHeader        = "X-MistyPass-Invitation-Signature"
	userInvitationProviderSignatureTimestamp     = "X-MistyPass-Invitation-Timestamp"
	userInvitationProviderSignatureAlgorithm     = "sha256"
	userInvitationProviderSignatureMaxClockSkew  = 5 * time.Minute
	userInvitationProviderWebhookMaxPayloadBytes = 1 << 20
)

type userInvitationProviderReceiptRequest struct {
	TenantID           string `json:"tenant_id"`
	UserID             string `json:"user_id"`
	DeliveryID         string `json:"delivery_id"`
	Status             string `json:"status"`
	Provider           string `json:"provider"`
	ProviderDeliveryID string `json:"provider_delivery_id"`
	ProviderError      string `json:"provider_error"`
	Retryable          bool   `json:"retryable"`
}

type userInvitationProviderDispatchResult struct {
	Status             string
	Provider           string
	ProviderDeliveryID string
	ProviderError      string
	Retryable          bool
}

func (s *server) dispatchUserInvitationEmail(
	r *http.Request,
	delivery access.UserInvitationDelivery,
	user access.AccessUser,
) (access.UserInvitationDelivery, bool) {
	provider := strings.ToLower(strings.TrimSpace(s.cfg.UserInvitationEmailProvider))
	if provider == "" || provider == "queue" {
		return delivery, false
	}

	var result userInvitationProviderDispatchResult
	switch provider {
	case "mock":
		result = userInvitationProviderDispatchResult{
			Status:             "sent",
			Provider:           "mock",
			ProviderDeliveryID: "mock_" + delivery.ID,
		}
	case "resend":
		result = s.dispatchUserInvitationResend(r, delivery, user)
	case "cloudflare":
		result = s.dispatchUserInvitationCloudflare(r, delivery, user)
	default:
		result = userInvitationProviderDispatchResult{
			Status:        "failed",
			Provider:      provider,
			ProviderError: "user invitation email provider is not supported",
		}
	}

	updated, err := s.accessSvc.RecordUserInvitationReceipt(
		delivery.TenantID,
		delivery.UserID,
		delivery.ID,
		result.Status,
		result.Provider,
		result.ProviderDeliveryID,
		result.ProviderError,
		result.Retryable,
	)
	if err != nil {
		return delivery, false
	}
	s.appendUserInvitationReceiptAudit(r, updated, "access")
	return updated, true
}

func (s *server) dispatchUserInvitationCloudflare(
	r *http.Request,
	delivery access.UserInvitationDelivery,
	user access.AccessUser,
) userInvitationProviderDispatchResult {
	provider, err := mail.NewCloudflareProvider(mail.CloudflareOptions{
		Endpoint:  strings.TrimSpace(s.cfg.CloudflareEmailEndpoint),
		AccountID: strings.TrimSpace(s.cfg.CloudflareEmailAccountID),
		APIToken:  strings.TrimSpace(s.cfg.CloudflareEmailAPIToken),
		From:      strings.TrimSpace(s.cfg.UserInvitationEmailFrom),
		Timeout:   firstNonZeroDuration(s.cfg.CloudflareEmailTimeout, 5*time.Second),
	})
	if err != nil {
		return userInvitationProviderDispatchResult{
			Status:        "failed",
			Provider:      "cloudflare",
			ProviderError: err.Error(),
			Retryable:     false,
		}
	}
	receipt, err := provider.Send(r.Context(), mail.Message{
		TenantID:       delivery.TenantID,
		To:             []string{delivery.Email},
		IdempotencyKey: delivery.ID,
		Subject:        "You have been invited to MistyPass",
		Text: fmt.Sprintf(
			"You have been invited to MistyPass as %s. Sign in with %s to finish account setup.",
			firstNonEmptyString(strings.TrimSpace(user.Name), "a user"),
			delivery.Email,
		),
	})
	if err != nil {
		result := userInvitationProviderDispatchResult{
			Status:        "failed",
			Provider:      "cloudflare",
			ProviderError: err.Error(),
			Retryable:     true,
		}
		if httpErr, ok := err.(mail.HTTPError); ok {
			result.Retryable = httpErr.Retryable()
		}
		return result
	}
	return userInvitationProviderDispatchResult{
		Status:             "sent",
		Provider:           receipt.Provider,
		ProviderDeliveryID: strings.TrimSpace(receipt.ProviderDeliveryID),
	}
}

func (s *server) dispatchUserInvitationResend(
	r *http.Request,
	delivery access.UserInvitationDelivery,
	user access.AccessUser,
) userInvitationProviderDispatchResult {
	endpoint := strings.TrimSpace(s.cfg.UserInvitationResendEndpoint)
	if endpoint == "" {
		endpoint = userInvitationResendEndpointDefault
	}
	apiKey := strings.TrimSpace(s.cfg.UserInvitationResendAPIKey)
	from := strings.TrimSpace(s.cfg.UserInvitationEmailFrom)
	if apiKey == "" || from == "" {
		return userInvitationProviderDispatchResult{
			Status:        "failed",
			Provider:      "resend",
			ProviderError: "resend provider requires api key/from",
			Retryable:     false,
		}
	}

	payload := map[string]any{
		"from":    from,
		"to":      []string{delivery.Email},
		"subject": "You have been invited to MistyPass",
		"text": fmt.Sprintf(
			"You have been invited to MistyPass as %s. Sign in with %s to finish account setup.",
			firstNonEmptyString(strings.TrimSpace(user.Name), "a user"),
			delivery.Email,
		),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return userInvitationProviderDispatchResult{
			Status:        "failed",
			Provider:      "resend",
			ProviderError: err.Error(),
			Retryable:     false,
		}
	}

	ctx := r.Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return userInvitationProviderDispatchResult{
			Status:        "failed",
			Provider:      "resend",
			ProviderError: err.Error(),
			Retryable:     false,
		}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", delivery.ID)

	client := &http.Client{Timeout: firstNonZeroDuration(s.cfg.UserInvitationResendTimeout, 5*time.Second)}
	resp, err := client.Do(req)
	if err != nil {
		return userInvitationProviderDispatchResult{
			Status:        "failed",
			Provider:      "resend",
			ProviderError: err.Error(),
			Retryable:     true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		providerError := strings.TrimSpace(string(readBody))
		if providerError == "" {
			providerError = fmt.Sprintf("resend provider http status %d", resp.StatusCode)
		}
		return userInvitationProviderDispatchResult{
			Status:        "failed",
			Provider:      "resend",
			ProviderError: providerError,
			Retryable:     resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError,
		}
	}

	readBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	var response struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(readBody, &response)
	return userInvitationProviderDispatchResult{
		Status:             "sent",
		Provider:           "resend",
		ProviderDeliveryID: strings.TrimSpace(response.ID),
	}
}

func (s *server) receiveUserInvitationProviderReceipt(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, userInvitationProviderWebhookMaxPayloadBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.authorizeUserInvitationProviderWebhook(w, r, body) {
		return
	}

	var request userInvitationProviderReceiptRequest
	if err := json.Unmarshal(body, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := strings.TrimSpace(request.TenantID)
	userID := strings.TrimSpace(request.UserID)
	deliveryID := strings.TrimSpace(request.DeliveryID)
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if deliveryID == "" {
		writeError(w, http.StatusBadRequest, "delivery_id is required")
		return
	}

	user, err := s.accessSvc.GetUser(tenantID, userID)
	if err != nil {
		handleAccessUserError(w, err)
		return
	}

	provider := strings.TrimSpace(request.Provider)
	if provider == "" {
		provider = "provider_webhook"
	}
	delivery, err := s.accessSvc.RecordUserInvitationReceipt(
		tenantID,
		user.ID,
		deliveryID,
		request.Status,
		provider,
		request.ProviderDeliveryID,
		request.ProviderError,
		request.Retryable,
	)
	if err != nil {
		handleUserInvitationError(w, err)
		return
	}
	s.appendUserInvitationReceiptAudit(nil, delivery, "user_invitation_provider")
	writeJSON(w, http.StatusOK, delivery)
}

func (s *server) authorizeUserInvitationProviderWebhook(w http.ResponseWriter, r *http.Request, body []byte) bool {
	secret := strings.TrimSpace(s.cfg.UserInvitationProviderWebhookSecret)
	if secret == "" {
		writeError(w, http.StatusServiceUnavailable, "user invitation provider webhook secret is not configured")
		return false
	}

	if signature := strings.TrimSpace(r.Header.Get(userInvitationProviderSignatureHeader)); signature != "" {
		timestamp := strings.TrimSpace(r.Header.Get(userInvitationProviderSignatureTimestamp))
		if timestamp == "" || !validUserInvitationProviderWebhookTimestamp(timestamp, time.Now().UTC()) {
			writeError(w, http.StatusUnauthorized, "invalid user invitation provider webhook timestamp")
			return false
		}
		expected := signUserInvitationProviderWebhookPayload(secret, timestamp, body)
		if !constantTimeStringEqual(signature, expected) {
			writeError(w, http.StatusUnauthorized, "invalid user invitation provider webhook signature")
			return false
		}
		return true
	}

	provided := strings.TrimSpace(r.Header.Get(userInvitationProviderSecretHeader))
	if provided == "" {
		if token, err := bearerToken(r.Header.Get("Authorization")); err == nil {
			provided = token
		}
	}
	if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid user invitation provider webhook secret")
		return false
	}
	return true
}

func signUserInvitationProviderWebhookPayload(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	mac.Write([]byte(strings.TrimSpace(timestamp)))
	mac.Write([]byte("."))
	mac.Write(body)
	return userInvitationProviderSignatureAlgorithm + "=" + hex.EncodeToString(mac.Sum(nil))
}

func validUserInvitationProviderWebhookTimestamp(value string, now time.Time) bool {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return false
	}
	at := time.Unix(seconds, 0).UTC()
	if at.After(now.Add(userInvitationProviderSignatureMaxClockSkew)) {
		return false
	}
	return !at.Before(now.Add(-userInvitationProviderSignatureMaxClockSkew))
}

func constantTimeStringEqual(left, right string) bool {
	return hmac.Equal([]byte(strings.TrimSpace(left)), []byte(strings.TrimSpace(right)))
}

func (s *server) appendUserInvitationReceiptAudit(r *http.Request, delivery access.UserInvitationDelivery, source string) {
	s.appendAuditLog(
		r,
		delivery.TenantID,
		"reference_user_invitation_receipt",
		fmt.Sprintf(
			"delivery_id=%s,user_id=%s,email=%s,status=%s,provider=%s,provider_delivery_id=%s,retryable=%t",
			delivery.ID,
			delivery.UserID,
			delivery.Email,
			delivery.Status,
			delivery.Provider,
			delivery.ProviderDeliveryID,
			delivery.Retryable,
		),
		source,
	)
}
