package config

import (
	"testing"
	"time"
)

func TestFromEnvDemoUsersDefaultByAppEnv(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("ENABLE_DEMO_USERS", "")
	cfg := FromEnv()
	if !cfg.EnableDemoUsers {
		t.Fatalf("expected demo users enabled by default in development env")
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

func TestConfigValidateRequiresJWTSecretInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "")
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
