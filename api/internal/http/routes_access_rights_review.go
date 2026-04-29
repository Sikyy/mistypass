package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/access"
	"github.com/mistypass/cloud/api/internal/modules/space"
)

type referenceAccessRightsSelectionRequest struct {
	TenantID          string   `json:"tenant_id"`
	RoleAssignmentIDs []string `json:"role_assignment_ids"`
	ShareIDs          []string `json:"share_ids"`
	ReviewedBy        string   `json:"reviewed_by"`
}

type referenceAccessRightsScheduleUpdateRequest struct {
	TenantID          string   `json:"tenant_id"`
	RoleAssignmentIDs []string `json:"role_assignment_ids"`
	ShareIDs          []string `json:"share_ids"`
	ValidFrom         *string  `json:"valid_from"`
	ValidUntil        string   `json:"valid_until"`
}

type referenceAccessRightsScheduleUpdateResult struct {
	TenantID                 string   `json:"tenant_id"`
	ValidFrom                string   `json:"valid_from,omitempty"`
	ValidUntil               string   `json:"valid_until"`
	UpdatedCount             int      `json:"updated_count"`
	UpdatedRoleAssignmentIDs []string `json:"updated_role_assignment_ids,omitempty"`
	UpdatedShareIDs          []string `json:"updated_share_ids,omitempty"`
}

type referenceAccessRightImpactItem struct {
	SourceType       string `json:"source_type"`
	ID               string `json:"id"`
	Name             string `json:"name"`
	Target           string `json:"target"`
	Status           string `json:"status"`
	NeedsReview      bool   `json:"needs_review"`
	ReviewedAt       string `json:"reviewed_at,omitempty"`
	ReviewedBy       string `json:"reviewed_by,omitempty"`
	AffectedUsers    int    `json:"affected_users"`
	AffectedTeams    int    `json:"affected_teams"`
	AffectedGroups   int    `json:"affected_groups"`
	AffectedPlaces   int    `json:"affected_places"`
	AffectedLocks    int    `json:"affected_locks"`
	RoleID           string `json:"role_id,omitempty"`
	AppliesToType    string `json:"applies_to_type,omitempty"`
	AppliesToID      string `json:"applies_to_id,omitempty"`
	AssigneeType     string `json:"assignee_type,omitempty"`
	AssigneeID       string `json:"assignee_id,omitempty"`
	ProviderRecordID string `json:"provider_record_id,omitempty"`
}

type referenceAccessRightsScheduleTemplate struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	ValidFrom    string              `json:"valid_from,omitempty"`
	ValidUntil   string              `json:"valid_until,omitempty"`
	DurationDays int                 `json:"duration_days,omitempty"`
	TimeWindows  []access.TimeWindow `json:"time_windows,omitempty"`
	SourceTypes  []string            `json:"source_types,omitempty"`
}

type referenceAccessRightsImpactPreview struct {
	TenantID         string                           `json:"tenant_id"`
	SelectedCount    int                              `json:"selected_count"`
	NeedsReviewCount int                              `json:"needs_review_count"`
	AffectedUsers    int                              `json:"affected_users"`
	AffectedTeams    int                              `json:"affected_teams"`
	AffectedGroups   int                              `json:"affected_groups"`
	AffectedPlaces   int                              `json:"affected_places"`
	AffectedLocks    int                              `json:"affected_locks"`
	Items            []referenceAccessRightImpactItem `json:"items"`
}

func (s *server) listReferenceAccessRightsScheduleTemplates(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id")); !ok {
		return
	}
	if _, ok := s.buildingScopeForRequest(w, r); !ok {
		return
	}
	writeCollection(w, r, http.StatusOK, referenceAccessRightsScheduleTemplates(time.Now().UTC()))
}

func (s *server) previewReferenceAccessRightsImpact(w http.ResponseWriter, r *http.Request) {
	request, tenantID, buildingScope, ok := s.parseReferenceAccessRightsSelection(w, r)
	if !ok {
		return
	}
	items, ok := s.referenceAccessRightsImpactItems(w, tenantID, buildingScope, request.RoleAssignmentIDs, request.ShareIDs)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, summarizeReferenceAccessRightsImpact(tenantID, items))
}

