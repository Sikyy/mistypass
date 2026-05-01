package httpx

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// GET /api/v1/analytics/access-summary
// ---------------------------------------------------------------------------

type accessSummaryByDoor struct {
	DoorID   string `json:"door_id"`
	DoorName string `json:"door_name"`
	Count    int    `json:"count"`
}

type accessSummaryByDay struct {
	Date    string `json:"date"`
	Granted int    `json:"granted"`
	Denied  int    `json:"denied"`
}

type accessSummaryResponse struct {
	Period      map[string]string     `json:"period"`
	TotalEvents int                   `json:"total_events"`
	ByResult    map[string]int        `json:"by_result"`
	ByDoor      []accessSummaryByDoor `json:"by_door"`
	ByDay       []accessSummaryByDay  `json:"by_day"`
	PeakHour    int                   `json:"peak_hour"`
}

func (s *server) getAccessSummary(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}

	startStr := strings.TrimSpace(r.URL.Query().Get("start"))
	endStr := strings.TrimSpace(r.URL.Query().Get("end"))
	if startStr == "" || endStr == "" {
		writeError(w, http.StatusBadRequest, "start and end query parameters are required (RFC3339)")
		return
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start parameter: must be RFC3339")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end parameter: must be RFC3339")
		return
	}

	filterBuildingID := strings.TrimSpace(r.URL.Query().Get("building_id"))

	items := s.eventSvc.ListAccessEvents(tenantID)
	if buildingScope != nil {
		items = filterAccessEventsByScope(items, buildingScope)
	}

	byResult := map[string]int{"granted": 0, "denied": 0}
	doorCounts := map[string]int{}
	dayCounts := map[string]*accessSummaryByDay{}
	hourCounts := [24]int{}

	for i := range items {
		ev := items[i]
		if ev.At.Before(start) || !ev.At.Before(end) {
			continue
		}
		if filterBuildingID != "" && ev.BuildingID != filterBuildingID {
			continue
		}

		switch {
		case strings.EqualFold(ev.Result, "success"), strings.EqualFold(ev.Result, "accepted"):
			byResult["granted"]++
		default:
			byResult["denied"]++
		}

		doorCounts[ev.DoorID]++

		dayKey := ev.At.UTC().Format("2006-01-02")
		dayEntry, exists := dayCounts[dayKey]
		if !exists {
			dayEntry = &accessSummaryByDay{Date: dayKey}
			dayCounts[dayKey] = dayEntry
		}
		switch {
		case strings.EqualFold(ev.Result, "success"), strings.EqualFold(ev.Result, "accepted"):
			dayEntry.Granted++
		default:
			dayEntry.Denied++
		}

		hourCounts[ev.At.UTC().Hour()]++
	}

	totalEvents := byResult["granted"] + byResult["denied"]

	// Resolve door names via space service.
	byDoor := make([]accessSummaryByDoor, 0, len(doorCounts))
	for doorID, count := range doorCounts {
		doorName := doorID
		if s.spaceSvc != nil {
			if door, err := s.spaceSvc.GetDoor(tenantID, doorID); err == nil {
				doorName = door.Name
			}
		}
		byDoor = append(byDoor, accessSummaryByDoor{
			DoorID:   doorID,
			DoorName: doorName,
			Count:    count,
		})
	}

	byDay := make([]accessSummaryByDay, 0, len(dayCounts))
	for _, entry := range dayCounts {
		byDay = append(byDay, *entry)
	}

	peakHour := 0
	peakCount := 0
	for h := 0; h < 24; h++ {
		if hourCounts[h] > peakCount {
			peakCount = hourCounts[h]
			peakHour = h
		}
	}

	writeJSON(w, http.StatusOK, accessSummaryResponse{
		Period:      map[string]string{"start": start.UTC().Format(time.RFC3339), "end": end.UTC().Format(time.RFC3339)},
		TotalEvents: totalEvents,
		ByResult:    byResult,
		ByDoor:      byDoor,
		ByDay:       byDay,
		PeakHour:    peakHour,
	})
}

// ---------------------------------------------------------------------------
// GET /api/v1/analytics/door-activity
// ---------------------------------------------------------------------------

