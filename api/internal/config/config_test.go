package config

import (
	"testing"
	"time"
)

func TestFromEnvDemoUsersDefaultByAppEnv(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("ENABLE_DEMO_USERS", "")
	cfg := FromEnv()
	if cfg.EnableDemoUsers {
		t.Fatalf("expected demo users disabled by default in development env")
	}

	t.Setenv("APP_ENV", "production")
	t.Setenv("ENABLE_DEMO_USERS", "")
	cfg = FromEnv()
	if cfg.EnableDemoUsers {
		t.Fatalf("expected demo users disabled by default in production env")
	}
}

func TestFromEnvDemoUsersCanBeOverridden(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("ENABLE_DEMO_USERS", "true")
	cfg := FromEnv()
	if !cfg.EnableDemoUsers {
		t.Fatalf("expected demo users override to be true")
	}
}

func TestFromEnvUserInvitationProviderDefaultsAndOverrides(t *testing.T) {
	t.Setenv("USER_INVITATION_EMAIL_PROVIDER", "")
	t.Setenv("USER_INVITATION_EMAIL_FROM", "")
	t.Setenv("USER_INVITATION_RESEND_ENDPOINT", "")
	t.Setenv("USER_INVITATION_RESEND_API_KEY", "")
	t.Setenv("USER_INVITATION_RESEND_TIMEOUT", "")
	t.Setenv("USER_INVITATION_PROVIDER_WEBHOOK_SECRET", "")
	t.Setenv("EMAIL_INBOUND_WEBHOOK_SECRET", "")

	cfg := FromEnv()
	if cfg.UserInvitationEmailProvider != "queue" {
		t.Fatalf("default invitation provider mismatch: got %s", cfg.UserInvitationEmailProvider)
	}
	if cfg.UserInvitationEmailFrom != "no-reply@mistypass.local" {
		t.Fatalf("default invitation from mismatch: got %s", cfg.UserInvitationEmailFrom)
	}
	if cfg.UserInvitationResendTimeout != 5*time.Second {
		t.Fatalf("default invitation resend timeout mismatch: got %s", cfg.UserInvitationResendTimeout)
	}
	if cfg.UserInvitationProviderWebhookSecret != "" {
		t.Fatalf("default invitation webhook secret mismatch: got %s", cfg.UserInvitationProviderWebhookSecret)
	}
	if cfg.EmailInboundWebhookSecret != "" {
		t.Fatalf("default email inbound webhook secret mismatch: got %s", cfg.EmailInboundWebhookSecret)
	}
	if cfg.MailProvider != "" {
		t.Fatalf("default mail provider mismatch: got %s", cfg.MailProvider)
	}
	if cfg.CloudflareEmailTimeout != 15*time.Second {
		t.Fatalf("default cloudflare email timeout mismatch: got %s", cfg.CloudflareEmailTimeout)
	}

	t.Setenv("USER_INVITATION_EMAIL_PROVIDER", "cloudflare")
	t.Setenv("USER_INVITATION_EMAIL_FROM", "invites@mistypass.local")
	t.Setenv("USER_INVITATION_RESEND_ENDPOINT", "https://api.resend.com/emails")
	t.Setenv("USER_INVITATION_RESEND_API_KEY", "re_invite_token")
	t.Setenv("USER_INVITATION_RESEND_TIMEOUT", "9s")
	t.Setenv("USER_INVITATION_PROVIDER_WEBHOOK_SECRET", "invite-webhook-secret")
	t.Setenv("EMAIL_INBOUND_WEBHOOK_SECRET", "email-inbound-secret")
	t.Setenv("MAIL_PROVIDER", "cloudflare")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "cf_account_123")
	t.Setenv("CLOUDFLARE_EMAIL_ENDPOINT", "https://api.cloudflare.test/accounts/{account_id}/email/sending/send")
	t.Setenv("CLOUDFLARE_EMAIL_API_TOKEN", "cf_email_token")
	t.Setenv("CLOUDFLARE_EMAIL_TIMEOUT", "7s")

	cfg = FromEnv()
	if cfg.UserInvitationEmailProvider != "cloudflare" {
		t.Fatalf("override invitation provider mismatch: got %s", cfg.UserInvitationEmailProvider)
	}
	if cfg.UserInvitationEmailFrom != "invites@mistypass.local" {
		t.Fatalf("override invitation from mismatch: got %s", cfg.UserInvitationEmailFrom)
	}
	if cfg.UserInvitationResendEndpoint != "https://api.resend.com/emails" {
		t.Fatalf("override invitation resend endpoint mismatch: got %s", cfg.UserInvitationResendEndpoint)
	}
	if cfg.UserInvitationResendAPIKey != "re_invite_token" {
		t.Fatalf("override invitation resend api key mismatch: got %s", cfg.UserInvitationResendAPIKey)
	}
	if cfg.UserInvitationResendTimeout != 9*time.Second {
		t.Fatalf("override invitation resend timeout mismatch: got %s", cfg.UserInvitationResendTimeout)
	}
	if cfg.UserInvitationProviderWebhookSecret != "invite-webhook-secret" {
		t.Fatalf("override invitation webhook secret mismatch: got %s", cfg.UserInvitationProviderWebhookSecret)
	}
	if cfg.EmailInboundWebhookSecret != "email-inbound-secret" {
		t.Fatalf("override email inbound webhook secret mismatch: got %s", cfg.EmailInboundWebhookSecret)
	}
	if cfg.MailProvider != "cloudflare" {
		t.Fatalf("override mail provider mismatch: got %s", cfg.MailProvider)
	}
	if cfg.CloudflareEmailAccountID != "cf_account_123" {
		t.Fatalf("override cloudflare account id mismatch: got %s", cfg.CloudflareEmailAccountID)
	}
	if cfg.CloudflareEmailEndpoint != "https://api.cloudflare.test/accounts/{account_id}/email/sending/send" {
		t.Fatalf("override cloudflare endpoint mismatch: got %s", cfg.CloudflareEmailEndpoint)
	}
	if cfg.CloudflareEmailAPIToken != "cf_email_token" {
		t.Fatalf("override cloudflare api token mismatch: got %s", cfg.CloudflareEmailAPIToken)
	}
	if cfg.CloudflareEmailTimeout != 7*time.Second {
		t.Fatalf("override cloudflare timeout mismatch: got %s", cfg.CloudflareEmailTimeout)
	}

	t.Setenv("USER_INVITATION_EMAIL_PROVIDER", "invalid-provider")
	t.Setenv("USER_INVITATION_RESEND_TIMEOUT", "500ms")
	t.Setenv("MAIL_PROVIDER", "invalid-provider")
	t.Setenv("CLOUDFLARE_EMAIL_TIMEOUT", "500ms")
	cfg = FromEnv()
	if cfg.UserInvitationEmailProvider != "queue" {
		t.Fatalf("invalid invitation provider should fallback to queue, got %s", cfg.UserInvitationEmailProvider)
	}
	if cfg.UserInvitationResendTimeout != 5*time.Second {
		t.Fatalf("sub-second invitation resend timeout should fallback, got %s", cfg.UserInvitationResendTimeout)
	}
	if cfg.MailProvider != "" {
		t.Fatalf("invalid mail provider should fallback empty, got %s", cfg.MailProvider)
	}
	if cfg.CloudflareEmailTimeout != 15*time.Second {
		t.Fatalf("sub-second cloudflare timeout should fallback, got %s", cfg.CloudflareEmailTimeout)
	}
}

func TestFromEnvReportEmailConfig(t *testing.T) {
	t.Setenv("REPORT_EMAIL_ENABLED", "")
	t.Setenv("REPORT_EMAIL_FROM", "")
	t.Setenv("GOTENBERG_URL", "")

	cfg := FromEnv()
	if cfg.ReportEmailEnabled {
		t.Fatalf("expected report email disabled by default")
	}
	if cfg.ReportEmailFrom != "" {
		t.Fatalf("default report email from mismatch: got %s", cfg.ReportEmailFrom)
	}
	if cfg.GotenbergURL != "http://localhost:3000" {
		t.Fatalf("default gotenberg url mismatch: got %s", cfg.GotenbergURL)
	}

	t.Setenv("REPORT_EMAIL_ENABLED", "true")
	t.Setenv("REPORT_EMAIL_FROM", "reports@mistyislet.com")
	t.Setenv("GOTENBERG_URL", "http://gotenberg:3000")

	cfg = FromEnv()
	if !cfg.ReportEmailEnabled {
		t.Fatalf("expected report email enabled override")
	}
	if cfg.ReportEmailFrom != "reports@mistyislet.com" {
		t.Fatalf("override report email from mismatch: got %s", cfg.ReportEmailFrom)
	}
	if cfg.GotenbergURL != "http://gotenberg:3000" {
		t.Fatalf("override gotenberg url mismatch: got %s", cfg.GotenbergURL)
	}
}