func (s *server) reviewReferenceAccessRights(w http.ResponseWriter, r *http.Request) {
	request, tenantID, buildingScope, ok := s.parseReferenceAccessRightsSelection(w, r)
	if !ok {
		return
	}
	items, ok := s.referenceAccessRightsImpactItems(w, tenantID, buildingScope, request.RoleAssignmentIDs, request.ShareIDs)
	if !ok {
		return
	}
	reviewedBy := strings.TrimSpace(request.ReviewedBy)
	if reviewedBy == "" {
		if user, authenticated := authenticatedUser(r); authenticated {
			reviewedBy = strings.TrimSpace(user.Email)
		}
	}
	result, err := s.accessSvc.ReviewAccessRights(tenantID, request.RoleAssignmentIDs, request.ShareIDs, reviewedBy)
	if err != nil {
		handleAccessReferenceError(w, err)
		return
	}
	preview := summarizeReferenceAccessRightsImpact(tenantID, items)
	s.appendAuditLog(
		r,
		tenantID,
		"reference_access_rights_reviewed",
		fmt.Sprintf(
			"reviewed_count=%d,skipped_count=%d,selected_count=%d,needs_review_count=%d,role_assignment_ids=%s,share_ids=%s",
			result.ReviewedCount,
			result.SkippedCount,
			preview.SelectedCount,
			preview.NeedsReviewCount,
			strings.Join(result.ReviewedRoleAssignmentIDs, "|"),
			strings.Join(result.ReviewedShareIDs, "|"),
		),
		"access",
	)
	writeJSON(w, http.StatusOK, result)
}

func (s *server) updateReferenceAccessRightsSchedule(w http.ResponseWriter, r *http.Request) {
	var request referenceAccessRightsScheduleUpdateRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	request.RoleAssignmentIDs = uniqueStrings(request.RoleAssignmentIDs)
	request.ShareIDs = uniqueStrings(request.ShareIDs)
	if len(request.RoleAssignmentIDs) == 0 && len(request.ShareIDs) == 0 {
		writeError(w, http.StatusBadRequest, access.ErrAccessRightSelectionRequired.Error())
		return
	}
	validUntil := strings.TrimSpace(request.ValidUntil)
	if validUntil == "" {
		writeError(w, http.StatusBadRequest, access.ErrValidUntilRequired.Error())
		return
	}
	validFrom := ""
	if request.ValidFrom != nil {
		validFrom = strings.TrimSpace(*request.ValidFrom)
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return
	}
	if _, ok := s.referenceAccessRightsImpactItems(w, tenantID, buildingScope, request.RoleAssignmentIDs, request.ShareIDs); !ok {
		return
	}

	result := referenceAccessRightsScheduleUpdateResult{
		TenantID:   tenantID,
		ValidFrom:  validFrom,
		ValidUntil: validUntil,
	}
	for _, assignmentID := range request.RoleAssignmentIDs {
		assignment, err := s.accessSvc.GetRoleAssignment(tenantID, assignmentID)
		if err != nil {
			handleAccessReferenceError(w, err)
			return
		}
		nextValidFrom := assignment.ValidFrom
		if request.ValidFrom != nil {
			nextValidFrom = validFrom
		}
		updated, err := s.accessSvc.UpdateRoleAssignment(tenantID, assignmentID, access.RoleAssignmentInput{
			TenantID:      tenantID,
			RoleID:        assignment.RoleID,
			AppliesToType: assignment.AppliesToType,
			AppliesToID:   assignment.AppliesToID,
			AssigneeType:  assignment.AssigneeType,
			AssigneeID:    assignment.AssigneeID,
			AssigneeEmail: assignment.AssigneeEmail,
			ValidFrom:     nextValidFrom,
			ValidUntil:    validUntil,
		})
		if err != nil {
			handleAccessReferenceError(w, err)
			return
		}
		result.UpdatedRoleAssignmentIDs = append(result.UpdatedRoleAssignmentIDs, updated.ID)
		result.UpdatedCount++
	}
	for _, shareID := range request.ShareIDs {
		share, err := s.accessSvc.GetTemporaryAccess(tenantID, shareID)
		if err != nil {
			handleAccessReferenceError(w, err)
			return
		}
		nextValidFrom := share.ValidFrom
		if request.ValidFrom != nil {
			nextValidFrom = validFrom
		}
		updated, err := s.accessSvc.UpdateTemporaryAccess(tenantID, shareID, access.TemporaryAccessInput{
			TenantID:           tenantID,
			ScopeType:          share.ScopeType,
			BuildingID:         share.BuildingID,
			AreaID:             share.AreaID,
			DoorID:             share.DoorID,
			GroupID:            share.GroupID,
			RoleID:             share.RoleID,
			DeliveryMethod:     share.DeliveryMethod,
			GranteeName:        share.GranteeName,
			GranteeGender:      share.GranteeGender,
			GranteePhone:       share.GranteePhone,
			GranteeEmail:       share.GranteeEmail,
			MobileModel:        share.MobileModel,
			PassType:           share.PassType,
			ValidFrom:          nextValidFrom,
			ValidUntil:         validUntil,
			AuthorizedByID:     share.AuthorizedByID,
			AuthorizedByEmail:  share.AuthorizedByEmail,
			AuthorizedByRole:   share.AuthorizedByRole,
			KeepAuthorizedTime: true,
			ReviewedAt:         share.ReviewedAt,
			ReviewedBy:         share.ReviewedBy,
		})
		if err != nil {
			handleAccessReferenceError(w, err)
			return
		}
		result.UpdatedShareIDs = append(result.UpdatedShareIDs, updated.ID)
		result.UpdatedCount++
	}
	s.appendAuditLog(
		r,
		tenantID,
		"reference_access_rights_schedule_updated",
		fmt.Sprintf(
			"updated_count=%d,valid_from=%s,valid_until=%s,role_assignment_ids=%s,share_ids=%s",
			result.UpdatedCount,
			validFrom,
			validUntil,
			strings.Join(result.UpdatedRoleAssignmentIDs, "|"),
			strings.Join(result.UpdatedShareIDs, "|"),
		),
		"access",
	)
	writeJSON(w, http.StatusOK, result)
}

