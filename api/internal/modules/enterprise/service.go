package enterprise

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mistypass/cloud/api/internal/retrybackoff"
)

var ErrTenantIDRequired = errors.New("tenant_id is required")
var ErrDomainRequired = errors.New("domain is required")
var ErrInvalidDomain = errors.New("invalid domain")
var ErrDomainAlreadyMapped = errors.New("domain already mapped")
var ErrDomainMappingNotFound = errors.New("domain mapping not found")
var ErrInvalidDomainMappingStatus = errors.New("invalid domain mapping status")
var ErrInvalidHRISConnectorVendor = errors.New("invalid hris connector vendor")
var ErrInvalidHRISConnectorStatus = errors.New("invalid hris connector status")
var ErrInvalidHRISConnectorSyncStrategy = errors.New("invalid hris connector sync strategy")
var ErrHRISConnectorAlreadyExists = errors.New("hris connector already exists for vendor")
var ErrHRISConnectorNotFound = errors.New("hris connector not found")
var ErrHRISConnectorInactive = errors.New("hris connector is inactive")
var ErrHRISWebhookReceiptNotFound = errors.New("hris webhook receipt not found")
var ErrHRISWebhookExecutionNotFound = errors.New("hris webhook execution not found")
var ErrInvalidHRISWebhookExecutionKind = errors.New("invalid hris webhook execution kind")
var ErrInvalidHRISWebhookExecutionStatus = errors.New("invalid hris webhook execution status")
var ErrInvalidHRISWebhookExecutionDispatchMode = errors.New("invalid hris webhook execution dispatch mode")
var ErrEmailRequired = errors.New("email is required")
var ErrDomainNotMapped = errors.New("domain is not mapped")
var ErrIDPConfigNotFound = errors.New("enterprise idp config not found")
var ErrInvalidIDPProvider = errors.New("invalid idp provider")
var ErrIssuerURLRequired = errors.New("issuer_url is required")
var ErrClientIDRequired = errors.New("client_id is required")
var ErrInvalidIDPStatus = errors.New("invalid idp status")
var ErrSAMLACSURLRequired = errors.New("saml_acs_url is required for saml provider")
var ErrInvalidSAMLACSURL = errors.New("saml_acs_url must use https://")
var ErrSAMLX509CertRequired = errors.New("saml_x509_cert is required for saml provider")
var ErrEmployeesRequired = errors.New("employees is required")
var ErrInvalidReconcileLimit = errors.New("reconcile limit must be >= 1")
var ErrSyncRequestIDRequired = errors.New("request_id is required")
var ErrSyncRequestNotFound = errors.New("sync request not found")
var ErrInvalidSyncSource = errors.New("invalid sync source")
var ErrEmployeeEmailDomainMismatch = errors.New("employee email domain does not match tenant domain mapping")
var ErrEmployeeNotFound = errors.New("enterprise employee not found")
var ErrEmployeeInactive = errors.New("enterprise employee is inactive")
var ErrEmployeeExternalIDConflict = errors.New("enterprise employee external_id conflict")
var ErrJITProvisionApprovalRequired = errors.New("enterprise jit provisioning requires approval")
var ErrJITProvisionApprovalNotFound = errors.New("enterprise jit provision approval not found")
var ErrInvalidJITProvisionApprovalDecision = errors.New("invalid jit provision approval decision")
var ErrInvalidJITProvisionApprovalExternalSyncStatus = errors.New("invalid jit provision approval external sync status")
var ErrInvalidSyncWorkerAlertSubscriptionOptions = errors.New("invalid enterprise sync worker alert subscription options")
var ErrSyncWorkerAlertDispatcherRequired = errors.New("enterprise sync worker alert dispatcher is required")
var ErrSyncWorkerAlertConfirmationRequired = errors.New("enterprise sync worker alert confirmation callback is required")
var ErrSyncWorkerAlertNotificationNotFound = errors.New("enterprise sync worker alert notification not found")
var ErrSyncWorkerAlertNotificationIDsRequired = errors.New("notification_ids is required")
var ErrSyncWorkerAlertRetryNotAllowed = errors.New("enterprise sync worker alert retry not allowed")
var ErrSyncWorkerAlertDispatchInFlight = errors.New("enterprise sync worker alert dispatch is already in flight")
var ErrEnterpriseHRISWebhookStateConflict = errors.New("enterprise hris webhook state conflict")
var ErrSyncWorkerAlertStateConflict = errors.New("enterprise sync worker alert state conflict")
var ErrAuthStateTokenRequired = errors.New("state_token is required")
var ErrAuthStateTokenNotFound = errors.New("state_token is invalid or expired")
var ErrAuthStateProviderMismatch = errors.New("state_token provider mismatch")
var ErrRedirectURIRequired = errors.New("redirect_uri is required")
var ErrInvalidRedirectURI = errors.New("redirect_uri must use https:// or http://localhost")
var ErrAccessSyncApplierRequired = errors.New("access sync applier is required")

const (
	HRISWebhookReceiptClaimReasonAttemptLimit = "attempt_limit"
	HRISWebhookReceiptClaimReasonCooldown     = "cooldown"
	HRISWebhookReceiptClaimReasonInFlight     = "in_flight"
	HRISWebhookReceiptClaimReasonNotQueueable = "not_queueable"

	HRISWebhookExecutionClaimReasonCooldown     = "cooldown"
	HRISWebhookExecutionClaimReasonInFlight     = "in_flight"
	HRISWebhookExecutionClaimReasonNotQueueable = "not_queueable"
)

const (
	HRISWebhookExecutionKindReceiptProcess = "receipt_process"
	HRISWebhookExecutionKindDLQReplay      = "dlq_replay"

	HRISWebhookExecutionStatusQueued    = "queued"
	HRISWebhookExecutionStatusRunning   = "running"
	HRISWebhookExecutionStatusSucceeded = "succeeded"
	HRISWebhookExecutionStatusFailed    = "failed"

	HRISWebhookExecutionDispatchModeWorkerTick        = "worker_tick"
	HRISWebhookExecutionDispatchModeWorkerTaskChannel = "worker_task_channel"
	HRISWebhookExecutionDispatchModeGoroutineFallback = "goroutine_fallback"
)

type DomainMapping struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Domain    string    `json:"domain"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TenantResolution struct {
	Email    string `json:"email"`
	Domain   string `json:"domain"`
	TenantID string `json:"tenant_id"`
	Matched  bool   `json:"matched"`
}

type HRISConnector struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	Vendor           string     `json:"vendor"`
	Status           string     `json:"status"`
	SyncStrategy     string     `json:"sync_strategy"`
	CredentialRef    string     `json:"credential_ref,omitempty"`
	WebhookSecretRef string     `json:"webhook_secret_ref,omitempty"`
	LastSyncAt       *time.Time `json:"last_sync_at,omitempty"`
	UpdatedBy        string     `json:"updated_by,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type HRISWebhookReceipt struct {
	ID            string            `json:"id"`
	TenantID      string            `json:"tenant_id"`
	ConnectorID   string            `json:"connector_id"`
	Vendor        string            `json:"vendor"`
	EventType     string            `json:"event_type,omitempty"`
	RequestID     string            `json:"request_id,omitempty"`
	ContentType   string            `json:"content_type,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	RawPayload    string            `json:"raw_payload,omitempty"`
	SourceIP      string            `json:"source_ip,omitempty"`
	Status        string            `json:"status"`
	AttemptCount  int               `json:"attempt_count,omitempty"`
	LastError     string            `json:"last_error,omitempty"`
	ReceivedAt    time.Time         `json:"received_at"`
	LastAttemptAt *time.Time        `json:"last_attempt_at,omitempty"`
	ProcessedAt   *time.Time        `json:"processed_at,omitempty"`
}

type HRISWebhookReceiptInput struct {
	EventType   string
	RequestID   string
	ContentType string
	Headers     map[string]string
	RawPayload  string
	SourceIP    string
}

type HRISWebhookExecution struct {
	ID                      string     `json:"id"`
	TenantID                string     `json:"tenant_id"`
	Kind                    string     `json:"kind"`
	TargetID                string     `json:"target_id"`
	ReceiptID               string     `json:"receipt_id,omitempty"`
	ConnectorID             string     `json:"connector_id,omitempty"`
	Vendor                  string     `json:"vendor,omitempty"`
	RequestID               string     `json:"request_id,omitempty"`
	EventType               string     `json:"event_type,omitempty"`
	FailureStage            string     `json:"failure_stage,omitempty"`
	AuditSource             string     `json:"audit_source,omitempty"`
	ExecutionMode           string     `json:"execution_mode,omitempty"`
	DispatchMode            string     `json:"dispatch_mode,omitempty"`
	Status                  string     `json:"status"`
	TargetStatus            string     `json:"target_status,omitempty"`
	RequestedBy             string     `json:"requested_by,omitempty"`
	ReplaySourceExecutionID string     `json:"replay_source_execution_id,omitempty"`
	ReplayRequireWorker     *bool      `json:"replay_require_worker,omitempty"`
	AttemptCount            int        `json:"attempt_count,omitempty"`
	RequeueCount            int        `json:"requeue_count,omitempty"`
	LastError               string     `json:"last_error,omitempty"`
	QueuedAt                time.Time  `json:"queued_at"`
	StartedAt               *time.Time `json:"started_at,omitempty"`
	FinishedAt              *time.Time `json:"finished_at,omitempty"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type HRISWebhookExecutionReplayConflictError struct {
	ExistingExecution HRISWebhookExecution
}

func (e *HRISWebhookExecutionReplayConflictError) Error() string {
	if e == nil || strings.TrimSpace(e.ExistingExecution.ID) == "" {
		return "hris webhook execution replay already queued or running"
	}
	return fmt.Sprintf(
		"hris webhook execution replay already queued or running: execution_id=%s,status=%s",
		e.ExistingExecution.ID,
		e.ExistingExecution.Status,
	)
}

type HRISWebhookExecutionInput struct {
	TenantID                string
	Kind                    string
	TargetID                string
	ReceiptID               string
	ConnectorID             string
	Vendor                  string
	RequestID               string
	EventType               string
	FailureStage            string
	AuditSource             string
	ExecutionMode           string
	DispatchMode            string
	TargetStatus            string
	RequestedBy             string
	ReplaySourceExecutionID string
	ReplayRequireWorker     *bool
}

type hrisWebhookReceiptDueIndexEntry struct {
	ReceiptID string    `json:"receipt_id"`
	DueAt     time.Time `json:"due_at"`
}

type IDPConfig struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Provider     string    `json:"provider"`
	IssuerURL    string    `json:"issuer_url"`
	ClientID     string    `json:"client_id"`
	AuthURL      string    `json:"auth_url,omitempty"`
	TokenURL     string    `json:"token_url,omitempty"`
	JWKSURL      string    `json:"jwks_url,omitempty"`
	UserInfoURL  string    `json:"user_info_url,omitempty"`
	SAMLACSURL   string    `json:"saml_acs_url,omitempty"`
	SAMLX509Cert string    `json:"saml_x509_cert,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
	Status       string    `json:"status"`
	SyncMode     string    `json:"sync_mode"`
	UpdatedBy    string    `json:"updated_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type IDPConfigValidationItem struct {
	Field   string `json:"field"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type IDPConfigValidation struct {
	TenantID  string                    `json:"tenant_id"`
	Provider  string                    `json:"provider"`
	Valid     bool                      `json:"valid"`
	Items     []IDPConfigValidationItem `json:"items"`
	CheckedAt time.Time                 `json:"checked_at"`
}

type EmployeeSyncInput struct {
	ExternalID        string `json:"external_id"`
	EmployeeNumber    string `json:"employee_number,omitempty"`
	Email             string `json:"email"`
	FullName          string `json:"full_name"`
	Department        string `json:"department"`
	JobTitle          string `json:"job_title"`
	Location          string `json:"location"`
	Phone             string `json:"phone,omitempty"`
	ManagerExternalID string `json:"manager_external_id,omitempty"`
	EmploymentStatus  string `json:"employment_status,omitempty"`
	JoinDate          string `json:"join_date,omitempty"`
	ResignDate        string `json:"resign_date,omitempty"`
	ShiftCode         string `json:"shift_code,omitempty"`
	ScheduleWindow    string `json:"schedule_window,omitempty"`
	LeaveStatus       string `json:"leave_status,omitempty"`
	CostCenter        string `json:"cost_center,omitempty"`
	PhotoURL          string `json:"photo_url,omitempty"`
	Status            string `json:"status"`
}

type EnterpriseEmployee struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	ExternalID        string    `json:"external_id"`
	EmployeeNumber    string    `json:"employee_number,omitempty"`
	Email             string    `json:"email"`
	FullName          string    `json:"full_name"`
	Department        string    `json:"department"`
	JobTitle          string    `json:"job_title"`
	Location          string    `json:"location"`
	Phone             string    `json:"phone,omitempty"`
	ManagerExternalID string    `json:"manager_external_id,omitempty"`
	EmploymentStatus  string    `json:"employment_status,omitempty"`
	JoinDate          string    `json:"join_date,omitempty"`
	ResignDate        string    `json:"resign_date,omitempty"`
	ShiftCode         string    `json:"shift_code,omitempty"`
	ScheduleWindow    string    `json:"schedule_window,omitempty"`
	LeaveStatus       string    `json:"leave_status,omitempty"`
	CostCenter        string    `json:"cost_center,omitempty"`
	PhotoURL          string    `json:"photo_url,omitempty"`
	AccessRole        string    `json:"access_role"`
	BuildingID        string    `json:"building_id"`
	GroupIDs          []string  `json:"group_ids,omitempty"`
	Status            string    `json:"status"`
	Source            string    `json:"source"`
	LastSyncedAt      time.Time `json:"last_synced_at"`
}

type JITProvisionProfile struct {
	FullName          string `json:"full_name"`
	Department        string `json:"department"`
	JobTitle          string `json:"job_title"`
	Location          string `json:"location"`
	Phone             string `json:"phone"`
	ManagerExternalID string `json:"manager_external_id"`
	EmploymentStatus  string `json:"employment_status"`
}

type JITProvisionApproval struct {
	ID                       string     `json:"id"`
	TenantID                 string     `json:"tenant_id"`
	Email                    string     `json:"email"`
	ExternalID               string     `json:"external_id,omitempty"`
	Provider                 string     `json:"provider,omitempty"`
	EmploymentStatus         string     `json:"employment_status,omitempty"`
	Status                   string     `json:"status"`
	Reason                   string     `json:"reason,omitempty"`
	ExternalSyncStatus       string     `json:"external_sync_status,omitempty"`
	ExternalSyncRef          string     `json:"external_sync_ref,omitempty"`
	ExternalSyncAttemptCount int        `json:"external_sync_attempt_count,omitempty"`
	ExternalSyncLastError    string     `json:"external_sync_last_error,omitempty"`
	ExternalSyncUpdatedAt    *time.Time `json:"external_sync_updated_at,omitempty"`
	ReviewedBy               string     `json:"reviewed_by,omitempty"`
	ReviewedAt               *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

type SyncJob struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Source      string    `json:"source"`
	Status      string    `json:"status"`
	Total       int       `json:"total"`
	Created     int       `json:"created"`
	Updated     int       `json:"updated"`
	Deactivated int       `json:"deactivated"`
	Rejected    int       `json:"rejected"`
	Actor       string    `json:"actor"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
}

type SyncResult struct {
	Job   SyncJob              `json:"job"`
	Items []EnterpriseEmployee `json:"items"`
}

type AccessSyncApplier func(items []EnterpriseEmployee) (created, updated, rejected int, err error)

