package wallet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
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
	DeviceName     string     `json:"device_name,omitempty"`
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
	WhatsAppTemplateName  string
	WhatsAppTemplateLang  string
	LarkAlertWebhookURL   string
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
	jobAlertWhatsAppTemplateName   string
	jobAlertWhatsAppTemplateLang   string
	jobAlertLarkWebhookURL         string
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
			{
				ID:       "wpt_nfc_card_demo",
				TenantID: "tenant_demo_jakarta",
				Provider: "physical",
				PassType: "nfc_card",
				ClassID:  "mistypass.nfc.card",
				Name:     "NFC 门禁卡",
				StyleConfig: map[string]string{
					"brand":    "MistyPass",
					"cardType": "desfire_ev3",
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
	s.jobAlertWhatsAppTemplateName = strings.TrimSpace(options.WhatsAppTemplateName)
	s.jobAlertWhatsAppTemplateLang = strings.TrimSpace(options.WhatsAppTemplateLang)
	s.jobAlertLarkWebhookURL = strings.TrimSpace(options.LarkAlertWebhookURL)
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
