package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                                                       string
	HTTPAddr                                                     string
	CORSOrigin                                                   string
	RedisAddr                                                    string
	RedisPassword                                                string
	RedisDB                                                      int
	RedisKeyPrefix                                               string
	RedisDialTimeout                                             time.Duration
	RedisReadTimeout                                             time.Duration
	RedisWriteTimeout                                            time.Duration
	MQTTEnabled                                                  bool
	MQTTBrokerURL                                                string
	MQTTTopicPrefix                                              string
	GatewayBootstrapToken                                        string
	NATSEnabled                                                  bool
	NATSServerURL                                                string
	NATSSubjectPrefix                                            string
	OTelEnabled                                                  bool
	OTelServiceName                                              string
	OTelExporterOTLPEndpoint                                     string
	OTelExporterOTLPInsecure                                     bool
	OTelTraceSampleRatio                                         float64
	OTelExportTimeout                                            time.Duration
	AuthAdminMFARequired                                         bool
	ExternalAuthEnabled                                          bool
	ExternalAuthProvider                                         string
	ExternalAuthUserInfoURL                                      string
	ExternalAuthTimeout                                          time.Duration
	ExternalAuthDefaultRole                                      string
	JWTSecret                                                    string
	JWTIssuer                                                    string
	JWTAccessTTL                                                 time.Duration
	JWTRefreshTTL                                                time.Duration
	EnableDemoUsers                                              bool
	WebAuthnRPDisplayName                                        string
	WebAuthnRPID                                                 string
	WebAuthnRPOrigins                                            []string
	UserInvitationEmailProvider                                  string
	UserInvitationEmailFrom                                      string
	UserInvitationResendEndpoint                                 string
	UserInvitationResendAPIKey                                   string
	UserInvitationResendTimeout                                  time.Duration
	UserInvitationProviderWebhookSecret                          string
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
	EnterpriseSyncWorkerAlertAutoRetryWorkerEnabled              bool
	EnterpriseSyncWorkerAlertAutoRetryWorkerInterval             time.Duration
	EnterpriseSyncWorkerAlertAutoRetryWorkerBatchSize            int
	EnterpriseSyncWorkerAlertAutoRetryWorkerMaxAttempts          int
	EnterpriseSyncWorkerAlertAutoRetryWorkerBaseBackoff          time.Duration
	EnterpriseSyncWorkerAlertAutoRetryWorkerMaxBackoff           time.Duration
	EnterpriseSyncWorkerAlertAutoRetryWorkerLockTTL              time.Duration
	EnterpriseJITApprovalExternalSyncWorkerEnabled               bool
	EnterpriseJITApprovalExternalSyncWorkerInterval              time.Duration
	EnterpriseJITApprovalExternalSyncWorkerBatchSize             int
	EnterpriseJITApprovalExternalSyncWorkerMaxAttempts           int
	EnterpriseJITApprovalExternalSyncWorkerRetryCooldown         time.Duration
	EnterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold int
	EnterpriseJITApprovalExternalSyncWorkerForceError            bool
	EnterpriseJITApprovalExternalSyncWorkerForceErrorTenantID    string
	EnterpriseJITApprovalExternalSyncCallbackToken               string
	EnterpriseHRISWebhookReceiptWorkerEnabled                    bool
	EnterpriseHRISWebhookReceiptWorkerInterval                   time.Duration
	EnterpriseHRISWebhookReceiptWorkerBatchSize                  int
	EnterpriseHRISWebhookReceiptWorkerMaxAttempts                int
	EnterpriseHRISWebhookReceiptWorkerRetryCooldown              time.Duration
	EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff            time.Duration
	EnterpriseHRISWebhookReceiptWorkerProcessingTimeout          time.Duration
	EnterpriseHRISWebhookReceiptWorkerAlertFailureThreshold      int
	EnterpriseHRISWebhookReceiptWorkerLockTTL                    time.Duration
	EnterpriseHRISWebhookDLQWorkerEnabled                        bool
	EnterpriseHRISWebhookDLQWorkerInterval                       time.Duration
	EnterpriseHRISWebhookDLQWorkerBatchSize                      int
	EnterpriseHRISWebhookDLQWorkerMaxAttempts                    int
	EnterpriseHRISWebhookDLQWorkerRetryCooldown                  time.Duration
	EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff                time.Duration
	EnterpriseHRISWebhookDLQWorkerProcessingTimeout              time.Duration
	EnterpriseHRISWebhookDLQWorkerAlertFailureThreshold          int
	EnterpriseHRISWebhookDLQWorkerLockTTL                        time.Duration
	EnterpriseHRISPullWorkerEnabled                              bool
	EnterpriseHRISPullWorkerInterval                             time.Duration
	EnterpriseHRISPullWorkerBatchSize                            int
	EnterpriseHRISPullWorkerMaxAttempts                          int
	EnterpriseHRISPullWorkerRetryCooldown                        time.Duration
	EnterpriseHRISPullWorkerRetryMaxBackoff                      time.Duration
	EnterpriseHRISPullWorkerProcessingTimeout                    time.Duration
	EnterpriseHRISPullWorkerReconcileInterval                    time.Duration
	EnterpriseHRISPullWorkerAlertFailureThreshold                int
	EnterpriseHRISPullWorkerLockTTL                              time.Duration
	HRISVaultMasterKey                                           string
	HRISVaultMasterKeyPrevious                                   string // for key rotation
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
	LarkAlertWebhookURL                                          string
	LarkVerificationToken                                        string
	UploadStorageDir                                             string
	UploadSigningKey                                             string
	UploadMaxSizeBytes                                           int64
	UploadURLTTL                                                 time.Duration
	SelfRegistrationEnabled                                      bool
	DefaultTimezone                                               string
	ReportEmailEnabled                                           bool
	CameraEnabled                                                bool
	CameraVaultMasterKey                                         string
	CameraSnapshotTimeoutSeconds                                 int
	CameraSnapshotRetentionDays                                  int
	CameraMaxSnapshotsPerEvent                                   int
	OAuth2Enabled                                                bool
}