func TestFromEnvCORSOriginDefaultAllowsLocalDevelopmentHosts(t *testing.T) {
	t.Setenv("CORS_ORIGIN", "")
	cfg := FromEnv()
	if cfg.CORSOrigin != "http://localhost:5173,http://localhost:5174,http://localhost:5175,http://127.0.0.1:5173,http://127.0.0.1:5175" {
		t.Fatalf("unexpected CORS origin default: %q", cfg.CORSOrigin)
	}
}

func TestConfigValidateRequiresJWTSecretInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("HRIS_VAULT_MASTER_KEY", "vault-master-key-001")
	t.Setenv("GATEWAY_BOOTSTRAP_TOKEN", "bootstrap-token-001")
	t.Setenv("ENABLE_DEMO_USERS", "false")
	t.Setenv("DISABLE_LOGIN_RATE_LIMIT", "false")
	cfg := FromEnv()
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validate to fail when JWT_SECRET is empty in production")
	}

	t.Setenv("JWT_SECRET", "test-secret")
	cfg = FromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validate to pass with JWT_SECRET in production: %v", err)
	}
}

func TestConfigValidateRequiresHRISVaultMasterKeyInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("HRIS_VAULT_MASTER_KEY", "")
	t.Setenv("GATEWAY_BOOTSTRAP_TOKEN", "bootstrap-token-001")
	t.Setenv("ENABLE_DEMO_USERS", "false")
	t.Setenv("DISABLE_LOGIN_RATE_LIMIT", "false")

	cfg := FromEnv()
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validate to fail when HRIS_VAULT_MASTER_KEY is empty in production")
	}

	t.Setenv("HRIS_VAULT_MASTER_KEY", "vault-master-key-001")
	cfg = FromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validate to pass with HRIS_VAULT_MASTER_KEY in production: %v", err)
	}
}

func TestConfigValidateRequiresGatewayBootstrapTokenInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("HRIS_VAULT_MASTER_KEY", "vault-master-key-001")
	t.Setenv("GATEWAY_BOOTSTRAP_TOKEN", "")
	t.Setenv("ENABLE_DEMO_USERS", "false")
	t.Setenv("DISABLE_LOGIN_RATE_LIMIT", "false")

	cfg := FromEnv()
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validate to fail when GATEWAY_BOOTSTRAP_TOKEN is empty in production")
	}

	t.Setenv("GATEWAY_BOOTSTRAP_TOKEN", "bootstrap-token-001")
	cfg = FromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validate to pass with GATEWAY_BOOTSTRAP_TOKEN in production: %v", err)
	}
}

func TestFromEnvGatewayMTLSConfig(t *testing.T) {
	t.Setenv("GATEWAY_MTLS_ADDR", "9443")
	t.Setenv("GATEWAY_MTLS_SERVER_CERT_PEM", "server-cert")
	t.Setenv("GATEWAY_MTLS_SERVER_KEY_PEM", "server-key")
	t.Setenv("GATEWAY_CA_CERT_PEM", "ca-cert")
	t.Setenv("GATEWAY_CA_KEY_PEM", "ca-key")
	t.Setenv("GATEWAY_MTLS_CERT_LIFETIME", "12h")
	t.Setenv("GATEWAY_MTLS_REVOKED_SERIALS", "01:02, 0A0B,01:02")

	cfg := FromEnv()
	if cfg.GatewayMTLSAddr != ":9443" {
		t.Fatalf("expected normalized gateway mTLS addr, got %q", cfg.GatewayMTLSAddr)
	}
	if cfg.GatewayMTLSServerCertPEM != "server-cert" {
		t.Fatalf("unexpected gateway mTLS server cert")
	}
	if cfg.GatewayMTLSServerKeyPEM != "server-key" {
		t.Fatalf("unexpected gateway mTLS server key")
	}
	if cfg.GatewayCACertPEM != "ca-cert" || cfg.GatewayCAKeyPEM != "ca-key" {
		t.Fatalf("unexpected gateway CA config")
	}
	if cfg.GatewayMTLSCertLifetime != 12*time.Hour {
		t.Fatalf("unexpected gateway mTLS cert lifetime: %s", cfg.GatewayMTLSCertLifetime)
	}
	if len(cfg.GatewayMTLSRevokedSerials) != 2 || cfg.GatewayMTLSRevokedSerials[0] != "01:02" || cfg.GatewayMTLSRevokedSerials[1] != "0A0B" {
		t.Fatalf("unexpected gateway mTLS revoked serials: %+v", cfg.GatewayMTLSRevokedSerials)
	}
}

func TestFromEnvGatewayRequireRequestNonce(t *testing.T) {
	t.Setenv("GATEWAY_REQUIRE_REQUEST_NONCE", "")
	t.Setenv("GATEWAY_WS_MAX_SESSION_TTL", "")
	t.Setenv("GATEWAY_MTLS_CERT_LIFETIME", "")
	cfg := FromEnv()
	if cfg.GatewayRequireRequestNonce {
		t.Fatalf("expected gateway request nonce requirement disabled by default")
	}
	if cfg.GatewayWebSocketMaxSessionTTL != 6*time.Hour {
		t.Fatalf("expected default gateway WS max session TTL 6h, got %s", cfg.GatewayWebSocketMaxSessionTTL)
	}
	if cfg.GatewayMTLSCertLifetime != 24*time.Hour {
		t.Fatalf("expected default gateway mTLS cert lifetime 24h, got %s", cfg.GatewayMTLSCertLifetime)
	}

	t.Setenv("GATEWAY_REQUIRE_REQUEST_NONCE", "true")
	t.Setenv("GATEWAY_WS_MAX_SESSION_TTL", "2h")
	t.Setenv("GATEWAY_MTLS_CERT_LIFETIME", "48h")
	cfg = FromEnv()
	if !cfg.GatewayRequireRequestNonce {
		t.Fatalf("expected gateway request nonce requirement to be enabled")
	}
	if cfg.GatewayWebSocketMaxSessionTTL != 2*time.Hour {
		t.Fatalf("expected gateway WS max session TTL override, got %s", cfg.GatewayWebSocketMaxSessionTTL)
	}
	if cfg.GatewayMTLSCertLifetime != 48*time.Hour {
		t.Fatalf("expected gateway mTLS cert lifetime override, got %s", cfg.GatewayMTLSCertLifetime)
	}

	t.Setenv("GATEWAY_WS_MAX_SESSION_TTL", "30s")
	t.Setenv("GATEWAY_MTLS_CERT_LIFETIME", "30m")
	cfg = FromEnv()
	if cfg.GatewayWebSocketMaxSessionTTL != 6*time.Hour {
		t.Fatalf("expected too-short gateway WS max session TTL to fallback, got %s", cfg.GatewayWebSocketMaxSessionTTL)
	}
	if cfg.GatewayMTLSCertLifetime != 24*time.Hour {
		t.Fatalf("expected too-short gateway mTLS cert lifetime to fallback, got %s", cfg.GatewayMTLSCertLifetime)
	}

	t.Setenv("GATEWAY_MTLS_CERT_LIFETIME", "96h")
	cfg = FromEnv()
	if cfg.GatewayMTLSCertLifetime != 24*time.Hour {
		t.Fatalf("expected too-long gateway mTLS cert lifetime to fallback, got %s", cfg.GatewayMTLSCertLifetime)
	}
}

func TestConfigValidateGatewayMTLSRequiresCertificates(t *testing.T) {
	cfg := Config{GatewayMTLSAddr: ":9443"}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validate to fail without gateway mTLS server cert")
	}

	cfg.GatewayMTLSServerCertPEM = "server-cert"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validate to fail without gateway mTLS server key")
	}

	cfg.GatewayMTLSServerKeyPEM = "server-key"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validate to fail without gateway CA cert/key")
	}

	cfg.GatewayCACertPEM = "ca-cert"
	cfg.GatewayCAKeyPEM = "ca-key"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validate to pass with gateway mTLS certs: %v", err)
	}
}

func TestConfigValidateRejectsDemoUsersInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("HRIS_VAULT_MASTER_KEY", "vault-master-key-001")
	t.Setenv("GATEWAY_BOOTSTRAP_TOKEN", "bootstrap-token-001")
	t.Setenv("DISABLE_LOGIN_RATE_LIMIT", "false")
	t.Setenv("ENABLE_DEMO_USERS", "true")

	cfg := FromEnv()
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validate to fail when ENABLE_DEMO_USERS is true in production")
	}
}

func TestConfigValidateRejectsDisabledRateLimitInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("HRIS_VAULT_MASTER_KEY", "vault-master-key-001")
	t.Setenv("GATEWAY_BOOTSTRAP_TOKEN", "bootstrap-token-001")
	t.Setenv("ENABLE_DEMO_USERS", "false")
	t.Setenv("DISABLE_LOGIN_RATE_LIMIT", "true")

	cfg := FromEnv()
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validate to fail when DISABLE_LOGIN_RATE_LIMIT is true in production")
	}
}

