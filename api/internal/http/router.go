package httpx

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
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
	"github.com/mistypass/cloud/api/internal/modules/camera"
	"github.com/mistypass/cloud/api/internal/modules/credential"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
	"github.com/mistypass/cloud/api/internal/modules/hikconnect"
	"github.com/mistypass/cloud/api/internal/modules/hris"
	"github.com/mistypass/cloud/api/internal/modules/hris/talenta"
	"github.com/mistypass/cloud/api/internal/modules/space"
	"github.com/mistypass/cloud/api/internal/modules/tenant"
	"github.com/mistypass/cloud/api/internal/modules/wallet"
	"github.com/mistypass/cloud/api/internal/pdfgen"
	"github.com/mistypass/cloud/api/internal/redistore"
	"github.com/mistypass/cloud/api/internal/state"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type credentialSighting struct {
	LockID string
	SeenAt time.Time
	Count  int
}

type server struct {
	cfg                           config.Config
	stateStore                    state.Store
	gatewayTokenStore             gatewayTokenStore
	gatewayDeviceCA               *gateway.DeviceCA // mTLS CA for signing gateway client certs
	instanceID                    string
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
	cameraSvc                     *camera.Service
	hikConnectSvc                 *hikconnect.Service
	credentialSvc                 *credential.Service
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
	trustedProxies                []*net.IPNet
	gwWSRegistry                  *gwWSRegistry
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
	gatewayNonceStore             gatewayNonceStore
	volatileStore                 *redistore.Store
	workerLeaseStore              workerLeaseStore
	workerQueueStore              workerQueueStore
	scheduledReportMu             sync.RWMutex
	scheduledReports              map[string]referenceScheduledReport
	scheduledReportSeq            int
	reportScheduleMu              sync.RWMutex
	reportSchedules               map[string]reportSchedule
	reportScheduleSeq             int
	emailInboundMu                sync.RWMutex
	emailInboundEvents            []emailInboundEvent
	emailInboundSeq               int
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
	memStoreMu                    sync.RWMutex
	memStore                      map[string]any
	credLastSeenMu                sync.RWMutex
	credLastSeen                  map[string]credentialSighting
	hrisHTTPClient                *http.Client
	gatewayNonceMu                sync.Mutex
	gatewayNonces                 map[string]time.Time // nonce → expiry (5-min dedup window)
	oauth2                        *oauth2Store
	pushDeviceMu                  sync.Mutex
	pushDevices                   map[string]pushDevice
	doorFavoriteMu                sync.RWMutex
	doorFavorites                 map[string]map[string]bool // userID → doorID → true
	orgStore                      orgMembershipStore
	magicLinkStore                magicLinkStore
	pdfRenderer                   *pdfgen.Renderer
	gotenbergClient               *pdfgen.GotenbergClient
	quit                          chan struct{}
}