func FromEnv() Config {
	var cfg Config
	loadServerConfig(&cfg)
	loadRedisConfig(&cfg)
	loadBrokerConfig(&cfg)
	loadOTelConfig(&cfg)
	loadAuthConfig(&cfg)
	loadUserInvitationConfig(&cfg)
	loadDatabaseConfig(&cfg)
	loadEnterpriseConfig(&cfg)
	loadGatewayConfig(&cfg)
	loadUploadConfig(&cfg)
	loadWalletConfig(&cfg)
	loadReportEmailConfig(&cfg)
	loadCameraConfig(&cfg)
	loadOAuth2Config(&cfg)
	return cfg
}

func loadServerConfig(cfg *Config) {
	cfg.AppEnv = envLowerOrDefault("APP_ENV", "development")
	cfg.HTTPAddr = normalizeHTTPAddr(envStringOrDefault("PORT", "8080"))
	cfg.CORSOrigin = envStringOrDefault("CORS_ORIGIN", "http://localhost:5173,http://localhost:5174,http://localhost:5175,http://127.0.0.1:5173,http://127.0.0.1:5175")
	cfg.DefaultTimezone = envStringOrDefault("DEFAULT_TIMEZONE", "UTC")
}

func loadRedisConfig(cfg *Config) {
	cfg.RedisAddr = envString("REDIS_ADDR")
	cfg.RedisPassword = envString("REDIS_PASSWORD")
	cfg.RedisDB = parseIntOrFallback(envString("REDIS_DB"), 0)
	if cfg.RedisDB < 0 {
		cfg.RedisDB = 0
	}
	cfg.RedisKeyPrefix = envStringOrDefault("REDIS_KEY_PREFIX", "mistypass")
	cfg.RedisDialTimeout = parseDurationOrFallback(envString("REDIS_DIAL_TIMEOUT"), 3*time.Second)
	if cfg.RedisDialTimeout < time.Second {
		cfg.RedisDialTimeout = 3 * time.Second
	}
	cfg.RedisReadTimeout = parseDurationOrFallback(envString("REDIS_READ_TIMEOUT"), 3*time.Second)
	if cfg.RedisReadTimeout < time.Second {
		cfg.RedisReadTimeout = 3 * time.Second
	}
	cfg.RedisWriteTimeout = parseDurationOrFallback(envString("REDIS_WRITE_TIMEOUT"), 3*time.Second)
	if cfg.RedisWriteTimeout < time.Second {
		cfg.RedisWriteTimeout = 3 * time.Second
	}
}

func loadBrokerConfig(cfg *Config) {
	cfg.MQTTEnabled = parseBoolOrFallback(envString("MQTT_ENABLED"), false)
	cfg.MQTTBrokerURL = envStringOrDefault("MQTT_BROKER_URL", "tcp://localhost:1883")
	cfg.MQTTTopicPrefix = envStringOrDefault("MQTT_TOPIC_PREFIX", "mistypass")
	cfg.GatewayBootstrapToken = envString("GATEWAY_BOOTSTRAP_TOKEN")
	cfg.NATSEnabled = parseBoolOrFallback(envString("NATS_ENABLED"), false)
	cfg.NATSServerURL = envStringOrDefault("NATS_SERVER_URL", "nats://localhost:4222")
	cfg.NATSSubjectPrefix = envStringOrDefault("NATS_SUBJECT_PREFIX", "mistypass")
}

func loadOTelConfig(cfg *Config) {
	cfg.OTelEnabled = parseBoolOrFallback(envString("OTEL_ENABLED"), false)
	cfg.OTelServiceName = envStringOrDefault("OTEL_SERVICE_NAME", "mistypass-api")
	cfg.OTelExporterOTLPEndpoint = envString("OTEL_EXPORTER_OTLP_ENDPOINT")
	cfg.OTelExporterOTLPInsecure = parseBoolOrFallback(envString("OTEL_EXPORTER_OTLP_INSECURE"), false)
	cfg.OTelTraceSampleRatio = parseFloatOrFallback(envString("OTEL_TRACE_SAMPLE_RATIO"), 1.0)
	if cfg.OTelTraceSampleRatio < 0 {
		cfg.OTelTraceSampleRatio = 0
	}
	if cfg.OTelTraceSampleRatio > 1 {
		cfg.OTelTraceSampleRatio = 1
	}
	cfg.OTelExportTimeout = parseDurationOrFallback(envString("OTEL_EXPORT_TIMEOUT"), 5*time.Second)
	if cfg.OTelExportTimeout < time.Second {
		cfg.OTelExportTimeout = 5 * time.Second
	}
}

