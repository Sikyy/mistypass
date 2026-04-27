package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/enterprise"
	"github.com/mistypass/cloud/api/internal/modules/event"
	"github.com/mistypass/cloud/api/internal/modules/gateway"
	"github.com/mistypass/cloud/api/internal/modules/space"
	"github.com/mistypass/cloud/api/internal/modules/wallet"
)

type referenceRoleAssignmentPayload struct {
	TenantID      string `json:"tenant_id"`
	RoleID        string `json:"role_id"`
	AppliesToType string `json:"applies_to_type"`
	AppliesToID   string `json:"applies_to_id"`
	AssigneeType  string `json:"assignee_type"`
	AssigneeID    string `json:"assignee_id"`
	AssigneeEmail string `json:"assignee_email"`
	ValidFrom     string `json:"valid_from"`
	ValidUntil    string `json:"valid_until"`
}

type referenceShare struct {
	ID                string `json:"id"`
	TenantID          string `json:"tenant_id"`
	Email             string `json:"email"`
	GroupID           string `json:"group_id,omitempty"`
	RoleID            string `json:"role_id"`
	PlaceID           string `json:"place_id,omitempty"`
	LockID            string `json:"lock_id,omitempty"`
	ValidFrom         string `json:"valid_from,omitempty"`
	ValidUntil        string `json:"valid_until"`
	Status            string `json:"status"`
	DeliveryMethod    string `json:"delivery_method"`
	GranteeName       string `json:"grantee_name,omitempty"`
	AuthorizedByID    string `json:"authorized_by_id,omitempty"`
	AuthorizedByEmail string `json:"authorized_by_email,omitempty"`
	AuthorizedByRole  string `json:"authorized_by_role,omitempty"`
	CreatedAt         string `json:"created_at"`
}

type referenceSharePayload struct {
	TenantID       string `json:"tenant_id"`
	Email          string `json:"email"`
	GroupID        string `json:"group_id"`
	RoleID         string `json:"role_id"`
	PlaceID        string `json:"place_id"`
	BuildingID     string `json:"building_id"`
	LockID         string `json:"lock_id"`
	DoorID         string `json:"door_id"`
	ValidFrom      string `json:"valid_from"`
	ValidUntil     string `json:"valid_until"`
	DeliveryMethod string `json:"delivery_method"`
	GranteeName    string `json:"grantee_name"`
	GranteePhone   string `json:"grantee_phone"`
	MobileModel    string `json:"mobile_model"`
	PassType       string `json:"pass_type"`
}

type referenceGroupLock struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	GroupID   string `json:"group_id"`
	LockID    string `json:"lock_id"`
	CreatedAt string `json:"created_at"`
}

