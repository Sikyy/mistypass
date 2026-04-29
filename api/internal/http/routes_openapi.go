package httpx

import (
	"net/http"
	"regexp"
	"sort"
	"strings"
)

type openAPIOperationDefinition struct {
	Method         string
	Path           string
	OperationID    string
	Tag            string
	Summary        string
	Collection     bool
	NoContent      bool
	Created        bool
	Public         bool
	ExtensionGroup string
	Conflict       bool
	Deprecated     bool
	Replacements   []string
}

func (s *server) getOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, buildOpenAPISpec())
}

func buildOpenAPISpec() map[string]any {
	definitions := openAPIOperationDefinitions()
	paths := make(map[string]any, len(definitions))
	for _, definition := range definitions {
		pathItem, _ := paths[definition.Path].(map[string]any)
		if pathItem == nil {
			pathItem = map[string]any{}
			paths[definition.Path] = pathItem
		}
		pathItem[strings.ToLower(definition.Method)] = buildOpenAPIOperation(definition)
	}

	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "Mistyislet API",
			"version":     "2026-04-29",
			"description": "Reference-style Mistyislet management API baseline with legacy compatibility routes separated from product extensions.",
		},
		"servers": []any{
			map[string]any{"url": "/"},
		},
		"tags":       openAPITags(definitions),
		"paths":      paths,
		"components": openAPIComponents(),
	}
}

func buildOpenAPIOperation(definition openAPIOperationDefinition) map[string]any {
	operation := map[string]any{
		"tags":        []string{definition.Tag},
		"operationId": definition.OperationID,
		"summary":     definition.Summary,
		"responses":   openAPIResponses(definition),
	}
	parameters := openAPIPathParameters(definition.Path)
	if definition.Collection {
		parameters = append(parameters,
			map[string]any{"$ref": "#/components/parameters/LimitParam"},
			map[string]any{"$ref": "#/components/parameters/OffsetParam"},
		)
	}
	if len(parameters) > 0 {
		operation["parameters"] = parameters
	}
	if !definition.Public {
		operation["security"] = []any{map[string]any{"bearerAuth": []string{}}}
	}
	if needsJSONRequestBody(definition.Method) {
		operation["requestBody"] = map[string]any{
			"required": true,
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{"type": "object", "additionalProperties": true},
				},
			},
		}
	}
	if definition.ExtensionGroup != "" {
		operation["x-mistyislet-extension"] = true
		operation["x-mistyislet-extension-group"] = definition.ExtensionGroup
	}
	if definition.Deprecated {
		operation["deprecated"] = true
		operation["x-mistyislet-legacy"] = true
		if len(definition.Replacements) > 0 {
			operation["x-mistyislet-replacements"] = definition.Replacements
		}
	}
	return operation
}

func needsJSONRequestBody(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func openAPIResponses(definition openAPIOperationDefinition) map[string]any {
	responses := map[string]any{}
	switch {
	case definition.NoContent:
		responses["204"] = map[string]any{"description": "No content"}
	case definition.Collection:
		responses["200"] = map[string]any{
			"description": "Collection response with pagination metadata.",
			"headers": map[string]any{
				"X-Collection-Range": map[string]any{
					"description": "Collection item range in the form `items start-end/total`.",
					"schema":      map[string]any{"type": "string"},
				},
			},
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{"$ref": "#/components/schemas/CollectionResponse"},
				},
			},
		}
	case definition.Created:
		responses["201"] = openAPIObjectResponse("Created")
	default:
		responses["200"] = openAPIObjectResponse("OK")
	}
	responses["401"] = map[string]any{"$ref": "#/components/responses/UnauthorizedError"}
	responses["403"] = map[string]any{"$ref": "#/components/responses/ForbiddenError"}
	responses["404"] = map[string]any{"$ref": "#/components/responses/NotFoundError"}
	if definition.Conflict {
		responses["409"] = map[string]any{"$ref": "#/components/responses/ConflictError"}
	}
	responses["422"] = map[string]any{"$ref": "#/components/responses/ValidationError"}
	return responses
}

func openAPIObjectResponse(description string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": "#/components/schemas/ObjectResponse"},
			},
		},
	}
}

var openAPIPathParameterPattern = regexp.MustCompile(`\{([^}]+)\}`)

func openAPIPathParameters(path string) []any {
	matches := openAPIPathParameterPattern.FindAllStringSubmatch(path, -1)
	if len(matches) == 0 {
		return nil
	}
	parameters := make([]any, 0, len(matches))
	seen := map[string]struct{}{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		parameters = append(parameters, map[string]any{
			"name":        name,
			"in":          "path",
			"required":    true,
			"description": "Resource identifier.",
			"schema":      map[string]any{"type": "string"},
		})
	}
	return parameters
}

func openAPITags(definitions []openAPIOperationDefinition) []any {
	seen := map[string]struct{}{}
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		if definition.Tag == "" {
			continue
		}
		if _, exists := seen[definition.Tag]; exists {
			continue
		}
		seen[definition.Tag] = struct{}{}
		names = append(names, definition.Tag)
	}
	sort.Strings(names)
	tags := make([]any, 0, len(names))
	for _, name := range names {
		tags = append(tags, map[string]any{
			"name":        name,
			"description": openAPITagDescription(name),
		})
	}
	return tags
}

func openAPITagDescription(name string) string {
	switch name {
	case "Access Extensions", "Credentials Extensions", "Hardware Extensions", "Wallet Operations":
		return "Mistyislet product extension endpoints."
	case "Legacy Compatibility":
		return "Deprecated migration endpoints with reference-style replacements."
	case "Mobile App":
		return "Resident mobile app endpoints."
	default:
		return "Reference-style resource endpoints."
	}
}