func loadAuthConfig(cfg *Config) {
	cfg.AuthAdminMFARequired = parseBoolOrFallback(envString("AUTH_ADMIN_MFA_REQUIRED"), false)
	cfg.ExternalAuthEnabled = parseBoolOrFallback(envString("EXTERNAL_AUTH_ENABLED"), false)
	cfg.ExternalAuthProvider = envLowerOrDefault("EXTERNAL_AUTH_PROVIDER", "generic_oidc")
	cfg.ExternalAuthUserInfoURL = envString("EXTERNAL_AUTH_USERINFO_URL")
	cfg.ExternalAuthTimeout = parseDurationOrFallback(envString("EXTERNAL_AUTH_TIMEOUT"), 8*time.Second)
	if cfg.ExternalAuthTimeout < time.Second {
		cfg.ExternalAuthTimeout = 8 * time.Second
	}
	cfg.ExternalAuthDefaultRole = envLowerOrDefault("EXTERNAL_AUTH_DEFAULT_ROLE", "resident")
	cfg.JWTSecret = envString("JWT_SECRET")
	cfg.JWTIssuer = envStringOrDefault("JWT_ISSUER", "mistypass-api")
	cfg.JWTAccessTTL = parseDurationOrFallback(envString("JWT_ACCESS_TTL"), time.Hour)
	cfg.JWTRefreshTTL = parseDurationOrFallback(envString("JWT_REFRESH_TTL"), 7*24*time.Hour)
	if raw := envString("ENABLE_DEMO_USERS"); raw != "" {
		cfg.EnableDemoUsers = parseBoolOrFallback(raw, false)
	}
	cfg.WebAuthnRPDisplayName = envStringOrDefault("WEBAUTHN_RP_DISPLAY_NAME", "MistyPass")
	cfg.WebAuthnRPID = envString("WEBAUTHN_RP_ID")
	if raw := envString("WEBAUTHN_RP_ORIGINS"); raw != "" {
		origins := strings.Split(raw, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		cfg.WebAuthnRPOrigins = origins
	}
}

func loadUserInvitationConfig(cfg *Config) {
	cfg.UserInvitationEmailProvider = envLowerOrDefault("USER_INVITATION_EMAIL_PROVIDER", "queue")
	switch cfg.UserInvitationEmailProvider {
	case "queue", "mock", "resend":
	default:
		cfg.UserInvitationEmailProvider = "queue"
	}
	cfg.UserInvitationEmailFrom = envStringOrDefault("USER_INVITATION_EMAIL_FROM", "no-reply@mistypass.local")
	cfg.UserInvitationResendEndpoint = envString("USER_INVITATION_RESEND_ENDPOINT")
	cfg.UserInvitationResendAPIKey = envString("USER_INVITATION_RESEND_API_KEY")
	cfg.UserInvitationResendTimeout = parseDurationOrFallback(
		envString("USER_INVITATION_RESEND_TIMEOUT"),
		5*time.Second,
	)
	if cfg.UserInvitationResendTimeout < time.Second {
		cfg.UserInvitationResendTimeout = 5 * time.Second
	}
	cfg.UserInvitationProviderWebhookSecret = envString("USER_INVITATION_PROVIDER_WEBHOOK_SECRET")
}

func loadDatabaseConfig(cfg *Config) {
	cfg.DatabaseURL = envString("DATABASE_URL")
	cfg.DatabaseAutoMigrate = parseBoolOrFallback(envString("DATABASE_AUTO_MIGRATE"), true)
}

func loadEnterpriseConfig(cfg *Config) {
	cfg.EnterpriseJITProvisionApprovalRequired = parseBoolOrFallback(
		envString("ENTERPRISE_JIT_PROVISION_APPROVAL_REQUIRED"),
		false,
	)
	loadEnterpriseSyncReconcileConfig(cfg)
	loadEnterpriseSyncWorkerAlertAutoRetryConfig(cfg)
	loadEnterpriseJITApprovalExternalSyncConfig(cfg)
	loadEnterpriseHRISWebhookReceiptConfig(cfg)
	loadEnterpriseHRISWebhookDLQConfig(cfg)
	loadEnterpriseHRISPullConfig(cfg)
	cfg.HRISVaultMasterKey = envString("HRIS_VAULT_MASTER_KEY")
	cfg.HRISVaultMasterKeyPrevious = envString("HRIS_VAULT_MASTER_KEY_PREVIOUS")
}

func loadEnterpriseSyncReconcileConfig(cfg *Config) {
	cfg.EnterpriseSyncReconcileWorkerEnabled = parseBoolOrFallback(
		envString("ENTERPRISE_SYNC_RECONCILE_WORKER_ENABLED"),
		false,
	)
	cfg.EnterpriseSyncReconcileWorkerInterval = parseDurationOrFallback(
		envString("ENTERPRISE_SYNC_RECONCILE_WORKER_INTERVAL"),
		30*time.Second,
	)
	cfg.EnterpriseSyncReconcileWorkerBatchSize = parseIntOrFallback(
		envString("ENTERPRISE_SYNC_RECONCILE_WORKER_BATCH_SIZE"),
		20,
	)
	if cfg.EnterpriseSyncReconcileWorkerBatchSize < 1 {
		cfg.EnterpriseSyncReconcileWorkerBatchSize = 1
	}
	cfg.EnterpriseSyncReconcileWorkerMaxAttempts = parseIntOrFallback(
		envString("ENTERPRISE_SYNC_RECONCILE_WORKER_MAX_ATTEMPTS"),
		5,
	)
	if cfg.EnterpriseSyncReconcileWorkerMaxAttempts < 1 {
		cfg.EnterpriseSyncReconcileWorkerMaxAttempts = 1
	}
	cfg.EnterpriseSyncReconcileWorkerRetryCooldown = parseDurationOrFallback(
		envString("ENTERPRISE_SYNC_RECONCILE_WORKER_RETRY_COOLDOWN"),
		30*time.Second,
	)
	cfg.EnterpriseSyncReconcileWorkerAlertFailureThreshold = parseIntOrFallback(
		envString("ENTERPRISE_SYNC_RECONCILE_WORKER_ALERT_FAILURE_THRESHOLD"),
		3,
	)
	if cfg.EnterpriseSyncReconcileWorkerAlertFailureThreshold < 1 {
		cfg.EnterpriseSyncReconcileWorkerAlertFailureThreshold = 1
	}
	cfg.EnterpriseSyncReconcileWorkerForceError = parseBoolOrFallback(
		envString("ENTERPRISE_SYNC_RECONCILE_WORKER_FORCE_ERROR"),
		false,
	)
	cfg.EnterpriseSyncReconcileWorkerForceErrorTenantID = envString(
		"ENTERPRISE_SYNC_RECONCILE_WORKER_FORCE_ERROR_TENANT_ID",
	)
}

func loadEnterpriseSyncWorkerAlertAutoRetryConfig(cfg *Config) {
	cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerEnabled = parseBoolOrFallback(
		envString("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_ENABLED"),
		false,
	)
	cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerInterval = parseDurationOrFallback(
		envString("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_INTERVAL"),
		30*time.Second,
	)
	cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBatchSize = parseIntOrFallback(
		envString("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_BATCH_SIZE"),
		20,
	)
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBatchSize < 1 {
		cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBatchSize = 1
	}
	cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxAttempts = parseIntOrFallback(
		envString("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_MAX_ATTEMPTS"),
		3,
	)
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxAttempts < 1 {
		cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxAttempts = 1
	}
	cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBaseBackoff = parseDurationOrFallback(
		envString("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_BASE_BACKOFF"),
		5*time.Minute,
	)
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBaseBackoff < time.Second {
		cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBaseBackoff = 5 * time.Minute
	}
	cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxBackoff = parseDurationOrFallback(
		envString("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_MAX_BACKOFF"),
		time.Hour,
	)
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxBackoff < cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBaseBackoff {
		cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxBackoff = cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBaseBackoff
	}
	cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerLockTTL = parseDurationOrFallback(
		envString("ENTERPRISE_SYNC_WORKER_ALERT_AUTO_RETRY_WORKER_LOCK_TTL"),
		10*time.Minute,
	)
	if cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerLockTTL < time.Second {
		cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerLockTTL = 10 * time.Minute
	}
}

func loadEnterpriseJITApprovalExternalSyncConfig(cfg *Config) {
	cfg.EnterpriseJITApprovalExternalSyncWorkerEnabled = parseBoolOrFallback(
		envString("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_ENABLED"),
		false,
	)
	cfg.EnterpriseJITApprovalExternalSyncWorkerInterval = parseDurationOrFallback(
		envString("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_INTERVAL"),
		30*time.Second,
	)
	cfg.EnterpriseJITApprovalExternalSyncWorkerBatchSize = parseIntOrFallback(
		envString("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_BATCH_SIZE"),
		20,
	)
	if cfg.EnterpriseJITApprovalExternalSyncWorkerBatchSize < 1 {
		cfg.EnterpriseJITApprovalExternalSyncWorkerBatchSize = 1
	}
	cfg.EnterpriseJITApprovalExternalSyncWorkerMaxAttempts = parseIntOrFallback(
		envString("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_MAX_ATTEMPTS"),
		5,
	)
	if cfg.EnterpriseJITApprovalExternalSyncWorkerMaxAttempts < 1 {
		cfg.EnterpriseJITApprovalExternalSyncWorkerMaxAttempts = 1
	}
	cfg.EnterpriseJITApprovalExternalSyncWorkerRetryCooldown = parseDurationOrFallback(
		envString("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_RETRY_COOLDOWN"),
		30*time.Second,
	)
	cfg.EnterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold = parseIntOrFallback(
		envString("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_ALERT_FAILURE_THRESHOLD"),
		3,
	)
	if cfg.EnterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold < 1 {
		cfg.EnterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold = 1
	}
	cfg.EnterpriseJITApprovalExternalSyncWorkerForceError = parseBoolOrFallback(
		envString("ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_FORCE_ERROR"),
		false,
	)
	cfg.EnterpriseJITApprovalExternalSyncWorkerForceErrorTenantID = envString(
		"ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_WORKER_FORCE_ERROR_TENANT_ID",
	)
	cfg.EnterpriseJITApprovalExternalSyncCallbackToken = envString(
		"ENTERPRISE_JIT_APPROVAL_EXTERNAL_SYNC_CALLBACK_TOKEN",
	)
}

func loadEnterpriseHRISWebhookReceiptConfig(cfg *Config) {
	cfg.EnterpriseHRISWebhookReceiptWorkerEnabled = parseBoolOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_ENABLED"),
		false,
	)
	cfg.EnterpriseHRISWebhookReceiptWorkerInterval = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_INTERVAL"),
		30*time.Second,
	)
	cfg.EnterpriseHRISWebhookReceiptWorkerBatchSize = parseIntOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_BATCH_SIZE"),
		20,
	)
	if cfg.EnterpriseHRISWebhookReceiptWorkerBatchSize < 1 {
		cfg.EnterpriseHRISWebhookReceiptWorkerBatchSize = 1
	}
	cfg.EnterpriseHRISWebhookReceiptWorkerMaxAttempts = parseIntOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_MAX_ATTEMPTS"),
		5,
	)
	if cfg.EnterpriseHRISWebhookReceiptWorkerMaxAttempts < 1 {
		cfg.EnterpriseHRISWebhookReceiptWorkerMaxAttempts = 1
	}
	cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_RETRY_COOLDOWN"),
		30*time.Second,
	)
	cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_RETRY_MAX_BACKOFF"),
		cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown,
	)
	if cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown <= 0 {
		cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff = 0
	} else if cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff < cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown {
		cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff = cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown
	}
	cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_PROCESSING_TIMEOUT"),
		5*time.Minute,
	)
	if cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout < time.Second {
		cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout = 5 * time.Minute
	}
	cfg.EnterpriseHRISWebhookReceiptWorkerAlertFailureThreshold = parseIntOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_ALERT_FAILURE_THRESHOLD"),
		3,
	)
	if cfg.EnterpriseHRISWebhookReceiptWorkerAlertFailureThreshold < 1 {
		cfg.EnterpriseHRISWebhookReceiptWorkerAlertFailureThreshold = 1
	}
	cfg.EnterpriseHRISWebhookReceiptWorkerLockTTL = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_RECEIPT_WORKER_LOCK_TTL"),
		10*time.Minute,
	)
	if cfg.EnterpriseHRISWebhookReceiptWorkerLockTTL < time.Second {
		cfg.EnterpriseHRISWebhookReceiptWorkerLockTTL = 10 * time.Minute
	}
}