func referenceAccessRightsScheduleTemplates(now time.Time) []referenceAccessRightsScheduleTemplate {
	generatedAt := now.UTC().Truncate(time.Second)
	sourceTypes := []string{"role_assignment", "share"}
	return []referenceAccessRightsScheduleTemplate{
		{
			ID:           "immediate_7_days",
			Name:         "Immediate, 7 days",
			Description:  "Starts immediately and expires after 7 days.",
			ValidUntil:   generatedAt.Add(7 * 24 * time.Hour).Format(time.RFC3339),
			DurationDays: 7,
			SourceTypes:  sourceTypes,
		},
		{
			ID:           "immediate_30_days",
			Name:         "Immediate, 30 days",
			Description:  "Starts immediately and expires after 30 days.",
			ValidUntil:   generatedAt.Add(30 * 24 * time.Hour).Format(time.RFC3339),
			DurationDays: 30,
			SourceTypes:  sourceTypes,
		},
		{
			ID:           "starts_tomorrow_7_days",
			Name:         "Starts tomorrow, 7 days",
			Description:  "Schedules access for the next day and expires after 7 days.",
			ValidFrom:    generatedAt.Add(24 * time.Hour).Format(time.RFC3339),
			ValidUntil:   generatedAt.Add(8 * 24 * time.Hour).Format(time.RFC3339),
			DurationDays: 7,
			SourceTypes:  sourceTypes,
		},
		{
			ID:          "weekdays_business_hours",
			Name:        "Weekdays, business hours",
			Description: "Monday to Friday, 07:00–19:00.",
			TimeWindows: []access.TimeWindow{
				{StartTime: "07:00", EndTime: "19:00", DayOfWeekSet: "weekday"},
			},
			SourceTypes: sourceTypes,
		},
		{
			ID:          "weekdays_extended",
			Name:        "Weekdays, extended hours",
			Description: "Monday to Friday, 06:00–22:00.",
			TimeWindows: []access.TimeWindow{
				{StartTime: "06:00", EndTime: "22:00", DayOfWeekSet: "weekday"},
			},
			SourceTypes: sourceTypes,
		},
		{
			ID:          "everyday_24h",
			Name:        "Every day, 24 hours",
			Description: "All days, no time restriction.",
			TimeWindows: []access.TimeWindow{
				{StartTime: "00:00", EndTime: "23:59", DayOfWeekSet: "all"},
			},
			SourceTypes: sourceTypes,
		},
	}
}

