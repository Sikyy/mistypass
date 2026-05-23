package httpx

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/space"
	"github.com/mistypass/cloud/api/internal/pdfgen"
)

func (s *server) exportReport(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	reportType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if reportType == "" {
		writeError(w, http.StatusBadRequest, "type query parameter is required (weekly_analytics, events, unlock_stats, user_presence, incidents, hardware)")
		return
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "pdf"
	}
	if format != "pdf" && format != "csv" && format != "json" {
		writeError(w, http.StatusBadRequest, "format must be one of: pdf, csv, json")
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
		writeError(w, http.StatusBadRequest, "invalid start: must be RFC3339")
		return
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end: must be RFC3339")
		return
	}

	placeID := strings.TrimSpace(r.URL.Query().Get("place_id"))

	meta := pdfgen.ReportMeta{
		TenantName:  s.resolveExportTenantName(tenantID),
		PlaceName:   s.resolveExportPlaceName(tenantID, placeID),
		PeriodStart: start,
		PeriodEnd:   end,
		GeneratedAt: time.Now().UTC(),
	}

	data, err := s.buildReportData(reportType, tenantID, placeID, start, end)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	switch format {
	case "pdf":
		pdfBytes, err := s.pdfRenderer.RenderPDF(s.gotenbergClient, reportType, meta, data)
		if err != nil {
			s.logger.Error("pdf render failed", "error", err, "type", reportType)
			writeError(w, http.StatusBadGateway, "PDF rendering failed: "+err.Error())
			return
		}
		filename := pdfgen.FormatPDFFilename(reportType, start, end)
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
		w.Write(pdfBytes)

	case "csv":
		csvBody := s.buildReportCSV(reportType, tenantID, placeID, start, end)
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", reportType+".csv"))
		w.Write([]byte(csvBody))

	default:
		writeJSON(w, http.StatusOK, map[string]any{"meta": meta, "data": data})
	}
}

func (s *server) buildReportData(reportType, tenantID, placeID string, start, end time.Time) (any, error) {
	switch reportType {
	case "weekly_analytics":
		return s.buildWeeklyAnalyticsData(tenantID, start, end), nil
	case "events":
		return s.buildEventsData(tenantID, start, end), nil
	case "unlock_stats":
		return s.buildUnlockStatsData(tenantID, start, end), nil
	case "user_presence":
		return s.buildUserPresenceData(tenantID, start, end), nil
	case "incidents":
		return s.buildIncidentsData(tenantID, start, end), nil
	case "hardware":
		return s.buildHardwareData(tenantID), nil
	default:
		return nil, fmt.Errorf("unknown report type: %s", reportType)
	}
}

func (s *server) buildWeeklyAnalyticsData(tenantID string, start, end time.Time) pdfgen.WeeklyAnalyticsData {
	events := s.eventSvc.ListAccessEvents(tenantID)
	doors := s.spaceSvc.ListDoors(tenantID)

	dailyMap := map[string]*pdfgen.DailyUsageRow{}
	heatmap := map[string]map[int]int{}
	unlocksByType := map[string]int{}
	doorCounts := map[string]int{}
	var failed []pdfgen.FailedAttemptRow
	usersByWeek := map[string]map[string]bool{}

	for _, e := range events {
		if e.At.Before(start) || !e.At.Before(end) {
			continue
		}
		dateKey := e.At.Format("2006-01-02")
		if dailyMap[dateKey] == nil {
			dailyMap[dateKey] = &pdfgen.DailyUsageRow{Date: dateKey}
		}
		dailyMap[dateKey].Unlocks++

		hour := e.At.Hour()
		if heatmap[e.Actor] == nil {
			heatmap[e.Actor] = map[int]int{}
		}
		heatmap[e.Actor][hour]++

		unlocksByType[e.Type]++
		doorCounts[e.DoorID]++

		if strings.EqualFold(e.Result, "denied") || strings.EqualFold(e.Result, "rejected") {
			failed = append(failed, pdfgen.FailedAttemptRow{
				Time: e.At.Format("Mon, Jan 2 2006 — 15:04:05"),
				User: e.Actor, Door: s.resolveDoorName(doors, e.DoorID),
				Reason: e.Result,
			})
		}

		_, week := e.At.ISOWeek()
		weekKey := fmt.Sprintf("W%d", week)
		if usersByWeek[weekKey] == nil {
			usersByWeek[weekKey] = map[string]bool{}
		}
		usersByWeek[weekKey][e.Actor] = true
	}

	var daily []pdfgen.DailyUsageRow
	for _, v := range dailyMap {
		daily = append(daily, *v)
	}

	var heatmapCells []pdfgen.HeatmapCell
	for user, hours := range heatmap {
		for h, c := range hours {
			heatmapCells = append(heatmapCells, pdfgen.HeatmapCell{User: user, Hour: h, Count: c})
		}
	}

	var topDoors []pdfgen.DoorRanking
	for doorID, count := range doorCounts {
		topDoors = append(topDoors, pdfgen.DoorRanking{Door: s.resolveDoorName(doors, doorID), Unlocks: count})
	}
	sortDoorRankings(topDoors)

	var weeklyUsers []pdfgen.WeeklyUserCount
	for wk, users := range usersByWeek {
		weeklyUsers = append(weeklyUsers, pdfgen.WeeklyUserCount{WeekLabel: wk, UniqueUsers: len(users)})
	}

	return pdfgen.WeeklyAnalyticsData{
		DailyUsage:        daily,
		HeatmapData:       heatmapCells,
		UnlocksByType:     unlocksByType,
		TopDoors:          topDoors,
		FailedAttempts:    failed,
		WeeklyUniqueUsers: weeklyUsers,
	}
}

