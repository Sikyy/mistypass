package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                                                       string
	HTTPAddr                                                     string
	CORSOrigin                                                   string
	JWTSecret                                                    string
	JWTIssuer                                                    string
	JWTAccessTTL                                                 time.Duration
	JWTRefreshTTL                                                time.Duration
	EnableDemoUsers                                              bool
	DatabaseURL                                                  string
	DatabaseAutoMigrate                                          bool
	EnterpriseJITProvisionApprovalRequired                       bool
	EnterpriseSyncReconcileWorkerEnabled                         bool
	EnterpriseSyncReconcileWorkerInterval                        time.Duration
	EnterpriseSyncReconcileWorkerBatchSize                       int
	EnterpriseSyncReconcileWorkerMaxAttempts                     int
	EnterpriseSyncReconcileWorkerRetryCooldown                   time.Duration
	EnterpriseSyncReconcileWorkerAlertFailureThreshold           int
	EnterpriseSyncReconcileWorkerForceError                      bool
	EnterpriseSyncReconcileWorkerForceErrorTenantID              string
	EnterpriseJITApprovalExternalSyncWorkerEnabled               bool
	EnterpriseJITApprovalExternalSyncWorkerInterval              time.Duration
	EnterpriseJITApprovalExternalSyncWorkerBatchSize             int
	EnterpriseJITApprovalExternalSyncWorkerMaxAttempts           int
	EnterpriseJITApprovalExternalSyncWorkerRetryCooldown         time.Duration
	EnterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold int
	EnterpriseJITApprovalExternalSyncWorkerForceError            bool
	EnterpriseJITApprovalExternalSyncWorkerForceErrorTenantID    string
	EnterpriseJITApprovalExternalSyncCallbackToken               string
	GatewayEventsBatchForceRetryableError                        bool
	GatewayEventsBatchForceRetryablePrefix                       string
	WalletJobProcessDefaultMaxRetry                              int
	WalletDLQCleanupDefaultLimit                                 int
	WalletDLQCleanupDefaultOlderThan                             time.Duration
	WalletDLQAlertThreshold                                      int
	WalletJobMetricsDefaultWindow                                time.Duration
	WalletAlertDispatchMockTransientFailCount                    int
	WalletAlertEmailProvider                                     string
	WalletAlertEmailFrom                                         string
	WalletAlertEmailReceiverMap                                  map[string][]string
	WalletAlertResendEndpoint                                    string
	WalletAlertResendAPIKey                                      string
	WalletAlertResendTimeout                                     time.Duration
	WalletAlertWhatsAppProvider                                  string
	WalletAlertWhatsAppReceiverMap                               map[string][]string
	WalletAlertWhatsAppEndpoint                                  string
	WalletAlertWhatsAppAPIKey                                    string
	WalletAlertWhatsAppPhoneNumberID                             string
	WalletAlertWhatsAppTimeout                                   time.Duration
}