func (s *server) parseReferenceAccessRightsSelection(
	w http.ResponseWriter,
	r *http.Request,
) (referenceAccessRightsSelectionRequest, string, map[string]struct{}, bool) {
	var request referenceAccessRightsSelectionRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return request, "", nil, false
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return request, "", nil, false
	}
	request.RoleAssignmentIDs = uniqueStrings(request.RoleAssignmentIDs)
	request.ShareIDs = uniqueStrings(request.ShareIDs)
	if len(request.RoleAssignmentIDs) == 0 && len(request.ShareIDs) == 0 {
		writeError(w, http.StatusBadRequest, access.ErrAccessRightSelectionRequired.Error())
		return request, "", nil, false
	}
	buildingScope, ok := s.buildingScopeForRequest(w, r)
	if !ok {
		return request, "", nil, false
	}
	return request, tenantID, buildingScope, true
}

func (s *server) referenceAccessRightsImpactItems(
	w http.ResponseWriter,
	tenantID string,
	buildingScope map[string]struct{},
	roleAssignmentIDs []string,
	shareIDs []string,
) ([]referenceAccessRightImpactItem, bool) {
	rolesByID := referenceRolesByID(s.accessSvc.ListRoles())
	usersByID := referenceUsersByID(s.accessSvc.ListUsers(tenantID))
	teamsByID := referenceTeamsByID(s.accessSvc.ListTeams(tenantID))
	groupsByID := referenceGroupsByID(s.accessSvc.ListUserGroups(tenantID))
	teamMembershipCountByTeam := referenceTeamMembershipCountByTeam(s.accessSvc.ListTeamMemberships(tenantID))
	places := s.spaceSvc.ListBuildings(tenantID)
	placesByID := referencePlacesByID(places)
	doorsByID := referenceDoorsByID(s.spaceSvc.ListDoors(tenantID))

	items := make([]referenceAccessRightImpactItem, 0, len(roleAssignmentIDs)+len(shareIDs))
	now := time.Now().UTC()
	for _, assignmentID := range uniqueStrings(roleAssignmentIDs) {
		assignment, err := s.accessSvc.GetRoleAssignment(tenantID, assignmentID)
		if err != nil {
			handleAccessReferenceError(w, err)
			return nil, false
		}
		if !roleAssignmentAllowedByBuildingScope(assignment, buildingScope) {
			writeError(w, http.StatusForbidden, "building scope forbidden")
			return nil, false
		}
		items = append(items, referenceRoleAssignmentImpactItem(
			assignment,
			rolesByID,
			usersByID,
			teamsByID,
			groupsByID,
			placesByID,
			len(places),
			teamMembershipCountByTeam,
			now,
		))
	}
	for _, shareID := range uniqueStrings(shareIDs) {
		share, err := s.accessSvc.GetTemporaryAccess(tenantID, shareID)
		if err != nil {
			handleAccessReferenceError(w, err)
			return nil, false
		}
		if !s.requireBuildingScope(w, buildingScope, share.BuildingID) {
			return nil, false
		}
		items = append(items, referenceShareImpactItem(share, rolesByID, groupsByID, placesByID, doorsByID, now))
	}
	return items, true
}

