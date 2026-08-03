package observability

import (
	"context"
	"testing"
)

func TestTraceEndpointUsesOTLPHTTPTracePath(t *testing.T) {
	tests := map[string]string{
		"http://collector:4318":           "http://collector:4318/v1/traces",
		"https://otel.example.com/base/":  "https://otel.example.com/base/v1/traces",
		"http://collector:4318/v1/traces": "http://collector:4318/v1/traces",
	}
	for input, expected := range tests {
		if actual := traceEndpoint(input); actual != expected {
			t.Fatalf("traceEndpoint(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestSetupDisabledDoesNotRequireCollector(t *testing.T) {
	shutdown, err := Setup(context.Background(), Config{})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
