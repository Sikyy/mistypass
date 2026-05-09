package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/visitor-groups
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appListVisitorGroups(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	// Load visitor groups from state store
	stateKey := "visitor_groups:" + tenantID + ":" + placeID
	var groups []visitorGroup
	if found, _ := s.stateStore.Load(stateKey, &groups); !found {
		groups = []visitorGroup{}
	}

	items := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		items = append(items, map[string]any{
			"id":                  g.ID,
			"name":                g.Name,
			"place_id":            placeID,
			"member_count":        len(g.Members),
			"auto_remove_expired": g.AutoRemoveExpired,
			"created_at":          g.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, items)
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/visitor-groups
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appCreateVisitorGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	var req struct {
		Name              string `json:"name"`
		AutoRemoveExpired string `json:"auto_remove_expired"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	newGroup := visitorGroup{
		ID:                "vg-" + now[14:19] + now[20:23],
		Name:              req.Name,
		PlaceID:           placeID,
		AutoRemoveExpired: req.AutoRemoveExpired == "true",
		CreatedAt:         now,
	}

	stateKey := "visitor_groups:" + tenantID + ":" + placeID
	var groups []visitorGroup
	s.stateStore.Load(stateKey, &groups)
	groups = append(groups, newGroup)
	_ = s.stateStore.Save(stateKey, groups)

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":                  newGroup.ID,
		"name":                newGroup.Name,
		"place_id":            placeID,
		"member_count":        0,
		"auto_remove_expired": newGroup.AutoRemoveExpired,
		"created_at":          newGroup.CreatedAt,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/app/places/{placeId}/visitor-groups/{groupId}/members
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appListVisitorGroupMembers(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	groupID := chi.URLParam(r, "groupId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	stateKey := "visitor_groups:" + tenantID + ":" + placeID
	var groups []visitorGroup
	if found, _ := s.stateStore.Load(stateKey, &groups); !found {
		writeError(w, http.StatusNotFound, "visitor group not found")
		return
	}

	for _, g := range groups {
		if g.ID == groupID {
			items := make([]map[string]any, 0, len(g.Members))
			for _, m := range g.Members {
				items = append(items, map[string]any{
					"id":           m.ID,
					"visitor_id":   m.VisitorID,
					"visitor_name": m.VisitorName,
					"expires_at":   m.ExpiresAt,
					"is_active":    m.IsActive,
				})
			}
			writeJSON(w, http.StatusOK, items)
			return
		}
	}

	writeError(w, http.StatusNotFound, "visitor group not found")
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/v1/app/places/{placeId}/visitor-groups/{groupId}/cleanup-expired
// ─────────────────────────────────────────────────────────────────────────────

func (s *server) appCleanupExpiredVisitors(w http.ResponseWriter, r *http.Request) {
	user, ok := authenticatedUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return
	}

	placeID := chi.URLParam(r, "placeId")
	groupID := chi.URLParam(r, "groupId")
	tenantID := user.TenantID

	if _, err := s.spaceSvc.GetBuilding(tenantID, placeID); err != nil {
		writeError(w, http.StatusNotFound, "place not found")
		return
	}

	stateKey := "visitor_groups:" + tenantID + ":" + placeID
	var groups []visitorGroup
	if found, _ := s.stateStore.Load(stateKey, &groups); !found {
		writeError(w, http.StatusNotFound, "visitor group not found")
		return
	}

	now := time.Now().UTC()
	removed := 0
	for i, g := range groups {
		if g.ID != groupID {
			continue
		}
		active := make([]visitorGroupMember, 0, len(g.Members))
		for _, m := range g.Members {
			expires, err := time.Parse(time.RFC3339, m.ExpiresAt)
			if err != nil || expires.After(now) {
				active = append(active, m)
			} else {
				removed++
			}
		}
		groups[i].Members = active
		if err := s.stateStore.Save(stateKey, groups); err != nil {
			s.loggerOrDefault().Warn("state store save failed", "key", stateKey, "error", err)
		}
		break
	}

	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

// visitorGroup is the state-stored representation.
type visitorGroup struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	PlaceID           string               `json:"place_id"`
	AutoRemoveExpired bool                  `json:"auto_remove_expired"`
	Members           []visitorGroupMember  `json:"members"`
	CreatedAt         string                `json:"created_at"`
}

type visitorGroupMember struct {
	ID          string `json:"id"`
	VisitorID   string `json:"visitor_id"`
	VisitorName string `json:"visitor_name"`
	ExpiresAt   string `json:"expires_at"`
	IsActive    bool   `json:"is_active"`
}