type SyncRequestRecord struct {
	RequestID           string     `json:"request_id"`
	TenantID            string     `json:"tenant_id"`
	ConnectorID         string     `json:"connector_id,omitempty"`
	RawPayloadRef       string     `json:"raw_payload_ref,omitempty"`
	Result              SyncResult `json:"result"`
	AccessApplied       bool       `json:"access_applied"`
	AccessCreated       int        `json:"access_created"`
	AccessUpdated       int        `json:"access_updated"`
	AccessRejected      int        `json:"access_rejected"`
	AccessAttemptCount  int        `json:"access_attempt_count"`
	LastAccessError     string     `json:"last_access_error,omitempty"`
	LastAccessAttemptAt *time.Time `json:"last_access_attempt_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

type PendingSyncReconcileResult struct {
	RequestID      string     `json:"request_id"`
	JobID          string     `json:"job_id"`
	AccessApplied  bool       `json:"access_applied"`
	AccessCreated  int        `json:"access_created"`
	AccessUpdated  int        `json:"access_updated"`
	AccessRejected int        `json:"access_rejected"`
	AttemptCount   int        `json:"access_attempt_count"`
	LastError      string     `json:"last_access_error,omitempty"`
	AttemptedAt    *time.Time `json:"last_access_attempt_at,omitempty"`
}

type BatchPendingSyncReconcileResult struct {
	Processed             int                          `json:"processed"`
	Applied               int                          `json:"applied"`
	Failed                int                          `json:"failed"`
	SkippedByAttemptLimit int                          `json:"skipped_by_attempt_limit,omitempty"`
	SkippedByCooldown     int                          `json:"skipped_by_cooldown,omitempty"`
	Items                 []PendingSyncReconcileResult `json:"items"`
}

type SyncWorkerAlertSubscriptionChannels struct {
	Email    bool `json:"email"`
	WhatsApp bool `json:"whatsapp"`
}

type SyncWorkerAlertSubscription struct {
	TenantID             string                              `json:"tenant_id"`
	Enabled              bool                                `json:"enabled"`
	WorkerAlertThreshold int                                 `json:"worker_alert_threshold"`
	WindowSeconds        int64                               `json:"window_seconds"`
	CooldownSeconds      int64                               `json:"cooldown_seconds"`
	Channels             SyncWorkerAlertSubscriptionChannels `json:"channels"`
	ReceiverGroups       []string                            `json:"receiver_groups,omitempty"`
	UpdatedAt            time.Time                           `json:"updated_at"`
}

type SyncWorkerAlertSubscriptionUpsertOptions struct {
	TenantID             string
	Enabled              bool
	WorkerAlertThreshold int
	Window               time.Duration
	Cooldown             time.Duration
	EmailEnabled         bool
	WhatsAppEnabled      bool
	ReceiverGroups       []string
}

type AuthStateToken struct {
	Token       string    `json:"state_token"`
	TenantID    string    `json:"tenant_id"`
	Provider    string    `json:"provider"`
	Email       string    `json:"email,omitempty"`
	RedirectURI string    `json:"redirect_uri"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

type compareAndSwapStateStore interface {
	CompareAndSwap(key string, expectedExists bool, expected any, next any) (bool, error)
}

const (
	stateKey                = "module_enterprise"
	hrisWebhookStateKey     = "module_enterprise_hris_webhook_runtime"
	syncWorkerAlertStateKey = "module_enterprise_sync_worker_alert"
)

const (
	defaultReconcileLimit              = 20
	maxReconcileLimit                  = 200
	defaultAuthStateTokenTTL           = 5 * time.Minute
	maxWebhookReceiptLimit             = 200
	maxWebhookExecutionLimit           = 500
	maxEnterpriseHRISWebhookCASRetries = 5
	maxSyncWorkerAlertCASRetries       = 5
)

type stateSnapshot struct {
	DomainMappings               []DomainMapping               `json:"domain_mappings"`
	HRISConnectors               []HRISConnector               `json:"hris_connectors,omitempty"`
	HRISWebhookReceipts          []HRISWebhookReceipt          `json:"hris_webhook_receipts,omitempty"`
	HRISWebhookExecutions        []HRISWebhookExecution        `json:"hris_webhook_executions,omitempty"`
	IDPConfigs                   map[string]IDPConfig          `json:"idp_configs"`
	Employees                    []EnterpriseEmployee          `json:"employees"`
	SyncJobs                     []SyncJob                     `json:"sync_jobs"`
	SyncRequestRecords           map[string]SyncRequestRecord  `json:"sync_request_records,omitempty"`
	JITProvisionApprovals        []JITProvisionApproval        `json:"jit_provision_approvals,omitempty"`
	SyncWorkerAlertSubscriptions []SyncWorkerAlertSubscription `json:"sync_worker_alert_subscriptions,omitempty"`
	SyncWorkerAlertNotifications []SyncWorkerAlertNotification `json:"sync_worker_alert_notifications,omitempty"`
	SyncWorkerAlertCooldowns     []SyncWorkerAlertCooldown     `json:"sync_worker_alert_cooldowns,omitempty"`
}

type hrisWebhookStateSnapshot struct {
	HRISWebhookReceipts         []HRISWebhookReceipt              `json:"hris_webhook_receipts,omitempty"`
	HRISWebhookExecutions       []HRISWebhookExecution            `json:"hris_webhook_executions,omitempty"`
	DueReceiptIDs               []hrisWebhookReceiptDueIndexEntry `json:"due_receipt_ids,omitempty"`
	QueuedReceiptExecutionIDs   []string                          `json:"queued_receipt_execution_ids,omitempty"`
	QueuedDLQReplayExecutionIDs []string                          `json:"queued_dlq_replay_execution_ids,omitempty"`
}

type syncWorkerAlertStateSnapshot struct {
	SyncWorkerAlertSubscriptions []SyncWorkerAlertSubscription `json:"sync_worker_alert_subscriptions,omitempty"`
	SyncWorkerAlertNotifications []SyncWorkerAlertNotification `json:"sync_worker_alert_notifications,omitempty"`
	SyncWorkerAlertCooldowns     []SyncWorkerAlertCooldown     `json:"sync_worker_alert_cooldowns,omitempty"`
	SyncWorkerAlertInFlights     []SyncWorkerAlertInFlight     `json:"sync_worker_alert_in_flights,omitempty"`
}

type Service struct {
	mu                           sync.RWMutex
	domainMappings               []DomainMapping
	hrisConnectors               []HRISConnector
	hrisWebhookReceipts          []HRISWebhookReceipt
	hrisWebhookExecutions        []HRISWebhookExecution
	dueReceiptIDs                []hrisWebhookReceiptDueIndexEntry
	queuedReceiptExecutionIDs    []string
	queuedDLQReplayExecutionIDs  []string
	idpConfigs                   map[string]IDPConfig
	employees                    []EnterpriseEmployee
	syncJobs                     []SyncJob
	syncRequestRecords           map[string]SyncRequestRecord
	jitProvisionApprovals        []JITProvisionApproval
	syncWorkerAlertSubscriptions []SyncWorkerAlertSubscription
	syncWorkerAlertNotifications []SyncWorkerAlertNotification
	syncWorkerAlertCooldowns     []SyncWorkerAlertCooldown
	syncWorkerAlertInFlights     []SyncWorkerAlertInFlight
	authStateTokens              map[string]AuthStateToken
	stateStore                   StateStore
}

func NewService() *Service {
	now := time.Now().UTC()
	return &Service{
		domainMappings: []DomainMapping{
			{
				ID:        "dm_001",
				TenantID:  "tenant_demo_jakarta",
				Domain:    "sudirman.co",
				Status:    "active",
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        "dm_002",
				TenantID:  "tenant_demo_factory",
				Domain:    "factory.local",
				Status:    "active",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		hrisConnectors:              []HRISConnector{},
		hrisWebhookReceipts:         []HRISWebhookReceipt{},
		hrisWebhookExecutions:       []HRISWebhookExecution{},
		dueReceiptIDs:               []hrisWebhookReceiptDueIndexEntry{},
		queuedReceiptExecutionIDs:   []string{},
		queuedDLQReplayExecutionIDs: []string{},
		idpConfigs: map[string]IDPConfig{
			"tenant_demo_jakarta": {
				ID:          "idp_001",
				TenantID:    "tenant_demo_jakarta",
				Provider:    "oidc",
				IssuerURL:   "https://id.sudirman.co",
				ClientID:    "mistypass-web-admin",
				AuthURL:     "https://id.sudirman.co/oauth2/auth",
				TokenURL:    "https://id.sudirman.co/oauth2/token",
				JWKSURL:     "https://id.sudirman.co/.well-known/jwks.json",
				UserInfoURL: "https://id.sudirman.co/oauth2/userinfo",
				Scopes:      []string{"openid", "profile", "email"},
				Status:      "active",
				SyncMode:    "jit",
				UpdatedBy:   "system",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		employees: []EnterpriseEmployee{
			{
				ID:               "emp_001",
				TenantID:         "tenant_demo_jakarta",
				ExternalID:       "hris-jkt-1001",
				Email:            "arief.putra@sudirman.co",
				FullName:         "Arief Putra",
				Department:       "Finance",
				JobTitle:         "Finance Manager",
				Location:         "Jakarta",
				EmploymentStatus: "active",
				AccessRole:       "resident",
				BuildingID:       "building_demo_001",
				GroupIDs:         []string{"ug_1001"},
				Status:           "active",
				Source:           "seed",
				LastSyncedAt:     now,
			},
		},
		syncJobs:                     []SyncJob{},
		syncRequestRecords:           map[string]SyncRequestRecord{},
		jitProvisionApprovals:        []JITProvisionApproval{},
		syncWorkerAlertSubscriptions: []SyncWorkerAlertSubscription{},
		syncWorkerAlertNotifications: []SyncWorkerAlertNotification{},
		syncWorkerAlertCooldowns:     []SyncWorkerAlertCooldown{},
		authStateTokens:              map[string]AuthStateToken{},
	}
}

func NewServiceWithStateStore(store StateStore) (*Service, error) {
	svc := NewService()
	svc.stateStore = store
	if err := svc.restoreFromStateStore(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *Service) ListDomainMappings(tenantID string) []DomainMapping {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]DomainMapping, 0, len(s.domainMappings))
	for i := range s.domainMappings {
		if filterTenantID != "" && s.domainMappings[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.domainMappings[i])
	}
	return items
}

func (s *Service) CreateDomainMapping(tenantID, domain, status string) (DomainMapping, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return DomainMapping{}, ErrTenantIDRequired
	}

	nextDomain, err := normalizeDomain(domain)
	if err != nil {
		return DomainMapping{}, err
	}

	nextStatus, err := normalizeDomainMappingStatus(status)
	if err != nil {
		return DomainMapping{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.domainMappings {
		if s.domainMappings[i].Domain == nextDomain {
			return DomainMapping{}, ErrDomainAlreadyMapped
		}
	}

	id, err := randomID("dm_")
	if err != nil {
		return DomainMapping{}, err
	}

	now := time.Now().UTC()
	record := DomainMapping{
		ID:        id,
		TenantID:  nextTenantID,
		Domain:    nextDomain,
		Status:    nextStatus,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.domainMappings = append([]DomainMapping{record}, s.domainMappings...)
	if err := s.persistLocked(); err != nil {
		return DomainMapping{}, err
	}

	return record, nil
}

func (s *Service) UpdateDomainMappingStatus(tenantID, mappingID, status string) (DomainMapping, error) {
	nextMappingID := strings.TrimSpace(mappingID)
	if nextMappingID == "" {
		return DomainMapping{}, ErrDomainMappingNotFound
	}

	nextStatus, err := normalizeDomainMappingStatus(status)
	if err != nil {
		return DomainMapping{}, err
	}

	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.domainMappings {
		if s.domainMappings[i].ID != nextMappingID {
			continue
		}
		if filterTenantID != "" && s.domainMappings[i].TenantID != filterTenantID {
			return DomainMapping{}, ErrDomainMappingNotFound
		}
		s.domainMappings[i].Status = nextStatus
		s.domainMappings[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return DomainMapping{}, err
		}
		return s.domainMappings[i], nil
	}

	return DomainMapping{}, ErrDomainMappingNotFound
}

func (s *Service) ListHRISConnectors(tenantID string) []HRISConnector {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]HRISConnector, 0, len(s.hrisConnectors))
	for i := range s.hrisConnectors {
		if filterTenantID != "" && strings.TrimSpace(s.hrisConnectors[i].TenantID) != filterTenantID {
			continue
		}
		items = append(items, cloneHRISConnector(s.hrisConnectors[i]))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items
}

func (s *Service) GetHRISConnector(tenantID, connectorID string) (HRISConnector, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISConnector{}, ErrTenantIDRequired
	}
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return HRISConnector{}, ErrHRISConnectorNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.hrisConnectors {
		item := s.hrisConnectors[i]
		if strings.TrimSpace(item.ID) != nextConnectorID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return HRISConnector{}, ErrHRISConnectorNotFound
		}
		return cloneHRISConnector(item), nil
	}
	return HRISConnector{}, ErrHRISConnectorNotFound
}

func (s *Service) GetHRISConnectorByID(connectorID string) (HRISConnector, error) {
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return HRISConnector{}, ErrHRISConnectorNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.hrisConnectors {
		item := s.hrisConnectors[i]
		if strings.TrimSpace(item.ID) != nextConnectorID {
			continue
		}
		return cloneHRISConnector(item), nil
	}
	return HRISConnector{}, ErrHRISConnectorNotFound
}

func (s *Service) CreateHRISConnector(
	tenantID string,
	vendor string,
	status string,
	syncStrategy string,
	credentialRef string,
	webhookSecretRef string,
	updatedBy string,
) (HRISConnector, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISConnector{}, ErrTenantIDRequired
	}
	nextVendor, err := normalizeHRISConnectorVendor(vendor)
	if err != nil {
		return HRISConnector{}, err
	}
	nextStatus, err := normalizeHRISConnectorStatus(status)
	if err != nil {
		return HRISConnector{}, err
	}
	nextSyncStrategy, err := normalizeHRISConnectorSyncStrategy(syncStrategy)
	if err != nil {
		return HRISConnector{}, err
	}
	nextCredentialRef := strings.TrimSpace(credentialRef)
	nextWebhookSecretRef := strings.TrimSpace(webhookSecretRef)
	nextUpdatedBy := strings.TrimSpace(updatedBy)
	if nextUpdatedBy == "" {
		nextUpdatedBy = "system"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.hrisConnectors {
		item := s.hrisConnectors[i]
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		if strings.TrimSpace(item.Vendor) == nextVendor {
			return HRISConnector{}, ErrHRISConnectorAlreadyExists
		}
	}

	connectorID, err := randomID("hrc_")
	if err != nil {
		return HRISConnector{}, err
	}
	now := time.Now().UTC()
	record := HRISConnector{
		ID:               connectorID,
		TenantID:         nextTenantID,
		Vendor:           nextVendor,
		Status:           nextStatus,
		SyncStrategy:     nextSyncStrategy,
		CredentialRef:    nextCredentialRef,
		WebhookSecretRef: nextWebhookSecretRef,
		UpdatedBy:        nextUpdatedBy,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.hrisConnectors = append([]HRISConnector{record}, s.hrisConnectors...)
	if err := s.persistLocked(); err != nil {
		return HRISConnector{}, err
	}
	return cloneHRISConnector(record), nil
}

func (s *Service) UpdateHRISConnector(
	tenantID string,
	connectorID string,
	status string,
	syncStrategy string,
	credentialRef string,
	webhookSecretRef string,
	updatedBy string,
) (HRISConnector, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISConnector{}, ErrTenantIDRequired
	}
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return HRISConnector{}, ErrHRISConnectorNotFound
	}

	nextStatus := ""
	if strings.TrimSpace(status) != "" {
		var err error
		nextStatus, err = normalizeHRISConnectorStatus(status)
		if err != nil {
			return HRISConnector{}, err
		}
	}
	nextSyncStrategy := ""
	if strings.TrimSpace(syncStrategy) != "" {
		var err error
		nextSyncStrategy, err = normalizeHRISConnectorSyncStrategy(syncStrategy)
		if err != nil {
			return HRISConnector{}, err
		}
	}
	nextCredentialRef := strings.TrimSpace(credentialRef)
	nextWebhookSecretRef := strings.TrimSpace(webhookSecretRef)
	nextUpdatedBy := strings.TrimSpace(updatedBy)
	if nextUpdatedBy == "" {
		nextUpdatedBy = "system"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.hrisConnectors {
		item := s.hrisConnectors[i]
		if strings.TrimSpace(item.ID) != nextConnectorID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return HRISConnector{}, ErrHRISConnectorNotFound
		}
		if nextStatus != "" {
			item.Status = nextStatus
		}
		if nextSyncStrategy != "" {
			item.SyncStrategy = nextSyncStrategy
		}
		if nextCredentialRef != "" {
			item.CredentialRef = nextCredentialRef
		}
		if nextWebhookSecretRef != "" {
			item.WebhookSecretRef = nextWebhookSecretRef
		}
		item.UpdatedBy = nextUpdatedBy
		item.UpdatedAt = time.Now().UTC()
		s.hrisConnectors[i] = item
		if err := s.persistLocked(); err != nil {
			return HRISConnector{}, err
		}
		return cloneHRISConnector(item), nil
	}

	return HRISConnector{}, ErrHRISConnectorNotFound
}

func (s *Service) MarkHRISConnectorSynced(
	tenantID string,
	connectorID string,
	syncedAt time.Time,
) (HRISConnector, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISConnector{}, ErrTenantIDRequired
	}
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return HRISConnector{}, ErrHRISConnectorNotFound
	}
	now := syncedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.hrisConnectors {
		item := s.hrisConnectors[i]
		if strings.TrimSpace(item.ID) != nextConnectorID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return HRISConnector{}, ErrHRISConnectorNotFound
		}
		item.LastSyncAt = &now
		item.UpdatedAt = now
		s.hrisConnectors[i] = item
		if err := s.persistLocked(); err != nil {
			return HRISConnector{}, err
		}
		return cloneHRISConnector(item), nil
	}

	return HRISConnector{}, ErrHRISConnectorNotFound
}

func (s *Service) ListHRISWebhookReceipts(tenantID, connectorID string, limit int) []HRISWebhookReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := listHRISWebhookReceipts(s.hrisWebhookReceipts, tenantID, connectorID)
	if limit <= 0 || limit > maxWebhookReceiptLimit {
		limit = maxWebhookReceiptLimit
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) ListAllHRISWebhookReceipts(tenantID, connectorID string) []HRISWebhookReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return listHRISWebhookReceipts(s.hrisWebhookReceipts, tenantID, connectorID)
}

func listHRISWebhookReceipts(items []HRISWebhookReceipt, tenantID, connectorID string) []HRISWebhookReceipt {
	filterTenantID := strings.TrimSpace(tenantID)
	filterConnectorID := strings.TrimSpace(connectorID)
	result := make([]HRISWebhookReceipt, 0, len(items))
	for i := range items {
		item := items[i]
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		if filterConnectorID != "" && strings.TrimSpace(item.ConnectorID) != filterConnectorID {
			continue
		}
		result = append(result, cloneHRISWebhookReceipt(item))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ReceivedAt.Equal(result[j].ReceivedAt) {
			return result[i].ID > result[j].ID
		}
		return result[i].ReceivedAt.After(result[j].ReceivedAt)
	})
	return result
}

func (s *Service) GetHRISWebhookReceipt(tenantID, receiptID string) (HRISWebhookReceipt, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISWebhookReceipt{}, ErrTenantIDRequired
	}
	nextReceiptID := strings.TrimSpace(receiptID)
	if nextReceiptID == "" {
		return HRISWebhookReceipt{}, ErrHRISWebhookReceiptNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.hrisWebhookReceipts {
		item := s.hrisWebhookReceipts[i]
		if strings.TrimSpace(item.ID) != nextReceiptID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return HRISWebhookReceipt{}, ErrHRISWebhookReceiptNotFound
		}
		return cloneHRISWebhookReceipt(item), nil
	}
	return HRISWebhookReceipt{}, ErrHRISWebhookReceiptNotFound
}

func findHRISWebhookReceiptByIDLocked(
	items []HRISWebhookReceipt,
	receiptID string,
) (HRISWebhookReceipt, bool) {
	nextReceiptID := strings.TrimSpace(receiptID)
	if nextReceiptID == "" {
		return HRISWebhookReceipt{}, false
	}
	for i := range items {
		if strings.TrimSpace(items[i].ID) != nextReceiptID {
			continue
		}
		return items[i], true
	}
	return HRISWebhookReceipt{}, false
}

func (s *Service) ReceiveHRISWebhookReceipt(connectorID string, input HRISWebhookReceiptInput) (HRISWebhookReceipt, error) {
	nextConnectorID := strings.TrimSpace(connectorID)
	if nextConnectorID == "" {
		return HRISWebhookReceipt{}, ErrHRISConnectorNotFound
	}

	nextEventType := strings.TrimSpace(input.EventType)
	nextRequestID := strings.TrimSpace(input.RequestID)
	nextContentType := strings.TrimSpace(input.ContentType)
	nextSourceIP := strings.TrimSpace(input.SourceIP)
	nextHeaders := make(map[string]string, len(input.Headers))
	for key, value := range input.Headers {
		nextKey := strings.ToLower(strings.TrimSpace(key))
		if nextKey == "" {
			continue
		}
		nextHeaders[nextKey] = strings.TrimSpace(value)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	connectorIndex := findHRISConnectorIndexLocked(s.hrisConnectors, nextConnectorID)
	if connectorIndex < 0 {
		return HRISWebhookReceipt{}, ErrHRISConnectorNotFound
	}
	connector := s.hrisConnectors[connectorIndex]
	if strings.TrimSpace(connector.Status) != "active" {
		return HRISWebhookReceipt{}, ErrHRISConnectorInactive
	}

	receiptID, err := randomID("whr_")
	if err != nil {
		return HRISWebhookReceipt{}, err
	}
	now := time.Now().UTC()
	record := HRISWebhookReceipt{
		ID:          receiptID,
		EventType:   nextEventType,
		RequestID:   nextRequestID,
		ContentType: nextContentType,
		Headers:     nextHeaders,
		RawPayload:  input.RawPayload,
		SourceIP:    nextSourceIP,
		Status:      "received",
		ReceivedAt:  now,
	}
	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		connectorIndex := findHRISConnectorIndexLocked(s.hrisConnectors, nextConnectorID)
		if connectorIndex < 0 {
			return false, ErrHRISConnectorNotFound
		}
		currentConnector := s.hrisConnectors[connectorIndex]
		if strings.TrimSpace(currentConnector.Status) != "active" {
			return false, ErrHRISConnectorInactive
		}

		record.TenantID = currentConnector.TenantID
		record.ConnectorID = currentConnector.ID
		record.Vendor = currentConnector.Vendor
		s.hrisWebhookReceipts = append([]HRISWebhookReceipt{record}, s.hrisWebhookReceipts...)
		s.upsertHRISWebhookReceiptDueIndexLocked(record.ID, record.ReceivedAt)
		return true, nil
	}); err != nil {
		return HRISWebhookReceipt{}, err
	}
	return cloneHRISWebhookReceipt(record), nil
}

func (s *Service) ListPendingHRISWebhookReceipts(tenantID string, limit int) []HRISWebhookReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]HRISWebhookReceipt, 0, len(s.hrisWebhookReceipts))
	for i := range s.hrisWebhookReceipts {
		item := s.hrisWebhookReceipts[i]
		if strings.TrimSpace(item.Status) != "received" {
			continue
		}
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		items = append(items, cloneHRISWebhookReceipt(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ReceivedAt.Equal(items[j].ReceivedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].ReceivedAt.After(items[j].ReceivedAt)
	})
	if limit <= 0 || limit > maxWebhookReceiptLimit {
		limit = maxWebhookReceiptLimit
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) ListRetryableHRISWebhookReceipts(tenantID string, limit int) []HRISWebhookReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]HRISWebhookReceipt, 0, len(s.hrisWebhookReceipts))
	for i := range s.hrisWebhookReceipts {
		item := s.hrisWebhookReceipts[i]
		status := strings.TrimSpace(item.Status)
		if status != "received" && status != "failed" {
			continue
		}
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		items = append(items, cloneHRISWebhookReceipt(item))
	}
	sort.Slice(items, func(i, j int) bool {
		leftAttemptAt := items[i].ReceivedAt
		if items[i].LastAttemptAt != nil {
			leftAttemptAt = *items[i].LastAttemptAt
		}
		rightAttemptAt := items[j].ReceivedAt
		if items[j].LastAttemptAt != nil {
			rightAttemptAt = *items[j].LastAttemptAt
		}
		if leftAttemptAt.Equal(rightAttemptAt) {
			return items[i].ID > items[j].ID
		}
		return leftAttemptAt.Before(rightAttemptAt)
	})
	if limit <= 0 || limit > maxWebhookReceiptLimit {
		limit = maxWebhookReceiptLimit
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) ListQueueableHRISWebhookReceipts(tenantID string, limit int) []HRISWebhookReceipt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]HRISWebhookReceipt, 0, len(s.hrisWebhookReceipts))
	for i := range s.hrisWebhookReceipts {
		item := s.hrisWebhookReceipts[i]
		status := strings.TrimSpace(item.Status)
		if status != "received" && status != "failed" && status != "processing" {
			continue
		}
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		items = append(items, cloneHRISWebhookReceipt(item))
	}
	sort.Slice(items, func(i, j int) bool {
		leftAttemptAt := items[i].ReceivedAt
		if items[i].LastAttemptAt != nil {
			leftAttemptAt = *items[i].LastAttemptAt
		}
		rightAttemptAt := items[j].ReceivedAt
		if items[j].LastAttemptAt != nil {
			rightAttemptAt = *items[j].LastAttemptAt
		}
		if leftAttemptAt.Equal(rightAttemptAt) {
			return items[i].ID > items[j].ID
		}
		return leftAttemptAt.Before(rightAttemptAt)
	})
	if limit <= 0 || limit > maxWebhookReceiptLimit {
		limit = maxWebhookReceiptLimit
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) ListClaimableHRISWebhookReceiptsWithBackoff(
	tenantID string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookReceipt {
	maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now = normalizeHRISWebhookReceiptClaimParams(
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)

	s.mu.RLock()
	defer s.mu.RUnlock()

	return listClaimableHRISWebhookReceiptsWithBackoffLocked(
		s.hrisWebhookReceipts,
		tenantID,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
		limit,
	)
}

func (s *Service) ListDueHRISWebhookReceiptsWithBackoff(
	tenantID string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookReceipt {
	maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now = normalizeHRISWebhookReceiptClaimParams(
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)

	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]HRISWebhookReceipt, 0, limit)
	seen := make(map[string]struct{}, len(s.dueReceiptIDs))
	for i := range s.dueReceiptIDs {
		entry := s.dueReceiptIDs[i]
		if !entry.DueAt.IsZero() && entry.DueAt.After(now) {
			break
		}

		item, ok := findHRISWebhookReceiptByIDLocked(s.hrisWebhookReceipts, entry.ReceiptID)
		if !ok {
			continue
		}
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		if hrisWebhookReceiptClaimReason(item, maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now) != "" {
			continue
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		items = append(items, cloneHRISWebhookReceipt(item))
		if limit > 0 && len(items) >= limit {
			return items
		}
	}

	fallbackItems := listClaimableHRISWebhookReceiptsWithBackoffLocked(
		s.hrisWebhookReceipts,
		tenantID,
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
		0,
	)
	for i := range fallbackItems {
		if _, exists := seen[fallbackItems[i].ID]; exists {
			continue
		}
		seen[fallbackItems[i].ID] = struct{}{}
		items = append(items, fallbackItems[i])
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items
}

func listClaimableHRISWebhookReceiptsWithBackoffLocked(
	allReceipts []HRISWebhookReceipt,
	tenantID string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookReceipt {
	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]HRISWebhookReceipt, 0, len(allReceipts))
	for i := range allReceipts {
		item := allReceipts[i]
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		if hrisWebhookReceiptClaimReason(item, maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now) != "" {
			continue
		}
		items = append(items, cloneHRISWebhookReceipt(item))
	}
	sort.Slice(items, func(i, j int) bool {
		leftAttemptAt := items[i].ReceivedAt
		if items[i].LastAttemptAt != nil {
			leftAttemptAt = *items[i].LastAttemptAt
		}
		rightAttemptAt := items[j].ReceivedAt
		if items[j].LastAttemptAt != nil {
			rightAttemptAt = *items[j].LastAttemptAt
		}
		if leftAttemptAt.Equal(rightAttemptAt) {
			return items[i].ID > items[j].ID
		}
		return leftAttemptAt.Before(rightAttemptAt)
	})
	if limit <= 0 || limit > maxWebhookReceiptLimit {
		limit = maxWebhookReceiptLimit
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) ClaimHRISWebhookReceiptForProcessing(
	tenantID string,
	receiptID string,
	maxAttempts int,
	retryCooldown time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (HRISWebhookReceipt, string, error) {
	return s.ClaimHRISWebhookReceiptForProcessingWithBackoff(
		tenantID,
		receiptID,
		maxAttempts,
		retryCooldown,
		retryCooldown,
		processingTimeout,
		now,
	)
}

func (s *Service) ClaimHRISWebhookReceiptForProcessingWithBackoff(
	tenantID string,
	receiptID string,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (HRISWebhookReceipt, string, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISWebhookReceipt{}, "", ErrTenantIDRequired
	}
	nextReceiptID := strings.TrimSpace(receiptID)
	if nextReceiptID == "" {
		return HRISWebhookReceipt{}, "", ErrHRISWebhookReceiptNotFound
	}
	maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now = normalizeHRISWebhookReceiptClaimParams(
		maxAttempts,
		retryCooldown,
		retryMaxBackoff,
		processingTimeout,
		now,
	)

	s.mu.Lock()
	defer s.mu.Unlock()

	var claimed HRISWebhookReceipt
	claimReason := ""
	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		for i := range s.hrisWebhookReceipts {
			if strings.TrimSpace(s.hrisWebhookReceipts[i].ID) != nextReceiptID {
				continue
			}
			if strings.TrimSpace(s.hrisWebhookReceipts[i].TenantID) != nextTenantID {
				return false, ErrHRISWebhookReceiptNotFound
			}

			if reason := hrisWebhookReceiptClaimReason(
				s.hrisWebhookReceipts[i],
				maxAttempts,
				retryCooldown,
				retryMaxBackoff,
				processingTimeout,
				now,
			); reason != "" {
				claimed = cloneHRISWebhookReceipt(s.hrisWebhookReceipts[i])
				claimReason = reason
				return false, nil
			}

			s.hrisWebhookReceipts[i].Status = "processing"
			s.hrisWebhookReceipts[i].LastError = ""
			s.hrisWebhookReceipts[i].ProcessedAt = nil
			s.hrisWebhookReceipts[i].AttemptCount++
			s.hrisWebhookReceipts[i].LastAttemptAt = &now
			s.upsertHRISWebhookReceiptDueIndexLocked(
				s.hrisWebhookReceipts[i].ID,
				hrisWebhookReceiptProcessingDueAt(now, processingTimeout),
			)
			claimed = cloneHRISWebhookReceipt(s.hrisWebhookReceipts[i])
			claimReason = ""
			return true, nil
		}
		return false, ErrHRISWebhookReceiptNotFound
	}); err != nil {
		return HRISWebhookReceipt{}, "", err
	}
	if claimReason != "" {
		return claimed, claimReason, nil
	}
	if claimed.ID == "" {
		return HRISWebhookReceipt{}, "", ErrHRISWebhookReceiptNotFound
	}
	return claimed, "", nil
}

func normalizeHRISWebhookReceiptClaimParams(
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) (int, time.Duration, time.Duration, time.Duration, time.Time) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryCooldown, retryMaxBackoff = retrybackoff.Normalize(retryCooldown, retryMaxBackoff)
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	return maxAttempts, retryCooldown, retryMaxBackoff, processingTimeout, now
}