func TestFromEnvWalletDefaults(t *testing.T) {
	t.Setenv("WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY", "")
	t.Setenv("WALLET_DLQ_CLEANUP_DEFAULT_LIMIT", "")
	t.Setenv("WALLET_DLQ_CLEANUP_DEFAULT_OLDER_THAN", "")
	t.Setenv("WALLET_DLQ_ALERT_THRESHOLD", "")
	t.Setenv("WALLET_JOB_METRICS_DEFAULT_WINDOW", "")
	t.Setenv("WALLET_ALERT_DISPATCH_MOCK_TRANSIENT_FAIL_COUNT", "")
	t.Setenv("WALLET_ALERT_EMAIL_PROVIDER", "")
	t.Setenv("WALLET_ALERT_EMAIL_FROM", "")
	t.Setenv("WALLET_ALERT_EMAIL_RECEIVER_MAP", "")
	t.Setenv("WALLET_ALERT_RESEND_ENDPOINT", "")
	t.Setenv("WALLET_ALERT_RESEND_API_KEY", "")
	t.Setenv("WALLET_ALERT_RESEND_TIMEOUT", "")
	t.Setenv("WALLET_ALERT_WHATSAPP_PROVIDER", "")
	t.Setenv("WALLET_ALERT_WHATSAPP_RECEIVER_MAP", "")
	t.Setenv("WALLET_ALERT_WHATSAPP_ENDPOINT", "")
	t.Setenv("WALLET_ALERT_WHATSAPP_API_KEY", "")
	t.Setenv("WALLET_ALERT_WHATSAPP_PHONE_NUMBER_ID", "")
	t.Setenv("WALLET_ALERT_WHATSAPP_TIMEOUT", "")

	cfg := FromEnv()
	if cfg.WalletJobProcessDefaultMaxRetry != 3 {
		t.Fatalf("default max_retry mismatch: got %d", cfg.WalletJobProcessDefaultMaxRetry)
	}
	if cfg.WalletDLQCleanupDefaultLimit != 50 {
		t.Fatalf("default dlq cleanup limit mismatch: got %d", cfg.WalletDLQCleanupDefaultLimit)
	}
	if cfg.WalletDLQCleanupDefaultOlderThan != 24*time.Hour {
		t.Fatalf("default dlq cleanup older_than mismatch: got %s", cfg.WalletDLQCleanupDefaultOlderThan)
	}
	if cfg.WalletDLQAlertThreshold != 20 {
		t.Fatalf("default dlq alert threshold mismatch: got %d", cfg.WalletDLQAlertThreshold)
	}
	if cfg.WalletJobMetricsDefaultWindow != 15*time.Minute {
		t.Fatalf("default metrics window mismatch: got %s", cfg.WalletJobMetricsDefaultWindow)
	}
	if cfg.WalletAlertDispatchMockTransientFailCount != 0 {
		t.Fatalf("default alert dispatch transient fail count mismatch: got %d", cfg.WalletAlertDispatchMockTransientFailCount)
	}
	if cfg.WalletAlertEmailProvider != "mock" {
		t.Fatalf("default alert email provider mismatch: got %s", cfg.WalletAlertEmailProvider)
	}
	if cfg.WalletAlertEmailFrom != "no-reply@mistypass.local" {
		t.Fatalf("default alert email from mismatch: got %s", cfg.WalletAlertEmailFrom)
	}
	if cfg.WalletAlertEmailReceiverMap != nil {
		t.Fatalf("default alert email receiver map mismatch: expected nil, got %+v", cfg.WalletAlertEmailReceiverMap)
	}
	if cfg.WalletAlertResendEndpoint != "" {
		t.Fatalf("default resend endpoint mismatch: got %s", cfg.WalletAlertResendEndpoint)
	}
	if cfg.WalletAlertResendAPIKey != "" {
		t.Fatalf("default resend api key mismatch: got %s", cfg.WalletAlertResendAPIKey)
	}
	if cfg.WalletAlertResendTimeout != 5*time.Second {
		t.Fatalf("default resend timeout mismatch: got %s", cfg.WalletAlertResendTimeout)
	}
	if cfg.WalletAlertWhatsAppProvider != "mock" {
		t.Fatalf("default whatsapp provider mismatch: got %s", cfg.WalletAlertWhatsAppProvider)
	}
	if cfg.WalletAlertWhatsAppReceiverMap != nil {
		t.Fatalf("default whatsapp receiver map mismatch: expected nil, got %+v", cfg.WalletAlertWhatsAppReceiverMap)
	}
	if cfg.WalletAlertWhatsAppEndpoint != "" {
		t.Fatalf("default whatsapp endpoint mismatch: got %s", cfg.WalletAlertWhatsAppEndpoint)
	}
	if cfg.WalletAlertWhatsAppAPIKey != "" {
		t.Fatalf("default whatsapp api key mismatch: got %s", cfg.WalletAlertWhatsAppAPIKey)
	}
	if cfg.WalletAlertWhatsAppPhoneNumberID != "" {
		t.Fatalf("default whatsapp phone number id mismatch: got %s", cfg.WalletAlertWhatsAppPhoneNumberID)
	}
	if cfg.WalletAlertWhatsAppTimeout != 5*time.Second {
		t.Fatalf("default whatsapp timeout mismatch: got %s", cfg.WalletAlertWhatsAppTimeout)
	}
}

func TestFromEnvWalletOverrides(t *testing.T) {
	t.Setenv("WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY", "7")
	t.Setenv("WALLET_DLQ_CLEANUP_DEFAULT_LIMIT", "88")
	t.Setenv("WALLET_DLQ_CLEANUP_DEFAULT_OLDER_THAN", "2h")
	t.Setenv("WALLET_DLQ_ALERT_THRESHOLD", "12")
	t.Setenv("WALLET_JOB_METRICS_DEFAULT_WINDOW", "900")
	t.Setenv("WALLET_ALERT_DISPATCH_MOCK_TRANSIENT_FAIL_COUNT", "2")
	t.Setenv("WALLET_ALERT_EMAIL_PROVIDER", "resend")
	t.Setenv("WALLET_ALERT_EMAIL_FROM", "alerts@mistypass.local")
	t.Setenv("WALLET_ALERT_EMAIL_RECEIVER_MAP", "security=security@example.com,sec2@example.com;ops=ops@example.com")
	t.Setenv("WALLET_ALERT_RESEND_ENDPOINT", "https://api.resend.com/emails")
	t.Setenv("WALLET_ALERT_RESEND_API_KEY", "re_test_token")
	t.Setenv("WALLET_ALERT_RESEND_TIMEOUT", "8s")
	t.Setenv("WALLET_ALERT_WHATSAPP_PROVIDER", "meta")
	t.Setenv("WALLET_ALERT_WHATSAPP_RECEIVER_MAP", "security=+62811111111,+62811111112;ops=+62822222222")
	t.Setenv("WALLET_ALERT_WHATSAPP_ENDPOINT", "https://graph.facebook.com/v22.0")
	t.Setenv("WALLET_ALERT_WHATSAPP_API_KEY", "wa_test_token")
	t.Setenv("WALLET_ALERT_WHATSAPP_PHONE_NUMBER_ID", "1234567890")
	t.Setenv("WALLET_ALERT_WHATSAPP_TIMEOUT", "7s")

	cfg := FromEnv()
	if cfg.WalletJobProcessDefaultMaxRetry != 7 {
		t.Fatalf("override max_retry mismatch: got %d", cfg.WalletJobProcessDefaultMaxRetry)
	}
	if cfg.WalletDLQCleanupDefaultLimit != 88 {
		t.Fatalf("override dlq cleanup limit mismatch: got %d", cfg.WalletDLQCleanupDefaultLimit)
	}
	if cfg.WalletDLQCleanupDefaultOlderThan != 2*time.Hour {
		t.Fatalf("override dlq cleanup older_than mismatch: got %s", cfg.WalletDLQCleanupDefaultOlderThan)
	}
	if cfg.WalletDLQAlertThreshold != 12 {
		t.Fatalf("override dlq alert threshold mismatch: got %d", cfg.WalletDLQAlertThreshold)
	}
	if cfg.WalletJobMetricsDefaultWindow != 15*time.Minute {
		t.Fatalf("override metrics window mismatch: got %s", cfg.WalletJobMetricsDefaultWindow)
	}
	if cfg.WalletAlertDispatchMockTransientFailCount != 2 {
		t.Fatalf("override alert dispatch transient fail count mismatch: got %d", cfg.WalletAlertDispatchMockTransientFailCount)
	}
	if cfg.WalletAlertEmailProvider != "resend" {
		t.Fatalf("override alert email provider mismatch: got %s", cfg.WalletAlertEmailProvider)
	}
	if cfg.WalletAlertEmailFrom != "alerts@mistypass.local" {
		t.Fatalf("override alert email from mismatch: got %s", cfg.WalletAlertEmailFrom)
	}
	if cfg.WalletAlertResendEndpoint != "https://api.resend.com/emails" {
		t.Fatalf("override resend endpoint mismatch: got %s", cfg.WalletAlertResendEndpoint)
	}
	if cfg.WalletAlertResendAPIKey != "re_test_token" {
		t.Fatalf("override resend api key mismatch: got %s", cfg.WalletAlertResendAPIKey)
	}
	if cfg.WalletAlertResendTimeout != 8*time.Second {
		t.Fatalf("override resend timeout mismatch: got %s", cfg.WalletAlertResendTimeout)
	}
	if cfg.WalletAlertWhatsAppProvider != "meta" {
		t.Fatalf("override whatsapp provider mismatch: got %s", cfg.WalletAlertWhatsAppProvider)
	}
	if cfg.WalletAlertWhatsAppEndpoint != "https://graph.facebook.com/v22.0" {
		t.Fatalf("override whatsapp endpoint mismatch: got %s", cfg.WalletAlertWhatsAppEndpoint)
	}
	if cfg.WalletAlertWhatsAppAPIKey != "wa_test_token" {
		t.Fatalf("override whatsapp api key mismatch: got %s", cfg.WalletAlertWhatsAppAPIKey)
	}
	if cfg.WalletAlertWhatsAppPhoneNumberID != "1234567890" {
		t.Fatalf("override whatsapp phone number id mismatch: got %s", cfg.WalletAlertWhatsAppPhoneNumberID)
	}
	if cfg.WalletAlertWhatsAppTimeout != 7*time.Second {
		t.Fatalf("override whatsapp timeout mismatch: got %s", cfg.WalletAlertWhatsAppTimeout)
	}
	if len(cfg.WalletAlertEmailReceiverMap) != 2 {
		t.Fatalf("override receiver map mismatch: %+v", cfg.WalletAlertEmailReceiverMap)
	}
	if len(cfg.WalletAlertEmailReceiverMap["security"]) != 2 || cfg.WalletAlertEmailReceiverMap["security"][0] != "security@example.com" {
		t.Fatalf("override receiver map security mismatch: %+v", cfg.WalletAlertEmailReceiverMap["security"])
	}
	if len(cfg.WalletAlertWhatsAppReceiverMap) != 2 {
		t.Fatalf("override whatsapp receiver map mismatch: %+v", cfg.WalletAlertWhatsAppReceiverMap)
	}
	if len(cfg.WalletAlertWhatsAppReceiverMap["security"]) != 2 || cfg.WalletAlertWhatsAppReceiverMap["security"][0] != "+62811111111" {
		t.Fatalf("override whatsapp receiver map security mismatch: %+v", cfg.WalletAlertWhatsAppReceiverMap["security"])
	}
}

