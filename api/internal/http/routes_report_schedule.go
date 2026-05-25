package httpx

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/mail"
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

type reportScheduleProviderStatus struct {
	Provider       string   `json:"provider"`
	Enabled        bool     `json:"enabled"`
	Configured     bool     `json:"configured"`
	Ready          bool     `json:"ready"`
	Endpoint       string   `json:"endpoint"`
	From           string   `json:"from"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	Missing        []string `json:"missing,omitempty"`
	Message        string   `json:"message"`
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

func (s *server) getReportScheduleProviderStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id")); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.reportMailProviderStatus())
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
		writeError(w, http.StatusBadRequest, "report_type must be one of: weekly_analytics, events, unlock_stats, user_presence, incidents, hardware")
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
			writeError(w, http.StatusBadRequest, "report_type must be one of: weekly_analytics, events, unlock_stats, user_presence, incidents, hardware")
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
	case "weekly_analytics", "access_summary":
		return "weekly_analytics", true
	case "events", "visitor_log":
		return "events", true
	case "unlock_stats", "door_activity", "door_usage":
		return "unlock_stats", true
	case "user_presence", "compliance":
		return "user_presence", true
	case "incidents", "alarm_metrics", "alarm_history":
		return "incidents", true
	case "hardware":
		return "hardware", true
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
// Send report schedule — deliver report via email provider.
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

	provider, err := s.reportMailProvider()
	if err != nil {
		s.logger.Warn("report email provider is not configured", "error", err)
		writeError(w, http.StatusNotImplemented, "email provider is not configured")
		return
	}

	// Build the report PDF and attach it.
	now := time.Now().UTC()
	periodEnd := now.Truncate(time.Second)
	periodStart := periodEnd.Add(-24 * time.Hour)

	reportType, ok := normalizeReportScheduleReportType(schedule.ReportType)
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown report type: "+schedule.ReportType)
		return
	}
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
	attachments := []mail.Attachment{{
		Filename: filename,
		Content:  base64.StdEncoding.EncodeToString(pdfBytes),
	}}

	subject := fmt.Sprintf("[Mistyislet] %s - %s (%s to %s)",
		schedule.Name, meta.PlaceName,
		periodStart.Format("Jan 2"), periodEnd.Format("Jan 2, 2006"))
	htmlBody := buildReportEmailHTML(schedule.Name, reportType, meta, filename)

	sendCtx, cancel := context.WithTimeout(context.Background(), s.reportMailTimeout())
	defer cancel()
	receipt, err := provider.Send(sendCtx, mail.Message{
		TenantID:       schedule.TenantID,
		To:             schedule.Recipients,
		IdempotencyKey: reportScheduleEmailIdempotencyKey(schedule.ID, periodEnd),
		Subject:        subject,
		HTML:           htmlBody,
		Attachments:    attachments,
		Metadata: map[string]string{
			"report_schedule_id": schedule.ID,
			"report_type":        reportType,
		},
	})
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

	s.appendAuditLog(r, tenantID, "report_schedule_sent", fmt.Sprintf("schedule_id=%s,report_type=%s,recipients=%d,provider=%s,provider_delivery_id=%s", schedule.ID, schedule.ReportType, len(schedule.Recipients), receipt.Provider, receipt.ProviderDeliveryID), "report_schedule")
	writeJSON(w, http.StatusOK, schedule)
}

// buildReportEmailHTML generates a branded HTML email body for a report schedule.
func buildReportEmailHTML(scheduleName, reportType string, meta pdfgen.ReportMeta, filename string) string {
	title := strings.TrimSpace(scheduleName)
	if title == "" {
		title = pdfgen.ReportTypeLabel(reportType)
	}
	period := meta.PeriodStart.Format("Jan 2, 2006") + " - " + meta.PeriodEnd.Format("Jan 2, 2006")

	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"></head>`)
	sb.WriteString(`<body style="margin:0;background:#f7f2e8;color:#141510;font-family:Inter,-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif">`)
	sb.WriteString(`<div style="max-width:640px;margin:0 auto;padding:28px 20px">`)
	sb.WriteString(`<div style="border:1px solid #2f302b;border-radius:8px;overflow:hidden;background:#fffdf7">`)
	sb.WriteString(`<div style="background:linear-gradient(145deg,#20231f,#070806);padding:24px;color:#f5f0e6">`)
	sb.WriteString(`<div style="font-size:13px;font-weight:700;letter-spacing:.04em">Mistyislet</div>`)
	sb.WriteString(`<div style="margin-top:22px;color:#62b7a8;font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase">`)
	sb.WriteString(html.EscapeString(pdfgen.ReportTypeLabel(reportType)))
	sb.WriteString(`</div>`)
	sb.WriteString(`<h1 style="margin:8px 0 0;font-size:28px;line-height:1.08;font-weight:400;color:#f5f0e6">`)
	sb.WriteString(html.EscapeString(title))
	sb.WriteString(`</h1>`)
	sb.WriteString(`<p style="margin:10px 0 0;color:#beb8aa;font-size:14px;line-height:1.5">`)
	sb.WriteString(html.EscapeString(meta.TenantName))
	sb.WriteString(` - `)
	sb.WriteString(html.EscapeString(meta.PlaceName))
	sb.WriteString(`</p></div>`)
	sb.WriteString(`<div style="padding:22px 24px">`)
	sb.WriteString(`<p style="margin:0 0 18px;color:#7c7568;font-size:14px;line-height:1.6">Your scheduled report is attached as a PDF. The document uses the same period and tenant scope shown below.</p>`)
	sb.WriteString(`<table style="width:100%;border-collapse:collapse;font-size:14px">`)
	writeReportEmailRow(&sb, "Period", period)
	writeReportEmailRow(&sb, "Report type", pdfgen.ReportTypeLabel(reportType))
	writeReportEmailRow(&sb, "Place", meta.PlaceName)
	writeReportEmailRow(&sb, "Attachment", filename)
	writeReportEmailRow(&sb, "Generated at", meta.GeneratedAt.Format(time.RFC3339))
	sb.WriteString(`</table>`)
	sb.WriteString(`<p style="margin:22px 0 0;color:#7c7568;font-size:12px;line-height:1.5">This report was generated automatically by Mistyislet. Open the admin console to adjust schedules, recipients, or report scope.</p>`)
	sb.WriteString(`</div></div></div></body></html>`)
	return sb.String()
}

