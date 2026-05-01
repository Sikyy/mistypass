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
	ResponseSchema string // e.g. "Place" → $ref: #/components/schemas/Place
	RequestSchema  string // e.g. "PlaceCreate" → $ref: #/components/schemas/PlaceCreate
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
			"version":     "2026-05-01",
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
		reqSchema := map[string]any{"type": "object", "additionalProperties": true}
		if definition.RequestSchema != "" {
			reqSchema = map[string]any{"$ref": "#/components/schemas/" + definition.RequestSchema}
		}
		operation["requestBody"] = map[string]any{
			"required": true,
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": reqSchema,
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
		itemSchema := map[string]any{"type": "object", "additionalProperties": true}
		if definition.ResponseSchema != "" {
			itemSchema = map[string]any{"$ref": "#/components/schemas/" + definition.ResponseSchema}
		}
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
					"schema": map[string]any{
						"type":     "object",
						"required": []string{"items", "pagination"},
						"properties": map[string]any{
							"items":      map[string]any{"type": "array", "items": itemSchema},
							"pagination": map[string]any{"$ref": "#/components/schemas/Pagination"},
						},
					},
				},
			},
		}
	case definition.Created:
		responses["201"] = openAPITypedObjectResponse("Created", definition.ResponseSchema)
	default:
		responses["200"] = openAPITypedObjectResponse("OK", definition.ResponseSchema)
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
	return openAPITypedObjectResponse(description, "")
}

