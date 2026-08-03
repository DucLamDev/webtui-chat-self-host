package config

import (
	"strings"
	"testing"
)

func TestLoadReadsOpenTelemetryConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("OTEL_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4318/")
	t.Setenv("OTEL_TRACE_SAMPLE_RATIO", "0.25")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Telemetry.Enabled || cfg.Telemetry.OTLPEndpoint != "http://otel-collector:4318" || cfg.Telemetry.SampleRatio != 0.25 {
		t.Fatalf("Telemetry config = %#v", cfg.Telemetry)
	}
}

func TestValidateRejectsUnsafeOpenTelemetryConfiguration(t *testing.T) {
	cfg := validConfigForTest()
	cfg.Telemetry = TelemetryConfig{
		Enabled:      true,
		OTLPEndpoint: "https://user:secret@otel.example.com?token=secret",
		SampleRatio:  2,
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "OTEL_EXPORTER_OTLP_ENDPOINT") ||
		!strings.Contains(err.Error(), "OTEL_TRACE_SAMPLE_RATIO") {
		t.Fatalf("Validate() error = %v, want endpoint and sample-ratio rejection", err)
	}
}
