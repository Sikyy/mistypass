package httpx

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

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
			"query", redactQueryForLog(r.URL.Path, r.URL.RawQuery),
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"request_id", requestID,
			"client_ip", s.clientIP(r),
			"user_agent", r.UserAgent(),
		)
	})
}

// sensitiveUploadQueryKeys are the signed-capability query parameters attached to
// the signed blob download/upload URLs (GET/PUT /api/v1/uploads/{id}). Logging the
// derived HMAC signature (and its bound uid/expires) would hand anyone with log
// access a short-lived capability to download or upload that specific blob —
// including credential photos and documents — so they are redacted before logging.
var sensitiveUploadQueryKeys = map[string]struct{}{
	"sig":     {},
	"uid":     {},
	"expires": {},
}

// redactQueryForLog returns a copy of rawQuery safe to write to application logs.
// For requests under /api/v1/uploads/, the values of the signed-capability params
// (sig/uid/expires) are replaced with REDACTED while preserving key order and any
// other params. For all other paths rawQuery is returned unchanged.
func redactQueryForLog(path, rawQuery string) string {
	if rawQuery == "" || !strings.HasPrefix(path, "/api/v1/uploads/") {
		return rawQuery
	}
	parts := strings.Split(rawQuery, "&")
	for i, part := range parts {
		key, _, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		// Match on the decoded key name: the uploads handlers read params via
		// r.URL.Query(), which percent-decodes keys, so "%73ig" reaches the
		// handler as a live "sig" capability and must be redacted just the same.
		decodedKey := key
		if unescaped, err := url.QueryUnescape(key); err == nil {
			decodedKey = unescaped
		}
		if _, sensitive := sensitiveUploadQueryKeys[decodedKey]; sensitive {
			parts[i] = key + "=REDACTED"
		}
	}
	return strings.Join(parts, "&")
}