func openAPITypedObjectResponse(description, schemaName string) map[string]any {
	schema := map[string]any{"$ref": "#/components/schemas/ObjectResponse"}
	if schemaName != "" {
		schema = map[string]any{"$ref": "#/components/schemas/" + schemaName}
	}
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": schema,
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
			"Place":           openAPIPlaceSchema(),
			"PlaceCreate":     openAPIPlaceCreateSchema(),
			"Lock":            openAPILockSchema(),
			"LockCreate":      openAPILockCreateSchema(),
			"User":            openAPIUserSchema(),
			"UserCreate":      openAPIUserCreateSchema(),
			"Group":           openAPIGroupSchema(),
			"Team":            openAPITeamSchema(),
			"TeamMembership":  openAPITeamMembershipSchema(),
			"Role":            openAPIRoleSchema(),
			"RoleAssignment":  openAPIRoleAssignmentSchema(),
			"Share":           openAPIShareSchema(),
			"Card":            openAPICardSchema(),
			"CardAssignment":  openAPICardAssignmentSchema(),
			"Controller":      openAPIControllerSchema(),
			"Reader":          openAPIReaderSchema(),
			"Terminal":        openAPITerminalSchema(),
			"AlertPolicy":     openAPIAlertPolicySchema(),
			"HolidayCalendar": openAPIHolidayCalendarSchema(),
			"TimeWindow":      openAPITimeWindowSchema(),
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

		{Method: http.MethodGet, Path: "/api/v1/places", OperationID: "fetchPlaces", Tag: "Places", Summary: "Fetch places.", Collection: true, ResponseSchema: "Place"},
		{Method: http.MethodPost, Path: "/api/v1/places", OperationID: "createPlace", Tag: "Places", Summary: "Create a place.", Created: true, ResponseSchema: "Place", RequestSchema: "PlaceCreate"},
		{Method: http.MethodGet, Path: "/api/v1/places/{placeID}", OperationID: "fetchPlace", Tag: "Places", Summary: "Fetch a place.", ResponseSchema: "Place"},
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

		{Method: http.MethodGet, Path: "/api/v1/locks", OperationID: "fetchLocks", Tag: "Locks", Summary: "Fetch locks.", Collection: true, ResponseSchema: "Lock"},
		{Method: http.MethodPost, Path: "/api/v1/locks", OperationID: "createLock", Tag: "Locks", Summary: "Create a lock.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/locks/{lockID}", OperationID: "fetchLock", Tag: "Locks", Summary: "Fetch a lock."},
		{Method: http.MethodPatch, Path: "/api/v1/locks/{lockID}", OperationID: "updateLock", Tag: "Locks", Summary: "Update a lock."},
		{Method: http.MethodDelete, Path: "/api/v1/locks/{lockID}", OperationID: "deleteLock", Tag: "Locks", Summary: "Delete a lock.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/locks/{lockID}/unlock", OperationID: "unlockLock", Tag: "Locks", Summary: "Unlock a lock."},
		{Method: http.MethodPost, Path: "/api/v1/locks/{lockID}/lock_down", OperationID: "lockDownLock", Tag: "Locks", Summary: "Lock down a lock."},
		{Method: http.MethodPost, Path: "/api/v1/locks/{lockID}/cancel_lockdown", OperationID: "cancelLockLockdown", Tag: "Locks", Summary: "Cancel lock lockdown."},

		{Method: http.MethodGet, Path: "/api/v1/controllers", OperationID: "fetchControllers", Tag: "Hardware", Summary: "Fetch controllers.", Collection: true, ResponseSchema: "Controller"},
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

		{Method: http.MethodGet, Path: "/api/v1/users", OperationID: "fetchUsers", Tag: "Users", Summary: "Fetch users.", Collection: true, ResponseSchema: "User"},
		{Method: http.MethodPost, Path: "/api/v1/users", OperationID: "createUser", Tag: "Users", Summary: "Create a user.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/users/{userID}", OperationID: "fetchUser", Tag: "Users", Summary: "Fetch a user."},
		{Method: http.MethodPatch, Path: "/api/v1/users/{userID}", OperationID: "updateUser", Tag: "Users", Summary: "Update a user."},
		{Method: http.MethodDelete, Path: "/api/v1/users/{userID}", OperationID: "deleteUser", Tag: "Users", Summary: "Delete a user.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/users/{userID}/invite", OperationID: "inviteUser", Tag: "Users", Summary: "Invite a user."},
		{Method: http.MethodGet, Path: "/api/v1/users/{userID}/invitations", OperationID: "fetchUserInvitations", Tag: "Users", Summary: "Fetch user invitations.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/users/{userID}/invitations/{deliveryID}/receipt", OperationID: "recordUserInvitationReceipt", Tag: "Users", Summary: "Record a user invitation receipt."},
		{Method: http.MethodGet, Path: "/api/v1/members", OperationID: "fetchMembers", Tag: "Users", Summary: "Fetch members.", Collection: true},

		{Method: http.MethodGet, Path: "/api/v1/teams", OperationID: "fetchTeams", Tag: "Teams", Summary: "Fetch teams.", Collection: true, ResponseSchema: "Team"},
		{Method: http.MethodPost, Path: "/api/v1/teams", OperationID: "createTeam", Tag: "Teams", Summary: "Create a team.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/teams/{teamID}", OperationID: "fetchTeam", Tag: "Teams", Summary: "Fetch a team."},
		{Method: http.MethodPatch, Path: "/api/v1/teams/{teamID}", OperationID: "updateTeam", Tag: "Teams", Summary: "Update a team."},
		{Method: http.MethodDelete, Path: "/api/v1/teams/{teamID}", OperationID: "deleteTeam", Tag: "Teams", Summary: "Delete a team.", NoContent: true},
		{Method: http.MethodGet, Path: "/api/v1/team_memberships", OperationID: "fetchTeamMemberships", Tag: "Teams", Summary: "Fetch team memberships.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/team_memberships", OperationID: "createTeamMembership", Tag: "Teams", Summary: "Create a team membership.", Created: true},
		{Method: http.MethodDelete, Path: "/api/v1/team_memberships/{membershipID}", OperationID: "deleteTeamMembership", Tag: "Teams", Summary: "Delete a team membership.", NoContent: true},

		{Method: http.MethodGet, Path: "/api/v1/groups", OperationID: "fetchGroups", Tag: "Groups", Summary: "Fetch groups.", Collection: true, ResponseSchema: "Group"},
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

		{Method: http.MethodGet, Path: "/api/v1/roles", OperationID: "fetchRoles", Tag: "Roles", Summary: "Fetch roles.", Collection: true, ResponseSchema: "Role"},
		{Method: http.MethodGet, Path: "/api/v1/roles/{roleID}", OperationID: "fetchRole", Tag: "Roles", Summary: "Fetch a role."},
		{Method: http.MethodGet, Path: "/api/v1/role_assignments", OperationID: "fetchRoleAssignments", Tag: "Roles", Summary: "Fetch role assignments.", Collection: true, ResponseSchema: "RoleAssignment"},
		{Method: http.MethodPost, Path: "/api/v1/role_assignments", OperationID: "createRoleAssignment", Tag: "Roles", Summary: "Create a role assignment.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/role_assignments/{assignmentID}", OperationID: "fetchRoleAssignment", Tag: "Roles", Summary: "Fetch a role assignment."},
		{Method: http.MethodPatch, Path: "/api/v1/role_assignments/{assignmentID}", OperationID: "updateRoleAssignment", Tag: "Roles", Summary: "Update a role assignment."},
		{Method: http.MethodDelete, Path: "/api/v1/role_assignments/{assignmentID}", OperationID: "deleteRoleAssignment", Tag: "Roles", Summary: "Delete a role assignment.", NoContent: true},
		{Method: http.MethodGet, Path: "/api/v1/shares", OperationID: "fetchShares", Tag: "Shares", Summary: "Fetch shares.", Collection: true, ResponseSchema: "Share"},
		{Method: http.MethodPost, Path: "/api/v1/shares", OperationID: "createShare", Tag: "Shares", Summary: "Create a share.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/shares/{shareID}", OperationID: "fetchShare", Tag: "Shares", Summary: "Fetch a share."},
		{Method: http.MethodPatch, Path: "/api/v1/shares/{shareID}", OperationID: "updateShare", Tag: "Shares", Summary: "Update a share."},
		{Method: http.MethodDelete, Path: "/api/v1/shares/{shareID}", OperationID: "deleteShare", Tag: "Shares", Summary: "Delete a share.", NoContent: true},

		{Method: http.MethodGet, Path: "/api/v1/cards", OperationID: "fetchCards", Tag: "Credentials", Summary: "Fetch cards.", Collection: true, ResponseSchema: "Card"},
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
		{Method: http.MethodGet, Path: "/api/v1/alert_policies", OperationID: "fetchAlertPolicies", Tag: "Alert Policies", Summary: "Fetch alert policies.", Collection: true, ResponseSchema: "AlertPolicy"},
		{Method: http.MethodPost, Path: "/api/v1/alert_policies", OperationID: "createAlertPolicy", Tag: "Alert Policies", Summary: "Create an alert policy.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/alert_policies/{policyID}", OperationID: "fetchAlertPolicy", Tag: "Alert Policies", Summary: "Fetch an alert policy."},
		{Method: http.MethodPatch, Path: "/api/v1/alert_policies/{policyID}", OperationID: "updateAlertPolicy", Tag: "Alert Policies", Summary: "Update an alert policy."},
		{Method: http.MethodDelete, Path: "/api/v1/alert_policies/{policyID}", OperationID: "deleteAlertPolicy", Tag: "Alert Policies", Summary: "Disable an alert policy.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/alert_policies/condition_preview", OperationID: "previewAlertPolicyCondition", Tag: "Alert Policies", Summary: "Preview an alert policy condition."},
		{Method: http.MethodPost, Path: "/api/v1/alert_policies/evaluate", OperationID: "evaluateAlertPoliciesForEvent", Tag: "Alert Policies", Summary: "Evaluate alert policies for an event."},
		{Method: http.MethodGet, Path: "/api/v1/alert_policies/notifications", OperationID: "fetchAlertPolicyNotifications", Tag: "Alert Policies", Summary: "Fetch alert policy notifications.", Collection: true},

		// Tenants
		{Method: http.MethodGet, Path: "/api/v1/tenants", OperationID: "fetchTenants", Tag: "Tenants", Summary: "Fetch tenants.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/tenants", OperationID: "createTenant", Tag: "Tenants", Summary: "Create a tenant.", Created: true},
		{Method: http.MethodPatch, Path: "/api/v1/tenants/{tenantID}/status", OperationID: "updateTenantStatus", Tag: "Tenants", Summary: "Update tenant status."},
		{Method: http.MethodGet, Path: "/api/v1/tenants/{tenantID}/topology", OperationID: "fetchTenantTopology", Tag: "Tenants", Summary: "Fetch tenant topology."},

		// User Groups
		{Method: http.MethodGet, Path: "/api/v1/user-groups", OperationID: "fetchUserGroups", Tag: "Access Control", Summary: "Fetch user groups.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/user-groups", OperationID: "createUserGroup", Tag: "Access Control", Summary: "Create a user group.", Created: true},
		{Method: http.MethodPatch, Path: "/api/v1/user-groups/{groupID}", OperationID: "updateUserGroup", Tag: "Access Control", Summary: "Update a user group."},

		// Temporary Access & Visitor Passes
		{Method: http.MethodGet, Path: "/api/v1/temporary-access", OperationID: "fetchTemporaryAccess", Tag: "Access Control", Summary: "Fetch temporary access grants.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/temporary-access", OperationID: "createTemporaryAccess", Tag: "Access Control", Summary: "Create a temporary access grant.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/visitor-passes", OperationID: "fetchVisitorPasses", Tag: "Visitors", Summary: "Fetch visitor passes.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/visitor-passes", OperationID: "createVisitorPass", Tag: "Visitors", Summary: "Create a visitor pass.", Created: true},

		// Credentials
		{Method: http.MethodGet, Path: "/api/v1/credentials", OperationID: "fetchCredentials", Tag: "Credentials", Summary: "Fetch credentials.", Collection: true},

		// Holiday Calendars
		{Method: http.MethodGet, Path: "/api/v1/holiday_calendars", OperationID: "fetchHolidayCalendars", Tag: "Schedules", Summary: "Fetch holiday calendars.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/holiday_calendars", OperationID: "createHolidayCalendar", Tag: "Schedules", Summary: "Create a holiday calendar.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/holiday_calendars/{calendarID}", OperationID: "fetchHolidayCalendar", Tag: "Schedules", Summary: "Fetch a holiday calendar."},
		{Method: http.MethodPatch, Path: "/api/v1/holiday_calendars/{calendarID}", OperationID: "updateHolidayCalendar", Tag: "Schedules", Summary: "Update a holiday calendar."},
		{Method: http.MethodDelete, Path: "/api/v1/holiday_calendars/{calendarID}", OperationID: "deleteHolidayCalendar", Tag: "Schedules", Summary: "Delete a holiday calendar.", NoContent: true},
		{Method: http.MethodGet, Path: "/api/v1/holiday_calendars/presets", OperationID: "fetchHolidayCalendarPresets", Tag: "Schedules", Summary: "Fetch holiday calendar presets.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/holiday_calendars/preset_countries", OperationID: "fetchHolidayCalendarPresetCountries", Tag: "Schedules", Summary: "Fetch preset countries.", Collection: true},

		// Schedules
		{Method: http.MethodGet, Path: "/api/v1/schedules", OperationID: "fetchSchedules", Tag: "Schedules", Summary: "Fetch schedules.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/schedules", OperationID: "createSchedule", Tag: "Schedules", Summary: "Create a schedule.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/schedules/{scheduleID}", OperationID: "fetchSchedule", Tag: "Schedules", Summary: "Fetch a schedule."},
		{Method: http.MethodPatch, Path: "/api/v1/schedules/{scheduleID}", OperationID: "updateSchedule", Tag: "Schedules", Summary: "Update a schedule."},
		{Method: http.MethodDelete, Path: "/api/v1/schedules/{scheduleID}", OperationID: "deleteSchedule", Tag: "Schedules", Summary: "Delete a schedule.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/access_rights/schedule/evaluate", OperationID: "evaluateAccessRightSchedule", Tag: "Schedules", Summary: "Evaluate an access right schedule."},

		// Invitations
		{Method: http.MethodGet, Path: "/api/v1/invitations", OperationID: "fetchInvitations", Tag: "Users", Summary: "Fetch invitations.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/invitations/{deliveryID}", OperationID: "fetchInvitation", Tag: "Users", Summary: "Fetch an invitation."},
		{Method: http.MethodPost, Path: "/api/v1/invitations/{deliveryID}/cancel", OperationID: "cancelInvitation", Tag: "Users", Summary: "Cancel an invitation."},
		{Method: http.MethodPost, Path: "/api/v1/invitations/{deliveryID}/resend", OperationID: "resendInvitation", Tag: "Users", Summary: "Resend an invitation."},
		{Method: http.MethodPost, Path: "/api/v1/users/invitations/provider-receipts", OperationID: "receiveUserInvitationProviderReceipt", Tag: "Users", Summary: "Receive invitation provider receipt.", Public: true},

		// Users Batch Operations
		{Method: http.MethodPost, Path: "/api/v1/users/batch-status", OperationID: "batchUpdateUserStatus", Tag: "Users", Summary: "Batch update user status."},
		{Method: http.MethodPost, Path: "/api/v1/users/batch-delete", OperationID: "batchDeleteUsers", Tag: "Users", Summary: "Batch delete users."},
		{Method: http.MethodPost, Path: "/api/v1/users/batch-invite", OperationID: "batchInviteUsers", Tag: "Users", Summary: "Batch invite users."},
		{Method: http.MethodPost, Path: "/api/v1/users/import-csv", OperationID: "importUsersCSV", Tag: "Users", Summary: "Import users from CSV."},
		{Method: http.MethodGet, Path: "/api/v1/users/export-csv", OperationID: "exportUsersCSV", Tag: "Users", Summary: "Export users to CSV."},

		// Auth: Building Scope & Password Auth
		{Method: http.MethodGet, Path: "/api/v1/auth/users/{userID}/building-scope", OperationID: "fetchAuthUserBuildingScope", Tag: "Authentication", Summary: "Fetch user building scope."},
		{Method: http.MethodPut, Path: "/api/v1/auth/users/{userID}/building-scope", OperationID: "updateAuthUserBuildingScope", Tag: "Authentication", Summary: "Update user building scope."},
		{Method: http.MethodPatch, Path: "/api/v1/auth/users/{userID}/password-auth", OperationID: "updateAuthUserPasswordAuth", Tag: "Authentication", Summary: "Toggle user password authentication."},

		// Alarms
		{Method: http.MethodGet, Path: "/api/v1/alarms", OperationID: "fetchAlarms", Tag: "Alarms", Summary: "Fetch alarms.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/alarms/stream", OperationID: "streamAlarms", Tag: "Alarms", Summary: "Stream alarms via SSE."},
		{Method: http.MethodPatch, Path: "/api/v1/alarms/{alarmID}/status", OperationID: "updateAlarmStatus", Tag: "Alarms", Summary: "Update alarm status."},

		// Events (Access + Device + Stream)
		{Method: http.MethodGet, Path: "/api/v1/events/access", OperationID: "fetchAccessEvents", Tag: "Events", Summary: "Fetch access events.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/events/device", OperationID: "fetchDeviceEvents", Tag: "Events", Summary: "Fetch device events.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/events/stream", OperationID: "streamEvents", Tag: "Events", Summary: "Stream events via SSE."},

		// Audit
		{Method: http.MethodGet, Path: "/api/v1/audit-logs", OperationID: "fetchAuditLogs", Tag: "Audit", Summary: "Fetch audit logs.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/audit/webhook/config", OperationID: "fetchAuditWebhookConfig", Tag: "Audit", Summary: "Fetch audit webhook config."},
		{Method: http.MethodPut, Path: "/api/v1/audit/webhook/config", OperationID: "updateAuditWebhookConfig", Tag: "Audit", Summary: "Update audit webhook config."},
		{Method: http.MethodGet, Path: "/api/v1/audit/webhook/deliveries", OperationID: "fetchAuditWebhookDeliveries", Tag: "Audit", Summary: "Fetch audit webhook deliveries.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/audit/webhook/dispatch", OperationID: "dispatchAuditWebhook", Tag: "Audit", Summary: "Dispatch audit webhook."},

		// Organization Advanced
		{Method: http.MethodPost, Path: "/api/v1/organization/export-audit", OperationID: "exportOrganizationAudit", Tag: "Organization", Summary: "Export organization audit log."},
		{Method: http.MethodPost, Path: "/api/v1/organization/rotate-webhooks", OperationID: "rotateOrganizationWebhooks", Tag: "Organization", Summary: "Rotate organization webhook secrets."},
		{Method: http.MethodPost, Path: "/api/v1/organization/disable", OperationID: "disableOrganization", Tag: "Organization", Summary: "Disable organization."},

		// Enterprise SSO
		{Method: http.MethodPost, Path: "/api/v1/enterprise/tenant/resolve", OperationID: "resolveEnterpriseTenant", Tag: "Enterprise", Summary: "Resolve enterprise tenant by email.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/auth/start", OperationID: "startEnterpriseAuth", Tag: "Enterprise", Summary: "Start enterprise SSO auth.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/auth/exchange", OperationID: "exchangeEnterpriseAuth", Tag: "Enterprise", Summary: "Exchange enterprise SSO token.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/auth/logout", OperationID: "logoutEnterpriseAuth", Tag: "Enterprise", Summary: "Logout enterprise SSO.", Public: true},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/auth/oidc/callback", OperationID: "enterpriseOIDCCallback", Tag: "Enterprise", Summary: "Enterprise OIDC callback.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/auth/saml/callback", OperationID: "enterpriseSAMLCallback", Tag: "Enterprise", Summary: "Enterprise SAML callback.", Public: true},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/domain-mappings", OperationID: "fetchEnterpriseDomainMappings", Tag: "Enterprise", Summary: "Fetch enterprise domain mappings.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/domain-mappings", OperationID: "createEnterpriseDomainMapping", Tag: "Enterprise", Summary: "Create enterprise domain mapping.", Created: true},
		{Method: http.MethodPatch, Path: "/api/v1/enterprise/domain-mappings/{mappingID}/status", OperationID: "updateEnterpriseDomainMappingStatus", Tag: "Enterprise", Summary: "Update enterprise domain mapping status."},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/idp-config", OperationID: "fetchEnterpriseIDPConfig", Tag: "Enterprise", Summary: "Fetch enterprise IdP config."},
		{Method: http.MethodPut, Path: "/api/v1/enterprise/idp-config", OperationID: "upsertEnterpriseIDPConfig", Tag: "Enterprise", Summary: "Create or update enterprise IdP config."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/idp-config/validate", OperationID: "validateEnterpriseIDPConfig", Tag: "Enterprise", Summary: "Validate enterprise IdP config."},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/employees", OperationID: "fetchEnterpriseEmployees", Tag: "Enterprise", Summary: "Fetch enterprise employees.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/employees/sync", OperationID: "syncEnterpriseEmployees", Tag: "Enterprise", Summary: "Sync enterprise employees."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/employees/sync/reconcile", OperationID: "reconcileEnterpriseEmployees", Tag: "Enterprise", Summary: "Reconcile enterprise employee sync."},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/sync-jobs", OperationID: "fetchEnterpriseSyncJobs", Tag: "Enterprise", Summary: "Fetch enterprise sync jobs.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/sync-requests", OperationID: "fetchEnterpriseSyncRequests", Tag: "Enterprise", Summary: "Fetch enterprise sync requests.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/sync-requests/reconcile-pending", OperationID: "reconcilePendingEnterpriseSyncRequests", Tag: "Enterprise", Summary: "Reconcile pending sync requests."},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/jit-provision-approvals", OperationID: "fetchEnterpriseJITApprovals", Tag: "Enterprise", Summary: "Fetch JIT provisioning approvals.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/jit-provision-approvals/external-sync-pending", OperationID: "fetchEnterpriseJITApprovalExternalSyncPending", Tag: "Enterprise", Summary: "Fetch pending external sync approvals.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/jit-provision-approvals/{approvalID}/review", OperationID: "reviewEnterpriseJITApproval", Tag: "Enterprise", Summary: "Review a JIT provisioning approval."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/jit-provision-approvals/{approvalID}/external-sync", OperationID: "syncEnterpriseJITApprovalExternal", Tag: "Enterprise", Summary: "Sync JIT approval to external system."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/jit-provision-approvals/external-sync/callback", OperationID: "enterpriseJITApprovalExternalSyncCallback", Tag: "Enterprise", Summary: "JIT approval external sync callback.", Public: true},

		// Enterprise HRIS
		{Method: http.MethodGet, Path: "/api/v1/enterprise/hris-connectors", OperationID: "fetchEnterpriseHRISConnectors", Tag: "Enterprise HRIS", Summary: "Fetch HRIS connectors.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/hris-connectors", OperationID: "createEnterpriseHRISConnector", Tag: "Enterprise HRIS", Summary: "Create HRIS connector.", Created: true},
		{Method: http.MethodPatch, Path: "/api/v1/enterprise/hris-connectors/{connectorID}", OperationID: "updateEnterpriseHRISConnector", Tag: "Enterprise HRIS", Summary: "Update HRIS connector."},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/hris-secrets", OperationID: "fetchEnterpriseHRISSecrets", Tag: "Enterprise HRIS", Summary: "Fetch HRIS secrets."},
		{Method: http.MethodPut, Path: "/api/v1/enterprise/hris-secrets", OperationID: "upsertEnterpriseHRISSecrets", Tag: "Enterprise HRIS", Summary: "Upsert HRIS secrets."},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/hris-pull-states", OperationID: "fetchEnterpriseHRISPullStates", Tag: "Enterprise HRIS", Summary: "Fetch HRIS pull states.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/hris-webhook/{connectorID}", OperationID: "receiveEnterpriseHRISWebhook", Tag: "Enterprise HRIS", Summary: "Receive HRIS webhook.", Public: true},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/hris-webhook-receipts", OperationID: "fetchEnterpriseHRISWebhookReceipts", Tag: "Enterprise HRIS", Summary: "Fetch HRIS webhook receipts.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/hris-webhook-receipts/{receiptID}/process", OperationID: "processEnterpriseHRISWebhookReceipt", Tag: "Enterprise HRIS", Summary: "Process HRIS webhook receipt."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/hris-webhook-receipts/process-batch", OperationID: "processEnterpriseHRISWebhookReceiptBatch", Tag: "Enterprise HRIS", Summary: "Process HRIS webhook receipts in batch."},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/hris-webhook-executions", OperationID: "fetchEnterpriseHRISWebhookExecutions", Tag: "Enterprise HRIS", Summary: "Fetch HRIS webhook executions.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/hris-webhook-executions/{executionID}", OperationID: "fetchEnterpriseHRISWebhookExecution", Tag: "Enterprise HRIS", Summary: "Fetch a HRIS webhook execution."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/hris-webhook-executions/{executionID}/replay", OperationID: "replayEnterpriseHRISWebhookExecution", Tag: "Enterprise HRIS", Summary: "Replay HRIS webhook execution."},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/hris-webhook-dlq", OperationID: "fetchEnterpriseHRISWebhookDLQ", Tag: "Enterprise HRIS", Summary: "Fetch HRIS webhook DLQ.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/hris-webhook-dlq/{entryID}/replay", OperationID: "replayEnterpriseHRISWebhookDLQEntry", Tag: "Enterprise HRIS", Summary: "Replay HRIS webhook DLQ entry."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/hris-webhook-dlq/replay-batch", OperationID: "replayEnterpriseHRISWebhookDLQBatch", Tag: "Enterprise HRIS", Summary: "Replay HRIS webhook DLQ batch."},

		// Enterprise Sync Worker Alerts
		{Method: http.MethodGet, Path: "/api/v1/enterprise/sync-worker-alerts", OperationID: "fetchEnterpriseSyncWorkerAlerts", Tag: "Enterprise", Summary: "Fetch sync worker alerts.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/sync-worker-alerts/summary", OperationID: "fetchEnterpriseSyncWorkerAlertSummary", Tag: "Enterprise", Summary: "Fetch sync worker alert summary."},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/sync-worker-alerts/notifications", OperationID: "fetchEnterpriseSyncWorkerAlertNotifications", Tag: "Enterprise", Summary: "Fetch sync worker alert notifications.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/sync-worker-alerts/notifications/export-csv", OperationID: "exportEnterpriseSyncWorkerAlertNotificationsCSV", Tag: "Enterprise", Summary: "Export sync worker alert notifications CSV."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/sync-worker-alerts/notifications/{notificationID}/retry", OperationID: "retryEnterpriseSyncWorkerAlertNotification", Tag: "Enterprise", Summary: "Retry sync worker alert notification."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/sync-worker-alerts/notifications/retry-batch", OperationID: "retryEnterpriseSyncWorkerAlertNotificationBatch", Tag: "Enterprise", Summary: "Retry sync worker alert notifications batch."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/sync-worker-alerts/notifications/suppress-batch", OperationID: "suppressEnterpriseSyncWorkerAlertNotificationBatch", Tag: "Enterprise", Summary: "Suppress sync worker alert notifications batch."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/sync-worker-alerts/notifications/restore-batch", OperationID: "restoreEnterpriseSyncWorkerAlertNotificationBatch", Tag: "Enterprise", Summary: "Restore sync worker alert notifications batch."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/sync-worker-alerts/notifications/auto-retry", OperationID: "autoRetryEnterpriseSyncWorkerAlertNotifications", Tag: "Enterprise", Summary: "Auto-retry sync worker alert notifications."},
		{Method: http.MethodPost, Path: "/api/v1/enterprise/sync-worker-alerts/dispatch", OperationID: "dispatchEnterpriseSyncWorkerAlerts", Tag: "Enterprise", Summary: "Dispatch sync worker alerts."},
		{Method: http.MethodGet, Path: "/api/v1/enterprise/sync-worker-alert-subscription", OperationID: "fetchEnterpriseSyncWorkerAlertSubscription", Tag: "Enterprise", Summary: "Fetch sync worker alert subscription."},
		{Method: http.MethodPut, Path: "/api/v1/enterprise/sync-worker-alert-subscription", OperationID: "updateEnterpriseSyncWorkerAlertSubscription", Tag: "Enterprise", Summary: "Update sync worker alert subscription."},

		// State Management
		{Method: http.MethodGet, Path: "/api/v1/state/change-log", OperationID: "fetchStateChangeLog", Tag: "State Management", Summary: "Fetch state change log.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/state/change-log/checkpoints", OperationID: "fetchStateChangeLogCheckpoints", Tag: "State Management", Summary: "Fetch state change log checkpoints.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/state/change-log/replay", OperationID: "replayStateChangeLog", Tag: "State Management", Summary: "Replay state change log."},
		{Method: http.MethodPost, Path: "/api/v1/state/change-log/replay/checkpoint", OperationID: "checkpointStateChangeLogReplay", Tag: "State Management", Summary: "Checkpoint state change log replay."},

		// Mobile App (Access)
		{Method: http.MethodGet, Path: "/api/v1/app/access/my-doors", OperationID: "fetchAppAccessMyDoors", Tag: "Mobile App", Summary: "Fetch resident accessible doors (filtered by user).", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/app/access/unlock", OperationID: "appUnlockDoor", Tag: "Mobile App", Summary: "Unlock a door from the mobile app."},
		{Method: http.MethodPost, Path: "/api/v1/app/access/qr-unlock", OperationID: "appQRUnlockDoor", Tag: "Mobile App", Summary: "Unlock a door via QR code."},

		// Gateway Access Rules (admin view)
		{Method: http.MethodGet, Path: "/api/v1/gateways/{gatewayID}/access-rules", OperationID: "fetchGatewayAccessRules", Tag: "Hardware Extensions", Summary: "Fetch gateway access rules.", Collection: true, ExtensionGroup: "legacy_gateway"},

		// Gateway events (single)
		{Method: http.MethodPost, Path: "/api/v1/gateway/events/access", OperationID: "submitGatewayAccessEvent", Tag: "Gateway Bootstrap", Summary: "Submit a single gateway access event.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/gateway/events/device", OperationID: "submitGatewayDeviceEvent", Tag: "Gateway Bootstrap", Summary: "Submit a single gateway device event.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/gateway/status", OperationID: "submitGatewayStatus", Tag: "Gateway Bootstrap", Summary: "Submit gateway status.", Public: true},

		// Locks/Places favorite/unfavorite
		{Method: http.MethodPost, Path: "/api/v1/locks/{lockID}/favorite", OperationID: "favoriteLock", Tag: "Locks", Summary: "Favorite a lock."},
		{Method: http.MethodPost, Path: "/api/v1/locks/{lockID}/unfavorite", OperationID: "unfavoriteLock", Tag: "Locks", Summary: "Unfavorite a lock."},
		{Method: http.MethodPost, Path: "/api/v1/places/{placeID}/favorite", OperationID: "favoritePlace", Tag: "Places", Summary: "Favorite a place."},
		{Method: http.MethodPost, Path: "/api/v1/places/{placeID}/unfavorite", OperationID: "unfavoritePlace", Tag: "Places", Summary: "Unfavorite a place."},

		// Controllers individual
		{Method: http.MethodGet, Path: "/api/v1/controllers/{controllerID}", OperationID: "fetchController", Tag: "Hardware", Summary: "Fetch a controller."},
		{Method: http.MethodPatch, Path: "/api/v1/controllers/{controllerID}", OperationID: "updateController", Tag: "Hardware", Summary: "Update a controller."},

		// Readers individual
		{Method: http.MethodGet, Path: "/api/v1/readers/{readerID}", OperationID: "fetchReader", Tag: "Hardware", Summary: "Fetch a reader."},
		{Method: http.MethodPatch, Path: "/api/v1/readers/{readerID}", OperationID: "updateReader", Tag: "Hardware", Summary: "Update a reader."},
		{Method: http.MethodPost, Path: "/api/v1/readers/{readerID}/reset_tamper", OperationID: "resetTamperReader", Tag: "Hardware", Summary: "Reset reader tamper state."},

		// Terminals CRUD
		{Method: http.MethodPost, Path: "/api/v1/terminals", OperationID: "createTerminal", Tag: "Hardware", Summary: "Create a terminal.", Created: true},
		{Method: http.MethodPut, Path: "/api/v1/terminals/{terminalID}", OperationID: "updateTerminal", Tag: "Hardware", Summary: "Update a terminal."},
		{Method: http.MethodDelete, Path: "/api/v1/terminals/{terminalID}", OperationID: "deleteTerminal", Tag: "Hardware", Summary: "Delete a terminal.", NoContent: true},

		// Members CRUD
		{Method: http.MethodPost, Path: "/api/v1/members", OperationID: "createMember", Tag: "Users", Summary: "Create a member.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/members/{memberID}", OperationID: "fetchMember", Tag: "Users", Summary: "Fetch a member."},
		{Method: http.MethodPatch, Path: "/api/v1/members/{memberID}", OperationID: "updateMember", Tag: "Users", Summary: "Update a member."},
		{Method: http.MethodDelete, Path: "/api/v1/members/{memberID}", OperationID: "deleteMember", Tag: "Users", Summary: "Delete a member.", NoContent: true},

		// Group Zones CRUD
		{Method: http.MethodPost, Path: "/api/v1/group_zones", OperationID: "createGroupZone", Tag: "Groups", Summary: "Create a group zone.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/group_zones/{groupZoneID}", OperationID: "fetchGroupZone", Tag: "Groups", Summary: "Fetch a group zone."},
		{Method: http.MethodDelete, Path: "/api/v1/group_zones/{groupZoneID}", OperationID: "deleteGroupZone", Tag: "Groups", Summary: "Delete a group zone.", NoContent: true},

		// Cards update/delete
		{Method: http.MethodPatch, Path: "/api/v1/cards/{cardID}", OperationID: "updateCard", Tag: "Credentials", Summary: "Update a card."},
		{Method: http.MethodDelete, Path: "/api/v1/cards/{cardID}", OperationID: "deleteCard", Tag: "Credentials", Summary: "Delete a card.", NoContent: true},

		// Card Assignments update/delete/activate/deactivate
		{Method: http.MethodPatch, Path: "/api/v1/card_assignments/{assignmentID}", OperationID: "updateCardAssignment", Tag: "Credentials", Summary: "Update a card assignment."},
		{Method: http.MethodDelete, Path: "/api/v1/card_assignments/{assignmentID}", OperationID: "deleteCardAssignment", Tag: "Credentials", Summary: "Delete a card assignment.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/card_assignments/{assignmentID}/activate", OperationID: "activateCardAssignment", Tag: "Credentials", Summary: "Activate a card assignment."},
		{Method: http.MethodPost, Path: "/api/v1/card_assignments/{assignmentID}/deactivate", OperationID: "deactivateCardAssignment", Tag: "Credentials", Summary: "Deactivate a card assignment."},

		// Reports create/delete
		{Method: http.MethodPost, Path: "/api/v1/reports", OperationID: "createReport", Tag: "Reports", Summary: "Create a report.", Created: true},
		{Method: http.MethodDelete, Path: "/api/v1/reports/{reportID}", OperationID: "deleteReport", Tag: "Reports", Summary: "Delete a report.", NoContent: true},

		// Elevators
		{Method: http.MethodGet, Path: "/api/v1/elevators", OperationID: "fetchElevators", Tag: "Elevators", Summary: "Fetch elevators.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/elevators", OperationID: "createElevator", Tag: "Elevators", Summary: "Create an elevator.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/elevators/{elevatorID}", OperationID: "fetchElevator", Tag: "Elevators", Summary: "Fetch an elevator."},
		{Method: http.MethodPatch, Path: "/api/v1/elevators/{elevatorID}", OperationID: "updateElevator", Tag: "Elevators", Summary: "Update an elevator."},
		{Method: http.MethodDelete, Path: "/api/v1/elevators/{elevatorID}", OperationID: "deleteElevator", Tag: "Elevators", Summary: "Delete an elevator.", NoContent: true},

		// Elevator Stops
		{Method: http.MethodGet, Path: "/api/v1/elevator_stops", OperationID: "fetchElevatorStops", Tag: "Elevators", Summary: "Fetch elevator stops.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/elevator_stops", OperationID: "createElevatorStop", Tag: "Elevators", Summary: "Create an elevator stop.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/elevator_stops/{elevatorStopID}", OperationID: "fetchElevatorStop", Tag: "Elevators", Summary: "Fetch an elevator stop."},
		{Method: http.MethodPatch, Path: "/api/v1/elevator_stops/{elevatorStopID}", OperationID: "updateElevatorStop", Tag: "Elevators", Summary: "Update an elevator stop."},
		{Method: http.MethodDelete, Path: "/api/v1/elevator_stops/{elevatorStopID}", OperationID: "deleteElevatorStop", Tag: "Elevators", Summary: "Delete an elevator stop.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/elevator_stops/{elevatorStopID}/lock_down", OperationID: "lockDownElevatorStop", Tag: "Elevators", Summary: "Lock down an elevator stop."},
		{Method: http.MethodPost, Path: "/api/v1/elevator_stops/{elevatorStopID}/cancel_lockdown", OperationID: "cancelElevatorStopLockdown", Tag: "Elevators", Summary: "Cancel elevator stop lockdown."},

		// Group Elevator Stops
		{Method: http.MethodGet, Path: "/api/v1/group_elevator_stops", OperationID: "fetchGroupElevatorStops", Tag: "Elevators", Summary: "Fetch group elevator stops.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/group_elevator_stops", OperationID: "createGroupElevatorStop", Tag: "Elevators", Summary: "Create a group elevator stop.", Created: true},
		{Method: http.MethodDelete, Path: "/api/v1/group_elevator_stops/{groupElevatorStopID}", OperationID: "deleteGroupElevatorStop", Tag: "Elevators", Summary: "Delete a group elevator stop.", NoContent: true},

		// Group Terminals
		{Method: http.MethodGet, Path: "/api/v1/group_terminals", OperationID: "fetchGroupTerminals", Tag: "Groups", Summary: "Fetch group terminals.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/group_terminals", OperationID: "createGroupTerminal", Tag: "Groups", Summary: "Create a group terminal.", Created: true},
		{Method: http.MethodDelete, Path: "/api/v1/group_terminals/{groupTerminalID}", OperationID: "deleteGroupTerminal", Tag: "Groups", Summary: "Delete a group terminal.", NoContent: true},

		// Presences
		{Method: http.MethodGet, Path: "/api/v1/presences", OperationID: "fetchPresences", Tag: "Visitors", Summary: "Fetch current presences.", Collection: true},

		// CSV Card Imports
		{Method: http.MethodGet, Path: "/api/v1/csv_card_imports", OperationID: "fetchCSVCardImports", Tag: "Credentials", Summary: "Fetch CSV card imports.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/csv_card_imports", OperationID: "createCSVCardImport", Tag: "Credentials", Summary: "Create a CSV card import.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/csv_card_imports/{importID}", OperationID: "fetchCSVCardImport", Tag: "Credentials", Summary: "Fetch a CSV card import."},

		// Locks occupancy modes
		{Method: http.MethodPost, Path: "/api/v1/locks/{lockID}/first_to_arrive", OperationID: "firstToArriveLock", Tag: "Locks", Summary: "Set lock to first-to-arrive mode."},
		{Method: http.MethodPost, Path: "/api/v1/locks/{lockID}/last_to_leave", OperationID: "lastToLeaveLock", Tag: "Locks", Summary: "Set lock to last-to-leave mode."},

		// Users self-signup + password change
		{Method: http.MethodPost, Path: "/api/v1/users/sign_up", OperationID: "userSignUp", Tag: "Authentication", Summary: "Self-service user registration.", Public: true, Created: true},
		{Method: http.MethodPost, Path: "/api/v1/users/password", OperationID: "changeUserPassword", Tag: "Authentication", Summary: "Change current user password."},
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

		{Method: http.MethodGet, Path: "/api/v1/network/topology", OperationID: "getNetworkTopology", Tag: "Network", Summary: "Device network topology graph for visualization."},

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

		// WebAuthn / Passkeys
		{Method: http.MethodPost, Path: "/api/v1/auth/webauthn/register/begin", OperationID: "webAuthnRegisterBegin", Tag: "Authentication", Summary: "Begin WebAuthn passkey registration."},
		{Method: http.MethodPost, Path: "/api/v1/auth/webauthn/register/finish", OperationID: "webAuthnRegisterFinish", Tag: "Authentication", Summary: "Finish WebAuthn passkey registration.", Created: true},
		{Method: http.MethodPost, Path: "/api/v1/auth/webauthn/login/begin", OperationID: "webAuthnLoginBegin", Tag: "Authentication", Summary: "Begin WebAuthn passkey login.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/auth/webauthn/login/finish", OperationID: "webAuthnLoginFinish", Tag: "Authentication", Summary: "Finish WebAuthn passkey login and issue JWT.", Public: true},
		{Method: http.MethodGet, Path: "/api/v1/auth/webauthn/credentials", OperationID: "fetchWebAuthnCredentials", Tag: "Authentication", Summary: "List registered passkeys for the current user.", Collection: true},
		{Method: http.MethodDelete, Path: "/api/v1/auth/webauthn/credentials/{credentialID}", OperationID: "deleteWebAuthnCredential", Tag: "Authentication", Summary: "Remove a registered passkey.", NoContent: true},

		// Password Reset
		{Method: http.MethodPost, Path: "/api/v1/auth/password-reset/request", OperationID: "requestPasswordReset", Tag: "Authentication", Summary: "Request a password reset token.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/auth/password-reset/confirm", OperationID: "confirmPasswordReset", Tag: "Authentication", Summary: "Confirm password reset with token.", Public: true},

		// Login Sessions
		{Method: http.MethodGet, Path: "/api/v1/auth/sessions", OperationID: "fetchLoginSessions", Tag: "Authentication", Summary: "List active login sessions for the current user.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/auth/sessions/revoke", OperationID: "revokeLoginSession", Tag: "Authentication", Summary: "Revoke a specific login session.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/auth/sessions/revoke-all", OperationID: "revokeAllLoginSessions", Tag: "Authentication", Summary: "Revoke all login sessions for the current user."},

		// MFA (User)
		{Method: http.MethodGet, Path: "/api/v1/auth/mfa/user/status", OperationID: "fetchUserMFAStatus", Tag: "Authentication", Summary: "Fetch user MFA enrollment status."},
		{Method: http.MethodPost, Path: "/api/v1/auth/mfa/user/setup", OperationID: "setupUserMFA", Tag: "Authentication", Summary: "Start user MFA TOTP enrollment."},
		{Method: http.MethodPost, Path: "/api/v1/auth/mfa/user/enable", OperationID: "enableUserMFA", Tag: "Authentication", Summary: "Confirm and enable user MFA."},
		{Method: http.MethodPost, Path: "/api/v1/auth/mfa/user/disable", OperationID: "disableUserMFA", Tag: "Authentication", Summary: "Disable user MFA.", NoContent: true},

		// MFA (Admin)
		{Method: http.MethodGet, Path: "/api/v1/auth/mfa/admin/status", OperationID: "fetchAdminMFAStatus", Tag: "Authentication", Summary: "Fetch admin MFA status."},
		{Method: http.MethodPost, Path: "/api/v1/auth/mfa/admin/setup", OperationID: "setupAdminMFA", Tag: "Authentication", Summary: "Start admin MFA TOTP enrollment."},
		{Method: http.MethodPost, Path: "/api/v1/auth/mfa/admin/enable", OperationID: "enableAdminMFA", Tag: "Authentication", Summary: "Confirm and enable admin MFA."},
		{Method: http.MethodPost, Path: "/api/v1/auth/mfa/admin/disable", OperationID: "disableAdminMFA", Tag: "Authentication", Summary: "Disable admin MFA.", NoContent: true},
		{Method: http.MethodPost, Path: "/api/v1/auth/mfa/admin/regenerate-recovery-codes", OperationID: "regenerateAdminMFARecoveryCodes", Tag: "Authentication", Summary: "Regenerate admin MFA recovery codes."},

		// Signed Upload URLs
		{Method: http.MethodPost, Path: "/api/v1/uploads/signed-url", OperationID: "createSignedUploadURL", Tag: "Uploads", Summary: "Generate a signed upload URL."},
		{Method: http.MethodPut, Path: "/api/v1/uploads/{uploadID}", OperationID: "uploadFile", Tag: "Uploads", Summary: "Upload a file using a signed URL.", Public: true},
		{Method: http.MethodGet, Path: "/api/v1/uploads/{uploadID}", OperationID: "downloadFile", Tag: "Uploads", Summary: "Download an uploaded file.", Public: true},
		{Method: http.MethodGet, Path: "/api/v1/uploads", OperationID: "fetchUserUploads", Tag: "Uploads", Summary: "List uploads for the current user.", Collection: true},

		// Guests
		{Method: http.MethodGet, Path: "/api/v1/guests", OperationID: "fetchGuests", Tag: "Visitors", Summary: "List guests.", Collection: true},
		{Method: http.MethodGet, Path: "/api/v1/guests/{guestID}", OperationID: "fetchGuest", Tag: "Visitors", Summary: "Fetch a guest."},
		{Method: http.MethodPost, Path: "/api/v1/guests", OperationID: "createGuest", Tag: "Visitors", Summary: "Register a new guest.", Created: true},
		{Method: http.MethodPatch, Path: "/api/v1/guests/{guestID}/status", OperationID: "updateGuestStatus", Tag: "Visitors", Summary: "Update guest status (check-in, check-out, cancel)."},
		{Method: http.MethodDelete, Path: "/api/v1/guests/{guestID}", OperationID: "deleteGuest", Tag: "Visitors", Summary: "Delete a guest record.", NoContent: true},

		// Analytics
		{Method: http.MethodGet, Path: "/api/v1/analytics/access-summary", OperationID: "getAccessSummary", Tag: "Analytics", Summary: "Access event counts by result, door, day, and peak hour."},
		{Method: http.MethodGet, Path: "/api/v1/analytics/door-activity", OperationID: "getDoorActivity", Tag: "Analytics", Summary: "Per-door access count, unique users, and hourly distribution."},
		{Method: http.MethodGet, Path: "/api/v1/analytics/alarm-metrics", OperationID: "getAlarmMetrics", Tag: "Analytics", Summary: "Alarm counts by severity and status with mean resolution time."},

		// Alarm Schedules
		{Method: http.MethodGet, Path: "/api/v1/alarm-schedules", OperationID: "fetchAlarmSchedules", Tag: "Alarm Schedules", Summary: "List alarm schedules.", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/alarm-schedules", OperationID: "createAlarmSchedule", Tag: "Alarm Schedules", Summary: "Create a recurring alarm schedule.", Created: true},
		{Method: http.MethodGet, Path: "/api/v1/alarm-schedules/calendar", OperationID: "getAlarmScheduleCalendar", Tag: "Alarm Schedules", Summary: "Weekly calendar view of all active alarm schedules."},
		{Method: http.MethodGet, Path: "/api/v1/alarm-schedules/{scheduleID}", OperationID: "fetchAlarmSchedule", Tag: "Alarm Schedules", Summary: "Fetch an alarm schedule."},
		{Method: http.MethodPatch, Path: "/api/v1/alarm-schedules/{scheduleID}", OperationID: "updateAlarmSchedule", Tag: "Alarm Schedules", Summary: "Update an alarm schedule."},
		{Method: http.MethodDelete, Path: "/api/v1/alarm-schedules/{scheduleID}", OperationID: "deleteAlarmSchedule", Tag: "Alarm Schedules", Summary: "Delete an alarm schedule.", NoContent: true},

		// Cameras (stubs — awaiting hardware integration)
		{Method: http.MethodGet, Path: "/api/v1/cameras", OperationID: "fetchCameras", Tag: "Cameras", Summary: "List cameras (stub — integration planned).", Collection: true},
		{Method: http.MethodPost, Path: "/api/v1/cameras", OperationID: "createCamera", Tag: "Cameras", Summary: "Register a camera (not yet implemented)."},
		{Method: http.MethodGet, Path: "/api/v1/cameras/{cameraID}", OperationID: "fetchCamera", Tag: "Cameras", Summary: "Fetch a camera (not yet implemented)."},
		{Method: http.MethodDelete, Path: "/api/v1/cameras/{cameraID}", OperationID: "deleteCamera", Tag: "Cameras", Summary: "Delete a camera (not yet implemented).", NoContent: true},

		// Organization Settings
		{Method: http.MethodGet, Path: "/api/v1/organization/settings", OperationID: "fetchOrganizationSettings", Tag: "Organization", Summary: "Fetch organization settings."},
		{Method: http.MethodPatch, Path: "/api/v1/organization/settings", OperationID: "updateOrganizationSettings", Tag: "Organization", Summary: "Update organization settings."},
		{Method: http.MethodPost, Path: "/api/v1/organization/transfer", OperationID: "transferOrganization", Tag: "Organization", Summary: "Transfer all resources from one tenant to another (super_admin only)."},
		{Method: http.MethodPost, Path: "/api/v1/organization/disable", OperationID: "disableOrganization", Tag: "Organization", Summary: "Disable an organization."},
		{Method: http.MethodPost, Path: "/api/v1/organization/export-audit", OperationID: "exportOrganizationAudit", Tag: "Organization", Summary: "Export organization audit log (not yet implemented)."},

		// Gateway Bootstrap (device token auth)
		{Method: http.MethodPost, Path: "/api/v1/gateway/register", OperationID: "registerGatewayBootstrap", Tag: "Gateway Bootstrap", Summary: "Register a new gateway.", Created: true, Public: true},
		{Method: http.MethodPost, Path: "/api/v1/gateway/activate", OperationID: "activateGatewayBootstrap", Tag: "Gateway Bootstrap", Summary: "Activate a registered gateway.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/gateway/heartbeat", OperationID: "gatewayHeartbeat", Tag: "Gateway Bootstrap", Summary: "Send gateway heartbeat.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/gateway/config/pull", OperationID: "pullGatewayConfig", Tag: "Gateway Bootstrap", Summary: "Pull gateway configuration and authorization cache.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/gateway/config/applied", OperationID: "confirmGatewayConfigApplied", Tag: "Gateway Bootstrap", Summary: "Confirm gateway config version applied.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/gateway/events/batch", OperationID: "submitGatewayEventsBatch", Tag: "Gateway Bootstrap", Summary: "Submit gateway events in batch.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/gateway/events/checkpoint", OperationID: "submitGatewayEventsCheckpoint", Tag: "Gateway Bootstrap", Summary: "Submit gateway events checkpoint.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/gateway/verify-credential", OperationID: "verifyGatewayCredential", Tag: "Gateway Bootstrap", Summary: "Verify a credential for access decision.", Public: true},
		{Method: http.MethodPost, Path: "/api/v1/gateway/ota/report", OperationID: "reportGatewayOTAStatus", Tag: "Gateway Bootstrap", Summary: "Report OTA firmware update result.", Public: true},
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

// --- Resource Schemas ---

func openAPIPlaceSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id", "tenant_id", "name"},
		"properties": map[string]any{
			"id":            map[string]any{"type": "string"},
			"resource_type": map[string]any{"type": "string", "enum": []string{"Place"}},
			"tenant_id":     map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"address":       map[string]any{"type": "string"},
			"region":        map[string]any{"type": "string"},
			"status":        map[string]any{"type": "string", "enum": []string{"active", "archived"}},
			"floors_count":  map[string]any{"type": "integer"},
			"locks_count":   map[string]any{"type": "integer"},
			"created_at":    map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPIPlaceCreateSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"name"},
		"properties": map[string]any{
			"tenant_id": map[string]any{"type": "string"},
			"name":      map[string]any{"type": "string"},
			"address":   map[string]any{"type": "string"},
			"region":    map[string]any{"type": "string"},
		},
	}
}

func openAPILockSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id", "name"},
		"properties": map[string]any{
			"id":            map[string]any{"type": "string"},
			"resource_type": map[string]any{"type": "string", "enum": []string{"Lock"}},
			"tenant_id":     map[string]any{"type": "string"},
			"place_id":      map[string]any{"type": "string"},
			"floor_id":      map[string]any{"type": "string"},
			"area_id":       map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"kind":          map[string]any{"type": "string", "enum": []string{"office", "server_room", "parking", "elevator", "turnstile", "generic"}},
			"status":        map[string]any{"type": "string", "enum": []string{"online", "offline", "locked_down"}},
			"controller_id": map[string]any{"type": "string"},
			"created_at":    map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPILockCreateSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"place_id", "name"},
		"properties": map[string]any{
			"tenant_id":  map[string]any{"type": "string"},
			"place_id":   map[string]any{"type": "string"},
			"floor_id":   map[string]any{"type": "string"},
			"area_id":    map[string]any{"type": "string"},
			"name":       map[string]any{"type": "string"},
			"kind":       map[string]any{"type": "string"},
			"status":     map[string]any{"type": "string"},
			"gateway_id": map[string]any{"type": "string"},
		},
	}
}

func openAPIUserSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id", "name", "email"},
		"properties": map[string]any{
			"id":          map[string]any{"type": "string"},
			"tenant_id":   map[string]any{"type": "string"},
			"building_id": map[string]any{"type": "string"},
			"name":        map[string]any{"type": "string"},
			"email":       map[string]any{"type": "string", "format": "email"},
			"role":        map[string]any{"type": "string"},
			"status":      map[string]any{"type": "string", "enum": []string{"active", "inactive", "suspended"}},
			"group_ids":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"sync_source": map[string]any{"type": "string"},
			"created_at":  map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPIUserCreateSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"name", "email"},
		"properties": map[string]any{
			"tenant_id":   map[string]any{"type": "string"},
			"building_id": map[string]any{"type": "string"},
			"name":        map[string]any{"type": "string"},
			"email":       map[string]any{"type": "string", "format": "email"},
			"role":        map[string]any{"type": "string"},
			"status":      map[string]any{"type": "string"},
			"group_ids":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}
}

func openAPIGroupSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id", "name"},
		"properties": map[string]any{
			"id":            map[string]any{"type": "string"},
			"resource_type": map[string]any{"type": "string", "enum": []string{"Group"}},
			"tenant_id":     map[string]any{"type": "string"},
			"place_id":      map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"description":   map[string]any{"type": "string"},
			"created_at":    map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPITeamSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id", "name"},
		"properties": map[string]any{
			"id":            map[string]any{"type": "string"},
			"resource_type": map[string]any{"type": "string", "enum": []string{"Team"}},
			"tenant_id":     map[string]any{"type": "string"},
			"name":          map[string]any{"type": "string"},
			"description":   map[string]any{"type": "string"},
			"created_at":    map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPITeamMembershipSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id", "team_id", "member_type", "member_id"},
		"properties": map[string]any{
			"id":          map[string]any{"type": "string"},
			"team_id":     map[string]any{"type": "string"},
			"member_type": map[string]any{"type": "string", "enum": []string{"User", "Guest"}},
			"member_id":   map[string]any{"type": "string"},
			"created_at":  map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPIRoleSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id", "name", "applies_to"},
		"properties": map[string]any{
			"id":          map[string]any{"type": "string"},
			"name":        map[string]any{"type": "string"},
			"applies_to":  map[string]any{"type": "string", "enum": []string{"Organization", "Place", "Group"}},
			"description": map[string]any{"type": "string"},
			"permissions": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "boolean"}},
			"built_in":    map[string]any{"type": "boolean"},
		},
	}
}

func openAPIRoleAssignmentSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id", "role_id", "applies_to_type", "assignee_type", "assignee_id"},
		"properties": map[string]any{
			"id":                  map[string]any{"type": "string"},
			"tenant_id":          map[string]any{"type": "string"},
			"role_id":            map[string]any{"type": "string"},
			"applies_to_type":    map[string]any{"type": "string", "enum": []string{"Organization", "Place", "Group"}},
			"applies_to_id":      map[string]any{"type": "string"},
			"assignee_type":      map[string]any{"type": "string", "enum": []string{"User", "Team", "Guest"}},
			"assignee_id":        map[string]any{"type": "string"},
			"assignee_email":     map[string]any{"type": "string", "format": "email"},
			"valid_from":         map[string]any{"type": "string", "format": "date-time"},
			"valid_until":        map[string]any{"type": "string", "format": "date-time"},
			"time_windows":       map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/TimeWindow"}},
			"exception_dates":    map[string]any{"type": "array", "items": map[string]any{"type": "string", "format": "date"}},
			"holiday_calendar_id": map[string]any{"type": "string"},
			"reviewed_at":        map[string]any{"type": "string", "format": "date-time"},
			"reviewed_by":        map[string]any{"type": "string"},
			"created_at":         map[string]any{"type": "string", "format": "date-time"},
			"updated_at":         map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPIShareSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id"},
		"properties": map[string]any{
			"id":                  map[string]any{"type": "string"},
			"tenant_id":          map[string]any{"type": "string"},
			"scope_type":         map[string]any{"type": "string", "enum": []string{"area", "door", "group"}},
			"place_id":           map[string]any{"type": "string"},
			"group_id":           map[string]any{"type": "string"},
			"grantee_name":       map[string]any{"type": "string"},
			"grantee_email":      map[string]any{"type": "string", "format": "email"},
			"valid_from":         map[string]any{"type": "string", "format": "date-time"},
			"valid_until":        map[string]any{"type": "string", "format": "date-time"},
			"time_windows":       map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/TimeWindow"}},
			"exception_dates":    map[string]any{"type": "array", "items": map[string]any{"type": "string", "format": "date"}},
			"created_at":         map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPICardSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id", "status"},
		"properties": map[string]any{
			"id":              map[string]any{"type": "string"},
			"tenant_id":       map[string]any{"type": "string"},
			"provider":        map[string]any{"type": "string"},
			"credential_kind": map[string]any{"type": "string", "enum": []string{"google_wallet", "apple_wallet", "physical_card"}},
			"template_id":     map[string]any{"type": "string"},
			"target_type":     map[string]any{"type": "string", "enum": []string{"user", "visitor"}},
			"target_id":       map[string]any{"type": "string"},
			"object_id":       map[string]any{"type": "string"},
			"status":          map[string]any{"type": "string", "enum": []string{"issued", "active", "suspended", "revoked", "expired"}},
			"save_link":       map[string]any{"type": "string", "format": "uri"},
			"uid":             map[string]any{"type": "string"},
			"card_number":     map[string]any{"type": "string"},
			"expires_at":      map[string]any{"type": "string", "format": "date-time"},
			"created_at":      map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPICardAssignmentSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id", "pass_id", "user_id"},
		"properties": map[string]any{
			"id":         map[string]any{"type": "string"},
			"tenant_id":  map[string]any{"type": "string"},
			"pass_id":    map[string]any{"type": "string"},
			"user_id":    map[string]any{"type": "string"},
			"status":     map[string]any{"type": "string"},
			"created_at": map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPIControllerSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id"},
		"properties": map[string]any{
			"id":            map[string]any{"type": "string"},
			"resource_type": map[string]any{"type": "string", "enum": []string{"Controller"}},
			"tenant_id":     map[string]any{"type": "string"},
			"place_id":      map[string]any{"type": "string"},
			"serial":        map[string]any{"type": "string"},
			"firmware":      map[string]any{"type": "string"},
			"status":        map[string]any{"type": "string", "enum": []string{"online", "offline", "provisioning"}},
			"lock_ids":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"created_at":    map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPIReaderSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id"},
		"properties": map[string]any{
			"id":            map[string]any{"type": "string"},
			"resource_type": map[string]any{"type": "string", "enum": []string{"Reader"}},
			"tenant_id":     map[string]any{"type": "string"},
			"controller_id": map[string]any{"type": "string"},
			"place_id":      map[string]any{"type": "string"},
			"serial":        map[string]any{"type": "string"},
			"status":        map[string]any{"type": "string"},
			"created_at":    map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPITerminalSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id"},
		"properties": map[string]any{
			"id":            map[string]any{"type": "string"},
			"resource_type": map[string]any{"type": "string", "enum": []string{"Terminal"}},
			"tenant_id":     map[string]any{"type": "string"},
			"controller_id": map[string]any{"type": "string"},
			"place_id":      map[string]any{"type": "string"},
			"serial":        map[string]any{"type": "string"},
			"status":        map[string]any{"type": "string"},
			"created_at":    map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPIAlertPolicySchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id", "name", "category"},
		"properties": map[string]any{
			"id":                   map[string]any{"type": "string"},
			"resource_type":        map[string]any{"type": "string", "enum": []string{"AlertPolicy"}},
			"tenant_id":            map[string]any{"type": "string"},
			"name":                 map[string]any{"type": "string"},
			"description":          map[string]any{"type": "string"},
			"category":             map[string]any{"type": "string", "enum": []string{"custom", "wallet_jobs", "enterprise_sync_worker"}},
			"trigger":              map[string]any{"type": "string"},
			"severity":             map[string]any{"type": "string", "enum": []string{"critical", "high", "medium", "low"}},
			"condition_expression": map[string]any{"type": "string"},
			"status":               map[string]any{"type": "string", "enum": []string{"active", "inactive"}},
			"enabled":              map[string]any{"type": "boolean"},
			"threshold":            map[string]any{"type": "integer"},
			"window_seconds":       map[string]any{"type": "integer"},
			"cooldown_seconds":     map[string]any{"type": "integer"},
			"channels": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"email":    map[string]any{"type": "boolean"},
					"whatsapp": map[string]any{"type": "boolean"},
				},
			},
			"receiver_groups": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"updated_at":      map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPIHolidayCalendarSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"id", "name"},
		"properties": map[string]any{
			"id":        map[string]any{"type": "string"},
			"tenant_id": map[string]any{"type": "string"},
			"name":      map[string]any{"type": "string"},
			"country":   map[string]any{"type": "string"},
			"entries": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"date", "name"},
					"properties": map[string]any{
						"date":        map[string]any{"type": "string", "format": "date"},
						"name":        map[string]any{"type": "string"},
						"description": map[string]any{"type": "string"},
					},
				},
			},
			"updated_at": map[string]any{"type": "string", "format": "date-time"},
		},
	}
}

func openAPITimeWindowSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"start_time", "end_time", "day_of_week_set"},
		"properties": map[string]any{
			"start_time":     map[string]any{"type": "string", "pattern": "^\\d{2}:\\d{2}$", "description": "HH:MM format"},
			"end_time":       map[string]any{"type": "string", "pattern": "^\\d{2}:\\d{2}$", "description": "HH:MM format"},
			"day_of_week_set": map[string]any{"type": "string", "description": "weekday, weekend, all, or comma-separated ISO days (1=Mon, 7=Sun)"},
			"timezone":       map[string]any{"type": "string", "description": "IANA timezone, e.g. Asia/Jakarta"},
		},
	}
}
