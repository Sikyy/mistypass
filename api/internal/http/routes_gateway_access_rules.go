package httpx

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// previewGatewayAccessRules returns the access rule cache for a gateway,
// allowing admins to preview what credentials can open which doors.
func (s *server) previewGatewayAccessRules(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}
	gatewayID := chi.URLParam(r, "gatewayID")
	gw, found := s.gatewaySvc.FindGatewayByID(tenantID, gatewayID)
	if !found {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}

	now := time.Now().UTC()
	cache := s.buildGatewayConfigAuthzCache(tenantID, gw.ID, gw.BoundDoorIDs, now)

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id":    gw.ID,
		"tenant_id":     tenantID,
		"bound_door_ids": gw.BoundDoorIDs,
		"generated_at":  now.Format(time.RFC3339),
		"access_rules":  cache.AccessRules,
		"counts": map[string]int{
			"access_rules": len(cache.AccessRules),
			"users":        cache.Counts.Users,
			"doors":        cache.Counts.Doors,
		},
	})
}
