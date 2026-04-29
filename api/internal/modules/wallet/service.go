package wallet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mistypass/cloud/api/internal/modules/wallet/alertdispatch"
	"github.com/mistypass/cloud/api/internal/modules/wallet/googleclient"
)

var ErrGoogleConfigNotFound = errors.New("google wallet config not found")
var ErrIssuerIDRequired = errors.New("issuer_id is required")
var ErrServiceAccountEmailRequired = errors.New("service_account_email is required")
var ErrKeyRefRequired = errors.New("key_ref is required")
var ErrInvalidConfigStatus = errors.New("invalid config status")
var ErrTemplateIDRequired = errors.New("template_id is required")
var ErrTemplateNameRequired = errors.New("template name is required")
var ErrTemplateNotFound = errors.New("template not found")
var ErrTemplateInactive = errors.New("template is not active")
var ErrInvalidTemplateStatus = errors.New("invalid template status")
var ErrInvalidPassType = errors.New("invalid pass_type")
var ErrInvalidTargetType = errors.New("invalid target_type")
var ErrTargetIDRequired = errors.New("target_id is required")
var ErrTargetIDsRequired = errors.New("target_ids is required")
var ErrPassNotFound = errors.New("pass not found")
var ErrPassIDRequired = errors.New("pass_id is required")
var ErrInvalidPassTransition = errors.New("invalid pass status transition")
var ErrPassDeliveryChannelRequired = errors.New("delivery channel is required")
var ErrInvalidPassDeliveryChannel = errors.New("invalid pass delivery channel")
var ErrPassDeliveryNotificationNotFound = errors.New("pass delivery notification not found")
var ErrPassDeliveryRetryNotAllowed = errors.New("pass delivery retry not allowed")
var ErrPhysicalCardTaskNotFound = errors.New("physical card task not found")
var ErrInvalidPhysicalCardTaskType = errors.New("invalid physical card task type")
var ErrInvalidPhysicalCardTaskStatus = errors.New("invalid physical card task status")
var ErrInvalidPhysicalCardTaskTransition = errors.New("invalid physical card task status transition")
var ErrPhysicalCardTaskEmployeePassRequired = errors.New("physical card task requires employee pass")
var ErrPhysicalCardInventoryCardNumberRequired = errors.New("physical card inventory card_number is required")
var ErrPhysicalCardInventoryUIDRequired = errors.New("physical card inventory uid is required")
var ErrPhysicalCardInventoryRecordsRequired = errors.New("physical card inventory records are required")
var ErrPhysicalCardInventoryNotFound = errors.New("physical card inventory not found")
var ErrPhysicalCardInventoryAlreadyExists = errors.New("physical card inventory already exists")
var ErrPhysicalCardInventoryIDsRequired = errors.New("physical card inventory ids are required")
var ErrInvalidPhysicalCardInventoryStatus = errors.New("invalid physical card inventory status")
var ErrInvalidPhysicalCardInventoryTransition = errors.New("invalid physical card inventory status transition")
var ErrPhysicalCardVendorNotFound = errors.New("physical card vendor not found")
var ErrJobNotFound = errors.New("job not found")
var ErrJobRetryNotAllowed = errors.New("job retry not allowed")
var ErrJobNotInDLQ = errors.New("job is not in dlq")
var ErrInvalidJobDLQOptions = errors.New("invalid wallet job dlq options")
var ErrInvalidJobProcessOptions = errors.New("invalid wallet job process options")
var ErrInvalidJobAlertSubscriptionOptions = errors.New("invalid wallet job alert subscription options")
var ErrInvalidJobAlertEmailProvider = errors.New("invalid wallet job alert email provider")
var ErrInvalidJobAlertWhatsAppProvider = errors.New("invalid wallet job alert whatsapp provider")
var ErrJobAlertNotificationNotFound = errors.New("wallet job alert notification not found")
var ErrJobAlertRetryNotAllowed = errors.New("wallet job alert retry not allowed")