type pushDevice struct {
	UserID       string
	TenantID     string
	FCMToken     string
	DeviceID     string
	Platform     string
	RegisteredAt time.Time
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

type gatewayNonceStore interface {
	MarkGatewayRequestNonce(gatewayID, nonce string, ttl time.Duration) (bool, error)
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

type orgMembershipStore interface {
	ListUserOrgMemberships(userID string) ([]state.OrgMembership, error)
	GetUserOrgMembership(userID, tenantID string) (state.OrgMembership, bool, error)
	UpdateOrgMembershipLastUsed(userID, tenantID string) error
}

type magicLinkStore interface {
	CreateMagicLinkToken(email, token string, expiresAt time.Time) error
	VerifyMagicLinkToken(token string) (string, error)
}

type loginRateLimitBucket struct {
	WindowStart time.Time
	Attempts    int
}

type authContextKey string

const authUserContextKey authContextKey = "auth_user"
const gatewayBootstrapStateKey = "http_gateway_bootstrap"

const (
	loginRateLimitMaxAttempts                  = 10
	loginRateLimitWindow                       = time.Minute
	loginRateLimitBucketTTL                    = 5 * time.Minute
	loginRateLimitBucketMaxKeys                = 10000
	apiRateLimitMaxRequests                    = 600
	apiRateLimitWindow                         = time.Minute
	apiRateLimitBucketTTL                      = 10 * time.Minute
	apiRateLimitBucketMaxKeys                  = 20000
	httpRequestTimeout                         = 60 * time.Second
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

func NewRouter(cfg config.Config, stateStore state.Store) (http.Handler, func(), error) {
	handler, srv, err := newRouterInternal(cfg, stateStore)
	if err != nil {
		return nil, nil, err
	}
	return handler, func() { close(srv.quit) }, nil
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
	credentialSvc := credential.NewService()
	cameraSvc := camera.NewService(nil)
	if cfg.CameraSnapshotRetentionDays > 0 {
		cameraSvc.SetSnapshotRetentionDays(cfg.CameraSnapshotRetentionDays)
	}
	if cfg.CameraMaxSnapshotsPerEvent > 0 {
		cameraSvc.SetMaxSnapshotsPerEvent(cfg.CameraMaxSnapshotsPerEvent)
	}
	if cfg.CameraSnapshotTimeoutSeconds > 0 {
		cameraSvc.SetSnapshotTimeoutSeconds(cfg.CameraSnapshotTimeoutSeconds)
	}
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
		credentialSvc, err = credential.NewServiceWithStateStore(stateStore)
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
		CloudflareEndpoint:    cfg.CloudflareEmailEndpoint,
		CloudflareAccountID:   cfg.CloudflareEmailAccountID,
		CloudflareAPIToken:    cfg.CloudflareEmailAPIToken,
		CloudflareTimeout:     cfg.CloudflareEmailTimeout,
		WhatsAppProvider:      cfg.WalletAlertWhatsAppProvider,
		WhatsAppReceiverMap:   cfg.WalletAlertWhatsAppReceiverMap,
		WhatsAppEndpoint:      cfg.WalletAlertWhatsAppEndpoint,
		WhatsAppAPIKey:        cfg.WalletAlertWhatsAppAPIKey,
		WhatsAppPhoneNumberID: cfg.WalletAlertWhatsAppPhoneNumberID,
		WhatsAppTimeout:       cfg.WalletAlertWhatsAppTimeout,
		WhatsAppTemplateName:  cfg.WalletAlertWhatsAppTemplateName,
		WhatsAppTemplateLang:  cfg.WalletAlertWhatsAppTemplateLang,
		LarkAlertWebhookURL:   cfg.LarkAlertWebhookURL,
	}); err != nil {
		return nil, nil, err
	}
	if jwtSecret := strings.TrimSpace(cfg.JWTSecret); jwtSecret != "" && len(auditSvc.HMACKey()) == 0 {
		auditHMACKey := sha256.Sum256([]byte("mistypass-audit-hmac:" + jwtSecret))
		auditSvc.SetHMACKey(auditHMACKey[:])
	}
	scheduledReports, scheduledReportSeq := defaultReferenceScheduledReports(time.Now().UTC())

	pdfRenderer, err := pdfgen.NewRenderer()
	if err != nil {
		return nil, nil, fmt.Errorf("init pdf renderer: %w", err)
	}
	gotenbergClient := pdfgen.NewGotenbergClient(cfg.GotenbergURL, nil)

	s := &server{
		cfg:                           cfg,
		stateStore:                    stateStore,
		instanceID:                    newAPIInstanceID(),
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
		credentialSvc:                 credentialSvc,
		cameraSvc:                     cameraSvc,
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
		trustedProxies:                parseTrustedProxyCIDRs(cfg.TrustedProxyCIDRs),
		gwWSRegistry:                  newGWWSRegistry(),
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
		reportSchedules:               map[string]reportSchedule{},
		emailInboundEvents:            []emailInboundEvent{},
		customAlertPolicies:           map[string]referenceAlertPolicy{},
		alertCooldowns:                map[string]time.Time{},
		hrisWebhookReceiptWorkerWake:  make(chan struct{}, 1),
		hrisWebhookDLQWorkerWake:      make(chan struct{}, 1),
		hrisWebhookReceiptWorkerQueue: make(chan enterpriseHRISWebhookReceiptQueuedTask, enterpriseHRISWebhookQueuedTaskBufferSize),
		hrisWebhookDLQWorkerQueue:     make(chan enterpriseHRISWebhookDLQQueuedTask, enterpriseHRISWebhookQueuedTaskBufferSize),
		externalAuthHTTPClient:        &http.Client{Timeout: cfg.ExternalAuthTimeout},
		hrisHTTPClient:                &http.Client{Timeout: firstNonZeroDuration(cfg.ExternalAuthTimeout, 15*time.Second)},
		oauth2:                        newOAuth2Store(),
		memStore:                      map[string]any{},
		credLastSeen:                  map[string]credentialSighting{},
		pushDevices:                   map[string]pushDevice{},
		pdfRenderer:                   pdfRenderer,
		gotenbergClient:               gotenbergClient,
		quit:                          make(chan struct{}),
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
		if err := s.startGatewayWebSocketPushSubscriber(cfg.NATSServerURL, cfg.NATSSubjectPrefix); err != nil {
			s.loggerOrDefault().Warn("gateway websocket push subscriber failed to start", "error", err)
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
		s.gatewayNonceStore = redisStore
		s.volatileStore = redisStore
		s.workerLeaseStore = redisStore
		s.workerQueueStore = redisStore
		s.logger.Info("redis volatile store enabled", "addr", cfg.RedisAddr, "db", cfg.RedisDB)
	}
	if cfg.HikISCEnabled && cfg.HikISCAppKey != "" && cfg.HikISCAppSecret != "" {
		if s.volatileStore != nil {
			iscClient := hikconnect.NewClient(cfg.HikISCHost, cfg.HikISCAppKey, cfg.HikISCAppSecret)
			s.hikConnectSvc = hikconnect.NewService(iscClient, s.volatileStore, cfg.HikISCTokenCacheTTL)
			s.logger.Info("hik-connect ISC service enabled", "host", cfg.HikISCHost)
		} else {
			s.logger.Warn("hik-connect ISC enabled but redis is not configured; cloud video unavailable")
		}
	}
	if tokenStore, ok := stateStore.(gatewayTokenStore); ok {
		s.gatewayTokenStore = tokenStore
	}
	if orgStore, ok := stateStore.(orgMembershipStore); ok {
		s.orgStore = orgStore
	}
	if mlStore, ok := stateStore.(magicLinkStore); ok {
		s.magicLinkStore = mlStore
	}
	// Initialize gateway mTLS CA.
	// In production, load from GATEWAY_CA_CERT_PEM + GATEWAY_CA_KEY_PEM env vars.
	// For dev, auto-generate a new CA (certs won't survive server restarts).
	if cfg.GatewayCACertPEM != "" && cfg.GatewayCAKeyPEM != "" {
		deviceCA, err := gateway.LoadDeviceCA([]byte(cfg.GatewayCACertPEM), []byte(cfg.GatewayCAKeyPEM))
		if err != nil {
			s.logger.Warn("failed to load gateway device CA, mTLS disabled", "error", err)
		} else {
			if err := deviceCA.SetCertificateLifetime(effectiveGatewayMTLSCertLifetime(cfg)); err != nil {
				return nil, nil, fmt.Errorf("gateway mTLS cert lifetime: %w", err)
			}
			s.gatewayDeviceCA = deviceCA
			s.logger.Info("gateway mTLS CA loaded from config", "cert_lifetime", deviceCA.CertificateLifetime())
		}
	} else {
		deviceCA, err := gateway.NewDeviceCA()
		if err != nil {
			s.logger.Warn("failed to create gateway device CA, mTLS disabled", "error", err)
		} else {
			if err := deviceCA.SetCertificateLifetime(effectiveGatewayMTLSCertLifetime(cfg)); err != nil {
				return nil, nil, fmt.Errorf("gateway mTLS cert lifetime: %w", err)
			}
			s.gatewayDeviceCA = deviceCA
			s.logger.Info("gateway mTLS CA auto-generated (dev mode — certs won't survive restart)", "cert_lifetime", deviceCA.CertificateLifetime())
		}
	}

	if err := s.restoreGatewayBootstrapState(); err != nil {
		return nil, nil, err
	}
	s.restoreAlertPoliciesFromState()
	s.restoreReportSchedulesFromState()
	s.restoreEmailInboundEventsFromState()

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(httpRequestTimeout))
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
		r.With(s.withLoginRateLimit).Post("/auth/refresh", s.refresh)
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
		r.With(s.withBearerToken).Delete("/user", s.deleteCurrentUser)
		r.With(s.withEnterprisePublicRateLimit).Get("/organizations/{domain}/public", s.getPublicOrganization)
		r.With(s.withEnterprisePublicRateLimit).Post("/organizations/find", s.findOrganizations)
		r.With(s.withEnterprisePublicRateLimit).Post("/enterprise/tenant/resolve", s.resolveEnterpriseTenantByEmail)
		r.With(s.withEnterprisePublicRateLimit).Post("/enterprise/auth/start", s.enterpriseAuthStart)
		r.With(s.withEnterprisePublicRateLimit).Post("/enterprise/auth/exchange", s.enterpriseAuthExchange)
		r.With(s.withEnterprisePublicRateLimit).Post("/enterprise/auth/logout", s.enterpriseAuthLogout)
		r.With(s.withEnterpriseWebhookRateLimit).Post("/enterprise/hris-webhook/{connectorID}", s.receiveEnterpriseHRISWebhook)
		r.With(s.withEnterpriseWebhookRateLimit).Post("/webhooks/email/inbound", s.receiveEmailInboundWebhook)
		r.With(s.withEnterprisePublicRateLimit).Get("/enterprise/auth/oidc/callback", s.enterpriseOIDCCallback)
		r.With(s.withEnterprisePublicRateLimit).Post("/enterprise/auth/saml/callback", s.enterpriseSAMLCallback)
		r.With(s.withEnterprisePublicRateLimit).Post("/enterprise/jit-provision-approvals/external-sync/callback", s.enterpriseJITApprovalExternalSyncCallback)
		r.With(s.withEnterprisePublicRateLimit).Get("/group_links/verify", s.verifyReferenceGroupLinkToken)
		r.With(s.withEnterprisePublicRateLimit).Post("/group_links/verify", s.verifyReferenceGroupLinkToken)
		r.With(s.withEnterpriseWebhookRateLimit).Post("/users/invitations/provider-receipts", s.receiveUserInvitationProviderReceipt)

		// Kisi-compatible routes (no /app prefix)
		r.Get("/organizations/{domain}/public", s.kisiOrgPublic)
		r.With(s.withLoginRateLimit).Post("/logins", s.kisiLogin)
		r.With(s.withLoginRateLimit).Post("/logins/resolve", s.kisiLogin)
		r.Group(func(kisiAuth chi.Router) {
			kisiAuth.Use(s.withBearerToken)
			kisiAuth.Get("/login", s.kisiGetLogin)
			kisiAuth.Get("/organization", s.kisiGetOrganization)
			kisiAuth.Get("/places", s.kisiListPlaces)
			kisiAuth.Get("/places/{id}", s.kisiGetPlace)
		})

		r.Route("/app", func(app chi.Router) {
			app.With(s.withLoginRateLimit).Post("/auth/login", s.appLogin)
			app.Post("/auth/refresh", s.appRefresh)

			// Enhanced auth endpoints (unauthenticated)
			app.With(s.withLoginRateLimit).Post("/auth/magic-link", s.appRequestMagicLink)
			app.With(s.withLoginRateLimit).Post("/auth/magic-link/verify", s.appVerifyMagicLink)
			app.Get("/auth/org-lookup", s.appOrgLookup)
			app.Get("/auth/org/{orgId}/methods", s.appOrgMethods)
			app.With(s.withLoginRateLimit).Post("/auth/sso/{orgId}", s.appInitiateSSO)
			app.With(s.withLoginRateLimit).Post("/auth/2fa/verify", s.appVerify2FA)
			app.With(s.withLoginRateLimit).Post("/auth/2fa/backup", s.appVerifyBackupCode)
			app.With(s.withLoginRateLimit).Post("/auth/register", s.appRegister)
			app.With(s.withLoginRateLimit).Post("/auth/restore-password", s.appRestorePassword)

			app.Group(func(protected chi.Router) {
				protected.Use(s.withBearerToken)
				protected.Use(s.requireRoles("resident", "tenant_admin", "building_admin", "building_manager", "super_admin", "security"))

				protected.Get("/me", s.appMeEnhanced)
				protected.Patch("/me", s.appUpdateMe)
				protected.Post("/me/avatar", s.appUploadAvatar)
				protected.Post("/me/change-password", s.appChangePassword)
				protected.Get("/me/logins", s.appListMyLogins)
				protected.Delete("/me/logins/{loginId}", s.appRevokeMyLogin)
				protected.Post("/me/primary-device", s.appSetPrimaryDevice)
				protected.Get("/credentials", s.appCredentialsEnhanced)
				protected.Post("/credentials/apple-pass", s.appEnrollApplePass)
				protected.Get("/credentials/nfc", s.appListNFCCards)
				protected.Post("/credentials/nfc", s.appBindNFCCard)
				protected.Delete("/credentials/nfc/{credentialId}", s.appUnbindNFCCard)
				protected.Post("/qr-token", s.appGenerateQRToken)
				protected.Get("/access/doors", s.appAccessDoors)
				protected.Get("/access/my-doors", s.appAccessMyDoorsEnhanced)
				protected.Post("/access/unlock", s.appUnlockDoor)
				protected.Post("/access/qr-unlock", s.appQRUnlock)
				protected.Post("/access/pin-unlock", s.appPINUnlock)
				protected.Get("/access/pin-code", s.appGetPINCode)
				protected.Put("/access/doors/{doorId}/favorite", s.appToggleDoorFavorite)
				protected.Get("/access/ble-token", s.appAccessBLEToken)
				protected.Get("/access/logs", s.appAccessLogsEnhanced)
				protected.Get("/visitor-passes", s.appListVisitorPassesEnhanced)
				protected.Post("/visitor-passes", s.appCreateVisitorPassEnhanced)
				protected.Post("/devices/register", s.appRegisterDevice)
				protected.Post("/devices/apns", s.appRegisterAPNSDevice)

				// Mobile camera endpoints
				protected.Get("/cameras", s.appListCameras)
				protected.Get("/cameras/{cameraID}/video-link", s.appCameraVideoLink)
				protected.Post("/cameras/{cameraID}/snapshot", s.appCameraSnapshot)
				protected.Get("/cameras/{cameraID}/cloud-token", s.appCameraCloudToken)
				protected.Get("/cameras/{cameraID}/cloud-recordings", s.appCameraCloudRecordings)
				protected.Patch("/cameras/{cameraId}", s.appRenameCamera)

				// Hardware management
				protected.Patch("/gateways/{gatewayId}", s.appRenameGateway)

				// Bookings
				protected.Get("/bookable-spaces", s.appListBookableSpaces)
				protected.Get("/bookable-spaces/{spaceID}/status", s.appGetBookableSpaceStatus)
				protected.Get("/bookings", s.appListBookings)
				protected.Post("/bookings", s.appCreateBooking)
				protected.Post("/bookings/{bookingID}/cancel", s.appCancelBooking)
				protected.Post("/bookings/{bookingID}/check-in", s.appCheckInBooking)
				protected.Post("/bookings/{bookingID}/check-out", s.appCheckOutBooking)

				// Alarms
				protected.Get("/alarms", s.appListAlarms)
				protected.Get("/alarms/stream", s.appStreamAlarms)
				protected.Patch("/alarms/{alarmID}/status", s.appUpdateAlarmStatus)
				protected.Get("/alarm-schedules", s.appListAlarmSchedules)
				protected.Get("/alarm-schedules/calendar", s.appGetAlarmCalendar)

				// Multi-org endpoints
				protected.Get("/orgs", s.appListOrgs)
				protected.Post("/orgs/{orgId}/switch", s.appSwitchOrg)
				protected.Get("/orgs/{orgId}/settings", s.appGetOrgSettings)
				protected.Put("/orgs/{orgId}/settings", s.appUpdateOrgSettings)

				// Org-scoped place endpoints
				protected.Get("/orgs/{orgId}/places", s.appListPlaces)
				protected.Get("/orgs/{orgId}/places/search", s.appSearchPlaces)

				// Place-scoped door endpoints
				protected.Get("/places/{placeId}/doors", s.appPlaceListDoors)
				protected.Get("/places/{placeId}/doors/search", s.appPlaceSearchDoors)
				protected.Post("/places/{placeId}/doors/{doorId}/unlock", s.appPlaceUnlockDoor)
				protected.Post("/places/{placeId}/doors/{doorId}/qr-unlock", s.appPlaceQRUnlock)
				protected.Post("/places/{placeId}/lockdown", s.appPlaceEnableLockdown)
				protected.Delete("/places/{placeId}/lockdown", s.appPlaceDisableLockdown)
				protected.Put("/places/{placeId}/doors/{doorId}/favorite", s.appPlaceFavoriteDoor)
				protected.Delete("/places/{placeId}/doors/{doorId}/favorite", s.appPlaceUnfavoriteDoor)
				protected.Post("/places/{placeId}/doors/{doorId}/lockdown", s.appPlaceEnableDoorLockdown)
				protected.Delete("/places/{placeId}/doors/{doorId}/lockdown", s.appPlaceDisableDoorLockdown)
				protected.Get("/places/{placeId}/doors/{doorId}/restrictions", s.appPlaceDoorRestrictions)
				protected.Get("/places/{placeId}/doors/{doorId}/schedules", s.appPlaceDoorSchedules)
				protected.Patch("/places/{placeId}/doors/{doorId}", s.appPlaceRenameDoor)
				protected.Put("/places/{placeId}/settings", s.appUpdatePlaceSettings)

				// Admin endpoints (place-scoped)
				protected.Route("/places/{placeId}", func(placeRouter chi.Router) {
					placeRouter.Use(s.requireRoles("super_admin", "tenant_admin", "building_admin"))
					// Users
					placeRouter.Get("/users", s.appAdminListUsers)
					placeRouter.Get("/users/", s.appAdminListUsers)
					placeRouter.Get("/users/search", s.appAdminSearchUsers)
					placeRouter.Post("/users", s.appAdminAddUser)
					placeRouter.Post("/users/", s.appAdminAddUser)
					placeRouter.Get("/users/{userId}", s.appAdminGetUser)
					placeRouter.Put("/users/{userId}/role", s.appAdminUpdateUserRole)
					placeRouter.Patch("/users/{userId}/role", s.appAdminUpdateUserRole)
					placeRouter.Delete("/users/{userId}", s.appAdminRemoveUser)
					placeRouter.Post("/users/{userId}/sign-out", s.appAdminSignOutUser)
					placeRouter.Get("/users/{userId}/logins", s.appAdminGetUserLogins)
					placeRouter.Get("/users/{userId}/access-rights", s.appAdminGetAccessRights)
					placeRouter.Post("/users/{userId}/share-access", s.appAdminShareAccess)
					placeRouter.Post("/users/invite", s.appAdminInviteUser)
					// Events
					placeRouter.Get("/events", s.appAdminListEvents)
					placeRouter.Get("/events/{eventId}", s.appAdminGetEvent)
					placeRouter.Get("/events/{eventId}/related", s.appAdminGetRelatedEvents)
					placeRouter.Get("/events/{eventId}/media", s.appAdminGetEventMedia)
					// Incidents
					placeRouter.Get("/incidents", s.appAdminListIncidents)
					placeRouter.Get("/incidents/{incidentId}", s.appAdminGetIncident)
					placeRouter.Get("/incidents/{incidentId}/occurrences", s.appAdminGetOccurrences)
					// Activity
					placeRouter.Get("/activity", s.appAdminGetUserActivity)
					placeRouter.Get("/activity/{eventId}", s.appAdminGetPresenceEvent)
					// Schedules
					placeRouter.Get("/schedules", s.appAdminListSchedules)
					placeRouter.Post("/schedules", s.appAdminCreateSchedule)
					placeRouter.Put("/schedules/{scheduleId}", s.appAdminUpdateSchedule)
					placeRouter.Delete("/schedules/{scheduleId}", s.appAdminDeleteSchedule)
					// Holiday regions
					placeRouter.Get("/holiday-regions", s.appAdminListHolidayRegions)
					placeRouter.Get("/holiday-regions/{regionId}/holidays", s.appAdminListHolidays)
					// Zones
					placeRouter.Get("/zones", s.appAdminListZones)
					placeRouter.Get("/zones/{zoneId}", s.appAdminGetZone)
					// Cards
					placeRouter.Get("/cards", s.appAdminListCards)
					placeRouter.Post("/cards/assign", s.appAdminAssignCard)
					placeRouter.Delete("/cards/{cardUid}", s.appAdminUnassignCard)
					placeRouter.Get("/cards/{cardUid}/status", s.appAdminGetCardStatus)
					placeRouter.Post("/cards/manual-token", s.appAdminManualCardToken)
					// Digital credentials
					placeRouter.Get("/credentials", s.appAdminListCredentials)
					placeRouter.Post("/credentials", s.appAdminCreateCredential)
					placeRouter.Get("/credentials/search", s.appAdminSearchCredentials)
					placeRouter.Get("/credentials/{credentialId}", s.appAdminGetCredential)
					// Teams
					placeRouter.Get("/teams", s.appAdminListTeams)
					placeRouter.Post("/teams", s.appAdminCreateTeam)
					placeRouter.Get("/teams/{teamId}", s.appAdminGetTeam)
					placeRouter.Put("/teams/{teamId}", s.appAdminUpdateTeam)
					placeRouter.Delete("/teams/{teamId}", s.appAdminDeleteTeam)
					placeRouter.Get("/teams/{teamId}/members", s.appAdminListTeamMembers)
					placeRouter.Post("/teams/{teamId}/members", s.appAdminAddTeamMember)
					placeRouter.Delete("/teams/{teamId}/members/{memberId}", s.appAdminRemoveTeamMember)
					placeRouter.Get("/teams/{teamId}/access-rights", s.appAdminListTeamAccessRights)
					placeRouter.Post("/teams/{teamId}/access-rights", s.appAdminAssignTeamAccessRight)
					placeRouter.Delete("/teams/{teamId}/access-rights/{accessRightId}", s.appAdminRemoveTeamAccessRight)
					// Access groups
					placeRouter.Get("/groups", s.appAdminListGroups)
					placeRouter.Post("/groups", s.appAdminCreateGroup)
					placeRouter.Patch("/groups/{groupId}", s.appAdminUpdateGroup)
					placeRouter.Delete("/groups/{groupId}", s.appAdminDeleteGroup)
					placeRouter.Get("/groups/{groupId}/members", s.appAdminListGroupMembers)
					placeRouter.Post("/groups/{groupId}/members", s.appAdminAddGroupMember)
					placeRouter.Delete("/groups/{groupId}/members/{memberId}", s.appAdminRemoveGroupMember)
					placeRouter.Get("/groups/{groupId}/doors", s.appAdminListGroupDoors)
					placeRouter.Post("/groups/{groupId}/doors", s.appAdminAddGroupDoor)
					placeRouter.Delete("/groups/{groupId}/doors/{doorId}", s.appAdminRemoveGroupDoor)
					// Visitor groups
					placeRouter.Get("/visitor-groups", s.appListVisitorGroups)
					placeRouter.Post("/visitor-groups", s.appCreateVisitorGroup)
					placeRouter.Get("/visitor-groups/{groupId}/members", s.appListVisitorGroupMembers)
					placeRouter.Post("/visitor-groups/{groupId}/cleanup-expired", s.appCleanupExpiredVisitors)
					// Analytics & Reports
					placeRouter.Get("/analytics/summary", s.appAnalyticsSummary)
					placeRouter.Get("/analytics/presence", s.appUserPresence)
					placeRouter.Get("/analytics/failed-attempts", s.appAnalyticsFailedAttempts)
					placeRouter.Post("/reports/export", s.appExportReport)
					// Guest management
					placeRouter.Get("/guests", s.appAdminListGuests)
					placeRouter.Post("/guests", s.appAdminCreateGuest)
					placeRouter.Patch("/guests/{guestId}", s.appAdminUpdateGuestStatus)
					placeRouter.Delete("/guests/{guestId}", s.appAdminDeleteGuest)
					// Credential lifecycle (admin)
					placeRouter.Post("/credentials/{credentialId}/revoke", s.appAdminRevokeCredential)
					placeRouter.Post("/credentials/{credentialId}/suspend", s.appAdminSuspendCredential)
					placeRouter.Post("/credentials/{credentialId}/activate", s.appAdminActivateCredential)
				})

				// Mobile BLE credential management
				protected.Post("/credentials/register", s.appRegisterMobileCredential)
				protected.Get("/credentials/mobile", s.appListMobileCredentials)
				protected.Delete("/credentials/mobile/{credentialID}", s.appRevokeMobileCredential)
				protected.Post("/credentials/mobile/{credentialID}/refresh", s.appRefreshMobileCredential)
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
			gatewayRouter.Get("/credentials/sync", s.gatewayCredentialSync)
			gatewayRouter.Post("/audit/batch", s.gatewayAuditBatch)
			gatewayRouter.Post("/cert/renew", s.gatewayBootstrapCertRenew) // mTLS cert renewal
			gatewayRouter.Get("/ws", s.gatewayWebSocket)                   // persistent TLS WebSocket
		})

		// Lark integration endpoints
		r.Route("/integrations/lark", func(larkRouter chi.Router) {
			// Event callback (no auth — Lark pushes events here, verified by token)
			larkRouter.Post("/events", s.larkEventCallback)
			// Authenticated endpoints
			larkRouter.Group(func(authed chi.Router) {
				authed.Use(s.withBearerToken)
				authed.Use(s.requireRoles("super_admin", "tenant_admin"))
				authed.Post("/bot/test", s.larkBotTest)
				authed.Post("/bot/alert", s.larkBotSendAlert)
				authed.Post("/sync", s.larkSyncUsers)
			})
		})

		// Google Workspace integration endpoints
		r.Route("/integrations/google-workspace", func(gwsRouter chi.Router) {
			gwsRouter.Use(s.withBearerToken)
			gwsRouter.Use(s.requireRoles("super_admin", "tenant_admin"))
			gwsRouter.Post("/sync", s.googleWorkspaceSyncUsers)
		})

		// Mobile credential admin endpoints
		r.Route("/credentials/mobile", func(credRouter chi.Router) {
			credRouter.Use(s.withBearerToken)
			credRouter.Use(s.requireRoles("super_admin", "tenant_admin"))
			credRouter.Get("/", s.adminListMobileCredentials)
			credRouter.Get("/{credentialID}", s.adminGetMobileCredential)
			credRouter.Post("/{credentialID}/revoke", s.adminRevokeMobileCredential)
			credRouter.Post("/revoke-user", s.adminRevokeAllUserCredentials)
		})

		// Southbound device gateway (Hikvision, ZKTeco direct control)
		r.Route("/gateway/southbound", func(sbRouter chi.Router) {
			sbRouter.Use(s.withBearerToken)
			sbRouter.Use(s.requireRoles("super_admin", "tenant_admin"))
			sbRouter.Post("/{provider}/{deviceID}/unlock", s.southboundUnlock)
			sbRouter.Post("/{provider}/{deviceID}/sync-users", s.southboundSyncUsers)
			sbRouter.Post("/{provider}/test", s.southboundTestConnection)
			// ZKTeco push receiver (no auth — device pushes events here)
			sbRouter.With(s.withGlobalAPIRateLimit).Post("/zkteco/push", s.southboundZKTecoPush)
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
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/gateways/{gatewayID}/status", s.updateGatewayStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/gateways/cert-revocations", s.listGatewayCertificateRevocations)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/gateways/cert-revocations", s.revokeGatewayCertificateSerial)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/gateways/cert-revocations/{serialNumber}", s.restoreGatewayCertificateSerial)
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

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/network/topology", s.getNetworkTopology)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin"), withDeprecatedEndpoint("/api/v1/role_assignments", "/api/v1/groups", "/api/v1/group_locks")).Get("/access-policies", s.listAccessPolicies)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin"), withDeprecatedEndpoint("/api/v1/role_assignments", "/api/v1/groups", "/api/v1/group_locks")).Post("/access-policies", s.createAccessPolicy)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin"), withDeprecatedEndpoint("/api/v1/role_assignments", "/api/v1/groups", "/api/v1/group_locks")).Patch("/access-policies/{policyID}", s.updateAccessPolicy)
			protected.Post("/verify-credential", s.verifyCredential)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/gateways/{gatewayID}/access-rules", s.previewGatewayAccessRules)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/organization/dashboard", s.getOrganizationDashboard)
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

			// Bookable Spaces
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin", "resident")).Get("/bookable-spaces", s.listBookableSpaces)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/bookable-spaces", s.createBookableSpace)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/bookable-spaces/{spaceID}", s.updateBookableSpace)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/bookable-spaces/{spaceID}", s.deleteBookableSpace)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin", "resident")).Get("/bookable-spaces/{spaceID}/status", s.getBookableSpaceStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin", "resident")).Get("/bookable-spaces/{spaceID}/usage", s.getBookableSpaceUsage)

			// Bookings
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin", "resident")).Get("/bookings", s.listBookings)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin", "resident")).Get("/bookings/{bookingID}", s.getBooking)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin", "resident")).Post("/bookings", s.createBooking)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin", "resident")).Patch("/bookings/{bookingID}", s.updateBooking)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin", "resident")).Delete("/bookings/{bookingID}", s.deleteBooking)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin", "resident")).Post("/bookings/{bookingID}/check-in", s.checkInBooking)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin", "resident")).Post("/bookings/{bookingID}/check-out", s.checkOutBooking)

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
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/group_elevator_stops/{groupElevatorStopID}", s.getGroupElevatorStop)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/group_elevator_stops", s.createGroupElevatorStop)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Delete("/group_elevator_stops/{groupElevatorStopID}", s.deleteGroupElevatorStop)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/group_terminals", s.listGroupTerminals)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/group_terminals/{groupTerminalID}", s.getGroupTerminal)
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
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/group_locks/{groupLockID}", s.getReferenceGroupLock)
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
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/team_memberships/{membershipID}", s.getReferenceTeamMembership)
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
			protected.Post("/card_assignments/{activationToken}/activate_with_token", s.activateReferenceCardAssignmentWithToken)
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
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/events/{eventID}/snapshots", s.listEventSnapshots)

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
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Patch("/cameras/{cameraID}", s.updateCamera)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/cameras/{cameraID}/video_link", s.fetchVideoLink)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/cameras/{cameraID}/test", s.testCameraConnection)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Post("/cameras/{cameraID}/snapshot", s.captureSnapshot)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/cameras/{cameraID}/snapshots", s.listCameraSnapshots)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/cameras/discover", s.discoverCameras)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Delete("/cameras/{cameraID}", s.deleteCamera)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/cameras/{cameraID}/cloud-bind", s.adminCameraCloudBind)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/cameras/{cameraID}/cloud-bind", s.adminCameraCloudUnbind)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/analytics/access-summary", s.getAccessSummary)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/analytics/door-activity", s.getDoorActivity)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/analytics/alarm-metrics", s.getAlarmMetrics)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/analytics/export", s.exportAnalytics)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/reports/export", s.exportReport)

			protected.With(s.requireRoles("super_admin", "tenant_admin")).Get("/report-schedules", s.listReportSchedules)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/report-schedules", s.createReportSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Get("/report-schedules/provider-status", s.getReportScheduleProviderStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Get("/report-schedules/{scheduleID}", s.getReportSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/report-schedules/{scheduleID}", s.updateReportSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/report-schedules/{scheduleID}", s.deleteReportSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/report-schedules/{scheduleID}/send", s.sendReportSchedule)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/webhooks/email/inbound/events", s.listEmailInboundEvents)

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

			// SCIM provisioning management (admin UI)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/scim/config", s.scimAdminGetConfig)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/scim/token", s.scimAdminGenerateToken)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Delete("/enterprise/scim/token", s.scimAdminRevokeToken)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/scim/test", s.scimAdminTestEndpoint)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/scim/logs", s.scimAdminListProvisioningLogs)

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

			// OAuth2 client management (admin CRUD)
			protected.With(s.oauth2Enabled, s.requireRoles("super_admin", "tenant_admin")).Post("/oauth2/clients", s.registerOAuth2Client)
			protected.With(s.oauth2Enabled, s.requireRoles("super_admin", "tenant_admin")).Get("/oauth2/clients", s.listOAuth2Clients)
			protected.With(s.oauth2Enabled, s.requireRoles("super_admin", "tenant_admin")).Get("/oauth2/clients/{clientID}", s.getOAuth2Client)
			protected.With(s.oauth2Enabled, s.requireRoles("super_admin", "tenant_admin")).Patch("/oauth2/clients/{clientID}", s.updateOAuth2Client)
			protected.With(s.oauth2Enabled, s.requireRoles("super_admin", "tenant_admin")).Delete("/oauth2/clients/{clientID}", s.deleteOAuth2Client)
		})
	})

	// OAuth2 protocol endpoints (outside protected group — handle their own auth)
	router.Route("/oauth2", func(oauthRouter chi.Router) {
		oauthRouter.Use(s.oauth2Enabled)
		oauthRouter.Get("/authorize", s.oauth2Authorize)
		oauthRouter.Post("/token", s.oauth2Token)
		oauthRouter.Post("/revoke", s.oauth2Revoke)
	})

	// SCIM 2.0 Server endpoints (outside protected group — uses Bearer token auth)
	router.Route("/scim/v2", func(scimRouter chi.Router) {
		scimRouter.Get("/ServiceProviderConfig", s.scimServiceProviderConfig)
		scimRouter.Get("/Schemas", s.scimSchemas)
		scimRouter.Get("/Users", s.scimListUsers)
		scimRouter.Get("/Users/{id}", s.scimGetUser)
		scimRouter.Post("/Users", s.scimCreateUser)
		scimRouter.Put("/Users/{id}", s.scimReplaceUser)
		scimRouter.Patch("/Users/{id}", s.scimPatchUser)
		scimRouter.Delete("/Users/{id}", s.scimDeleteUser)
	})

	s.startEnterpriseSyncReconcileWorker()
	s.startEnterpriseSyncWorkerAlertAutoRetryWorker()
	s.startEnterpriseJITApprovalExternalSyncWorker()
	s.startEnterpriseHRISWebhookReceiptWorker()
	s.startEnterpriseHRISWebhookDLQWorker()
	s.startEnterpriseHRISPullWorker()
	s.startAlertPolicyEventScheduler()
	go s.startReportScheduler(s.quit)

	return router, s, nil
}