func FromEnv() Config {
	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if appEnv == "" {
		appEnv = "development"
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	addr := port
	if !strings.Contains(port, ":") {
		addr = ":" + port
	}

	origin := strings.TrimSpace(os.Getenv("CORS_ORIGIN"))
	if origin == "" {
		origin = "http://localhost:5173"
	}

	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))

	jwtIssuer := strings.TrimSpace(os.Getenv("JWT_ISSUER"))
	if jwtIssuer == "" {
		jwtIssuer = "mistypass-api"
	}

	accessTTL := parseDurationOrFallback(
		strings.TrimSpace(os.Getenv("JWT_ACCESS_TTL")),
		time.Hour,
	)
	refreshTTL := parseDurationOrFallback(
		strings.TrimSpace(os.Getenv("JWT_REFRESH_TTL")),
		7*24*time.Hour,
	)
	enableDemoUsers := appEnv != "production" && appEnv != "prod"
	enableDemoUsersRaw := strings.TrimSpace(os.Getenv("ENABLE_DEMO_USERS"))
	if enableDemoUsersRaw != "" {
		enableDemoUsers = parseBoolOrFallback(enableDemoUsersRaw, enableDemoUsers)
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	dbAutoMigrate := parseBoolOrFallback(
		strings.TrimSpace(os.Getenv("DATABASE_AUTO_MIGRATE")),
		true,
	)
	enterpriseJITProvisionApprovalRequired := parseBoolOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_JIT_PROVISION_APPROVAL_REQUIRED")),
		false,
	)
	enterpriseSyncReconcileWorkerEnabled := parseBoolOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_SYNC_RECONCILE_WORKER_ENABLED")),
		false,
	)
	enterpriseSyncReconcileWorkerInterval := parseDurationOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_SYNC_RECONCILE_WORKER_INTERVAL")),
		30*time.Second,
	)
	enterpriseSyncReconcileWorkerBatchSize := parseIntOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_SYNC_RECONCILE_WORKER_BATCH_SIZE")),
		20,
	)
	if enterpriseSyncReconcileWorkerBatchSize < 1 {
		enterpriseSyncReconcileWorkerBatchSize = 1
	}
	enterpriseSyncReconcileWorkerMaxAttempts := parseIntOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_SYNC_RECONCILE_WORKER_MAX_ATTEMPTS")),
		5,
	)
	if enterpriseSyncReconcileWorkerMaxAttempts < 1 {
		enterpriseSyncReconcileWorkerMaxAttempts = 1
	}
	enterpriseSyncReconcileWorkerRetryCooldown := parseDurationOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_SYNC_RECONCILE_WORKER_RETRY_COOLDOWN")),
		30*time.Second,
	)
	enterpriseSyncReconcileWorkerAlertFailureThreshold := parseIntOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_SYNC_RECONCILE_WORKER_ALERT_FAILURE_THRESHOLD")),
		3,
	)
	if enterpriseSyncReconcileWorkerAlertFailureThreshold < 1 {
		enterpriseSyncReconcileWorkerAlertFailureThreshold = 1
	}
	enterpriseSyncReconcileWorkerForceError := parseBoolOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_SYNC_RECONCILE_WORKER_FORCE_ERROR")),
		false,
	)
	enterpriseSyncReconcileWorkerForceErrorTenantID := strings.TrimSpace(
		os.Getenv("ENTERPRISE_SYNC_RECONCILE_WORKER_FORCE_ERROR_TENANT_ID"),
	)
	enterpriseJITApprovalExternalSyncWorkerEnabled := parseBoolOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_ENABLED")),
		false,
	)
	enterpriseJITApprovalExternalSyncWorkerInterval := parseDurationOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_INTERVAL")),
		30*time.Second,
	)
	enterpriseJITApprovalExternalSyncWorkerBatchSize := parseIntOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_BATCH_SIZE")),
		20,
	)
	if enterpriseJITApprovalExternalSyncWorkerBatchSize < 1 {
		enterpriseJITApprovalExternalSyncWorkerBatchSize = 1
	}
	enterpriseJITApprovalExternalSyncWorkerMaxAttempts := parseIntOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_MAX_ATTEMPTS")),
		5,
	)
	if enterpriseJITApprovalExternalSyncWorkerMaxAttempts < 1 {
		enterpriseJITApprovalExternalSyncWorkerMaxAttempts = 1
	}
	enterpriseJITApprovalExternalSyncWorkerRetryCooldown := parseDurationOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_RETRY_COOLDOWN")),
		30*time.Second,
	)
	enterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold := parseIntOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_ALERT_FAILURE_THRESHOLD")),
		3,
	)
	if enterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold < 1 {
		enterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold = 1
	}
	enterpriseJITApprovalExternalSyncWorkerForceError := parseBoolOrFallback(
		strings.TrimSpace(os.Getenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_FORCE_ERROR")),
		false,
	)
	enterpriseJITApprovalExternalSyncWorkerForceErrorTenantID := strings.TrimSpace(
		os.Getenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_FORCE_ERROR_TENANT_ID"),
	)
	enterpriseJITApprovalExternalSyncCallbackToken := strings.TrimSpace(
		os.Getenv("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_CALLBACK_TOKEN"),
	)
	gatewayEventsBatchForceRetryableError := parseBoolOrFallback(
		strings.TrimSpace(os.Getenv("GATEWAY_EVENTS_BATCH_FORCE_RETRYABLE_ERROR")),
		false,
	)
	gatewayEventsBatchForceRetryablePrefix := strings.TrimSpace(
		os.Getenv("GATEWAY_EVENTS_BATCH_FORCE_RETRYABLE_PREFIX"),
	)
	if gatewayEventsBatchForceRetryablePrefix == "" {
		gatewayEventsBatchForceRetryablePrefix = "force-retry-"
	}
	walletJobProcessDefaultMaxRetry := parseIntOrFallback(
		strings.TrimSpace(os.Getenv("WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY")),
		3,
	)
	if walletJobProcessDefaultMaxRetry < 1 {
		walletJobProcessDefaultMaxRetry = 1
	}
	walletDLQCleanupDefaultLimit := parseIntOrFallback(
		strings.TrimSpace(os.Getenv("WALLET_DLQ_CLEANUP_DEFAULT_LIMIT")),
		50,
	)
	if walletDLQCleanupDefaultLimit < 1 {
		walletDLQCleanupDefaultLimit = 1
	}
	walletDLQCleanupDefaultOlderThan := parseDurationOrFallback(
		strings.TrimSpace(os.Getenv("WALLET_DLQ_CLEANUP_DEFAULT_OLDER_THAN")),
		24*time.Hour,
	)
	if walletDLQCleanupDefaultOlderThan < time.Second {
		walletDLQCleanupDefaultOlderThan = 24 * time.Hour
	}
	walletDLQAlertThreshold := parseIntOrFallback(
		strings.TrimSpace(os.Getenv("WALLET_DLQ_ALERT_THRESHOLD")),
		20,
	)
	if walletDLQAlertThreshold < 1 {
		walletDLQAlertThreshold = 1
	}
	walletJobMetricsDefaultWindow := parseDurationOrFallback(
		strings.TrimSpace(os.Getenv("WALLET_JOB_METRICS_DEFAULT_WINDOW")),
		15*time.Minute,
	)
	if walletJobMetricsDefaultWindow < time.Second {
		walletJobMetricsDefaultWindow = 15 * time.Minute
	}
	walletAlertDispatchMockTransientFailCount := parseIntOrFallback(
		strings.TrimSpace(os.Getenv("WALLET_ALERT_DISPATCH_MOCK_TRANSIENT_FAIL_COUNT")),
		0,
	)
	if walletAlertDispatchMockTransientFailCount < 0 {
		walletAlertDispatchMockTransientFailCount = 0
	}
	if walletAlertDispatchMockTransientFailCount > 100 {
		walletAlertDispatchMockTransientFailCount = 100
	}
	walletAlertEmailProvider := strings.ToLower(strings.TrimSpace(os.Getenv("WALLET_ALERT_EMAIL_PROVIDER")))
	if walletAlertEmailProvider == "" {
		walletAlertEmailProvider = "mock"
	}
	switch walletAlertEmailProvider {
	case "mock", "resend":
	case "spaceemail":
		walletAlertEmailProvider = "resend"
	default:
		walletAlertEmailProvider = "mock"
	}
	walletAlertEmailFrom := strings.TrimSpace(os.Getenv("WALLET_ALERT_EMAIL_FROM"))
	if walletAlertEmailFrom == "" {
		walletAlertEmailFrom = "no-reply@mistypass.local"
	}
	walletAlertEmailReceiverMap := parseGroupEmailMap(
		strings.TrimSpace(os.Getenv("WALLET_ALERT_EMAIL_RECEIVER_MAP")),
	)
	walletAlertResendEndpoint := strings.TrimSpace(os.Getenv("WALLET_ALERT_RESEND_ENDPOINT"))
	if walletAlertResendEndpoint == "" {
		walletAlertResendEndpoint = strings.TrimSpace(os.Getenv("WALLET_ALERT_SPACEEMAIL_ENDPOINT"))
	}
	walletAlertResendAPIKey := strings.TrimSpace(os.Getenv("WALLET_ALERT_RESEND_API_KEY"))
	if walletAlertResendAPIKey == "" {
		walletAlertResendAPIKey = strings.TrimSpace(os.Getenv("WALLET_ALERT_SPACEEMAIL_API_KEY"))
	}
	walletAlertResendTimeoutRaw := strings.TrimSpace(os.Getenv("WALLET_ALERT_RESEND_TIMEOUT"))
	if walletAlertResendTimeoutRaw == "" {
		walletAlertResendTimeoutRaw = strings.TrimSpace(os.Getenv("WALLET_ALERT_SPACEEMAIL_TIMEOUT"))
	}
	walletAlertResendTimeout := parseDurationOrFallback(
		walletAlertResendTimeoutRaw,
		5*time.Second,
	)
	if walletAlertResendTimeout < time.Second {
		walletAlertResendTimeout = 5 * time.Second
	}
	walletAlertWhatsAppProvider := strings.ToLower(strings.TrimSpace(os.Getenv("WALLET_ALERT_WHATSAPP_PROVIDER")))
	if walletAlertWhatsAppProvider == "" {
		walletAlertWhatsAppProvider = "mock"
	}
	switch walletAlertWhatsAppProvider {
	case "mock", "meta":
	default:
		walletAlertWhatsAppProvider = "mock"
	}
	walletAlertWhatsAppReceiverMap := parseGroupEmailMap(
		strings.TrimSpace(os.Getenv("WALLET_ALERT_WHATSAPP_RECEIVER_MAP")),
	)
	walletAlertWhatsAppEndpoint := strings.TrimSpace(os.Getenv("WALLET_ALERT_WHATSAPP_ENDPOINT"))
	walletAlertWhatsAppAPIKey := strings.TrimSpace(os.Getenv("WALLET_ALERT_WHATSAPP_API_KEY"))
	walletAlertWhatsAppPhoneNumberID := strings.TrimSpace(os.Getenv("WALLET_ALERT_WHATSAPP_PHONE_NUMBER_ID"))
	walletAlertWhatsAppTimeout := parseDurationOrFallback(
		strings.TrimSpace(os.Getenv("WALLET_ALERT_WHATSAPP_TIMEOUT")),
		5*time.Second,
	)
	if walletAlertWhatsAppTimeout < time.Second {
		walletAlertWhatsAppTimeout = 5 * time.Second
	}

	return Config{
		AppEnv:                                   appEnv,
		HTTPAddr:                                 addr,
		CORSOrigin:                               origin,
		JWTSecret:                                jwtSecret,
		JWTIssuer:                                jwtIssuer,
		JWTAccessTTL:                             accessTTL,
		JWTRefreshTTL:                            refreshTTL,
		EnableDemoUsers:                          enableDemoUsers,
		DatabaseURL:                              dbURL,
		DatabaseAutoMigrate:                      dbAutoMigrate,
		EnterpriseJITProvisionApprovalRequired:   enterpriseJITProvisionApprovalRequired,
		EnterpriseSyncReconcileWorkerEnabled:     enterpriseSyncReconcileWorkerEnabled,
		EnterpriseSyncReconcileWorkerInterval:    enterpriseSyncReconcileWorkerInterval,
		EnterpriseSyncReconcileWorkerBatchSize:   enterpriseSyncReconcileWorkerBatchSize,
		EnterpriseSyncReconcileWorkerMaxAttempts: enterpriseSyncReconcileWorkerMaxAttempts,
		EnterpriseSyncReconcileWorkerRetryCooldown:                   enterpriseSyncReconcileWorkerRetryCooldown,
		EnterpriseSyncReconcileWorkerAlertFailureThreshold:           enterpriseSyncReconcileWorkerAlertFailureThreshold,
		EnterpriseSyncReconcileWorkerForceError:                      enterpriseSyncReconcileWorkerForceError,
		EnterpriseSyncReconcileWorkerForceErrorTenantID:              enterpriseSyncReconcileWorkerForceErrorTenantID,
		EnterpriseJITApprovalExternalSyncWorkerEnabled:               enterpriseJITApprovalExternalSyncWorkerEnabled,
		EnterpriseJITApprovalExternalSyncWorkerInterval:              enterpriseJITApprovalExternalSyncWorkerInterval,
		EnterpriseJITApprovalExternalSyncWorkerBatchSize:             enterpriseJITApprovalExternalSyncWorkerBatchSize,
		EnterpriseJITApprovalExternalSyncWorkerMaxAttempts:           enterpriseJITApprovalExternalSyncWorkerMaxAttempts,
		EnterpriseJITApprovalExternalSyncWorkerRetryCooldown:         enterpriseJITApprovalExternalSyncWorkerRetryCooldown,
		EnterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold: enterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold,
		EnterpriseJITApprovalExternalSyncWorkerForceError:            enterpriseJITApprovalExternalSyncWorkerForceError,
		EnterpriseJITApprovalExternalSyncWorkerForceErrorTenantID:    enterpriseJITApprovalExternalSyncWorkerForceErrorTenantID,
		EnterpriseJITApprovalExternalSyncCallbackToken:               enterpriseJITApprovalExternalSyncCallbackToken,
		GatewayEventsBatchForceRetryableError:                        gatewayEventsBatchForceRetryableError,
		GatewayEventsBatchForceRetryablePrefix:                       gatewayEventsBatchForceRetryablePrefix,
		WalletJobProcessDefaultMaxRetry:                              walletJobProcessDefaultMaxRetry,
		WalletDLQCleanupDefaultLimit:                                 walletDLQCleanupDefaultLimit,
		WalletDLQCleanupDefaultOlderThan:                             walletDLQCleanupDefaultOlderThan,
		WalletDLQAlertThreshold:                                      walletDLQAlertThreshold,
		WalletJobMetricsDefaultWindow:                                walletJobMetricsDefaultWindow,
		WalletAlertDispatchMockTransientFailCount:                    walletAlertDispatchMockTransientFailCount,
		WalletAlertEmailProvider:                                     walletAlertEmailProvider,
		WalletAlertEmailFrom:                                         walletAlertEmailFrom,
		WalletAlertEmailReceiverMap:                                  walletAlertEmailReceiverMap,
		WalletAlertResendEndpoint:                                    walletAlertResendEndpoint,
		WalletAlertResendAPIKey:                                      walletAlertResendAPIKey,
		WalletAlertResendTimeout:                                     walletAlertResendTimeout,
		WalletAlertWhatsAppProvider:                                  walletAlertWhatsAppProvider,
		WalletAlertWhatsAppReceiverMap:                               walletAlertWhatsAppReceiverMap,
		WalletAlertWhatsAppEndpoint:                                  walletAlertWhatsAppEndpoint,
		WalletAlertWhatsAppAPIKey:                                    walletAlertWhatsAppAPIKey,
		WalletAlertWhatsAppPhoneNumberID:                             walletAlertWhatsAppPhoneNumberID,
		WalletAlertWhatsAppTimeout:                                   walletAlertWhatsAppTimeout,
	}
}

