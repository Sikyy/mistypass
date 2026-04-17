package httpx

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/wallet"
)

func (s *server) getWalletGoogleConfig(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	config, err := s.walletSvc.GetGoogleConfig(tenantID)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrGoogleConfigNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, config)
}

func (s *server) upsertWalletGoogleConfig(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID            string `json:"tenant_id"`
		IssuerID            string `json:"issuer_id"`
		ServiceAccountEmail string `json:"service_account_email"`
		KeyRef              string `json:"key_ref"`
		Status              string `json:"status"`
		Actor               string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	request.TenantID = tenantID

	config, err := s.walletSvc.UpsertGoogleConfig(
		request.TenantID,
		request.IssuerID,
		request.ServiceAccountEmail,
		request.KeyRef,
		request.Status,
		request.Actor,
	)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrIssuerIDRequired),
			errors.Is(err, wallet.ErrServiceAccountEmailRequired),
			errors.Is(err, wallet.ErrKeyRefRequired),
			errors.Is(err, wallet.ErrInvalidConfigStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, config)
}

func (s *server) validateWalletGoogleConfig(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID            string `json:"tenant_id"`
		IssuerID            string `json:"issuer_id"`
		ServiceAccountEmail string `json:"service_account_email"`
		KeyRef              string `json:"key_ref"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	result := s.walletSvc.ValidateGoogleConfig(
		tenantID,
		request.IssuerID,
		request.ServiceAccountEmail,
		request.KeyRef,
	)

	writeJSON(w, http.StatusOK, result)
}

func (s *server) listWalletTemplates(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.walletSvc.ListTemplates(tenantID),
	})
}

func (s *server) createWalletTemplate(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID    string            `json:"tenant_id"`
		PassType    string            `json:"pass_type"`
		ClassID     string            `json:"class_id"`
		Name        string            `json:"name"`
		StyleConfig map[string]string `json:"style_config"`
		Status      string            `json:"status"`
		Actor       string            `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	request.TenantID = tenantID

	created, err := s.walletSvc.CreateTemplate(
		request.TenantID,
		request.PassType,
		request.ClassID,
		request.Name,
		request.StyleConfig,
		request.Status,
		request.Actor,
	)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrTemplateNameRequired),
			errors.Is(err, wallet.ErrInvalidPassType),
			errors.Is(err, wallet.ErrInvalidTemplateStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (s *server) updateWalletTemplateStatus(w http.ResponseWriter, r *http.Request) {
	templateID := chi.URLParam(r, "templateID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	var request struct {
		Status string `json:"status"`
		Actor  string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := s.walletSvc.UpdateTemplateStatus(tenantID, templateID, request.Status, request.Actor)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrTemplateIDRequired), errors.Is(err, wallet.ErrInvalidTemplateStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, wallet.ErrTemplateNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *server) issueWalletPass(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID   string `json:"tenant_id"`
		TemplateID string `json:"template_id"`
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		ExpiresAt  string `json:"expires_at"`
		Actor      string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	request.TenantID = tenantID

	issued, err := s.walletSvc.IssuePass(
		request.TenantID,
		request.TemplateID,
		request.TargetType,
		request.TargetID,
		request.ExpiresAt,
		request.Actor,
	)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrTemplateIDRequired),
			errors.Is(err, wallet.ErrInvalidTargetType),
			errors.Is(err, wallet.ErrTargetIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, wallet.ErrTemplateNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, wallet.ErrTemplateInactive):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, issued)
}

