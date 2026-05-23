# PDF Report Export System Design

**Date:** 2026-05-23
**Status:** Approved
**Approach:** Go HTML Templates + Gotenberg (Docker sidecar)

## Overview

Replace the existing minimal text-only PDF generator (`generateSimplePDF`) with a professional report system that produces Kisi-style PDF reports with charts, tables, heatmaps, and branding. Reports are rendered as HTML/CSS + Chart.js templates, converted to PDF by a Gotenberg sidecar service. The system serves both web and Android clients through the existing API, and supports scheduled email delivery.

## Architecture

```
Clients (Web Admin, Android, Cron Scheduler)
    │
    ▼
GET /api/v1/reports/export?type={type}&format=pdf&...
    │
    ▼
Go API Server
    │
    ├─ 1. Query data (events, doors, alarms, users, hardware)
    ├─ 2. Inject into Go html/template → full HTML document
    ├─ 3. POST HTML to Gotenberg API
    └─ 4. Return PDF bytes to client
    │
    ▼ HTTP POST (internal network)
Gotenberg (Docker sidecar, gotenberg/gotenberg:8)
    POST /forms/chromium/convert/html
    Chromium headless renders HTML → PDF
```

Gotenberg runs on an internal network only, not exposed externally. The Go API is the sole caller. Android and Web clients never contact Gotenberg directly.

## Package Structure

```
api/internal/pdfgen/
├── gotenberg.go              # Gotenberg HTTP client
├── renderer.go               # data → HTML → PDF pipeline
├── assets/
│   ├── logo.png              # MistyPass logo (base64-inlined into HTML)
│   └── fonts/                # Optional brand fonts
└── templates/
    ├── base.html             # Shared layout: header, footer, CSS, Chart.js
    ├── weekly_analytics.html
    ├── events.html
    ├── unlock_stats.html
    ├── user_presence.html
    ├── incidents.html
    └── hardware.html
```

## Gotenberg Client

`gotenberg.go` wraps communication with the Gotenberg HTTP API.

```go
type GotenbergClient struct {
    baseURL    string
    httpClient *http.Client
}

type PDFOptions struct {
    PaperWidth      float64 // 8.27 (A4)
    PaperHeight     float64 // 11.69 (A4)
    MarginTop       float64
    MarginBottom    float64
    MarginLeft      float64
    MarginRight     float64
    PrintBackground bool    // true — preserve chart colors and backgrounds
    WaitDelay       string  // "500ms" — wait for Chart.js to finish rendering
    Scale           float64 // 1.0
}

func (c *GotenbergClient) ConvertHTML(html []byte, opts PDFOptions) ([]byte, error)
```

The `ConvertHTML` method sends a `multipart/form-data` POST to `{baseURL}/forms/chromium/convert/html` with `index.html` as the file field and PDF options as form fields. Returns raw PDF bytes.

## Renderer Pipeline

`renderer.go` orchestrates the full flow.

```go
func (r *Renderer) Render(reportType string, tenantID string, placeID string, start time.Time, end time.Time) ([]byte, error)
```

Steps:
1. **Query data** — call the appropriate service methods based on `reportType` (eventSvc, alarmSvc, doorSvc, etc.)
2. **Assemble template data** — build a struct containing:
   - `ReportMeta`: tenant name, place name, date range, generated-at timestamp
   - `LogoBase64`: base64-encoded logo PNG for inline `<img>` embedding
   - `DataJSON`: JSON-serialized report-specific data for Chart.js consumption
3. **Render HTML** — `template.ParseFiles("base.html", "<report_type>.html")` then `tmpl.Execute(buf, data)` to produce a complete HTML document
4. **Convert to PDF** — `gotenbergClient.ConvertHTML(htmlBytes, defaultA4Opts)`
5. **Return PDF bytes**

## HTML Template Design

### base.html (Shared Layout)

Provides:
- **Brand header**: logo (base64 `<img>`), company name, place name, report date range
- **Page footer**: page number, "Generated: {timestamp}", MistyPass watermark
- **CSS variables**: `--brand-primary: #5046E5` (purple theme, referencing Kisi style)
- **`@page` rules**: A4 dimensions, margins
- **Chart.js**: `<script>` tag with Chart.js inlined (not CDN, to avoid network dependency inside Gotenberg)

Data injection pattern — Go template serializes query results as JSON into a `<script>` block:

```html
<script>
  const reportData = {{.DataJSON}};
  // Chart.js initialization follows...
</script>
```

Chromium in Gotenberg executes the JS, renders charts onto `<canvas>`, then captures the rendered page as PDF.

### Report Templates

Each template extends `base.html` and defines report-specific sections.

| Report | Tables | Chart Types |
|--------|--------|-------------|
| weekly_analytics | Daily usage, failed unlock attempts | Bar (daily activity), heatmap (user x hour), pie (unlock types), ranking table (top doors) |
| events | Access event details | Bar (hourly distribution), KPI cards (granted/denied) |
| unlock_stats | Unlock method breakdown | Pie (BLE/PIN/fingerprint/card), trend line chart |
| user_presence | User attendance records | Heatmap (user x date), daily unique users line chart |
| incidents | Alarm/incident records | Bar (by severity), pie (by status), timeline table |
| hardware | Device status inventory | Pie (online/offline), bar (battery/signal), device list table |

### Chart.js Heatmap

