package httpx

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mistypass/cloud/api/internal/bus"
	"github.com/mistypass/cloud/api/internal/config"
	secretcrypto "github.com/mistypass/cloud/api/internal/crypto"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/alarm"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
	"github.com/mistypass/cloud/api/internal/modules/hris"
	"github.com/mistypass/cloud/api/internal/modules/hris/talenta"
	"github.com/mistypass/cloud/api/internal/modules/space"
	"github.com/mistypass/cloud/api/internal/modules/tenant"
	"github.com/mistypass/cloud/api/internal/modules/wallet"
	"github.com/mistypass/cloud/api/internal/redistore"
	"github.com/mistypass/cloud/api/internal/state"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type server struct {
	cfg                           config.Config
	stateStore                    state.Store
	gatewayTokenStore             gatewayTokenStore
	logger                        *slog.Logger
	authService                   *auth.Service
	webAuthnEngine                *auth.WebAuthnEngine
	tenantSvc                     *tenant.Service
	spaceSvc                      *space.Service
	gatewaySvc                    *gateway.Service
	accessSvc                     *access.Service
	eventSvc                      *event.Service
	alarmSvc                      *alarm.Service
	auditSvc                      *audit.Service
	walletSvc                     *wallet.Service
	enterpriseSvc                 *enterprise.Service
	hrisVaultSvc                  *hris.VaultService
	hrisDLQSvc                    *hris.DLQService
	hrisPullStateSvc              *hris.PullStateService
	hrisNormalizerRegistry        *hris.Registry
	hrisPullRegistry              *hris.PullRegistry
	stateChangeReader             stateChangeReader
	stateChangeReplayer           stateChangeReplayer
	stateChangeCheckpointReader   stateChangeCheckpointReader
	stateChangeCheckpointReplayer stateChangeCheckpointReplayer
	gatewayTokenMu                sync.RWMutex
	gatewayDeviceTokens           map[string]string
	gatewayBatchFailureMu         sync.Mutex
	gatewayBatchFailureSeen       map[string]struct{}
	gatewayAuthzAckMu             sync.RWMutex
	gatewayAuthzAckVersion        map[string]string
	loginRateLimitMu              sync.Mutex
	loginRateLimitBuckets         map[string]loginRateLimitBucket
	apiRateLimitMu                sync.Mutex
	apiRateLimitBuckets           map[string]loginRateLimitBucket
	enterprisePublicRateLimitMu   sync.Mutex
	enterprisePublicRateBuckets   map[string]loginRateLimitBucket
	enterpriseWebhookRateLimitMu  sync.Mutex
	enterpriseWebhookRateBuckets  map[string]loginRateLimitBucket
	rateLimitStore                rateLimitStore
	volatileStore                 *redistore.Store
	workerLeaseStore              workerLeaseStore
	workerQueueStore              workerQueueStore
	scheduledReportMu             sync.RWMutex
	scheduledReports              map[string]referenceScheduledReport
	scheduledReportSeq            int
	customAlertPolicyMu           sync.RWMutex
	customAlertPolicies           map[string]referenceAlertPolicy
	customAlertPolicySeq          int
	alertNotificationMu           sync.RWMutex
	alertNotifications            []alertNotification
	alertCooldownMu               sync.RWMutex
	alertCooldowns                map[string]time.Time
	hrisWebhookReceiptWorkerWake  chan struct{}
	hrisWebhookDLQWorkerWake      chan struct{}
	hrisWebhookReceiptWorkerQueue chan enterpriseHRISWebhookReceiptQueuedTask
	hrisWebhookDLQWorkerQueue     chan enterpriseHRISWebhookDLQQueuedTask
	messageBus                    bus.Publisher
	externalAuthHTTPClient        *http.Client
	hrisHTTPClient                *http.Client
	gatewayNonceMu                sync.Mutex
	gatewayNonces                 map[string]time.Time // nonce → expiry (5-min dedup window)
}

type enterpriseHRISWebhookReceiptQueuedTask struct {
	Receipt     enterprise.HRISWebhookReceipt
	RecordDLQ   bool
	ExecutionID string
}

type enterpriseHRISWebhookDLQQueuedTask struct {
	TenantID    string
	Entry       hris.DeadLetterEntry
	AuditSource string
	ExecutionID string
}

type stateChangeReader interface {
	ListStateChanges(stateKey string, limit int) ([]state.StateChangeRecord, error)
}

type gatewayTokenStore interface {
	UpsertGatewayDeviceToken(gatewayID, deviceToken string) error
	VerifyGatewayDeviceToken(gatewayID, providedToken string) (exists bool, matched bool, err error)
}

type stateChangeReplayer interface {
	ReplayStateChanges(stateKey string, fromID int64, limit int) (state.ReplayStateChangesResult, error)
}

type stateChangeCheckpointReader interface {
	ListReplayCheckpoints(stateKey string, limit int) ([]state.ReplayCheckpoint, error)
}

type stateChangeCheckpointReplayer interface {
	ReplayStateChangesFromCheckpoint(stateKey string, limit int) (state.ReplayFromCheckpointResult, error)
}

type rateLimitStore interface {
	AllowRateLimit(scope, key string, now time.Time, window time.Duration, maxAttempts int) (bool, time.Duration, error)
}

type workerLeaseStore interface {
	TryAcquireLease(key, token string, ttl time.Duration) (bool, error)
	ReleaseLease(key, token string) error
}

type workerQueueStore interface {
	EnqueueWorkerQueue(queueName, itemID string) error
	ClaimWorkerQueueBatch(queueName string, batchSize int, visibilityTimeout time.Duration) ([]redistore.WorkerQueueClaim, error)
	AckWorkerQueue(queueName, itemID, claimToken string) (bool, error)
	RequeueWorkerQueue(queueName, itemID, claimToken string) (bool, error)
	DescribeWorkerQueue(queueName string, itemIDs []string) (redistore.WorkerQueueTelemetry, error)
}

type loginRateLimitBucket struct {
	WindowStart time.Time
	Attempts    int
}

type authContextKey string

const authUserContextKey authContextKey = "auth_user"
const gatewayEventsBatchMaxItems = 200
const gatewayBootstrapStateKey = "http_gateway_bootstrap"

const (
	gatewayConfigAuthzCacheTTLSeconds          = 300
	gatewayConfigAuthzCacheMaxStaleSeconds     = 900
	gatewayConfigAuthzCacheRefreshRetrySeconds = 30
	loginRateLimitMaxAttempts                  = 10
	loginRateLimitWindow                       = time.Minute
	loginRateLimitBucketTTL                    = 5 * time.Minute
	loginRateLimitBucketMaxKeys                = 10000
	apiRateLimitMaxRequests                    = 600
	apiRateLimitWindow                         = time.Minute
	apiRateLimitBucketTTL                      = 10 * time.Minute
	apiRateLimitBucketMaxKeys                  = 20000
	enterprisePublicRateLimitMaxRequests       = 60
	enterprisePublicRateLimitWindow            = time.Minute
	enterprisePublicRateLimitBucketTTL         = 10 * time.Minute
	enterprisePublicRateLimitBucketMaxKeys     = 10000
	enterpriseWebhookRateLimitMaxRequests      = 240
	enterpriseWebhookRateLimitWindow           = time.Minute
	enterpriseWebhookRateLimitBucketTTL        = 10 * time.Minute
	enterpriseWebhookRateLimitBucketMaxKeys    = 10000
	enterpriseSyncWorkerAlertAutoRetryLeaseKey = "enterprise_sync_worker_alert_auto_retry"
	enterpriseHRISWebhookReceiptLeaseKey       = "enterprise_hris_webhook_receipt_worker"
	enterpriseHRISWebhookDLQLeaseKey           = "enterprise_hris_webhook_dlq_worker"
	enterpriseHRISPullLeaseKey                 = "enterprise_hris_pull_worker"
	enterpriseHRISWebhookReceiptExecutionQueue = "enterprise_hris_webhook_receipt_execution"
	enterpriseHRISWebhookDLQExecutionQueue     = "enterprise_hris_webhook_dlq_execution"
	enterpriseHRISWebhookQueuedTaskBufferSize  = 128
)

func NewRouter(cfg config.Config, stateStore state.Store) (http.Handler, error) {
	handler, _, err := newRouterInternal(cfg, stateStore)
	return handler, err
}

func newRouterInternal(cfg config.Config, stateStore state.Store) (http.Handler, *server, error) {
	tenantSvc := tenant.NewService()
	spaceSvc := space.NewService()
	accessSvc := access.NewService()
	gatewaySvc := gateway.NewService()
	eventSvc := event.NewService()
	alarmSvc := alarm.NewService()
	auditSvc := audit.NewService()
	walletSvc := wallet.NewService()
	enterpriseSvc := enterprise.NewService()
	hrisVaultMasterKey, err := resolveHRISVaultMasterKey(cfg.HRISVaultMasterKey)
	if err != nil {
		return nil, nil, err
	}
	hrisVaultSvc := hris.NewVaultService(hrisVaultMasterKey)
	hrisDLQSvc := hris.NewDLQService()
	hrisPullStateSvc := hris.NewPullStateService()
	hrisNormalizerRegistry := hris.NewRegistry(talenta.NewNormalizer())
	hrisPullRegistry := hris.NewPullRegistry(talenta.NewPullAdapter())
	var stateChangeReaderSvc stateChangeReader
	var stateChangeReplayerSvc stateChangeReplayer
	var stateChangeCheckpointReaderSvc stateChangeCheckpointReader
	var stateChangeCheckpointReplayerSvc stateChangeCheckpointReplayer
	if stateStore != nil {
		var err error
		if reader, ok := stateStore.(stateChangeReader); ok {
			stateChangeReaderSvc = reader
		}
		if replayer, ok := stateStore.(stateChangeReplayer); ok {
			stateChangeReplayerSvc = replayer
		}
		if reader, ok := stateStore.(stateChangeCheckpointReader); ok {
			stateChangeCheckpointReaderSvc = reader
		}
		if replayer, ok := stateStore.(stateChangeCheckpointReplayer); ok {
			stateChangeCheckpointReplayerSvc = replayer
		}
		tenantSvc, err = tenant.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, nil, err
		}
		spaceSvc, err = space.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, nil, err
		}
		accessSvc, err = access.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, nil, err
		}
		gatewaySvc, err = gateway.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, nil, err
		}
		eventSvc, err = event.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, nil, err
		}
		alarmSvc, err = alarm.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, nil, err
		}
		auditSvc, err = audit.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, nil, err
		}
		if jwtSecret := strings.TrimSpace(cfg.JWTSecret); jwtSecret != "" {
			auditHMACKey := sha256.Sum256([]byte("mistypass-audit-hmac:" + jwtSecret))
			auditSvc.SetHMACKey(auditHMACKey[:])
		}
		walletSvc, err = wallet.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, nil, err
		}
		enterpriseSvc, err = enterprise.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, nil, err
		}
		hrisVaultSvc, err = hris.NewVaultServiceWithStateStore(hrisVaultMasterKey, stateStore)
		if err != nil {
			return nil, nil, err
		}
		hrisDLQSvc, err = hris.NewDLQServiceWithStateStore(stateStore)
		if err != nil {
			return nil, nil, err
		}
		hrisPullStateSvc, err = hris.NewPullStateServiceWithStateStore(stateStore)
		if err != nil {
			return nil, nil, err
		}
	}
	walletSvc.SetJobAlertMockTransientFailCount(cfg.WalletAlertDispatchMockTransientFailCount)
	if err := walletSvc.SetJobAlertEmailDeliveryOptions(wallet.JobAlertEmailDeliveryOptions{
		Provider:              cfg.WalletAlertEmailProvider,
		EmailFrom:             cfg.WalletAlertEmailFrom,
		ReceiverMap:           cfg.WalletAlertEmailReceiverMap,
		ResendEndpoint:        cfg.WalletAlertResendEndpoint,
		ResendAPIKey:          cfg.WalletAlertResendAPIKey,
		ResendTimeout:         cfg.WalletAlertResendTimeout,
		WhatsAppProvider:      cfg.WalletAlertWhatsAppProvider,
		WhatsAppReceiverMap:   cfg.WalletAlertWhatsAppReceiverMap,
		WhatsAppEndpoint:      cfg.WalletAlertWhatsAppEndpoint,
		WhatsAppAPIKey:        cfg.WalletAlertWhatsAppAPIKey,
		WhatsAppPhoneNumberID: cfg.WalletAlertWhatsAppPhoneNumberID,
		WhatsAppTimeout:       cfg.WalletAlertWhatsAppTimeout,
	}); err != nil {
		return nil, nil, err
	}
	scheduledReports, scheduledReportSeq := defaultReferenceScheduledReports(time.Now().UTC())

	s := &server{
		cfg:                           cfg,
		stateStore:                    stateStore,
		logger:                        slog.Default(),
		authService:                   auth.NewService(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAccessTTL, cfg.JWTRefreshTTL, cfg.EnableDemoUsers),
		tenantSvc:                     tenantSvc,
		spaceSvc:                      spaceSvc,
		gatewaySvc:                    gatewaySvc,
		accessSvc:                     accessSvc,
		eventSvc:                      eventSvc,
		alarmSvc:                      alarmSvc,
		auditSvc:                      auditSvc,
		walletSvc:                     walletSvc,
		enterpriseSvc:                 enterpriseSvc,
		hrisVaultSvc:                  hrisVaultSvc,
		hrisDLQSvc:                    hrisDLQSvc,
		hrisPullStateSvc:              hrisPullStateSvc,
		hrisNormalizerRegistry:        hrisNormalizerRegistry,
		hrisPullRegistry:              hrisPullRegistry,
		stateChangeReader:             stateChangeReaderSvc,
		stateChangeReplayer:           stateChangeReplayerSvc,
		stateChangeCheckpointReader:   stateChangeCheckpointReaderSvc,
		stateChangeCheckpointReplayer: stateChangeCheckpointReplayerSvc,
		gatewayDeviceTokens:           map[string]string{},
		gatewayBatchFailureSeen:       map[string]struct{}{},
		gatewayAuthzAckVersion:        map[string]string{},
		gatewayNonces:                 map[string]time.Time{},
		loginRateLimitBuckets:         map[string]loginRateLimitBucket{},
		apiRateLimitBuckets:           map[string]loginRateLimitBucket{},
		enterprisePublicRateBuckets:   map[string]loginRateLimitBucket{},
		enterpriseWebhookRateBuckets:  map[string]loginRateLimitBucket{},
		scheduledReports:              scheduledReports,
		scheduledReportSeq:            scheduledReportSeq,
		customAlertPolicies:           map[string]referenceAlertPolicy{},
		alertCooldowns:                map[string]time.Time{},
		hrisWebhookReceiptWorkerWake:  make(chan struct{}, 1),
		hrisWebhookDLQWorkerWake:      make(chan struct{}, 1),
		hrisWebhookReceiptWorkerQueue: make(chan enterpriseHRISWebhookReceiptQueuedTask, enterpriseHRISWebhookQueuedTaskBufferSize),
		hrisWebhookDLQWorkerQueue:     make(chan enterpriseHRISWebhookDLQQueuedTask, enterpriseHRISWebhookQueuedTaskBufferSize),
		externalAuthHTTPClient:        &http.Client{Timeout: cfg.ExternalAuthTimeout},
		hrisHTTPClient:                &http.Client{Timeout: firstNonZeroDuration(cfg.ExternalAuthTimeout, 15*time.Second)},
	}
	webAuthnEngine, err := auth.NewWebAuthnEngine(cfg.WebAuthnRPDisplayName, cfg.WebAuthnRPID, cfg.WebAuthnRPOrigins)
	if err != nil {
		return nil, nil, fmt.Errorf("webauthn init: %w", err)
	}
	s.webAuthnEngine = webAuthnEngine
	s.authService.SetAdminMFARequired(cfg.AuthAdminMFARequired)
	messageBus, err := bus.NewPublisher(cfg.NATSEnabled, cfg.NATSServerURL, cfg.NATSSubjectPrefix)
	if err != nil {
		return nil, nil, err
	}
	s.messageBus = messageBus
	if s.messageBus.Enabled() {
		s.loggerOrDefault().Info(
			"nats internal bus enabled",
			"server_url", cfg.NATSServerURL,
			"subject_prefix", cfg.NATSSubjectPrefix,
		)
		if err := s.startGatewayEventSubscriber(cfg.NATSServerURL, cfg.NATSSubjectPrefix); err != nil {
			s.loggerOrDefault().Warn("gateway event subscriber failed to start", "error", err)
		}
	}
	if authPersistence, ok := stateStore.(auth.Persistence); ok {
		if err := s.authService.SetPersistence(authPersistence); err != nil {
			return nil, nil, err
		}
		if s.webAuthnEngine != nil {
			s.webAuthnEngine.SetPersistence(authPersistence)
		}
	}
	// Attach secret vault for encrypting MFA TOTP secrets at rest.
	// Derives a separate key from the HRIS vault master key using HKDF domain separation.
	if masterKey := strings.TrimSpace(cfg.HRISVaultMasterKey); masterKey != "" {
		var mfaVault *secretcrypto.Vault
		if prevKey := strings.TrimSpace(cfg.HRISVaultMasterKeyPrevious); prevKey != "" {
			mfaVault = secretcrypto.NewVaultWithRotation(masterKey, "mfa-totp-secrets", prevKey)
			s.logger.Info("mfa secret vault enabled with key rotation", "versions", mfaVault.KeyVersionCount(), "current", mfaVault.CurrentVersion())
		} else {
			mfaVault = secretcrypto.NewVault(masterKey, "mfa-totp-secrets")
			s.logger.Info("mfa secret vault enabled (AES-256-GCM + HKDF)")
		}
		s.authService.SetSecretVault(mfaVault)

		// Auto re-encrypt any secrets still on older key versions
		if reEncrypted, err := s.authService.ReEncryptMFASecrets(); err != nil {
			s.logger.Error("mfa secret re-encryption failed", "err", err)
		} else if reEncrypted > 0 {
			s.logger.Info("mfa secrets re-encrypted to current key version", "count", reEncrypted)
		}
	}
	if strings.TrimSpace(cfg.RedisAddr) != "" {
		redisStore, err := redistore.New(redistore.Options{
			Addr:         cfg.RedisAddr,
			Password:     cfg.RedisPassword,
			DB:           cfg.RedisDB,
			KeyPrefix:    cfg.RedisKeyPrefix,
			DialTimeout:  cfg.RedisDialTimeout,
			ReadTimeout:  cfg.RedisReadTimeout,
			WriteTimeout: cfg.RedisWriteTimeout,
		})
		if err != nil {
			return nil, nil, err
		}
		if err := s.authService.SetVolatileStore(redisStore); err != nil {
			_ = redisStore.Close()
			return nil, nil, err
		}
		s.rateLimitStore = redisStore
		s.volatileStore = redisStore
		s.workerLeaseStore = redisStore
		s.workerQueueStore = redisStore
		s.logger.Info("redis volatile store enabled", "addr", cfg.RedisAddr, "db", cfg.RedisDB)
	}
	if tokenStore, ok := stateStore.(gatewayTokenStore); ok {
		s.gatewayTokenStore = tokenStore
	}
	if err := s.restoreGatewayBootstrapState(); err != nil {
		return nil, nil, err
	}
	s.restoreAlertPoliciesFromState()

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(15 * time.Second))
	router.Use(s.withCORS)
	router.Use(s.withTrace)
	router.Use(s.withRequestLog)

	router.Get("/healthz", s.healthz)
	router.Handle("/metrics", promhttp.Handler())
	router.Route("/api/v1", func(r chi.Router) {
		r.Use(s.withGlobalAPIRateLimit)
		r.With(s.withLoginRateLimit).Post("/auth/login", s.login)
		r.With(s.withLoginRateLimit).Post("/auth/external/login", s.externalLogin)
		r.Get("/openapi.json", s.getOpenAPISpec)
		r.Post("/auth/refresh", s.refresh)
		r.Put("/uploads/{uploadID}", s.uploadFile)
		r.Get("/uploads/{uploadID}", s.downloadFile)
		r.With(s.withLoginRateLimit).Post("/users/sign_up", s.userSignUp)
		r.With(s.withLoginRateLimit).Post("/auth/password-reset/request", s.requestPasswordReset)
		r.With(s.withLoginRateLimit).Post("/auth/password-reset/confirm", s.confirmPasswordReset)
		r.With(s.withLoginRateLimit).Post("/auth/webauthn/login/begin", s.webAuthnLoginBegin)
		r.With(s.withLoginRateLimit).Post("/auth/webauthn/login/finish", s.webAuthnLoginFinish)
		r.With(s.withBearerToken).Post("/auth/logout", s.logout)
		r.With(s.withBearerToken).Get("/me", s.me)
		r.With(s.withBearerToken).Get("/user", s.getCurrentUserProfile)
		r.With(s.withBearerToken).Patch("/user", s.updateCurrentUserProfile)
		r.With(s.withEnterprisePublicRateLimit).Post("/enterprise/tenant/resolve", s.resolveEnterpriseTenantByEmail)
		r.With(s.withEnterprisePublicRateLimit).Post("/enterprise/auth/start", s.enterpriseAuthStart)
		r.With(s.withEnterprisePublicRateLimit).Post("/enterprise/auth/exchange", s.enterpriseAuthExchange)
		r.With(s.withEnterprisePublicRateLimit).Post("/enterprise/auth/logout", s.enterpriseAuthLogout)
		r.With(s.withEnterpriseWebhookRateLimit).Post("/enterprise/hris-webhook/{connectorID}", s.receiveEnterpriseHRISWebhook)
		r.With(s.withEnterprisePublicRateLimit).Get("/enterprise/auth/oidc/callback", s.enterpriseOIDCCallback)
		r.With(s.withEnterprisePublicRateLimit).Post("/enterprise/auth/saml/callback", s.enterpriseSAMLCallback)
		r.With(s.withEnterprisePublicRateLimit).Post("/enterprise/jit-provision-approvals/external-sync/callback", s.enterpriseJITApprovalExternalSyncCallback)
		r.With(s.withEnterprisePublicRateLimit).Get("/group_links/verify", s.verifyReferenceGroupLinkToken)
		r.With(s.withEnterprisePublicRateLimit).Post("/group_links/verify", s.verifyReferenceGroupLinkToken)
		r.With(s.withEnterpriseWebhookRateLimit).Post("/users/invitations/provider-receipts", s.receiveUserInvitationProviderReceipt)

		r.Route("/app", func(app chi.Router) {
			app.With(s.withLoginRateLimit).Post("/auth/login", s.appLogin)
			app.Post("/auth/refresh", s.appRefresh)

			app.Group(func(protected chi.Router) {
				protected.Use(s.withBearerToken)
				protected.Use(s.requireRoles("resident"))

				protected.Get("/me", s.appMe)
				protected.Get("/credentials", s.appCredentials)
				protected.Post("/credentials/apple-pass", s.appEnrollApplePass)
				protected.Get("/access/doors", s.appAccessDoors)
				protected.Get("/access/my-doors", s.appAccessMyDoors)
				protected.Post("/access/unlock", s.appUnlockDoor)
				protected.Post("/access/qr-unlock", s.appQRUnlock)
				protected.Get("/access/ble-token", s.appAccessBLEToken)
				protected.Get("/access/logs", s.appAccessLogs)
				protected.Post("/visitor-passes", s.appCreateVisitorPass)
			})
		})

		r.Route("/gateway", func(gatewayRouter chi.Router) {
			gatewayRouter.Post("/register", s.gatewayBootstrapRegister)
			gatewayRouter.Post("/activate", s.gatewayBootstrapActivate)
			gatewayRouter.Post("/heartbeat", s.gatewayBootstrapHeartbeat)
			gatewayRouter.Post("/status", s.gatewayBootstrapStatus)
			gatewayRouter.Post("/config/pull", s.gatewayBootstrapConfigPull)
			gatewayRouter.Post("/config/applied", s.gatewayBootstrapConfigApplied)
			gatewayRouter.Post("/events/access", s.gatewayBootstrapAccessEvent)
			gatewayRouter.Post("/events/device", s.gatewayBootstrapDeviceEvent)
			gatewayRouter.Post("/events/batch", s.gatewayBootstrapEventsBatch)
			gatewayRouter.Post("/events/checkpoint", s.gatewayBootstrapEventsCheckpoint)
			gatewayRouter.Post("/verify-credential", s.verifyCredential)
			gatewayRouter.Post("/ota/report", s.gatewayBootstrapOTAReport)
		})

		r.Group(func(protected chi.Router) {
			protected.Use(s.withBearerToken)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Get("/auth/users/{userID}/building-scope", s.getAuthUserBuildingScope)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Put("/auth/users/{userID}/building-scope", s.updateAuthUserBuildingScope)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/auth/users/{userID}/password-auth", s.updateAuthUserPasswordAuth)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Get("/auth/mfa/admin/status", s.getAdminMFAStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/auth/mfa/admin/setup", s.setupAdminMFA)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/auth/mfa/admin/enable", s.enableAdminMFA)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/auth/mfa/admin/disable", s.disableAdminMFA)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/auth/mfa/admin/regenerate-recovery-codes", s.regenerateAdminMFARecoveryCodes)

			protected.Get("/auth/mfa/user/status", s.getUserMFAStatus)
			protected.Post("/auth/mfa/user/setup", s.setupUserMFA)
			protected.Post("/auth/mfa/user/enable", s.enableUserMFA)
			protected.Post("/auth/mfa/user/disable", s.disableUserMFA)

			protected.Post("/auth/webauthn/register/begin", s.webAuthnRegisterBegin)
			protected.Post("/auth/webauthn/register/finish", s.webAuthnRegisterFinish)
			protected.Get("/auth/webauthn/credentials", s.webAuthnListCredentials)
			protected.Delete("/auth/webauthn/credentials/{credentialID}", s.webAuthnDeleteCredential)

			protected.Post("/uploads/signed-url", s.requestSignedUploadURL)
			protected.Get("/uploads", s.listUserUploads)

			protected.Get("/auth/sessions", s.listLoginSessions)
			protected.Post("/auth/sessions/revoke", s.revokeLoginSession)
			protected.Post("/auth/sessions/revoke-all", s.revokeAllLoginSessions)

			protected.With(s.requireRoles("super_admin")).Get("/tenants", s.listTenants)
			protected.With(s.requireRoles("super_admin")).Post("/tenants", s.createTenant)
			protected.With(s.requireRoles("super_admin")).Patch("/tenants/{tenantID}/status", s.updateTenantStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/tenants/{tenantID}/topology", s.getTenantTopology)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin"), withDeprecatedEndpoint("/api/v1/places")).Get("/buildings", s.listBuildings)
			protected.With(s.requireRoles("super_admin", "tenant_admin"), withDeprecatedEndpoint("/api/v1/places")).Post("/buildings", s.createBuilding)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/floors", s.listFloors)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/floors", s.createFloor)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/floors/{floorID}", s.getFloor)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/floors/{floorID}", s.updateFloor)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/floors/{floorID}", s.deleteFloor)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/areas", s.listAreas)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/areas", s.createArea)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/areas/{areaID}", s.getArea)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/areas/{areaID}", s.updateArea)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin"), withDeprecatedEndpoint("/api/v1/locks")).Get("/doors", s.listDoors)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin"), withDeprecatedEndpoint("/api/v1/locks")).Post("/doors", s.createDoor)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin"), withDeprecatedEndpoint("/api/v1/door_groups")).Get("/door-groups", s.listDoorGroups)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin"), withDeprecatedEndpoint("/api/v1/door_groups")).Post("/door-groups", s.createDoorGroup)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/door_groups", s.listReferenceDoorGroups)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/places", s.listReferencePlaces)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/places", s.createReferencePlace)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/places/{placeID}", s.getReferencePlace)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/places/{placeID}", s.updateReferencePlace)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/places/{placeID}", s.deleteReferencePlace)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/places/{placeID}/lock_down", s.lockDownReferencePlace)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/places/{placeID}/cancel_lockdown", s.cancelReferencePlaceLockdown)
			protected.Post("/places/{placeID}/favorite", s.favoriteReferencePlace)
			protected.Post("/places/{placeID}/unfavorite", s.unfavoriteReferencePlace)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/locks", s.listReferenceLocks)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/locks", s.createReferenceLock)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/locks/{lockID}", s.getReferenceLock)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/locks/{lockID}", s.updateReferenceLock)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/locks/{lockID}", s.deleteReferenceLock)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/locks/{lockID}/unlock", s.unlockReferenceLock)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/locks/{lockID}/lock_down", s.lockDownReferenceLock)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/locks/{lockID}/cancel_lockdown", s.cancelReferenceLockLockdown)
			protected.Post("/locks/{lockID}/favorite", s.favoriteReferenceLock)
			protected.Post("/locks/{lockID}/unfavorite", s.unfavoriteReferenceLock)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/locks/{lockID}/first_to_arrive", s.firstToArriveReferenceLock)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/locks/{lockID}/last_to_leave", s.lastToLeaveReferenceLock)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin"), withDeprecatedEndpoint("/api/v1/controllers", "/api/v1/readers", "/api/v1/terminals")).Get("/gateways", s.listGateways)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/gateways/serial-inventory", s.listGatewaySerialInventory)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/gateways/serial-inventory/import", s.importGatewaySerialInventory)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/gateways/serial-inventory/import-csv", s.importGatewaySerialInventoryCSV)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/gateways/serial-inventory/batch-status", s.batchUpdateGatewaySerialInventoryStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/gateways/serial-inventory/{serialNumber}/status", s.updateGatewaySerialInventoryStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/gateways/serial-inventory/export-csv", s.exportGatewaySerialInventoryCSV)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/gateways/events/checkpoint/summary", s.listGatewayEventCheckpointSummary)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/register", s.registerGateway)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/{gatewayID}/bind-door", s.bindGatewayDoor)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/{gatewayID}/unbind-door", s.unbindGatewayDoor)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/{gatewayID}/devices", s.registerGatewayDevice)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/{gatewayID}/devices/{deviceID}/rs485/telemetry", s.reportGatewayDeviceRS485Telemetry)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/{gatewayID}/devices/probe-legacy", s.probeGatewayLegacyDevices)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/{gatewayID}/config/publish", s.publishGatewayConfig)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/{gatewayID}/reboot", s.rebootGateway)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin", "operator")).Get("/gateways/{gatewayID}/mqtt/bootstrap", s.getGatewayMQTTBootstrap)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/gateways/{gatewayID}/ota/tasks", s.createGatewayOTATask)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/gateways/{gatewayID}/ota/tasks", s.listGatewayOTATasks)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/gateways/{gatewayID}/ota/tasks/{taskID}/status", s.updateGatewayOTATaskStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/gateways/{gatewayID}/events/checkpoint", s.listGatewayEventCheckpoints)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin"), withDeprecatedEndpoint("/api/v1/role_assignments", "/api/v1/groups", "/api/v1/group_locks")).Get("/access-policies", s.listAccessPolicies)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin"), withDeprecatedEndpoint("/api/v1/role_assignments", "/api/v1/groups", "/api/v1/group_locks")).Post("/access-policies", s.createAccessPolicy)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin"), withDeprecatedEndpoint("/api/v1/role_assignments", "/api/v1/groups", "/api/v1/group_locks")).Patch("/access-policies/{policyID}", s.updateAccessPolicy)
			protected.Post("/verify-credential", s.verifyCredential)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/gateways/{gatewayID}/access-rules", s.previewGatewayAccessRules)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Get("/organization/settings", s.getOrganizationSettings)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/organization/settings", s.updateOrganizationSettings)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/organization/export-audit", s.exportOrganizationAudit)
			protected.With(s.requireRoles("super_admin")).Post("/organization/rotate-webhooks", s.rotateOrganizationWebhooks)
			protected.With(s.requireRoles("super_admin")).Post("/organization/disable", s.disableOrganization)
			protected.With(s.requireRoles("super_admin")).Post("/organization/transfer", s.transferOrganization)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/invitations", s.listInvitations)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/invitations/{deliveryID}", s.getInvitation)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/invitations/{deliveryID}/cancel", s.cancelInvitation)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/invitations/{deliveryID}/resend", s.resendInvitation)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/users", s.listUsers)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/users", s.createUser)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/users/{userID}", s.getUser)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/users/{userID}/invite", s.inviteUser)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/users/{userID}/invitations", s.listUserInvitations)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/users/{userID}/invitations/{deliveryID}/receipt", s.recordUserInvitationReceipt)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/users/{userID}", s.updateUser)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/users/{userID}", s.deleteUser)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/users/batch-status", s.batchUpdateUserStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/users/batch-delete", s.batchDeleteUsers)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/users/batch-invite", s.batchInviteUsers)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/users/export-csv", s.exportUsersCSV)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/users/import-csv", s.importUsersCSV)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/user-groups", s.listUserGroups)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/user-groups", s.createUserGroup)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/user-groups/{groupID}", s.updateUserGroup)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin"), withDeprecatedEndpoint("/api/v1/shares")).Get("/temporary-access", s.listTemporaryAccess)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin"), withDeprecatedEndpoint("/api/v1/shares")).Post("/temporary-access", s.createTemporaryAccess)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/visitor-passes", s.listVisitorPasses)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/visitor-passes", s.createVisitorPass)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/guests", s.listGuests)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/guests/{guestID}", s.getGuest)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/guests", s.createGuest)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/guests/{guestID}/status", s.updateGuestStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/guests/{guestID}", s.deleteGuest)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/elevators", s.listElevators)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/elevators", s.createElevator)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/elevators/{elevatorID}", s.getElevator)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/elevators/{elevatorID}", s.updateElevator)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/elevators/{elevatorID}", s.deleteElevator)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/elevator_stops", s.listElevatorStops)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/elevator_stops", s.createElevatorStop)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/elevator_stops/{elevatorStopID}", s.getElevatorStop)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/elevator_stops/{elevatorStopID}", s.updateElevatorStop)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/elevator_stops/{elevatorStopID}", s.deleteElevatorStop)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/elevator_stops/{elevatorStopID}/lock_down", s.lockDownElevatorStop)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/elevator_stops/{elevatorStopID}/cancel_lockdown", s.cancelElevatorStopLockdown)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/group_elevator_stops", s.listGroupElevatorStops)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/group_elevator_stops", s.createGroupElevatorStop)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/group_elevator_stops/{groupElevatorStopID}", s.deleteGroupElevatorStop)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/group_terminals", s.listGroupTerminals)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/group_terminals", s.createGroupTerminal)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/group_terminals/{groupTerminalID}", s.deleteGroupTerminal)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/presences", s.listPresences)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/csv_card_imports", s.listCSVCardImports)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/csv_card_imports", s.createCSVCardImport)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/csv_card_imports/{importID}", s.getCSVCardImport)
			protected.Post("/users/password", s.changeUserPassword)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/groups", s.listReferenceGroups)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/groups", s.createReferenceGroup)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/groups/{groupID}", s.getReferenceGroup)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/groups/{groupID}", s.updateReferenceGroup)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/groups/{groupID}", s.deleteReferenceGroup)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/group_locks", s.listReferenceGroupLocks)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/group_locks", s.createReferenceGroupLock)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/group_locks/{groupLockID}", s.deleteReferenceGroupLock)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/group_zones", s.listReferenceGroupZones)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/group_zones", s.createReferenceGroupZone)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/group_zones/{groupZoneID}", s.getReferenceGroupZone)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/group_zones/{groupZoneID}", s.deleteReferenceGroupZone)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/group_links", s.listReferenceGroupLinks)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/group_links", s.createReferenceGroupLink)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/group_links/{groupLinkID}", s.getReferenceGroupLink)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/group_links/{groupLinkID}", s.updateReferenceGroupLink)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/group_links/{groupLinkID}", s.deleteReferenceGroupLink)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/controllers", s.listReferenceControllers)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/controllers/{controllerToken}/assign", s.assignReferenceController)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/controllers/{controllerID}/deassign", s.deassignReferenceController)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/controllers/{controllerID}/locks", s.bindReferenceControllerLock)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/controllers/{controllerID}/locks/{lockID}", s.unbindReferenceControllerLock)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/controllers/{controllerID}/config/publish", s.publishReferenceControllerConfig)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/controllers/{controllerID}/reboot", s.rebootReferenceController)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/controllers/{controllerID}", s.getReferenceController)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/controllers/{controllerID}", s.updateReferenceController)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/readers", s.listReferenceReaders)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/readers/{readerToken}/assign", s.assignReferenceReader)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/readers/{readerID}/deassign", s.deassignReferenceReader)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/readers/{readerID}/reboot", s.rebootReferenceReader)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/readers/{readerID}", s.getReferenceReader)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/readers/{readerID}", s.updateReferenceReader)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/readers/{readerID}/reset_tamper", s.resetTamperReferenceReader)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/terminals", s.listReferenceTerminals)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/terminals/{terminalID}", s.getReferenceTerminal)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/terminals/{terminalID}/reboot", s.rebootReferenceTerminal)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/terminals/{terminalID}/trigger", s.triggerReferenceTerminal)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/terminals", s.createReferenceTerminal)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Put("/terminals/{terminalID}", s.updateReferenceTerminal)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/terminals/{terminalID}", s.deleteReferenceTerminal)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/alert_policies", s.listReferenceAlertPolicies)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Post("/alert_policies/condition_preview", s.previewReferenceAlertPolicyCondition)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Post("/alert_policies/evaluate", s.evaluateReferenceAlertPoliciesForEvent)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/alert_policies/notifications", s.listAlertNotificationsHandler)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/alert_policies", s.createReferenceAlertPolicy)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/alert_policies/{policyID}", s.getReferenceAlertPolicy)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/alert_policies/{policyID}", s.updateReferenceAlertPolicy)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/alert_policies/{policyID}", s.deleteReferenceAlertPolicy)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/reports", s.listReferenceReports)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/reports/{reportID}", s.getReferenceReport)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/reports/{reportID}/download", s.downloadReferenceReport)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/reports", s.createReferenceReport)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/reports/{reportID}", s.deleteReferenceReport)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/scheduled_reports", s.listReferenceScheduledReports)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/scheduled_reports", s.createReferenceScheduledReport)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/scheduled_reports/{scheduledReportID}", s.getReferenceScheduledReport)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/scheduled_reports/{scheduledReportID}", s.updateReferenceScheduledReport)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/scheduled_reports/{scheduledReportID}", s.deleteReferenceScheduledReport)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/teams", s.listReferenceTeams)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/teams/{teamID}", s.getReferenceTeam)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/teams", s.createReferenceTeam)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/teams/{teamID}", s.updateReferenceTeam)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/teams/{teamID}", s.deleteReferenceTeam)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/team_memberships", s.listReferenceTeamMemberships)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/team_memberships", s.createReferenceTeamMembership)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/team_memberships/{membershipID}", s.deleteReferenceTeamMembership)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/roles", s.listReferenceRoles)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/roles/{roleID}", s.getReferenceRole)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/role_assignments", s.listReferenceRoleAssignments)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/role_assignments", s.createReferenceRoleAssignment)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/role_assignments/{assignmentID}", s.getReferenceRoleAssignment)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/role_assignments/{assignmentID}", s.updateReferenceRoleAssignment)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/role_assignments/{assignmentID}", s.deleteReferenceRoleAssignment)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/members", s.listReferenceMembers)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/members", s.createReferenceMember)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/members/{memberID}", s.getReferenceMembers)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/members/{memberID}", s.updateReferenceMember)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/members/{memberID}", s.deleteReferenceMember)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/schedules", s.listSchedules)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/schedules", s.createSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/schedules/{scheduleID}", s.getSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/schedules/{scheduleID}", s.updateSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/schedules/{scheduleID}", s.deleteSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/access_rights/schedule_templates", s.listReferenceAccessRightsScheduleTemplates)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/access_rights/schedule", s.updateReferenceAccessRightsSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Post("/access_rights/impact_preview", s.previewReferenceAccessRightsImpact)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/access_rights/review", s.reviewReferenceAccessRights)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Post("/access_rights/schedule/evaluate", s.evaluateReferenceAccessRightsSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/holiday_calendars", s.listHolidayCalendars)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/holiday_calendars", s.createHolidayCalendar)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/holiday_calendars/preset_countries", s.listHolidayCalendarPresetCountries)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/holiday_calendars/presets", s.listHolidayCalendarPresets)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/holiday_calendars/{calendarID}", s.getHolidayCalendar)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/holiday_calendars/{calendarID}", s.updateHolidayCalendar)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/holiday_calendars/{calendarID}", s.deleteHolidayCalendar)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/shares", s.listReferenceShares)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/shares", s.createReferenceShare)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/shares/{shareID}", s.getReferenceShare)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/shares/{shareID}", s.updateReferenceShare)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/shares/{shareID}", s.deleteReferenceShare)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/cards", s.listReferenceCards)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/cards/{cardID}", s.getReferenceCard)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/cards", s.createReferenceCard)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/card_assignments", s.listReferenceCardAssignments)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/card_assignments/{assignmentID}", s.getReferenceCardAssignment)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/card_assignments", s.createReferenceCardAssignment)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/cards/{cardID}/assign", s.assignReferenceCard)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/cards/{cardID}/deassign", s.deassignReferenceCard)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/cards/{cardID}/activate", s.activateReferenceCard)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/cards/{cardID}/deactivate", s.deactivateReferenceCard)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/cards/{cardID}/revoke", s.revokeReferenceCard)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/cards/{cardID}", s.updateReferenceCard)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/cards/{cardID}", s.deleteReferenceCard)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/card_assignments/{assignmentID}", s.updateReferenceCardAssignment)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/card_assignments/{assignmentID}", s.deleteReferenceCardAssignment)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/card_assignments/{assignmentID}/activate", s.activateReferenceCardAssignment)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/card_assignments/{assignmentID}/deactivate", s.deactivateReferenceCardAssignment)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Post("/event_sets", s.createReferenceEventSet)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/event_sets/{eventSetID}", s.getReferenceEventSet)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/events/meta", s.getReferenceEventMetadata)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/events/types", s.listReferenceEventTypes)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/integrations", s.listReferenceIntegrations)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/integrations", s.createReferenceIntegration)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/integrations/{integrationID}", s.getReferenceIntegration)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/integrations/{integrationID}", s.updateReferenceIntegration)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/integrations/{integrationID}", s.deleteReferenceIntegration)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin"), withDeprecatedEndpoint("/api/v1/event_sets")).Get("/events/access", s.listAccessEvents)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/events/device", s.listDeviceEvents)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/events/stream", s.streamEvents)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/alarms", s.listAlarms)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/alarms/stream", s.streamAlarms)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Patch("/alarms/{alarmID}/status", s.updateAlarmStatus)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Post("/alarm-schedules", s.createAlarmSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/alarm-schedules", s.listAlarmSchedules)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/alarm-schedules/calendar", s.getAlarmScheduleCalendar)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/alarm-schedules/{scheduleID}", s.getAlarmSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Patch("/alarm-schedules/{scheduleID}", s.updateAlarmSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Delete("/alarm-schedules/{scheduleID}", s.deleteAlarmSchedule)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/cameras", s.listCameras)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Post("/cameras", s.createCamera)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/cameras/{cameraID}", s.getCamera)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Delete("/cameras/{cameraID}", s.deleteCamera)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/analytics/access-summary", s.getAccessSummary)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/analytics/door-activity", s.getDoorActivity)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/analytics/alarm-metrics", s.getAlarmMetrics)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/audit-logs", s.listAuditLogs)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Get("/audit/webhook/config", s.getAuditWebhookConfig)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Put("/audit/webhook/config", s.upsertAuditWebhookConfig)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/audit/webhook/deliveries", s.listAuditWebhookDeliveries)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/audit/webhook/dispatch", s.dispatchAuditWebhook)
			protected.With(s.requireRoles("super_admin")).Get("/state/change-log", s.listStateChangeLog)
			protected.With(s.requireRoles("super_admin")).Post("/state/change-log/replay", s.replayStateChangeLog)
			protected.With(s.requireRoles("super_admin")).Get("/state/change-log/checkpoints", s.listStateChangeLogCheckpoints)
			protected.With(s.requireRoles("super_admin")).Post("/state/change-log/replay/checkpoint", s.replayStateChangeLogFromCheckpoint)

			protected.With(s.requireRoles("super_admin", "tenant_admin")).Get("/wallet/google/config", s.getWalletGoogleConfig)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Put("/wallet/google/config", s.upsertWalletGoogleConfig)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/google/config/validate", s.validateWalletGoogleConfig)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/templates", s.listWalletTemplates)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/templates", s.createWalletTemplate)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/wallet/templates/{templateID}/status", s.updateWalletTemplateStatus)

			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/passes/issue", s.issueWalletPass)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/passes/issue-batch", s.issueWalletPassBatch)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator"), withDeprecatedEndpoint("/api/v1/cards", "/api/v1/card_assignments")).Get("/wallet/passes", s.listWalletPasses)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/passes/{passID}", s.getWalletPass)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/passes/{passID}/save-link", s.getWalletPassSaveLink)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/wallet/passes/{passID}/suspend", s.suspendWalletPass)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/wallet/passes/{passID}/activate", s.activateWalletPass)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/wallet/passes/{passID}/revoke", s.revokeWalletPass)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/deliveries", s.listWalletPassDeliveries)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/deliveries/dispatch", s.dispatchWalletPassDelivery)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/deliveries/{notificationID}/retry", s.retryWalletPassDelivery)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/physical-card-vendors", s.listWalletPhysicalCardVendors)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/physical-card-inventory", s.listWalletPhysicalCardInventory)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/physical-card-inventory", s.createWalletPhysicalCardInventoryItem)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/physical-card-inventory/scan", s.scanWalletPhysicalCardInventory)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/physical-card-inventory/import", s.importWalletPhysicalCardInventory)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/physical-card-inventory/import-csv", s.importWalletPhysicalCardInventoryCSV)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/wallet/physical-card-inventory/batch-status", s.batchUpdateWalletPhysicalCardInventoryStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/wallet/physical-card-inventory/{inventoryID}/status", s.updateWalletPhysicalCardInventoryStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/physical-card-tasks", s.listWalletPhysicalCardTasks)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/physical-card-tasks", s.createWalletPhysicalCardTask)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/wallet/physical-card-tasks/{taskID}/status", s.updateWalletPhysicalCardTaskStatus)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/jobs", s.listWalletJobs)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/jobs/summary", s.listWalletJobSummary)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/jobs/metrics/trend", s.listWalletJobMetricsTrend)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/jobs/metrics", s.listWalletJobMetrics)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/jobs/alert-subscription", s.getWalletJobAlertSubscription)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/jobs/alert-notifications", s.listWalletJobAlertNotifications)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/jobs/alert-notifications/{notificationID}/retry", s.retryWalletJobAlertNotification)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/jobs/alerts/dispatch", s.dispatchWalletJobAlerts)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Put("/wallet/jobs/alert-subscription", s.upsertWalletJobAlertSubscription)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/jobs/dlq/cleanup/archives", s.listWalletDLQCleanupArchives)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/jobs/dlq/requeue", s.requeueWalletDLQJobs)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/jobs/dlq/cleanup", s.cleanupWalletDLQJobs)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/jobs/{jobID}", s.getWalletJob)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/jobs/{jobID}/retry", s.retryWalletJob)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/jobs/{jobID}/dlq/requeue", s.requeueWalletDLQJob)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/jobs/process", s.processWalletJobs)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/audit-logs", s.listWalletAuditLogs)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/domain-mappings", s.listEnterpriseDomainMappings)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/domain-mappings", s.createEnterpriseDomainMapping)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/enterprise/domain-mappings/{mappingID}/status", s.updateEnterpriseDomainMappingStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/hris-connectors", s.listEnterpriseHRISConnectors)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/hris-connectors", s.createEnterpriseHRISConnector)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/enterprise/hris-connectors/{connectorID}", s.updateEnterpriseHRISConnector)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/hris-secrets", s.listEnterpriseHRISSecrets)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/hris-webhook-receipts", s.listEnterpriseHRISWebhookReceipts)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/hris-webhook-receipts/{receiptID}/process", s.processEnterpriseHRISWebhookReceiptEntry)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/hris-webhook-receipts/process-batch", s.processBatchEnterpriseHRISWebhookReceipts)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/hris-pull-states", s.listEnterpriseHRISPullStates)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Put("/enterprise/hris-secrets", s.upsertEnterpriseHRISSecret)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/hris-webhook-dlq", s.listEnterpriseHRISWebhookDLQ)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/hris-webhook-dlq/replay-batch", s.replayBatchEnterpriseHRISWebhookDLQ)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/hris-webhook-dlq/{entryID}/replay", s.replayEnterpriseHRISWebhookDLQ)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/hris-webhook-executions", s.listEnterpriseHRISWebhookExecutions)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/hris-webhook-executions/{executionID}", s.getEnterpriseHRISWebhookExecution)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/hris-webhook-executions/{executionID}/replay", s.replayEnterpriseHRISWebhookExecution)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/idp-config", s.getEnterpriseIDPConfig)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Put("/enterprise/idp-config", s.upsertEnterpriseIDPConfig)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/idp-config/validate", s.validateEnterpriseIDPConfig)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/employees", s.listEnterpriseEmployees)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/jit-provision-approvals", s.listEnterpriseJITProvisionApprovals)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/jit-provision-approvals/external-sync-pending", s.listEnterpriseJITProvisionApprovalExternalSyncPending)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/jit-provision-approvals/{approvalID}/review", s.reviewEnterpriseJITProvisionApproval)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/jit-provision-approvals/{approvalID}/external-sync", s.updateEnterpriseJITProvisionApprovalExternalSync)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/employees/sync", s.syncEnterpriseEmployees)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/employees/sync/reconcile", s.reconcileEnterpriseEmployeeSync)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/sync-requests", s.listEnterpriseSyncRequests)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/sync-worker-alert-subscription", s.getEnterpriseSyncWorkerAlertSubscription)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/sync-worker-alerts", s.listEnterpriseSyncWorkerAlerts)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/sync-worker-alerts/summary", s.listEnterpriseSyncWorkerAlertSummary)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/sync-worker-alerts/notifications/export-csv", s.exportEnterpriseSyncWorkerAlertNotificationsCSV)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/sync-worker-alerts/notifications", s.listEnterpriseSyncWorkerAlertNotifications)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/sync-worker-alerts/notifications/auto-retry", s.autoRetryEnterpriseSyncWorkerAlertNotifications)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/sync-worker-alerts/notifications/restore-batch", s.restoreEnterpriseSyncWorkerAlertNotificationsBatch)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/sync-worker-alerts/dispatch", s.dispatchEnterpriseSyncWorkerAlerts)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/sync-worker-alerts/notifications/retry-batch", s.retryEnterpriseSyncWorkerAlertNotificationsBatch)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/sync-worker-alerts/notifications/suppress-batch", s.suppressEnterpriseSyncWorkerAlertNotificationsBatch)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/sync-worker-alerts/notifications/{notificationID}/retry", s.retryEnterpriseSyncWorkerAlertNotification)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Put("/enterprise/sync-worker-alert-subscription", s.upsertEnterpriseSyncWorkerAlertSubscription)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/sync-requests/reconcile-pending", s.reconcilePendingEnterpriseSyncRequests)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/sync-jobs", s.listEnterpriseSyncJobs)
		})
	})

	s.startEnterpriseSyncReconcileWorker()
	s.startEnterpriseSyncWorkerAlertAutoRetryWorker()
	s.startEnterpriseJITApprovalExternalSyncWorker()
	s.startEnterpriseHRISWebhookReceiptWorker()
	s.startEnterpriseHRISWebhookDLQWorker()
	s.startEnterpriseHRISPullWorker()
	s.startAlertPolicyEventScheduler()

	return router, s, nil
}