func loadEnterpriseHRISWebhookDLQConfig(cfg *Config) {
	cfg.EnterpriseHRISWebhookDLQWorkerEnabled = parseBoolOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_ENABLED"),
		false,
	)
	cfg.EnterpriseHRISWebhookDLQWorkerInterval = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_INTERVAL"),
		30*time.Second,
	)
	cfg.EnterpriseHRISWebhookDLQWorkerBatchSize = parseIntOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_BATCH_SIZE"),
		20,
	)
	if cfg.EnterpriseHRISWebhookDLQWorkerBatchSize < 1 {
		cfg.EnterpriseHRISWebhookDLQWorkerBatchSize = 1
	}
	cfg.EnterpriseHRISWebhookDLQWorkerMaxAttempts = parseIntOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_MAX_ATTEMPTS"),
		5,
	)
	if cfg.EnterpriseHRISWebhookDLQWorkerMaxAttempts < 1 {
		cfg.EnterpriseHRISWebhookDLQWorkerMaxAttempts = 1
	}
	cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_RETRY_COOLDOWN"),
		30*time.Second,
	)
	cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_RETRY_MAX_BACKOFF"),
		cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown,
	)
	if cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown <= 0 {
		cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff = 0
	} else if cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff < cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown {
		cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff = cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown
	}
	cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_PROCESSING_TIMEOUT"),
		5*time.Minute,
	)
	if cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout < time.Second {
		cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout = 5 * time.Minute
	}
	cfg.EnterpriseHRISWebhookDLQWorkerAlertFailureThreshold = parseIntOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_ALERT_FAILURE_THRESHOLD"),
		3,
	)
	if cfg.EnterpriseHRISWebhookDLQWorkerAlertFailureThreshold < 1 {
		cfg.EnterpriseHRISWebhookDLQWorkerAlertFailureThreshold = 1
	}
	cfg.EnterpriseHRISWebhookDLQWorkerLockTTL = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_WEBHOOK_DLQ_WORKER_LOCK_TTL"),
		10*time.Minute,
	)
	if cfg.EnterpriseHRISWebhookDLQWorkerLockTTL < time.Second {
		cfg.EnterpriseHRISWebhookDLQWorkerLockTTL = 10 * time.Minute
	}
}

