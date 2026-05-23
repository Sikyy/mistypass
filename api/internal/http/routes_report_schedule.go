package httpx

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/pdfgen"
)

// ---------------------------------------------------------------------------
// Report schedule — email delivery scheduling for analytics reports
// ---------------------------------------------------------------------------

const stateKeyReportSchedule = "module_report_schedule"

type reportSchedule struct {
	ID         string   `json:"id"`
	TenantID   string   `json:"tenant_id"`
	Name       string   `json:"name"`
	ReportType string   `json:"report_type"`
	Frequency  string   `json:"frequency"`
	Recipients []string `json:"recipients"`
	Format     string   `json:"format"`
	DayOfWeek  int      `json:"day_of_week"`
	Enabled    bool     `json:"enabled"`
	LastSentAt string   `json:"last_sent_at,omitempty"`
	NextRunAt  string   `json:"next_run_at,omitempty"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

type resendAttachment struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

type reportSchedulePayload struct {
	TenantID   string   `json:"tenant_id"`
	Name       string   `json:"name"`
	ReportType string   `json:"report_type"`
	Frequency  string   `json:"frequency"`
	Recipients []string `json:"recipients"`
	Format     string   `json:"format"`
	DayOfWeek  *int     `json:"day_of_week"`
	Enabled    *bool    `json:"enabled"`
}

type reportScheduleMutationRequest struct {
	ReportSchedule *reportSchedulePayload `json:"report_schedule"`
	TenantID       string                 `json:"tenant_id"`
	Name           string                 `json:"name"`
	ReportType     string                 `json:"report_type"`
	Frequency      string                 `json:"frequency"`
	Recipients     []string               `json:"recipients"`
	Format         string                 `json:"format"`
	DayOfWeek      *int                   `json:"day_of_week"`
	Enabled        *bool                  `json:"enabled"`
}

func (req reportScheduleMutationRequest) payload() reportSchedulePayload {
	if req.ReportSchedule != nil {
		return *req.ReportSchedule
	}
	return reportSchedulePayload{
		TenantID:   req.TenantID,
		Name:       req.Name,
		ReportType: req.ReportType,
		Frequency:  req.Frequency,
		Recipients: req.Recipients,
		Format:     req.Format,
		DayOfWeek:  req.DayOfWeek,
		Enabled:    req.Enabled,
	}
}

type reportScheduleStateSnapshot struct {
	Schedules []reportSchedule `json:"schedules"`
	Seq       int              `json:"seq"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *server) listReportSchedules(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	s.reportScheduleMu.RLock()
	items := make([]reportSchedule, 0, len(s.reportSchedules))
	for _, item := range s.reportSchedules {
		if item.TenantID != tenantID {
			continue
		}
		items = append(items, item)
	}
	s.reportScheduleMu.RUnlock()

	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) getReportSchedule(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	scheduleID := strings.TrimSpace(chi.URLParam(r, "scheduleID"))

	s.reportScheduleMu.RLock()
	item, exists := s.reportSchedules[scheduleID]
	s.reportScheduleMu.RUnlock()

	if !exists || item.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "report schedule not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *server) createReportSchedule(w http.ResponseWriter, r *http.Request) {
	var req reportScheduleMutationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := req.payload()

	tenantID, ok := s.resolveTenantID(w, r, payload.TenantID)
	if !ok {
		return
	}
	if strings.TrimSpace(tenantID) == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	reportType, ok := normalizeReportScheduleReportType(payload.ReportType)
	if !ok {
		writeError(w, http.StatusBadRequest, "report_type must be one of: access_summary, door_activity, alarm_metrics")
		return
	}

	frequency, ok := normalizeReportScheduleFrequency(payload.Frequency)
	if !ok {
		writeError(w, http.StatusBadRequest, "frequency must be one of: daily, weekly, monthly")
		return
	}

	format, ok := normalizeReportScheduleFormat(payload.Format)
	if !ok {
		writeError(w, http.StatusBadRequest, "format must be one of: json, pdf, csv")
		return
	}

	recipients := uniqueStrings(payload.Recipients)
	if len(recipients) == 0 {
		writeError(w, http.StatusBadRequest, "recipients are required")
		return
	}

	dayOfWeek := 0
	if payload.DayOfWeek != nil {
		dayOfWeek = *payload.DayOfWeek
	}
	if dayOfWeek < 0 || dayOfWeek > 6 {
		writeError(w, http.StatusBadRequest, "day_of_week must be 0-6")
		return
	}

	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}

	now := time.Now().UTC().Truncate(time.Second)

	s.reportScheduleMu.Lock()
	s.reportScheduleSeq++
	item := reportSchedule{
		ID:         fmt.Sprintf("rs_%06d", s.reportScheduleSeq),
		TenantID:   tenantID,
		Name:       name,
		ReportType: reportType,
		Frequency:  frequency,
		Recipients: recipients,
		Format:     format,
		DayOfWeek:  dayOfWeek,
		Enabled:    enabled,
		CreatedAt:  now.Format(time.RFC3339),
		UpdatedAt:  now.Format(time.RFC3339),
	}
	s.reportSchedules[item.ID] = item
	s.persistReportSchedulesLocked()
	s.reportScheduleMu.Unlock()

	s.appendAuditLog(r, tenantID, "report_schedule_created", fmt.Sprintf("schedule_id=%s,report_type=%s,frequency=%s", item.ID, item.ReportType, item.Frequency), "report_schedule")
	writeJSON(w, http.StatusCreated, item)
}