func (cfg Config) Validate() error {
	nextEnv := strings.ToLower(strings.TrimSpace(cfg.AppEnv))
	if nextEnv == "production" || nextEnv == "prod" {
		if strings.TrimSpace(cfg.JWTSecret) == "" {
			return errors.New("JWT_SECRET is required when APP_ENV is production")
		}
	}
	return nil
}

func parseDurationOrFallback(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}

	if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	if value, err := time.ParseDuration(raw); err == nil && value > 0 {
		return value
	}

	return fallback
}

func parseBoolOrFallback(raw string, fallback bool) bool {
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func parseIntOrFallback(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func parseGroupEmailMap(raw string) map[string][]string {
	if raw == "" {
		return nil
	}

	groups := make(map[string][]string)
	segments := strings.Split(raw, ";")
	for i := range segments {
		next := strings.TrimSpace(segments[i])
		if next == "" {
			continue
		}
		parts := strings.SplitN(next, "=", 2)
		if len(parts) != 2 {
			continue
		}
		group := strings.ToLower(strings.TrimSpace(parts[0]))
		if group == "" {
			continue
		}
		recipients := strings.Split(parts[1], ",")
		unique := make([]string, 0, len(recipients))
		seen := map[string]struct{}{}
		for j := range recipients {
			email := strings.TrimSpace(recipients[j])
			if email == "" {
				continue
			}
			key := strings.ToLower(email)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			unique = append(unique, email)
		}
		if len(unique) == 0 {
			continue
		}
		groups[group] = unique
	}
	if len(groups) == 0 {
		return nil
	}
	return groups
}