func TestFromEnvWalletInvalidValuesFallback(t *testing.T) {
	t.Setenv("WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY", "-1")
	t.Setenv("WALLET_DLQ_CLEANUP_DEFAULT_LIMIT", "0")
	t.Setenv("WALLET_DLQ_CLEANUP_DEFAULT_OLDER_THAN", "500ms")
	t.Setenv("WALLET_DLQ_ALERT_THRESHOLD", "-5")
	t.Setenv("WALLET_JOB_METRICS_DEFAULT_WINDOW", "500ms")
	t.Setenv("WALLET_ALERT_DISPATCH_MOCK_TRANSIENT_FAIL_COUNT", "-3")
	t.Setenv("WALLET_ALERT_EMAIL_PROVIDER", "invalid-provider")
	t.Setenv("WALLET_ALERT_EMAIL_FROM", "")
	t.Setenv("WALLET_ALERT_EMAIL_RECEIVER_MAP", "security=;invalid")
	t.Setenv("WALLET_ALERT_RESEND_TIMEOUT", "500ms")
	t.Setenv("WALLET_ALERT_WHATSAPP_PROVIDER", "invalid-provider")
	t.Setenv("WALLET_ALERT_WHATSAPP_RECEIVER_MAP", "security=;invalid")
	t.Setenv("WALLET_ALERT_WHATSAPP_TIMEOUT", "500ms")

	cfg := FromEnv()
	if cfg.WalletJobProcessDefaultMaxRetry != 1 {
		t.Fatalf("fallback max_retry mismatch: got %d", cfg.WalletJobProcessDefaultMaxRetry)
	}
	if cfg.WalletDLQCleanupDefaultLimit != 1 {
		t.Fatalf("fallback dlq cleanup limit mismatch: got %d", cfg.WalletDLQCleanupDefaultLimit)
	}
	if cfg.WalletDLQCleanupDefaultOlderThan != 24*time.Hour {
		t.Fatalf("fallback dlq cleanup older_than mismatch: got %s", cfg.WalletDLQCleanupDefaultOlderThan)
	}
	if cfg.WalletDLQAlertThreshold != 1 {
		t.Fatalf("fallback dlq alert threshold mismatch: got %d", cfg.WalletDLQAlertThreshold)
	}
	if cfg.WalletJobMetricsDefaultWindow != 15*time.Minute {
		t.Fatalf("fallback metrics window mismatch: got %s", cfg.WalletJobMetricsDefaultWindow)
	}
	if cfg.WalletAlertDispatchMockTransientFailCount != 0 {
		t.Fatalf("fallback alert dispatch transient fail count mismatch: got %d", cfg.WalletAlertDispatchMockTransientFailCount)
	}
	if cfg.WalletAlertEmailProvider != "mock" {
		t.Fatalf("fallback alert email provider mismatch: got %s", cfg.WalletAlertEmailProvider)
	}
	if cfg.WalletAlertEmailFrom != "no-reply@mistypass.local" {
		t.Fatalf("fallback alert email from mismatch: got %s", cfg.WalletAlertEmailFrom)
	}
	if cfg.WalletAlertEmailReceiverMap != nil {
		t.Fatalf("fallback alert email receiver map mismatch: expected nil, got %+v", cfg.WalletAlertEmailReceiverMap)
	}
	if cfg.WalletAlertResendTimeout != 5*time.Second {
		t.Fatalf("fallback resend timeout mismatch: got %s", cfg.WalletAlertResendTimeout)
	}
	if cfg.WalletAlertWhatsAppProvider != "mock" {
		t.Fatalf("fallback whatsapp provider mismatch: got %s", cfg.WalletAlertWhatsAppProvider)
	}
	if cfg.WalletAlertWhatsAppReceiverMap != nil {
		t.Fatalf("fallback whatsapp receiver map mismatch: expected nil, got %+v", cfg.WalletAlertWhatsAppReceiverMap)
	}
	if cfg.WalletAlertWhatsAppTimeout != 5*time.Second {
		t.Fatalf("fallback whatsapp timeout mismatch: got %s", cfg.WalletAlertWhatsAppTimeout)
	}
}

func TestFromEnvMQTTDefaults(t *testing.T) {
	t.Setenv("MQTT_ENABLED", "")
	t.Setenv("MQTT_BROKER_URL", "")
	t.Setenv("MQTT_TOPIC_PREFIX", "")

	cfg := FromEnv()
	if cfg.MQTTEnabled {
		t.Fatalf("expected mqtt disabled by default")
	}
	if cfg.MQTTBrokerURL != "tcp://localhost:1883" {
		t.Fatalf("unexpected mqtt broker default: %q", cfg.MQTTBrokerURL)
	}
	if cfg.MQTTTopicPrefix != "mistypass" {
		t.Fatalf("unexpected mqtt topic prefix default: %q", cfg.MQTTTopicPrefix)
	}
}

func TestConfigValidateMQTT(t *testing.T) {
	t.Setenv("MQTT_ENABLED", "true")
	t.Setenv("MQTT_BROKER_URL", "tcp://emqx:1883")
	t.Setenv("MQTT_TOPIC_PREFIX", "mistypass")

	cfg := FromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected mqtt validate success, got %v", err)
	}

	t.Setenv("MQTT_BROKER_URL", "://invalid")
	cfg = FromEnv()
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected mqtt validate failure for invalid URL")
	}
}

func TestFromEnvNATSDefaults(t *testing.T) {
	t.Setenv("NATS_ENABLED", "")
	t.Setenv("NATS_SERVER_URL", "")
	t.Setenv("NATS_SUBJECT_PREFIX", "")

	cfg := FromEnv()
	if cfg.NATSEnabled {
		t.Fatalf("expected nats disabled by default")
	}
	if cfg.NATSServerURL != "nats://localhost:4222" {
		t.Fatalf("unexpected nats server default: %q", cfg.NATSServerURL)
	}
	if cfg.NATSSubjectPrefix != "mistypass" {
		t.Fatalf("unexpected nats subject prefix default: %q", cfg.NATSSubjectPrefix)
	}
}

func TestConfigValidateNATS(t *testing.T) {
	t.Setenv("NATS_ENABLED", "true")
	t.Setenv("NATS_SERVER_URL", "nats://nats:4222")
	t.Setenv("NATS_SUBJECT_PREFIX", "mistypass")

	cfg := FromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected nats validate success, got %v", err)
	}

	t.Setenv("NATS_SERVER_URL", "://invalid")
	cfg = FromEnv()
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected nats validate failure for invalid URL")
	}
}

