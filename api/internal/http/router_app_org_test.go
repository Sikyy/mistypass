package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/mistypass/cloud/api/internal/config"
	"github.com/mistypass/cloud/api/internal/state"
)

// ─────────────────────────────────────────────────────────────────────────────
// In-memory stubs for orgMembershipStore and magicLinkStore interfaces
// ─────────────────────────────────────────────────────────────────────────────

type memOrgStore struct {
	mu          sync.Mutex
	memberships []state.OrgMembership
}

func (m *memOrgStore) ListUserOrgMemberships(userID string) ([]state.OrgMembership, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []state.OrgMembership
	for _, mem := range m.memberships {
		if mem.UserID == userID {
			result = append(result, mem)
		}
	}
	return result, nil
}

func (m *memOrgStore) GetUserOrgMembership(userID, tenantID string) (state.OrgMembership, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mem := range m.memberships {
		if mem.UserID == userID && mem.TenantID == tenantID {
			return mem, true, nil
		}
	}
	return state.OrgMembership{}, false, nil
}

func (m *memOrgStore) UpdateOrgMembershipLastUsed(userID, tenantID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for i, mem := range m.memberships {
		if mem.UserID == userID && mem.TenantID == tenantID {
			m.memberships[i].LastUsedAt = &now
			return nil
		}
	}
	return nil
}

type memMagicLinkStore struct {
	mu     sync.Mutex
	tokens map[string]string    // token → email
	expiry map[string]time.Time // token → expiresAt
}

func newMemMagicLinkStore() *memMagicLinkStore {
	return &memMagicLinkStore{
		tokens: make(map[string]string),
		expiry: make(map[string]time.Time),
	}
}

func (m *memMagicLinkStore) CreateMagicLinkToken(email, token string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[token] = email
	m.expiry[token] = expiresAt
	return nil
}

func (m *memMagicLinkStore) VerifyMagicLinkToken(token string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	email, ok := m.tokens[token]
	if !ok {
		return "", fmt.Errorf("invalid or expired magic link token")
	}
	if time.Now().UTC().After(m.expiry[token]) {
		delete(m.tokens, token)
		delete(m.expiry, token)
		return "", fmt.Errorf("magic link token has expired")
	}
	// Consume the token (one-time use)
	delete(m.tokens, token)
	delete(m.expiry, token)
	return email, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper: create a test server with injected org + magic-link stores
// ─────────────────────────────────────────────────────────────────────────────

func newOrgTestServer(t *testing.T, memberships []state.OrgMembership) (*server, http.Handler) {
	t.Helper()
	srv, handler := newTestServerWithHandler(t, config.Config{
		JWTSecret:       "app-org-test-" + t.Name(),
		EnableDemoUsers: true,
	})
	srv.orgStore = &memOrgStore{memberships: memberships}
	srv.magicLinkStore = newMemMagicLinkStore()
	return srv, handler
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

// TestAppOrgList verifies that GET /app/orgs returns the orgs a user belongs to.
func TestAppOrgList(t *testing.T) {
	now := time.Now().UTC()
	_, handler := newOrgTestServer(t, []state.OrgMembership{
		{
			ID:       "mem_001",
			UserID:   "usr_resident_jkt_001",
			TenantID: "tenant_demo_jakarta",
			Role:     "resident",
			JoinedAt: now,
		},
	})

	token := referenceAPILogin(t, handler, "resident.jakarta@mistypass.local")

	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/orgs", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var orgs []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &orgs); err != nil {
		t.Fatalf("decode orgs: %v body=%s", err, rec.Body.String())
	}
	if len(orgs) < 1 {
		t.Fatalf("expected at least 1 org, got %d", len(orgs))
	}

	found := false
	for _, org := range orgs {
		if org.ID == "tenant_demo_jakarta" {
			found = true
			if org.Role != "resident" {
				t.Errorf("expected role=resident, got %q", org.Role)
			}
			if org.Name == "" {
				t.Errorf("expected org name to be populated")
			}
		}
	}
	if !found {
		t.Errorf("expected tenant_demo_jakarta in org list, got %+v", orgs)
	}
}

func TestAppOrgListFallsBackToAuthenticatedTenant(t *testing.T) {
	_, handler := newOrgTestServer(t, nil)

	token := referenceAPILogin(t, handler, "tenant.admin@sudirman.co")

	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/orgs", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var orgs []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &orgs); err != nil {
		t.Fatalf("decode orgs: %v body=%s", err, rec.Body.String())
	}
	if len(orgs) != 1 {
		t.Fatalf("expected fallback org, got %d: %+v", len(orgs), orgs)
	}
	if orgs[0].ID != "tenant_demo_jakarta" {
		t.Fatalf("expected tenant_demo_jakarta, got %q", orgs[0].ID)
	}
	if orgs[0].Role != "tenant_admin" {
		t.Fatalf("expected tenant_admin role, got %q", orgs[0].Role)
	}
	if orgs[0].Name == "" {
		t.Fatalf("expected org name to be populated")
	}
}