func (s *server) issueWalletPassBatch(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID      string   `json:"tenant_id"`
		TemplateID    string   `json:"template_id"`
		TargetType    string   `json:"target_type"`
		TargetIDs     []string `json:"target_ids"`
		ExpiresAt     string   `json:"expires_at"`
		Actor         string   `json:"actor"`
		ExecutionMode string   `json:"execution_mode"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	request.TenantID = tenantID

	executionMode := strings.ToLower(strings.TrimSpace(request.ExecutionMode))
	if executionMode == "" {
		executionMode = "inline"
	}

	var (
		jobs []wallet.IssueJob
		err  error
	)
	switch executionMode {
	case "inline":
		jobs, err = s.walletSvc.IssuePassBatch(
			request.TenantID,
			request.TemplateID,
			request.TargetType,
			request.TargetIDs,
			request.ExpiresAt,
			request.Actor,
		)
	case "queued":
		jobs, err = s.walletSvc.IssuePassBatchQueued(
			request.TenantID,
			request.TemplateID,
			request.TargetType,
			request.TargetIDs,
			request.ExpiresAt,
			request.Actor,
		)
	default:
		writeError(w, http.StatusBadRequest, "execution_mode must be inline or queued")
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrTemplateIDRequired),
			errors.Is(err, wallet.ErrInvalidTargetType),
			errors.Is(err, wallet.ErrTargetIDsRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, wallet.ErrTemplateNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, wallet.ErrTemplateInactive):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"items":          jobs,
		"execution_mode": executionMode,
	})
}

func (s *server) listWalletPasses(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.walletSvc.ListPasses(tenantID),
	})
}

func (s *server) getWalletPass(w http.ResponseWriter, r *http.Request) {
	passID := chi.URLParam(r, "passID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	record, err := s.walletSvc.GetPass(tenantID, passID)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrPassNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, record)
}

func (s *server) getWalletPassSaveLink(w http.ResponseWriter, r *http.Request) {
	passID := chi.URLParam(r, "passID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	link, err := s.walletSvc.GetSaveLink(tenantID, passID)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrPassNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"save_link": link,
	})
}

func (s *server) listWalletPassDeliveries(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.walletSvc.ListPassDeliveryNotifications(tenantID, r.URL.Query().Get("pass_id")),
	})
}

func (s *server) dispatchWalletPassDelivery(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID           string   `json:"tenant_id"`
		PassID             string   `json:"pass_id"`
		Channels           []string `json:"channels"`
		EmailRecipients    []string `json:"email_recipients"`
		WhatsAppRecipients []string `json:"whatsapp_recipients"`
		Actor              string   `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	request.TenantID = tenantID

	record, err := s.walletSvc.DispatchPassDelivery(
		request.TenantID,
		request.PassID,
		request.Channels,
		request.EmailRecipients,
		request.WhatsAppRecipients,
		request.Actor,
	)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrPassIDRequired),
			errors.Is(err, wallet.ErrPassDeliveryChannelRequired),
			errors.Is(err, wallet.ErrInvalidPassDeliveryChannel):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, wallet.ErrPassNotFound), errors.Is(err, wallet.ErrTemplateNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, record)
}

func (s *server) retryWalletPassDelivery(w http.ResponseWriter, r *http.Request) {
	notificationID := strings.TrimSpace(chi.URLParam(r, "notificationID"))
	if notificationID == "" {
		writeError(w, http.StatusBadRequest, "notificationID is required")
		return
	}

	var request struct {
		TenantID string `json:"tenant_id"`
		Actor    string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	record, err := s.walletSvc.RetryPassDeliveryNotification(tenantID, notificationID, request.Actor)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrPassDeliveryNotificationNotFound), errors.Is(err, wallet.ErrPassNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, wallet.ErrPassDeliveryRetryNotAllowed):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, record)
}

func (s *server) listWalletPhysicalCardTasks(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.walletSvc.ListPhysicalCardTasks(tenantID),
	})
}

