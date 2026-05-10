package httpx

import (
	"net/http"
	"strings"

	"github.com/mistypass/cloud/api/internal/modules/gateway"
)

func (s *server) gatewayRecordFromDeviceRequest(w http.ResponseWriter, r *http.Request) (gateway.Gateway, bool) {
	gatewayID := strings.TrimSpace(r.URL.Query().Get("gateway_id"))
	if gatewayID == "" {
		gatewayID = strings.TrimSpace(r.Header.Get("X-Gateway-ID"))
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	if tenantID == "" {
		tenantID = strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	}
	if gatewayID == "" || tenantID == "" {
		writeError(w, http.StatusBadRequest, "gateway_id and tenant_id are required")
		return gateway.Gateway{}, false
	}

	record, ok := s.findGatewayByTenant(tenantID, gatewayID)
	if !ok {
		writeError(w, http.StatusNotFound, "gateway not found")
		return gateway.Gateway{}, false
	}
	if !s.authorizeGatewayDeviceToken(w, r, record.ID) {
		return gateway.Gateway{}, false
	}
	return record, true
}