type referenceController struct {
	ID           string   `json:"id"`
	ResourceType string   `json:"resource_type"`
	TenantID     string   `json:"tenant_id"`
	PlaceID      string   `json:"place_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	DeviceID     string   `json:"device_id"`
	Token        string   `json:"token"`
	Status       string   `json:"status"`
	Configured   bool     `json:"configured"`
	LockIDs      []string `json:"lock_ids,omitempty"`
	LastSeenAt   string   `json:"last_seen_at"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type referenceReader struct {
	ID           string   `json:"id"`
	ResourceType string   `json:"resource_type"`
	TenantID     string   `json:"tenant_id"`
	PlaceID      string   `json:"place_id"`
	ControllerID string   `json:"controller_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	DeviceID     string   `json:"device_id"`
	Token        string   `json:"token"`
	Model        string   `json:"model"`
	Protocol     string   `json:"protocol"`
	Status       string   `json:"status"`
	Configured   bool     `json:"configured"`
	LockIDs      []string `json:"lock_ids,omitempty"`
	LastSeenAt   string   `json:"last_seen_at"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

type referenceTerminalPlace struct {
	ID           string `json:"id"`
	ResourceType string `json:"resource_type"`
	Name         string `json:"name"`
}

type referenceTerminal struct {
	ID                        string                 `json:"id"`
	ResourceType              string                 `json:"resource_type"`
	TenantID                  string                 `json:"tenant_id"`
	CreatedAt                 string                 `json:"created_at"`
	UpdatedAt                 string                 `json:"updated_at"`
	Name                      string                 `json:"name"`
	Description               string                 `json:"description"`
	PlaceID                   string                 `json:"place_id"`
	Place                     referenceTerminalPlace `json:"place"`
	MarketplaceInstallationID *string                `json:"marketplace_installation_id"`
	ControllerID              string                 `json:"controller_id,omitempty"`
	ReaderID                  string                 `json:"reader_id,omitempty"`
	Status                    string                 `json:"status,omitempty"`
	LastSeenAt                string                 `json:"last_seen_at,omitempty"`
}

type referenceIntegration struct {
	ID           string `json:"id"`
	ResourceType string `json:"resource_type"`
	TenantID     string `json:"tenant_id"`
	Type         string `json:"type"`
	Provider     string `json:"provider"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Configured   bool   `json:"configured"`
	SyncMode     string `json:"sync_mode,omitempty"`
	SourceID     string `json:"source_id,omitempty"`
	LastSyncAt   string `json:"last_sync_at,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type referenceCard struct {
	ID              string `json:"id"`
	ResourceType    string `json:"resource_type"`
	TenantID        string `json:"tenant_id"`
	Status          string `json:"status"`
	Token           string `json:"token"`
	UID             string `json:"uid,omitempty"`
	CardNumber      string `json:"card_number,omitempty"`
	Provider        string `json:"provider"`
	TemplateID      string `json:"template_id"`
	UserID          string `json:"user_id,omitempty"`
	AssigneeType    string `json:"assignee_type,omitempty"`
	AssigneeID      string `json:"assignee_id,omitempty"`
	ActivationToken string `json:"activation_token,omitempty"`
	LastUsedAt      string `json:"last_used_at,omitempty"`
	IssuedAt        string `json:"issued_at"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type referenceCardAssignment struct {
	ID           string        `json:"id"`
	ResourceType string        `json:"resource_type"`
	TenantID     string        `json:"tenant_id"`
	Status       string        `json:"status"`
	CardID       string        `json:"card_id"`
	Card         referenceCard `json:"card"`
	AssigneeType string        `json:"assignee_type"`
	AssigneeID   string        `json:"assignee_id"`
	UserID       string        `json:"user_id,omitempty"`
	CreatedAt    string        `json:"created_at"`
	UpdatedAt    string        `json:"updated_at"`
}

type referenceEventSetPayload struct {
	Interval        string `json:"interval"`
	EventPlaceID    string `json:"event_place_id"`
	PlaceID         string `json:"place_id"`
	EventType       string `json:"event_type"`
	EventUUID       string `json:"event_uuid"`
	EventSuccess    *bool  `json:"event_success"`
	EventObjectID   string `json:"event_object_id"`
	EventObjectType string `json:"event_object_type"`
}

type referenceEventSet struct {
	ID              string           `json:"id"`
	CreatedAt       string           `json:"created_at"`
	Status          string           `json:"status"`
	Interval        string           `json:"interval,omitempty"`
	EventPlaceID    string           `json:"event_place_id,omitempty"`
	PlaceID         string           `json:"place_id,omitempty"`
	EventType       string           `json:"event_type,omitempty"`
	EventUUID       string           `json:"event_uuid,omitempty"`
	EventSuccess    *bool            `json:"event_success,omitempty"`
	EventObjectID   string           `json:"event_object_id,omitempty"`
	EventObjectType string           `json:"event_object_type,omitempty"`
	Events          []referenceEvent `json:"events"`
	Cursor          string           `json:"cursor,omitempty"`
}

type referenceEvent struct {
	UUID       string `json:"uuid"`
	Type       string `json:"type"`
	ActorType  string `json:"actor_type,omitempty"`
	ActorID    string `json:"actor_id,omitempty"`
	ActorName  string `json:"actor_name,omitempty"`
	ActorEmail string `json:"actor_email,omitempty"`
	ObjectType string `json:"object_type,omitempty"`
	ObjectID   string `json:"object_id,omitempty"`
	ObjectName string `json:"object_name,omitempty"`
	PlaceID    string `json:"place_id,omitempty"`
	LockID     string `json:"lock_id,omitempty"`
	Success    bool   `json:"success"`
	Result     string `json:"result"`
	Detail     string `json:"detail,omitempty"`
	CreatedAt  string `json:"created_at"`
}

func (s *server) listReferencePlaces(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.spaceSvc.ListBuildings(tenantID)
	if buildingScope != nil {
		items = filterBuildingsByScope(items, buildingScope)
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" {
		items = filterBuildingsByID(items, placeID)
	}
	writeCollection(w, http.StatusOK, items)
}

func (s *server) createReferencePlace(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Place struct {
			TenantID string `json:"tenant_id"`
			Name     string `json:"name"`
			Address  string `json:"address"`
			Region   string `json:"region"`
		} `json:"place"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.Place.TenantID)
	if !ok {
		return
	}
	created, err := s.spaceSvc.CreateBuilding(tenantID, request.Place.Name, request.Place.Address, request.Place.Region)
	if err != nil {
		switch {
		case errors.Is(err, space.ErrTenantIDRequired), errors.Is(err, space.ErrBuildingNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) listReferenceLocks(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.spaceSvc.ListDoors(tenantID)
	if buildingScope != nil {
		items = filterDoorsByScope(items, buildingScope)
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" {
		items = filterDoorsByPlaceID(items, placeID)
	}
	writeCollection(w, http.StatusOK, items)
}

func (s *server) createReferenceLock(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Lock struct {
			TenantID   string `json:"tenant_id"`
			PlaceID    string `json:"place_id"`
			BuildingID string `json:"building_id"`
			FloorID    string `json:"floor_id"`
			AreaID     string `json:"area_id"`
			Name       string `json:"name"`
			GatewayID  string `json:"gateway_id"`
			Kind       string `json:"kind"`
			Status     string `json:"status"`
			Mode       string `json:"mode"`
			State      string `json:"state"`
		} `json:"lock"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.Lock.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	buildingID := firstNonEmptyString(request.Lock.PlaceID, request.Lock.BuildingID)
	if !s.requireBuildingScope(w, buildingScope, buildingID) {
		return
	}
	kind := firstNonEmptyString(request.Lock.Kind, request.Lock.Mode)
	status := firstNonEmptyString(request.Lock.Status, request.Lock.State)

	created, err := s.spaceSvc.CreateDoor(
		tenantID,
		buildingID,
		request.Lock.FloorID,
		request.Lock.AreaID,
		request.Lock.Name,
		request.Lock.GatewayID,
		kind,
		status,
	)
	if err != nil {
		handleSpaceMutationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) listReferenceGroups(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListUserGroups(tenantID)
	if buildingScope != nil {
		items = filterUserGroupsByScope(items, buildingScope)
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" {
		items = filterUserGroupsByPlaceID(items, placeID)
	}
	writeCollection(w, http.StatusOK, items)
}

func (s *server) createReferenceGroup(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Group struct {
			TenantID    string   `json:"tenant_id"`
			PlaceID     string   `json:"place_id"`
			BuildingID  string   `json:"building_id"`
			Name        string   `json:"name"`
			Description string   `json:"description"`
			MemberIDs   []string `json:"member_ids"`
			Members     []string `json:"members"`
		} `json:"group"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.Group.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	buildingID := firstNonEmptyString(request.Group.PlaceID, request.Group.BuildingID)
	if buildingScope != nil && buildingID == "" {
		writeError(w, http.StatusBadRequest, "place_id is required for Place Admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, buildingID) {
		return
	}
	members := request.Group.MemberIDs
	if len(members) == 0 {
		members = request.Group.Members
	}
	created, err := s.accessSvc.CreateUserGroup(tenantID, buildingID, request.Group.Name, request.Group.Description, members)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrTenantIDRequired), errors.Is(err, access.ErrUserGroupNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) listReferenceGroupLocks(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	doorGroups := s.spaceSvc.ListDoorGroups(tenantID)
	doors := s.spaceSvc.ListDoors(tenantID)
	if buildingScope != nil {
		allowedDoorIDs := allowedDoorIDsByBuildingScope(doors, buildingScope)
		doorGroups = filterDoorGroupsByScope(doorGroups, allowedDoorIDs)
		doors = filterDoorsByScope(doors, buildingScope)
	}
	doorsByID := make(map[string]space.Door, len(doors))
	for i := range doors {
		doorsByID[doors[i].ID] = doors[i]
	}
	groupID := strings.TrimSpace(r.URL.Query().Get("group_id"))
	lockID := strings.TrimSpace(r.URL.Query().Get("lock_id"))
	placeID := strings.TrimSpace(r.URL.Query().Get("place_id"))

	items := make([]referenceGroupLock, 0)
	for i := range doorGroups {
		if groupID != "" && doorGroups[i].ID != groupID {
			continue
		}
		for _, doorID := range doorGroups[i].DoorIDs {
			if lockID != "" && doorID != lockID {
				continue
			}
			door, exists := doorsByID[doorID]
			if !exists {
				continue
			}
			if placeID != "" && door.BuildingID != placeID {
				continue
			}
			items = append(items, referenceGroupLock{
				ID:        doorGroups[i].ID + ":" + doorID,
				TenantID:  doorGroups[i].TenantID,
				GroupID:   doorGroups[i].ID,
				LockID:    doorID,
				CreatedAt: doorGroups[i].CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			})
		}
	}
	writeCollection(w, http.StatusOK, items)
}

func (s *server) listReferenceControllers(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.gatewaySvc.List(tenantID)
	if buildingScope != nil {
		items = filterGatewaysByScope(items, buildingScope)
	}
	controllers := make([]referenceController, 0, len(items))
	for i := range items {
		controller := referenceControllerFromGateway(items[i])
		if !controllerMatchesQuery(controller, r) {
			continue
		}
		controllers = append(controllers, controller)
	}
	sortReferenceControllers(controllers, r)
	writeCollection(w, http.StatusOK, controllers)
}

func (s *server) listReferenceReaders(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.gatewaySvc.List(tenantID)
	if buildingScope != nil {
		items = filterGatewaysByScope(items, buildingScope)
	}
	readers := make([]referenceReader, 0)
	for i := range items {
		for j := range items[i].Devices {
			if !isReaderDevice(items[i].Devices[j]) {
				continue
			}
			reader := referenceReaderFromGatewayDevice(items[i], items[i].Devices[j])
			if !readerMatchesQuery(reader, r) {
				continue
			}
			readers = append(readers, reader)
		}
	}
	sortReferenceReaders(readers, r)
	writeCollection(w, http.StatusOK, readers)
}

func (s *server) listReferenceTerminals(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	places := map[string]space.Building{}
	for _, place := range s.spaceSvc.ListBuildings(tenantID) {
		places[place.ID] = place
	}
	items := s.gatewaySvc.List(tenantID)
	if buildingScope != nil {
		items = filterGatewaysByScope(items, buildingScope)
	}
	terminals := make([]referenceTerminal, 0)
	for i := range items {
		placeName := items[i].BuildingID
		if place, exists := places[items[i].BuildingID]; exists {
			placeName = place.Name
		}
		for j := range items[i].Devices {
			if !isReaderDevice(items[i].Devices[j]) {
				continue
			}
			terminal := referenceTerminalFromGatewayDevice(items[i], items[i].Devices[j], placeName)
			if !terminalMatchesQuery(terminal, r) {
				continue
			}
			terminals = append(terminals, terminal)
		}
	}
	sortReferenceTerminals(terminals, r)
	writeCollection(w, http.StatusOK, terminals)
}

func (s *server) listReferenceIntegrations(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	integrations := make([]referenceIntegration, 0)
	if tenantID != "" {
		if config, err := s.enterpriseSvc.GetIDPConfig(tenantID); err == nil {
			integration := referenceIntegrationFromIDPConfig(config)
			if integrationMatchesQuery(integration, r) {
				integrations = append(integrations, integration)
			}
		} else if !errors.Is(err, enterprise.ErrIDPConfigNotFound) {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	connectors := s.enterpriseSvc.ListHRISConnectors(tenantID)
	for i := range connectors {
		integration := referenceIntegrationFromHRISConnector(connectors[i])
		if !integrationMatchesQuery(integration, r) {
			continue
		}
		integrations = append(integrations, integration)
	}
	sortReferenceIntegrations(integrations, r)
	writeCollection(w, http.StatusOK, integrations)
}

func (s *server) listReferenceTeams(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListTeams(tenantID)
	if buildingScope != nil {
		items = filterTeamsByScope(items, buildingScope)
	}
	items = filterTeamsByQuery(items, r)
	writeCollection(w, http.StatusOK, items)
}

func (s *server) listReferenceTeamMemberships(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	teams := s.accessSvc.ListTeams(tenantID)
	var teamIDs map[string]struct{}
	if buildingScope != nil {
		teams = filterTeamsByScope(teams, buildingScope)
		teamIDs = make(map[string]struct{}, len(teams))
		for i := range teams {
			teamIDs[teams[i].ID] = struct{}{}
		}
	}
	items := s.accessSvc.ListTeamMemberships(tenantID)
	items = filterTeamMembershipsByQuery(items, teamIDs, r)
	writeCollection(w, http.StatusOK, items)
}

func (s *server) listReferenceRoles(w http.ResponseWriter, r *http.Request) {
	appliesTo := strings.TrimSpace(r.URL.Query().Get("applies_to"))
	items := s.accessSvc.ListRoles()
	if appliesTo != "" {
		filtered := make([]access.Role, 0, len(items))
		for i := range items {
			if strings.EqualFold(items[i].AppliesTo, appliesTo) {
				filtered = append(filtered, items[i])
			}
		}
		items = filtered
	}
	writeCollection(w, http.StatusOK, items)
}

func (s *server) getReferenceRole(w http.ResponseWriter, r *http.Request) {
	roleID := strings.TrimSpace(chi.URLParam(r, "roleID"))
	for _, role := range s.accessSvc.ListRoles() {
		if role.ID == roleID {
			writeJSON(w, http.StatusOK, role)
			return
		}
	}
	writeError(w, http.StatusNotFound, access.ErrRoleNotFound.Error())
}

func (s *server) listReferenceRoleAssignments(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListRoleAssignments(tenantID)
	if buildingScope != nil {
		items = filterRoleAssignmentsByBuildingScope(items, buildingScope)
	}
	items = filterRoleAssignmentsByQuery(items, r)
	writeCollection(w, http.StatusOK, items)
}

func (s *server) createReferenceRoleAssignment(w http.ResponseWriter, r *http.Request) {
	var request struct {
		RoleAssignment referenceRoleAssignmentPayload `json:"role_assignment"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.RoleAssignment.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !validateRoleAssignmentScope(w, buildingScope, request.RoleAssignment.AppliesToType, request.RoleAssignment.AppliesToID) {
		return
	}
	created, err := s.accessSvc.CreateRoleAssignment(referenceRoleAssignmentInput(tenantID, request.RoleAssignment))
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) updateReferenceRoleAssignment(w http.ResponseWriter, r *http.Request) {
	assignmentID := chi.URLParam(r, "assignmentID")
	var request struct {
		RoleAssignment referenceRoleAssignmentPayload `json:"role_assignment"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.RoleAssignment.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if !validateRoleAssignmentScope(w, buildingScope, request.RoleAssignment.AppliesToType, request.RoleAssignment.AppliesToID) {
		return
	}
	updated, err := s.accessSvc.UpdateRoleAssignment(tenantID, assignmentID, referenceRoleAssignmentInput(tenantID, request.RoleAssignment))
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) listReferenceMembers(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListUsers(tenantID)
	if buildingScope != nil {
		items = filterUsersByScope(items, buildingScope)
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" {
		items = filterAccessUsersByPlaceID(items, placeID)
	}
	writeCollection(w, http.StatusOK, items)
}

func (s *server) listReferenceShares(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	items := s.accessSvc.ListTemporaryAccess(tenantID)
	if buildingScope != nil {
		items = filterTemporaryAccessByScope(items, buildingScope)
	}
	shares := make([]referenceShare, 0, len(items))
	for i := range items {
		share := referenceShareFromTemporaryAccess(items[i])
		if !shareMatchesQuery(share, r) {
			continue
		}
		shares = append(shares, share)
	}
	writeCollection(w, http.StatusOK, shares)
}

func (s *server) createReferenceShare(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Share referenceSharePayload `json:"share"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.Share.TenantID)
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	buildingID := firstNonEmptyString(request.Share.PlaceID, request.Share.BuildingID)
	if buildingScope != nil && buildingID == "" {
		writeError(w, http.StatusBadRequest, "place_id is required for Place Admin")
		return
	}
	if !s.requireBuildingScope(w, buildingScope, buildingID) {
		return
	}
	lockID := firstNonEmptyString(request.Share.LockID, request.Share.DoorID)
	scopeType := "all"
	if buildingID != "" {
		scopeType = "building"
	}
	if lockID != "" {
		scopeType = "door"
	}
	authorizedByID := "system"
	authorizedByEmail := ""
	authorizedByRole := "system"
	if user, ok := authenticatedUser(r); ok {
		authorizedByID = strings.TrimSpace(user.ID)
		authorizedByEmail = strings.TrimSpace(user.Email)
		authorizedByRole = strings.TrimSpace(user.Role)
	}
	granteeName := firstNonEmptyString(request.Share.GranteeName, request.Share.Email)
	granteePhone := firstNonEmptyString(request.Share.GranteePhone, "not_provided")
	passType := firstNonEmptyString(request.Share.PassType, "share")

	created, err := s.accessSvc.CreateTemporaryAccess(
		tenantID,
		scopeType,
		buildingID,
		"",
		lockID,
		firstNonEmptyString(request.Share.DeliveryMethod, "email_qr"),
		granteeName,
		"",
		granteePhone,
		request.Share.Email,
		request.Share.MobileModel,
		passType,
		request.Share.ValidUntil,
		authorizedByID,
		authorizedByEmail,
		authorizedByRole,
	)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, referenceShareFromTemporaryAccess(created))
}

func (s *server) listReferenceCards(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	passes := s.walletSvc.ListPasses(tenantID)
	items := make([]referenceCard, 0, len(passes))
	for i := range passes {
		card := referenceCardFromPass(passes[i])
		if !cardMatchesQuery(card, r) {
			continue
		}
		items = append(items, card)
	}
	writeCollection(w, http.StatusOK, items)
}

func (s *server) listReferenceCardAssignments(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	passes := s.walletSvc.ListPasses(tenantID)
	items := make([]referenceCardAssignment, 0, len(passes))
	for i := range passes {
		assignment := referenceCardAssignmentFromPass(passes[i])
		if !cardAssignmentMatchesQuery(assignment, r) {
			continue
		}
		items = append(items, assignment)
	}
	writeCollection(w, http.StatusOK, items)
}

func (s *server) activateReferenceCard(w http.ResponseWriter, r *http.Request) {
	s.changeReferenceCardStatus(w, r, "active")
}

func (s *server) deactivateReferenceCard(w http.ResponseWriter, r *http.Request) {
	s.changeReferenceCardStatus(w, r, "suspended")
}

func (s *server) createReferenceEventSet(w http.ResponseWriter, r *http.Request) {
	var request struct {
		EventSet referenceEventSetPayload `json:"event_set"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	placeID := firstNonEmptyString(request.EventSet.PlaceID, request.EventSet.EventPlaceID)
	if !s.requireBuildingScope(w, buildingScope, placeIDForEventSetScope(placeID, buildingScope)) {
		return
	}
	writeJSON(w, http.StatusOK, s.referenceEventSetSnapshot(tenantID, buildingScope, request.EventSet))
}

func (s *server) getReferenceEventSet(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	payload := referenceEventSetPayload{
		PlaceID:   strings.TrimSpace(r.URL.Query().Get("place_id")),
		EventType: strings.TrimSpace(r.URL.Query().Get("event_type")),
		EventUUID: strings.TrimSpace(r.URL.Query().Get("event_uuid")),
		Interval:  strings.TrimSpace(r.URL.Query().Get("interval")),
	}
	if !s.requireBuildingScope(w, buildingScope, placeIDForEventSetScope(payload.PlaceID, buildingScope)) {
		return
	}
	set := s.referenceEventSetSnapshot(tenantID, buildingScope, payload)
	set.ID = strings.TrimSpace(chi.URLParam(r, "eventSetID"))
	if set.ID == "" {
		set.ID = referenceEventSetID(tenantID, payload)
	}
	writeJSON(w, http.StatusOK, set)
}

func (s *server) getReferenceEventMetadata(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"object_type_to_action": map[string][]string{
			"Lock":       {"access_granted", "access_denied"},
			"Controller": {"gateway_event"},
			"Reader":     {"gateway_event"},
		},
	})
}

func (s *server) listReferenceEventTypes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []string{
		"access_granted",
		"access_denied",
		"gateway_event",
	})
}

func (s *server) changeReferenceCardStatus(w http.ResponseWriter, r *http.Request, status string) {
	cardID := strings.TrimSpace(chi.URLParam(r, "cardID"))
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	var (
		record wallet.PassInstance
		err    error
	)
	switch status {
	case "active":
		record, err = s.walletSvc.ActivatePass(tenantID, cardID, "admin")
	case "suspended":
		record, err = s.walletSvc.SuspendPass(tenantID, cardID, "admin")
	default:
		writeError(w, http.StatusBadRequest, "invalid card status operation")
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, wallet.ErrPassNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, wallet.ErrInvalidPassTransition):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, referenceCardFromPass(record))
}

func referenceRoleAssignmentInput(tenantID string, payload referenceRoleAssignmentPayload) access.RoleAssignmentInput {
	return access.RoleAssignmentInput{
		TenantID:      tenantID,
		RoleID:        payload.RoleID,
		AppliesToType: payload.AppliesToType,
		AppliesToID:   payload.AppliesToID,
		AssigneeType:  payload.AssigneeType,
		AssigneeID:    payload.AssigneeID,
		AssigneeEmail: payload.AssigneeEmail,
		ValidFrom:     payload.ValidFrom,
		ValidUntil:    payload.ValidUntil,
	}
}

func validateRoleAssignmentScope(w http.ResponseWriter, scope map[string]struct{}, appliesToType, appliesToID string) bool {
	if scope == nil {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(appliesToType), "Place") {
		writeError(w, http.StatusForbidden, "place scope forbidden")
		return false
	}
	placeID := strings.TrimSpace(appliesToID)
	if placeID == "" {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return false
	}
	if _, exists := scope[placeID]; !exists {
		writeError(w, http.StatusForbidden, "building scope forbidden")
		return false
	}
	return true
}

func filterRoleAssignmentsByBuildingScope(items []access.RoleAssignment, scope map[string]struct{}) []access.RoleAssignment {
	filtered := make([]access.RoleAssignment, 0, len(items))
	for i := range items {
		if items[i].AppliesToType != "Place" {
			continue
		}
		if _, exists := scope[items[i].AppliesToID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterTeamsByScope(items []access.Team, scope map[string]struct{}) []access.Team {
	filtered := make([]access.Team, 0, len(items))
	for i := range items {
		if _, exists := scope[items[i].PlaceID]; !exists {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterTeamsByQuery(items []access.Team, r *http.Request) []access.Team {
	idFilter := commaSet(r.URL.Query().Get("ids"))
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	scope := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	placeID := strings.TrimSpace(r.URL.Query().Get("place_id"))

	filtered := make([]access.Team, 0, len(items))
	for i := range items {
		if len(idFilter) > 0 {
			if _, exists := idFilter[items[i].ID]; !exists {
				continue
			}
		}
		if query != "" && !strings.Contains(strings.ToLower(items[i].Name), query) {
			continue
		}
		if scope != "" && !strings.EqualFold(items[i].Scope, scope) {
			continue
		}
		if placeID != "" && items[i].PlaceID != placeID {
			continue
		}
		filtered = append(filtered, items[i])
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if strings.TrimSpace(r.URL.Query().Get("sort")) == "-name" {
			return filtered[i].Name > filtered[j].Name
		}
		return filtered[i].Name < filtered[j].Name
	})
	return filtered
}

func filterTeamMembershipsByQuery(items []access.TeamMembership, allowedTeamIDs map[string]struct{}, r *http.Request) []access.TeamMembership {
	idFilter := commaSet(r.URL.Query().Get("ids"))
	teamID := strings.TrimSpace(r.URL.Query().Get("team_id"))
	memberType := strings.TrimSpace(r.URL.Query().Get("member_type"))
	memberID := strings.TrimSpace(r.URL.Query().Get("member_id"))

	filtered := make([]access.TeamMembership, 0, len(items))
	for i := range items {
		if allowedTeamIDs != nil {
			if _, exists := allowedTeamIDs[items[i].TeamID]; !exists {
				continue
			}
		}
		if len(idFilter) > 0 {
			if _, exists := idFilter[items[i].ID]; !exists {
				continue
			}
		}
		if teamID != "" && items[i].TeamID != teamID {
			continue
		}
		if memberType != "" && !strings.EqualFold(items[i].MemberType, memberType) {
			continue
		}
		if memberID != "" && items[i].MemberID != memberID {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func filterRoleAssignmentsByQuery(items []access.RoleAssignment, r *http.Request) []access.RoleAssignment {
	roleID := strings.TrimSpace(r.URL.Query().Get("role_id"))
	appliesToType := strings.TrimSpace(r.URL.Query().Get("applies_to_type"))
	appliesToID := strings.TrimSpace(r.URL.Query().Get("applies_to_id"))
	assigneeType := strings.TrimSpace(r.URL.Query().Get("assignee_type"))
	assigneeID := strings.TrimSpace(r.URL.Query().Get("assignee_id"))

	filtered := make([]access.RoleAssignment, 0, len(items))
	for i := range items {
		if roleID != "" && items[i].RoleID != roleID {
			continue
		}
		if appliesToType != "" && !strings.EqualFold(items[i].AppliesToType, appliesToType) {
			continue
		}
		if appliesToID != "" && items[i].AppliesToID != appliesToID {
			continue
		}
		if assigneeType != "" && !strings.EqualFold(items[i].AssigneeType, assigneeType) {
			continue
		}
		if assigneeID != "" && items[i].AssigneeID != assigneeID {
			continue
		}
		filtered = append(filtered, items[i])
	}
	return filtered
}

func referenceShareFromTemporaryAccess(item access.TemporaryAccess) referenceShare {
	status := "active"
	return referenceShare{
		ID:                item.ID,
		TenantID:          item.TenantID,
		Email:             item.GranteeEmail,
		RoleID:            "role_group_access",
		PlaceID:           item.BuildingID,
		LockID:            item.DoorID,
		ValidUntil:        item.ValidUntil,
		Status:            status,
		DeliveryMethod:    item.DeliveryMethod,
		GranteeName:       item.GranteeName,
		AuthorizedByID:    item.AuthorizedByID,
		AuthorizedByEmail: item.AuthorizedByEmail,
		AuthorizedByRole:  item.AuthorizedByRole,
		CreatedAt:         item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func referenceCardFromPass(item wallet.PassInstance) referenceCard {
	assigneeType := ""
	userID := ""
	if strings.EqualFold(item.TargetType, "user") {
		assigneeType = "User"
		userID = item.TargetID
	} else if strings.EqualFold(item.TargetType, "visitor") {
		assigneeType = "Guest"
	}

	return referenceCard{
		ID:              item.ID,
		ResourceType:    "Card",
		TenantID:        item.TenantID,
		Status:          referenceCardStatus(item.Status),
		Token:           item.ObjectID,
		UID:             item.ObjectID,
		CardNumber:      item.ID,
		Provider:        item.Provider,
		TemplateID:      item.TemplateID,
		UserID:          userID,
		AssigneeType:    assigneeType,
		AssigneeID:      item.TargetID,
		ActivationToken: item.ID,
		IssuedAt:        item.IssuedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		ExpiresAt:       item.ExpiresAt,
		CreatedAt:       item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func referenceCardAssignmentFromPass(item wallet.PassInstance) referenceCardAssignment {
	card := referenceCardFromPass(item)
	return referenceCardAssignment{
		ID:           "ca_" + item.ID,
		ResourceType: "CardAssignment",
		TenantID:     item.TenantID,
		Status:       referenceCardAssignmentStatus(item.Status),
		CardID:       item.ID,
		Card:         card,
		AssigneeType: card.AssigneeType,
		AssigneeID:   card.AssigneeID,
		UserID:       card.UserID,
		CreatedAt:    item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *server) referenceEventSetSnapshot(tenantID string, buildingScope map[string]struct{}, payload referenceEventSetPayload) referenceEventSet {
	accessEvents := s.eventSvc.ListAccessEvents(tenantID)
	deviceEvents := s.eventSvc.ListDeviceEvents(tenantID)
	if buildingScope != nil {
		accessEvents = filterAccessEventsByScope(accessEvents, buildingScope)
		deviceEvents = filterDeviceEventsByScope(deviceEvents, buildingScope)
	}

	events := make([]referenceEvent, 0, len(accessEvents)+len(deviceEvents))
	for i := range accessEvents {
		item := referenceEventFromAccessEvent(accessEvents[i])
		if !referenceEventMatchesPayload(item, payload) {
			continue
		}
		events = append(events, item)
	}
	for i := range deviceEvents {
		item := referenceEventFromDeviceEvent(deviceEvents[i])
		if !referenceEventMatchesPayload(item, payload) {
			continue
		}
		events = append(events, item)
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].CreatedAt > events[j].CreatedAt
	})

	now := ""
	if len(events) > 0 {
		now = events[0].CreatedAt
	} else {
		now = "1970-01-01T00:00:00Z"
	}
	placeID := firstNonEmptyString(payload.PlaceID, payload.EventPlaceID)
	return referenceEventSet{
		ID:              referenceEventSetID(tenantID, payload),
		CreatedAt:       now,
		Status:          "finished",
		Interval:        strings.TrimSpace(payload.Interval),
		EventPlaceID:    strings.TrimSpace(payload.EventPlaceID),
		PlaceID:         strings.TrimSpace(placeID),
		EventType:       strings.TrimSpace(payload.EventType),
		EventUUID:       strings.TrimSpace(payload.EventUUID),
		EventSuccess:    payload.EventSuccess,
		EventObjectID:   strings.TrimSpace(payload.EventObjectID),
		EventObjectType: strings.TrimSpace(payload.EventObjectType),
		Events:          events,
		Cursor:          "",
	}
}

func referenceEventFromAccessEvent(item event.AccessEvent) referenceEvent {
	return referenceEvent{
		UUID:       item.ID,
		Type:       item.Type,
		ActorType:  "User",
		ActorID:    item.Actor,
		ActorName:  item.Actor,
		ActorEmail: actorEmail(item.Actor),
		ObjectType: "Lock",
		ObjectID:   item.DoorID,
		ObjectName: item.DoorID,
		PlaceID:    item.BuildingID,
		LockID:     item.DoorID,
		Success:    item.Result == "success",
		Result:     item.Result,
		CreatedAt:  item.At.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func referenceEventFromDeviceEvent(item event.DeviceEvent) referenceEvent {
	return referenceEvent{
		UUID:       item.ID,
		Type:       item.Type,
		ActorType:  "Controller",
		ActorID:    item.GatewayID,
		ActorName:  item.GatewayID,
		ObjectType: "Controller",
		ObjectID:   item.GatewayID,
		ObjectName: item.GatewayID,
		PlaceID:    item.BuildingID,
		Success:    item.Result == "success",
		Result:     item.Result,
		Detail:     item.Detail,
		CreatedAt:  item.At.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func referenceEventMatchesPayload(item referenceEvent, payload referenceEventSetPayload) bool {
	placeID := firstNonEmptyString(payload.PlaceID, payload.EventPlaceID)
	if placeID != "" && item.PlaceID != placeID {
		return false
	}
	if payload.EventType != "" && !commaSetContains(payload.EventType, item.Type) {
		return false
	}
	if payload.EventUUID != "" && !commaSetContains(payload.EventUUID, item.UUID) {
		return false
	}
	if payload.EventSuccess != nil && item.Success != *payload.EventSuccess {
		return false
	}
	if payload.EventObjectID != "" && item.ObjectID != payload.EventObjectID {
		return false
	}
	if payload.EventObjectType != "" && !strings.EqualFold(item.ObjectType, payload.EventObjectType) {
		return false
	}
	return true
}

func referenceEventSetID(tenantID string, payload referenceEventSetPayload) string {
	placeID := firstNonEmptyString(payload.PlaceID, payload.EventPlaceID, "all")
	eventType := strings.TrimSpace(payload.EventType)
	if eventType == "" {
		eventType = "all"
	}
	return "event_set_" + strings.NewReplacer(" ", "_", ",", "_", "/", "_").Replace(tenantID+"_"+placeID+"_"+eventType)
}

func placeIDForEventSetScope(placeID string, scope map[string]struct{}) string {
	nextPlaceID := strings.TrimSpace(placeID)
	if nextPlaceID != "" || scope == nil {
		return nextPlaceID
	}
	if len(scope) == 1 {
		for id := range scope {
			return id
		}
	}
	return ""
}

func actorEmail(actor string) string {
	nextActor := strings.TrimSpace(actor)
	if strings.Contains(nextActor, "@") {
		return nextActor
	}
	if nextActor == "" || strings.Contains(nextActor, " ") {
		return ""
	}
	return nextActor + "@mistypass.local"
}

func referenceControllerFromGateway(item gateway.Gateway) referenceController {
	timestamp := item.LastSeenAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	return referenceController{
		ID:           item.ID,
		ResourceType: "Controller",
		TenantID:     item.TenantID,
		PlaceID:      item.BuildingID,
		Name:         item.SerialNumber,
		Description:  fmt.Sprintf("%d-device controller", item.DeviceCapacity),
		DeviceID:     item.SerialNumber,
		Token:        item.SerialNumber,
		Status:       item.Status,
		Configured:   len(item.BoundDoorIDs) > 0 || len(item.Devices) > 0,
		LockIDs:      append([]string(nil), item.BoundDoorIDs...),
		LastSeenAt:   timestamp,
		CreatedAt:    timestamp,
		UpdatedAt:    timestamp,
	}
}

func referenceReaderFromGatewayDevice(parent gateway.Gateway, item gateway.GatewayDevice) referenceReader {
	timestamp := item.LastSeenAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	return referenceReader{
		ID:           item.ID,
		ResourceType: "Reader",
		TenantID:     parent.TenantID,
		PlaceID:      parent.BuildingID,
		ControllerID: parent.ID,
		Name:         item.SerialNumber,
		Description:  titleizeReference(item.Kind),
		DeviceID:     item.SerialNumber,
		Token:        item.SerialNumber,
		Model:        readerModel(item),
		Protocol:     item.Protocol,
		Status:       item.Status,
		Configured:   item.GatewayID != "",
		LockIDs:      append([]string(nil), parent.BoundDoorIDs...),
		LastSeenAt:   timestamp,
		CreatedAt:    timestamp,
		UpdatedAt:    timestamp,
	}
}

func referenceTerminalFromGatewayDevice(parent gateway.Gateway, item gateway.GatewayDevice, placeName string) referenceTerminal {
	timestamp := item.LastSeenAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	terminalName := item.SerialNumber
	if strings.TrimSpace(terminalName) == "" {
		terminalName = item.ID
	}
	return referenceTerminal{
		ID:           "terminal_" + item.ID,
		ResourceType: "Terminal",
		TenantID:     parent.TenantID,
		CreatedAt:    timestamp,
		UpdatedAt:    timestamp,
		Name:         terminalName + " Terminal",
		Description:  titleizeReference(item.Kind) + " access terminal",
		PlaceID:      parent.BuildingID,
		Place: referenceTerminalPlace{
			ID:           parent.BuildingID,
			ResourceType: "Place",
			Name:         placeName,
		},
		MarketplaceInstallationID: nil,
		ControllerID:              parent.ID,
		ReaderID:                  item.ID,
		Status:                    item.Status,
		LastSeenAt:                timestamp,
	}
}

func referenceIntegrationFromIDPConfig(config enterprise.IDPConfig) referenceIntegration {
	createdAt := config.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	updatedAt := config.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	provider := strings.TrimSpace(config.Provider)
	if provider == "" {
		provider = "idp"
	}
	return referenceIntegration{
		ID:           "integration_" + config.ID,
		ResourceType: "Integration",
		TenantID:     config.TenantID,
		Type:         "identity_provider",
		Provider:     provider,
		Name:         strings.ToUpper(provider) + " Identity Provider",
		Description:  "Admin sign-in and just-in-time user sync",
		Status:       config.Status,
		Configured:   strings.TrimSpace(config.IssuerURL) != "" && strings.TrimSpace(config.ClientID) != "",
		SyncMode:     config.SyncMode,
		SourceID:     config.ID,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func referenceIntegrationFromHRISConnector(connector enterprise.HRISConnector) referenceIntegration {
	createdAt := connector.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	updatedAt := connector.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	lastSyncAt := ""
	if connector.LastSyncAt != nil {
		lastSyncAt = connector.LastSyncAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	vendor := strings.TrimSpace(connector.Vendor)
	if vendor == "" {
		vendor = "hris"
	}
	return referenceIntegration{
		ID:           "integration_" + connector.ID,
		ResourceType: "Integration",
		TenantID:     connector.TenantID,
		Type:         "hris",
		Provider:     vendor,
		Name:         strings.ToUpper(vendor) + " HRIS",
		Description:  "Directory and employee access sync",
		Status:       connector.Status,
		Configured:   strings.TrimSpace(connector.CredentialRef) != "" || strings.TrimSpace(connector.WebhookSecretRef) != "",
		SyncMode:     connector.SyncStrategy,
		SourceID:     connector.ID,
		LastSyncAt:   lastSyncAt,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func isReaderDevice(item gateway.GatewayDevice) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(item.Kind)), "reader")
}

func readerModel(item gateway.GatewayDevice) string {
	if strings.Contains(strings.ToLower(item.Kind), "legacy") {
		return "reader-1.0"
	}
	return "reader-pro-3.0"
}

func titleizeReference(value string) string {
	parts := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(value, "_", " "), "-", " "))
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, " ")
}

func referenceCardStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active":
		return "activated"
	case "issued":
		return "unassigned"
	default:
		return "deactivated"
	}
}

func referenceCardAssignmentStatus(status string) string {
	if strings.EqualFold(strings.TrimSpace(status), "active") {
		return "activated"
	}
	return "deactivated"
}

func shareMatchesQuery(share referenceShare, r *http.Request) bool {
	if roleID := strings.TrimSpace(r.URL.Query().Get("role_id")); roleID != "" && share.RoleID != roleID {
		return false
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" && share.PlaceID != placeID {
		return false
	}
	if lockID := strings.TrimSpace(r.URL.Query().Get("lock_id")); lockID != "" && share.LockID != lockID {
		return false
	}
	if email := strings.TrimSpace(r.URL.Query().Get("email")); email != "" && !strings.EqualFold(share.Email, email) {
		return false
	}
	return true
}

func cardMatchesQuery(card referenceCard, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[card.ID]; !exists {
			return false
		}
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(card.Status, status) {
		return false
	}
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" && card.Token != token {
		return false
	}
	if uid := strings.TrimSpace(r.URL.Query().Get("uid")); uid != "" && card.UID != uid {
		return false
	}
	if userID := strings.TrimSpace(r.URL.Query().Get("user_id")); userID != "" && userID != "me" && card.UserID != userID {
		return false
	}
	if cardNumber := strings.TrimSpace(r.URL.Query().Get("card_number")); cardNumber != "" && card.CardNumber != cardNumber {
		return false
	}
	if cardID := strings.TrimSpace(r.URL.Query().Get("card_id")); cardID != "" && card.ID != cardID {
		return false
	}
	return true
}

func cardAssignmentMatchesQuery(assignment referenceCardAssignment, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[assignment.ID]; !exists {
			return false
		}
	}
	if cardID := strings.TrimSpace(r.URL.Query().Get("card_id")); cardID != "" && assignment.CardID != cardID {
		return false
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(assignment.Status, status) {
		return false
	}
	if assigneeType := strings.TrimSpace(r.URL.Query().Get("assignee_type")); assigneeType != "" && !strings.EqualFold(assignment.AssigneeType, assigneeType) {
		return false
	}
	if assigneeID := strings.TrimSpace(r.URL.Query().Get("assignee_id")); assigneeID != "" && assignment.AssigneeID != assigneeID {
		return false
	}
	if userID := strings.TrimSpace(r.URL.Query().Get("user_id")); userID != "" && userID != "me" && assignment.UserID != userID {
		return false
	}
	return true
}

func controllerMatchesQuery(controller referenceController, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[controller.ID]; !exists {
			return false
		}
	}
	if query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query"))); query != "" {
		haystack := strings.ToLower(controller.Name + " " + controller.DeviceID + " " + controller.Token)
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" && controller.PlaceID != placeID {
		return false
	}
	if lockID := strings.TrimSpace(r.URL.Query().Get("lock_id")); lockID != "" && !containsString(controller.LockIDs, lockID) {
		return false
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(controller.Status, status) {
		return false
	}
	return true
}

func readerMatchesQuery(reader referenceReader, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[reader.ID]; !exists {
			return false
		}
	}
	if query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query"))); query != "" {
		haystack := strings.ToLower(reader.Name + " " + reader.DeviceID + " " + reader.Token)
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	if model := strings.TrimSpace(r.URL.Query().Get("model")); model != "" && !strings.EqualFold(reader.Model, model) {
		return false
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" && reader.PlaceID != placeID {
		return false
	}
	if lockID := strings.TrimSpace(r.URL.Query().Get("lock_id")); lockID != "" && !containsString(reader.LockIDs, lockID) {
		return false
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(reader.Status, status) {
		return false
	}
	return true
}

func terminalMatchesQuery(terminal referenceTerminal, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[terminal.ID]; !exists {
			return false
		}
	}
	if query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query"))); query != "" {
		haystack := strings.ToLower(terminal.Name + " " + terminal.Description + " " + terminal.Place.Name)
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	if placeID := strings.TrimSpace(r.URL.Query().Get("place_id")); placeID != "" && terminal.PlaceID != placeID {
		return false
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(terminal.Status, status) {
		return false
	}
	return true
}

func integrationMatchesQuery(integration referenceIntegration, r *http.Request) bool {
	if idFilter := commaSet(r.URL.Query().Get("ids")); len(idFilter) > 0 {
		if _, exists := idFilter[integration.ID]; !exists {
			return false
		}
	}
	if query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query"))); query != "" {
		haystack := strings.ToLower(integration.Name + " " + integration.Description + " " + integration.Provider)
		if !strings.Contains(haystack, query) {
			return false
		}
	}
	if integrationType := strings.TrimSpace(r.URL.Query().Get("type")); integrationType != "" && !strings.EqualFold(integration.Type, integrationType) {
		return false
	}
	if provider := strings.TrimSpace(r.URL.Query().Get("provider")); provider != "" && !strings.EqualFold(integration.Provider, provider) {
		return false
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && !strings.EqualFold(integration.Status, status) {
		return false
	}
	return true
}

func sortReferenceControllers(items []referenceController, r *http.Request) {
	sort.SliceStable(items, func(i, j int) bool {
		if strings.TrimSpace(r.URL.Query().Get("sort")) == "-name" {
			return items[i].Name > items[j].Name
		}
		return items[i].Name < items[j].Name
	})
}

func sortReferenceReaders(items []referenceReader, r *http.Request) {
	sort.SliceStable(items, func(i, j int) bool {
		if strings.TrimSpace(r.URL.Query().Get("sort")) == "-name" {
			return items[i].Name > items[j].Name
		}
		return items[i].Name < items[j].Name
	})
}

func sortReferenceTerminals(items []referenceTerminal, r *http.Request) {
	sort.SliceStable(items, func(i, j int) bool {
		if strings.TrimSpace(r.URL.Query().Get("sort")) == "-name" {
			return items[i].Name > items[j].Name
		}
		return items[i].Name < items[j].Name
	})
}

func sortReferenceIntegrations(items []referenceIntegration, r *http.Request) {
	sort.SliceStable(items, func(i, j int) bool {
		if strings.TrimSpace(r.URL.Query().Get("sort")) == "-name" {
			return items[i].Name > items[j].Name
		}
		if items[i].Type == items[j].Type {
			return items[i].Name < items[j].Name
		}
		return items[i].Type < items[j].Type
	})
}

func containsString(values []string, target string) bool {
	for i := range values {
		if values[i] == target {
			return true
		}
	}
	return false
}

func commaSetContains(values string, target string) bool {
	_, exists := commaSet(values)[strings.TrimSpace(target)]
	return exists
}

func commaSet(value string) map[string]struct{} {
	parts := strings.Split(value, ",")
	items := make(map[string]struct{}, len(parts))
	for i := range parts {
		next := strings.TrimSpace(parts[i])
		if next == "" {
			continue
		}
		items[next] = struct{}{}
	}
	return items
}

func filterBuildingsByID(items []space.Building, id string) []space.Building {
	filtered := make([]space.Building, 0, 1)
	for i := range items {
		if items[i].ID == id {
			filtered = append(filtered, items[i])
		}
	}
	return filtered
}

func filterDoorsByPlaceID(items []space.Door, placeID string) []space.Door {
	filtered := make([]space.Door, 0, len(items))
	for i := range items {
		if items[i].BuildingID == placeID {
			filtered = append(filtered, items[i])
		}
	}
	return filtered
}

func filterUserGroupsByPlaceID(items []access.UserGroup, placeID string) []access.UserGroup {
	filtered := make([]access.UserGroup, 0, len(items))
	for i := range items {
		if items[i].BuildingID == placeID {
			filtered = append(filtered, items[i])
		}
	}
	return filtered
}

func filterAccessUsersByPlaceID(items []access.AccessUser, placeID string) []access.AccessUser {
	filtered := make([]access.AccessUser, 0, len(items))
	for i := range items {
		if items[i].BuildingID == placeID {
			filtered = append(filtered, items[i])
		}
	}
	return filtered
}

func writeCollection(w http.ResponseWriter, status int, items any) {
	count := collectionCount(items)
	last := count - 1
	if last < 0 {
		last = 0
	}
	w.Header().Set("X-Collection-Range", fmt.Sprintf("items 0-%d/%d", last, count))
	writeJSON(w, status, map[string]any{"items": items})
}

func collectionCount(items any) int {
	switch typed := items.(type) {
	case []space.Building:
		return len(typed)
	case []space.Door:
		return len(typed)
	case []access.UserGroup:
		return len(typed)
	case []referenceGroupLock:
		return len(typed)
	case []referenceController:
		return len(typed)
	case []referenceReader:
		return len(typed)
	case []referenceTerminal:
		return len(typed)
	case []referenceIntegration:
		return len(typed)
	case []access.Team:
		return len(typed)
	case []access.TeamMembership:
		return len(typed)
	case []access.Role:
		return len(typed)
	case []access.RoleAssignment:
		return len(typed)
	case []access.AccessUser:
		return len(typed)
	case []referenceShare:
		return len(typed)
	case []referenceCard:
		return len(typed)
	case []referenceCardAssignment:
		return len(typed)
	default:
		return 0
	}
}

func handleAccessReferenceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, access.ErrTenantIDRequired),
		errors.Is(err, access.ErrRoleIDRequired),
		errors.Is(err, access.ErrInvalidRoleScope),
		errors.Is(err, access.ErrInvalidAssigneeType),
		errors.Is(err, access.ErrAppliesToIDRequired),
		errors.Is(err, access.ErrAssigneeIDRequired),
		errors.Is(err, access.ErrInvalidScopeType),
		errors.Is(err, access.ErrDeliveryMethodInvalid),
		errors.Is(err, access.ErrGranteeNameRequired),
		errors.Is(err, access.ErrGranteePhoneRequired),
		errors.Is(err, access.ErrGranteeEmailRequired),
		errors.Is(err, access.ErrValidUntilRequired):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, access.ErrRoleNotFound), errors.Is(err, access.ErrRoleAssignmentNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func handleSpaceMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, space.ErrTenantIDRequired),
		errors.Is(err, space.ErrBuildingIDRequired),
		errors.Is(err, space.ErrFloorIDRequired),
		errors.Is(err, space.ErrAreaIDRequired),
		errors.Is(err, space.ErrDoorNameRequired),
		errors.Is(err, space.ErrFloorBuildingMismatch),
		errors.Is(err, space.ErrAreaFloorMismatch),
		errors.Is(err, space.ErrInvalidDoorKind),
		errors.Is(err, space.ErrInvalidDoorStatus):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, space.ErrTenantOwnershipMismatch):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, space.ErrBuildingNotFound),
		errors.Is(err, space.ErrFloorNotFound),
		errors.Is(err, space.ErrAreaNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