type gatewayBootstrapStateSnapshot struct {
	DeviceTokens map[string]string `json:"device_tokens,omitempty"`
}

func (s *server) loggerOrDefault() *slog.Logger {
	if s != nil && s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

func notifyWorkerWake(ch chan struct{}) bool {
	if ch == nil {
		return false
	}
	select {
	case ch <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *server) notifyEnterpriseHRISWebhookReceiptWorker() bool {
	if s == nil {
		return false
	}
	return notifyWorkerWake(s.hrisWebhookReceiptWorkerWake)
}

func (s *server) notifyEnterpriseHRISWebhookDLQWorker() bool {
	if s == nil {
		return false
	}
	return notifyWorkerWake(s.hrisWebhookDLQWorkerWake)
}

func (s *server) enqueueEnterpriseHRISWebhookReceiptQueuedTask(task enterpriseHRISWebhookReceiptQueuedTask) bool {
	if s == nil || !s.cfg.EnterpriseHRISWebhookReceiptWorkerEnabled || s.hrisWebhookReceiptWorkerQueue == nil {
		return false
	}
	select {
	case s.hrisWebhookReceiptWorkerQueue <- task:
		return true
	default:
		return false
	}
}

func (s *server) enqueueEnterpriseHRISWebhookDLQQueuedTask(task enterpriseHRISWebhookDLQQueuedTask) bool {
	if s == nil || !s.cfg.EnterpriseHRISWebhookDLQWorkerEnabled || s.hrisWebhookDLQWorkerQueue == nil {
		return false
	}
	select {
	case s.hrisWebhookDLQWorkerQueue <- task:
		return true
	default:
		return false
	}
}

func (s *server) restoreGatewayBootstrapState() error {
	if s.stateStore == nil {
		return nil
	}
	var snapshot gatewayBootstrapStateSnapshot
	found, err := s.stateStore.Load(gatewayBootstrapStateKey, &snapshot)
	if err != nil {
		return err
	}
	if s.gatewayTokenStore != nil {
		for gatewayID, token := range snapshot.DeviceTokens {
			nextGatewayID := strings.TrimSpace(gatewayID)
			nextToken := strings.TrimSpace(token)
			if nextGatewayID == "" || nextToken == "" {
				continue
			}
			if upsertErr := s.gatewayTokenStore.UpsertGatewayDeviceToken(nextGatewayID, nextToken); upsertErr != nil {
				return upsertErr
			}
		}
		return nil
	}

	s.gatewayTokenMu.Lock()
	defer s.gatewayTokenMu.Unlock()

	if !found {
		return s.persistGatewayBootstrapStateLocked()
	}
	for gatewayID, token := range snapshot.DeviceTokens {
		nextGatewayID := strings.TrimSpace(gatewayID)
		nextToken := strings.TrimSpace(token)
		if nextGatewayID == "" || nextToken == "" {
			continue
		}
		s.gatewayDeviceTokens[nextGatewayID] = nextToken
	}
	return nil
}

func (s *server) persistGatewayBootstrapStateLocked() error {
	if s.stateStore == nil {
		return nil
	}
	if s.gatewayTokenStore != nil {
		return nil
	}
	return s.stateStore.Save(gatewayBootstrapStateKey, gatewayBootstrapStateSnapshot{
		DeviceTokens: cloneGatewayDeviceTokenMap(s.gatewayDeviceTokens),
	})
}

func cloneGatewayDeviceTokenMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for gatewayID, token := range input {
		nextGatewayID := strings.TrimSpace(gatewayID)
		nextToken := strings.TrimSpace(token)
		if nextGatewayID == "" || nextToken == "" {
			continue
		}
		output[nextGatewayID] = nextToken
	}
	return output
}

func (s *server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := allowedCORSOrigin(s.cfg.CORSOrigin, r.Header.Get("Origin")); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if origin != "*" {
				w.Header().Add("Vary", "Origin")
			}
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Expose-Headers", "X-Collection-Range, Deprecation, Link, X-MistyPass-Replacement")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func allowedCORSOrigin(configuredOrigin, requestOrigin string) string {
	configuredOrigin = strings.TrimSpace(configuredOrigin)
	if configuredOrigin == "" {
		return ""
	}
	requestOrigin = strings.TrimSpace(requestOrigin)
	fallbackOrigin := ""
	for _, candidate := range strings.Split(configuredOrigin, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if fallbackOrigin == "" {
			fallbackOrigin = candidate
		}
		if candidate == "*" {
			return candidate
		}
		if requestOrigin != "" && candidate == requestOrigin {
			return requestOrigin
		}
	}
	if requestOrigin == "" {
		return fallbackOrigin
	}
	return ""
}

func (s *server) withLoginRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := requestClientIP(r)
		if clientIP == "" {
			clientIP = "unknown"
		}

		allowed, retryAfter := s.allowLoginAttempt(clientIP, time.Now().UTC())
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, "too many login attempts, retry later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *server) withGlobalAPIRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := requestClientIP(r)
		if clientIP == "" {
			clientIP = "unknown"
		}

		allowed, retryAfter := s.allowGlobalAPIAttempt(clientIP, time.Now().UTC())
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, "too many api requests, retry later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *server) withEnterprisePublicRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := requestClientIP(r)
		if clientIP == "" {
			clientIP = "unknown"
		}

		allowed, retryAfter := s.allowEnterprisePublicAttempt(clientIP, time.Now().UTC())
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, "too many enterprise public requests, retry later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *server) withEnterpriseWebhookRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := requestClientIP(r)
		if clientIP == "" {
			clientIP = "unknown"
		}

		allowed, retryAfter := s.allowEnterpriseWebhookAttempt(clientIP, time.Now().UTC())
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, "too many enterprise webhook requests, retry later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func requestClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		first := strings.TrimSpace(parts[0])
		if ip := net.ParseIP(first); ip != nil {
			return ip.String()
		}
	}

	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return ip.String()
		}
	}

	remote := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remote); err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
	}
	if ip := net.ParseIP(remote); ip != nil {
		return ip.String()
	}

	return ""
}

