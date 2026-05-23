package pdfgen

import "time"

type ReportMeta struct {
	TenantName  string
	PlaceName   string
	PeriodStart time.Time
	PeriodEnd   time.Time
	GeneratedAt time.Time
}

type WeeklyAnalyticsData struct {
	DailyUsage        []DailyUsageRow    `json:"daily_usage"`
	HeatmapData       []HeatmapCell      `json:"heatmap_data"`
	UnlocksByType     map[string]int     `json:"unlocks_by_type"`
	TopDoors          []DoorRanking      `json:"top_doors"`
	FailedAttempts    []FailedAttemptRow `json:"failed_attempts"`
	WeeklyUniqueUsers []WeeklyUserCount  `json:"weekly_unique_users"`
}

type DailyUsageRow struct {
	Date        string `json:"date"`
	Unlocks     int    `json:"unlocks"`
	UniqueUsers int    `json:"unique_users"`
	Occupancy   int    `json:"occupancy"`
}

type HeatmapCell struct {
	User  string `json:"user"`
	Hour  int    `json:"hour"`
	Count int    `json:"count"`
}

type DoorRanking struct {
	Door    string `json:"door"`
	Unlocks int    `json:"unlocks"`
}

type FailedAttemptRow struct {
	Time   string `json:"time"`
	User   string `json:"user"`
	Door   string `json:"door"`
	Reason string `json:"reason"`
}

type WeeklyUserCount struct {
	WeekLabel   string `json:"week_label"`
	UniqueUsers int    `json:"unique_users"`
}

type EventsData struct {
	TotalEvents int        `json:"total_events"`
	Granted     int        `json:"granted"`
	Denied      int        `json:"denied"`
	PeakHour    int        `json:"peak_hour"`
	HourlyDist  []int      `json:"hourly_dist"`
	Events      []EventRow `json:"events"`
}

type EventRow struct {
	Time   string `json:"time"`
	User   string `json:"user"`
	Door   string `json:"door"`
	Result string `json:"result"`
	Method string `json:"method"`
}

type UnlockStatsData struct {
	ByMethod map[string]int     `json:"by_method"`
	Trend    []UnlockTrendPoint `json:"trend"`
	Total    int                `json:"total"`
}

type UnlockTrendPoint struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type UserPresenceData struct {
	HeatmapData      []PresenceHeatmapCell `json:"heatmap_data"`
	DailyUniqueUsers []DailyUniqueCount    `json:"daily_unique_users"`
	Users            []UserPresenceRow     `json:"users"`
}

type PresenceHeatmapCell struct {
	User  string `json:"user"`
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type DailyUniqueCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type UserPresenceRow struct {
	User      string `json:"user"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
	Total     int    `json:"total"`
}

type IncidentsData struct {
	Total      int            `json:"total"`
	BySeverity map[string]int `json:"by_severity"`
	ByStatus   map[string]int `json:"by_status"`
	Incidents  []IncidentRow  `json:"incidents"`
}

type IncidentRow struct {
	Time     string `json:"time"`
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Door     string `json:"door"`
	Status   string `json:"status"`
}

type HardwareData struct {
	Online      int             `json:"online"`
	Offline     int             `json:"offline"`
	Devices     []DeviceRow     `json:"devices"`
	BatteryDist []BatteryBucket `json:"battery_dist"`
	SignalDist  []SignalBucket  `json:"signal_dist"`
}

type DeviceRow struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Battery  int    `json:"battery"`
	Signal   int    `json:"signal"`
	LastSeen string `json:"last_seen"`
}

type BatteryBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type SignalBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}
