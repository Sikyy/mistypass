package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/mistypass/cloud/api/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func setupOpenTelemetry(ctx context.Context, cfg config.Config) (func(context.Context) error, error) {
	if !cfg.OTelEnabled {
		return func(context.Context) error { return nil }, nil
	}

	opts := []otlptracegrpc.Option{}
	endpoint := strings.TrimSpace(cfg.OTelExporterOTLPEndpoint)
	if endpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(endpoint))
	}
	if cfg.OTelExporterOTLPInsecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exportCtx, cancel := context.WithTimeout(ctx, cfg.OTelExportTimeout)
	defer cancel()
	exporter, err := otlptracegrpc.New(exportCtx, opts...)
	if err != nil {
		return nil, fmt.Errorf("init otlp trace exporter: %w", err)
	}

	res, err := resource.New(
		exportCtx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(strings.TrimSpace(cfg.OTelServiceName)),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("init otel resource: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.OTelTraceSampleRatio))),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tracerProvider.Shutdown, nil
}