func (s *server) createWalletPhysicalCardTask(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID   string `json:"tenant_id"`
		PassID     string `json:"pass_id"`
		TaskType   string `json:"task_type"`
		CardNumber string `json:"card_number"`
		Note       string `json:"note"`
		Actor      string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	request.TenantID = tenantID

	created, err := s.walletSvc.CreatePhysicalCardTask(
		request.TenantID,
		request.PassID,
		request.TaskType,
		request.CardNumber,
		request.Note,
		request.Actor,
	)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrPassIDRequired),
			errors.Is(err, wallet.ErrInvalidPhysicalCardTaskType),
			errors.Is(err, wallet.ErrPhysicalCardTaskEmployeePassRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, wallet.ErrPassNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (s *server) updateWalletPhysicalCardTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	var request struct {
		TenantID   string `json:"tenant_id"`
		Status     string `json:"status"`
		CardNumber string `json:"card_number"`
		Note       string `json:"note"`
		Actor      string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	request.TenantID = tenantID

	updated, err := s.walletSvc.UpdatePhysicalCardTaskStatus(
		request.TenantID,
		taskID,
		request.Status,
		request.CardNumber,
		request.Note,
		request.Actor,
	)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrInvalidPhysicalCardTaskStatus):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, wallet.ErrInvalidPhysicalCardTaskTransition):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, wallet.ErrPhysicalCardTaskNotFound), errors.Is(err, wallet.ErrPassNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *server) suspendWalletPass(w http.ResponseWriter, r *http.Request) {
	s.changeWalletPassStatus(w, r, "suspended")
}

func (s *server) activateWalletPass(w http.ResponseWriter, r *http.Request) {
	s.changeWalletPassStatus(w, r, "active")
}

func (s *server) revokeWalletPass(w http.ResponseWriter, r *http.Request) {
	s.changeWalletPassStatus(w, r, "revoked")
}

func (s *server) changeWalletPassStatus(w http.ResponseWriter, r *http.Request, status string) {
	passID := chi.URLParam(r, "passID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	var (
		record wallet.PassInstance
		err    error
	)

	switch status {
	case "suspended":
		record, err = s.walletSvc.SuspendPass(tenantID, passID, "admin")
	case "active":
		record, err = s.walletSvc.ActivatePass(tenantID, passID, "admin")
	case "revoked":
		record, err = s.walletSvc.RevokePass(tenantID, passID, "admin")
	default:
		writeError(w, http.StatusBadRequest, "invalid wallet pass status operation")
		return
	}

	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrPassNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, wallet.ErrInvalidPassTransition):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, record)
}

func (s *server) listWalletJobs(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.walletSvc.ListJobs(tenantID),
	})
}

func (s *server) listWalletJobSummary(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	maxRetry := s.cfg.WalletJobProcessDefaultMaxRetry
	if maxRetry <= 0 {
		maxRetry = 3
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("max_retry")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "max_retry must be a non-negative integer")
			return
		}
		if parsed > 0 {
			maxRetry = parsed
		}
	}

	writeJSON(w, http.StatusOK, s.walletSvc.GetJobSummary(tenantID, maxRetry))
}

func (s *server) listWalletJobMetrics(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	window := s.cfg.WalletJobMetricsDefaultWindow
	if window < time.Second {
		window = 15 * time.Minute
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("window_seconds")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "window_seconds must be an integer > 0")
			return
		}
		window = time.Duration(parsed) * time.Second
	}

	maxRetry := s.cfg.WalletJobProcessDefaultMaxRetry
	if maxRetry <= 0 {
		maxRetry = 3
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("max_retry")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "max_retry must be a non-negative integer")
			return
		}
		if parsed > 0 {
			maxRetry = parsed
		}
	}

	dlqAlertThreshold := s.cfg.WalletDLQAlertThreshold
	if dlqAlertThreshold <= 0 {
		dlqAlertThreshold = 20
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("dlq_alert_threshold")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "dlq_alert_threshold must be an integer > 0")
			return
		}
		dlqAlertThreshold = parsed
	}

	writeJSON(
		w,
		http.StatusOK,
		s.walletSvc.GetJobMetrics(tenantID, window, maxRetry, dlqAlertThreshold),
	)
}

