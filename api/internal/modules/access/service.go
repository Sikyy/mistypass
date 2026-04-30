package access

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

var ErrTenantIDRequired = errors.New("tenant_id is required")
var ErrUserRequired = errors.New("user is required")
var ErrDoorIDRequired = errors.New("door_id is required")
var ErrValidUntilRequired = errors.New("valid_until is required")
var ErrTemporaryAccessNotFound = errors.New("temporary access not found")
var ErrHostRequired = errors.New("host is required")
var ErrVisitorRequired = errors.New("visitor is required")
var ErrExpiresAtRequired = errors.New("expires_at is required")
var ErrPolicyNameRequired = errors.New("policy name is required")
var ErrPolicyNotFound = errors.New("policy not found")
var ErrInvalidPolicyStatus = errors.New("invalid policy status")
var ErrInvalidScopeType = errors.New("invalid scope type")
var ErrUserNotFound = errors.New("user not found")
var ErrUserNameRequired = errors.New("user name is required")
var ErrUserEmailRequired = errors.New("user email is required")
var ErrInvalidUserStatus = errors.New("invalid user status")
var ErrUserInvitationDeliveryNotFound = errors.New("user invitation delivery not found")
var ErrInvalidUserInvitationStatus = errors.New("invalid user invitation status")
var ErrUserGroupNameRequired = errors.New("user group name is required")
var ErrUserGroupNotFound = errors.New("user group not found")
var ErrGroupIDRequired = errors.New("group_id is required")
var ErrGroupLinkNameRequired = errors.New("group link name is required")
var ErrGroupLinkNotFound = errors.New("group link not found")
var ErrGroupLinkTokenRequired = errors.New("group link token is required")
var ErrGroupLinkTokenInvalid = errors.New("group link token is invalid")
var ErrGroupLinkDisabled = errors.New("group link is disabled")
var ErrGroupLinkNotYetValid = errors.New("group link is not yet valid")
var ErrGroupLinkExpired = errors.New("group link has expired")
var ErrInvalidGroupLinkValidityWindow = errors.New("invalid group link validity window")
var ErrInvalidGroupLinkQRCodeType = errors.New("invalid group link quick_response_code_type")
var ErrTeamNameRequired = errors.New("team name is required")
var ErrTeamNotFound = errors.New("team not found")
var ErrTeamIDRequired = errors.New("team_id is required")
var ErrInvalidTeamMemberType = errors.New("invalid team member type")
var ErrTeamMemberIDRequired = errors.New("member_id is required")
var ErrTeamMembershipNotFound = errors.New("team membership not found")
var ErrDeliveryMethodInvalid = errors.New("invalid delivery method")
var ErrGranteeNameRequired = errors.New("grantee_name is required")
var ErrGranteeEmailRequired = errors.New("grantee_email is required")
var ErrGranteePhoneRequired = errors.New("grantee_phone is required")
var ErrRoleIDRequired = errors.New("role_id is required")
var ErrRoleNotFound = errors.New("role not found")
var ErrRoleAssignmentNotFound = errors.New("role assignment not found")
var ErrInvalidRoleScope = errors.New("invalid role applies_to scope")
var ErrInvalidAssigneeType = errors.New("invalid assignee type")
var ErrAppliesToIDRequired = errors.New("applies_to_id is required")
var ErrAssigneeIDRequired = errors.New("assignee_id is required")
var ErrAccessRightSelectionRequired = errors.New("access right selection is required")
var ErrUserIDsRequired = errors.New("user_ids is required")
var ErrUsersImportRecordsRequired = errors.New("import records are required")

type BatchUserStatusResult struct {
	TenantID string `json:"tenant_id"`
	Status   string `json:"status"`
	Updated  int    `json:"updated"`
	Skipped  int    `json:"skipped"`
	NotFound int    `json:"not_found"`
	UserIDs  []string `json:"user_ids"`
}

type BatchUserDeleteResult struct {
	TenantID string `json:"tenant_id"`
	Deleted  int    `json:"deleted"`
	NotFound int    `json:"not_found"`
	UserIDs  []string `json:"user_ids"`
}

type BatchUserInviteResult struct {
	TenantID   string `json:"tenant_id"`
	Queued     int    `json:"queued"`
	Skipped    int    `json:"skipped"`
	NotFound   int    `json:"not_found"`
	UserIDs    []string `json:"user_ids"`
}

type UserImportRecord struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	BuildingID string `json:"building_id"`
}

type UserImportResult struct {
	TenantID string `json:"tenant_id"`
	Created  int    `json:"created"`
	Updated  int    `json:"updated"`
	Skipped  int    `json:"skipped"`
	Errors   int    `json:"errors"`
}

type Role struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	AppliesTo   string          `json:"applies_to"`
	Description string          `json:"description,omitempty"`
	Permissions map[string]bool `json:"permissions"`
	BuiltIn     bool            `json:"built_in"`
}

type TimeWindow struct {
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	DayOfWeekSet string `json:"day_of_week_set"`
	Timezone     string `json:"timezone,omitempty"`
}