func hrisWebhookReceiptClaimReason(
	item HRISWebhookReceipt,
	maxAttempts int,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	processingTimeout time.Duration,
	now time.Time,
) string {
	status := strings.TrimSpace(item.Status)
	switch status {
	case "received":
		return ""
	case "failed":
		if maxAttempts > 0 && item.AttemptCount >= maxAttempts {
			return HRISWebhookReceiptClaimReasonAttemptLimit
		}
		if retryDelay := retrybackoff.Exponential(
			item.AttemptCount,
			retryCooldown,
			retryMaxBackoff,
		); retryDelay > 0 && item.LastAttemptAt != nil {
			if item.LastAttemptAt.Add(retryDelay).After(now) {
				return HRISWebhookReceiptClaimReasonCooldown
			}
		}
		return ""
	case "processing":
		if item.LastAttemptAt != nil && item.LastAttemptAt.Add(processingTimeout).After(now) {
			return HRISWebhookReceiptClaimReasonInFlight
		}
		return ""
	default:
		return HRISWebhookReceiptClaimReasonNotQueueable
	}
}

func isQueueableHRISWebhookReceiptStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "received", "failed", "processing":
		return true
	default:
		return false
	}
}

func hrisWebhookReceiptDueIndexHeuristic(item HRISWebhookReceipt) time.Time {
	if item.LastAttemptAt != nil && !item.LastAttemptAt.IsZero() {
		return item.LastAttemptAt.UTC()
	}
	return item.ReceivedAt.UTC()
}

func hrisWebhookReceiptProcessingDueAt(now time.Time, processingTimeout time.Duration) time.Time {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if processingTimeout <= 0 {
		return now
	}
	return now.Add(processingTimeout)
}

func hrisWebhookReceiptFailureDueAt(
	item HRISWebhookReceipt,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
	now time.Time,
) time.Time {
	base := now
	if item.LastAttemptAt != nil && !item.LastAttemptAt.IsZero() {
		base = item.LastAttemptAt.UTC()
	} else if now.IsZero() {
		base = time.Now().UTC()
	} else {
		base = now.UTC()
	}
	return base.Add(retrybackoff.Exponential(item.AttemptCount, retryCooldown, retryMaxBackoff))
}

func sortHRISWebhookReceiptDueIndexEntries(items []hrisWebhookReceiptDueIndexEntry) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].DueAt.Equal(items[j].DueAt) {
			return items[i].ReceiptID < items[j].ReceiptID
		}
		return items[i].DueAt.Before(items[j].DueAt)
	})
}

func (s *Service) upsertHRISWebhookReceiptDueIndexLocked(receiptID string, dueAt time.Time) {
	nextReceiptID := strings.TrimSpace(receiptID)
	if nextReceiptID == "" {
		return
	}
	nextDueAt := dueAt
	if nextDueAt.IsZero() {
		nextDueAt = time.Now().UTC()
	} else {
		nextDueAt = nextDueAt.UTC()
	}
	for i := range s.dueReceiptIDs {
		if strings.TrimSpace(s.dueReceiptIDs[i].ReceiptID) != nextReceiptID {
			continue
		}
		s.dueReceiptIDs[i].DueAt = nextDueAt
		sortHRISWebhookReceiptDueIndexEntries(s.dueReceiptIDs)
		return
	}
	s.dueReceiptIDs = append(s.dueReceiptIDs, hrisWebhookReceiptDueIndexEntry{
		ReceiptID: nextReceiptID,
		DueAt:     nextDueAt,
	})
	sortHRISWebhookReceiptDueIndexEntries(s.dueReceiptIDs)
}

func (s *Service) removeHRISWebhookReceiptDueIndexLocked(receiptID string) {
	nextReceiptID := strings.TrimSpace(receiptID)
	if nextReceiptID == "" || len(s.dueReceiptIDs) == 0 {
		return
	}
	filtered := s.dueReceiptIDs[:0]
	for i := range s.dueReceiptIDs {
		if strings.TrimSpace(s.dueReceiptIDs[i].ReceiptID) == nextReceiptID {
			continue
		}
		filtered = append(filtered, s.dueReceiptIDs[i])
	}
	s.dueReceiptIDs = filtered
}

func (s *Service) normalizeHRISWebhookReceiptDueIndexLocked() {
	if len(s.hrisWebhookReceipts) == 0 {
		s.dueReceiptIDs = nil
		return
	}

	existing := make(map[string]hrisWebhookReceiptDueIndexEntry, len(s.dueReceiptIDs))
	for i := range s.dueReceiptIDs {
		entry := s.dueReceiptIDs[i]
		nextReceiptID := strings.TrimSpace(entry.ReceiptID)
		if nextReceiptID == "" {
			continue
		}
		if entry.DueAt.IsZero() {
			continue
		}
		entry.ReceiptID = nextReceiptID
		entry.DueAt = entry.DueAt.UTC()
		existing[nextReceiptID] = entry
	}

	normalized := make([]hrisWebhookReceiptDueIndexEntry, 0, len(s.hrisWebhookReceipts))
	for i := range s.hrisWebhookReceipts {
		item := s.hrisWebhookReceipts[i]
		if !isQueueableHRISWebhookReceiptStatus(item.Status) {
			continue
		}
		nextReceiptID := strings.TrimSpace(item.ID)
		if nextReceiptID == "" {
			continue
		}
		entry, ok := existing[nextReceiptID]
		if !ok {
			entry = hrisWebhookReceiptDueIndexEntry{
				ReceiptID: nextReceiptID,
				DueAt:     hrisWebhookReceiptDueIndexHeuristic(item),
			}
		}
		normalized = append(normalized, entry)
	}
	sortHRISWebhookReceiptDueIndexEntries(normalized)
	s.dueReceiptIDs = normalized
}

func buildHRISWebhookReceiptDueIndexEntries(
	items []HRISWebhookReceipt,
) []hrisWebhookReceiptDueIndexEntry {
	result := make([]hrisWebhookReceiptDueIndexEntry, 0, len(items))
	for i := range items {
		if !isQueueableHRISWebhookReceiptStatus(items[i].Status) {
			continue
		}
		nextReceiptID := strings.TrimSpace(items[i].ID)
		if nextReceiptID == "" {
			continue
		}
		result = append(result, hrisWebhookReceiptDueIndexEntry{
			ReceiptID: nextReceiptID,
			DueAt:     hrisWebhookReceiptDueIndexHeuristic(items[i]),
		})
	}
	sortHRISWebhookReceiptDueIndexEntries(result)
	return result
}

func (s *Service) MarkHRISWebhookReceiptStarted(tenantID, receiptID string) (HRISWebhookReceipt, error) {
	now := time.Now().UTC()
	return s.updateHRISWebhookReceiptStatus(
		tenantID,
		receiptID,
		"processing",
		"",
		nil,
		func(item *HRISWebhookReceipt) {
			item.AttemptCount++
			item.LastAttemptAt = &now
		},
		func(item HRISWebhookReceipt) {
			s.upsertHRISWebhookReceiptDueIndexLocked(item.ID, now)
		},
	)
}

func (s *Service) MarkHRISWebhookReceiptProcessed(tenantID, receiptID string) (HRISWebhookReceipt, error) {
	now := time.Now().UTC()
	return s.updateHRISWebhookReceiptStatus(
		tenantID,
		receiptID,
		"processed",
		"",
		&now,
		nil,
		func(item HRISWebhookReceipt) {
			s.removeHRISWebhookReceiptDueIndexLocked(item.ID)
		},
	)
}

func (s *Service) MarkHRISWebhookReceiptSkipped(tenantID, receiptID, reason string) (HRISWebhookReceipt, error) {
	now := time.Now().UTC()
	return s.updateHRISWebhookReceiptStatus(
		tenantID,
		receiptID,
		"skipped",
		reason,
		&now,
		nil,
		func(item HRISWebhookReceipt) {
			s.removeHRISWebhookReceiptDueIndexLocked(item.ID)
		},
	)
}

func (s *Service) MarkHRISWebhookReceiptFailed(tenantID, receiptID string, failure error) (HRISWebhookReceipt, error) {
	return s.MarkHRISWebhookReceiptFailedWithBackoff(tenantID, receiptID, failure, 0, 0)
}

func (s *Service) MarkHRISWebhookReceiptFailedWithBackoff(
	tenantID, receiptID string,
	failure error,
	retryCooldown time.Duration,
	retryMaxBackoff time.Duration,
) (HRISWebhookReceipt, error) {
	now := time.Now().UTC()
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	return s.updateHRISWebhookReceiptStatus(
		tenantID,
		receiptID,
		"failed",
		message,
		&now,
		nil,
		func(item HRISWebhookReceipt) {
			s.upsertHRISWebhookReceiptDueIndexLocked(
				item.ID,
				hrisWebhookReceiptFailureDueAt(item, retryCooldown, retryMaxBackoff, now),
			)
		},
	)
}

func (s *Service) MarkHRISWebhookReceiptDLQ(tenantID, receiptID string, failure error) (HRISWebhookReceipt, error) {
	now := time.Now().UTC()
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	return s.updateHRISWebhookReceiptStatus(
		tenantID,
		receiptID,
		"dlq",
		message,
		&now,
		nil,
		func(item HRISWebhookReceipt) {
			s.removeHRISWebhookReceiptDueIndexLocked(item.ID)
		},
	)
}

func (s *Service) RestoreHRISWebhookReceipt(snapshot HRISWebhookReceipt) (HRISWebhookReceipt, error) {
	nextTenantID := strings.TrimSpace(snapshot.TenantID)
	if nextTenantID == "" {
		return HRISWebhookReceipt{}, ErrTenantIDRequired
	}
	nextReceiptID := strings.TrimSpace(snapshot.ID)
	if nextReceiptID == "" {
		return HRISWebhookReceipt{}, ErrHRISWebhookReceiptNotFound
	}
	restoredSnapshot := cloneHRISWebhookReceipt(snapshot)

	s.mu.Lock()
	defer s.mu.Unlock()

	var restored HRISWebhookReceipt
	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		for i := range s.hrisWebhookReceipts {
			if strings.TrimSpace(s.hrisWebhookReceipts[i].ID) != nextReceiptID {
				continue
			}
			if strings.TrimSpace(s.hrisWebhookReceipts[i].TenantID) != nextTenantID {
				return false, ErrHRISWebhookReceiptNotFound
			}
			s.hrisWebhookReceipts[i] = cloneHRISWebhookReceipt(restoredSnapshot)
			restored = cloneHRISWebhookReceipt(s.hrisWebhookReceipts[i])
			return true, nil
		}
		return false, ErrHRISWebhookReceiptNotFound
	}); err != nil {
		return HRISWebhookReceipt{}, err
	}
	return restored, nil
}

func (s *Service) updateHRISWebhookReceiptStatus(
	tenantID string,
	receiptID string,
	status string,
	lastError string,
	processedAt *time.Time,
	mutate func(item *HRISWebhookReceipt),
	adjustDueIndex func(item HRISWebhookReceipt),
) (HRISWebhookReceipt, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISWebhookReceipt{}, ErrTenantIDRequired
	}
	nextReceiptID := strings.TrimSpace(receiptID)
	if nextReceiptID == "" {
		return HRISWebhookReceipt{}, ErrHRISWebhookReceiptNotFound
	}
	nextStatus := strings.ToLower(strings.TrimSpace(status))
	if nextStatus == "" {
		nextStatus = "received"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var updated HRISWebhookReceipt
	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		for i := range s.hrisWebhookReceipts {
			if strings.TrimSpace(s.hrisWebhookReceipts[i].ID) != nextReceiptID {
				continue
			}
			if strings.TrimSpace(s.hrisWebhookReceipts[i].TenantID) != nextTenantID {
				return false, ErrHRISWebhookReceiptNotFound
			}
			s.hrisWebhookReceipts[i].Status = nextStatus
			s.hrisWebhookReceipts[i].LastError = strings.TrimSpace(lastError)
			if processedAt == nil {
				s.hrisWebhookReceipts[i].ProcessedAt = nil
			} else {
				nextProcessedAt := processedAt.UTC()
				s.hrisWebhookReceipts[i].ProcessedAt = &nextProcessedAt
			}
			if mutate != nil {
				mutate(&s.hrisWebhookReceipts[i])
			}
			if adjustDueIndex != nil {
				adjustDueIndex(s.hrisWebhookReceipts[i])
			}
			updated = cloneHRISWebhookReceipt(s.hrisWebhookReceipts[i])
			return true, nil
		}
		return false, ErrHRISWebhookReceiptNotFound
	}); err != nil {
		return HRISWebhookReceipt{}, err
	}
	return updated, nil
}

func (s *Service) CreateHRISWebhookExecution(input HRISWebhookExecutionInput) (HRISWebhookExecution, error) {
	nextTenantID := strings.TrimSpace(input.TenantID)
	if nextTenantID == "" {
		return HRISWebhookExecution{}, ErrTenantIDRequired
	}
	nextKind := normalizeHRISWebhookExecutionKind(input.Kind)
	if nextKind == "" {
		return HRISWebhookExecution{}, ErrInvalidHRISWebhookExecutionKind
	}
	nextTargetID := strings.TrimSpace(input.TargetID)
	if nextTargetID == "" {
		return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
	}
	nextDispatchMode := normalizeHRISWebhookExecutionDispatchMode(input.DispatchMode)
	if nextDispatchMode == "" {
		return HRISWebhookExecution{}, ErrInvalidHRISWebhookExecutionDispatchMode
	}

	executionID, err := randomID("hwe_")
	if err != nil {
		return HRISWebhookExecution{}, err
	}
	now := time.Now().UTC()
	record := HRISWebhookExecution{
		ID:                      executionID,
		TenantID:                nextTenantID,
		Kind:                    nextKind,
		TargetID:                nextTargetID,
		ReceiptID:               strings.TrimSpace(input.ReceiptID),
		ConnectorID:             strings.TrimSpace(input.ConnectorID),
		Vendor:                  strings.TrimSpace(input.Vendor),
		RequestID:               strings.TrimSpace(input.RequestID),
		EventType:               strings.TrimSpace(input.EventType),
		FailureStage:            strings.TrimSpace(input.FailureStage),
		AuditSource:             strings.TrimSpace(input.AuditSource),
		ExecutionMode:           strings.TrimSpace(input.ExecutionMode),
		DispatchMode:            nextDispatchMode,
		Status:                  HRISWebhookExecutionStatusQueued,
		TargetStatus:            strings.TrimSpace(input.TargetStatus),
		RequestedBy:             strings.TrimSpace(input.RequestedBy),
		ReplaySourceExecutionID: strings.TrimSpace(input.ReplaySourceExecutionID),
		ReplayRequireWorker:     cloneOptionalBool(input.ReplayRequireWorker),
		QueuedAt:                now,
		UpdatedAt:               now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		if strings.TrimSpace(record.ReplaySourceExecutionID) != "" {
			if existing, ok := findActiveHRISWebhookReplayExecutionLocked(
				s.hrisWebhookExecutions,
				record.TenantID,
				record.ReplaySourceExecutionID,
			); ok {
				return false, &HRISWebhookExecutionReplayConflictError{
					ExistingExecution: cloneHRISWebhookExecution(existing),
				}
			}
		}
		s.hrisWebhookExecutions = append([]HRISWebhookExecution{record}, s.hrisWebhookExecutions...)
		if len(s.hrisWebhookExecutions) > maxWebhookExecutionLimit {
			s.hrisWebhookExecutions = s.hrisWebhookExecutions[:maxWebhookExecutionLimit]
		}
		return true, nil
	}); err != nil {
		return HRISWebhookExecution{}, err
	}
	return cloneHRISWebhookExecution(record), nil
}

func (s *Service) ListHRISWebhookExecutions(tenantID string, limit int) []HRISWebhookExecution {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := listHRISWebhookExecutions(s.hrisWebhookExecutions, tenantID)
	if limit <= 0 || limit >= len(items) {
		return items
	}
	return items[:limit]
}

func (s *Service) ListAllHRISWebhookExecutions(tenantID string) []HRISWebhookExecution {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return listHRISWebhookExecutions(s.hrisWebhookExecutions, tenantID)
}