func (s *server) allowLoginAttempt(key string, now time.Time) (bool, time.Duration) {
	if s.rateLimitStore != nil {
		allowed, retryAfter, err := s.rateLimitStore.AllowRateLimit(
			"login",
			key,
			now,
			loginRateLimitWindow,
			loginRateLimitMaxAttempts,
		)
		if err == nil {
			return allowed, retryAfter
		}
		s.warnRateLimitFallback("login", key, err)
	}

	s.loginRateLimitMu.Lock()
	defer s.loginRateLimitMu.Unlock()

	s.compactLoginRateBuckets(now)
	bucket, found := s.loginRateLimitBuckets[key]
	if !found || now.Sub(bucket.WindowStart) >= loginRateLimitWindow {
		s.loginRateLimitBuckets[key] = loginRateLimitBucket{
			WindowStart: now,
			Attempts:    1,
		}
		return true, 0
	}

	if bucket.Attempts >= loginRateLimitMaxAttempts {
		retryAfter := loginRateLimitWindow - now.Sub(bucket.WindowStart)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	bucket.Attempts++
	s.loginRateLimitBuckets[key] = bucket
	return true, 0
}

func (s *server) compactLoginRateBuckets(now time.Time) {
	if len(s.loginRateLimitBuckets) == 0 {
		return
	}

	for key, bucket := range s.loginRateLimitBuckets {
		if now.Sub(bucket.WindowStart) > loginRateLimitBucketTTL {
			delete(s.loginRateLimitBuckets, key)
		}
	}

	if len(s.loginRateLimitBuckets) <= loginRateLimitBucketMaxKeys {
		return
	}

	for key := range s.loginRateLimitBuckets {
		delete(s.loginRateLimitBuckets, key)
		if len(s.loginRateLimitBuckets) <= loginRateLimitBucketMaxKeys/2 {
			break
		}
	}
}

func (s *server) allowGlobalAPIAttempt(key string, now time.Time) (bool, time.Duration) {
	if s.rateLimitStore != nil {
		allowed, retryAfter, err := s.rateLimitStore.AllowRateLimit(
			"global_api",
			key,
			now,
			apiRateLimitWindow,
			apiRateLimitMaxRequests,
		)
		if err == nil {
			return allowed, retryAfter
		}
		s.warnRateLimitFallback("global_api", key, err)
	}

	s.apiRateLimitMu.Lock()
	defer s.apiRateLimitMu.Unlock()

	s.compactGlobalAPIRateBuckets(now)
	bucket, found := s.apiRateLimitBuckets[key]
	if !found || now.Sub(bucket.WindowStart) >= apiRateLimitWindow {
		s.apiRateLimitBuckets[key] = loginRateLimitBucket{
			WindowStart: now,
			Attempts:    1,
		}
		return true, 0
	}

	if bucket.Attempts >= apiRateLimitMaxRequests {
		retryAfter := apiRateLimitWindow - now.Sub(bucket.WindowStart)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	bucket.Attempts++
	s.apiRateLimitBuckets[key] = bucket
	return true, 0
}

func (s *server) allowEnterprisePublicAttempt(key string, now time.Time) (bool, time.Duration) {
	if s.rateLimitStore != nil {
		allowed, retryAfter, err := s.rateLimitStore.AllowRateLimit(
			"enterprise_public",
			key,
			now,
			enterprisePublicRateLimitWindow,
			enterprisePublicRateLimitMaxRequests,
		)
		if err == nil {
			return allowed, retryAfter
		}
		s.warnRateLimitFallback("enterprise_public", key, err)
	}

	s.enterprisePublicRateLimitMu.Lock()
	defer s.enterprisePublicRateLimitMu.Unlock()

	s.compactEnterprisePublicRateBuckets(now)
	bucket, found := s.enterprisePublicRateBuckets[key]
	if !found || now.Sub(bucket.WindowStart) >= enterprisePublicRateLimitWindow {
		s.enterprisePublicRateBuckets[key] = loginRateLimitBucket{
			WindowStart: now,
			Attempts:    1,
		}
		return true, 0
	}

	if bucket.Attempts >= enterprisePublicRateLimitMaxRequests {
		retryAfter := enterprisePublicRateLimitWindow - now.Sub(bucket.WindowStart)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	bucket.Attempts++
	s.enterprisePublicRateBuckets[key] = bucket
	return true, 0
}

func (s *server) allowEnterpriseWebhookAttempt(key string, now time.Time) (bool, time.Duration) {
	if s.rateLimitStore != nil {
		allowed, retryAfter, err := s.rateLimitStore.AllowRateLimit(
			"enterprise_webhook",
			key,
			now,
			enterpriseWebhookRateLimitWindow,
			enterpriseWebhookRateLimitMaxRequests,
		)
		if err == nil {
			return allowed, retryAfter
		}
		s.warnRateLimitFallback("enterprise_webhook", key, err)
	}

	s.enterpriseWebhookRateLimitMu.Lock()
	defer s.enterpriseWebhookRateLimitMu.Unlock()

	s.compactEnterpriseWebhookRateBuckets(now)
	bucket, found := s.enterpriseWebhookRateBuckets[key]
	if !found || now.Sub(bucket.WindowStart) >= enterpriseWebhookRateLimitWindow {
		s.enterpriseWebhookRateBuckets[key] = loginRateLimitBucket{
			WindowStart: now,
			Attempts:    1,
		}
		return true, 0
	}

	if bucket.Attempts >= enterpriseWebhookRateLimitMaxRequests {
		retryAfter := enterpriseWebhookRateLimitWindow - now.Sub(bucket.WindowStart)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}

	bucket.Attempts++
	s.enterpriseWebhookRateBuckets[key] = bucket
	return true, 0
}

func (s *server) compactGlobalAPIRateBuckets(now time.Time) {
	if len(s.apiRateLimitBuckets) == 0 {
		return
	}

	for key, bucket := range s.apiRateLimitBuckets {
		if now.Sub(bucket.WindowStart) > apiRateLimitBucketTTL {
			delete(s.apiRateLimitBuckets, key)
		}
	}

	if len(s.apiRateLimitBuckets) <= apiRateLimitBucketMaxKeys {
		return
	}

	for key := range s.apiRateLimitBuckets {
		delete(s.apiRateLimitBuckets, key)
		if len(s.apiRateLimitBuckets) <= apiRateLimitBucketMaxKeys/2 {
			break
		}
	}
}

func (s *server) compactEnterprisePublicRateBuckets(now time.Time) {
	if len(s.enterprisePublicRateBuckets) == 0 {
		return
	}

	for key, bucket := range s.enterprisePublicRateBuckets {
		if now.Sub(bucket.WindowStart) > enterprisePublicRateLimitBucketTTL {
			delete(s.enterprisePublicRateBuckets, key)
		}
	}

	if len(s.enterprisePublicRateBuckets) <= enterprisePublicRateLimitBucketMaxKeys {
		return
	}

	for key := range s.enterprisePublicRateBuckets {
		delete(s.enterprisePublicRateBuckets, key)
		if len(s.enterprisePublicRateBuckets) <= enterprisePublicRateLimitBucketMaxKeys/2 {
			break
		}
	}
}

func (s *server) compactEnterpriseWebhookRateBuckets(now time.Time) {
	if len(s.enterpriseWebhookRateBuckets) == 0 {
		return
	}

	for key, bucket := range s.enterpriseWebhookRateBuckets {
		if now.Sub(bucket.WindowStart) > enterpriseWebhookRateLimitBucketTTL {
			delete(s.enterpriseWebhookRateBuckets, key)
		}
	}

	if len(s.enterpriseWebhookRateBuckets) <= enterpriseWebhookRateLimitBucketMaxKeys {
		return
	}

	for key := range s.enterpriseWebhookRateBuckets {
		delete(s.enterpriseWebhookRateBuckets, key)
		if len(s.enterpriseWebhookRateBuckets) <= enterpriseWebhookRateLimitBucketMaxKeys/2 {
			break
		}
	}
}

func (s *server) warnRateLimitFallback(scope, key string, err error) {
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn("redis rate limit fallback to in-memory buckets", "scope", scope, "key", key, "err", err)
}

func (s *server) withBearerToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		user, err := s.authService.VerifyAccessToken(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid access token")
			return
		}

		ctx := context.WithValue(r.Context(), authUserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *server) appMe(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *server) appCredentials(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	passes := s.walletSvc.ListPasses(user.TenantID)
	items := make([]map[string]any, 0, len(passes))
	for i := range passes {
		if passes[i].TargetType != "user" {
			continue
		}
		if passes[i].TargetID != user.ID {
			continue
		}

		items = append(items, map[string]any{
			"id":              passes[i].ID,
			"type":            firstNonEmptyString(passes[i].CredentialKind, "wallet"),
			"provider":        passes[i].Provider,
			"credential_kind": passes[i].CredentialKind,
			"status":          passes[i].Status,
			"save_link":       passes[i].SaveLink,
			"issued_at":       passes[i].IssuedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) appEnrollApplePass(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var request struct {
		DeviceID   string `json:"device_id"`
		PassSerial string `json:"pass_serial"`
		ExpiresAt  string `json:"expires_at"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	enrolled, err := s.walletSvc.EnrollApplePass(
		user.TenantID,
		user.ID,
		request.DeviceID,
		request.PassSerial,
		request.ExpiresAt,
		firstNonEmptyString(user.Email, user.ID, "resident"),
	)
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrTargetIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"pass":              enrolled,
		"credential_kind":   "apple_wallet",
		"enrollment_source": "self_service",
	})
}

func (s *server) appAccessDoors(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	doors := s.spaceSvc.ListDoors(user.TenantID)
	items := make([]map[string]any, 0, len(doors))
	for i := range doors {
		items = append(items, map[string]any{
			"id":          doors[i].ID,
			"name":        doors[i].Name,
			"building_id": doors[i].BuildingID,
			"area_id":     doors[i].AreaID,
			"status":      doors[i].Status,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) appAccessBLEToken(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	token, err := randomHexID(16)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ble_token":  "ble_" + token,
		"tenant_id":  user.TenantID,
		"issued_at":  time.Now().UTC(),
		"expires_in": 300,
		"token_type": "bearer",
		"user_id":    user.ID,
	})
}

func (s *server) appAccessLogs(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	events := s.eventSvc.ListAccessEvents(user.TenantID)
	writeJSON(w, http.StatusOK, map[string]any{
		"items": events,
	})
}

func (s *server) appCreateVisitorPass(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	var request struct {
		BuildingID     string `json:"building_id"`
		Visitor        string `json:"visitor"`
		DeliveryMethod string `json:"delivery_method"`
		ExpiresAt      string `json:"expires_at"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := s.accessSvc.CreateVisitorPass(
		user.TenantID,
		request.BuildingID,
		user.Email,
		request.Visitor,
		request.DeliveryMethod,
		request.ExpiresAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrTenantIDRequired),
			errors.Is(err, access.ErrHostRequired),
			errors.Is(err, access.ErrVisitorRequired),
			errors.Is(err, access.ErrDeliveryMethodInvalid),
			errors.Is(err, access.ErrExpiresAtRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (s *server) gatewayBootstrapConfigPull(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID      string `json:"gateway_id"`
		TenantID       string `json:"tenant_id"`
		CurrentVersion string `json:"current_version"`
		AuthzVersion   string `json:"authz_cache_version"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, ok := s.findGatewayByTenant(request.TenantID, request.GatewayID)
	if !ok {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.authorizeGatewayDeviceToken(w, r, record.ID) {
		return
	}
	if !s.validateGatewayRequestNonce(w, r) {
		return
	}

	snapshot, err := s.gatewaySvc.PullConfig(request.TenantID, request.GatewayID)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	currentVersion := strings.TrimSpace(request.CurrentVersion)
	desiredVersion := strings.TrimSpace(snapshot.DesiredVersion)
	fetchedAt := time.Now().UTC()
	authzCache := s.buildGatewayConfigAuthzCache(request.TenantID, snapshot.GatewayID, snapshot.BoundDoorIDs, fetchedAt)
	reportedAuthzVersion := strings.TrimSpace(request.AuthzVersion)
	authzCache.VersionReported = reportedAuthzVersion
	authzCache.Status = gatewayConfigAuthzResolveStatus(
		authzCache.StatusCodes,
		reportedAuthzVersion,
		authzCache.Version,
		authzCache.Policy.RollbackVersion,
	)

	// Include pending OTA tasks so the gateway can discover firmware updates.
	var pendingOTA []gateway.GatewayOTATask
	if allOTA, otaErr := s.gatewaySvc.ListOTATasks(request.TenantID, request.GatewayID); otaErr == nil {
		for _, task := range allOTA {
			if task.Status == "queued" || task.Status == "dispatching" {
				pendingOTA = append(pendingOTA, task)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id":        snapshot.GatewayID,
		"tenant_id":         snapshot.TenantID,
		"current_version":   currentVersion,
		"desired_version":   desiredVersion,
		"should_apply":      desiredVersion != "" && desiredVersion != currentVersion,
		"desired_at":        snapshot.DesiredUpdatedAt,
		"applied_version":   snapshot.AppliedVersion,
		"applied_at":        snapshot.AppliedAt,
		"bound_door_ids":    snapshot.BoundDoorIDs,
		"devices":           snapshot.Devices,
		"authz_cache":       authzCache,
		"pending_ota_tasks": pendingOTA,
		"fetched_at":        fetchedAt,
	})
}

func (s *server) gatewayBootstrapConfigApplied(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID         string `json:"gateway_id"`
		TenantID          string `json:"tenant_id"`
		Version           string `json:"version"`
		AuthzCacheVersion string `json:"authz_cache_version"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, ok := s.findGatewayByTenant(request.TenantID, request.GatewayID)
	if !ok {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.authorizeGatewayDeviceToken(w, r, record.ID) {
		return
	}

	snapshot, err := s.gatewaySvc.MarkConfigApplied(request.TenantID, request.GatewayID, request.Version)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired), errors.Is(err, gateway.ErrConfigVersionRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	ackedAt := time.Now().UTC()
	expectedAuthzCache := s.buildGatewayConfigAuthzCache(snapshot.TenantID, snapshot.GatewayID, snapshot.BoundDoorIDs, ackedAt)
	reportedAuthzCacheVersion := strings.TrimSpace(request.AuthzCacheVersion)
	authzCacheVersionMatch := reportedAuthzCacheVersion == "" || reportedAuthzCacheVersion == expectedAuthzCache.Version
	authzCacheStatus := gatewayConfigAuthzResolveStatus(
		expectedAuthzCache.StatusCodes,
		reportedAuthzCacheVersion,
		expectedAuthzCache.Version,
		expectedAuthzCache.Policy.RollbackVersion,
	)
	if reportedAuthzCacheVersion != "" && authzCacheVersionMatch {
		s.setGatewayAuthzCacheAckVersion(snapshot.GatewayID, reportedAuthzCacheVersion)
	}
	if reportedAuthzCacheVersion != "" && !authzCacheVersionMatch {
		s.appendAuditLog(
			r,
			snapshot.TenantID,
			"gateway_config_authz_cache_version_drift",
			fmt.Sprintf(
				"gateway=%s applied=%s reported=%s expected=%s",
				snapshot.GatewayID,
				strings.TrimSpace(snapshot.AppliedVersion),
				reportedAuthzCacheVersion,
				expectedAuthzCache.Version,
			),
			"gateway_config",
		)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id":      snapshot.GatewayID,
		"tenant_id":       snapshot.TenantID,
		"desired_version": snapshot.DesiredVersion,
		"applied_version": snapshot.AppliedVersion,
		"in_sync": strings.TrimSpace(snapshot.DesiredVersion) == "" ||
			strings.TrimSpace(snapshot.DesiredVersion) == strings.TrimSpace(snapshot.AppliedVersion),
		"desired_at": snapshot.DesiredUpdatedAt,
		"applied_at": snapshot.AppliedAt,
		"authz_cache": map[string]any{
			"version_reported": reportedAuthzCacheVersion,
			"version_expected": expectedAuthzCache.Version,
			"version_match":    authzCacheVersionMatch,
			"status":           authzCacheStatus,
			"generated_at":     expectedAuthzCache.GeneratedAt,
			"expires_at":       expectedAuthzCache.ExpiresAt,
			"ttl_seconds":      expectedAuthzCache.TTLSeconds,
			"policy":           expectedAuthzCache.Policy,
			"status_codes":     expectedAuthzCache.StatusCodes,
		},
		"acked_at": ackedAt,
	})
}

func (s *server) gatewayBootstrapAccessEvent(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID      string `json:"gateway_id"`
		TenantID       string `json:"tenant_id"`
		EventID        string `json:"event_id"`
		RequestID      string `json:"request_id"`
		BuildingID     string `json:"building_id"`
		AreaID         string `json:"area_id"`
		Type           string `json:"type"`
		Actor          string `json:"actor"`
		DoorID         string `json:"door_id"`
		Result         string `json:"result"`
		OccurredAt     string `json:"occurred_at"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, ok := s.findGatewayByTenant(request.TenantID, request.GatewayID)
	if !ok {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.authorizeGatewayDeviceToken(w, r, record.ID) {
		return
	}

	idempotencyKey := strings.TrimSpace(request.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(request.RequestID)
	}
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(request.EventID)
	}

	eventType := strings.TrimSpace(request.Type)
	if eventType == "" {
		eventType = "access_event"
	}
	result := strings.TrimSpace(request.Result)
	if result == "" {
		result = "accepted"
	}
	buildingID := strings.TrimSpace(request.BuildingID)
	if buildingID == "" {
		buildingID = strings.TrimSpace(record.BuildingID)
	}

	occurredAt := time.Now().UTC()
	if raw := strings.TrimSpace(request.OccurredAt); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "occurred_at must be RFC3339")
			return
		}
		occurredAt = parsed.UTC()
	}

	saved, deduped, err := s.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
		ID:             request.EventID,
		IdempotencyKey: idempotencyKey,
		TenantID:       record.TenantID,
		BuildingID:     buildingID,
		AreaID:         request.AreaID,
		Type:           eventType,
		Actor:          request.Actor,
		DoorID:         request.DoorID,
		GatewayID:      record.ID,
		Result:         result,
		At:             occurredAt,
	})
	if err != nil {
		switch {
		case errors.Is(err, event.ErrTenantIDRequired),
			errors.Is(err, event.ErrGatewayIDRequired),
			errors.Is(err, event.ErrAccessEventTypeRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	var queueProgress gateway.GatewayQueueIngestTotal
	if deduped {
		queueProgress, err = s.gatewaySvc.GetQueueIngestTotal(record.TenantID, record.ID, "default")
		if err != nil {
			if errors.Is(err, gateway.ErrGatewayQueueIngestTotalNotFound) {
				accessTotal, deviceTotal := s.eventSvc.CountEventsByGateway(record.TenantID, record.ID)
				queueProgress = gateway.GatewayQueueIngestTotal{
					GatewayID:     record.ID,
					Queue:         "default",
					IngestedTotal: accessTotal + deviceTotal,
					UpdatedAt:     time.Now().UTC(),
				}
			} else {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	} else {
		queueProgress, err = s.gatewaySvc.AddQueueIngestTotal(record.TenantID, record.ID, "default", 1)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.appendAuditLog(
		r,
		record.TenantID,
		gatewayAccessAuditAction(saved.Result),
		gatewayAccessEventAuditTarget(
			record.ID,
			"default",
			saved.ID,
			saved.Type,
			saved.Result,
			saved.DoorID,
			saved.Actor,
			saved.IdempotencyKey,
			deduped,
			saved.At,
		),
		"gateway_access_event",
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"event_id":        saved.ID,
		"status":          "accepted",
		"deduplicated":    deduped,
		"idempotency_key": saved.IdempotencyKey,
		"queue_progress": map[string]any{
			"queue":          queueProgress.Queue,
			"ingested_total": queueProgress.IngestedTotal,
			"updated_at":     queueProgress.UpdatedAt,
		},
		"received_at": time.Now().UTC(),
		"occurred_at": saved.At,
	})
}

func (s *server) gatewayBootstrapDeviceEvent(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID      string `json:"gateway_id"`
		TenantID       string `json:"tenant_id"`
		EventID        string `json:"event_id"`
		RequestID      string `json:"request_id"`
		BuildingID     string `json:"building_id"`
		Type           string `json:"type"`
		Detail         string `json:"detail"`
		Result         string `json:"result"`
		OccurredAt     string `json:"occurred_at"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, ok := s.findGatewayByTenant(request.TenantID, request.GatewayID)
	if !ok {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.authorizeGatewayDeviceToken(w, r, record.ID) {
		return
	}

	idempotencyKey := strings.TrimSpace(request.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(request.RequestID)
	}
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(request.EventID)
	}

	eventType := strings.TrimSpace(request.Type)
	if eventType == "" {
		eventType = "gateway_event"
	}
	result := strings.TrimSpace(request.Result)
	if result == "" {
		result = "accepted"
	}
	buildingID := strings.TrimSpace(request.BuildingID)
	if buildingID == "" {
		buildingID = strings.TrimSpace(record.BuildingID)
	}

	occurredAt := time.Now().UTC()
	if raw := strings.TrimSpace(request.OccurredAt); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "occurred_at must be RFC3339")
			return
		}
		occurredAt = parsed.UTC()
	}

	saved, deduped, err := s.eventSvc.IngestDeviceEvent(event.IngestDeviceEventInput{
		ID:             request.EventID,
		IdempotencyKey: idempotencyKey,
		TenantID:       record.TenantID,
		BuildingID:     buildingID,
		Type:           eventType,
		GatewayID:      record.ID,
		Detail:         request.Detail,
		Result:         result,
		At:             occurredAt,
	})
	if err != nil {
		switch {
		case errors.Is(err, event.ErrTenantIDRequired),
			errors.Is(err, event.ErrGatewayIDRequired),
			errors.Is(err, event.ErrDeviceEventTypeRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	var queueProgress gateway.GatewayQueueIngestTotal
	if deduped {
		queueProgress, err = s.gatewaySvc.GetQueueIngestTotal(record.TenantID, record.ID, "default")
		if err != nil {
			if errors.Is(err, gateway.ErrGatewayQueueIngestTotalNotFound) {
				accessTotal, deviceTotal := s.eventSvc.CountEventsByGateway(record.TenantID, record.ID)
				queueProgress = gateway.GatewayQueueIngestTotal{
					GatewayID:     record.ID,
					Queue:         "default",
					IngestedTotal: accessTotal + deviceTotal,
					UpdatedAt:     time.Now().UTC(),
				}
			} else {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	} else {
		queueProgress, err = s.gatewaySvc.AddQueueIngestTotal(record.TenantID, record.ID, "default", 1)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	s.appendAuditLog(
		r,
		record.TenantID,
		gatewayDeviceAuditAction(saved.Type),
		gatewayDeviceEventAuditTarget(
			record.ID,
			"default",
			saved.ID,
			saved.Type,
			saved.Result,
			saved.Detail,
			saved.IdempotencyKey,
			deduped,
			saved.At,
		),
		"gateway_device_event",
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"event_id":        saved.ID,
		"status":          "accepted",
		"deduplicated":    deduped,
		"idempotency_key": saved.IdempotencyKey,
		"queue_progress": map[string]any{
			"queue":          queueProgress.Queue,
			"ingested_total": queueProgress.IngestedTotal,
			"updated_at":     queueProgress.UpdatedAt,
		},
		"received_at": time.Now().UTC(),
		"occurred_at": saved.At,
	})
}

func (s *server) gatewayBootstrapEventsBatch(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID    string `json:"gateway_id"`
		TenantID     string `json:"tenant_id"`
		Queue        string `json:"queue"`
		AccessEvents []struct {
			EventID        string `json:"event_id"`
			RequestID      string `json:"request_id"`
			IdempotencyKey string `json:"idempotency_key"`
			BuildingID     string `json:"building_id"`
			AreaID         string `json:"area_id"`
			Type           string `json:"type"`
			Actor          string `json:"actor"`
			DoorID         string `json:"door_id"`
			Result         string `json:"result"`
			OccurredAt     string `json:"occurred_at"`
		} `json:"access_events"`
		DeviceEvents []struct {
			EventID        string `json:"event_id"`
			RequestID      string `json:"request_id"`
			IdempotencyKey string `json:"idempotency_key"`
			BuildingID     string `json:"building_id"`
			Type           string `json:"type"`
			Detail         string `json:"detail"`
			Result         string `json:"result"`
			OccurredAt     string `json:"occurred_at"`
		} `json:"device_events"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(request.AccessEvents) == 0 && len(request.DeviceEvents) == 0 {
		writeError(w, http.StatusBadRequest, "batch events are required")
		return
	}
	totalItems := len(request.AccessEvents) + len(request.DeviceEvents)
	if totalItems > gatewayEventsBatchMaxItems {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("batch size exceeded: max %d", gatewayEventsBatchMaxItems))
		return
	}

	record, ok := s.findGatewayByTenant(request.TenantID, request.GatewayID)
	if !ok {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.authorizeGatewayDeviceToken(w, r, record.ID) {
		return
	}
	if !s.validateGatewayRequestNonce(w, r) {
		return
	}
	queue := normalizeGatewayCheckpointQueue(request.Queue)
	receivedAt := time.Now().UTC()

	accessCreated := 0
	accessDeduplicated := 0
	accessFailed := 0
	accessRetryable := 0
	accessIDs := make([]string, 0, len(request.AccessEvents))
	accessResults := make([]map[string]any, 0, len(request.AccessEvents))
	retryAccessEvents := make([]map[string]any, 0, len(request.AccessEvents))
	for i := range request.AccessEvents {
		occurredAt, err := parseGatewayOccurredAt(request.AccessEvents[i].OccurredAt)
		if err != nil {
			accessFailed++
			accessResults = append(accessResults, map[string]any{
				"index":     i,
				"event_id":  strings.TrimSpace(request.AccessEvents[i].EventID),
				"status":    "failed",
				"error":     "occurred_at must be RFC3339",
				"retryable": false,
			})
			continue
		}
		if err := s.gatewayBatchForcedRetryableError(request.AccessEvents[i].EventID); err != nil {
			retryable := isGatewayBatchFailureRetryable(err)
			accessFailed++
			accessResults = append(accessResults, map[string]any{
				"index":     i,
				"event_id":  strings.TrimSpace(request.AccessEvents[i].EventID),
				"status":    "failed",
				"error":     err.Error(),
				"retryable": retryable,
			})
			if retryable {
				accessRetryable++
				retryAccessEvents = append(retryAccessEvents, map[string]any{
					"event_id":        request.AccessEvents[i].EventID,
					"request_id":      request.AccessEvents[i].RequestID,
					"idempotency_key": request.AccessEvents[i].IdempotencyKey,
					"building_id":     request.AccessEvents[i].BuildingID,
					"area_id":         request.AccessEvents[i].AreaID,
					"type":            request.AccessEvents[i].Type,
					"actor":           request.AccessEvents[i].Actor,
					"door_id":         request.AccessEvents[i].DoorID,
					"result":          request.AccessEvents[i].Result,
					"occurred_at":     request.AccessEvents[i].OccurredAt,
				})
			}
			continue
		}
		idempotencyKey := strings.TrimSpace(request.AccessEvents[i].IdempotencyKey)
		if idempotencyKey == "" {
			idempotencyKey = strings.TrimSpace(request.AccessEvents[i].RequestID)
		}
		if idempotencyKey == "" {
			idempotencyKey = strings.TrimSpace(request.AccessEvents[i].EventID)
		}
		eventType := strings.TrimSpace(request.AccessEvents[i].Type)
		if eventType == "" {
			eventType = "access_event"
		}
		result := strings.TrimSpace(request.AccessEvents[i].Result)
		if result == "" {
			result = "accepted"
		}
		buildingID := strings.TrimSpace(request.AccessEvents[i].BuildingID)
		if buildingID == "" {
			buildingID = strings.TrimSpace(record.BuildingID)
		}

		saved, deduped, err := s.eventSvc.IngestAccessEvent(event.IngestAccessEventInput{
			ID:             request.AccessEvents[i].EventID,
			IdempotencyKey: idempotencyKey,
			TenantID:       record.TenantID,
			BuildingID:     buildingID,
			AreaID:         request.AccessEvents[i].AreaID,
			Type:           eventType,
			Actor:          request.AccessEvents[i].Actor,
			DoorID:         request.AccessEvents[i].DoorID,
			GatewayID:      record.ID,
			Result:         result,
			At:             occurredAt,
		})
		if err != nil {
			retryable := isGatewayBatchFailureRetryable(err)
			accessFailed++
			accessResults = append(accessResults, map[string]any{
				"index":     i,
				"event_id":  strings.TrimSpace(request.AccessEvents[i].EventID),
				"status":    "failed",
				"error":     err.Error(),
				"retryable": retryable,
			})
			if retryable {
				accessRetryable++
				retryAccessEvents = append(retryAccessEvents, map[string]any{
					"event_id":        request.AccessEvents[i].EventID,
					"request_id":      request.AccessEvents[i].RequestID,
					"idempotency_key": request.AccessEvents[i].IdempotencyKey,
					"building_id":     request.AccessEvents[i].BuildingID,
					"area_id":         request.AccessEvents[i].AreaID,
					"type":            request.AccessEvents[i].Type,
					"actor":           request.AccessEvents[i].Actor,
					"door_id":         request.AccessEvents[i].DoorID,
					"result":          request.AccessEvents[i].Result,
					"occurred_at":     request.AccessEvents[i].OccurredAt,
				})
			}
			continue
		}
		if deduped {
			accessDeduplicated++
		} else {
			accessCreated++
		}
		accessIDs = append(accessIDs, saved.ID)
		accessResults = append(accessResults, map[string]any{
			"index":           i,
			"event_id":        saved.ID,
			"status":          "accepted",
			"deduplicated":    deduped,
			"idempotency_key": saved.IdempotencyKey,
		})
		s.appendAuditLog(
			r,
			record.TenantID,
			gatewayAccessAuditAction(saved.Result),
			gatewayAccessEventAuditTarget(
				record.ID,
				queue,
				saved.ID,
				saved.Type,
				saved.Result,
				saved.DoorID,
				saved.Actor,
				saved.IdempotencyKey,
				deduped,
				saved.At,
			),
			"gateway_access_event_batch",
		)
	}

	deviceCreated := 0
	deviceDeduplicated := 0
	deviceFailed := 0
	deviceRetryable := 0
	deviceIDs := make([]string, 0, len(request.DeviceEvents))
	deviceResults := make([]map[string]any, 0, len(request.DeviceEvents))
	retryDeviceEvents := make([]map[string]any, 0, len(request.DeviceEvents))
	for i := range request.DeviceEvents {
		occurredAt, err := parseGatewayOccurredAt(request.DeviceEvents[i].OccurredAt)
		if err != nil {
			deviceFailed++
			deviceResults = append(deviceResults, map[string]any{
				"index":     i,
				"event_id":  strings.TrimSpace(request.DeviceEvents[i].EventID),
				"status":    "failed",
				"error":     "occurred_at must be RFC3339",
				"retryable": false,
			})
			continue
		}
		if err := s.gatewayBatchForcedRetryableError(request.DeviceEvents[i].EventID); err != nil {
			retryable := isGatewayBatchFailureRetryable(err)
			deviceFailed++
			deviceResults = append(deviceResults, map[string]any{
				"index":     i,
				"event_id":  strings.TrimSpace(request.DeviceEvents[i].EventID),
				"status":    "failed",
				"error":     err.Error(),
				"retryable": retryable,
			})
			if retryable {
				deviceRetryable++
				retryDeviceEvents = append(retryDeviceEvents, map[string]any{
					"event_id":        request.DeviceEvents[i].EventID,
					"request_id":      request.DeviceEvents[i].RequestID,
					"idempotency_key": request.DeviceEvents[i].IdempotencyKey,
					"building_id":     request.DeviceEvents[i].BuildingID,
					"type":            request.DeviceEvents[i].Type,
					"detail":          request.DeviceEvents[i].Detail,
					"result":          request.DeviceEvents[i].Result,
					"occurred_at":     request.DeviceEvents[i].OccurredAt,
				})
			}
			continue
		}
		idempotencyKey := strings.TrimSpace(request.DeviceEvents[i].IdempotencyKey)
		if idempotencyKey == "" {
			idempotencyKey = strings.TrimSpace(request.DeviceEvents[i].RequestID)
		}
		if idempotencyKey == "" {
			idempotencyKey = strings.TrimSpace(request.DeviceEvents[i].EventID)
		}
		eventType := strings.TrimSpace(request.DeviceEvents[i].Type)
		if eventType == "" {
			eventType = "gateway_event"
		}
		result := strings.TrimSpace(request.DeviceEvents[i].Result)
		if result == "" {
			result = "accepted"
		}
		buildingID := strings.TrimSpace(request.DeviceEvents[i].BuildingID)
		if buildingID == "" {
			buildingID = strings.TrimSpace(record.BuildingID)
		}

		saved, deduped, err := s.eventSvc.IngestDeviceEvent(event.IngestDeviceEventInput{
			ID:             request.DeviceEvents[i].EventID,
			IdempotencyKey: idempotencyKey,
			TenantID:       record.TenantID,
			BuildingID:     buildingID,
			Type:           eventType,
			GatewayID:      record.ID,
			Detail:         request.DeviceEvents[i].Detail,
			Result:         result,
			At:             occurredAt,
		})
		if err != nil {
			retryable := isGatewayBatchFailureRetryable(err)
			deviceFailed++
			deviceResults = append(deviceResults, map[string]any{
				"index":     i,
				"event_id":  strings.TrimSpace(request.DeviceEvents[i].EventID),
				"status":    "failed",
				"error":     err.Error(),
				"retryable": retryable,
			})
			if retryable {
				deviceRetryable++
				retryDeviceEvents = append(retryDeviceEvents, map[string]any{
					"event_id":        request.DeviceEvents[i].EventID,
					"request_id":      request.DeviceEvents[i].RequestID,
					"idempotency_key": request.DeviceEvents[i].IdempotencyKey,
					"building_id":     request.DeviceEvents[i].BuildingID,
					"type":            request.DeviceEvents[i].Type,
					"detail":          request.DeviceEvents[i].Detail,
					"result":          request.DeviceEvents[i].Result,
					"occurred_at":     request.DeviceEvents[i].OccurredAt,
				})
			}
			continue
		}
		if deduped {
			deviceDeduplicated++
		} else {
			deviceCreated++
		}
		deviceIDs = append(deviceIDs, saved.ID)
		deviceResults = append(deviceResults, map[string]any{
			"index":           i,
			"event_id":        saved.ID,
			"status":          "accepted",
			"deduplicated":    deduped,
			"idempotency_key": saved.IdempotencyKey,
		})
		s.appendAuditLog(
			r,
			record.TenantID,
			gatewayDeviceAuditAction(saved.Type),
			gatewayDeviceEventAuditTarget(
				record.ID,
				queue,
				saved.ID,
				saved.Type,
				saved.Result,
				saved.Detail,
				saved.IdempotencyKey,
				deduped,
				saved.At,
			),
			"gateway_device_event_batch",
		)
	}

	totalFailed := accessFailed + deviceFailed
	totalRetryableFailed := accessRetryable + deviceRetryable
	acceptedTotal := totalItems - totalFailed
	createdTotal := accessCreated + deviceCreated
	queueIngestedTotal := 0
	if createdTotal > 0 {
		progress, err := s.gatewaySvc.AddQueueIngestTotal(record.TenantID, record.ID, queue, createdTotal)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		queueIngestedTotal = progress.IngestedTotal
	} else {
		progress, err := s.gatewaySvc.GetQueueIngestTotal(record.TenantID, record.ID, queue)
		if err == nil {
			queueIngestedTotal = progress.IngestedTotal
		} else if errors.Is(err, gateway.ErrGatewayQueueIngestTotalNotFound) && queue == "default" {
			accessTotal, deviceTotal := s.eventSvc.CountEventsByGateway(record.TenantID, record.ID)
			queueIngestedTotal = accessTotal + deviceTotal
		}
	}
	suggestedCheckpointID := gatewayBatchSuggestedCheckpointID(record.ID, queue, receivedAt)
	queueStatusCode := gatewayBatchQueueStatusCode(totalFailed, totalRetryableFailed)
	nextAction := gatewayBatchNextAction(totalFailed, totalRetryableFailed)
	status := "accepted"
	if totalFailed > 0 {
		status = "partial"
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":      status,
		"gateway_id":  record.ID,
		"tenant_id":   record.TenantID,
		"received_at": receivedAt,
		"access": map[string]any{
			"received":     len(request.AccessEvents),
			"created":      accessCreated,
			"deduplicated": accessDeduplicated,
			"failed":       accessFailed,
			"event_ids":    accessIDs,
			"results":      accessResults,
		},
		"device": map[string]any{
			"received":     len(request.DeviceEvents),
			"created":      deviceCreated,
			"deduplicated": deviceDeduplicated,
			"failed":       deviceFailed,
			"event_ids":    deviceIDs,
			"results":      deviceResults,
		},
		"totals": map[string]any{
			"received":             totalItems,
			"created":              accessCreated + deviceCreated,
			"deduplicated":         accessDeduplicated + deviceDeduplicated,
			"failed":               totalFailed,
			"retryable_failed":     totalRetryableFailed,
			"non_retryable_failed": totalFailed - totalRetryableFailed,
		},
		"retry_subset": map[string]any{
			"gateway_id":    record.ID,
			"tenant_id":     record.TenantID,
			"queue":         queue,
			"access_events": retryAccessEvents,
			"device_events": retryDeviceEvents,
		},
		"queue_hint": map[string]any{
			"queue":                 queue,
			"checkpoint_id":         suggestedCheckpointID,
			"acked_increment":       acceptedTotal,
			"server_ingested_total": queueIngestedTotal,
			"status_code":           queueStatusCode,
			"next_action":           nextAction,
			"retry_subset_present":  totalRetryableFailed > 0,
		},
	})
}

func (s *server) gatewayBootstrapEventsCheckpoint(w http.ResponseWriter, r *http.Request) {
	var request struct {
		GatewayID      string `json:"gateway_id"`
		TenantID       string `json:"tenant_id"`
		Queue          string `json:"queue"`
		CheckpointID   string `json:"checkpoint_id"`
		LastRequestID  string `json:"last_request_id"`
		AckedCount     int    `json:"acked_count"`
		LastOccurredAt string `json:"last_occurred_at"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, ok := s.findGatewayByTenant(request.TenantID, request.GatewayID)
	if !ok {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	if !s.authorizeGatewayDeviceToken(w, r, record.ID) {
		return
	}

	queue := strings.TrimSpace(request.Queue)
	if queue == "" {
		queue = "default"
	}
	if request.AckedCount >= 0 {
		serverEventTotal := -1
		serverEventTotalSource := ""
		progress, err := s.gatewaySvc.GetQueueIngestTotal(request.TenantID, request.GatewayID, queue)
		if err == nil {
			serverEventTotal = progress.IngestedTotal
			serverEventTotalSource = "queue_ingest_total"
		}
		if serverEventTotal < 0 && queue == "default" {
			accessTotal, deviceTotal := s.eventSvc.CountEventsByGateway(record.TenantID, record.ID)
			serverEventTotal = accessTotal + deviceTotal
			serverEventTotalSource = "event_rows_fallback"
		}
		if serverEventTotal >= 0 && request.AckedCount > serverEventTotal {
			response := map[string]any{
				"error":               "event checkpoint acked_count exceeds server event total",
				"next_action":         "retry_with_server_event_total",
				"server_event_total":  serverEventTotal,
				"server_total_source": serverEventTotalSource,
				"queue":               queue,
			}
			latest, err := s.gatewaySvc.GetEventCheckpoint(request.TenantID, request.GatewayID, queue)
			if err == nil {
				response["checkpoint"] = map[string]any{
					"gateway_id":       latest.GatewayID,
					"tenant_id":        record.TenantID,
					"queue":            latest.Queue,
					"checkpoint_id":    latest.CheckpointID,
					"last_request_id":  latest.LastRequestID,
					"acked_count":      latest.AckedCount,
					"last_occurred_at": latest.LastOccurredAt,
					"updated_at":       latest.UpdatedAt,
				}
			}
			writeJSON(w, http.StatusConflict, response)
			return
		}
	}
	var lastOccurredAt *time.Time
	if raw := strings.TrimSpace(request.LastOccurredAt); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "last_occurred_at must be RFC3339")
			return
		}
		ts := parsed.UTC()
		lastOccurredAt = &ts
	}

	checkpoint, err := s.gatewaySvc.UpsertEventCheckpoint(
		request.TenantID,
		request.GatewayID,
		queue,
		request.CheckpointID,
		request.LastRequestID,
		request.AckedCount,
		lastOccurredAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayEventCheckpointAckedCountRegression):
			latest, latestErr := s.gatewaySvc.GetEventCheckpoint(request.TenantID, request.GatewayID, queue)
			if latestErr == nil {
				writeJSON(w, http.StatusConflict, map[string]any{
					"error":       err.Error(),
					"next_action": "retry_with_non_regressing_acked_count",
					"checkpoint": map[string]any{
						"gateway_id":       latest.GatewayID,
						"tenant_id":        record.TenantID,
						"queue":            latest.Queue,
						"checkpoint_id":    latest.CheckpointID,
						"last_request_id":  latest.LastRequestID,
						"acked_count":      latest.AckedCount,
						"last_occurred_at": latest.LastOccurredAt,
						"updated_at":       latest.UpdatedAt,
					},
				})
				return
			}
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired),
			errors.Is(err, gateway.ErrGatewayEventCheckpointQueueRequired),
			errors.Is(err, gateway.ErrGatewayEventCheckpointRequired),
			errors.Is(err, gateway.ErrGatewayEventCheckpointAckedCountInvalid):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	target := fmt.Sprintf(
		"gateway=%s queue=%s checkpoint=%s acked=%d last_request=%s",
		checkpoint.GatewayID,
		checkpoint.Queue,
		checkpoint.CheckpointID,
		checkpoint.AckedCount,
		checkpoint.LastRequestID,
	)
	s.appendAuditLog(r, record.TenantID, "gateway_event_checkpoint_reported", target, "gateway_event_checkpoint")

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id":       checkpoint.GatewayID,
		"tenant_id":        record.TenantID,
		"queue":            checkpoint.Queue,
		"checkpoint_id":    checkpoint.CheckpointID,
		"last_request_id":  checkpoint.LastRequestID,
		"acked_count":      checkpoint.AckedCount,
		"last_occurred_at": checkpoint.LastOccurredAt,
		"updated_at":       checkpoint.UpdatedAt,
	})
}

func (s *server) listGatewayEventCheckpoints(w http.ResponseWriter, r *http.Request) {
	gatewayID := chi.URLParam(r, "gatewayID")
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if buildingScope != nil {
		gw, exists := s.findGatewayByTenant(tenantID, gatewayID)
		if !exists {
			writeError(w, http.StatusNotFound, "gateway not found")
			return
		}
		if !s.requireBuildingScope(w, buildingScope, gw.BuildingID) {
			return
		}
	}

	items, err := s.gatewaySvc.ListEventCheckpoints(tenantID, gatewayID)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, gateway.ErrGatewayIDRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (s *server) listGatewayEventCheckpointSummary(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	gatewayID := strings.TrimSpace(r.URL.Query().Get("gateway_id"))
	queue := strings.TrimSpace(r.URL.Query().Get("queue"))

	limit := 100
	limitInput := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitInput != "" {
		parsedLimit, err := strconv.Atoi(limitInput)
		if err != nil || parsedLimit < 0 {
			writeError(w, http.StatusBadRequest, "limit must be an integer >= 0")
			return
		}
		limit = parsedLimit
	}
	trendWindowMinutes := 60
	trendWindowInput := strings.TrimSpace(r.URL.Query().Get("trend_window_minutes"))
	if trendWindowInput != "" {
		parsedWindow, err := strconv.Atoi(trendWindowInput)
		if err != nil || parsedWindow <= 0 {
			writeError(w, http.StatusBadRequest, "trend_window_minutes must be an integer > 0")
			return
		}
		trendWindowMinutes = parsedWindow
	}

	gateways := s.gatewaySvc.List(tenantID)
	gatewayByID := make(map[string]gateway.Gateway, len(gateways))
	allowedGatewayIDs := make(map[string]struct{}, len(gateways))
	for i := range gateways {
		gatewayByID[gateways[i].ID] = gateways[i]
		if buildingScope != nil {
			if _, exists := buildingScope[gateways[i].BuildingID]; !exists {
				continue
			}
		}
		allowedGatewayIDs[gateways[i].ID] = struct{}{}
	}
	if gatewayID != "" {
		if _, exists := gatewayByID[gatewayID]; !exists {
			writeError(w, http.StatusNotFound, "gateway not found")
			return
		}
		if buildingScope != nil {
			if _, allowed := allowedGatewayIDs[gatewayID]; !allowed {
				writeError(w, http.StatusForbidden, "building scope forbidden")
				return
			}
		}
	}

	checkpoints, err := s.gatewaySvc.ListEventCheckpointsByTenant(tenantID, gatewayID, queue)
	if err != nil {
		switch {
		case errors.Is(err, gateway.ErrGatewayNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	trendWindowUntil := time.Now().UTC()
	trendWindowSince := trendWindowUntil.Add(-time.Duration(trendWindowMinutes) * time.Minute)
	checkpointAuditLogs := []audit.Log{}
	if s.auditSvc != nil {
		checkpointAuditLogs = s.auditSvc.ListFiltered(
			tenantID,
			"gateway_event_checkpoint_reported",
			"gateway_event_checkpoint",
			0,
		)
		checkpointAuditLogs = filterAuditLogsByTimeRange(checkpointAuditLogs, &trendWindowSince, &trendWindowUntil)
	}
	queueTrends, trendSummary := buildGatewayCheckpointWindowTrends(
		checkpointAuditLogs,
		allowedGatewayIDs,
		gatewayID,
		queue,
	)

	items := make([]map[string]any, 0, len(checkpoints))
	totalLag := 0
	totalAcked := 0
	totalEvents := 0
	for i := range checkpoints {
		if _, allowed := allowedGatewayIDs[checkpoints[i].GatewayID]; !allowed {
			continue
		}
		gw, exists := gatewayByID[checkpoints[i].GatewayID]
		if !exists {
			continue
		}
		queueKey := gatewayCheckpointTrendKey(checkpoints[i].GatewayID, checkpoints[i].Queue)
		itemTrend := queueTrends[queueKey]
		if itemTrend.Direction == "" {
			itemTrend.Direction = "flat"
		}
		accessCount, deviceCount := s.eventSvc.CountEventsByGateway(tenantID, checkpoints[i].GatewayID)
		eventTotal := accessCount + deviceCount
		lag := eventTotal - checkpoints[i].AckedCount
		if lag < 0 {
			lag = 0
		}

		totalLag += lag
		totalAcked += checkpoints[i].AckedCount
		totalEvents += eventTotal

		items = append(items, map[string]any{
			"gateway_id":         checkpoints[i].GatewayID,
			"tenant_id":          gw.TenantID,
			"building_id":        gw.BuildingID,
			"queue":              checkpoints[i].Queue,
			"checkpoint_id":      checkpoints[i].CheckpointID,
			"last_request_id":    checkpoints[i].LastRequestID,
			"acked_count":        checkpoints[i].AckedCount,
			"event_total":        eventTotal,
			"access_event_total": accessCount,
			"device_event_total": deviceCount,
			"lag_count":          lag,
			"last_occurred_at":   checkpoints[i].LastOccurredAt,
			"updated_at":         checkpoints[i].UpdatedAt,
			"time_window_trend": map[string]any{
				"report_total":    itemTrend.ReportTotal,
				"acked_delta":     itemTrend.AckedDelta,
				"direction":       itemTrend.Direction,
				"first_report_at": itemTrend.FirstReportAt,
				"last_report_at":  itemTrend.LastReportAt,
			},
		})
	}

	sort.Slice(items, func(i, j int) bool {
		lagI, _ := items[i]["lag_count"].(int)
		lagJ, _ := items[j]["lag_count"].(int)
		if lagI != lagJ {
			return lagI > lagJ
		}
		atI, _ := items[i]["updated_at"].(time.Time)
		atJ, _ := items[j]["updated_at"].(time.Time)
		return atI.After(atJ)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"totals": map[string]any{
			"queues":      len(items),
			"event_total": totalEvents,
			"acked_total": totalAcked,
			"lag_total":   totalLag,
		},
		"time_window_trend": map[string]any{
			"window_minutes":    trendWindowMinutes,
			"since":             trendWindowSince,
			"until":             trendWindowUntil,
			"report_total":      trendSummary.ReportTotal,
			"gateway_total":     trendSummary.GatewayTotal,
			"queue_total":       trendSummary.QueueTotal,
			"acked_delta_total": trendSummary.AckedDeltaTotal,
			"direction":         trendSummary.Direction,
			"last_report_at":    trendSummary.LastReportAt,
		},
	})
}

func parseGatewayOccurredAt(raw string) (time.Time, error) {
	next := strings.TrimSpace(raw)
	if next == "" {
		return time.Now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, next)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func gatewayBatchSuggestedCheckpointID(gatewayID, queue string, receivedAt time.Time) string {
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" {
		nextGatewayID = "gateway"
	}
	nextQueue := normalizeGatewayCheckpointQueue(queue)
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	return fmt.Sprintf("%s-%s-%d", nextGatewayID, nextQueue, receivedAt.UnixMilli())
}

func gatewayBatchQueueStatusCode(totalFailed, totalRetryableFailed int) string {
	if totalRetryableFailed > 0 {
		return "QUEUE_RETRY_SUBSET_REQUIRED"
	}
	if totalFailed > 0 {
		return "QUEUE_PARTIAL_NON_RETRYABLE"
	}
	return "QUEUE_READY_TO_CHECKPOINT"
}

func gatewayBatchNextAction(totalFailed, totalRetryableFailed int) string {
	if totalRetryableFailed > 0 {
		return "replay_retry_subset_then_report_checkpoint"
	}
	if totalFailed > 0 {
		return "report_checkpoint_with_non_retryable_failures"
	}
	return "report_checkpoint"
}

func gatewayAccessAuditAction(result string) string {
	next := strings.ToLower(strings.TrimSpace(result))
	switch next {
	case "accepted", "success", "granted", "allow":
		return "gateway_access_grant_recorded"
	case "denied", "rejected", "forbidden", "deny":
		return "gateway_access_deny_recorded"
	default:
		return "gateway_access_event_recorded"
	}
}

func gatewayDeviceAuditAction(eventType string) string {
	next := strings.ToLower(strings.TrimSpace(eventType))
	switch {
	case strings.Contains(next, "tamper"):
		return "gateway_tamper_event_recorded"
	case strings.Contains(next, "timeout"):
		return "gateway_door_timeout_recorded"
	case strings.Contains(next, "rex"):
		return "gateway_rex_event_recorded"
	default:
		return "gateway_device_event_recorded"
	}
}

func gatewayAccessEventAuditTarget(
	gatewayID,
	queue,
	eventID,
	eventType,
	result,
	doorID,
	actor,
	idempotencyKey string,
	deduplicated bool,
	occurredAt time.Time,
) string {
	return strings.TrimSpace(
		fmt.Sprintf(
			"gateway=%s queue=%s event=%s type=%s result=%s door=%s actor=%s deduplicated=%t idempotency_key=%s occurred_at=%s",
			strings.TrimSpace(gatewayID),
			normalizeGatewayCheckpointQueue(queue),
			strings.TrimSpace(eventID),
			strings.TrimSpace(eventType),
			strings.TrimSpace(result),
			strings.TrimSpace(doorID),
			strings.TrimSpace(actor),
			deduplicated,
			strings.TrimSpace(idempotencyKey),
			occurredAt.UTC().Format(time.RFC3339),
		),
	)
}

func gatewayDeviceEventAuditTarget(
	gatewayID,
	queue,
	eventID,
	eventType,
	result,
	detail,
	idempotencyKey string,
	deduplicated bool,
	occurredAt time.Time,
) string {
	return strings.TrimSpace(
		fmt.Sprintf(
			"gateway=%s queue=%s event=%s type=%s result=%s detail=%s deduplicated=%t idempotency_key=%s occurred_at=%s",
			strings.TrimSpace(gatewayID),
			normalizeGatewayCheckpointQueue(queue),
			strings.TrimSpace(eventID),
			strings.TrimSpace(eventType),
			strings.TrimSpace(result),
			strings.TrimSpace(detail),
			deduplicated,
			strings.TrimSpace(idempotencyKey),
			occurredAt.UTC().Format(time.RFC3339),
		),
	)
}

func isGatewayBatchFailureRetryable(err error) bool {
	if err == nil {
		return false
	}
	return !errors.Is(err, event.ErrTenantIDRequired) &&
		!errors.Is(err, event.ErrGatewayIDRequired) &&
		!errors.Is(err, event.ErrAccessEventTypeRequired) &&
		!errors.Is(err, event.ErrDeviceEventTypeRequired)
}

func (s *server) gatewayBatchForcedRetryableError(eventID string) error {
	if !s.cfg.GatewayEventsBatchForceRetryableError {
		return nil
	}
	nextEventID := strings.TrimSpace(eventID)
	if nextEventID == "" {
		return nil
	}
	prefix := strings.TrimSpace(s.cfg.GatewayEventsBatchForceRetryablePrefix)
	if prefix == "" {
		prefix = "force-retry-"
	}
	if !strings.HasPrefix(nextEventID, prefix) {
		return nil
	}

	s.gatewayBatchFailureMu.Lock()
	defer s.gatewayBatchFailureMu.Unlock()
	if _, exists := s.gatewayBatchFailureSeen[nextEventID]; exists {
		return nil
	}
	s.gatewayBatchFailureSeen[nextEventID] = struct{}{}
	return errors.New("forced retryable batch failure for testing")
}

type gatewayConfigAuthzScope struct {
	BuildingIDs []string `json:"building_ids,omitempty"`
	AreaIDs     []string `json:"area_ids,omitempty"`
	DoorIDs     []string `json:"door_ids,omitempty"`
}

type gatewayConfigAuthzCacheCounts struct {
	Doors           int `json:"doors"`
	Policies        int `json:"policies"`
	TemporaryAccess int `json:"temporary_access"`
	VisitorPasses   int `json:"visitor_passes"`
	Users           int `json:"users"`
	UserGroups      int `json:"user_groups"`
	AccessRules     int `json:"access_rules"`
}

type gatewayConfigAuthzCachePolicy struct {
	FallbackMode        string    `json:"fallback_mode"`
	NoCacheBehavior     string    `json:"no_cache_behavior"`
	MaxStaleSeconds     int       `json:"max_stale_seconds"`
	StaleUntil          time.Time `json:"stale_until"`
	RefreshRetrySeconds int       `json:"refresh_retry_seconds"`
	RollbackVersion     string    `json:"rollback_version"`
}

type gatewayConfigAuthzStatusCodes struct {
	Fresh   string `json:"fresh"`
	Stale   string `json:"stale"`
	Missing string `json:"missing"`
	Drift   string `json:"drift"`
}

type gatewayConfigAccessRule struct {
	CredentialType string              `json:"credential_type"`
	CredentialData string              `json:"credential_data"`
	UserID         string              `json:"user_id"`
	UserEmail      string              `json:"user_email"`
	LockIDs        []string            `json:"lock_ids"`
	TimeWindows    []access.TimeWindow `json:"time_windows,omitempty"`
	ExceptionDates []string            `json:"exception_dates,omitempty"`
	ValidFrom      string              `json:"valid_from,omitempty"`
	ValidUntil     string              `json:"valid_until,omitempty"`
}

type gatewayConfigAuthzCache struct {
	Version         string                        `json:"version"`
	VersionReported string                        `json:"version_reported,omitempty"`
	Status          string                        `json:"status,omitempty"`
	GeneratedAt     time.Time                     `json:"generated_at"`
	ExpiresAt       time.Time                     `json:"expires_at"`
	TTLSeconds      int                           `json:"ttl_seconds"`
	Scope           gatewayConfigAuthzScope       `json:"scope"`
	Policy          gatewayConfigAuthzCachePolicy `json:"policy"`
	StatusCodes     gatewayConfigAuthzStatusCodes `json:"status_codes"`
	Counts          gatewayConfigAuthzCacheCounts `json:"counts"`
	Doors           []space.Door                  `json:"doors,omitempty"`
	Policies        []access.Policy               `json:"policies,omitempty"`
	TemporaryAccess []access.TemporaryAccess      `json:"temporary_access,omitempty"`
	VisitorPasses   []access.VisitorPass          `json:"visitor_passes,omitempty"`
	Users           []access.AccessUser           `json:"users,omitempty"`
	UserGroups      []access.UserGroup            `json:"user_groups,omitempty"`
	AccessRules     []gatewayConfigAccessRule      `json:"access_rules,omitempty"`
	LockdownLocks   []string                       `json:"lockdown_locks,omitempty"`
}

func (s *server) buildGatewayConfigAuthzCache(tenantID, gatewayID string, boundDoorIDs []string, generatedAt time.Time) gatewayConfigAuthzCache {
	scope, doorIDSet, buildingIDSet, areaIDSet, doors := s.gatewayConfigAuthzScopeFromBoundDoors(tenantID, boundDoorIDs)
	hasBoundDoors := len(scope.DoorIDs) > 0
	generatedAtUTC := generatedAt.UTC()
	rollbackVersion := s.gatewayAuthzCacheAckVersion(gatewayID)

	userGroups := s.gatewayConfigAuthzUserGroups(tenantID, hasBoundDoors, buildingIDSet)
	allowedUserGroupIDs := make(map[string]struct{}, len(userGroups))
	for i := range userGroups {
		allowedUserGroupIDs[userGroups[i].ID] = struct{}{}
	}

	cache := gatewayConfigAuthzCache{
		GeneratedAt: generatedAtUTC,
		ExpiresAt:   generatedAtUTC.Add(time.Duration(gatewayConfigAuthzCacheTTLSeconds) * time.Second),
		TTLSeconds:  gatewayConfigAuthzCacheTTLSeconds,
		Scope:       scope,
		Policy: gatewayConfigAuthzCachePolicy{
			FallbackMode:        "use_last_acknowledged",
			NoCacheBehavior:     "deny_all",
			MaxStaleSeconds:     gatewayConfigAuthzCacheMaxStaleSeconds,
			StaleUntil:          generatedAtUTC.Add(time.Duration(gatewayConfigAuthzCacheMaxStaleSeconds) * time.Second),
			RefreshRetrySeconds: gatewayConfigAuthzCacheRefreshRetrySeconds,
			RollbackVersion:     rollbackVersion,
		},
		StatusCodes: gatewayConfigAuthzStatusCodes{
			Fresh:   "AUTHZ_CACHE_FRESH",
			Stale:   "AUTHZ_CACHE_STALE",
			Missing: "AUTHZ_CACHE_MISSING",
			Drift:   "AUTHZ_CACHE_DRIFT",
		},
		Doors:    doors,
		Policies: s.gatewayConfigAuthzPolicies(tenantID, hasBoundDoors, doorIDSet, buildingIDSet, areaIDSet),
		TemporaryAccess: s.gatewayConfigAuthzTemporaryAccess(
			tenantID,
			hasBoundDoors,
			doorIDSet,
			buildingIDSet,
			areaIDSet,
		),
		VisitorPasses: s.gatewayConfigAuthzVisitorPasses(tenantID, hasBoundDoors, buildingIDSet),
		Users:         s.gatewayConfigAuthzUsers(tenantID, hasBoundDoors, buildingIDSet, allowedUserGroupIDs),
		UserGroups:    userGroups,
	}
	cache.AccessRules = s.buildGatewayAccessRules(tenantID, boundDoorIDs, cache.Users, cache.UserGroups)
	cache.Counts = gatewayConfigAuthzCacheCounts{
		Doors:           len(cache.Doors),
		Policies:        len(cache.Policies),
		TemporaryAccess: len(cache.TemporaryAccess),
		VisitorPasses:   len(cache.VisitorPasses),
		Users:           len(cache.Users),
		UserGroups:      len(cache.UserGroups),
		AccessRules:     len(cache.AccessRules),
	}
	cache.Version = gatewayConfigAuthzCacheVersion(cache)
	return cache
}

func (s *server) buildGatewayAccessRules(
	tenantID string,
	boundDoorIDs []string,
	users []access.AccessUser,
	userGroups []access.UserGroup,
) []gatewayConfigAccessRule {
	if s.walletSvc == nil || len(boundDoorIDs) == 0 {
		return nil
	}

	boundDoorSet := make(map[string]struct{}, len(boundDoorIDs))
	for _, id := range boundDoorIDs {
		boundDoorSet[id] = struct{}{}
	}

	// Build user → accessible lock IDs (within this gateway's bound doors)
	userLockAccess := make(map[string][]string) // userID → []lockID
	doorGroups := s.spaceSvc.ListDoorGroups(tenantID)
	allDoors := s.spaceSvc.ListDoors(tenantID)

	for _, user := range users {
		lockIDs := make(map[string]struct{})

		// Via user groups → door groups
		for _, ug := range userGroups {
			if !containsString(user.GroupIDs, ug.ID) {
				continue
			}
			// Door groups in same building
			for _, dg := range doorGroups {
				for _, doorID := range dg.DoorIDs {
					if _, bound := boundDoorSet[doorID]; bound {
						lockIDs[doorID] = struct{}{}
					}
				}
			}
			// All doors in the building where this group applies
			for _, door := range allDoors {
				if _, bound := boundDoorSet[door.ID]; !bound {
					continue
				}
				if door.BuildingID == ug.BuildingID || door.BuildingID == ug.PlaceID {
					lockIDs[door.ID] = struct{}{}
				}
			}
		}

		// Via role assignments
		for _, ra := range s.accessSvc.ListRoleAssignments(tenantID) {
			if ra.AssigneeType != "User" || ra.AssigneeID != user.ID {
				continue
			}
			for _, door := range allDoors {
				if _, bound := boundDoorSet[door.ID]; !bound {
					continue
				}
				if ra.AppliesToType == "Organization" || (ra.AppliesToType == "Place" && ra.AppliesToID == door.BuildingID) {
					lockIDs[door.ID] = struct{}{}
				}
			}
		}

		if len(lockIDs) > 0 {
			ids := make([]string, 0, len(lockIDs))
			for id := range lockIDs {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			userLockAccess[user.ID] = ids
		}
	}

	// Map credentials → users → access rules
	rules := make([]gatewayConfigAccessRule, 0)

	// Physical card inventory → NFC UIDs
	for _, item := range s.walletSvc.ListPhysicalCardInventory(tenantID, "") {
		if item.UID == "" || item.AssignedPassID == "" {
			continue
		}
		pass, err := s.walletSvc.GetPass(tenantID, item.AssignedPassID)
		if err != nil || pass.TargetType != "user" || pass.Status != "active" {
			continue
		}
		lockIDs, exists := userLockAccess[pass.TargetID]
		if !exists {
			continue
		}
		user := findUserByID(users, pass.TargetID)
		rules = append(rules, gatewayConfigAccessRule{
			CredentialType: "nfc_uid",
			CredentialData: item.UID,
			UserID:         pass.TargetID,
			UserEmail:      userEmail(user),
			LockIDs:        lockIDs,
		})
		if item.CardNumber != "" {
			rules = append(rules, gatewayConfigAccessRule{
				CredentialType: "card_number",
				CredentialData: item.CardNumber,
				UserID:         pass.TargetID,
				UserEmail:      userEmail(user),
				LockIDs:        lockIDs,
			})
		}
	}

	// Passes with UID (cards registered directly)
	for _, pass := range s.walletSvc.ListPasses(tenantID) {
		if pass.UID == "" || pass.TargetType != "user" || pass.Status != "active" {
			continue
		}
		lockIDs, exists := userLockAccess[pass.TargetID]
		if !exists {
			continue
		}
		// Skip if already covered by physical card inventory
		alreadyCovered := false
		for _, r := range rules {
			if r.CredentialType == "nfc_uid" && r.CredentialData == pass.UID {
				alreadyCovered = true
				break
			}
		}
		if alreadyCovered {
			continue
		}
		user := findUserByID(users, pass.TargetID)
		rules = append(rules, gatewayConfigAccessRule{
			CredentialType: "nfc_uid",
			CredentialData: pass.UID,
			UserID:         pass.TargetID,
			UserEmail:      userEmail(user),
			LockIDs:        lockIDs,
		})
	}

	return rules
}

func findUserByID(users []access.AccessUser, id string) *access.AccessUser {
	for i := range users {
		if users[i].ID == id {
			return &users[i]
		}
	}
	return nil
}

func userEmail(user *access.AccessUser) string {
	if user == nil {
		return ""
	}
	return user.Email
}

func gatewayConfigAuthzResolveStatus(
	codes gatewayConfigAuthzStatusCodes,
	reportedVersion,
	expectedVersion,
	rollbackVersion string,
) string {
	reported := strings.TrimSpace(reportedVersion)
	expected := strings.TrimSpace(expectedVersion)
	rollback := strings.TrimSpace(rollbackVersion)
	if reported == "" {
		return codes.Missing
	}
	if expected != "" && reported == expected {
		return codes.Fresh
	}
	if rollback != "" && reported == rollback {
		return codes.Stale
	}
	return codes.Drift
}

func (s *server) gatewayConfigAuthzScopeFromBoundDoors(
	tenantID string,
	boundDoorIDs []string,
) (gatewayConfigAuthzScope, map[string]struct{}, map[string]struct{}, map[string]struct{}, []space.Door) {
	doorIDs := sortedUniqueTrimmedIDs(boundDoorIDs)
	doorIDSet := make(map[string]struct{}, len(doorIDs))
	for i := range doorIDs {
		doorIDSet[doorIDs[i]] = struct{}{}
	}
	buildingIDSet := map[string]struct{}{}
	areaIDSet := map[string]struct{}{}

	scope := gatewayConfigAuthzScope{
		DoorIDs: doorIDs,
	}
	if len(doorIDSet) == 0 || s.spaceSvc == nil {
		return scope, doorIDSet, buildingIDSet, areaIDSet, nil
	}

	doors := make([]space.Door, 0, len(doorIDSet))
	allDoors := s.spaceSvc.ListDoors(tenantID)
	for i := range allDoors {
		doorID := strings.TrimSpace(allDoors[i].ID)
		if _, exists := doorIDSet[doorID]; !exists {
			continue
		}
		doors = append(doors, allDoors[i])
		if buildingID := strings.TrimSpace(allDoors[i].BuildingID); buildingID != "" {
			buildingIDSet[buildingID] = struct{}{}
		}
		if areaID := strings.TrimSpace(allDoors[i].AreaID); areaID != "" {
			areaIDSet[areaID] = struct{}{}
		}
	}
	sort.Slice(doors, func(i, j int) bool {
		return doors[i].ID < doors[j].ID
	})

	scope.BuildingIDs = sortedSetKeys(buildingIDSet)
	scope.AreaIDs = sortedSetKeys(areaIDSet)
	return scope, doorIDSet, buildingIDSet, areaIDSet, doors
}

func (s *server) gatewayConfigAuthzPolicies(
	tenantID string,
	hasBoundDoors bool,
	doorIDSet, buildingIDSet, areaIDSet map[string]struct{},
) []access.Policy {
	if !hasBoundDoors || s.accessSvc == nil {
		return nil
	}
	items := make([]access.Policy, 0)
	policies := s.accessSvc.ListPolicies(tenantID)
	for i := range policies {
		if strings.ToLower(strings.TrimSpace(policies[i].Status)) != "active" {
			continue
		}
		if !gatewayConfigAuthzScopeMatch(
			policies[i].ScopeType,
			policies[i].BuildingID,
			policies[i].AreaID,
			policies[i].DoorID,
			hasBoundDoors,
			doorIDSet,
			buildingIDSet,
			areaIDSet,
		) {
			continue
		}
		items = append(items, policies[i])
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == items[j].ID {
			return items[i].Name < items[j].Name
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func (s *server) gatewayConfigAuthzTemporaryAccess(
	tenantID string,
	hasBoundDoors bool,
	doorIDSet, buildingIDSet, areaIDSet map[string]struct{},
) []access.TemporaryAccess {
	if !hasBoundDoors || s.accessSvc == nil {
		return nil
	}
	items := make([]access.TemporaryAccess, 0)
	records := s.accessSvc.ListTemporaryAccess(tenantID)
	for i := range records {
		if !gatewayConfigAuthzScopeMatch(
			records[i].ScopeType,
			records[i].BuildingID,
			records[i].AreaID,
			records[i].DoorID,
			hasBoundDoors,
			doorIDSet,
			buildingIDSet,
			areaIDSet,
		) {
			continue
		}
		items = append(items, records[i])
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == items[j].ID {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func (s *server) gatewayConfigAuthzVisitorPasses(
	tenantID string,
	hasBoundDoors bool,
	buildingIDSet map[string]struct{},
) []access.VisitorPass {
	if !hasBoundDoors || s.accessSvc == nil {
		return nil
	}
	items := make([]access.VisitorPass, 0)
	records := s.accessSvc.ListVisitorPasses(tenantID)
	for i := range records {
		buildingID := strings.TrimSpace(records[i].BuildingID)
		if buildingID != "" {
			if _, exists := buildingIDSet[buildingID]; !exists {
				continue
			}
		}
		items = append(items, records[i])
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == items[j].ID {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func (s *server) gatewayConfigAuthzUsers(
	tenantID string,
	hasBoundDoors bool,
	buildingIDSet, allowedUserGroupIDs map[string]struct{},
) []access.AccessUser {
	if !hasBoundDoors || s.accessSvc == nil {
		return nil
	}
	items := make([]access.AccessUser, 0)
	users := s.accessSvc.ListUsers(tenantID)
	for i := range users {
		if strings.ToLower(strings.TrimSpace(users[i].Status)) != "active" {
			continue
		}
		buildingID := strings.TrimSpace(users[i].BuildingID)
		if buildingID != "" {
			if _, exists := buildingIDSet[buildingID]; !exists {
				continue
			}
		}
		record := users[i]
		record.GroupIDs = filterAndSortUserGroupIDs(users[i].GroupIDs, allowedUserGroupIDs)
		items = append(items, record)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == items[j].ID {
			return items[i].Email < items[j].Email
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func (s *server) gatewayConfigAuthzUserGroups(
	tenantID string,
	hasBoundDoors bool,
	buildingIDSet map[string]struct{},
) []access.UserGroup {
	if !hasBoundDoors || s.accessSvc == nil {
		return nil
	}
	items := make([]access.UserGroup, 0)
	groups := s.accessSvc.ListUserGroups(tenantID)
	for i := range groups {
		buildingID := strings.TrimSpace(groups[i].BuildingID)
		if buildingID != "" {
			if _, exists := buildingIDSet[buildingID]; !exists {
				continue
			}
		}
		record := groups[i]
		record.Members = sortedUniqueTrimmedIDs(groups[i].Members)
		items = append(items, record)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ID == items[j].ID {
			return items[i].Name < items[j].Name
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func gatewayConfigAuthzScopeMatch(
	scopeType, buildingID, areaID, doorID string,
	hasBoundDoors bool,
	doorIDSet, buildingIDSet, areaIDSet map[string]struct{},
) bool {
	switch strings.ToLower(strings.TrimSpace(scopeType)) {
	case "", "all":
		return hasBoundDoors
	case "building":
		_, exists := buildingIDSet[strings.TrimSpace(buildingID)]
		return exists
	case "area":
		_, exists := areaIDSet[strings.TrimSpace(areaID)]
		return exists
	case "door":
		_, exists := doorIDSet[strings.TrimSpace(doorID)]
		return exists
	default:
		return false
	}
}

func gatewayConfigAuthzCacheVersion(cache gatewayConfigAuthzCache) string {
	payload := struct {
		TTLSeconds      int                           `json:"ttl_seconds"`
		Scope           gatewayConfigAuthzScope       `json:"scope"`
		Counts          gatewayConfigAuthzCacheCounts `json:"counts"`
		Doors           []space.Door                  `json:"doors,omitempty"`
		Policies        []access.Policy               `json:"policies,omitempty"`
		TemporaryAccess []access.TemporaryAccess      `json:"temporary_access,omitempty"`
		VisitorPasses   []access.VisitorPass          `json:"visitor_passes,omitempty"`
		Users           []access.AccessUser           `json:"users,omitempty"`
		UserGroups      []access.UserGroup            `json:"user_groups,omitempty"`
	}{
		TTLSeconds:      cache.TTLSeconds,
		Scope:           cache.Scope,
		Counts:          cache.Counts,
		Doors:           cache.Doors,
		Policies:        cache.Policies,
		TemporaryAccess: cache.TemporaryAccess,
		VisitorPasses:   cache.VisitorPasses,
		Users:           cache.Users,
		UserGroups:      cache.UserGroups,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "authz-unavailable"
	}
	sum := sha256.Sum256(raw)
	return "authz-" + hex.EncodeToString(sum[:])
}

func filterAndSortUserGroupIDs(values []string, allowed map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for i := range values {
		next := strings.TrimSpace(values[i])
		if next == "" {
			continue
		}
		if len(allowed) > 0 {
			if _, exists := allowed[next]; !exists {
				continue
			}
		}
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		items = append(items, next)
	}
	if len(items) == 0 {
		return nil
	}
	sort.Strings(items)
	return items
}

func sortedSetKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	items := make([]string, 0, len(values))
	for value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		items = append(items, value)
	}
	if len(items) == 0 {
		return nil
	}
	sort.Strings(items)
	return items
}

func sortedUniqueTrimmedIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for i := range values {
		next := strings.TrimSpace(values[i])
		if next == "" {
			continue
		}
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		items = append(items, next)
	}
	if len(items) == 0 {
		return nil
	}
	sort.Strings(items)
	return items
}

type gatewayCheckpointAuditMetrics struct {
	GatewayID    string
	Queue        string
	CheckpointID string
	AckedCount   int
	LastRequest  string
}

type gatewayCheckpointQueueTrend struct {
	ReportTotal   int
	AckedDelta    int
	Direction     string
	FirstReportAt *time.Time
	LastReportAt  *time.Time
}

type gatewayCheckpointWindowTrendSummary struct {
	ReportTotal     int
	GatewayTotal    int
	QueueTotal      int
	AckedDeltaTotal int
	Direction       string
	LastReportAt    *time.Time
}

func buildGatewayCheckpointWindowTrends(
	logs []audit.Log,
	allowedGatewayIDs map[string]struct{},
	filterGatewayID,
	filterQueue string,
) (map[string]gatewayCheckpointQueueTrend, gatewayCheckpointWindowTrendSummary) {
	type accumulator struct {
		reportTotal int
		firstAt     time.Time
		lastAt      time.Time
		firstAcked  int
		lastAcked   int
	}

	filteredGatewayID := strings.TrimSpace(filterGatewayID)
	filteredQueue := normalizeGatewayCheckpointQueue(filterQueue)
	filterByQueue := strings.TrimSpace(filterQueue) != ""
	accumulators := make(map[string]accumulator)
	gatewayIDs := make(map[string]struct{})

	for i := range logs {
		metrics := parseGatewayCheckpointAuditMetrics(logs[i].Target)
		if metrics.GatewayID == "" {
			continue
		}
		if allowedGatewayIDs != nil {
			if _, allowed := allowedGatewayIDs[metrics.GatewayID]; !allowed {
				continue
			}
		}
		if filteredGatewayID != "" && metrics.GatewayID != filteredGatewayID {
			continue
		}
		if filterByQueue && metrics.Queue != filteredQueue {
			continue
		}

		gatewayIDs[metrics.GatewayID] = struct{}{}
		key := gatewayCheckpointTrendKey(metrics.GatewayID, metrics.Queue)
		entry, exists := accumulators[key]
		if !exists {
			accumulators[key] = accumulator{
				reportTotal: 1,
				firstAt:     logs[i].At,
				lastAt:      logs[i].At,
				firstAcked:  metrics.AckedCount,
				lastAcked:   metrics.AckedCount,
			}
			continue
		}

		entry.reportTotal++
		if logs[i].At.Before(entry.firstAt) {
			entry.firstAt = logs[i].At
			entry.firstAcked = metrics.AckedCount
		}
		if logs[i].At.After(entry.lastAt) || logs[i].At.Equal(entry.lastAt) {
			entry.lastAt = logs[i].At
			entry.lastAcked = metrics.AckedCount
		}
		accumulators[key] = entry
	}

	items := make(map[string]gatewayCheckpointQueueTrend, len(accumulators))
	summary := gatewayCheckpointWindowTrendSummary{
		GatewayTotal: len(gatewayIDs),
	}
	for key, entry := range accumulators {
		ackedDelta := entry.lastAcked - entry.firstAcked
		first := entry.firstAt
		last := entry.lastAt
		items[key] = gatewayCheckpointQueueTrend{
			ReportTotal:   entry.reportTotal,
			AckedDelta:    ackedDelta,
			Direction:     checkpointTrendDirection(ackedDelta),
			FirstReportAt: &first,
			LastReportAt:  &last,
		}
		summary.ReportTotal += entry.reportTotal
		summary.AckedDeltaTotal += ackedDelta
		if summary.LastReportAt == nil || last.After(*summary.LastReportAt) {
			copyLast := last
			summary.LastReportAt = &copyLast
		}
	}
	summary.QueueTotal = len(items)
	summary.Direction = checkpointTrendDirection(summary.AckedDeltaTotal)
	return items, summary
}

func parseGatewayCheckpointAuditMetrics(rawTarget string) gatewayCheckpointAuditMetrics {
	values := parseAuditTargetKeyValues(rawTarget)
	return gatewayCheckpointAuditMetrics{
		GatewayID:    strings.TrimSpace(values["gateway"]),
		Queue:        normalizeGatewayCheckpointQueue(values["queue"]),
		CheckpointID: strings.TrimSpace(values["checkpoint"]),
		AckedCount:   parseIntOrZero(values["acked"]),
		LastRequest:  strings.TrimSpace(values["last_request"]),
	}
}

func parseAuditTargetKeyValues(rawTarget string) map[string]string {
	parts := strings.Fields(strings.TrimSpace(rawTarget))
	values := make(map[string]string, len(parts))
	for i := range parts {
		pair := strings.SplitN(parts[i], "=", 2)
		if len(pair) != 2 {
			continue
		}
		key := strings.TrimSpace(pair[0])
		value := strings.TrimSpace(pair[1])
		if key == "" || value == "" {
			continue
		}
		values[key] = value
	}
	return values
}

func gatewayCheckpointTrendKey(gatewayID, queue string) string {
	return strings.TrimSpace(gatewayID) + "|" + normalizeGatewayCheckpointQueue(queue)
}

func normalizeGatewayCheckpointQueue(queue string) string {
	next := strings.TrimSpace(queue)
	if next == "" {
		return "default"
	}
	return next
}

func checkpointTrendDirection(delta int) string {
	if delta > 0 {
		return "up"
	}
	if delta < 0 {
		return "down"
	}
	return "flat"
}

func (s *server) getAuthUserBuildingScope(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	target, err := s.authService.GetUserByID(userID)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if !s.canManageAuthUserBuildingScope(w, r, target) {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":      target.ID,
		"email":        target.Email,
		"role":         target.Role,
		"tenant_id":    target.TenantID,
		"building_ids": target.BuildingIDs,
	})
}

func (s *server) updateAuthUserBuildingScope(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	target, err := s.authService.GetUserByID(userID)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if !s.canManageAuthUserBuildingScope(w, r, target) {
		return
	}

	var request struct {
		BuildingIDs []string `json:"building_ids"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := s.authService.UpdateUserBuildingScope(userID, request.BuildingIDs)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, auth.ErrUserRoleUnsupported):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":      updated.ID,
		"email":        updated.Email,
		"role":         updated.Role,
		"tenant_id":    updated.TenantID,
		"building_ids": updated.BuildingIDs,
	})
}

func (s *server) updateAuthUserPasswordAuth(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	var request struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := s.authService.UpdateUserPasswordAuth(userID, request.Enabled)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	actor, _ := authenticatedUser(r)
	s.appendAuditLog(r, updated.TenantID, "auth_user_password_auth_updated",
		fmt.Sprintf("user_id=%s,enabled=%v,by=%s", updated.ID, request.Enabled, actor.Email), "auth")
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":               updated.ID,
		"email":                 updated.Email,
		"password_auth_enabled": updated.PasswordAuthEnabled,
	})
}

func (s *server) canManageAuthUserBuildingScope(w http.ResponseWriter, r *http.Request, target auth.User) bool {
	actor, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return false
	}

	actorRole := strings.ToLower(strings.TrimSpace(actor.Role))
	if actorRole == "super_admin" {
		return true
	}
	if actorRole != "tenant_admin" {
		writeError(w, http.StatusForbidden, "forbidden")
		return false
	}

	actorTenantID := strings.TrimSpace(actor.TenantID)
	targetTenantID := strings.TrimSpace(target.TenantID)
	if actorTenantID == "" || targetTenantID == "" || actorTenantID != targetTenantID {
		writeError(w, http.StatusForbidden, "tenant scope forbidden")
		return false
	}

	return true
}

func (s *server) issueEnterpriseTrustedSession(
	tenantID string,
	syncMode string,
	loginEmail string,
	identitySubject string,
	jitProfile enterpriseJITProvisionProfile,
) (auth.LoginResponse, bool, string, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	nextEmail := strings.TrimSpace(loginEmail)
	nextIdentitySubject := strings.TrimSpace(identitySubject)
	nextSyncMode := strings.TrimSpace(syncMode)

	if nextSyncMode != "jit" {
		response, err := s.authService.LoginByTrustedIdentity(nextEmail)
		if err != nil {
			return auth.LoginResponse{}, false, nextIdentitySubject, err
		}
		return response, false, nextIdentitySubject, nil
	}
	if enterpriseEmploymentStatusBlocksSession(jitProfile.EmploymentStatus) {
		return auth.LoginResponse{}, false, nextIdentitySubject, enterprise.ErrEmployeeInactive
	}
	if s.cfg.EnterpriseJITProvisionApprovalRequired {
		matched, matchErr := s.enterpriseSvc.HasActiveJITEmployeeIdentity(
			nextTenantID,
			nextEmail,
			nextIdentitySubject,
		)
		if matchErr != nil {
			return auth.LoginResponse{}, false, nextIdentitySubject, matchErr
		}
		if !matched {
			approved, approvalErr := s.enterpriseSvc.HasApprovedJITProvisionApproval(
				nextTenantID,
				nextEmail,
				nextIdentitySubject,
			)
			if approvalErr != nil {
				return auth.LoginResponse{}, false, nextIdentitySubject, approvalErr
			}
			if !approved {
				return auth.LoginResponse{}, false, nextIdentitySubject, enterprise.ErrJITProvisionApprovalRequired
			}
		}
	}

	employee, employeeCreated, employeeErr := s.enterpriseSvc.ResolveOrProvisionJITEmployeeWithProfile(
		nextTenantID,
		nextEmail,
		nextIdentitySubject,
		enterprise.JITProvisionProfile{
			FullName:          jitProfile.FullName,
			Department:        jitProfile.Department,
			JobTitle:          jitProfile.JobTitle,
			Location:          jitProfile.Location,
			Phone:             jitProfile.Phone,
			ManagerExternalID: jitProfile.ManagerExternalID,
			EmploymentStatus:  jitProfile.EmploymentStatus,
		},
	)
	if employeeErr != nil {
		return auth.LoginResponse{}, false, nextIdentitySubject, employeeErr
	}
	if nextIdentitySubject == "" {
		nextIdentitySubject = strings.TrimSpace(employee.ExternalID)
	}

	userExisted := false
	_, probeErr := s.authService.LoginByTrustedIdentity(nextEmail)
	if probeErr == nil {
		userExisted = true
	} else if !errors.Is(probeErr, auth.ErrInvalidCredentials) {
		return auth.LoginResponse{}, false, nextIdentitySubject, probeErr
	}

	jitUser := enterpriseJITTrustedUser(nextTenantID, employee, nextEmail, nextIdentitySubject)
	response, err := s.authService.LoginByTrustedUser(jitUser)
	if err != nil {
		return auth.LoginResponse{}, false, nextIdentitySubject, err
	}
	jitApplied := employeeCreated || !userExisted
	return response, jitApplied, nextIdentitySubject, nil
}

func enterpriseTrustedSessionErrorStatusCode(err error) int {
	switch {
	case errors.Is(err, enterprise.ErrEmployeeInactive):
		return http.StatusForbidden
	case errors.Is(err, enterprise.ErrJITProvisionApprovalRequired):
		return http.StatusForbidden
	case errors.Is(err, enterprise.ErrEmployeeExternalIDConflict):
		return http.StatusConflict
	case errors.Is(err, enterprise.ErrEmailRequired), errors.Is(err, enterprise.ErrInvalidDomain):
		return http.StatusBadRequest
	case errors.Is(err, auth.ErrInvalidCredentials):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func (s *server) applyEnterpriseJITDeprovisionOnInactive(
	r *http.Request,
	tenantID, provider, email, externalID, employmentStatus string,
	err error,
) {
	if !errors.Is(err, enterprise.ErrEmployeeInactive) {
		return
	}

	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return
	}

	revokedRefreshCount := 0
	downgradedLocal := false
	beforeRole := ""
	afterRole := ""
	beforeBuildingScopeCount := 0
	afterBuildingScopeCount := 0
	if s.authService != nil {
		revokedRefreshCount = s.authService.RevokeRefreshTokensByUserEmail(email)
		beforeUser, afterUser, applied := s.authService.DowngradeTrustedUserToLeastPrivilegeByEmail(email, nextTenantID)
		downgradedLocal = applied
		beforeRole = strings.TrimSpace(beforeUser.Role)
		afterRole = strings.TrimSpace(afterUser.Role)
		beforeBuildingScopeCount = len(beforeUser.BuildingIDs)
		afterBuildingScopeCount = len(afterUser.BuildingIDs)
	}

	target := strings.TrimSpace(
		fmt.Sprintf(
			"provider=%s,email=%s,external_id=%s,employment_status=%s,revoked_refresh=%d,downgraded_local=%t,old_role=%s,new_role=%s,old_building_scope=%d,new_building_scope=%d",
			strings.TrimSpace(provider),
			strings.ToLower(strings.TrimSpace(email)),
			strings.TrimSpace(externalID),
			normalizeEmploymentStatus(employmentStatus),
			revokedRefreshCount,
			downgradedLocal,
			beforeRole,
			afterRole,
			beforeBuildingScopeCount,
			afterBuildingScopeCount,
		),
	)
	if target == "" {
		target = "enterprise_jit_deprovision_applied"
	}
	s.appendAuditLog(r, nextTenantID, "enterprise_jit_deprovision_applied", target, "enterprise_auth")
}

func (s *server) applyEnterpriseJITApprovalRequiredAudit(
	r *http.Request,
	tenantID, provider, email, externalID string,
	employmentStatus string,
	err error,
) {
	if !errors.Is(err, enterprise.ErrJITProvisionApprovalRequired) {
		return
	}

	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return
	}

	approvalID := ""
	if s.enterpriseSvc != nil {
		approval, approvalErr := s.enterpriseSvc.UpsertJITProvisionApprovalRequest(
			nextTenantID,
			email,
			externalID,
			provider,
			employmentStatus,
		)
		if approvalErr == nil {
			approvalID = strings.TrimSpace(approval.ID)
		}
	}

	target := strings.TrimSpace(
		fmt.Sprintf(
			"provider=%s,email=%s,external_id=%s,employment_status=%s,approval_id=%s,reason=jit_auto_provision_requires_approval",
			strings.TrimSpace(provider),
			strings.ToLower(strings.TrimSpace(email)),
			strings.TrimSpace(externalID),
			normalizeEmploymentStatus(employmentStatus),
			approvalID,
		),
	)
	if target == "" {
		target = "enterprise_jit_approval_required"
	}
	s.appendAuditLog(r, nextTenantID, "enterprise_jit_approval_required", target, "enterprise_auth")
}

type enterpriseJITProvisionProfile struct {
	FullName          string
	Department        string
	JobTitle          string
	Location          string
	Phone             string
	ManagerExternalID string
	EmploymentStatus  string
}

func enterpriseJITProfileFromOIDCIdentity(identity enterprise.OIDCIdentity) enterpriseJITProvisionProfile {
	return enterpriseJITProvisionProfile{
		FullName:          strings.TrimSpace(identity.Name),
		Department:        strings.TrimSpace(identity.Department),
		JobTitle:          strings.TrimSpace(identity.JobTitle),
		Location:          strings.TrimSpace(identity.Location),
		Phone:             strings.TrimSpace(identity.Phone),
		ManagerExternalID: strings.TrimSpace(identity.ManagerExternalID),
		EmploymentStatus:  normalizeEmploymentStatus(identity.EmploymentStatus),
	}
}

func enterpriseJITProfileFromSAMLIdentity(identity enterprise.SAMLIdentity) enterpriseJITProvisionProfile {
	return enterpriseJITProvisionProfile{
		FullName: samlAttributeFirst(identity.Attributes,
			"name",
			"displayname",
			"display_name",
			"full_name",
			"fullname",
			"cn",
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
		),
		Department: samlAttributeFirst(identity.Attributes,
			"department",
			"dept",
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/department",
		),
		JobTitle: samlAttributeFirst(identity.Attributes,
			"job_title",
			"title",
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/title",
		),
		Location: samlAttributeFirst(identity.Attributes,
			"location",
			"city",
			"office",
			"physicaldeliveryofficename",
		),
		Phone: samlAttributeFirst(identity.Attributes,
			"phone_number",
			"phone",
			"mobile",
			"telephone",
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/mobilephone",
		),
		ManagerExternalID: samlAttributeFirst(identity.Attributes,
			"manager_external_id",
			"manager_id",
			"manager",
		),
		EmploymentStatus: enterpriseJITEmploymentStatusFromSAML(identity),
	}
}

func enterpriseJITEmploymentStatusFromSAML(identity enterprise.SAMLIdentity) string {
	status := samlAttributeFirst(identity.Attributes,
		"employment_status",
		"employmentstatus",
		"status",
	)
	if status != "" {
		return normalizeEmploymentStatus(status)
	}

	activeFlag := samlAttributeFirst(identity.Attributes, "active")
	if activeFlag == "" {
		return ""
	}
	return normalizeEmploymentStatus(activeFlag)
}

func normalizeEmploymentStatus(raw string) string {
	return enterprise.NormalizeEmploymentStatus(raw)
}

func enterpriseEmploymentStatusBlocksSession(status string) bool {
	return enterprise.EmploymentStatusBlocksSession(status)
}

func samlAttributeFirst(attributes map[string][]string, keys ...string) string {
	if len(attributes) == 0 {
		return ""
	}
	for i := range keys {
		key := strings.ToLower(strings.TrimSpace(keys[i]))
		if key == "" {
			continue
		}
		values := attributes[key]
		for j := range values {
			value := strings.TrimSpace(values[j])
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func buildEnterpriseOIDCAuthorizeURL(config enterprise.IDPConfig, stateToken, redirectURI string) (string, error) {
	baseURL := strings.TrimSpace(config.AuthURL)
	if baseURL == "" {
		baseURL = strings.TrimSuffix(strings.TrimSpace(config.IssuerURL), "/") + "/oauth2/auth"
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid oidc auth_url: %s", baseURL)
	}

	scopes := config.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email"}
	}
	query := parsed.Query()
	query.Set("client_id", config.ClientID)
	query.Set("response_type", "code")
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(scopes, " "))
	query.Set("state", stateToken)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func enterpriseOIDCTokenURL(config enterprise.IDPConfig) (string, error) {
	baseURL := strings.TrimSpace(config.TokenURL)
	if baseURL == "" {
		baseURL = strings.TrimSuffix(strings.TrimSpace(config.IssuerURL), "/") + "/oauth2/token"
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid oidc token_url: %s", baseURL)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid oidc token_url: %s", baseURL)
	}
	return parsed.String(), nil
}

func exchangeEnterpriseOIDCCodeForIDToken(
	ctx context.Context,
	config enterprise.IDPConfig,
	code string,
	redirectURI string,
) (string, int, error) {
	return exchangeEnterpriseOIDCCodeForIDTokenWithClient(
		ctx,
		&http.Client{Timeout: 10 * time.Second},
		config,
		code,
		redirectURI,
	)
}

func exchangeEnterpriseOIDCCodeForIDTokenWithClient(
	ctx context.Context,
	client *http.Client,
	config enterprise.IDPConfig,
	code string,
	redirectURI string,
) (string, int, error) {
	nextCode := strings.TrimSpace(code)
	if nextCode == "" {
		return "", http.StatusBadRequest, errors.New("oidc authorization code is required")
	}
	nextClientID := strings.TrimSpace(config.ClientID)
	if nextClientID == "" {
		return "", http.StatusBadRequest, errors.New("oidc client_id is required")
	}

	tokenURL, err := enterpriseOIDCTokenURL(config)
	if err != nil {
		return "", http.StatusBadRequest, err
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", nextCode)
	form.Set("client_id", nextClientID)
	nextRedirectURI := strings.TrimSpace(redirectURI)
	if nextRedirectURI != "" {
		form.Set("redirect_uri", nextRedirectURI)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("failed to build oidc token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", http.StatusBadGateway, fmt.Errorf("oidc token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", http.StatusBadGateway, fmt.Errorf("failed to read oidc token response: %w", err)
	}

	var tokenPayload struct {
		IDToken          string `json:"id_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(bodyBytes, &tokenPayload)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(tokenPayload.Error)
		if message == "" {
			message = strings.TrimSpace(string(bodyBytes))
		}
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		if detail := strings.TrimSpace(tokenPayload.ErrorDescription); detail != "" && !strings.Contains(message, detail) {
			message = message + ": " + detail
		}
		return "", http.StatusUnauthorized, fmt.Errorf("oidc code exchange failed: %s", message)
	}

	idToken := strings.TrimSpace(tokenPayload.IDToken)
	if idToken == "" {
		return "", http.StatusBadGateway, errors.New("oidc token response missing id_token")
	}
	return idToken, http.StatusOK, nil
}

func buildEnterpriseSAMLSSOURL(config enterprise.IDPConfig, stateToken, redirectURI string) (string, error) {
	baseURL := strings.TrimSpace(config.AuthURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(config.IssuerURL)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid saml sso url: %s", baseURL)
	}

	query := parsed.Query()
	query.Set("RelayState", stateToken)
	query.Set("redirect_uri", redirectURI)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func enterpriseJITTrustedUser(
	tenantID string,
	employee enterprise.EnterpriseEmployee,
	fallbackEmail string,
	identitySubject string,
) auth.User {
	nextTenantID := strings.TrimSpace(tenantID)
	nextEmail := strings.TrimSpace(employee.Email)
	if nextEmail == "" {
		nextEmail = strings.TrimSpace(fallbackEmail)
	}

	nextRole := strings.ToLower(strings.TrimSpace(employee.AccessRole))
	if nextRole == "" {
		nextRole = "resident"
	}

	var buildingIDs []string
	buildingID := strings.TrimSpace(employee.BuildingID)
	if buildingID != "" {
		buildingIDs = []string{buildingID}
	}

	return auth.User{
		ID: enterpriseJITUserID(
			nextTenantID,
			nextEmail,
			strings.TrimSpace(employee.ExternalID),
			identitySubject,
		),
		Email:       nextEmail,
		Role:        nextRole,
		TenantID:    nextTenantID,
		BuildingIDs: buildingIDs,
	}
}

func enterpriseJITUserID(tenantID, email, externalID, identitySubject string) string {
	nextTenantID := strings.TrimSpace(tenantID)
	nextEmail := strings.ToLower(strings.TrimSpace(email))
	nextExternalID := strings.TrimSpace(externalID)
	nextIdentitySubject := strings.TrimSpace(identitySubject)
	if nextExternalID == "" {
		nextExternalID = nextIdentitySubject
	}
	if nextExternalID == "" {
		nextExternalID = nextEmail
	}

	seed := nextTenantID + "|" + nextEmail + "|" + nextExternalID
	digest := sha256.Sum256([]byte(seed))
	return "usr_ent_jit_" + hex.EncodeToString(digest[:8])
}

type enterpriseSyncWorkerAlertItem struct {
	ID                    string    `json:"id"`
	TenantID              string    `json:"tenant_id"`
	Actor                 string    `json:"actor"`
	Role                  string    `json:"role"`
	Action                string    `json:"action"`
	WorkerAction          string    `json:"worker_action"`
	WorkerKind            string    `json:"worker_kind"`
	WorkerLabel           string    `json:"worker_label"`
	Source                string    `json:"source"`
	At                    time.Time `json:"at"`
	Failed                int       `json:"failed"`
	Threshold             int       `json:"threshold"`
	Processed             int       `json:"processed"`
	Applied               int       `json:"applied"`
	ConsecutiveFailures   int       `json:"consecutive_failures,omitempty"`
	FailureAgeSeconds     int       `json:"failure_age_seconds,omitempty"`
	SkippedByAttemptLimit int       `json:"skipped_by_attempt_limit"`
	SkippedByCooldown     int       `json:"skipped_by_cooldown"`
	ConnectorID           string    `json:"connector_id,omitempty"`
	Vendor                string    `json:"vendor,omitempty"`
	EventType             string    `json:"event_type,omitempty"`
	RequestID             string    `json:"request_id,omitempty"`
	FailureStage          string    `json:"failure_stage,omitempty"`
	Mode                  string    `json:"mode,omitempty"`
	RawTarget             string    `json:"raw_target"`
}

type enterpriseSyncWorkerAlertSummaryItem struct {
	TenantID                  string    `json:"tenant_id"`
	WorkerAction              string    `json:"worker_action"`
	WorkerKind                string    `json:"worker_kind"`
	WorkerLabel               string    `json:"worker_label"`
	Count                     int       `json:"count"`
	FirstSeenAt               time.Time `json:"first_seen_at"`
	LastSeenAt                time.Time `json:"last_seen_at"`
	LastFailed                int       `json:"last_failed"`
	LastThreshold             int       `json:"last_threshold"`
	LastProcessed             int       `json:"last_processed"`
	LastApplied               int       `json:"last_applied"`
	LastConsecutiveFailures   int       `json:"last_consecutive_failures,omitempty"`
	LastFailureAgeSeconds     int       `json:"last_failure_age_seconds,omitempty"`
	LastSkippedByAttemptLimit int       `json:"last_skipped_by_attempt_limit"`
	LastSkippedByCooldown     int       `json:"last_skipped_by_cooldown"`
}

func buildEnterpriseSyncWorkerAlerts(logs []audit.Log) []enterpriseSyncWorkerAlertItem {
	items := make([]enterpriseSyncWorkerAlertItem, 0, len(logs))
	for i := range logs {
		metrics := parseEnterpriseSyncWorkerAlertMetrics(logs[i].Target)
		action := strings.TrimSpace(logs[i].Action)
		items = append(items, enterpriseSyncWorkerAlertItem{
			ID:                    logs[i].ID,
			TenantID:              logs[i].TenantID,
			Actor:                 logs[i].Actor,
			Role:                  logs[i].Role,
			Action:                action,
			WorkerAction:          action,
			WorkerKind:            enterpriseSyncWorkerAlertKind(action),
			WorkerLabel:           enterpriseSyncWorkerAlertLabel(action),
			Source:                logs[i].Source,
			At:                    logs[i].At,
			Failed:                metrics.Failed,
			Threshold:             metrics.Threshold,
			Processed:             metrics.Processed,
			Applied:               metrics.Applied,
			ConsecutiveFailures:   metrics.ConsecutiveFailures,
			FailureAgeSeconds:     metrics.FailureAgeSeconds,
			SkippedByAttemptLimit: metrics.SkippedByAttemptLimit,
			SkippedByCooldown:     metrics.SkippedByCooldown,
			ConnectorID:           metrics.ConnectorID,
			Vendor:                metrics.Vendor,
			EventType:             metrics.EventType,
			RequestID:             metrics.RequestID,
			FailureStage:          metrics.FailureStage,
			Mode:                  metrics.Mode,
			RawTarget:             strings.TrimSpace(logs[i].Target),
		})
	}
	return items
}

func buildEnterpriseSyncWorkerAlertSummary(logs []audit.Log) []enterpriseSyncWorkerAlertSummaryItem {
	summaries := make(map[string]enterpriseSyncWorkerAlertSummaryItem)
	for i := range logs {
		tenantID := strings.TrimSpace(logs[i].TenantID)
		if tenantID == "" {
			continue
		}
		action := strings.TrimSpace(logs[i].Action)
		summaryKey := tenantID + "|" + action
		entry, exists := summaries[summaryKey]
		if !exists {
			entry = enterpriseSyncWorkerAlertSummaryItem{
				TenantID:     tenantID,
				WorkerAction: action,
				WorkerKind:   enterpriseSyncWorkerAlertKind(action),
				WorkerLabel:  enterpriseSyncWorkerAlertLabel(action),
				FirstSeenAt:  logs[i].At,
				LastSeenAt:   logs[i].At,
			}
		}
		entry.Count++
		if logs[i].At.Before(entry.FirstSeenAt) {
			entry.FirstSeenAt = logs[i].At
		}
		if !exists || logs[i].At.After(entry.LastSeenAt) {
			metrics := parseEnterpriseSyncWorkerAlertMetrics(logs[i].Target)
			entry.LastSeenAt = logs[i].At
			entry.LastFailed = metrics.Failed
			entry.LastThreshold = metrics.Threshold
			entry.LastProcessed = metrics.Processed
			entry.LastApplied = metrics.Applied
			entry.LastConsecutiveFailures = metrics.ConsecutiveFailures
			entry.LastFailureAgeSeconds = metrics.FailureAgeSeconds
			entry.LastSkippedByAttemptLimit = metrics.SkippedByAttemptLimit
			entry.LastSkippedByCooldown = metrics.SkippedByCooldown
		}
		summaries[summaryKey] = entry
	}

	items := make([]enterpriseSyncWorkerAlertSummaryItem, 0, len(summaries))
	for _, item := range summaries {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].LastSeenAt.Equal(items[j].LastSeenAt) {
			if items[i].TenantID != items[j].TenantID {
				return items[i].TenantID < items[j].TenantID
			}
			return items[i].WorkerAction < items[j].WorkerAction
		}
		return items[i].LastSeenAt.After(items[j].LastSeenAt)
	})
	return items
}

type enterpriseSyncWorkerAlertMetrics struct {
	Failed                int
	Threshold             int
	Processed             int
	Applied               int
	ConsecutiveFailures   int
	FailureAgeSeconds     int
	SkippedByAttemptLimit int
	SkippedByCooldown     int
	ConnectorID           string
	Vendor                string
	EventType             string
	RequestID             string
	FailureStage          string
	Mode                  string
}

func parseEnterpriseSyncWorkerAlertMetrics(rawTarget string) enterpriseSyncWorkerAlertMetrics {
	parts := strings.Fields(strings.TrimSpace(rawTarget))
	if len(parts) == 0 {
		return enterpriseSyncWorkerAlertMetrics{}
	}

	values := make(map[string]string, len(parts))
	for i := range parts {
		pair := strings.SplitN(parts[i], "=", 2)
		if len(pair) != 2 {
			continue
		}
		key := strings.TrimSpace(pair[0])
		value := strings.TrimSpace(pair[1])
		if key == "" || value == "" {
			continue
		}
		values[key] = value
	}

	return enterpriseSyncWorkerAlertMetrics{
		Failed:                parseIntOrZero(values["failed"]),
		Threshold:             parseIntOrZero(values["threshold"]),
		Processed:             parseIntOrZero(values["processed"]),
		Applied:               parseEnterpriseSyncWorkerAlertMetricWithFallback(values, "applied", "replayed", "synced"),
		ConsecutiveFailures:   parseIntOrZero(values["consecutive_failures"]),
		FailureAgeSeconds:     parseIntOrZero(values["failure_age_seconds"]),
		SkippedByAttemptLimit: parseIntOrZero(values["skipped_attempt_limit"]),
		SkippedByCooldown:     parseIntOrZero(values["skipped_cooldown"]),
		ConnectorID:           strings.TrimSpace(values["connector_id"]),
		Vendor:                strings.TrimSpace(values["vendor"]),
		EventType:             strings.TrimSpace(values["event_type"]),
		RequestID:             strings.TrimSpace(values["request_id"]),
		FailureStage:          strings.TrimSpace(values["failure_stage"]),
		Mode:                  strings.TrimSpace(values["mode"]),
	}
}

func parseEnterpriseSyncWorkerAlertMetricWithFallback(values map[string]string, keys ...string) int {
	for i := range keys {
		value, ok := values[keys[i]]
		if !ok {
			continue
		}
		return parseIntOrZero(value)
	}
	return 0
}

func enterpriseSyncWorkerAlertActions() []string {
	return []string{
		"enterprise_sync_reconcile_worker_alert",
		"enterprise_hris_webhook_receipt_worker_alert",
		"enterprise_hris_webhook_dlq_worker_alert",
		"enterprise_hris_pull_worker_alert",
		"enterprise_hris_webhook_processing_alert",
	}
}

func enterpriseSyncWorkerAlertKind(action string) string {
	switch strings.TrimSpace(action) {
	case "enterprise_sync_reconcile_worker_alert":
		return "sync_reconcile"
	case "enterprise_hris_webhook_receipt_worker_alert":
		return "hris_webhook_receipt_queue"
	case "enterprise_hris_webhook_dlq_worker_alert":
		return "hris_webhook_dlq"
	case "enterprise_hris_pull_worker_alert":
		return "hris_pull"
	case "enterprise_hris_webhook_processing_alert":
		return "hris_webhook_processing"
	default:
		return "unknown"
	}
}

func enterpriseSyncWorkerAlertLabel(action string) string {
	switch strings.TrimSpace(action) {
	case "enterprise_sync_reconcile_worker_alert":
		return "Enterprise Sync Reconcile"
	case "enterprise_hris_webhook_receipt_worker_alert":
		return "HRIS Webhook Receipt Queue"
	case "enterprise_hris_webhook_dlq_worker_alert":
		return "HRIS Webhook DLQ"
	case "enterprise_hris_pull_worker_alert":
		return "HRIS Pull Reconcile"
	case "enterprise_hris_webhook_processing_alert":
		return "HRIS Webhook Processing"
	default:
		return strings.TrimSpace(action)
	}
}

func parseIntOrZero(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}

func parseRFC3339TimeRange(rawSince, rawUntil string) (*time.Time, *time.Time, error) {
	sinceInput := strings.TrimSpace(rawSince)
	untilInput := strings.TrimSpace(rawUntil)

	var since *time.Time
	var until *time.Time
	if sinceInput != "" {
		parsed, err := time.Parse(time.RFC3339, sinceInput)
		if err != nil {
			return nil, nil, errors.New("since must be RFC3339 timestamp")
		}
		parsedUTC := parsed.UTC()
		since = &parsedUTC
	}
	if untilInput != "" {
		parsed, err := time.Parse(time.RFC3339, untilInput)
		if err != nil {
			return nil, nil, errors.New("until must be RFC3339 timestamp")
		}
		parsedUTC := parsed.UTC()
		until = &parsedUTC
	}
	if since != nil && until != nil && since.After(*until) {
		return nil, nil, errors.New("since must be <= until")
	}
	return since, until, nil
}

func filterAuditLogsByTimeRange(logs []audit.Log, since, until *time.Time) []audit.Log {
	if since == nil && until == nil {
		return logs
	}
	items := make([]audit.Log, 0, len(logs))
	for i := range logs {
		at := logs[i].At
		if since != nil && at.Before(*since) {
			continue
		}
		if until != nil && at.After(*until) {
			continue
		}
		items = append(items, logs[i])
	}
	return items
}

const enterpriseAccessSyncSource = "enterprise_employee_sync"

func enterpriseEmployeesToAccessBatchInputs(items []enterprise.EnterpriseEmployee) []access.BatchUpsertUserByEmailInput {
	accessInputs := make([]access.BatchUpsertUserByEmailInput, 0, len(items))
	for i := range items {
		employee := items[i]
		accessInputs = append(accessInputs, access.BatchUpsertUserByEmailInput{
			BuildingID: employee.BuildingID,
			Name:       employee.FullName,
			Email:      employee.Email,
			Role:       employee.AccessRole,
			Status:     employee.Status,
			GroupIDs:   employee.GroupIDs,
			SyncSource: enterpriseAccessSyncSource,
			SyncRef:    enterpriseEmployeeAccessSyncRef(employee),
		})
	}
	return accessInputs
}

func enterpriseEmployeeAccessSyncRef(employee enterprise.EnterpriseEmployee) string {
	externalID := strings.TrimSpace(employee.ExternalID)
	if externalID != "" {
		return "external_id:" + externalID
	}
	email := strings.ToLower(strings.TrimSpace(employee.Email))
	if email != "" {
		return "email:" + email
	}
	return ""
}

func (s *server) startEnterpriseSyncReconcileWorker() {
	if !s.cfg.EnterpriseSyncReconcileWorkerEnabled {
		return
	}
	interval := s.cfg.EnterpriseSyncReconcileWorkerInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	batchSize := s.cfg.EnterpriseSyncReconcileWorkerBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	maxAttempts := s.cfg.EnterpriseSyncReconcileWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown := s.cfg.EnterpriseSyncReconcileWorkerRetryCooldown
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	alertFailureThreshold := s.cfg.EnterpriseSyncReconcileWorkerAlertFailureThreshold
	if alertFailureThreshold <= 0 {
		alertFailureThreshold = 1
	}
	forceError := s.cfg.EnterpriseSyncReconcileWorkerForceError
	forceErrorTenantID := strings.TrimSpace(s.cfg.EnterpriseSyncReconcileWorkerForceErrorTenantID)

	s.loggerOrDefault().Info(
		"enterprise sync reconcile worker enabled",
		"interval", interval,
		"batch_size", batchSize,
		"max_attempts", maxAttempts,
		"retry_cooldown", retryCooldown,
		"alert_threshold", alertFailureThreshold,
		"force_error", forceError,
		"force_error_tenant_id", forceErrorTenantID,
	)

	go func() {
		s.runEnterpriseSyncReconcileWorkerTick(
			batchSize,
			maxAttempts,
			retryCooldown,
			alertFailureThreshold,
			forceError,
			forceErrorTenantID,
		)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.runEnterpriseSyncReconcileWorkerTick(
				batchSize,
				maxAttempts,
				retryCooldown,
				alertFailureThreshold,
				forceError,
				forceErrorTenantID,
			)
		}
	}()
}

func (s *server) startEnterpriseSyncWorkerAlertAutoRetryWorker() {
	if !s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerEnabled {
		return
	}
	interval := s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	batchSize := s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBatchSize
	if batchSize <= 0 {
		batchSize = 20
	}
	maxAttempts := s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	baseBackoff := s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerBaseBackoff
	if baseBackoff <= 0 {
		baseBackoff = 5 * time.Minute
	}
	maxBackoff := s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerMaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = time.Hour
	}
	if maxBackoff < baseBackoff {
		maxBackoff = baseBackoff
	}
	lockTTL := s.cfg.EnterpriseSyncWorkerAlertAutoRetryWorkerLockTTL
	if lockTTL <= 0 {
		lockTTL = 10 * time.Minute
	}

	s.loggerOrDefault().Info(
		"enterprise sync worker alert auto retry worker enabled",
		"interval", interval,
		"batch_size", batchSize,
		"max_attempts", maxAttempts,
		"base_backoff", baseBackoff,
		"max_backoff", maxBackoff,
		"lock_ttl", lockTTL,
		"lease_enabled", s.workerLeaseStore != nil,
	)
	if s.workerLeaseStore == nil {
		s.loggerOrDefault().Warn(
			"enterprise sync worker alert auto retry worker running without redis lease; duplicate retries remain possible in multi-instance deployments",
		)
	}

	go func() {
		s.runEnterpriseSyncWorkerAlertAutoRetryWorkerTickWithLease(
			batchSize,
			maxAttempts,
			baseBackoff,
			maxBackoff,
			lockTTL,
		)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.runEnterpriseSyncWorkerAlertAutoRetryWorkerTickWithLease(
				batchSize,
				maxAttempts,
				baseBackoff,
				maxBackoff,
				lockTTL,
			)
		}
	}()
}

type enterpriseJITApprovalExternalSyncWorkerResult struct {
	Processed             int
	Synced                int
	Failed                int
	SkippedByAttemptLimit int
	SkippedByCooldown     int
}

type enterpriseSyncWorkerAlertAutoRetryWorkerResult struct {
	Processed  int
	Retried    int
	Failed     int
	Skipped    int
	Suppressed int
}

type enterpriseHRISWebhookReceiptWorkerResult struct {
	Processed             int
	Synced                int
	Skipped               int
	Failed                int
	SkippedByInFlight     int
	SkippedByAttemptLimit int
	SkippedByCooldown     int
	LastConnectorID       string
	LastVendor            string
	LastRequestID         string
	LastEventType         string
}

type enterpriseHRISWebhookDLQWorkerResult struct {
	Processed             int
	Replayed              int
	Failed                int
	SkippedByInFlight     int
	SkippedByAttemptLimit int
	SkippedByCooldown     int
	LastConnectorID       string
	LastVendor            string
	LastRequestID         string
	LastEventType         string
	LastFailureStage      string
}

type enterpriseHRISPullWorkerResult struct {
	Processed             int
	Synced                int
	Failed                int
	ConsecutiveFailures   int
	FailureAgeSeconds     int
	SkippedByInFlight     int
	SkippedByAttemptLimit int
	SkippedByCooldown     int
	LastConnectorID       string
	LastVendor            string
	LastMode              string
}

type enterpriseHRISWebhookQueuedExecution struct {
	Execution  enterprise.HRISWebhookExecution
	QueueName  string
	QueueClaim *redistore.WorkerQueueClaim
}

func (s *server) listQueuedEnterpriseHRISWebhookExecutions(
	kind string,
	batchSize int,
	processingTimeout time.Duration,
	now time.Time,
) []enterpriseHRISWebhookQueuedExecution {
	if s == nil || s.enterpriseSvc == nil {
		return nil
	}
	if batchSize <= 0 {
		batchSize = 1
	}
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	items := make([]enterpriseHRISWebhookQueuedExecution, 0, batchSize)
	seen := make(map[string]struct{}, batchSize)
	if s.workerQueueStore != nil {
		queueName := enterpriseHRISWebhookExecutionQueueName(kind)
		if queueName != "" {
			claims, err := s.workerQueueStore.ClaimWorkerQueueBatch(queueName, batchSize, processingTimeout)
			if err != nil {
				s.loggerOrDefault().Error(
					"enterprise hris webhook execution queue claim failed",
					"kind", kind,
					"queue", queueName,
					"batch_size", batchSize,
					"visibility_timeout", processingTimeout,
					"err", err,
				)
			} else {
				for i := range claims {
					item, ok := s.getQueuedEnterpriseHRISWebhookExecutionCandidate(kind, claims[i].ItemID)
					if !ok {
						s.acknowledgeEnterpriseHRISWebhookExecutionQueueClaim(
							queueName,
							kind,
							claims[i],
						)
						continue
					}
					if _, exists := seen[item.ID]; exists {
						s.acknowledgeEnterpriseHRISWebhookExecutionQueueClaim(
							queueName,
							kind,
							claims[i],
						)
						continue
					}
					seen[item.ID] = struct{}{}
					claim := claims[i]
					items = append(items, enterpriseHRISWebhookQueuedExecution{
						Execution:  item,
						QueueName:  queueName,
						QueueClaim: &claim,
					})
					if len(items) >= batchSize {
						return items
					}
				}
			}
		}
	}

	fallbackItems := s.enterpriseSvc.ListIndexedClaimableHRISWebhookExecutions(
		kind,
		processingTimeout,
		now,
		batchSize,
	)
	fallbackItems = s.filterIndexedEnterpriseHRISWebhookExecutionFallbackItems(kind, fallbackItems)
	for i := range fallbackItems {
		if _, exists := seen[fallbackItems[i].ID]; exists {
			continue
		}
		seen[fallbackItems[i].ID] = struct{}{}
		items = append(items, enterpriseHRISWebhookQueuedExecution{Execution: fallbackItems[i]})
		if len(items) >= batchSize {
			break
		}
	}
	return items
}

func (s *server) filterIndexedEnterpriseHRISWebhookExecutionFallbackItems(
	kind string,
	items []enterprise.HRISWebhookExecution,
) []enterprise.HRISWebhookExecution {
	if s == nil || s.workerQueueStore == nil || len(items) == 0 {
		return items
	}
	queueName := enterpriseHRISWebhookExecutionQueueName(kind)
	if queueName == "" {
		return items
	}

	queuedIDs := make([]string, 0, len(items))
	for i := range items {
		if strings.TrimSpace(items[i].Status) != enterprise.HRISWebhookExecutionStatusQueued {
			continue
		}
		queuedIDs = append(queuedIDs, items[i].ID)
	}
	if len(queuedIDs) == 0 {
		return items
	}

	telemetry, err := s.workerQueueStore.DescribeWorkerQueue(queueName, queuedIDs)
	if err != nil {
		s.loggerOrDefault().Warn(
			"describe indexed enterprise hris webhook execution fallback items failed",
			"kind", kind,
			"queue", queueName,
			"err", err,
		)
		return items
	}

	filtered := make([]enterprise.HRISWebhookExecution, 0, len(items))
	for i := range items {
		item := items[i]
		if strings.TrimSpace(item.Status) != enterprise.HRISWebhookExecutionStatusQueued {
			filtered = append(filtered, item)
			continue
		}
		state, ok := telemetry.Items[item.ID]
		if !ok || strings.TrimSpace(state.State) == "" || strings.TrimSpace(state.State) == redistore.WorkerQueueStateMissing {
			filtered = append(filtered, item)
			continue
		}
	}
	return filtered
}

func enterpriseHRISWebhookExecutionQueueName(kind string) string {
	switch strings.TrimSpace(kind) {
	case enterprise.HRISWebhookExecutionKindReceiptProcess:
		return enterpriseHRISWebhookReceiptExecutionQueue
	case enterprise.HRISWebhookExecutionKindDLQReplay:
		return enterpriseHRISWebhookDLQExecutionQueue
	default:
		return ""
	}
}

func (s *server) getQueuedEnterpriseHRISWebhookExecutionCandidate(
	kind string,
	executionID string,
) (enterprise.HRISWebhookExecution, bool) {
	item, ok := s.lookupQueuedEnterpriseHRISWebhookExecutionCandidate(kind, executionID)
	if ok {
		return item, true
	}
	if s == nil || s.enterpriseSvc == nil {
		return enterprise.HRISWebhookExecution{}, false
	}
	if err := s.enterpriseSvc.RefreshCoreState(); err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook execution shared state refresh after external queue miss failed",
			"kind", kind,
			"execution_id", executionID,
			"err", err,
		)
		return enterprise.HRISWebhookExecution{}, false
	}
	return s.lookupQueuedEnterpriseHRISWebhookExecutionCandidate(kind, executionID)
}

func (s *server) lookupQueuedEnterpriseHRISWebhookExecutionCandidate(
	kind string,
	executionID string,
) (enterprise.HRISWebhookExecution, bool) {
	if s == nil || s.enterpriseSvc == nil {
		return enterprise.HRISWebhookExecution{}, false
	}
	item, err := s.enterpriseSvc.GetHRISWebhookExecutionByID(executionID)
	if err != nil {
		return enterprise.HRISWebhookExecution{}, false
	}
	if strings.TrimSpace(item.Kind) != strings.TrimSpace(kind) {
		return enterprise.HRISWebhookExecution{}, false
	}
	if strings.TrimSpace(item.ExecutionMode) != enterpriseExecutionModeQueued {
		return enterprise.HRISWebhookExecution{}, false
	}
	switch strings.TrimSpace(item.Status) {
	case enterprise.HRISWebhookExecutionStatusQueued, enterprise.HRISWebhookExecutionStatusRunning:
	default:
		return enterprise.HRISWebhookExecution{}, false
	}
	if strings.TrimSpace(item.DispatchMode) != enterprise.HRISWebhookExecutionDispatchModeWorkerTick {
		return enterprise.HRISWebhookExecution{}, false
	}
	if strings.TrimSpace(item.TargetID) == "" {
		return enterprise.HRISWebhookExecution{}, false
	}
	return item, true
}

func (s *server) refreshEnterpriseHRISWebhookReceiptWorkerState() error {
	if s == nil {
		return nil
	}
	if s.enterpriseSvc != nil {
		if err := s.enterpriseSvc.RefreshCoreState(); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) refreshEnterpriseHRISWebhookDLQWorkerState() error {
	if s == nil {
		return nil
	}
	if s.enterpriseSvc != nil {
		if err := s.enterpriseSvc.RefreshCoreState(); err != nil {
			return err
		}
	}
	if s.hrisDLQSvc != nil {
		if err := s.hrisDLQSvc.RefreshState(); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) refreshEnterpriseHRISWebhookExecutionTargetSharedState(
	kind string,
	tenantID string,
	executionID string,
	targetID string,
) {
	if s == nil {
		return
	}

	var err error
	switch strings.TrimSpace(kind) {
	case enterprise.HRISWebhookExecutionKindReceiptProcess:
		err = s.refreshEnterpriseHRISWebhookReceiptWorkerState()
	case enterprise.HRISWebhookExecutionKindDLQReplay:
		err = s.refreshEnterpriseHRISWebhookDLQWorkerState()
	default:
		return
	}
	if err != nil {
		s.loggerOrDefault().Warn(
			"enterprise hris webhook execution target shared state refresh failed",
			"kind", kind,
			"tenant_id", tenantID,
			"execution_id", executionID,
			"target_id", targetID,
			"err", err,
		)
	}
}

func (s *server) refreshEnterpriseHRISPullWorkerState() error {
	if s == nil {
		return nil
	}
	if s.enterpriseSvc != nil {
		if err := s.enterpriseSvc.RefreshCoreState(); err != nil {
			return err
		}
	}
	if s.hrisPullStateSvc != nil {
		if err := s.hrisPullStateSvc.RefreshState(); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) runQueuedEnterpriseHRISWebhookReceiptExecutions(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
) int {
	if batchSize <= 0 || s == nil || s.enterpriseSvc == nil {
		return 0
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	now := time.Now().UTC()
	items := s.listQueuedEnterpriseHRISWebhookExecutions(
		enterprise.HRISWebhookExecutionKindReceiptProcess,
		batchSize,
		processingTimeout,
		now,
	)
	processed := 0
	for i := range items {
		queuedItem := items[i]
		execution := queuedItem.Execution
		originalExecutionStatus := strings.TrimSpace(execution.Status)
		claimed, claimReason, err := s.enterpriseSvc.ClaimHRISWebhookExecution(
			execution.TenantID,
			execution.ID,
			processingTimeout,
			now,
		)
		if err != nil {
			s.loggerOrDefault().Error(
				"enterprise hris webhook receipt queued execution claim failed",
				"tenant_id", execution.TenantID,
				"execution_id", execution.ID,
				"receipt_id", execution.TargetID,
				"err", err,
			)
			continue
		}
		if claimReason != "" {
			s.handleQueuedEnterpriseHRISWebhookExecutionClaimSkip(queuedItem, claimed, claimReason)
			continue
		}
		execution = claimed
		s.refreshEnterpriseHRISWebhookExecutionTargetSharedState(
			execution.Kind,
			execution.TenantID,
			execution.ID,
			execution.TargetID,
		)

		receipt, err := s.enterpriseSvc.GetHRISWebhookReceipt(execution.TenantID, execution.TargetID)
		if err != nil {
			_, _ = s.enterpriseSvc.AcknowledgeHRISWebhookExecution(execution.TenantID, execution.ID, "", err)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}

		requeued, err := s.requeueEnterpriseHRISWebhookReceiptExecutionForFreshTarget(
			queuedItem,
			execution,
			receipt,
			originalExecutionStatus,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			now,
		)
		if err != nil {
			_, _ = s.enterpriseSvc.AcknowledgeHRISWebhookExecution(execution.TenantID, execution.ID, strings.TrimSpace(receipt.Status), err)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}
		if requeued {
			processed++
			continue
		}
		switch strings.TrimSpace(receipt.Status) {
		case "processed", "skipped":
			s.completeHRISWebhookReceiptExecution(receipt, execution.ID, nil)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		case "failed", "dlq":
			s.completeHRISWebhookReceiptExecution(
				receipt,
				execution.ID,
				errors.New(firstNonEmptyString(receipt.LastError, receipt.Status)),
			)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}

		recordDLQ := receipt.AttemptCount >= maxAttempts
		s.completeHRISWebhookReceiptExecution(
			receipt,
			execution.ID,
			s.processEnterpriseHRISWebhookReceipt(nil, receipt, recordDLQ),
		)
		s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
		processed++
	}
	return processed
}

func (s *server) runQueuedEnterpriseHRISWebhookDLQExecutions(batchSize int) int {
	return s.runQueuedEnterpriseHRISWebhookDLQExecutionsWithRetryBackoffAndProcessingTimeout(
		batchSize,
		1,
		0,
		0,
		5*time.Minute,
	)
}

func (s *server) runQueuedEnterpriseHRISWebhookDLQExecutionsWithRetryBackoffAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
) int {
	if batchSize <= 0 || s == nil || s.enterpriseSvc == nil || s.hrisDLQSvc == nil {
		return 0
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	now := time.Now().UTC()
	items := s.listQueuedEnterpriseHRISWebhookExecutions(
		enterprise.HRISWebhookExecutionKindDLQReplay,
		batchSize,
		processingTimeout,
		now,
	)
	processed := 0
	for i := range items {
		queuedItem := items[i]
		execution := queuedItem.Execution
		originalExecutionStatus := strings.TrimSpace(execution.Status)
		claimed, claimReason, err := s.enterpriseSvc.ClaimHRISWebhookExecution(
			execution.TenantID,
			execution.ID,
			processingTimeout,
			now,
		)
		if err != nil {
			s.loggerOrDefault().Error(
				"enterprise hris webhook dlq queued execution claim failed",
				"tenant_id", execution.TenantID,
				"execution_id", execution.ID,
				"entry_id", execution.TargetID,
				"err", err,
			)
			continue
		}
		if claimReason != "" {
			s.handleQueuedEnterpriseHRISWebhookExecutionClaimSkip(queuedItem, claimed, claimReason)
			continue
		}
		execution = claimed
		s.refreshEnterpriseHRISWebhookExecutionTargetSharedState(
			execution.Kind,
			execution.TenantID,
			execution.ID,
			execution.TargetID,
		)

		entry, err := s.hrisDLQSvc.GetEntry(execution.TargetID)
		if err != nil {
			_, _ = s.enterpriseSvc.AcknowledgeHRISWebhookExecution(execution.TenantID, execution.ID, "", err)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}

		requeued, err := s.requeueEnterpriseHRISWebhookDLQExecutionForFreshTarget(
			queuedItem,
			execution,
			entry,
			originalExecutionStatus,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			now,
		)
		if err != nil {
			_, _ = s.enterpriseSvc.AcknowledgeHRISWebhookExecution(execution.TenantID, execution.ID, strings.TrimSpace(entry.Status), err)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}
		if requeued {
			processed++
			continue
		}
		switch strings.TrimSpace(entry.Status) {
		case "resolved":
			s.completeHRISWebhookDLQExecutionSuccess(execution.TenantID, entry, execution.ID)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		case "replaying":
			// Continue below and let the current worker replay the claimed entry.
		default:
			s.completeHRISWebhookDLQExecution(
				execution.TenantID,
				entry,
				execution.ID,
				fmt.Errorf("queued dlq execution target is no longer replaying: %s", entry.Status),
			)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}

		updated, err := s.replayEnterpriseHRISWebhookDLQClaimedEntry(
			nil,
			execution.TenantID,
			entry,
			firstNonEmptyString(execution.AuditSource, "enterprise_sync_worker"),
		)
		if err != nil {
			s.completeHRISWebhookDLQExecution(execution.TenantID, entry, execution.ID, err)
			s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
			processed++
			continue
		}
		s.completeHRISWebhookDLQExecutionSuccess(execution.TenantID, updated, execution.ID)
		s.acknowledgeQueuedEnterpriseHRISWebhookExecution(queuedItem)
		processed++
	}
	return processed
}

func (s *server) requeueEnterpriseHRISWebhookReceiptExecutionForFreshTarget(
	item enterpriseHRISWebhookQueuedExecution,
	execution enterprise.HRISWebhookExecution,
	receipt enterprise.HRISWebhookReceipt,
	originalExecutionStatus string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (bool, error) {
	if s == nil || s.enterpriseSvc == nil {
		return false, nil
	}
	if originalExecutionStatus != enterprise.HRISWebhookExecutionStatusRunning {
		return false, nil
	}
	runtime := describeHRISWebhookReceiptQueueState(
		receipt,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)
	if runtime.State != enterprise.HRISWebhookReceiptClaimReasonInFlight || runtime.ProcessingDeadlineAt == nil {
		return false, nil
	}
	requeued, err := s.enterpriseSvc.RequeueHRISWebhookExecution(
		execution.TenantID,
		execution.ID,
		strings.TrimSpace(receipt.Status),
		*runtime.ProcessingDeadlineAt,
		nil,
	)
	if err != nil {
		return false, err
	}
	s.requeueQueuedEnterpriseHRISWebhookExecution(item, requeued)
	return true, nil
}

func (s *server) reenqueueEnterpriseHRISWebhookExecution(
	execution enterprise.HRISWebhookExecution,
) {
	if s == nil {
		return
	}
	if strings.TrimSpace(execution.Status) != enterprise.HRISWebhookExecutionStatusQueued {
		return
	}
	if strings.TrimSpace(execution.ExecutionMode) != enterpriseExecutionModeQueued {
		return
	}
	if strings.TrimSpace(execution.DispatchMode) != enterprise.HRISWebhookExecutionDispatchModeWorkerTick {
		return
	}
	queueName := enterpriseHRISWebhookExecutionQueueName(execution.Kind)
	if queueName == "" {
		return
	}
	s.enqueueEnterpriseHRISWebhookExecution(
		queueName,
		execution.ID,
		execution.TenantID,
		execution.Kind,
	)
}

func (s *server) reenqueueEnterpriseHRISWebhookExecutionOnCooldown(
	execution enterprise.HRISWebhookExecution,
	claimReason string,
) {
	if s == nil || strings.TrimSpace(claimReason) != enterprise.HRISWebhookExecutionClaimReasonCooldown {
		return
	}
	s.reenqueueEnterpriseHRISWebhookExecution(execution)
}

func (s *server) acknowledgeEnterpriseHRISWebhookExecutionQueueClaim(
	queueName string,
	kind string,
	claim redistore.WorkerQueueClaim,
) bool {
	if s == nil || s.workerQueueStore == nil {
		return false
	}
	nextQueueName := strings.TrimSpace(queueName)
	nextExecutionID := strings.TrimSpace(claim.ItemID)
	nextClaimToken := strings.TrimSpace(claim.ClaimToken)
	if nextQueueName == "" || nextExecutionID == "" || nextClaimToken == "" {
		return false
	}
	applied, err := s.workerQueueStore.AckWorkerQueue(nextQueueName, nextExecutionID, nextClaimToken)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook execution queue ack failed",
			"kind", kind,
			"queue", nextQueueName,
			"execution_id", nextExecutionID,
			"err", err,
		)
		return false
	}
	if applied {
		return true
	}
	queueState, visibilityDeadlineAt := s.describeEnterpriseHRISWebhookExecutionQueueItem(nextQueueName, nextExecutionID)
	s.loggerOrDefault().Info(
		"enterprise hris webhook execution queue ack ignored stale claim",
		"kind", kind,
		"queue", nextQueueName,
		"execution_id", nextExecutionID,
		"queue_state", queueState,
		"visibility_deadline_at", visibilityDeadlineAt,
	)
	return false
}

func (s *server) acknowledgeQueuedEnterpriseHRISWebhookExecution(
	item enterpriseHRISWebhookQueuedExecution,
) bool {
	if item.QueueClaim == nil {
		return false
	}
	return s.acknowledgeEnterpriseHRISWebhookExecutionQueueClaim(
		item.QueueName,
		item.Execution.Kind,
		*item.QueueClaim,
	)
}

func (s *server) requeueEnterpriseHRISWebhookExecutionQueueClaim(
	queueName string,
	kind string,
	claim redistore.WorkerQueueClaim,
) bool {
	if s == nil || s.workerQueueStore == nil {
		return false
	}
	nextQueueName := strings.TrimSpace(queueName)
	nextExecutionID := strings.TrimSpace(claim.ItemID)
	nextClaimToken := strings.TrimSpace(claim.ClaimToken)
	if nextQueueName == "" || nextExecutionID == "" || nextClaimToken == "" {
		return false
	}
	applied, err := s.workerQueueStore.RequeueWorkerQueue(nextQueueName, nextExecutionID, nextClaimToken)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook execution queue requeue failed",
			"kind", kind,
			"queue", nextQueueName,
			"execution_id", nextExecutionID,
			"err", err,
		)
		return false
	}
	if applied {
		return true
	}
	queueState, visibilityDeadlineAt := s.describeEnterpriseHRISWebhookExecutionQueueItem(nextQueueName, nextExecutionID)
	s.loggerOrDefault().Warn(
		"enterprise hris webhook execution queue requeue missed active claim; falling back to enqueue",
		"kind", kind,
		"queue", nextQueueName,
		"execution_id", nextExecutionID,
		"queue_state", queueState,
		"visibility_deadline_at", visibilityDeadlineAt,
	)
	return false
}

func (s *server) requeueQueuedEnterpriseHRISWebhookExecution(
	item enterpriseHRISWebhookQueuedExecution,
	execution enterprise.HRISWebhookExecution,
) {
	if item.QueueClaim != nil && s.requeueEnterpriseHRISWebhookExecutionQueueClaim(
		item.QueueName,
		firstNonEmptyString(item.Execution.Kind, execution.Kind),
		*item.QueueClaim,
	) {
		return
	}
	s.reenqueueEnterpriseHRISWebhookExecution(execution)
}

func (s *server) handleQueuedEnterpriseHRISWebhookExecutionClaimSkip(
	item enterpriseHRISWebhookQueuedExecution,
	execution enterprise.HRISWebhookExecution,
	claimReason string,
) {
	if strings.TrimSpace(claimReason) == enterprise.HRISWebhookExecutionClaimReasonCooldown {
		s.requeueQueuedEnterpriseHRISWebhookExecution(item, execution)
		return
	}
	s.acknowledgeQueuedEnterpriseHRISWebhookExecution(item)
}

func (s *server) describeEnterpriseHRISWebhookExecutionQueueItem(
	queueName string,
	executionID string,
) (string, string) {
	if s == nil || s.workerQueueStore == nil {
		return "", ""
	}
	nextQueueName := strings.TrimSpace(queueName)
	nextExecutionID := strings.TrimSpace(executionID)
	if nextQueueName == "" || nextExecutionID == "" {
		return "", ""
	}
	telemetry, err := s.workerQueueStore.DescribeWorkerQueue(nextQueueName, []string{nextExecutionID})
	if err != nil {
		s.loggerOrDefault().Warn(
			"describe enterprise hris webhook execution queue item failed",
			"queue", nextQueueName,
			"execution_id", nextExecutionID,
			"err", err,
		)
		return "", ""
	}
	item, ok := telemetry.Items[nextExecutionID]
	if !ok {
		return "", ""
	}
	visibilityDeadlineAt := ""
	if item.VisibilityDeadlineAt != nil {
		visibilityDeadlineAt = item.VisibilityDeadlineAt.UTC().Format(time.RFC3339)
	}
	return strings.TrimSpace(item.State), visibilityDeadlineAt
}

func (s *server) requeueEnterpriseHRISWebhookDLQExecutionForFreshTarget(
	item enterpriseHRISWebhookQueuedExecution,
	execution enterprise.HRISWebhookExecution,
	entry hris.DeadLetterEntry,
	originalExecutionStatus string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (bool, error) {
	if s == nil || s.enterpriseSvc == nil {
		return false, nil
	}
	if originalExecutionStatus != enterprise.HRISWebhookExecutionStatusRunning {
		return false, nil
	}
	runtime := describeHRISWebhookDLQReplayState(
		entry,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)
	if runtime.State != hris.DLQEntryClaimReasonInFlight || runtime.ProcessingDeadlineAt == nil {
		return false, nil
	}
	requeued, err := s.enterpriseSvc.RequeueHRISWebhookExecution(
		execution.TenantID,
		execution.ID,
		strings.TrimSpace(entry.Status),
		*runtime.ProcessingDeadlineAt,
		nil,
	)
	if err != nil {
		return false, err
	}
	s.requeueQueuedEnterpriseHRISWebhookExecution(item, requeued)
	return true, nil
}

func (s *server) startEnterpriseJITApprovalExternalSyncWorker() {
	if !s.cfg.EnterpriseJITApprovalExternalSyncWorkerEnabled {
		return
	}
	interval := s.cfg.EnterpriseJITApprovalExternalSyncWorkerInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	batchSize := s.cfg.EnterpriseJITApprovalExternalSyncWorkerBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	maxAttempts := s.cfg.EnterpriseJITApprovalExternalSyncWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown := s.cfg.EnterpriseJITApprovalExternalSyncWorkerRetryCooldown
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	alertFailureThreshold := s.cfg.EnterpriseJITApprovalExternalSyncWorkerAlertFailureThreshold
	if alertFailureThreshold <= 0 {
		alertFailureThreshold = 1
	}
	forceError := s.cfg.EnterpriseJITApprovalExternalSyncWorkerForceError
	forceErrorTenantID := strings.TrimSpace(s.cfg.EnterpriseJITApprovalExternalSyncWorkerForceErrorTenantID)

	s.loggerOrDefault().Info(
		"enterprise jit approval external sync worker enabled",
		"interval", interval,
		"batch_size", batchSize,
		"max_attempts", maxAttempts,
		"retry_cooldown", retryCooldown,
		"alert_threshold", alertFailureThreshold,
		"force_error", forceError,
		"force_error_tenant_id", forceErrorTenantID,
	)

	go func() {
		s.runEnterpriseJITApprovalExternalSyncWorkerTick(
			batchSize,
			maxAttempts,
			retryCooldown,
			alertFailureThreshold,
			forceError,
			forceErrorTenantID,
		)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.runEnterpriseJITApprovalExternalSyncWorkerTick(
				batchSize,
				maxAttempts,
				retryCooldown,
				alertFailureThreshold,
				forceError,
				forceErrorTenantID,
			)
		}
	}()
}

func (s *server) startEnterpriseHRISWebhookReceiptWorker() {
	if !s.cfg.EnterpriseHRISWebhookReceiptWorkerEnabled {
		return
	}
	interval := s.cfg.EnterpriseHRISWebhookReceiptWorkerInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	batchSize := s.cfg.EnterpriseHRISWebhookReceiptWorkerBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	maxAttempts := s.cfg.EnterpriseHRISWebhookReceiptWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown := s.cfg.EnterpriseHRISWebhookReceiptWorkerRetryCooldown
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	retryMaxBackoff := s.cfg.EnterpriseHRISWebhookReceiptWorkerRetryMaxBackoff
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	processingTimeout := s.cfg.EnterpriseHRISWebhookReceiptWorkerProcessingTimeout
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	alertFailureThreshold := s.cfg.EnterpriseHRISWebhookReceiptWorkerAlertFailureThreshold
	if alertFailureThreshold <= 0 {
		alertFailureThreshold = 1
	}
	lockTTL := s.cfg.EnterpriseHRISWebhookReceiptWorkerLockTTL
	if lockTTL <= 0 {
		lockTTL = 10 * time.Minute
	}

	s.loggerOrDefault().Info(
		"enterprise hris webhook receipt worker enabled",
		"interval", interval,
		"batch_size", batchSize,
		"max_attempts", maxAttempts,
		"retry_cooldown", retryCooldown,
		"retry_max_backoff", retryMaxBackoff,
		"processing_timeout", processingTimeout,
		"alert_threshold", alertFailureThreshold,
		"lock_ttl", lockTTL,
		"lease_enabled", s.workerLeaseStore != nil,
	)
	if s.workerLeaseStore == nil {
		s.loggerOrDefault().Warn(
			"enterprise hris webhook receipt worker running without redis lease; duplicate receipt processing remains possible in multi-instance deployments",
		)
	}

	go func() {
		s.runEnterpriseHRISWebhookReceiptWorkerTickWithLeaseAndRetryBackoff(
			batchSize,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			alertFailureThreshold,
			lockTTL,
		)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runEnterpriseHRISWebhookReceiptWorkerTickWithLeaseAndRetryBackoff(
					batchSize,
					maxAttempts,
					retryCooldown,
					retryMaxBackoff,
					processingTimeout,
					alertFailureThreshold,
					lockTTL,
				)
			case <-s.hrisWebhookReceiptWorkerWake:
				s.runEnterpriseHRISWebhookReceiptWorkerTickWithLeaseAndRetryBackoff(
					batchSize,
					maxAttempts,
					retryCooldown,
					retryMaxBackoff,
					processingTimeout,
					alertFailureThreshold,
					lockTTL,
				)
			case task := <-s.hrisWebhookReceiptWorkerQueue:
				s.processQueuedEnterpriseHRISWebhookReceipt(task.Receipt, task.RecordDLQ, task.ExecutionID)
			}
		}
	}()
}

func (s *server) startEnterpriseHRISWebhookDLQWorker() {
	if !s.cfg.EnterpriseHRISWebhookDLQWorkerEnabled {
		return
	}
	interval := s.cfg.EnterpriseHRISWebhookDLQWorkerInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	batchSize := s.cfg.EnterpriseHRISWebhookDLQWorkerBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	maxAttempts := s.cfg.EnterpriseHRISWebhookDLQWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown := s.cfg.EnterpriseHRISWebhookDLQWorkerRetryCooldown
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	retryMaxBackoff := s.cfg.EnterpriseHRISWebhookDLQWorkerRetryMaxBackoff
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	processingTimeout := s.cfg.EnterpriseHRISWebhookDLQWorkerProcessingTimeout
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	alertFailureThreshold := s.cfg.EnterpriseHRISWebhookDLQWorkerAlertFailureThreshold
	if alertFailureThreshold <= 0 {
		alertFailureThreshold = 1
	}
	lockTTL := s.cfg.EnterpriseHRISWebhookDLQWorkerLockTTL
	if lockTTL <= 0 {
		lockTTL = 10 * time.Minute
	}

	s.loggerOrDefault().Info(
		"enterprise hris webhook dlq worker enabled",
		"interval", interval,
		"batch_size", batchSize,
		"max_attempts", maxAttempts,
		"retry_cooldown", retryCooldown,
		"retry_max_backoff", retryMaxBackoff,
		"processing_timeout", processingTimeout,
		"alert_threshold", alertFailureThreshold,
		"lock_ttl", lockTTL,
		"lease_enabled", s.workerLeaseStore != nil,
	)
	if s.workerLeaseStore == nil {
		s.loggerOrDefault().Warn(
			"enterprise hris webhook dlq worker running without redis lease; duplicate dlq replays remain possible in multi-instance deployments",
		)
	}

	go func() {
		s.runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
			batchSize,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			alertFailureThreshold,
			lockTTL,
		)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
					batchSize,
					maxAttempts,
					retryCooldown,
					retryMaxBackoff,
					processingTimeout,
					alertFailureThreshold,
					lockTTL,
				)
			case <-s.hrisWebhookDLQWorkerWake:
				s.runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
					batchSize,
					maxAttempts,
					retryCooldown,
					retryMaxBackoff,
					processingTimeout,
					alertFailureThreshold,
					lockTTL,
				)
			case task := <-s.hrisWebhookDLQWorkerQueue:
				s.replayQueuedEnterpriseHRISWebhookDLQEntry(task.TenantID, task.Entry, task.AuditSource, task.ExecutionID)
			}
		}
	}()
}

func (s *server) startEnterpriseHRISPullWorker() {
	if !s.cfg.EnterpriseHRISPullWorkerEnabled {
		return
	}
	interval := s.cfg.EnterpriseHRISPullWorkerInterval
	if interval <= 0 {
		interval = time.Hour
	}
	batchSize := s.cfg.EnterpriseHRISPullWorkerBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	maxAttempts := s.cfg.EnterpriseHRISPullWorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown := s.cfg.EnterpriseHRISPullWorkerRetryCooldown
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	retryMaxBackoff := s.cfg.EnterpriseHRISPullWorkerRetryMaxBackoff
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	processingTimeout := s.cfg.EnterpriseHRISPullWorkerProcessingTimeout
	if processingTimeout <= 0 {
		processingTimeout = 30 * time.Minute
	}
	reconcileInterval := s.cfg.EnterpriseHRISPullWorkerReconcileInterval
	if reconcileInterval <= 0 {
		reconcileInterval = 24 * time.Hour
	}
	alertFailureThreshold := s.cfg.EnterpriseHRISPullWorkerAlertFailureThreshold
	if alertFailureThreshold <= 0 {
		alertFailureThreshold = 1
	}
	lockTTL := s.cfg.EnterpriseHRISPullWorkerLockTTL
	if lockTTL <= 0 {
		lockTTL = 10 * time.Minute
	}

	s.loggerOrDefault().Info(
		"enterprise hris pull worker enabled",
		"interval", interval,
		"batch_size", batchSize,
		"max_attempts", maxAttempts,
		"retry_cooldown", retryCooldown,
		"retry_max_backoff", retryMaxBackoff,
		"processing_timeout", processingTimeout,
		"reconcile_interval", reconcileInterval,
		"alert_threshold", alertFailureThreshold,
		"lock_ttl", lockTTL,
		"lease_enabled", s.workerLeaseStore != nil,
	)
	if s.workerLeaseStore == nil {
		s.loggerOrDefault().Warn(
			"enterprise hris pull worker running without redis lease; duplicate pull ticks remain possible in multi-instance deployments",
		)
	}

	go func() {
		s.runEnterpriseHRISPullWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
			batchSize,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			reconcileInterval,
			processingTimeout,
			alertFailureThreshold,
			lockTTL,
		)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.runEnterpriseHRISPullWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
				batchSize,
				maxAttempts,
				retryCooldown,
				retryMaxBackoff,
				reconcileInterval,
				processingTimeout,
				alertFailureThreshold,
				lockTTL,
			)
		}
	}()
}

func (s *server) runEnterpriseJITApprovalExternalSyncWorkerTick(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	alertFailureThreshold int,
	forceError bool,
	forceErrorTenantID string,
) {
	now := time.Now().UTC()
	allApprovals := s.enterpriseSvc.ListJITProvisionApprovals("", "", 0)
	tenantIDs := pendingJITApprovalExternalSyncTenantIDs(allApprovals, maxAttempts, retryCooldown, now)
	if len(tenantIDs) == 0 {
		return
	}

	for i := range tenantIDs {
		tenantID := tenantIDs[i]
		items := s.enterpriseSvc.ListJITProvisionApprovals(tenantID, "", 0)
		result := enterpriseJITApprovalExternalSyncWorkerResult{}
		for j := range items {
			if result.Processed >= batchSize {
				break
			}
			item := items[j]
			if !enterpriseJITApprovalExternalSyncCandidate(item) {
				continue
			}
			syncStatus := strings.TrimSpace(item.ExternalSyncStatus)
			if syncStatus == "failed" {
				if maxAttempts > 0 && item.ExternalSyncAttemptCount >= maxAttempts {
					result.SkippedByAttemptLimit++
					continue
				}
				if retryCooldown > 0 && item.ExternalSyncUpdatedAt != nil {
					retryReadyAt := item.ExternalSyncUpdatedAt.Add(retryCooldown)
					if retryReadyAt.After(now) {
						result.SkippedByCooldown++
						continue
					}
				}
			}

			nextStatus := "synced"
			nextRef := fmt.Sprintf("worker-auto-sync:%d", now.UnixNano())
			nextErr := ""
			if forceError {
				if forceErrorTenantID == "" || tenantID == forceErrorTenantID {
					nextStatus = "failed"
					nextRef = "worker-force-error"
					nextErr = "forced enterprise jit approval external sync worker failure"
				}
			}
			updated, err := s.enterpriseSvc.UpdateJITProvisionApprovalExternalSync(
				tenantID,
				item.ID,
				nextStatus,
				nextRef,
				nextErr,
			)
			if err != nil {
				result.Failed++
				result.Processed++
				s.loggerOrDefault().Error(
					"enterprise jit approval external sync worker failed",
					"tenant_id", tenantID,
					"approval_id", item.ID,
					"err", err,
				)
				continue
			}
			if strings.TrimSpace(updated.ExternalSyncStatus) == "synced" {
				result.Synced++
			} else {
				result.Failed++
			}
			result.Processed++
		}

		if result.Processed == 0 {
			if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 {
				s.loggerOrDefault().Info(
					"enterprise jit approval external sync worker skipped",
					"tenant_id", tenantID,
					"processed", 0,
					"skipped_attempt_limit", result.SkippedByAttemptLimit,
					"skipped_cooldown", result.SkippedByCooldown,
				)
			}
			continue
		}
		if result.Failed >= alertFailureThreshold {
			s.loggerOrDefault().Warn(
				"enterprise jit approval external sync worker alert",
				"tenant_id", tenantID,
				"failed", result.Failed,
				"threshold", alertFailureThreshold,
			)
			s.appendEnterpriseJITApprovalExternalSyncWorkerAlertAudit(tenantID, result, alertFailureThreshold)
		}
		s.loggerOrDefault().Info(
			"enterprise jit approval external sync worker finished",
			"tenant_id", tenantID,
			"processed", result.Processed,
			"synced", result.Synced,
			"failed", result.Failed,
			"skipped_attempt_limit", result.SkippedByAttemptLimit,
			"skipped_cooldown", result.SkippedByCooldown,
		)
	}
}

func enterpriseJITApprovalExternalSyncCandidate(item enterprise.JITProvisionApproval) bool {
	status := strings.TrimSpace(item.Status)
	if status != "approved" && status != "rejected" {
		return false
	}
	syncStatus := strings.TrimSpace(item.ExternalSyncStatus)
	if syncStatus == "" {
		syncStatus = "pending"
	}
	return syncStatus == "pending" || syncStatus == "failed"
}

func (s *server) appendEnterpriseJITApprovalExternalSyncWorkerAlertAudit(
	tenantID string,
	result enterpriseJITApprovalExternalSyncWorkerResult,
	alertFailureThreshold int,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	if result.Failed < alertFailureThreshold {
		return
	}
	target := strings.TrimSpace(
		fmt.Sprintf(
			"failed=%d threshold=%d processed=%d synced=%d skipped_attempt_limit=%d skipped_cooldown=%d",
			result.Failed,
			alertFailureThreshold,
			result.Processed,
			result.Synced,
			result.SkippedByAttemptLimit,
			result.SkippedByCooldown,
		),
	)
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_jit_approval_external_sync_worker_alert",
		target,
		"enterprise_sync_worker",
	)
}

func (s *server) runEnterpriseSyncReconcileWorkerTick(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	alertFailureThreshold int,
	forceError bool,
	forceErrorTenantID string,
) {
	allRecords := s.enterpriseSvc.ListSyncRequestRecords("", 0)
	tenantIDs := pendingSyncRequestTenantIDs(allRecords)
	if len(tenantIDs) == 0 {
		return
	}

	for i := range tenantIDs {
		tenantID := tenantIDs[i]
		result, err := s.enterpriseSvc.ReconcilePendingSyncRequestsWithPolicy(
			tenantID,
			batchSize,
			maxAttempts,
			retryCooldown,
			func(items []enterprise.EnterpriseEmployee) (int, int, int, error) {
				if forceError {
					if forceErrorTenantID == "" || tenantID == forceErrorTenantID {
						return 0, 0, 0, errors.New("forced enterprise sync reconcile worker apply failure")
					}
				}
				accessInputs := enterpriseEmployeesToAccessBatchInputs(items)
				return s.accessSvc.UpsertUsersByEmail(tenantID, accessInputs)
			},
		)
		if err != nil {
			s.loggerOrDefault().Error(
				"enterprise sync reconcile worker failed",
				"tenant_id", tenantID,
				"err", err,
			)
			continue
		}
		if result.Processed == 0 {
			if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 {
				s.loggerOrDefault().Info(
					"enterprise sync reconcile worker skipped",
					"tenant_id", tenantID,
					"processed", 0,
					"skipped_attempt_limit", result.SkippedByAttemptLimit,
					"skipped_cooldown", result.SkippedByCooldown,
				)
				s.appendEnterpriseSyncWorkerAlertAudit(tenantID, result, alertFailureThreshold)
			}
			continue
		}
		if result.Failed >= alertFailureThreshold {
			s.loggerOrDefault().Warn(
				"enterprise sync reconcile worker alert",
				"tenant_id", tenantID,
				"failed", result.Failed,
				"threshold", alertFailureThreshold,
			)
			s.appendEnterpriseSyncWorkerAlertAudit(tenantID, result, alertFailureThreshold)
		}
		s.loggerOrDefault().Info(
			"enterprise sync reconcile worker finished",
			"tenant_id", tenantID,
			"processed", result.Processed,
			"applied", result.Applied,
			"failed", result.Failed,
			"skipped_attempt_limit", result.SkippedByAttemptLimit,
			"skipped_cooldown", result.SkippedByCooldown,
		)
	}
}

func (s *server) runEnterpriseSyncWorkerAlertAutoRetryWorkerTick(
	batchSize int,
	maxAttempts int,
	baseBackoff time.Duration,
	maxBackoff time.Duration,
) {
	if batchSize <= 0 {
		batchSize = 1
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if baseBackoff <= 0 {
		baseBackoff = 5 * time.Minute
	}
	if maxBackoff <= 0 {
		maxBackoff = time.Hour
	}
	if maxBackoff < baseBackoff {
		maxBackoff = baseBackoff
	}
	if s.enterpriseSvc == nil || s.walletSvc == nil {
		return
	}

	now := time.Now().UTC()
	allNotifications := s.enterpriseSvc.ListSyncWorkerAlertNotificationsWithOptions(
		enterprise.SyncWorkerAlertNotificationListOptions{
			Limit: 0,
		},
	)
	tenantIDs := pendingSyncWorkerAlertAutoRetryTenantIDs(allNotifications, now)
	if len(tenantIDs) == 0 {
		return
	}

	for i := range tenantIDs {
		tenantID := tenantIDs[i]
		result, err := s.enterpriseSvc.AutoRetrySyncWorkerAlertNotifications(enterprise.SyncWorkerAlertNotificationAutoRetryInput{
			TenantID:    tenantID,
			Limit:       batchSize,
			MaxAttempts: maxAttempts,
			BaseBackoff: baseBackoff,
			MaxBackoff:  maxBackoff,
			RetriedAt:   now,
			Dispatch: func(input enterprise.SyncWorkerAlertDeliveryInput) enterprise.SyncWorkerAlertDeliveryResult {
				delivery := s.walletSvc.DispatchAlert(wallet.AlertDeliveryInput{
					TenantID:       input.TenantID,
					Channels:       input.Channels,
					ReceiverGroups: input.ReceiverGroups,
					IdempotencyKey: input.IdempotencyKey,
					EmailSubject:   input.EmailSubject,
					EmailText:      input.EmailText,
					WhatsAppText:   input.WhatsAppText,
				})
				return enterprise.SyncWorkerAlertDeliveryResult{
					Status:         delivery.Status,
					Reason:         delivery.Reason,
					Provider:       delivery.Provider,
					ProviderError:  delivery.ProviderError,
					Retryable:      delivery.Retryable,
					ChannelResults: delivery.ChannelResults,
				}
			},
		})
		if err != nil {
			s.loggerOrDefault().Error(
				"enterprise sync worker alert auto retry worker failed",
				"tenant_id", tenantID,
				"err", err,
			)
			continue
		}
		if result.TotalNotifications == 0 {
			continue
		}
		workerResult := enterpriseSyncWorkerAlertAutoRetryWorkerResult{
			Processed:  result.TotalNotifications,
			Retried:    result.Retried,
			Failed:     result.Failed,
			Skipped:    result.Skipped,
			Suppressed: result.Suppressed,
		}
		s.loggerOrDefault().Info(
			"enterprise sync worker alert auto retry worker finished",
			"tenant_id", tenantID,
			"processed", workerResult.Processed,
			"retried", workerResult.Retried,
			"failed", workerResult.Failed,
			"skipped", workerResult.Skipped,
			"suppressed", workerResult.Suppressed,
		)
		s.appendEnterpriseSyncWorkerAlertAutoRetryWorkerAudit(tenantID, workerResult)
	}
}

func (s *server) runEnterpriseSyncWorkerAlertAutoRetryWorkerTickWithLease(
	batchSize int,
	maxAttempts int,
	baseBackoff time.Duration,
	maxBackoff time.Duration,
	lockTTL time.Duration,
) {
	if s.workerLeaseStore == nil || lockTTL <= 0 {
		s.runEnterpriseSyncWorkerAlertAutoRetryWorkerTick(batchSize, maxAttempts, baseBackoff, maxBackoff)
		return
	}

	token, err := randomHexID(16)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise sync worker alert auto retry worker lease token generation failed",
			"err", err,
		)
		return
	}
	acquired, err := s.workerLeaseStore.TryAcquireLease(enterpriseSyncWorkerAlertAutoRetryLeaseKey, token, lockTTL)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise sync worker alert auto retry worker lease acquire failed",
			"err", err,
		)
		return
	}
	if !acquired {
		s.loggerOrDefault().Info(
			"enterprise sync worker alert auto retry worker lease unavailable; skipping tick",
			"lease_key", enterpriseSyncWorkerAlertAutoRetryLeaseKey,
		)
		return
	}
	defer func() {
		if err := s.workerLeaseStore.ReleaseLease(enterpriseSyncWorkerAlertAutoRetryLeaseKey, token); err != nil {
			s.loggerOrDefault().Error(
				"enterprise sync worker alert auto retry worker lease release failed",
				"lease_key", enterpriseSyncWorkerAlertAutoRetryLeaseKey,
				"err", err,
			)
		}
	}()

	s.runEnterpriseSyncWorkerAlertAutoRetryWorkerTick(batchSize, maxAttempts, baseBackoff, maxBackoff)
}

func (s *server) runEnterpriseHRISWebhookReceiptWorkerTickWithLease(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	s.runEnterpriseHRISWebhookReceiptWorkerTickWithLeaseAndRetryBackoff(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		processingTimeout,
		alertFailureThreshold,
		lockTTL,
	)
}

func (s *server) runEnterpriseHRISWebhookReceiptWorkerTickWithLeaseAndRetryBackoff(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	if s.workerLeaseStore == nil || lockTTL <= 0 {
		s.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(
			batchSize,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			alertFailureThreshold,
		)
		return
	}

	token, err := randomHexID(16)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook receipt worker lease token generation failed",
			"err", err,
		)
		return
	}
	acquired, err := s.workerLeaseStore.TryAcquireLease(enterpriseHRISWebhookReceiptLeaseKey, token, lockTTL)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook receipt worker lease acquire failed",
			"err", err,
		)
		return
	}
	if !acquired {
		s.loggerOrDefault().Info(
			"enterprise hris webhook receipt worker lease unavailable; skipping tick",
			"lease_key", enterpriseHRISWebhookReceiptLeaseKey,
		)
		return
	}
	defer func() {
		if err := s.workerLeaseStore.ReleaseLease(enterpriseHRISWebhookReceiptLeaseKey, token); err != nil {
			s.loggerOrDefault().Error(
				"enterprise hris webhook receipt worker lease release failed",
				"lease_key", enterpriseHRISWebhookReceiptLeaseKey,
				"err", err,
			)
		}
	}()

	s.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISWebhookReceiptWorkerTick(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
) {
	s.runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		processingTimeout,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISWebhookReceiptWorkerTickWithRetryBackoff(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
) {
	if batchSize <= 0 {
		batchSize = 1
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	if alertFailureThreshold <= 0 {
		alertFailureThreshold = 1
	}
	if s.enterpriseSvc == nil {
		return
	}
	if err := s.refreshEnterpriseHRISWebhookReceiptWorkerState(); err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook receipt worker state refresh failed",
			"err", err,
		)
		return
	}
	processedQueuedExecutions := s.runQueuedEnterpriseHRISWebhookReceiptExecutions(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
	)
	if processedQueuedExecutions >= batchSize {
		return
	}
	batchSize -= processedQueuedExecutions
	now := time.Now().UTC()
	allReceipts := s.enterpriseSvc.ListDueHRISWebhookReceiptsWithBackoff(
		"",
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
		0,
	)
	tenantIDs := pendingHRISWebhookReceiptTenantIDs(allReceipts)
	if len(tenantIDs) == 0 {
		return
	}
	receiptsByTenant := groupHRISWebhookReceiptsByTenant(allReceipts)

	for i := range tenantIDs {
		tenantID := tenantIDs[i]
		items := receiptsByTenant[tenantID]
		result := enterpriseHRISWebhookReceiptWorkerResult{}
		for j := range items {
			if result.Processed >= batchSize {
				break
			}
			item := items[j]
			status := strings.TrimSpace(item.Status)
			if status != "received" && status != "failed" && status != "processing" {
				continue
			}

			claimed, skipReason, err := s.enterpriseSvc.ClaimHRISWebhookReceiptForProcessingWithBackoff(
				tenantID,
				item.ID,
				maxAttempts,
				retryCooldown,
				retryMaxBackoff,
				processingTimeout,
				now,
			)
			if err != nil {
				s.loggerOrDefault().Error(
					"enterprise hris webhook receipt worker claim failed",
					"tenant_id", tenantID,
					"receipt_id", item.ID,
					"err", err,
				)
				continue
			}
			switch skipReason {
			case "":
			case enterprise.HRISWebhookReceiptClaimReasonAttemptLimit:
				result.SkippedByAttemptLimit++
				continue
			case enterprise.HRISWebhookReceiptClaimReasonCooldown:
				result.SkippedByCooldown++
				continue
			case enterprise.HRISWebhookReceiptClaimReasonInFlight:
				result.SkippedByInFlight++
				continue
			default:
				continue
			}

			recordDLQ := claimed.AttemptCount >= maxAttempts
			if err := s.processEnterpriseHRISWebhookReceipt(nil, claimed, recordDLQ); err != nil {
				result.Processed++
				result.Failed++
				result.LastConnectorID = strings.TrimSpace(claimed.ConnectorID)
				result.LastVendor = strings.TrimSpace(claimed.Vendor)
				result.LastRequestID = strings.TrimSpace(claimed.RequestID)
				result.LastEventType = strings.TrimSpace(claimed.EventType)
				s.loggerOrDefault().Error(
					"enterprise hris webhook receipt worker failed",
					"tenant_id", tenantID,
					"receipt_id", claimed.ID,
					"err", err,
				)
				continue
			}

			updated, err := s.enterpriseSvc.GetHRISWebhookReceipt(tenantID, claimed.ID)
			if err == nil && strings.TrimSpace(updated.Status) == "skipped" {
				result.Skipped++
			} else {
				result.Synced++
			}
			result.Processed++
		}

		if result.Processed == 0 {
			if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 || result.SkippedByInFlight > 0 {
				s.loggerOrDefault().Info(
					"enterprise hris webhook receipt worker skipped",
					"tenant_id", tenantID,
					"processed", 0,
					"skipped_in_flight", result.SkippedByInFlight,
					"skipped_attempt_limit", result.SkippedByAttemptLimit,
					"skipped_cooldown", result.SkippedByCooldown,
				)
				if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 {
					s.appendEnterpriseHRISWebhookReceiptWorkerAlertAudit(tenantID, result, alertFailureThreshold)
				}
			}
			continue
		}
		if result.Failed >= alertFailureThreshold {
			s.loggerOrDefault().Warn(
				"enterprise hris webhook receipt worker alert",
				"tenant_id", tenantID,
				"failed", result.Failed,
				"threshold", alertFailureThreshold,
			)
			s.appendEnterpriseHRISWebhookReceiptWorkerAlertAudit(tenantID, result, alertFailureThreshold)
		}
		s.loggerOrDefault().Info(
			"enterprise hris webhook receipt worker finished",
			"tenant_id", tenantID,
			"processed", result.Processed,
			"synced", result.Synced,
			"skipped", result.Skipped,
			"failed", result.Failed,
			"skipped_in_flight", result.SkippedByInFlight,
			"skipped_attempt_limit", result.SkippedByAttemptLimit,
			"skipped_cooldown", result.SkippedByCooldown,
		)
	}
}

