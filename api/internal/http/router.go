package httpx

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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
	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/alarm"
	"github.com/mistypass/cloud/api/internal/modules/audit"
	"github.com/mistypass/cloud/api/internal/modules/auth"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
	"github.com/mistypass/cloud/api/internal/modules/space"
	"github.com/mistypass/cloud/api/internal/modules/tenant"
	"github.com/mistypass/cloud/api/internal/modules/wallet"
	"github.com/mistypass/cloud/api/internal/state"
)

type server struct {
	cfg                           config.Config
	stateStore                    state.Store
	authService                   *auth.Service
	tenantSvc                     *tenant.Service
	spaceSvc                      *space.Service
	gatewaySvc                    *gateway.Service
	accessSvc                     *access.Service
	eventSvc                      *event.Service
	alarmSvc                      *alarm.Service
	auditSvc                      *audit.Service
	walletSvc                     *wallet.Service
	enterpriseSvc                 *enterprise.Service
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
}

type stateChangeReader interface {
	ListStateChanges(stateKey string, limit int) ([]state.StateChangeRecord, error)
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
)

func NewRouter(cfg config.Config, stateStore state.Store) (http.Handler, error) {
	tenantSvc := tenant.NewService()
	spaceSvc := space.NewService()
	accessSvc := access.NewService()
	gatewaySvc := gateway.NewService()
	eventSvc := event.NewService()
	alarmSvc := alarm.NewService()
	auditSvc := audit.NewService()
	walletSvc := wallet.NewService()
	enterpriseSvc := enterprise.NewService()
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
			return nil, err
		}
		spaceSvc, err = space.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, err
		}
		accessSvc, err = access.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, err
		}
		gatewaySvc, err = gateway.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, err
		}
		eventSvc, err = event.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, err
		}
		alarmSvc, err = alarm.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, err
		}
		auditSvc, err = audit.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, err
		}
		walletSvc, err = wallet.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, err
		}
		enterpriseSvc, err = enterprise.NewServiceWithStateStore(stateStore)
		if err != nil {
			return nil, err
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
		return nil, err
	}

	s := &server{
		cfg:                           cfg,
		stateStore:                    stateStore,
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
		stateChangeReader:             stateChangeReaderSvc,
		stateChangeReplayer:           stateChangeReplayerSvc,
		stateChangeCheckpointReader:   stateChangeCheckpointReaderSvc,
		stateChangeCheckpointReplayer: stateChangeCheckpointReplayerSvc,
		gatewayDeviceTokens: map[string]string{
			"gw_demo_001": "gw_demo_token_jkt",
			"gw_demo_002": "gw_demo_token_factory",
		},
		gatewayBatchFailureSeen: map[string]struct{}{},
		gatewayAuthzAckVersion:  map[string]string{},
		loginRateLimitBuckets:   map[string]loginRateLimitBucket{},
	}
	if err := s.restoreGatewayBootstrapState(); err != nil {
		return nil, err
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(15 * time.Second))
	router.Use(s.withCORS)

	router.Get("/healthz", s.healthz)
	router.Route("/api/v1", func(r chi.Router) {
		r.With(s.withLoginRateLimit).Post("/auth/login", s.login)
		r.Post("/auth/refresh", s.refresh)
		r.With(s.withBearerToken).Post("/auth/logout", s.logout)
		r.With(s.withBearerToken).Get("/me", s.me)
		r.Post("/enterprise/tenant/resolve", s.resolveEnterpriseTenantByEmail)
		r.Post("/enterprise/auth/start", s.enterpriseAuthStart)
		r.Post("/enterprise/auth/exchange", s.enterpriseAuthExchange)
		r.Post("/enterprise/auth/logout", s.enterpriseAuthLogout)
		r.Get("/enterprise/auth/oidc/callback", s.enterpriseOIDCCallback)
		r.Post("/enterprise/auth/saml/callback", s.enterpriseSAMLCallback)
		r.Post("/enterprise/jit-provision-approvals/external-sync/callback", s.enterpriseJITApprovalExternalSyncCallback)

		r.Route("/app", func(app chi.Router) {
			app.With(s.withLoginRateLimit).Post("/auth/login", s.appLogin)
			app.Post("/auth/refresh", s.appRefresh)

			app.Group(func(protected chi.Router) {
				protected.Use(s.withBearerToken)
				protected.Use(s.requireRoles("resident"))

				protected.Get("/me", s.appMe)
				protected.Get("/credentials", s.appCredentials)
				protected.Get("/access/doors", s.appAccessDoors)
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
		})

		r.Group(func(protected chi.Router) {
			protected.Use(s.withBearerToken)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Get("/auth/users/{userID}/building-scope", s.getAuthUserBuildingScope)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Put("/auth/users/{userID}/building-scope", s.updateAuthUserBuildingScope)

			protected.With(s.requireRoles("super_admin")).Get("/tenants", s.listTenants)
			protected.With(s.requireRoles("super_admin")).Post("/tenants", s.createTenant)
			protected.With(s.requireRoles("super_admin")).Patch("/tenants/{tenantID}/status", s.updateTenantStatus)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/tenants/{tenantID}/topology", s.getTenantTopology)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/buildings", s.listBuildings)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/buildings", s.createBuilding)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/floors", s.listFloors)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/floors", s.createFloor)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/areas", s.listAreas)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/areas", s.createArea)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/doors", s.listDoors)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/doors", s.createDoor)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/door-groups", s.listDoorGroups)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/door-groups", s.createDoorGroup)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/gateways", s.listGateways)
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
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/gateways/{gatewayID}/events/checkpoint", s.listGatewayEventCheckpoints)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/access-policies", s.listAccessPolicies)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/access-policies", s.createAccessPolicy)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/access-policies/{policyID}", s.updateAccessPolicy)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/users", s.listUsers)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/users", s.createUser)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/user-groups", s.listUserGroups)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/user-groups", s.createUserGroup)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Patch("/user-groups/{groupID}", s.updateUserGroup)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/temporary-access", s.listTemporaryAccess)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/temporary-access", s.createTemporaryAccess)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/visitor-passes", s.listVisitorPasses)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "building_admin")).Post("/visitor-passes", s.createVisitorPass)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/events/access", s.listAccessEvents)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/events/device", s.listDeviceEvents)

			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Get("/alarms", s.listAlarms)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator", "building_admin")).Patch("/alarms/{alarmID}/status", s.updateAlarmStatus)
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
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/passes", s.listWalletPasses)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/passes/{passID}", s.getWalletPass)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/passes/{passID}/save-link", s.getWalletPassSaveLink)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/wallet/passes/{passID}/suspend", s.suspendWalletPass)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/wallet/passes/{passID}/activate", s.activateWalletPass)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Patch("/wallet/passes/{passID}/revoke", s.revokeWalletPass)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/wallet/deliveries", s.listWalletPassDeliveries)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/deliveries/dispatch", s.dispatchWalletPassDelivery)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/wallet/deliveries/{notificationID}/retry", s.retryWalletPassDelivery)
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
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/sync-worker-alerts", s.listEnterpriseSyncWorkerAlerts)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/sync-worker-alerts/summary", s.listEnterpriseSyncWorkerAlertSummary)
			protected.With(s.requireRoles("super_admin", "tenant_admin")).Post("/enterprise/sync-requests/reconcile-pending", s.reconcilePendingEnterpriseSyncRequests)
			protected.With(s.requireRoles("super_admin", "tenant_admin", "operator")).Get("/enterprise/sync-jobs", s.listEnterpriseSyncJobs)
		})
	})

	s.startEnterpriseSyncReconcileWorker()
	s.startEnterpriseJITApprovalExternalSyncWorker()

	return router, nil
}