func (s *Service) FindActiveHRISWebhookReplayExecution(
	tenantID string,
	sourceExecutionID string,
) (HRISWebhookExecution, bool) {
	nextTenantID := strings.TrimSpace(tenantID)
	nextSourceExecutionID := strings.TrimSpace(sourceExecutionID)
	if nextTenantID == "" || nextSourceExecutionID == "" {
		return HRISWebhookExecution{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := findActiveHRISWebhookReplayExecutionLocked(
		s.hrisWebhookExecutions,
		nextTenantID,
		nextSourceExecutionID,
	)
	if !ok {
		return HRISWebhookExecution{}, false
	}
	return cloneHRISWebhookExecution(item), true
}

func (s *Service) ListQueuedHRISWebhookExecutions(kind string, limit int) []HRISWebhookExecution {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.listQueuedHRISWebhookExecutionsLocked(kind, limit)
}

func (s *Service) ListClaimableHRISWebhookExecutions(
	kind string,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookExecution {
	processingTimeout, now = normalizeHRISWebhookExecutionClaimParams(processingTimeout, now)

	s.mu.RLock()
	defer s.mu.RUnlock()

	return listClaimableHRISWebhookExecutionsLocked(
		s.hrisWebhookExecutions,
		kind,
		processingTimeout,
		now,
		limit,
	)
}

func (s *Service) ListIndexedClaimableHRISWebhookExecutions(
	kind string,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookExecution {
	processingTimeout, now = normalizeHRISWebhookExecutionClaimParams(processingTimeout, now)

	s.mu.RLock()
	defer s.mu.RUnlock()

	return listIndexedClaimableHRISWebhookExecutionsLocked(
		s.hrisWebhookExecutions,
		s.listQueuedHRISWebhookExecutionsLocked(kind, limit),
		kind,
		processingTimeout,
		now,
		limit,
	)
}

func listHRISWebhookExecutions(items []HRISWebhookExecution, tenantID string) []HRISWebhookExecution {
	filterTenantID := strings.TrimSpace(tenantID)
	result := make([]HRISWebhookExecution, 0, len(items))
	for i := range items {
		item := items[i]
		if filterTenantID != "" && strings.TrimSpace(item.TenantID) != filterTenantID {
			continue
		}
		result = append(result, cloneHRISWebhookExecution(item))
	}
	return result
}

func (s *Service) listQueuedHRISWebhookExecutionsLocked(kind string, limit int) []HRISWebhookExecution {
	var ids []string
	switch normalizeHRISWebhookExecutionKind(kind) {
	case HRISWebhookExecutionKindReceiptProcess:
		ids = s.queuedReceiptExecutionIDs
	case HRISWebhookExecutionKindDLQReplay:
		ids = s.queuedDLQReplayExecutionIDs
	default:
		return nil
	}
	if len(ids) == 0 {
		return nil
	}

	items := make([]HRISWebhookExecution, 0, len(ids))
	for i := range ids {
		item, ok := findHRISWebhookExecutionByIDLocked(s.hrisWebhookExecutions, ids[i])
		if !ok || !isQueuedHRISWebhookExecutionCandidate(item) {
			continue
		}
		items = append(items, cloneHRISWebhookExecution(item))
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items
}

func findHRISWebhookExecutionByIDLocked(
	items []HRISWebhookExecution,
	executionID string,
) (HRISWebhookExecution, bool) {
	nextExecutionID := strings.TrimSpace(executionID)
	if nextExecutionID == "" {
		return HRISWebhookExecution{}, false
	}
	for i := range items {
		if strings.TrimSpace(items[i].ID) != nextExecutionID {
			continue
		}
		return items[i], true
	}
	return HRISWebhookExecution{}, false
}

func isQueuedHRISWebhookExecutionCandidate(item HRISWebhookExecution) bool {
	if !isWorkerManagedHRISWebhookExecutionCandidate(item) {
		return false
	}
	return strings.TrimSpace(item.Status) == HRISWebhookExecutionStatusQueued
}

func isWorkerManagedHRISWebhookExecutionCandidate(item HRISWebhookExecution) bool {
	switch normalizeHRISWebhookExecutionKind(item.Kind) {
	case HRISWebhookExecutionKindReceiptProcess, HRISWebhookExecutionKindDLQReplay:
	default:
		return false
	}
	if strings.TrimSpace(item.ExecutionMode) != "queued" {
		return false
	}
	if strings.TrimSpace(item.DispatchMode) != HRISWebhookExecutionDispatchModeWorkerTick {
		return false
	}
	return strings.TrimSpace(item.TargetID) != ""
}

func normalizeHRISWebhookExecutionClaimParams(
	processingTimeout time.Duration,
	now time.Time,
) (time.Duration, time.Time) {
	if processingTimeout <= 0 {
		processingTimeout = 5 * time.Minute
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	return processingTimeout, now
}

func hrisWebhookExecutionClaimReason(
	item HRISWebhookExecution,
	processingTimeout time.Duration,
	now time.Time,
) string {
	if !isWorkerManagedHRISWebhookExecutionCandidate(item) {
		return HRISWebhookExecutionClaimReasonNotQueueable
	}
	switch strings.TrimSpace(item.Status) {
	case HRISWebhookExecutionStatusQueued:
		if !item.QueuedAt.IsZero() && item.QueuedAt.UTC().After(now) {
			return HRISWebhookExecutionClaimReasonCooldown
		}
		return ""
	case HRISWebhookExecutionStatusRunning:
		if item.StartedAt != nil && item.StartedAt.Add(processingTimeout).After(now) {
			return HRISWebhookExecutionClaimReasonInFlight
		}
		return ""
	default:
		return HRISWebhookExecutionClaimReasonNotQueueable
	}
}

func hrisWebhookExecutionClaimSortTime(item HRISWebhookExecution) time.Time {
	if strings.TrimSpace(item.Status) == HRISWebhookExecutionStatusQueued && !item.QueuedAt.IsZero() {
		return item.QueuedAt.UTC()
	}
	if item.StartedAt != nil && !item.StartedAt.IsZero() {
		return item.StartedAt.UTC()
	}
	return item.QueuedAt.UTC()
}

func listClaimableHRISWebhookExecutionsLocked(
	items []HRISWebhookExecution,
	kind string,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookExecution {
	nextKind := normalizeHRISWebhookExecutionKind(kind)
	if nextKind == "" {
		return nil
	}

	filtered := make([]HRISWebhookExecution, 0, len(items))
	for i := range items {
		item := items[i]
		if normalizeHRISWebhookExecutionKind(item.Kind) != nextKind {
			continue
		}
		if hrisWebhookExecutionClaimReason(item, processingTimeout, now) != "" {
			continue
		}
		filtered = append(filtered, cloneHRISWebhookExecution(item))
	}
	sort.Slice(filtered, func(i, j int) bool {
		leftAt := hrisWebhookExecutionClaimSortTime(filtered[i])
		rightAt := hrisWebhookExecutionClaimSortTime(filtered[j])
		if leftAt.Equal(rightAt) {
			return filtered[i].ID < filtered[j].ID
		}
		return leftAt.Before(rightAt)
	})
	if limit > 0 && len(filtered) > limit {
		return filtered[:limit]
	}
	return filtered
}

func listIndexedClaimableHRISWebhookExecutionsLocked(
	allItems []HRISWebhookExecution,
	indexedQueued []HRISWebhookExecution,
	kind string,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookExecution {
	dueQueued := listDueQueuedHRISWebhookExecutionsFromIndex(indexedQueued, now, limit)
	staleRunning := listStaleRunningHRISWebhookExecutionsLocked(
		allItems,
		kind,
		processingTimeout,
		now,
		limit,
	)
	if len(dueQueued) == 0 {
		if limit > 0 && len(staleRunning) > limit {
			return staleRunning[:limit]
		}
		return staleRunning
	}
	if len(staleRunning) == 0 {
		if limit > 0 && len(dueQueued) > limit {
			return dueQueued[:limit]
		}
		return dueQueued
	}

	merged := make([]HRISWebhookExecution, 0, len(dueQueued)+len(staleRunning))
	leftIndex := 0
	rightIndex := 0
	for leftIndex < len(dueQueued) && rightIndex < len(staleRunning) {
		leftAt := hrisWebhookExecutionClaimSortTime(dueQueued[leftIndex])
		rightAt := hrisWebhookExecutionClaimSortTime(staleRunning[rightIndex])
		if leftAt.Before(rightAt) || (leftAt.Equal(rightAt) && dueQueued[leftIndex].ID < staleRunning[rightIndex].ID) {
			merged = append(merged, dueQueued[leftIndex])
			leftIndex++
		} else {
			merged = append(merged, staleRunning[rightIndex])
			rightIndex++
		}
		if limit > 0 && len(merged) >= limit {
			return merged[:limit]
		}
	}
	for leftIndex < len(dueQueued) {
		merged = append(merged, dueQueued[leftIndex])
		leftIndex++
		if limit > 0 && len(merged) >= limit {
			return merged[:limit]
		}
	}
	for rightIndex < len(staleRunning) {
		merged = append(merged, staleRunning[rightIndex])
		rightIndex++
		if limit > 0 && len(merged) >= limit {
			return merged[:limit]
		}
	}
	return merged
}

func listDueQueuedHRISWebhookExecutionsFromIndex(
	indexedQueued []HRISWebhookExecution,
	now time.Time,
	limit int,
) []HRISWebhookExecution {
	if len(indexedQueued) == 0 {
		return nil
	}
	items := make([]HRISWebhookExecution, 0, len(indexedQueued))
	for i := range indexedQueued {
		if !indexedQueued[i].QueuedAt.IsZero() && indexedQueued[i].QueuedAt.UTC().After(now) {
			break
		}
		items = append(items, cloneHRISWebhookExecution(indexedQueued[i]))
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items
}

func listStaleRunningHRISWebhookExecutionsLocked(
	items []HRISWebhookExecution,
	kind string,
	processingTimeout time.Duration,
	now time.Time,
	limit int,
) []HRISWebhookExecution {
	nextKind := normalizeHRISWebhookExecutionKind(kind)
	if nextKind == "" {
		return nil
	}

	filtered := make([]HRISWebhookExecution, 0, len(items))
	for i := range items {
		item := items[i]
		if normalizeHRISWebhookExecutionKind(item.Kind) != nextKind {
			continue
		}
		if strings.TrimSpace(item.Status) != HRISWebhookExecutionStatusRunning {
			continue
		}
		if hrisWebhookExecutionClaimReason(item, processingTimeout, now) != "" {
			continue
		}
		filtered = append(filtered, cloneHRISWebhookExecution(item))
	}
	sort.Slice(filtered, func(i, j int) bool {
		leftAt := hrisWebhookExecutionClaimSortTime(filtered[i])
		rightAt := hrisWebhookExecutionClaimSortTime(filtered[j])
		if leftAt.Equal(rightAt) {
			return filtered[i].ID < filtered[j].ID
		}
		return leftAt.Before(rightAt)
	})
	if limit > 0 && len(filtered) > limit {
		return filtered[:limit]
	}
	return filtered
}

func buildQueuedHRISWebhookExecutionIDs(
	items []HRISWebhookExecution,
	kind string,
) []string {
	nextKind := normalizeHRISWebhookExecutionKind(kind)
	filtered := make([]HRISWebhookExecution, 0, len(items))
	for i := range items {
		item := items[i]
		if normalizeHRISWebhookExecutionKind(item.Kind) != nextKind {
			continue
		}
		if !isQueuedHRISWebhookExecutionCandidate(item) {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].QueuedAt.Equal(filtered[j].QueuedAt) {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].QueuedAt.Before(filtered[j].QueuedAt)
	})

	result := make([]string, 0, len(filtered))
	for i := range filtered {
		if strings.TrimSpace(filtered[i].ID) == "" {
			continue
		}
		result = append(result, filtered[i].ID)
	}
	return result
}

func (s *Service) GetHRISWebhookExecution(tenantID, executionID string) (HRISWebhookExecution, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISWebhookExecution{}, ErrTenantIDRequired
	}
	nextExecutionID := strings.TrimSpace(executionID)
	if nextExecutionID == "" {
		return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.hrisWebhookExecutions {
		item := s.hrisWebhookExecutions[i]
		if strings.TrimSpace(item.ID) != nextExecutionID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
		}
		return cloneHRISWebhookExecution(item), nil
	}
	return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
}

func (s *Service) GetHRISWebhookExecutionByID(executionID string) (HRISWebhookExecution, error) {
	nextExecutionID := strings.TrimSpace(executionID)
	if nextExecutionID == "" {
		return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := findHRISWebhookExecutionByIDLocked(s.hrisWebhookExecutions, nextExecutionID)
	if !ok {
		return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
	}
	return cloneHRISWebhookExecution(item), nil
}

func (s *Service) UpdateHRISWebhookExecutionDispatchMode(tenantID, executionID, dispatchMode string) (HRISWebhookExecution, error) {
	nextDispatchMode := normalizeHRISWebhookExecutionDispatchMode(dispatchMode)
	if nextDispatchMode == "" {
		return HRISWebhookExecution{}, ErrInvalidHRISWebhookExecutionDispatchMode
	}
	return s.updateHRISWebhookExecution(
		tenantID,
		executionID,
		func(item *HRISWebhookExecution) {
			item.DispatchMode = nextDispatchMode
		},
	)
}

func (s *Service) MarkHRISWebhookExecutionRunning(tenantID, executionID string) (HRISWebhookExecution, error) {
	now := time.Now().UTC()
	return s.updateHRISWebhookExecution(
		tenantID,
		executionID,
		func(item *HRISWebhookExecution) {
			item.Status = HRISWebhookExecutionStatusRunning
			item.StartedAt = &now
			item.FinishedAt = nil
			item.LastError = ""
			item.AttemptCount++
		},
	)
}

func (s *Service) ClaimHRISWebhookExecution(
	tenantID string,
	executionID string,
	processingTimeout time.Duration,
	now time.Time,
) (HRISWebhookExecution, string, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISWebhookExecution{}, "", ErrTenantIDRequired
	}
	nextExecutionID := strings.TrimSpace(executionID)
	if nextExecutionID == "" {
		return HRISWebhookExecution{}, "", ErrHRISWebhookExecutionNotFound
	}
	processingTimeout, now = normalizeHRISWebhookExecutionClaimParams(processingTimeout, now)

	s.mu.Lock()
	defer s.mu.Unlock()

	var claimed HRISWebhookExecution
	claimReason := ""
	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		for i := range s.hrisWebhookExecutions {
			if strings.TrimSpace(s.hrisWebhookExecutions[i].ID) != nextExecutionID {
				continue
			}
			if strings.TrimSpace(s.hrisWebhookExecutions[i].TenantID) != nextTenantID {
				return false, ErrHRISWebhookExecutionNotFound
			}
			if reason := hrisWebhookExecutionClaimReason(s.hrisWebhookExecutions[i], processingTimeout, now); reason != "" {
				claimed = cloneHRISWebhookExecution(s.hrisWebhookExecutions[i])
				claimReason = reason
				return false, nil
			}
			s.hrisWebhookExecutions[i].Status = HRISWebhookExecutionStatusRunning
			s.hrisWebhookExecutions[i].StartedAt = &now
			s.hrisWebhookExecutions[i].FinishedAt = nil
			s.hrisWebhookExecutions[i].LastError = ""
			s.hrisWebhookExecutions[i].AttemptCount++
			s.hrisWebhookExecutions[i].UpdatedAt = now
			claimed = cloneHRISWebhookExecution(s.hrisWebhookExecutions[i])
			claimReason = ""
			return true, nil
		}
		return false, ErrHRISWebhookExecutionNotFound
	}); err != nil {
		return HRISWebhookExecution{}, "", err
	}
	if claimReason != "" {
		return claimed, claimReason, nil
	}
	if claimed.ID == "" {
		return HRISWebhookExecution{}, "", ErrHRISWebhookExecutionNotFound
	}
	return claimed, "", nil
}

func (s *Service) RequeueHRISWebhookExecution(
	tenantID string,
	executionID string,
	targetStatus string,
	retryAt time.Time,
	failure error,
) (HRISWebhookExecution, error) {
	now := time.Now().UTC()
	nextRetryAt := now
	if !retryAt.IsZero() {
		nextRetryAt = retryAt.UTC()
	}
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	nextTargetStatus := strings.TrimSpace(targetStatus)
	return s.updateHRISWebhookExecution(
		tenantID,
		executionID,
		func(item *HRISWebhookExecution) {
			item.Status = HRISWebhookExecutionStatusQueued
			item.TargetStatus = nextTargetStatus
			item.LastError = message
			item.RequeueCount++
			item.QueuedAt = nextRetryAt
			item.StartedAt = nil
			item.FinishedAt = nil
		},
	)
}

func (s *Service) AcknowledgeHRISWebhookExecution(
	tenantID string,
	executionID string,
	targetStatus string,
	failure error,
) (HRISWebhookExecution, error) {
	if failure != nil {
		return s.MarkHRISWebhookExecutionFailed(tenantID, executionID, targetStatus, failure)
	}
	return s.MarkHRISWebhookExecutionSucceeded(tenantID, executionID, targetStatus)
}

func (s *Service) MarkHRISWebhookExecutionSucceeded(tenantID, executionID, targetStatus string) (HRISWebhookExecution, error) {
	now := time.Now().UTC()
	nextTargetStatus := strings.TrimSpace(targetStatus)
	return s.updateHRISWebhookExecution(
		tenantID,
		executionID,
		func(item *HRISWebhookExecution) {
			item.Status = HRISWebhookExecutionStatusSucceeded
			item.TargetStatus = nextTargetStatus
			item.LastError = ""
			if item.StartedAt == nil {
				item.StartedAt = &now
			}
			item.FinishedAt = &now
		},
	)
}

func (s *Service) MarkHRISWebhookExecutionFailed(
	tenantID string,
	executionID string,
	targetStatus string,
	failure error,
) (HRISWebhookExecution, error) {
	now := time.Now().UTC()
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	nextTargetStatus := strings.TrimSpace(targetStatus)
	return s.updateHRISWebhookExecution(
		tenantID,
		executionID,
		func(item *HRISWebhookExecution) {
			item.Status = HRISWebhookExecutionStatusFailed
			item.TargetStatus = nextTargetStatus
			item.LastError = message
			if item.StartedAt == nil {
				item.StartedAt = &now
			}
			item.FinishedAt = &now
		},
	)
}

func (s *Service) updateHRISWebhookExecution(
	tenantID string,
	executionID string,
	mutate func(item *HRISWebhookExecution),
) (HRISWebhookExecution, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HRISWebhookExecution{}, ErrTenantIDRequired
	}
	nextExecutionID := strings.TrimSpace(executionID)
	if nextExecutionID == "" {
		return HRISWebhookExecution{}, ErrHRISWebhookExecutionNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var updated HRISWebhookExecution
	if err := s.mutateHRISWebhookStateLocked(func() (bool, error) {
		for i := range s.hrisWebhookExecutions {
			if strings.TrimSpace(s.hrisWebhookExecutions[i].ID) != nextExecutionID {
				continue
			}
			if strings.TrimSpace(s.hrisWebhookExecutions[i].TenantID) != nextTenantID {
				return false, ErrHRISWebhookExecutionNotFound
			}
			if mutate != nil {
				mutate(&s.hrisWebhookExecutions[i])
			}
			s.hrisWebhookExecutions[i].UpdatedAt = time.Now().UTC()
			updated = cloneHRISWebhookExecution(s.hrisWebhookExecutions[i])
			return true, nil
		}
		return false, ErrHRISWebhookExecutionNotFound
	}); err != nil {
		return HRISWebhookExecution{}, err
	}
	return updated, nil
}

func normalizeHRISWebhookExecutionKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case HRISWebhookExecutionKindReceiptProcess:
		return HRISWebhookExecutionKindReceiptProcess
	case HRISWebhookExecutionKindDLQReplay:
		return HRISWebhookExecutionKindDLQReplay
	default:
		return ""
	}
}

func normalizeHRISWebhookExecutionDispatchMode(dispatchMode string) string {
	switch strings.ToLower(strings.TrimSpace(dispatchMode)) {
	case HRISWebhookExecutionDispatchModeWorkerTick:
		return HRISWebhookExecutionDispatchModeWorkerTick
	case HRISWebhookExecutionDispatchModeWorkerTaskChannel:
		return HRISWebhookExecutionDispatchModeWorkerTaskChannel
	case HRISWebhookExecutionDispatchModeGoroutineFallback:
		return HRISWebhookExecutionDispatchModeGoroutineFallback
	default:
		return ""
	}
}

func findActiveHRISWebhookReplayExecutionLocked(
	items []HRISWebhookExecution,
	tenantID string,
	sourceExecutionID string,
) (HRISWebhookExecution, bool) {
	nextTenantID := strings.TrimSpace(tenantID)
	nextSourceExecutionID := strings.TrimSpace(sourceExecutionID)
	if nextTenantID == "" || nextSourceExecutionID == "" {
		return HRISWebhookExecution{}, false
	}
	for i := range items {
		item := items[i]
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		if strings.TrimSpace(item.ReplaySourceExecutionID) != nextSourceExecutionID {
			continue
		}
		switch strings.TrimSpace(item.Status) {
		case HRISWebhookExecutionStatusQueued, HRISWebhookExecutionStatusRunning:
			return item, true
		}
	}
	return HRISWebhookExecution{}, false
}

func (s *Service) ResolveTenantByEmail(email string) (TenantResolution, error) {
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return TenantResolution{}, ErrEmailRequired
	}

	domain, err := emailDomain(nextEmail)
	if err != nil {
		return TenantResolution{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	bestTenantID := ""
	bestDomain := ""
	for i := range s.domainMappings {
		if s.domainMappings[i].Status != "active" {
			continue
		}
		mappedDomain := s.domainMappings[i].Domain
		if !domainMatches(domain, mappedDomain) {
			continue
		}
		if len(mappedDomain) <= len(bestDomain) {
			continue
		}
		bestDomain = mappedDomain
		bestTenantID = s.domainMappings[i].TenantID
	}

	if bestTenantID != "" {
		return TenantResolution{
			Email:    nextEmail,
			Domain:   domain,
			TenantID: bestTenantID,
			Matched:  true,
		}, nil
	}

	return TenantResolution{}, ErrDomainNotMapped
}

func (s *Service) GetIDPConfig(tenantID string) (IDPConfig, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return IDPConfig{}, ErrTenantIDRequired
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	config, exists := s.idpConfigs[nextTenantID]
	if !exists {
		return IDPConfig{}, ErrIDPConfigNotFound
	}
	config.Scopes = append([]string(nil), config.Scopes...)
	return config, nil
}

func (s *Service) UpsertIDPConfig(
	tenantID, provider, issuerURL, clientID, authURL, tokenURL, jwksURL, userInfoURL, samlACSURL, samlX509Cert, status, syncMode, actor string,
	scopes []string,
) (IDPConfig, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return IDPConfig{}, ErrTenantIDRequired
	}

	nextProvider, err := normalizeProvider(provider)
	if err != nil {
		return IDPConfig{}, err
	}

	nextIssuerURL := strings.TrimSpace(issuerURL)
	if nextIssuerURL == "" {
		return IDPConfig{}, ErrIssuerURLRequired
	}

	nextClientID := strings.TrimSpace(clientID)
	if nextClientID == "" {
		return IDPConfig{}, ErrClientIDRequired
	}

	nextSAMLACSURL := strings.TrimSpace(samlACSURL)
	if nextSAMLACSURL == "" {
		// Keep backward compatibility: if existing clients filled auth_url for SAML ACS endpoint.
		nextSAMLACSURL = strings.TrimSpace(authURL)
	}
	nextSAMLX509Cert := strings.TrimSpace(samlX509Cert)
	if nextProvider == "saml" {
		if nextSAMLACSURL == "" {
			return IDPConfig{}, ErrSAMLACSURLRequired
		}
		if !looksLikeHTTPSURL(nextSAMLACSURL) {
			return IDPConfig{}, ErrInvalidSAMLACSURL
		}
		if nextSAMLX509Cert == "" {
			return IDPConfig{}, ErrSAMLX509CertRequired
		}
	}

	nextStatus, err := normalizeIDPStatus(status)
	if err != nil {
		return IDPConfig{}, err
	}

	nextSyncMode := normalizeSyncMode(syncMode)
	nextActor := strings.TrimSpace(actor)
	if nextActor == "" {
		nextActor = "system"
	}
	nextScopes := normalizeScopes(scopes)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	createdAt := now
	configID := ""
	if current, exists := s.idpConfigs[nextTenantID]; exists {
		createdAt = current.CreatedAt
		configID = current.ID
	}
	if configID == "" {
		configID, err = randomID("idp_")
		if err != nil {
			return IDPConfig{}, err
		}
	}

	record := IDPConfig{
		ID:           configID,
		TenantID:     nextTenantID,
		Provider:     nextProvider,
		IssuerURL:    nextIssuerURL,
		ClientID:     nextClientID,
		AuthURL:      strings.TrimSpace(authURL),
		TokenURL:     strings.TrimSpace(tokenURL),
		JWKSURL:      strings.TrimSpace(jwksURL),
		UserInfoURL:  strings.TrimSpace(userInfoURL),
		SAMLACSURL:   nextSAMLACSURL,
		SAMLX509Cert: nextSAMLX509Cert,
		Scopes:       nextScopes,
		Status:       nextStatus,
		SyncMode:     nextSyncMode,
		UpdatedBy:    nextActor,
		CreatedAt:    createdAt,
		UpdatedAt:    now,
	}

	s.idpConfigs[nextTenantID] = record
	if err := s.persistLocked(); err != nil {
		return IDPConfig{}, err
	}
	return record, nil
}

func (s *Service) ValidateIDPConfig(
	tenantID, provider, issuerURL, clientID, authURL, tokenURL, jwksURL, userInfoURL, samlACSURL, samlX509Cert string,
) IDPConfigValidation {
	nextTenantID := strings.TrimSpace(tenantID)
	nextProvider := strings.ToLower(strings.TrimSpace(provider))

	result := IDPConfigValidation{
		TenantID:  nextTenantID,
		Provider:  nextProvider,
		Valid:     true,
		Items:     make([]IDPConfigValidationItem, 0, 8),
		CheckedAt: time.Now().UTC(),
	}

	push := func(field, status, message string) {
		result.Items = append(result.Items, IDPConfigValidationItem{
			Field:   field,
			Status:  status,
			Message: message,
		})
		if status == "error" {
			result.Valid = false
		}
	}

	if nextTenantID == "" {
		push("tenant_id", "error", ErrTenantIDRequired.Error())
	} else {
		push("tenant_id", "ok", "tenant_id looks good")
	}

	if _, err := normalizeProvider(nextProvider); err != nil {
		push("provider", "error", err.Error())
	} else {
		push("provider", "ok", "provider is supported")
	}

	if strings.TrimSpace(issuerURL) == "" {
		push("issuer_url", "error", ErrIssuerURLRequired.Error())
	} else if !looksLikeHTTPSURL(issuerURL) {
		push("issuer_url", "error", "issuer_url must use https://")
	} else {
		push("issuer_url", "ok", "issuer_url format is valid")
	}

	if strings.TrimSpace(clientID) == "" {
		push("client_id", "error", ErrClientIDRequired.Error())
	} else {
		push("client_id", "ok", "client_id looks good")
	}

	if strings.TrimSpace(authURL) != "" && !looksLikeHTTPSURL(authURL) {
		push("auth_url", "error", "auth_url must use https://")
	} else if strings.TrimSpace(authURL) != "" {
		push("auth_url", "ok", "auth_url format is valid")
	}
	if strings.TrimSpace(tokenURL) != "" && !looksLikeHTTPSURL(tokenURL) {
		push("token_url", "error", "token_url must use https://")
	} else if strings.TrimSpace(tokenURL) != "" {
		push("token_url", "ok", "token_url format is valid")
	}
	if strings.TrimSpace(jwksURL) != "" && !looksLikeHTTPSURL(jwksURL) {
		push("jwks_url", "error", "jwks_url must use https://")
	} else if strings.TrimSpace(jwksURL) != "" {
		push("jwks_url", "ok", "jwks_url format is valid")
	}
	if strings.TrimSpace(userInfoURL) != "" && !looksLikeHTTPSURL(userInfoURL) {
		push("user_info_url", "error", "user_info_url must use https://")
	} else if strings.TrimSpace(userInfoURL) != "" {
		push("user_info_url", "ok", "user_info_url format is valid")
	}

	nextSAMLACSURL := strings.TrimSpace(samlACSURL)
	if nextSAMLACSURL == "" && strings.TrimSpace(authURL) != "" {
		nextSAMLACSURL = strings.TrimSpace(authURL)
		push("saml_acs_url", "warn", "fallback to auth_url for saml_acs_url")
	}

	if nextProvider == "saml" {
		if nextSAMLACSURL == "" {
			push("saml_acs_url", "error", ErrSAMLACSURLRequired.Error())
		} else if !looksLikeHTTPSURL(nextSAMLACSURL) {
			push("saml_acs_url", "error", ErrInvalidSAMLACSURL.Error())
		} else {
			push("saml_acs_url", "ok", "saml_acs_url format is valid")
		}

		if strings.TrimSpace(samlX509Cert) == "" {
			push("saml_x509_cert", "error", ErrSAMLX509CertRequired.Error())
		} else if _, err := normalizeSAMLX509Certificate(strings.TrimSpace(samlX509Cert)); err != nil {
			push("saml_x509_cert", "error", err.Error())
		} else {
			push("saml_x509_cert", "ok", "saml_x509_cert is valid")
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if hasActiveDomainForTenant(s.domainMappings, nextTenantID) {
		push("domain_mapping", "ok", "active domain mapping exists")
	} else {
		push("domain_mapping", "warn", "no active domain mapping for tenant")
	}

	return result
}

func (s *Service) ListEmployees(tenantID string) []EnterpriseEmployee {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]EnterpriseEmployee, 0, len(s.employees))
	for i := range s.employees {
		if filterTenantID != "" && s.employees[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, cloneEmployee(s.employees[i]))
	}
	return items
}

func (s *Service) GetEmployeeByEmail(tenantID, email string) (EnterpriseEmployee, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return EnterpriseEmployee{}, ErrTenantIDRequired
	}
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return EnterpriseEmployee{}, ErrEmailRequired
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	found := false
	candidate := EnterpriseEmployee{}
	for i := range s.employees {
		item := s.employees[i]
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		if normalizeEmail(item.Email) != nextEmail {
			continue
		}
		if normalizeEmployeeStatus(item.Status) != "active" {
			continue
		}
		if !found || item.LastSyncedAt.After(candidate.LastSyncedAt) {
			candidate = cloneEmployee(item)
			found = true
		}
	}
	if !found {
		return EnterpriseEmployee{}, ErrEmployeeNotFound
	}

	return candidate, nil
}

func (s *Service) HasActiveJITEmployeeIdentity(tenantID, email, externalID string) (bool, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return false, ErrTenantIDRequired
	}
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return false, ErrEmailRequired
	}
	nextExternalID := strings.TrimSpace(externalID)

	domain, err := emailDomain(nextEmail)
	if err != nil {
		return false, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantDomains := activeDomainsForTenant(s.domainMappings, nextTenantID)
	if !domainMatchesAny(domain, tenantDomains) {
		return false, ErrEmployeeEmailDomainMismatch
	}

	targetIndex := findJITEmployeeMatchIndexLocked(s.employees, nextTenantID, nextEmail, nextExternalID)
	if targetIndex < 0 {
		return false, nil
	}

	current := s.employees[targetIndex]
	if normalizeEmployeeStatus(current.Status) != "active" || EmploymentStatusBlocksSession(current.EmploymentStatus) {
		return false, ErrEmployeeInactive
	}
	currentExternalID := strings.TrimSpace(current.ExternalID)
	if nextExternalID != "" && currentExternalID != "" && currentExternalID != nextExternalID {
		return false, ErrEmployeeExternalIDConflict
	}
	return true, nil
}

func (s *Service) UpsertJITProvisionApprovalRequest(
	tenantID string,
	email string,
	externalID string,
	provider string,
	employmentStatus string,
) (JITProvisionApproval, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return JITProvisionApproval{}, ErrTenantIDRequired
	}
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return JITProvisionApproval{}, ErrEmailRequired
	}
	nextExternalID := strings.TrimSpace(externalID)
	nextProvider := strings.ToLower(strings.TrimSpace(provider))
	nextEmploymentStatus := NormalizeEmploymentStatus(employmentStatus)

	domain, err := emailDomain(nextEmail)
	if err != nil {
		return JITProvisionApproval{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tenantDomains := activeDomainsForTenant(s.domainMappings, nextTenantID)
	if !domainMatchesAny(domain, tenantDomains) {
		return JITProvisionApproval{}, ErrEmployeeEmailDomainMismatch
	}

	for i := range s.jitProvisionApprovals {
		item := s.jitProvisionApprovals[i]
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		if normalizeEmail(item.Email) != nextEmail {
			continue
		}
		if strings.TrimSpace(item.Status) != "pending" {
			continue
		}
		if nextExternalID != "" && strings.TrimSpace(item.ExternalID) != "" && strings.TrimSpace(item.ExternalID) != nextExternalID {
			continue
		}

		item.ExternalID = chooseNonEmpty(nextExternalID, strings.TrimSpace(item.ExternalID))
		item.Provider = chooseNonEmpty(nextProvider, strings.TrimSpace(item.Provider))
		item.EmploymentStatus = chooseNonEmpty(nextEmploymentStatus, strings.TrimSpace(item.EmploymentStatus))
		item.UpdatedAt = time.Now().UTC()
		s.jitProvisionApprovals[i] = item
		if err := s.persistLocked(); err != nil {
			return JITProvisionApproval{}, err
		}
		return cloneJITProvisionApproval(item), nil
	}

	approvalID, err := randomID("jap_")
	if err != nil {
		return JITProvisionApproval{}, err
	}
	now := time.Now().UTC()
	record := JITProvisionApproval{
		ID:               approvalID,
		TenantID:         nextTenantID,
		Email:            nextEmail,
		ExternalID:       nextExternalID,
		Provider:         nextProvider,
		EmploymentStatus: nextEmploymentStatus,
		Status:           "pending",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.jitProvisionApprovals = append([]JITProvisionApproval{record}, s.jitProvisionApprovals...)
	if err := s.persistLocked(); err != nil {
		return JITProvisionApproval{}, err
	}
	return cloneJITProvisionApproval(record), nil
}

func (s *Service) HasApprovedJITProvisionApproval(tenantID, email, externalID string) (bool, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return false, ErrTenantIDRequired
	}
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return false, ErrEmailRequired
	}
	nextExternalID := strings.TrimSpace(externalID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.jitProvisionApprovals {
		item := s.jitProvisionApprovals[i]
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		if normalizeEmail(item.Email) != nextEmail {
			continue
		}
		if strings.TrimSpace(item.Status) != "approved" {
			continue
		}
		itemExternalID := strings.TrimSpace(item.ExternalID)
		if nextExternalID != "" && itemExternalID != "" && itemExternalID != nextExternalID {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (s *Service) ListJITProvisionApprovals(tenantID, status string, limit int) []JITProvisionApproval {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := strings.TrimSpace(tenantID)
	filterStatus := strings.ToLower(strings.TrimSpace(status))
	items := make([]JITProvisionApproval, 0, len(s.jitProvisionApprovals))
	for i := range s.jitProvisionApprovals {
		item := s.jitProvisionApprovals[i]
		if nextTenantID != "" && strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		if filterStatus != "" && strings.ToLower(strings.TrimSpace(item.Status)) != filterStatus {
			continue
		}
		items = append(items, cloneJITProvisionApproval(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) ReviewJITProvisionApproval(
	tenantID string,
	approvalID string,
	decision string,
	reviewedBy string,
	reason string,
) (JITProvisionApproval, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return JITProvisionApproval{}, ErrTenantIDRequired
	}
	nextApprovalID := strings.TrimSpace(approvalID)
	if nextApprovalID == "" {
		return JITProvisionApproval{}, ErrJITProvisionApprovalNotFound
	}
	nextDecision, err := normalizeJITProvisionApprovalDecision(decision)
	if err != nil {
		return JITProvisionApproval{}, err
	}
	nextReviewedBy := strings.TrimSpace(reviewedBy)
	if nextReviewedBy == "" {
		nextReviewedBy = "system"
	}
	nextReason := strings.TrimSpace(reason)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jitProvisionApprovals {
		item := s.jitProvisionApprovals[i]
		if strings.TrimSpace(item.ID) != nextApprovalID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return JITProvisionApproval{}, ErrJITProvisionApprovalNotFound
		}
		now := time.Now().UTC()
		item.Status = nextDecision
		item.ReviewedBy = nextReviewedBy
		item.Reason = chooseNonEmpty(nextReason, item.Reason)
		item.ExternalSyncStatus = "pending"
		item.ExternalSyncLastError = ""
		item.ExternalSyncRef = ""
		item.ExternalSyncUpdatedAt = nil
		item.UpdatedAt = now
		item.ReviewedAt = &now
		s.jitProvisionApprovals[i] = item
		if err := s.persistLocked(); err != nil {
			return JITProvisionApproval{}, err
		}
		return cloneJITProvisionApproval(item), nil
	}
	return JITProvisionApproval{}, ErrJITProvisionApprovalNotFound
}

func (s *Service) ListPendingJITProvisionApprovalExternalSync(tenantID string, limit int) []JITProvisionApproval {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := strings.TrimSpace(tenantID)
	items := make([]JITProvisionApproval, 0, len(s.jitProvisionApprovals))
	for i := range s.jitProvisionApprovals {
		item := s.jitProvisionApprovals[i]
		if nextTenantID != "" && strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		status := strings.TrimSpace(item.Status)
		if status != "approved" && status != "rejected" {
			continue
		}
		if strings.TrimSpace(item.ExternalSyncStatus) != "pending" {
			continue
		}
		items = append(items, cloneJITProvisionApproval(item))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) UpdateJITProvisionApprovalExternalSync(
	tenantID string,
	approvalID string,
	status string,
	externalSyncRef string,
	lastError string,
) (JITProvisionApproval, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return JITProvisionApproval{}, ErrTenantIDRequired
	}
	nextApprovalID := strings.TrimSpace(approvalID)
	if nextApprovalID == "" {
		return JITProvisionApproval{}, ErrJITProvisionApprovalNotFound
	}
	nextStatus, err := normalizeJITProvisionApprovalExternalSyncStatus(status)
	if err != nil {
		return JITProvisionApproval{}, err
	}
	nextRef := strings.TrimSpace(externalSyncRef)
	nextLastError := strings.TrimSpace(lastError)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jitProvisionApprovals {
		item := s.jitProvisionApprovals[i]
		if strings.TrimSpace(item.ID) != nextApprovalID {
			continue
		}
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			return JITProvisionApproval{}, ErrJITProvisionApprovalNotFound
		}

		now := time.Now().UTC()
		item.ExternalSyncStatus = nextStatus
		item.ExternalSyncRef = nextRef
		item.ExternalSyncUpdatedAt = &now
		item.UpdatedAt = now
		switch nextStatus {
		case "synced":
			item.ExternalSyncLastError = ""
		case "failed":
			item.ExternalSyncAttemptCount++
			item.ExternalSyncLastError = nextLastError
		}

		s.jitProvisionApprovals[i] = item
		if err := s.persistLocked(); err != nil {
			return JITProvisionApproval{}, err
		}
		return cloneJITProvisionApproval(item), nil
	}
	return JITProvisionApproval{}, ErrJITProvisionApprovalNotFound
}

func (s *Service) ResolveOrProvisionJITEmployee(
	tenantID string,
	email string,
	externalID string,
	fullName string,
	department string,
	jobTitle string,
	location string,
) (EnterpriseEmployee, bool, error) {
	return s.ResolveOrProvisionJITEmployeeWithProfile(
		tenantID,
		email,
		externalID,
		JITProvisionProfile{
			FullName:   fullName,
			Department: department,
			JobTitle:   jobTitle,
			Location:   location,
		},
	)
}

func (s *Service) ResolveOrProvisionJITEmployeeWithProfile(
	tenantID string,
	email string,
	externalID string,
	profile JITProvisionProfile,
) (EnterpriseEmployee, bool, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return EnterpriseEmployee{}, false, ErrTenantIDRequired
	}
	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return EnterpriseEmployee{}, false, ErrEmailRequired
	}
	nextExternalID := strings.TrimSpace(externalID)
	nextFullName := strings.TrimSpace(profile.FullName)
	nextDepartment := strings.TrimSpace(profile.Department)
	nextJobTitle := strings.TrimSpace(profile.JobTitle)
	nextLocation := strings.TrimSpace(profile.Location)
	nextPhone := normalizeEmployeePhone(profile.Phone)
	nextManagerExternalID := strings.TrimSpace(profile.ManagerExternalID)
	nextEmploymentStatus := NormalizeEmploymentStatus(profile.EmploymentStatus)
	if nextEmploymentStatus == "" {
		nextEmploymentStatus = "active"
	}
	if EmploymentStatusBlocksSession(nextEmploymentStatus) {
		return EnterpriseEmployee{}, false, ErrEmployeeInactive
	}

	domain, err := emailDomain(nextEmail)
	if err != nil {
		return EnterpriseEmployee{}, false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tenantDomains := activeDomainsForTenant(s.domainMappings, nextTenantID)
	if !domainMatchesAny(domain, tenantDomains) {
		return EnterpriseEmployee{}, false, ErrEmployeeEmailDomainMismatch
	}

	targetIndex := findJITEmployeeMatchIndexLocked(s.employees, nextTenantID, nextEmail, nextExternalID)
	if targetIndex >= 0 {
		current := s.employees[targetIndex]
		if normalizeEmployeeStatus(current.Status) != "active" || EmploymentStatusBlocksSession(current.EmploymentStatus) {
			return EnterpriseEmployee{}, false, ErrEmployeeInactive
		}

		currentExternalID := strings.TrimSpace(current.ExternalID)
		if nextExternalID != "" && currentExternalID != "" && currentExternalID != nextExternalID {
			return EnterpriseEmployee{}, false, ErrEmployeeExternalIDConflict
		}
		if nextExternalID != "" {
			current.ExternalID = nextExternalID
		}
		current.Email = nextEmail

		preferDirectorySnapshot := hasDirectorySnapshotPrioritySource(current.Source)
		if nextFullName != "" {
			if !preferDirectorySnapshot || strings.TrimSpace(current.FullName) == "" {
				current.FullName = nextFullName
			}
		} else if strings.TrimSpace(current.FullName) == "" {
			current.FullName = nextEmail
		}

		profileChanged := false
		if nextDepartment != "" && (!preferDirectorySnapshot || strings.TrimSpace(current.Department) == "") {
			if current.Department != nextDepartment {
				current.Department = nextDepartment
				profileChanged = true
			}
		}
		if nextJobTitle != "" && (!preferDirectorySnapshot || strings.TrimSpace(current.JobTitle) == "") {
			if current.JobTitle != nextJobTitle {
				current.JobTitle = nextJobTitle
				profileChanged = true
			}
		}
		if nextLocation != "" && (!preferDirectorySnapshot || strings.TrimSpace(current.Location) == "") {
			if current.Location != nextLocation {
				current.Location = nextLocation
				profileChanged = true
			}
		}
		if nextPhone != "" && (!preferDirectorySnapshot || strings.TrimSpace(current.Phone) == "") {
			current.Phone = nextPhone
		}
		if nextManagerExternalID != "" && (!preferDirectorySnapshot || strings.TrimSpace(current.ManagerExternalID) == "") {
			current.ManagerExternalID = nextManagerExternalID
		}
		if nextEmploymentStatus != "" && (!preferDirectorySnapshot || strings.TrimSpace(current.EmploymentStatus) == "") {
			current.EmploymentStatus = nextEmploymentStatus
		} else if strings.TrimSpace(current.EmploymentStatus) == "" {
			current.EmploymentStatus = "active"
		}
		if strings.TrimSpace(current.AccessRole) == "" || profileChanged {
			role, buildingID, groupIDs := assignAccessTemplate(
				nextTenantID,
				current.Department,
				current.JobTitle,
				current.Location,
			)
			current.AccessRole = role
			current.BuildingID = buildingID
			current.GroupIDs = append([]string(nil), groupIDs...)
		}
		if strings.TrimSpace(current.Source) == "" || strings.TrimSpace(current.Source) == "jit_provision" {
			current.Source = "jit_provision"
		}
		current.Status = normalizeEmployeeStatus(current.EmploymentStatus)
		if normalizeEmployeeStatus(current.Status) != "active" {
			return EnterpriseEmployee{}, false, ErrEmployeeInactive
		}
		current.LastSyncedAt = time.Now().UTC()
		s.employees[targetIndex] = current
		if err := s.persistLocked(); err != nil {
			return EnterpriseEmployee{}, false, err
		}
		return cloneEmployee(current), false, nil
	}

	role, buildingID, groupIDs := assignAccessTemplate(nextTenantID, nextDepartment, nextJobTitle, nextLocation)
	employeeID, err := randomID("emp_")
	if err != nil {
		return EnterpriseEmployee{}, false, err
	}
	nextExternalForCreate := nextExternalID
	if nextExternalForCreate == "" {
		nextExternalForCreate = "jit_email:" + nextEmail
	}
	nextFullNameForCreate := nextFullName
	if nextFullNameForCreate == "" {
		nextFullNameForCreate = nextEmail
	}
	now := time.Now().UTC()
	record := EnterpriseEmployee{
		ID:                employeeID,
		TenantID:          nextTenantID,
		ExternalID:        nextExternalForCreate,
		Email:             nextEmail,
		FullName:          nextFullNameForCreate,
		Department:        nextDepartment,
		JobTitle:          nextJobTitle,
		Location:          nextLocation,
		Phone:             nextPhone,
		ManagerExternalID: nextManagerExternalID,
		EmploymentStatus:  nextEmploymentStatus,
		AccessRole:        role,
		BuildingID:        buildingID,
		GroupIDs:          append([]string(nil), groupIDs...),
		Status:            normalizeEmployeeStatus(nextEmploymentStatus),
		Source:            "jit_provision",
		LastSyncedAt:      now,
	}
	s.employees = append([]EnterpriseEmployee{record}, s.employees...)
	if err := s.persistLocked(); err != nil {
		return EnterpriseEmployee{}, false, err
	}
	return cloneEmployee(record), true, nil
}

func (s *Service) StartAuthStateToken(
	tenantID string,
	provider string,
	email string,
	redirectURI string,
	ttl time.Duration,
) (AuthStateToken, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return AuthStateToken{}, ErrTenantIDRequired
	}
	nextProvider, err := normalizeProvider(provider)
	if err != nil {
		return AuthStateToken{}, err
	}
	nextRedirectURI := strings.TrimSpace(redirectURI)
	if nextRedirectURI == "" {
		return AuthStateToken{}, ErrRedirectURIRequired
	}
	if !looksLikeAllowedRedirectURI(nextRedirectURI) {
		return AuthStateToken{}, ErrInvalidRedirectURI
	}
	nextEmail := normalizeEmail(email)
	if ttl <= 0 {
		ttl = defaultAuthStateTokenTTL
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredAuthStateTokensLocked(time.Now().UTC())

	if s.authStateTokens == nil {
		s.authStateTokens = make(map[string]AuthStateToken)
	}

	tokenValue, err := randomID("st_")
	if err != nil {
		return AuthStateToken{}, err
	}
	now := time.Now().UTC()
	record := AuthStateToken{
		Token:       tokenValue,
		TenantID:    nextTenantID,
		Provider:    nextProvider,
		Email:       nextEmail,
		RedirectURI: nextRedirectURI,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
	}
	s.authStateTokens[tokenValue] = record
	return record, nil
}

func (s *Service) ConsumeAuthStateToken(token string, expectedProvider string) (AuthStateToken, error) {
	nextToken := strings.TrimSpace(token)
	if nextToken == "" {
		return AuthStateToken{}, ErrAuthStateTokenRequired
	}
	nextExpectedProvider := strings.ToLower(strings.TrimSpace(expectedProvider))
	if nextExpectedProvider != "" {
		if _, err := normalizeProvider(nextExpectedProvider); err != nil {
			return AuthStateToken{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.cleanupExpiredAuthStateTokensLocked(now)

	record, exists := s.authStateTokens[nextToken]
	if !exists {
		return AuthStateToken{}, ErrAuthStateTokenNotFound
	}
	if nextExpectedProvider != "" && record.Provider != nextExpectedProvider {
		delete(s.authStateTokens, nextToken)
		return AuthStateToken{}, ErrAuthStateProviderMismatch
	}
	if !record.ExpiresAt.After(now) {
		delete(s.authStateTokens, nextToken)
		return AuthStateToken{}, ErrAuthStateTokenNotFound
	}

	delete(s.authStateTokens, nextToken)
	return record, nil
}

func (s *Service) SyncEmployees(tenantID, source, actor string, inputs []EmployeeSyncInput) (SyncResult, error) {
	nextTenantID, nextSource, nextActor, _, err := normalizeSyncEmployeesRequest(tenantID, source, actor, "", inputs)
	if err != nil {
		return SyncResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.syncEmployeesLocked(nextTenantID, nextSource, nextActor, inputs)
	if err != nil {
		return SyncResult{}, err
	}
	if err := s.persistLocked(); err != nil {
		return SyncResult{}, err
	}
	return result, nil
}

func (s *Service) SyncEmployeesWithAccessUpsert(
	tenantID, source, actor string,
	requestID string,
	inputs []EmployeeSyncInput,
	applier AccessSyncApplier,
) (SyncResult, int, int, int, error) {
	return s.SyncEmployeesWithAccessUpsertMetadata(
		tenantID,
		source,
		actor,
		requestID,
		"",
		"",
		inputs,
		applier,
	)
}

func (s *Service) SyncEmployeesWithAccessUpsertMetadata(
	tenantID, source, actor string,
	requestID string,
	connectorID string,
	rawPayloadRef string,
	inputs []EmployeeSyncInput,
	applier AccessSyncApplier,
) (SyncResult, int, int, int, error) {
	nextTenantID, nextSource, nextActor, nextRequestID, err := normalizeSyncEmployeesRequest(tenantID, source, actor, requestID, inputs)
	if err != nil {
		return SyncResult{}, 0, 0, 0, err
	}
	nextConnectorID := strings.TrimSpace(connectorID)
	nextRawPayloadRef := strings.TrimSpace(rawPayloadRef)
	if applier == nil {
		return SyncResult{}, 0, 0, 0, ErrAccessSyncApplierRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	recordKey := syncRequestRecordKey(nextTenantID, nextRequestID)
	if recordKey != "" {
		if record, exists := s.syncRequestRecords[recordKey]; exists {
			return s.applyAccessForSyncRecordLocked(recordKey, nextRequestID, record, applier)
		}
	}

	previousEmployees := cloneEmployees(s.employees)
	previousSyncJobs := cloneSyncJobs(s.syncJobs)
	previousSyncRequestRecords := cloneSyncRequestRecords(s.syncRequestRecords)

	result, err := s.syncEmployeesLocked(nextTenantID, nextSource, nextActor, inputs)
	if err != nil {
		return SyncResult{}, 0, 0, 0, err
	}
	if recordKey != "" {
		if s.syncRequestRecords == nil {
			s.syncRequestRecords = make(map[string]SyncRequestRecord)
		}
		s.syncRequestRecords[recordKey] = SyncRequestRecord{
			RequestID:     nextRequestID,
			TenantID:      nextTenantID,
			ConnectorID:   nextConnectorID,
			RawPayloadRef: nextRawPayloadRef,
			Result:        cloneSyncResult(result),
			AccessApplied: false,
			CreatedAt:     time.Now().UTC(),
		}
	}
	if err := s.persistLocked(); err != nil {
		return SyncResult{}, 0, 0, 0, err
	}

	if recordKey != "" {
		record := s.syncRequestRecords[recordKey]
		reconciledResult, created, updated, rejected, applyErr := s.applyAccessForSyncRecordLocked(recordKey, nextRequestID, record, applier)
		if applyErr == nil {
			return reconciledResult, created, updated, rejected, nil
		}
		s.employees = previousEmployees
		s.syncJobs = previousSyncJobs
		s.syncRequestRecords = previousSyncRequestRecords
		if rollbackErr := s.persistLocked(); rollbackErr != nil {
			return SyncResult{}, 0, 0, 0, fmt.Errorf(
				"access sync failed: %v; enterprise rollback failed: %w",
				applyErr,
				rollbackErr,
			)
		}
		return SyncResult{}, 0, 0, 0, fmt.Errorf("access sync failed, enterprise sync rolled back: %w", applyErr)
	}

	created, updated, rejected, applyErr := applier(result.Items)
	if applyErr == nil {
		return cloneSyncResult(result), created, updated, rejected, nil
	}

	s.employees = previousEmployees
	s.syncJobs = previousSyncJobs
	s.syncRequestRecords = previousSyncRequestRecords
	if rollbackErr := s.persistLocked(); rollbackErr != nil {
		return SyncResult{}, 0, 0, 0, fmt.Errorf(
			"access sync failed: %v; enterprise rollback failed: %w",
			applyErr,
			rollbackErr,
		)
	}
	return SyncResult{}, 0, 0, 0, fmt.Errorf("access sync failed, enterprise sync rolled back: %w", applyErr)
}

func (s *Service) ReconcileSyncRequestAccess(
	tenantID string,
	requestID string,
	applier AccessSyncApplier,
) (SyncResult, int, int, int, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return SyncResult{}, 0, 0, 0, ErrTenantIDRequired
	}
	nextRequestID := strings.TrimSpace(requestID)
	if nextRequestID == "" {
		return SyncResult{}, 0, 0, 0, ErrSyncRequestIDRequired
	}
	if applier == nil {
		return SyncResult{}, 0, 0, 0, ErrAccessSyncApplierRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	recordKey := syncRequestRecordKey(nextTenantID, nextRequestID)
	record, exists := s.syncRequestRecords[recordKey]
	if !exists {
		return SyncResult{}, 0, 0, 0, ErrSyncRequestNotFound
	}
	return s.applyAccessForSyncRecordLocked(recordKey, nextRequestID, record, applier)
}

func (s *Service) ListSyncRequestRecords(tenantID string, limit int) []SyncRequestRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]SyncRequestRecord, 0, len(s.syncRequestRecords))
	for _, record := range s.syncRequestRecords {
		if filterTenantID != "" && strings.TrimSpace(record.TenantID) != filterTenantID {
			continue
		}
		items = append(items, cloneSyncRequestRecord(record))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].RequestID > items[j].RequestID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func (s *Service) GetSyncRequestRecord(tenantID string, requestID string) (SyncRequestRecord, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return SyncRequestRecord{}, ErrTenantIDRequired
	}
	nextRequestID := strings.TrimSpace(requestID)
	if nextRequestID == "" {
		return SyncRequestRecord{}, ErrSyncRequestIDRequired
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	recordKey := syncRequestRecordKey(nextTenantID, nextRequestID)
	record, exists := s.syncRequestRecords[recordKey]
	if !exists {
		return SyncRequestRecord{}, ErrSyncRequestNotFound
	}
	return cloneSyncRequestRecord(record), nil
}

func (s *Service) ReconcilePendingSyncRequests(
	tenantID string,
	limit int,
	applier AccessSyncApplier,
) (BatchPendingSyncReconcileResult, error) {
	return s.ReconcilePendingSyncRequestsWithPolicy(tenantID, limit, 0, 0, applier)
}

func (s *Service) ReconcilePendingSyncRequestsWithPolicy(
	tenantID string,
	limit int,
	maxAttempts int,
	retryCooldown time.Duration,
	applier AccessSyncApplier,
) (BatchPendingSyncReconcileResult, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return BatchPendingSyncReconcileResult{}, ErrTenantIDRequired
	}
	nextLimit, err := normalizeReconcileLimit(limit)
	if err != nil {
		return BatchPendingSyncReconcileResult{}, err
	}
	if applier == nil {
		return BatchPendingSyncReconcileResult{}, ErrAccessSyncApplierRequired
	}
	if maxAttempts < 0 {
		maxAttempts = 0
	}
	if retryCooldown < 0 {
		retryCooldown = 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	type pendingRecord struct {
		key    string
		record SyncRequestRecord
	}
	now := time.Now().UTC()
	result := BatchPendingSyncReconcileResult{
		Items: make([]PendingSyncReconcileResult, 0, len(s.syncRequestRecords)),
	}
	pendingRecords := make([]pendingRecord, 0, len(s.syncRequestRecords))
	for key, record := range s.syncRequestRecords {
		if strings.TrimSpace(record.TenantID) != nextTenantID {
			continue
		}
		if record.AccessApplied {
			continue
		}
		if maxAttempts > 0 && record.AccessAttemptCount >= maxAttempts {
			result.SkippedByAttemptLimit++
			continue
		}
		if retryCooldown > 0 && record.LastAccessAttemptAt != nil {
			retryReadyAt := record.LastAccessAttemptAt.Add(retryCooldown)
			if retryReadyAt.After(now) {
				result.SkippedByCooldown++
				continue
			}
		}
		pendingRecords = append(pendingRecords, pendingRecord{
			key:    key,
			record: record,
		})
	}

	sort.Slice(pendingRecords, func(i, j int) bool {
		if pendingRecords[i].record.CreatedAt.Equal(pendingRecords[j].record.CreatedAt) {
			return pendingRecords[i].record.RequestID < pendingRecords[j].record.RequestID
		}
		return pendingRecords[i].record.CreatedAt.Before(pendingRecords[j].record.CreatedAt)
	})
	if len(pendingRecords) > nextLimit {
		pendingRecords = pendingRecords[:nextLimit]
	}
	result.Items = make([]PendingSyncReconcileResult, 0, len(pendingRecords))
	for i := range pendingRecords {
		pending := pendingRecords[i]
		_, _, _, _, applyErr := s.applyAccessForSyncRecordLocked(
			pending.key,
			pending.record.RequestID,
			pending.record,
			applier,
		)

		latestRecord := s.syncRequestRecords[pending.key]
		item := PendingSyncReconcileResult{
			RequestID:      latestRecord.RequestID,
			JobID:          latestRecord.Result.Job.ID,
			AccessApplied:  latestRecord.AccessApplied,
			AccessCreated:  latestRecord.AccessCreated,
			AccessUpdated:  latestRecord.AccessUpdated,
			AccessRejected: latestRecord.AccessRejected,
			AttemptCount:   latestRecord.AccessAttemptCount,
			LastError:      latestRecord.LastAccessError,
		}
		if latestRecord.LastAccessAttemptAt != nil {
			attemptedAt := *latestRecord.LastAccessAttemptAt
			item.AttemptedAt = &attemptedAt
		}
		result.Items = append(result.Items, item)
		result.Processed++

		if applyErr != nil {
			result.Failed++
			continue
		}
		result.Applied++
	}

	return result, nil
}

func (s *Service) applyAccessForSyncRecordLocked(
	recordKey string,
	requestID string,
	record SyncRequestRecord,
	applier AccessSyncApplier,
) (SyncResult, int, int, int, error) {
	if record.AccessApplied {
		return cloneSyncResult(record.Result), record.AccessCreated, record.AccessUpdated, record.AccessRejected, nil
	}

	attemptAt := time.Now().UTC()
	record.AccessAttemptCount++
	record.LastAccessAttemptAt = &attemptAt

	created, updated, rejected, applyErr := applier(record.Result.Items)
	if applyErr != nil {
		record.LastAccessError = strings.TrimSpace(applyErr.Error())
		s.syncRequestRecords[recordKey] = record
		// Best-effort persistence for compensation audit trail.
		_ = s.persistLocked()
		return SyncResult{}, 0, 0, 0, fmt.Errorf("access sync retry failed for request_id %s: %w", requestID, applyErr)
	}
	record.AccessApplied = true
	record.AccessCreated = created
	record.AccessUpdated = updated
	record.AccessRejected = rejected
	record.LastAccessError = ""
	s.syncRequestRecords[recordKey] = record
	// Idempotency cache persistence should not fail the successful sync path.
	_ = s.persistLocked()
	return cloneSyncResult(record.Result), created, updated, rejected, nil
}

func (s *Service) syncEmployeesLocked(
	nextTenantID, nextSource, nextActor string,
	inputs []EmployeeSyncInput,
) (SyncResult, error) {
	startedAt := time.Now().UTC()
	jobID, err := randomID("syn_")
	if err != nil {
		return SyncResult{}, err
	}

	tenantDomains := activeDomainsForTenant(s.domainMappings, nextTenantID)
	existingByExternalID := make(map[string]int)
	existingByEmail := make(map[string]int)
	for i := range s.employees {
		if s.employees[i].TenantID != nextTenantID {
			continue
		}
		externalID := strings.TrimSpace(s.employees[i].ExternalID)
		if externalID != "" {
			existingByExternalID[externalID] = i
		}
		existingByEmail[normalizeEmail(s.employees[i].Email)] = i
	}
	createdRecords := make([]EnterpriseEmployee, 0, len(inputs))
	createdByExternalID := make(map[string]int)
	createdByEmail := make(map[string]int)

	resultItems := make([]EnterpriseEmployee, 0, len(inputs))
	created := 0
	updated := 0
	deactivated := 0
	rejected := 0

	for i := range inputs {
		externalID := strings.TrimSpace(inputs[i].ExternalID)
		email := normalizeEmail(inputs[i].Email)
		if email == "" {
			rejected++
			continue
		}

		domain, err := emailDomain(email)
		if err != nil {
			rejected++
			continue
		}
		if !domainMatchesAny(domain, tenantDomains) {
			rejected++
			continue
		}

		employmentStatus := NormalizeEmploymentStatus(inputs[i].EmploymentStatus)
		if employmentStatus == "" {
			employmentStatus = NormalizeEmploymentStatus(inputs[i].Status)
		}
		if employmentStatus == "" {
			employmentStatus = "active"
		}
		status := normalizeEmployeeStatus(employmentStatus)
		phone := normalizeEmployeePhone(inputs[i].Phone)
		managerExternalID := strings.TrimSpace(inputs[i].ManagerExternalID)
		employeeNumber := strings.TrimSpace(inputs[i].EmployeeNumber)
		joinDate := strings.TrimSpace(inputs[i].JoinDate)
		resignDate := strings.TrimSpace(inputs[i].ResignDate)
		shiftCode := strings.TrimSpace(inputs[i].ShiftCode)
		scheduleWindow := strings.TrimSpace(inputs[i].ScheduleWindow)
		leaveStatus := strings.TrimSpace(inputs[i].LeaveStatus)
		costCenter := strings.TrimSpace(inputs[i].CostCenter)
		photoURL := strings.TrimSpace(inputs[i].PhotoURL)
		role, buildingID, groupIDs := assignAccessTemplate(nextTenantID, inputs[i].Department, inputs[i].JobTitle, inputs[i].Location)
		now := time.Now().UTC()

		existingExternalIndex, hasExistingExternalMatch := -1, false
		if externalID != "" {
			existingExternalIndex, hasExistingExternalMatch = existingByExternalID[externalID]
		}
		createdExternalIndex, hasCreatedExternalMatch := -1, false
		if externalID != "" {
			createdExternalIndex, hasCreatedExternalMatch = createdByExternalID[externalID]
		}
		existingEmailIndex, hasExistingEmailMatch := existingByEmail[email]
		createdEmailIndex, hasCreatedEmailMatch := createdByEmail[email]

		hasExternalMatch := hasExistingExternalMatch || hasCreatedExternalMatch
		hasEmailMatch := hasExistingEmailMatch || hasCreatedEmailMatch

		targetIsCreated := false
		targetIndex := -1
		switch {
		case hasExternalMatch && hasEmailMatch:
			switch {
			case hasExistingExternalMatch && hasExistingEmailMatch && existingExternalIndex == existingEmailIndex:
				targetIndex = existingExternalIndex
			case hasCreatedExternalMatch && hasCreatedEmailMatch && createdExternalIndex == createdEmailIndex:
				targetIndex = createdExternalIndex
				targetIsCreated = true
			default:
				rejected++
				continue
			}
		case hasExternalMatch:
			if hasExistingExternalMatch {
				targetIndex = existingExternalIndex
			} else {
				targetIndex = createdExternalIndex
				targetIsCreated = true
			}
		case hasEmailMatch:
			if hasExistingEmailMatch {
				targetIndex = existingEmailIndex
			} else {
				targetIndex = createdEmailIndex
				targetIsCreated = true
			}
		}

		if targetIsCreated {
			currentExternalID := strings.TrimSpace(createdRecords[targetIndex].ExternalID)
			if externalID != "" && currentExternalID != "" && currentExternalID != externalID {
				rejected++
				continue
			}

			previousEmail := normalizeEmail(createdRecords[targetIndex].Email)
			previousExternalID := strings.TrimSpace(createdRecords[targetIndex].ExternalID)
			createdRecords[targetIndex].ExternalID = externalID
			createdRecords[targetIndex].EmployeeNumber = employeeNumber
			createdRecords[targetIndex].Email = email
			createdRecords[targetIndex].FullName = strings.TrimSpace(inputs[i].FullName)
			createdRecords[targetIndex].Department = strings.TrimSpace(inputs[i].Department)
			createdRecords[targetIndex].JobTitle = strings.TrimSpace(inputs[i].JobTitle)
			createdRecords[targetIndex].Location = strings.TrimSpace(inputs[i].Location)
			createdRecords[targetIndex].Phone = phone
			createdRecords[targetIndex].ManagerExternalID = managerExternalID
			createdRecords[targetIndex].EmploymentStatus = employmentStatus
			createdRecords[targetIndex].JoinDate = joinDate
			createdRecords[targetIndex].ResignDate = resignDate
			createdRecords[targetIndex].ShiftCode = shiftCode
			createdRecords[targetIndex].ScheduleWindow = scheduleWindow
			createdRecords[targetIndex].LeaveStatus = leaveStatus
			createdRecords[targetIndex].CostCenter = costCenter
			createdRecords[targetIndex].PhotoURL = photoURL
			createdRecords[targetIndex].AccessRole = role
			createdRecords[targetIndex].BuildingID = buildingID
			createdRecords[targetIndex].GroupIDs = append([]string(nil), groupIDs...)
			createdRecords[targetIndex].Status = status
			createdRecords[targetIndex].Source = nextSource
			createdRecords[targetIndex].LastSyncedAt = now

			if previousEmail != email {
				if previousIndex, exists := createdByEmail[previousEmail]; exists && previousIndex == targetIndex {
					delete(createdByEmail, previousEmail)
				}
			}
			createdByEmail[email] = targetIndex
			if previousExternalID != "" && previousExternalID != externalID {
				if previousIndex, exists := createdByExternalID[previousExternalID]; exists && previousIndex == targetIndex {
					delete(createdByExternalID, previousExternalID)
				}
			}
			if externalID != "" {
				createdByExternalID[externalID] = targetIndex
			}
			continue
		}

		if targetIndex >= 0 {
			currentExternalID := strings.TrimSpace(s.employees[targetIndex].ExternalID)
			if externalID != "" && currentExternalID != "" && currentExternalID != externalID {
				rejected++
				continue
			}

			previousEmail := normalizeEmail(s.employees[targetIndex].Email)
			previousExternalID := strings.TrimSpace(s.employees[targetIndex].ExternalID)
			s.employees[targetIndex].ExternalID = externalID
			s.employees[targetIndex].EmployeeNumber = employeeNumber
			s.employees[targetIndex].Email = email
			s.employees[targetIndex].FullName = strings.TrimSpace(inputs[i].FullName)
			s.employees[targetIndex].Department = strings.TrimSpace(inputs[i].Department)
			s.employees[targetIndex].JobTitle = strings.TrimSpace(inputs[i].JobTitle)
			s.employees[targetIndex].Location = strings.TrimSpace(inputs[i].Location)
			s.employees[targetIndex].Phone = phone
			s.employees[targetIndex].ManagerExternalID = managerExternalID
			s.employees[targetIndex].EmploymentStatus = employmentStatus
			s.employees[targetIndex].JoinDate = joinDate
			s.employees[targetIndex].ResignDate = resignDate
			s.employees[targetIndex].ShiftCode = shiftCode
			s.employees[targetIndex].ScheduleWindow = scheduleWindow
			s.employees[targetIndex].LeaveStatus = leaveStatus
			s.employees[targetIndex].CostCenter = costCenter
			s.employees[targetIndex].PhotoURL = photoURL
			s.employees[targetIndex].AccessRole = role
			s.employees[targetIndex].BuildingID = buildingID
			s.employees[targetIndex].GroupIDs = append([]string(nil), groupIDs...)
			s.employees[targetIndex].Status = status
			s.employees[targetIndex].Source = nextSource
			s.employees[targetIndex].LastSyncedAt = now

			if previousEmail != email {
				if previousIndex, exists := existingByEmail[previousEmail]; exists && previousIndex == targetIndex {
					delete(existingByEmail, previousEmail)
				}
			}
			existingByEmail[email] = targetIndex
			if previousExternalID != "" && previousExternalID != externalID {
				if previousIndex, exists := existingByExternalID[previousExternalID]; exists && previousIndex == targetIndex {
					delete(existingByExternalID, previousExternalID)
				}
			}
			if externalID != "" {
				existingByExternalID[externalID] = targetIndex
			}

			updated++
			if status != "active" {
				deactivated++
			}
			resultItems = append(resultItems, cloneEmployee(s.employees[targetIndex]))
			continue
		}

		employeeID, err := randomID("emp_")
		if err != nil {
			return SyncResult{}, err
		}

		record := EnterpriseEmployee{
			ID:                employeeID,
			TenantID:          nextTenantID,
			ExternalID:        externalID,
			EmployeeNumber:    employeeNumber,
			Email:             email,
			FullName:          strings.TrimSpace(inputs[i].FullName),
			Department:        strings.TrimSpace(inputs[i].Department),
			JobTitle:          strings.TrimSpace(inputs[i].JobTitle),
			Location:          strings.TrimSpace(inputs[i].Location),
			Phone:             phone,
			ManagerExternalID: managerExternalID,
			EmploymentStatus:  employmentStatus,
			JoinDate:          joinDate,
			ResignDate:        resignDate,
			ShiftCode:         shiftCode,
			ScheduleWindow:    scheduleWindow,
			LeaveStatus:       leaveStatus,
			CostCenter:        costCenter,
			PhotoURL:          photoURL,
			AccessRole:        role,
			BuildingID:        buildingID,
			GroupIDs:          append([]string(nil), groupIDs...),
			Status:            status,
			Source:            nextSource,
			LastSyncedAt:      now,
		}
		createdIndex := len(createdRecords)
		createdByEmail[email] = createdIndex
		if externalID != "" {
			createdByExternalID[externalID] = createdIndex
		}
		createdRecords = append(createdRecords, record)
		created++
		resultItems = append(resultItems, cloneEmployee(record))
	}
	if len(createdRecords) > 0 {
		s.employees = append(createdRecords, s.employees...)
	}

	job := SyncJob{
		ID:          jobID,
		TenantID:    nextTenantID,
		Source:      nextSource,
		Status:      "completed",
		Total:       len(inputs),
		Created:     created,
		Updated:     updated,
		Deactivated: deactivated,
		Rejected:    rejected,
		Actor:       nextActor,
		StartedAt:   startedAt,
		EndedAt:     time.Now().UTC(),
	}
	s.syncJobs = append([]SyncJob{job}, s.syncJobs...)

	return SyncResult{
		Job:   job,
		Items: resultItems,
	}, nil
}

func normalizeSyncEmployeesRequest(
	tenantID, source, actor, requestID string,
	inputs []EmployeeSyncInput,
) (nextTenantID, nextSource, nextActor, nextRequestID string, err error) {
	nextTenantID = strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return "", "", "", "", ErrTenantIDRequired
	}
	if len(inputs) == 0 {
		return "", "", "", "", ErrEmployeesRequired
	}

	nextSource, err = normalizeSyncSource(source)
	if err != nil {
		return "", "", "", "", err
	}

	nextActor = strings.TrimSpace(actor)
	if nextActor == "" {
		nextActor = "system"
	}

	nextRequestID = strings.TrimSpace(requestID)
	return nextTenantID, nextSource, nextActor, nextRequestID, nil
}

func normalizeReconcileLimit(input int) (int, error) {
	if input < 0 {
		return 0, ErrInvalidReconcileLimit
	}
	if input == 0 {
		return defaultReconcileLimit, nil
	}
	if input > maxReconcileLimit {
		return maxReconcileLimit, nil
	}
	return input, nil
}

func (s *Service) ListSyncJobs(tenantID string) []SyncJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]SyncJob, 0, len(s.syncJobs))
	for i := range s.syncJobs {
		if filterTenantID != "" && s.syncJobs[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.syncJobs[i])
	}
	return items
}

func (s *Service) GetSyncWorkerAlertSubscription(tenantID string) (SyncWorkerAlertSubscription, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.refreshSyncWorkerAlertStateLocked(); err != nil {
		return SyncWorkerAlertSubscription{}, false
	}

	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return SyncWorkerAlertSubscription{}, false
	}
	for i := range s.syncWorkerAlertSubscriptions {
		if s.syncWorkerAlertSubscriptions[i].TenantID != nextTenantID {
			continue
		}
		return cloneSyncWorkerAlertSubscription(s.syncWorkerAlertSubscriptions[i]), true
	}
	return SyncWorkerAlertSubscription{}, false
}

func (s *Service) UpsertSyncWorkerAlertSubscription(
	input SyncWorkerAlertSubscriptionUpsertOptions,
) (SyncWorkerAlertSubscription, error) {
	resolved, err := resolveSyncWorkerAlertSubscriptionUpsertOptions(input)
	if err != nil {
		return SyncWorkerAlertSubscription{}, err
	}

	record := SyncWorkerAlertSubscription{
		TenantID:             resolved.TenantID,
		Enabled:              resolved.Enabled,
		WorkerAlertThreshold: resolved.WorkerAlertThreshold,
		WindowSeconds:        int64(resolved.Window.Seconds()),
		CooldownSeconds:      int64(resolved.Cooldown.Seconds()),
		Channels: SyncWorkerAlertSubscriptionChannels{
			Email:    resolved.EmailEnabled,
			WhatsApp: resolved.WhatsAppEnabled,
		},
		ReceiverGroups: normalizeSyncWorkerAlertSubscriptionReceiverGroups(resolved.ReceiverGroups),
		UpdatedAt:      time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.mutateSyncWorkerAlertStateLocked(func() error {
		upserted := false
		for i := range s.syncWorkerAlertSubscriptions {
			if s.syncWorkerAlertSubscriptions[i].TenantID != resolved.TenantID {
				continue
			}
			s.syncWorkerAlertSubscriptions[i] = cloneSyncWorkerAlertSubscription(record)
			upserted = true
			break
		}
		if !upserted {
			s.syncWorkerAlertSubscriptions = append(
				[]SyncWorkerAlertSubscription{cloneSyncWorkerAlertSubscription(record)},
				s.syncWorkerAlertSubscriptions...,
			)
		}
		return nil
	}); err != nil {
		return SyncWorkerAlertSubscription{}, err
	}
	return cloneSyncWorkerAlertSubscription(record), nil
}

func (s *Service) restoreFromStateStore() error {
	if s.stateStore == nil {
		return nil
	}

	var snapshot stateSnapshot
	found, err := s.stateStore.Load(stateKey, &snapshot)
	if err != nil {
		return err
	}
	if !found {
		s.mu.Lock()
		initialSnapshot := s.coreStateSnapshotLocked()
		s.mu.Unlock()
		return s.stateStore.Save(stateKey, initialSnapshot)
	}

	var alertSnapshot syncWorkerAlertStateSnapshot
	alertFound, err := s.stateStore.Load(syncWorkerAlertStateKey, &alertSnapshot)
	if err != nil {
		return err
	}

	var hrisSnapshot hrisWebhookStateSnapshot
	hrisFound, err := s.stateStore.Load(hrisWebhookStateKey, &hrisSnapshot)
	if err != nil {
		return err
	}
	if !hrisFound {
		hrisSnapshot = hrisWebhookStateSnapshotFromLegacyStateSnapshot(snapshot)
		if hasHRISWebhookStateSnapshot(hrisSnapshot) {
			if err := s.stateStore.Save(hrisWebhookStateKey, hrisSnapshot); err != nil {
				return err
			}
			if err := s.stateStore.Save(stateKey, coreStateSnapshotFromSnapshot(snapshot)); err != nil {
				return err
			}
		}
	}
	if !alertFound {
		alertSnapshot = syncWorkerAlertStateSnapshotFromLegacyStateSnapshot(snapshot)
		if hasSyncWorkerAlertStateSnapshot(alertSnapshot) {
			if err := s.stateStore.Save(syncWorkerAlertStateKey, alertSnapshot); err != nil {
				return err
			}
			if err := s.stateStore.Save(stateKey, coreStateSnapshotFromSnapshot(snapshot)); err != nil {
				return err
			}
		}
	}

	s.mu.Lock()
	s.restoreCoreStateLocked(snapshot)
	s.restoreHRISWebhookStateLocked(hrisSnapshot)
	s.restoreSyncWorkerAlertStateLocked(alertSnapshot)
	if s.authStateTokens == nil {
		s.authStateTokens = make(map[string]AuthStateToken)
	}
	s.mu.Unlock()

	return nil
}

func (s *Service) RefreshCoreState() error {
	if s == nil || s.stateStore == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshCoreStateLocked()
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, s.coreStateSnapshotLocked())
}

func (s *Service) loadCoreStateLocked() (stateSnapshot, bool, error) {
	if s.stateStore == nil {
		return stateSnapshot{}, false, nil
	}

	var snapshot stateSnapshot
	found, err := s.stateStore.Load(stateKey, &snapshot)
	if err != nil {
		return stateSnapshot{}, false, err
	}
	if !found {
		return stateSnapshot{}, false, nil
	}
	return snapshot, true, nil
}

func (s *Service) refreshCoreStateLocked() error {
	snapshot, found, err := s.loadCoreStateLocked()
	if err != nil {
		return err
	}
	if found {
		s.restoreCoreStateLocked(snapshot)
	}

	hrisSnapshot, hrisFound, err := s.loadHRISWebhookStateLocked()
	if err != nil {
		return err
	}
	if hrisFound {
		s.restoreHRISWebhookStateLocked(hrisSnapshot)
	} else {
		s.restoreHRISWebhookStateLocked(hrisWebhookStateSnapshot{})
	}
	if s.authStateTokens == nil {
		s.authStateTokens = make(map[string]AuthStateToken)
	}
	return nil
}

func (s *Service) loadHRISWebhookStateLocked() (hrisWebhookStateSnapshot, bool, error) {
	if s.stateStore == nil {
		return hrisWebhookStateSnapshot{}, false, nil
	}

	var snapshot hrisWebhookStateSnapshot
	found, err := s.stateStore.Load(hrisWebhookStateKey, &snapshot)
	if err != nil {
		return hrisWebhookStateSnapshot{}, false, err
	}
	if !found {
		return hrisWebhookStateSnapshot{}, false, nil
	}
	return snapshot, true, nil
}

func (s *Service) persistHRISWebhookStateLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(hrisWebhookStateKey, s.hrisWebhookStateSnapshotLocked())
}