// TestAppOrgSwitch verifies that POST /app/orgs/{orgId}/switch issues new tokens.
func TestAppOrgSwitch(t *testing.T) {
	now := time.Now().UTC()
	_, handler := newOrgTestServer(t, []state.OrgMembership{
		{
			ID:       "mem_001",
			UserID:   "usr_resident_jkt_001",
			TenantID: "tenant_demo_jakarta",
			Role:     "resident",
			JoinedAt: now,
		},
	})

	token := referenceAPILogin(t, handler, "resident.jakarta@mistypass.local")

	rec := referenceAPIRequest(t, handler, http.MethodPost, "/api/v1/app/orgs/tenant_demo_jakarta/switch", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		OrgID        string `json:"org_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode switch response: %v", err)
	}
	if result.AccessToken == "" {
		t.Errorf("expected access_token in switch response")
	}
	if result.RefreshToken == "" {
		t.Errorf("expected refresh_token in switch response")
	}
	if result.OrgID != "tenant_demo_jakarta" {
		t.Errorf("expected org_id=tenant_demo_jakarta, got %q", result.OrgID)
	}
	if result.ExpiresIn <= 0 {
		t.Errorf("expected positive expires_in, got %d", result.ExpiresIn)
	}

	// Verify the new token works for authenticated requests
	rec2 := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/me", result.AccessToken, nil)
	if rec2.Code != http.StatusOK {
		t.Fatalf("new token should be valid for /app/me, got %d body=%s", rec2.Code, rec2.Body.String())
	}
}

func TestAppOrgSwitchFallsBackToAuthenticatedTenant(t *testing.T) {
	_, handler := newOrgTestServer(t, nil)

	token := referenceAPILogin(t, handler, "tenant.admin@sudirman.co")

	rec := referenceAPIRequest(t, handler, http.MethodPost, "/api/v1/app/orgs/tenant_demo_jakarta/switch", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var result struct {
		AccessToken string `json:"access_token"`
		OrgID       string `json:"org_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode switch response: %v", err)
	}
	if result.AccessToken == "" {
		t.Fatalf("expected access_token in switch response")
	}
	if result.OrgID != "tenant_demo_jakarta" {
		t.Fatalf("expected org_id=tenant_demo_jakarta, got %q", result.OrgID)
	}
}

