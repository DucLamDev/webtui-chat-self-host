package requestcontext

import (
	"context"
	"testing"
)

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), " request-123 ")
	if got := RequestID(ctx); got != "request-123" {
		t.Fatalf("RequestID() = %q", got)
	}
	if got := RequestID(context.Background()); got != "" {
		t.Fatalf("empty RequestID() = %q", got)
	}
}
