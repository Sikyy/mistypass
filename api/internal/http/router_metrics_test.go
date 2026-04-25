package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mistypass/cloud/api/internal/config"
)

func TestMetricsEndpointExposed(t *testing.T) {
	router, err := NewRouter(config.Config{}, nil)
	if err != nil {
		t.Fatalf("expected router init success: %v", err)
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRec := httptest.NewRecorder()
	router.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("expected healthz status %d, got %d", http.StatusOK, healthRec.Code)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	router.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("expected metrics status %d, got %d", http.StatusOK, metricsRec.Code)
	}

	metricsBody := metricsRec.Body.String()
	if !strings.Contains(metricsBody, "mistypass_http_requests_total") {
		t.Fatalf("expected metrics to contain mistypass_http_requests_total")
	}
	if !strings.Contains(metricsBody, "mistypass_http_request_duration_seconds") {
		t.Fatalf("expected metrics to contain mistypass_http_request_duration_seconds")
	}
}
