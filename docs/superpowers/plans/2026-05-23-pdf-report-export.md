# PDF Report Export System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the text-only PDF generator with a professional report system using Go HTML templates + Gotenberg Docker sidecar, producing Kisi-style reports with Chart.js charts for all 6 report types, with scheduled email delivery via Resend API.

**Architecture:** Go backend renders HTML templates with Chart.js chart data injected as JSON, sends the HTML to a Gotenberg Docker container which uses Chromium headless to produce PDF. Templates are embedded in the Go binary via `//go:embed`. Email delivery uses the existing Resend API integration with base64-encoded PDF attachments.

**Tech Stack:** Go 1.25, html/template, embed, Chart.js 4.x, chartjs-chart-matrix plugin, Gotenberg 8, Resend API, chi router, Docker Compose

---

## File Structure

```
api/internal/pdfgen/
├── gotenberg.go              # Gotenberg HTTP client (ConvertHTML)
├── gotenberg_test.go         # Client test with httptest mock
├── renderer.go               # Render(reportType, meta, data) → PDF bytes
├── renderer_test.go          # Template parsing + HTML output tests
├── types.go                  # Shared types (ReportMeta, ReportData variants)
├── assets/
│   └── logo.png              # MistyPass brand logo
└── templates/
    ├── base.html             # Shared layout: header, footer, CSS, Chart.js
    ├── weekly_analytics.html # Weekly analytics report content
    ├── events.html           # Access events report content
    ├── unlock_stats.html     # Unlock statistics report content
    ├── user_presence.html    # User presence report content
    ├── incidents.html        # Incidents/alarms report content
    └── hardware.html         # Hardware status report content

api/internal/config/config.go          # Add GotenbergURL field
api/internal/http/router.go            # Wire pdfgen into server struct
api/internal/http/routes_report_export.go   # New unified export endpoint
api/internal/http/routes_report_schedule.go # Upgrade send to attach PDF
api/internal/http/routes_analytics.go       # Remove old generateSimplePDF
docker-compose.yml                     # Add gotenberg service
```

---

### Task 1: Docker Compose + Config

**Files:**
- Modify: `docker-compose.yml`
- Modify: `api/internal/config/config.go`

- [ ] **Step 1: Add Gotenberg service to docker-compose.yml**

Add the gotenberg service after the existing services:

```yaml
  gotenberg:
    image: gotenberg/gotenberg:8
    restart: unless-stopped
    command:
      - "gotenberg"
      - "--api-timeout=30s"
      - "--log-level=warn"
    networks:
      - internal
    deploy:
      resources:
        limits:
          memory: 512M
```

Add `GOTENBERG_URL=http://gotenberg:3000` to the `api` service environment and add `gotenberg` to its `depends_on`.

- [ ] **Step 2: Add GotenbergURL to Config struct**

In `api/internal/config/config.go`, add to the Config struct (near line 149, after `ReportEmailEnabled`):

```go
GotenbergURL string
```

In the `loadReportEmailConfig` function (near line 1037), add:

```go
cfg.GotenbergURL = envStringOrDefault("GOTENBERG_URL", "http://localhost:3000")
```

- [ ] **Step 3: Commit**

```bash
git add docker-compose.yml api/internal/config/config.go
git commit -m "infra: add Gotenberg sidecar and GOTENBERG_URL config"
```

---

### Task 2: Gotenberg HTTP Client

**Files:**
- Create: `api/internal/pdfgen/gotenberg.go`
- Create: `api/internal/pdfgen/gotenberg_test.go`

- [ ] **Step 1: Write the failing test**

Create `api/internal/pdfgen/gotenberg_test.go`:

```go
package pdfgen

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConvertHTML_Success(t *testing.T) {
	fakePDF := []byte("%PDF-1.4 fake content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/forms/chromium/convert/html" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		ct := r.Header.Get("Content-Type")
		if ct == "" || len(ct) < 10 {
			t.Error("expected multipart content-type")
		}
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		file, _, err := r.FormFile("files")
		if err != nil {
			t.Fatalf("expected files field: %v", err)
		}
		file.Close()
		if r.FormValue("printBackground") != "true" {
			t.Error("expected printBackground=true")
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Write(fakePDF)
	}))
	defer srv.Close()

	client := NewGotenbergClient(srv.URL, nil)
	result, err := client.ConvertHTML([]byte("<html><body>Hello</body></html>"), DefaultPDFOptions())
	if err != nil {
		t.Fatalf("ConvertHTML error: %v", err)
	}
	if string(result) != string(fakePDF) {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestConvertHTML_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("chromium crashed"))
	}))
	defer srv.Close()

	client := NewGotenbergClient(srv.URL, nil)
	_, err := client.ConvertHTML([]byte("<html></html>"), DefaultPDFOptions())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/siky/code/MistyPass/.claude/worktrees/funny-shockley-960a91/api && go test ./internal/pdfgen/ -run TestConvertHTML -v`
Expected: Compilation error — package/types don't exist yet.

- [ ] **Step 3: Write the implementation**

Create `api/internal/pdfgen/gotenberg.go`:

```go
package pdfgen

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"time"
)

type GotenbergClient struct {
	baseURL string
	client  *http.Client
}

type PDFOptions struct {
	PaperWidth      float64
	PaperHeight     float64
	MarginTop       float64
	MarginBottom    float64
	MarginLeft      float64
	MarginRight     float64
	PrintBackground bool
	WaitDelay       string
	Scale           float64
}

func DefaultPDFOptions() PDFOptions {
	return PDFOptions{
		PaperWidth:      8.27,
		PaperHeight:     11.69,
		MarginTop:       0.4,
		MarginBottom:    0.4,
		MarginLeft:      0.4,
		MarginRight:     0.4,
		PrintBackground: true,
		WaitDelay:       "500ms",
		Scale:           1.0,
	}
}

func NewGotenbergClient(baseURL string, client *http.Client) *GotenbergClient {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &GotenbergClient{baseURL: baseURL, client: client}
}

func (c *GotenbergClient) ConvertHTML(html []byte, opts PDFOptions) ([]byte, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="files"; filename="index.html"`)
	header.Set("Content-Type", "text/html; charset=utf-8")
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, fmt.Errorf("create file part: %w", err)
	}
	if _, err := part.Write(html); err != nil {
		return nil, fmt.Errorf("write html: %w", err)
	}

	formFields := map[string]string{
		"paperWidth":      strconv.FormatFloat(opts.PaperWidth, 'f', 2, 64),
		"paperHeight":     strconv.FormatFloat(opts.PaperHeight, 'f', 2, 64),
		"marginTop":       strconv.FormatFloat(opts.MarginTop, 'f', 2, 64),
		"marginBottom":    strconv.FormatFloat(opts.MarginBottom, 'f', 2, 64),
		"marginLeft":      strconv.FormatFloat(opts.MarginLeft, 'f', 2, 64),
		"marginRight":     strconv.FormatFloat(opts.MarginRight, 'f', 2, 64),
		"printBackground": strconv.FormatBool(opts.PrintBackground),
		"scale":           strconv.FormatFloat(opts.Scale, 'f', 2, 64),
	}
	if opts.WaitDelay != "" {
		formFields["waitDelay"] = opts.WaitDelay
	}
	for k, v := range formFields {
		if err := writer.WriteField(k, v); err != nil {
			return nil, fmt.Errorf("write field %s: %w", k, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/forms/chromium/convert/html", &body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("gotenberg returned %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func (c *GotenbergClient) Healthy() bool {
	resp, err := c.client.Get(c.baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/siky/code/MistyPass/.claude/worktrees/funny-shockley-960a91/api && go test ./internal/pdfgen/ -run TestConvertHTML -v`
Expected: Both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/pdfgen/gotenberg.go api/internal/pdfgen/gotenberg_test.go
git commit -m "feat(pdfgen): add Gotenberg HTTP client with tests"
```

---

### Task 3: Types and Brand Assets

**Files:**
- Create: `api/internal/pdfgen/types.go`
- Create: `api/internal/pdfgen/assets/logo.png`

- [ ] **Step 1: Create types.go with shared report data types**

Create `api/internal/pdfgen/types.go`:

```go
package pdfgen

import "time"

type ReportMeta struct {
	TenantName string
	PlaceName  string
	PeriodStart time.Time
	PeriodEnd   time.Time
	GeneratedAt time.Time
}

type WeeklyAnalyticsData struct {
	DailyUsage []DailyUsageRow  `json:"daily_usage"`
	HeatmapData []HeatmapCell   `json:"heatmap_data"`
	UnlocksByType map[string]int `json:"unlocks_by_type"`
	TopDoors []DoorRanking       `json:"top_doors"`
	FailedAttempts []FailedAttemptRow `json:"failed_attempts"`
	WeeklyUniqueUsers []WeeklyUserCount `json:"weekly_unique_users"`
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
	TotalEvents int            `json:"total_events"`
	Granted     int            `json:"granted"`
	Denied      int            `json:"denied"`
	PeakHour    int            `json:"peak_hour"`
	HourlyDist  []int          `json:"hourly_dist"`
	Events      []EventRow     `json:"events"`
}

type EventRow struct {
	Time   string `json:"time"`
	User   string `json:"user"`
	Door   string `json:"door"`
	Result string `json:"result"`
	Method string `json:"method"`
}

type UnlockStatsData struct {
	ByMethod map[string]int    `json:"by_method"`
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
	Online       int              `json:"online"`
	Offline      int              `json:"offline"`
	Devices      []DeviceRow      `json:"devices"`
	BatteryDist  []BatteryBucket  `json:"battery_dist"`
	SignalDist   []SignalBucket   `json:"signal_dist"`
}

type DeviceRow struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Battery    int    `json:"battery"`
	Signal     int    `json:"signal"`
	LastSeen   string `json:"last_seen"`
}

type BatteryBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type SignalBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}
```

- [ ] **Step 2: Add placeholder logo asset**

Create a 200x60 placeholder PNG at `api/internal/pdfgen/assets/logo.png`. For now, use any small PNG file (the real brand logo will be swapped in later). A 1x1 white pixel PNG is sufficient for development:

```bash
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82' > api/internal/pdfgen/assets/logo.png
```

- [ ] **Step 3: Commit**

```bash
git add api/internal/pdfgen/types.go api/internal/pdfgen/assets/logo.png
git commit -m "feat(pdfgen): add report data types and logo asset"
```

---

### Task 4: Base HTML Template

**Files:**
- Create: `api/internal/pdfgen/templates/base.html`

- [ ] **Step 1: Create the shared base template**

Create `api/internal/pdfgen/templates/base.html`:

```html
{{define "base"}}
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Meta.TenantName}} — Report</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4/dist/chart.umd.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/chartjs-chart-matrix@2/dist/chartjs-chart-matrix.min.js"></script>
<style>
  :root {
    --brand: #5046E5;
    --brand-light: #E8E7FB;
    --text-heading: #111827;
    --text-body: #374151;
    --text-subtle: #6B7280;
    --border: #E5E7EB;
    --bg-page: #FFFFFF;
    --bg-card: #FFFFFF;
    --success: #10B981;
    --danger: #EF4444;
    --warning: #F59E0B;
  }

  @page {
    size: A4;
    margin: 12mm 14mm 16mm 14mm;
  }

  * { margin: 0; padding: 0; box-sizing: border-box; }

  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    font-size: 11px;
    color: var(--text-body);
    background: var(--bg-page);
    line-height: 1.5;
  }

  .report-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding-bottom: 12px;
    border-bottom: 3px solid var(--brand);
    margin-bottom: 20px;
  }

  .report-header .brand {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .report-header .brand img {
    height: 32px;
  }

  .report-header .brand-name {
    font-size: 14px;
    font-weight: 700;
    color: var(--text-heading);
  }

  .report-header .place-name {
    font-size: 22px;
    font-weight: 700;
    color: var(--text-heading);
    margin-top: 2px;
  }

  .report-header .meta {
    text-align: right;
    font-size: 10px;
    color: var(--text-subtle);
  }

  .report-header .date-range {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-heading);
  }

  .section {
    margin-bottom: 24px;
    break-inside: avoid;
  }

  .section-title {
    font-size: 13px;
    font-weight: 700;
    color: var(--brand);
    margin-bottom: 10px;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .section-title::before {
    content: '';
    width: 8px;
    height: 8px;
    background: var(--brand);
    border-radius: 2px;
    display: inline-block;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 10px;
  }

  table thead th {
    background: var(--brand);
    color: #fff;
    padding: 6px 8px;
    text-align: left;
    font-weight: 600;
    font-size: 9px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  table tbody td {
    padding: 5px 8px;
    border-bottom: 1px solid var(--border);
    color: var(--text-body);
  }

  table tbody tr:nth-child(even) {
    background: #F9FAFB;
  }

  .chart-container {
    position: relative;
    width: 100%;
    max-height: 200px;
    margin: 8px 0;
  }

  .chart-container canvas {
    max-height: 200px;
  }

  .kpi-row {
    display: flex;
    gap: 12px;
    margin-bottom: 16px;
  }

  .kpi-card {
    flex: 1;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 10px 12px;
    text-align: center;
  }

  .kpi-card .kpi-value {
    font-size: 22px;
    font-weight: 700;
    color: var(--text-heading);
  }

  .kpi-card .kpi-label {
    font-size: 9px;
    color: var(--text-subtle);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-top: 2px;
  }

  .grid-2 {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  .grid-3 {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 16px;
  }

  .page-break {
    break-before: page;
  }

  .footer {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 6px 14mm;
    font-size: 8px;
    color: var(--text-subtle);
    display: flex;
    justify-content: space-between;
    border-top: 1px solid var(--border);
  }
</style>
</head>
<body>

<div class="report-header">
  <div>
    <div class="brand">
      <img src="data:image/png;base64,{{.LogoBase64}}" alt="Logo">
      <span class="brand-name">{{.Meta.TenantName}}</span>
    </div>
    <div class="place-name">{{.Meta.PlaceName}}</div>
  </div>
  <div class="meta">
    <div class="date-range">
      {{.Meta.PeriodStart.Format "Jan 2, 2006"}} — {{.Meta.PeriodEnd.Format "Jan 2, 2006"}}
    </div>
    <div>Weekly Report</div>
  </div>
</div>

{{template "content" .}}

<div class="footer">
  <span>Generated: {{.Meta.GeneratedAt.Format "2006-01-02 15:04 UTC"}}</span>
  <span>MistyPass</span>
</div>

<script>
const reportData = {{.DataJSON}};
Chart.defaults.font.family = "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif";
Chart.defaults.font.size = 10;
Chart.defaults.color = '#374151';
{{template "charts" .}}
</script>
</body>
</html>
{{end}}
```

- [ ] **Step 2: Commit**

```bash
git add api/internal/pdfgen/templates/base.html
git commit -m "feat(pdfgen): add base HTML template with brand header and Chart.js"
```

---

### Task 5: Renderer Pipeline

**Files:**
- Create: `api/internal/pdfgen/renderer.go`
- Create: `api/internal/pdfgen/renderer_test.go`

- [ ] **Step 1: Write the failing test**

Create `api/internal/pdfgen/renderer_test.go`:

```go
package pdfgen

import (
	"strings"
	"testing"
	"time"
)

func TestRenderHTML_WeeklyAnalytics(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	meta := ReportMeta{
		TenantName:  "Test Corp",
		PlaceName:   "HQ",
		PeriodStart: time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC),
		GeneratedAt: time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC),
	}
	data := WeeklyAnalyticsData{
		DailyUsage: []DailyUsageRow{
			{Date: "2026-05-17", Unlocks: 50, UniqueUsers: 10, Occupancy: 80},
		},
		UnlocksByType: map[string]int{"BLE": 30, "PIN": 20},
		TopDoors:      []DoorRanking{{Door: "Main Entrance", Unlocks: 45}},
	}
	html, err := r.RenderHTML("weekly_analytics", meta, data)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	output := string(html)
	if !strings.Contains(output, "Test Corp") {
		t.Error("expected tenant name in output")
	}
	if !strings.Contains(output, "HQ") {
		t.Error("expected place name in output")
	}
	if !strings.Contains(output, "chart.js") || !strings.Contains(output, "Chart") {
		t.Error("expected Chart.js in output")
	}
	if !strings.Contains(output, "reportData") {
		t.Error("expected reportData JSON injection")
	}
}

func TestRenderHTML_InvalidType(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	meta := ReportMeta{
		TenantName:  "Test",
		PlaceName:   "HQ",
		PeriodStart: time.Now(),
		PeriodEnd:   time.Now(),
		GeneratedAt: time.Now(),
	}
	_, err = r.RenderHTML("nonexistent_type", meta, nil)
	if err == nil {
		t.Error("expected error for invalid report type")
	}
}

func TestAllTemplatesParse(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	types := []string{
		"weekly_analytics", "events", "unlock_stats",
		"user_presence", "incidents", "hardware",
	}
	for _, rt := range types {
		if _, exists := r.templates[rt]; !exists {
			t.Errorf("template %q not loaded", rt)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/siky/code/MistyPass/.claude/worktrees/funny-shockley-960a91/api && go test ./internal/pdfgen/ -run TestRender -v`
Expected: Compilation error — Renderer type doesn't exist yet.

- [ ] **Step 3: Write the implementation**

Create `api/internal/pdfgen/renderer.go`:

```go
package pdfgen

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets/logo.png
var logoPNG []byte

var validReportTypes = map[string]bool{
	"weekly_analytics": true,
	"events":           true,
	"unlock_stats":     true,
	"user_presence":    true,
	"incidents":        true,
	"hardware":         true,
}

type Renderer struct {
	templates  map[string]*template.Template
	logoBase64 string
}

type templateData struct {
	Meta       ReportMeta
	LogoBase64 string
	DataJSON   template.JS
}

func NewRenderer() (*Renderer, error) {
	logoB64 := base64.StdEncoding.EncodeToString(logoPNG)
	templates := make(map[string]*template.Template)

	for rt := range validReportTypes {
		tmpl, err := template.ParseFS(templateFS,
			"templates/base.html",
			"templates/"+rt+".html",
		)
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", rt, err)
		}
		templates[rt] = tmpl
	}

	return &Renderer{
		templates:  templates,
		logoBase64: logoB64,
	}, nil
}

func (r *Renderer) RenderHTML(reportType string, meta ReportMeta, data any) ([]byte, error) {
	tmpl, ok := r.templates[reportType]
	if !ok {
		return nil, fmt.Errorf("unknown report type: %s", reportType)
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}

	td := templateData{
		Meta:       meta,
		LogoBase64: r.logoBase64,
		DataJSON:   template.JS(dataJSON),
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", td); err != nil {
		return nil, fmt.Errorf("execute template %s: %w", reportType, err)
	}
	return buf.Bytes(), nil
}

func (r *Renderer) RenderPDF(client *GotenbergClient, reportType string, meta ReportMeta, data any) ([]byte, error) {
	html, err := r.RenderHTML(reportType, meta, data)
	if err != nil {
		return nil, err
	}
	return client.ConvertHTML(html, DefaultPDFOptions())
}

func FormatPDFFilename(reportType string, start, end time.Time) string {
	return fmt.Sprintf("%s_%s_%s.pdf",
		reportType,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/siky/code/MistyPass/.claude/worktrees/funny-shockley-960a91/api && go test ./internal/pdfgen/ -run TestRender -v && go test ./internal/pdfgen/ -run TestAllTemplates -v`
Expected: Tests pass (once report templates exist — they will fail until Task 6-11 create the templates). For now, `TestAllTemplatesParse` will fail on missing template files, so only run the first two tests.

Note: `TestAllTemplatesParse` and `TestRenderHTML_WeeklyAnalytics` will fail until the report templates are created in Tasks 6-11. This is expected — move on to creating the templates.

- [ ] **Step 5: Commit**

```bash
git add api/internal/pdfgen/renderer.go api/internal/pdfgen/renderer_test.go
git commit -m "feat(pdfgen): add renderer pipeline with HTML + PDF generation"
```

---

### Task 6: Weekly Analytics Template

**Files:**
- Create: `api/internal/pdfgen/templates/weekly_analytics.html`

- [ ] **Step 1: Create the template**

Create `api/internal/pdfgen/templates/weekly_analytics.html`:

```html
{{define "content"}}
<div class="section">
  <div class="section-title">Daily Place Usage</div>
  <table>
    <thead>
      <tr>
        <th>Date</th>
        <th># Unlocks</th>
        <th># Unique Users</th>
        <th>% Occupancy</th>
      </tr>
    </thead>
    <tbody id="daily-usage-body"></tbody>
  </table>
</div>

<div class="grid-2">
  <div class="section">
    <div class="section-title">Unique Users Unlock Heatmap</div>
    <div class="chart-container" style="max-height:240px">
      <canvas id="heatmapChart"></canvas>
    </div>
  </div>
  <div class="section">
    <div class="section-title">Weekly Unique Users</div>
    <div class="chart-container">
      <canvas id="weeklyUsersChart"></canvas>
    </div>
  </div>
</div>

<div class="page-break"></div>

<div class="grid-2">
  <div class="section">
    <div class="section-title">Unlocks by Type</div>
    <div class="chart-container">
      <canvas id="unlockTypeChart"></canvas>
    </div>
  </div>
  <div class="section">
    <div class="section-title">Top 5 Most Used Doors</div>
    <table>
      <thead>
        <tr><th>Door</th><th>Unlocks</th></tr>
      </thead>
      <tbody id="top-doors-body"></tbody>
    </table>
  </div>
</div>

<div class="section">
  <div class="section-title">Recent Failed Unlock Attempts</div>
  <table>
    <thead>
      <tr>
        <th>Time</th>
        <th>User</th>
        <th>Door</th>
        <th>Reason</th>
      </tr>
    </thead>
    <tbody id="failed-attempts-body"></tbody>
  </table>
</div>
{{end}}

{{define "charts"}}
// --- Populate tables from data ---
(function() {
  const d = reportData;

  // Daily usage table
  const tbody1 = document.getElementById('daily-usage-body');
  (d.daily_usage || []).forEach(function(row) {
    const tr = document.createElement('tr');
    tr.innerHTML = '<td>'+row.date+'</td><td>'+row.unlocks+'</td><td>'+row.unique_users+'</td><td>'+row.occupancy+'%</td>';
    tbody1.appendChild(tr);
  });

  // Top doors table
  const tbody2 = document.getElementById('top-doors-body');
  (d.top_doors || []).slice(0, 5).forEach(function(row) {
    const tr = document.createElement('tr');
    tr.innerHTML = '<td>'+row.door+'</td><td>'+row.unlocks+'</td>';
    tbody2.appendChild(tr);
  });

  // Failed attempts table
  const tbody3 = document.getElementById('failed-attempts-body');
  (d.failed_attempts || []).slice(0, 15).forEach(function(row) {
    const tr = document.createElement('tr');
    tr.innerHTML = '<td>'+row.time+'</td><td>'+row.user+'</td><td>'+row.door+'</td><td>'+row.reason+'</td>';
    tbody3.appendChild(tr);
  });

  // Unlock type pie chart
  const typeLabels = Object.keys(d.unlocks_by_type || {});
  const typeValues = typeLabels.map(function(k) { return d.unlocks_by_type[k]; });
  const colors = ['#5046E5','#10B981','#F59E0B','#EF4444','#8B5CF6','#EC4899'];
  new Chart(document.getElementById('unlockTypeChart'), {
    type: 'doughnut',
    data: {
      labels: typeLabels,
      datasets: [{
        data: typeValues,
        backgroundColor: colors.slice(0, typeLabels.length),
        borderWidth: 1
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: { legend: { position: 'right', labels: { font: { size: 9 } } } }
    }
  });

  // Weekly unique users bar chart
  var weeklyData = d.weekly_unique_users || [];
  new Chart(document.getElementById('weeklyUsersChart'), {
    type: 'bar',
    data: {
      labels: weeklyData.map(function(w) { return w.week_label; }),
      datasets: [{
        label: 'Unique Users',
        data: weeklyData.map(function(w) { return w.unique_users; }),
        backgroundColor: '#5046E5',
        borderRadius: 3
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      scales: { y: { beginAtZero: true, ticks: { font: { size: 9 } } }, x: { ticks: { font: { size: 8 } } } },
      plugins: { legend: { display: false } }
    }
  });

  // Heatmap (matrix chart)
  var heatmapCells = d.heatmap_data || [];
  var users = [...new Set(heatmapCells.map(function(c) { return c.user; }))];
  var maxCount = Math.max(1, ...heatmapCells.map(function(c) { return c.count; }));
  new Chart(document.getElementById('heatmapChart'), {
    type: 'matrix',
    data: {
      datasets: [{
        data: heatmapCells.map(function(c) {
          return { x: c.hour, y: users.indexOf(c.user), v: c.count };
        }),
        backgroundColor: function(ctx) {
          var v = ctx.raw ? ctx.raw.v : 0;
          var alpha = Math.max(0.1, v / maxCount);
          return 'rgba(80,70,229,' + alpha + ')';
        },
        width: function(ctx) { return (ctx.chart.chartArea || {width:300}).width / 24 - 1; },
        height: function(ctx) { return (ctx.chart.chartArea || {height:200}).height / Math.max(users.length, 1) - 1; }
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      scales: {
        x: { type: 'linear', position: 'top', min: -0.5, max: 23.5,
          ticks: { stepSize: 1, font: { size: 7 }, callback: function(v) { return v + ':00'; } } },
        y: { type: 'linear', min: -0.5, max: users.length - 0.5,
          ticks: { stepSize: 1, font: { size: 8 }, callback: function(v) { return users[v] || ''; } } }
      },
      plugins: { legend: { display: false }, tooltip: {
        callbacks: { label: function(ctx) { return ctx.raw.v + ' unlocks'; } }
      } }
    }
  });
})();
{{end}}
```

- [ ] **Step 2: Run renderer test to verify template loads**

Run: `cd /Users/siky/code/MistyPass/.claude/worktrees/funny-shockley-960a91/api && go test ./internal/pdfgen/ -run TestRenderHTML_WeeklyAnalytics -v`
Expected: PASS — HTML rendered with tenant name, Chart.js, and reportData.

- [ ] **Step 3: Commit**

```bash
git add api/internal/pdfgen/templates/weekly_analytics.html
git commit -m "feat(pdfgen): add weekly analytics report template"
```

---

### Task 7: Events Template

**Files:**
- Create: `api/internal/pdfgen/templates/events.html`

- [ ] **Step 1: Create the template**

Create `api/internal/pdfgen/templates/events.html`:

```html
{{define "content"}}
<div class="kpi-row">
  <div class="kpi-card">
    <div class="kpi-value" id="kpi-total">--</div>
    <div class="kpi-label">Total Events</div>
  </div>
  <div class="kpi-card">
    <div class="kpi-value" id="kpi-granted" style="color:var(--success)">--</div>
    <div class="kpi-label">Granted</div>
  </div>
  <div class="kpi-card">
    <div class="kpi-value" id="kpi-denied" style="color:var(--danger)">--</div>
    <div class="kpi-label">Denied</div>
  </div>
  <div class="kpi-card">
    <div class="kpi-value" id="kpi-peak">--</div>
    <div class="kpi-label">Peak Hour</div>
  </div>
</div>

<div class="section">
  <div class="section-title">Hourly Distribution</div>
  <div class="chart-container">
    <canvas id="hourlyChart"></canvas>
  </div>
</div>

<div class="section">
  <div class="section-title">Access Events</div>
  <table>
    <thead>
      <tr><th>Time</th><th>User</th><th>Door</th><th>Result</th><th>Method</th></tr>
    </thead>
    <tbody id="events-body"></tbody>
  </table>
</div>
{{end}}

{{define "charts"}}
(function() {
  var d = reportData;
  document.getElementById('kpi-total').textContent = d.total_events || 0;
  document.getElementById('kpi-granted').textContent = d.granted || 0;
  document.getElementById('kpi-denied').textContent = d.denied || 0;
  document.getElementById('kpi-peak').textContent = d.peak_hour != null ? d.peak_hour + ':00' : '--';

  var hourly = d.hourly_dist || new Array(24).fill(0);
  var labels = [];
  for (var i = 0; i < 24; i++) labels.push(i + ':00');
  new Chart(document.getElementById('hourlyChart'), {
    type: 'bar',
    data: {
      labels: labels,
      datasets: [{
        label: 'Events',
        data: hourly,
        backgroundColor: '#5046E5',
        borderRadius: 2
      }]
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      scales: { y: { beginAtZero: true }, x: { ticks: { font: { size: 8 } } } },
      plugins: { legend: { display: false } }
    }
  });

  var tbody = document.getElementById('events-body');
  (d.events || []).slice(0, 50).forEach(function(e) {
    var tr = document.createElement('tr');
    var resultColor = e.result === 'granted' ? 'var(--success)' : 'var(--danger)';
    tr.innerHTML = '<td>'+e.time+'</td><td>'+e.user+'</td><td>'+e.door+'</td><td style="color:'+resultColor+';font-weight:600">'+e.result+'</td><td>'+e.method+'</td>';
    tbody.appendChild(tr);
  });
})();
{{end}}
```

- [ ] **Step 2: Commit**

```bash
git add api/internal/pdfgen/templates/events.html
git commit -m "feat(pdfgen): add events report template"
```

---

### Task 8: Unlock Stats Template

**Files:**
- Create: `api/internal/pdfgen/templates/unlock_stats.html`

- [ ] **Step 1: Create the template**

Create `api/internal/pdfgen/templates/unlock_stats.html`:

```html
{{define "content"}}
<div class="kpi-row">
  <div class="kpi-card">
    <div class="kpi-value" id="kpi-total">--</div>
    <div class="kpi-label">Total Unlocks</div>
  </div>
</div>

<div class="grid-2">
  <div class="section">
    <div class="section-title">Unlocks by Method</div>
    <div class="chart-container">
      <canvas id="methodChart"></canvas>
    </div>
  </div>
  <div class="section">
    <div class="section-title">Method Breakdown</div>
    <table>
      <thead><tr><th>Method</th><th>Count</th><th>%</th></tr></thead>
      <tbody id="method-body"></tbody>
    </table>
  </div>
</div>

<div class="section">
  <div class="section-title">Unlock Trend</div>
  <div class="chart-container">
    <canvas id="trendChart"></canvas>
  </div>
</div>
{{end}}

{{define "charts"}}
(function() {
  var d = reportData;
  document.getElementById('kpi-total').textContent = d.total || 0;

  var methods = Object.keys(d.by_method || {});
  var counts = methods.map(function(m) { return d.by_method[m]; });
  var total = counts.reduce(function(a, b) { return a + b; }, 0) || 1;
  var colors = ['#5046E5','#10B981','#F59E0B','#EF4444','#8B5CF6','#EC4899'];

  new Chart(document.getElementById('methodChart'), {
    type: 'doughnut',
    data: { labels: methods, datasets: [{ data: counts, backgroundColor: colors.slice(0, methods.length), borderWidth: 1 }] },
    options: { responsive: true, maintainAspectRatio: false, plugins: { legend: { position: 'right', labels: { font: { size: 9 } } } } }
  });

  var tbody = document.getElementById('method-body');
  methods.forEach(function(m, i) {
    var tr = document.createElement('tr');
    var pct = ((d.by_method[m] / total) * 100).toFixed(1);
    tr.innerHTML = '<td>'+m+'</td><td>'+d.by_method[m]+'</td><td>'+pct+'%</td>';
    tbody.appendChild(tr);
  });

  var trend = d.trend || [];
  new Chart(document.getElementById('trendChart'), {
    type: 'line',
    data: {
      labels: trend.map(function(t) { return t.date; }),
      datasets: [{ label: 'Unlocks', data: trend.map(function(t) { return t.count; }), borderColor: '#5046E5', backgroundColor: 'rgba(80,70,229,0.1)', fill: true, tension: 0.3, pointRadius: 2 }]
    },
    options: { responsive: true, maintainAspectRatio: false, scales: { y: { beginAtZero: true }, x: { ticks: { font: { size: 8 } } } }, plugins: { legend: { display: false } } }
  });
})();
{{end}}
```

- [ ] **Step 2: Commit**

```bash
git add api/internal/pdfgen/templates/unlock_stats.html
git commit -m "feat(pdfgen): add unlock stats report template"
```

---

### Task 9: User Presence Template

**Files:**
- Create: `api/internal/pdfgen/templates/user_presence.html`

- [ ] **Step 1: Create the template**

Create `api/internal/pdfgen/templates/user_presence.html`:

```html
{{define "content"}}
<div class="section">
  <div class="section-title">User Presence Heatmap</div>
  <div class="chart-container" style="max-height:280px">
    <canvas id="presenceHeatmap"></canvas>
  </div>
</div>

<div class="section">
  <div class="section-title">Daily Unique Users</div>
  <div class="chart-container">
    <canvas id="dailyUsersChart"></canvas>
  </div>
</div>

<div class="page-break"></div>

<div class="section">
  <div class="section-title">User Activity Summary</div>
  <table>
    <thead>
      <tr><th>User</th><th>First Seen</th><th>Last Seen</th><th>Total Events</th></tr>
    </thead>
    <tbody id="users-body"></tbody>
  </table>
</div>
{{end}}

{{define "charts"}}
(function() {
  var d = reportData;

  // Daily unique users line chart
  var daily = d.daily_unique_users || [];
  new Chart(document.getElementById('dailyUsersChart'), {
    type: 'line',
    data: {
      labels: daily.map(function(r) { return r.date; }),
      datasets: [{ label: 'Unique Users', data: daily.map(function(r) { return r.count; }), borderColor: '#5046E5', backgroundColor: 'rgba(80,70,229,0.1)', fill: true, tension: 0.3, pointRadius: 3 }]
    },
    options: { responsive: true, maintainAspectRatio: false, scales: { y: { beginAtZero: true } }, plugins: { legend: { display: false } } }
  });

  // Presence heatmap
  var cells = d.heatmap_data || [];
  var users = [...new Set(cells.map(function(c) { return c.user; }))];
  var dates = [...new Set(cells.map(function(c) { return c.date; }))].sort();
  var maxV = Math.max(1, ...cells.map(function(c) { return c.count; }));

  new Chart(document.getElementById('presenceHeatmap'), {
    type: 'matrix',
    data: {
      datasets: [{
        data: cells.map(function(c) { return { x: dates.indexOf(c.date), y: users.indexOf(c.user), v: c.count }; }),
        backgroundColor: function(ctx) {
          var v = ctx.raw ? ctx.raw.v : 0;
          return 'rgba(80,70,229,' + Math.max(0.1, v / maxV) + ')';
        },
        width: function(ctx) { return (ctx.chart.chartArea || {width:400}).width / Math.max(dates.length, 1) - 1; },
        height: function(ctx) { return (ctx.chart.chartArea || {height:200}).height / Math.max(users.length, 1) - 1; }
      }]
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      scales: {
        x: { type: 'linear', min: -0.5, max: dates.length - 0.5,
          ticks: { stepSize: 1, font: { size: 7 }, callback: function(v) { return dates[v] || ''; } } },
        y: { type: 'linear', min: -0.5, max: users.length - 0.5,
          ticks: { stepSize: 1, font: { size: 8 }, callback: function(v) { return users[v] || ''; } } }
      },
      plugins: { legend: { display: false } }
    }
  });

  // Users table
  var tbody = document.getElementById('users-body');
  (d.users || []).forEach(function(u) {
    var tr = document.createElement('tr');
    tr.innerHTML = '<td>'+u.user+'</td><td>'+u.first_seen+'</td><td>'+u.last_seen+'</td><td>'+u.total+'</td>';
    tbody.appendChild(tr);
  });
})();
{{end}}
```

- [ ] **Step 2: Commit**

```bash
git add api/internal/pdfgen/templates/user_presence.html
git commit -m "feat(pdfgen): add user presence report template"
```

---

### Task 10: Incidents Template

**Files:**
- Create: `api/internal/pdfgen/templates/incidents.html`

- [ ] **Step 1: Create the template**

Create `api/internal/pdfgen/templates/incidents.html`:

```html
{{define "content"}}
<div class="kpi-row">
  <div class="kpi-card">
    <div class="kpi-value" id="kpi-total">--</div>
    <div class="kpi-label">Total Incidents</div>
  </div>
  <div class="kpi-card">
    <div class="kpi-value" id="kpi-open" style="color:var(--danger)">--</div>
    <div class="kpi-label">Open</div>
  </div>
  <div class="kpi-card">
    <div class="kpi-value" id="kpi-resolved" style="color:var(--success)">--</div>
    <div class="kpi-label">Resolved</div>
  </div>
</div>

<div class="grid-2">
  <div class="section">
    <div class="section-title">By Severity</div>
    <div class="chart-container">
      <canvas id="severityChart"></canvas>
    </div>
  </div>
  <div class="section">
    <div class="section-title">By Status</div>
    <div class="chart-container">
      <canvas id="statusChart"></canvas>
    </div>
  </div>
</div>

<div class="section">
  <div class="section-title">Incident Timeline</div>
  <table>
    <thead>
      <tr><th>Time</th><th>Type</th><th>Severity</th><th>Door</th><th>Status</th></tr>
    </thead>
    <tbody id="incidents-body"></tbody>
  </table>
</div>
{{end}}

{{define "charts"}}
(function() {
  var d = reportData;
  document.getElementById('kpi-total').textContent = d.total || 0;
  document.getElementById('kpi-open').textContent = (d.by_status || {}).open || 0;
  document.getElementById('kpi-resolved').textContent = (d.by_status || {}).resolved || 0;

  var sevLabels = Object.keys(d.by_severity || {});
  var sevValues = sevLabels.map(function(k) { return d.by_severity[k]; });
  var sevColors = { critical: '#EF4444', high: '#F59E0B', medium: '#5046E5', low: '#10B981', unknown: '#6B7280' };

  new Chart(document.getElementById('severityChart'), {
    type: 'bar',
    data: {
      labels: sevLabels,
      datasets: [{ data: sevValues, backgroundColor: sevLabels.map(function(s) { return sevColors[s] || '#6B7280'; }), borderRadius: 3 }]
    },
    options: { responsive: true, maintainAspectRatio: false, scales: { y: { beginAtZero: true } }, plugins: { legend: { display: false } } }
  });

  var statLabels = Object.keys(d.by_status || {});
  var statValues = statLabels.map(function(k) { return d.by_status[k]; });
  var statColors = { open: '#EF4444', resolved: '#10B981', acknowledged: '#F59E0B' };

  new Chart(document.getElementById('statusChart'), {
    type: 'doughnut',
    data: {
      labels: statLabels,
      datasets: [{ data: statValues, backgroundColor: statLabels.map(function(s) { return statColors[s] || '#6B7280'; }), borderWidth: 1 }]
    },
    options: { responsive: true, maintainAspectRatio: false, plugins: { legend: { position: 'right', labels: { font: { size: 9 } } } } }
  });

  var tbody = document.getElementById('incidents-body');
  (d.incidents || []).slice(0, 30).forEach(function(inc) {
    var tr = document.createElement('tr');
    var sevStyle = 'font-weight:600;color:' + (sevColors[inc.severity] || '#6B7280');
    var statStyle = 'font-weight:600;color:' + (statColors[inc.status] || '#6B7280');
    tr.innerHTML = '<td>'+inc.time+'</td><td>'+inc.type+'</td><td style="'+sevStyle+'">'+inc.severity+'</td><td>'+inc.door+'</td><td style="'+statStyle+'">'+inc.status+'</td>';
    tbody.appendChild(tr);
  });
})();
{{end}}
```

- [ ] **Step 2: Commit**

```bash
git add api/internal/pdfgen/templates/incidents.html
git commit -m "feat(pdfgen): add incidents report template"
```

---

### Task 11: Hardware Template

**Files:**
- Create: `api/internal/pdfgen/templates/hardware.html`

- [ ] **Step 1: Create the template**

Create `api/internal/pdfgen/templates/hardware.html`:

```html
{{define "content"}}
<div class="kpi-row">
  <div class="kpi-card">
    <div class="kpi-value" id="kpi-online" style="color:var(--success)">--</div>
    <div class="kpi-label">Online</div>
  </div>
  <div class="kpi-card">
    <div class="kpi-value" id="kpi-offline" style="color:var(--danger)">--</div>
    <div class="kpi-label">Offline</div>
  </div>
</div>

<div class="grid-2">
  <div class="section">
    <div class="section-title">Device Status</div>
    <div class="chart-container">
      <canvas id="statusChart"></canvas>
    </div>
  </div>
  <div class="section">
    <div class="section-title">Battery Distribution</div>
    <div class="chart-container">
      <canvas id="batteryChart"></canvas>
    </div>
  </div>
</div>

<div class="section">
  <div class="section-title">Signal Strength Distribution</div>
  <div class="chart-container">
    <canvas id="signalChart"></canvas>
  </div>
</div>

<div class="section">
  <div class="section-title">Device Inventory</div>
  <table>
    <thead>
      <tr><th>Name</th><th>Type</th><th>Status</th><th>Battery</th><th>Signal</th><th>Last Seen</th></tr>
    </thead>
    <tbody id="devices-body"></tbody>
  </table>
</div>
{{end}}

{{define "charts"}}
(function() {
  var d = reportData;
  document.getElementById('kpi-online').textContent = d.online || 0;
  document.getElementById('kpi-offline').textContent = d.offline || 0;

  new Chart(document.getElementById('statusChart'), {
    type: 'doughnut',
    data: {
      labels: ['Online', 'Offline'],
      datasets: [{ data: [d.online || 0, d.offline || 0], backgroundColor: ['#10B981', '#EF4444'], borderWidth: 1 }]
    },
    options: { responsive: true, maintainAspectRatio: false, plugins: { legend: { position: 'right' } } }
  });

  var battery = d.battery_dist || [];
  new Chart(document.getElementById('batteryChart'), {
    type: 'bar',
    data: {
      labels: battery.map(function(b) { return b.label; }),
      datasets: [{ data: battery.map(function(b) { return b.count; }), backgroundColor: '#5046E5', borderRadius: 3 }]
    },
    options: { responsive: true, maintainAspectRatio: false, scales: { y: { beginAtZero: true } }, plugins: { legend: { display: false } } }
  });

  var signal = d.signal_dist || [];
  new Chart(document.getElementById('signalChart'), {
    type: 'bar',
    data: {
      labels: signal.map(function(s) { return s.label; }),
      datasets: [{ data: signal.map(function(s) { return s.count; }), backgroundColor: '#F59E0B', borderRadius: 3 }]
    },
    options: { responsive: true, maintainAspectRatio: false, scales: { y: { beginAtZero: true } }, plugins: { legend: { display: false } } }
  });

  var tbody = document.getElementById('devices-body');
  (d.devices || []).forEach(function(dev) {
    var tr = document.createElement('tr');
    var statusColor = dev.status === 'online' ? 'var(--success)' : 'var(--danger)';
    tr.innerHTML = '<td>'+dev.name+'</td><td>'+dev.type+'</td><td style="color:'+statusColor+';font-weight:600">'+dev.status+'</td><td>'+dev.battery+'%</td><td>'+dev.signal+'%</td><td>'+dev.last_seen+'</td>';
    tbody.appendChild(tr);
  });
})();
{{end}}
```

- [ ] **Step 2: Run all template parse tests**

Run: `cd /Users/siky/code/MistyPass/.claude/worktrees/funny-shockley-960a91/api && go test ./internal/pdfgen/ -v`
Expected: All tests PASS — all 6 templates parse, WeeklyAnalytics renders correctly, invalid type returns error.

- [ ] **Step 3: Commit**

```bash
git add api/internal/pdfgen/templates/hardware.html
git commit -m "feat(pdfgen): add hardware report template"
```

---

### Task 12: Unified Export API Endpoint

**Files:**
- Create: `api/internal/http/routes_report_export.go`
- Modify: `api/internal/http/router.go` (wire pdfgen + add route)

- [ ] **Step 1: Add pdfgen fields to server struct**

In `api/internal/http/router.go`, add to the `server` struct (near the report schedule fields):

```go
pdfRenderer    *pdfgen.Renderer
gotenbergClient *pdfgen.GotenbergClient
```

Add the import:

```go
"github.com/mistypass/cloud/api/internal/pdfgen"
```

In the `NewRouter` function, after service initialization, initialize pdfgen:

```go
pdfRenderer, err := pdfgen.NewRenderer()
if err != nil {
    return nil, nil, fmt.Errorf("init pdf renderer: %w", err)
}
gotenbergClient := pdfgen.NewGotenbergClient(cfg.GotenbergURL, nil)

s := &server{
    // ... existing fields ...
    pdfRenderer:     pdfRenderer,
    gotenbergClient: gotenbergClient,
}
```

In the route registration section, add the new export route inside the authenticated `/api/v1` group:

```go
r.Get("/reports/export", s.exportReport)
```

- [ ] **Step 2: Create routes_report_export.go**

Create `api/internal/http/routes_report_export.go`:

```go
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
	gateways := s.gatewaySvc.ListGateways(tenantID)
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
	t, err := s.tenantSvc.GetTenant(tenantID)
	if err != nil {
		return tenantID
	}
	return t.Name
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
	// Delegate to existing CSV builders where possible
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
```

Note: The `resolveDoorName` helper uses a simplified interface. During implementation, adapt it to the actual `space.Door` type's field names (likely `door.ID` and `door.Name`). Check `s.spaceSvc.ListDoors()` return type and adjust accordingly.

Similarly, `buildHardwareData` uses `s.gatewaySvc.ListGateways()` — verify the return type has `Name`, `Status`, `LastSeenAt` fields, or adapt to the actual gateway struct.

- [ ] **Step 3: Commit**

```bash
git add api/internal/http/routes_report_export.go api/internal/http/router.go
git commit -m "feat: add unified /reports/export endpoint with PDF rendering"
```

---

### Task 13: Upgrade Report Schedule Send with PDF Attachment

**Files:**
- Modify: `api/internal/http/routes_report_schedule.go`

- [ ] **Step 1: Update sendReportViaResend to support attachments**

In `api/internal/http/routes_report_schedule.go`, update the `sendReportViaResend` function signature and payload to support base64-encoded PDF attachments:

```go
type resendAttachment struct {
	Filename string `json:"filename"`
	Content  string `json:"content"` // base64-encoded
}

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
	// ... rest of function stays the same
}
```

- [ ] **Step 2: Update sendReportSchedule handler to render PDF and attach it**

Replace the section in `sendReportSchedule` that builds the HTML body (around lines 445-450) with:

```go
	// Render PDF report
	reportType := schedule.ReportType
	data, err := s.buildReportData(reportType, tenantID, "", periodStart, periodEnd)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to build report data: "+err.Error())
		return
	}
	meta := pdfgen.ReportMeta{
		TenantName:  s.resolveExportTenantName(tenantID),
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

	htmlBody := "<p>Please find your scheduled report attached.</p>"
	err = sendReportViaResend(r.Context(), resendEndpoint, resendAPIKey, emailFrom, schedule.Recipients, subject, htmlBody, attachments, resendTimeout)
```

Add the import for `"encoding/base64"` and `"github.com/mistypass/cloud/api/internal/pdfgen"` at the top of the file.

- [ ] **Step 3: Update all existing callers of sendReportViaResend**

Search for any other callers of `sendReportViaResend` and add the `attachments` parameter (pass `nil` if no attachment is needed).

- [ ] **Step 4: Commit**

```bash
git add api/internal/http/routes_report_schedule.go
git commit -m "feat: upgrade report schedule send to attach PDF via Resend API"
```

---

### Task 14: Remove Old generateSimplePDF

**Files:**
- Modify: `api/internal/http/routes_analytics.go`

- [ ] **Step 1: Update analytics export to use new renderer**

In `routes_analytics.go`, replace the `case "pdf"` block in `exportAnalytics` (around lines 397-401) to use the new pdfgen renderer:

```go
	case "pdf":
		data, err := s.buildReportData(reportType, tenantID, "", start, end)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		meta := pdfgen.ReportMeta{
			TenantName:  s.resolveExportTenantName(tenantID),
			PlaceName:   "All Locations",
			PeriodStart: start,
			PeriodEnd:   end,
			GeneratedAt: time.Now().UTC(),
		}
		// Map old analytics types to new report types
		pdfType := reportType
		switch reportType {
		case "access_summary":
			pdfType = "events"
		case "door_activity":
			pdfType = "weekly_analytics"
		case "alarm_metrics":
			pdfType = "incidents"
		}
		pdfBytes, err := s.pdfRenderer.RenderPDF(s.gotenbergClient, pdfType, meta, data)
		if err != nil {
			s.logger.Error("pdf render failed", "error", err)
			writeError(w, http.StatusBadGateway, "PDF rendering failed: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", reportType+".pdf"))
		w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
		w.Write(pdfBytes)
```

Add import for `"github.com/mistypass/cloud/api/internal/pdfgen"`.

- [ ] **Step 2: Update reference report download to use new renderer**

In `routes_reports.go`, replace the `if format == "pdf"` block in `downloadReferenceReport` (lines 188-195) similarly:

```go
	if format == "pdf" {
		meta := pdfgen.ReportMeta{
			TenantName:  s.resolveExportTenantName(tenantID),
			PlaceName:   "All Locations",
			PeriodStart: time.Now().Add(-7 * 24 * time.Hour),
			PeriodEnd:   time.Now(),
			GeneratedAt: time.Now().UTC(),
		}
		pdfType := "events"
		switch reportID {
		case referenceReportDoorAlarmID:
			pdfType = "incidents"
		case referenceReportAuditActivityID:
			pdfType = "events"
		}
		data, _ := s.buildReportData(pdfType, tenantID, placeID, meta.PeriodStart, meta.PeriodEnd)
		pdfBytes, err := s.pdfRenderer.RenderPDF(s.gotenbergClient, pdfType, meta, data)
		if err != nil {
			writeError(w, http.StatusBadGateway, "PDF rendering failed")
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", reportID+".pdf"))
		w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
		w.Write(pdfBytes)
		return
	}
```

- [ ] **Step 3: Delete old generateSimplePDF and pdfEscapeString**

Remove the `generateSimplePDF` function (lines 503-567) and `pdfEscapeString` function (lines 569-574) from `routes_analytics.go`. Also remove the `parseCSVToRows` helper if it's only used by the old PDF path (check first).

- [ ] **Step 4: Run full test suite**

Run: `cd /Users/siky/code/MistyPass/.claude/worktrees/funny-shockley-960a91/api && go build ./... && go test ./... -count=1`
Expected: Build succeeds, all tests pass.

- [ ] **Step 5: Commit**

```bash
git add api/internal/http/routes_analytics.go api/internal/http/routes_reports.go
git commit -m "refactor: replace generateSimplePDF with pdfgen renderer across all endpoints"
```

---

### Task 15: Background Report Scheduler

**Files:**
- Modify: `api/internal/http/router.go` (start scheduler goroutine)
- Modify: `api/internal/http/routes_report_schedule.go` (add scheduler loop)

- [ ] **Step 1: Add the scheduler loop function**

In `routes_report_schedule.go`, add the background scheduler:

```go
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

	// Update schedule timestamps
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
```

Add imports for `"context"`, `"encoding/base64"`, and `"github.com/mistypass/cloud/api/internal/pdfgen"` at the top.

- [ ] **Step 2: Start the scheduler in NewRouter**

In `router.go`, in the `NewRouter` function, after the server is fully initialized and before returning, start the scheduler goroutine and add its stop channel to the cleanup:

```go
schedulerStop := make(chan struct{})
go s.startReportScheduler(schedulerStop)

// Add to the stopWorkers cleanup function
originalStop := stopWorkers
stopWorkers = func() {
    close(schedulerStop)
    if originalStop != nil {
        originalStop()
    }
}
```

- [ ] **Step 3: Add NextRunAt field to reportSchedule if missing**

Verify the `reportSchedule` struct has a `NextRunAt` field. If not, add:

```go
NextRunAt string `json:"next_run_at,omitempty"`
```

Also ensure `Enabled` and `Frequency` fields exist (check the struct definition — the existing struct uses `Enabled bool` and `Frequency string`).

- [ ] **Step 4: Commit**

```bash
git add api/internal/http/routes_report_schedule.go api/internal/http/router.go
git commit -m "feat: add background report scheduler with PDF email delivery"
```

---

### Task 16: Integration Smoke Test

**Files:**
- None (manual verification)

- [ ] **Step 1: Start Gotenberg locally**

```bash
docker run --rm -p 3000:3000 gotenberg/gotenberg:8
```

- [ ] **Step 2: Start the API server**

```bash
cd /Users/siky/code/MistyPass/.claude/worktrees/funny-shockley-960a91/api
GOTENBERG_URL=http://localhost:3000 go run ./cmd/api/
```

- [ ] **Step 3: Test PDF export**

```bash
curl -o test_report.pdf "http://localhost:8080/api/v1/reports/export?type=weekly_analytics&format=pdf&tenant_id=tenant_demo_jakarta&start=2026-05-01T00:00:00Z&end=2026-05-23T23:59:59Z" -H "Authorization: Bearer <token>"
```

Open `test_report.pdf` and verify:
- Brand header with logo and tenant name
- Charts rendered (not blank canvases)
- Tables populated with data
- Professional layout matching Kisi style
- Page breaks working correctly

- [ ] **Step 4: Test each report type**

Repeat the curl for each type: `events`, `unlock_stats`, `user_presence`, `incidents`, `hardware`. Verify each produces a valid PDF with the correct charts and tables.
