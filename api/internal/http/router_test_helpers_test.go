package httpx

import (
	"net/http"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

// newTestServerWithHandler creates a full router and returns the underlying server
// for tests that need direct access to services (e.g., event ingestion).
func newTestServerWithHandler(t *testing.T, cfg config.Config) (*server, http.Handler) {
	t.Helper()
	handler, srv, err := newRouterInternal(cfg, nil)
	if err != nil {
		t.Fatalf("newTestServerWithHandler: %v", err)
	}
	return srv, handler
}