type HolidayCalendar struct {
	ID        string         `json:"id"`
	TenantID  string         `json:"tenant_id"`
	Name      string         `json:"name"`
	Country   string         `json:"country,omitempty"`
	Entries   []HolidayEntry `json:"entries"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type HolidayEntry struct {
	Date        string `json:"date"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type RoleAssignment struct {
	ID                string       `json:"id"`
	TenantID          string       `json:"tenant_id"`
	RoleID            string       `json:"role_id"`
	AppliesToType     string       `json:"applies_to_type"`
	AppliesToID       string       `json:"applies_to_id"`
	AssigneeType      string       `json:"assignee_type"`
	AssigneeID        string       `json:"assignee_id"`
	AssigneeEmail     string       `json:"assignee_email,omitempty"`
	ValidFrom         string       `json:"valid_from,omitempty"`
	ValidUntil        string       `json:"valid_until,omitempty"`
	TimeWindows       []TimeWindow `json:"time_windows,omitempty"`
	ExceptionDates    []string     `json:"exception_dates,omitempty"`
	HolidayCalendarID string       `json:"holiday_calendar_id,omitempty"`
	ReviewedAt        string       `json:"reviewed_at,omitempty"`
	ReviewedBy        string       `json:"reviewed_by,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

type RoleAssignmentInput struct {
	TenantID          string
	RoleID            string
	AppliesToType     string
	AppliesToID       string
	AssigneeType      string
	AssigneeID        string
	AssigneeEmail     string
	ValidFrom         string
	ValidUntil        string
	TimeWindows       []TimeWindow
	ExceptionDates    []string
	HolidayCalendarID string
}

type Policy struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Name       string    `json:"name"`
	ScopeType  string    `json:"scope_type"`
	BuildingID string    `json:"building_id,omitempty"`
	AreaID     string    `json:"area_id,omitempty"`
	DoorID     string    `json:"door_id,omitempty"`
	Schedule   string    `json:"schedule"`
	Members    int       `json:"members"`
	Status     string    `json:"status"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AccessUser struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	BuildingID string    `json:"building_id,omitempty"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Role       string    `json:"role"`
	Status     string    `json:"status"`
	GroupIDs   []string  `json:"group_ids,omitempty"`
	SyncSource string    `json:"sync_source,omitempty"`
	SyncRef    string    `json:"sync_ref,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserInvitationDelivery struct {
	ID                 string     `json:"id"`
	ResourceType       string     `json:"resource_type"`
	TenantID           string     `json:"tenant_id"`
	UserID             string     `json:"user_id"`
	Email              string     `json:"email"`
	PlaceID            string     `json:"place_id,omitempty"`
	DeliveryMethod     string     `json:"delivery_method"`
	Status             string     `json:"status"`
	Provider           string     `json:"provider,omitempty"`
	ProviderDeliveryID string     `json:"provider_delivery_id,omitempty"`
	ProviderError      string     `json:"provider_error,omitempty"`
	Retryable          bool       `json:"retryable,omitempty"`
	QueuedAt           time.Time  `json:"queued_at"`
	DeliveredAt        *time.Time `json:"delivered_at,omitempty"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type BatchUpsertUserByEmailInput struct {
	BuildingID string
	Name       string
	Email      string
	Role       string
	Status     string
	GroupIDs   []string
	SyncSource string
	SyncRef    string
}

type UserGroup struct {
	ID                              string    `json:"id"`
	ResourceType                    string    `json:"resource_type,omitempty"`
	TenantID                        string    `json:"tenant_id"`
	BuildingID                      string    `json:"building_id,omitempty"`
	PlaceID                         string    `json:"place_id,omitempty"`
	Name                            string    `json:"name"`
	Description                     string    `json:"description"`
	LoginEnabled                    bool      `json:"login_enabled"`
	GeofenceRestrictionEnabled      bool      `json:"geofence_restriction_enabled"`
	GeofenceRestrictionRadius       float64   `json:"geofence_restriction_radius"`
	PrimaryDeviceRestrictionEnabled bool      `json:"primary_device_restriction_enabled"`
	ManagedDeviceRestrictionEnabled bool      `json:"managed_device_restriction_enabled"`
	ReaderRestrictionEnabled        bool      `json:"reader_restriction_enabled"`
	TimeRestrictionEnabled          bool      `json:"time_restriction_enabled"`
	TapToAccessRestrictionEnabled   bool      `json:"tap_to_access_restriction_enabled"`
	TimeRestrictionTimeZone         string    `json:"time_restriction_time_zone,omitempty"`
	Members                         []string  `json:"members,omitempty"`
	UsersCount                      int       `json:"users_count"`
	LocksCount                      int       `json:"locks_count"`
	ElevatorStopsCount              int       `json:"elevator_stops_count"`
	CreatedAt                       time.Time `json:"created_at"`
	UpdatedAt                       time.Time `json:"updated_at"`
}

type UserGroupRestrictionsInput struct {
	LoginEnabled                    *bool
	GeofenceRestrictionEnabled      *bool
	GeofenceRestrictionRadius       *float64
	PrimaryDeviceRestrictionEnabled *bool
	ManagedDeviceRestrictionEnabled *bool
	ReaderRestrictionEnabled        *bool
	TimeRestrictionEnabled          *bool
	TapToAccessRestrictionEnabled   *bool
	TimeRestrictionTimeZone         *string
}

type GroupLink struct {
	ID                     string    `json:"id"`
	ResourceType           string    `json:"resource_type"`
	TenantID               string    `json:"tenant_id"`
	GroupID                string    `json:"group_id"`
	GroupName              string    `json:"group_name,omitempty"`
	Name                   string    `json:"name"`
	Email                  string    `json:"email,omitempty"`
	Phone                  string    `json:"phone,omitempty"`
	LastUsedAt             string    `json:"last_used_at,omitempty"`
	LinkEnabled            bool      `json:"link_enabled"`
	QuickResponseCodeType  string    `json:"quick_response_code_type,omitempty"`
	ValidFrom              string    `json:"valid_from,omitempty"`
	ValidUntil             string    `json:"valid_until,omitempty"`
	CreatedByType          string    `json:"created_by_type,omitempty"`
	CreatedByID            string    `json:"created_by_id,omitempty"`
	CreatedByEmail         string    `json:"created_by_email,omitempty"`
	CreatedByName          string    `json:"created_by_name,omitempty"`
	IssuedByID             string    `json:"issued_by_id,omitempty"`
	Secret                 string    `json:"secret,omitempty"`
	QuickResponseCodeToken string    `json:"quick_response_code_token,omitempty"`
	QuickResponseCodeImage string    `json:"quick_response_code_image,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type GroupLinkInput struct {
	TenantID              string
	GroupID               string
	Name                  string
	Email                 string
	Phone                 string
	LinkEnabled           *bool
	QuickResponseCodeType string
	ValidFrom             string
	ValidUntil            string
	CreatedByType         string
	CreatedByID           string
	CreatedByEmail        string
	CreatedByName         string
}

type GroupLinkUpdateInput struct {
	TenantID              string
	GroupID               *string
	Name                  *string
	Email                 *string
	Phone                 *string
	LinkEnabled           *bool
	QuickResponseCodeType *string
	ValidFrom             *string
	ValidUntil            *string
}

type Team struct {
	ID           string    `json:"id"`
	ResourceType string    `json:"resource_type"`
	TenantID     string    `json:"tenant_id"`
	Name         string    `json:"name"`
	Scope        string    `json:"scope"`
	PlaceID      string    `json:"place_id,omitempty"`
	Description  string    `json:"description,omitempty"`
	Source       string    `json:"source,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type TeamMembership struct {
	ID           string    `json:"id"`
	ResourceType string    `json:"resource_type"`
	TenantID     string    `json:"tenant_id"`
	TeamID       string    `json:"team_id"`
	MemberType   string    `json:"member_type"`
	MemberID     string    `json:"member_id"`
	MemberEmail  string    `json:"member_email,omitempty"`
	MemberName   string    `json:"member_name,omitempty"`
	Source       string    `json:"source,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type TemporaryAccess struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	ScopeType         string    `json:"scope_type"`
	BuildingID        string    `json:"building_id,omitempty"`
	AreaID            string    `json:"area_id,omitempty"`
	DoorID            string    `json:"door_id,omitempty"`
	GroupID           string    `json:"group_id,omitempty"`
	RoleID            string    `json:"role_id,omitempty"`
	DeliveryMethod    string    `json:"delivery_method"`
	GranteeName       string    `json:"grantee_name"`
	GranteeGender     string    `json:"grantee_gender,omitempty"`
	GranteePhone      string    `json:"grantee_phone"`
	GranteeEmail      string    `json:"grantee_email"`
	MobileModel       string    `json:"mobile_model,omitempty"`
	PassType          string    `json:"pass_type,omitempty"`
	ValidFrom          string       `json:"valid_from,omitempty"`
	ValidUntil         string       `json:"valid_until"`
	TimeWindows        []TimeWindow `json:"time_windows,omitempty"`
	ExceptionDates     []string     `json:"exception_dates,omitempty"`
	HolidayCalendarID  string       `json:"holiday_calendar_id,omitempty"`
	AuthorizedByID     string       `json:"authorized_by_id,omitempty"`
	AuthorizedByEmail  string       `json:"authorized_by_email,omitempty"`
	AuthorizedByRole   string       `json:"authorized_by_role,omitempty"`
	AuthorizedAt       time.Time    `json:"authorized_at"`
	ReviewedAt         string       `json:"reviewed_at,omitempty"`
	ReviewedBy         string       `json:"reviewed_by,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
}

type TemporaryAccessInput struct {
	TenantID           string
	ScopeType          string
	BuildingID         string
	AreaID             string
	DoorID             string
	GroupID            string
	RoleID             string
	DeliveryMethod     string
	GranteeName        string
	GranteeGender      string
	GranteePhone       string
	GranteeEmail       string
	MobileModel        string
	PassType           string
	ValidFrom          string
	ValidUntil         string
	AuthorizedByID     string
	AuthorizedByEmail  string
	AuthorizedByRole   string
	KeepAuthorizedTime bool
	ReviewedAt         string
	ReviewedBy         string
}

type AccessRightsReviewResult struct {
	TenantID                  string   `json:"tenant_id"`
	ReviewedAt                string   `json:"reviewed_at"`
	ReviewedBy                string   `json:"reviewed_by"`
	ReviewedCount             int      `json:"reviewed_count"`
	SkippedCount              int      `json:"skipped_count"`
	ReviewedRoleAssignmentIDs []string `json:"reviewed_role_assignment_ids,omitempty"`
	ReviewedShareIDs          []string `json:"reviewed_share_ids,omitempty"`
}

type VisitorPass struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	BuildingID     string    `json:"building_id,omitempty"`
	Host           string    `json:"host"`
	Visitor        string    `json:"visitor"`
	DeliveryMethod string    `json:"delivery_method"`
	ExpiresAt      string    `json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
}

type StateStore interface {
	Load(key string, dst any) (bool, error)
	Save(key string, value any) error
}

const stateKey = "module_access"

type stateSnapshot struct {
	Users                    []AccessUser             `json:"users"`
	UserInvitationDeliveries []UserInvitationDelivery `json:"user_invitation_deliveries"`
	UserGroups               []UserGroup              `json:"user_groups"`
	Policies                 []Policy                 `json:"policies"`
	TemporaryAccess          []TemporaryAccess        `json:"temporary_access"`
	VisitorPasses            []VisitorPass            `json:"visitor_passes"`
	RoleAssignments          []RoleAssignment         `json:"role_assignments"`
	Teams                    []Team                   `json:"teams"`
	TeamMemberships          []TeamMembership         `json:"team_memberships"`
	GroupLinks               []GroupLink              `json:"group_links"`
}

type Schedule struct {
	ID                string       `json:"id"`
	TenantID          string       `json:"tenant_id"`
	Name              string       `json:"name"`
	Description       string       `json:"description,omitempty"`
	ValidFrom         string       `json:"valid_from,omitempty"`
	ValidUntil        string       `json:"valid_until,omitempty"`
	TimeWindows       []TimeWindow `json:"time_windows,omitempty"`
	ExceptionDates    []string     `json:"exception_dates,omitempty"`
	HolidayCalendarID string       `json:"holiday_calendar_id,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

var (
	ErrScheduleNotFound     = errors.New("schedule not found")
	ErrScheduleNameRequired = errors.New("schedule name is required")
)

type OrganizationSettings struct {
	TenantID              string    `json:"tenant_id"`
	Name                  string    `json:"name"`
	PrimaryDomain         string    `json:"primary_domain"`
	Timezone              string    `json:"timezone"`
	SupportEmail          string    `json:"support_email"`
	EmailNotifications    bool      `json:"email_notifications"`
	PushNotifications     bool      `json:"push_notifications"`
	WeeklyReports         bool      `json:"weekly_reports"`
	EnforceMFA            bool      `json:"enforce_mfa"`
	PasswordPolicy        string    `json:"password_policy"`
	SessionTimeoutMinutes int       `json:"session_timeout_minutes"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type Service struct {
	mu                       sync.RWMutex
	users                    []AccessUser
	userInvitationDeliveries []UserInvitationDelivery
	userGroups               []UserGroup
	policies                 []Policy
	temporaryAccess          []TemporaryAccess
	visitorPasses            []VisitorPass
	roleAssignments          []RoleAssignment
	teams                    []Team
	teamMemberships          []TeamMembership
	groupLinks               []GroupLink
	holidayCalendars         []HolidayCalendar
	schedules                []Schedule
	organizationSettings     map[string]OrganizationSettings
	stateStore               StateStore
}

func NewService() *Service {
	now := time.Now().UTC()
	return &Service{
		users: []AccessUser{
			{
				ID:         "usr_1001",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_001",
				Name:       "Andri Pratama",
				Email:      "andri.pratama@mistypass.local",
				Role:       "employee",
				Status:     "active",
				GroupIDs:   []string{"ug_common_office_jkt"},
				CreatedAt:  now,
			},
			{
				ID:         "usr_1002",
				TenantID:   "tenant_demo_factory",
				BuildingID: "building_demo_003",
				Name:       "Rina Hartono",
				Email:      "rina.hartono@mistypass.local",
				Role:       "operator",
				Status:     "active",
				GroupIDs:   []string{"ug_security_fct"},
				CreatedAt:  now,
			},
			{
				ID:         "usr_resident_jkt_001",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_001",
				Name:       "Jakarta Resident",
				Email:      "resident.jakarta@mistypass.local",
				Role:       "employee",
				Status:     "active",
				GroupIDs:   []string{"ug_common_office_jkt"},
				CreatedAt:  now,
			},
			{
				ID:         "usr_resident_siky_001",
				TenantID:   "tenant_demo_jakarta",
				BuildingID: "building_demo_001",
				Name:       "Siky",
				Email:      "siky",
				Role:       "employee",
				Status:     "active",
				GroupIDs:   []string{"ug_common_office_jkt"},
				CreatedAt:  now,
			},
		},
		userGroups: []UserGroup{
			{
				ID:                              "ug_common_office_jkt",
				ResourceType:                    "Group",
				TenantID:                        "tenant_demo_jakarta",
				BuildingID:                      "building_demo_001",
				PlaceID:                         "building_demo_001",
				Name:                            "Common Office Access",
				Description:                     "Default office/public access for regular employees",
				LoginEnabled:                    true,
				GeofenceRestrictionEnabled:      true,
				GeofenceRestrictionRadius:       150,
				PrimaryDeviceRestrictionEnabled: true,
				TapToAccessRestrictionEnabled:   true,
				Members:                         []string{"usr_1001"},
				CreatedAt:                       now,
				UpdatedAt:                       now,
			},
			{
				ID:          "ug_security_jkt",
				TenantID:    "tenant_demo_jakarta",
				BuildingID:  "building_demo_001",
				Name:        "Security Response Team",
				Description: "Security incident response and patrol access",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "ug_building_ops_jkt",
				TenantID:    "tenant_demo_jakarta",
				BuildingID:  "building_demo_001",
				Name:        "Building Operations",
				Description: "Facility maintenance and engineering access preset",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "ug_tenant_admin_jkt",
				TenantID:    "tenant_demo_jakarta",
				BuildingID:  "building_demo_001",
				Name:        "Tenant Platform Admin",
				Description: "Tenant-level management permissions",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "ug_common_office_fct",
				TenantID:    "tenant_demo_factory",
				BuildingID:  "building_demo_003",
				Name:        "Common Office Access",
				Description: "Default office/public access for factory employees",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "ug_security_fct",
				TenantID:    "tenant_demo_factory",
				BuildingID:  "building_demo_003",
				Name:        "Factory Security",
				Description: "Plant security operators and emergency response",
				Members:     []string{"usr_1002"},
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "ug_building_ops_fct",
				TenantID:    "tenant_demo_factory",
				BuildingID:  "building_demo_003",
				Name:        "Factory Operations",
				Description: "Factory engineering and facility operations preset",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "ug_tenant_admin_fct",
				TenantID:    "tenant_demo_factory",
				BuildingID:  "building_demo_003",
				Name:        "Factory Tenant Admin",
				Description: "Factory tenant-level management permissions",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		policies: []Policy{
			{
				ID:         "plc_1001",
				TenantID:   "tenant_demo_jakarta",
				Name:       "Finance Workhour Access",
				ScopeType:  "area",
				BuildingID: "building_demo_001",
				AreaID:     "area_demo_001",
				Schedule:   "Mon-Fri 07:00-19:00",
				Members:    86,
				Status:     "active",
				UpdatedAt:  now,
			},
			{
				ID:         "plc_1002",
				TenantID:   "tenant_demo_factory",
				Name:       "Packing Zone Strict Access",
				ScopeType:  "door",
				BuildingID: "building_demo_003",
				AreaID:     "area_demo_003",
				DoorID:     "door_fct_029",
				Schedule:   "On-duty only",
				Members:    9,
				Status:     "active",
				UpdatedAt:  now,
			},
		},
		temporaryAccess: []TemporaryAccess{
			{
				ID:                "tmp_7781",
				TenantID:          "tenant_demo_jakarta",
				ScopeType:         "door",
				BuildingID:        "building_demo_002",
				AreaID:            "area_demo_002",
				DoorID:            "door_jkt_014",
				DeliveryMethod:    "wallet",
				GranteeName:       "Andri Pratama",
				GranteeGender:     "male",
				GranteePhone:      "+62-811-1234-5678",
				GranteeEmail:      "andri.pratama@mistypass.local",
				MobileModel:       "Pixel 8",
				PassType:          "employee",
				ValidUntil:        "2026-04-11 20:00",
				AuthorizedByID:    "usr_tenant_admin_jkt_001",
				AuthorizedByEmail: "tenant.admin@sudirman.co",
				AuthorizedByRole:  "tenant_admin",
				AuthorizedAt:      now,
				CreatedAt:         now,
			},
		},
		visitorPasses: []VisitorPass{
			{
				ID:             "vst_2201",
				TenantID:       "tenant_demo_jakarta",
				BuildingID:     "building_demo_001",
				Host:           "Rina H.",
				Visitor:        "PT Arsitek Solusi",
				DeliveryMethod: "email_qr",
				ExpiresAt:      "2026-04-11 17:30",
				CreatedAt:      now,
			},
		},
		teams: []Team{
			{
				ID:           "team_engineering_jkt",
				ResourceType: "Team",
				TenantID:     "tenant_demo_jakarta",
				Name:         "Engineering Team",
				Scope:        "place",
				PlaceID:      "building_demo_001",
				Description:  "Office engineering staff managed through directory sync",
				Source:       "SCIM group: engineering",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				ID:           "team_operations_jkt",
				ResourceType: "Team",
				TenantID:     "tenant_demo_jakarta",
				Name:         "Operations Team",
				Scope:        "place",
				PlaceID:      "building_demo_001",
				Description:  "Building operations and facility support",
				Source:       "Manual",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				ID:           "team_factory_security",
				ResourceType: "Team",
				TenantID:     "tenant_demo_factory",
				Name:         "Factory Security Team",
				Scope:        "place",
				PlaceID:      "building_demo_003",
				Description:  "Plant security operators and emergency response",
				Source:       "Manual",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
		teamMemberships: []TeamMembership{
			{
				ID:           "tm_engineering_andri",
				ResourceType: "TeamMembership",
				TenantID:     "tenant_demo_jakarta",
				TeamID:       "team_engineering_jkt",
				MemberType:   "User",
				MemberID:     "usr_1001",
				MemberEmail:  "andri.pratama@mistypass.local",
				MemberName:   "Andri Pratama",
				Source:       "SCIM",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				ID:           "tm_operations_admin",
				ResourceType: "TeamMembership",
				TenantID:     "tenant_demo_jakarta",
				TeamID:       "team_operations_jkt",
				MemberType:   "User",
				MemberID:     "usr_place_admin_sudirman_001",
				MemberEmail:  "place.admin.sudirman@mistypass.local",
				MemberName:   "Sudirman Place Admin",
				Source:       "Manual",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			{
				ID:           "tm_factory_rina",
				ResourceType: "TeamMembership",
				TenantID:     "tenant_demo_factory",
				TeamID:       "team_factory_security",
				MemberType:   "User",
				MemberID:     "usr_1002",
				MemberEmail:  "rina.hartono@mistypass.local",
				MemberName:   "Rina Hartono",
				Source:       "Manual",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
		},
		roleAssignments: []RoleAssignment{
			{
				ID:            "ra_org_admin_jkt_001",
				TenantID:      "tenant_demo_jakarta",
				RoleID:        "role_organization_admin",
				AppliesToType: "Organization",
				AppliesToID:   "tenant_demo_jakarta",
				AssigneeType:  "User",
				AssigneeID:    "usr_organization_admin_jkt_001",
				AssigneeEmail: "organization.admin@mistypass.local",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
			{
				ID:            "ra_place_admin_sudirman_001",
				TenantID:      "tenant_demo_jakarta",
				RoleID:        "role_place_admin",
				AppliesToType: "Place",
				AppliesToID:   "building_demo_001",
				AssigneeType:  "User",
				AssigneeID:    "usr_place_admin_sudirman_001",
				AssigneeEmail: "place.admin.sudirman@mistypass.local",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
			{
				ID:            "ra_engineering_group_access_001",
				TenantID:      "tenant_demo_jakarta",
				RoleID:        "role_group_access",
				AppliesToType: "Group",
				AppliesToID:   "ug_common_office_jkt",
				AssigneeType:  "Team",
				AssigneeID:    "team_engineering_jkt",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
			{
				ID:            "ra_org_admin_fct_001",
				TenantID:      "tenant_demo_factory",
				RoleID:        "role_organization_admin",
				AppliesToType: "Organization",
				AppliesToID:   "tenant_demo_factory",
				AssigneeType:  "User",
				AssigneeID:    "usr_tenant_admin_fct_001",
				AssigneeEmail: "tenant.admin@factory.local",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
		groupLinks: []GroupLink{
			{
				ID:                     "gl_common_office_guest_jkt",
				ResourceType:           "GroupLink",
				TenantID:               "tenant_demo_jakarta",
				GroupID:                "ug_common_office_jkt",
				GroupName:              "Common Office Access",
				Name:                   "Common Office Guest Link",
				Email:                  "guest.common-office@mistypass.local",
				LinkEnabled:            true,
				QuickResponseCodeType:  "online",
				ValidUntil:             "2026-05-01T10:00:00Z",
				CreatedByType:          "User",
				CreatedByID:            "usr_organization_admin_jkt_001",
				CreatedByEmail:         "organization.admin@mistypass.local",
				Secret:                 "gls_demo_common_office",
				QuickResponseCodeToken: "glq_demo_common_office",
				CreatedAt:              now,
				UpdatedAt:              now,
			},
		},
		organizationSettings: make(map[string]OrganizationSettings),
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

func (s *Service) ListRoles() []Role {
	return builtInRoles()
}

func (s *Service) ListRoleAssignments(tenantID string) []RoleAssignment {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]RoleAssignment, 0, len(s.roleAssignments))
	for i := range s.roleAssignments {
		if filterTenantID != "" && s.roleAssignments[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.roleAssignments[i])
	}
	return items
}

func (s *Service) GetRoleAssignment(tenantID, assignmentID string) (RoleAssignment, error) {
	nextID := strings.TrimSpace(assignmentID)
	if nextID == "" {
		return RoleAssignment{}, ErrRoleAssignmentNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.roleAssignments {
		if s.roleAssignments[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.roleAssignments[i].TenantID != filterTenantID {
			return RoleAssignment{}, ErrRoleAssignmentNotFound
		}
		return s.roleAssignments[i], nil
	}
	return RoleAssignment{}, ErrRoleAssignmentNotFound
}

func (s *Service) CreateRoleAssignment(input RoleAssignmentInput) (RoleAssignment, error) {
	normalized, err := normalizeRoleAssignmentInput(input)
	if err != nil {
		return RoleAssignment{}, err
	}

	id, err := accessID("ra_")
	if err != nil {
		return RoleAssignment{}, err
	}

	now := time.Now().UTC()
	record := RoleAssignment{
		ID:            id,
		TenantID:      normalized.TenantID,
		RoleID:        normalized.RoleID,
		AppliesToType: normalized.AppliesToType,
		AppliesToID:   normalized.AppliesToID,
		AssigneeType:  normalized.AssigneeType,
		AssigneeID:    normalized.AssigneeID,
		AssigneeEmail: normalizeEmail(normalized.AssigneeEmail),
		ValidFrom:     strings.TrimSpace(normalized.ValidFrom),
		ValidUntil:    strings.TrimSpace(normalized.ValidUntil),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.mu.Lock()
	s.roleAssignments = append([]RoleAssignment{record}, s.roleAssignments...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return RoleAssignment{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) UpdateRoleAssignment(tenantID, assignmentID string, input RoleAssignmentInput) (RoleAssignment, error) {
	nextID := strings.TrimSpace(assignmentID)
	if nextID == "" {
		return RoleAssignment{}, ErrRoleAssignmentNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)
	input.TenantID = firstNonEmpty(input.TenantID, filterTenantID)
	normalized, err := normalizeRoleAssignmentInput(input)
	if err != nil {
		return RoleAssignment{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.roleAssignments {
		if s.roleAssignments[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.roleAssignments[i].TenantID != filterTenantID {
			return RoleAssignment{}, ErrRoleAssignmentNotFound
		}

		s.roleAssignments[i].TenantID = normalized.TenantID
		s.roleAssignments[i].RoleID = normalized.RoleID
		s.roleAssignments[i].AppliesToType = normalized.AppliesToType
		s.roleAssignments[i].AppliesToID = normalized.AppliesToID
		s.roleAssignments[i].AssigneeType = normalized.AssigneeType
		s.roleAssignments[i].AssigneeID = normalized.AssigneeID
		s.roleAssignments[i].AssigneeEmail = normalizeEmail(normalized.AssigneeEmail)
		s.roleAssignments[i].ValidFrom = strings.TrimSpace(normalized.ValidFrom)
		s.roleAssignments[i].ValidUntil = strings.TrimSpace(normalized.ValidUntil)
		s.roleAssignments[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return RoleAssignment{}, err
		}
		return s.roleAssignments[i], nil
	}

	return RoleAssignment{}, ErrRoleAssignmentNotFound
}

func (s *Service) DeleteRoleAssignment(tenantID, assignmentID string) error {
	nextID := strings.TrimSpace(assignmentID)
	if nextID == "" {
		return ErrRoleAssignmentNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.roleAssignments {
		if s.roleAssignments[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.roleAssignments[i].TenantID != filterTenantID {
			return ErrRoleAssignmentNotFound
		}
		original := cloneRoleAssignments(s.roleAssignments)
		s.roleAssignments = append(s.roleAssignments[:i], s.roleAssignments[i+1:]...)
		if err := s.persistLocked(); err != nil {
			s.roleAssignments = original
			return err
		}
		return nil
	}
	return ErrRoleAssignmentNotFound
}

func (s *Service) ReviewAccessRights(tenantID string, roleAssignmentIDs, shareIDs []string, reviewedBy string) (AccessRightsReviewResult, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return AccessRightsReviewResult{}, ErrTenantIDRequired
	}
	nextRoleAssignmentIDs := uniqueIDs(roleAssignmentIDs)
	nextShareIDs := uniqueIDs(shareIDs)
	if len(nextRoleAssignmentIDs) == 0 && len(nextShareIDs) == 0 {
		return AccessRightsReviewResult{}, ErrAccessRightSelectionRequired
	}
	nextReviewedBy := strings.TrimSpace(reviewedBy)
	if nextReviewedBy == "" {
		nextReviewedBy = "system"
	}
	reviewedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z07:00")

	s.mu.Lock()
	defer s.mu.Unlock()

	roleAssignmentIndexes := make([]int, 0, len(nextRoleAssignmentIDs))
	for _, assignmentID := range nextRoleAssignmentIDs {
		found := -1
		for i := range s.roleAssignments {
			if s.roleAssignments[i].ID != assignmentID {
				continue
			}
			if s.roleAssignments[i].TenantID != nextTenantID {
				return AccessRightsReviewResult{}, ErrRoleAssignmentNotFound
			}
			found = i
			break
		}
		if found < 0 {
			return AccessRightsReviewResult{}, ErrRoleAssignmentNotFound
		}
		roleAssignmentIndexes = append(roleAssignmentIndexes, found)
	}

	shareIndexes := make([]int, 0, len(nextShareIDs))
	for _, shareID := range nextShareIDs {
		found := -1
		for i := range s.temporaryAccess {
			if s.temporaryAccess[i].ID != shareID {
				continue
			}
			if s.temporaryAccess[i].TenantID != nextTenantID {
				return AccessRightsReviewResult{}, ErrTemporaryAccessNotFound
			}
			found = i
			break
		}
		if found < 0 {
			return AccessRightsReviewResult{}, ErrTemporaryAccessNotFound
		}
		shareIndexes = append(shareIndexes, found)
	}

	originalAssignments := cloneRoleAssignments(s.roleAssignments)
	originalShares := cloneTemporaryAccess(s.temporaryAccess)
	result := AccessRightsReviewResult{
		TenantID:      nextTenantID,
		ReviewedAt:    reviewedAt,
		ReviewedBy:    nextReviewedBy,
		ReviewedCount: 0,
		SkippedCount:  0,
	}

	for _, index := range roleAssignmentIndexes {
		if strings.TrimSpace(s.roleAssignments[index].ReviewedAt) != "" {
			result.SkippedCount++
			continue
		}
		s.roleAssignments[index].ReviewedAt = reviewedAt
		s.roleAssignments[index].ReviewedBy = nextReviewedBy
		s.roleAssignments[index].UpdatedAt = time.Now().UTC()
		result.ReviewedRoleAssignmentIDs = append(result.ReviewedRoleAssignmentIDs, s.roleAssignments[index].ID)
		result.ReviewedCount++
	}
	for _, index := range shareIndexes {
		if strings.TrimSpace(s.temporaryAccess[index].ReviewedAt) != "" {
			result.SkippedCount++
			continue
		}
		s.temporaryAccess[index].ReviewedAt = reviewedAt
		s.temporaryAccess[index].ReviewedBy = nextReviewedBy
		result.ReviewedShareIDs = append(result.ReviewedShareIDs, s.temporaryAccess[index].ID)
		result.ReviewedCount++
	}

	if err := s.persistLocked(); err != nil {
		s.roleAssignments = originalAssignments
		s.temporaryAccess = originalShares
		return AccessRightsReviewResult{}, err
	}
	return result, nil
}

func (s *Service) ListUsers(tenantID string) []AccessUser {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]AccessUser, 0, len(s.users))
	for i := range s.users {
		if filterTenantID != "" && s.users[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.users[i])
	}
	return items
}

func (s *Service) GetUser(tenantID, userID string) (AccessUser, error) {
	nextID := strings.TrimSpace(userID)
	if nextID == "" {
		return AccessUser{}, ErrUserNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.users {
		if s.users[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.users[i].TenantID != filterTenantID {
			return AccessUser{}, ErrUserNotFound
		}
		return cloneAccessUser(s.users[i]), nil
	}
	return AccessUser{}, ErrUserNotFound
}

func (s *Service) CreateUser(tenantID, buildingID, name, email, role, status string, groupIDs []string) (AccessUser, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return AccessUser{}, ErrTenantIDRequired
	}

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return AccessUser{}, ErrUserNameRequired
	}

	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return AccessUser{}, ErrUserEmailRequired
	}

	nextStatus, err := normalizeUserStatus(status)
	if err != nil {
		return AccessUser{}, err
	}

	nextRole := strings.ToLower(strings.TrimSpace(role))
	if nextRole == "" {
		nextRole = "employee"
	}

	id, err := accessID("usr_")
	if err != nil {
		return AccessUser{}, err
	}

	record := AccessUser{
		ID:         id,
		TenantID:   nextTenantID,
		BuildingID: strings.TrimSpace(buildingID),
		Name:       nextName,
		Email:      nextEmail,
		Role:       nextRole,
		Status:     nextStatus,
		GroupIDs:   uniqueIDs(groupIDs),
		CreatedAt:  time.Now().UTC(),
	}

	s.mu.Lock()
	s.users = append([]AccessUser{record}, s.users...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return AccessUser{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) UpdateUser(tenantID, userID, buildingID, name, email, role, status string, groupIDs []string) (AccessUser, error) {
	nextID := strings.TrimSpace(userID)
	if nextID == "" {
		return AccessUser{}, ErrUserNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return AccessUser{}, ErrUserNameRequired
	}

	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return AccessUser{}, ErrUserEmailRequired
	}

	nextStatus, err := normalizeUserStatus(status)
	if err != nil {
		return AccessUser{}, err
	}

	nextRole := strings.ToLower(strings.TrimSpace(role))
	if nextRole == "" {
		nextRole = "employee"
	}
	nextBuildingID := strings.TrimSpace(buildingID)
	nextGroupIDs := uniqueIDs(groupIDs)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.users {
		if s.users[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.users[i].TenantID != filterTenantID {
			return AccessUser{}, ErrUserNotFound
		}

		s.users[i].BuildingID = nextBuildingID
		s.users[i].Name = nextName
		s.users[i].Email = nextEmail
		s.users[i].Role = nextRole
		s.users[i].Status = nextStatus
		s.users[i].GroupIDs = nextGroupIDs
		if err := s.persistLocked(); err != nil {
			return AccessUser{}, err
		}
		return cloneAccessUser(s.users[i]), nil
	}

	return AccessUser{}, ErrUserNotFound
}

func (s *Service) DeleteUser(tenantID, userID string) (AccessUser, error) {
	nextID := strings.TrimSpace(userID)
	if nextID == "" {
		return AccessUser{}, ErrUserNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	foundIndex := -1
	for i := range s.users {
		if s.users[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.users[i].TenantID != filterTenantID {
			return AccessUser{}, ErrUserNotFound
		}
		foundIndex = i
		break
	}
	if foundIndex == -1 {
		return AccessUser{}, ErrUserNotFound
	}

	originalUsers := cloneAccessUsers(s.users)
	originalGroups := cloneUserGroups(s.userGroups)
	originalRoleAssignments := cloneRoleAssignments(s.roleAssignments)
	originalTeamMemberships := cloneTeamMemberships(s.teamMemberships)

	removed := cloneAccessUser(s.users[foundIndex])
	s.users = append(s.users[:foundIndex], s.users[foundIndex+1:]...)
	for i := range s.userGroups {
		s.userGroups[i].Members = removeID(s.userGroups[i].Members, nextID)
	}

	nextAssignments := make([]RoleAssignment, 0, len(s.roleAssignments))
	for i := range s.roleAssignments {
		if strings.EqualFold(s.roleAssignments[i].AssigneeType, "User") && s.roleAssignments[i].AssigneeID == nextID {
			continue
		}
		nextAssignments = append(nextAssignments, s.roleAssignments[i])
	}
	s.roleAssignments = nextAssignments

	nextMemberships := make([]TeamMembership, 0, len(s.teamMemberships))
	for i := range s.teamMemberships {
		if strings.EqualFold(s.teamMemberships[i].MemberType, "User") && s.teamMemberships[i].MemberID == nextID {
			continue
		}
		nextMemberships = append(nextMemberships, s.teamMemberships[i])
	}
	s.teamMemberships = nextMemberships

	if err := s.persistLocked(); err != nil {
		s.users = originalUsers
		s.userGroups = originalGroups
		s.roleAssignments = originalRoleAssignments
		s.teamMemberships = originalTeamMemberships
		return AccessUser{}, err
	}
	return removed, nil
}

func (s *Service) BatchUpdateUserStatus(tenantID string, userIDs []string, status string) (BatchUserStatusResult, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	if filterTenantID == "" {
		return BatchUserStatusResult{}, ErrTenantIDRequired
	}
	if len(userIDs) == 0 {
		return BatchUserStatusResult{}, ErrUserIDsRequired
	}
	nextStatus, err := normalizeUserStatus(status)
	if err != nil {
		return BatchUserStatusResult{}, err
	}

	targetSet := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		if v := strings.TrimSpace(id); v != "" {
			targetSet[v] = struct{}{}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var updated, skipped, notFound int
	updatedIDs := make([]string, 0, len(targetSet))
	found := make(map[string]struct{}, len(targetSet))

	for i := range s.users {
		if s.users[i].TenantID != filterTenantID {
			continue
		}
		if _, ok := targetSet[s.users[i].ID]; !ok {
			continue
		}
		found[s.users[i].ID] = struct{}{}
		if strings.EqualFold(s.users[i].Status, nextStatus) {
			skipped++
			continue
		}
		s.users[i].Status = nextStatus
		updated++
		updatedIDs = append(updatedIDs, s.users[i].ID)
	}
	notFound = len(targetSet) - len(found)

	if updated > 0 {
		if err := s.persistLocked(); err != nil {
			return BatchUserStatusResult{}, err
		}
	}

	return BatchUserStatusResult{
		TenantID: filterTenantID,
		Status:   nextStatus,
		Updated:  updated,
		Skipped:  skipped,
		NotFound: notFound,
		UserIDs:  updatedIDs,
	}, nil
}

func (s *Service) BatchDeleteUsers(tenantID string, userIDs []string) (BatchUserDeleteResult, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	if filterTenantID == "" {
		return BatchUserDeleteResult{}, ErrTenantIDRequired
	}
	if len(userIDs) == 0 {
		return BatchUserDeleteResult{}, ErrUserIDsRequired
	}

	targetSet := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		if v := strings.TrimSpace(id); v != "" {
			targetSet[v] = struct{}{}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var deleted, notFound int
	deletedIDs := make([]string, 0, len(targetSet))
	nextUsers := make([]AccessUser, 0, len(s.users))
	removedIDs := make(map[string]struct{}, len(targetSet))

	for i := range s.users {
		if s.users[i].TenantID != filterTenantID {
			nextUsers = append(nextUsers, s.users[i])
			continue
		}
		if _, ok := targetSet[s.users[i].ID]; !ok {
			nextUsers = append(nextUsers, s.users[i])
			continue
		}
		removedIDs[s.users[i].ID] = struct{}{}
		deleted++
		deletedIDs = append(deletedIDs, s.users[i].ID)
	}
	notFound = len(targetSet) - deleted

	if deleted > 0 {
		s.users = nextUsers
		// clean up group memberships
		for i := range s.userGroups {
			for rid := range removedIDs {
				s.userGroups[i].Members = removeID(s.userGroups[i].Members, rid)
			}
		}
		// clean up role assignments
		nextAssignments := make([]RoleAssignment, 0, len(s.roleAssignments))
		for i := range s.roleAssignments {
			if strings.EqualFold(s.roleAssignments[i].AssigneeType, "User") {
				if _, ok := removedIDs[s.roleAssignments[i].AssigneeID]; ok {
					continue
				}
			}
			nextAssignments = append(nextAssignments, s.roleAssignments[i])
		}
		s.roleAssignments = nextAssignments
		// clean up team memberships
		nextMemberships := make([]TeamMembership, 0, len(s.teamMemberships))
		for i := range s.teamMemberships {
			if strings.EqualFold(s.teamMemberships[i].MemberType, "User") {
				if _, ok := removedIDs[s.teamMemberships[i].MemberID]; ok {
					continue
				}
			}
			nextMemberships = append(nextMemberships, s.teamMemberships[i])
		}
		s.teamMemberships = nextMemberships
		if err := s.persistLocked(); err != nil {
			return BatchUserDeleteResult{}, err
		}
	}

	return BatchUserDeleteResult{
		TenantID: filterTenantID,
		Deleted:  deleted,
		NotFound: notFound,
		UserIDs:  deletedIDs,
	}, nil
}

func (s *Service) BatchInviteUsers(tenantID string, userIDs []string, deliveryMethod string) (BatchUserInviteResult, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	if filterTenantID == "" {
		return BatchUserInviteResult{}, ErrTenantIDRequired
	}
	if len(userIDs) == 0 {
		return BatchUserInviteResult{}, ErrUserIDsRequired
	}
	nextMethod := strings.ToLower(strings.TrimSpace(deliveryMethod))
	if nextMethod == "" {
		nextMethod = "email"
	}
	if nextMethod != "email" && nextMethod != "email_qr" {
		return BatchUserInviteResult{}, ErrDeliveryMethodInvalid
	}

	targetSet := make(map[string]struct{}, len(userIDs))
	for _, id := range userIDs {
		if v := strings.TrimSpace(id); v != "" {
			targetSet[v] = struct{}{}
		}
	}

	var queued, skipped, notFound int
	queuedIDs := make([]string, 0, len(targetSet))

	s.mu.RLock()
	users := cloneAccessUsers(s.users)
	s.mu.RUnlock()

	found := make(map[string]struct{}, len(targetSet))
	for i := range users {
		if users[i].TenantID != filterTenantID {
			continue
		}
		if _, ok := targetSet[users[i].ID]; !ok {
			continue
		}
		found[users[i].ID] = struct{}{}
		if strings.EqualFold(users[i].Status, "suspended") || strings.EqualFold(users[i].Status, "disabled") {
			skipped++
			continue
		}
		_, _, err := s.CreateUserInvitationDelivery(filterTenantID, users[i].ID, nextMethod)
		if err != nil {
			skipped++
			continue
		}
		queued++
		queuedIDs = append(queuedIDs, users[i].ID)
	}
	notFound = len(targetSet) - len(found)

	return BatchUserInviteResult{
		TenantID: filterTenantID,
		Queued:   queued,
		Skipped:  skipped,
		NotFound: notFound,
		UserIDs:  queuedIDs,
	}, nil
}

func (s *Service) ExportUsersCSV(tenantID string) (string, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	if filterTenantID == "" {
		return "", ErrTenantIDRequired
	}

	s.mu.RLock()
	users := cloneAccessUsers(s.users)
	s.mu.RUnlock()

	var buf strings.Builder
	buf.WriteString("id,name,email,role,status,building_id,sync_source,created_at\n")
	for i := range users {
		if users[i].TenantID != filterTenantID {
			continue
		}
		buf.WriteString(csvEscape(users[i].ID))
		buf.WriteByte(',')
		buf.WriteString(csvEscape(users[i].Name))
		buf.WriteByte(',')
		buf.WriteString(csvEscape(users[i].Email))
		buf.WriteByte(',')
		buf.WriteString(csvEscape(users[i].Role))
		buf.WriteByte(',')
		buf.WriteString(csvEscape(users[i].Status))
		buf.WriteByte(',')
		buf.WriteString(csvEscape(users[i].BuildingID))
		buf.WriteByte(',')
		buf.WriteString(csvEscape(users[i].SyncSource))
		buf.WriteByte(',')
		buf.WriteString(users[i].CreatedAt.UTC().Format(time.RFC3339))
		buf.WriteByte('\n')
	}
	return buf.String(), nil
}

func (s *Service) ImportUsersCSV(tenantID string, records []UserImportRecord) (UserImportResult, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	if filterTenantID == "" {
		return UserImportResult{}, ErrTenantIDRequired
	}
	if len(records) == 0 {
		return UserImportResult{}, ErrUsersImportRecordsRequired
	}

	var created, updated, skipped, errCount int

	for i := range records {
		email := normalizeEmail(records[i].Email)
		if email == "" {
			errCount++
			continue
		}
		name := strings.TrimSpace(records[i].Name)
		if name == "" {
			name = email
		}
		role := strings.ToLower(strings.TrimSpace(records[i].Role))
		if role == "" {
			role = "employee"
		}
		status := strings.ToLower(strings.TrimSpace(records[i].Status))
		if status == "" {
			status = "active"
		}
		buildingID := strings.TrimSpace(records[i].BuildingID)

		_, isNew, err := s.UpsertUserByEmail(
			filterTenantID,
			buildingID,
			name,
			email,
			role,
			status,
			nil,
		)
		if err != nil {
			errCount++
			continue
		}
		if isNew {
			created++
		} else {
			updated++
		}
	}

	return UserImportResult{
		TenantID: filterTenantID,
		Created:  created,
		Updated:  updated,
		Skipped:  skipped,
		Errors:   errCount,
	}, nil
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func (s *Service) ListUserInvitationDeliveries(tenantID, userID string) []UserInvitationDelivery {
	filterTenantID := strings.TrimSpace(tenantID)
	filterUserID := strings.TrimSpace(userID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]UserInvitationDelivery, 0, len(s.userInvitationDeliveries))
	for i := range s.userInvitationDeliveries {
		if filterTenantID != "" && s.userInvitationDeliveries[i].TenantID != filterTenantID {
			continue
		}
		if filterUserID != "" && s.userInvitationDeliveries[i].UserID != filterUserID {
			continue
		}
		items = append(items, cloneUserInvitationDelivery(s.userInvitationDeliveries[i]))
	}
	return items
}

func (s *Service) CreateUserInvitationDelivery(tenantID, userID, deliveryMethod string) (UserInvitationDelivery, AccessUser, error) {
	nextUserID := strings.TrimSpace(userID)
	if nextUserID == "" {
		return UserInvitationDelivery{}, AccessUser{}, ErrUserNotFound
	}
	nextMethod, err := normalizeUserInvitationDeliveryMethod(deliveryMethod)
	if err != nil {
		return UserInvitationDelivery{}, AccessUser{}, err
	}
	filterTenantID := strings.TrimSpace(tenantID)

	id, err := accessID("user_invite_")
	if err != nil {
		return UserInvitationDelivery{}, AccessUser{}, err
	}
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	userIndex := -1
	for i := range s.users {
		if s.users[i].ID != nextUserID {
			continue
		}
		if filterTenantID != "" && s.users[i].TenantID != filterTenantID {
			return UserInvitationDelivery{}, AccessUser{}, ErrUserNotFound
		}
		userIndex = i
		break
	}
	if userIndex < 0 {
		return UserInvitationDelivery{}, AccessUser{}, ErrUserNotFound
	}

	user := cloneAccessUser(s.users[userIndex])
	record := UserInvitationDelivery{
		ID:             id,
		ResourceType:   "UserInvitationDelivery",
		TenantID:       user.TenantID,
		UserID:         user.ID,
		Email:          user.Email,
		PlaceID:        user.BuildingID,
		DeliveryMethod: nextMethod,
		Status:         "queued",
		QueuedAt:       now,
		UpdatedAt:      now,
	}
	s.userInvitationDeliveries = append([]UserInvitationDelivery{record}, s.userInvitationDeliveries...)
	if err := s.persistLocked(); err != nil {
		return UserInvitationDelivery{}, AccessUser{}, err
	}
	return cloneUserInvitationDelivery(record), user, nil
}

func (s *Service) RecordUserInvitationReceipt(
	tenantID, userID, deliveryID, status, provider, providerDeliveryID, providerError string,
	retryable bool,
) (UserInvitationDelivery, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextUserID := strings.TrimSpace(userID)
	nextDeliveryID := strings.TrimSpace(deliveryID)
	if nextUserID == "" {
		return UserInvitationDelivery{}, ErrUserNotFound
	}
	if nextDeliveryID == "" {
		return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
	}
	nextStatus, err := normalizeUserInvitationStatus(status)
	if err != nil {
		return UserInvitationDelivery{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	deliveryIndex := -1
	for i := range s.userInvitationDeliveries {
		if s.userInvitationDeliveries[i].ID != nextDeliveryID || s.userInvitationDeliveries[i].UserID != nextUserID {
			continue
		}
		if filterTenantID != "" && s.userInvitationDeliveries[i].TenantID != filterTenantID {
			return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
		}
		deliveryIndex = i
		break
	}
	if deliveryIndex < 0 {
		return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
	}

	now := time.Now().UTC()
	s.userInvitationDeliveries[deliveryIndex].Status = nextStatus
	s.userInvitationDeliveries[deliveryIndex].Provider = strings.TrimSpace(provider)
	s.userInvitationDeliveries[deliveryIndex].ProviderDeliveryID = strings.TrimSpace(providerDeliveryID)
	s.userInvitationDeliveries[deliveryIndex].ProviderError = strings.TrimSpace(providerError)
	s.userInvitationDeliveries[deliveryIndex].Retryable = retryable
	s.userInvitationDeliveries[deliveryIndex].UpdatedAt = now
	if nextStatus == "sent" {
		deliveredAt := now
		s.userInvitationDeliveries[deliveryIndex].DeliveredAt = &deliveredAt
	}
	if err := s.persistLocked(); err != nil {
		return UserInvitationDelivery{}, err
	}
	return cloneUserInvitationDelivery(s.userInvitationDeliveries[deliveryIndex]), nil
}

// --- Independent invitation resource methods ---

func (s *Service) GetInvitationDelivery(tenantID, deliveryID string) (UserInvitationDelivery, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextDeliveryID := strings.TrimSpace(deliveryID)
	if nextDeliveryID == "" {
		return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.userInvitationDeliveries {
		if s.userInvitationDeliveries[i].ID != nextDeliveryID {
			continue
		}
		if filterTenantID != "" && s.userInvitationDeliveries[i].TenantID != filterTenantID {
			continue
		}
		return cloneUserInvitationDelivery(s.userInvitationDeliveries[i]), nil
	}
	return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
}

func (s *Service) CancelInvitationDelivery(tenantID, deliveryID string) (UserInvitationDelivery, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextDeliveryID := strings.TrimSpace(deliveryID)
	if nextDeliveryID == "" {
		return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.userInvitationDeliveries {
		if s.userInvitationDeliveries[i].ID != nextDeliveryID {
			continue
		}
		if filterTenantID != "" && s.userInvitationDeliveries[i].TenantID != filterTenantID {
			continue
		}
		if s.userInvitationDeliveries[i].Status == "sent" {
			return UserInvitationDelivery{}, errors.New("cannot cancel a delivered invitation")
		}
		if s.userInvitationDeliveries[i].Status == "cancelled" {
			return cloneUserInvitationDelivery(s.userInvitationDeliveries[i]), nil
		}
		s.userInvitationDeliveries[i].Status = "cancelled"
		s.userInvitationDeliveries[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return UserInvitationDelivery{}, err
		}
		return cloneUserInvitationDelivery(s.userInvitationDeliveries[i]), nil
	}
	return UserInvitationDelivery{}, ErrUserInvitationDeliveryNotFound
}

func (s *Service) ResendInvitationDelivery(tenantID, deliveryID string) (UserInvitationDelivery, AccessUser, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextDeliveryID := strings.TrimSpace(deliveryID)
	if nextDeliveryID == "" {
		return UserInvitationDelivery{}, AccessUser{}, ErrUserInvitationDeliveryNotFound
	}

	s.mu.RLock()
	var original *UserInvitationDelivery
	for i := range s.userInvitationDeliveries {
		if s.userInvitationDeliveries[i].ID != nextDeliveryID {
			continue
		}
		if filterTenantID != "" && s.userInvitationDeliveries[i].TenantID != filterTenantID {
			continue
		}
		d := cloneUserInvitationDelivery(s.userInvitationDeliveries[i])
		original = &d
		break
	}
	s.mu.RUnlock()

	if original == nil {
		return UserInvitationDelivery{}, AccessUser{}, ErrUserInvitationDeliveryNotFound
	}
	return s.CreateUserInvitationDelivery(original.TenantID, original.UserID, original.DeliveryMethod)
}

func (s *Service) UpsertUserByEmail(
	tenantID, buildingID, name, email, role, status string,
	groupIDs []string,
) (AccessUser, bool, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return AccessUser{}, false, ErrTenantIDRequired
	}

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return AccessUser{}, false, ErrUserNameRequired
	}

	nextEmail := normalizeEmail(email)
	if nextEmail == "" {
		return AccessUser{}, false, ErrUserEmailRequired
	}

	nextStatus, err := normalizeUserStatus(status)
	if err != nil {
		return AccessUser{}, false, err
	}

	nextRole := strings.ToLower(strings.TrimSpace(role))
	if nextRole == "" {
		nextRole = "employee"
	}

	nextBuildingID := strings.TrimSpace(buildingID)
	nextGroupIDs := uniqueIDs(groupIDs)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.users {
		if s.users[i].TenantID != nextTenantID {
			continue
		}
		if normalizeEmail(s.users[i].Email) != nextEmail {
			continue
		}

		s.users[i].BuildingID = nextBuildingID
		s.users[i].Name = nextName
		s.users[i].Email = nextEmail
		s.users[i].Role = nextRole
		s.users[i].Status = nextStatus
		s.users[i].GroupIDs = nextGroupIDs
		if err := s.persistLocked(); err != nil {
			return AccessUser{}, false, err
		}

		return s.users[i], false, nil
	}

	id, err := accessID("usr_")
	if err != nil {
		return AccessUser{}, false, err
	}

	record := AccessUser{
		ID:         id,
		TenantID:   nextTenantID,
		BuildingID: nextBuildingID,
		Name:       nextName,
		Email:      nextEmail,
		Role:       nextRole,
		Status:     nextStatus,
		GroupIDs:   nextGroupIDs,
		CreatedAt:  time.Now().UTC(),
	}
	s.users = append([]AccessUser{record}, s.users...)
	if err := s.persistLocked(); err != nil {
		return AccessUser{}, false, err
	}

	return record, true, nil
}

func (s *Service) UpsertUsersByEmail(
	tenantID string,
	inputs []BatchUpsertUserByEmailInput,
) (created, updated, rejected int, err error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return 0, 0, 0, ErrTenantIDRequired
	}
	if len(inputs) == 0 {
		return 0, 0, 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	originalUsers := cloneAccessUsers(s.users)
	existingByEmail := make(map[string]int)
	existingBySyncRef := make(map[string]int)
	for i := range s.users {
		if s.users[i].TenantID != nextTenantID {
			continue
		}
		existingByEmail[normalizeEmail(s.users[i].Email)] = i
		if syncKey := accessSyncIdentityKey(s.users[i].SyncSource, s.users[i].SyncRef); syncKey != "" {
			existingBySyncRef[syncKey] = i
		}
	}

	createdRecords := make([]AccessUser, 0, len(inputs))
	createdByEmail := make(map[string]int)
	createdBySyncRef := make(map[string]int)

	for i := range inputs {
		nextName := strings.TrimSpace(inputs[i].Name)
		nextEmail := normalizeEmail(inputs[i].Email)
		if nextName == "" || nextEmail == "" {
			rejected++
			continue
		}

		nextStatus, normalizeErr := normalizeUserStatus(inputs[i].Status)
		if normalizeErr != nil {
			rejected++
			continue
		}

		nextRole := strings.ToLower(strings.TrimSpace(inputs[i].Role))
		if nextRole == "" {
			nextRole = "employee"
		}
		nextBuildingID := strings.TrimSpace(inputs[i].BuildingID)
		nextGroupIDs := uniqueIDs(inputs[i].GroupIDs)
		nextSyncSource := normalizeSyncSource(inputs[i].SyncSource)
		nextSyncRef := strings.TrimSpace(inputs[i].SyncRef)
		if (nextSyncSource == "") != (nextSyncRef == "") {
			rejected++
			continue
		}
		nextSyncKey := accessSyncIdentityKey(nextSyncSource, nextSyncRef)

		existingSyncIndex, hasExistingSyncMatch := -1, false
		if nextSyncKey != "" {
			existingSyncIndex, hasExistingSyncMatch = existingBySyncRef[nextSyncKey]
		}
		createdSyncIndex, hasCreatedSyncMatch := -1, false
		if nextSyncKey != "" {
			createdSyncIndex, hasCreatedSyncMatch = createdBySyncRef[nextSyncKey]
		}

		existingEmailIndex, hasExistingEmailMatch := existingByEmail[nextEmail]
		createdEmailIndex, hasCreatedEmailMatch := createdByEmail[nextEmail]

		hasSyncMatch := hasExistingSyncMatch || hasCreatedSyncMatch
		hasEmailMatch := hasExistingEmailMatch || hasCreatedEmailMatch

		targetIsCreated := false
		targetIndex := -1
		switch {
		case hasSyncMatch && hasEmailMatch:
			switch {
			case hasExistingSyncMatch && hasExistingEmailMatch && existingSyncIndex == existingEmailIndex:
				targetIndex = existingSyncIndex
			case hasCreatedSyncMatch && hasCreatedEmailMatch && createdSyncIndex == createdEmailIndex:
				targetIndex = createdSyncIndex
				targetIsCreated = true
			default:
				rejected++
				continue
			}
		case hasSyncMatch:
			if hasExistingSyncMatch {
				targetIndex = existingSyncIndex
			} else {
				targetIndex = createdSyncIndex
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
			currentSyncKey := accessSyncIdentityKey(createdRecords[targetIndex].SyncSource, createdRecords[targetIndex].SyncRef)
			if nextSyncKey != "" && currentSyncKey != "" && nextSyncKey != currentSyncKey {
				rejected++
				continue
			}
			previousEmail := normalizeEmail(createdRecords[targetIndex].Email)
			createdRecords[targetIndex].BuildingID = nextBuildingID
			createdRecords[targetIndex].Name = nextName
			createdRecords[targetIndex].Email = nextEmail
			createdRecords[targetIndex].Role = nextRole
			createdRecords[targetIndex].Status = nextStatus
			createdRecords[targetIndex].GroupIDs = nextGroupIDs
			if nextSyncKey != "" {
				createdRecords[targetIndex].SyncSource = nextSyncSource
				createdRecords[targetIndex].SyncRef = nextSyncRef
				createdBySyncRef[nextSyncKey] = targetIndex
			}
			if previousEmail != nextEmail {
				if previousIndex, exists := createdByEmail[previousEmail]; exists && previousIndex == targetIndex {
					delete(createdByEmail, previousEmail)
				}
			}
			createdByEmail[nextEmail] = targetIndex
			continue
		}

		if targetIndex >= 0 {
			currentSyncKey := accessSyncIdentityKey(s.users[targetIndex].SyncSource, s.users[targetIndex].SyncRef)
			if nextSyncKey != "" && currentSyncKey != "" && nextSyncKey != currentSyncKey {
				rejected++
				continue
			}
			previousEmail := normalizeEmail(s.users[targetIndex].Email)
			s.users[targetIndex].BuildingID = nextBuildingID
			s.users[targetIndex].Name = nextName
			s.users[targetIndex].Email = nextEmail
			s.users[targetIndex].Role = nextRole
			s.users[targetIndex].Status = nextStatus
			s.users[targetIndex].GroupIDs = nextGroupIDs
			if nextSyncKey != "" {
				s.users[targetIndex].SyncSource = nextSyncSource
				s.users[targetIndex].SyncRef = nextSyncRef
				existingBySyncRef[nextSyncKey] = targetIndex
			}
			if previousEmail != nextEmail {
				if previousIndex, exists := existingByEmail[previousEmail]; exists && previousIndex == targetIndex {
					delete(existingByEmail, previousEmail)
				}
			}
			existingByEmail[nextEmail] = targetIndex
			updated++
			continue
		}

		id, idErr := accessID("usr_")
		if idErr != nil {
			s.users = originalUsers
			return 0, 0, 0, idErr
		}

		record := AccessUser{
			ID:         id,
			TenantID:   nextTenantID,
			BuildingID: nextBuildingID,
			Name:       nextName,
			Email:      nextEmail,
			Role:       nextRole,
			Status:     nextStatus,
			GroupIDs:   nextGroupIDs,
			SyncSource: nextSyncSource,
			SyncRef:    nextSyncRef,
			CreatedAt:  time.Now().UTC(),
		}
		createdIndex := len(createdRecords)
		createdByEmail[nextEmail] = createdIndex
		if nextSyncKey != "" {
			createdBySyncRef[nextSyncKey] = createdIndex
		}
		createdRecords = append(createdRecords, record)
		created++
	}

	if len(createdRecords) > 0 {
		s.users = append(createdRecords, s.users...)
	}

	if created > 0 || updated > 0 {
		if persistErr := s.persistLocked(); persistErr != nil {
			s.users = originalUsers
			return 0, 0, 0, persistErr
		}
	}

	return created, updated, rejected, nil
}

func (s *Service) ListUserGroups(tenantID string) []UserGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]UserGroup, 0, len(s.userGroups))
	for i := range s.userGroups {
		if filterTenantID != "" && s.userGroups[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, cloneUserGroup(s.userGroups[i]))
	}
	return items
}

func (s *Service) GetUserGroup(tenantID, groupID string) (UserGroup, error) {
	nextID := strings.TrimSpace(groupID)
	if nextID == "" {
		return UserGroup{}, ErrUserGroupNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.userGroups {
		if s.userGroups[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.userGroups[i].TenantID != filterTenantID {
			return UserGroup{}, ErrUserGroupNotFound
		}
		return cloneUserGroup(s.userGroups[i]), nil
	}

	return UserGroup{}, ErrUserGroupNotFound
}

func (s *Service) ListTeams(tenantID string) []Team {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]Team, 0, len(s.teams))
	for i := range s.teams {
		if filterTenantID != "" && s.teams[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.teams[i])
	}
	return items
}

func (s *Service) GetTeam(tenantID, teamID string) (Team, error) {
	nextID := strings.TrimSpace(teamID)
	if nextID == "" {
		return Team{}, ErrTeamNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.teams {
		if s.teams[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.teams[i].TenantID != filterTenantID {
			return Team{}, ErrTeamNotFound
		}
		return s.teams[i], nil
	}

	return Team{}, ErrTeamNotFound
}

func (s *Service) ListTeamMemberships(tenantID string) []TeamMembership {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]TeamMembership, 0, len(s.teamMemberships))
	for i := range s.teamMemberships {
		if filterTenantID != "" && s.teamMemberships[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.teamMemberships[i])
	}
	return items
}

func (s *Service) CreateTeam(tenantID, name, scope, placeID, description, source string) (Team, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return Team{}, ErrTenantIDRequired
	}
	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Team{}, ErrTeamNameRequired
	}
	nextScope, err := normalizeTeamScope(scope, placeID)
	if err != nil {
		return Team{}, err
	}
	nextPlaceID := strings.TrimSpace(placeID)
	if nextScope == "organization" {
		nextPlaceID = ""
	}
	id, err := accessID("team_")
	if err != nil {
		return Team{}, err
	}
	now := time.Now().UTC()
	record := Team{
		ID:           id,
		ResourceType: "Team",
		TenantID:     nextTenantID,
		Name:         nextName,
		Scope:        nextScope,
		PlaceID:      nextPlaceID,
		Description:  strings.TrimSpace(description),
		Source:       firstNonEmpty(strings.TrimSpace(source), "Manual"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.mu.Lock()
	s.teams = append([]Team{record}, s.teams...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return Team{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) UpdateTeam(tenantID, teamID, name, scope, placeID, description, source string) (Team, error) {
	nextID := strings.TrimSpace(teamID)
	if nextID == "" {
		return Team{}, ErrTeamNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)
	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Team{}, ErrTeamNameRequired
	}
	nextScope, err := normalizeTeamScope(scope, placeID)
	if err != nil {
		return Team{}, err
	}
	nextPlaceID := strings.TrimSpace(placeID)
	if nextScope == "organization" {
		nextPlaceID = ""
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.teams {
		if s.teams[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.teams[i].TenantID != filterTenantID {
			return Team{}, ErrTeamNotFound
		}
		s.teams[i].Name = nextName
		s.teams[i].Scope = nextScope
		s.teams[i].PlaceID = nextPlaceID
		s.teams[i].Description = strings.TrimSpace(description)
		s.teams[i].Source = firstNonEmpty(strings.TrimSpace(source), s.teams[i].Source, "Manual")
		s.teams[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return Team{}, err
		}
		return s.teams[i], nil
	}

	return Team{}, ErrTeamNotFound
}

func (s *Service) DeleteTeam(tenantID, teamID string) error {
	nextID := strings.TrimSpace(teamID)
	if nextID == "" {
		return ErrTeamNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	foundIndex := -1
	for i := range s.teams {
		if s.teams[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.teams[i].TenantID != filterTenantID {
			return ErrTeamNotFound
		}
		foundIndex = i
		break
	}
	if foundIndex < 0 {
		return ErrTeamNotFound
	}

	originalTeams := cloneTeams(s.teams)
	originalMemberships := cloneTeamMemberships(s.teamMemberships)
	originalRoleAssignments := cloneRoleAssignments(s.roleAssignments)

	s.teams = append(s.teams[:foundIndex], s.teams[foundIndex+1:]...)
	nextMemberships := make([]TeamMembership, 0, len(s.teamMemberships))
	for i := range s.teamMemberships {
		if s.teamMemberships[i].TeamID == nextID {
			continue
		}
		nextMemberships = append(nextMemberships, s.teamMemberships[i])
	}
	s.teamMemberships = nextMemberships
	nextAssignments := make([]RoleAssignment, 0, len(s.roleAssignments))
	for i := range s.roleAssignments {
		if s.roleAssignments[i].AssigneeType == "Team" && s.roleAssignments[i].AssigneeID == nextID {
			continue
		}
		nextAssignments = append(nextAssignments, s.roleAssignments[i])
	}
	s.roleAssignments = nextAssignments

	if err := s.persistLocked(); err != nil {
		s.teams = originalTeams
		s.teamMemberships = originalMemberships
		s.roleAssignments = originalRoleAssignments
		return err
	}

	return nil
}

func (s *Service) CreateTeamMembership(tenantID, teamID, memberType, memberID, memberEmail, memberName, source string) (TeamMembership, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return TeamMembership{}, ErrTenantIDRequired
	}
	nextTeamID := strings.TrimSpace(teamID)
	if nextTeamID == "" {
		return TeamMembership{}, ErrTeamIDRequired
	}
	nextMemberType, err := normalizeTeamMemberType(memberType)
	if err != nil {
		return TeamMembership{}, err
	}
	nextMemberID := strings.TrimSpace(memberID)
	if nextMemberID == "" {
		return TeamMembership{}, ErrTeamMemberIDRequired
	}
	id, err := accessID("tm_")
	if err != nil {
		return TeamMembership{}, err
	}
	now := time.Now().UTC()
	record := TeamMembership{
		ID:           id,
		ResourceType: "TeamMembership",
		TenantID:     nextTenantID,
		TeamID:       nextTeamID,
		MemberType:   nextMemberType,
		MemberID:     nextMemberID,
		MemberEmail:  strings.TrimSpace(memberEmail),
		MemberName:   strings.TrimSpace(memberName),
		Source:       firstNonEmpty(strings.TrimSpace(source), "Manual"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	teamFound := false
	for i := range s.teams {
		if s.teams[i].ID == nextTeamID && s.teams[i].TenantID == nextTenantID {
			teamFound = true
			break
		}
	}
	if !teamFound {
		return TeamMembership{}, ErrTeamNotFound
	}
	for i := range s.teamMemberships {
		if s.teamMemberships[i].TenantID == nextTenantID &&
			s.teamMemberships[i].TeamID == nextTeamID &&
			s.teamMemberships[i].MemberType == nextMemberType &&
			s.teamMemberships[i].MemberID == nextMemberID {
			s.teamMemberships[i].MemberEmail = record.MemberEmail
			s.teamMemberships[i].MemberName = record.MemberName
			s.teamMemberships[i].Source = record.Source
			s.teamMemberships[i].UpdatedAt = now
			if err := s.persistLocked(); err != nil {
				return TeamMembership{}, err
			}
			return s.teamMemberships[i], nil
		}
	}

	s.teamMemberships = append([]TeamMembership{record}, s.teamMemberships...)
	if err := s.persistLocked(); err != nil {
		return TeamMembership{}, err
	}
	return record, nil
}

func (s *Service) DeleteTeamMembership(tenantID, membershipID string) error {
	nextID := strings.TrimSpace(membershipID)
	if nextID == "" {
		return ErrTeamMembershipNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.teamMemberships {
		if s.teamMemberships[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.teamMemberships[i].TenantID != filterTenantID {
			return ErrTeamMembershipNotFound
		}
		original := cloneTeamMemberships(s.teamMemberships)
		s.teamMemberships = append(s.teamMemberships[:i], s.teamMemberships[i+1:]...)
		if err := s.persistLocked(); err != nil {
			s.teamMemberships = original
			return err
		}
		return nil
	}

	return ErrTeamMembershipNotFound
}

func (s *Service) CreateUserGroup(tenantID, buildingID, name, description string, members []string) (UserGroup, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return UserGroup{}, ErrTenantIDRequired
	}

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return UserGroup{}, ErrUserGroupNameRequired
	}

	id, err := accessID("ug_")
	if err != nil {
		return UserGroup{}, err
	}

	now := time.Now().UTC()
	record := UserGroup{
		ID:                        id,
		ResourceType:              "Group",
		TenantID:                  nextTenantID,
		BuildingID:                strings.TrimSpace(buildingID),
		PlaceID:                   strings.TrimSpace(buildingID),
		Name:                      nextName,
		Description:               strings.TrimSpace(description),
		LoginEnabled:              true,
		GeofenceRestrictionRadius: 150,
		Members:                   uniqueIDs(members),
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}

	s.mu.Lock()
	s.userGroups = append([]UserGroup{record}, s.userGroups...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return UserGroup{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) UpdateUserGroup(tenantID, groupID, buildingID, name, description string, members []string) (UserGroup, error) {
	nextID := strings.TrimSpace(groupID)
	if nextID == "" {
		return UserGroup{}, ErrUserGroupNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return UserGroup{}, ErrUserGroupNameRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.userGroups {
		if s.userGroups[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.userGroups[i].TenantID != filterTenantID {
			return UserGroup{}, ErrUserGroupNotFound
		}

		s.userGroups[i].Name = nextName
		s.userGroups[i].BuildingID = strings.TrimSpace(buildingID)
		s.userGroups[i].PlaceID = strings.TrimSpace(buildingID)
		s.userGroups[i].Description = strings.TrimSpace(description)
		s.userGroups[i].Members = uniqueIDs(members)
		s.userGroups[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return UserGroup{}, err
		}
		return cloneUserGroup(s.userGroups[i]), nil
	}

	return UserGroup{}, ErrUserGroupNotFound
}

func (s *Service) DeleteUserGroup(tenantID, groupID string) error {
	nextID := strings.TrimSpace(groupID)
	if nextID == "" {
		return ErrUserGroupNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	foundIndex := -1
	for i := range s.userGroups {
		if s.userGroups[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.userGroups[i].TenantID != filterTenantID {
			return ErrUserGroupNotFound
		}
		foundIndex = i
		break
	}
	if foundIndex == -1 {
		return ErrUserGroupNotFound
	}

	originalGroups := cloneUserGroups(s.userGroups)
	originalUsers := cloneAccessUsers(s.users)
	originalRoleAssignments := cloneRoleAssignments(s.roleAssignments)
	originalGroupLinks := cloneGroupLinks(s.groupLinks)

	s.userGroups = append(s.userGroups[:foundIndex], s.userGroups[foundIndex+1:]...)
	for i := range s.users {
		s.users[i].GroupIDs = removeID(s.users[i].GroupIDs, nextID)
	}
	nextAssignments := make([]RoleAssignment, 0, len(s.roleAssignments))
	for i := range s.roleAssignments {
		if s.roleAssignments[i].AppliesToType == "Group" && s.roleAssignments[i].AppliesToID == nextID {
			continue
		}
		nextAssignments = append(nextAssignments, s.roleAssignments[i])
	}
	s.roleAssignments = nextAssignments
	nextGroupLinks := make([]GroupLink, 0, len(s.groupLinks))
	for i := range s.groupLinks {
		if s.groupLinks[i].GroupID == nextID {
			continue
		}
		nextGroupLinks = append(nextGroupLinks, s.groupLinks[i])
	}
	s.groupLinks = nextGroupLinks

	if err := s.persistLocked(); err != nil {
		s.userGroups = originalGroups
		s.users = originalUsers
		s.roleAssignments = originalRoleAssignments
		s.groupLinks = originalGroupLinks
		return err
	}

	return nil
}

func (s *Service) UpdateUserGroupRestrictions(tenantID, groupID string, input UserGroupRestrictionsInput) (UserGroup, error) {
	nextID := strings.TrimSpace(groupID)
	if nextID == "" {
		return UserGroup{}, ErrUserGroupNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.userGroups {
		if s.userGroups[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.userGroups[i].TenantID != filterTenantID {
			return UserGroup{}, ErrUserGroupNotFound
		}
		applyUserGroupDefaults(&s.userGroups[i])
		if input.LoginEnabled != nil {
			s.userGroups[i].LoginEnabled = *input.LoginEnabled
		}
		if input.GeofenceRestrictionEnabled != nil {
			s.userGroups[i].GeofenceRestrictionEnabled = *input.GeofenceRestrictionEnabled
		}
		if input.GeofenceRestrictionRadius != nil {
			s.userGroups[i].GeofenceRestrictionRadius = *input.GeofenceRestrictionRadius
		}
		if input.PrimaryDeviceRestrictionEnabled != nil {
			s.userGroups[i].PrimaryDeviceRestrictionEnabled = *input.PrimaryDeviceRestrictionEnabled
		}
		if input.ManagedDeviceRestrictionEnabled != nil {
			s.userGroups[i].ManagedDeviceRestrictionEnabled = *input.ManagedDeviceRestrictionEnabled
		}
		if input.ReaderRestrictionEnabled != nil {
			s.userGroups[i].ReaderRestrictionEnabled = *input.ReaderRestrictionEnabled
		}
		if input.TimeRestrictionEnabled != nil {
			s.userGroups[i].TimeRestrictionEnabled = *input.TimeRestrictionEnabled
		}
		if input.TapToAccessRestrictionEnabled != nil {
			s.userGroups[i].TapToAccessRestrictionEnabled = *input.TapToAccessRestrictionEnabled
		}
		if input.TimeRestrictionTimeZone != nil {
			s.userGroups[i].TimeRestrictionTimeZone = strings.TrimSpace(*input.TimeRestrictionTimeZone)
		}
		s.userGroups[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return UserGroup{}, err
		}
		return cloneUserGroup(s.userGroups[i]), nil
	}

	return UserGroup{}, ErrUserGroupNotFound
}

func (s *Service) ListGroupLinks(tenantID string) []GroupLink {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]GroupLink, 0, len(s.groupLinks))
	for i := range s.groupLinks {
		if filterTenantID != "" && s.groupLinks[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, cloneGroupLink(s.groupLinks[i]))
	}
	return items
}

func (s *Service) GetGroupLink(tenantID, groupLinkID string) (GroupLink, error) {
	nextID := strings.TrimSpace(groupLinkID)
	if nextID == "" {
		return GroupLink{}, ErrGroupLinkNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.groupLinks {
		if s.groupLinks[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.groupLinks[i].TenantID != filterTenantID {
			return GroupLink{}, ErrGroupLinkNotFound
		}
		return cloneGroupLink(s.groupLinks[i]), nil
	}
	return GroupLink{}, ErrGroupLinkNotFound
}

func (s *Service) CreateGroupLink(input GroupLinkInput) (GroupLink, error) {
	nextTenantID := strings.TrimSpace(input.TenantID)
	if nextTenantID == "" {
		return GroupLink{}, ErrTenantIDRequired
	}
	nextGroupID := strings.TrimSpace(input.GroupID)
	if nextGroupID == "" {
		return GroupLink{}, ErrGroupIDRequired
	}
	nextName := strings.TrimSpace(input.Name)
	if nextName == "" {
		return GroupLink{}, ErrGroupLinkNameRequired
	}
	qrType, err := normalizeGroupLinkQRCodeType(input.QuickResponseCodeType)
	if err != nil {
		return GroupLink{}, err
	}
	id, err := accessID("gl_")
	if err != nil {
		return GroupLink{}, err
	}
	secret, err := accessID("gls_")
	if err != nil {
		return GroupLink{}, err
	}
	qrToken := ""
	if qrType != "" {
		qrToken, err = accessID("glq_")
		if err != nil {
			return GroupLink{}, err
		}
	}
	linkEnabled := true
	if input.LinkEnabled != nil {
		linkEnabled = *input.LinkEnabled
	}

	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	groupName := ""
	for i := range s.userGroups {
		if s.userGroups[i].ID != nextGroupID {
			continue
		}
		if s.userGroups[i].TenantID != nextTenantID {
			return GroupLink{}, ErrUserGroupNotFound
		}
		groupName = s.userGroups[i].Name
		break
	}
	if groupName == "" {
		return GroupLink{}, ErrUserGroupNotFound
	}

	record := GroupLink{
		ID:                     id,
		ResourceType:           "GroupLink",
		TenantID:               nextTenantID,
		GroupID:                nextGroupID,
		GroupName:              groupName,
		Name:                   nextName,
		Email:                  normalizeEmail(input.Email),
		Phone:                  strings.TrimSpace(input.Phone),
		LinkEnabled:            linkEnabled,
		QuickResponseCodeType:  qrType,
		ValidFrom:              strings.TrimSpace(input.ValidFrom),
		ValidUntil:             strings.TrimSpace(input.ValidUntil),
		CreatedByType:          normalizeGroupLinkCreatorType(input.CreatedByType),
		CreatedByID:            strings.TrimSpace(input.CreatedByID),
		CreatedByEmail:         normalizeEmail(input.CreatedByEmail),
		CreatedByName:          strings.TrimSpace(input.CreatedByName),
		IssuedByID:             strings.TrimSpace(input.CreatedByID),
		Secret:                 secret,
		QuickResponseCodeToken: qrToken,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	s.groupLinks = append([]GroupLink{record}, s.groupLinks...)
	if err := s.persistLocked(); err != nil {
		return GroupLink{}, err
	}
	return cloneGroupLink(record), nil
}

func (s *Service) UpdateGroupLink(tenantID, groupLinkID string, input GroupLinkUpdateInput) (GroupLink, error) {
	nextID := strings.TrimSpace(groupLinkID)
	if nextID == "" {
		return GroupLink{}, ErrGroupLinkNotFound
	}
	filterTenantID := firstNonEmpty(tenantID, input.TenantID)

	var nextGroupID string
	if input.GroupID != nil {
		nextGroupID = strings.TrimSpace(*input.GroupID)
		if nextGroupID == "" {
			return GroupLink{}, ErrGroupIDRequired
		}
	}
	var nextName string
	if input.Name != nil {
		nextName = strings.TrimSpace(*input.Name)
		if nextName == "" {
			return GroupLink{}, ErrGroupLinkNameRequired
		}
	}
	var nextQRCodeType string
	if input.QuickResponseCodeType != nil {
		qrType, err := normalizeGroupLinkQRCodeType(*input.QuickResponseCodeType)
		if err != nil {
			return GroupLink{}, err
		}
		nextQRCodeType = qrType
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.groupLinks {
		if s.groupLinks[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.groupLinks[i].TenantID != filterTenantID {
			return GroupLink{}, ErrGroupLinkNotFound
		}

		original := cloneGroupLinks(s.groupLinks)
		record := s.groupLinks[i]
		if input.GroupID != nil {
			groupName := ""
			for j := range s.userGroups {
				if s.userGroups[j].ID != nextGroupID {
					continue
				}
				if s.userGroups[j].TenantID != record.TenantID {
					return GroupLink{}, ErrUserGroupNotFound
				}
				groupName = s.userGroups[j].Name
				break
			}
			if groupName == "" {
				return GroupLink{}, ErrUserGroupNotFound
			}
			record.GroupID = nextGroupID
			record.GroupName = groupName
		}
		if input.Name != nil {
			record.Name = nextName
		}
		if input.Email != nil {
			record.Email = normalizeEmail(*input.Email)
		}
		if input.Phone != nil {
			record.Phone = strings.TrimSpace(*input.Phone)
		}
		if input.LinkEnabled != nil {
			record.LinkEnabled = *input.LinkEnabled
		}
		if input.QuickResponseCodeType != nil {
			record.QuickResponseCodeType = nextQRCodeType
			if nextQRCodeType == "" {
				record.QuickResponseCodeToken = ""
				record.QuickResponseCodeImage = ""
			} else if record.QuickResponseCodeToken == "" {
				qrToken, err := accessID("glq_")
				if err != nil {
					return GroupLink{}, err
				}
				record.QuickResponseCodeToken = qrToken
			}
		}
		if input.ValidFrom != nil {
			record.ValidFrom = strings.TrimSpace(*input.ValidFrom)
		}
		if input.ValidUntil != nil {
			record.ValidUntil = strings.TrimSpace(*input.ValidUntil)
		}
		record.ResourceType = "GroupLink"
		record.UpdatedAt = time.Now().UTC()
		s.groupLinks[i] = record
		if err := s.persistLocked(); err != nil {
			s.groupLinks = original
			return GroupLink{}, err
		}
		return cloneGroupLink(record), nil
	}
	return GroupLink{}, ErrGroupLinkNotFound
}

func (s *Service) DeleteGroupLink(tenantID, groupLinkID string) error {
	nextID := strings.TrimSpace(groupLinkID)
	if nextID == "" {
		return ErrGroupLinkNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.groupLinks {
		if s.groupLinks[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.groupLinks[i].TenantID != filterTenantID {
			return ErrGroupLinkNotFound
		}
		original := cloneGroupLinks(s.groupLinks)
		s.groupLinks = append(s.groupLinks[:i], s.groupLinks[i+1:]...)
		if err := s.persistLocked(); err != nil {
			s.groupLinks = original
			return err
		}
		return nil
	}
	return ErrGroupLinkNotFound
}

func (s *Service) VerifyGroupLinkToken(tenantID, token string, verifiedAt time.Time) (GroupLink, error) {
	nextToken := strings.TrimSpace(token)
	if nextToken == "" {
		return GroupLink{}, ErrGroupLinkTokenRequired
	}
	filterTenantID := strings.TrimSpace(tenantID)
	if verifiedAt.IsZero() {
		verifiedAt = time.Now().UTC()
	} else {
		verifiedAt = verifiedAt.UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.groupLinks {
		record := s.groupLinks[i]
		if filterTenantID != "" && record.TenantID != filterTenantID {
			continue
		}
		if record.Secret != nextToken && record.QuickResponseCodeToken != nextToken {
			continue
		}
		if !record.LinkEnabled {
			return GroupLink{}, ErrGroupLinkDisabled
		}
		if validFrom, ok, err := parseGroupLinkInstant(record.ValidFrom); err != nil {
			return GroupLink{}, err
		} else if ok && verifiedAt.Before(validFrom) {
			return GroupLink{}, ErrGroupLinkNotYetValid
		}
		if validUntil, ok, err := parseGroupLinkInstant(record.ValidUntil); err != nil {
			return GroupLink{}, err
		} else if ok && verifiedAt.After(validUntil) {
			return GroupLink{}, ErrGroupLinkExpired
		}

		original := cloneGroupLinks(s.groupLinks)
		record.LastUsedAt = verifiedAt.Format(time.RFC3339)
		record.ResourceType = "GroupLink"
		record.UpdatedAt = verifiedAt
		s.groupLinks[i] = record
		if err := s.persistLocked(); err != nil {
			s.groupLinks = original
			return GroupLink{}, err
		}
		return cloneGroupLink(record), nil
	}
	return GroupLink{}, ErrGroupLinkTokenInvalid
}

func (s *Service) ListPolicies(tenantID string) []Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]Policy, 0, len(s.policies))
	for i := range s.policies {
		if filterTenantID != "" && s.policies[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.policies[i])
	}
	return items
}

func (s *Service) CreatePolicy(
	tenantID, name, scopeType, buildingID, areaID, doorID, schedule string,
	members int,
	status string,
) (Policy, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return Policy{}, ErrTenantIDRequired
	}

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Policy{}, ErrPolicyNameRequired
	}

	nextScopeType, err := normalizeScopeType(scopeType)
	if err != nil {
		return Policy{}, err
	}

	nextStatus, err := normalizePolicyStatus(status)
	if err != nil {
		return Policy{}, err
	}

	nextMembers := members
	if nextMembers < 0 {
		nextMembers = 0
	}

	id, err := accessID("plc_")
	if err != nil {
		return Policy{}, err
	}

	record := Policy{
		ID:         id,
		TenantID:   nextTenantID,
		Name:       nextName,
		ScopeType:  nextScopeType,
		BuildingID: strings.TrimSpace(buildingID),
		AreaID:     strings.TrimSpace(areaID),
		DoorID:     strings.TrimSpace(doorID),
		Schedule:   strings.TrimSpace(schedule),
		Members:    nextMembers,
		Status:     nextStatus,
		UpdatedAt:  time.Now().UTC(),
	}

	s.mu.Lock()
	s.policies = append([]Policy{record}, s.policies...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return Policy{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) UpdatePolicy(
	tenantID, policyID, name, scopeType, buildingID, areaID, doorID, schedule string,
	members int,
	status string,
) (Policy, error) {
	nextID := strings.TrimSpace(policyID)
	if nextID == "" {
		return Policy{}, ErrPolicyNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Policy{}, ErrPolicyNameRequired
	}

	nextScopeType, err := normalizeScopeType(scopeType)
	if err != nil {
		return Policy{}, err
	}
	nextStatus, err := normalizePolicyStatus(status)
	if err != nil {
		return Policy{}, err
	}

	nextMembers := members
	if nextMembers < 0 {
		nextMembers = 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.policies {
		if s.policies[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.policies[i].TenantID != filterTenantID {
			return Policy{}, ErrPolicyNotFound
		}

		s.policies[i].Name = nextName
		s.policies[i].ScopeType = nextScopeType
		s.policies[i].BuildingID = strings.TrimSpace(buildingID)
		s.policies[i].AreaID = strings.TrimSpace(areaID)
		s.policies[i].DoorID = strings.TrimSpace(doorID)
		s.policies[i].Schedule = strings.TrimSpace(schedule)
		s.policies[i].Members = nextMembers
		s.policies[i].Status = nextStatus
		s.policies[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return Policy{}, err
		}
		return s.policies[i], nil
	}

	return Policy{}, ErrPolicyNotFound
}

func (s *Service) ListTemporaryAccess(tenantID string) []TemporaryAccess {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]TemporaryAccess, 0, len(s.temporaryAccess))
	for i := range s.temporaryAccess {
		if filterTenantID != "" && s.temporaryAccess[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.temporaryAccess[i])
	}
	return items
}

func (s *Service) GetTemporaryAccess(tenantID, accessID string) (TemporaryAccess, error) {
	nextID := strings.TrimSpace(accessID)
	if nextID == "" {
		return TemporaryAccess{}, ErrTemporaryAccessNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.temporaryAccess {
		if s.temporaryAccess[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.temporaryAccess[i].TenantID != filterTenantID {
			return TemporaryAccess{}, ErrTemporaryAccessNotFound
		}
		return s.temporaryAccess[i], nil
	}
	return TemporaryAccess{}, ErrTemporaryAccessNotFound
}

func (s *Service) CreateTemporaryAccess(
	tenantID, scopeType, buildingID, areaID, doorID, deliveryMethod,
	granteeName, granteeGender, granteePhone, granteeEmail, mobileModel, passType, validUntil,
	authorizedByID, authorizedByEmail, authorizedByRole string,
) (TemporaryAccess, error) {
	return s.CreateTemporaryAccessWithInput(TemporaryAccessInput{
		TenantID:          tenantID,
		ScopeType:         scopeType,
		BuildingID:        buildingID,
		AreaID:            areaID,
		DoorID:            doorID,
		DeliveryMethod:    deliveryMethod,
		GranteeName:       granteeName,
		GranteeGender:     granteeGender,
		GranteePhone:      granteePhone,
		GranteeEmail:      granteeEmail,
		MobileModel:       mobileModel,
		PassType:          passType,
		ValidUntil:        validUntil,
		AuthorizedByID:    authorizedByID,
		AuthorizedByEmail: authorizedByEmail,
		AuthorizedByRole:  authorizedByRole,
	})
}

func (s *Service) CreateTemporaryAccessWithInput(input TemporaryAccessInput) (TemporaryAccess, error) {
	resolved, err := normalizeTemporaryAccessInput(input)
	if err != nil {
		return TemporaryAccess{}, err
	}

	id, err := accessID("tmp_")
	if err != nil {
		return TemporaryAccess{}, err
	}

	now := time.Now().UTC()
	record := TemporaryAccess{
		ID:                id,
		TenantID:          resolved.TenantID,
		ScopeType:         resolved.ScopeType,
		BuildingID:        resolved.BuildingID,
		AreaID:            resolved.AreaID,
		DoorID:            resolved.DoorID,
		GroupID:           resolved.GroupID,
		RoleID:            resolved.RoleID,
		DeliveryMethod:    resolved.DeliveryMethod,
		GranteeName:       resolved.GranteeName,
		GranteeGender:     resolved.GranteeGender,
		GranteePhone:      resolved.GranteePhone,
		GranteeEmail:      resolved.GranteeEmail,
		MobileModel:       resolved.MobileModel,
		PassType:          resolved.PassType,
		ValidFrom:         resolved.ValidFrom,
		ValidUntil:        resolved.ValidUntil,
		AuthorizedByID:    resolved.AuthorizedByID,
		AuthorizedByEmail: resolved.AuthorizedByEmail,
		AuthorizedByRole:  resolved.AuthorizedByRole,
		AuthorizedAt:      now,
		ReviewedAt:        strings.TrimSpace(resolved.ReviewedAt),
		ReviewedBy:        strings.TrimSpace(resolved.ReviewedBy),
		CreatedAt:         now,
	}

	s.mu.Lock()
	s.temporaryAccess = append([]TemporaryAccess{record}, s.temporaryAccess...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return TemporaryAccess{}, err
	}
	s.mu.Unlock()

	return record, nil
}

func (s *Service) UpdateTemporaryAccess(tenantID, accessID string, input TemporaryAccessInput) (TemporaryAccess, error) {
	nextID := strings.TrimSpace(accessID)
	if nextID == "" {
		return TemporaryAccess{}, ErrTemporaryAccessNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)
	input.TenantID = firstNonEmpty(input.TenantID, filterTenantID)
	resolved, err := normalizeTemporaryAccessInput(input)
	if err != nil {
		return TemporaryAccess{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.temporaryAccess {
		if s.temporaryAccess[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.temporaryAccess[i].TenantID != filterTenantID {
			return TemporaryAccess{}, ErrTemporaryAccessNotFound
		}
		authorizedAt := time.Now().UTC()
		if input.KeepAuthorizedTime {
			authorizedAt = s.temporaryAccess[i].AuthorizedAt
		}
		createdAt := s.temporaryAccess[i].CreatedAt
		s.temporaryAccess[i] = TemporaryAccess{
			ID:                nextID,
			TenantID:          resolved.TenantID,
			ScopeType:         resolved.ScopeType,
			BuildingID:        resolved.BuildingID,
			AreaID:            resolved.AreaID,
			DoorID:            resolved.DoorID,
			GroupID:           resolved.GroupID,
			RoleID:            resolved.RoleID,
			DeliveryMethod:    resolved.DeliveryMethod,
			GranteeName:       resolved.GranteeName,
			GranteeGender:     resolved.GranteeGender,
			GranteePhone:      resolved.GranteePhone,
			GranteeEmail:      resolved.GranteeEmail,
			MobileModel:       resolved.MobileModel,
			PassType:          resolved.PassType,
			ValidFrom:         resolved.ValidFrom,
			ValidUntil:        resolved.ValidUntil,
			AuthorizedByID:    resolved.AuthorizedByID,
			AuthorizedByEmail: resolved.AuthorizedByEmail,
			AuthorizedByRole:  resolved.AuthorizedByRole,
			AuthorizedAt:      authorizedAt,
			ReviewedAt:        strings.TrimSpace(resolved.ReviewedAt),
			ReviewedBy:        strings.TrimSpace(resolved.ReviewedBy),
			CreatedAt:         createdAt,
		}
		if err := s.persistLocked(); err != nil {
			return TemporaryAccess{}, err
		}
		return s.temporaryAccess[i], nil
	}
	return TemporaryAccess{}, ErrTemporaryAccessNotFound
}

func (s *Service) DeleteTemporaryAccess(tenantID, accessID string) error {
	nextID := strings.TrimSpace(accessID)
	if nextID == "" {
		return ErrTemporaryAccessNotFound
	}
	filterTenantID := strings.TrimSpace(tenantID)

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.temporaryAccess {
		if s.temporaryAccess[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.temporaryAccess[i].TenantID != filterTenantID {
			return ErrTemporaryAccessNotFound
		}
		original := cloneTemporaryAccess(s.temporaryAccess)
		s.temporaryAccess = append(s.temporaryAccess[:i], s.temporaryAccess[i+1:]...)
		if err := s.persistLocked(); err != nil {
			s.temporaryAccess = original
			return err
		}
		return nil
	}
	return ErrTemporaryAccessNotFound
}

func normalizeTemporaryAccessInput(input TemporaryAccessInput) (TemporaryAccessInput, error) {
	nextTenantID := strings.TrimSpace(input.TenantID)
	if nextTenantID == "" {
		return TemporaryAccessInput{}, ErrTenantIDRequired
	}

	nextScopeType, err := normalizeScopeType(input.ScopeType)
	if err != nil {
		return TemporaryAccessInput{}, err
	}

	nextMethod, err := normalizeDeliveryMethod(input.DeliveryMethod)
	if err != nil {
		return TemporaryAccessInput{}, err
	}

	nextGranteeName := strings.TrimSpace(input.GranteeName)
	if nextGranteeName == "" {
		return TemporaryAccessInput{}, ErrGranteeNameRequired
	}
	nextGranteePhone := strings.TrimSpace(input.GranteePhone)
	if nextGranteePhone == "" {
		return TemporaryAccessInput{}, ErrGranteePhoneRequired
	}
	nextGranteeEmail := strings.TrimSpace(input.GranteeEmail)
	if nextGranteeEmail == "" {
		return TemporaryAccessInput{}, ErrGranteeEmailRequired
	}

	nextValidUntil := strings.TrimSpace(input.ValidUntil)
	if nextValidUntil == "" {
		return TemporaryAccessInput{}, ErrValidUntilRequired
	}

	return TemporaryAccessInput{
		TenantID:          nextTenantID,
		ScopeType:         nextScopeType,
		BuildingID:        strings.TrimSpace(input.BuildingID),
		AreaID:            strings.TrimSpace(input.AreaID),
		DoorID:            strings.TrimSpace(input.DoorID),
		GroupID:           strings.TrimSpace(input.GroupID),
		RoleID:            firstNonEmpty(input.RoleID, "role_group_access"),
		DeliveryMethod:    nextMethod,
		GranteeName:       nextGranteeName,
		GranteeGender:     strings.TrimSpace(input.GranteeGender),
		GranteePhone:      nextGranteePhone,
		GranteeEmail:      nextGranteeEmail,
		MobileModel:       strings.TrimSpace(input.MobileModel),
		PassType:          strings.TrimSpace(input.PassType),
		ValidFrom:         strings.TrimSpace(input.ValidFrom),
		ValidUntil:        nextValidUntil,
		AuthorizedByID:    strings.TrimSpace(input.AuthorizedByID),
		AuthorizedByEmail: normalizeEmail(input.AuthorizedByEmail),
		AuthorizedByRole:  strings.TrimSpace(input.AuthorizedByRole),
		ReviewedAt:        strings.TrimSpace(input.ReviewedAt),
		ReviewedBy:        strings.TrimSpace(input.ReviewedBy),
	}, nil
}

func (s *Service) ListVisitorPasses(tenantID string) []VisitorPass {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filterTenantID := strings.TrimSpace(tenantID)
	items := make([]VisitorPass, 0, len(s.visitorPasses))
	for i := range s.visitorPasses {
		if filterTenantID != "" && s.visitorPasses[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.visitorPasses[i])
	}
	return items
}

func (s *Service) CreateVisitorPass(tenantID, buildingID, host, visitor, deliveryMethod, expiresAt string) (VisitorPass, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return VisitorPass{}, ErrTenantIDRequired
	}

	nextHost := strings.TrimSpace(host)
	if nextHost == "" {
		return VisitorPass{}, ErrHostRequired
	}

	nextVisitor := strings.TrimSpace(visitor)
	if nextVisitor == "" {
		return VisitorPass{}, ErrVisitorRequired
	}

	nextMethod, err := normalizeDeliveryMethod(deliveryMethod)
	if err != nil {
		return VisitorPass{}, err
	}

	nextExpiresAt := strings.TrimSpace(expiresAt)
	if nextExpiresAt == "" {
		return VisitorPass{}, ErrExpiresAtRequired
	}

	id, err := accessID("vst_")
	if err != nil {
		return VisitorPass{}, err
	}

	record := VisitorPass{
		ID:             id,
		TenantID:       nextTenantID,
		BuildingID:     strings.TrimSpace(buildingID),
		Host:           nextHost,
		Visitor:        nextVisitor,
		DeliveryMethod: nextMethod,
		ExpiresAt:      nextExpiresAt,
		CreatedAt:      time.Now().UTC(),
	}

	s.mu.Lock()
	s.visitorPasses = append([]VisitorPass{record}, s.visitorPasses...)
	if err := s.persistLocked(); err != nil {
		s.mu.Unlock()
		return VisitorPass{}, err
	}
	s.mu.Unlock()

	return record, nil
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
			Users:                    cloneAccessUsers(s.users),
			UserInvitationDeliveries: cloneUserInvitationDeliveries(s.userInvitationDeliveries),
			UserGroups:               cloneUserGroups(s.userGroups),
			Policies:                 clonePolicies(s.policies),
			TemporaryAccess:          cloneTemporaryAccess(s.temporaryAccess),
			VisitorPasses:            cloneVisitorPasses(s.visitorPasses),
			RoleAssignments:          cloneRoleAssignments(s.roleAssignments),
			Teams:                    cloneTeams(s.teams),
			TeamMemberships:          cloneTeamMemberships(s.teamMemberships),
			GroupLinks:               cloneGroupLinks(s.groupLinks),
		})
	}

	s.mu.Lock()
	s.users = cloneAccessUsers(snapshot.Users)
	s.userInvitationDeliveries = cloneUserInvitationDeliveries(snapshot.UserInvitationDeliveries)
	s.userGroups = cloneUserGroups(snapshot.UserGroups)
	s.policies = clonePolicies(snapshot.Policies)
	s.temporaryAccess = cloneTemporaryAccess(snapshot.TemporaryAccess)
	s.visitorPasses = cloneVisitorPasses(snapshot.VisitorPasses)
	if len(snapshot.RoleAssignments) > 0 {
		s.roleAssignments = cloneRoleAssignments(snapshot.RoleAssignments)
	}
	if len(snapshot.Teams) > 0 {
		s.teams = cloneTeams(snapshot.Teams)
	}
	if len(snapshot.TeamMemberships) > 0 {
		s.teamMemberships = cloneTeamMemberships(snapshot.TeamMemberships)
	}
	if len(snapshot.GroupLinks) > 0 {
		s.groupLinks = cloneGroupLinks(snapshot.GroupLinks)
	}
	s.mu.Unlock()
	return nil
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		Users:                    cloneAccessUsers(s.users),
		UserInvitationDeliveries: cloneUserInvitationDeliveries(s.userInvitationDeliveries),
		UserGroups:               cloneUserGroups(s.userGroups),
		Policies:                 clonePolicies(s.policies),
		TemporaryAccess:          cloneTemporaryAccess(s.temporaryAccess),
		VisitorPasses:            cloneVisitorPasses(s.visitorPasses),
		RoleAssignments:          cloneRoleAssignments(s.roleAssignments),
		Teams:                    cloneTeams(s.teams),
		TeamMemberships:          cloneTeamMemberships(s.teamMemberships),
		GroupLinks:               cloneGroupLinks(s.groupLinks),
	})
}

func cloneAccessUsers(items []AccessUser) []AccessUser {
	output := make([]AccessUser, 0, len(items))
	for i := range items {
		output = append(output, cloneAccessUser(items[i]))
	}
	return output
}

func cloneAccessUser(item AccessUser) AccessUser {
	record := item
	record.GroupIDs = append([]string(nil), item.GroupIDs...)
	return record
}

func cloneUserInvitationDeliveries(items []UserInvitationDelivery) []UserInvitationDelivery {
	output := make([]UserInvitationDelivery, 0, len(items))
	for i := range items {
		output = append(output, cloneUserInvitationDelivery(items[i]))
	}
	return output
}

func cloneUserInvitationDelivery(item UserInvitationDelivery) UserInvitationDelivery {
	record := item
	record.ResourceType = "UserInvitationDelivery"
	if item.DeliveredAt != nil {
		deliveredAt := *item.DeliveredAt
		record.DeliveredAt = &deliveredAt
	}
	return record
}

func cloneUserGroups(items []UserGroup) []UserGroup {
	output := make([]UserGroup, 0, len(items))
	for i := range items {
		output = append(output, cloneUserGroup(items[i]))
	}
	return output
}

func cloneUserGroup(item UserGroup) UserGroup {
	record := item
	applyUserGroupDefaults(&record)
	record.Members = append([]string(nil), item.Members...)
	return record
}

func applyUserGroupDefaults(item *UserGroup) {
	if item == nil {
		return
	}
	item.ResourceType = "Group"
	if strings.TrimSpace(item.PlaceID) == "" {
		item.PlaceID = strings.TrimSpace(item.BuildingID)
	}
	if item.GeofenceRestrictionRadius <= 0 {
		item.GeofenceRestrictionRadius = 150
	}
	item.UsersCount = len(item.Members)
}

func clonePolicies(items []Policy) []Policy {
	output := make([]Policy, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneTemporaryAccess(items []TemporaryAccess) []TemporaryAccess {
	output := make([]TemporaryAccess, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneVisitorPasses(items []VisitorPass) []VisitorPass {
	output := make([]VisitorPass, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneGroupLinks(items []GroupLink) []GroupLink {
	output := make([]GroupLink, 0, len(items))
	for i := range items {
		output = append(output, cloneGroupLink(items[i]))
	}
	return output
}

func cloneGroupLink(item GroupLink) GroupLink {
	record := item
	record.ResourceType = "GroupLink"
	return record
}

func cloneRoleAssignments(items []RoleAssignment) []RoleAssignment {
	output := make([]RoleAssignment, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneTeams(items []Team) []Team {
	output := make([]Team, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func cloneTeamMemberships(items []TeamMembership) []TeamMembership {
	output := make([]TeamMembership, 0, len(items))
	for i := range items {
		output = append(output, items[i])
	}
	return output
}

func builtInRoles() []Role {
	return []Role{
		{
			ID:          "role_organization_admin",
			Name:        "Organization Admin",
			AppliesTo:   "Organization",
			Description: "Manage organization users, places, access, credentials, integrations, and reports.",
			Permissions: map[string]bool{
				"places_read":            true,
				"places_write":           true,
				"locks_read":             true,
				"locks_write":            true,
				"locks_unlock":           true,
				"users_read":             true,
				"users_write":            true,
				"groups_read":            true,
				"groups_write":           true,
				"teams_read":             true,
				"teams_write":            true,
				"team_memberships_read":  true,
				"team_memberships_write": true,
				"roles_read":             true,
				"role_assignments_read":  true,
				"role_assignments_write": true,
				"shares_read":            true,
				"shares_write":           true,
				"cards_read":             true,
				"cards_write":            true,
				"integrations_read":      true,
				"integrations_write":     true,
				"reports_read":           true,
			},
			BuiltIn: true,
		},
		{
			ID:          "role_place_admin",
			Name:        "Place Admin",
			AppliesTo:   "Place",
			Description: "Manage assigned place users, groups, locks, hardware, and local events.",
			Permissions: map[string]bool{
				"places_read":            true,
				"locks_read":             true,
				"locks_write":            true,
				"locks_unlock":           true,
				"users_read":             true,
				"users_write":            true,
				"groups_read":            true,
				"groups_write":           true,
				"teams_read":             true,
				"team_memberships_read":  true,
				"role_assignments_read":  true,
				"role_assignments_write": true,
				"shares_read":            true,
				"shares_write":           true,
				"events_read":            true,
			},
			BuiltIn: true,
		},
		{
			ID:          "role_group_access",
			Name:        "Group Access",
			AppliesTo:   "Group",
			Description: "Grant access through a group without creating a management persona.",
			Permissions: map[string]bool{
				"locks_read":   true,
				"locks_unlock": true,
			},
			BuiltIn: true,
		},
	}
}

func normalizeRoleAssignmentInput(input RoleAssignmentInput) (RoleAssignmentInput, error) {
	nextTenantID := strings.TrimSpace(input.TenantID)
	if nextTenantID == "" {
		return RoleAssignmentInput{}, ErrTenantIDRequired
	}

	nextRoleID := strings.TrimSpace(input.RoleID)
	if nextRoleID == "" {
		return RoleAssignmentInput{}, ErrRoleIDRequired
	}
	role, exists := roleByID(nextRoleID)
	if !exists {
		return RoleAssignmentInput{}, ErrRoleNotFound
	}

	nextAppliesToType, err := normalizeRoleScope(input.AppliesToType)
	if err != nil {
		return RoleAssignmentInput{}, err
	}
	if role.AppliesTo != nextAppliesToType {
		return RoleAssignmentInput{}, ErrInvalidRoleScope
	}

	nextAppliesToID := strings.TrimSpace(input.AppliesToID)
	if nextAppliesToID == "" {
		return RoleAssignmentInput{}, ErrAppliesToIDRequired
	}

	nextAssigneeType, err := normalizeAssigneeType(input.AssigneeType)
	if err != nil {
		return RoleAssignmentInput{}, err
	}
	nextAssigneeID := strings.TrimSpace(input.AssigneeID)
	if nextAssigneeID == "" {
		return RoleAssignmentInput{}, ErrAssigneeIDRequired
	}

	return RoleAssignmentInput{
		TenantID:      nextTenantID,
		RoleID:        nextRoleID,
		AppliesToType: nextAppliesToType,
		AppliesToID:   nextAppliesToID,
		AssigneeType:  nextAssigneeType,
		AssigneeID:    nextAssigneeID,
		AssigneeEmail: strings.TrimSpace(input.AssigneeEmail),
		ValidFrom:     strings.TrimSpace(input.ValidFrom),
		ValidUntil:    strings.TrimSpace(input.ValidUntil),
	}, nil
}

func roleByID(roleID string) (Role, bool) {
	nextRoleID := strings.TrimSpace(roleID)
	for _, role := range builtInRoles() {
		if role.ID == nextRoleID {
			return role, true
		}
	}
	return Role{}, false
}

func normalizeRoleScope(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "organization":
		return "Organization", nil
	case "place":
		return "Place", nil
	case "group":
		return "Group", nil
	default:
		return "", ErrInvalidRoleScope
	}
}

func normalizeAssigneeType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "user":
		return "User", nil
	case "team":
		return "Team", nil
	case "guest":
		return "Guest", nil
	default:
		return "", ErrInvalidAssigneeType
	}
}

func normalizeTeamScope(scope, placeID string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "", "organization":
		if strings.TrimSpace(placeID) != "" {
			return "place", nil
		}
		return "organization", nil
	case "place":
		return "place", nil
	default:
		return "", ErrInvalidScopeType
	}
}

func normalizeTeamMemberType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "user", "":
		return "User", nil
	case "guest":
		return "Guest", nil
	default:
		return "", ErrInvalidTeamMemberType
	}
}

func firstNonEmpty(values ...string) string {
	for i := range values {
		value := strings.TrimSpace(values[i])
		if value != "" {
			return value
		}
	}
	return ""
}

func accessID(prefix string) (string, error) {
	raw := make([]byte, 4)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw), nil
}

func uniqueIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}

	if len(items) == 0 {
		return nil
	}

	return items
}

func removeID(values []string, target string) []string {
	if len(values) == 0 {
		return nil
	}
	nextTarget := strings.TrimSpace(target)
	if nextTarget == "" {
		return uniqueIDs(values)
	}
	items := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || value == nextTarget {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func normalizePolicyStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "active":
		return "active", nil
	case "inactive":
		return "inactive", nil
	case "draft":
		return "draft", nil
	default:
		return "", ErrInvalidPolicyStatus
	}
}

func normalizeUserStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "active":
		return "active", nil
	case "inactive":
		return "inactive", nil
	case "suspended":
		return "suspended", nil
	default:
		return "", ErrInvalidUserStatus
	}
}

func normalizeUserInvitationDeliveryMethod(method string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "", "email":
		return "email", nil
	case "email_qr":
		return "email_qr", nil
	default:
		return "", ErrDeliveryMethodInvalid
	}
}

func normalizeUserInvitationStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "sent", "delivered":
		return "sent", nil
	case "failed", "bounced", "rejected", "undelivered":
		return "failed", nil
	default:
		return "", ErrInvalidUserInvitationStatus
	}
}

func normalizeScopeType(scopeType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(scopeType)) {
	case "", "all":
		return "all", nil
	case "building":
		return "building", nil
	case "area":
		return "area", nil
	case "door":
		return "door", nil
	default:
		return "", ErrInvalidScopeType
	}
}

func normalizeDeliveryMethod(method string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "", "wallet":
		return "wallet", nil
	case "email_qr":
		return "email_qr", nil
	default:
		return "", ErrDeliveryMethodInvalid
	}
}

func normalizeGroupLinkQRCodeType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "none", "null":
		return "", nil
	case "online":
		return "online", nil
	case "offline":
		return "offline", nil
	default:
		return "", ErrInvalidGroupLinkQRCodeType
	}
}

func parseGroupLinkInstant(value string) (time.Time, bool, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return time.Time{}, false, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for i := range layouts {
		parsed, err := time.Parse(layouts[i], raw)
		if err == nil {
			return parsed.UTC(), true, nil
		}
	}
	return time.Time{}, true, ErrInvalidGroupLinkValidityWindow
}

func normalizeGroupLinkCreatorType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "user":
		return "User"
	case "marketplaceinstallation", "marketplace_installation":
		return "MarketplaceInstallation"
	default:
		return ""
	}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeSyncSource(source string) string {
	return strings.ToLower(strings.TrimSpace(source))
}

func accessSyncIdentityKey(source, ref string) string {
	nextSource := normalizeSyncSource(source)
	nextRef := strings.TrimSpace(ref)
	if nextSource == "" || nextRef == "" {
		return ""
	}
	return nextSource + ":" + nextRef
}

// --- Holiday Calendar ---

var ErrHolidayCalendarNameRequired = errors.New("holiday calendar name is required")
var ErrHolidayCalendarNotFound = errors.New("holiday calendar not found")

func (s *Service) ListHolidayCalendars(tenantID string) []HolidayCalendar {
	filterTenantID := strings.TrimSpace(tenantID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]HolidayCalendar, 0, len(s.holidayCalendars))
	for i := range s.holidayCalendars {
		if filterTenantID != "" && s.holidayCalendars[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.holidayCalendars[i])
	}
	return items
}

func (s *Service) GetHolidayCalendar(tenantID, calendarID string) (HolidayCalendar, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextID := strings.TrimSpace(calendarID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.holidayCalendars {
		if s.holidayCalendars[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.holidayCalendars[i].TenantID != filterTenantID {
			continue
		}
		return s.holidayCalendars[i], nil
	}
	return HolidayCalendar{}, ErrHolidayCalendarNotFound
}

func (s *Service) CreateHolidayCalendar(tenantID, name, country string, entries []HolidayEntry) (HolidayCalendar, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return HolidayCalendar{}, ErrTenantIDRequired
	}
	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return HolidayCalendar{}, ErrHolidayCalendarNameRequired
	}
	idBytes := make([]byte, 6)
	rand.Read(idBytes)
	now := time.Now().UTC()
	cal := HolidayCalendar{
		ID:        "hcal_" + hex.EncodeToString(idBytes),
		TenantID:  nextTenantID,
		Name:      nextName,
		Country:   strings.TrimSpace(country),
		Entries:   normalizeHolidayEntries(entries),
		UpdatedAt: now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.holidayCalendars = append(s.holidayCalendars, cal)
	_ = s.persistLocked()
	return cal, nil
}

func (s *Service) UpdateHolidayCalendar(tenantID, calendarID, name, country string, entries []HolidayEntry) (HolidayCalendar, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	nextID := strings.TrimSpace(calendarID)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.holidayCalendars {
		if s.holidayCalendars[i].ID != nextID {
			continue
		}
		if nextTenantID != "" && s.holidayCalendars[i].TenantID != nextTenantID {
			continue
		}
		if n := strings.TrimSpace(name); n != "" {
			s.holidayCalendars[i].Name = n
		}
		if c := strings.TrimSpace(country); c != "" {
			s.holidayCalendars[i].Country = c
		}
		if entries != nil {
			s.holidayCalendars[i].Entries = normalizeHolidayEntries(entries)
		}
		s.holidayCalendars[i].UpdatedAt = time.Now().UTC()
		_ = s.persistLocked()
		return s.holidayCalendars[i], nil
	}
	return HolidayCalendar{}, ErrHolidayCalendarNotFound
}

func (s *Service) DeleteHolidayCalendar(tenantID, calendarID string) error {
	nextTenantID := strings.TrimSpace(tenantID)
	nextID := strings.TrimSpace(calendarID)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.holidayCalendars {
		if s.holidayCalendars[i].ID != nextID {
			continue
		}
		if nextTenantID != "" && s.holidayCalendars[i].TenantID != nextTenantID {
			continue
		}
		s.holidayCalendars = append(s.holidayCalendars[:i], s.holidayCalendars[i+1:]...)
		_ = s.persistLocked()
		return nil
	}
	return ErrHolidayCalendarNotFound
}

func normalizeHolidayEntries(entries []HolidayEntry) []HolidayEntry {
	if entries == nil {
		return nil
	}
	result := make([]HolidayEntry, 0, len(entries))
	for _, e := range entries {
		date := strings.TrimSpace(e.Date)
		if date == "" {
			continue
		}
		result = append(result, HolidayEntry{
			Date:        date,
			Name:        strings.TrimSpace(e.Name),
			Description: strings.TrimSpace(e.Description),
		})
	}
	return result
}

// --- Schedule CRUD ---

func (s *Service) ListSchedules(tenantID string) []Schedule {
	filterTenantID := strings.TrimSpace(tenantID)
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]Schedule, 0, len(s.schedules))
	for i := range s.schedules {
		if filterTenantID != "" && s.schedules[i].TenantID != filterTenantID {
			continue
		}
		items = append(items, s.schedules[i])
	}
	return items
}

func (s *Service) GetSchedule(tenantID, scheduleID string) (Schedule, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextID := strings.TrimSpace(scheduleID)
	if nextID == "" {
		return Schedule{}, ErrScheduleNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.schedules {
		if s.schedules[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.schedules[i].TenantID != filterTenantID {
			continue
		}
		return s.schedules[i], nil
	}
	return Schedule{}, ErrScheduleNotFound
}

func (s *Service) CreateSchedule(
	tenantID, name, description, validFrom, validUntil, holidayCalendarID string,
	timeWindows []TimeWindow, exceptionDates []string,
) (Schedule, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return Schedule{}, ErrTenantIDRequired
	}
	nextName := strings.TrimSpace(name)
	if nextName == "" {
		return Schedule{}, ErrScheduleNameRequired
	}

	id, err := accessID("sched_")
	if err != nil {
		return Schedule{}, err
	}
	now := time.Now().UTC()

	schedule := Schedule{
		ID:                id,
		TenantID:          nextTenantID,
		Name:              nextName,
		Description:       strings.TrimSpace(description),
		ValidFrom:         strings.TrimSpace(validFrom),
		ValidUntil:        strings.TrimSpace(validUntil),
		TimeWindows:       timeWindows,
		ExceptionDates:    normalizeExceptionDates(exceptionDates),
		HolidayCalendarID: strings.TrimSpace(holidayCalendarID),
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.schedules = append([]Schedule{schedule}, s.schedules...)
	if err := s.persistLocked(); err != nil {
		return Schedule{}, err
	}
	return schedule, nil
}

func (s *Service) UpdateSchedule(
	tenantID, scheduleID, name, description, validFrom, validUntil, holidayCalendarID string,
	timeWindows []TimeWindow, exceptionDates []string,
) (Schedule, error) {
	filterTenantID := strings.TrimSpace(tenantID)
	nextID := strings.TrimSpace(scheduleID)
	if nextID == "" {
		return Schedule{}, ErrScheduleNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.schedules {
		if s.schedules[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.schedules[i].TenantID != filterTenantID {
			continue
		}
		if n := strings.TrimSpace(name); n != "" {
			s.schedules[i].Name = n
		}
		if d := strings.TrimSpace(description); d != "" {
			s.schedules[i].Description = d
		}
		if vf := strings.TrimSpace(validFrom); vf != "" {
			s.schedules[i].ValidFrom = vf
		}
		if vu := strings.TrimSpace(validUntil); vu != "" {
			s.schedules[i].ValidUntil = vu
		}
		if timeWindows != nil {
			s.schedules[i].TimeWindows = timeWindows
		}
		if exceptionDates != nil {
			s.schedules[i].ExceptionDates = normalizeExceptionDates(exceptionDates)
		}
		if hcID := strings.TrimSpace(holidayCalendarID); hcID != "" {
			s.schedules[i].HolidayCalendarID = hcID
		}
		s.schedules[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return Schedule{}, err
		}
		return s.schedules[i], nil
	}
	return Schedule{}, ErrScheduleNotFound
}

func (s *Service) DeleteSchedule(tenantID, scheduleID string) error {
	filterTenantID := strings.TrimSpace(tenantID)
	nextID := strings.TrimSpace(scheduleID)
	if nextID == "" {
		return ErrScheduleNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.schedules {
		if s.schedules[i].ID != nextID {
			continue
		}
		if filterTenantID != "" && s.schedules[i].TenantID != filterTenantID {
			continue
		}
		s.schedules = append(s.schedules[:i], s.schedules[i+1:]...)
		if err := s.persistLocked(); err != nil {
			return err
		}
		return nil
	}
	return ErrScheduleNotFound
}

func normalizeExceptionDates(dates []string) []string {
	if dates == nil {
		return nil
	}
	result := make([]string, 0, len(dates))
	for _, d := range dates {
		trimmed := strings.TrimSpace(d)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// --- Organization Settings ---

func defaultOrganizationSettings(tenantID string) OrganizationSettings {
	return OrganizationSettings{
		TenantID:              tenantID,
		Name:                  "Mistyislet",
		PrimaryDomain:         "mistypass.local",
		Timezone:              "Asia/Jakarta",
		SupportEmail:          "support@mistypass.local",
		EmailNotifications:    true,
		PushNotifications:     true,
		WeeklyReports:         false,
		EnforceMFA:            false,
		PasswordPolicy:        "standard",
		SessionTimeoutMinutes: 480,
		UpdatedAt:             time.Now().UTC(),
	}
}

func (s *Service) GetOrganizationSettings(tenantID string) OrganizationSettings {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return defaultOrganizationSettings(nextTenantID)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if settings, exists := s.organizationSettings[nextTenantID]; exists {
		return settings
	}
	return defaultOrganizationSettings(nextTenantID)
}

func (s *Service) UpdateOrganizationSettings(
	tenantID string,
	name, primaryDomain, timezone, supportEmail *string,
	emailNotifications, pushNotifications, weeklyReports, enforceMFA *bool,
	passwordPolicy *string,
	sessionTimeoutMinutes *int,
) (OrganizationSettings, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return OrganizationSettings{}, ErrTenantIDRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	settings, exists := s.organizationSettings[nextTenantID]
	if !exists {
		settings = defaultOrganizationSettings(nextTenantID)
	}

	if name != nil {
		settings.Name = strings.TrimSpace(*name)
	}
	if primaryDomain != nil {
		settings.PrimaryDomain = strings.TrimSpace(*primaryDomain)
	}
	if timezone != nil {
		settings.Timezone = strings.TrimSpace(*timezone)
	}
	if supportEmail != nil {
		settings.SupportEmail = strings.TrimSpace(*supportEmail)
	}
	if emailNotifications != nil {
		settings.EmailNotifications = *emailNotifications
	}
	if pushNotifications != nil {
		settings.PushNotifications = *pushNotifications
	}
	if weeklyReports != nil {
		settings.WeeklyReports = *weeklyReports
	}
	if enforceMFA != nil {
		settings.EnforceMFA = *enforceMFA
	}
	if passwordPolicy != nil {
		p := strings.TrimSpace(*passwordPolicy)
		if p == "standard" || p == "strict" {
			settings.PasswordPolicy = p
		}
	}
	if sessionTimeoutMinutes != nil && *sessionTimeoutMinutes > 0 {
		settings.SessionTimeoutMinutes = *sessionTimeoutMinutes
	}
	settings.UpdatedAt = time.Now().UTC()
	s.organizationSettings[nextTenantID] = settings

	if err := s.persistLocked(); err != nil {
		return OrganizationSettings{}, err
	}
	return settings, nil
}

// --- Schedule Evaluation ---

type ScheduleEvaluation struct {
	IsActive         bool         `json:"is_active"`
	Reason           string       `json:"reason"`
	ValidFrom        string       `json:"valid_from,omitempty"`
	ValidUntil       string       `json:"valid_until,omitempty"`
	TimeWindows      []TimeWindow `json:"time_windows,omitempty"`
	ExceptionDates   []string     `json:"exception_dates,omitempty"`
	HolidayCalendar  string       `json:"holiday_calendar,omitempty"`
	EvaluatedAt      string       `json:"evaluated_at"`
	NextActiveWindow string       `json:"next_active_window,omitempty"`
}

func EvaluateSchedule(now time.Time, validFrom, validUntil string, timeWindows []TimeWindow, exceptionDates []string, holidays []HolidayEntry) ScheduleEvaluation {
	eval := ScheduleEvaluation{
		EvaluatedAt: now.UTC().Format(time.RFC3339),
		ValidFrom:   validFrom,
		ValidUntil:  validUntil,
		TimeWindows: timeWindows,
	}

	// check date range
	if validFrom != "" {
		from, err := time.Parse(time.RFC3339, validFrom)
		if err == nil && now.Before(from) {
			eval.Reason = "not_yet_valid"
			return eval
		}
	}
	if validUntil != "" {
		until, err := time.Parse(time.RFC3339, validUntil)
		if err == nil && now.After(until) {
			eval.Reason = "expired"
			return eval
		}
	}

	// check exception dates
	todayStr := now.Format("2006-01-02")
	for _, d := range exceptionDates {
		if strings.TrimSpace(d) == todayStr {
			eval.Reason = "exception_date"
			return eval
		}
	}

	// check holiday calendar
	for _, h := range holidays {
		if strings.TrimSpace(h.Date) == todayStr {
			eval.Reason = "holiday:" + h.Name
			return eval
		}
	}

	// check time windows
	if len(timeWindows) > 0 {
		inWindow := false
		for _, tw := range timeWindows {
			if isInTimeWindow(now, tw) {
				inWindow = true
				break
			}
		}
		if !inWindow {
			eval.Reason = "outside_time_window"
			return eval
		}
	}

	eval.IsActive = true
	eval.Reason = "active"
	return eval
}

func isInTimeWindow(now time.Time, tw TimeWindow) bool {
	// check day of week
	if !isDayInSet(now.Weekday(), tw.DayOfWeekSet) {
		return false
	}
	// check time range
	startTime := parseHHMM(tw.StartTime)
	endTime := parseHHMM(tw.EndTime)
	if startTime < 0 || endTime < 0 {
		return true // invalid time range means no restriction
	}
	currentMinutes := now.Hour()*60 + now.Minute()
	return currentMinutes >= startTime && currentMinutes < endTime
}

func isDayInSet(weekday time.Weekday, daySet string) bool {
	normalized := strings.ToLower(strings.TrimSpace(daySet))
	if normalized == "" || normalized == "all" || normalized == "everyday" {
		return true
	}
	if normalized == "weekday" || normalized == "weekdays" {
		return weekday >= time.Monday && weekday <= time.Friday
	}
	if normalized == "weekend" || normalized == "weekends" {
		return weekday == time.Saturday || weekday == time.Sunday
	}
	// parse comma-separated ISO days: 1=Mon, 7=Sun
	isoDay := int(weekday)
	if isoDay == 0 {
		isoDay = 7 // Sunday
	}
	for _, part := range strings.Split(normalized, ",") {
		d := strings.TrimSpace(part)
		if d == "" {
			continue
		}
		n := 0
		for _, ch := range d {
			if ch >= '0' && ch <= '9' {
				n = n*10 + int(ch-'0')
			}
		}
		if n == isoDay {
			return true
		}
	}
	return false
}

func parseHHMM(s string) int {
	parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(parts) != 2 {
		return -1
	}
	h, hErr := strings.CutPrefix(parts[0], "0")
	if hErr {
		// trimmed leading zero
	}
	_ = h
	hour := 0
	for _, ch := range strings.TrimSpace(parts[0]) {
		if ch >= '0' && ch <= '9' {
			hour = hour*10 + int(ch-'0')
		}
	}
	minute := 0
	for _, ch := range strings.TrimSpace(parts[1]) {
		if ch >= '0' && ch <= '9' {
			minute = minute*10 + int(ch-'0')
		}
	}
	if hour > 23 || minute > 59 {
		return -1
	}
	return hour*60 + minute
}