func (s *Service) mutateHRISWebhookStateLocked(mutator func() (bool, error)) error {
	if s.stateStore == nil {
		changed, err := mutator()
		if err != nil {
			return err
		}
		if changed {
			s.normalizeHRISWebhookReceiptDueIndexLocked()
			s.syncQueuedHRISWebhookExecutionIndicesLocked()
		}
		return nil
	}

	casStore, hasCAS := s.stateStore.(compareAndSwapStateStore)
	if !hasCAS {
		if err := s.refreshCoreStateLocked(); err != nil {
			return err
		}
		changed, err := mutator()
		if err != nil || !changed {
			return err
		}
		s.normalizeHRISWebhookReceiptDueIndexLocked()
		s.syncQueuedHRISWebhookExecutionIndicesLocked()
		return s.persistHRISWebhookStateLocked()
	}

	baseSnapshot := s.hrisWebhookStateSnapshotLocked()
	for attempt := 0; attempt < maxEnterpriseHRISWebhookCASRetries; attempt++ {
		snapshot, found, err := s.loadHRISWebhookStateLocked()
		if err != nil {
			return err
		}
		if found {
			s.restoreHRISWebhookStateLocked(snapshot)
		} else {
			s.restoreHRISWebhookStateLocked(baseSnapshot)
		}

		changed, err := mutator()
		if err != nil {
			if found {
				s.restoreHRISWebhookStateLocked(snapshot)
			} else {
				s.restoreHRISWebhookStateLocked(baseSnapshot)
			}
			return err
		}
		if !changed {
			if found {
				s.restoreHRISWebhookStateLocked(snapshot)
			} else {
				s.restoreHRISWebhookStateLocked(baseSnapshot)
			}
			return err
		}
		s.normalizeHRISWebhookReceiptDueIndexLocked()
		s.syncQueuedHRISWebhookExecutionIndicesLocked()

		persisted, err := casStore.CompareAndSwap(
			hrisWebhookStateKey,
			found,
			snapshot,
			s.hrisWebhookStateSnapshotLocked(),
		)
		if err != nil {
			if found {
				s.restoreHRISWebhookStateLocked(snapshot)
			} else {
				s.restoreHRISWebhookStateLocked(baseSnapshot)
			}
			return err
		}
		if persisted {
			return nil
		}
	}
	if snapshot, found, err := s.loadHRISWebhookStateLocked(); err == nil {
		if found {
			s.restoreHRISWebhookStateLocked(snapshot)
		} else {
			s.restoreHRISWebhookStateLocked(baseSnapshot)
		}
	} else {
		s.restoreHRISWebhookStateLocked(baseSnapshot)
	}
	return ErrEnterpriseHRISWebhookStateConflict
}

