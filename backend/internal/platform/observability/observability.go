package observability

import (
	"context"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Config struct {
	Enabled     bool
	Endpoint    string
	ServiceName string
	Version     string
	Environment string
	SampleRatio float64
}

type Shutdown func(context.Context) error

func Setup(ctx context.Context, config Config) (Shutdown, error) {
	if !config.Enabled {
		return func(context.Context) error { return nil }, nil
	}
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(traceEndpoint(config.Endpoint)),
		otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
		otlptracehttp.WithMaxRequestSize(4*1024*1024),
		otlptracehttp.WithTimeout(10*time.Second),
	)
	if err != nil {
		return nil, err
	}

	ratio := config.SampleRatio
	if ratio <= 0 || ratio > 1 {
		ratio = 0.1
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
		sdktrace.WithResource(resource.NewSchemaless(
			attribute.String("service.name", strings.TrimSpace(config.ServiceName)),
			attribute.String("service.version", strings.TrimSpace(config.Version)),
			attribute.String("deployment.environment.name", strings.TrimSpace(config.Environment)),
		)),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return provider.Shutdown, nil
}

func traceEndpoint(endpoint string) string {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return strings.TrimSpace(endpoint)
	}
	path := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(path, "/v1/traces") {
		path += "/v1/traces"
	}
	parsed.Path = path
	return parsed.String()
}
