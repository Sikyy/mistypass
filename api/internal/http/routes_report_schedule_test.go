package httpx

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/pdfgen"
)

func TestNormalizeReportScheduleReportType(t *testing.T) {
	tests := map[string]string{
		"weekly_analytics": "weekly_analytics",
		"access_summary":   "weekly_analytics",
		"events":           "events",
		"visitor_log":      "events",
		"unlock_stats":     "unlock_stats",
		"door_usage":       "unlock_stats",
		"alarm_history":    "incidents",
		"alarm_metrics":    "incidents",
		"hardware":         "hardware",
	}

	for input, expected := range tests {
		got, ok := normalizeReportScheduleReportType(input)
		if !ok {
			t.Fatalf("expected %q to normalize", input)
		}
		if got != expected {
			t.Fatalf("normalize %q = %q, want %q", input, got, expected)
		}
	}

	if _, ok := normalizeReportScheduleReportType("unknown"); ok {
		t.Fatal("expected unknown report type to be rejected")
	}
}

func TestBuildReportEmailHTMLUsesMistyisletBrand(t *testing.T) {
	meta := pdfgen.ReportMeta{
		TenantName:  "Test Tenant",
		PlaceName:   "Jakarta HQ",
		PeriodStart: time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC),
		GeneratedAt: time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC),
	}

	body := buildReportEmailHTML("Weekly ops", "weekly_analytics", meta, "weekly_analytics.pdf")
	for _, expected := range []string{"Mistyislet", "Weekly ops", "Weekly Analytics", "Jakarta HQ", "weekly_analytics.pdf"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected email body to contain %q, body=%s", expected, body)
		}
	}
	if strings.Contains(body, "Please find your scheduled report attached") {
		t.Fatal("expected placeholder email copy to be removed")
	}
}