func (s *Service) persistSyncWorkerAlertStateLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(syncWorkerAlertStateKey, s.syncWorkerAlertStateSnapshotLocked())
}

func (s *Service) loadSyncWorkerAlertStateLocked() (syncWorkerAlertStateSnapshot, bool, error) {
	if s.stateStore == nil {
		return syncWorkerAlertStateSnapshot{}, false, nil
	}

	var snapshot syncWorkerAlertStateSnapshot
	found, err := s.stateStore.Load(syncWorkerAlertStateKey, &snapshot)
	if err != nil {
		return syncWorkerAlertStateSnapshot{}, false, err
	}
	if !found {
		return syncWorkerAlertStateSnapshot{}, false, nil
	}
	return snapshot, true, nil
}

func (s *Service) refreshSyncWorkerAlertStateLocked() error {
	if s.stateStore == nil {
		return nil
	}

	casStore, hasCAS := s.stateStore.(compareAndSwapStateStore)
	now := time.Now().UTC()
	for attempt := 0; attempt < maxSyncWorkerAlertCASRetries; attempt++ {
		snapshot, found, err := s.loadSyncWorkerAlertStateLocked()
		if err != nil {
			return err
		}
		if !found {
			s.restoreSyncWorkerAlertStateLocked(syncWorkerAlertStateSnapshot{})
			return nil
		}

		s.restoreSyncWorkerAlertStateLocked(snapshot)
		recoveredNotifications, recoveredCooldowns := s.recoverExpiredSyncWorkerAlertInFlightsLocked(now)
		flightCount := len(s.syncWorkerAlertInFlights)
		s.pruneExpiredSyncWorkerAlertInFlightsLocked(now)
		if len(recoveredNotifications) == 0 && len(recoveredCooldowns) == 0 && len(s.syncWorkerAlertInFlights) == flightCount {
			return nil
		}

		cleaned := s.syncWorkerAlertStateSnapshotLocked()
		if !hasCAS {
			return s.stateStore.Save(syncWorkerAlertStateKey, cleaned)
		}

		persisted, err := casStore.CompareAndSwap(
			syncWorkerAlertStateKey,
			true,
			snapshot,
			cleaned,
		)
		if err != nil {
			return err
		}
		if persisted {
			return nil
		}
	}
	return ErrSyncWorkerAlertStateConflict
}