func (s *server) updateReportSchedule(w http.ResponseWriter, r *http.Request) {
	var req reportScheduleMutationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := req.payload()

	tenantID, ok := s.resolveTenantID(w, r, firstNonEmptyString(payload.TenantID, r.URL.Query().Get("tenant_id")))
	if !ok {
		return
	}
	scheduleID := strings.TrimSpace(chi.URLParam(r, "scheduleID"))

	s.reportScheduleMu.RLock()
	current, exists := s.reportSchedules[scheduleID]
	s.reportScheduleMu.RUnlock()

	if !exists || current.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "report schedule not found")
		return
	}

	next := current

	if name := strings.TrimSpace(payload.Name); name != "" {
		next.Name = name
	}
	if payload.ReportType != "" {
		reportType, ok := normalizeReportScheduleReportType(payload.ReportType)
		if !ok {
			writeError(w, http.StatusBadRequest, "report_type must be one of: access_summary, door_activity, alarm_metrics")
			return
		}
		next.ReportType = reportType
	}
	if payload.Frequency != "" {
		frequency, ok := normalizeReportScheduleFrequency(payload.Frequency)
		if !ok {
			writeError(w, http.StatusBadRequest, "frequency must be one of: daily, weekly, monthly")
			return
		}
		next.Frequency = frequency
	}
	if payload.Format != "" {
		format, ok := normalizeReportScheduleFormat(payload.Format)
		if !ok {
			writeError(w, http.StatusBadRequest, "format must be one of: json, pdf, csv")
			return
		}
		next.Format = format
	}
	if len(payload.Recipients) > 0 {
		next.Recipients = uniqueStrings(payload.Recipients)
	}
	if payload.DayOfWeek != nil {
		dow := *payload.DayOfWeek
		if dow < 0 || dow > 6 {
			writeError(w, http.StatusBadRequest, "day_of_week must be 0-6")
			return
		}
		next.DayOfWeek = dow
	}
	if payload.Enabled != nil {
		next.Enabled = *payload.Enabled
	}

	now := time.Now().UTC().Truncate(time.Second)
	next.UpdatedAt = now.Format(time.RFC3339)

	s.reportScheduleMu.Lock()
	s.reportSchedules[next.ID] = next
	s.persistReportSchedulesLocked()
	s.reportScheduleMu.Unlock()

	s.appendAuditLog(r, tenantID, "report_schedule_updated", fmt.Sprintf("schedule_id=%s,report_type=%s,enabled=%v", next.ID, next.ReportType, next.Enabled), "report_schedule")
	writeJSON(w, http.StatusOK, next)
}

func (s *server) deleteReportSchedule(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	scheduleID := strings.TrimSpace(chi.URLParam(r, "scheduleID"))

	s.reportScheduleMu.RLock()
	item, exists := s.reportSchedules[scheduleID]
	s.reportScheduleMu.RUnlock()

	if !exists || item.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "report schedule not found")
		return
	}

	s.reportScheduleMu.Lock()
	delete(s.reportSchedules, item.ID)
	s.persistReportSchedulesLocked()
	s.reportScheduleMu.Unlock()

	s.appendAuditLog(r, tenantID, "report_schedule_deleted", fmt.Sprintf("schedule_id=%s,report_type=%s", item.ID, item.ReportType), "report_schedule")
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// State persistence
// ---------------------------------------------------------------------------

// persistReportSchedulesLocked saves report schedules to the state store.
// Caller must hold reportScheduleMu.
func (s *server) persistReportSchedulesLocked() {
	if s.stateStore == nil {
		return
	}
	schedules := make([]reportSchedule, 0, len(s.reportSchedules))
	for _, rs := range s.reportSchedules {
		schedules = append(schedules, rs)
	}
	_ = s.stateStore.Save(stateKeyReportSchedule, reportScheduleStateSnapshot{
		Schedules: schedules,
		Seq:       s.reportScheduleSeq,
	})
}