func effectiveGatewayMTLSCertLifetime(cfg config.Config) time.Duration {
	if cfg.GatewayMTLSCertLifetime <= 0 {
		return gateway.DefaultDeviceCACertLifetime
	}
	return cfg.GatewayMTLSCertLifetime
}

func newAPIInstanceID() string {
	id, err := randomHexID(8)
	if err != nil {
		return fmt.Sprintf("api-%d", time.Now().UnixNano())
	}
	return "api-" + id
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
			"card_number":     passes[i].CardNumber,
			"user_id":         user.ID,
			"issued_at":       passes[i].IssuedAt,
			"created_at":      passes[i].IssuedAt,
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

	// Build door name lookup for enrichment
	doors := s.spaceSvc.ListDoors(user.TenantID)
	doorNames := make(map[string]string, len(doors))
	for _, d := range doors {
		doorNames[d.ID] = d.Name
	}

	// Parse pagination params
	offset, limit := parsePagination(r, len(events))
	total := len(events)

	// Apply pagination
	if offset > len(events) {
		offset = len(events)
	}
	end := offset + limit
	if end > len(events) {
		end = len(events)
	}
	page := events[offset:end]

	// Enrich with door_name
	items := make([]map[string]any, 0, len(page))
	for _, ev := range page {
		items = append(items, map[string]any{
			"id":        ev.ID,
			"door_id":   ev.DoorID,
			"door_name": doorNames[ev.DoorID],
			"type":      ev.Type,
			"result":    ev.Result,
			"actor":     ev.Actor,
			"at":        ev.At,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"pagination": map[string]any{
			"offset":   offset,
			"limit":    limit,
			"total":    total,
			"has_more": end < total,
		},
	})
}

func (s *server) appListVisitorPasses(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	all := s.accessSvc.ListVisitorPasses(user.TenantID)
	total := len(all)
	offset, limit := parsePagination(r, total)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": all[offset:end],
		"pagination": map[string]any{
			"offset":   offset,
			"limit":    limit,
			"total":    total,
			"has_more": end < total,
		},
	})
}

func (s *server) appRegisterDevice(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}
	var request struct {
		FCMToken    string `json:"fcm_token"`
		DeviceID    string `json:"device_id"`
		DeviceModel string `json:"device_model"`
		Platform    string `json:"platform"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.FCMToken == "" {
		writeError(w, http.StatusBadRequest, "fcm_token is required")
		return
	}
	s.pushDeviceMu.Lock()
	s.pushDevices[request.FCMToken] = pushDevice{
		UserID:       user.ID,
		TenantID:     user.TenantID,
		FCMToken:     request.FCMToken,
		DeviceID:     request.DeviceID,
		Platform:     request.Platform,
		RegisteredAt: time.Now(),
	}
	s.pushDeviceMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"status": "registered"})
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