func (s *server) runEnterpriseHRISWebhookDLQWorkerTickWithLease(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	s.runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		0,
		alertFailureThreshold,
		lockTTL,
	)
}

func (s *server) runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	s.runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		processingTimeout,
		alertFailureThreshold,
		lockTTL,
	)
}

func (s *server) runEnterpriseHRISWebhookDLQWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	if s.workerLeaseStore == nil || lockTTL <= 0 {
		s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
			batchSize,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			alertFailureThreshold,
		)
		return
	}

	token, err := randomHexID(16)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook dlq worker lease token generation failed",
			"err", err,
		)
		return
	}
	acquired, err := s.workerLeaseStore.TryAcquireLease(enterpriseHRISWebhookDLQLeaseKey, token, lockTTL)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook dlq worker lease acquire failed",
			"err", err,
		)
		return
	}
	if !acquired {
		s.loggerOrDefault().Info(
			"enterprise hris webhook dlq worker lease unavailable; skipping tick",
			"lease_key", enterpriseHRISWebhookDLQLeaseKey,
		)
		return
	}
	defer func() {
		if err := s.workerLeaseStore.ReleaseLease(enterpriseHRISWebhookDLQLeaseKey, token); err != nil {
			s.loggerOrDefault().Error(
				"enterprise hris webhook dlq worker lease release failed",
				"lease_key", enterpriseHRISWebhookDLQLeaseKey,
				"err", err,
			)
		}
	}()

	s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISWebhookDLQWorkerTick(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	alertFailureThreshold int,
) {
	s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		0,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISWebhookDLQWorkerTickWithProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
) {
	s.runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		processingTimeout,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISWebhookDLQWorkerTickWithRetryBackoffAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
) {
	if s.hrisDLQSvc == nil {
		return
	}
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	if err := s.refreshEnterpriseHRISWebhookDLQWorkerState(); err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris webhook dlq worker state refresh failed",
			"err", err,
		)
		return
	}
	processedQueuedExecutions := s.runQueuedEnterpriseHRISWebhookDLQExecutionsWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
	)
	if processedQueuedExecutions >= batchSize {
		return
	}
	batchSize -= processedQueuedExecutions
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	now := time.Now().UTC()
	allEntries := s.hrisDLQSvc.ListDueEntriesForReplayWithBackoff(
		"",
		"",
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
		0,
	)
	tenantIDs := pendingHRISWebhookDLQTenantIDs(allEntries, maxAttempts, retryCooldown, now)
	if len(tenantIDs) == 0 {
		return
	}
	entriesByTenant := groupHRISWebhookDLQEntriesByTenant(allEntries)

	for i := range tenantIDs {
		tenantID := tenantIDs[i]
		items := entriesByTenant[tenantID]
		result := enterpriseHRISWebhookDLQWorkerResult{}
		for j := range items {
			if result.Processed >= batchSize {
				break
			}
			item := items[j]
			status := strings.TrimSpace(item.Status)
			if status != "dlq" && status != "replaying" {
				continue
			}
			claimed, skipReason, err := s.hrisDLQSvc.ClaimEntryForReplayWithBackoff(
				item.ID,
				maxAttempts,
				retryCooldown,
				retryMaxBackoff,
				processingTimeout,
				now,
			)
			if err != nil {
				s.loggerOrDefault().Error(
					"enterprise hris webhook dlq worker claim failed",
					"tenant_id", tenantID,
					"entry_id", item.ID,
					"err", err,
				)
				continue
			}
			switch skipReason {
			case "":
			case hris.DLQEntryClaimReasonAttemptLimit:
				result.SkippedByAttemptLimit++
				continue
			case hris.DLQEntryClaimReasonCooldown:
				result.SkippedByCooldown++
				continue
			case hris.DLQEntryClaimReasonInFlight:
				result.SkippedByInFlight++
				continue
			default:
				continue
			}

			if _, err := s.replayEnterpriseHRISWebhookDLQClaimedEntry(nil, tenantID, claimed, "enterprise_sync_worker"); err != nil {
				result.Failed++
				result.Processed++
				result.LastConnectorID = strings.TrimSpace(claimed.ConnectorID)
				result.LastVendor = strings.TrimSpace(claimed.Vendor)
				result.LastRequestID = strings.TrimSpace(claimed.RequestID)
				result.LastEventType = strings.TrimSpace(claimed.EventType)
				result.LastFailureStage = strings.TrimSpace(claimed.FailureStage)
				s.loggerOrDefault().Error(
					"enterprise hris webhook dlq worker replay failed",
					"tenant_id", tenantID,
					"entry_id", claimed.ID,
					"err", err,
				)
				continue
			}
			result.Replayed++
			result.Processed++
		}

		if result.Processed == 0 {
			if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 || result.SkippedByInFlight > 0 {
				s.loggerOrDefault().Info(
					"enterprise hris webhook dlq worker skipped",
					"tenant_id", tenantID,
					"processed", 0,
					"skipped_in_flight", result.SkippedByInFlight,
					"skipped_attempt_limit", result.SkippedByAttemptLimit,
					"skipped_cooldown", result.SkippedByCooldown,
				)
				if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 {
					s.appendEnterpriseHRISWebhookDLQWorkerAlertAudit(tenantID, result, alertFailureThreshold)
				}
			}
			continue
		}
		if result.Failed >= alertFailureThreshold {
			s.loggerOrDefault().Warn(
				"enterprise hris webhook dlq worker alert",
				"tenant_id", tenantID,
				"failed", result.Failed,
				"threshold", alertFailureThreshold,
			)
			s.appendEnterpriseHRISWebhookDLQWorkerAlertAudit(tenantID, result, alertFailureThreshold)
		}
		s.loggerOrDefault().Info(
			"enterprise hris webhook dlq worker finished",
			"tenant_id", tenantID,
			"processed", result.Processed,
			"replayed", result.Replayed,
			"failed", result.Failed,
			"skipped_in_flight", result.SkippedByInFlight,
			"skipped_attempt_limit", result.SkippedByAttemptLimit,
			"skipped_cooldown", result.SkippedByCooldown,
		)
	}
}

