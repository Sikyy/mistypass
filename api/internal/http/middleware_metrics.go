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