func referenceRoleAssignmentImpactItem(
	assignment access.RoleAssignment,
	rolesByID map[string]access.Role,
	usersByID map[string]access.AccessUser,
	teamsByID map[string]access.Team,
	groupsByID map[string]access.UserGroup,
	placesByID map[string]space.Building,
	organizationPlaceCount int,
	teamMembershipCountByTeam map[string]int,
	now time.Time,
) referenceAccessRightImpactItem {
	status := referenceAccessRightStatus(assignment.ValidFrom, assignment.ValidUntil, "", now)
	item := referenceAccessRightImpactItem{
		SourceType:       "role_assignment",
		ID:               assignment.ID,
		Name:             referenceRoleAssignmentAssigneeLabel(assignment, usersByID, teamsByID),
		Target:           referenceRoleAssignmentTargetLabel(assignment, groupsByID, placesByID),
		Status:           status,
		NeedsReview:      status != "enabled" && strings.TrimSpace(assignment.ReviewedAt) == "",
		ReviewedAt:       assignment.ReviewedAt,
		ReviewedBy:       assignment.ReviewedBy,
		RoleID:           assignment.RoleID,
		AppliesToType:    assignment.AppliesToType,
		AppliesToID:      assignment.AppliesToID,
		AssigneeType:     assignment.AssigneeType,
		AssigneeID:       assignment.AssigneeID,
		ProviderRecordID: assignment.ID,
	}
	_ = rolesByID
	switch assignment.AssigneeType {
	case "Team":
		item.AffectedTeams = 1
		item.AffectedUsers = teamMembershipCountByTeam[assignment.AssigneeID]
	default:
		item.AffectedUsers = 1
	}
	switch assignment.AppliesToType {
	case "Organization":
		item.AffectedPlaces = organizationPlaceCount
	case "Place":
		item.AffectedPlaces = 1
	case "Group":
		item.AffectedGroups = 1
		if group, exists := groupsByID[assignment.AppliesToID]; exists && strings.TrimSpace(group.PlaceID) != "" {
			item.AffectedPlaces = 1
		}
	}
	return item
}

func referenceShareImpactItem(
	share access.TemporaryAccess,
	rolesByID map[string]access.Role,
	groupsByID map[string]access.UserGroup,
	placesByID map[string]space.Building,
	doorsByID map[string]space.Door,
	now time.Time,
) referenceAccessRightImpactItem {
	status := referenceAccessRightStatus(share.ValidFrom, share.ValidUntil, "active", now)
	item := referenceAccessRightImpactItem{
		SourceType:       "share",
		ID:               share.ID,
		Name:             firstNonEmptyString(share.GranteeName, share.GranteeEmail, "Access link"),
		Target:           referenceShareTargetLabel(share, groupsByID, placesByID, doorsByID),
		Status:           status,
		NeedsReview:      status != "enabled" && strings.TrimSpace(share.ReviewedAt) == "",
		ReviewedAt:       share.ReviewedAt,
		ReviewedBy:       share.ReviewedBy,
		RoleID:           firstNonEmptyString(share.RoleID, "role_group_access"),
		ProviderRecordID: share.ID,
		AffectedUsers:    1,
	}
	_ = rolesByID
	if strings.TrimSpace(share.GroupID) != "" {
		item.AffectedGroups = 1
	}
	if strings.TrimSpace(share.BuildingID) != "" || strings.TrimSpace(share.DoorID) != "" {
		item.AffectedPlaces = 1
	} else if group, exists := groupsByID[share.GroupID]; exists && strings.TrimSpace(group.PlaceID) != "" {
		item.AffectedPlaces = 1
	}
	if strings.TrimSpace(share.DoorID) != "" {
		item.AffectedLocks = 1
	}
	return item
}

func summarizeReferenceAccessRightsImpact(tenantID string, items []referenceAccessRightImpactItem) referenceAccessRightsImpactPreview {
	preview := referenceAccessRightsImpactPreview{
		TenantID:       tenantID,
		SelectedCount:  len(items),
		Items:          items,
		AffectedUsers:  0,
		AffectedTeams:  0,
		AffectedGroups: 0,
		AffectedPlaces: 0,
		AffectedLocks:  0,
	}
	for i := range items {
		if items[i].NeedsReview {
			preview.NeedsReviewCount++
		}
		preview.AffectedUsers += items[i].AffectedUsers
		preview.AffectedTeams += items[i].AffectedTeams
		preview.AffectedGroups += items[i].AffectedGroups
		preview.AffectedPlaces += items[i].AffectedPlaces
		preview.AffectedLocks += items[i].AffectedLocks
	}
	return preview
}

