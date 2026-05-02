package httpx

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/alarm"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/event"
)

const (
	referenceReportAccessActivityID = "report_access_activity"
	referenceReportDoorAlarmID      = "report_door_alarm_summary"
	referenceReportAuditActivityID  = "report_audit_activity"
)

type referenceReportMetric struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value any    `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

type referenceReport struct {
	ID           string                  `json:"id"`
	ResourceType string                  `json:"resource_type"`
	TenantID     string                  `json:"tenant_id"`
	Name         string                  `json:"name"`
	Description  string                  `json:"description,omitempty"`
	Category     string                  `json:"category"`
	ReportType   string                  `json:"report_type"`
	Status       string                  `json:"status"`
	Severity     string                  `json:"severity,omitempty"`
	PlaceID      string                  `json:"place_id,omitempty"`
	PeriodStart  string                  `json:"period_start"`
	PeriodEnd    string                  `json:"period_end"`
	GeneratedAt  string                  `json:"generated_at"`
	Format       string                  `json:"format"`
	DownloadURL  string                  `json:"download_url"`
	Metrics      []referenceReportMetric `json:"metrics"`
}

type referenceScheduledReport struct {
	ID           string   `json:"id"`
	ResourceType string   `json:"resource_type"`
	TenantID     string   `json:"tenant_id"`
	ReportID     string   `json:"report_id"`
	Name         string   `json:"name"`
	Status       string   `json:"status"`
	Cadence      string   `json:"cadence"`
	Format       string   `json:"format"`
	PlaceID      string   `json:"place_id,omitempty"`
	Recipients   []string `json:"recipients,omitempty"`
	LastRunAt    string   `json:"last_run_at,omitempty"`
	NextRunAt    string   `json:"next_run_at,omitempty"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type referenceScheduledReportPayload struct {
	TenantID   string   `json:"tenant_id"`
	ReportID   string   `json:"report_id"`
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Cadence    string   `json:"cadence"`
	Format     string   `json:"format"`
	PlaceID    string   `json:"place_id"`
	Recipients []string `json:"recipients"`
}

type referenceScheduledReportMutationRequest struct {
	ScheduledReport *referenceScheduledReportPayload `json:"scheduled_report"`
	TenantID        string                           `json:"tenant_id"`
	ReportID        string                           `json:"report_id"`
	Name            string                           `json:"name"`
	Status          string                           `json:"status"`
	Cadence         string                           `json:"cadence"`
	Format          string                           `json:"format"`
	PlaceID         string                           `json:"place_id"`
	Recipients      []string                         `json:"recipients"`
}

func (request referenceScheduledReportMutationRequest) payload() referenceScheduledReportPayload {
	if request.ScheduledReport != nil {
		return *request.ScheduledReport
	}
	return referenceScheduledReportPayload{
		TenantID:   request.TenantID,
		ReportID:   request.ReportID,
		Name:       request.Name,
		Status:     request.Status,
		Cadence:    request.Cadence,
		Format:     request.Format,
		PlaceID:    request.PlaceID,
		Recipients: request.Recipients,
	}
}

func defaultReferenceScheduledReports(now time.Time) (map[string]referenceScheduledReport, int) {
	timestamp := now.UTC().Truncate(time.Second)
	items := []referenceScheduledReport{
		{
			ID:           "scheduled_report_daily_access_demo",
			ResourceType: "ScheduledReport",
			TenantID:     "tenant_demo_jakarta",
			ReportID:     referenceReportAccessActivityID,
			Name:         "Daily access digest",
			Status:       "active",
			Cadence:      "daily",
			Format:       "csv",
			Recipients:   []string{"security@mistypass.local"},
			LastRunAt:    timestamp.Add(-24 * time.Hour).Format(time.RFC3339),
			NextRunAt:    timestamp.Add(24 * time.Hour).Format(time.RFC3339),
			CreatedAt:    timestamp.Add(-7 * 24 * time.Hour).Format(time.RFC3339),
			UpdatedAt:    timestamp.Add(-24 * time.Hour).Format(time.RFC3339),
		},
	}
	output := make(map[string]referenceScheduledReport, len(items))
	for i := range items {
		output[items[i].ID] = items[i]
	}
	return output, len(items)
}

