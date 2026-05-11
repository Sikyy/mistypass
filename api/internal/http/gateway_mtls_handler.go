package httpx

import (
	"context"
	"net/http"
	"strings"
)

type gatewayMTLSRequiredContextKey struct{}

var gatewayMTLSPathAllowlist = map[string]struct{}{
	"/api/v1/gateway/register":          {},
	"/api/v1/gateway/heartbeat":         {},
	"/api/v1/gateway/status":            {},
	"/api/v1/gateway/config/pull":       {},
	"/api/v1/gateway/config/applied":    {},
	"/api/v1/gateway/events/access":     {},
	"/api/v1/gateway/events/device":     {},
	"/api/v1/gateway/events/batch":      {},
	"/api/v1/gateway/events/checkpoint": {},
	"/api/v1/gateway/verify-credential": {},
	"/api/v1/gateway/ota/report":        {},
	"/api/v1/gateway/credentials/sync":  {},
	"/api/v1/gateway/audit/batch":       {},
	"/api/v1/gateway/cert/renew":        {},
	"/api/v1/gateway/ws":                {},
}

// GatewayMTLSOnlyHandler limits a dedicated mTLS listener to gateway endpoints.
// Initial registration is allowed with bootstrap-token auth so a device can
// obtain its first client certificate. All other gateway routes must present a
// TLS client cert verified by the listener's ClientCAs.
func GatewayMTLSOnlyHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL == nil || !gatewayMTLSPathAllowed(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/api/v1/gateway/register" {
			next.ServeHTTP(w, r)
			return
		}
		if verifiedGatewayClientCertificate(r) == nil {
			writeError(w, http.StatusUnauthorized, "valid gateway client certificate required")
			return
		}
		ctx := context.WithValue(r.Context(), gatewayMTLSRequiredContextKey{}, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func gatewayMTLSPathAllowed(path string) bool {
	_, ok := gatewayMTLSPathAllowlist[strings.TrimRight(path, "/")]
	return ok
}

func gatewayMTLSRequired(r *http.Request) bool {
	if r == nil {
		return false
	}
	required, _ := r.Context().Value(gatewayMTLSRequiredContextKey{}).(bool)
	return required
}
