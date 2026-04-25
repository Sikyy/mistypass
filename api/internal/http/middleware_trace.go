package httpx

import (
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func (s *server) withTrace(next http.Handler) http.Handler {
	if s == nil || !s.cfg.OTelEnabled {
		return next
	}

	tracer := otel.Tracer("mistypass/http")
	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		propagator = propagation.TraceContext{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := requestRoutePattern(r)
		if route == "" {
			route = strings.TrimSpace(r.URL.Path)
		}
		if route == "" {
			route = "/"
		}

		parentCtx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		spanName := fmt.Sprintf("%s %s", strings.ToUpper(strings.TrimSpace(r.Method)), route)
		ctx, span := tracer.Start(parentCtx, spanName, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		wrapped := &requestLogResponseWriter{ResponseWriter: w}
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		status := wrapped.status
		if status == 0 {
			status = http.StatusOK
		}
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.route", route),
			attribute.String("url.path", r.URL.Path),
			attribute.Int("http.response.status_code", status),
		)
		if status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(status))
		} else {
			span.SetStatus(codes.Ok, http.StatusText(status))
		}
	})
}
