package httpx

import (
	"fmt"
	"net/http"
)

func (s *server) transferOrganization(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceTenantID string `json:"source_tenant_id"`
		TargetTenantID string `json:"target_tenant_id"`
		Confirm        bool   `json:"confirm"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.SourceTenantID == "" || req.TargetTenantID == "" {
		writeError(w, http.StatusBadRequest, "source_tenant_id and target_tenant_id are required")
		return
	}
	if !req.Confirm {
		writeError(w, http.StatusBadRequest, "confirm must be true to proceed with organization transfer")
		return
	}

	// Verify both tenants exist by fetching their settings
	sourceSettings := s.accessSvc.GetOrganizationSettings(req.SourceTenantID)
	if sourceSettings.Status == "disabled" {
		writeError(w, http.StatusBadRequest, "source organization is disabled")
		return
	}
	if sourceSettings.Status == "transferred" {
		writeError(w, http.StatusBadRequest, "source organization has already been transferred")
		return
	}

	if err := s.accessSvc.TransferOrganization(req.SourceTenantID, req.TargetTenantID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.appendAuditLog(r, req.SourceTenantID, "organization_transferred",
		fmt.Sprintf("source=%s,target=%s", req.SourceTenantID, req.TargetTenantID),
		"access",
	)
	s.appendAuditLog(r, req.TargetTenantID, "organization_received_transfer",
		fmt.Sprintf("source=%s,target=%s", req.SourceTenantID, req.TargetTenantID),
		"access",
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "transferred",
		"source_tenant_id": req.SourceTenantID,
		"target_tenant_id": req.TargetTenantID,
		"message":          "organization resources have been transferred",
	})
}