func (s *Service) mutateSyncWorkerAlertStateLocked(mutator func() error) error {
	if s.stateStore == nil {
		return mutator()
	}

	casStore, hasCAS := s.stateStore.(compareAndSwapStateStore)
	if !hasCAS {
		if err := s.refreshSyncWorkerAlertStateLocked(); err != nil {
			return err
		}
		if err := mutator(); err != nil {
			return err
		}
		return s.persistSyncWorkerAlertStateLocked()
	}

	for attempt := 0; attempt < maxSyncWorkerAlertCASRetries; attempt++ {
		snapshot, found, err := s.loadSyncWorkerAlertStateLocked()
		if err != nil {
			return err
		}
		if found {
			s.restoreSyncWorkerAlertStateLocked(snapshot)
		} else {
			s.restoreSyncWorkerAlertStateLocked(syncWorkerAlertStateSnapshot{})
		}
		if err := mutator(); err != nil {
			return err
		}
		persisted, err := casStore.CompareAndSwap(
			syncWorkerAlertStateKey,
			found,
			snapshot,
			s.syncWorkerAlertStateSnapshotLocked(),
		)
		if err != nil {
			return err
		}
		if persisted {
			return nil
		}
	}
	return ErrSyncWorkerAlertStateConflict
}

func (s *Service) appendSyncWorkerAlertStateDeltaLocked(
	notifications []SyncWorkerAlertNotification,
	cooldowns []SyncWorkerAlertCooldown,
) error {
	if len(notifications) == 0 && len(cooldowns) == 0 {
		return nil
	}
	return s.mutateSyncWorkerAlertStateLocked(func() error {
		s.prependSyncWorkerAlertNotificationsLocked(notifications)
		s.applySyncWorkerAlertCooldownUpdatesLocked(cooldowns)
		return nil
	})
}

func (s *Service) prependSyncWorkerAlertNotificationsLocked(items []SyncWorkerAlertNotification) {
	if len(items) == 0 {
		return
	}
	cloned := cloneSyncWorkerAlertNotifications(items)
	s.syncWorkerAlertNotifications = append(cloned, s.syncWorkerAlertNotifications...)
	if len(s.syncWorkerAlertNotifications) > maxSyncWorkerAlertNotificationLimit {
		s.syncWorkerAlertNotifications = s.syncWorkerAlertNotifications[:maxSyncWorkerAlertNotificationLimit]
	}
}

func (s *Service) applySyncWorkerAlertCooldownUpdatesLocked(items []SyncWorkerAlertCooldown) {
	for i := range items {
		next := items[i]
		tenantID := strings.TrimSpace(next.TenantID)
		fingerprint := strings.TrimSpace(next.Fingerprint)
		if tenantID == "" || fingerprint == "" {
			continue
		}
		s.upsertSyncWorkerAlertCooldownLocked(tenantID, fingerprint, next.LastSentAt)
	}
}

func (s *Service) coreStateSnapshotLocked() stateSnapshot {
	return stateSnapshot{
		DomainMappings:        cloneDomainMappings(s.domainMappings),
		HRISConnectors:        cloneHRISConnectors(s.hrisConnectors),
		IDPConfigs:            cloneIDPConfigs(s.idpConfigs),
		Employees:             cloneEmployees(s.employees),
		SyncJobs:              cloneSyncJobs(s.syncJobs),
		SyncRequestRecords:    cloneSyncRequestRecords(s.syncRequestRecords),
		JITProvisionApprovals: cloneJITProvisionApprovals(s.jitProvisionApprovals),
	}
}

func (s *Service) hrisWebhookStateSnapshotLocked() hrisWebhookStateSnapshot {
	return hrisWebhookStateSnapshot{
		HRISWebhookReceipts:         cloneHRISWebhookReceipts(s.hrisWebhookReceipts),
		HRISWebhookExecutions:       cloneHRISWebhookExecutions(s.hrisWebhookExecutions),
		DueReceiptIDs:               cloneHRISWebhookReceiptDueIndexEntries(s.dueReceiptIDs),
		QueuedReceiptExecutionIDs:   append([]string(nil), s.queuedReceiptExecutionIDs...),
		QueuedDLQReplayExecutionIDs: append([]string(nil), s.queuedDLQReplayExecutionIDs...),
	}
}

func (s *Service) syncWorkerAlertStateSnapshotLocked() syncWorkerAlertStateSnapshot {
	return syncWorkerAlertStateSnapshot{
		SyncWorkerAlertSubscriptions: cloneSyncWorkerAlertSubscriptions(s.syncWorkerAlertSubscriptions),
		SyncWorkerAlertNotifications: cloneSyncWorkerAlertNotifications(s.syncWorkerAlertNotifications),
		SyncWorkerAlertCooldowns:     cloneSyncWorkerAlertCooldowns(s.syncWorkerAlertCooldowns),
		SyncWorkerAlertInFlights:     cloneSyncWorkerAlertInFlights(s.syncWorkerAlertInFlights),
	}
}

func (s *Service) restoreSyncWorkerAlertStateLocked(snapshot syncWorkerAlertStateSnapshot) {
	s.syncWorkerAlertSubscriptions = cloneSyncWorkerAlertSubscriptions(snapshot.SyncWorkerAlertSubscriptions)
	s.syncWorkerAlertNotifications = cloneSyncWorkerAlertNotifications(snapshot.SyncWorkerAlertNotifications)
	s.syncWorkerAlertCooldowns = cloneSyncWorkerAlertCooldowns(snapshot.SyncWorkerAlertCooldowns)
	s.syncWorkerAlertInFlights = cloneSyncWorkerAlertInFlights(snapshot.SyncWorkerAlertInFlights)
}

func (s *Service) restoreCoreStateLocked(snapshot stateSnapshot) {
	s.domainMappings = cloneDomainMappings(snapshot.DomainMappings)
	s.hrisConnectors = cloneHRISConnectors(snapshot.HRISConnectors)
	s.idpConfigs = cloneIDPConfigs(snapshot.IDPConfigs)
	s.employees = cloneEmployees(snapshot.Employees)
	s.syncJobs = cloneSyncJobs(snapshot.SyncJobs)
	s.syncRequestRecords = cloneSyncRequestRecords(snapshot.SyncRequestRecords)
	s.jitProvisionApprovals = cloneJITProvisionApprovals(snapshot.JITProvisionApprovals)
}

func (s *Service) restoreHRISWebhookStateLocked(snapshot hrisWebhookStateSnapshot) {
	s.hrisWebhookReceipts = cloneHRISWebhookReceipts(snapshot.HRISWebhookReceipts)
	s.hrisWebhookExecutions = cloneHRISWebhookExecutions(snapshot.HRISWebhookExecutions)
	s.dueReceiptIDs = cloneHRISWebhookReceiptDueIndexEntries(snapshot.DueReceiptIDs)
	s.queuedReceiptExecutionIDs = append([]string(nil), snapshot.QueuedReceiptExecutionIDs...)
	s.queuedDLQReplayExecutionIDs = append([]string(nil), snapshot.QueuedDLQReplayExecutionIDs...)
	s.normalizeHRISWebhookReceiptDueIndexLocked()
	s.syncQueuedHRISWebhookExecutionIndicesLocked()
}

func coreStateSnapshotFromSnapshot(snapshot stateSnapshot) stateSnapshot {
	return stateSnapshot{
		DomainMappings:        cloneDomainMappings(snapshot.DomainMappings),
		HRISConnectors:        cloneHRISConnectors(snapshot.HRISConnectors),
		IDPConfigs:            cloneIDPConfigs(snapshot.IDPConfigs),
		Employees:             cloneEmployees(snapshot.Employees),
		SyncJobs:              cloneSyncJobs(snapshot.SyncJobs),
		SyncRequestRecords:    cloneSyncRequestRecords(snapshot.SyncRequestRecords),
		JITProvisionApprovals: cloneJITProvisionApprovals(snapshot.JITProvisionApprovals),
	}
}

func hrisWebhookStateSnapshotFromLegacyStateSnapshot(snapshot stateSnapshot) hrisWebhookStateSnapshot {
	state := hrisWebhookStateSnapshot{
		HRISWebhookReceipts:   cloneHRISWebhookReceipts(snapshot.HRISWebhookReceipts),
		HRISWebhookExecutions: cloneHRISWebhookExecutions(snapshot.HRISWebhookExecutions),
	}
	state.DueReceiptIDs = buildHRISWebhookReceiptDueIndexEntries(state.HRISWebhookReceipts)
	state.QueuedReceiptExecutionIDs = buildQueuedHRISWebhookExecutionIDs(
		state.HRISWebhookExecutions,
		HRISWebhookExecutionKindReceiptProcess,
	)
	state.QueuedDLQReplayExecutionIDs = buildQueuedHRISWebhookExecutionIDs(
		state.HRISWebhookExecutions,
		HRISWebhookExecutionKindDLQReplay,
	)
	return state
}

func syncWorkerAlertStateSnapshotFromLegacyStateSnapshot(snapshot stateSnapshot) syncWorkerAlertStateSnapshot {
	return syncWorkerAlertStateSnapshot{
		SyncWorkerAlertSubscriptions: cloneSyncWorkerAlertSubscriptions(snapshot.SyncWorkerAlertSubscriptions),
		SyncWorkerAlertNotifications: cloneSyncWorkerAlertNotifications(snapshot.SyncWorkerAlertNotifications),
		SyncWorkerAlertCooldowns:     cloneSyncWorkerAlertCooldowns(snapshot.SyncWorkerAlertCooldowns),
	}
}