func TestReportScheduleProviderStatus(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "report-provider-status-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	recorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/report-schedules/provider-status?tenant_id=tenant_demo_jakarta", token, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected provider status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var status reportScheduleProviderStatus
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode provider status: %v", err)
	}
	if status.Provider != "resend" || status.Enabled || status.Configured || status.Ready {
		t.Fatalf("unexpected disabled provider status: %+v", status)
	}
	if !containsString(status.Missing, "REPORT_EMAIL_ENABLED") || !containsString(status.Missing, "USER_INVITATION_RESEND_API_KEY") {
		t.Fatalf("expected missing enabled/api key fields, got %+v", status)
	}

	readyRouter, _, err := NewRouter(config.Config{
		JWTSecret:                    "report-provider-ready-test-secret",
		EnableDemoUsers:              true,
		ReportEmailEnabled:           true,
		ReportEmailFrom:              "reports@mistypass.local",
		UserInvitationEmailFrom:      "invites@mistypass.local",
		UserInvitationResendEndpoint: "https://api.resend.test/emails",
		UserInvitationResendAPIKey:   "re_test_key",
		UserInvitationResendTimeout:  9 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("expected ready router: %v", err)
	}
	readyToken := referenceAPILogin(t, readyRouter, "organization.admin@mistypass.local")

	readyRecorder := referenceAPIRequest(t, readyRouter, http.MethodGet, "/api/v1/report-schedules/provider-status?tenant_id=tenant_demo_jakarta", readyToken, nil)
	if readyRecorder.Code != http.StatusOK {
		t.Fatalf("expected ready provider status 200, got %d body=%s", readyRecorder.Code, readyRecorder.Body.String())
	}
	var readyStatus reportScheduleProviderStatus
	if err := json.Unmarshal(readyRecorder.Body.Bytes(), &readyStatus); err != nil {
		t.Fatalf("decode ready provider status: %v", err)
	}
	if !readyStatus.Enabled || !readyStatus.Configured || !readyStatus.Ready {
		t.Fatalf("expected ready provider status, got %+v", readyStatus)
	}
	if readyStatus.From != "reports@mistypass.local" || readyStatus.Endpoint != "https://api.resend.test/emails" || readyStatus.TimeoutSeconds != 9 {
		t.Fatalf("unexpected ready provider details: %+v", readyStatus)
	}

	cloudflareRouter, _, err := NewRouter(config.Config{
		JWTSecret:                "report-provider-cloudflare-test-secret",
		EnableDemoUsers:          true,
		MailProvider:             "cloudflare",
		ReportEmailEnabled:       true,
		ReportEmailFrom:          "reports@mistypass.local",
		UserInvitationEmailFrom:  "invites@mistypass.local",
		CloudflareEmailEndpoint:  "https://api.cloudflare.test/accounts/{account_id}/email/sending/send",
		CloudflareEmailAccountID: "cf_account_123",
		CloudflareEmailAPIToken:  "cf_email_token",
		CloudflareEmailTimeout:   7 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("expected cloudflare router: %v", err)
	}
	cloudflareToken := referenceAPILogin(t, cloudflareRouter, "organization.admin@mistypass.local")

	cloudflareRecorder := referenceAPIRequest(t, cloudflareRouter, http.MethodGet, "/api/v1/report-schedules/provider-status?tenant_id=tenant_demo_jakarta", cloudflareToken, nil)
	if cloudflareRecorder.Code != http.StatusOK {
		t.Fatalf("expected cloudflare provider status 200, got %d body=%s", cloudflareRecorder.Code, cloudflareRecorder.Body.String())
	}
	var cloudflareStatus reportScheduleProviderStatus
	if err := json.Unmarshal(cloudflareRecorder.Body.Bytes(), &cloudflareStatus); err != nil {
		t.Fatalf("decode cloudflare provider status: %v", err)
	}
	if cloudflareStatus.Provider != "cloudflare" || !cloudflareStatus.Enabled || !cloudflareStatus.Configured || !cloudflareStatus.Ready {
		t.Fatalf("expected ready cloudflare provider status, got %+v", cloudflareStatus)
	}
	if cloudflareStatus.From != "reports@mistypass.local" ||
		cloudflareStatus.Endpoint != "https://api.cloudflare.test/accounts/cf_account_123/email/sending/send" ||
		cloudflareStatus.TimeoutSeconds != 7 {
		t.Fatalf("unexpected cloudflare provider details: %+v", cloudflareStatus)
	}

	cloudflareFallbackRouter, _, err := NewRouter(config.Config{
		JWTSecret:                "report-provider-cloudflare-fallback-test-secret",
		EnableDemoUsers:          true,
		MailProvider:             "cloudflare",
		ReportEmailEnabled:       true,
		ReportEmailFrom:          "reports@mistypass.local",
		UserInvitationEmailFrom:  "invites@mistypass.local",
		CloudflareEmailEndpoint:  "https://api.cloudflare.test/accounts/{account_id}/email/sending/send",
		CloudflareEmailAccountID: "cf_account_123",
		CloudflareEmailAPIToken:  "cf_email_token",
		CloudflareEmailTimeout:   500 * time.Millisecond,
	}, nil)
	if err != nil {
		t.Fatalf("expected cloudflare fallback router: %v", err)
	}
	cloudflareFallbackToken := referenceAPILogin(t, cloudflareFallbackRouter, "organization.admin@mistypass.local")

	cloudflareFallbackRecorder := referenceAPIRequest(t, cloudflareFallbackRouter, http.MethodGet, "/api/v1/report-schedules/provider-status?tenant_id=tenant_demo_jakarta", cloudflareFallbackToken, nil)
	if cloudflareFallbackRecorder.Code != http.StatusOK {
		t.Fatalf("expected cloudflare fallback provider status 200, got %d body=%s", cloudflareFallbackRecorder.Code, cloudflareFallbackRecorder.Body.String())
	}
	var cloudflareFallbackStatus reportScheduleProviderStatus
	if err := json.Unmarshal(cloudflareFallbackRecorder.Body.Bytes(), &cloudflareFallbackStatus); err != nil {
		t.Fatalf("decode cloudflare fallback provider status: %v", err)
	}
	if cloudflareFallbackStatus.TimeoutSeconds != 15 {
		t.Fatalf("expected 15s cloudflare fallback timeout, got %+v", cloudflareFallbackStatus)
	}
}

func TestCreateReportScheduleSetsNextRunAt(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "report-schedule-nextrun-create-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	body := []byte(`{"tenant_id":"tenant_demo_jakarta","name":"Daily ops","report_type":"events","frequency":"daily","recipients":["ops@example.com"],"format":"pdf"}`)
	recorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/report-schedules", token, body)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var created reportSchedule
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created schedule: %v", err)
	}
	if created.NextRunAt == "" {
		t.Fatal("expected next_run_at to be set on create so the background scheduler can pick the schedule up")
	}
	nextRun, err := time.Parse(time.RFC3339, created.NextRunAt)
	if err != nil {
		t.Fatalf("parse next_run_at: %v", err)
	}
	until := time.Until(nextRun)
	if until < 23*time.Hour || until > 25*time.Hour {
		t.Fatalf("expected daily schedule next_run_at ~24h out, got %s (in %s)", created.NextRunAt, until)
	}
}

