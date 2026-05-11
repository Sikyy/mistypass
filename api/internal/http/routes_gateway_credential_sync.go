package httpx

import (
	"net/http"
	"strconv"
)

// GET /gateway/credentials/sync?tenant_id={tenant}&gateway_id={gateway}&since_version={version}
// Returns scoped credential changes since the given sync version.
// Used by gateways for incremental credential cache updates.
func (s *server) gatewayCredentialSync(w http.ResponseWriter, r *http.Request) {
	record, ok := s.gatewayRecordFromDeviceRequest(w, r)
	if !ok {
		return
	}

	sinceVersion := int64(0)
	if v := r.URL.Query().Get("since_version"); v != "" {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since_version")
			return
		}
		sinceVersion = parsed
	}

	userLockAccess := s.buildGatewayUserLockAccess(record.TenantID, record.BoundDoorIDs)
	creds := s.credentialSvc.BuildGatewayCredentialSyncSince(record.TenantID, record.BoundDoorIDs, userLockAccess, sinceVersion)

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id":       record.ID,
		"tenant_id":        record.TenantID,
		"bound_door_ids":   record.BoundDoorIDs,
		"credentials":      creds,
		"credential_count": len(creds),
	})
}
