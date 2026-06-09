package httpx

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// sensitiveQueryParams are signed-URL credentials that must never be written to
// request logs. They appear on the signed blob endpoints under /api/v1/uploads/
// (avatars, credential photos, documents, OTA firmware). Logging the raw `sig`
// hands anyone with log access a short-lived download/upload capability.
var sensitiveQueryParams = []string{"sig", "uid", "expires"}

// redactSensitiveQuery returns rawQuery with signed-URL credential values replaced
// by "REDACTED" for the /api/v1/uploads/ blob endpoints. Other paths and queries
// without sensitive params are returned unchanged (no re-encoding).
func redactSensitiveQuery(path, rawQuery string) string {
	if rawQuery == "" || !strings.HasPrefix(path, "/api/v1/uploads/") {
		return rawQuery
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "REDACTED"
	}
	redacted := false
	for _, key := range sensitiveQueryParams {
		if values.Has(key) {
			values.Set(key, "REDACTED")
			redacted = true
		}
	}
	if !redacted {
		return rawQuery
	}
	return values.Encode()
}

type requestLogResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *requestLogResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *requestLogResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

// Flush forwards to the underlying ResponseWriter so SSE / streaming handlers
// (which type-assert http.Flusher) keep working through this logging middleware.
// Without it, w.(http.Flusher) fails and the alarms stream returns 500.
var _ http.Flusher = (*requestLogResponseWriter)(nil)

func (w *requestLogResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *server) withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC()
		wrapped := &requestLogResponseWriter{ResponseWriter: w}
		next.ServeHTTP(wrapped, r)
		duration := time.Since(start)

		status := wrapped.status
		if status == 0 {
			status = http.StatusOK
		}
		recordHTTPMetrics(r, status, duration)

		logger := s.loggerOrDefault()

		requestID := strings.TrimSpace(middleware.GetReqID(r.Context()))
		if requestID == "" {
			requestID = "-"
		}

		logger.Info(
			"http request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"query", redactSensitiveQuery(r.URL.Path, r.URL.RawQuery),
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"request_id", requestID,
			"client_ip", s.clientIP(r),
			"user_agent", r.UserAgent(),
		)
	})
}