func loadEnterpriseHRISPullConfig(cfg *Config) {
	cfg.EnterpriseHRISPullWorkerEnabled = parseBoolOrFallback(
		envString("ENTERPRISE_HRIS_PULL_WORKER_ENABLED"),
		false,
	)
	cfg.EnterpriseHRISPullWorkerInterval = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_PULL_WORKER_INTERVAL"),
		time.Hour,
	)
	cfg.EnterpriseHRISPullWorkerBatchSize = parseIntOrFallback(
		envString("ENTERPRISE_HRIS_PULL_WORKER_BATCH_SIZE"),
		10,
	)
	if cfg.EnterpriseHRISPullWorkerBatchSize < 1 {
		cfg.EnterpriseHRISPullWorkerBatchSize = 1
	}
	cfg.EnterpriseHRISPullWorkerMaxAttempts = parseIntOrFallback(
		envString("ENTERPRISE_HRIS_PULL_WORKER_MAX_ATTEMPTS"),
		5,
	)
	if cfg.EnterpriseHRISPullWorkerMaxAttempts < 1 {
		cfg.EnterpriseHRISPullWorkerMaxAttempts = 1
	}
	cfg.EnterpriseHRISPullWorkerRetryCooldown = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_PULL_WORKER_RETRY_COOLDOWN"),
		30*time.Minute,
	)
	cfg.EnterpriseHRISPullWorkerRetryMaxBackoff = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_PULL_WORKER_RETRY_MAX_BACKOFF"),
		cfg.EnterpriseHRISPullWorkerRetryCooldown,
	)
	if cfg.EnterpriseHRISPullWorkerRetryCooldown <= 0 {
		cfg.EnterpriseHRISPullWorkerRetryMaxBackoff = 0
	} else if cfg.EnterpriseHRISPullWorkerRetryMaxBackoff < cfg.EnterpriseHRISPullWorkerRetryCooldown {
		cfg.EnterpriseHRISPullWorkerRetryMaxBackoff = cfg.EnterpriseHRISPullWorkerRetryCooldown
	}
	cfg.EnterpriseHRISPullWorkerProcessingTimeout = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_PULL_WORKER_PROCESSING_TIMEOUT"),
		30*time.Minute,
	)
	if cfg.EnterpriseHRISPullWorkerProcessingTimeout < time.Second {
		cfg.EnterpriseHRISPullWorkerProcessingTimeout = 30 * time.Minute
	}
	cfg.EnterpriseHRISPullWorkerReconcileInterval = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_PULL_WORKER_RECONCILE_INTERVAL"),
		24*time.Hour,
	)
	if cfg.EnterpriseHRISPullWorkerReconcileInterval <= 0 {
		cfg.EnterpriseHRISPullWorkerReconcileInterval = 24 * time.Hour
	}
	cfg.EnterpriseHRISPullWorkerAlertFailureThreshold = parseIntOrFallback(
		envString("ENTERPRISE_HRIS_PULL_WORKER_ALERT_FAILURE_THRESHOLD"),
		3,
	)
	if cfg.EnterpriseHRISPullWorkerAlertFailureThreshold < 1 {
		cfg.EnterpriseHRISPullWorkerAlertFailureThreshold = 1
	}
	cfg.EnterpriseHRISPullWorkerLockTTL = parseDurationOrFallback(
		envString("ENTERPRISE_HRIS_PULL_WORKER_LOCK_TTL"),
		10*time.Minute,
	)
	if cfg.EnterpriseHRISPullWorkerLockTTL < time.Second {
		cfg.EnterpriseHRISPullWorkerLockTTL = 10 * time.Minute
	}
}