func (s *server) buildEventsData(tenantID string, start, end time.Time) pdfgen.EventsData {
	events := s.eventSvc.ListAccessEvents(tenantID)
	doors := s.spaceSvc.ListDoors(tenantID)

	var granted, denied int
	hourly := make([]int, 24)
	peakHour := 0
	var rows []pdfgen.EventRow

	for _, e := range events {
		if e.At.Before(start) || !e.At.Before(end) {
			continue
		}
		h := e.At.Hour()
		hourly[h]++
		if hourly[h] > hourly[peakHour] {
			peakHour = h
		}
		switch {
		case strings.EqualFold(e.Result, "success"), strings.EqualFold(e.Result, "accepted"), strings.EqualFold(e.Result, "granted"):
			granted++
		default:
			denied++
		}
		rows = append(rows, pdfgen.EventRow{
			Time: e.At.Format("2006-01-02 15:04:05"), User: e.Actor,
			Door: s.resolveDoorName(doors, e.DoorID), Result: e.Result, Method: e.Type,
		})
	}
	return pdfgen.EventsData{
		TotalEvents: granted + denied, Granted: granted, Denied: denied,
		PeakHour: peakHour, HourlyDist: hourly, Events: rows,
	}
}

func (s *server) buildUnlockStatsData(tenantID string, start, end time.Time) pdfgen.UnlockStatsData {
	events := s.eventSvc.ListAccessEvents(tenantID)
	byMethod := map[string]int{}
	trendMap := map[string]int{}
	total := 0

	for _, e := range events {
		if e.At.Before(start) || !e.At.Before(end) {
			continue
		}
		method := e.Type
		if method == "" {
			method = "unknown"
		}
		byMethod[method]++
		trendMap[e.At.Format("2006-01-02")]++
		total++
	}

	var trend []pdfgen.UnlockTrendPoint
	for date, count := range trendMap {
		trend = append(trend, pdfgen.UnlockTrendPoint{Date: date, Count: count})
	}
	return pdfgen.UnlockStatsData{ByMethod: byMethod, Trend: trend, Total: total}
}

func (s *server) buildUserPresenceData(tenantID string, start, end time.Time) pdfgen.UserPresenceData {
	events := s.eventSvc.ListAccessEvents(tenantID)
	userFirst := map[string]time.Time{}
	userLast := map[string]time.Time{}
	userTotal := map[string]int{}
	presenceMap := map[string]map[string]int{}
	dailyUsers := map[string]map[string]bool{}

	for _, e := range events {
		if e.At.Before(start) || !e.At.Before(end) {
			continue
		}
		if _, ok := userFirst[e.Actor]; !ok {
			userFirst[e.Actor] = e.At
		}
		userLast[e.Actor] = e.At
		userTotal[e.Actor]++

		date := e.At.Format("2006-01-02")
		if presenceMap[e.Actor] == nil {
			presenceMap[e.Actor] = map[string]int{}
		}
		presenceMap[e.Actor][date]++

		if dailyUsers[date] == nil {
			dailyUsers[date] = map[string]bool{}
		}
		dailyUsers[date][e.Actor] = true
	}

	var heatmap []pdfgen.PresenceHeatmapCell
	for user, dates := range presenceMap {
		for date, count := range dates {
			heatmap = append(heatmap, pdfgen.PresenceHeatmapCell{User: user, Date: date, Count: count})
		}
	}

	var daily []pdfgen.DailyUniqueCount
	for date, users := range dailyUsers {
		daily = append(daily, pdfgen.DailyUniqueCount{Date: date, Count: len(users)})
	}

	var users []pdfgen.UserPresenceRow
	for user, total := range userTotal {
		users = append(users, pdfgen.UserPresenceRow{
			User: user, FirstSeen: userFirst[user].Format("2006-01-02 15:04"),
			LastSeen: userLast[user].Format("2006-01-02 15:04"), Total: total,
		})
	}
	return pdfgen.UserPresenceData{HeatmapData: heatmap, DailyUniqueUsers: daily, Users: users}
}