func (s *server) runEnterpriseHRISPullWorkerTick(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	reconcileInterval time.Duration,
	alertFailureThreshold int,
) {
	s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		reconcileInterval,
		0,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISPullWorkerTickWithLease(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	reconcileInterval time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	s.runEnterpriseHRISPullWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		reconcileInterval,
		0,
		alertFailureThreshold,
		lockTTL,
	)
}

func (s *server) runEnterpriseHRISPullWorkerTickWithLeaseAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	reconcileInterval time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	s.runEnterpriseHRISPullWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		reconcileInterval,
		processingTimeout,
		alertFailureThreshold,
		lockTTL,
	)
}

func (s *server) runEnterpriseHRISPullWorkerTickWithLeaseAndRetryBackoffAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	reconcileInterval time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
	lockTTL time.Duration,
) {
	if s.workerLeaseStore == nil || lockTTL <= 0 {
		s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(
			batchSize,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			reconcileInterval,
			processingTimeout,
			alertFailureThreshold,
		)
		return
	}

	token, err := randomHexID(16)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris pull worker lease token generation failed",
			"err", err,
		)
		return
	}
	acquired, err := s.workerLeaseStore.TryAcquireLease(enterpriseHRISPullLeaseKey, token, lockTTL)
	if err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris pull worker lease acquire failed",
			"err", err,
		)
		return
	}
	if !acquired {
		s.loggerOrDefault().Info(
			"enterprise hris pull worker lease unavailable; skipping tick",
			"lease_key", enterpriseHRISPullLeaseKey,
		)
		return
	}
	defer func() {
		if err := s.workerLeaseStore.ReleaseLease(enterpriseHRISPullLeaseKey, token); err != nil {
			s.loggerOrDefault().Error(
				"enterprise hris pull worker lease release failed",
				"lease_key", enterpriseHRISPullLeaseKey,
				"err", err,
			)
		}
	}()

	s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		reconcileInterval,
		processingTimeout,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISPullWorkerTickWithProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	reconcileInterval time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
) {
	s.runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(
		batchSize,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		reconcileInterval,
		processingTimeout,
		alertFailureThreshold,
	)
}