func loadGatewayConfig(cfg *Config) {
	cfg.GatewayEventsBatchForceRetryableError = parseBoolOrFallback(
		envString("GATEWAY_EVENTS_BATCH_FORCE_RETRYABLE_ERROR"),
		false,
	)
	cfg.GatewayEventsBatchForceRetryablePrefix = envStringOrDefault(
		"GATEWAY_EVENTS_BATCH_FORCE_RETRYABLE_PREFIX",
		"force-retry-",
	)
}

func loadUploadConfig(cfg *Config) {
	cfg.UploadStorageDir = envStringOrDefault("UPLOAD_STORAGE_DIR", "")
	cfg.UploadSigningKey = envString("UPLOAD_SIGNING_KEY")
	cfg.UploadMaxSizeBytes = int64(parseIntOrFallback(envString("UPLOAD_MAX_SIZE_BYTES"), 10*1024*1024)) // 10MB
	if cfg.UploadMaxSizeBytes <= 0 {
		cfg.UploadMaxSizeBytes = 10 * 1024 * 1024
	}
	cfg.UploadURLTTL = parseDurationOrFallback(envString("UPLOAD_URL_TTL"), 15*time.Minute)
	if cfg.UploadURLTTL < time.Minute {
		cfg.UploadURLTTL = 15 * time.Minute
	}
	// Self-registration is disabled by default; enable with SELF_REGISTRATION_ENABLED=true
	cfg.SelfRegistrationEnabled = parseBoolOrFallback(envString("SELF_REGISTRATION_ENABLED"), false)
}