func openAPIComponents() map[string]any {
	errorSchema := map[string]any{
		"type":     "object",
		"required": []string{"error", "message", "code", "status"},
		"properties": map[string]any{
			"error":   map[string]any{"type": "string"},
			"message": map[string]any{"type": "string"},
			"code":    map[string]any{"type": "string"},
			"status":  map[string]any{"type": "string"},
		},
	}
	return map[string]any{
		"securitySchemes": map[string]any{
			"bearerAuth":       map[string]any{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"},
			"kisiLogin":        map[string]any{"type": "apiKey", "in": "header", "name": "Authorization", "description": "Compatibility scheme: `KISI-LOGIN <token>`."},
			"kisiAccessKey":    map[string]any{"type": "apiKey", "in": "header", "name": "Authorization", "description": "Compatibility scheme: `KISI-ACCESS-KEY <key>`."},
			"kisiGroupLink":    map[string]any{"type": "apiKey", "in": "header", "name": "Authorization", "description": "Compatibility scheme: `KISI-GROUP-LINK <token>`."},
			"kisiService":      map[string]any{"type": "apiKey", "in": "header", "name": "Authorization", "description": "Compatibility scheme: `KISI-SERVICE <token>`."},
			"webhookSignature": map[string]any{"type": "apiKey", "in": "header", "name": "X-Signature"},
		},
		"parameters": map[string]any{
			"LimitParam": map[string]any{
				"name":        "limit",
				"in":          "query",
				"required":    false,
				"description": "Maximum number of items to return.",
				"schema":      map[string]any{"type": "integer", "minimum": 0},
			},
			"OffsetParam": map[string]any{
				"name":        "offset",
				"in":          "query",
				"required":    false,
				"description": "Zero-based item offset.",
				"schema":      map[string]any{"type": "integer", "minimum": 0},
			},
		},
		"schemas": map[string]any{
			"Error": errorSchema,
			"Pagination": map[string]any{
				"type":     "object",
				"required": []string{"offset", "limit", "total", "has_more"},
				"properties": map[string]any{
					"offset":   map[string]any{"type": "integer", "minimum": 0},
					"limit":    map[string]any{"type": "integer", "minimum": 0},
					"total":    map[string]any{"type": "integer", "minimum": 0},
					"has_more": map[string]any{"type": "boolean"},
				},
			},
			"CollectionResponse": map[string]any{
				"type":     "object",
				"required": []string{"items", "pagination"},
				"properties": map[string]any{
					"items":      map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": true}},
					"pagination": map[string]any{"$ref": "#/components/schemas/Pagination"},
				},
			},
			"ObjectResponse": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
		"responses": map[string]any{
			"UnauthorizedError": errorResponseComponent("Unauthorized"),
			"ForbiddenError":    errorResponseComponent("Forbidden"),
			"NotFoundError":     errorResponseComponent("Not found"),
			"ConflictError":     errorResponseComponent("Conflict"),
			"ValidationError":   errorResponseComponent("Validation error"),
		},
	}
}

func errorResponseComponent(description string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": "#/components/schemas/Error"},
			},
		},
	}
}

