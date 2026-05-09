package httpx

import (
	"fmt"
	"hash/crc32"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mistypass/cloud/api/internal/modules/auth"
)

func kisiNumericID(id string) int64 {
	return int64(crc32.ChecksumIEEE([]byte(id)))
}

// GET /organizations/{domain}/public — Kisi-compatible org lookup
func (s *server) kisiOrgPublic(w http.ResponseWriter, r *http.Request) {
	domain := strings.TrimSpace(strings.ToLower(chi.URLParam(r, "domain")))
	if domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}

	normalize := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, "_", ""), "-", "")
	}
	normalizedDomain := normalize(domain)

	tenants := s.tenantSvc.List()
	for _, t := range tenants {
		tenantID := strings.ToLower(t.ID)
		tenantName := strings.ToLower(t.Name)
		if tenantID == domain || tenantName == domain || normalize(tenantID) == normalizedDomain || normalize(tenantName) == normalizedDomain {
			buildings := s.spaceSvc.ListBuildings(t.ID)
			writeJSON(w, http.StatusOK, map[string]any{
				"id":                               int(kisiNumericID(t.ID)),
				"name":                             t.Name,
				"address":                          "",
				"domain":                           t.ID,
				"logo":                             "",
				"color":                            "#4A69FF",
				"places_count":                     len(buildings),
				"time_zone":                        "Asia/Jakarta",
				"sso_flow_enabled":                 false,
				"web_authentication_api_enabled":    false,
			})
			return
		}
	}

	writeError(w, http.StatusNotFound, "organization not found")
}

// POST /logins — Kisi-compatible classic login
func (s *server) kisiLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login struct {
			Type            string `json:"type"`
			OsName          string `json:"os_name"`
			DeviceBrand     string `json:"device_brand"`
			DeviceModel     string `json:"device_model"`
			ResolutionToken string `json:"resolution_token"`
			DeviceUID       string `json:"device_uid"`
			Expire          bool   `json:"expire"`
		} `json:"login"`
		User struct {
			Domain   string `json:"domain"`
			Email    string `json:"email"`
			Password string `json:"password"`
		} `json:"user"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	email := strings.TrimSpace(req.User.Email)
	password := req.User.Password
	domain := strings.TrimSpace(req.User.Domain)

	if email == "" || password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	// Find tenant by domain
	var tenantID string
	normalize := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, "_", ""), "-", "")
	}
	normalizedDomain := normalize(strings.ToLower(domain))
	for _, t := range s.tenantSvc.List() {
		tid := strings.ToLower(t.ID)
		if tid == strings.ToLower(domain) || normalize(tid) == normalizedDomain {
			tenantID = t.ID
			break
		}
	}
	if tenantID == "" && domain != "" {
		writeError(w, http.StatusUnauthorized, "organization not found")
		return
	}

	response, err := s.authService.Login(auth.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	now := time.Now().UTC()
	writeJSON(w, http.StatusOK, map[string]any{
		"id":               kisiNumericID(response.User.ID),
		"name":             req.Login.DeviceModel,
		"primary":          true,
		"last_used_at":     now.Format(time.RFC3339),
		"device_brand":     req.Login.DeviceBrand,
		"device_model":     req.Login.DeviceModel,
		"secret":           response.AccessToken,
		"scram_credentials": nil,
		"user": map[string]any{
			"id":    kisiNumericID(response.User.ID),
			"email": response.User.Email,
			"name":  response.User.Name,
		},
	})
}

// GET /login — Get current login session
func (s *server) kisiGetLogin(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":               kisiNumericID(user.ID),
		"name":             "Android Device",
		"primary":          true,
		"last_used_at":     time.Now().UTC().Format(time.RFC3339),
		"device_brand":     "Xiaomi",
		"device_model":     "15",
		"secret":           "",
		"scram_credentials": nil,
		"user": map[string]any{
			"id":    kisiNumericID(user.ID),
			"email": user.Email,
			"name":  user.Name,
		},
	})
}

// GET /organization — Get current organization
func (s *server) kisiGetOrganization(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	tenantID := user.TenantID
	tenants := s.tenantSvc.List()
	for _, t := range tenants {
		if t.ID == tenantID {
			buildings := s.spaceSvc.ListBuildings(tenantID)
			writeJSON(w, http.StatusOK, map[string]any{
				"id":                               int(kisiNumericID(t.ID)),
				"name":                             t.Name,
				"address":                          "",
				"domain":                           t.ID,
				"logo":                             "",
				"color":                            "#4A69FF",
				"places_count":                     len(buildings),
				"time_zone":                        "Asia/Jakarta",
				"sso_flow_enabled":                 false,
				"web_authentication_api_enabled":    false,
			})
			return
		}
	}

	writeError(w, http.StatusNotFound, "organization not found")
}

// GET /places — Kisi-compatible list places
func (s *server) kisiListPlaces(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	tenantID := user.TenantID
	buildings := s.spaceSvc.ListBuildings(tenantID)

	items := make([]map[string]any, 0, len(buildings))
	for _, b := range buildings {
		items = append(items, map[string]any{
			"id":           placeNumericID(b.ID),
			"name":         b.Name,
			"description":  nil,
			"address":      b.Address,
			"image":        nil,
			"logo":         nil,
			"tz_time_zone": nil,
			"latitude":     nil,
			"longitude":    nil,
			"locked_down":  false,
			"favorite":     false,
		})
	}

	writeJSON(w, http.StatusOK, items)
}

// GET /places/{id} — Kisi-compatible get place
func (s *server) kisiGetPlace(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	tenantID := user.TenantID
	buildings := s.spaceSvc.ListBuildings(tenantID)

	placeIDParam := chi.URLParam(r, "id")
	for _, b := range buildings {
		if strings.EqualFold(placeIDParam, b.ID) || placeIDParam == fmt.Sprintf("%d", placeNumericID(b.ID)) {
			writeJSON(w, http.StatusOK, map[string]any{
				"id":           placeNumericID(b.ID),
				"name":         b.Name,
				"description":  nil,
				"address":      b.Address,
				"image":        nil,
				"logo":         nil,
				"tz_time_zone": nil,
				"latitude":     nil,
				"longitude":    nil,
				"locked_down":  false,
				"favorite":     false,
			})
			return
		}
	}

	writeError(w, http.StatusNotFound, "place not found")
}
