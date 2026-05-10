package httpx

import (
	"log/slog"
	"net/http"
	"strconv"
)

// GET /gateway/credentials/sync?since_version={version}
// Returns credentials changed since the given sync version.
// Used by gateways for incremental credential cache updates.
func (s *server) gatewayCredentialSync(w http.ResponseWriter, r *http.Request) {
	if !s.authorizeGatewayBootstrapToken(w, r) {
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

	creds, err := s.credentialSvc.GetGatewayCredentialSyncSince(sinceVersion)
	if err != nil {
		slog.Error("credential sync failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"credentials": creds,
	})
}