func (s *server) listWalletJobMetricsTrend(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	window := s.cfg.WalletJobMetricsDefaultWindow
	if window < time.Second {
		window = 15 * time.Minute
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("window_seconds")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "window_seconds must be an integer > 0")
			return
		}
		window = time.Duration(parsed) * time.Second
	}

	bucketCount := 12
	if raw := strings.TrimSpace(r.URL.Query().Get("bucket_count")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "bucket_count must be an integer > 0")
			return
		}
		if parsed > 120 {
			writeError(w, http.StatusBadRequest, "bucket_count must be <= 120")
			return
		}
		bucketCount = parsed
	}

	maxRetry := s.cfg.WalletJobProcessDefaultMaxRetry
	if maxRetry <= 0 {
		maxRetry = 3
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("max_retry")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "max_retry must be a non-negative integer")
			return
		}
		if parsed > 0 {
			maxRetry = parsed
		}
	}

	dlqAlertThreshold := s.cfg.WalletDLQAlertThreshold
	if dlqAlertThreshold <= 0 {
		dlqAlertThreshold = 20
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("dlq_alert_threshold")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "dlq_alert_threshold must be an integer > 0")
			return
		}
		dlqAlertThreshold = parsed
	}

	writeJSON(
		w,
		http.StatusOK,
		s.walletSvc.GetJobMetricsTrend(tenantID, window, bucketCount, maxRetry, dlqAlertThreshold),
	)
}

func (s *server) defaultWalletJobAlertSubscription(tenantID string) wallet.JobAlertSubscription {
	dlqAlertThreshold := s.cfg.WalletDLQAlertThreshold
	if dlqAlertThreshold <= 0 {
		dlqAlertThreshold = 20
	}
	window := s.cfg.WalletJobMetricsDefaultWindow
	if window < time.Second {
		window = 15 * time.Minute
	}
	return wallet.JobAlertSubscription{
		TenantID:          tenantID,
		Enabled:           true,
		DLQAlertThreshold: dlqAlertThreshold,
		WindowSeconds:     int64(window.Seconds()),
		CooldownSeconds:   int64((15 * time.Minute).Seconds()),
		Channels: wallet.JobAlertSubscriptionChannels{
			Email:    true,
			WhatsApp: false,
		},
		ReceiverGroups: []string{"security"},
		UpdatedAt:      time.Now().UTC(),
	}
}

func (s *server) getWalletJobAlertSubscription(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	record, found := s.walletSvc.GetJobAlertSubscription(tenantID)
	if !found {
		record = s.defaultWalletJobAlertSubscription(tenantID)
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *server) listWalletJobAlertNotifications(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer > 0")
			return
		}
		if parsed > 500 {
			writeError(w, http.StatusBadRequest, "limit must be <= 500")
			return
		}
		limit = parsed
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.walletSvc.ListJobAlertNotifications(tenantID, limit),
	})
}

func (s *server) dispatchWalletJobAlerts(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID          string `json:"tenant_id"`
		WindowSeconds     int    `json:"window_seconds"`
		MaxRetry          int    `json:"max_retry"`
		DLQAlertThreshold int    `json:"dlq_alert_threshold"`
		Actor             string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	subscription, found := s.walletSvc.GetJobAlertSubscription(tenantID)
	if !found {
		subscription = s.defaultWalletJobAlertSubscription(tenantID)
	}

	window := time.Duration(subscription.WindowSeconds) * time.Second
	if window < time.Second {
		window = s.cfg.WalletJobMetricsDefaultWindow
		if window < time.Second {
			window = 15 * time.Minute
		}
	}
	if request.WindowSeconds < 0 {
		writeError(w, http.StatusBadRequest, "window_seconds must be an integer >= 0")
		return
	}
	if request.WindowSeconds > 0 {
		window = time.Duration(request.WindowSeconds) * time.Second
	}

	maxRetry := s.cfg.WalletJobProcessDefaultMaxRetry
	if maxRetry <= 0 {
		maxRetry = 3
	}
	if request.MaxRetry < 0 {
		writeError(w, http.StatusBadRequest, "max_retry must be a non-negative integer")
		return
	}
	if request.MaxRetry > 0 {
		maxRetry = request.MaxRetry
	}

	dlqAlertThreshold := subscription.DLQAlertThreshold
	if dlqAlertThreshold <= 0 {
		dlqAlertThreshold = s.cfg.WalletDLQAlertThreshold
		if dlqAlertThreshold <= 0 {
			dlqAlertThreshold = 20
		}
	}
	if request.DLQAlertThreshold < 0 {
		writeError(w, http.StatusBadRequest, "dlq_alert_threshold must be an integer >= 0")
		return
	}
	if request.DLQAlertThreshold > 0 {
		dlqAlertThreshold = request.DLQAlertThreshold
	}

	metrics := s.walletSvc.GetJobMetrics(tenantID, window, maxRetry, dlqAlertThreshold)

	dispatchSubscription := subscription
	dispatchSubscription.TenantID = tenantID
	dispatchSubscription.WindowSeconds = int64(window.Seconds())
	dispatchSubscription.DLQAlertThreshold = dlqAlertThreshold

	result, err := s.walletSvc.DispatchJobMetricsAlerts(dispatchSubscription, metrics.Alerts, request.Actor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id":           result.TenantID,
		"window_seconds":      int64(window.Seconds()),
		"max_retry":           maxRetry,
		"dlq_alert_threshold": dlqAlertThreshold,
		"total_alerts":        result.TotalAlerts,
		"dispatched":          result.Dispatched,
		"skipped":             result.Skipped,
		"failed":              result.Failed,
		"items":               result.Items,
		"updated_at":          result.UpdatedAt,
	})
}

