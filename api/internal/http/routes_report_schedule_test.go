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