// restoreReportSchedulesFromState loads report schedules from the state store on startup.
func (s *server) restoreReportSchedulesFromState() {
	if s.stateStore == nil {
		return
	}
	var snapshot reportScheduleStateSnapshot
	found, err := s.stateStore.Load(stateKeyReportSchedule, &snapshot)
	if err != nil || !found {
		return
	}
	s.reportScheduleMu.Lock()
	defer s.reportScheduleMu.Unlock()
	for _, rs := range snapshot.Schedules {
		s.reportSchedules[rs.ID] = rs
	}
	if snapshot.Seq > s.reportScheduleSeq {
		s.reportScheduleSeq = snapshot.Seq
	}
}

// ---------------------------------------------------------------------------
// Normalization helpers
// ---------------------------------------------------------------------------

func normalizeReportScheduleReportType(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "access_summary":
		return "access_summary", true
	case "door_activity":
		return "door_activity", true
	case "alarm_metrics":
		return "alarm_metrics", true
	default:
		return "", false
	}
}

func normalizeReportScheduleFrequency(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "daily":
		return "daily", true
	case "weekly":
		return "weekly", true
	case "monthly":
		return "monthly", true
	default:
		return "", false
	}
}

func normalizeReportScheduleFormat(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "json":
		return "json", true
	case "pdf":
		return "pdf", true
	case "csv":
		return "csv", true
	default:
		return "", false
	}
}

// ---------------------------------------------------------------------------
// Send report schedule — deliver report via email (Resend)
// ---------------------------------------------------------------------------

func (s *server) sendReportSchedule(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	scheduleID := strings.TrimSpace(chi.URLParam(r, "scheduleID"))

	s.reportScheduleMu.RLock()
	schedule, exists := s.reportSchedules[scheduleID]
	s.reportScheduleMu.RUnlock()

	if !exists || schedule.TenantID != tenantID {
		writeError(w, http.StatusNotFound, "report schedule not found")
		return
	}

	// Check that report email delivery is enabled.
	if !s.cfg.ReportEmailEnabled {
		s.logger.Warn("report email delivery is disabled (set REPORT_EMAIL_ENABLED=true)")
		writeError(w, http.StatusNotImplemented, "report email delivery is not enabled")
		return
	}

	// Validate Resend configuration — reuse user-invitation Resend config.
	resendEndpoint := strings.TrimSpace(s.cfg.UserInvitationResendEndpoint)
	if resendEndpoint == "" {
		resendEndpoint = "https://api.resend.com/emails"
	}
	resendAPIKey := strings.TrimSpace(s.cfg.UserInvitationResendAPIKey)
	if resendAPIKey == "" {
		s.logger.Warn("Resend API key not configured (set USER_INVITATION_RESEND_API_KEY)")
		writeError(w, http.StatusNotImplemented, "email provider is not configured")
		return
	}
	emailFrom := strings.TrimSpace(s.cfg.UserInvitationEmailFrom)
	if emailFrom == "" {
		emailFrom = "no-reply@mistypass.local"
	}

	// Build the report PDF and attach it.
	now := time.Now().UTC()
	periodEnd := now.Truncate(time.Second)
	periodStart := periodEnd.Add(-24 * time.Hour)

	reportType := schedule.ReportType
	data, err := s.buildReportData(reportType, schedule.TenantID, "", periodStart, periodEnd)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to build report data: "+err.Error())
		return
	}
	meta := pdfgen.ReportMeta{
		TenantName:  s.resolveExportTenantName(schedule.TenantID),
		PlaceName:   "All Locations",
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		GeneratedAt: now,
	}
	pdfBytes, err := s.pdfRenderer.RenderPDF(s.gotenbergClient, reportType, meta, data)
	if err != nil {
		s.logger.Error("pdf render failed for schedule", "error", err, "schedule_id", scheduleID)
		writeError(w, http.StatusBadGateway, "PDF rendering failed: "+err.Error())
		return
	}

	filename := pdfgen.FormatPDFFilename(reportType, periodStart, periodEnd)
	attachments := []resendAttachment{{
		Filename: filename,
		Content:  base64.StdEncoding.EncodeToString(pdfBytes),
	}}

	subject := fmt.Sprintf("[MistyPass] %s — %s (%s to %s)",
		schedule.Name, meta.PlaceName,
		periodStart.Format("Jan 2"), periodEnd.Format("Jan 2, 2006"))
	htmlBody := "<p>Please find your scheduled report attached.</p>"

	resendTimeout := s.cfg.UserInvitationResendTimeout
	if resendTimeout < time.Second {
		resendTimeout = 5 * time.Second
	}
	err = sendReportViaResend(r.Context(), resendEndpoint, resendAPIKey, emailFrom, schedule.Recipients, subject, htmlBody, attachments, resendTimeout)
	if err != nil {
		s.logger.Error("failed to send report email", "error", err, "schedule_id", scheduleID)
		writeError(w, http.StatusBadGateway, "failed to send report email: "+err.Error())
		return
	}

	// Update last_sent_at.
	s.reportScheduleMu.Lock()
	if current, ok := s.reportSchedules[scheduleID]; ok {
		current.LastSentAt = now.Truncate(time.Second).Format(time.RFC3339)
		current.UpdatedAt = now.Truncate(time.Second).Format(time.RFC3339)
		s.reportSchedules[scheduleID] = current
		schedule = current
		s.persistReportSchedulesLocked()
	}
	s.reportScheduleMu.Unlock()

	s.appendAuditLog(r, tenantID, "report_schedule_sent", fmt.Sprintf("schedule_id=%s,report_type=%s,recipients=%d", schedule.ID, schedule.ReportType, len(schedule.Recipients)), "report_schedule")
	writeJSON(w, http.StatusOK, schedule)
}