func (s *server) retryWalletJobAlertNotification(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID string `json:"tenant_id"`
		Actor    string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	notificationID := strings.TrimSpace(chi.URLParam(r, "notificationID"))
	if notificationID == "" {
		writeError(w, http.StatusBadRequest, "notificationID is required")
		return
	}

	record, err := s.walletSvc.RetryJobAlertNotification(tenantID, notificationID, request.Actor)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrJobAlertNotificationNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, wallet.ErrJobAlertRetryNotAllowed):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *server) upsertWalletJobAlertSubscription(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID          string   `json:"tenant_id"`
		Enabled           *bool    `json:"enabled"`
		DLQAlertThreshold int      `json:"dlq_alert_threshold"`
		WindowSeconds     int      `json:"window_seconds"`
		CooldownSeconds   int      `json:"cooldown_seconds"`
		ReceiverGroups    []string `json:"receiver_groups"`
		Channels          *struct {
			Email    *bool `json:"email"`
			WhatsApp *bool `json:"whatsapp"`
		} `json:"channels"`
		Actor string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	current, found := s.walletSvc.GetJobAlertSubscription(tenantID)
	if !found {
		current = s.defaultWalletJobAlertSubscription(tenantID)
	}

	enabled := current.Enabled
	if request.Enabled != nil {
		enabled = *request.Enabled
	}

	dlqAlertThreshold := request.DLQAlertThreshold
	if dlqAlertThreshold == 0 {
		dlqAlertThreshold = current.DLQAlertThreshold
	}

	windowSeconds := request.WindowSeconds
	if windowSeconds == 0 {
		windowSeconds = int(current.WindowSeconds)
	}

	cooldownSeconds := request.CooldownSeconds
	if cooldownSeconds == 0 {
		cooldownSeconds = int(current.CooldownSeconds)
	}

	emailEnabled := current.Channels.Email
	whatsAppEnabled := current.Channels.WhatsApp
	if request.Channels != nil {
		if request.Channels.Email != nil {
			emailEnabled = *request.Channels.Email
		}
		if request.Channels.WhatsApp != nil {
			whatsAppEnabled = *request.Channels.WhatsApp
		}
	}

	receiverGroups := current.ReceiverGroups
	if request.ReceiverGroups != nil {
		receiverGroups = request.ReceiverGroups
	}

	record, err := s.walletSvc.UpsertJobAlertSubscription(
		wallet.JobAlertSubscriptionUpsertOptions{
			TenantID:          tenantID,
			Enabled:           enabled,
			DLQAlertThreshold: dlqAlertThreshold,
			Window:            time.Duration(windowSeconds) * time.Second,
			Cooldown:          time.Duration(cooldownSeconds) * time.Second,
			EmailEnabled:      emailEnabled,
			WhatsAppEnabled:   whatsAppEnabled,
			ReceiverGroups:    receiverGroups,
			Actor:             request.Actor,
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrInvalidJobAlertSubscriptionOptions):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, record)
}

func (s *server) listWalletDLQCleanupArchives(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
			return
		}
		if parsed > 0 {
			limit = parsed
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.walletSvc.ListDLQCleanupArchives(tenantID, limit),
	})
}