type doorActivityEntry struct {
	DoorID             string `json:"door_id"`
	TotalAccess        int    `json:"total_access"`
	UniqueUsers        int    `json:"unique_users"`
	HourlyDistribution [24]int `json:"hourly_distribution"`
}

type doorActivityResponse struct {
	Doors []doorActivityEntry `json:"doors"`
}

func (s *server) getDoorActivity(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}

	filterDoorID := strings.TrimSpace(r.URL.Query().Get("door_id"))
	filterBuildingID := strings.TrimSpace(r.URL.Query().Get("building_id"))

	if filterDoorID == "" && filterBuildingID == "" {
		writeError(w, http.StatusBadRequest, "door_id or building_id query parameter is required")
		return
	}

	daysStr := strings.TrimSpace(r.URL.Query().Get("days"))
	days := 7
	if daysStr != "" {
		parsed, err := strconv.Atoi(daysStr)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "days must be a positive integer")
			return
		}
		days = parsed
	}
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)

	items := s.eventSvc.ListAccessEvents(tenantID)
	if buildingScope != nil {
		items = filterAccessEventsByScope(items, buildingScope)
	}

	type doorAgg struct {
		totalAccess        int
		uniqueUsers        map[string]struct{}
		hourlyDistribution [24]int
	}
	doorMap := map[string]*doorAgg{}

	for i := range items {
		ev := items[i]
		if ev.At.Before(cutoff) {
			continue
		}
		if filterDoorID != "" && ev.DoorID != filterDoorID {
			continue
		}
		if filterBuildingID != "" && ev.BuildingID != filterBuildingID {
			continue
		}

		agg, exists := doorMap[ev.DoorID]
		if !exists {
			agg = &doorAgg{uniqueUsers: map[string]struct{}{}}
			doorMap[ev.DoorID] = agg
		}
		agg.totalAccess++
		if ev.Actor != "" {
			agg.uniqueUsers[ev.Actor] = struct{}{}
		}
		agg.hourlyDistribution[ev.At.UTC().Hour()]++
	}

	doors := make([]doorActivityEntry, 0, len(doorMap))
	for doorID, agg := range doorMap {
		doors = append(doors, doorActivityEntry{
			DoorID:             doorID,
			TotalAccess:        agg.totalAccess,
			UniqueUsers:        len(agg.uniqueUsers),
			HourlyDistribution: agg.hourlyDistribution,
		})
	}

	writeJSON(w, http.StatusOK, doorActivityResponse{
		Doors: doors,
	})
}

// ---------------------------------------------------------------------------
// GET /api/v1/analytics/alarm-metrics
// ---------------------------------------------------------------------------

type alarmMetricsResponse struct {
	Total                   int            `json:"total"`
	BySeverity              map[string]int `json:"by_severity"`
	ByStatus                map[string]int `json:"by_status"`
	MeanResolutionMinutes   float64        `json:"mean_resolution_minutes"`
}

func (s *server) getAlarmMetrics(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}

	startStr := strings.TrimSpace(r.URL.Query().Get("start"))
	endStr := strings.TrimSpace(r.URL.Query().Get("end"))
	if startStr == "" || endStr == "" {
		writeError(w, http.StatusBadRequest, "start and end query parameters are required (RFC3339)")
		return
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start parameter: must be RFC3339")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end parameter: must be RFC3339")
		return
	}

	items := s.alarmSvc.List(tenantID)
	if buildingScope != nil {
		items = filterAlarmsByScope(items, buildingScope)
	}

	bySeverity := map[string]int{}
	byStatus := map[string]int{}
	resolvedCount := 0
	var totalResolutionMinutes float64

	now := time.Now().UTC()

	for i := range items {
		alm := items[i]
		if alm.CreatedAt.Before(start) || !alm.CreatedAt.Before(end) {
			continue
		}

		severity := strings.ToLower(strings.TrimSpace(alm.Severity))
		if severity == "" {
			severity = "unknown"
		}
		bySeverity[severity]++

		status := strings.ToLower(strings.TrimSpace(alm.Status))
		if status == "" {
			status = "unknown"
		}
		byStatus[status]++

		if status == "resolved" || status == "false_positive" {
			resolvedCount++
			totalResolutionMinutes += now.Sub(alm.CreatedAt).Minutes()
		}
	}

	total := 0
	for _, count := range bySeverity {
		total += count
	}

	meanResolutionMinutes := float64(0)
	if resolvedCount > 0 {
		meanResolutionMinutes = totalResolutionMinutes / float64(resolvedCount)
	}

	writeJSON(w, http.StatusOK, alarmMetricsResponse{
		Total:                 total,
		BySeverity:            bySeverity,
		ByStatus:              byStatus,
		MeanResolutionMinutes: meanResolutionMinutes,
	})
}