// sendReportViaResend sends an HTML email through the Resend API.
func sendReportViaResend(ctx context.Context, endpoint, apiKey, from string, to []string, subject, html string, attachments []resendAttachment, timeout time.Duration) error {
	payload := map[string]any{
		"from":    from,
		"to":      append([]string(nil), to...),
		"subject": subject,
		"html":    html,
	}
	if len(attachments) > 0 {
		payload["attachments"] = attachments
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal email payload: %w", err)
	}

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("resend api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// buildReportEmailHTML generates a simple HTML email body for a report schedule.
func buildReportEmailHTML(schedule reportSchedule, tenantID string, periodStart, periodEnd time.Time) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"></head><body style="font-family:sans-serif;max-width:600px;margin:0 auto;padding:20px">`)
	sb.WriteString(`<h2 style="color:#1a1a1a;border-bottom:2px solid #e0e0e0;padding-bottom:8px">`)
	sb.WriteString(htmlEscape(schedule.Name))
	sb.WriteString(`</h2>`)
	sb.WriteString(`<table style="width:100%;border-collapse:collapse;margin:16px 0">`)
	writeReportEmailRow(&sb, "Report type", schedule.ReportType)
	writeReportEmailRow(&sb, "Frequency", schedule.Frequency)
	writeReportEmailRow(&sb, "Format", schedule.Format)
	writeReportEmailRow(&sb, "Tenant", tenantID)
	writeReportEmailRow(&sb, "Period", periodStart.Format("2006-01-02 15:04")+" to "+periodEnd.Format("2006-01-02 15:04")+" UTC")
	writeReportEmailRow(&sb, "Recipients", strings.Join(schedule.Recipients, ", "))
	writeReportEmailRow(&sb, "Generated at", periodEnd.Format(time.RFC3339))
	sb.WriteString(`</table>`)
	sb.WriteString(`<p style="color:#666;font-size:13px;margin-top:24px">This report was generated automatically by MistyPass. `)
	sb.WriteString(`Log in to the admin panel to view the full report or adjust this schedule.</p>`)
	sb.WriteString(`</body></html>`)
	return sb.String()
}

func writeReportEmailRow(sb *strings.Builder, label, value string) {
	sb.WriteString(`<tr><td style="padding:6px 12px 6px 0;font-weight:600;color:#555;white-space:nowrap;vertical-align:top">`)
	sb.WriteString(htmlEscape(label))
	sb.WriteString(`</td><td style="padding:6px 0;color:#1a1a1a">`)
	sb.WriteString(htmlEscape(value))
	sb.WriteString(`</td></tr>`)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// ---------------------------------------------------------------------------
// Background report scheduler
// ---------------------------------------------------------------------------

func (s *server) startReportScheduler(stop <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	s.logger.Info("report scheduler started")

	for {
		select {
		case <-stop:
			s.logger.Info("report scheduler stopped")
			return
		case <-ticker.C:
			s.runScheduledReports()
		}
	}
}

func (s *server) runScheduledReports() {
	if !s.cfg.ReportEmailEnabled {
		return
	}
	now := time.Now().UTC()

	s.reportScheduleMu.RLock()
	var due []reportSchedule
	for _, sched := range s.reportSchedules {
		if !sched.Enabled {
			continue
		}
		if sched.NextRunAt == "" {
			continue
		}
		nextRun, err := time.Parse(time.RFC3339, sched.NextRunAt)
		if err != nil {
			continue
		}
		if now.Before(nextRun) {
			continue
		}
		due = append(due, sched)
	}
	s.reportScheduleMu.RUnlock()

	if len(due) == 0 {
		return
	}

	resendEndpoint := strings.TrimSpace(s.cfg.UserInvitationResendEndpoint)
	if resendEndpoint == "" {
		resendEndpoint = "https://api.resend.com/emails"
	}
	resendAPIKey := strings.TrimSpace(s.cfg.UserInvitationResendAPIKey)
	if resendAPIKey == "" {
		return
	}
	emailFrom := strings.TrimSpace(s.cfg.UserInvitationEmailFrom)
	if emailFrom == "" {
		emailFrom = "no-reply@mistypass.local"
	}
	resendTimeout := s.cfg.UserInvitationResendTimeout
	if resendTimeout < time.Second {
		resendTimeout = 5 * time.Second
	}

	for _, sched := range due {
		s.executeScheduledReport(sched, now, resendEndpoint, resendAPIKey, emailFrom, resendTimeout)
	}
}

func (s *server) executeScheduledReport(sched reportSchedule, now time.Time, resendEndpoint, resendAPIKey, emailFrom string, timeout time.Duration) {
	periodEnd := now.Truncate(time.Second)
	var periodStart time.Time
	switch sched.Frequency {
	case "daily":
		periodStart = periodEnd.Add(-24 * time.Hour)
	case "weekly":
		periodStart = periodEnd.Add(-7 * 24 * time.Hour)
	case "monthly":
		periodStart = periodEnd.AddDate(0, -1, 0)
	case "quarterly":
		periodStart = periodEnd.AddDate(0, -3, 0)
	default:
		periodStart = periodEnd.Add(-7 * 24 * time.Hour)
	}

	data, err := s.buildReportData(sched.ReportType, sched.TenantID, "", periodStart, periodEnd)
	if err != nil {
		s.logger.Error("scheduled report data build failed", "error", err, "schedule_id", sched.ID)
		return
	}

	meta := pdfgen.ReportMeta{
		TenantName:  s.resolveExportTenantName(sched.TenantID),
		PlaceName:   "All Locations",
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		GeneratedAt: now,
	}

	pdfBytes, err := s.pdfRenderer.RenderPDF(s.gotenbergClient, sched.ReportType, meta, data)
	if err != nil {
		s.logger.Error("scheduled report PDF render failed", "error", err, "schedule_id", sched.ID)
		return
	}

	filename := pdfgen.FormatPDFFilename(sched.ReportType, periodStart, periodEnd)
	attachments := []resendAttachment{{
		Filename: filename,
		Content:  base64.StdEncoding.EncodeToString(pdfBytes),
	}}

	subject := fmt.Sprintf("[MistyPass] %s — %s (%s to %s)",
		sched.Name, meta.PlaceName,
		periodStart.Format("Jan 2"), periodEnd.Format("Jan 2, 2006"))
	htmlBody := "<p>Please find your scheduled report attached.</p>"

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err = sendReportViaResend(ctx, resendEndpoint, resendAPIKey, emailFrom, sched.Recipients, subject, htmlBody, attachments, timeout)
	if err != nil {
		s.logger.Error("scheduled report email failed", "error", err, "schedule_id", sched.ID)
		return
	}

	s.reportScheduleMu.Lock()
	if current, ok := s.reportSchedules[sched.ID]; ok {
		current.LastSentAt = now.Format(time.RFC3339)
		current.UpdatedAt = now.Format(time.RFC3339)
		var nextRun time.Time
		switch sched.Frequency {
		case "daily":
			nextRun = now.Add(24 * time.Hour)
		case "weekly":
			nextRun = now.Add(7 * 24 * time.Hour)
		case "monthly":
			nextRun = now.AddDate(0, 1, 0)
		case "quarterly":
			nextRun = now.AddDate(0, 3, 0)
		default:
			nextRun = now.Add(7 * 24 * time.Hour)
		}
		current.NextRunAt = nextRun.Format(time.RFC3339)
		s.reportSchedules[sched.ID] = current
		s.persistReportSchedulesLocked()
	}
	s.reportScheduleMu.Unlock()

	s.logger.Info("scheduled report sent", "schedule_id", sched.ID, "recipients", len(sched.Recipients))
}
