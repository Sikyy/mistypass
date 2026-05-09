package httpx

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestOpenAPISpecDocumentsReferenceExtensionsAndErrors(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "openapi-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	recorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/openapi.json", "", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected openapi status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var spec map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &spec); err != nil {
		t.Fatalf("decode openapi spec: %v", err)
	}
	if got := spec["openapi"]; got != "3.0.3" {
		t.Fatalf("expected OpenAPI 3.0.3, got %#v", got)
	}

	paths := mustOpenAPIMap(t, spec, "paths")
	placesGet := mustOpenAPIOperation(t, paths, "/api/v1/places", "get")
	if got := placesGet["operationId"]; got != "fetchPlaces" {
		t.Fatalf("expected places operationId fetchPlaces, got %#v", got)
	}
	assertOpenAPIParameterRef(t, placesGet, "#/components/parameters/LimitParam")
	assertOpenAPIParameterRef(t, placesGet, "#/components/parameters/OffsetParam")
	assertOpenAPICollectionResponse(t, placesGet)
	assertOpenAPIBearerSecurity(t, placesGet)

	doorGroupsGet := mustOpenAPIOperation(t, paths, "/api/v1/door_groups", "get")
	if got := doorGroupsGet["operationId"]; got != "fetchDoorGroups" {
		t.Fatalf("expected door groups operationId fetchDoorGroups, got %#v", got)
	}
	if got := doorGroupsGet["x-mistyislet-extension"]; got != true {
		t.Fatalf("expected door groups extension marker, got %#v", got)
	}
	if got := doorGroupsGet["x-mistyislet-extension-group"]; got != "access" {
		t.Fatalf("expected door groups extension group access, got %#v", got)
	}

	physicalScanPost := mustOpenAPIOperation(t, paths, "/api/v1/wallet/physical-card-inventory/scan", "post")
	if got := physicalScanPost["operationId"]; got != "scanWalletPhysicalCardInventoryItem" {
		t.Fatalf("expected physical scan operationId, got %#v", got)
	}
	if got := physicalScanPost["x-mistyislet-extension-group"]; got != "physical_cards" {
		t.Fatalf("expected physical cards extension group, got %#v", got)
	}

	physicalStatusPatch := mustOpenAPIOperation(t, paths, "/api/v1/wallet/physical-card-inventory/{inventoryID}/status", "patch")
	if got := physicalStatusPatch["operationId"]; got != "updateWalletPhysicalCardInventoryStatus" {
		t.Fatalf("expected physical inventory status operationId, got %#v", got)
	}
	physicalStatusResponses := mustOpenAPIMap(t, physicalStatusPatch, "responses")
	if _, ok := physicalStatusResponses["409"]; !ok {
		t.Fatalf("expected physical inventory status 409 response, got %#v", physicalStatusResponses)
	}

	buildingsGet := mustOpenAPIOperation(t, paths, "/api/v1/buildings", "get")
	if got := buildingsGet["deprecated"]; got != true {
		t.Fatalf("expected legacy buildings deprecated marker, got %#v", got)
	}
	if got := buildingsGet["x-mistyislet-legacy"]; got != true {
		t.Fatalf("expected legacy buildings marker, got %#v", got)
	}
	assertOpenAPIStringSliceContains(t, buildingsGet["x-mistyislet-replacements"], "/api/v1/places")

	components := mustOpenAPIMap(t, spec, "components")
	schemas := mustOpenAPIMap(t, components, "schemas")
	errorSchema := mustOpenAPIMap(t, schemas, "Error")
	assertOpenAPIStringSliceContains(t, errorSchema["required"], "error")
	assertOpenAPIStringSliceContains(t, errorSchema["required"], "message")
	assertOpenAPIStringSliceContains(t, errorSchema["required"], "code")
	assertOpenAPIStringSliceContains(t, errorSchema["required"], "status")

	responses := mustOpenAPIMap(t, components, "responses")
	if _, ok := responses["ConflictError"]; !ok {
		t.Fatalf("expected ConflictError response component, got %#v", responses)
	}
	if _, ok := responses["ValidationError"]; !ok {
		t.Fatalf("expected ValidationError response component, got %#v", responses)
	}
	securitySchemes := mustOpenAPIMap(t, components, "securitySchemes")
	if _, ok := securitySchemes["bearerAuth"]; !ok {
		t.Fatalf("expected bearerAuth security scheme, got %#v", securitySchemes)
	}

	operationIDs := map[string]string{}
	for path, pathValue := range paths {
		pathItem, ok := pathValue.(map[string]any)
		if !ok {
			t.Fatalf("expected path item for %s, got %#v", path, pathValue)
		}
		for method, operationValue := range pathItem {
			operation, ok := operationValue.(map[string]any)
			if !ok {
				t.Fatalf("expected operation for %s %s, got %#v", method, path, operationValue)
			}
			operationID, _ := operation["operationId"].(string)
			if operationID == "" {
				t.Fatalf("expected operationId for %s %s", method, path)
			}
			if previous, exists := operationIDs[operationID]; exists {
				t.Fatalf("duplicate operationId %s for %s %s and %s", operationID, method, path, previous)
			}
			operationIDs[operationID] = method + " " + path
		}
	}
}

func mustOpenAPIMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("expected map at key %s, got %#v", key, parent[key])
	}
	return value
}

func mustOpenAPIOperation(t *testing.T, paths map[string]any, path, method string) map[string]any {
	t.Helper()
	pathItem, ok := paths[path].(map[string]any)
	if !ok {
		t.Fatalf("expected path %s, got %#v", path, paths[path])
	}
	operation, ok := pathItem[method].(map[string]any)
	if !ok {
		t.Fatalf("expected %s operation for %s, got %#v", method, path, pathItem[method])
	}
	return operation
}

func assertOpenAPIParameterRef(t *testing.T, operation map[string]any, ref string) {
	t.Helper()
	parameters, ok := operation["parameters"].([]any)
	if !ok {
		t.Fatalf("expected operation parameters, got %#v", operation["parameters"])
	}
	for _, parameter := range parameters {
		parameterMap, ok := parameter.(map[string]any)
		if ok && parameterMap["$ref"] == ref {
			return
		}
	}
	t.Fatalf("expected parameter ref %s in %#v", ref, parameters)
}

func assertOpenAPICollectionResponse(t *testing.T, operation map[string]any) {
	t.Helper()
	responses := mustOpenAPIMap(t, operation, "responses")
	okResponse := mustOpenAPIMap(t, responses, "200")
	headers := mustOpenAPIMap(t, okResponse, "headers")
	if _, ok := headers["X-Collection-Range"]; !ok {
		t.Fatalf("expected X-Collection-Range header, got %#v", headers)
	}
	content := mustOpenAPIMap(t, okResponse, "content")
	applicationJSON := mustOpenAPIMap(t, content, "application/json")
	schema := mustOpenAPIMap(t, applicationJSON, "schema")
	// accept both: generic $ref CollectionResponse or inline typed schema with items+pagination
	if ref, ok := schema["$ref"]; ok {
		if ref != "#/components/schemas/CollectionResponse" {
			t.Fatalf("expected collection response schema ref, got %#v", ref)
		}
		return
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected collection schema properties, got %#v", schema)
	}
	if _, ok := props["items"]; !ok {
		t.Fatalf("expected items property in collection schema")
	}
	if _, ok := props["pagination"]; !ok {
		t.Fatalf("expected pagination property in collection schema")
	}
}

func assertOpenAPIBearerSecurity(t *testing.T, operation map[string]any) {
	t.Helper()
	security, ok := operation["security"].([]any)
	if !ok || len(security) == 0 {
		t.Fatalf("expected operation security, got %#v", operation["security"])
	}
	first, ok := security[0].(map[string]any)
	if !ok {
		t.Fatalf("expected security map, got %#v", security[0])
	}
	if _, ok := first["bearerAuth"]; !ok {
		t.Fatalf("expected bearerAuth security, got %#v", first)
	}
}

func assertOpenAPIStringSliceContains(t *testing.T, value any, expected string) {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected string slice containing %s, got %#v", expected, value)
	}
	for _, item := range items {
		if item == expected {
			return
		}
	}
	t.Fatalf("expected %#v to contain %s", items, expected)
}