func TestFromEnvAuthAndExternalAuthDefaults(t *testing.T) {
	t.Setenv("AUTH_ADMIN_MFA_REQUIRED", "")
	t.Setenv("EXTERNAL_AUTH_ENABLED", "")
	t.Setenv("EXTERNAL_AUTH_PROVIDER", "")
	t.Setenv("EXTERNAL_AUTH_USERINFO_URL", "")
	t.Setenv("EXTERNAL_AUTH_TIMEOUT", "")
	t.Setenv("EXTERNAL_AUTH_DEFAULT_ROLE", "")

	cfg := FromEnv()
	if cfg.AuthAdminMFARequired {
		t.Fatalf("expected auth admin mfa disabled by default")
	}
	if cfg.ExternalAuthEnabled {
		t.Fatalf("expected external auth disabled by default")
	}
	if cfg.ExternalAuthProvider != "generic_oidc" {
		t.Fatalf("unexpected external auth provider default: %q", cfg.ExternalAuthProvider)
	}
	if cfg.ExternalAuthTimeout != 8*time.Second {
		t.Fatalf("unexpected external auth timeout default: %s", cfg.ExternalAuthTimeout)
	}
	if cfg.ExternalAuthDefaultRole != "resident" {
		t.Fatalf("unexpected external auth default role: %q", cfg.ExternalAuthDefaultRole)
	}
}

func TestConfigValidateExternalAuth(t *testing.T) {
	t.Setenv("EXTERNAL_AUTH_ENABLED", "true")
	t.Setenv("EXTERNAL_AUTH_PROVIDER", "casdoor")
	t.Setenv("EXTERNAL_AUTH_USERINFO_URL", "https://auth.example.com/api/userinfo")
	t.Setenv("EXTERNAL_AUTH_DEFAULT_ROLE", "tenant_admin")

	cfg := FromEnv()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected external auth validate success, got %v", err)
	}

	t.Setenv("EXTERNAL_AUTH_PROVIDER", "invalid")
	cfg = FromEnv()
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected external auth provider validate failure")
	}

	t.Setenv("EXTERNAL_AUTH_PROVIDER", "casdoor")
	t.Setenv("EXTERNAL_AUTH_USERINFO_URL", "://invalid")
	cfg = FromEnv()
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected external auth userinfo url validate failure")
	}
}

func TestFromEnvWalletAlertProviderBackwardCompatibility(t *testing.T) {
	t.Setenv("WALLET_ALERT_EMAIL_PROVIDER", "spaceemail")
	t.Setenv("WALLET_ALERT_SPACEEMAIL_ENDPOINT", "https://legacy.spaceemail.example/send")
	t.Setenv("WALLET_ALERT_SPACEEMAIL_API_KEY", "legacy-token")
	t.Setenv("WALLET_ALERT_SPACEEMAIL_TIMEOUT", "9s")

	cfg := FromEnv()
	if cfg.WalletAlertEmailProvider != "resend" {
		t.Fatalf("legacy provider should map to resend, got %s", cfg.WalletAlertEmailProvider)
	}
	if cfg.WalletAlertResendEndpoint != "https://legacy.spaceemail.example/send" {
		t.Fatalf("legacy endpoint should map to resend endpoint, got %s", cfg.WalletAlertResendEndpoint)
	}
	if cfg.WalletAlertResendAPIKey != "legacy-token" {
		t.Fatalf("legacy api key should map to resend api key, got %s", cfg.WalletAlertResendAPIKey)
	}
	if cfg.WalletAlertResendTimeout != 9*time.Second {
		t.Fatalf("legacy timeout should map to resend timeout, got %s", cfg.WalletAlertResendTimeout)
	}
}

func TestFromEnvWalletAlertCloudflareProvider(t *testing.T) {
	t.Setenv("WALLET_ALERT_EMAIL_PROVIDER", "cloudflare")

	cfg := FromEnv()
	if cfg.WalletAlertEmailProvider != "cloudflare" {
		t.Fatalf("cloudflare provider should be accepted, got %s", cfg.WalletAlertEmailProvider)
	}
}

func TestFromEnvEnterpriseJITProvisionApprovalRequired(t *testing.T) {
	t.Setenv("ENTERPRISE_JIT_PROVISION_APPROVAL_REQUIRED", "")
	cfg := FromEnv()
	if cfg.EnterpriseJITProvisionApprovalRequired {
		t.Fatalf("default enterprise jit approval flag should be false")
	}

	t.Setenv("ENTERPRISE_JIT_PROVISION_APPROVAL_REQUIRED", "true")
	cfg = FromEnv()
	if !cfg.EnterpriseJITProvisionApprovalRequired {
		t.Fatalf("expected enterprise jit approval flag override to be true")
	}
}

func TestFromEnvEnterpriseSyncWorkerAlertAutoRetryWorkerDefaultsAndOverrides(t *testing.T) {
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_ENABLED", "")
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_INTERVAL", "")
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_BATCH_SIZE", "")
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_MAX_ATTEMPTS", "")
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_BASE_BACKOFF", "")
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_MAX_BACKOFF", "")
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_LOCK_TTL", "")

	cfg := FromEnv()
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerEnabled {
		t.Fatalf("expected sync worker alert auto retry worker default disabled")
	}
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerInterval != 30*time.Second {
		t.Fatalf("unexpected auto retry worker interval: %s", cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerInterval)
	}
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBatchSize != 20 {
		t.Fatalf("unexpected auto retry worker batch size: %d", cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBatchSize)
	}
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxAttempts != 3 {
		t.Fatalf("unexpected auto retry worker max attempts: %d", cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxAttempts)
	}
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBaseBackoff != 5*time.Minute {
		t.Fatalf("unexpected auto retry worker base backoff: %s", cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBaseBackoff)
	}
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxBackoff != time.Hour {
		t.Fatalf("unexpected auto retry worker max backoff: %s", cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxBackoff)
	}
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerLockTTL != 10*time.Minute {
		t.Fatalf("unexpected auto retry worker lock ttl: %s", cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerLockTTL)
	}

	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_ENABLED", "true")
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_INTERVAL", "45s")
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_BATCH_SIZE", "7")
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_MAX_ATTEMPTS", "5")
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_BASE_BACKOFF", "90s")
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_MAX_BACKOFF", "15m")
	t.Setenv("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_LOCK_TTL", "12m")

	cfg = FromEnv()
	if !cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerEnabled {
		t.Fatalf("expected sync worker alert auto retry worker enabled override")
	}
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerInterval != 45*time.Second {
		t.Fatalf("unexpected auto retry worker interval override: %s", cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerInterval)
	}
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBatchSize != 7 {
		t.Fatalf("unexpected auto retry worker batch size override: %d", cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBatchSize)
	}
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxAttempts != 5 {
		t.Fatalf("unexpected auto retry worker max attempts override: %d", cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxAttempts)
	}
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBaseBackoff != 90*time.Second {
		t.Fatalf("unexpected auto retry worker base backoff override: %s", cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBaseBackoff)
	}
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxBackoff != 15*time.Minute {
		t.Fatalf("unexpected auto retry worker max backoff override: %s", cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxBackoff)
	}
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerLockTTL != 12*time.Minute {
		t.Fatalf("unexpected auto retry worker lock ttl override: %s", cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerLockTTL)
	}
}