type GoogleConfig struct {
	ID                  string    `json:"id"`
	TenantID            string    `json:"tenant_id"`
	Provider            string    `json:"provider"`
	IssuerID            string    `json:"issuer_id"`
	ServiceAccountEmail string    `json:"service_account_email"`
	KeyRef              string    `json:"key_ref"`
	Status              string    `json:"status"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type ConfigValidationItem struct {
	Field   string `json:"field"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type GoogleConfigValidation struct {
	Provider  string                 `json:"provider"`
	TenantID  string                 `json:"tenant_id"`
	Valid     bool                   `json:"valid"`
	Items     []ConfigValidationItem `json:"items"`
	CheckedAt time.Time              `json:"checked_at"`
}

type PassTemplate struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	Provider    string            `json:"provider"`
	PassType    string            `json:"pass_type"`
	ClassID     string            `json:"class_id"`
	Name        string            `json:"name"`
	StyleConfig map[string]string `json:"style_config,omitempty"`
	Status      string            `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type PassInstance struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	Provider       string     `json:"provider"`
	CredentialKind string     `json:"credential_kind,omitempty"`
	TemplateID     string     `json:"template_id"`
	TargetType     string     `json:"target_type"`
	TargetID       string     `json:"target_id"`
	ObjectID       string     `json:"object_id"`
	Token          string     `json:"token,omitempty"`
	UID            string     `json:"uid,omitempty"`
	CardNumber     string     `json:"card_number,omitempty"`
	Status         string     `json:"status"`
	SaveLink       string     `json:"save_link"`
	ExpiresAt      string     `json:"expires_at,omitempty"`
	IssuedAt       time.Time  `json:"issued_at"`
	ActivatedAt    *time.Time `json:"activated_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedBy      string     `json:"created_by"`
	UpdatedBy      string     `json:"updated_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type IssueJob struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Provider     string    `json:"provider"`
	BatchID      string    `json:"batch_id"`
	TemplateID   string    `json:"template_id"`
	TargetType   string    `json:"target_type"`
	TargetID     string    `json:"target_id"`
	ExpiresAt    string    `json:"expires_at,omitempty"`
	PassID       string    `json:"pass_id,omitempty"`
	Status       string    `json:"status"`
	RetryCount   int       `json:"retry_count"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type JobProcessOptions struct {
	TenantID    string
	Limit       int
	WorkerCount int
	MaxRetry    int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	Actor       string
}

type JobProcessResult struct {
	TenantID        string    `json:"tenant_id"`
	Limit           int       `json:"limit"`
	WorkerCount     int       `json:"worker_count"`
	MaxRetry        int       `json:"max_retry"`
	Claimed         int       `json:"claimed"`
	Succeeded       int       `json:"succeeded"`
	Failed          int       `json:"failed"`
	DLQ             int       `json:"dlq"`
	Skipped         int       `json:"skipped"`
	Retried         int       `json:"retried"`
	PendingAfter    int       `json:"pending_after"`
	ProcessedJobIDs []string  `json:"processed_job_ids"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at"`
}

type JobSummary struct {
	TenantID           string         `json:"tenant_id"`
	MaxRetry           int            `json:"max_retry"`
	Total              int            `json:"total"`
	Pending            int            `json:"pending"`
	Processing         int            `json:"processing"`
	Success            int            `json:"success"`
	Failed             int            `json:"failed"`
	DLQ                int            `json:"dlq"`
	RetryableFailed    int            `json:"retryable_failed"`
	NonRetryableFailed int            `json:"non_retryable_failed"`
	ErrorCodeBreakdown map[string]int `json:"error_code_breakdown,omitempty"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type JobMetrics struct {
	TenantID          string            `json:"tenant_id"`
	MaxRetry          int               `json:"max_retry"`
	DLQAlertThreshold int               `json:"dlq_alert_threshold"`
	Summary           JobSummary        `json:"summary"`
	Window            JobMetricsWindow  `json:"window"`
	Alerts            []JobMetricsAlert `json:"alerts,omitempty"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type JobMetricsWindow struct {
	WindowSeconds      int64          `json:"window_seconds"`
	Since              time.Time      `json:"since"`
	Until              time.Time      `json:"until"`
	Created            int            `json:"created"`
	Updated            int            `json:"updated"`
	Pending            int            `json:"pending"`
	Processing         int            `json:"processing"`
	Success            int            `json:"success"`
	Failed             int            `json:"failed"`
	DLQ                int            `json:"dlq"`
	ErrorCodeBreakdown map[string]int `json:"error_code_breakdown,omitempty"`
}

type JobMetricsAlert struct {
	Type      string `json:"type"`
	ErrorCode string `json:"error_code,omitempty"`
	Count     int    `json:"count"`
	Threshold int    `json:"threshold"`
}

type JobMetricsTrend struct {
	TenantID          string                  `json:"tenant_id"`
	MaxRetry          int                     `json:"max_retry"`
	DLQAlertThreshold int                     `json:"dlq_alert_threshold"`
	WindowSeconds     int64                   `json:"window_seconds"`
	BucketSeconds     int64                   `json:"bucket_seconds"`
	BucketCount       int                     `json:"bucket_count"`
	Since             time.Time               `json:"since"`
	Until             time.Time               `json:"until"`
	Summary           JobSummary              `json:"summary"`
	Alerts            []JobMetricsAlert       `json:"alerts,omitempty"`
	Buckets           []JobMetricsTrendBucket `json:"buckets"`
	UpdatedAt         time.Time               `json:"updated_at"`
}

type JobMetricsTrendBucket struct {
	Index              int            `json:"index"`
	Start              time.Time      `json:"start"`
	End                time.Time      `json:"end"`
	Created            int            `json:"created"`
	Updated            int            `json:"updated"`
	Pending            int            `json:"pending"`
	Processing         int            `json:"processing"`
	Success            int            `json:"success"`
	Failed             int            `json:"failed"`
	DLQ                int            `json:"dlq"`
	ErrorCodeBreakdown map[string]int `json:"error_code_breakdown,omitempty"`
}

type JobAlertSubscriptionChannels struct {
	Email    bool `json:"email"`
	WhatsApp bool `json:"whatsapp"`
}

type JobAlertSubscription struct {
	TenantID          string                       `json:"tenant_id"`
	Enabled           bool                         `json:"enabled"`
	DLQAlertThreshold int                          `json:"dlq_alert_threshold"`
	WindowSeconds     int64                        `json:"window_seconds"`
	CooldownSeconds   int64                        `json:"cooldown_seconds"`
	Channels          JobAlertSubscriptionChannels `json:"channels"`
	ReceiverGroups    []string                     `json:"receiver_groups,omitempty"`
	UpdatedAt         time.Time                    `json:"updated_at"`
}

type JobAlertSubscriptionUpsertOptions struct {
	TenantID          string
	Enabled           bool
	DLQAlertThreshold int
	Window            time.Duration
	Cooldown          time.Duration
	EmailEnabled      bool
	WhatsAppEnabled   bool
	ReceiverGroups    []string
	Actor             string
}

type JobAlertEmailDeliveryOptions struct {
	Provider              string
	EmailFrom             string
	ReceiverMap           map[string][]string
	ResendEndpoint        string
	ResendAPIKey          string
	ResendTimeout         time.Duration
	WhatsAppProvider      string
	WhatsAppReceiverMap   map[string][]string
	WhatsAppEndpoint      string
	WhatsAppAPIKey        string
	WhatsAppPhoneNumberID string
	WhatsAppTimeout       time.Duration
}

type JobAlertChannelResult struct {
	Channel                string   `json:"channel"`
	Status                 string   `json:"status"`
	Reason                 string   `json:"reason,omitempty"`
	Provider               string   `json:"provider,omitempty"`
	ProviderError          string   `json:"provider_error,omitempty"`
	ProviderDeliveryID     string   `json:"provider_delivery_id,omitempty"`
	ProviderDeliveryStatus string   `json:"provider_delivery_status,omitempty"`
	Retryable              bool     `json:"retryable"`
	Receivers              []string `json:"receivers,omitempty"`
}

type JobAlertNotification struct {
	ID                   string                  `json:"id"`
	TenantID             string                  `json:"tenant_id"`
	Type                 string                  `json:"type"`
	ErrorCode            string                  `json:"error_code,omitempty"`
	Count                int                     `json:"count"`
	Threshold            int                     `json:"threshold"`
	Channels             []string                `json:"channels,omitempty"`
	ReceiverGroups       []string                `json:"receiver_groups,omitempty"`
	Status               string                  `json:"status"`
	Reason               string                  `json:"reason,omitempty"`
	IdempotencyKey       string                  `json:"idempotency_key,omitempty"`
	Attempt              int                     `json:"attempt,omitempty"`
	Retryable            bool                    `json:"retryable"`
	Provider             string                  `json:"provider,omitempty"`
	ProviderError        string                  `json:"provider_error,omitempty"`
	ChannelResults       []JobAlertChannelResult `json:"channel_results,omitempty"`
	SourceNotificationID string                  `json:"source_notification_id,omitempty"`
	TriggeredAt          time.Time               `json:"triggered_at"`
}

type JobAlertDispatchResult struct {
	TenantID    string                 `json:"tenant_id"`
	TotalAlerts int                    `json:"total_alerts"`
	Dispatched  int                    `json:"dispatched"`
	Skipped     int                    `json:"skipped"`
	Failed      int                    `json:"failed"`
	Items       []JobAlertNotification `json:"items,omitempty"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type JobAlertDispatchCooldown struct {
	TenantID   string    `json:"tenant_id"`
	ErrorCode  string    `json:"error_code"`
	LastSentAt time.Time `json:"last_sent_at"`
}

type JobDLQRequeueOptions struct {
	TenantID         string
	Limit            int
	ErrorCode        string
	TargetIDOverride string
	Actor            string
}

type JobDLQRequeueResult struct {
	TenantID      string    `json:"tenant_id"`
	Limit         int       `json:"limit"`
	Requeued      int       `json:"requeued"`
	Skipped       int       `json:"skipped"`
	RemainingDLQ  int       `json:"remaining_dlq"`
	ProcessedJobs []string  `json:"processed_jobs"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type JobDLQCleanupOptions struct {
	TenantID  string
	Limit     int
	ErrorCode string
	OlderThan time.Duration
	Actor     string
}

type JobDLQCleanupResult struct {
	TenantID      string    `json:"tenant_id"`
	Limit         int       `json:"limit"`
	Removed       int       `json:"removed"`
	RemainingDLQ  int       `json:"remaining_dlq"`
	ProcessedJobs []string  `json:"processed_jobs"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type JobDLQCleanupArchive struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	Limit            int       `json:"limit"`
	ErrorCode        string    `json:"error_code,omitempty"`
	OlderThanSeconds int64     `json:"older_than_seconds"`
	Actor            string    `json:"actor"`
	Removed          int       `json:"removed"`
	RemainingDLQ     int       `json:"remaining_dlq"`
	ProcessedJobs    []string  `json:"processed_jobs,omitempty"`
	At               time.Time `json:"at"`
}

type AuditLog struct {
	ID       string    `json:"id"`
	TenantID string    `json:"tenant_id"`
	Action   string    `json:"action"`
	Actor    string    `json:"actor"`
	TargetID string    `json:"target_id"`
	Result   string    `json:"result"`
	At       time.Time `json:"at"`
}

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

const stateKey = "module_wallet"

type stateSnapshot struct {
	Config                    *GoogleConfig               `json:"config"`
	Templates                 []PassTemplate              `json:"templates"`
	Passes                    []PassInstance              `json:"passes"`
	PassDeliveryNotifications []PassDeliveryNotification  `json:"pass_delivery_notifications,omitempty"`
	PhysicalCardTasks         []PhysicalCardTask          `json:"physical_card_tasks,omitempty"`
	PhysicalCardVendors       []PhysicalCardVendor        `json:"physical_card_vendors,omitempty"`
	PhysicalCardInventory     []PhysicalCardInventoryItem `json:"physical_card_inventory,omitempty"`
	Jobs                      []IssueJob                  `json:"jobs"`
	AuditLogs                 []AuditLog                  `json:"audit_logs"`
	DLQCleanupArchives        []JobDLQCleanupArchive      `json:"dlq_cleanup_archives,omitempty"`
	JobAlertSubscriptions     []JobAlertSubscription      `json:"job_alert_subscriptions,omitempty"`
	JobAlertNotifications     []JobAlertNotification      `json:"job_alert_notifications,omitempty"`
	JobAlertCooldowns         []JobAlertDispatchCooldown  `json:"job_alert_cooldowns,omitempty"`
}

type Service struct {
	mu                             sync.RWMutex
	config                         *GoogleConfig
	templates                      []PassTemplate
	passes                         []PassInstance
	passDeliveryNotifications      []PassDeliveryNotification
	physicalCardTasks              []PhysicalCardTask
	physicalCardVendors            []PhysicalCardVendor
	physicalCardInventory          []PhysicalCardInventoryItem
	jobs                           []IssueJob
	auditLogs                      []AuditLog
	dlqCleanupArchives             []JobDLQCleanupArchive
	jobAlertSubscriptions          []JobAlertSubscription
	jobAlertNotifications          []JobAlertNotification
	jobAlertCooldowns              []JobAlertDispatchCooldown
	jobAlertMockTransientFailCount int
	jobAlertEmailProvider          string
	jobAlertEmailFrom              string
	jobAlertEmailReceiverMap       map[string][]string
	jobAlertEmailSender            alertEmailSender
	jobAlertWhatsAppProvider       string
	jobAlertWhatsAppReceiverMap    map[string][]string
	jobAlertWhatsAppSender         alertWhatsAppSender
	stateStore                     StateStore
	applePassProvider              *ApplePassProvider
	googleWalletProvider           *GoogleWalletProvider
}

func NewService() *Service {
	now := time.Now().UTC()
	activated := now.Add(-2 * time.Hour)
	return &Service{
		config: &GoogleConfig{
			ID:                  "wcfg_google_demo",
			TenantID:            "tenant_demo_jakarta",
			Provider:            "google",
			IssuerID:            "issuer_demo_jakarta",
			ServiceAccountEmail: "wallet-issuer@mistypass.iam.gserviceaccount.com",
			KeyRef:              "kms://projects/mistypass/locations/global/keyRings/wallet/cryptoKeys/issuer",
			Status:              "active",
			CreatedAt:           now,
			UpdatedAt:           now,
		},
		templates: []PassTemplate{
			{
				ID:       "wpt_employee_demo",
				TenantID: "tenant_demo_jakarta",
				Provider: "google",
				PassType: "employee",
				ClassID:  "mistypass.employee.class",
				Name:     "员工门禁卡（Google Wallet）",
				StyleConfig: map[string]string{
					"brand":      "MistyPass",
					"themeColor": "#0F766E",
				},
				Status:    "active",
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:       "wpt_visitor_demo",
				TenantID: "tenant_demo_jakarta",
				Provider: "google",
				PassType: "visitor",
				ClassID:  "mistypass.visitor.class",
				Name:     "访客通行证（Google Wallet）",
				StyleConfig: map[string]string{
					"brand":      "MistyPass",
					"themeColor": "#0369A1",
				},
				Status:    "active",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		passes: []PassInstance{
			{
				ID:             "wps_demo_1001",
				TenantID:       "tenant_demo_jakarta",
				Provider:       "google",
				CredentialKind: "google_wallet",
				TemplateID:     "wpt_employee_demo",
				TargetType:     "user",
				TargetID:       "usr_1001",
				ObjectID:       "mistypass.employee.class.wps_demo_1001",
				Status:         "active",
				SaveLink:       "https://pay.google.com/gp/v/save/wps_demo_1001",
				IssuedAt:       now.Add(-4 * time.Hour),
				ActivatedAt:    &activated,
				CreatedBy:      "system",
				UpdatedBy:      "system",
				CreatedAt:      now.Add(-4 * time.Hour),
				UpdatedAt:      activated,
			},
		},
		passDeliveryNotifications: []PassDeliveryNotification{
			{
				ID:         "wdn_demo_1001",
				TenantID:   "tenant_demo_jakarta",
				PassID:     "wps_demo_1001",
				TemplateID: "wpt_employee_demo",
				TargetType: "user",
				TargetID:   "usr_1001",
				Channels:   []string{"email", "whatsapp"},
				Status:     "sent",
				Attempt:    1,
				Provider:   "mock",
				ChannelResults: []PassDeliveryChannelResult{
					{
						Channel:   "email",
						Status:    "sent",
						Provider:  "mock",
						Retryable: false,
						Receivers: []string{"alice@mistypass.local"},
					},
					{
						Channel:   "whatsapp",
						Status:    "sent",
						Provider:  "mock",
						Retryable: false,
						Receivers: []string{"+628111111111"},
					},
				},
				TriggeredAt: now.Add(-70 * time.Minute),
			},
		},
		physicalCardTasks: []PhysicalCardTask{
			{
				ID:          "wpc_demo_1001",
				TenantID:    "tenant_demo_jakarta",
				PassID:      "wps_demo_1001",
				TemplateID:  "wpt_employee_demo",
				TargetType:  "user",
				TargetID:    "usr_1001",
				TaskType:    "issue",
				Status:      "ready",
				CardNumber:  "CARD-1001",
				InventoryID: "wpci_demo_1001",
				VendorID:    "wpcv_nusacard_demo",
				VendorName:  "NusaCard Fulfillment",
				Note:        "等待前台交付实体卡并完成绑定",
				PassStatus:  "active",
				CreatedBy:   "system",
				UpdatedBy:   "system",
				CreatedAt:   now.Add(-90 * time.Minute),
				UpdatedAt:   now.Add(-20 * time.Minute),
			},
		},
		physicalCardVendors: []PhysicalCardVendor{
			{
				ID:        "wpcv_nusacard_demo",
				TenantID:  "tenant_demo_jakarta",
				Name:      "NusaCard Fulfillment",
				Provider:  "nusacard",
				Status:    "active",
				CreatedAt: now.Add(-24 * time.Hour),
				UpdatedAt: now.Add(-24 * time.Hour),
			},
			{
				ID:        "wpcv_internal_demo",
				TenantID:  "tenant_demo_jakarta",
				Name:      "Internal Badge Desk",
				Provider:  "internal",
				Status:    "active",
				CreatedAt: now.Add(-24 * time.Hour),
				UpdatedAt: now.Add(-24 * time.Hour),
			},
		},
		physicalCardInventory: []PhysicalCardInventoryItem{
			{
				ID:             "wpci_demo_1001",
				TenantID:       "tenant_demo_jakarta",
				CardNumber:     "CARD-1001",
				UID:            "UID-1001",
				VendorID:       "wpcv_nusacard_demo",
				VendorName:     "NusaCard Fulfillment",
				Status:         "reserved",
				AssignedPassID: "wps_demo_1001",
				ActiveTaskID:   "wpc_demo_1001",
				CreatedAt:      now.Add(-24 * time.Hour),
				UpdatedAt:      now.Add(-20 * time.Minute),
			},
			{
				ID:         "wpci_demo_1002",
				TenantID:   "tenant_demo_jakarta",
				CardNumber: "CARD-1002",
				UID:        "UID-1002",
				VendorID:   "wpcv_nusacard_demo",
				VendorName: "NusaCard Fulfillment",
				Status:     "available",
				CreatedAt:  now.Add(-24 * time.Hour),
				UpdatedAt:  now.Add(-24 * time.Hour),
			},
		},
		jobs: []IssueJob{
			{
				ID:         "wjb_demo_2201",
				TenantID:   "tenant_demo_jakarta",
				Provider:   "google",
				BatchID:    "wbt_demo_0001",
				TemplateID: "wpt_employee_demo",
				TargetType: "user",
				TargetID:   "usr_1001",
				PassID:     "wps_demo_1001",
				Status:     "success",
				CreatedAt:  now.Add(-4 * time.Hour),
				UpdatedAt:  now.Add(-4 * time.Hour),
			},
		},
		auditLogs: []AuditLog{
			{
				ID:       "wal_demo_0001",
				TenantID: "tenant_demo_jakarta",
				Action:   "wallet.pass.issue",
				Actor:    "system",
				TargetID: "wps_demo_1001",
				Result:   "success",
				At:       now.Add(-4 * time.Hour),
			},
		},
		jobAlertSubscriptions: []JobAlertSubscription{},
		jobAlertNotifications: []JobAlertNotification{},
		jobAlertCooldowns:     []JobAlertDispatchCooldown{},
		jobAlertEmailProvider: "mock",
		jobAlertEmailFrom:     "no-reply@mistypass.local",
		jobAlertEmailReceiverMap: map[string][]string{
			"security": {"security@mistypass.local"},
		},
		jobAlertWhatsAppProvider: "mock",
		jobAlertWhatsAppReceiverMap: map[string][]string{
			"security": {"+10000000000"},
		},
		jobAlertWhatsAppSender: newWhatsAppMockSender(),
		applePassProvider:      NewApplePassProvider(ApplePassConfig{}),
		googleWalletProvider:   NewGoogleWalletProvider(GoogleWalletConfig{}),
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

func (s *Service) SetJobAlertMockTransientFailCount(count int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if count < 0 {
		count = 0
	}
	if count > 100 {
		count = 100
	}
	s.jobAlertMockTransientFailCount = count
}

func (s *Service) SetJobAlertEmailDeliveryOptions(options JobAlertEmailDeliveryOptions) error {
	nextProvider := strings.ToLower(strings.TrimSpace(options.Provider))
	if nextProvider == "" {
		nextProvider = "mock"
	}
	nextEmailFrom := strings.TrimSpace(options.EmailFrom)
	if nextEmailFrom == "" {
		nextEmailFrom = "no-reply@mistypass.local"
	}
	nextReceiverMap := normalizeJobAlertReceiverMap(options.ReceiverMap)
	if len(nextReceiverMap) == 0 {
		nextReceiverMap = map[string][]string{
			"security": {"security@mistypass.local"},
		}
	}
	nextWhatsAppProvider := strings.ToLower(strings.TrimSpace(options.WhatsAppProvider))
	if nextWhatsAppProvider == "" {
		nextWhatsAppProvider = "mock"
	}
	nextWhatsAppReceiverMap := normalizeJobAlertReceiverMap(options.WhatsAppReceiverMap)
	if len(nextWhatsAppReceiverMap) == 0 {
		nextWhatsAppReceiverMap = map[string][]string{
			"security": {"+10000000000"},
		}
	}

	var sender alertEmailSender
	switch nextProvider {
	case "mock":
		sender = nil
	case "resend", "spaceemail":
		nextProvider = "resend"
		resendSender, err := newResendSender(
			options.ResendEndpoint,
			options.ResendAPIKey,
			nextEmailFrom,
			options.ResendTimeout,
		)
		if err != nil {
			return err
		}
		sender = resendSender
	default:
		return ErrInvalidJobAlertEmailProvider
	}
	var whatsAppSender alertWhatsAppSender
	switch nextWhatsAppProvider {
	case "mock":
		whatsAppSender = newWhatsAppMockSender()
	case "meta":
		metaSender, err := newMetaWhatsAppSender(
			options.WhatsAppEndpoint,
			options.WhatsAppAPIKey,
			options.WhatsAppPhoneNumberID,
			options.WhatsAppTimeout,
		)
		if err != nil {
			return err
		}
		whatsAppSender = metaSender
	default:
		return ErrInvalidJobAlertWhatsAppProvider
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobAlertEmailProvider = nextProvider
	s.jobAlertEmailFrom = nextEmailFrom
	s.jobAlertEmailReceiverMap = nextReceiverMap
	s.jobAlertEmailSender = sender
	s.jobAlertWhatsAppProvider = nextWhatsAppProvider
	s.jobAlertWhatsAppReceiverMap = nextWhatsAppReceiverMap
	s.jobAlertWhatsAppSender = whatsAppSender
	return nil
}

func (s *Service) GetGoogleConfig(tenantID string) (GoogleConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.config == nil {
		return GoogleConfig{}, ErrGoogleConfigNotFound
	}
	nextTenantID := normalizeTenantID(tenantID)
	if s.config.TenantID != nextTenantID {
		return GoogleConfig{}, ErrGoogleConfigNotFound
	}

	return *s.config, nil
}

func (s *Service) UpsertGoogleConfig(tenantID, issuerID, serviceAccountEmail, keyRef, status, actor string) (GoogleConfig, error) {
	nextIssuerID := strings.TrimSpace(issuerID)
	if nextIssuerID == "" {
		return GoogleConfig{}, ErrIssuerIDRequired
	}

	nextServiceAccountEmail := strings.TrimSpace(serviceAccountEmail)
	if nextServiceAccountEmail == "" {
		return GoogleConfig{}, ErrServiceAccountEmailRequired
	}

	nextKeyRef := strings.TrimSpace(keyRef)
	if nextKeyRef == "" {
		return GoogleConfig{}, ErrKeyRefRequired
	}

	nextStatus, err := normalizeConfigStatus(status)
	if err != nil {
		return GoogleConfig{}, err
	}

	nextActor := normalizeActor(actor)
	nextTenantID := normalizeTenantID(tenantID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	createdAt := now
	configID := ""
	if s.config != nil {
		createdAt = s.config.CreatedAt
		configID = s.config.ID
	}
	if configID == "" {
		configID, err = walletID("wcfg_")
		if err != nil {
			return GoogleConfig{}, err
		}
	}

	s.config = &GoogleConfig{
		ID:                  configID,
		TenantID:            nextTenantID,
		Provider:            "google",
		IssuerID:            nextIssuerID,
		ServiceAccountEmail: nextServiceAccountEmail,
		KeyRef:              nextKeyRef,
		Status:              nextStatus,
		CreatedAt:           createdAt,
		UpdatedAt:           now,
	}

	s.appendAuditLocked(nextTenantID, "wallet.google.config.upsert", nextActor, s.config.ID, "success")
	if err := s.persistLocked(); err != nil {
		return GoogleConfig{}, err
	}

	return *s.config, nil
}

func (s *Service) ValidateGoogleConfig(tenantID, issuerID, serviceAccountEmail, keyRef string) GoogleConfigValidation {
	nextTenantID := normalizeTenantID(tenantID)
	result := GoogleConfigValidation{
		Provider:  "google",
		TenantID:  nextTenantID,
		Valid:     true,
		Items:     make([]ConfigValidationItem, 0, 6),
		CheckedAt: time.Now().UTC(),
	}

	push := func(field, status, message string) {
		result.Items = append(result.Items, ConfigValidationItem{
			Field:   field,
			Status:  status,
			Message: message,
		})
		if status == "error" {
			result.Valid = false
		}
	}

	nextIssuerID := strings.TrimSpace(issuerID)
	if nextIssuerID == "" {
		push("issuer_id", "error", ErrIssuerIDRequired.Error())
	} else {
		push("issuer_id", "ok", "issuer_id looks good")
	}

	nextServiceAccountEmail := strings.TrimSpace(serviceAccountEmail)
	if nextServiceAccountEmail == "" {
		push("service_account_email", "error", ErrServiceAccountEmailRequired.Error())
	} else if !looksLikeServiceAccountEmail(nextServiceAccountEmail) {
		push("service_account_email", "error", "service_account_email must be a google service account address")
	} else {
		push("service_account_email", "ok", "service_account_email format is valid")
	}

	nextKeyRef := strings.TrimSpace(keyRef)
	if nextKeyRef == "" {
		push("key_ref", "error", ErrKeyRefRequired.Error())
	} else if !looksLikeKeyRef(nextKeyRef) {
		push("key_ref", "error", "key_ref must start with kms://, sm://, secret://, env://, or file://")
	} else {
		push("key_ref", "ok", "key_ref format is valid")
	}

	s.mu.RLock()
	hasConfig := s.config != nil && s.config.TenantID == nextTenantID
	s.mu.RUnlock()
	if hasConfig {
		push("existing_config", "ok", "existing tenant config found")
	} else {
		push("existing_config", "warn", "no existing tenant config, next save will create one")
	}

	if !result.Valid {
		return result
	}

	if !isRemoteGoogleValidationEnabled() {
		push("google_remote_validation", "warn", "remote google validation disabled; set WALLET_GOOGLE_REMOTE_VALIDATE=true to enable live issuer verification")
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), remoteGoogleValidationTimeout())
	defer cancel()

	report, err := googleclient.ValidateConfig(ctx, googleclient.ValidateInput{
		IssuerID:            nextIssuerID,
		ServiceAccountEmail: nextServiceAccountEmail,
		KeyRef:              nextKeyRef,
	})
	if err != nil {
		push("google_remote_validation", "error", err.Error())
		return result
	}
	for i := range report.Items {
		push(report.Items[i].Field, report.Items[i].Status, report.Items[i].Message)
	}

	return result
}

func (s *Service) ListTemplates(tenantID string) []PassTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	items := make([]PassTemplate, 0, len(s.templates))
	for i := range s.templates {
		if s.templates[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, cloneTemplate(s.templates[i]))
	}
	return items
}

func (s *Service) CreateTemplate(tenantID, passType, classID, name string, styleConfig map[string]string, status, actor string) (PassTemplate, error) {
	nextPassType, err := normalizePassType(passType)
	if err != nil {
		return PassTemplate{}, err
	}

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return PassTemplate{}, ErrTemplateNameRequired
	}

	nextStatus, err := normalizeTemplateStatus(status)
	if err != nil {
		return PassTemplate{}, err
	}

	nextClassID := strings.TrimSpace(classID)
	if nextClassID == "" {
		nextClassID = fmt.Sprintf("mistypass.%s.class", nextPassType)
	}

	id, err := walletID("wpt_")
	if err != nil {
		return PassTemplate{}, err
	}

	now := time.Now().UTC()
	record := PassTemplate{
		ID:          id,
		TenantID:    normalizeTenantID(tenantID),
		Provider:    "google",
		PassType:    nextPassType,
		ClassID:     nextClassID,
		Name:        nextName,
		StyleConfig: cloneStringMap(styleConfig),
		Status:      nextStatus,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.mu.Lock()
	s.templates = append([]PassTemplate{record}, s.templates...)
	s.appendAuditLocked(record.TenantID, "wallet.template.create", normalizeActor(actor), record.ID, "success")
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return PassTemplate{}, err
	}
	s.mu.Unlock()

	return cloneTemplate(record), nil
}

func (s *Service) UpdateTemplateStatus(tenantID, templateID, status, actor string) (PassTemplate, error) {
	nextTemplateID := strings.TrimSpace(templateID)
	if nextTemplateID == "" {
		return PassTemplate{}, ErrTemplateIDRequired
	}

	nextStatus, err := normalizeTemplateStatus(status)
	if err != nil {
		return PassTemplate{}, err
	}

	nextTenantID := normalizeTenantID(tenantID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.templates {
		if s.templates[i].ID != nextTemplateID {
			continue
		}
		if s.templates[i].TenantID != nextTenantID {
			return PassTemplate{}, ErrTemplateNotFound
		}

		s.templates[i].Status = nextStatus
		s.templates[i].UpdatedAt = now
		s.appendAuditLocked(s.templates[i].TenantID, "wallet.template.status", normalizeActor(actor), s.templates[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return PassTemplate{}, err
		}
		return cloneTemplate(s.templates[i]), nil
	}

	return PassTemplate{}, ErrTemplateNotFound
}

func (s *Service) ListPasses(tenantID string) []PassInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	items := make([]PassInstance, 0, len(s.passes))
	for i := range s.passes {
		if s.passes[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, s.passes[i])
	}
	return items
}

func (s *Service) GetPass(tenantID, passID string) (PassInstance, error) {
	nextPassID := strings.TrimSpace(passID)
	if nextPassID == "" {
		return PassInstance{}, ErrPassNotFound
	}

	nextTenantID := normalizeTenantID(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.passes {
		if s.passes[i].ID == nextPassID {
			if s.passes[i].TenantID != nextTenantID {
				return PassInstance{}, ErrPassNotFound
			}
			return s.passes[i], nil
		}
	}
	return PassInstance{}, ErrPassNotFound
}

func (s *Service) IssuePass(tenantID, templateID, targetType, targetID, expiresAt, actor string) (PassInstance, error) {
	nextTemplateID := strings.TrimSpace(templateID)
	if nextTemplateID == "" {
		return PassInstance{}, ErrTemplateIDRequired
	}

	nextTargetType, err := normalizeTargetType(targetType)
	if err != nil {
		return PassInstance{}, err
	}

	nextTargetID := strings.TrimSpace(targetID)
	if nextTargetID == "" {
		return PassInstance{}, ErrTargetIDRequired
	}

	now := time.Now().UTC()
	nextActor := normalizeActor(actor)
	nextTenantID := normalizeTenantID(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	template, found := findTemplateByID(s.templates, nextTemplateID)
	if !found {
		return PassInstance{}, ErrTemplateNotFound
	}
	if template.TenantID != nextTenantID {
		return PassInstance{}, ErrTemplateNotFound
	}
	if template.Status != "active" {
		return PassInstance{}, ErrTemplateInactive
	}

	record, err := s.createPassRecord(nextTenantID, template, nextTargetType, nextTargetID, "", "", strings.TrimSpace(expiresAt), nextActor, now)
	if err != nil {
		return PassInstance{}, err
	}

	s.passes = append([]PassInstance{record}, s.passes...)
	s.appendAuditLocked(nextTenantID, "wallet.pass.issue", nextActor, record.ID, "success")
	if err := s.persistLocked(); err != nil {
		return PassInstance{}, err
	}

	return record, nil
}

func (s *Service) CreateUnassignedCard(tenantID, templateID, token, uid, cardNumber, cardType, expiresAt, actor string) (PassInstance, error) {
	nextTemplateID := strings.TrimSpace(templateID)
	if nextTemplateID == "" {
		return PassInstance{}, ErrTemplateIDRequired
	}

	now := time.Now().UTC()
	nextActor := normalizeActor(actor)
	nextTenantID := normalizeTenantID(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	template, found := findTemplateByID(s.templates, nextTemplateID)
	if !found {
		return PassInstance{}, ErrTemplateNotFound
	}
	if template.TenantID != nextTenantID {
		return PassInstance{}, ErrTemplateNotFound
	}
	if template.Status != "active" {
		return PassInstance{}, ErrTemplateInactive
	}

	id, err := walletID("wps_")
	if err != nil {
		return PassInstance{}, err
	}
	objectID := firstNonEmpty(strings.TrimSpace(token), strings.TrimSpace(uid), strings.TrimSpace(cardNumber))
	if objectID == "" {
		objectID = fmt.Sprintf("%s.%s", template.ClassID, id)
	}
	provider, credentialKind := normalizeCardCredentialProvider(cardType, token, uid, cardNumber, template.Provider)
	saveLink := ""
	if credentialKind == "google_wallet" {
		saveLink = fmt.Sprintf("https://pay.google.com/gp/v/save/%s", id)
	}
	record := PassInstance{
		ID:             id,
		TenantID:       nextTenantID,
		Provider:       provider,
		CredentialKind: credentialKind,
		TemplateID:     nextTemplateID,
		ObjectID:       objectID,
		Token:          strings.TrimSpace(token),
		UID:            strings.TrimSpace(uid),
		CardNumber:     strings.TrimSpace(cardNumber),
		Status:         "issued",
		SaveLink:       saveLink,
		ExpiresAt:      strings.TrimSpace(expiresAt),
		IssuedAt:       now,
		CreatedBy:      nextActor,
		UpdatedBy:      nextActor,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.passes = append([]PassInstance{record}, s.passes...)
	s.appendAuditLocked(nextTenantID, "wallet.card.create", nextActor, record.ID, "success")
	if err := s.persistLocked(); err != nil {
		return PassInstance{}, err
	}

	return record, nil
}

func (s *Service) EnrollApplePass(tenantID, userID, deviceID, passSerial, expiresAt, actor string) (PassInstance, error) {
	nextTargetID := strings.TrimSpace(userID)
	if nextTargetID == "" {
		return PassInstance{}, ErrTargetIDRequired
	}

	nextTenantID := normalizeTenantID(tenantID)
	nextDeviceID := strings.TrimSpace(deviceID)
	nextPassSerial := strings.TrimSpace(passSerial)
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.passes {
		if s.passes[i].TenantID != nextTenantID || s.passes[i].TargetType != "user" || s.passes[i].TargetID != nextTargetID {
			continue
		}
		if credentialKindForProvider(s.passes[i].Provider, s.passes[i].TargetType) != "apple_wallet" && strings.ToLower(strings.TrimSpace(s.passes[i].CredentialKind)) != "apple_wallet" {
			continue
		}
		if s.passes[i].Status == "revoked" {
			continue
		}
		if nextPassSerial != "" && s.passes[i].ObjectID != nextPassSerial {
			continue
		}
		if nextDeviceID != "" && s.passes[i].Token != nextDeviceID {
			continue
		}
		if nextPassSerial == "" && nextDeviceID == "" {
			return s.passes[i], nil
		}
		if nextPassSerial != "" || nextDeviceID != "" {
			return s.passes[i], nil
		}
	}

	id, err := walletID("wps_")
	if err != nil {
		return PassInstance{}, err
	}
	objectID := nextPassSerial
	if objectID == "" {
		objectID = fmt.Sprintf("apple.%s.%s", nextTargetID, id)
	}
	record := PassInstance{
		ID:             id,
		TenantID:       nextTenantID,
		Provider:       "apple",
		CredentialKind: "apple_wallet",
		TemplateID:     "wpt_apple_pass_self",
		TargetType:     "user",
		TargetID:       nextTargetID,
		ObjectID:       objectID,
		Token:          nextDeviceID,
		Status:         "active",
		SaveLink:       "",
		ExpiresAt:      strings.TrimSpace(expiresAt),
		IssuedAt:       now,
		ActivatedAt:    &now,
		CreatedBy:      nextActor,
		UpdatedBy:      nextActor,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.passes = append([]PassInstance{record}, s.passes...)
	s.appendAuditLocked(nextTenantID, "wallet.apple_pass.enroll", nextActor, record.ID, "success")
	if err := s.persistLocked(); err != nil {
		return PassInstance{}, err
	}

	return record, nil
}

func (s *Service) AssignPass(tenantID, passID, targetType, targetID, actor string) (PassInstance, error) {
	nextPassID := strings.TrimSpace(passID)
	if nextPassID == "" {
		return PassInstance{}, ErrPassNotFound
	}
	nextTargetType, err := normalizeTargetType(targetType)
	if err != nil {
		return PassInstance{}, err
	}
	nextTargetID := strings.TrimSpace(targetID)
	if nextTargetID == "" {
		return PassInstance{}, ErrTargetIDRequired
	}
	nextTenantID := normalizeTenantID(tenantID)
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.passes {
		if s.passes[i].ID != nextPassID {
			continue
		}
		if s.passes[i].TenantID != nextTenantID {
			return PassInstance{}, ErrPassNotFound
		}
		if s.passes[i].Status == "revoked" {
			return PassInstance{}, ErrInvalidPassTransition
		}
		s.passes[i].TargetType = nextTargetType
		s.passes[i].TargetID = nextTargetID
		s.passes[i].Status = "active"
		s.passes[i].UpdatedBy = nextActor
		s.passes[i].UpdatedAt = now
		value := now
		s.passes[i].ActivatedAt = &value
		s.passes[i].RevokedAt = nil

		s.appendAuditLocked(nextTenantID, "wallet.card.assign", nextActor, s.passes[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return PassInstance{}, err
		}

		return s.passes[i], nil
	}

	return PassInstance{}, ErrPassNotFound
}

func (s *Service) DeassignPass(tenantID, passID, actor string) (PassInstance, error) {
	nextPassID := strings.TrimSpace(passID)
	if nextPassID == "" {
		return PassInstance{}, ErrPassNotFound
	}
	nextTenantID := normalizeTenantID(tenantID)
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.passes {
		if s.passes[i].ID != nextPassID {
			continue
		}
		if s.passes[i].TenantID != nextTenantID {
			return PassInstance{}, ErrPassNotFound
		}
		if s.passes[i].Status == "revoked" {
			return PassInstance{}, ErrInvalidPassTransition
		}
		s.passes[i].TargetType = ""
		s.passes[i].TargetID = ""
		s.passes[i].Status = "issued"
		s.passes[i].UpdatedBy = nextActor
		s.passes[i].UpdatedAt = now
		s.passes[i].ActivatedAt = nil

		s.appendAuditLocked(nextTenantID, "wallet.card.deassign", nextActor, s.passes[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return PassInstance{}, err
		}

		return s.passes[i], nil
	}

	return PassInstance{}, ErrPassNotFound
}

func (s *Service) IssuePassBatch(tenantID, templateID, targetType string, targetIDs []string, expiresAt, actor string) ([]IssueJob, error) {
	nextTemplateID := strings.TrimSpace(templateID)
	if nextTemplateID == "" {
		return nil, ErrTemplateIDRequired
	}

	nextTargetType, err := normalizeTargetType(targetType)
	if err != nil {
		return nil, err
	}

	if len(targetIDs) == 0 {
		return nil, ErrTargetIDsRequired
	}

	batchID, err := walletID("wbt_")
	if err != nil {
		return nil, err
	}

	nextActor := normalizeActor(actor)
	nextTenantID := normalizeTenantID(tenantID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	template, found := findTemplateByID(s.templates, nextTemplateID)
	if !found {
		return nil, ErrTemplateNotFound
	}
	if template.TenantID != nextTenantID {
		return nil, ErrTemplateNotFound
	}
	if template.Status != "active" {
		return nil, ErrTemplateInactive
	}

	jobs := make([]IssueJob, 0, len(targetIDs))
	for _, rawTargetID := range targetIDs {
		targetID := strings.TrimSpace(rawTargetID)
		jobID, idErr := walletID("wjb_")
		if idErr != nil {
			return nil, idErr
		}

		job := IssueJob{
			ID:         jobID,
			TenantID:   nextTenantID,
			Provider:   firstNonEmpty(template.Provider, "google"),
			BatchID:    batchID,
			TemplateID: nextTemplateID,
			TargetType: nextTargetType,
			TargetID:   targetID,
			ExpiresAt:  strings.TrimSpace(expiresAt),
			Status:     "pending",
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		if targetID == "" {
			job.Status = "failed"
			job.ErrorCode = "target_id_required"
			job.ErrorMessage = ErrTargetIDRequired.Error()
			s.jobs = append([]IssueJob{job}, s.jobs...)
			jobs = append(jobs, job)
			continue
		}

		record, issueErr := s.createPassRecord(nextTenantID, template, nextTargetType, targetID, "", "", strings.TrimSpace(expiresAt), nextActor, now)
		if issueErr != nil {
			job.Status = "failed"
			job.ErrorCode = "issue_failed"
			job.ErrorMessage = issueErr.Error()
			s.jobs = append([]IssueJob{job}, s.jobs...)
			jobs = append(jobs, job)
			continue
		}

		s.passes = append([]PassInstance{record}, s.passes...)
		job.Status = "success"
		job.PassID = record.ID
		s.jobs = append([]IssueJob{job}, s.jobs...)
		jobs = append(jobs, job)
	}

	s.appendAuditLocked(nextTenantID, "wallet.pass.issue_batch", nextActor, batchID, "success")
	if err := s.persistLocked(); err != nil {
		return nil, err
	}

	return jobs, nil
}

func (s *Service) IssuePassBatchQueued(tenantID, templateID, targetType string, targetIDs []string, expiresAt, actor string) ([]IssueJob, error) {
	nextTemplateID := strings.TrimSpace(templateID)
	if nextTemplateID == "" {
		return nil, ErrTemplateIDRequired
	}

	nextTargetType, err := normalizeTargetType(targetType)
	if err != nil {
		return nil, err
	}

	if len(targetIDs) == 0 {
		return nil, ErrTargetIDsRequired
	}

	batchID, err := walletID("wbt_")
	if err != nil {
		return nil, err
	}

	nextActor := normalizeActor(actor)
	nextTenantID := normalizeTenantID(tenantID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	template, found := findTemplateByID(s.templates, nextTemplateID)
	if !found {
		return nil, ErrTemplateNotFound
	}
	if template.TenantID != nextTenantID {
		return nil, ErrTemplateNotFound
	}
	if template.Status != "active" {
		return nil, ErrTemplateInactive
	}

	jobs := make([]IssueJob, 0, len(targetIDs))
	for _, rawTargetID := range targetIDs {
		targetID := strings.TrimSpace(rawTargetID)
		jobID, idErr := walletID("wjb_")
		if idErr != nil {
			return nil, idErr
		}

		job := IssueJob{
			ID:         jobID,
			TenantID:   nextTenantID,
			Provider:   firstNonEmpty(template.Provider, "google"),
			BatchID:    batchID,
			TemplateID: nextTemplateID,
			TargetType: nextTargetType,
			TargetID:   targetID,
			ExpiresAt:  strings.TrimSpace(expiresAt),
			Status:     "pending",
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if targetID == "" {
			job.Status = "failed"
			job.ErrorCode = "target_id_required"
			job.ErrorMessage = ErrTargetIDRequired.Error()
		}

		s.jobs = append([]IssueJob{job}, s.jobs...)
		jobs = append(jobs, job)
	}

	s.appendAuditLocked(nextTenantID, "wallet.pass.issue_batch_queued", nextActor, batchID, "success")
	if err := s.persistLocked(); err != nil {
		return nil, err
	}

	return jobs, nil
}

func (s *Service) GetSaveLink(tenantID, passID string) (string, error) {
	record, err := s.GetPass(tenantID, passID)
	if err != nil {
		return "", err
	}
	return record.SaveLink, nil
}

func (s *Service) SuspendPass(tenantID, passID, actor string) (PassInstance, error) {
	return s.updatePassStatus(tenantID, passID, "suspended", actor)
}

func (s *Service) ActivatePass(tenantID, passID, actor string) (PassInstance, error) {
	return s.updatePassStatus(tenantID, passID, "active", actor)
}

func (s *Service) RevokePass(tenantID, passID, actor string) (PassInstance, error) {
	return s.updatePassStatus(tenantID, passID, "revoked", actor)
}

func (s *Service) ListJobs(tenantID string) []IssueJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	items := make([]IssueJob, 0, len(s.jobs))
	for i := range s.jobs {
		if s.jobs[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, s.jobs[i])
	}
	return items
}

func (s *Service) GetJob(tenantID, jobID string) (IssueJob, error) {
	nextJobID := strings.TrimSpace(jobID)
	if nextJobID == "" {
		return IssueJob{}, ErrJobNotFound
	}

	nextTenantID := normalizeTenantID(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.jobs {
		if s.jobs[i].ID == nextJobID {
			if s.jobs[i].TenantID != nextTenantID {
				return IssueJob{}, ErrJobNotFound
			}
			return s.jobs[i], nil
		}
	}

	return IssueJob{}, ErrJobNotFound
}

func (s *Service) RetryJob(tenantID, jobID, targetID, actor string) (IssueJob, error) {
	nextJobID := strings.TrimSpace(jobID)
	if nextJobID == "" {
		return IssueJob{}, ErrJobNotFound
	}

	nextTenantID := normalizeTenantID(tenantID)
	nextActor := normalizeActor(actor)
	overrideTargetID := strings.TrimSpace(targetID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if s.jobs[i].ID != nextJobID {
			continue
		}
		if s.jobs[i].TenantID != nextTenantID {
			return IssueJob{}, ErrJobNotFound
		}
		if s.jobs[i].Status == "success" {
			return IssueJob{}, ErrJobRetryNotAllowed
		}

		templateID := strings.TrimSpace(s.jobs[i].TemplateID)
		if templateID == "" {
			return IssueJob{}, ErrTemplateIDRequired
		}

		template, found := findTemplateByID(s.templates, templateID)
		if !found || template.TenantID != nextTenantID {
			return IssueJob{}, ErrTemplateNotFound
		}
		if template.Status != "active" {
			return IssueJob{}, ErrTemplateInactive
		}

		retryTargetID := strings.TrimSpace(s.jobs[i].TargetID)
		if overrideTargetID != "" {
			retryTargetID = overrideTargetID
		}

		s.jobs[i].RetryCount++
		s.jobs[i].UpdatedAt = now
		s.jobs[i].TargetID = retryTargetID
		s.jobs[i].ErrorCode = ""
		s.jobs[i].ErrorMessage = ""

		if retryTargetID == "" {
			s.jobs[i].Status = "failed"
			s.jobs[i].ErrorCode = "target_id_required"
			s.jobs[i].ErrorMessage = ErrTargetIDRequired.Error()
			s.appendAuditLocked(nextTenantID, "wallet.job.retry", nextActor, s.jobs[i].ID, "failed")
			if err := s.persistLocked(); err != nil {
				return IssueJob{}, err
			}
			return s.jobs[i], nil
		}

		record, err := s.createPassRecord(
			nextTenantID,
			template,
			s.jobs[i].TargetType,
			retryTargetID,
			"", "",
			s.jobs[i].ExpiresAt,
			nextActor,
			now,
		)
		if err != nil {
			s.jobs[i].Status = "failed"
			s.jobs[i].ErrorCode = "issue_failed"
			s.jobs[i].ErrorMessage = err.Error()
			s.appendAuditLocked(nextTenantID, "wallet.job.retry", nextActor, s.jobs[i].ID, "failed")
			if persistErr := s.persistLocked(); persistErr != nil {
				return IssueJob{}, persistErr
			}
			return s.jobs[i], nil
		}

		s.passes = append([]PassInstance{record}, s.passes...)
		s.jobs[i].PassID = record.ID
		s.jobs[i].Status = "success"
		s.jobs[i].ErrorCode = ""
		s.jobs[i].ErrorMessage = ""

		s.appendAuditLocked(nextTenantID, "wallet.job.retry", nextActor, s.jobs[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return IssueJob{}, err
		}
		return s.jobs[i], nil
	}

	return IssueJob{}, ErrJobNotFound
}

func (s *Service) RequeueDLQJob(tenantID, jobID, targetID, actor string) (IssueJob, error) {
	nextJobID := strings.TrimSpace(jobID)
	if nextJobID == "" {
		return IssueJob{}, ErrJobNotFound
	}

	nextTenantID := normalizeTenantID(tenantID)
	nextActor := normalizeActor(actor)
	overrideTargetID := strings.TrimSpace(targetID)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if s.jobs[i].ID != nextJobID {
			continue
		}
		if s.jobs[i].TenantID != nextTenantID {
			return IssueJob{}, ErrJobNotFound
		}
		if s.jobs[i].Status != "dlq" {
			return IssueJob{}, ErrJobNotInDLQ
		}

		if overrideTargetID != "" {
			s.jobs[i].TargetID = overrideTargetID
		}
		if strings.TrimSpace(s.jobs[i].TargetID) == "" {
			return IssueJob{}, ErrTargetIDRequired
		}

		s.jobs[i].Status = "pending"
		s.jobs[i].RetryCount = 0
		s.jobs[i].ErrorCode = ""
		s.jobs[i].ErrorMessage = ""
		s.jobs[i].UpdatedAt = now

		s.appendAuditLocked(nextTenantID, "wallet.job.dlq_requeue", nextActor, s.jobs[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return IssueJob{}, err
		}
		return s.jobs[i], nil
	}

	return IssueJob{}, ErrJobNotFound
}

func (s *Service) RequeueDLQJobs(options JobDLQRequeueOptions) (JobDLQRequeueResult, error) {
	resolvedOptions, err := normalizeJobDLQRequeueOptions(options)
	if err != nil {
		return JobDLQRequeueResult{}, err
	}

	result := JobDLQRequeueResult{
		TenantID:  resolvedOptions.TenantID,
		Limit:     resolvedOptions.Limit,
		UpdatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if result.Requeued >= resolvedOptions.Limit {
			break
		}
		if s.jobs[i].TenantID != resolvedOptions.TenantID {
			continue
		}
		if s.jobs[i].Status != "dlq" {
			continue
		}
		if resolvedOptions.ErrorCode != "" && strings.TrimSpace(s.jobs[i].ErrorCode) != resolvedOptions.ErrorCode {
			continue
		}

		targetID := strings.TrimSpace(s.jobs[i].TargetID)
		if resolvedOptions.TargetIDOverride != "" {
			targetID = resolvedOptions.TargetIDOverride
		}
		if targetID == "" {
			result.Skipped++
			result.ProcessedJobs = append(result.ProcessedJobs, s.jobs[i].ID)
			continue
		}

		s.jobs[i].TargetID = targetID
		s.jobs[i].Status = "pending"
		s.jobs[i].RetryCount = 0
		s.jobs[i].ErrorCode = ""
		s.jobs[i].ErrorMessage = ""
		s.jobs[i].UpdatedAt = time.Now().UTC()
		result.Requeued++
		result.ProcessedJobs = append(result.ProcessedJobs, s.jobs[i].ID)
		s.appendAuditLocked(s.jobs[i].TenantID, "wallet.job.dlq_requeue_batch", resolvedOptions.Actor, s.jobs[i].ID, "success")
	}

	result.RemainingDLQ = countDLQJobsByTenantLocked(s.jobs, resolvedOptions.TenantID)
	if err := s.persistLocked(); err != nil {
		return JobDLQRequeueResult{}, err
	}
	return result, nil
}

func (s *Service) CleanupDLQJobs(options JobDLQCleanupOptions) (JobDLQCleanupResult, error) {
	resolvedOptions, err := normalizeJobDLQCleanupOptions(options)
	if err != nil {
		return JobDLQCleanupResult{}, err
	}

	result := JobDLQCleanupResult{
		TenantID:  resolvedOptions.TenantID,
		Limit:     resolvedOptions.Limit,
		UpdatedAt: time.Now().UTC(),
	}

	cutoff := time.Now().UTC().Add(-resolvedOptions.OlderThan)

	s.mu.Lock()
	defer s.mu.Unlock()

	nextJobs := make([]IssueJob, 0, len(s.jobs))
	for i := range s.jobs {
		if result.Removed >= resolvedOptions.Limit {
			nextJobs = append(nextJobs, s.jobs[i])
			continue
		}
		if s.jobs[i].TenantID != resolvedOptions.TenantID {
			nextJobs = append(nextJobs, s.jobs[i])
			continue
		}
		if s.jobs[i].Status != "dlq" {
			nextJobs = append(nextJobs, s.jobs[i])
			continue
		}
		if resolvedOptions.ErrorCode != "" && strings.TrimSpace(s.jobs[i].ErrorCode) != resolvedOptions.ErrorCode {
			nextJobs = append(nextJobs, s.jobs[i])
			continue
		}
		if s.jobs[i].UpdatedAt.After(cutoff) {
			nextJobs = append(nextJobs, s.jobs[i])
			continue
		}

		result.Removed++
		result.ProcessedJobs = append(result.ProcessedJobs, s.jobs[i].ID)
		s.appendAuditLocked(s.jobs[i].TenantID, "wallet.job.dlq_cleanup", resolvedOptions.Actor, s.jobs[i].ID, "success")
	}

	s.jobs = nextJobs
	result.RemainingDLQ = countDLQJobsByTenantLocked(s.jobs, resolvedOptions.TenantID)
	s.appendDLQCleanupArchiveLocked(resolvedOptions, result)
	if err := s.persistLocked(); err != nil {
		return JobDLQCleanupResult{}, err
	}
	return result, nil
}

func (s *Service) GetJobSummary(tenantID string, maxRetry int) JobSummary {
	nextTenantID := normalizeTenantID(tenantID)
	if maxRetry <= 0 {
		maxRetry = 3
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return buildJobSummary(nextTenantID, maxRetry, s.jobs, time.Now().UTC())
}

func (s *Service) GetJobMetrics(tenantID string, window time.Duration, maxRetry, dlqAlertThreshold int) JobMetrics {
	nextTenantID := normalizeTenantID(tenantID)
	if maxRetry <= 0 {
		maxRetry = 3
	}
	if window < time.Second {
		window = 15 * time.Minute
	}
	if dlqAlertThreshold <= 0 {
		dlqAlertThreshold = 20
	}

	now := time.Now().UTC()
	since := now.Add(-window)

	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := buildJobSummary(nextTenantID, maxRetry, s.jobs, now)
	metrics := JobMetrics{
		TenantID:          nextTenantID,
		MaxRetry:          maxRetry,
		DLQAlertThreshold: dlqAlertThreshold,
		Summary:           summary,
		Window: JobMetricsWindow{
			WindowSeconds:      int64(window.Seconds()),
			Since:              since,
			Until:              now,
			ErrorCodeBreakdown: map[string]int{},
		},
		UpdatedAt: now,
	}

	dlqErrorCodeBreakdown := map[string]int{}
	for i := range s.jobs {
		if s.jobs[i].TenantID != nextTenantID {
			continue
		}
		if s.jobs[i].Status == "dlq" {
			errorCode := strings.TrimSpace(s.jobs[i].ErrorCode)
			if errorCode == "" {
				errorCode = "unknown"
			}
			dlqErrorCodeBreakdown[errorCode]++
		}

		updatedAt := s.jobs[i].UpdatedAt
		if updatedAt.Before(since) || updatedAt.After(now) {
			continue
		}
		metrics.Window.Updated++
		switch s.jobs[i].Status {
		case "pending":
			metrics.Window.Pending++
		case "processing":
			metrics.Window.Processing++
		case "success":
			metrics.Window.Success++
		case "failed":
			metrics.Window.Failed++
		case "dlq":
			metrics.Window.DLQ++
		}

		errorCode := strings.TrimSpace(s.jobs[i].ErrorCode)
		if errorCode != "" {
			metrics.Window.ErrorCodeBreakdown[errorCode]++
		}
	}

	for i := range s.jobs {
		if s.jobs[i].TenantID != nextTenantID {
			continue
		}
		createdAt := s.jobs[i].CreatedAt
		if createdAt.Before(since) || createdAt.After(now) {
			continue
		}
		metrics.Window.Created++
	}

	if len(metrics.Window.ErrorCodeBreakdown) == 0 {
		metrics.Window.ErrorCodeBreakdown = nil
	}

	metrics.Alerts = buildJobMetricsAlerts(dlqErrorCodeBreakdown, dlqAlertThreshold)
	return metrics
}

func (s *Service) GetJobMetricsTrend(
	tenantID string,
	window time.Duration,
	bucketCount, maxRetry, dlqAlertThreshold int,
) JobMetricsTrend {
	nextTenantID := normalizeTenantID(tenantID)
	if maxRetry <= 0 {
		maxRetry = 3
	}
	if window < time.Second {
		window = 15 * time.Minute
	}
	if dlqAlertThreshold <= 0 {
		dlqAlertThreshold = 20
	}
	if bucketCount <= 0 {
		bucketCount = 12
	}
	if bucketCount > 120 {
		bucketCount = 120
	}
	if maxBucketsBySeconds := int(window.Seconds()); maxBucketsBySeconds > 0 && bucketCount > maxBucketsBySeconds {
		bucketCount = maxBucketsBySeconds
	}
	if bucketCount <= 0 {
		bucketCount = 1
	}

	now := time.Now().UTC()
	since := now.Add(-window)
	windowNS := window.Nanoseconds()
	bucketSeconds := window.Seconds() / float64(bucketCount)

	s.mu.RLock()
	defer s.mu.RUnlock()

	trend := JobMetricsTrend{
		TenantID:          nextTenantID,
		MaxRetry:          maxRetry,
		DLQAlertThreshold: dlqAlertThreshold,
		WindowSeconds:     int64(window.Seconds()),
		BucketSeconds:     int64(bucketSeconds),
		BucketCount:       bucketCount,
		Since:             since,
		Until:             now,
		Summary:           buildJobSummary(nextTenantID, maxRetry, s.jobs, now),
		Buckets:           make([]JobMetricsTrendBucket, bucketCount),
		UpdatedAt:         now,
	}
	if trend.BucketSeconds < 1 {
		trend.BucketSeconds = 1
	}

	for i := range trend.Buckets {
		startNS := windowNS * int64(i) / int64(bucketCount)
		endNS := windowNS * int64(i+1) / int64(bucketCount)
		end := since.Add(time.Duration(endNS))
		if i == bucketCount-1 {
			end = now
		}
		trend.Buckets[i] = JobMetricsTrendBucket{
			Index:              i,
			Start:              since.Add(time.Duration(startNS)),
			End:                end,
			ErrorCodeBreakdown: map[string]int{},
		}
	}

	dlqErrorCodeBreakdown := map[string]int{}
	for i := range s.jobs {
		if s.jobs[i].TenantID != nextTenantID {
			continue
		}

		if s.jobs[i].Status == "dlq" {
			errorCode := strings.TrimSpace(s.jobs[i].ErrorCode)
			if errorCode == "" {
				errorCode = "unknown"
			}
			dlqErrorCodeBreakdown[errorCode]++
		}

		if idx, ok := resolveMetricsTrendBucketIndex(s.jobs[i].UpdatedAt, since, now, bucketCount, windowNS); ok {
			trend.Buckets[idx].Updated++
			switch s.jobs[i].Status {
			case "pending":
				trend.Buckets[idx].Pending++
			case "processing":
				trend.Buckets[idx].Processing++
			case "success":
				trend.Buckets[idx].Success++
			case "failed":
				trend.Buckets[idx].Failed++
			case "dlq":
				trend.Buckets[idx].DLQ++
			}
			errorCode := strings.TrimSpace(s.jobs[i].ErrorCode)
			if errorCode != "" {
				trend.Buckets[idx].ErrorCodeBreakdown[errorCode]++
			}
		}

		if idx, ok := resolveMetricsTrendBucketIndex(s.jobs[i].CreatedAt, since, now, bucketCount, windowNS); ok {
			trend.Buckets[idx].Created++
		}
	}

	for i := range trend.Buckets {
		if len(trend.Buckets[i].ErrorCodeBreakdown) == 0 {
			trend.Buckets[i].ErrorCodeBreakdown = nil
		}
	}
	trend.Alerts = buildJobMetricsAlerts(dlqErrorCodeBreakdown, dlqAlertThreshold)
	return trend
}

func resolveMetricsTrendBucketIndex(
	timestamp, since, until time.Time,
	bucketCount int,
	windowNS int64,
) (int, bool) {
	if timestamp.Before(since) || timestamp.After(until) || bucketCount <= 0 || windowNS <= 0 {
		return 0, false
	}
	offset := timestamp.Sub(since).Nanoseconds()
	index := int(offset * int64(bucketCount) / windowNS)
	if index < 0 {
		return 0, false
	}
	if index >= bucketCount {
		index = bucketCount - 1
	}
	return index, true
}

func buildJobSummary(tenantID string, maxRetry int, jobs []IssueJob, now time.Time) JobSummary {
	summary := JobSummary{
		TenantID:           tenantID,
		MaxRetry:           maxRetry,
		ErrorCodeBreakdown: map[string]int{},
		UpdatedAt:          now,
	}

	for i := range jobs {
		if jobs[i].TenantID != tenantID {
			continue
		}
		summary.Total++
		switch jobs[i].Status {
		case "pending":
			summary.Pending++
		case "processing":
			summary.Processing++
		case "success":
			summary.Success++
		case "failed":
			summary.Failed++
		case "dlq":
			summary.DLQ++
		}

		if jobs[i].Status == "failed" {
			if isAutoRetryableJob(jobs[i], maxRetry) {
				summary.RetryableFailed++
			} else {
				summary.NonRetryableFailed++
			}
		}
		if jobs[i].Status == "dlq" {
			summary.NonRetryableFailed++
		}

		errorCode := strings.TrimSpace(jobs[i].ErrorCode)
		if errorCode != "" {
			summary.ErrorCodeBreakdown[errorCode]++
		}
	}
	if len(summary.ErrorCodeBreakdown) == 0 {
		summary.ErrorCodeBreakdown = nil
	}
	return summary
}

func buildJobMetricsAlerts(dlqErrorCodeBreakdown map[string]int, threshold int) []JobMetricsAlert {
	items := make([]JobMetricsAlert, 0, len(dlqErrorCodeBreakdown))
	for errorCode, count := range dlqErrorCodeBreakdown {
		if count < threshold {
			continue
		}
		items = append(items, JobMetricsAlert{
			Type:      "dlq_error_code_threshold",
			ErrorCode: errorCode,
			Count:     count,
			Threshold: threshold,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].ErrorCode < items[j].ErrorCode
		}
		return items[i].Count > items[j].Count
	})
	return items
}

type jobProcessOutcome struct {
	jobID   string
	status  string
	retried bool
}

func (s *Service) ProcessIssueJobs(options JobProcessOptions) (JobProcessResult, error) {
	resolvedOptions, err := normalizeJobProcessOptions(options)
	if err != nil {
		return JobProcessResult{}, err
	}

	result := JobProcessResult{
		TenantID:    resolvedOptions.TenantID,
		Limit:       resolvedOptions.Limit,
		WorkerCount: resolvedOptions.WorkerCount,
		MaxRetry:    resolvedOptions.MaxRetry,
		StartedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	claimedJobIDs := s.claimProcessableJobsLocked(resolvedOptions.TenantID, resolvedOptions.Limit, resolvedOptions.MaxRetry)
	s.mu.Unlock()

	result.Claimed = len(claimedJobIDs)
	if len(claimedJobIDs) == 0 {
		result.PendingAfter = s.countProcessableJobs(resolvedOptions.TenantID, resolvedOptions.MaxRetry)
		result.CompletedAt = time.Now().UTC()
		return result, nil
	}

	workerCount := resolvedOptions.WorkerCount
	if workerCount > len(claimedJobIDs) {
		workerCount = len(claimedJobIDs)
	}

	jobCh := make(chan string, len(claimedJobIDs))
	outcomeCh := make(chan jobProcessOutcome, len(claimedJobIDs))

	for i := range claimedJobIDs {
		jobCh <- claimedJobIDs[i]
	}
	close(jobCh)

	var wg sync.WaitGroup
	wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			for jobID := range jobCh {
				outcome := s.processClaimedIssueJob(jobID, resolvedOptions)
				outcomeCh <- outcome
			}
		}()
	}
	wg.Wait()
	close(outcomeCh)

	for outcome := range outcomeCh {
		result.ProcessedJobIDs = append(result.ProcessedJobIDs, outcome.jobID)
		if outcome.retried {
			result.Retried++
		}
		switch outcome.status {
		case "success":
			result.Succeeded++
		case "failed":
			result.Failed++
		case "dlq":
			result.DLQ++
		default:
			result.Skipped++
		}
	}

	result.PendingAfter = s.countProcessableJobs(resolvedOptions.TenantID, resolvedOptions.MaxRetry)
	result.CompletedAt = time.Now().UTC()
	return result, nil
}

func normalizeJobDLQRequeueOptions(input JobDLQRequeueOptions) (JobDLQRequeueOptions, error) {
	output := JobDLQRequeueOptions{
		TenantID:         normalizeTenantID(input.TenantID),
		Limit:            input.Limit,
		ErrorCode:        strings.TrimSpace(input.ErrorCode),
		TargetIDOverride: strings.TrimSpace(input.TargetIDOverride),
		Actor:            normalizeActor(input.Actor),
	}

	if output.Limit <= 0 {
		output.Limit = 20
	}
	if output.Limit > 500 {
		return JobDLQRequeueOptions{}, ErrInvalidJobDLQOptions
	}

	return output, nil
}

func normalizeJobDLQCleanupOptions(input JobDLQCleanupOptions) (JobDLQCleanupOptions, error) {
	output := JobDLQCleanupOptions{
		TenantID:  normalizeTenantID(input.TenantID),
		Limit:     input.Limit,
		ErrorCode: strings.TrimSpace(input.ErrorCode),
		OlderThan: input.OlderThan,
		Actor:     normalizeActor(input.Actor),
	}

	if output.Limit <= 0 {
		output.Limit = 50
	}
	if output.Limit > 1000 {
		return JobDLQCleanupOptions{}, ErrInvalidJobDLQOptions
	}
	if output.OlderThan <= 0 {
		output.OlderThan = 24 * time.Hour
	}
	if output.OlderThan > 365*24*time.Hour {
		return JobDLQCleanupOptions{}, ErrInvalidJobDLQOptions
	}

	return output, nil
}

func normalizeJobProcessOptions(input JobProcessOptions) (JobProcessOptions, error) {
	output := JobProcessOptions{
		TenantID:    normalizeTenantID(input.TenantID),
		Limit:       input.Limit,
		WorkerCount: input.WorkerCount,
		MaxRetry:    input.MaxRetry,
		BaseBackoff: input.BaseBackoff,
		MaxBackoff:  input.MaxBackoff,
		Actor:       normalizeActor(input.Actor),
	}

	if output.Limit <= 0 {
		output.Limit = 20
	}
	if output.Limit > 500 {
		return JobProcessOptions{}, ErrInvalidJobProcessOptions
	}

	if output.WorkerCount <= 0 {
		output.WorkerCount = 2
	}
	if output.WorkerCount > 32 {
		return JobProcessOptions{}, ErrInvalidJobProcessOptions
	}

	if output.MaxRetry < 0 {
		return JobProcessOptions{}, ErrInvalidJobProcessOptions
	}
	if output.MaxRetry == 0 {
		output.MaxRetry = 3
	}
	if output.MaxRetry > 20 {
		return JobProcessOptions{}, ErrInvalidJobProcessOptions
	}

	if output.BaseBackoff < 0 || output.MaxBackoff < 0 {
		return JobProcessOptions{}, ErrInvalidJobProcessOptions
	}
	if output.BaseBackoff == 0 {
		output.BaseBackoff = 200 * time.Millisecond
	}
	if output.MaxBackoff == 0 {
		output.MaxBackoff = 5 * time.Second
	}
	if output.MaxBackoff < output.BaseBackoff {
		return JobProcessOptions{}, ErrInvalidJobProcessOptions
	}

	return output, nil
}

func (s *Service) claimProcessableJobsLocked(tenantID string, limit, maxRetry int) []string {
	jobIDs := make([]string, 0, limit)
	now := time.Now().UTC()
	for i := range s.jobs {
		if len(jobIDs) >= limit {
			break
		}
		if s.jobs[i].TenantID != tenantID {
			continue
		}
		if s.jobs[i].Status == "pending" {
			s.jobs[i].Status = "processing"
			s.jobs[i].UpdatedAt = now
			s.jobs[i].ErrorCode = ""
			s.jobs[i].ErrorMessage = ""
			jobIDs = append(jobIDs, s.jobs[i].ID)
			continue
		}
		if !isAutoRetryableJob(s.jobs[i], maxRetry) {
			continue
		}
		s.jobs[i].RetryCount++
		s.jobs[i].Status = "processing"
		s.jobs[i].UpdatedAt = now
		s.jobs[i].ErrorCode = ""
		s.jobs[i].ErrorMessage = ""
		jobIDs = append(jobIDs, s.jobs[i].ID)
	}
	return jobIDs
}

func (s *Service) countProcessableJobs(tenantID string, maxRetry int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for i := range s.jobs {
		if s.jobs[i].TenantID != tenantID {
			continue
		}
		if s.jobs[i].Status == "pending" || isAutoRetryableJob(s.jobs[i], maxRetry) {
			count++
		}
	}
	return count
}

func isAutoRetryableJob(job IssueJob, maxRetry int) bool {
	return job.Status == "failed" &&
		strings.TrimSpace(job.ErrorCode) == "issue_failed" &&
		job.RetryCount < maxRetry
}

func (s *Service) processClaimedIssueJob(jobID string, options JobProcessOptions) jobProcessOutcome {
	s.mu.RLock()
	jobIndex := -1
	for i := range s.jobs {
		if s.jobs[i].ID == jobID {
			jobIndex = i
			break
		}
	}
	if jobIndex < 0 {
		s.mu.RUnlock()
		return jobProcessOutcome{jobID: jobID, status: "skipped"}
	}
	job := s.jobs[jobIndex]
	s.mu.RUnlock()

	if job.Status != "processing" {
		return jobProcessOutcome{jobID: jobID, status: "skipped"}
	}

	retried := job.RetryCount > 0
	if retried {
		sleepDuration := exponentialBackoff(job.RetryCount, options.BaseBackoff, options.MaxBackoff)
		if sleepDuration > 0 {
			time.Sleep(sleepDuration)
		}
	}

	if strings.TrimSpace(job.TargetID) == "" {
		status := s.setJobFailure(jobID, "target_id_required", ErrTargetIDRequired.Error(), options.Actor, false, options.MaxRetry)
		return jobProcessOutcome{jobID: jobID, status: status, retried: retried}
	}

	s.mu.RLock()
	template, found := findTemplateByID(s.templates, strings.TrimSpace(job.TemplateID))
	s.mu.RUnlock()
	if !found || template.TenantID != options.TenantID {
		status := s.setJobFailure(jobID, "template_not_found", ErrTemplateNotFound.Error(), options.Actor, false, options.MaxRetry)
		return jobProcessOutcome{jobID: jobID, status: status, retried: retried}
	}
	if template.Status != "active" {
		status := s.setJobFailure(jobID, "template_inactive", ErrTemplateInactive.Error(), options.Actor, false, options.MaxRetry)
		return jobProcessOutcome{jobID: jobID, status: status, retried: retried}
	}

	record, err := s.createPassRecord(
		options.TenantID,
		template,
		job.TargetType,
		job.TargetID,
		"", "",
		job.ExpiresAt,
		options.Actor,
		time.Now().UTC(),
	)
	if err != nil {
		status := s.setJobFailure(jobID, "issue_failed", err.Error(), options.Actor, true, options.MaxRetry)
		return jobProcessOutcome{jobID: jobID, status: status, retried: retried}
	}

	if updateErr := s.setJobSuccess(jobID, record, options.Actor); updateErr != nil {
		return jobProcessOutcome{jobID: jobID, status: "failed", retried: retried}
	}

	return jobProcessOutcome{jobID: jobID, status: "success", retried: retried}
}

func (s *Service) setJobFailure(jobID, errorCode, errorMessage, actor string, retryable bool, maxRetry int) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if s.jobs[i].ID != jobID {
			continue
		}
		s.jobs[i].Status = resolveJobFailureStatus(s.jobs[i], strings.TrimSpace(errorCode), retryable, maxRetry)
		s.jobs[i].ErrorCode = strings.TrimSpace(errorCode)
		s.jobs[i].ErrorMessage = strings.TrimSpace(errorMessage)
		s.jobs[i].UpdatedAt = time.Now().UTC()
		auditResult := "failed"
		if s.jobs[i].Status == "dlq" {
			auditResult = "dlq"
		}
		s.appendAuditLocked(s.jobs[i].TenantID, "wallet.job.process", normalizeActor(actor), s.jobs[i].ID, auditResult)
		_ = s.persistLocked()
		return s.jobs[i].Status
	}
	return "skipped"
}

func resolveJobFailureStatus(job IssueJob, errorCode string, retryable bool, maxRetry int) string {
	if !retryable {
		return "dlq"
	}
	if strings.TrimSpace(errorCode) != "issue_failed" {
		return "dlq"
	}
	if maxRetry <= 0 {
		maxRetry = 3
	}
	if job.RetryCount >= maxRetry {
		return "dlq"
	}
	return "failed"
}

func countDLQJobsByTenantLocked(items []IssueJob, tenantID string) int {
	count := 0
	for i := range items {
		if items[i].TenantID != tenantID {
			continue
		}
		if items[i].Status == "dlq" {
			count++
		}
	}
	return count
}

func (s *Service) setJobSuccess(jobID string, record PassInstance, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.jobs {
		if s.jobs[i].ID != jobID {
			continue
		}
		if s.jobs[i].Status != "processing" {
			return nil
		}
		s.passes = append([]PassInstance{record}, s.passes...)
		s.jobs[i].PassID = record.ID
		s.jobs[i].Status = "success"
		s.jobs[i].ErrorCode = ""
		s.jobs[i].ErrorMessage = ""
		s.jobs[i].UpdatedAt = time.Now().UTC()
		s.appendAuditLocked(s.jobs[i].TenantID, "wallet.job.process", normalizeActor(actor), s.jobs[i].ID, "success")
		return s.persistLocked()
	}
	return ErrJobNotFound
}

func exponentialBackoff(retryCount int, base, max time.Duration) time.Duration {
	if retryCount <= 0 || base <= 0 {
		return 0
	}
	delay := base
	for i := 1; i < retryCount; i++ {
		delay *= 2
		if delay >= max {
			return max
		}
	}
	if delay > max {
		return max
	}
	return delay
}

func (s *Service) ListAuditLogs(tenantID string) []AuditLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	items := make([]AuditLog, 0, len(s.auditLogs))
	for i := range s.auditLogs {
		if s.auditLogs[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, s.auditLogs[i])
	}
	return items
}

func (s *Service) ListDLQCleanupArchives(tenantID string, limit int) []JobDLQCleanupArchive {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	if limit <= 0 {
		limit = 20
	}
	if limit > 500 {
		limit = 500
	}

	items := make([]JobDLQCleanupArchive, 0, limit)
	for i := range s.dlqCleanupArchives {
		if s.dlqCleanupArchives[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, cloneDLQCleanupArchive(s.dlqCleanupArchives[i]))
		if len(items) >= limit {
			break
		}
	}
	return items
}

func (s *Service) GetJobAlertSubscription(tenantID string) (JobAlertSubscription, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	for i := range s.jobAlertSubscriptions {
		if s.jobAlertSubscriptions[i].TenantID != nextTenantID {
			continue
		}
		return cloneJobAlertSubscription(s.jobAlertSubscriptions[i]), true
	}
	return JobAlertSubscription{}, false
}

func (s *Service) UpsertJobAlertSubscription(
	input JobAlertSubscriptionUpsertOptions,
) (JobAlertSubscription, error) {
	resolved, err := resolveJobAlertSubscriptionUpsertOptions(input)
	if err != nil {
		return JobAlertSubscription{}, err
	}

	now := time.Now().UTC()
	record := JobAlertSubscription{
		TenantID:          resolved.TenantID,
		Enabled:           resolved.Enabled,
		DLQAlertThreshold: resolved.DLQAlertThreshold,
		WindowSeconds:     int64(resolved.Window.Seconds()),
		CooldownSeconds:   int64(resolved.Cooldown.Seconds()),
		Channels: JobAlertSubscriptionChannels{
			Email:    resolved.EmailEnabled,
			WhatsApp: resolved.WhatsAppEnabled,
		},
		ReceiverGroups: normalizeReceiverGroups(resolved.ReceiverGroups),
		UpdatedAt:      now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	upserted := false
	for i := range s.jobAlertSubscriptions {
		if s.jobAlertSubscriptions[i].TenantID != resolved.TenantID {
			continue
		}
		s.jobAlertSubscriptions[i] = cloneJobAlertSubscription(record)
		upserted = true
		break
	}
	if !upserted {
		s.jobAlertSubscriptions = append(
			[]JobAlertSubscription{cloneJobAlertSubscription(record)},
			s.jobAlertSubscriptions...,
		)
	}
	s.appendAuditLocked(
		resolved.TenantID,
		"wallet.job.alert_subscription.upsert",
		normalizeActor(resolved.Actor),
		resolved.TenantID,
		"success",
	)
	if err := s.persistLocked(); err != nil {
		return JobAlertSubscription{}, err
	}
	return cloneJobAlertSubscription(record), nil
}

func (s *Service) ListJobAlertNotifications(tenantID string, limit int) []JobAlertNotification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nextTenantID := normalizeTenantID(tenantID)
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	items := make([]JobAlertNotification, 0, limit)
	for i := range s.jobAlertNotifications {
		if s.jobAlertNotifications[i].TenantID != nextTenantID {
			continue
		}
		items = append(items, cloneJobAlertNotification(s.jobAlertNotifications[i]))
		if len(items) >= limit {
			break
		}
	}
	return items
}

func (s *Service) DispatchJobMetricsAlerts(
	subscription JobAlertSubscription,
	alerts []JobMetricsAlert,
	actor string,
) (JobAlertDispatchResult, error) {
	now := time.Now().UTC()
	nextTenantID := normalizeTenantID(subscription.TenantID)
	cooldown := time.Duration(subscription.CooldownSeconds) * time.Second
	if cooldown < 0 {
		cooldown = 0
	}
	nextActor := normalizeActor(actor)
	channels := resolveAlertChannels(subscription.Channels)
	receiverGroups := normalizeReceiverGroups(subscription.ReceiverGroups)
	if len(receiverGroups) == 0 {
		receiverGroups = []string{"security"}
	}

	result := JobAlertDispatchResult{
		TenantID:    nextTenantID,
		TotalAlerts: len(alerts),
		UpdatedAt:   now,
	}
	if len(alerts) == 0 {
		return result, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result.Items = make([]JobAlertNotification, 0, len(alerts))
	for i := range alerts {
		errorCode := alertdispatch.NormalizeErrorCode(alerts[i].ErrorCode)
		planned := alertdispatch.Plan(alertdispatch.PlanInput{
			Subscription: alertdispatch.Subscription{
				TenantID:       nextTenantID,
				Enabled:        subscription.Enabled,
				Channels:       channels,
				ReceiverGroups: receiverGroups,
			},
			Alert: alertdispatch.Alert{
				Type:      alerts[i].Type,
				ErrorCode: alerts[i].ErrorCode,
				Count:     alerts[i].Count,
				Threshold: alerts[i].Threshold,
			},
			InCooldown: s.isJobAlertInCooldownLocked(nextTenantID, errorCode, cooldown, now),
		})

		record := JobAlertNotification{
			TenantID:       planned.TenantID,
			Type:           planned.Type,
			ErrorCode:      planned.ErrorCode,
			Count:          planned.Count,
			Threshold:      planned.Threshold,
			Channels:       append([]string(nil), planned.Channels...),
			ReceiverGroups: append([]string(nil), planned.ReceiverGroups...),
			Status:         planned.Status,
			Reason:         planned.Reason,
			IdempotencyKey: planned.IdempotencyKey,
			Provider:       s.jobAlertEmailProvider,
			TriggeredAt:    now,
		}
		if record.Status == "ready" {
			attempt := s.nextJobAlertAttemptLocked(nextTenantID, planned.IdempotencyKey)
			record.Attempt = attempt
			status, reason, provider, providerError, retryable, channelResults := s.dispatchJobAlertWithProviderLocked(record, attempt)
			record.Status = status
			record.Reason = reason
			record.Provider = provider
			record.ProviderError = providerError
			record.Retryable = retryable
			record.ChannelResults = cloneJobAlertChannelResults(channelResults)
		} else {
			record.ChannelResults = buildStaticJobAlertChannelResults(record.Channels, record.Status, record.Reason)
		}

		id, err := walletID("wan_")
		if err != nil {
			id = fmt.Sprintf("wan_fallback_%d", time.Now().UnixNano())
		}
		record.ID = id

		if record.Status == "sent" {
			result.Dispatched++
			s.upsertJobAlertCooldownLocked(nextTenantID, errorCode, now)
			s.appendAuditLocked(nextTenantID, "wallet.job.alert.dispatch", nextActor, errorCode, "sent")
		} else if record.Status == "failed" {
			result.Failed++
			auditResult := "failed"
			if record.Reason != "" {
				auditResult = "failed:" + record.Reason
			}
			s.appendAuditLocked(nextTenantID, "wallet.job.alert.dispatch", nextActor, errorCode, auditResult)
		} else {
			result.Skipped++
			auditResult := "skipped"
			if record.Reason != "" {
				auditResult = "skipped:" + record.Reason
			}
			s.appendAuditLocked(nextTenantID, "wallet.job.alert.dispatch", nextActor, errorCode, auditResult)
		}

		result.Items = append(result.Items, cloneJobAlertNotification(record))
	}

	s.jobAlertNotifications = append(cloneJobAlertNotifications(result.Items), s.jobAlertNotifications...)
	if len(s.jobAlertNotifications) > 5000 {
		s.jobAlertNotifications = s.jobAlertNotifications[:5000]
	}

	if err := s.persistLocked(); err != nil {
		return JobAlertDispatchResult{}, err
	}
	return result, nil
}

func (s *Service) RetryJobAlertNotification(tenantID, notificationID, actor string) (JobAlertNotification, error) {
	nextTenantID := normalizeTenantID(tenantID)
	nextNotificationID := strings.TrimSpace(notificationID)
	if nextNotificationID == "" {
		return JobAlertNotification{}, ErrJobAlertNotificationNotFound
	}
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	var source JobAlertNotification
	found := false
	for i := range s.jobAlertNotifications {
		if s.jobAlertNotifications[i].TenantID != nextTenantID {
			continue
		}
		if s.jobAlertNotifications[i].ID != nextNotificationID {
			continue
		}
		source = cloneJobAlertNotification(s.jobAlertNotifications[i])
		found = true
		break
	}
	if !found {
		return JobAlertNotification{}, ErrJobAlertNotificationNotFound
	}
	if source.Status != "failed" || !source.Retryable {
		return JobAlertNotification{}, ErrJobAlertRetryNotAllowed
	}

	idempotencyKey := strings.TrimSpace(source.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = buildJobAlertNotificationIdempotencyKey(
			nextTenantID,
			strings.TrimSpace(source.Type),
			strings.TrimSpace(source.ErrorCode),
			source.Threshold,
		)
	}

	record := cloneJobAlertNotification(source)
	id, err := walletID("wan_")
	if err != nil {
		id = fmt.Sprintf("wan_fallback_%d", time.Now().UnixNano())
	}
	record.ID = id
	record.IdempotencyKey = idempotencyKey
	record.SourceNotificationID = source.ID
	record.TriggeredAt = now
	record.Provider = s.jobAlertEmailProvider
	record.ProviderError = ""
	record.Reason = ""
	record.Retryable = false

	if s.hasSentJobAlertByIdempotencyLocked(nextTenantID, idempotencyKey) {
		record.Status = "skipped"
		record.Reason = "idempotent_already_sent"
		record.ChannelResults = buildStaticJobAlertChannelResults(record.Channels, record.Status, record.Reason)
		auditResult := "skipped:" + record.Reason
		s.appendAuditLocked(nextTenantID, "wallet.job.alert.retry", nextActor, source.ID, auditResult)
	} else {
		attempt := s.nextJobAlertAttemptLocked(nextTenantID, idempotencyKey)
		record.Attempt = attempt
		status, reason, provider, providerError, retryable, channelResults := s.dispatchJobAlertWithProviderLocked(record, attempt)
		record.Status = status
		record.Reason = reason
		record.Provider = provider
		record.ProviderError = providerError
		record.Retryable = retryable
		record.ChannelResults = cloneJobAlertChannelResults(channelResults)
		if record.Status == "sent" {
			if record.Reason == "" {
				record.ProviderError = ""
			}
			s.upsertJobAlertCooldownLocked(nextTenantID, record.ErrorCode, now)
			s.appendAuditLocked(nextTenantID, "wallet.job.alert.retry", nextActor, source.ID, "sent")
		} else {
			auditResult := "failed"
			if record.Reason != "" {
				auditResult = "failed:" + record.Reason
			}
			s.appendAuditLocked(nextTenantID, "wallet.job.alert.retry", nextActor, source.ID, auditResult)
		}
	}

	s.jobAlertNotifications = append([]JobAlertNotification{cloneJobAlertNotification(record)}, s.jobAlertNotifications...)
	if len(s.jobAlertNotifications) > 5000 {
		s.jobAlertNotifications = s.jobAlertNotifications[:5000]
	}

	if err := s.persistLocked(); err != nil {
		return JobAlertNotification{}, err
	}
	return cloneJobAlertNotification(record), nil
}

func (s *Service) nextJobAlertAttemptLocked(tenantID, idempotencyKey string) int {
	nextAttempt := 1
	for i := range s.jobAlertNotifications {
		if s.jobAlertNotifications[i].TenantID != tenantID {
			continue
		}
		if s.jobAlertNotifications[i].IdempotencyKey != idempotencyKey {
			continue
		}
		if s.jobAlertNotifications[i].Attempt >= nextAttempt {
			nextAttempt = s.jobAlertNotifications[i].Attempt + 1
		}
	}
	return nextAttempt
}

func (s *Service) hasSentJobAlertByIdempotencyLocked(tenantID, idempotencyKey string) bool {
	if idempotencyKey == "" {
		return false
	}
	for i := range s.jobAlertNotifications {
		if s.jobAlertNotifications[i].TenantID != tenantID {
			continue
		}
		if s.jobAlertNotifications[i].IdempotencyKey != idempotencyKey {
			continue
		}
		if s.jobAlertNotifications[i].Status == "sent" {
			return true
		}
	}
	return false
}

func (s *Service) dispatchJobAlertWithProviderLocked(
	record JobAlertNotification,
	attempt int,
) (status, reason, provider, providerError string, retryable bool, channelResults []JobAlertChannelResult) {
	if attempt < 1 {
		attempt = 1
	}
	normalizedChannels := normalizeDispatchChannels(record.Channels)
	if len(normalizedChannels) == 0 {
		return "skipped", "channel_disabled", "", "", false, nil
	}

	channelResults = make([]JobAlertChannelResult, 0, len(normalizedChannels))
	for i := range normalizedChannels {
		switch normalizedChannels[i] {
		case "email":
			channelResults = append(channelResults, s.dispatchJobAlertEmailChannelLocked(record, attempt))
		case "whatsapp":
			channelResults = append(channelResults, s.dispatchJobAlertWhatsAppChannelLocked(record))
		default:
			channelResults = append(channelResults, JobAlertChannelResult{
				Channel: normalizedChannels[i],
				Status:  "skipped",
				Reason:  "channel_disabled",
			})
		}
	}

	return summarizeJobAlertChannelResults(channelResults)
}

func (s *Service) dispatchJobAlertEmailChannelLocked(record JobAlertNotification, attempt int) JobAlertChannelResult {
	nextProvider := s.jobAlertEmailProvider
	if nextProvider == "" {
		nextProvider = "mock"
	}

	recipients := s.resolveAlertEmailReceiversLocked(record.ReceiverGroups)
	if len(recipients) == 0 {
		return JobAlertChannelResult{
			Channel:       "email",
			Status:        "failed",
			Reason:        "email_receivers_not_configured",
			Provider:      nextProvider,
			ProviderError: "email receivers not configured",
			Retryable:     false,
		}
	}

	if nextProvider == "resend" || nextProvider == "spaceemail" {
		nextProvider = "resend"
		if s.jobAlertEmailSender == nil {
			return JobAlertChannelResult{
				Channel:       "email",
				Status:        "failed",
				Reason:        "provider_not_configured",
				Provider:      nextProvider,
				ProviderError: "resend sender is not configured",
				Retryable:     false,
				Receivers:     recipients,
			}
		}
		subject, text := buildJobAlertEmailMessage(record)
		sendResult, err := s.jobAlertEmailSender.Send(
			context.Background(),
			AlertEmailSendInput{
				TenantID:       record.TenantID,
				To:             recipients,
				IdempotencyKey: record.IdempotencyKey,
				Subject:        subject,
				Text:           text,
			},
		)
		if err != nil {
			channelRetryable := isJobAlertProviderRetryable(err)
			reason := "provider_error"
			if channelRetryable {
				reason = "provider_transient_error"
			}
			return JobAlertChannelResult{
				Channel:       "email",
				Status:        "failed",
				Reason:        reason,
				Provider:      nextProvider,
				ProviderError: strings.TrimSpace(err.Error()),
				Retryable:     channelRetryable,
				Receivers:     recipients,
			}
		}
		return JobAlertChannelResult{
			Channel:                "email",
			Status:                 "sent",
			Provider:               nextProvider,
			ProviderDeliveryID:     strings.TrimSpace(sendResult.ProviderDeliveryID),
			ProviderDeliveryStatus: strings.TrimSpace(sendResult.ProviderDeliveryStatus),
			Retryable:              false,
			Receivers:              recipients,
		}
	}

	if s.jobAlertMockTransientFailCount > 0 &&
		!s.hasSentJobAlertByIdempotencyLocked(record.TenantID, record.IdempotencyKey) &&
		attempt <= s.jobAlertMockTransientFailCount {
		return JobAlertChannelResult{
			Channel:       "email",
			Status:        "failed",
			Reason:        "provider_transient_error",
			Provider:      "mock",
			ProviderError: "provider_unavailable",
			Retryable:     true,
			Receivers:     recipients,
		}
	}
	return JobAlertChannelResult{
		Channel:   "email",
		Status:    "sent",
		Provider:  "mock",
		Retryable: false,
		Receivers: recipients,
	}
}

func (s *Service) dispatchJobAlertWhatsAppChannelLocked(record JobAlertNotification) JobAlertChannelResult {
	nextProvider := s.jobAlertWhatsAppProvider
	if nextProvider == "" {
		nextProvider = "mock"
	}
	receivers := s.resolveAlertWhatsAppReceiversLocked(record.ReceiverGroups)
	if len(receivers) == 0 {
		return JobAlertChannelResult{
			Channel:       "whatsapp",
			Status:        "failed",
			Reason:        "whatsapp_receivers_not_configured",
			Provider:      nextProvider,
			ProviderError: "whatsapp receivers not configured",
			Retryable:     false,
		}
	}

	if nextProvider == "mock" {
		if s.jobAlertWhatsAppSender == nil {
			return JobAlertChannelResult{
				Channel:       "whatsapp",
				Status:        "failed",
				Reason:        "provider_not_configured",
				Provider:      "mock",
				ProviderError: "mock whatsapp sender is not configured",
				Retryable:     false,
				Receivers:     receivers,
			}
		}
		sendResult, err := s.jobAlertWhatsAppSender.Send(
			context.Background(),
			AlertWhatsAppSendInput{
				TenantID:       record.TenantID,
				To:             receivers,
				IdempotencyKey: record.IdempotencyKey,
				Text:           buildJobAlertWhatsAppMessage(record),
			},
		)
		if err != nil {
			channelRetryable := isJobAlertProviderRetryable(err)
			reason := "provider_error"
			if channelRetryable {
				reason = "provider_transient_error"
			}
			return JobAlertChannelResult{
				Channel:       "whatsapp",
				Status:        "failed",
				Reason:        reason,
				Provider:      "mock",
				ProviderError: strings.TrimSpace(err.Error()),
				Retryable:     channelRetryable,
				Receivers:     receivers,
			}
		}
		return JobAlertChannelResult{
			Channel:                "whatsapp",
			Status:                 "sent",
			Provider:               "mock",
			ProviderDeliveryID:     strings.TrimSpace(sendResult.ProviderDeliveryID),
			ProviderDeliveryStatus: strings.TrimSpace(sendResult.ProviderDeliveryStatus),
			Retryable:              false,
			Receivers:              receivers,
		}
	}

	if nextProvider == "meta" {
		if s.jobAlertWhatsAppSender == nil {
			return JobAlertChannelResult{
				Channel:       "whatsapp",
				Status:        "failed",
				Reason:        "provider_not_configured",
				Provider:      "meta",
				ProviderError: "meta whatsapp sender is not configured",
				Retryable:     false,
				Receivers:     receivers,
			}
		}
		sendResult, err := s.jobAlertWhatsAppSender.Send(
			context.Background(),
			AlertWhatsAppSendInput{
				TenantID:       record.TenantID,
				To:             receivers,
				IdempotencyKey: record.IdempotencyKey,
				Text:           buildJobAlertWhatsAppMessage(record),
			},
		)
		if err != nil {
			channelRetryable := isJobAlertProviderRetryable(err)
			reason := "provider_error"
			if channelRetryable {
				reason = "provider_transient_error"
			}
			return JobAlertChannelResult{
				Channel:       "whatsapp",
				Status:        "failed",
				Reason:        reason,
				Provider:      "meta",
				ProviderError: strings.TrimSpace(err.Error()),
				Retryable:     channelRetryable,
				Receivers:     receivers,
			}
		}
		return JobAlertChannelResult{
			Channel:                "whatsapp",
			Status:                 "sent",
			Provider:               "meta",
			ProviderDeliveryID:     strings.TrimSpace(sendResult.ProviderDeliveryID),
			ProviderDeliveryStatus: strings.TrimSpace(sendResult.ProviderDeliveryStatus),
			Retryable:              false,
			Receivers:              receivers,
		}
	}

	return JobAlertChannelResult{
		Channel:   "whatsapp",
		Status:    "failed",
		Reason:    "provider_not_supported",
		Provider:  nextProvider,
		Retryable: false,
		Receivers: receivers,
	}
}

func normalizeDispatchChannels(channels []string) []string {
	if len(channels) == 0 {
		return nil
	}
	output := make([]string, 0, len(channels))
	seen := map[string]struct{}{}
	for i := range channels {
		next := strings.ToLower(strings.TrimSpace(channels[i]))
		if next == "" {
			continue
		}
		if _, exists := seen[next]; exists {
			continue
		}
		seen[next] = struct{}{}
		output = append(output, next)
	}
	if len(output) == 0 {
		return nil
	}
	return output
}

func buildStaticJobAlertChannelResults(channels []string, status, reason string) []JobAlertChannelResult {
	normalized := normalizeDispatchChannels(channels)
	if len(normalized) == 0 {
		return nil
	}
	output := make([]JobAlertChannelResult, 0, len(normalized))
	for i := range normalized {
		output = append(output, JobAlertChannelResult{
			Channel: normalized[i],
			Status:  status,
			Reason:  reason,
		})
	}
	return output
}

func summarizeJobAlertChannelResults(
	channelResults []JobAlertChannelResult,
) (string, string, string, string, bool, []JobAlertChannelResult) {
	if len(channelResults) == 0 {
		return "skipped", "channel_disabled", "", "", false, channelResults
	}

	anySent := false
	anyFailed := false
	retryable := false
	firstFailed := JobAlertChannelResult{}
	firstSkipped := JobAlertChannelResult{}
	for i := range channelResults {
		switch channelResults[i].Status {
		case "sent":
			anySent = true
		case "failed":
			if !anyFailed {
				firstFailed = channelResults[i]
			}
			anyFailed = true
			if channelResults[i].Retryable {
				retryable = true
			}
		case "skipped":
			if firstSkipped.Channel == "" {
				firstSkipped = channelResults[i]
			}
		}
	}

	if anySent {
		provider := summarizeProvider(channelResults)
		if anyFailed {
			return "sent", "partial_channel_failure", provider, firstFailed.ProviderError, retryable, channelResults
		}
		return "sent", "", provider, "", false, channelResults
	}

	if anyFailed {
		return "failed", firstFailed.Reason, firstFailed.Provider, firstFailed.ProviderError, retryable, channelResults
	}

	if firstSkipped.Channel != "" {
		return "skipped", firstSkipped.Reason, firstSkipped.Provider, "", false, channelResults
	}
	return "skipped", "channel_disabled", "", "", false, channelResults
}

func summarizeProvider(channelResults []JobAlertChannelResult) string {
	providers := map[string]struct{}{}
	for i := range channelResults {
		if channelResults[i].Status != "sent" {
			continue
		}
		next := strings.TrimSpace(channelResults[i].Provider)
		if next == "" {
			continue
		}
		providers[next] = struct{}{}
	}
	if len(providers) == 0 {
		return ""
	}
	if len(providers) == 1 {
		for key := range providers {
			return key
		}
	}
	return "multi"
}

func isJobAlertProviderRetryable(err error) bool {
	if err == nil {
		return false
	}
	var httpErr AlertEmailHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Retryable()
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "temporar") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused")
}

func buildJobAlertNotificationIdempotencyKey(tenantID, alertType, errorCode string, threshold int) string {
	return alertdispatch.BuildNotificationIdempotencyKey(tenantID, alertType, errorCode, threshold)
}

func (s *Service) isJobAlertInCooldownLocked(
	tenantID, errorCode string,
	cooldown time.Duration,
	now time.Time,
) bool {
	if cooldown <= 0 {
		return false
	}
	for i := range s.jobAlertCooldowns {
		if s.jobAlertCooldowns[i].TenantID != tenantID {
			continue
		}
		if s.jobAlertCooldowns[i].ErrorCode != errorCode {
			continue
		}
		return now.Sub(s.jobAlertCooldowns[i].LastSentAt) < cooldown
	}
	return false
}

func (s *Service) upsertJobAlertCooldownLocked(tenantID, errorCode string, now time.Time) {
	for i := range s.jobAlertCooldowns {
		if s.jobAlertCooldowns[i].TenantID != tenantID {
			continue
		}
		if s.jobAlertCooldowns[i].ErrorCode != errorCode {
			continue
		}
		s.jobAlertCooldowns[i].LastSentAt = now
		return
	}
	s.jobAlertCooldowns = append(
		[]JobAlertDispatchCooldown{
			{
				TenantID:   tenantID,
				ErrorCode:  errorCode,
				LastSentAt: now,
			},
		},
		s.jobAlertCooldowns...,
	)
	if len(s.jobAlertCooldowns) > 2000 {
		s.jobAlertCooldowns = s.jobAlertCooldowns[:2000]
	}
}

func (s *Service) updatePassStatus(tenantID, passID, status, actor string) (PassInstance, error) {
	nextPassID := strings.TrimSpace(passID)
	if nextPassID == "" {
		return PassInstance{}, ErrPassNotFound
	}

	nextTenantID := normalizeTenantID(tenantID)
	nextStatus := strings.TrimSpace(status)
	nextActor := normalizeActor(actor)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.passes {
		if s.passes[i].ID != nextPassID {
			continue
		}
		if s.passes[i].TenantID != nextTenantID {
			return PassInstance{}, ErrPassNotFound
		}

		if !canTransitPassStatus(s.passes[i].Status, nextStatus) {
			return PassInstance{}, ErrInvalidPassTransition
		}

		s.passes[i].Status = nextStatus
		s.passes[i].UpdatedBy = nextActor
		s.passes[i].UpdatedAt = now
		if nextStatus == "active" {
			value := now
			s.passes[i].ActivatedAt = &value
		}
		if nextStatus == "revoked" {
			value := now
			s.passes[i].RevokedAt = &value
		}

		s.appendAuditLocked(s.passes[i].TenantID, "wallet.pass."+nextStatus, nextActor, s.passes[i].ID, "success")
		if err := s.persistLocked(); err != nil {
			return PassInstance{}, err
		}

		return s.passes[i], nil
	}

	return PassInstance{}, ErrPassNotFound
}

func (s *Service) createPassRecord(tenantID string, template PassTemplate, targetType, targetID, holderName, holderEmail, expiresAt, actor string, now time.Time) (PassInstance, error) {
	id, err := walletID("wps_")
	if err != nil {
		return PassInstance{}, err
	}

	provider := firstNonEmpty(template.Provider, "google")
	objectID := fmt.Sprintf("%s.%s", template.ClassID, id)
	saveLink := ""
	nfcPayload := ""

	switch strings.ToLower(provider) {
	case "apple":
		if s.applePassProvider != nil {
			bundle, err := s.applePassProvider.IssuePass(tenantID, holderName, holderEmail, id)
			if err == nil {
				objectID = bundle.SerialNumber
				saveLink = bundle.SaveLink
				nfcPayload = bundle.NfcPayload
			}
		}
	case "google", "":
		if s.googleWalletProvider != nil {
			classID := firstNonEmpty(template.ClassID, "mistyislet.access.default")
			obj, err := s.googleWalletProvider.IssuePassObject(tenantID, id, classID, holderName, holderEmail)
			if err == nil {
				objectID = obj.ObjectID
				saveLink = obj.SaveLink
				nfcPayload = obj.NfcPayload
			}
		}
		if saveLink == "" {
			saveLink = fmt.Sprintf("https://pay.google.com/gp/v/save/%s", id)
		}
	}

	record := PassInstance{
		ID:             id,
		TenantID:       tenantID,
		Provider:       provider,
		CredentialKind: credentialKindForProvider(provider, targetType),
		TemplateID:     template.ID,
		TargetType:     targetType,
		TargetID:       targetID,
		ObjectID:       objectID,
		Token:          nfcPayload,
		Status:         "issued",
		SaveLink:       saveLink,
		ExpiresAt:      expiresAt,
		IssuedAt:       now,
		CreatedBy:      actor,
		UpdatedBy:      actor,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	return record, nil
}

func normalizeCardCredentialProvider(cardType, token, uid, cardNumber, fallbackProvider string) (string, string) {
	normalizedType := strings.ToLower(strings.TrimSpace(cardType))
	hasPhysicalIdentifier := strings.TrimSpace(token) != "" || strings.TrimSpace(uid) != "" || strings.TrimSpace(cardNumber) != ""
	switch normalizedType {
	case "apple", "apple_wallet", "apple_pass", "apple_passes":
		return "apple", "apple_wallet"
	case "google", "google_wallet", "wallet":
		return "google", "google_wallet"
	case "physical", "physical_card", "card", "desfire", "mifare", "mifare_desfire", "third_party_hf", "hid", "fob":
		if normalizedType == "physical" || normalizedType == "physical_card" || normalizedType == "card" {
			return "physical_card", "physical_card"
		}
		return normalizedType, "physical_card"
	}
	if hasPhysicalIdentifier {
		return "physical_card", "physical_card"
	}
	provider := firstNonEmpty(strings.ToLower(strings.TrimSpace(fallbackProvider)), "google")
	return provider, credentialKindForProvider(provider, "")
}

func credentialKindForProvider(provider, targetType string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "apple":
		return "apple_wallet"
	case "google", "":
		if strings.EqualFold(strings.TrimSpace(targetType), "visitor") {
			return "google_wallet"
		}
		return "google_wallet"
	case "physical_card", "desfire", "mifare", "mifare_desfire", "third_party_hf", "hid", "fob":
		return "physical_card"
	default:
		return "credential"
	}
}

func findTemplateByID(items []PassTemplate, templateID string) (PassTemplate, bool) {
	for i := range items {
		if items[i].ID == templateID {
			return items[i], true
		}
	}
	return PassTemplate{}, false
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
		return s.stateStore.Save(stateKey, stateSnapshot{
			Config:                    cloneGoogleConfig(s.config),
			Templates:                 cloneTemplates(s.templates),
			Passes:                    clonePasses(s.passes),
			PassDeliveryNotifications: clonePassDeliveryNotifications(s.passDeliveryNotifications),
			PhysicalCardTasks:         clonePhysicalCardTasks(s.physicalCardTasks),
			PhysicalCardVendors:       clonePhysicalCardVendors(s.physicalCardVendors),
			PhysicalCardInventory:     clonePhysicalCardInventory(s.physicalCardInventory),
			Jobs:                      cloneJobs(s.jobs),
			AuditLogs:                 cloneAuditLogs(s.auditLogs),
			DLQCleanupArchives:        cloneDLQCleanupArchives(s.dlqCleanupArchives),
			JobAlertSubscriptions:     cloneJobAlertSubscriptions(s.jobAlertSubscriptions),
			JobAlertNotifications:     cloneJobAlertNotifications(s.jobAlertNotifications),
			JobAlertCooldowns:         cloneJobAlertCooldowns(s.jobAlertCooldowns),
		})
	}

	s.mu.Lock()
	s.config = cloneGoogleConfig(snapshot.Config)
	s.templates = cloneTemplates(snapshot.Templates)
	s.passes = clonePasses(snapshot.Passes)
	s.passDeliveryNotifications = clonePassDeliveryNotifications(snapshot.PassDeliveryNotifications)
	s.physicalCardTasks = clonePhysicalCardTasks(snapshot.PhysicalCardTasks)
	s.physicalCardVendors = clonePhysicalCardVendors(snapshot.PhysicalCardVendors)
	s.physicalCardInventory = clonePhysicalCardInventory(snapshot.PhysicalCardInventory)
	s.jobs = cloneJobs(snapshot.Jobs)
	s.auditLogs = cloneAuditLogs(snapshot.AuditLogs)
	s.dlqCleanupArchives = cloneDLQCleanupArchives(snapshot.DLQCleanupArchives)
	s.jobAlertSubscriptions = cloneJobAlertSubscriptions(snapshot.JobAlertSubscriptions)
	s.jobAlertNotifications = cloneJobAlertNotifications(snapshot.JobAlertNotifications)
	s.jobAlertCooldowns = cloneJobAlertCooldowns(snapshot.JobAlertCooldowns)
	s.mu.Unlock()

	return nil
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		Config:                    cloneGoogleConfig(s.config),
		Templates:                 cloneTemplates(s.templates),
		Passes:                    clonePasses(s.passes),
		PassDeliveryNotifications: clonePassDeliveryNotifications(s.passDeliveryNotifications),
		PhysicalCardTasks:         clonePhysicalCardTasks(s.physicalCardTasks),
		PhysicalCardVendors:       clonePhysicalCardVendors(s.physicalCardVendors),
		PhysicalCardInventory:     clonePhysicalCardInventory(s.physicalCardInventory),
		Jobs:                      cloneJobs(s.jobs),
		AuditLogs:                 cloneAuditLogs(s.auditLogs),
		DLQCleanupArchives:        cloneDLQCleanupArchives(s.dlqCleanupArchives),
		JobAlertSubscriptions:     cloneJobAlertSubscriptions(s.jobAlertSubscriptions),
		JobAlertNotifications:     cloneJobAlertNotifications(s.jobAlertNotifications),
		JobAlertCooldowns:         cloneJobAlertCooldowns(s.jobAlertCooldowns),
	})
}

func (s *Service) appendAuditLocked(tenantID, action, actor, targetID, result string) {
	id, err := walletID("wal_")
	if err != nil {
		id = fmt.Sprintf("wal_fallback_%d", time.Now().UnixNano())
	}

	s.auditLogs = append([]AuditLog{
		{
			ID:       id,
			TenantID: tenantID,
			Action:   action,
			Actor:    actor,
			TargetID: targetID,
			Result:   result,
			At:       time.Now().UTC(),
		},
	}, s.auditLogs...)
}

func (s *Service) appendDLQCleanupArchiveLocked(
	options JobDLQCleanupOptions,
	result JobDLQCleanupResult,
) {
	id, err := walletID("wdca_")
	if err != nil {
		id = fmt.Sprintf("wdca_fallback_%d", time.Now().UnixNano())
	}

	s.dlqCleanupArchives = append([]JobDLQCleanupArchive{
		{
			ID:               id,
			TenantID:         options.TenantID,
			Limit:            options.Limit,
			ErrorCode:        strings.TrimSpace(options.ErrorCode),
			OlderThanSeconds: int64(options.OlderThan.Seconds()),
			Actor:            normalizeActor(options.Actor),
			Removed:          result.Removed,
			RemainingDLQ:     result.RemainingDLQ,
			ProcessedJobs:    append([]string(nil), result.ProcessedJobs...),
			At:               time.Now().UTC(),
		},
	}, s.dlqCleanupArchives...)
	if len(s.dlqCleanupArchives) > 2000 {
		s.dlqCleanupArchives = s.dlqCleanupArchives[:2000]
	}
}

func normalizeConfigStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "active":
		return "active", nil
	case "inactive":
		return "inactive", nil
	default:
		return "", ErrInvalidConfigStatus
	}
}

func looksLikeServiceAccountEmail(value string) bool {
	email := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(email, "@") && strings.HasSuffix(email, ".iam.gserviceaccount.com")
}

func looksLikeKeyRef(value string) bool {
	next := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(next, "kms://") ||
		strings.HasPrefix(next, "sm://") ||
		strings.HasPrefix(next, "secret://") ||
		strings.HasPrefix(next, "env://") ||
		strings.HasPrefix(next, "file://")
}

func normalizePassType(passType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(passType)) {
	case "employee":
		return "employee", nil
	case "visitor":
		return "visitor", nil
	default:
		return "", ErrInvalidPassType
	}
}

func normalizeTemplateStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "active":
		return "active", nil
	case "inactive":
		return "inactive", nil
	default:
		return "", ErrInvalidTemplateStatus
	}
}

func normalizeTargetType(targetType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "user":
		return "user", nil
	case "visitor":
		return "visitor", nil
	default:
		return "", ErrInvalidTargetType
	}
}

func canTransitPassStatus(current, next string) bool {
	if current == next {
		return true
	}

	switch next {
	case "suspended":
		return current == "active" || current == "issued"
	case "active":
		return current == "issued" || current == "suspended"
	case "revoked":
		return current == "issued" || current == "active" || current == "suspended"
	default:
		return false
	}
}

func normalizeActor(actor string) string {
	nextActor := strings.TrimSpace(actor)
	if nextActor == "" {
		return "system"
	}
	return nextActor
}

func firstNonEmpty(values ...string) string {
	for i := range values {
		if strings.TrimSpace(values[i]) != "" {
			return strings.TrimSpace(values[i])
		}
	}
	return ""
}

func normalizeTenantID(tenantID string) string {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return "tenant_demo_jakarta"
	}
	return nextTenantID
}

func resolveJobAlertSubscriptionUpsertOptions(
	input JobAlertSubscriptionUpsertOptions,
) (JobAlertSubscriptionUpsertOptions, error) {
	next := JobAlertSubscriptionUpsertOptions{
		TenantID:        normalizeTenantID(input.TenantID),
		Enabled:         input.Enabled,
		EmailEnabled:    input.EmailEnabled,
		WhatsAppEnabled: input.WhatsAppEnabled,
		ReceiverGroups:  normalizeReceiverGroups(input.ReceiverGroups),
		Actor:           normalizeActor(input.Actor),
	}

	nextThreshold := input.DLQAlertThreshold
	if nextThreshold < 1 || nextThreshold > 100000 {
		return JobAlertSubscriptionUpsertOptions{}, ErrInvalidJobAlertSubscriptionOptions
	}
	next.DLQAlertThreshold = nextThreshold

	nextWindow := input.Window
	if nextWindow < time.Second || nextWindow > 7*24*time.Hour {
		return JobAlertSubscriptionUpsertOptions{}, ErrInvalidJobAlertSubscriptionOptions
	}
	next.Window = nextWindow

	nextCooldown := input.Cooldown
	if nextCooldown < 0 || nextCooldown > 7*24*time.Hour {
		return JobAlertSubscriptionUpsertOptions{}, ErrInvalidJobAlertSubscriptionOptions
	}
	next.Cooldown = nextCooldown

	if len(next.ReceiverGroups) > 20 {
		return JobAlertSubscriptionUpsertOptions{}, ErrInvalidJobAlertSubscriptionOptions
	}
	if len(next.ReceiverGroups) == 0 {
		next.ReceiverGroups = []string{"security"}
	}
	if next.Enabled && !next.EmailEnabled && !next.WhatsAppEnabled {
		return JobAlertSubscriptionUpsertOptions{}, ErrInvalidJobAlertSubscriptionOptions
	}
	return next, nil
}

func isRemoteGoogleValidationEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("WALLET_GOOGLE_REMOTE_VALIDATE"))
	if raw == "" {
		return false
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return enabled
}

func remoteGoogleValidationTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("WALLET_GOOGLE_REMOTE_TIMEOUT"))
	if raw == "" {
		return 8 * time.Second
	}
	timeout, err := time.ParseDuration(raw)
	if err != nil || timeout <= 0 {
		return 8 * time.Second
	}
	return timeout
}

func cloneGoogleConfig(input *GoogleConfig) *GoogleConfig {
	if input == nil {
		return nil
	}
	record := *input
	return &record
}

func cloneTemplates(items []PassTemplate) []PassTemplate {
	output := make([]PassTemplate, 0, len(items))
	for i := range items {
		output = append(output, cloneTemplate(items[i]))
	}
	return output
}

func clonePasses(items []PassInstance) []PassInstance {
	output := make([]PassInstance, 0, len(items))
	for i := range items {
		record := items[i]
		if items[i].ActivatedAt != nil {
			value := *items[i].ActivatedAt
			record.ActivatedAt = &value
		}
		if items[i].RevokedAt != nil {
			value := *items[i].RevokedAt
			record.RevokedAt = &value
		}
		output = append(output, record)
	}
	return output
}

func cloneJobs(items []IssueJob) []IssueJob {
	output := make([]IssueJob, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneAuditLogs(items []AuditLog) []AuditLog {
	output := make([]AuditLog, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneDLQCleanupArchives(items []JobDLQCleanupArchive) []JobDLQCleanupArchive {
	output := make([]JobDLQCleanupArchive, 0, len(items))
	for i := range items {
		output = append(output, cloneDLQCleanupArchive(items[i]))
	}
	return output
}

func cloneDLQCleanupArchive(input JobDLQCleanupArchive) JobDLQCleanupArchive {
	output := input
	if len(input.ProcessedJobs) > 0 {
		output.ProcessedJobs = append([]string(nil), input.ProcessedJobs...)
	}
	return output
}

func cloneJobAlertSubscriptions(items []JobAlertSubscription) []JobAlertSubscription {
	output := make([]JobAlertSubscription, 0, len(items))
	for i := range items {
		output = append(output, cloneJobAlertSubscription(items[i]))
	}
	return output
}

func cloneJobAlertSubscription(input JobAlertSubscription) JobAlertSubscription {
	output := input
	output.ReceiverGroups = normalizeReceiverGroups(input.ReceiverGroups)
	return output
}

func cloneJobAlertNotifications(items []JobAlertNotification) []JobAlertNotification {
	output := make([]JobAlertNotification, 0, len(items))
	for i := range items {
		output = append(output, cloneJobAlertNotification(items[i]))
	}
	return output
}

func cloneJobAlertNotification(input JobAlertNotification) JobAlertNotification {
	output := input
	output.Channels = append([]string(nil), input.Channels...)
	output.ReceiverGroups = normalizeReceiverGroups(input.ReceiverGroups)
	output.ChannelResults = cloneJobAlertChannelResults(input.ChannelResults)
	return output
}

func cloneJobAlertChannelResults(items []JobAlertChannelResult) []JobAlertChannelResult {
	if len(items) == 0 {
		return nil
	}
	output := make([]JobAlertChannelResult, 0, len(items))
	for i := range items {
		record := items[i]
		record.Receivers = append([]string(nil), items[i].Receivers...)
		output = append(output, record)
	}
	return output
}

func cloneJobAlertCooldowns(items []JobAlertDispatchCooldown) []JobAlertDispatchCooldown {
	output := make([]JobAlertDispatchCooldown, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneTemplate(input PassTemplate) PassTemplate {
	output := input
	output.StyleConfig = cloneStringMap(input.StyleConfig)
	return output
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}

	items := make(map[string]string, len(source))
	for key, value := range source {
		nextKey := strings.TrimSpace(key)
		if nextKey == "" {
			continue
		}
		items[nextKey] = strings.TrimSpace(value)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func normalizeReceiverGroups(items []string) []string {
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

func normalizeJobAlertReceiverMap(source map[string][]string) map[string][]string {
	if len(source) == 0 {
		return nil
	}
	output := make(map[string][]string)
	for key, values := range source {
		group := strings.ToLower(strings.TrimSpace(key))
		if group == "" {
			continue
		}
		unique := make([]string, 0, len(values))
		seen := map[string]struct{}{}
		for i := range values {
			next := strings.TrimSpace(values[i])
			if next == "" {
				continue
			}
			lower := strings.ToLower(next)
			if _, exists := seen[lower]; exists {
				continue
			}
			seen[lower] = struct{}{}
			unique = append(unique, next)
		}
		if len(unique) == 0 {
			continue
		}
		output[group] = unique
	}
	if len(output) == 0 {
		return nil
	}
	return output
}

func (s *Service) resolveAlertEmailReceiversLocked(receiverGroups []string) []string {
	normalizedGroups := normalizeReceiverGroups(receiverGroups)
	receivers := make([]string, 0, len(normalizedGroups))
	seen := map[string]struct{}{}
	for i := range normalizedGroups {
		group := strings.TrimSpace(normalizedGroups[i])
		if group == "" {
			continue
		}
		groupKey := strings.ToLower(group)
		if mapped, exists := s.jobAlertEmailReceiverMap[groupKey]; exists {
			for j := range mapped {
				next := strings.TrimSpace(mapped[j])
				if next == "" {
					continue
				}
				key := strings.ToLower(next)
				if _, duplicated := seen[key]; duplicated {
					continue
				}
				seen[key] = struct{}{}
				receivers = append(receivers, next)
			}
			continue
		}
		if looksLikeEmail(group) {
			key := strings.ToLower(group)
			if _, duplicated := seen[key]; duplicated {
				continue
			}
			seen[key] = struct{}{}
			receivers = append(receivers, group)
		}
	}
	return receivers
}

func (s *Service) resolveAlertWhatsAppReceiversLocked(receiverGroups []string) []string {
	normalizedGroups := normalizeReceiverGroups(receiverGroups)
	receivers := make([]string, 0, len(normalizedGroups))
	seen := map[string]struct{}{}
	for i := range normalizedGroups {
		group := strings.TrimSpace(normalizedGroups[i])
		if group == "" {
			continue
		}
		groupKey := strings.ToLower(group)
		if mapped, exists := s.jobAlertWhatsAppReceiverMap[groupKey]; exists {
			for j := range mapped {
				next := strings.TrimSpace(mapped[j])
				if next == "" {
					continue
				}
				key := strings.ToLower(next)
				if _, duplicated := seen[key]; duplicated {
					continue
				}
				seen[key] = struct{}{}
				receivers = append(receivers, next)
			}
			continue
		}
		if looksLikePhoneNumber(group) {
			key := strings.ToLower(group)
			if _, duplicated := seen[key]; duplicated {
				continue
			}
			seen[key] = struct{}{}
			receivers = append(receivers, group)
		}
	}
	return receivers
}

func buildJobAlertEmailMessage(record JobAlertNotification) (subject, text string) {
	subject = fmt.Sprintf(
		"[MistyPass][%s] Wallet 告警：%s",
		strings.TrimSpace(record.TenantID),
		strings.TrimSpace(record.ErrorCode),
	)
	text = fmt.Sprintf(
		"tenant=%s\ntype=%s\nerror_code=%s\ncount=%d\nthreshold=%d\ntriggered_at=%s",
		strings.TrimSpace(record.TenantID),
		strings.TrimSpace(record.Type),
		strings.TrimSpace(record.ErrorCode),
		record.Count,
		record.Threshold,
		record.TriggeredAt.Format(time.RFC3339),
	)
	return subject, text
}

func buildJobAlertWhatsAppMessage(record JobAlertNotification) string {
	return fmt.Sprintf(
		"[MistyPass][%s] Wallet告警 error_code=%s count=%d threshold=%d at=%s",
		strings.TrimSpace(record.TenantID),
		strings.TrimSpace(record.ErrorCode),
		record.Count,
		record.Threshold,
		record.TriggeredAt.Format(time.RFC3339),
	)
}

func looksLikeEmail(value string) bool {
	next := strings.TrimSpace(value)
	at := strings.Index(next, "@")
	if at <= 0 || at >= len(next)-1 {
		return false
	}
	return strings.Contains(next[at+1:], ".")
}

func looksLikePhoneNumber(value string) bool {
	next := strings.TrimSpace(value)
	if next == "" {
		return false
	}
	if strings.HasPrefix(next, "whatsapp:") {
		next = strings.TrimPrefix(next, "whatsapp:")
	}
	next = strings.TrimPrefix(next, "+")
	if len(next) < 8 || len(next) > 20 {
		return false
	}
	for _, r := range next {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func resolveAlertChannels(channels JobAlertSubscriptionChannels) []string {
	output := make([]string, 0, 2)
	if channels.Email {
		output = append(output, "email")
	}
	if channels.WhatsApp {
		output = append(output, "whatsapp")
	}
	return output
}

func walletID(prefix string) (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw), nil
}
