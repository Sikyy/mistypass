package httpx

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var httpRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "mistypass_http_requests_total",
		Help: "Total number of HTTP requests handled by MistyPass API.",
	},
	[]string{"method", "route", "status"},
)

var httpRequestDurationSeconds = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "mistypass_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	},
	[]string{"method", "route"},
)

var gatewayRequestNonceValidationTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "mistypass_gateway_request_nonce_validation_total",
		Help: "Gateway HTTP request nonce validation results.",
	},
	[]string{"result"},
)

var gatewayMTLSAuthTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "mistypass_gateway_mtls_auth_total",
		Help: "Gateway mTLS authentication results.",
	},
	[]string{"result"},
)

var gatewayCertificateRevocationsTotal = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "mistypass_gateway_mtls_cert_revocations_total",
		Help: "Current gateway mTLS certificate serial revocations.",
	},
	[]string{"source"},
)

var gatewayWebSocketAuthTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "mistypass_gateway_websocket_auth_total",
		Help: "Gateway WebSocket authentication mode and failure results.",
	},
	[]string{"mode"},
)

var gatewayWebSocketSessionsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "mistypass_gateway_websocket_sessions_total",
		Help: "Gateway WebSocket session lifecycle events.",
	},
	[]string{"event"},
)

var gatewayAuthzCacheReportsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "mistypass_gateway_authz_cache_reports_total",
		Help: "Gateway authz cache report status from pull/apply acknowledgements.",
	},
	[]string{"status"},
)

var gatewayWebSocketPushFanoutTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "mistypass_gateway_websocket_push_fanout_total",
		Help: "Gateway WebSocket push fanout results across API replicas.",
	},
	[]string{"result"},
)

func recordHTTPMetrics(r *http.Request, status int, duration time.Duration) {
	if r == nil {
		return
	}
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = "UNKNOWN"
	}
	route := requestRoutePattern(r)
	if route == "" {
		route = r.URL.Path
	}
	if strings.TrimSpace(route) == "" {
		route = "/"
	}
	statusLabel := strconv.Itoa(status)

	httpRequestsTotal.WithLabelValues(method, route, statusLabel).Inc()
	httpRequestDurationSeconds.WithLabelValues(method, route).Observe(duration.Seconds())
}

func recordGatewayNonceValidation(result string) {
	result = strings.TrimSpace(result)
	if result == "" {
		result = "unknown"
	}
	gatewayRequestNonceValidationTotal.WithLabelValues(result).Inc()
}

func recordGatewayMTLSAuth(result string) {
	result = strings.TrimSpace(result)
	if result == "" {
		result = "unknown"
	}
	gatewayMTLSAuthTotal.WithLabelValues(result).Inc()
}

func setGatewayCertificateRevocationMetric(source string, count int) {
	source = strings.TrimSpace(source)
	if source == "" {
		source = "runtime"
	}
	if count < 0 {
		count = 0
	}
	gatewayCertificateRevocationsTotal.WithLabelValues(source).Set(float64(count))
}

func recordGatewayWebSocketAuth(mode string) {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "unknown"
	}
	gatewayWebSocketAuthTotal.WithLabelValues(mode).Inc()
}

func recordGatewayWebSocketSession(event string) {
	event = strings.TrimSpace(event)
	if event == "" {
		event = "unknown"
	}
	gatewayWebSocketSessionsTotal.WithLabelValues(event).Inc()
}

func recordGatewayAuthzCacheReport(status string) {
	status = strings.TrimSpace(status)
	if status == "" {
		status = "unknown"
	}
	gatewayAuthzCacheReportsTotal.WithLabelValues(status).Inc()
}

func recordGatewayWebSocketPushFanout(result string) {
	result = strings.TrimSpace(result)
	if result == "" {
		result = "unknown"
	}
	gatewayWebSocketPushFanoutTotal.WithLabelValues(result).Inc()
}

func requestRoutePattern(r *http.Request) string {
	if r == nil {
		return ""
	}
	ctx := chi.RouteContext(r.Context())
	if ctx == nil {
		return ""
	}
	return strings.TrimSpace(ctx.RoutePattern())
}