Chart.js does not natively support heatmaps. Use the `chartjs-chart-matrix` plugin for heatmap rendering (user x hour grids, user x date grids). The plugin is loaded alongside Chart.js in `base.html`.

## Unified Export API

### New Endpoint

```
GET /api/v1/reports/export?type={type}&format={format}&tenant_id=...&place_id=...&start=...&end=...
```

Parameters:
- `type` (required): `weekly_analytics`, `events`, `unlock_stats`, `user_presence`, `incidents`, `hardware`
- `format` (required): `pdf`, `csv`, `json`
- `tenant_id` (required): tenant scope
- `place_id` (optional): place scope
- `start` (required): RFC3339 period start
- `end` (required): RFC3339 period end

Response:
- `format=pdf`: `Content-Type: application/pdf`, `Content-Disposition: attachment; filename="..."`, raw PDF bytes
- `format=csv`: `Content-Type: text/csv`, CSV body
- `format=json`: `Content-Type: application/json`, JSON body

### Migration from Old Endpoints

The existing endpoints continue to work but internally delegate to the new `pdfgen.Renderer`:
- `GET /api/v1/analytics/export?format=pdf` — maps `type` parameter to the new renderer
- `GET /api/v1/reports/{reportID}/download?format=pdf` — maps `reportID` to the new renderer

Once the new unified endpoint is stable, the old endpoints can be deprecated.

### Android Compatibility

The Android `AdminExportScreen` calls `adminRepository.exportReport(placeId, request)` which hits the backend export API. The 6 report types in the Android UI (`weekly_analytics`, `events`, `unlock_stats`, `user_presence`, `incidents`, `hardware`) map directly to the new `type` parameter. No Android code changes needed — the backend returns a professional PDF instead of the old text-only version.

## Scheduled Reports

### ReportScheduler

A background goroutine that executes scheduled report deliveries.

```go
type ReportScheduler struct {
    renderer  *Renderer
    mailer    *Mailer
    store     ScheduleStore   // existing scheduled report storage
    ticker    *time.Ticker    // 1-minute tick
}
```

On each tick:
1. Load all schedules with `status=active`
2. For each schedule where `now >= nextRunAt`:
   - Compute date range based on cadence:
     - `daily` → previous 1 day
     - `weekly` → previous 7 days
     - `monthly` → previous calendar month
     - `quarterly` → previous 3 months
   - Call `renderer.Render(type, tenantID, placeID, start, end)`
   - Call `mailer.Send(recipients, subject, pdfBytes)`
   - Update `lastRunAt` and compute `nextRunAt`

### Manual Trigger

```
POST /api/v1/report-schedules/{id}/run
→ 200 { "status": "sent", "recipients": ["..."] }
```

Immediately executes a scheduled report (useful for testing and preview).

### Email Format

```
From:    reports@mistypass.com
Subject: [MistyPass] Weekly Analytics Report — {PlaceName} ({start} - {end})
Body:    "Please find attached your scheduled report."
Attach:  weekly_analytics_2026-05-17_2026-05-23.pdf
```

### Mailer

Uses Go `net/smtp` (standard library) with `PLAIN` auth. No external mail library needed.

### SMTP Configuration

Via environment variables:

```
SMTP_HOST=...
SMTP_PORT=587
SMTP_USER=...
SMTP_PASSWORD=...
SMTP_FROM=reports@mistypass.com
```

## Docker Deployment

Add Gotenberg to the existing docker-compose:

```yaml
services:
  api:
    environment:
      - GOTENBERG_URL=http://gotenberg:3000
    depends_on:
      - gotenberg

  gotenberg:
    image: gotenberg/gotenberg:8
    restart: unless-stopped
    command:
      - "gotenberg"
      - "--api-timeout=30s"
      - "--chromium-allow-list=file:///tmp/.*"
    networks:
      - internal
    deploy:
      resources:
        limits:
          memory: 512M
```

Resource estimates:
- Gotenberg idle: ~80MB RAM
- Per-render peak: ~200-300MB (Chromium startup + rendering)
- Minimum VPS requirement: **1GB RAM** (API + Gotenberg combined)
- Render time per report: ~1-3 seconds

Concurrency: Gotenberg queues requests by default. Scheduled batch generation runs serially to avoid memory spikes.

Health check: API pings `GOTENBERG_URL/health` on startup, logs connectivity status. If unreachable, PDF export endpoints return 503 without affecting other API functionality.

## Error Handling

- Gotenberg unavailable: return HTTP 503 with `{"error": "PDF rendering service unavailable"}`
- Gotenberg timeout (>30s): return HTTP 504
- Invalid report type: return HTTP 400
- No data for date range: generate PDF with "No data available" message (not an error)
- SMTP failure on scheduled report: log error, do not retry immediately, will retry on next tick cycle

## Out of Scope

- No changes to Android app code (API-compatible, PDF quality upgrades automatically)
- No changes to web admin export button logic (still downloads blob)
- No in-browser PDF preview (direct download only)
- No multi-language templates (English only for v1)
- No PDF password protection or encryption

## Implementation Order

1. Docker: add Gotenberg service to docker-compose
2. `pdfgen` package: `gotenberg.go` client + `renderer.go` pipeline
3. `base.html`: shared layout with branding, CSS, Chart.js
4. Report templates: implement all 6 HTML templates with charts
5. Unified export endpoint + replace old `generateSimplePDF` calls
6. Scheduled report executor + email delivery
7. Remove old `generateSimplePDF` and `pdfEscapeString` code
