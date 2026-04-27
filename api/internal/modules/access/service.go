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
var ErrHostRequired = errors.New("host is required")
var ErrVisitorRequired = errors.New("visitor is required")
var ErrExpiresAtRequired = errors.New("expires_at is required")
var ErrPolicyNameRequired = errors.New("policy name is required")
var ErrPolicyNotFound = errors.New("policy not found")
var ErrInvalidPolicyStatus = errors.New("invalid policy status")
var ErrInvalidScopeType = errors.New("invalid scope type")
var ErrUserNameRequired = errors.New("user name is required")
var ErrUserEmailRequired = errors.New("user email is required")
var ErrInvalidUserStatus = errors.New("invalid user status")
var ErrUserGroupNameRequired = errors.New("user group name is required")
var ErrUserGroupNotFound = errors.New("user group not found")
var ErrTeamNameRequired = errors.New("team name is required")
var ErrTeamNotFound = errors.New("team not found")
var ErrTeamIDRequired = errors.New("team_id is required")
var ErrInvalidTeamMemberType = errors.New("invalid team member type")
var ErrTeamMemberIDRequired = errors.New("member_id is required")
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

type Role struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	AppliesTo   string          `json:"applies_to"`
	Description string          `json:"description,omitempty"`
	Permissions map[string]bool `json:"permissions"`
	BuiltIn     bool            `json:"built_in"`
}

type RoleAssignment struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	RoleID        string    `json:"role_id"`
	AppliesToType string    `json:"applies_to_type"`
	AppliesToID   string    `json:"applies_to_id"`
	AssigneeType  string    `json:"assignee_type"`
	AssigneeID    string    `json:"assignee_id"`
	AssigneeEmail string    `json:"assignee_email,omitempty"`
	ValidFrom     string    `json:"valid_from,omitempty"`
	ValidUntil    string    `json:"valid_until,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RoleAssignmentInput struct {
	TenantID      string
	RoleID        string
	AppliesToType string
	AppliesToID   string
	AssigneeType  string
	AssigneeID    string
	AssigneeEmail string
	ValidFrom     string
	ValidUntil    string
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
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	BuildingID  string    `json:"building_id,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Members     []string  `json:"members,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
	DeliveryMethod    string    `json:"delivery_method"`
	GranteeName       string    `json:"grantee_name"`
	GranteeGender     string    `json:"grantee_gender,omitempty"`
	GranteePhone      string    `json:"grantee_phone"`
	GranteeEmail      string    `json:"grantee_email"`
	MobileModel       string    `json:"mobile_model,omitempty"`
	PassType          string    `json:"pass_type,omitempty"`
	ValidUntil        string    `json:"valid_until"`
	AuthorizedByID    string    `json:"authorized_by_id,omitempty"`
	AuthorizedByEmail string    `json:"authorized_by_email,omitempty"`
	AuthorizedByRole  string    `json:"authorized_by_role,omitempty"`
	AuthorizedAt      time.Time `json:"authorized_at"`
	CreatedAt         time.Time `json:"created_at"`
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
	Users           []AccessUser      `json:"users"`
	UserGroups      []UserGroup       `json:"user_groups"`
	Policies        []Policy          `json:"policies"`
	TemporaryAccess []TemporaryAccess `json:"temporary_access"`
	VisitorPasses   []VisitorPass     `json:"visitor_passes"`
	RoleAssignments []RoleAssignment  `json:"role_assignments"`
	Teams           []Team            `json:"teams"`
	TeamMemberships []TeamMembership  `json:"team_memberships"`
}

type Service struct {
	mu              sync.RWMutex
	users           []AccessUser
	userGroups      []UserGroup
	policies        []Policy
	temporaryAccess []TemporaryAccess
	visitorPasses   []VisitorPass
	roleAssignments []RoleAssignment
	teams           []Team
	teamMemberships []TeamMembership
	stateStore      StateStore
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
		},
		userGroups: []UserGroup{
			{
				ID:          "ug_common_office_jkt",
				TenantID:    "tenant_demo_jakarta",
				BuildingID:  "building_demo_001",
				Name:        "Common Office Access",
				Description: "Default office/public access for regular employees",
				Members:     []string{"usr_1001"},
				CreatedAt:   now,
				UpdatedAt:   now,
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
		items = append(items, s.userGroups[i])
	}
	return items
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
		ID:          id,
		TenantID:    nextTenantID,
		BuildingID:  strings.TrimSpace(buildingID),
		Name:        nextName,
		Description: strings.TrimSpace(description),
		Members:     uniqueIDs(members),
		CreatedAt:   now,
		UpdatedAt:   now,
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
		s.userGroups[i].Description = strings.TrimSpace(description)
		s.userGroups[i].Members = uniqueIDs(members)
		s.userGroups[i].UpdatedAt = time.Now().UTC()
		if err := s.persistLocked(); err != nil {
			return UserGroup{}, err
		}
		return s.userGroups[i], nil
	}

	return UserGroup{}, ErrUserGroupNotFound
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