func referenceAccessRightStatus(validFrom, validUntil, rawStatus string, now time.Time) string {
	normalizedStatus := strings.ToLower(strings.TrimSpace(rawStatus))
	switch normalizedStatus {
	case "expired", "revoked", "disabled", "inactive":
		return normalizedStatus
	case "pending", "scheduled":
		return normalizedStatus
	}
	if startsAt, ok := parseReferenceAccessRightTime(validFrom); ok && startsAt.After(now) {
		return "scheduled"
	}
	if expiresAt, ok := parseReferenceAccessRightTime(validUntil); ok && expiresAt.Before(now) {
		return "expired"
	}
	return "enabled"
}

func parseReferenceAccessRightTime(value string) (time.Time, bool) {
	nextValue := strings.TrimSpace(value)
	if nextValue == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, nextValue)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func referenceRoleAssignmentAssigneeLabel(
	assignment access.RoleAssignment,
	usersByID map[string]access.AccessUser,
	teamsByID map[string]access.Team,
) string {
	switch assignment.AssigneeType {
	case "User":
		if user, exists := usersByID[assignment.AssigneeID]; exists {
			return user.Name
		}
		return firstNonEmptyString(assignment.AssigneeEmail, assignment.AssigneeID)
	case "Team":
		if team, exists := teamsByID[assignment.AssigneeID]; exists {
			return team.Name
		}
		return firstNonEmptyString(assignment.AssigneeEmail, assignment.AssigneeID)
	default:
		return firstNonEmptyString(assignment.AssigneeEmail, assignment.AssigneeID)
	}
}

func referenceRoleAssignmentTargetLabel(
	assignment access.RoleAssignment,
	groupsByID map[string]access.UserGroup,
	placesByID map[string]space.Building,
) string {
	switch assignment.AppliesToType {
	case "Organization":
		return "Organization"
	case "Place":
		if place, exists := placesByID[assignment.AppliesToID]; exists {
			return place.Name
		}
	case "Group":
		if group, exists := groupsByID[assignment.AppliesToID]; exists {
			return group.Name
		}
	}
	return assignment.AppliesToID
}

func referenceShareTargetLabel(
	share access.TemporaryAccess,
	groupsByID map[string]access.UserGroup,
	placesByID map[string]space.Building,
	doorsByID map[string]space.Door,
) string {
	if strings.TrimSpace(share.GroupID) != "" {
		if group, exists := groupsByID[share.GroupID]; exists {
			return group.Name
		}
		return share.GroupID
	}
	if strings.TrimSpace(share.DoorID) != "" {
		if door, exists := doorsByID[share.DoorID]; exists {
			return door.Name
		}
		return share.DoorID
	}
	if strings.TrimSpace(share.BuildingID) != "" {
		if place, exists := placesByID[share.BuildingID]; exists {
			return place.Name
		}
		return share.BuildingID
	}
	return "Access link"
}

func referenceRolesByID(items []access.Role) map[string]access.Role {
	output := make(map[string]access.Role, len(items))
	for i := range items {
		output[items[i].ID] = items[i]
	}
	return output
}

func referenceUsersByID(items []access.AccessUser) map[string]access.AccessUser {
	output := make(map[string]access.AccessUser, len(items))
	for i := range items {
		output[items[i].ID] = items[i]
	}
	return output
}

func referenceTeamsByID(items []access.Team) map[string]access.Team {
	output := make(map[string]access.Team, len(items))
	for i := range items {
		output[items[i].ID] = items[i]
	}
	return output
}

func referenceGroupsByID(items []access.UserGroup) map[string]access.UserGroup {
	output := make(map[string]access.UserGroup, len(items))
	for i := range items {
		output[items[i].ID] = items[i]
	}
	return output
}

func referencePlacesByID(items []space.Building) map[string]space.Building {
	output := make(map[string]space.Building, len(items))
	for i := range items {
		output[items[i].ID] = items[i]
	}
	return output
}

func referenceDoorsByID(items []space.Door) map[string]space.Door {
	output := make(map[string]space.Door, len(items))
	for i := range items {
		output[items[i].ID] = items[i]
	}
	return output
}

