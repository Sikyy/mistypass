package httpx

import (
	"net/http"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestLegacyEndpointsAdvertiseReferenceReplacements(t *testing.T) {
	router, err := NewRouter(config.Config{
		JWTSecret:       "legacy-deprecation-test-secret",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}
	token := referenceAPILogin(t, router, "organization.admin@mistypass.local")

	tests := []struct {
		name         string
		path         string
		replacements []string
	}{
		{name: "buildings", path: "/api/v1/buildings?tenant_id=tenant_demo_jakarta", replacements: []string{"/api/v1/places"}},
		{name: "doors", path: "/api/v1/doors?tenant_id=tenant_demo_jakarta", replacements: []string{"/api/v1/locks"}},
		{name: "door-groups", path: "/api/v1/door-groups?tenant_id=tenant_demo_jakarta", replacements: []string{"/api/v1/door_groups"}},
		{name: "gateways", path: "/api/v1/gateways?tenant_id=tenant_demo_jakarta", replacements: []string{"/api/v1/controllers", "/api/v1/readers", "/api/v1/terminals"}},
		{name: "access-policies", path: "/api/v1/access-policies?tenant_id=tenant_demo_jakarta", replacements: []string{"/api/v1/role_assignments", "/api/v1/groups", "/api/v1/group_locks"}},
		{name: "temporary-access", path: "/api/v1/temporary-access?tenant_id=tenant_demo_jakarta", replacements: []string{"/api/v1/shares"}},
		{name: "events-access", path: "/api/v1/events/access?tenant_id=tenant_demo_jakarta", replacements: []string{"/api/v1/event_sets"}},
		{name: "wallet-passes", path: "/api/v1/wallet/passes?tenant_id=tenant_demo_jakarta", replacements: []string{"/api/v1/cards", "/api/v1/card_assignments"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := referenceAPIRequest(t, router, http.MethodGet, tt.path, token, nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("expected legacy endpoint status 200, got %d body=%s", recorder.Code, recorder.Body.String())
			}
			if got := recorder.Header().Get("Deprecation"); got != "true" {
				t.Fatalf("expected Deprecation header true, got %q", got)
			}
			if got, want := recorder.Header().Get("X-MistyPass-Replacement"), strings.Join(tt.replacements, ", "); got != want {
				t.Fatalf("expected replacement %q, got %q", want, got)
			}
			linkHeaders := strings.Join(recorder.Header().Values("Link"), ", ")
			for _, replacement := range tt.replacements {
				if !strings.Contains(linkHeaders, "<"+replacement+">") {
					t.Fatalf("expected successor Link header for %q, got %q", replacement, linkHeaders)
				}
			}
			if !strings.Contains(linkHeaders, `rel="successor-version"`) {
				t.Fatalf("expected successor Link rel, got %q", linkHeaders)
			}
		})
	}

	placesRecorder := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/places?tenant_id=tenant_demo_jakarta", token, nil)
	if placesRecorder.Code != http.StatusOK {
		t.Fatalf("expected places status 200, got %d body=%s", placesRecorder.Code, placesRecorder.Body.String())
	}
	if got := placesRecorder.Header().Get("Deprecation"); got != "" {
		t.Fatalf("expected reference endpoint not to be marked deprecated, got %q", got)
	}
}