func (s *server) runEnterpriseHRISPullWorkerTickWithRetryBackoffAndProcessingTimeout(
	batchSize int,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	reconcileInterval time.Duration,
	processingTimeout time.Duration,
	alertFailureThreshold int,
) {
	if s.enterpriseSvc == nil || s.hrisPullRegistry == nil || s.hrisPullStateSvc == nil {
		return
	}
	if retryCooldown < 0 {
		retryCooldown = 0
	}
	if retryCooldown <= 0 {
		retryMaxBackoff = 0
	} else if retryMaxBackoff < retryCooldown {
		retryMaxBackoff = retryCooldown
	}
	if processingTimeout <= 0 {
		processingTimeout = 30 * time.Minute
	}
	if err := s.refreshEnterpriseHRISPullWorkerState(); err != nil {
		s.loggerOrDefault().Error(
			"enterprise hris pull worker shared state refresh failed",
			"err", err,
		)
		return
	}

	now := time.Now().UTC()
	connectors := s.enterpriseSvc.ListHRISConnectors("")
	if len(connectors) == 0 {
		return
	}
	sort.Slice(connectors, func(i, j int) bool {
		leftTenantID := strings.TrimSpace(connectors[i].TenantID)
		rightTenantID := strings.TrimSpace(connectors[j].TenantID)
		if leftTenantID != rightTenantID {
			return leftTenantID < rightTenantID
		}
		leftVendor := strings.TrimSpace(connectors[i].Vendor)
		rightVendor := strings.TrimSpace(connectors[j].Vendor)
		if leftVendor != rightVendor {
			return leftVendor < rightVendor
		}
		return strings.TrimSpace(connectors[i].ID) < strings.TrimSpace(connectors[j].ID)
	})

	stateMap := make(map[string]hris.ConnectorPullState)
	for _, item := range s.hrisPullStateSvc.ListStates("") {
		stateMap[strings.TrimSpace(item.ConnectorID)] = item
	}

	resultsByTenant := make(map[string]*enterpriseHRISPullWorkerResult)
	processedCount := 0
	for i := range connectors {
		connector := connectors[i]
		if !enterpriseHRISPullConnectorCandidate(connector) {
			continue
		}
		adapter, ok := s.hrisPullRegistry.Get(connector.Vendor)
		if !ok {
			continue
		}

		tenantID := strings.TrimSpace(connector.TenantID)
		if tenantID == "" {
			continue
		}
		result := resultsByTenant[tenantID]
		if result == nil {
			result = &enterpriseHRISPullWorkerResult{}
			resultsByTenant[tenantID] = result
		}

		state := stateMap[strings.TrimSpace(connector.ID)]
		mode := enterpriseHRISPullMode(connector, state, now, reconcileInterval)
		pullInput := hris.PullInput{
			Connector:         connector,
			LastSuccessAt:     state.LastSuccessAt,
			LastFullSuccessAt: enterpriseHRISLastFullSuccessAt(connector, state),
			Mode:              mode,
			Now:               now,
		}
		if mode == "" {
			continue
		}

		if processedCount >= batchSize {
			break
		}

		claimedState, skipReason, err := s.hrisPullStateSvc.ClaimStateForPullWithBackoff(
			tenantID,
			connector.ID,
			connector.Vendor,
			mode,
			maxAttempts,
			retryCooldown,
			retryMaxBackoff,
			processingTimeout,
			now,
		)
		if err != nil {
			s.loggerOrDefault().Error(
				"enterprise hris pull worker claim failed",
				"tenant_id", tenantID,
				"connector_id", connector.ID,
				"err", err,
			)
			continue
		}
		switch skipReason {
		case "":
			state = claimedState
		case hris.PullStateClaimReasonAttemptLimit:
			result.SkippedByAttemptLimit++
			updateEnterpriseHRISPullWorkerStatefulMetrics(result, connector, claimedState, mode, now)
			continue
		case hris.PullStateClaimReasonCooldown:
			result.SkippedByCooldown++
			updateEnterpriseHRISPullWorkerStatefulMetrics(result, connector, claimedState, mode, now)
			continue
		case hris.PullStateClaimReasonInFlight:
			result.SkippedByInFlight++
			updateEnterpriseHRISPullWorkerStatefulMetrics(result, connector, claimedState, mode, now)
			continue
		default:
			continue
		}

		credentialValue := ""
		if strings.TrimSpace(connector.CredentialRef) != "" {
			if s.hrisVaultSvc == nil {
				result.Processed++
				result.Failed++
				result.LastConnectorID = strings.TrimSpace(connector.ID)
				result.LastVendor = strings.TrimSpace(connector.Vendor)
				result.LastMode = strings.TrimSpace(mode)
				processedCount++
				updatedState, _ := s.hrisPullStateSvc.MarkFailed(tenantID, connector.ID, connector.Vendor, now, errors.New("hris credential vault unavailable"))
				updateEnterpriseHRISPullWorkerStatefulMetrics(result, connector, updatedState, mode, now)
				s.loggerOrDefault().Error(
					"enterprise hris pull worker credential vault unavailable",
					"tenant_id", tenantID,
					"connector_id", connector.ID,
				)
				continue
			}
			resolvedCredential, err := s.hrisVaultSvc.ResolveSecretRef(connector.CredentialRef)
			if err != nil {
				result.Processed++
				result.Failed++
				result.LastConnectorID = strings.TrimSpace(connector.ID)
				result.LastVendor = strings.TrimSpace(connector.Vendor)
				result.LastMode = strings.TrimSpace(mode)
				processedCount++
				updatedState, _ := s.hrisPullStateSvc.MarkFailed(tenantID, connector.ID, connector.Vendor, now, err)
				updateEnterpriseHRISPullWorkerStatefulMetrics(result, connector, updatedState, mode, now)
				s.loggerOrDefault().Error(
					"enterprise hris pull worker credential resolution failed",
					"tenant_id", tenantID,
					"connector_id", connector.ID,
					"err", err,
				)
				continue
			}
			credentialValue = resolvedCredential.Value
		}

		pullInput.CredentialValue = credentialValue
		if mode == hris.PullModeIncremental && !hris.SupportsIncrementalPull(adapter, pullInput) {
			mode = hris.PullModeFull
			pullInput.Mode = mode
			claimedState, err := s.hrisPullStateSvc.MarkStarted(tenantID, connector.ID, connector.Vendor, mode, now)
			if err != nil {
				result.Processed++
				result.Failed++
				result.LastConnectorID = strings.TrimSpace(connector.ID)
				result.LastVendor = strings.TrimSpace(connector.Vendor)
				result.LastMode = strings.TrimSpace(mode)
				processedCount++
				s.loggerOrDefault().Error(
					"enterprise hris pull worker fallback to full claim update failed",
					"tenant_id", tenantID,
					"connector_id", connector.ID,
					"err", err,
				)
				continue
			}
			state = claimedState
		}

		if err := s.processEnterpriseHRISPullConnector(connector, credentialValue, state, mode, now); err != nil {
			updatedState, _ := s.hrisPullStateSvc.MarkFailed(tenantID, connector.ID, connector.Vendor, now, err)
			result.Processed++
			result.Failed++
			updateEnterpriseHRISPullWorkerStatefulMetrics(result, connector, updatedState, mode, now)
			processedCount++
			s.loggerOrDefault().Error(
				"enterprise hris pull worker failed",
				"tenant_id", tenantID,
				"connector_id", connector.ID,
				"vendor", connector.Vendor,
				"err", err,
			)
			continue
		}

		result.Processed++
		result.Synced++
		processedCount++
	}

	for tenantID, result := range resultsByTenant {
		if result.Processed == 0 {
			if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 || result.SkippedByInFlight > 0 {
				s.loggerOrDefault().Info(
					"enterprise hris pull worker skipped",
					"tenant_id", tenantID,
					"processed", 0,
					"skipped_in_flight", result.SkippedByInFlight,
					"skipped_attempt_limit", result.SkippedByAttemptLimit,
					"skipped_cooldown", result.SkippedByCooldown,
				)
				if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 {
					s.appendEnterpriseHRISPullWorkerAlertAudit(tenantID, *result, alertFailureThreshold)
				}
			}
			continue
		}
		if shouldAppendEnterpriseHRISPullWorkerAlertAudit(*result, alertFailureThreshold) {
			s.loggerOrDefault().Warn(
				"enterprise hris pull worker alert",
				"tenant_id", tenantID,
				"failed", result.Failed,
				"threshold", alertFailureThreshold,
				"consecutive_failures", result.ConsecutiveFailures,
				"failure_age_seconds", result.FailureAgeSeconds,
			)
			s.appendEnterpriseHRISPullWorkerAlertAudit(tenantID, *result, alertFailureThreshold)
		}
		s.loggerOrDefault().Info(
			"enterprise hris pull worker finished",
			"tenant_id", tenantID,
			"processed", result.Processed,
			"synced", result.Synced,
			"failed", result.Failed,
			"skipped_in_flight", result.SkippedByInFlight,
			"skipped_attempt_limit", result.SkippedByAttemptLimit,
			"skipped_cooldown", result.SkippedByCooldown,
		)
	}
}