type gatewayBootstrapStateSnapshot struct {
	DeviceTokens map[string]string `json:"device_tokens,omitempty"`
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
		w.Header().Set("Access-Control-Allow-Origin", s.cfg.CORSOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
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
			"id":        passes[i].ID,
			"type":      "wallet",
			"status":    passes[i].Status,
			"save_link": passes[i].SaveLink,
			"issued_at": passes[i].IssuedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
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

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id":      snapshot.GatewayID,
		"tenant_id":       snapshot.TenantID,
		"current_version": currentVersion,
		"desired_version": desiredVersion,
		"should_apply":    desiredVersion != "" && desiredVersion != currentVersion,
		"desired_at":      snapshot.DesiredUpdatedAt,
		"applied_version": snapshot.AppliedVersion,
		"applied_at":      snapshot.AppliedAt,
		"bound_door_ids":  snapshot.BoundDoorIDs,
		"devices":         snapshot.Devices,
		"authz_cache":     authzCache,
		"fetched_at":      fetchedAt,
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
	cache.Counts = gatewayConfigAuthzCacheCounts{
		Doors:           len(cache.Doors),
		Policies:        len(cache.Policies),
		TemporaryAccess: len(cache.TemporaryAccess),
		VisitorPasses:   len(cache.VisitorPasses),
		Users:           len(cache.Users),
		UserGroups:      len(cache.UserGroups),
	}
	cache.Version = gatewayConfigAuthzCacheVersion(cache)
	return cache
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
	Source                string    `json:"source"`
	At                    time.Time `json:"at"`
	Failed                int       `json:"failed"`
	Threshold             int       `json:"threshold"`
	Processed             int       `json:"processed"`
	Applied               int       `json:"applied"`
	SkippedByAttemptLimit int       `json:"skipped_by_attempt_limit"`
	SkippedByCooldown     int       `json:"skipped_by_cooldown"`
	RawTarget             string    `json:"raw_target"`
}

type enterpriseSyncWorkerAlertSummaryItem struct {
	TenantID                  string    `json:"tenant_id"`
	Count                     int       `json:"count"`
	FirstSeenAt               time.Time `json:"first_seen_at"`
	LastSeenAt                time.Time `json:"last_seen_at"`
	LastFailed                int       `json:"last_failed"`
	LastThreshold             int       `json:"last_threshold"`
	LastProcessed             int       `json:"last_processed"`
	LastApplied               int       `json:"last_applied"`
	LastSkippedByAttemptLimit int       `json:"last_skipped_by_attempt_limit"`
	LastSkippedByCooldown     int       `json:"last_skipped_by_cooldown"`
}

func buildEnterpriseSyncWorkerAlerts(logs []audit.Log) []enterpriseSyncWorkerAlertItem {
	items := make([]enterpriseSyncWorkerAlertItem, 0, len(logs))
	for i := range logs {
		metrics := parseEnterpriseSyncWorkerAlertMetrics(logs[i].Target)
		items = append(items, enterpriseSyncWorkerAlertItem{
			ID:                    logs[i].ID,
			TenantID:              logs[i].TenantID,
			Actor:                 logs[i].Actor,
			Role:                  logs[i].Role,
			Action:                logs[i].Action,
			Source:                logs[i].Source,
			At:                    logs[i].At,
			Failed:                metrics.Failed,
			Threshold:             metrics.Threshold,
			Processed:             metrics.Processed,
			Applied:               metrics.Applied,
			SkippedByAttemptLimit: metrics.SkippedByAttemptLimit,
			SkippedByCooldown:     metrics.SkippedByCooldown,
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
		entry, exists := summaries[tenantID]
		if !exists {
			entry = enterpriseSyncWorkerAlertSummaryItem{
				TenantID:    tenantID,
				FirstSeenAt: logs[i].At,
				LastSeenAt:  logs[i].At,
			}
		}
		entry.Count++
		if logs[i].At.Before(entry.FirstSeenAt) {
			entry.FirstSeenAt = logs[i].At
		}
		if logs[i].At.After(entry.LastSeenAt) || logs[i].At.Equal(entry.LastSeenAt) {
			metrics := parseEnterpriseSyncWorkerAlertMetrics(logs[i].Target)
			entry.LastSeenAt = logs[i].At
			entry.LastFailed = metrics.Failed
			entry.LastThreshold = metrics.Threshold
			entry.LastProcessed = metrics.Processed
			entry.LastApplied = metrics.Applied
			entry.LastSkippedByAttemptLimit = metrics.SkippedByAttemptLimit
			entry.LastSkippedByCooldown = metrics.SkippedByCooldown
		}
		summaries[tenantID] = entry
	}

	items := make([]enterpriseSyncWorkerAlertSummaryItem, 0, len(summaries))
	for _, item := range summaries {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].LastSeenAt.Equal(items[j].LastSeenAt) {
			return items[i].TenantID < items[j].TenantID
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
	SkippedByAttemptLimit int
	SkippedByCooldown     int
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
		Applied:               parseIntOrZero(values["applied"]),
		SkippedByAttemptLimit: parseIntOrZero(values["skipped_attempt_limit"]),
		SkippedByCooldown:     parseIntOrZero(values["skipped_cooldown"]),
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

	log.Printf(
		"enterprise sync reconcile worker enabled interval=%s batch_size=%d max_attempts=%d retry_cooldown=%s alert_threshold=%d force_error=%t force_error_tenant_id=%q",
		interval,
		batchSize,
		maxAttempts,
		retryCooldown,
		alertFailureThreshold,
		forceError,
		forceErrorTenantID,
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

type enterpriseJITApprovalExternalSyncWorkerResult struct {
	Processed             int
	Synced                int
	Failed                int
	SkippedByAttemptLimit int
	SkippedByCooldown     int
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

	log.Printf(
		"enterprise jit approval external sync worker enabled interval=%s batch_size=%d max_attempts=%d retry_cooldown=%s alert_threshold=%d force_error=%t force_error_tenant_id=%q",
		interval,
		batchSize,
		maxAttempts,
		retryCooldown,
		alertFailureThreshold,
		forceError,
		forceErrorTenantID,
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
				log.Printf(
					"enterprise jit approval external sync worker tenant=%s approval_id=%s failed: %v",
					tenantID,
					item.ID,
					err,
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
				log.Printf(
					"enterprise jit approval external sync worker tenant=%s processed=0 skipped_attempt_limit=%d skipped_cooldown=%d",
					tenantID,
					result.SkippedByAttemptLimit,
					result.SkippedByCooldown,
				)
			}
			continue
		}
		if result.Failed >= alertFailureThreshold {
			log.Printf(
				"enterprise jit approval external sync worker alert tenant=%s failed=%d threshold=%d",
				tenantID,
				result.Failed,
				alertFailureThreshold,
			)
			s.appendEnterpriseJITApprovalExternalSyncWorkerAlertAudit(tenantID, result, alertFailureThreshold)
		}
		log.Printf(
			"enterprise jit approval external sync worker tenant=%s processed=%d synced=%d failed=%d skipped_attempt_limit=%d skipped_cooldown=%d",
			tenantID,
			result.Processed,
			result.Synced,
			result.Failed,
			result.SkippedByAttemptLimit,
			result.SkippedByCooldown,
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
			log.Printf(
				"enterprise sync reconcile worker tenant=%s failed: %v",
				tenantID,
				err,
			)
			continue
		}
		if result.Processed == 0 {
			if result.SkippedByAttemptLimit > 0 || result.SkippedByCooldown > 0 {
				log.Printf(
					"enterprise sync reconcile worker tenant=%s processed=0 skipped_attempt_limit=%d skipped_cooldown=%d",
					tenantID,
					result.SkippedByAttemptLimit,
					result.SkippedByCooldown,
				)
			}
			continue
		}
		if result.Failed >= alertFailureThreshold {
			log.Printf(
				"enterprise sync reconcile worker alert tenant=%s failed=%d threshold=%d",
				tenantID,
				result.Failed,
				alertFailureThreshold,
			)
			s.appendEnterpriseSyncWorkerAlertAudit(tenantID, result, alertFailureThreshold)
		}
		log.Printf(
			"enterprise sync reconcile worker tenant=%s processed=%d applied=%d failed=%d skipped_attempt_limit=%d skipped_cooldown=%d",
			tenantID,
			result.Processed,
			result.Applied,
			result.Failed,
			result.SkippedByAttemptLimit,
			result.SkippedByCooldown,
		)
	}
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
	if result.Failed < alertFailureThreshold {
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
	if len(scope) == 0 {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return nil, false
	}

	return scope, true
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
	if user, ok := authenticatedUser(r); ok {
		if strings.TrimSpace(user.Email) != "" {
			actor = strings.TrimSpace(user.Email)
		}
		if strings.TrimSpace(user.Role) != "" {
			role = strings.TrimSpace(user.Role)
		}
	}

	_, _ = s.auditSvc.Append(
		nextTenantID,
		actor,
		role,
		strings.TrimSpace(action),
		strings.TrimSpace(target),
		strings.TrimSpace(source),
	)
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
	s.gatewayTokenMu.Lock()
	defer s.gatewayTokenMu.Unlock()
	if s.gatewayDeviceTokens == nil {
		s.gatewayDeviceTokens = map[string]string{}
	}
	s.gatewayDeviceTokens[nextGatewayID] = nextDeviceToken
	if err := s.persistGatewayBootstrapStateLocked(); err != nil {
		log.Printf("gateway bootstrap token persist failed: gateway=%s err=%v", nextGatewayID, err)
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

	nextGatewayID := strings.TrimSpace(gatewayID)
	s.gatewayTokenMu.RLock()
	expected, exists := s.gatewayDeviceTokens[nextGatewayID]
	s.gatewayTokenMu.RUnlock()
	if !exists || strings.TrimSpace(expected) == "" {
		writeError(w, http.StatusUnauthorized, "device not registered")
		return false
	}
	if provided != expected {
		writeError(w, http.StatusUnauthorized, "invalid device token")
		return false
	}

	return true
}

func randomHexID(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
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