func writeReportEmailRow(sb *strings.Builder, label, value string) {
	sb.WriteString(`<tr><td style="padding:8px 14px 8px 0;border-top:1px solid #e8dfd1;font-weight:700;color:#141510;white-space:nowrap;vertical-align:top">`)
	sb.WriteString(html.EscapeString(label))
	sb.WriteString(`</td><td style="padding:8px 0;border-top:1px solid #e8dfd1;color:#7c7568">`)
	sb.WriteString(html.EscapeString(value))
	sb.WriteString(`</td></tr>`)
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

	provider, err := s.reportMailProvider()
	if err != nil {
		s.logger.Warn("scheduled report email provider is not configured", "error", err)
		return
	}

	for _, sched := range due {
		s.executeScheduledReport(sched, now, provider)
	}
}

func (s *server) executeScheduledReport(sched reportSchedule, now time.Time, provider mail.Provider) {
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

	reportType, ok := normalizeReportScheduleReportType(sched.ReportType)
	if !ok {
		s.logger.Error("scheduled report has unknown type", "schedule_id", sched.ID, "report_type", sched.ReportType)
		return
	}

	data, err := s.buildReportData(reportType, sched.TenantID, "", periodStart, periodEnd)
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

	pdfBytes, err := s.pdfRenderer.RenderPDF(s.gotenbergClient, reportType, meta, data)
	if err != nil {
		s.logger.Error("scheduled report PDF render failed", "error", err, "schedule_id", sched.ID)
		return
	}

	filename := pdfgen.FormatPDFFilename(reportType, periodStart, periodEnd)
	attachments := []mail.Attachment{{
		Filename: filename,
		Content:  base64.StdEncoding.EncodeToString(pdfBytes),
	}}

	subject := fmt.Sprintf("[Mistyislet] %s - %s (%s to %s)",
		sched.Name, meta.PlaceName,
		periodStart.Format("Jan 2"), periodEnd.Format("Jan 2, 2006"))
	htmlBody := buildReportEmailHTML(sched.Name, reportType, meta, filename)

	ctx, cancel := context.WithTimeout(context.Background(), s.reportMailTimeout())
	defer cancel()
	receipt, err := provider.Send(ctx, mail.Message{
		TenantID:       sched.TenantID,
		To:             sched.Recipients,
		IdempotencyKey: reportScheduleEmailIdempotencyKey(sched.ID, periodEnd),
		Subject:        subject,
		HTML:           htmlBody,
		Attachments:    attachments,
		Metadata: map[string]string{
			"report_schedule_id": sched.ID,
			"report_type":        reportType,
		},
	})
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

	s.logger.Info("scheduled report sent", "schedule_id", sched.ID, "recipients", len(sched.Recipients), "provider", receipt.Provider, "provider_delivery_id", receipt.ProviderDeliveryID)
}