func (s *server) listReferenceReports(w http.ResponseWriter, r *http.Request) {
	tenantID, placeID, buildingScope, ok := s.referenceReportRequestScope(w, r, r.URL.Query().Get("tenant_id"), r.URL.Query().Get("place_id"))
	if !ok {
		return
	}
	items := s.referenceReports(tenantID, placeID, buildingScope, time.Now().UTC())
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	if category != "" || query != "" {
		filtered := make([]referenceReport, 0, len(items))
		for i := range items {
			if category != "" && !strings.EqualFold(items[i].Category, category) {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(items[i].Name+" "+items[i].Description+" "+items[i].Category), query) {
				continue
			}
			filtered = append(filtered, items[i])
		}
		items = filtered
	}
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) getReferenceReport(w http.ResponseWriter, r *http.Request) {
	tenantID, placeID, buildingScope, ok := s.referenceReportRequestScope(w, r, r.URL.Query().Get("tenant_id"), r.URL.Query().Get("place_id"))
	if !ok {
		return
	}
	reportID := strings.TrimSpace(chi.URLParam(r, "reportID"))
	report, exists := referenceReportByID(s.referenceReports(tenantID, placeID, buildingScope, time.Now().UTC()), reportID)
	if !exists {
		writeError(w, http.StatusNotFound, "report not found")
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *server) downloadReferenceReport(w http.ResponseWriter, r *http.Request) {
	tenantID, placeID, buildingScope, ok := s.referenceReportRequestScope(w, r, r.URL.Query().Get("tenant_id"), r.URL.Query().Get("place_id"))
	if !ok {
		return
	}
	reportID := strings.TrimSpace(chi.URLParam(r, "reportID"))
	if _, exists := referenceReportByID(s.referenceReports(tenantID, placeID, buildingScope, time.Now().UTC()), reportID); !exists {
		writeError(w, http.StatusNotFound, "report not found")
		return
	}
	csvBody, ok := s.referenceReportCSV(reportID, tenantID, placeID, buildingScope)
	if !ok {
		writeError(w, http.StatusNotFound, "report not found")
		return
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "pdf" {
		rows := parseCSVToRows(csvBody)
		pdfBytes := generateSimplePDF("Report: "+reportID, rows)
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", reportID+".pdf"))
		w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
		_, _ = w.Write(pdfBytes)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", reportID+".csv"))
	_, _ = w.Write([]byte(csvBody))
}

func (s *server) listReferenceScheduledReports(w http.ResponseWriter, r *http.Request) {
	tenantID, placeID, buildingScope, ok := s.referenceReportRequestScope(w, r, r.URL.Query().Get("tenant_id"), r.URL.Query().Get("place_id"))
	if !ok {
		return
	}
	items := s.referenceScheduledReports(tenantID, placeID, buildingScope)
	reportID := strings.TrimSpace(r.URL.Query().Get("report_id"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if reportID != "" || status != "" {
		filtered := make([]referenceScheduledReport, 0, len(items))
		for i := range items {
			if reportID != "" && items[i].ReportID != reportID {
				continue
			}
			if status != "" && !strings.EqualFold(items[i].Status, status) {
				continue
			}
			filtered = append(filtered, items[i])
		}
		items = filtered
	}
	writeCollection(w, r, http.StatusOK, items)
}

func (s *server) getReferenceScheduledReport(w http.ResponseWriter, r *http.Request) {
	tenantID, _, buildingScope, ok := s.referenceReportRequestScope(w, r, r.URL.Query().Get("tenant_id"), "")
	if !ok {
		return
	}
	item, exists := s.referenceScheduledReportByID(tenantID, chi.URLParam(r, "scheduledReportID"), buildingScope)
	if !exists {
		writeError(w, http.StatusNotFound, "scheduled report not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *server) createReferenceScheduledReport(w http.ResponseWriter, r *http.Request) {
	var request referenceScheduledReportMutationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, placeID, buildingScope, ok := s.referenceReportRequestScope(w, r, payload.TenantID, payload.PlaceID)
	if !ok {
		return
	}
	if buildingScope != nil && placeID == "" {
		writeError(w, http.StatusBadRequest, "place_id is required for scoped scheduled reports")
		return
	}
	report, exists := referenceReportByID(s.referenceReports(tenantID, placeID, buildingScope, time.Now().UTC()), strings.TrimSpace(payload.ReportID))
	if !exists {
		writeError(w, http.StatusBadRequest, "report_id is invalid")
		return
	}
	recipients := uniqueStrings(payload.Recipients)
	if len(recipients) == 0 {
		writeError(w, http.StatusBadRequest, "recipients are required")
		return
	}
	cadence, ok := normalizeReferenceScheduledReportCadence(payload.Cadence)
	if !ok {
		writeError(w, http.StatusBadRequest, "cadence is invalid")
		return
	}
	status, ok := normalizeReferenceScheduledReportStatus(payload.Status)
	if !ok {
		writeError(w, http.StatusBadRequest, "status is invalid")
		return
	}
	format, ok := normalizeReferenceScheduledReportFormat(payload.Format)
	if !ok {
		writeError(w, http.StatusBadRequest, "format is invalid")
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	s.scheduledReportMu.Lock()
	s.scheduledReportSeq++
	item := referenceScheduledReport{
		ID:           fmt.Sprintf("scheduled_report_%06d", s.scheduledReportSeq),
		ResourceType: "ScheduledReport",
		TenantID:     tenantID,
		ReportID:     report.ID,
		Name:         firstNonEmptyString(payload.Name, report.Name),
		Status:       status,
		Cadence:      cadence,
		Format:       format,
		PlaceID:      placeID,
		Recipients:   recipients,
		NextRunAt:    nextReferenceScheduledReportRun(now, cadence),
		CreatedAt:    now.Format(time.RFC3339),
		UpdatedAt:    now.Format(time.RFC3339),
	}
	s.scheduledReports[item.ID] = item
	s.scheduledReportMu.Unlock()

	s.appendAuditLog(r, tenantID, "reference_scheduled_report_created", fmt.Sprintf("scheduled_report_id=%s,report_id=%s,place_id=%s", item.ID, item.ReportID, item.PlaceID), "reports")
	writeJSON(w, http.StatusCreated, item)
}

func (s *server) updateReferenceScheduledReport(w http.ResponseWriter, r *http.Request) {
	var request referenceScheduledReportMutationRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	payload := request.payload()
	tenantID, _, buildingScope, ok := s.referenceReportRequestScope(w, r, firstNonEmptyString(payload.TenantID, r.URL.Query().Get("tenant_id")), "")
	if !ok {
		return
	}
	scheduledReportID := strings.TrimSpace(chi.URLParam(r, "scheduledReportID"))
	current, exists := s.referenceScheduledReportByID(tenantID, scheduledReportID, buildingScope)
	if !exists {
		writeError(w, http.StatusNotFound, "scheduled report not found")
		return
	}

	next := current
	if nextReportID := strings.TrimSpace(payload.ReportID); nextReportID != "" {
		if _, exists := referenceReportByID(s.referenceReports(tenantID, next.PlaceID, buildingScope, time.Now().UTC()), nextReportID); !exists {
			writeError(w, http.StatusBadRequest, "report_id is invalid")
			return
		}
		next.ReportID = nextReportID
	}
	if nextName := strings.TrimSpace(payload.Name); nextName != "" {
		next.Name = nextName
	}
	if nextPlaceID := strings.TrimSpace(payload.PlaceID); nextPlaceID != "" {
		if !s.requireBuildingScope(w, buildingScope, nextPlaceID) {
			return
		}
		next.PlaceID = nextPlaceID
	}
	if len(payload.Recipients) > 0 {
		next.Recipients = uniqueStrings(payload.Recipients)
	}
	if nextCadence := strings.TrimSpace(payload.Cadence); nextCadence != "" {
		cadence, ok := normalizeReferenceScheduledReportCadence(nextCadence)
		if !ok {
			writeError(w, http.StatusBadRequest, "cadence is invalid")
			return
		}
		next.Cadence = cadence
	}
	if nextStatus := strings.TrimSpace(payload.Status); nextStatus != "" {
		status, ok := normalizeReferenceScheduledReportStatus(nextStatus)
		if !ok {
			writeError(w, http.StatusBadRequest, "status is invalid")
			return
		}
		next.Status = status
	}
	if nextFormat := strings.TrimSpace(payload.Format); nextFormat != "" {
		format, ok := normalizeReferenceScheduledReportFormat(nextFormat)
		if !ok {
			writeError(w, http.StatusBadRequest, "format is invalid")
			return
		}
		next.Format = format
	}
	now := time.Now().UTC().Truncate(time.Second)
	next.UpdatedAt = now.Format(time.RFC3339)
	if next.Status == "active" {
		next.NextRunAt = nextReferenceScheduledReportRun(now, next.Cadence)
	}

	s.scheduledReportMu.Lock()
	s.scheduledReports[next.ID] = next
	s.scheduledReportMu.Unlock()

	s.appendAuditLog(r, tenantID, "reference_scheduled_report_updated", fmt.Sprintf("scheduled_report_id=%s,report_id=%s,status=%s,cadence=%s", next.ID, next.ReportID, next.Status, next.Cadence), "reports")
	writeJSON(w, http.StatusOK, next)
}

func (s *server) deleteReferenceScheduledReport(w http.ResponseWriter, r *http.Request) {
	tenantID, _, buildingScope, ok := s.referenceReportRequestScope(w, r, r.URL.Query().Get("tenant_id"), "")
	if !ok {
		return
	}
	item, exists := s.referenceScheduledReportByID(tenantID, chi.URLParam(r, "scheduledReportID"), buildingScope)
	if !exists {
		writeError(w, http.StatusNotFound, "scheduled report not found")
		return
	}
	s.scheduledReportMu.Lock()
	delete(s.scheduledReports, item.ID)
	s.scheduledReportMu.Unlock()
	s.appendAuditLog(r, tenantID, "reference_scheduled_report_deleted", fmt.Sprintf("scheduled_report_id=%s,report_id=%s,place_id=%s", item.ID, item.ReportID, item.PlaceID), "reports")
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) referenceReportRequestScope(w http.ResponseWriter, r *http.Request, requestedTenantID, requestedPlaceID string) (string, string, map[string]struct{}, bool) {
	tenantID, ok := s.resolveTenantID(w, r, requestedTenantID)
	if !ok {
		return "", "", nil, false
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return "", "", nil, false
	}
	placeID := strings.TrimSpace(requestedPlaceID)
	if placeID != "" {
		if !s.requireBuildingScope(w, buildingScope, placeID) {
			return "", "", nil, false
		}
		buildingScope = map[string]struct{}{placeID: {}}
	}
	return tenantID, placeID, buildingScope, true
}

func (s *server) referenceReports(tenantID, placeID string, buildingScope map[string]struct{}, now time.Time) []referenceReport {
	generatedAt := now.UTC().Truncate(time.Second)
	periodStart := generatedAt.Add(-24 * time.Hour).Format(time.RFC3339)
	periodEnd := generatedAt.Format(time.RFC3339)
	accessEvents, deviceEvents, alarms, auditLogs := s.referenceReportData(tenantID, buildingScope)
	downloadQuery := url.Values{}
	downloadQuery.Set("tenant_id", tenantID)
	if strings.TrimSpace(placeID) != "" {
		downloadQuery.Set("place_id", strings.TrimSpace(placeID))
	}
	downloadSuffix := downloadQuery.Encode()
	deniedAccessCount := referenceAccessDeniedCount(accessEvents)
	openAlarmCount := referenceOpenAlarmCount(alarms)
	highAlarmCount := referenceHighAlarmCount(alarms)
	reports := []referenceReport{
		{
			ID:           referenceReportAccessActivityID,
			ResourceType: "Report",
			TenantID:     tenantID,
			Name:         "Access activity",
			Description:  "Access grants, denials, and device signals for the current scope.",
			Category:     "access",
			ReportType:   "activity",
			Status:       "ready",
			Severity:     referenceReportSeverity(deniedAccessCount > 0, false),
			PlaceID:      placeID,
			PeriodStart:  periodStart,
			PeriodEnd:    periodEnd,
			GeneratedAt:  periodEnd,
			Format:       "csv",
			DownloadURL:  "/api/v1/reports/" + referenceReportAccessActivityID + "/download?" + downloadSuffix,
			Metrics: []referenceReportMetric{
				{Key: "access_events", Label: "Access events", Value: len(accessEvents), Unit: "events"},
				{Key: "granted", Label: "Granted", Value: len(accessEvents) - deniedAccessCount, Unit: "events"},
				{Key: "denied", Label: "Denied", Value: deniedAccessCount, Unit: "events"},
				{Key: "device_events", Label: "Device events", Value: len(deviceEvents), Unit: "events"},
			},
		},
		{
			ID:           referenceReportDoorAlarmID,
			ResourceType: "Report",
			TenantID:     tenantID,
			Name:         "Door alarm summary",
			Description:  "Open alarms, severity mix, and door-level alarm history for the current scope.",
			Category:     "doors",
			ReportType:   "summary",
			Status:       "ready",
			Severity:     referenceReportSeverity(openAlarmCount > 0, highAlarmCount > 0),
			PlaceID:      placeID,
			PeriodStart:  periodStart,
			PeriodEnd:    periodEnd,
			GeneratedAt:  periodEnd,
			Format:       "csv",
			DownloadURL:  "/api/v1/reports/" + referenceReportDoorAlarmID + "/download?" + downloadSuffix,
			Metrics: []referenceReportMetric{
				{Key: "alarms", Label: "Alarms", Value: len(alarms), Unit: "alarms"},
				{Key: "open", Label: "Open", Value: openAlarmCount, Unit: "alarms"},
				{Key: "high_priority", Label: "High priority", Value: highAlarmCount, Unit: "alarms"},
				{Key: "resolved", Label: "Resolved", Value: referenceResolvedAlarmCount(alarms), Unit: "alarms"},
			},
		},
	}
	if buildingScope == nil {
		reports = append(reports, referenceReport{
			ID:           referenceReportAuditActivityID,
			ResourceType: "Report",
			TenantID:     tenantID,
			Name:         "Audit activity",
			Description:  "Administrative audit activity, actors, and source systems.",
			Category:     "audit",
			ReportType:   "activity",
			Status:       "ready",
			Severity:     "info",
			PlaceID:      placeID,
			PeriodStart:  periodStart,
			PeriodEnd:    periodEnd,
			GeneratedAt:  periodEnd,
			Format:       "csv",
			DownloadURL:  "/api/v1/reports/" + referenceReportAuditActivityID + "/download?" + downloadSuffix,
			Metrics: []referenceReportMetric{
				{Key: "audit_logs", Label: "Audit logs", Value: len(auditLogs), Unit: "logs"},
				{Key: "actors", Label: "Actors", Value: referenceAuditActorCount(auditLogs), Unit: "actors"},
				{Key: "api_sources", Label: "API sources", Value: referenceAuditSourceCount(auditLogs), Unit: "sources"},
				{Key: "last_activity", Label: "Last activity", Value: referenceLastAuditActivity(auditLogs)},
			},
		})
	}
	return reports
}

func (s *server) referenceReportData(tenantID string, buildingScope map[string]struct{}) ([]event.AccessEvent, []event.DeviceEvent, []alarm.Alarm, []audit.Log) {
	accessEvents := []event.AccessEvent{}
	if s.eventSvc != nil {
		accessEvents = s.eventSvc.ListAccessEvents(tenantID)
	}
	deviceEvents := []event.DeviceEvent{}
	if s.eventSvc != nil {
		deviceEvents = s.eventSvc.ListDeviceEvents(tenantID)
	}
	alarms := []alarm.Alarm{}
	if s.alarmSvc != nil {
		alarms = s.alarmSvc.List(tenantID)
	}
	auditLogs := []audit.Log{}
	if s.auditSvc != nil {
		auditLogs = s.auditSvc.List(tenantID)
	}
	if buildingScope != nil {
		accessEvents = filterAccessEventsByScope(accessEvents, buildingScope)
		deviceEvents = filterDeviceEventsByScope(deviceEvents, buildingScope)
		alarms = filterAlarmsByScope(alarms, buildingScope)
	}
	return accessEvents, deviceEvents, alarms, auditLogs
}

func (s *server) referenceReportCSV(reportID, tenantID, placeID string, buildingScope map[string]struct{}) (string, bool) {
	accessEvents, deviceEvents, alarms, auditLogs := s.referenceReportData(tenantID, buildingScope)
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	switch reportID {
	case referenceReportAccessActivityID:
		_ = writer.Write([]string{"kind", "id", "type", "result", "place_id", "lock_id", "actor", "created_at"})
		for i := range accessEvents {
			_ = writer.Write([]string{"access", accessEvents[i].ID, accessEvents[i].Type, accessEvents[i].Result, accessEvents[i].BuildingID, accessEvents[i].DoorID, accessEvents[i].Actor, accessEvents[i].At.UTC().Format(time.RFC3339)})
		}
		for i := range deviceEvents {
			_ = writer.Write([]string{"device", deviceEvents[i].ID, deviceEvents[i].Type, deviceEvents[i].Result, deviceEvents[i].BuildingID, "", deviceEvents[i].Detail, deviceEvents[i].At.UTC().Format(time.RFC3339)})
		}
	case referenceReportDoorAlarmID:
		_ = writer.Write([]string{"id", "type", "severity", "status", "place_id", "area_id", "lock_id", "location", "created_at"})
		for i := range alarms {
			_ = writer.Write([]string{alarms[i].ID, alarms[i].Type, alarms[i].Severity, alarms[i].Status, alarms[i].BuildingID, alarms[i].AreaID, alarms[i].DoorID, alarms[i].Location, alarms[i].CreatedAt.UTC().Format(time.RFC3339)})
		}
	case referenceReportAuditActivityID:
		_ = writer.Write([]string{"id", "actor", "role", "action", "target", "source", "at"})
		for i := range auditLogs {
			_ = writer.Write([]string{auditLogs[i].ID, auditLogs[i].Actor, auditLogs[i].Role, auditLogs[i].Action, auditLogs[i].Target, auditLogs[i].Source, auditLogs[i].At.UTC().Format(time.RFC3339)})
		}
	default:
		return "", false
	}
	writer.Flush()
	return buffer.String(), writer.Error() == nil
}

func (s *server) referenceScheduledReports(tenantID, placeID string, buildingScope map[string]struct{}) []referenceScheduledReport {
	s.scheduledReportMu.RLock()
	defer s.scheduledReportMu.RUnlock()
	items := make([]referenceScheduledReport, 0, len(s.scheduledReports))
	for _, item := range s.scheduledReports {
		if strings.TrimSpace(item.TenantID) != tenantID {
			continue
		}
		if placeID != "" && strings.TrimSpace(item.PlaceID) != placeID {
			continue
		}
		if !referenceScheduledReportAllowedByBuildingScope(item, buildingScope) {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items
}

func (s *server) referenceScheduledReportByID(tenantID, scheduledReportID string, buildingScope map[string]struct{}) (referenceScheduledReport, bool) {
	targetID := strings.TrimSpace(scheduledReportID)
	s.scheduledReportMu.RLock()
	item, exists := s.scheduledReports[targetID]
	s.scheduledReportMu.RUnlock()
	if !exists || strings.TrimSpace(item.TenantID) != tenantID || !referenceScheduledReportAllowedByBuildingScope(item, buildingScope) {
		return referenceScheduledReport{}, false
	}
	return item, true
}

func referenceReportByID(items []referenceReport, reportID string) (referenceReport, bool) {
	targetID := strings.TrimSpace(reportID)
	for i := range items {
		if items[i].ID == targetID {
			return items[i], true
		}
	}
	return referenceReport{}, false
}

func referenceScheduledReportAllowedByBuildingScope(item referenceScheduledReport, scope map[string]struct{}) bool {
	if scope == nil {
		return true
	}
	placeID := strings.TrimSpace(item.PlaceID)
	if placeID == "" {
		return false
	}
	_, exists := scope[placeID]
	return exists
}

func normalizeReferenceScheduledReportCadence(value string) (string, bool) {
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

func normalizeReferenceScheduledReportStatus(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "active":
		return "active", true
	case "paused":
		return "paused", true
	default:
		return "", false
	}
}

func normalizeReferenceScheduledReportFormat(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "csv":
		return "csv", true
	case "json":
		return "json", true
	default:
		return "", false
	}
}

func nextReferenceScheduledReportRun(now time.Time, cadence string) string {
	switch cadence {
	case "weekly":
		return now.UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	case "monthly":
		return now.UTC().AddDate(0, 1, 0).Format(time.RFC3339)
	default:
		return now.UTC().Add(24 * time.Hour).Format(time.RFC3339)
	}
}

func referenceAccessDeniedCount(items []event.AccessEvent) int {
	count := 0
	for i := range items {
		if strings.EqualFold(items[i].Result, "denied") || strings.Contains(strings.ToLower(items[i].Type), "denied") {
			count++
		}
	}
	return count
}

func referenceOpenAlarmCount(items []alarm.Alarm) int {
	count := 0
	for i := range items {
		switch strings.ToLower(strings.TrimSpace(items[i].Status)) {
		case "resolved", "false_positive":
			continue
		default:
			count++
		}
	}
	return count
}

func referenceResolvedAlarmCount(items []alarm.Alarm) int {
	count := 0
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].Status), "resolved") {
			count++
		}
	}
	return count
}

func referenceHighAlarmCount(items []alarm.Alarm) int {
	count := 0
	for i := range items {
		switch strings.ToLower(strings.TrimSpace(items[i].Severity)) {
		case "critical", "high":
			count++
		}
	}
	return count
}

func referenceAuditActorCount(items []audit.Log) int {
	actors := map[string]struct{}{}
	for i := range items {
		if actor := strings.TrimSpace(items[i].Actor); actor != "" {
			actors[actor] = struct{}{}
		}
	}
	return len(actors)
}

func referenceAuditSourceCount(items []audit.Log) int {
	sources := map[string]struct{}{}
	for i := range items {
		if source := strings.TrimSpace(items[i].Source); source != "" {
			sources[source] = struct{}{}
		}
	}
	return len(sources)
}

func referenceLastAuditActivity(items []audit.Log) string {
	var latest time.Time
	for i := range items {
		if items[i].At.After(latest) {
			latest = items[i].At
		}
	}
	if latest.IsZero() {
		return ""
	}
	return latest.UTC().Format(time.RFC3339)
}

func referenceReportSeverity(hasWarning, hasDanger bool) string {
	switch {
	case hasDanger:
		return "danger"
	case hasWarning:
		return "warning"
	default:
		return "info"
	}
}

func parseCSVToRows(csvBody string) [][]string {
	reader := csv.NewReader(strings.NewReader(csvBody))
	rows, err := reader.ReadAll()
	if err != nil {
		return [][]string{{"Error parsing CSV"}}
	}
	return rows
}
