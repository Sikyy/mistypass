package enterprise

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
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
	Nonce       string    `json:"-"` // OIDC nonce sent in the authorize request; never returned to clients
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
	consumedSAMLAssertions       map[string]time.Time
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