func (s *server) buildIncidentsData(tenantID string, start, end time.Time) pdfgen.IncidentsData {
	alarms := s.alarmSvc.List(tenantID)
	bySeverity := map[string]int{}
	byStatus := map[string]int{}
	var rows []pdfgen.IncidentRow

	for _, a := range alarms {
		if a.CreatedAt.Before(start) || !a.CreatedAt.Before(end) {
			continue
		}
		sev := strings.ToLower(a.Severity)
		if sev == "" {
			sev = "unknown"
		}
		bySeverity[sev]++
		stat := strings.ToLower(a.Status)
		if stat == "" {
			stat = "unknown"
		}
		byStatus[stat]++
		rows = append(rows, pdfgen.IncidentRow{
			Time: a.CreatedAt.Format("2006-01-02 15:04:05"), Type: a.Type,
			Severity: sev, Door: a.DoorID, Status: stat,
		})
	}
	total := 0
	for _, c := range bySeverity {
		total += c
	}
	return pdfgen.IncidentsData{Total: total, BySeverity: bySeverity, ByStatus: byStatus, Incidents: rows}
}

func (s *server) buildHardwareData(tenantID string) pdfgen.HardwareData {
	gateways := s.gatewaySvc.List(tenantID)
	var online, offline int
	var devices []pdfgen.DeviceRow

	for _, gw := range gateways {
		status := "offline"
		if strings.EqualFold(gw.Status, "connected") || strings.EqualFold(gw.Status, "online") {
			status = "online"
			online++
		} else {
			offline++
		}
		name := gw.SerialNumber
		if name == "" {
			name = gw.ID
		}
		devices = append(devices, pdfgen.DeviceRow{
			Name: name, Type: "gateway", Status: status,
			Battery: 100, Signal: 100,
			LastSeen: gw.LastSeenAt.Format("2006-01-02 15:04"),
		})
	}

	return pdfgen.HardwareData{
		Online: online, Offline: offline, Devices: devices,
		BatteryDist: []pdfgen.BatteryBucket{
			{Label: "0-25%", Count: 0}, {Label: "26-50%", Count: 0},
			{Label: "51-75%", Count: 0}, {Label: "76-100%", Count: online + offline},
		},
		SignalDist: []pdfgen.SignalBucket{
			{Label: "Weak", Count: 0}, {Label: "Fair", Count: 0},
			{Label: "Good", Count: 0}, {Label: "Strong", Count: online + offline},
		},
	}
}

func (s *server) resolveExportTenantName(tenantID string) string {
	for _, t := range s.tenantSvc.List() {
		if t.ID == tenantID {
			return t.Name
		}
	}
	return tenantID
}

func (s *server) resolveExportPlaceName(tenantID, placeID string) string {
	if placeID == "" {
		return "All Locations"
	}
	buildings := s.spaceSvc.ListBuildings(tenantID)
	for _, b := range buildings {
		if b.ID == placeID {
			return b.Name
		}
	}
	return placeID
}

func (s *server) resolveDoorName(doors []space.Door, doorID string) string {
	for _, d := range doors {
		if d.ID == doorID {
			if d.Name != "" {
				return d.Name
			}
			return d.ID
		}
	}
	return doorID
}

func sortDoorRankings(rankings []pdfgen.DoorRanking) {
	for i := 0; i < len(rankings); i++ {
		for j := i + 1; j < len(rankings); j++ {
			if rankings[j].Unlocks > rankings[i].Unlocks {
				rankings[i], rankings[j] = rankings[j], rankings[i]
			}
		}
	}
}

func (s *server) buildReportCSV(reportType, tenantID, placeID string, start, end time.Time) string {
	switch reportType {
	case "events", "weekly_analytics":
		_, rows := s.exportAccessSummaryRows(tenantID, start, end)
		var sb strings.Builder
		for _, row := range rows {
			sb.WriteString(strings.Join(row, ","))
			sb.WriteString("\n")
		}
		return sb.String()
	case "incidents":
		_, rows := s.exportAlarmMetricsRows(tenantID, start, end)
		var sb strings.Builder
		for _, row := range rows {
			sb.WriteString(strings.Join(row, ","))
			sb.WriteString("\n")
		}
		return sb.String()
	default:
		return "report_type,not_implemented\n"
	}
}