func (s *Service) CreateTemporaryAccess(
	tenantID, scopeType, buildingID, areaID, doorID, deliveryMethod,
	granteeName, granteeGender, granteePhone, granteeEmail, mobileModel, passType, validUntil,
	authorizedByID, authorizedByEmail, authorizedByRole string,
) (TemporaryAccess, error) {
	nextTenantID := strings.TrimSpace(tenantID)
	if nextTenantID == "" {
		return TemporaryAccess{}, ErrTenantIDRequired
	}

	nextScopeType, err := normalizeScopeType(scopeType)
	if err != nil {
		return TemporaryAccess{}, err
	}

	nextMethod, err := normalizeDeliveryMethod(deliveryMethod)
	if err != nil {
		return TemporaryAccess{}, err
	}

	nextGranteeName := strings.TrimSpace(granteeName)
	if nextGranteeName == "" {
		return TemporaryAccess{}, ErrGranteeNameRequired
	}
	nextGranteePhone := strings.TrimSpace(granteePhone)
	if nextGranteePhone == "" {
		return TemporaryAccess{}, ErrGranteePhoneRequired
	}
	nextGranteeEmail := strings.TrimSpace(granteeEmail)
	if nextGranteeEmail == "" {
		return TemporaryAccess{}, ErrGranteeEmailRequired
	}

	nextValidUntil := strings.TrimSpace(validUntil)
	if nextValidUntil == "" {
		return TemporaryAccess{}, ErrValidUntilRequired
	}

	id, err := accessID("tmp_")
	if err != nil {
		return TemporaryAccess{}, err
	}

	record := TemporaryAccess{
		ID:                id,
		TenantID:          nextTenantID,
		ScopeType:         nextScopeType,
		BuildingID:        strings.TrimSpace(buildingID),
		AreaID:            strings.TrimSpace(areaID),
		DoorID:            strings.TrimSpace(doorID),
		DeliveryMethod:    nextMethod,
		GranteeName:       nextGranteeName,
		GranteeGender:     strings.TrimSpace(granteeGender),
		GranteePhone:      nextGranteePhone,
		GranteeEmail:      nextGranteeEmail,
		MobileModel:       strings.TrimSpace(mobileModel),
		PassType:          strings.TrimSpace(passType),
		ValidUntil:        nextValidUntil,
		AuthorizedByID:    strings.TrimSpace(authorizedByID),
		AuthorizedByEmail: normalizeEmail(authorizedByEmail),
		AuthorizedByRole:  strings.TrimSpace(authorizedByRole),
		AuthorizedAt:      time.Now().UTC(),
		CreatedAt:         time.Now().UTC(),
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
			Users:           cloneAccessUsers(s.users),
			UserGroups:      cloneUserGroups(s.userGroups),
			Policies:        clonePolicies(s.policies),
			TemporaryAccess: cloneTemporaryAccess(s.temporaryAccess),
			VisitorPasses:   cloneVisitorPasses(s.visitorPasses),
			RoleAssignments: cloneRoleAssignments(s.roleAssignments),
			Teams:           cloneTeams(s.teams),
			TeamMemberships: cloneTeamMemberships(s.teamMemberships),
		})
	}

	s.mu.Lock()
	s.users = cloneAccessUsers(snapshot.Users)
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
	s.mu.Unlock()
	return nil
}

func (s *Service) persistLocked() error {
	if s.stateStore == nil {
		return nil
	}
	return s.stateStore.Save(stateKey, stateSnapshot{
		Users:           cloneAccessUsers(s.users),
		UserGroups:      cloneUserGroups(s.userGroups),
		Policies:        clonePolicies(s.policies),
		TemporaryAccess: cloneTemporaryAccess(s.temporaryAccess),
		VisitorPasses:   cloneVisitorPasses(s.visitorPasses),
		RoleAssignments: cloneRoleAssignments(s.roleAssignments),
		Teams:           cloneTeams(s.teams),
		TeamMemberships: cloneTeamMemberships(s.teamMemberships),
	})
}

func cloneAccessUsers(items []AccessUser) []AccessUser {
	output := make([]AccessUser, 0, len(items))
	for i := range items {
		record := items[i]
		record.GroupIDs = append([]string(nil), items[i].GroupIDs...)
		output = append(output, record)
	}
	return output
}

func cloneUserGroups(items []UserGroup) []UserGroup {
	output := make([]UserGroup, 0, len(items))
	for i := range items {
		record := items[i]
		record.Members = append([]string(nil), items[i].Members...)
		output = append(output, record)
	}
	return output
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
