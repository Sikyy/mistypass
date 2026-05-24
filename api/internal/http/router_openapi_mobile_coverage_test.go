package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/config"
)

func TestOpenAPIMobileCoverage(t *testing.T) {
	router, cleanup, err := NewRouter(config.Config{
		JWTSecret:       "openapi-mobile-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	actualRoutes, err := mobileRouteSetFromRouter(router)
	if err != nil {
		t.Fatalf("walk router: %v", err)
	}
	generatedRoutes := mobileRouteSetFromSpec(t, BuildOpenAPISpec())
	if missing := sortedRouteDiff(actualRoutes, generatedRoutes); len(missing) > 0 {
		t.Fatalf("mobile OpenAPI is missing router routes:\n%s", strings.Join(missing, "\n"))
	}
	if stale := sortedRouteDiff(generatedRoutes, actualRoutes); len(stale) > 0 {
		t.Fatalf("mobile OpenAPI documents routes not present in router:\n%s", strings.Join(stale, "\n"))
	}

	mobileSpecBytes, err := os.ReadFile("../../../docs/openapi-mobile.json")
	if err != nil {
		t.Fatalf("read checked-in mobile OpenAPI: %v", err)
	}
	var mobileSpec map[string]any
	if err := json.Unmarshal(mobileSpecBytes, &mobileSpec); err != nil {
		t.Fatalf("decode checked-in mobile OpenAPI: %v", err)
	}
	checkedInRoutes := mobileRouteSetFromSpec(t, mobileSpec)
	if missing := sortedRouteDiff(generatedRoutes, checkedInRoutes); len(missing) > 0 {
		t.Fatalf("checked-in mobile OpenAPI is missing generated routes:\n%s", strings.Join(missing, "\n"))
	}
	if stale := sortedRouteDiff(checkedInRoutes, generatedRoutes); len(stale) > 0 {
		t.Fatalf("checked-in mobile OpenAPI documents stale routes:\n%s", strings.Join(stale, "\n"))
	}
}

func mobileRouteSetFromRouter(handler http.Handler) (map[string]struct{}, error) {
	routes, ok := handler.(chi.Routes)
	if !ok {
		return nil, fmt.Errorf("router does not expose chi routes")
	}
	routeSet := map[string]struct{}{}
	err := chi.Walk(routes, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(route, "/api/v1/app/") {
			return nil
		}
		method = strings.ToUpper(method)
		switch method {
		case http.MethodHead, http.MethodOptions:
			return nil
		}
		routeSet[method+" "+route] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for route := range routeSet {
		method, path, ok := strings.Cut(route, " ")
		if !ok || !strings.HasSuffix(path, "/") || path == "/" {
			continue
		}
		canonicalRoute := method + " " + strings.TrimSuffix(path, "/")
		if _, exists := routeSet[canonicalRoute]; exists {
			delete(routeSet, route)
		}
	}
	return routeSet, nil
}

func mobileRouteSetFromSpec(t *testing.T, spec map[string]any) map[string]struct{} {
	t.Helper()
	paths := mustOpenAPIMap(t, spec, "paths")
	routeSet := map[string]struct{}{}
	for path, value := range paths {
		if !strings.HasPrefix(path, "/api/v1/app/") {
			continue
		}
		pathItem, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("expected OpenAPI path item for %s, got %#v", path, value)
		}
		for method := range pathItem {
			method = strings.ToUpper(method)
			if !isOpenAPIMethod(method) {
				continue
			}
			routeSet[method+" "+path] = struct{}{}
		}
	}
	return routeSet
}

func isOpenAPIMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func sortedRouteDiff(left, right map[string]struct{}) []string {
	diff := make([]string, 0)
	for route := range left {
		if _, ok := right[route]; !ok {
			diff = append(diff, route)
		}
	}
	sort.Strings(diff)
	return diff
}