// TestAppOrgSwitchUnauthorized verifies that switching to an org
// the user doesn't belong to returns 403.
func TestAppOrgSwitchUnauthorized(t *testing.T) {
	now := time.Now().UTC()
	_, handler := newOrgTestServer(t, []state.OrgMembership{
		{
			ID:       "mem_001",
			UserID:   "usr_resident_jkt_001",
			TenantID: "tenant_demo_jakarta",
			Role:     "resident",
			JoinedAt: now,
		},
		// No membership for tenant_demo_factory
	})

	token := referenceAPILogin(t, handler, "resident.jakarta@mistypass.local")

	rec := referenceAPIRequest(t, handler, http.MethodPost, "/api/v1/app/orgs/tenant_demo_factory/switch", token, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-member org switch, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestAppPlaceList verifies that GET /app/orgs/{orgId}/places returns places
// for the user's org.
func TestAppPlaceList(t *testing.T) {
	now := time.Now().UTC()
	_, handler := newOrgTestServer(t, []state.OrgMembership{
		{
			ID:       "mem_001",
			UserID:   "usr_resident_jkt_001",
			TenantID: "tenant_demo_jakarta",
			Role:     "resident",
			JoinedAt: now,
		},
	})

	token := referenceAPILogin(t, handler, "resident.jakarta@mistypass.local")

	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/orgs/tenant_demo_jakarta/places", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var places []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Address   string `json:"address"`
		OrgID     string `json:"org_id"`
		DoorCount int    `json:"door_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &places); err != nil {
		t.Fatalf("decode places: %v body=%s", err, rec.Body.String())
	}

	if len(places) == 0 {
		t.Fatalf("expected at least 1 place")
	}

	// Verify all places belong to the correct org
	for _, place := range places {
		if place.OrgID != "tenant_demo_jakarta" {
			t.Errorf("expected org_id=tenant_demo_jakarta, got %q for place %s", place.OrgID, place.ID)
		}
		if place.Name == "" {
			t.Errorf("expected name for place %s", place.ID)
		}
	}
}

// TestAppPlaceDoors verifies that GET /app/places/{placeId}/doors returns
// doors scoped to the given place (building).
func TestAppPlaceDoors(t *testing.T) {
	_, handler := newOrgTestServer(t, nil)

	token := referenceAPILogin(t, handler, "resident.jakarta@mistypass.local")

	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/places/building_demo_001/doors", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var result struct {
		Items []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			BuildingID    string `json:"building_id"`
			GatewayStatus string `json:"gateway_status"`
			CanUnlock     bool   `json:"can_unlock"`
		} `json:"items"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode doors: %v body=%s", err, rec.Body.String())
	}

	if result.Pagination.Total == 0 {
		t.Fatalf("expected at least 1 door in building_demo_001")
	}

	// All doors should belong to building_demo_001
	for _, door := range result.Items {
		if door.BuildingID != "building_demo_001" {
			t.Errorf("expected building_id=building_demo_001, got %q for door %s", door.BuildingID, door.ID)
		}
		if door.Name == "" {
			t.Errorf("expected name for door %s", door.ID)
		}
	}
}

// TestAppPlaceDoors_WrongTenant verifies that a user cannot list doors
// for a place belonging to a different tenant.
func TestAppPlaceDoors_WrongTenant(t *testing.T) {
	_, handler := newOrgTestServer(t, nil)

	// resident.jakarta belongs to tenant_demo_jakarta
	token := referenceAPILogin(t, handler, "resident.jakarta@mistypass.local")

	// building_demo_003 belongs to tenant_demo_factory
	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/places/building_demo_003/doors", token, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-tenant place, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestAppBackwardCompat verifies that the old /app/access/my-doors endpoint
// still works with a regular token (no org switch needed).
func TestAppBackwardCompat(t *testing.T) {
	router, _, err := NewRouter(config.Config{
		JWTSecret:       "app-backward-compat-test",
		EnableDemoUsers: true,
	}, nil)
	if err != nil {
		t.Fatalf("expected router: %v", err)
	}

	token := referenceAPILogin(t, router, "resident.jakarta@mistypass.local")

	rec := referenceAPIRequest(t, router, http.MethodGet, "/api/v1/app/access/my-doors", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for backward-compat my-doors, got %d body=%s", rec.Code, rec.Body.String())
	}

	var result struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode my-doors: %v body=%s", err, rec.Body.String())
	}

	if result.Pagination.Total == 0 {
		t.Fatalf("expected at least 1 door from backward-compat endpoint")
	}
}

// TestAppMagicLinkFlow verifies the magic link request+verify flow
// produces a valid access token.
func TestAppMagicLinkFlow(t *testing.T) {
	srv, handler := newOrgTestServer(t, nil)
	mlStore := srv.magicLinkStore.(*memMagicLinkStore)

	// Step 1: Request a magic link
	body := []byte(`{"email":"resident.jakarta@mistypass.local"}`)
	rec := referenceAPIRequest(t, handler, http.MethodPost, "/api/v1/app/auth/magic-link", "", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("request magic link: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var reqResult struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &reqResult)
	if reqResult.Message == "" {
		t.Errorf("expected message in magic link response")
	}

	// Step 2: Retrieve the token from the in-memory store (simulating
	// the user clicking the magic link in their email).
	mlStore.mu.Lock()
	var magicToken string
	for tok := range mlStore.tokens {
		magicToken = tok
		break
	}
	mlStore.mu.Unlock()

	if magicToken == "" {
		t.Fatalf("expected magic link token to be stored")
	}

	// Step 3: Verify the magic link token
	verifyBody := []byte(fmt.Sprintf(`{"token":"%s"}`, magicToken))
	rec = referenceAPIRequest(t, handler, http.MethodPost, "/api/v1/app/auth/magic-link/verify", "", verifyBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify magic link: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var verifyResult struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		User         struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &verifyResult); err != nil {
		t.Fatalf("decode verify response: %v", err)
	}
	if verifyResult.AccessToken == "" {
		t.Errorf("expected access_token from magic link verify")
	}
	if verifyResult.User.Email != "resident.jakarta@mistypass.local" {
		t.Errorf("expected email=resident.jakarta@mistypass.local, got %q", verifyResult.User.Email)
	}

	// Step 4: Verify the token is consumed (second verify should fail)
	rec = referenceAPIRequest(t, handler, http.MethodPost, "/api/v1/app/auth/magic-link/verify", "", verifyBody)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for reused magic link token, got %d", rec.Code)
	}
}

// TestAppOrgLookup verifies that GET /app/auth/org-lookup returns org info
// when queried by tenant name or ID.
func TestAppOrgLookup(t *testing.T) {
	_, handler := newOrgTestServer(t, nil)

	// Lookup by tenant ID (case-insensitive match)
	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/auth/org-lookup?domain=tenant_demo_jakarta", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var result struct {
		OrgID   string   `json:"org_id"`
		Domain  string   `json:"domain"`
		Name    string   `json:"name"`
		Methods []string `json:"methods"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode org lookup: %v body=%s", err, rec.Body.String())
	}
	if result.OrgID != "tenant_demo_jakarta" {
		t.Errorf("expected org_id=tenant_demo_jakarta, got %q", result.OrgID)
	}
	if result.Name == "" {
		t.Errorf("expected name for org lookup result")
	}
	if len(result.Methods) == 0 {
		t.Errorf("expected at least one auth method")
	}
}

// TestAppOrgLookup_NotFound verifies that lookup for a non-existent domain
// returns 404.
func TestAppOrgLookup_NotFound(t *testing.T) {
	_, handler := newOrgTestServer(t, nil)

	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/auth/org-lookup?domain=nonexistent.org", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestAppOrgLookup_MissingDomain verifies that lookup without a domain param
// returns 400.
func TestAppOrgLookup_MissingDomain(t *testing.T) {
	_, handler := newOrgTestServer(t, nil)

	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/auth/org-lookup", "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestAppOrgList_MultipleOrgs verifies that a user with memberships in multiple
// orgs sees all of them.
func TestAppOrgList_MultipleOrgs(t *testing.T) {
	now := time.Now().UTC()
	_, handler := newOrgTestServer(t, []state.OrgMembership{
		{
			ID:       "mem_001",
			UserID:   "usr_resident_jkt_001",
			TenantID: "tenant_demo_jakarta",
			Role:     "resident",
			JoinedAt: now,
		},
		{
			ID:       "mem_002",
			UserID:   "usr_resident_jkt_001",
			TenantID: "tenant_demo_factory",
			Role:     "tenant_admin",
			JoinedAt: now,
		},
	})

	token := referenceAPILogin(t, handler, "resident.jakarta@mistypass.local")

	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/orgs", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var orgs []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &orgs); err != nil {
		t.Fatalf("decode orgs: %v", err)
	}
	if len(orgs) != 2 {
		t.Fatalf("expected 2 orgs, got %d: %+v", len(orgs), orgs)
	}

	// Verify both orgs are present with correct roles
	orgMap := map[string]string{}
	for _, org := range orgs {
		orgMap[org.ID] = org.Role
	}
	if orgMap["tenant_demo_jakarta"] != "resident" {
		t.Errorf("expected role=resident for jakarta, got %q", orgMap["tenant_demo_jakarta"])
	}
	if orgMap["tenant_demo_factory"] != "tenant_admin" {
		t.Errorf("expected role=tenant_admin for factory, got %q", orgMap["tenant_demo_factory"])
	}
}

// TestAppOrgSwitch_ThenListPlaces verifies the full flow:
// login → list orgs → switch org → list places in the new org.
func TestAppOrgSwitch_ThenListPlaces(t *testing.T) {
	now := time.Now().UTC()
	_, handler := newOrgTestServer(t, []state.OrgMembership{
		{
			ID:       "mem_001",
			UserID:   "usr_resident_jkt_001",
			TenantID: "tenant_demo_jakarta",
			Role:     "resident",
			JoinedAt: now,
		},
		{
			ID:       "mem_002",
			UserID:   "usr_resident_jkt_001",
			TenantID: "tenant_demo_factory",
			Role:     "tenant_admin",
			JoinedAt: now,
		},
	})

	// Step 1: Login as Jakarta resident
	token := referenceAPILogin(t, handler, "resident.jakarta@mistypass.local")

	// Step 2: List orgs
	rec := referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/orgs", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list orgs: expected 200, got %d", rec.Code)
	}

	// Step 3: Switch to factory org
	rec = referenceAPIRequest(t, handler, http.MethodPost, "/api/v1/app/orgs/tenant_demo_factory/switch", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("switch org: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var switchResult struct {
		AccessToken string `json:"access_token"`
		OrgID       string `json:"org_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &switchResult)
	newToken := switchResult.AccessToken
	if newToken == "" {
		t.Fatalf("expected new access_token after org switch")
	}

	// Step 4: List places in the factory org using the new token
	rec = referenceAPIRequest(t, handler, http.MethodGet, "/api/v1/app/orgs/tenant_demo_factory/places", newToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list places: expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var places []struct {
		ID    string `json:"id"`
		OrgID string `json:"org_id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &places)
	if len(places) == 0 {
		t.Fatalf("expected at least 1 place in factory org")
	}
	for _, place := range places {
		if place.OrgID != "tenant_demo_factory" {
			t.Errorf("expected org_id=tenant_demo_factory, got %q", place.OrgID)
		}
	}
}