func referenceTeamMembershipCountByTeam(items []access.TeamMembership) map[string]int {
	output := map[string]int{}
	for i := range items {
		output[items[i].TeamID]++
	}
	return output
}

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	output := make([]string, 0, len(items))
	for _, item := range items {
		nextItem := strings.TrimSpace(item)
		if nextItem == "" {
			continue
		}
		if _, exists := seen[nextItem]; exists {
			continue
		}
		seen[nextItem] = struct{}{}
		output = append(output, nextItem)
	}
	return output
}

// --- Schedule Evaluation ---

func (s *server) evaluateReferenceAccessRightsSchedule(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	var request struct {
		TenantID          string              `json:"tenant_id"`
		ValidFrom         string              `json:"valid_from"`
		ValidUntil        string              `json:"valid_until"`
		TimeWindows       []access.TimeWindow `json:"time_windows"`
		ExceptionDates    []string            `json:"exception_dates"`
		HolidayCalendarID string              `json:"holiday_calendar_id"`
		EvaluateAt        string              `json:"evaluate_at"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.TenantID != "" {
		tenantID = request.TenantID
	}

	now := time.Now().UTC()
	if request.EvaluateAt != "" {
		if parsed, err := time.Parse(time.RFC3339, request.EvaluateAt); err == nil {
			now = parsed
		}
	}

	var holidays []access.HolidayEntry
	calID := strings.TrimSpace(request.HolidayCalendarID)
	if calID != "" {
		if cal, err := s.accessSvc.GetHolidayCalendar(tenantID, calID); err == nil {
			holidays = cal.Entries
		}
	}

	eval := access.EvaluateSchedule(now, request.ValidFrom, request.ValidUntil, request.TimeWindows, request.ExceptionDates, holidays)
	writeJSON(w, http.StatusOK, eval)
}

// --- Holiday Calendar CRUD ---

func (s *server) listHolidayCalendars(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.accessSvc.ListHolidayCalendars(tenantID),
	})
}

func (s *server) createHolidayCalendar(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID string                `json:"tenant_id"`
		Name     string                `json:"name"`
		Country  string                `json:"country"`
		Entries  []access.HolidayEntry `json:"entries"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tenantID, ok := s.resolveTenantID(w, r, request.TenantID)
	if !ok {
		return
	}
	cal, err := s.accessSvc.CreateHolidayCalendar(tenantID, request.Name, request.Country, request.Entries)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrTenantIDRequired),
			errors.Is(err, access.ErrHolidayCalendarNameRequired):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	s.appendAuditLog(r, tenantID, "holiday_calendar_created", fmt.Sprintf("calendar_id=%s,name=%s,entries=%d", cal.ID, cal.Name, len(cal.Entries)), "access")
	writeJSON(w, http.StatusCreated, cal)
}

func (s *server) getHolidayCalendar(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	cal, err := s.accessSvc.GetHolidayCalendar(tenantID, chi.URLParam(r, "calendarID"))
	if err != nil {
		switch {
		case errors.Is(err, access.ErrHolidayCalendarNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, cal)
}

func (s *server) updateHolidayCalendar(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	var request struct {
		Name    string                `json:"name"`
		Country string                `json:"country"`
		Entries []access.HolidayEntry `json:"entries"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cal, err := s.accessSvc.UpdateHolidayCalendar(tenantID, chi.URLParam(r, "calendarID"), request.Name, request.Country, request.Entries)
	if err != nil {
		switch {
		case errors.Is(err, access.ErrHolidayCalendarNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	s.appendAuditLog(r, tenantID, "holiday_calendar_updated", fmt.Sprintf("calendar_id=%s,name=%s,entries=%d", cal.ID, cal.Name, len(cal.Entries)), "access")
	writeJSON(w, http.StatusOK, cal)
}

func (s *server) deleteHolidayCalendar(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	calendarID := chi.URLParam(r, "calendarID")
	if err := s.accessSvc.DeleteHolidayCalendar(tenantID, calendarID); err != nil {
		switch {
		case errors.Is(err, access.ErrHolidayCalendarNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	s.appendAuditLog(r, tenantID, "holiday_calendar_deleted", fmt.Sprintf("calendar_id=%s", calendarID), "access")
	w.WriteHeader(http.StatusNoContent)
}
