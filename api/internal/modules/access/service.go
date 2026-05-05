package access

import (
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
var ErrOAuth2ClientNotFound = errors.New("oauth2 client not found")
var ErrOAuth2ClientNameRequired = errors.New("oauth2 client name is required")
var ErrOAuth2ClientRedirectURIRequired = errors.New("at least one redirect_uri is required")

// OAuth2Client represents a registered OAuth2 client application.
// The SecretHash field is persisted; ClientSecret (plaintext) is only
// populated at creation time and is never stored.
type OAuth2Client struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Name         string    `json:"name"`
	ClientSecret string    `json:"client_secret,omitempty"` // only at creation, never persisted
	SecretHash   string    `json:"secret_hash"`             // bcrypt hash, persisted
	RedirectURIs []string  `json:"redirect_uris"`
	Scopes       []string  `json:"scopes"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

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

type Elevator struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	PlaceID     string    `json:"place_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	StopsCount  int       `json:"elevator_stops_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ElevatorStop struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	ElevatorID string    `json:"elevator_id"`
	FloorID    string    `json:"floor_id,omitempty"`
	Name       string    `json:"name"`
	Status     string    `json:"status"` // "active", "locked_down"
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type GroupElevatorStop struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	GroupID        string    `json:"group_id"`
	ElevatorStopID string    `json:"elevator_stop_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type GroupTerminal struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	GroupID    string    `json:"group_id"`
	TerminalID string    `json:"terminal_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type Presence struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	PlaceID    string    `json:"place_id"`
	UserID     string    `json:"user_id"`
	UserName   string    `json:"user_name,omitempty"`
	UserEmail  string    `json:"user_email,omitempty"`
	EnteredAt  time.Time `json:"entered_at"`
	ExitedAt   string    `json:"exited_at,omitempty"`
}

type CSVCardImport struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	FileName   string    `json:"file_name"`
	Status     string    `json:"status"` // "pending", "processing", "completed", "failed"
	TotalRows  int       `json:"total_rows"`
	Imported   int       `json:"imported"`
	Failed     int       `json:"failed"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Guest struct {
	ID                 string    `json:"id"`
	TenantID           string    `json:"tenant_id"`
	BuildingID         string    `json:"building_id,omitempty"`
	Name               string    `json:"name"`
	Email              string    `json:"email,omitempty"`
	Phone              string    `json:"phone"`
	Company            string    `json:"company,omitempty"`
	Purpose            string    `json:"purpose,omitempty"`
	HostName           string    `json:"host_name"`
	HostEmail          string    `json:"host_email,omitempty"`
	HostPhone          string    `json:"host_phone,omitempty"`
	IDDocumentType     string    `json:"id_document_type,omitempty"`   // "KTP", "KITAS", "ITAS" or empty
	IDDocumentNumber   string    `json:"id_document_number,omitempty"` // optional
	Status             string    `json:"status"`                       // "expected", "checked_in", "checked_out", "cancelled"
	CheckedInAt        string    `json:"checked_in_at,omitempty"`
	CheckedOutAt       string    `json:"checked_out_at,omitempty"`
	ExpectedAt         string    `json:"expected_at,omitempty"`
	NotifyHost         bool      `json:"notify_host,omitempty"`
	HostNotifiedAt       string    `json:"host_notified_at,omitempty"`
	AccessToken          string    `json:"access_token,omitempty"`
	AccessTokenExpiresAt string    `json:"access_token_expires_at,omitempty"`
	DoorIDs              []string  `json:"door_ids,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type BookableSpace struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	Name             string    `json:"name"`
	Description      string    `json:"description,omitempty"`
	SpaceType        string    `json:"space_type"`
	CapacityMode     string    `json:"capacity_mode"`
	MaxCapacity      int       `json:"max_capacity"`
	CurrentOccupancy int       `json:"current_occupancy"`
	LockID           string    `json:"lock_id,omitempty"`
	RequiresBooking  bool      `json:"requires_booking"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Booking struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	SpaceID      string    `json:"space_id"`
	UserID       string    `json:"user_id"`
	UserName     string    `json:"user_name"`
	Title        string    `json:"title,omitempty"`
	StartTime    string    `json:"start_time"`
	EndTime      string    `json:"end_time"`
	Status       string    `json:"status"`
	CheckedInAt  string    `json:"checked_in_at,omitempty"`
	CheckedOutAt string    `json:"checked_out_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type BookableSpaceStatus struct {
	Space          BookableSpace `json:"space"`
	ActiveBookings []Booking     `json:"active_bookings"`
}

type CreateBookableSpaceInput struct {
	TenantID        string
	Name            string
	Description     string
	SpaceType       string
	CapacityMode    string
	MaxCapacity     int
	LockID          string
	RequiresBooking bool
	Enabled         bool
}

type UpdateBookableSpaceInput struct {
	Name            *string
	Description     *string
	SpaceType       *string
	CapacityMode    *string
	MaxCapacity     *int
	LockID          *string
	RequiresBooking *bool
	Enabled         *bool
}

type CreateBookingInput struct {
	TenantID  string
	SpaceID   string
	UserID    string
	UserName  string
	Title     string
	StartTime string
	EndTime   string
}

type UpdateBookingInput struct {
	Title     *string
	StartTime *string
	EndTime   *string
	Status    *string
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
	Guests                   []Guest                  `json:"guests,omitempty"`
	RoleAssignments          []RoleAssignment         `json:"role_assignments"`
	Teams                    []Team                   `json:"teams"`
	TeamMemberships          []TeamMembership         `json:"team_memberships"`
	GroupLinks               []GroupLink              `json:"group_links"`
	BookableSpaces           []BookableSpace          `json:"bookable_spaces,omitempty"`
	Bookings                 []Booking                `json:"bookings,omitempty"`
	OAuth2Clients            []OAuth2Client           `json:"oauth2_clients,omitempty"`
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
	WebAuthnEnabled       bool      `json:"webauthn_enabled"`
	PasswordPolicy        string    `json:"password_policy"`
	SessionTimeoutMinutes int       `json:"session_timeout_minutes"`
	WebhookSigningKey     string    `json:"webhook_signing_key,omitempty"`
	WebhookRotatedAt      time.Time `json:"webhook_rotated_at,omitempty"`
	Status                string    `json:"status"`
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
	guests                   []Guest
	elevators                []Elevator
	elevatorStops            []ElevatorStop
	groupElevatorStops       []GroupElevatorStop
	groupTerminals           []GroupTerminal
	presences                []Presence
	csvCardImports           []CSVCardImport
	roleAssignments          []RoleAssignment
	teams                    []Team
	teamMemberships          []TeamMembership
	groupLinks               []GroupLink
	bookableSpaces           []BookableSpace
	bookings                 []Booking
	oauth2Clients            []OAuth2Client
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
				ValidUntil:             "2099-12-31T23:59:59Z",
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