// ---------------------------------------------------------------------------
// GET /api/v1/analytics/export?type=...&format=...&tenant_id=...&start=...&end=...
// ---------------------------------------------------------------------------

func (s *server) exportAnalytics(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	reportType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if reportType == "" {
		writeError(w, http.StatusBadRequest, "type query parameter is required (access_summary, door_activity, alarm_metrics)")
		return
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "pdf" && format != "csv" {
		writeError(w, http.StatusBadRequest, "format must be one of: json, pdf, csv")
		return
	}

	startStr := strings.TrimSpace(r.URL.Query().Get("start"))
	endStr := strings.TrimSpace(r.URL.Query().Get("end"))
	if startStr == "" || endStr == "" {
		writeError(w, http.StatusBadRequest, "start and end query parameters are required (RFC3339)")
		return
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start parameter: must be RFC3339")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end parameter: must be RFC3339")
		return
	}

	var title string
	var rows [][]string

	switch reportType {
	case "access_summary":
		title, rows = s.exportAccessSummaryRows(tenantID, start, end)
	case "door_activity":
		title, rows = s.exportDoorActivityRows(tenantID, start, end)
	case "alarm_metrics":
		title, rows = s.exportAlarmMetricsRows(tenantID, start, end)
	default:
		writeError(w, http.StatusBadRequest, "type must be one of: access_summary, door_activity, alarm_metrics")
		return
	}

	switch format {
	case "pdf":
		pdfBytes := generateSimplePDF(title, rows)
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", reportType+".pdf"))
		w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
		_, _ = w.Write(pdfBytes)
	case "csv":
		var sb strings.Builder
		for _, row := range rows {
			sb.WriteString(strings.Join(row, ","))
			sb.WriteString("\n")
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", reportType+".csv"))
		_, _ = w.Write([]byte(sb.String()))
	default:
		writeJSON(w, http.StatusOK, map[string]any{
			"title": title,
			"rows":  rows,
		})
	}
}

func (s *server) exportAccessSummaryRows(tenantID string, start, end time.Time) (string, [][]string) {
	items := s.eventSvc.ListAccessEvents(tenantID)
	granted, denied := 0, 0
	for i := range items {
		if items[i].At.Before(start) || !items[i].At.Before(end) {
			continue
		}
		switch {
		case strings.EqualFold(items[i].Result, "success"), strings.EqualFold(items[i].Result, "accepted"):
			granted++
		default:
			denied++
		}
	}
	title := fmt.Sprintf("Access Summary (%s to %s)", start.UTC().Format("2006-01-02"), end.UTC().Format("2006-01-02"))
	rows := [][]string{
		{"Metric", "Value"},
		{"Total Events", strconv.Itoa(granted + denied)},
		{"Granted", strconv.Itoa(granted)},
		{"Denied", strconv.Itoa(denied)},
		{"Period Start", start.UTC().Format(time.RFC3339)},
		{"Period End", end.UTC().Format(time.RFC3339)},
	}
	return title, rows
}

func (s *server) exportDoorActivityRows(tenantID string, start, end time.Time) (string, [][]string) {
	items := s.eventSvc.ListAccessEvents(tenantID)
	doorCounts := map[string]int{}
	for i := range items {
		if items[i].At.Before(start) || !items[i].At.Before(end) {
			continue
		}
		doorCounts[items[i].DoorID]++
	}
	title := fmt.Sprintf("Door Activity (%s to %s)", start.UTC().Format("2006-01-02"), end.UTC().Format("2006-01-02"))
	rows := [][]string{{"Door ID", "Access Count"}}
	for doorID, count := range doorCounts {
		rows = append(rows, []string{doorID, strconv.Itoa(count)})
	}
	return title, rows
}

func (s *server) exportAlarmMetricsRows(tenantID string, start, end time.Time) (string, [][]string) {
	alarms := s.alarmSvc.List(tenantID)
	bySeverity := map[string]int{}
	byStatus := map[string]int{}
	for i := range alarms {
		if alarms[i].CreatedAt.Before(start) || !alarms[i].CreatedAt.Before(end) {
			continue
		}
		severity := strings.ToLower(strings.TrimSpace(alarms[i].Severity))
		if severity == "" {
			severity = "unknown"
		}
		bySeverity[severity]++
		status := strings.ToLower(strings.TrimSpace(alarms[i].Status))
		if status == "" {
			status = "unknown"
		}
		byStatus[status]++
	}

	title := fmt.Sprintf("Alarm Metrics (%s to %s)", start.UTC().Format("2006-01-02"), end.UTC().Format("2006-01-02"))
	rows := [][]string{{"Metric", "Value"}}
	total := 0
	for _, c := range bySeverity {
		total += c
	}
	rows = append(rows, []string{"Total Alarms", strconv.Itoa(total)})
	for severity, count := range bySeverity {
		rows = append(rows, []string{"Severity: " + severity, strconv.Itoa(count)})
	}
	for status, count := range byStatus {
		rows = append(rows, []string{"Status: " + status, strconv.Itoa(count)})
	}
	return title, rows
}

// ---------------------------------------------------------------------------
// Simple PDF generator — creates a minimal valid PDF 1.4 document with text
// ---------------------------------------------------------------------------

func generateSimplePDF(title string, rows [][]string) []byte {
	// Build the text content for the PDF page stream.
	var content strings.Builder
	content.WriteString("BT\n")
	content.WriteString("/F1 16 Tf\n")
	content.WriteString("50 750 Td\n")
	content.WriteString(fmt.Sprintf("(%s) Tj\n", pdfEscapeString(title)))
	content.WriteString("/F1 10 Tf\n")
	content.WriteString("0 -24 Td\n")

	for _, row := range rows {
		line := strings.Join(row, "    ")
		content.WriteString(fmt.Sprintf("0 -14 Td\n(%s) Tj\n", pdfEscapeString(line)))
	}

	// Timestamp footer
	content.WriteString("0 -28 Td\n")
	content.WriteString(fmt.Sprintf("/F1 8 Tf\n(Generated: %s) Tj\n", pdfEscapeString(time.Now().UTC().Format(time.RFC3339))))
	content.WriteString("ET\n")
	stream := content.String()

	// Build minimal PDF 1.4 structure.
	var pdf strings.Builder
	offsets := make([]int, 5)

	pdf.WriteString("%PDF-1.4\n")

	// Object 1: Catalog
	offsets[0] = pdf.Len()
	pdf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2: Pages
	offsets[1] = pdf.Len()
	pdf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Object 3: Page
	offsets[2] = pdf.Len()
	pdf.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n")

	// Object 4: Stream (page content)
	offsets[3] = pdf.Len()
	pdf.WriteString(fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj\n", len(stream), stream))

	// Object 5: Font
	offsets[4] = pdf.Len()
	pdf.WriteString("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Courier >>\nendobj\n")

	// Cross-reference table
	xrefOffset := pdf.Len()
	pdf.WriteString("xref\n")
	pdf.WriteString(fmt.Sprintf("0 %d\n", len(offsets)+1))
	pdf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		pdf.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}

	// Trailer
	pdf.WriteString("trailer\n")
	pdf.WriteString(fmt.Sprintf("<< /Size %d /Root 1 0 R >>\n", len(offsets)+1))
	pdf.WriteString("startxref\n")
	pdf.WriteString(fmt.Sprintf("%d\n", xrefOffset))
	pdf.WriteString("%%EOF\n")

	return []byte(pdf.String())
}

func pdfEscapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	return s
}