func (s *server) reportMailProvider() (mail.Provider, error) {
	switch s.reportMailProviderName() {
	case "cloudflare":
		return mail.NewCloudflareProvider(mail.CloudflareOptions{
			Endpoint:  strings.TrimSpace(s.cfg.CloudflareEmailEndpoint),
			AccountID: strings.TrimSpace(s.cfg.CloudflareEmailAccountID),
			APIToken:  strings.TrimSpace(s.cfg.CloudflareEmailAPIToken),
			From:      s.reportMailFrom(),
			Timeout:   s.reportMailTimeout(),
		})
	default:
		return mail.NewResendProvider(mail.ResendOptions{
			Endpoint: strings.TrimSpace(s.cfg.UserInvitationResendEndpoint),
			APIKey:   strings.TrimSpace(s.cfg.UserInvitationResendAPIKey),
			From:     s.reportMailFrom(),
			Timeout:  s.reportMailTimeout(),
		})
	}
}

func (s *server) reportMailProviderStatus() reportScheduleProviderStatus {
	provider := s.reportMailProviderName()
	from := s.reportMailFrom()
	timeout := s.reportMailTimeout()
	missing := make([]string, 0, 4)
	if !s.cfg.ReportEmailEnabled {
		missing = append(missing, "REPORT_EMAIL_ENABLED")
	}
	if from == "" {
		missing = append(missing, "REPORT_EMAIL_FROM")
	}

	endpoint := ""
	configured := false
	switch provider {
	case "cloudflare":
		endpoint = firstNonEmptyString(strings.TrimSpace(s.cfg.CloudflareEmailEndpoint), mail.CloudflareEmailEndpointDefault)
		apiToken := strings.TrimSpace(s.cfg.CloudflareEmailAPIToken)
		accountID := strings.TrimSpace(s.cfg.CloudflareEmailAccountID)
		requiresAccountID := strings.Contains(endpoint, "{account_id}")
		if apiToken == "" {
			missing = append(missing, "CLOUDFLARE_EMAIL_API_TOKEN")
		}
		if requiresAccountID && accountID == "" {
			missing = append(missing, "CLOUDFLARE_ACCOUNT_ID")
		}
		if requiresAccountID && accountID != "" {
			endpoint = strings.ReplaceAll(endpoint, "{account_id}", accountID)
		}
		configured = apiToken != "" && from != "" && (!requiresAccountID || accountID != "")
	default:
		endpoint = firstNonEmptyString(strings.TrimSpace(s.cfg.UserInvitationResendEndpoint), mail.ResendEndpointDefault)
		apiKey := strings.TrimSpace(s.cfg.UserInvitationResendAPIKey)
		if apiKey == "" {
			missing = append(missing, "USER_INVITATION_RESEND_API_KEY")
		}
		configured = apiKey != "" && from != ""
	}

	ready := s.cfg.ReportEmailEnabled && configured
	message := "Report email provider is ready."
	if !s.cfg.ReportEmailEnabled {
		message = "Report email delivery is disabled."
	} else if !configured {
		message = "Report email provider is missing required configuration."
	}

	return reportScheduleProviderStatus{
		Provider:       provider,
		Enabled:        s.cfg.ReportEmailEnabled,
		Configured:     configured,
		Ready:          ready,
		Endpoint:       endpoint,
		From:           from,
		TimeoutSeconds: int(timeout.Seconds()),
		Missing:        missing,
		Message:        message,
	}
}

func (s *server) reportMailProviderName() string {
	provider := strings.ToLower(strings.TrimSpace(s.cfg.MailProvider))
	if provider == "" {
		provider = strings.ToLower(strings.TrimSpace(s.cfg.UserInvitationEmailProvider))
	}
	switch provider {
	case "cloudflare":
		return "cloudflare"
	case "resend", "spaceemail":
		return "resend"
	default:
		return "resend"
	}
}

func (s *server) reportMailFrom() string {
	return firstNonEmptyString(
		strings.TrimSpace(s.cfg.ReportEmailFrom),
		strings.TrimSpace(s.cfg.UserInvitationEmailFrom),
		"no-reply@mistypass.local",
	)
}

func (s *server) reportMailTimeout() time.Duration {
	if s.reportMailProviderName() == "cloudflare" {
		timeout := s.cfg.CloudflareEmailTimeout
		if timeout < time.Second {
			return 15 * time.Second
		}
		return timeout
	}
	timeout := s.cfg.UserInvitationResendTimeout
	if timeout < time.Second {
		timeout = 5 * time.Second
	}
	return timeout
}

func reportScheduleEmailIdempotencyKey(scheduleID string, periodEnd time.Time) string {
	return "report_schedule:" + strings.TrimSpace(scheduleID) + ":" + periodEnd.UTC().Format(time.RFC3339)
}
