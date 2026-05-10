package httpx

import (
	"log/slog"
	"net/http"

	"github.com/mistypass/cloud/api/internal/modules/audit"
)

// POST /gateway/audit/batch
// Receives a batch of offline audit entries from the gateway.
func (s *server) gatewayAuditBatch(w http.ResponseWriter, r *http.Request) {
	record, ok := s.gatewayRecordFromDeviceRequest(w, r)
	if !ok {
		return
	}

	var req struct {
		Entries []audit.OfflineAuditEntry `json:"entries"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Entries) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"gateway_id": record.ID,
			"tenant_id":  record.TenantID,
			"accepted":   0,
		})
		return
	}

	if len(req.Entries) > 500 {
		writeError(w, http.StatusBadRequest, "batch too large, max 500 entries")
		return
	}

	accepted, err := s.auditSvc.IngestOfflineBatchForGateway(r.Context(), audit.OfflineBatchContext{
		TenantID:  record.TenantID,
		GatewayID: record.ID,
	}, req.Entries)
	if err != nil {
		slog.Error("audit batch ingestion failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"gateway_id": record.ID,
		"tenant_id":  record.TenantID,
		"accepted":   accepted,
	})
}