func openAPIOperationDefinitions() []openAPIOperationDefinition {
	definitions := []openAPIOperationDefinition{
		{Method: http.MethodPost, Path: "/api/v1/auth/login", OperationID: "createLoginSession", Tag: "Authentication", Summary: "Create a login session.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/auth/external/login", OperationID: "createExternalLoginSession", Tag: "Authentication", Summary: "Create an external identity login session.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/auth/refresh", OperationID: "refreshLoginSession", Tag: "Authentication", Summary: "Refresh a login session.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/auth/logout", OperationID: "deleteLoginSession", Tag: "Authentication", Summary: "Log out the current user.", NoContent: true},
		{Method: http.MethodGet, Path: "/api/v1/me", OperationID: "fetchLegacyCurrentUser", Tag: "Authentication", Summary: "Fetch the legacy current user profile."},
		{Method: http.MethodGet, Path: "/api/v1/user", OperationID: "fetchCurrentUser", Tag: "Authentication", Summary: "Fetch the current user profile."},
		{Method: http.MethodPatch, Path: "/api/v1/user", OperationID: "updateCurrentUser", Tag: "Authentication", Summary: "Update the current user profile."},

		{Method: http.MethodPost, Path: "/api/v1/app/auth/login", OperationID: "createAppLoginSession", Tag: "Mobile App", Summary: "Create a resident app login session.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/app/auth/refresh", OperationID: "refreshAppLoginSession", Tag: "Mobile App", Summary: "Refresh a resident app login session.", Public: true},
		{Method: http.MethodGet, Path: "/api/v1/app/me", OperationID: "fetchAppCurrentUser", Tag: "Mobile App", Summary: "Fetch resident app profile."},
		{Method: http.MethodGet, Path: "/api/v1/app/credentials", OperationID: "fetchAppCredentials", Tag: "Mobile App", Summary: "Fetch resident app credentials.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/app/credentials/apple-pass", OperationID: "enrollAppApplePass", Tag: "Mobile App", Summary: "Enroll a resident Apple Wallet pass.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/app/access/doors", OperationID: "fetchAppAccessDoors", Tag: "Mobile App", Summary: "Fetch resident accessible doors.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/app/access/ble-token", OperationID: "fetchAppAccessBLEToken", Tag: "Mobile App", Summary: "Fetch resident BLE unlock token."},
		{Method: http.MethodGet, Path: "/api/v1/app/access/logs", OperationID: "fetchAppAccessLogs", Tag: "Mobile App", Summary: "Fetch resident access logs.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/app/visitor-passes", OperationID: "createAppVisitorPass", Tag: "Mobile App", Summary: "Create a resident visitor pass.", Created: true},

		{Method: http.MethodGet, Path: "/api/v1/places", OperationID: "fetchPlaces", Tag: "Places", Summary: "Fetch places.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/places", OperationID: "createPlace", Tag: "Places", Summary: "Create a place.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/places/{placeID}", OperationID: "fetchPlace", Tag: "Places", Summary: "Fetch a place."},
		{Method: http.MethodPatch, Path: "/api/v1/places/{placeID}", OperationID: "updatePlace", Tag: "Places", Summary: "Update a place."},
		{Method: http.MethodDelete, Path: "/api/v1/places/{placeID}", OperationID: "deletePlace", Tag: "Places", Summary: "Archive a place.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/places/{placeID}/lock_down", OperationID: "lockDownPlace", Tag: "Places", Summary: "Lock down a place."},
		{Method: http.MethodPost, Path: "/api/v1/places/{placeID}/cancel_lockdown", OperationID: "cancelPlaceLockdown", Tag: "Places", Summary: "Cancel place lockdown."},
		{Method: http.MethodGet, Path: "/api/v1/floors", OperationID: "fetchFloors", Tag: "Places", Summary: "Fetch floors.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/floors", OperationID: "createFloor", Tag: "Places", Summary: "Create a floor.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/floors/{floorID}", OperationID: "fetchFloor", Tag: "Places", Summary: "Fetch a floor."},
		{Method: http.MethodPatch, Path: "/api/v1/floors/{floorID}", OperationID: "updateFloor", Tag: "Places", Summary: "Update a floor."},
		{Method: http.MethodDelete, Path: "/api/v1/floors/{floorID}", OperationID: "deleteFloor", Tag: "Places", Summary: "Delete a floor.", NoContent: true},
		{Method: http.MethodGet, Path: "/api/v1/areas", OperationID: "fetchAreas", Tag: "Places", Summary: "Fetch areas.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/areas", OperationID: "createArea", Tag: "Places", Summary: "Create an area.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/areas/{areaID}", OperationID: "fetchArea", Tag: "Places", Summary: "Fetch an area."},
		{Method: http.MethodPatch, Path: "/api/v1/areas/{areaID}", OperationID: "updateArea", Tag: "Places", Summary: "Update an area."},

		{Method: http.MethodGet, Path: "/api/v1/locks", OperationID: "fetchLocks", Tag: "Locks", Summary: "Fetch locks.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/locks", OperationID: "createLock", Tag: "Locks", Summary: "Create a lock.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/locks/{lockID}", OperationID: "fetchLock", Tag: "Locks", Summary: "Fetch a lock."},
		{Method: http.MethodPatch, Path: "/api/v1/locks/{lockID}", OperationID: "updateLock", Tag: "Locks", Summary: "Update a lock."},
		{Method: http.MethodDelete, Path: "/api/v1/locks/{lockID}", OperationID: "deleteLock", Tag: "Locks", Summary: "Delete a lock.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/locks/{lockID}/unlock", OperationID: "unlockLock", Tag: "Locks", Summary: "Unlock a lock."},
		{Method: http.MethodPost, Path: "/api/v1/locks/{lockID}/lock_down", OperationID: "lockDownLock", Tag: "Locks", Summary: "Lock down a lock."},
		{Method: http.MethodPost, Path: "/api/v1/locks/{lockID}/cancel_lockdown", OperationID: "cancelLockLockdown", Tag: "Locks", Summary: "Cancel lock lockdown."},

		{Method: http.MethodGet, Path: "/api/v1/controllers", OperationID: "fetchControllers", Tag: "Hardware", Summary: "Fetch controllers.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/controllers/{controllerToken}/assign", OperationID: "assignController", Tag: "Hardware", Summary: "Assign a controller."},
		{Method: http.MethodPost, Path: "/api/v1/controllers/{controllerID}/deassign", OperationID: "deassignController", Tag: "Hardware", Summary: "Deassign a controller."},
		{Method: http.MethodPost, Path: "/api/v1/controllers/{controllerID}/locks", OperationID: "bindControllerLock", Tag: "Hardware", Summary: "Bind a controller to a lock.", Created: true},
		{Method: http.MethodDelete, Path: "/api/v1/controllers/{controllerID}/locks/{lockID}", OperationID: "unbindControllerLock", Tag: "Hardware", Summary: "Unbind a controller from a lock.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/controllers/{controllerID}/config/publish", OperationID: "publishControllerConfig", Tag: "Hardware", Summary: "Publish controller config."},
		{Method: http.MethodPost, Path: "/api/v1/controllers/{controllerID}/reboot", OperationID: "rebootController", Tag: "Hardware", Summary: "Reboot a controller."},
		{Method: http.MethodGet, Path: "/api/v1/readers", OperationID: "fetchReaders", Tag: "Hardware", Summary: "Fetch readers.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/readers/{readerToken}/assign", OperationID: "assignReader", Tag: "Hardware", Summary: "Assign a reader."},
		{Method: http.MethodPost, Path: "/api/v1/readers/{readerID}/deassign", OperationID: "deassignReader", Tag: "Hardware", Summary: "Deassign a reader."},
		{Method: http.MethodPost, Path: "/api/v1/readers/{readerID}/reboot", OperationID: "rebootReader", Tag: "Hardware", Summary: "Reboot a reader."},
		{Method: http.MethodGet, Path: "/api/v1/terminals", OperationID: "fetchTerminals", Tag: "Hardware", Summary: "Fetch terminals.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/terminals/{terminalID}", OperationID: "fetchTerminal", Tag: "Hardware", Summary: "Fetch a terminal."},
		{Method: http.MethodPost, Path: "/api/v1/terminals/{terminalID}/reboot", OperationID: "rebootTerminal", Tag: "Hardware", Summary: "Reboot a terminal."},
		{Method: http.MethodPost, Path: "/api/v1/terminals/{terminalID}/trigger", OperationID: "triggerTerminal", Tag: "Hardware", Summary: "Trigger a terminal command."},

		{Method: http.MethodGet, Path: "/api/v1/users", OperationID: "fetchUsers", Tag: "Users", Summary: "Fetch users.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/users", OperationID: "createUser", Tag: "Users", Summary: "Create a user.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/users/{userID}", OperationID: "fetchUser", Tag: "Users", Summary: "Fetch a user."},
		{Method: http.MethodPatch, Path: "/api/v1/users/{userID}", OperationID: "updateUser", Tag: "Users", Summary: "Update a user."},
		{Method: http.MethodDelete, Path: "/api/v1/users/{userID}", OperationID: "deleteUser", Tag: "Users", Summary: "Delete a user.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/users/{userID}/invite", OperationID: "inviteUser", Tag: "Users", Summary: "Invite a user."},
		{Method: http.MethodGet, Path: "/api/v1/users/{userID}/invitations", OperationID: "fetchUserInvitations", Tag: "Users", Summary: "Fetch user invitations.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/users/{userID}/invitations/{deliveryID}/receipt", OperationID: "recordUserInvitationReceipt", Tag: "Users", Summary: "Record a user invitation receipt."},
		{Method: http.MethodGet, Path: "/api/v1/members", OperationID: "fetchMembers", Tag: "Users", Summary: "Fetch members.", Collection: true},

		{Method: http.MethodGet, Path: "/api/v1/teams", OperationID: "fetchTeams", Tag: "Teams", Summary: "Fetch teams.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/teams", OperationID: "createTeam", Tag: "Teams", Summary: "Create a team.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/teams/{teamID}", OperationID: "fetchTeam", Tag: "Teams", Summary: "Fetch a team."},
		{Method: http.MethodPatch, Path: "/api/v1/teams/{teamID}", OperationID: "updateTeam", Tag: "Teams", Summary: "Update a team."},
		{Method: http.MethodDelete, Path: "/api/v1/teams/{teamID}", OperationID: "deleteTeam", Tag: "Teams", Summary: "Delete a team.", NoContent: true},
		{Method: http.MethodGet, Path: "/api/v1/team_memberships", OperationID: "fetchTeamMemberships", Tag: "Teams", Summary: "Fetch team memberships.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/team_memberships", OperationID: "createTeamMembership", Tag: "Teams", Summary: "Create a team membership.", Created: true},
		{Method: http.MethodDelete, Path: "/api/v1/team_memberships/{membershipID}", OperationID: "deleteTeamMembership", Tag: "Teams", Summary: "Delete a team membership.", NoContent: true},

		{Method: http.MethodGet, Path: "/api/v1/groups", OperationID: "fetchGroups", Tag: "Groups", Summary: "Fetch groups.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/groups", OperationID: "createGroup", Tag: "Groups", Summary: "Create a group.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/groups/{groupID}", OperationID: "fetchGroup", Tag: "Groups", Summary: "Fetch a group."},
		{Method: http.MethodPatch, Path: "/api/v1/groups/{groupID}", OperationID: "updateGroup", Tag: "Groups", Summary: "Update a group."},
		{Method: http.MethodDelete, Path: "/api/v1/groups/{groupID}", OperationID: "deleteGroup", Tag: "Groups", Summary: "Delete a group.", NoContent: true},
		{Method: http.MethodGet, Path: "/api/v1/group_locks", OperationID: "fetchGroupLocks", Tag: "Groups", Summary: "Fetch group locks.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/group_locks", OperationID: "createGroupLock", Tag: "Groups", Summary: "Create a group lock.", Created: true},
		{Method: http.MethodDelete, Path: "/api/v1/group_locks/{groupLockID}", OperationID: "deleteGroupLock", Tag: "Groups", Summary: "Delete a group lock.", NoContent: true},
		{Method: http.MethodGet, Path: "/api/v1/group_zones", OperationID: "fetchGroupZones", Tag: "Groups", Summary: "Fetch group zones.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/group_links", OperationID: "fetchGroupLinks", Tag: "Groups", Summary: "Fetch group links.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/group_links", OperationID: "createGroupLink", Tag: "Groups", Summary: "Create a group link.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/group_links/{groupLinkID}", OperationID: "fetchGroupLink", Tag: "Groups", Summary: "Fetch a group link."},
		{Method: http.MethodPatch, Path: "/api/v1/group_links/{groupLinkID}", OperationID: "updateGroupLink", Tag: "Groups", Summary: "Update a group link."},
		{Method: http.MethodDelete, Path: "/api/v1/group_links/{groupLinkID}", OperationID: "deleteGroupLink", Tag: "Groups", Summary: "Delete a group link.", NoContent: true},
		{Method: http.MethodGet, Path: "/api/v1/group_links/verify", OperationID: "verifyGroupLinkToken", Tag: "Groups", Summary: "Verify a group link token.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/group_links/verify", OperationID: "claimGroupLinkToken", Tag: "Groups", Summary: "Claim a group link token.", Public: true},

		{Method: http.MethodGet, Path: "/api/v1/roles", OperationID: "fetchRoles", Tag: "Roles", Summary: "Fetch roles.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/roles/{roleID}", OperationID: "fetchRole", Tag: "Roles", Summary: "Fetch a role."},
		{Method: http.MethodGet, Path: "/api/v1/role_assignments", OperationID: "fetchRoleAssignments", Tag: "Roles", Summary: "Fetch role assignments.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/role_assignments", OperationID: "createRoleAssignment", Tag: "Roles", Summary: "Create a role assignment.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/role_assignments/{assignmentID}", OperationID: "fetchRoleAssignment", Tag: "Roles", Summary: "Fetch a role assignment."},
		{Method: http.MethodPatch, Path: "/api/v1/role_assignments/{assignmentID}", OperationID: "updateRoleAssignment", Tag: "Roles", Summary: "Update a role assignment."},
		{Method: http.MethodDelete, Path: "/api/v1/role_assignments/{assignmentID}", OperationID: "deleteRoleAssignment", Tag: "Roles", Summary: "Delete a role assignment.", NoContent: true},
		{Method: http.MethodGet, Path: "/api/v1/shares", OperationID: "fetchShares", Tag: "Shares", Summary: "Fetch shares.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/shares", OperationID: "createShare", Tag: "Shares", Summary: "Create a share.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/shares/{shareID}", OperationID: "fetchShare", Tag: "Shares", Summary: "Fetch a share."},
		{Method: http.MethodPatch, Path: "/api/v1/shares/{shareID}", OperationID: "updateShare", Tag: "Shares", Summary: "Update a share."},
		{Method: http.MethodDelete, Path: "/api/v1/shares/{shareID}", OperationID: "deleteShare", Tag: "Shares", Summary: "Delete a share.", NoContent: true},

		{Method: http.MethodGet, Path: "/api/v1/cards", OperationID: "fetchCards", Tag: "Credentials", Summary: "Fetch cards.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/cards", OperationID: "createCard", Tag: "Credentials", Summary: "Create a card.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/cards/{cardID}", OperationID: "fetchCard", Tag: "Credentials", Summary: "Fetch a card."},
		{Method: http.MethodGet, Path: "/api/v1/card_assignments", OperationID: "fetchCardAssignments", Tag: "Credentials", Summary: "Fetch card assignments.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/card_assignments", OperationID: "createCardAssignment", Tag: "Credentials", Summary: "Create a card assignment.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/card_assignments/{assignmentID}", OperationID: "fetchCardAssignment", Tag: "Credentials", Summary: "Fetch a card assignment."},
		{Method: http.MethodPost, Path: "/api/v1/cards/{cardID}/assign", OperationID: "assignCard", Tag: "Credentials", Summary: "Assign a card."},
		{Method: http.MethodPost, Path: "/api/v1/cards/{cardID}/deassign", OperationID: "deassignCard", Tag: "Credentials", Summary: "Deassign a card."},
		{Method: http.MethodPost, Path: "/api/v1/cards/{cardID}/activate", OperationID: "activateCard", Tag: "Credentials", Summary: "Activate a card."},
		{Method: http.MethodPost, Path: "/api/v1/cards/{cardID}/deactivate", OperationID: "deactivateCard", Tag: "Credentials", Summary: "Deactivate a card."},
		{Method: http.MethodPost, Path: "/api/v1/cards/{cardID}/revoke", OperationID: "revokeCard", Tag: "Credentials", Summary: "Revoke a card."},

		{Method: http.MethodPost, Path: "/api/v1/event_sets", OperationID: "createEventSet", Tag: "Events", Summary: "Create an event set.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/event_sets/{eventSetID}", OperationID: "fetchEventSet", Tag: "Events", Summary: "Fetch an event set."},
		{Method: http.MethodGet, Path: "/api/v1/events/meta", OperationID: "fetchEventMetadata", Tag: "Events", Summary: "Fetch event metadata."},
		{Method: http.MethodGet, Path: "/api/v1/events/types", OperationID: "fetchEventTypes", Tag: "Events", Summary: "Fetch event types.", Collection: true},

		{Method: http.MethodGet, Path: "/api/v1/reports", OperationID: "fetchReports", Tag: "Reports", Summary: "Fetch reports.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/reports/{reportID}", OperationID: "fetchReport", Tag: "Reports", Summary: "Fetch a report."},
		{Method: http.MethodGet, Path: "/api/v1/reports/{reportID}/download", OperationID: "downloadReport", Tag: "Reports", Summary: "Download a report."},
		{Method: http.MethodGet, Path: "/api/v1/scheduled_reports", OperationID: "fetchScheduledReports", Tag: "Reports", Summary: "Fetch scheduled reports.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/scheduled_reports", OperationID: "createScheduledReport", Tag: "Reports", Summary: "Create a scheduled report.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/scheduled_reports/{scheduledReportID}", OperationID: "fetchScheduledReport", Tag: "Reports", Summary: "Fetch a scheduled report."},
		{Method: http.MethodPatch, Path: "/api/v1/scheduled_reports/{scheduledReportID}", OperationID: "updateScheduledReport", Tag: "Reports", Summary: "Update a scheduled report."},
		{Method: http.MethodDelete, Path: "/api/v1/scheduled_reports/{scheduledReportID}", OperationID: "deleteScheduledReport", Tag: "Reports", Summary: "Delete a scheduled report.", NoContent: true},

		{Method: http.MethodGet, Path: "/api/v1/integrations", OperationID: "fetchIntegrations", Tag: "Integrations", Summary: "Fetch integrations.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/integrations", OperationID: "createIntegration", Tag: "Integrations", Summary: "Create an integration.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/integrations/{integrationID}", OperationID: "fetchIntegration", Tag: "Integrations", Summary: "Fetch an integration."},
		{Method: http.MethodPatch, Path: "/api/v1/integrations/{integrationID}", OperationID: "updateIntegration", Tag: "Integrations", Summary: "Update an integration."},
		{Method: http.MethodDelete, Path: "/api/v1/integrations/{integrationID}", OperationID: "deleteIntegration", Tag: "Integrations", Summary: "Disable an integration.", NoContent: true},
		{Method: http.MethodGet, Path: "/api/v1/alert_policies", OperationID: "fetchAlertPolicies", Tag: "Alert Policies", Summary: "Fetch alert policies.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/alert_policies", OperationID: "createAlertPolicy", Tag: "Alert Policies", Summary: "Create an alert policy.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/alert_policies/{policyID}", OperationID: "fetchAlertPolicy", Tag: "Alert Policies", Summary: "Fetch an alert policy."},
		{Method: http.MethodPatch, Path: "/api/v1/alert_policies/{policyID}", OperationID: "updateAlertPolicy", Tag: "Alert Policies", Summary: "Update an alert policy."},
		{Method: http.MethodDelete, Path: "/api/v1/alert_policies/{policyID}", OperationID: "deleteAlertPolicy", Tag: "Alert Policies", Summary: "Disable an alert policy.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/alert_policies/condition_preview", OperationID: "previewAlertPolicyCondition", Tag: "Alert Policies", Summary: "Preview an alert policy condition."},
		{Method: http.MethodPost, Path: "/api/v1/alert_policies/evaluate", OperationID: "evaluateAlertPoliciesForEvent", Tag: "Alert Policies", Summary: "Evaluate alert policies for an event."},
	}

	definitions = append(definitions, openAPIExtensionOperationDefinitions()...)
	definitions = append(definitions, openAPILegacyOperationDefinitions()...)
	return definitions
}