func (s *server) processEnterpriseHRISPullConnector(
	connector enterprise.HRISConnector,
	credentialValue string,
	state hris.ConnectorPullState,
	mode string,
	now time.Time,
) error {
	if strings.TrimSpace(connector.CredentialRef) == "" {
		return errors.New("hris connector credential_ref is required for pull sync")
	}
	if strings.TrimSpace(credentialValue) == "" {
		return errors.New("hris connector credential value is required for pull sync")
	}

	timeout := firstNonZeroDuration(s.cfg.ExternalAuthTimeout, 15*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pullResult, err := s.hrisPullRegistry.Pull(ctx, hris.PullInput{
		Connector:         connector,
		CredentialValue:   credentialValue,
		LastSuccessAt:     state.LastSuccessAt,
		LastFullSuccessAt: enterpriseHRISLastFullSuccessAt(connector, state),
		Mode:              mode,
		HTTPClient:        s.hrisHTTPClient,
		Now:               now,
	})
	if err != nil {
		return err
	}

	source := strings.TrimSpace(pullResult.Source)
	if source == "" {
		source = hris.SyncSourceForVendor(connector.Vendor)
	}
	requestID := strings.TrimSpace(pullResult.RequestID)
	if requestID == "" {
		requestID = hris.NormalizeVendor(connector.Vendor) + ":" + strings.TrimSpace(connector.ID) + ":pull:" + now.UTC().Format("20060102t150405z")
	}
	pullMode := hris.NormalizePullMode(pullResult.Mode)
	if pullMode == "" {
		pullMode = hris.NormalizePullMode(mode)
	}

	inputs := buildEnterpriseHRISPullReconcileInputs(
		s.enterpriseSvc.ListEmployees(connector.TenantID),
		source,
		pullResult.Employees,
		pullMode == hris.PullModeFull,
	)
	if len(inputs) > 0 {
		_, _, _, _, err = s.enterpriseSvc.SyncEmployeesWithAccessUpsertMetadata(
			connector.TenantID,
			source,
			hris.SyncActor,
			requestID,
			connector.ID,
			enterpriseHRISPullRawPayloadRef(requestID),
			inputs,
			func(items []enterprise.EnterpriseEmployee) (int, int, int, error) {
				accessInputs := enterpriseEmployeesToAccessBatchInputs(items)
				return s.accessSvc.UpsertUsersByEmail(connector.TenantID, accessInputs)
			},
		)
		if err != nil {
			return err
		}
	}

	if _, err := s.enterpriseSvc.MarkHRISConnectorSynced(connector.TenantID, connector.ID, now); err != nil {
		return err
	}
	if _, err := s.hrisPullStateSvc.MarkSucceeded(connector.TenantID, connector.ID, connector.Vendor, pullMode, requestID, now); err != nil {
		return err
	}
	return nil
}

func enterpriseHRISPullConnectorCandidate(connector enterprise.HRISConnector) bool {
	if strings.TrimSpace(connector.Status) != "active" {
		return false
	}
	switch strings.TrimSpace(connector.SyncStrategy) {
	case "pull", "hybrid", "":
		return true
	default:
		return false
	}
}

func enterpriseHRISPullMode(
	connector enterprise.HRISConnector,
	state hris.ConnectorPullState,
	now time.Time,
	reconcileInterval time.Duration,
) string {
	lastSuccessAt := enterpriseHRISLastSuccessAt(connector, state)
	if lastSuccessAt == nil {
		return hris.PullModeFull
	}
	if enterpriseHRISFullReconcileDue(connector, state, now, reconcileInterval) {
		return hris.PullModeFull
	}
	return hris.PullModeIncremental
}

func enterpriseHRISLastSuccessAt(
	connector enterprise.HRISConnector,
	state hris.ConnectorPullState,
) *time.Time {
	lastSuccessAt := cloneTimePointerLocal(connector.LastSyncAt)
	if state.LastSuccessAt != nil {
		if lastSuccessAt == nil || state.LastSuccessAt.After(*lastSuccessAt) {
			lastSuccessAt = cloneTimePointerLocal(state.LastSuccessAt)
		}
	}
	return lastSuccessAt
}

func enterpriseHRISLastFullSuccessAt(
	connector enterprise.HRISConnector,
	state hris.ConnectorPullState,
) *time.Time {
	if state.LastFullSuccessAt != nil {
		return cloneTimePointerLocal(state.LastFullSuccessAt)
	}
	if state.LastSuccessAt == nil && connector.LastSyncAt != nil {
		return cloneTimePointerLocal(connector.LastSyncAt)
	}
	return nil
}

func enterpriseHRISFullReconcileDue(
	connector enterprise.HRISConnector,
	state hris.ConnectorPullState,
	now time.Time,
	reconcileInterval time.Duration,
) bool {
	lastFullSuccessAt := enterpriseHRISLastFullSuccessAt(connector, state)
	if lastFullSuccessAt == nil {
		return true
	}
	if reconcileInterval <= 0 {
		reconcileInterval = 24 * time.Hour
	}
	return !lastFullSuccessAt.Add(reconcileInterval).After(now)
}

func buildEnterpriseHRISPullReconcileInputs(
	existing []enterprise.EnterpriseEmployee,
	source string,
	pulled []enterprise.EmployeeSyncInput,
	fullSnapshot bool,
) []enterprise.EmployeeSyncInput {
	output := make([]enterprise.EmployeeSyncInput, 0, len(pulled))
	seenExternalIDs := make(map[string]struct{}, len(pulled))
	seenEmails := make(map[string]struct{}, len(pulled))
	for i := range pulled {
		item := pulled[i]
		if nextExternalID := strings.TrimSpace(item.ExternalID); nextExternalID != "" {
			seenExternalIDs[nextExternalID] = struct{}{}
		}
		if nextEmail := strings.ToLower(strings.TrimSpace(item.Email)); nextEmail != "" {
			seenEmails[nextEmail] = struct{}{}
		}
		output = append(output, item)
	}
	if !fullSnapshot {
		return output
	}

	nextSource := strings.TrimSpace(source)
	for i := range existing {
		item := existing[i]
		if strings.TrimSpace(item.Source) != nextSource {
			continue
		}
		externalID := strings.TrimSpace(item.ExternalID)
		if externalID != "" {
			if _, ok := seenExternalIDs[externalID]; ok {
				continue
			}
		} else {
			email := strings.ToLower(strings.TrimSpace(item.Email))
			if email == "" {
				continue
			}
			if _, ok := seenEmails[email]; ok {
				continue
			}
		}
		if strings.TrimSpace(item.Status) == "inactive" && enterprise.EmploymentStatusBlocksSession(item.EmploymentStatus) {
			continue
		}

		output = append(output, enterprise.EmployeeSyncInput{
			ExternalID:        item.ExternalID,
			EmployeeNumber:    item.EmployeeNumber,
			Email:             item.Email,
			FullName:          item.FullName,
			Department:        item.Department,
			JobTitle:          item.JobTitle,
			Location:          item.Location,
			Phone:             item.Phone,
			ManagerExternalID: item.ManagerExternalID,
			EmploymentStatus:  "inactive",
			JoinDate:          item.JoinDate,
			ResignDate:        firstNonEmptyString(item.ResignDate, time.Now().UTC().Format("2006-01-02")),
			ShiftCode:         item.ShiftCode,
			ScheduleWindow:    item.ScheduleWindow,
			LeaveStatus:       item.LeaveStatus,
			CostCenter:        item.CostCenter,
			PhotoURL:          item.PhotoURL,
			Status:            "inactive",
		})
	}
	return output
}

func enterpriseHRISPullRawPayloadRef(requestID string) string {
	nextRequestID := strings.TrimSpace(requestID)
	if nextRequestID == "" {
		return ""
	}
	return "hris_pull_run:" + nextRequestID
}

func cloneTimePointerLocal(input *time.Time) *time.Time {
	if input == nil {
		return nil
	}
	value := *input
	return &value
}

func shouldAppendEnterpriseWorkerAlertAudit(
	processed int,
	failed int,
	alertFailureThreshold int,
	skippedByAttemptLimit int,
	skippedByCooldown int,
) bool {
	if failed >= alertFailureThreshold {
		return true
	}
	return processed == 0 && (skippedByAttemptLimit > 0 || skippedByCooldown > 0)
}

func enterpriseHRISPullFailureAgeSeconds(now time.Time, lastFailureAt *time.Time) int {
	if lastFailureAt == nil {
		return 0
	}
	if now.Before(*lastFailureAt) {
		return 0
	}
	return int(now.Sub(*lastFailureAt).Seconds())
}

func updateEnterpriseHRISPullWorkerStatefulMetrics(
	result *enterpriseHRISPullWorkerResult,
	connector enterprise.HRISConnector,
	state hris.ConnectorPullState,
	mode string,
	now time.Time,
) {
	if result == nil {
		return
	}
	consecutiveFailures := state.ConsecutiveFailures
	failureAgeSeconds := enterpriseHRISPullFailureAgeSeconds(now, state.LastFailureAt)
	if consecutiveFailures <= 0 && failureAgeSeconds <= 0 {
		return
	}
	if consecutiveFailures < result.ConsecutiveFailures {
		return
	}
	if consecutiveFailures == result.ConsecutiveFailures && failureAgeSeconds <= result.FailureAgeSeconds {
		return
	}
	result.ConsecutiveFailures = consecutiveFailures
	result.FailureAgeSeconds = failureAgeSeconds
	result.LastConnectorID = strings.TrimSpace(connector.ID)
	result.LastVendor = strings.TrimSpace(connector.Vendor)
	result.LastMode = strings.TrimSpace(mode)
}

func shouldAppendEnterpriseHRISPullWorkerAlertAudit(
	result enterpriseHRISPullWorkerResult,
	alertFailureThreshold int,
) bool {
	if shouldAppendEnterpriseWorkerAlertAudit(
		result.Processed,
		result.Failed,
		alertFailureThreshold,
		result.SkippedByAttemptLimit,
		result.SkippedByCooldown,
	) {
		return true
	}
	threshold := alertFailureThreshold
	if threshold <= 0 {
		threshold = 1
	}
	return result.ConsecutiveFailures >= threshold
}

func (s *server) appendEnterpriseSyncWorkerAlertAudit(
	tenantID string,
	result enterprise.BatchPendingSyncReconcileResult,
	alertFailureThreshold int,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	if !shouldAppendEnterpriseWorkerAlertAudit(
		result.Processed,
		result.Failed,
		alertFailureThreshold,
		result.SkippedByAttemptLimit,
		result.SkippedByCooldown,
	) {
		return
	}
	target := strings.TrimSpace(
		fmt.Sprintf(
			"failed=%d threshold=%d processed=%d applied=%d skipped_attempt_limit=%d skipped_cooldown=%d",
			result.Failed,
			alertFailureThreshold,
			result.Processed,
			result.Applied,
			result.SkippedByAttemptLimit,
			result.SkippedByCooldown,
		),
	)
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_sync_reconcile_worker_alert",
		target,
		"enterprise_sync_worker",
	)
}

