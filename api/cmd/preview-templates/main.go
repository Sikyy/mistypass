package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/mistypass/cloud/api/internal/pdfgen"
)

func main() {
	renderer, err := pdfgen.NewRenderer()
	if err != nil {
		log.Fatal(err)
	}

	now := time.Now().UTC()
	meta := pdfgen.ReportMeta{
		TenantName:  "Acme Corporation",
		PlaceName:   "Jakarta HQ",
		PeriodStart: now.Add(-7 * 24 * time.Hour),
		PeriodEnd:   now,
		GeneratedAt: now,
	}

	reports := map[string]any{
		"weekly_analytics": sampleWeeklyAnalytics(),
		"events":           sampleEvents(),
		"unlock_stats":     sampleUnlockStats(),
		"user_presence":    sampleUserPresence(),
		"incidents":        sampleIncidents(),
		"hardware":         sampleHardware(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>PDF Template Preview</title>
<style>
:root{color-scheme:dark;--obsidian:#070806;--graphite:#141510;--mist:#F5F0E6;--smoke:#BEB8AA;--teal:#62B7A8}
*{box-sizing:border-box}body{font-family:Inter,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;min-height:100vh;margin:0;background:radial-gradient(ellipse 70% 45% at 70% 8%,rgba(98,183,168,.13),transparent 62%),var(--obsidian);color:var(--mist)}
main{width:min(760px,calc(100% - 40px));margin:0 auto;padding:72px 0}
.kicker{color:var(--teal);font-size:12px;font-weight:700;letter-spacing:.14em;text-transform:uppercase}.title{max-width:560px;margin:16px 0 12px;font-size:48px;font-weight:400;line-height:1.04}.copy{max-width:480px;color:rgba(245,240,230,.68);font-size:16px;line-height:1.5}
.grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;margin-top:34px}
a{display:flex;align-items:center;justify-content:space-between;gap:16px;min-height:58px;border:1px solid rgba(245,240,230,.15);border-radius:6px;padding:14px 16px;background:rgba(245,240,230,.035);color:var(--mist);text-decoration:none}
a:hover{border-color:rgba(98,183,168,.55);background:rgba(98,183,168,.08)}a span{color:var(--teal)}
</style></head><body><main>
<div class="kicker">Mistyislet PDF Preview</div>
<h1 class="title">Report templates</h1>
<p class="copy">Preview the HTML that Gotenberg converts to PDF. Each template uses the Mistyislet report palette and shared layout.</p>
<div class="grid">
<a href="/weekly_analytics">Weekly Analytics <span>Open</span></a>
<a href="/events">Access Events <span>Open</span></a>
<a href="/unlock_stats">Unlock Statistics <span>Open</span></a>
<a href="/user_presence">User Presence <span>Open</span></a>
<a href="/incidents">Incidents <span>Open</span></a>
<a href="/hardware">Hardware Health <span>Open</span></a>
</div>
</main>
</body></html>`)
	})

	for rt, data := range reports {
		rt, data := rt, data
		mux.HandleFunc("/"+rt, func(w http.ResponseWriter, r *http.Request) {
			html, err := renderer.RenderHTML(rt, meta, data)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(html)
		})
	}

	addr := ":9876"
	log.Printf("Preview server at http://localhost%s", addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}

func sampleWeeklyAnalytics() pdfgen.WeeklyAnalyticsData {
	return pdfgen.WeeklyAnalyticsData{
		DailyUsage: []pdfgen.DailyUsageRow{
			{Date: "2026-05-17", Unlocks: 142, UniqueUsers: 28, Occupancy: 65},
			{Date: "2026-05-18", Unlocks: 38, UniqueUsers: 8, Occupancy: 15},
			{Date: "2026-05-19", Unlocks: 155, UniqueUsers: 31, Occupancy: 72},
			{Date: "2026-05-20", Unlocks: 168, UniqueUsers: 34, Occupancy: 78},
			{Date: "2026-05-21", Unlocks: 130, UniqueUsers: 26, Occupancy: 60},
			{Date: "2026-05-22", Unlocks: 145, UniqueUsers: 30, Occupancy: 68},
			{Date: "2026-05-23", Unlocks: 92, UniqueUsers: 20, Occupancy: 45},
		},
		HeatmapData: func() []pdfgen.HeatmapCell {
			users := []string{"Alice Chen", "Bob Wang", "Charlie Liu", "Diana Tan", "Eric Zhang"}
			var cells []pdfgen.HeatmapCell
			for _, u := range users {
				for h := 7; h < 20; h++ {
					count := 0
					if h >= 8 && h <= 18 {
						count = (h*7 + len(u)) % 12
					}
					if count > 0 {
						cells = append(cells, pdfgen.HeatmapCell{User: u, Hour: h, Count: count})
					}
				}
			}
			return cells
		}(),
		UnlocksByType: map[string]int{
			"BLE": 420, "PIN": 180, "Fingerprint": 95, "Card": 65, "Remote": 30,
		},
		TopDoors: []pdfgen.DoorRanking{
			{Door: "Main Entrance", Unlocks: 285},
			{Door: "Server Room", Unlocks: 142},
			{Door: "Meeting Room A", Unlocks: 118},
			{Door: "Parking Gate", Unlocks: 95},
			{Door: "Back Door", Unlocks: 72},
		},
		FailedAttempts: []pdfgen.FailedAttemptRow{
			{Time: "Thu, May 22 2026 — 14:32:10", User: "Unknown", Door: "Server Room", Reason: "denied"},
			{Time: "Wed, May 21 2026 — 09:15:42", User: "Bob Wang", Door: "Main Entrance", Reason: "expired_credential"},
			{Time: "Tue, May 20 2026 — 18:45:00", User: "Unknown", Door: "Back Door", Reason: "denied"},
		},
		WeeklyUniqueUsers: []pdfgen.WeeklyUserCount{
			{WeekLabel: "W19", UniqueUsers: 25},
			{WeekLabel: "W20", UniqueUsers: 32},
			{WeekLabel: "W21", UniqueUsers: 28},
		},
	}
}

func sampleEvents() pdfgen.EventsData {
	hourly := make([]int, 24)
	hourly[7] = 12
	hourly[8] = 45
	hourly[9] = 38
	hourly[10] = 22
	hourly[11] = 18
	hourly[12] = 30
	hourly[13] = 25
	hourly[14] = 20
	hourly[15] = 15
	hourly[16] = 12
	hourly[17] = 35
	hourly[18] = 28
	hourly[19] = 8

	return pdfgen.EventsData{
		TotalEvents: 328,
		Granted:     302,
		Denied:      26,
		PeakHour:    8,
		HourlyDist:  hourly,
		Events: []pdfgen.EventRow{
			{Time: "2026-05-23 08:12:30", User: "Alice Chen", Door: "Main Entrance", Result: "granted", Method: "BLE"},
			{Time: "2026-05-23 08:15:42", User: "Bob Wang", Door: "Main Entrance", Result: "granted", Method: "Fingerprint"},
			{Time: "2026-05-23 08:22:10", User: "Charlie Liu", Door: "Parking Gate", Result: "granted", Method: "Card"},
			{Time: "2026-05-23 08:30:55", User: "Unknown", Door: "Server Room", Result: "denied", Method: "PIN"},
			{Time: "2026-05-23 09:01:20", User: "Diana Tan", Door: "Meeting Room A", Result: "granted", Method: "BLE"},
			{Time: "2026-05-23 09:15:38", User: "Eric Zhang", Door: "Main Entrance", Result: "granted", Method: "BLE"},
			{Time: "2026-05-23 09:45:12", User: "Unknown", Door: "Back Door", Result: "denied", Method: "Card"},
			{Time: "2026-05-23 10:02:00", User: "Alice Chen", Door: "Server Room", Result: "granted", Method: "Fingerprint"},
		},
	}
}

func sampleUnlockStats() pdfgen.UnlockStatsData {
	return pdfgen.UnlockStatsData{
		Total: 870,
		ByMethod: map[string]int{
			"BLE": 420, "PIN": 180, "Fingerprint": 130, "Card": 95, "Remote": 45,
		},
		Trend: []pdfgen.UnlockTrendPoint{
			{Date: "2026-05-17", Count: 142},
			{Date: "2026-05-18", Count: 38},
			{Date: "2026-05-19", Count: 155},
			{Date: "2026-05-20", Count: 168},
			{Date: "2026-05-21", Count: 130},
			{Date: "2026-05-22", Count: 145},
			{Date: "2026-05-23", Count: 92},
		},
	}
}

func sampleUserPresence() pdfgen.UserPresenceData {
	users := []string{"Alice Chen", "Bob Wang", "Charlie Liu", "Diana Tan", "Eric Zhang"}
	dates := []string{"2026-05-19", "2026-05-20", "2026-05-21", "2026-05-22", "2026-05-23"}
	var heatmap []pdfgen.PresenceHeatmapCell
	for i, u := range users {
		for j, d := range dates {
			count := (i*3 + j*2 + 1) % 8
			if count > 0 {
				heatmap = append(heatmap, pdfgen.PresenceHeatmapCell{User: u, Date: d, Count: count})
			}
		}
	}

	return pdfgen.UserPresenceData{
		HeatmapData: heatmap,
		DailyUniqueUsers: []pdfgen.DailyUniqueCount{
			{Date: "2026-05-19", Count: 31},
			{Date: "2026-05-20", Count: 34},
			{Date: "2026-05-21", Count: 26},
			{Date: "2026-05-22", Count: 30},
			{Date: "2026-05-23", Count: 20},
		},
		Users: []pdfgen.UserPresenceRow{
			{User: "Alice Chen", FirstSeen: "2026-05-19 08:12", LastSeen: "2026-05-23 17:30", Total: 45},
			{User: "Bob Wang", FirstSeen: "2026-05-19 08:30", LastSeen: "2026-05-23 18:00", Total: 38},
			{User: "Charlie Liu", FirstSeen: "2026-05-19 09:00", LastSeen: "2026-05-23 16:45", Total: 32},
			{User: "Diana Tan", FirstSeen: "2026-05-20 08:15", LastSeen: "2026-05-23 17:15", Total: 28},
			{User: "Eric Zhang", FirstSeen: "2026-05-19 07:45", LastSeen: "2026-05-22 18:30", Total: 22},
		},
	}
}

func sampleIncidents() pdfgen.IncidentsData {
	return pdfgen.IncidentsData{
		Total:      18,
		BySeverity: map[string]int{"critical": 2, "high": 5, "medium": 8, "low": 3},
		ByStatus:   map[string]int{"open": 4, "resolved": 12, "acknowledged": 2},
		Incidents: []pdfgen.IncidentRow{
			{Time: "2026-05-23 14:32:10", Type: "forced_entry", Severity: "critical", Door: "Server Room", Status: "open"},
			{Time: "2026-05-23 09:15:42", Type: "tamper_alert", Severity: "high", Door: "Back Door", Status: "resolved"},
			{Time: "2026-05-22 18:45:00", Type: "door_held_open", Severity: "medium", Door: "Main Entrance", Status: "resolved"},
			{Time: "2026-05-22 11:20:30", Type: "repeated_denied", Severity: "high", Door: "Server Room", Status: "acknowledged"},
			{Time: "2026-05-21 16:05:15", Type: "device_offline", Severity: "medium", Door: "Parking Gate", Status: "resolved"},
			{Time: "2026-05-21 08:30:00", Type: "forced_entry", Severity: "critical", Door: "Back Door", Status: "open"},
			{Time: "2026-05-20 14:12:45", Type: "door_held_open", Severity: "low", Door: "Meeting Room A", Status: "resolved"},
		},
	}
}

func sampleHardware() pdfgen.HardwareData {
	return pdfgen.HardwareData{
		Online:  6,
		Offline: 2,
		Devices: []pdfgen.DeviceRow{
			{Name: "GW-001-MAIN", Type: "gateway", Status: "online", Battery: 100, Signal: 95, LastSeen: "2026-05-23 15:30"},
			{Name: "GW-002-SERVER", Type: "gateway", Status: "online", Battery: 100, Signal: 88, LastSeen: "2026-05-23 15:29"},
			{Name: "GW-003-PARKING", Type: "gateway", Status: "online", Battery: 85, Signal: 72, LastSeen: "2026-05-23 15:28"},
			{Name: "GW-004-MEETING", Type: "gateway", Status: "online", Battery: 100, Signal: 90, LastSeen: "2026-05-23 15:30"},
			{Name: "GW-005-BACK", Type: "gateway", Status: "offline", Battery: 45, Signal: 30, LastSeen: "2026-05-22 18:00"},
			{Name: "GW-006-LOBBY", Type: "gateway", Status: "online", Battery: 92, Signal: 85, LastSeen: "2026-05-23 15:30"},
			{Name: "GW-007-FLOOR2", Type: "gateway", Status: "online", Battery: 78, Signal: 82, LastSeen: "2026-05-23 15:25"},
			{Name: "GW-008-STORAGE", Type: "gateway", Status: "offline", Battery: 12, Signal: 15, LastSeen: "2026-05-20 10:00"},
		},
		BatteryDist: []pdfgen.BatteryBucket{
			{Label: "0-25%", Count: 1},
			{Label: "26-50%", Count: 1},
			{Label: "51-75%", Count: 0},
			{Label: "76-100%", Count: 6},
		},
		SignalDist: []pdfgen.SignalBucket{
			{Label: "Weak", Count: 1},
			{Label: "Fair", Count: 1},
			{Label: "Good", Count: 3},
			{Label: "Strong", Count: 3},
		},
	}
}