func loadWalletConfig(cfg *Config) {
	cfg.WalletJobProcessDefaultMaxRetry = parseIntOrFallback(
		envString("WALLET_JOB_PROCESS_DEFAULT_MAX_RETRY"),
		3,
	)
	if cfg.WalletJobProcessDefaultMaxRetry < 1 {
		cfg.WalletJobProcessDefaultMaxRetry = 1
	}
	cfg.WalletDLQCleanupDefaultLimit = parseIntOrFallback(
		envString("WALLET_DLQ_CLEANUP_DEFAULT_LIMIT"),
		50,
	)
	if cfg.WalletDLQCleanupDefaultLimit < 1 {
		cfg.WalletDLQCleanupDefaultLimit = 1
	}
	cfg.WalletDLQCleanupDefaultOlderThan = parseDurationOrFallback(
		envString("WALLET_DLQ_CLEANUP_DEFAULT_OLDER_THAN"),
		24*time.Hour,
	)
	if cfg.WalletDLQCleanupDefaultOlderThan < time.Second {
		cfg.WalletDLQCleanupDefaultOlderThan = 24 * time.Hour
	}
	cfg.WalletDLQAlertThreshold = parseIntOrFallback(
		envString("WALLET_DLQ_ALERT_THRESHOLD"),
		20,
	)
	if cfg.WalletDLQAlertThreshold < 1 {
		cfg.WalletDLQAlertThreshold = 1
	}
	cfg.WalletJobMetricsDefaultWindow = parseDurationOrFallback(
		envString("WALLET_JOB_METRICS_DEFAULT_WINDOW"),
		15*time.Minute,
	)
	if cfg.WalletJobMetricsDefaultWindow < time.Second {
		cfg.WalletJobMetricsDefaultWindow = 15 * time.Minute
	}
	cfg.WalletAlertDispatchMockTransientFailCount = parseIntOrFallback(
		envString("WALLET_ALERT_DISPATCH_MOCK_TRANSIENT_FAIL_COUNT"),
		0,
	)
	if cfg.WalletAlertDispatchMockTransientFailCount < 0 {
		cfg.WalletAlertDispatchMockTransientFailCount = 0
	}
	if cfg.WalletAlertDispatchMockTransientFailCount > 100 {
		cfg.WalletAlertDispatchMockTransientFailCount = 100
	}
	loadWalletEmailAlertConfig(cfg)
	loadWalletWhatsAppAlertConfig(cfg)
}

func loadWalletEmailAlertConfig(cfg *Config) {
	cfg.WalletAlertEmailProvider = envLowerOrDefault("WALLET_ALERT_EMAIL_PROVIDER", "mock")
	switch cfg.WalletAlertEmailProvider {
	case "mock", "resend":
	case "spaceemail":
		cfg.WalletAlertEmailProvider = "resend"
	default:
		cfg.WalletAlertEmailProvider = "mock"
	}
	cfg.WalletAlertEmailFrom = envStringOrDefault("WALLET_ALERT_EMAIL_FROM", "no-reply@mistypass.local")
	cfg.WalletAlertEmailReceiverMap = parseGroupEmailMap(envString("WALLET_ALERT_EMAIL_RECEIVER_MAP"))
	cfg.WalletAlertResendEndpoint = envString("WALLET_ALERT_RESEND_ENDPOINT")
	if cfg.WalletAlertResendEndpoint == "" {
		cfg.WalletAlertResendEndpoint = envString("WALLET_ALERT_SPACEEMAIL_ENDPOINT")
	}
	cfg.WalletAlertResendAPIKey = envString("WALLET_ALERT_RESEND_API_KEY")
	if cfg.WalletAlertResendAPIKey == "" {
		cfg.WalletAlertResendAPIKey = envString("WALLET_ALERT_SPACEEMAIL_API_KEY")
	}
	timeoutRaw := envString("WALLET_ALERT_RESEND_TIMEOUT")
	if timeoutRaw == "" {
		timeoutRaw = envString("WALLET_ALERT_SPACEEMAIL_TIMEOUT")
	}
	cfg.WalletAlertResendTimeout = parseDurationOrFallback(timeoutRaw, 5*time.Second)
	if cfg.WalletAlertResendTimeout < time.Second {
		cfg.WalletAlertResendTimeout = 5 * time.Second
	}
}

func loadWalletWhatsAppAlertConfig(cfg *Config) {
	cfg.WalletAlertWhatsAppProvider = envLowerOrDefault("WALLET_ALERT_WHATSAPP_PROVIDER", "mock")
	switch cfg.WalletAlertWhatsAppProvider {
	case "mock", "meta":
	default:
		cfg.WalletAlertWhatsAppProvider = "mock"
	}
	cfg.WalletAlertWhatsAppReceiverMap = parseGroupEmailMap(envString("WALLET_ALERT_WHATSAPP_RECEIVER_MAP"))
	cfg.WalletAlertWhatsAppEndpoint = envString("WALLET_ALERT_WHATSAPP_ENDPOINT")
	cfg.WalletAlertWhatsAppAPIKey = envString("WALLET_ALERT_WHATSAPP_API_KEY")
	cfg.WalletAlertWhatsAppPhoneNumberID = envString("WALLET_ALERT_WHATSAPP_PHONE_NUMBER_ID")
	cfg.WalletAlertWhatsAppTimeout = parseDurationOrFallback(
		envString("WALLET_ALERT_WHATSAPP_TIMEOUT"),
		5*time.Second,
	)
	if cfg.WalletAlertWhatsAppTimeout < time.Second {
		cfg.WalletAlertWhatsAppTimeout = 5 * time.Second
	}

	// Lark Bot webhook URL (optional)
	cfg.LarkAlertWebhookURL = envString("LARK_ALERT_WEBHOOK_URL")

	// Lark Event Subscription verification token (for validating incoming events)
	cfg.LarkVerificationToken = envString("LARK_VERIFICATION_TOKEN")
}