func (s *server) getWalletJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	record, err := s.walletSvc.GetJob(tenantID, jobID)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrJobNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, record)
}

func (s *server) retryWalletJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	var request struct {
		TenantID string `json:"tenant_id"`
		TargetID string `json:"target_id"`
		Actor    string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	record, err := s.walletSvc.RetryJob(tenantID, jobID, request.TargetID, request.Actor)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrJobNotFound), errors.Is(err, wallet.ErrTemplateNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, wallet.ErrTemplateInactive), errors.Is(err, wallet.ErrJobRetryNotAllowed):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, wallet.ErrTemplateIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, record)
}

func (s *server) requeueWalletDLQJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	var request struct {
		TenantID string `json:"tenant_id"`
		TargetID string `json:"target_id"`
		Actor    string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	record, err := s.walletSvc.RequeueDLQJob(tenantID, jobID, request.TargetID, request.Actor)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrJobNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, wallet.ErrTargetIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, wallet.ErrJobNotInDLQ):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, record)
}

func (s *server) requeueWalletDLQJobs(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID         string `json:"tenant_id"`
		Limit            int    `json:"limit"`
		ErrorCode        string `json:"error_code"`
		TargetIDOverride string `json:"target_id_override"`
		Actor            string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	result, err := s.walletSvc.RequeueDLQJobs(wallet.JobDLQRequeueOptions{
		TenantID:         tenantID,
		Limit:            request.Limit,
		ErrorCode:        request.ErrorCode,
		TargetIDOverride: request.TargetIDOverride,
		Actor:            request.Actor,
	})
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrInvalidJobDLQOptions):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *server) cleanupWalletDLQJobs(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID         string `json:"tenant_id"`
		Limit            int    `json:"limit"`
		ErrorCode        string `json:"error_code"`
		OlderThanSeconds int    `json:"older_than_seconds"`
		Actor            string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	limit := request.Limit
	if limit <= 0 {
		limit = s.cfg.WalletDLQCleanupDefaultLimit
		if limit <= 0 {
			limit = 50
		}
	}
	olderThan := s.cfg.WalletDLQCleanupDefaultOlderThan
	if olderThan < time.Second {
		olderThan = 24 * time.Hour
	}
	if request.OlderThanSeconds > 0 {
		olderThan = time.Duration(request.OlderThanSeconds) * time.Second
	}

	options := wallet.JobDLQCleanupOptions{
		TenantID:  tenantID,
		Limit:     limit,
		ErrorCode: request.ErrorCode,
		OlderThan: olderThan,
		Actor:     request.Actor,
	}

	result, err := s.walletSvc.CleanupDLQJobs(options)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrInvalidJobDLQOptions):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *server) processWalletJobs(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID      string `json:"tenant_id"`
		Limit         int    `json:"limit"`
		WorkerCount   int    `json:"worker_count"`
		MaxRetry      int    `json:"max_retry"`
		BaseBackoffMS int    `json:"base_backoff_ms"`
		MaxBackoffMS  int    `json:"max_backoff_ms"`
		Actor         string `json:"actor"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}

	options := wallet.JobProcessOptions{
		TenantID:    tenantID,
		Limit:       request.Limit,
		WorkerCount: request.WorkerCount,
		Actor:       request.Actor,
	}
	maxRetry := request.MaxRetry
	if maxRetry <= 0 {
		maxRetry = s.cfg.WalletJobProcessDefaultMaxRetry
		if maxRetry <= 0 {
			maxRetry = 3
		}
	}
	options.MaxRetry = maxRetry
	if request.BaseBackoffMS > 0 {
		options.BaseBackoff = time.Duration(request.BaseBackoffMS) * time.Millisecond
	}
	if request.MaxBackoffMS > 0 {
		options.MaxBackoff = time.Duration(request.MaxBackoffMS) * time.Millisecond
	}

	result, err := s.walletSvc.ProcessIssueJobs(options)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrInvalidJobProcessOptions):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *server) listWalletAuditLogs(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.walletSvc.ListAuditLogs(tenantID),
	})
}
