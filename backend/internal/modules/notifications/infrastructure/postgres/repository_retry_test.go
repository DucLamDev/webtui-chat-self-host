package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/pusherror"
)

func TestDeferredRetryDelayUsesProviderHintWithinSafeBounds(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want time.Duration
	}{
		{name: "no hint", err: pusherror.DeferredError(errors.New("pending")), want: 5 * time.Second},
		{name: "below minimum", err: pusherror.RetryAfter(errors.New("throttled"), time.Second), want: 5 * time.Second},
		{name: "provider hint", err: pusherror.RetryAfter(errors.New("throttled"), 75*time.Second), want: 75 * time.Second},
		{name: "above maximum", err: pusherror.RetryAfter(errors.New("throttled"), time.Hour), want: 5 * time.Minute},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := deferredRetryDelay(test.err); got != test.want {
				t.Fatalf("deferredRetryDelay() = %s, want %s", got, test.want)
			}
		})
	}
}