func hasHRISWebhookStateSnapshot(snapshot hrisWebhookStateSnapshot) bool {
	return len(snapshot.HRISWebhookReceipts) > 0 ||
		len(snapshot.HRISWebhookExecutions) > 0 ||
		len(snapshot.DueReceiptIDs) > 0 ||
		len(snapshot.QueuedReceiptExecutionIDs) > 0 ||
		len(snapshot.QueuedDLQReplayExecutionIDs) > 0
}

func (s *Service) syncQueuedHRISWebhookExecutionIndicesLocked() {
	s.queuedReceiptExecutionIDs = buildQueuedHRISWebhookExecutionIDs(
		s.hrisWebhookExecutions,
		HRISWebhookExecutionKindReceiptProcess,
	)
	s.queuedDLQReplayExecutionIDs = buildQueuedHRISWebhookExecutionIDs(
		s.hrisWebhookExecutions,
		HRISWebhookExecutionKindDLQReplay,
	)
}

func hasSyncWorkerAlertStateSnapshot(snapshot syncWorkerAlertStateSnapshot) bool {
	return len(snapshot.SyncWorkerAlertSubscriptions) > 0 ||
		len(snapshot.SyncWorkerAlertNotifications) > 0 ||
		len(snapshot.SyncWorkerAlertCooldowns) > 0 ||
		len(snapshot.SyncWorkerAlertInFlights) > 0
}

func (s *Service) cleanupExpiredAuthStateTokensLocked(now time.Time) {
	if len(s.authStateTokens) == 0 {
		return
	}
	for token, item := range s.authStateTokens {
		if !item.ExpiresAt.After(now) {
			delete(s.authStateTokens, token)
		}
	}
}

func hasActiveDomainForTenant(items []DomainMapping, tenantID string) bool {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return false
	}
	for i := range items {
		if items[i].TenantID == nextTenantID && items[i].Status == "active" {
			return true
		}
	}
	return false
}

func activeDomainsForTenant(items []DomainMapping, tenantID string) map[string]struct{} {
	nextTenantID := strings.TrimSpace(tenantID)
	output := map[string]struct{}{}
	if nextTenantID == "" {
		return output
	}
	for i := range items {
		if items[i].TenantID != nextTenantID || items[i].Status != "active" {
			continue
		}
		output[items[i].Domain] = struct{}{}
	}
	return output
}

func domainMatchesAny(domain string, patterns map[string]struct{}) bool {
	for pattern := range patterns {
		if domainMatches(domain, pattern) {
			return true
		}
	}
	return false
}

func assignAccessTemplate(tenantID, department, jobTitle, location string) (string, string, []string) {
	dept := strings.ToLower(strings.TrimSpace(department))
	title := strings.ToLower(strings.TrimSpace(jobTitle))
	loc := strings.ToLower(strings.TrimSpace(location))
	tags := strings.Join([]string{dept, title}, " ")

	role := "resident"
	switch {
	case containsAny(tags, "security", "satpam", "guard", "hse", "safety"):
		role = "operator"
	case containsAny(tags, "facility", "engineering", "building", "maintenance", "teknisi"):
		role = "building_admin"
	case containsAny(tags, "it", "identity", "admin", "hris", "iam"):
		role = "tenant_admin"
	}

	buildingID := "building_demo_001"
	groupIDs := templateGroupIDs(strings.TrimSpace(tenantID), role)
	if strings.TrimSpace(tenantID) == "tenant_demo_factory" {
		buildingID = "building_demo_003"
	}

	if strings.Contains(loc, "factory") || strings.Contains(loc, "pabrik") || strings.Contains(loc, "bandung") || strings.Contains(loc, "bekasi") {
		buildingID = "building_demo_003"
	}
	if strings.Contains(loc, "jakarta") || strings.Contains(loc, "jkt") || strings.Contains(loc, "sudirman") {
		buildingID = "building_demo_001"
	}

	return role, buildingID, groupIDs
}

func templateGroupIDs(tenantID, role string) []string {
	nextTenantID := strings.TrimSpace(tenantID)
	nextRole := strings.TrimSpace(role)
	switch nextTenantID {
	case "tenant_demo_factory":
		switch nextRole {
		case "operator":
			return []string{"ug_security_fct"}
		case "building_admin":
			return []string{"ug_building_ops_fct"}
		case "tenant_admin":
			return []string{"ug_tenant_admin_fct"}
		default:
			return []string{"ug_common_office_fct"}
		}
	default:
		switch nextRole {
		case "operator":
			return []string{"ug_security_jkt"}
		case "building_admin":
			return []string{"ug_building_ops_jkt"}
		case "tenant_admin":
			return []string{"ug_tenant_admin_jkt"}
		default:
			return []string{"ug_common_office_jkt"}
		}
	}
}

func containsAny(input string, terms ...string) bool {
	for i := range terms {
		if strings.Contains(input, strings.ToLower(strings.TrimSpace(terms[i]))) {
			return true
		}
	}
	return false
}

func findJITEmployeeMatchIndexLocked(items []EnterpriseEmployee, tenantID, email, externalID string) int {
	nextTenantID := strings.TrimSpace(tenantID)
	nextEmail := normalizeEmail(email)
	nextExternalID := strings.TrimSpace(externalID)

	externalMatchIndex := -1
	emailMatchIndex := -1
	for i := range items {
		item := items[i]
		if strings.TrimSpace(item.TenantID) != nextTenantID {
			continue
		}
		itemExternalID := strings.TrimSpace(item.ExternalID)
		itemEmail := normalizeEmail(item.Email)
		if nextExternalID != "" && itemExternalID == nextExternalID {
			externalMatchIndex = i
			break
		}
		if emailMatchIndex < 0 && itemEmail == nextEmail {
			emailMatchIndex = i
		}
	}
	if externalMatchIndex >= 0 {
		return externalMatchIndex
	}
	return emailMatchIndex
}

func hasDirectorySnapshotPrioritySource(source string) bool {
	next := strings.ToLower(strings.TrimSpace(source))
	return strings.Contains(next, "scim") || strings.Contains(next, "hris")
}

func normalizeDomain(input string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(input))
	domain = strings.TrimPrefix(domain, "@")
	if domain == "" {
		return "", ErrDomainRequired
	}
	if strings.Contains(domain, "://") || strings.Contains(domain, "/") || strings.Contains(domain, " ") {
		return "", ErrInvalidDomain
	}
	if strings.Count(domain, ".") < 1 {
		return "", ErrInvalidDomain
	}
	return domain, nil
}

func domainMatches(actualDomain, mappedDomain string) bool {
	nextActual := strings.ToLower(strings.TrimSpace(actualDomain))
	nextMapped := strings.ToLower(strings.TrimSpace(mappedDomain))
	if nextActual == "" || nextMapped == "" {
		return false
	}
	if nextActual == nextMapped {
		return true
	}
	return strings.HasSuffix(nextActual, "."+nextMapped)
}

func normalizeDomainMappingStatus(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "", "active":
		return "active", nil
	case "inactive":
		return "inactive", nil
	default:
		return "", ErrInvalidDomainMappingStatus
	}
}

func normalizeHRISConnectorVendor(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "talenta":
		return "talenta", nil
	case "gadjian":
		return "gadjian", nil
	case "greatday":
		return "greatday", nil
	case "linovhr":
		return "linovhr", nil
	case "sunfish":
		return "sunfish", nil
	default:
		return "", ErrInvalidHRISConnectorVendor
	}
}

func normalizeHRISConnectorStatus(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "", "active":
		return "active", nil
	case "inactive":
		return "inactive", nil
	default:
		return "", ErrInvalidHRISConnectorStatus
	}
}

func normalizeHRISConnectorSyncStrategy(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "", "hybrid":
		return "hybrid", nil
	case "webhook":
		return "webhook", nil
	case "pull":
		return "pull", nil
	default:
		return "", ErrInvalidHRISConnectorSyncStrategy
	}
}

func normalizeProvider(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "oidc":
		return "oidc", nil
	case "saml":
		return "saml", nil
	default:
		return "", ErrInvalidIDPProvider
	}
}

func normalizeIDPStatus(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "", "active":
		return "active", nil
	case "inactive":
		return "inactive", nil
	default:
		return "", ErrInvalidIDPStatus
	}
}

func NormalizeEmploymentStatus(input string) string {
	next := strings.ToLower(strings.TrimSpace(input))
	switch next {
	case "true", "1", "yes", "y":
		return "active"
	case "false", "0", "no", "n":
		return "inactive"
	case "inactive", "terminated", "disabled", "suspended", "deprovisioned":
		return next
	case "active":
		return "active"
	default:
		return next
	}
}

func EmploymentStatusBlocksSession(status string) bool {
	switch NormalizeEmploymentStatus(status) {
	case "inactive", "terminated", "disabled", "suspended", "deprovisioned":
		return true
	default:
		return false
	}
}

var allowedSyncSources = map[string]struct{}{
	"csv_import":    {},
	"hris":          {},
	"hris_import":   {},
	"hris_talenta":  {},
	"hris_gadjian":  {},
	"hris_greatday": {},
	"hris_linovhr":  {},
	"hris_sunfish":  {},
	"jit_provision": {},
	"manual":        {},
	"manual_sync":   {},
	"scim":          {},
	"scim_sync":     {},
	"seed":          {},
}

func normalizeSyncSource(input string) (string, error) {
	next := strings.ToLower(strings.TrimSpace(input))
	if next == "" {
		return "manual_sync", nil
	}
	if _, exists := allowedSyncSources[next]; !exists {
		return "", ErrInvalidSyncSource
	}
	return next, nil
}

func normalizeSyncMode(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "scheduled":
		return "scheduled"
	case "manual":
		return "manual"
	default:
		return "jit"
	}
}

func normalizeJITProvisionApprovalDecision(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "approved":
		return "approved", nil
	case "rejected":
		return "rejected", nil
	default:
		return "", ErrInvalidJITProvisionApprovalDecision
	}
}

func normalizeJITProvisionApprovalExternalSyncStatus(input string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "synced":
		return "synced", nil
	case "failed":
		return "failed", nil
	default:
		return "", ErrInvalidJITProvisionApprovalExternalSyncStatus
	}
}

func resolveSyncWorkerAlertSubscriptionUpsertOptions(
	input SyncWorkerAlertSubscriptionUpsertOptions,
) (SyncWorkerAlertSubscriptionUpsertOptions, error) {
	nextTenantID := strings.TrimSpace(input.TenantID)
	if nextTenantID == "" {
		return SyncWorkerAlertSubscriptionUpsertOptions{}, ErrTenantIDRequired
	}

	next := SyncWorkerAlertSubscriptionUpsertOptions{
		TenantID:        nextTenantID,
		Enabled:         input.Enabled,
		EmailEnabled:    input.EmailEnabled,
		WhatsAppEnabled: input.WhatsAppEnabled,
		ReceiverGroups:  normalizeSyncWorkerAlertSubscriptionReceiverGroups(input.ReceiverGroups),
	}

	nextThreshold := input.WorkerAlertThreshold
	if nextThreshold < 1 || nextThreshold > 100000 {
		return SyncWorkerAlertSubscriptionUpsertOptions{}, ErrInvalidSyncWorkerAlertSubscriptionOptions
	}
	next.WorkerAlertThreshold = nextThreshold

	nextWindow := input.Window
	if nextWindow < time.Second || nextWindow > 7*24*time.Hour {
		return SyncWorkerAlertSubscriptionUpsertOptions{}, ErrInvalidSyncWorkerAlertSubscriptionOptions
	}
	next.Window = nextWindow

	nextCooldown := input.Cooldown
	if nextCooldown < 0 || nextCooldown > 7*24*time.Hour {
		return SyncWorkerAlertSubscriptionUpsertOptions{}, ErrInvalidSyncWorkerAlertSubscriptionOptions
	}
	next.Cooldown = nextCooldown

	if len(next.ReceiverGroups) > 20 {
		return SyncWorkerAlertSubscriptionUpsertOptions{}, ErrInvalidSyncWorkerAlertSubscriptionOptions
	}
	if len(next.ReceiverGroups) == 0 {
		next.ReceiverGroups = []string{"security"}
	}
	if next.Enabled && !next.EmailEnabled && !next.WhatsAppEnabled {
		return SyncWorkerAlertSubscriptionUpsertOptions{}, ErrInvalidSyncWorkerAlertSubscriptionOptions
	}
	return next, nil
}

func normalizeScopes(input []string) []string {
	items := make([]string, 0, len(input))
	set := make(map[string]struct{}, len(input))
	for i := range input {
		next := strings.ToLower(strings.TrimSpace(input[i]))
		if next == "" {
			continue
		}
		if _, exists := set[next]; exists {
			continue
		}
		set[next] = struct{}{}
		items = append(items, next)
	}
	return items
}

func normalizeEmployeeStatus(input string) string {
	if EmploymentStatusBlocksSession(input) {
		return "inactive"
	}
	return "active"
}

func normalizeEmployeePhone(input string) string {
	next := strings.TrimSpace(input)
	if next == "" {
		return ""
	}

	builder := strings.Builder{}
	for i := 0; i < len(next); i++ {
		ch := next[i]
		if ch >= '0' && ch <= '9' {
			builder.WriteByte(ch)
			continue
		}
		if ch == '+' && builder.Len() == 0 {
			builder.WriteByte(ch)
		}
	}
	normalized := builder.String()
	if normalized == "" || normalized == "+" {
		return next
	}
	if strings.HasPrefix(normalized, "00") {
		return "+" + strings.TrimPrefix(normalized, "00")
	}
	if strings.HasPrefix(normalized, "+") {
		return normalized
	}
	if strings.HasPrefix(normalized, "62") && len(normalized) >= 10 {
		return "+" + normalized
	}
	return next
}

func chooseNonEmpty(primary, fallback string) string {
	nextPrimary := strings.TrimSpace(primary)
	if nextPrimary != "" {
		return nextPrimary
	}
	return strings.TrimSpace(fallback)
}

func normalizeEmail(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func syncRequestRecordKey(tenantID, requestID string) string {
	nextTenantID := strings.TrimSpace(tenantID)
	nextRequestID := strings.TrimSpace(requestID)
	if nextTenantID == "" || nextRequestID == "" {
		return ""
	}
	return nextTenantID + ":" + nextRequestID
}

func emailDomain(email string) (string, error) {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "", ErrInvalidDomain
	}
	return normalizeDomain(parts[1])
}

func looksLikeHTTPSURL(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "https://")
}

func looksLikeAllowedRedirectURI(input string) bool {
	next := strings.ToLower(strings.TrimSpace(input))
	if strings.HasPrefix(next, "https://") {
		return true
	}
	return strings.HasPrefix(next, "http://localhost")
}

func randomID(prefix string) (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw), nil
}

func cloneEmployee(input EnterpriseEmployee) EnterpriseEmployee {
	output := input
	output.GroupIDs = append([]string(nil), input.GroupIDs...)
	return output
}

func cloneDomainMappings(items []DomainMapping) []DomainMapping {
	output := make([]DomainMapping, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneHRISConnector(input HRISConnector) HRISConnector {
	output := input
	if input.LastSyncAt != nil {
		lastSyncAt := *input.LastSyncAt
		output.LastSyncAt = &lastSyncAt
	}
	return output
}

func cloneHRISConnectors(items []HRISConnector) []HRISConnector {
	output := make([]HRISConnector, 0, len(items))
	for i := range items {
		output = append(output, cloneHRISConnector(items[i]))
	}
	return output
}

func cloneHRISWebhookReceipt(input HRISWebhookReceipt) HRISWebhookReceipt {
	output := input
	output.Headers = cloneStringMap(input.Headers)
	if input.LastAttemptAt != nil {
		lastAttemptAt := *input.LastAttemptAt
		output.LastAttemptAt = &lastAttemptAt
	}
	if input.ProcessedAt != nil {
		processedAt := *input.ProcessedAt
		output.ProcessedAt = &processedAt
	}
	return output
}

func cloneHRISWebhookReceipts(items []HRISWebhookReceipt) []HRISWebhookReceipt {
	output := make([]HRISWebhookReceipt, 0, len(items))
	for i := range items {
		output = append(output, cloneHRISWebhookReceipt(items[i]))
	}
	return output
}

func cloneHRISWebhookExecution(input HRISWebhookExecution) HRISWebhookExecution {
	output := input
	output.ReplayRequireWorker = cloneOptionalBool(input.ReplayRequireWorker)
	if input.StartedAt != nil {
		startedAt := input.StartedAt.UTC()
		output.StartedAt = &startedAt
	}
	if input.FinishedAt != nil {
		finishedAt := input.FinishedAt.UTC()
		output.FinishedAt = &finishedAt
	}
	return output
}

func cloneOptionalBool(input *bool) *bool {
	if input == nil {
		return nil
	}
	value := *input
	return &value
}

func cloneHRISWebhookExecutions(items []HRISWebhookExecution) []HRISWebhookExecution {
	output := make([]HRISWebhookExecution, 0, len(items))
	for i := range items {
		output = append(output, cloneHRISWebhookExecution(items[i]))
	}
	return output
}

func cloneHRISWebhookReceiptDueIndexEntries(
	items []hrisWebhookReceiptDueIndexEntry,
) []hrisWebhookReceiptDueIndexEntry {
	output := make([]hrisWebhookReceiptDueIndexEntry, 0, len(items))
	for i := range items {
		item := items[i]
		item.ReceiptID = strings.TrimSpace(item.ReceiptID)
		if item.ReceiptID == "" {
			continue
		}
		item.DueAt = item.DueAt.UTC()
		output = append(output, item)
	}
	return output
}

func cloneIDPConfigs(items map[string]IDPConfig) map[string]IDPConfig {
	output := make(map[string]IDPConfig, len(items))
	for tenantID, record := range items {
		record.Scopes = append([]string(nil), record.Scopes...)
		output[tenantID] = record
	}
	return output
}

func cloneEmployees(items []EnterpriseEmployee) []EnterpriseEmployee {
	output := make([]EnterpriseEmployee, 0, len(items))
	for i := range items {
		output = append(output, cloneEmployee(items[i]))
	}
	return output
}

func cloneSyncJobs(items []SyncJob) []SyncJob {
	output := make([]SyncJob, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneSyncResult(input SyncResult) SyncResult {
	output := SyncResult{
		Job:   input.Job,
		Items: make([]EnterpriseEmployee, 0, len(input.Items)),
	}
	for i := range input.Items {
		output.Items = append(output.Items, cloneEmployee(input.Items[i]))
	}
	return output
}

func cloneSyncRequestRecord(input SyncRequestRecord) SyncRequestRecord {
	output := input
	output.Result = cloneSyncResult(input.Result)
	if input.LastAccessAttemptAt != nil {
		attemptAt := *input.LastAccessAttemptAt
		output.LastAccessAttemptAt = &attemptAt
	}
	return output
}

func cloneSyncRequestRecords(items map[string]SyncRequestRecord) map[string]SyncRequestRecord {
	if len(items) == 0 {
		return map[string]SyncRequestRecord{}
	}
	output := make(map[string]SyncRequestRecord, len(items))
	for key, record := range items {
		output[key] = cloneSyncRequestRecord(record)
	}
	return output
}

func cloneJITProvisionApproval(input JITProvisionApproval) JITProvisionApproval {
	output := input
	if input.ReviewedAt != nil {
		reviewedAt := *input.ReviewedAt
		output.ReviewedAt = &reviewedAt
	}
	return output
}

func cloneJITProvisionApprovals(items []JITProvisionApproval) []JITProvisionApproval {
	output := make([]JITProvisionApproval, 0, len(items))
	for i := range items {
		output = append(output, cloneJITProvisionApproval(items[i]))
	}
	return output
}

func cloneSyncWorkerAlertSubscriptions(items []SyncWorkerAlertSubscription) []SyncWorkerAlertSubscription {
	output := make([]SyncWorkerAlertSubscription, 0, len(items))
	for i := range items {
		output = append(output, cloneSyncWorkerAlertSubscription(items[i]))
	}
	return output
}

func cloneSyncWorkerAlertSubscription(input SyncWorkerAlertSubscription) SyncWorkerAlertSubscription {
	output := input
	output.ReceiverGroups = normalizeSyncWorkerAlertSubscriptionReceiverGroups(input.ReceiverGroups)
	return output
}

func cloneSyncWorkerAlertNotifications(items []SyncWorkerAlertNotification) []SyncWorkerAlertNotification {
	output := make([]SyncWorkerAlertNotification, 0, len(items))
	for i := range items {
		output = append(output, cloneSyncWorkerAlertNotification(items[i]))
	}
	return output
}

func cloneSyncWorkerAlertNotification(input SyncWorkerAlertNotification) SyncWorkerAlertNotification {
	output := input
	output.Channels = append([]string(nil), input.Channels...)
	output.ReceiverGroups = normalizeSyncWorkerAlertSubscriptionReceiverGroups(input.ReceiverGroups)
	output.ChannelResults = cloneSyncWorkerAlertChannelResults(input.ChannelResults)
	if input.NextRetryAt != nil {
		nextRetryAt := *input.NextRetryAt
		output.NextRetryAt = &nextRetryAt
	}
	if input.LastConfirmAttemptAt != nil {
		lastConfirmAttemptAt := *input.LastConfirmAttemptAt
		output.LastConfirmAttemptAt = &lastConfirmAttemptAt
	}
	return output
}

func cloneSyncWorkerAlertCooldowns(items []SyncWorkerAlertCooldown) []SyncWorkerAlertCooldown {
	output := make([]SyncWorkerAlertCooldown, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneSyncWorkerAlertInFlights(items []SyncWorkerAlertInFlight) []SyncWorkerAlertInFlight {
	output := make([]SyncWorkerAlertInFlight, 0, len(items))
	for i := range items {
		item := items[i]
		item.Notification = cloneSyncWorkerAlertNotification(items[i].Notification)
		output = append(output, item)
	}
	return output
}

func findHRISConnectorIndexLocked(items []HRISConnector, connectorID string) int {
	nextConnectorID := strings.TrimSpace(connectorID)
	for i := range items {
		if strings.TrimSpace(items[i].ID) == nextConnectorID {
			return i
		}
	}
	return -1
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func normalizeSyncWorkerAlertSubscriptionReceiverGroups(items []string) []string {
	if len(items) == 0 {
		return nil
	}

	output := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for i := range items {
		next := strings.TrimSpace(items[i])
		if next == "" {
			continue
		}
		key := strings.ToLower(next)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		output = append(output, next)
	}
	if len(output) == 0 {
		return nil
	}
	return output
}