func TestFromEnvEnterpriseJITApprovalExternalSyncWorkerDefaultsAndOverrides(t *testing.T) {
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_ENABLED", "")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_INTERVAL", "")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_BATCH_SIZE", "")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_MAX_ATTEMPTS", "")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_RETRY_COOLDOWN", "")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_ALERT_FAILURE_THRESHOLD", "")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_FORCE_ERROR", "")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_FORCE_ERROR_TENANT_ID", "")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_CALLBACK_TOKEN", "")

	cfg := FromEnv()
	if cfg.EnterpriseJITApprovalExternalSyncWorkerEnabled {
		t.Fatalf("expected external sync worker default disabled")
	}
	if cfg.EnterpriseJITApprovalExternalSyncWorkerInterval != 30*time.Second {
		t.Fatalf("unexpected default external sync worker interval: %s", cfg.EnterpriseJITApprovalExternalSyncWorkerInterval)
	}
	if cfg.EnterpriseJITApprovalExternalSyncWorkerBatchSize != 20 {
		t.Fatalf("unexpected default external sync worker batch size: %d", cfg.EnterpriseJITApprovalExternalSyncWorkerBatchSize)
	}
	if cfg.EnterpriseJITApprovalExternalSyncWorkerMaxAttempts != 5 {
		t.Fatalf("unexpected default external sync worker max attempts: %d", cfg.EnterpriseJITApprovalExternalSyncWorkerMaxAttempts)
	}
	if cfg.EnterpriseJITApprovalExternalSyncWorkerRetryCooldown != 30*time.Second {
		t.Fatalf("unexpected default external sync worker cooldown: %s", cfg.EnterpriseJITApprovalExternalSyncWorkerRetryCooldown)
	}
	if cfg.EnterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold != 3 {
		t.Fatalf("unexpected default external sync worker alert threshold: %d", cfg.EnterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold)
	}
	if cfg.EnterpriseJITApprovalExternalSyncCallbackToken != "" {
		t.Fatalf("expected default callback token empty, got %q", cfg.EnterpriseJITApprovalExternalSyncCallbackToken)
	}

	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_ENABLED", "true")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_INTERVAL", "45s")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_BATCH_SIZE", "7")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_MAX_ATTEMPTS", "8")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_RETRY_COOLDOWN", "90s")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_ALERT_FAILURE_THRESHOLD", "2")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_FORCE_ERROR", "true")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_FORCE_ERROR_TENANT_ID", "tenant_demo_jakarta")
	t.Setenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_CALLBACK_TOKEN", "cb-token")

	cfg = FromEnv()
	if !cfg.EnterpriseJITApprovalExternalSyncWorkerEnabled {
		t.Fatalf("expected external sync worker enabled override")
	}
	if cfg.EnterpriseJITApprovalExternalSyncWorkerInterval != 45*time.Second {
		t.Fatalf("unexpected external sync worker interval override: %s", cfg.EnterpriseJITApprovalExternalSyncWorkerInterval)
	}
	if cfg.EnterpriseJITApprovalExternalSyncWorkerBatchSize != 7 {
		t.Fatalf("unexpected external sync worker batch size override: %d", cfg.EnterpriseJITApprovalExternalSyncWorkerBatchSize)
	}
	if cfg.EnterpriseJITApprovalExternalSyncWorkerMaxAttempts != 8 {
		t.Fatalf("unexpected external sync worker max attempts override: %d", cfg.EnterpriseJITApprovalExternalSyncWorkerMaxAttempts)
	}
	if cfg.EnterpriseJITApprovalExternalSyncWorkerRetryCooldown != 90*time.Second {
		t.Fatalf("unexpected external sync worker cooldown override: %s", cfg.EnterpriseJITApprovalExternalSyncWorkerRetryCooldown)
	}
	if cfg.EnterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold != 2 {
		t.Fatalf("unexpected external sync worker alert threshold override: %d", cfg.EnterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold)
	}
	if !cfg.EnterpriseJITApprovalExternalSyncWorkerForceError {
		t.Fatalf("expected external sync worker force error override")
	}
	if cfg.EnterpriseJITApprovalExternalSyncWorkerForceErrorTenantID != "tenant_demo_jakarta" {
		t.Fatalf("unexpected external sync worker force error tenant id override: %s", cfg.EnterpriseJITApprovalExternalSyncWorkerForceErrorTenantID)
	}
	if cfg.EnterpriseJITApprovalExternalSyncCallbackToken != "cb-token" {
		t.Fatalf("unexpected callback token override: %s", cfg.EnterpriseJITApprovalExternalSyncCallbackToken)
	}
}