func openAPIExtensionOperationDefinitions() []openAPIOperationDefinition {
	return []openAPIOperationDefinition{
		{Method: http.MethodGet, Path: "/api/v1/door_groups", OperationID: "fetchDoorGroups", Tag: "Access Extensions", Summary: "Fetch door groups.", Collection: true, ExtensionGroup: "access"},
		{Method: http.MethodGet, Path: "/api/v1/access_rights/schedule_templates", OperationID: "fetchAccessRightScheduleTemplates", Tag: "Access Extensions", Summary: "Fetch access right schedule templates.", Collection: true, ExtensionGroup: "access_rights"},
		{Method: http.MethodPatch, Path: "/api/v1/access_rights/schedule", OperationID: "updateAccessRightSchedule", Tag: "Access Extensions", Summary: "Bulk update access right schedules.", ExtensionGroup: "access_rights"},
		{Method: http.MethodPost, Path: "/api/v1/access_rights/impact_preview", OperationID: "previewAccessRightImpact", Tag: "Access Extensions", Summary: "Preview access right impact.", ExtensionGroup: "access_rights"},
		{Method: http.MethodPost, Path: "/api/v1/access_rights/review", OperationID: "reviewAccessRights", Tag: "Access Extensions", Summary: "Review access rights.", ExtensionGroup: "access_rights"},

		{Method: http.MethodGet, Path: "/api/v1/gateways/serial-inventory", OperationID: "fetchGatewaySerialInventory", Tag: "Hardware Extensions", Summary: "Fetch gateway serial inventory.", Collection: true, ExtensionGroup: "hardware_inventory"},
		{Method: http.MethodPost, Path: "/api/v1/gateways/serial-inventory/import", OperationID: "importGatewaySerialInventory", Tag: "Hardware Extensions", Summary: "Import gateway serial inventory.", ExtensionGroup: "hardware_inventory"},
		{Method: http.MethodPost, Path: "/api/v1/gateways/serial-inventory/import-csv", OperationID: "importGatewaySerialInventoryCSV", Tag: "Hardware Extensions", Summary: "Import gateway serial inventory from CSV.", ExtensionGroup: "hardware_inventory"},
		{Method: http.MethodPatch, Path: "/api/v1/gateways/serial-inventory/batch-status", OperationID: "batchUpdateGatewaySerialInventoryStatus", Tag: "Hardware Extensions", Summary: "Batch update gateway serial inventory status.", ExtensionGroup: "hardware_inventory"},
		{Method: http.MethodPatch, Path: "/api/v1/gateways/serial-inventory/{serialNumber}/status", OperationID: "updateGatewaySerialInventoryStatus", Tag: "Hardware Extensions", Summary: "Update gateway serial inventory status.", ExtensionGroup: "hardware_inventory"},
		{Method: http.MethodGet, Path: "/api/v1/gateways/serial-inventory/export-csv", OperationID: "exportGatewaySerialInventoryCSV", Tag: "Hardware Extensions", Summary: "Export gateway serial inventory CSV.", ExtensionGroup: "hardware_inventory"},
		{Method: http.MethodGet, Path: "/api/v1/gateways/events/checkpoint/summary", OperationID: "fetchGatewayEventCheckpointSummary", Tag: "Hardware Extensions", Summary: "Fetch gateway event checkpoint summary.", Collection: true, ExtensionGroup: "gateway_events"},
		{Method: http.MethodPost, Path: "/api/v1/gateways/register", OperationID: "registerGateway", Tag: "Hardware Extensions", Summary: "Register a legacy gateway.", ExtensionGroup: "legacy_gateway"},
		{Method: http.MethodPost, Path: "/api/v1/gateways/{gatewayID}/bind-door", OperationID: "bindGatewayDoor", Tag: "Hardware Extensions", Summary: "Bind a legacy gateway door.", ExtensionGroup: "legacy_gateway"},
		{Method: http.MethodPost, Path: "/api/v1/gateways/{gatewayID}/unbind-door", OperationID: "unbindGatewayDoor", Tag: "Hardware Extensions", Summary: "Unbind a legacy gateway door.", ExtensionGroup: "legacy_gateway"},
		{Method: http.MethodPost, Path: "/api/v1/gateways/{gatewayID}/devices", OperationID: "registerGatewayDevice", Tag: "Hardware Extensions", Summary: "Register a gateway device.", ExtensionGroup: "legacy_gateway"},
		{Method: http.MethodPost, Path: "/api/v1/gateways/{gatewayID}/devices/{deviceID}/rs485/telemetry", OperationID: "reportGatewayDeviceRS485Telemetry", Tag: "Hardware Extensions", Summary: "Report gateway device RS485 telemetry.", ExtensionGroup: "legacy_gateway"},
		{Method: http.MethodPost, Path: "/api/v1/gateways/{gatewayID}/devices/probe-legacy", OperationID: "probeGatewayLegacyDevices", Tag: "Hardware Extensions", Summary: "Probe legacy gateway devices.", ExtensionGroup: "legacy_gateway"},
		{Method: http.MethodPost, Path: "/api/v1/gateways/{gatewayID}/config/publish", OperationID: "publishGatewayConfig", Tag: "Hardware Extensions", Summary: "Publish legacy gateway config.", ExtensionGroup: "legacy_gateway"},
		{Method: http.MethodPost, Path: "/api/v1/gateways/{gatewayID}/reboot", OperationID: "rebootGateway", Tag: "Hardware Extensions", Summary: "Reboot a legacy gateway.", ExtensionGroup: "legacy_gateway"},
		{Method: http.MethodGet, Path: "/api/v1/gateways/{gatewayID}/mqtt/bootstrap", OperationID: "fetchGatewayMQTTBootstrap", Tag: "Hardware Extensions", Summary: "Fetch gateway MQTT bootstrap.", ExtensionGroup: "legacy_gateway"},
		{Method: http.MethodPost, Path: "/api/v1/gateways/{gatewayID}/ota/tasks", OperationID: "createGatewayOTATask", Tag: "Hardware Extensions", Summary: "Create gateway OTA task.", Created: true, ExtensionGroup: "legacy_gateway"},
		{Method: http.MethodGet, Path: "/api/v1/gateways/{gatewayID}/ota/tasks", OperationID: "fetchGatewayOTATasks", Tag: "Hardware Extensions", Summary: "Fetch gateway OTA tasks.", Collection: true, ExtensionGroup: "legacy_gateway"},
		{Method: http.MethodPatch, Path: "/api/v1/gateways/{gatewayID}/ota/tasks/{taskID}/status", OperationID: "updateGatewayOTATaskStatus", Tag: "Hardware Extensions", Summary: "Update gateway OTA task status.", ExtensionGroup: "legacy_gateway"},
		{Method: http.MethodGet, Path: "/api/v1/gateways/{gatewayID}/events/checkpoint", OperationID: "fetchGatewayEventCheckpoints", Tag: "Hardware Extensions", Summary: "Fetch gateway event checkpoints.", Collection: true, ExtensionGroup: "gateway_events"},

		{Method: http.MethodGet, Path: "/api/v1/wallet/google/config", OperationID: "fetchWalletGoogleConfig", Tag: "Credentials Extensions", Summary: "Fetch Google Wallet config.", ExtensionGroup: "google_wallet"},
		{Method: http.MethodPut, Path: "/api/v1/wallet/google/config", OperationID: "updateWalletGoogleConfig", Tag: "Credentials Extensions", Summary: "Update Google Wallet config.", ExtensionGroup: "google_wallet"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/google/config/validate", OperationID: "validateWalletGoogleConfig", Tag: "Credentials Extensions", Summary: "Validate Google Wallet config.", ExtensionGroup: "google_wallet"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/templates", OperationID: "fetchWalletTemplates", Tag: "Credentials Extensions", Summary: "Fetch wallet templates.", Collection: true, ExtensionGroup: "wallet_templates"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/templates", OperationID: "createWalletTemplate", Tag: "Credentials Extensions", Summary: "Create a wallet template.", Created: true, ExtensionGroup: "wallet_templates"},
		{Method: http.MethodPatch, Path: "/api/v1/wallet/templates/{templateID}/status", OperationID: "updateWalletTemplateStatus", Tag: "Credentials Extensions", Summary: "Update wallet template status.", ExtensionGroup: "wallet_templates"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/passes/issue", OperationID: "issueWalletPass", Tag: "Credentials Extensions", Summary: "Issue a wallet pass.", Created: true, ExtensionGroup: "wallet_passes"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/passes/issue-batch", OperationID: "issueWalletPassBatch", Tag: "Credentials Extensions", Summary: "Issue wallet passes in batch.", Created: true, ExtensionGroup: "wallet_passes"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/passes/{passID}", OperationID: "fetchWalletPass", Tag: "Credentials Extensions", Summary: "Fetch a wallet pass.", ExtensionGroup: "wallet_passes"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/passes/{passID}/save-link", OperationID: "fetchWalletPassSaveLink", Tag: "Credentials Extensions", Summary: "Fetch a wallet pass save link.", ExtensionGroup: "wallet_passes"},
		{Method: http.MethodPatch, Path: "/api/v1/wallet/passes/{passID}/suspend", OperationID: "suspendWalletPass", Tag: "Credentials Extensions", Summary: "Suspend a wallet pass.", ExtensionGroup: "wallet_passes"},
		{Method: http.MethodPatch, Path: "/api/v1/wallet/passes/{passID}/activate", OperationID: "activateWalletPass", Tag: "Credentials Extensions", Summary: "Activate a wallet pass.", ExtensionGroup: "wallet_passes"},
		{Method: http.MethodPatch, Path: "/api/v1/wallet/passes/{passID}/revoke", OperationID: "revokeWalletPass", Tag: "Credentials Extensions", Summary: "Revoke a wallet pass.", ExtensionGroup: "wallet_passes"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/deliveries", OperationID: "fetchWalletPassDeliveries", Tag: "Credentials Extensions", Summary: "Fetch wallet pass deliveries.", Collection: true, ExtensionGroup: "wallet_deliveries"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/deliveries/dispatch", OperationID: "dispatchWalletPassDelivery", Tag: "Credentials Extensions", Summary: "Dispatch a wallet pass delivery.", ExtensionGroup: "wallet_deliveries"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/deliveries/{notificationID}/retry", OperationID: "retryWalletPassDelivery", Tag: "Credentials Extensions", Summary: "Retry a wallet pass delivery.", ExtensionGroup: "wallet_deliveries"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/physical-card-vendors", OperationID: "fetchWalletPhysicalCardVendors", Tag: "Credentials Extensions", Summary: "Fetch physical card vendors.", Collection: true, ExtensionGroup: "physical_cards"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/physical-card-inventory", OperationID: "fetchWalletPhysicalCardInventory", Tag: "Credentials Extensions", Summary: "Fetch physical card inventory.", Collection: true, ExtensionGroup: "physical_cards"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/physical-card-inventory", OperationID: "createWalletPhysicalCardInventoryItem", Tag: "Credentials Extensions", Summary: "Create a physical card inventory item.", Created: true, ExtensionGroup: "physical_cards"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/physical-card-inventory/scan", OperationID: "scanWalletPhysicalCardInventoryItem", Tag: "Credentials Extensions", Summary: "Scan a physical card inventory item.", ExtensionGroup: "physical_cards"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/physical-card-inventory/import", OperationID: "importWalletPhysicalCardInventory", Tag: "Credentials Extensions", Summary: "Import physical card inventory.", ExtensionGroup: "physical_cards"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/physical-card-inventory/import-csv", OperationID: "importWalletPhysicalCardInventoryCSV", Tag: "Credentials Extensions", Summary: "Import physical card inventory from CSV.", ExtensionGroup: "physical_cards"},
		{Method: http.MethodPatch, Path: "/api/v1/wallet/physical-card-inventory/batch-status", OperationID: "batchUpdateWalletPhysicalCardInventoryStatus", Tag: "Credentials Extensions", Summary: "Batch update physical card inventory status.", ExtensionGroup: "physical_cards", Conflict: true},
		{Method: http.MethodPatch, Path: "/api/v1/wallet/physical-card-inventory/{inventoryID}/status", OperationID: "updateWalletPhysicalCardInventoryStatus", Tag: "Credentials Extensions", Summary: "Update physical card inventory status.", ExtensionGroup: "physical_cards", Conflict: true},
		{Method: http.MethodGet, Path: "/api/v1/wallet/physical-card-tasks", OperationID: "fetchWalletPhysicalCardTasks", Tag: "Credentials Extensions", Summary: "Fetch physical card tasks.", Collection: true, ExtensionGroup: "physical_cards"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/physical-card-tasks", OperationID: "createWalletPhysicalCardTask", Tag: "Credentials Extensions", Summary: "Create a physical card task.", Created: true, ExtensionGroup: "physical_cards"},
		{Method: http.MethodPatch, Path: "/api/v1/wallet/physical-card-tasks/{taskID}/status", OperationID: "updateWalletPhysicalCardTaskStatus", Tag: "Credentials Extensions", Summary: "Update physical card task status.", ExtensionGroup: "physical_cards"},

		{Method: http.MethodGet, Path: "/api/v1/wallet/jobs", OperationID: "fetchWalletJobs", Tag: "Wallet Operations", Summary: "Fetch wallet jobs.", Collection: true, ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/jobs/{jobID}", OperationID: "fetchWalletJob", Tag: "Wallet Operations", Summary: "Fetch a wallet job.", ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/jobs/{jobID}/retry", OperationID: "retryWalletJob", Tag: "Wallet Operations", Summary: "Retry a wallet job.", ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/jobs/{jobID}/dlq/requeue", OperationID: "requeueWalletDLQJob", Tag: "Wallet Operations", Summary: "Requeue a wallet DLQ job.", ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/jobs/process", OperationID: "processWalletJobs", Tag: "Wallet Operations", Summary: "Process wallet jobs.", ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/jobs/summary", OperationID: "fetchWalletJobSummary", Tag: "Wallet Operations", Summary: "Fetch wallet job summary.", ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/jobs/metrics/trend", OperationID: "fetchWalletJobMetricsTrend", Tag: "Wallet Operations", Summary: "Fetch wallet job metrics trend.", Collection: true, ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/jobs/metrics", OperationID: "fetchWalletJobMetrics", Tag: "Wallet Operations", Summary: "Fetch wallet job metrics.", ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/jobs/alert-subscription", OperationID: "fetchWalletJobAlertSubscription", Tag: "Wallet Operations", Summary: "Fetch wallet job alert subscription.", ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodPut, Path: "/api/v1/wallet/jobs/alert-subscription", OperationID: "updateWalletJobAlertSubscription", Tag: "Wallet Operations", Summary: "Update wallet job alert subscription.", ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/jobs/alert-notifications", OperationID: "fetchWalletJobAlertNotifications", Tag: "Wallet Operations", Summary: "Fetch wallet job alert notifications.", Collection: true, ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/jobs/alert-notifications/{notificationID}/retry", OperationID: "retryWalletJobAlertNotification", Tag: "Wallet Operations", Summary: "Retry wallet job alert notification.", ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/jobs/alerts/dispatch", OperationID: "dispatchWalletJobAlerts", Tag: "Wallet Operations", Summary: "Dispatch wallet job alerts.", ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/jobs/dlq/cleanup/archives", OperationID: "fetchWalletDLQCleanupArchives", Tag: "Wallet Operations", Summary: "Fetch wallet DLQ cleanup archives.", Collection: true, ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/jobs/dlq/requeue", OperationID: "requeueWalletDLQJobs", Tag: "Wallet Operations", Summary: "Requeue wallet DLQ jobs.", ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodPost, Path: "/api/v1/wallet/jobs/dlq/cleanup", OperationID: "cleanupWalletDLQJobs", Tag: "Wallet Operations", Summary: "Clean up wallet DLQ jobs.", ExtensionGroup: "wallet_jobs"},
		{Method: http.MethodGet, Path: "/api/v1/wallet/audit-logs", OperationID: "fetchWalletAuditLogs", Tag: "Wallet Operations", Summary: "Fetch wallet audit logs.", Collection: true, ExtensionGroup: "wallet_jobs"},
	}
}

func openAPILegacyOperationDefinitions() []openAPIOperationDefinition {
	return []openAPIOperationDefinition{
		{Method: http.MethodGet, Path: "/api/v1/buildings", OperationID: "fetchLegacyBuildings", Tag: "Legacy Compatibility", Summary: "Fetch legacy buildings.", Collection: true, Deprecated: true, Replacements: []string{"/api/v1/places"}},
		{Method: http.MethodPost, Path: "/api/v1/buildings", OperationID: "createLegacyBuilding", Tag: "Legacy Compatibility", Summary: "Create a legacy building.", Created: true, Deprecated: true, Replacements: []string{"/api/v1/places"}},
		{Method: http.MethodGet, Path: "/api/v1/doors", OperationID: "fetchLegacyDoors", Tag: "Legacy Compatibility", Summary: "Fetch legacy doors.", Collection: true, Deprecated: true, Replacements: []string{"/api/v1/locks"}},
		{Method: http.MethodPost, Path: "/api/v1/doors", OperationID: "createLegacyDoor", Tag: "Legacy Compatibility", Summary: "Create a legacy door.", Created: true, Deprecated: true, Replacements: []string{"/api/v1/locks"}},
		{Method: http.MethodGet, Path: "/api/v1/door-groups", OperationID: "fetchLegacyDoorGroups", Tag: "Legacy Compatibility", Summary: "Fetch legacy door groups.", Collection: true, Deprecated: true, Replacements: []string{"/api/v1/door_groups"}},
		{Method: http.MethodPost, Path: "/api/v1/door-groups", OperationID: "createLegacyDoorGroup", Tag: "Legacy Compatibility", Summary: "Create a legacy door group.", Created: true, Deprecated: true, Replacements: []string{"/api/v1/door_groups"}},
		{Method: http.MethodGet, Path: "/api/v1/gateways", OperationID: "fetchLegacyGateways", Tag: "Legacy Compatibility", Summary: "Fetch legacy gateways.", Collection: true, Deprecated: true, Replacements: []string{"/api/v1/controllers", "/api/v1/readers", "/api/v1/terminals"}},
		{Method: http.MethodGet, Path: "/api/v1/events/access", OperationID: "fetchLegacyAccessEvents", Tag: "Legacy Compatibility", Summary: "Fetch legacy access events.", Collection: true, Deprecated: true, Replacements: []string{"/api/v1/event_sets"}},
		{Method: http.MethodGet, Path: "/api/v1/wallet/passes", OperationID: "fetchLegacyWalletPasses", Tag: "Legacy Compatibility", Summary: "Fetch legacy wallet passes.", Collection: true, Deprecated: true, Replacements: []string{"/api/v1/cards", "/api/v1/card_assignments"}},
		{Method: http.MethodGet, Path: "/api/v1/access-policies", OperationID: "fetchLegacyAccessPolicies", Tag: "Legacy Compatibility", Summary: "Fetch legacy access policies.", Collection: true, Deprecated: true, Replacements: []string{"/api/v1/role_assignments", "/api/v1/groups", "/api/v1/group_locks"}},
		{Method: http.MethodPost, Path: "/api/v1/access-policies", OperationID: "createLegacyAccessPolicy", Tag: "Legacy Compatibility", Summary: "Create a legacy access policy.", Created: true, Deprecated: true, Replacements: []string{"/api/v1/role_assignments", "/api/v1/groups", "/api/v1/group_locks"}},
		{Method: http.MethodPatch, Path: "/api/v1/access-policies/{policyID}", OperationID: "updateLegacyAccessPolicy", Tag: "Legacy Compatibility", Summary: "Update a legacy access policy.", Deprecated: true, Replacements: []string{"/api/v1/role_assignments", "/api/v1/groups", "/api/v1/group_locks"}},
		{Method: http.MethodGet, Path: "/api/v1/temporary-access", OperationID: "fetchLegacyTemporaryAccess", Tag: "Legacy Compatibility", Summary: "Fetch legacy temporary access.", Collection: true, Deprecated: true, Replacements: []string{"/api/v1/shares"}},
		{Method: http.MethodPost, Path: "/api/v1/temporary-access", OperationID: "createLegacyTemporaryAccess", Tag: "Legacy Compatibility", Summary: "Create legacy temporary access.", Created: true, Deprecated: true, Replacements: []string{"/api/v1/shares"}},
	}
}