func TestUpdateReportScheduleRecomputesNextRunAtOnFrequencyChange(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "report-schedule-nextrun-update-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	createBody := []byte(`{"tenant_id":"tenant_demo_jakarta","name":"Ops digest","report_type":"events","frequency":"daily","recipients":["ops@example.com"],"format":"pdf"}`)
	createRecorder := referenceAPIRequest(t, router, http.MethodPost, "/api/v1/report-schedules", token, createBody)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected create 201, got %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created reportSchedule
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created schedule: %v", err)
	}

	updateBody := []byte(`{"tenant_id":"tenant_demo_jakarta","frequency":"weekly"}`)
	updateRecorder := referenceAPIRequest(t, router, http.MethodPatch, "/api/v1/report-schedules/"+created.ID, token, updateBody)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated reportSchedule
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated schedule: %v", err)
	}
	nextRun, err := time.Parse(time.RFC3339, updated.NextRunAt)
	if err != nil {
		t.Fatalf("parse updated next_run_at %q: %v", updated.NextRunAt, err)
	}
	until := time.Until(nextRun)
	if until < 6*24*time.Hour+23*time.Hour || until > 7*24*time.Hour+time.Hour {
		t.Fatalf("expected weekly schedule next_run_at ~7d out after frequency change, got %s (in %s)", updated.NextRunAt, until)
	}
}

func TestRunScheduledReportsBackfillsMissingNextRunAt(t *testing.T) {
	s := &server{
		cfg: config.Config{ReportEmailEnabled: true},
		reportSchedules: map[string]reportSchedule{
			"rs_legacy": {ID: "rs_legacy", TenantID: "tenant_demo_jakarta", Frequency: "weekly", Enabled: true},
		},
	}

	s.runScheduledReports()

	s.reportScheduleMu.RLock()
	got := s.reportSchedules["rs_legacy"]
	s.reportScheduleMu.RUnlock()
	if got.NextRunAt == "" {
		t.Fatal("expected runScheduledReports to backfill next_run_at for legacy schedules persisted without one")
	}
	nextRun, err := time.Parse(time.RFC3339, got.NextRunAt)
	if err != nil {
		t.Fatalf("parse backfilled next_run_at %q: %v", got.NextRunAt, err)
	}
	if until := time.Until(nextRun); until < 6*24*time.Hour+23*time.Hour || until > 7*24*time.Hour+time.Hour {
		t.Fatalf("expected backfilled weekly next_run_at ~7d out, got %s (in %s)", got.NextRunAt, until)
	}
}