func (s *server) appendEnterpriseSyncWorkerAlertAutoRetryWorkerAudit(
	tenantID string,
	result enterpriseSyncWorkerAlertAutoRetryWorkerResult,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	target := strings.TrimSpace(
		fmt.Sprintf(
			"processed=%d retried=%d failed=%d skipped=%d suppressed=%d",
			result.Processed,
			result.Retried,
			result.Failed,
			result.Skipped,
			result.Suppressed,
		),
	)
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_sync_worker_alert_auto_retry_worker_completed",
		target,
		"enterprise_sync_worker",
	)
}

func (s *server) appendEnterpriseHRISWebhookReceiptWorkerAlertAudit(
	tenantID string,
	result enterpriseHRISWebhookReceiptWorkerResult,
	alertFailureThreshold int,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	if !shouldAppendEnterpriseWorkerAlertAudit(
		result.Processed,
		result.Failed,
		alertFailureThreshold,
		result.SkippedByAttemptLimit,
		result.SkippedByCooldown,
	) {
		return
	}
	targetParts := []string{
		fmt.Sprintf("failed=%d", result.Failed),
		fmt.Sprintf("threshold=%d", alertFailureThreshold),
		fmt.Sprintf("processed=%d", result.Processed),
		fmt.Sprintf("synced=%d", result.Synced),
		fmt.Sprintf("skipped=%d", result.Skipped),
		fmt.Sprintf("skipped_in_flight=%d", result.SkippedByInFlight),
		fmt.Sprintf("skipped_attempt_limit=%d", result.SkippedByAttemptLimit),
		fmt.Sprintf("skipped_cooldown=%d", result.SkippedByCooldown),
	}
	if nextConnectorID := strings.TrimSpace(result.LastConnectorID); nextConnectorID != "" {
		targetParts = append(targetParts, "connector_id="+nextConnectorID)
	}
	if nextVendor := strings.TrimSpace(result.LastVendor); nextVendor != "" {
		targetParts = append(targetParts, "vendor="+nextVendor)
	}
	if nextRequestID := strings.TrimSpace(result.LastRequestID); nextRequestID != "" {
		targetParts = append(targetParts, "request_id="+nextRequestID)
	}
	if nextEventType := strings.TrimSpace(result.LastEventType); nextEventType != "" {
		targetParts = append(targetParts, "event_type="+nextEventType)
	}
	target := strings.Join(targetParts, " ")
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_webhook_receipt_worker_alert",
		target,
		"enterprise_sync_worker",
	)
}

func (s *server) appendEnterpriseHRISWebhookDLQWorkerAlertAudit(
	tenantID string,
	result enterpriseHRISWebhookDLQWorkerResult,
	alertFailureThreshold int,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	if !shouldAppendEnterpriseWorkerAlertAudit(
		result.Processed,
		result.Failed,
		alertFailureThreshold,
		result.SkippedByAttemptLimit,
		result.SkippedByCooldown,
	) {
		return
	}
	targetParts := []string{
		fmt.Sprintf("failed=%d", result.Failed),
		fmt.Sprintf("threshold=%d", alertFailureThreshold),
		fmt.Sprintf("processed=%d", result.Processed),
		fmt.Sprintf("replayed=%d", result.Replayed),
		fmt.Sprintf("skipped_in_flight=%d", result.SkippedByInFlight),
		fmt.Sprintf("skipped_attempt_limit=%d", result.SkippedByAttemptLimit),
		fmt.Sprintf("skipped_cooldown=%d", result.SkippedByCooldown),
	}
	if nextConnectorID := strings.TrimSpace(result.LastConnectorID); nextConnectorID != "" {
		targetParts = append(targetParts, "connector_id="+nextConnectorID)
	}
	if nextVendor := strings.TrimSpace(result.LastVendor); nextVendor != "" {
		targetParts = append(targetParts, "vendor="+nextVendor)
	}
	if nextRequestID := strings.TrimSpace(result.LastRequestID); nextRequestID != "" {
		targetParts = append(targetParts, "request_id="+nextRequestID)
	}
	if nextEventType := strings.TrimSpace(result.LastEventType); nextEventType != "" {
		targetParts = append(targetParts, "event_type="+nextEventType)
	}
	if nextFailureStage := strings.TrimSpace(result.LastFailureStage); nextFailureStage != "" {
		targetParts = append(targetParts, "failure_stage="+nextFailureStage)
	}
	target := strings.Join(targetParts, " ")
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_webhook_dlq_worker_alert",
		target,
		"enterprise_sync_worker",
	)
}

func (s *server) appendEnterpriseHRISPullWorkerAlertAudit(
	tenantID string,
	result enterpriseHRISPullWorkerResult,
	alertFailureThreshold int,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	if !shouldAppendEnterpriseHRISPullWorkerAlertAudit(result, alertFailureThreshold) {
		return
	}
	targetParts := []string{
		fmt.Sprintf("failed=%d", result.Failed),
		fmt.Sprintf("threshold=%d", alertFailureThreshold),
		fmt.Sprintf("processed=%d", result.Processed),
		fmt.Sprintf("synced=%d", result.Synced),
		fmt.Sprintf("consecutive_failures=%d", result.ConsecutiveFailures),
		fmt.Sprintf("failure_age_seconds=%d", result.FailureAgeSeconds),
		fmt.Sprintf("skipped_in_flight=%d", result.SkippedByInFlight),
		fmt.Sprintf("skipped_attempt_limit=%d", result.SkippedByAttemptLimit),
		fmt.Sprintf("skipped_cooldown=%d", result.SkippedByCooldown),
	}
	if nextConnectorID := strings.TrimSpace(result.LastConnectorID); nextConnectorID != "" {
		targetParts = append(targetParts, "connector_id="+nextConnectorID)
	}
	if nextVendor := strings.TrimSpace(result.LastVendor); nextVendor != "" {
		targetParts = append(targetParts, "vendor="+nextVendor)
	}
	if nextMode := strings.TrimSpace(result.LastMode); nextMode != "" {
		targetParts = append(targetParts, "mode="+nextMode)
	}
	target := strings.Join(targetParts, " ")
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_pull_worker_alert",
		target,
		"enterprise_sync_worker",
	)
}

func (s *server) appendEnterpriseHRISWebhookProcessingAlertAudit(
	tenantID string,
	connectorID string,
	vendor string,
	eventType string,
	requestID string,
	failureStage string,
) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}
	target := strings.TrimSpace(
		fmt.Sprintf(
			"failed=1 threshold=1 processed=1 applied=0 connector_id=%s vendor=%s event_type=%s request_id=%s failure_stage=%s",
			strings.TrimSpace(connectorID),
			strings.TrimSpace(vendor),
			strings.TrimSpace(eventType),
			strings.TrimSpace(requestID),
			strings.TrimSpace(failureStage),
		),
	)
	_, _ = s.auditSvc.Append(
		nextTenantID,
		"enterprise_sync_worker",
		"system",
		"enterprise_hris_webhook_processing_alert",
		target,
		"enterprise_sync_worker",
	)
}

func pendingSyncWorkerAlertAutoRetryTenantIDs(
	items []enterprise.SyncWorkerAlertNotification,
	now time.Time,
) []string {
	set := make(map[string]struct{})
	for i := range items {
		if items[i].Status != "failed" || !items[i].Retryable || items[i].NextRetryAt == nil {
			continue
		}
		if items[i].NextRetryAt.After(now) {
			continue
		}
		tenantID := strings.TrimSpace(items[i].TenantID)
		if tenantID == "" {
			continue
		}
		set[tenantID] = struct{}{}
	}
	tenantIDs := make([]string, 0, len(set))
	for tenantID := range set {
		tenantIDs = append(tenantIDs, tenantID)
	}
	sort.Strings(tenantIDs)
	return tenantIDs
}

func pendingSyncRequestTenantIDs(records []enterprise.SyncRequestRecord) []string {
	set := make(map[string]struct{})
	for i := range records {
		if records[i].AccessApplied {
			continue
		}
		nextTenantID := strings.TrimSpace(records[i].TenantID)
		if nextTenantID == "" {
			continue
		}
		set[nextTenantID] = struct{}{}
	}

	items := make([]string, 0, len(set))
	for tenantID := range set {
		items = append(items, tenantID)
	}
	sort.Strings(items)
	return items
}

func pendingHRISWebhookReceiptTenantIDs(items []enterprise.HRISWebhookReceipt) []string {
	set := make(map[string]struct{})
	for i := range items {
		status := strings.TrimSpace(items[i].Status)
		if status != "received" && status != "failed" && status != "processing" {
			continue
		}
		nextTenantID := strings.TrimSpace(items[i].TenantID)
		if nextTenantID == "" {
			continue
		}
		set[nextTenantID] = struct{}{}
	}

	output := make([]string, 0, len(set))
	for tenantID := range set {
		output = append(output, tenantID)
	}
	sort.Strings(output)
	return output
}

func groupHRISWebhookReceiptsByTenant(items []enterprise.HRISWebhookReceipt) map[string][]enterprise.HRISWebhookReceipt {
	grouped := make(map[string][]enterprise.HRISWebhookReceipt)
	for i := range items {
		tenantID := strings.TrimSpace(items[i].TenantID)
		if tenantID == "" {
			continue
		}
		grouped[tenantID] = append(grouped[tenantID], items[i])
	}
	return grouped
}

func pendingHRISWebhookDLQTenantIDs(
	entries []hris.DeadLetterEntry,
	_ int,
	_ time.Duration,
	_ time.Time,
) []string {
	set := make(map[string]struct{})
	for i := range entries {
		status := strings.TrimSpace(entries[i].Status)
		if status != "dlq" && status != "replaying" {
			continue
		}
		// Keep cooldown/attempt-limit and in-flight replaying entries in scope so skip-only ticks
		// can still emit the expected worker logs/audit.
		nextTenantID := strings.TrimSpace(entries[i].TenantID)
		if nextTenantID == "" {
			continue
		}
		set[nextTenantID] = struct{}{}
	}

	items := make([]string, 0, len(set))
	for tenantID := range set {
		items = append(items, tenantID)
	}
	sort.Strings(items)
	return items
}

func groupHRISWebhookDLQEntriesByTenant(entries []hris.DeadLetterEntry) map[string][]hris.DeadLetterEntry {
	grouped := make(map[string][]hris.DeadLetterEntry)
	for i := range entries {
		tenantID := strings.TrimSpace(entries[i].TenantID)
		if tenantID == "" {
			continue
		}
		grouped[tenantID] = append(grouped[tenantID], entries[i])
	}
	return grouped
}

func pendingJITApprovalExternalSyncTenantIDs(
	records []enterprise.JITProvisionApproval,
	maxAttempts int,
	retryCooldown time.Duration,
	now time.Time,
) []string {
	set := make(map[string]struct{})
	for i := range records {
		item := records[i]
		if !enterpriseJITApprovalExternalSyncCandidate(item) {
			continue
		}
		syncStatus := strings.TrimSpace(item.ExternalSyncStatus)
		if syncStatus == "failed" {
			if maxAttempts > 0 && item.ExternalSyncAttemptCount >= maxAttempts {
				continue
			}
			if retryCooldown > 0 && item.ExternalSyncUpdatedAt != nil {
				retryReadyAt := item.ExternalSyncUpdatedAt.Add(retryCooldown)
				if retryReadyAt.After(now) {
					continue
				}
			}
		}
		nextTenantID := strings.TrimSpace(item.TenantID)
		if nextTenantID == "" {
			continue
		}
		set[nextTenantID] = struct{}{}
	}

	items := make([]string, 0, len(set))
	for tenantID := range set {
		items = append(items, tenantID)
	}
	sort.Strings(items)
	return items
}

func bearerToken(authHeader string) (string, error) {
	header := strings.TrimSpace(authHeader)
	if !strings.HasPrefix(header, "Bearer ") {
		return "", errors.New("missing bearer token")
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" {
		return "", errors.New("missing bearer token")
	}

	return token, nil
}

func authenticatedUser(r *http.Request) (auth.User, bool) {
	value := r.Context().Value(authUserContextKey)
	user, ok := value.(auth.User)
	return user, ok
}

func firstNonEmptyString(items ...string) string {
	for i := range items {
		next := strings.TrimSpace(items[i])
		if next != "" {
			return next
		}
	}
	return ""
}

func firstNonZeroDuration(items ...time.Duration) time.Duration {
	for i := range items {
		if items[i] > 0 {
			return items[i]
		}
	}
	return 0
}

func (s *server) requireRoles(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for i := range roles {
		role := strings.ToLower(strings.TrimSpace(roles[i]))
		if role == "" {
			continue
		}
		allowed[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := authenticatedUser(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "invalid access token")
				return
			}
			role := strings.ToLower(strings.TrimSpace(user.Role))
			if _, exists := allowed[role]; !exists {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *server) resolveTenantID(w http.ResponseWriter, r *http.Request, requestedTenantID string) (string, bool) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return "", false
	}

	requested := strings.TrimSpace(requestedTenantID)
	role := strings.ToLower(strings.TrimSpace(user.Role))
	if role == "super_admin" {
		return requested, true
	}

	tokenTenantID := strings.TrimSpace(user.TenantID)
	if tokenTenantID == "" {
		writeError(w, http.StatusForbidden, "forbidden")
		return "", false
	}
	if requested != "" && requested != tokenTenantID {
		writeError(w, http.StatusForbidden, "tenant scope forbidden")
		return "", false
	}

	return tokenTenantID, true
}

func (s *server) buildingScopeForRequest(w http.ResponseWriter, r *http.Request) (map[string]struct{}, bool) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return nil, false
	}

	role := strings.ToLower(strings.TrimSpace(user.Role))
	if role != "building_admin" {
		return nil, true
	}

	scope := normalizeBuildingScope(user.BuildingIDs)
	if scope == nil {
		scope = map[string]struct{}{}
	}
	for buildingID := range s.buildingScopeFromRoleAssignments(user) {
		scope[buildingID] = struct{}{}
	}
	if len(scope) == 0 {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return nil, false
	}

	return scope, true
}

func (s *server) buildingScopeFromRoleAssignments(user auth.User) map[string]struct{} {
	scope := map[string]struct{}{}
	if s == nil || s.accessSvc == nil {
		return scope
	}
	tenantID := strings.TrimSpace(user.TenantID)
	if tenantID == "" {
		return scope
	}

	userID := strings.TrimSpace(user.ID)
	userEmail := strings.ToLower(strings.TrimSpace(user.Email))
	userTeamIDs := map[string]struct{}{}
	for _, membership := range s.accessSvc.ListTeamMemberships(tenantID) {
		if strings.EqualFold(strings.TrimSpace(membership.MemberType), "User") &&
			((userID != "" && strings.TrimSpace(membership.MemberID) == userID) ||
				(userEmail != "" && strings.EqualFold(strings.TrimSpace(membership.MemberEmail), userEmail))) {
			if teamID := strings.TrimSpace(membership.TeamID); teamID != "" {
				userTeamIDs[teamID] = struct{}{}
			}
		}
	}

	for _, assignment := range s.accessSvc.ListRoleAssignments(tenantID) {
		if strings.TrimSpace(assignment.RoleID) != "role_place_admin" ||
			!strings.EqualFold(strings.TrimSpace(assignment.AppliesToType), "Place") {
			continue
		}
		placeID := strings.TrimSpace(assignment.AppliesToID)
		if placeID == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(assignment.AssigneeType)) {
		case "user":
			if (userID != "" && strings.TrimSpace(assignment.AssigneeID) == userID) ||
				(userEmail != "" && strings.EqualFold(strings.TrimSpace(assignment.AssigneeEmail), userEmail)) {
				scope[placeID] = struct{}{}
			}
		case "team":
			if _, exists := userTeamIDs[strings.TrimSpace(assignment.AssigneeID)]; exists {
				scope[placeID] = struct{}{}
			}
		}
	}

	return scope
}

func (s *server) requireBuildingScope(w http.ResponseWriter, scope map[string]struct{}, buildingID string) bool {
	if scope == nil {
		return true
	}
	nextBuildingID := strings.TrimSpace(buildingID)
	if nextBuildingID == "" {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return false
	}
	if _, exists := scope[nextBuildingID]; !exists {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return false
	}

	return true
}

func normalizeBuildingScope(buildingIDs []string) map[string]struct{} {
	scope := make(map[string]struct{}, len(buildingIDs))
	for i := range buildingIDs {
		nextBuildingID := strings.TrimSpace(buildingIDs[i])
		if nextBuildingID == "" {
			continue
		}
		scope[nextBuildingID] = struct{}{}
	}
	return scope
}

func filterTopologyByBuildingScope(input space.TenantTopology, scope map[string]struct{}) space.TenantTopology {
	return space.TenantTopology{
		TenantID:  input.TenantID,
		Buildings: filterBuildingsByScope(input.Buildings, scope),
		Floors:    filterFloorsByScope(input.Floors, scope),
		Areas:     filterAreasByScope(input.Areas, scope),
		Doors:     filterDoorsByScope(input.Doors, scope),
	}
}

func filterBuildingsByScope(items []space.Building, scope map[string]struct{}) []space.Building {
	filtered := make([]space.Building, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].ID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterFloorsByScope(items []space.Floor, scope map[string]struct{}) []space.Floor {
	filtered := make([]space.Floor, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterAreasByScope(items []space.Area, scope map[string]struct{}) []space.Area {
	filtered := make([]space.Area, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterDoorsByScope(items []space.Door, scope map[string]struct{}) []space.Door {
	filtered := make([]space.Door, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func allowedDoorIDsByBuildingScope(doors []space.Door, scope map[string]struct{}) map[string]struct{} {
	allowed := make(map[string]struct{}, len(doors))
	for i := range doors {
		if _, exists := scope[doors[i].BuildingID]; !exists {
			continue
		}
		allowed[doors[i].ID] = struct{}{}
	}
	return allowed
}

func filterDoorGroupsByScope(items []space.DoorGroup, allowedDoorIDs map[string]struct{}) []space.DoorGroup {
	filtered := make([]space.DoorGroup, 0, len(items))
	for i := range items {
		nextDoorIDs := make([]string, 0, len(items[i].DoorIDs))
		for j := range items[i].DoorIDs {
			if _, exists := allowedDoorIDs[items[i].DoorIDs[j]]; !exists {
				continue
			}
			nextDoorIDs = append(nextDoorIDs, items[i].DoorIDs[j])
		}
		if len(nextDoorIDs) == 0 {
			continue
		}
		record := items[i]
		record.DoorIDs = nextDoorIDs
		filtered = append(filtered, record)
	}
	return filtered
}

func filterGatewaysByScope(items []gateway.Gateway, scope map[string]struct{}) []gateway.Gateway {
	filtered := make([]gateway.Gateway, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterPoliciesByScope(items []access.Policy, scope map[string]struct{}) []access.Policy {
	filtered := make([]access.Policy, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterTemporaryAccessByScope(items []access.TemporaryAccess, scope map[string]struct{}) []access.TemporaryAccess {
	filtered := make([]access.TemporaryAccess, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterUsersByScope(items []access.AccessUser, scope map[string]struct{}) []access.AccessUser {
	filtered := make([]access.AccessUser, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterUserGroupsByScope(items []access.UserGroup, scope map[string]struct{}) []access.UserGroup {
	filtered := make([]access.UserGroup, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterVisitorPassesByScope(items []access.VisitorPass, scope map[string]struct{}) []access.VisitorPass {
	filtered := make([]access.VisitorPass, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterAccessEventsByScope(items []event.AccessEvent, scope map[string]struct{}) []event.AccessEvent {
	filtered := make([]event.AccessEvent, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterDeviceEventsByScope(items []event.DeviceEvent, scope map[string]struct{}) []event.DeviceEvent {
	filtered := make([]event.DeviceEvent, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterAlarmsByScope(items []alarm.Alarm, scope map[string]struct{}) []alarm.Alarm {
	filtered := make([]alarm.Alarm, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].BuildingID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func (s *server) appendAuditLog(r *http.Request, tenantID, action, target, source string) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" || s.auditSvc == nil {
		return
	}

	actor := "system"
	role := "system"
	if r != nil {
		if user, ok := authenticatedUser(r); ok {
			if strings.TrimSpace(user.Email) != "" {
				actor = strings.TrimSpace(user.Email)
			}
			if strings.TrimSpace(user.Role) != "" {
				role = strings.TrimSpace(user.Role)
			}
		}
	}

	record, err := s.auditSvc.Append(
		nextTenantID,
		actor,
		role,
		strings.TrimSpace(action),
		strings.TrimSpace(target),
		strings.TrimSpace(source),
	)
	if err != nil {
		return
	}
	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
	}
	s.publishInternalEvent(
		ctx,
		"audit.log.appended",
		map[string]any{
			"tenant_id": nextTenantID,
			"log":       record,
		},
		map[string]string{
			"tenant_id": nextTenantID,
			"action":    strings.TrimSpace(action),
			"source":    strings.TrimSpace(source),
		},
	)
}

func (s *server) publishInternalEvent(
	ctx context.Context,
	subject string,
	payload any,
	headers map[string]string,
) {
	if s.messageBus == nil {
		return
	}
	if err := s.messageBus.PublishJSON(ctx, subject, payload, headers); err != nil {
		s.loggerOrDefault().Warn(
			"publish internal event failed",
			"subject", strings.TrimSpace(subject),
			"err", err,
		)
	}
}

func (s *server) findGatewayByTenant(tenantID, gatewayID string) (gateway.Gateway, bool) {
	items := s.gatewaySvc.List(strings.TrimSpace(tenantID))
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" {
		return gateway.Gateway{}, false
	}

	for i := range items {
		if items[i].ID == nextGatewayID {
			return items[i], true
		}
	}
	return gateway.Gateway{}, false
}

func (s *server) setGatewayDeviceToken(gatewayID, deviceToken string) {
	nextGatewayID := strings.TrimSpace(gatewayID)
	nextDeviceToken := strings.TrimSpace(deviceToken)
	if nextGatewayID == "" || nextDeviceToken == "" {
		return
	}
	if s.gatewayTokenStore != nil {
		if err := s.gatewayTokenStore.UpsertGatewayDeviceToken(nextGatewayID, nextDeviceToken); err != nil {
			s.loggerOrDefault().Error(
				"gateway bootstrap token persist failed",
				"gateway_id", nextGatewayID,
				"err", err,
			)
		}
		return
	}
	s.gatewayTokenMu.Lock()
	defer s.gatewayTokenMu.Unlock()
	if s.gatewayDeviceTokens == nil {
		s.gatewayDeviceTokens = map[string]string{}
	}
	s.gatewayDeviceTokens[nextGatewayID] = nextDeviceToken
	if err := s.persistGatewayBootstrapStateLocked(); err != nil {
		s.loggerOrDefault().Error(
			"gateway bootstrap token persist failed",
			"gateway_id", nextGatewayID,
			"err", err,
		)
	}
}

func (s *server) setGatewayAuthzCacheAckVersion(gatewayID, version string) {
	nextGatewayID := strings.TrimSpace(gatewayID)
	nextVersion := strings.TrimSpace(version)
	if nextGatewayID == "" || nextVersion == "" {
		return
	}
	s.gatewayAuthzAckMu.Lock()
	defer s.gatewayAuthzAckMu.Unlock()
	if s.gatewayAuthzAckVersion == nil {
		s.gatewayAuthzAckVersion = map[string]string{}
	}
	s.gatewayAuthzAckVersion[nextGatewayID] = nextVersion
}

func (s *server) gatewayAuthzCacheAckVersion(gatewayID string) string {
	nextGatewayID := strings.TrimSpace(gatewayID)
	if nextGatewayID == "" {
		return ""
	}
	s.gatewayAuthzAckMu.RLock()
	defer s.gatewayAuthzAckMu.RUnlock()
	if s.gatewayAuthzAckVersion == nil {
		return ""
	}
	return strings.TrimSpace(s.gatewayAuthzAckVersion[nextGatewayID])
}

func (s *server) authorizeGatewayDeviceToken(w http.ResponseWriter, r *http.Request, gatewayID string) bool {
	provided := strings.TrimSpace(r.Header.Get("X-Device-Token"))
	if provided == "" {
		if token, err := bearerToken(r.Header.Get("Authorization")); err == nil {
			provided = token
		}
	}
	if provided == "" {
		writeError(w, http.StatusUnauthorized, "missing device token")
		return false
	}
	if s.gatewayTokenStore != nil {
		exists, matched, err := s.gatewayTokenStore.VerifyGatewayDeviceToken(strings.TrimSpace(gatewayID), provided)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "device token verification failed")
			return false
		}
		if exists && matched {
			return true
		}
		// Fallback: accept bootstrap token for unregistered/demo devices
		bootstrapToken := strings.TrimSpace(s.cfg.GatewayBootstrapToken)
		if bootstrapToken != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(bootstrapToken)) == 1 {
			return true
		}
		if !exists {
			writeError(w, http.StatusUnauthorized, "device not registered")
		} else {
			writeError(w, http.StatusUnauthorized, "invalid device token")
		}
		return false
	}

	nextGatewayID := strings.TrimSpace(gatewayID)
	s.gatewayTokenMu.RLock()
	expected, exists := s.gatewayDeviceTokens[nextGatewayID]
	s.gatewayTokenMu.RUnlock()
	if exists && strings.TrimSpace(expected) != "" {
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1 {
			return true
		}
	}

	// Fallback: accept bootstrap token for unregistered/demo devices
	bootstrapToken := strings.TrimSpace(s.cfg.GatewayBootstrapToken)
	if bootstrapToken != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(bootstrapToken)) == 1 {
		return true
	}

	if !exists || strings.TrimSpace(expected) == "" {
		writeError(w, http.StatusUnauthorized, "device not registered")
	} else {
		writeError(w, http.StatusUnauthorized, "invalid device token")
	}
	return false
}

func (s *server) authorizeGatewayBootstrapToken(w http.ResponseWriter, r *http.Request) bool {
	expected := strings.TrimSpace(s.cfg.GatewayBootstrapToken)
	if expected == "" {
		writeError(w, http.StatusServiceUnavailable, "gateway bootstrap token is not configured")
		return false
	}

	provided := strings.TrimSpace(r.Header.Get("X-Bootstrap-Token"))
	if provided == "" {
		if token, err := bearerToken(r.Header.Get("Authorization")); err == nil {
			provided = token
		}
	}
	if provided == "" {
		writeError(w, http.StatusUnauthorized, "missing gateway bootstrap token")
		return false
	}
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid gateway bootstrap token")
		return false
	}

	return true
}

// validateGatewayRequestNonce checks the X-Request-Nonce and X-Request-Timestamp headers
// for replay protection. Returns true if valid (or if headers are absent for backwards compat).
func (s *server) validateGatewayRequestNonce(w http.ResponseWriter, r *http.Request) bool {
	nonce := strings.TrimSpace(r.Header.Get("X-Request-Nonce"))
	tsRaw := strings.TrimSpace(r.Header.Get("X-Request-Timestamp"))

	// Backwards compatible: if no nonce headers, allow (old agents)
	if nonce == "" && tsRaw == "" {
		return true
	}

	// If one is present, both must be present
	if nonce == "" || tsRaw == "" {
		writeError(w, http.StatusBadRequest, "X-Request-Nonce and X-Request-Timestamp must both be present")
		return false
	}

	// Validate timestamp within 5-minute window
	ts, err := time.Parse(time.RFC3339, tsRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid X-Request-Timestamp format")
		return false
	}
	const nonceWindow = 5 * time.Minute
	now := time.Now().UTC()
	if now.Sub(ts) > nonceWindow || ts.Sub(now) > nonceWindow {
		writeError(w, http.StatusUnauthorized, "request timestamp outside acceptable window")
		return false
	}

	// Check nonce uniqueness
	s.gatewayNonceMu.Lock()
	// Clean expired nonces first
	for k, exp := range s.gatewayNonces {
		if now.After(exp) {
			delete(s.gatewayNonces, k)
		}
	}
	if _, exists := s.gatewayNonces[nonce]; exists {
		s.gatewayNonceMu.Unlock()
		writeError(w, http.StatusConflict, "duplicate request nonce")
		return false
	}
	s.gatewayNonces[nonce] = now.Add(nonceWindow)
	s.gatewayNonceMu.Unlock()

	return true
}

func randomHexID(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func resolveHRISVaultMasterKey(raw string) (string, error) {
	next := strings.TrimSpace(raw)
	if next != "" {
		return next, nil
	}
	return randomHexID(32)
}

func parseSerialInventoryCSV(content string) ([]gateway.SerialInventoryImportItem, error) {
	nextContent := strings.TrimSpace(content)
	if nextContent == "" {
		return nil, gateway.ErrSerialInventoryRecordsRequired
	}

	reader := csv.NewReader(strings.NewReader(nextContent))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gateway.ErrSerialInventoryRecordsRequired
	}

	start := 0
	if isSerialInventoryCSVHeader(rows[0]) {
		start = 1
	}

	records := make([]gateway.SerialInventoryImportItem, 0, len(rows)-start)
	for i := start; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}

		serialNumber := strings.TrimSpace(row[0])
		productType := ""
		batchCode := ""
		source := ""

		if len(row) > 1 {
			productType = strings.TrimSpace(row[1])
		}
		if len(row) > 2 {
			batchCode = strings.TrimSpace(row[2])
		}
		if len(row) > 3 {
			source = strings.TrimSpace(row[3])
		}

		if serialNumber == "" && productType == "" {
			continue
		}

		records = append(records, gateway.SerialInventoryImportItem{
			SerialNumber: serialNumber,
			ProductType:  productType,
			BatchCode:    batchCode,
			Source:       source,
		})
	}

	if len(records) == 0 {
		return nil, gateway.ErrSerialInventoryRecordsRequired
	}
	return records, nil
}

func isSerialInventoryCSVHeader(row []string) bool {
	if len(row) < 2 {
		return false
	}
	column0 := strings.ToLower(strings.TrimSpace(row[0]))
	column1 := strings.ToLower(strings.TrimSpace(row[1]))
	return column0 == "serial_number" && column1 == "product_type"
}