func (cfg Config) Validate() error {
	nextEnv := strings.ToLower(strings.TrimSpace(cfg.AppEnv))
	if nextEnv == "production" || nextEnv == "prod" {
		if strings.TrimSpace(cfg.JWTSecret) == "" {
			return errors.New("JWT_SECRET is required when APP_ENV is production")
		}
		if strings.TrimSpace(cfg.HRISVaultMasterKey) == "" {
			return errors.New("HRIS_VAULT_MASTER_KEY is required when APP_ENV is production")
		}
		if strings.TrimSpace(cfg.GatewayBootstrapToken) == "" {
			return errors.New("GATEWAY_BOOTSTRAP_TOKEN is required when APP_ENV is production")
		}
		if strings.TrimSpace(cfg.UploadStorageDir) != "" && strings.TrimSpace(cfg.UploadSigningKey) == "" {
			return errors.New("UPLOAD_SIGNING_KEY is required when uploads are enabled in production")
		}
	}
	if cfg.OTelEnabled {
		if strings.TrimSpace(cfg.OTelExporterOTLPEndpoint) == "" {
			return errors.New("OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_ENABLED is true")
		}
		if cfg.OTelTraceSampleRatio < 0 || cfg.OTelTraceSampleRatio > 1 {
			return errors.New("OTEL_TRACE_SAMPLE_RATIO must be between 0 and 1")
		}
	}
	if cfg.MQTTEnabled {
		brokerURL := strings.TrimSpace(cfg.MQTTBrokerURL)
		if brokerURL == "" {
			return errors.New("MQTT_BROKER_URL is required when MQTT_ENABLED is true")
		}
		parsed, err := url.Parse(brokerURL)
		if err != nil || strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
			return errors.New("MQTT_BROKER_URL must be a valid URL when MQTT_ENABLED is true")
		}
	}
	if cfg.NATSEnabled {
		serverURL := strings.TrimSpace(cfg.NATSServerURL)
		if serverURL == "" {
			return errors.New("NATS_SERVER_URL is required when NATS_ENABLED is true")
		}
		parsed, err := url.Parse(serverURL)
		if err != nil || strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
			return errors.New("NATS_SERVER_URL must be a valid URL when NATS_ENABLED is true")
		}
	}
	if cfg.ExternalAuthEnabled {
		provider := strings.ToLower(strings.TrimSpace(cfg.ExternalAuthProvider))
		switch provider {
		case "generic_oidc", "casdoor", "ory_kratos":
		default:
			return errors.New("EXTERNAL_AUTH_PROVIDER must be one of generic_oidc|casdoor|ory_kratos when EXTERNAL_AUTH_ENABLED is true")
		}
		userInfoURL := strings.TrimSpace(cfg.ExternalAuthUserInfoURL)
		if userInfoURL == "" {
			return errors.New("EXTERNAL_AUTH_USERINFO_URL is required when EXTERNAL_AUTH_ENABLED is true")
		}
		parsed, err := url.Parse(userInfoURL)
		if err != nil || strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
			return errors.New("EXTERNAL_AUTH_USERINFO_URL must be a valid URL when EXTERNAL_AUTH_ENABLED is true")
		}
		role := strings.ToLower(strings.TrimSpace(cfg.ExternalAuthDefaultRole))
		switch role {
		case "super_admin", "tenant_admin", "operator", "building_admin", "resident":
		default:
			return errors.New("EXTERNAL_AUTH_DEFAULT_ROLE must be one of super_admin|tenant_admin|operator|building_admin|resident when EXTERNAL_AUTH_ENABLED is true")
		}
	}
	return nil
}

func envString(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func envStringOrDefault(key, fallback string) string {
	value := envString(key)
	if value == "" {
		return fallback
	}
	return value
}

func envLowerOrDefault(key, fallback string) string {
	value := strings.ToLower(envString(key))
	if value == "" {
		return fallback
	}
	return value
}

func normalizeHTTPAddr(port string) string {
	if strings.Contains(port, ":") {
		return port
	}
	return ":" + port
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

func parseFloatOrFallback(raw string, fallback float64) float64 {
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
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

func loadReportEmailConfig(cfg *Config) {
	cfg.ReportEmailEnabled = parseBoolOrFallback(envString("REPORT_EMAIL_ENABLED"), false)
}

func loadOAuth2Config(cfg *Config) {
	cfg.OAuth2Enabled = parseBoolOrFallback(envString("OAUTH2_ENABLED"), false)
}

func loadCameraConfig(cfg *Config) {
	cfg.CameraEnabled = parseBoolOrFallback(envString("CAMERA_ENABLED"), false)
	cfg.CameraVaultMasterKey = envString("CAMERA_VAULT_MASTER_KEY")
	cfg.CameraSnapshotTimeoutSeconds = parseIntOrFallback(envString("CAMERA_SNAPSHOT_TIMEOUT"), 10)
	cfg.CameraSnapshotRetentionDays = parseIntOrFallback(envString("CAMERA_SNAPSHOT_RETENTION_DAYS"), 30)
	cfg.CameraMaxSnapshotsPerEvent = parseIntOrFallback(envString("CAMERA_MAX_SNAPSHOTS_PER_EVENT"), 3)
}
