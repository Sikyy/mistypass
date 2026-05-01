package httpx

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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