func TestFromEnvEnterpriseHRISWebhookDLQWorkerDefaultsAndOverrides(t *testing.T) {
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_ENABLED", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_INTERVAL", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_BATCH_SIZE", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_MAX_ATTEMPTS", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_RETRY_COOLDOWN", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_RETRY_MAX_BACKOFF", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_PROCESSING_TIMEOUT", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_ALERT_FAILURE_THRESHOLD", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_LOCK_TTL", "")

	cfg := FromEnv()
	if cfg.EnterpriseHRISWebhookDLQWorkerEnabled {
		t.Fatalf("expected hris webhook dlq worker default disabled")
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerInterval != 30*time.Second {
		t.Fatalf("unexpected hris webhook dlq worker interval: %s", cfg.EnterpriseHRISWebhookDLQWorkerInterval)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerBatchSize != 20 {
		t.Fatalf("unexpected hris webhook dlq worker batch size: %d", cfg.EnterpriseHRISWebhookDLQWorkerBatchSize)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerMaxAttempts != 5 {
		t.Fatalf("unexpected hris webhook dlq worker max attempts: %d", cfg.EnterpriseHRISWebhookDLQWorkerMaxAttempts)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown != 30*time.Second {
		t.Fatalf("unexpected hris webhook dlq worker cooldown: %s", cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff != 30*time.Second {
		t.Fatalf("unexpected hris webhook dlq worker max backoff: %s", cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout != 5*time.Minute {
		t.Fatalf("unexpected hris webhook dlq worker processing timeout: %s", cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerAlertFailureThreshold != 3 {
		t.Fatalf("unexpected hris webhook dlq worker alert threshold: %d", cfg.EnterpriseHRISWebhookDLQWorkerAlertFailureThreshold)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerLockTTL != 10*time.Minute {
		t.Fatalf("unexpected hris webhook dlq worker lock ttl: %s", cfg.EnterpriseHRISWebhookDLQWorkerLockTTL)
	}

	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_ENABLED", "true")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_INTERVAL", "45s")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_BATCH_SIZE", "7")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_MAX_ATTEMPTS", "8")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_RETRY_COOLDOWN", "90s")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_RETRY_MAX_BACKOFF", "4m")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_PROCESSING_TIMEOUT", "75s")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_ALERT_FAILURE_THRESHOLD", "2")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_LOCK_TTL", "12m")

	cfg = FromEnv()
	if !cfg.EnterpriseHRISWebhookDLQWorkerEnabled {
		t.Fatalf("expected hris webhook dlq worker enabled override")
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerInterval != 45*time.Second {
		t.Fatalf("unexpected hris webhook dlq worker interval override: %s", cfg.EnterpriseHRISWebhookDLQWorkerInterval)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerBatchSize != 7 {
		t.Fatalf("unexpected hris webhook dlq worker batch size override: %d", cfg.EnterpriseHRISWebhookDLQWorkerBatchSize)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerMaxAttempts != 8 {
		t.Fatalf("unexpected hris webhook dlq worker max attempts override: %d", cfg.EnterpriseHRISWebhookDLQWorkerMaxAttempts)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown != 90*time.Second {
		t.Fatalf("unexpected hris webhook dlq worker cooldown override: %s", cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff != 4*time.Minute {
		t.Fatalf("unexpected hris webhook dlq worker max backoff override: %s", cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout != 75*time.Second {
		t.Fatalf("unexpected hris webhook dlq worker processing timeout override: %s", cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerAlertFailureThreshold != 2 {
		t.Fatalf("unexpected hris webhook dlq worker alert threshold override: %d", cfg.EnterpriseHRISWebhookDLQWorkerAlertFailureThreshold)
	}
	if cfg.EnterpriseHRISWebhookDLQWorkerLockTTL != 12*time.Minute {
		t.Fatalf("unexpected hris webhook dlq worker lock ttl override: %s", cfg.EnterpriseHRISWebhookDLQWorkerLockTTL)
	}

	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_RETRY_MAX_BACKOFF", "30s")

	cfg = FromEnv()
	if cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff != 90*time.Second {
		t.Fatalf("expected hris webhook dlq worker max backoff to clamp to cooldown, got %s", cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff)
	}

	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_LOCK_TTL", "500ms")

	cfg = FromEnv()
	if cfg.EnterpriseHRISWebhookDLQWorkerLockTTL != 10*time.Minute {
		t.Fatalf("expected sub-second hris webhook dlq worker lock ttl to fall back to default, got %s", cfg.EnterpriseHRISWebhookDLQWorkerLockTTL)
	}

	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_PROCESSING_TIMEOUT", "500ms")

	cfg = FromEnv()
	if cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout != 5*time.Minute {
		t.Fatalf("expected sub-second hris webhook dlq worker processing timeout to fall back to default, got %s", cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout)
	}
}

func TestFromEnvEnterpriseHRISWebhookReceiptWorkerDefaultsAndOverrides(t *testing.T) {
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_ENABLED", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_INTERVAL", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_BATCH_SIZE", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_MAX_ATTEMPTS", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_RETRY_COOLDOWN", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_RETRY_MAX_BACKOFF", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_PROCESSING_TIMEOUT", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_ALERT_FAILURE_THRESHOLD", "")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_LOCK_TTL", "")

	cfg := FromEnv()
	if cfg.EnterpriseHRISWebhookReceiptWorkerEnabled {
		t.Fatalf("expected hris webhook receipt worker default disabled")
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerInterval != 30*time.Second {
		t.Fatalf("unexpected hris webhook receipt worker interval: %s", cfg.EnterpriseHRISWebhookReceiptWorkerInterval)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerBatchSize != 20 {
		t.Fatalf("unexpected hris webhook receipt worker batch size: %d", cfg.EnterpriseHRISWebhookReceiptWorkerBatchSize)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerMaxAttempts != 5 {
		t.Fatalf("unexpected hris webhook receipt worker max attempts: %d", cfg.EnterpriseHRISWebhookReceiptWorkerMaxAttempts)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown != 30*time.Second {
		t.Fatalf("unexpected hris webhook receipt worker cooldown: %s", cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff != 30*time.Second {
		t.Fatalf("unexpected hris webhook receipt worker max backoff: %s", cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout != 5*time.Minute {
		t.Fatalf("unexpected hris webhook receipt worker processing timeout: %s", cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerAlertFailureThreshold != 3 {
		t.Fatalf("unexpected hris webhook receipt worker alert threshold: %d", cfg.EnterpriseHRISWebhookReceiptWorkerAlertFailureThreshold)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerLockTTL != 10*time.Minute {
		t.Fatalf("unexpected hris webhook receipt worker lock ttl: %s", cfg.EnterpriseHRISWebhookReceiptWorkerLockTTL)
	}

	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_ENABLED", "true")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_INTERVAL", "15s")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_BATCH_SIZE", "5")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_MAX_ATTEMPTS", "4")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_RETRY_COOLDOWN", "45s")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_RETRY_MAX_BACKOFF", "3m")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_PROCESSING_TIMEOUT", "90s")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_ALERT_FAILURE_THRESHOLD", "2")
	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_LOCK_TTL", "11m")

	cfg = FromEnv()
	if !cfg.EnterpriseHRISWebhookReceiptWorkerEnabled {
		t.Fatalf("expected hris webhook receipt worker enabled override")
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerInterval != 15*time.Second {
		t.Fatalf("unexpected hris webhook receipt worker interval override: %s", cfg.EnterpriseHRISWebhookReceiptWorkerInterval)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerBatchSize != 5 {
		t.Fatalf("unexpected hris webhook receipt worker batch size override: %d", cfg.EnterpriseHRISWebhookReceiptWorkerBatchSize)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerMaxAttempts != 4 {
		t.Fatalf("unexpected hris webhook receipt worker max attempts override: %d", cfg.EnterpriseHRISWebhookReceiptWorkerMaxAttempts)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown != 45*time.Second {
		t.Fatalf("unexpected hris webhook receipt worker cooldown override: %s", cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff != 3*time.Minute {
		t.Fatalf("unexpected hris webhook receipt worker max backoff override: %s", cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout != 90*time.Second {
		t.Fatalf("unexpected hris webhook receipt worker processing timeout override: %s", cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerAlertFailureThreshold != 2 {
		t.Fatalf("unexpected hris webhook receipt worker alert threshold override: %d", cfg.EnterpriseHRISWebhookReceiptWorkerAlertFailureThreshold)
	}
	if cfg.EnterpriseHRISWebhookReceiptWorkerLockTTL != 11*time.Minute {
		t.Fatalf("unexpected hris webhook receipt worker lock ttl override: %s", cfg.EnterpriseHRISWebhookReceiptWorkerLockTTL)
	}

	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_RETRY_MAX_BACKOFF", "30s")

	cfg = FromEnv()
	if cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff != 45*time.Second {
		t.Fatalf("expected hris webhook receipt worker max backoff to clamp to cooldown, got %s", cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff)
	}

	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_LOCK_TTL", "500ms")

	cfg = FromEnv()
	if cfg.EnterpriseHRISWebhookReceiptWorkerLockTTL != 10*time.Minute {
		t.Fatalf("expected sub-second hris webhook receipt worker lock ttl to fall back to default, got %s", cfg.EnterpriseHRISWebhookReceiptWorkerLockTTL)
	}

	t.Setenv("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_PROCESSING_TIMEOUT", "500ms")

	cfg = FromEnv()
	if cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout != 5*time.Minute {
		t.Fatalf("expected sub-second hris webhook receipt worker processing timeout to fall back to default, got %s", cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout)
	}
}

func TestFromEnvEnterpriseHRISPullWorkerDefaultsAndOverrides(t *testing.T) {
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_ENABLED", "")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_INTERVAL", "")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_BATCH_SIZE", "")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_MAX_ATTEMPTS", "")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_RETRY_COOLDOWN", "")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_RETRY_MAX_BACKOFF", "")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_PROCESSING_TIMEOUT", "")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_RECONCILE_INTERVAL", "")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_ALERT_FAILURE_THRESHOLD", "")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_LOCK_TTL", "")

	cfg := FromEnv()
	if cfg.EnterpriseHRISPullWorkerEnabled {
		t.Fatalf("expected hris pull worker default disabled")
	}
	if cfg.EnterpriseHRISPullWorkerInterval != time.Hour {
		t.Fatalf("unexpected hris pull worker interval: %s", cfg.EnterpriseHRISPullWorkerInterval)
	}
	if cfg.EnterpriseHRISPullWorkerBatchSize != 10 {
		t.Fatalf("unexpected hris pull worker batch size: %d", cfg.EnterpriseHRISPullWorkerBatchSize)
	}
	if cfg.EnterpriseHRISPullWorkerMaxAttempts != 5 {
		t.Fatalf("unexpected hris pull worker max attempts: %d", cfg.EnterpriseHRISPullWorkerMaxAttempts)
	}
	if cfg.EnterpriseHRISPullWorkerRetryCooldown != 30*time.Minute {
		t.Fatalf("unexpected hris pull worker retry cooldown: %s", cfg.EnterpriseHRISPullWorkerRetryCooldown)
	}
	if cfg.EnterpriseHRISPullWorkerRetryMaxBackoff != 30*time.Minute {
		t.Fatalf("unexpected hris pull worker retry max backoff: %s", cfg.EnterpriseHRISPullWorkerRetryMaxBackoff)
	}
	if cfg.EnterpriseHRISPullWorkerProcessingTimeout != 30*time.Minute {
		t.Fatalf("unexpected hris pull worker processing timeout: %s", cfg.EnterpriseHRISPullWorkerProcessingTimeout)
	}
	if cfg.EnterpriseHRISPullWorkerReconcileInterval != 24*time.Hour {
		t.Fatalf("unexpected hris pull worker reconcile interval: %s", cfg.EnterpriseHRISPullWorkerReconcileInterval)
	}
	if cfg.EnterpriseHRISPullWorkerAlertFailureThreshold != 3 {
		t.Fatalf("unexpected hris pull worker alert threshold: %d", cfg.EnterpriseHRISPullWorkerAlertFailureThreshold)
	}
	if cfg.EnterpriseHRISPullWorkerLockTTL != 10*time.Minute {
		t.Fatalf("unexpected hris pull worker lock ttl: %s", cfg.EnterpriseHRISPullWorkerLockTTL)
	}

	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_ENABLED", "true")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_INTERVAL", "2h")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_BATCH_SIZE", "4")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_MAX_ATTEMPTS", "8")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_RETRY_COOLDOWN", "45m")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_RETRY_MAX_BACKOFF", "3h")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_PROCESSING_TIMEOUT", "75m")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_RECONCILE_INTERVAL", "12h")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_ALERT_FAILURE_THRESHOLD", "2")
	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_LOCK_TTL", "14m")

	cfg = FromEnv()
	if !cfg.EnterpriseHRISPullWorkerEnabled {
		t.Fatalf("expected hris pull worker enabled override")
	}
	if cfg.EnterpriseHRISPullWorkerInterval != 2*time.Hour {
		t.Fatalf("unexpected hris pull worker interval override: %s", cfg.EnterpriseHRISPullWorkerInterval)
	}
	if cfg.EnterpriseHRISPullWorkerBatchSize != 4 {
		t.Fatalf("unexpected hris pull worker batch size override: %d", cfg.EnterpriseHRISPullWorkerBatchSize)
	}
	if cfg.EnterpriseHRISPullWorkerMaxAttempts != 8 {
		t.Fatalf("unexpected hris pull worker max attempts override: %d", cfg.EnterpriseHRISPullWorkerMaxAttempts)
	}
	if cfg.EnterpriseHRISPullWorkerRetryCooldown != 45*time.Minute {
		t.Fatalf("unexpected hris pull worker retry cooldown override: %s", cfg.EnterpriseHRISPullWorkerRetryCooldown)
	}
	if cfg.EnterpriseHRISPullWorkerRetryMaxBackoff != 3*time.Hour {
		t.Fatalf("unexpected hris pull worker retry max backoff override: %s", cfg.EnterpriseHRISPullWorkerRetryMaxBackoff)
	}
	if cfg.EnterpriseHRISPullWorkerProcessingTimeout != 75*time.Minute {
		t.Fatalf("unexpected hris pull worker processing timeout override: %s", cfg.EnterpriseHRISPullWorkerProcessingTimeout)
	}
	if cfg.EnterpriseHRISPullWorkerReconcileInterval != 12*time.Hour {
		t.Fatalf("unexpected hris pull worker reconcile interval override: %s", cfg.EnterpriseHRISPullWorkerReconcileInterval)
	}
	if cfg.EnterpriseHRISPullWorkerAlertFailureThreshold != 2 {
		t.Fatalf("unexpected hris pull worker alert threshold override: %d", cfg.EnterpriseHRISPullWorkerAlertFailureThreshold)
	}
	if cfg.EnterpriseHRISPullWorkerLockTTL != 14*time.Minute {
		t.Fatalf("unexpected hris pull worker lock ttl override: %s", cfg.EnterpriseHRISPullWorkerLockTTL)
	}

	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_RETRY_MAX_BACKOFF", "30m")

	cfg = FromEnv()
	if cfg.EnterpriseHRISPullWorkerRetryMaxBackoff != 45*time.Minute {
		t.Fatalf("expected hris pull worker retry max backoff to clamp to cooldown, got %s", cfg.EnterpriseHRISPullWorkerRetryMaxBackoff)
	}

	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_LOCK_TTL", "500ms")

	cfg = FromEnv()
	if cfg.EnterpriseHRISPullWorkerLockTTL != 10*time.Minute {
		t.Fatalf("expected sub-second hris pull worker lock ttl to fall back to default, got %s", cfg.EnterpriseHRISPullWorkerLockTTL)
	}

	t.Setenv("ENTERPRISE_HRIS_PULL_WORKER_PROCESSING_TIMEOUT", "500ms")

	cfg = FromEnv()
	if cfg.EnterpriseHRISPullWorkerProcessingTimeout != 30*time.Minute {
		t.Fatalf("expected sub-second hris pull worker processing timeout to fall back to default, got %s", cfg.EnterpriseHRISPullWorkerProcessingTimeout)
	}
}

func TestFromEnvOTelDefaultsAndOverrides(t *testing.T) {
	t.Setenv("OTEL_ENABLED", "")
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "")
	t.Setenv("OTEL_TRACE_SAMPLE_RATIO", "")
	t.Setenv("OTEL_EXPORT_TIMEOUT", "")

	cfg := FromEnv()
	if cfg.OTelEnabled {
		t.Fatalf("default OTel enabled mismatch: got true")
	}
	if cfg.OTelServiceName != "mistypass-api" {
		t.Fatalf("default OTel service name mismatch: got %s", cfg.OTelServiceName)
	}
	if cfg.OTelExporterOTLPEndpoint != "" {
		t.Fatalf("default OTel endpoint mismatch: got %s", cfg.OTelExporterOTLPEndpoint)
	}
	if cfg.OTelExporterOTLPInsecure {
		t.Fatalf("default OTel insecure mismatch: got true")
	}
	if cfg.OTelTraceSampleRatio != 1.0 {
		t.Fatalf("default OTel sample ratio mismatch: got %f", cfg.OTelTraceSampleRatio)
	}
	if cfg.OTelExportTimeout != 5*time.Second {
		t.Fatalf("default OTel export timeout mismatch: got %s", cfg.OTelExportTimeout)
	}

	t.Setenv("OTEL_ENABLED", "true")
	t.Setenv("OTEL_SERVICE_NAME", "mistypass-api-test")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Setenv("OTEL_TRACE_SAMPLE_RATIO", "0.25")
	t.Setenv("OTEL_EXPORT_TIMEOUT", "7s")
	cfg = FromEnv()
	if !cfg.OTelEnabled {
		t.Fatalf("override OTel enabled mismatch: got false")
	}
	if cfg.OTelServiceName != "mistypass-api-test" {
		t.Fatalf("override OTel service name mismatch: got %s", cfg.OTelServiceName)
	}
	if cfg.OTelExporterOTLPEndpoint != "localhost:4317" {
		t.Fatalf("override OTel endpoint mismatch: got %s", cfg.OTelExporterOTLPEndpoint)
	}
	if !cfg.OTelExporterOTLPInsecure {
		t.Fatalf("override OTel insecure mismatch: got false")
	}
	if cfg.OTelTraceSampleRatio != 0.25 {
		t.Fatalf("override OTel sample ratio mismatch: got %f", cfg.OTelTraceSampleRatio)
	}
	if cfg.OTelExportTimeout != 7*time.Second {
		t.Fatalf("override OTel export timeout mismatch: got %s", cfg.OTelExportTimeout)
	}
}

func TestConfigValidateOTel(t *testing.T) {
	cfg := Config{
		AppEnv:                   "development",
		OTelEnabled:              true,
		OTelExporterOTLPEndpoint: "",
		OTelTraceSampleRatio:     1.0,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validate to fail when OTEL enabled without endpoint")
	}

	cfg.OTelExporterOTLPEndpoint = "localhost:4317"
	cfg.OTelTraceSampleRatio = 2
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validate to fail when sample ratio out of range")
	}

	cfg.OTelTraceSampleRatio = 0.5
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected validate to pass with valid OTel config: %v", err)
	}
}

func TestFromEnvRedisDefaultsAndOverrides(t *testing.T) {
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("REDIS_KEY_PREFIX", "")
	t.Setenv("REDIS_DIAL_TIMEOUT", "")
	t.Setenv("REDIS_READ_TIMEOUT", "")
	t.Setenv("REDIS_WRITE_TIMEOUT", "")

	cfg := FromEnv()
	if cfg.RedisAddr != "" {
		t.Fatalf("default redis addr mismatch: got %q", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "" {
		t.Fatalf("default redis password mismatch: got %q", cfg.RedisPassword)
	}
	if cfg.RedisDB != 0 {
		t.Fatalf("default redis db mismatch: got %d", cfg.RedisDB)
	}
	if cfg.RedisKeyPrefix != "mistypass" {
		t.Fatalf("default redis key prefix mismatch: got %q", cfg.RedisKeyPrefix)
	}
	if cfg.RedisDialTimeout != 3*time.Second {
		t.Fatalf("default redis dial timeout mismatch: got %s", cfg.RedisDialTimeout)
	}
	if cfg.RedisReadTimeout != 3*time.Second {
		t.Fatalf("default redis read timeout mismatch: got %s", cfg.RedisReadTimeout)
	}
	if cfg.RedisWriteTimeout != 3*time.Second {
		t.Fatalf("default redis write timeout mismatch: got %s", cfg.RedisWriteTimeout)
	}

	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("REDIS_PASSWORD", "redis-pass")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("REDIS_KEY_PREFIX", "mistypass-dev")
	t.Setenv("REDIS_DIAL_TIMEOUT", "4s")
	t.Setenv("REDIS_READ_TIMEOUT", "5s")
	t.Setenv("REDIS_WRITE_TIMEOUT", "6s")

	cfg = FromEnv()
	if cfg.RedisAddr != "127.0.0.1:6379" {
		t.Fatalf("override redis addr mismatch: got %q", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "redis-pass" {
		t.Fatalf("override redis password mismatch: got %q", cfg.RedisPassword)
	}
	if cfg.RedisDB != 2 {
		t.Fatalf("override redis db mismatch: got %d", cfg.RedisDB)
	}
	if cfg.RedisKeyPrefix != "mistypass-dev" {
		t.Fatalf("override redis key prefix mismatch: got %q", cfg.RedisKeyPrefix)
	}
	if cfg.RedisDialTimeout != 4*time.Second {
		t.Fatalf("override redis dial timeout mismatch: got %s", cfg.RedisDialTimeout)
	}
	if cfg.RedisReadTimeout != 5*time.Second {
		t.Fatalf("override redis read timeout mismatch: got %s", cfg.RedisReadTimeout)
	}
	if cfg.RedisWriteTimeout != 6*time.Second {
		t.Fatalf("override redis write timeout mismatch: got %s", cfg.RedisWriteTimeout)
	}
}

func TestFromEnvHRISVaultMasterKey(t *testing.T) {
	t.Setenv("HRIS_VAULT_MASTER_KEY", "")

	cfg := FromEnv()
	if cfg.HRISVaultMasterKey != "" {
		t.Fatalf("default hris vault master key mismatch: got %q", cfg.HRISVaultMasterKey)
	}

	t.Setenv("HRIS_VAULT_MASTER_KEY", "vault-master-key-001")
	cfg = FromEnv()
	if cfg.HRISVaultMasterKey != "vault-master-key-001" {
		t.Fatalf("override hris vault master key mismatch: got %q", cfg.HRISVaultMasterKey)
	}
}

func TestFromEnvRedisInvalidValuesFallback(t *testing.T) {
	t.Setenv("REDIS_DB", "-1")
	t.Setenv("REDIS_KEY_PREFIX", "")
	t.Setenv("REDIS_DIAL_TIMEOUT", "500ms")
	t.Setenv("REDIS_READ_TIMEOUT", "500ms")
	t.Setenv("REDIS_WRITE_TIMEOUT", "500ms")

	cfg := FromEnv()
	if cfg.RedisDB != 0 {
		t.Fatalf("invalid redis db should fallback to 0: got %d", cfg.RedisDB)
	}
	if cfg.RedisKeyPrefix != "mistypass" {
		t.Fatalf("invalid redis key prefix should fallback: got %q", cfg.RedisKeyPrefix)
	}
	if cfg.RedisDialTimeout != 3*time.Second {
		t.Fatalf("invalid redis dial timeout should fallback: got %s", cfg.RedisDialTimeout)
	}
	if cfg.RedisReadTimeout != 3*time.Second {
		t.Fatalf("invalid redis read timeout should fallback: got %s", cfg.RedisReadTimeout)
	}
	if cfg.RedisWriteTimeout != 3*time.Second {
		t.Fatalf("invalid redis write timeout should fallback: got %s", cfg.RedisWriteTimeout)
	}
}
