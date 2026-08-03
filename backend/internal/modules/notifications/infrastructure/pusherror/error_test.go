package pusherror

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestPermanentErrorSurvivesWrapping(t *testing.T) {
	err := fmt.Errorf("delivery: %w", PermanentError(errors.New("token is gone")))
	if !IsPermanent(err) {
		t.Fatal("wrapped permanent error was not recognized")
	}
}

func TestRetryAfterIsDeferredAndPreservesLongestJoinedDelay(t *testing.T) {
	short := fmt.Errorf("first destination: %w", RetryAfter(errors.New("throttled"), 20*time.Second))
	long := fmt.Errorf("second destination: %w", RetryAfter(errors.New("busy"), 90*time.Second))
	combined := errors.Join(short, TerminalError(errors.New("dead")), long)

	if !IsDeferred(combined) || HasNonDeferredRetryable(combined) {
		t.Fatalf("RetryAfter classification = deferred:%v non-deferred:%v", IsDeferred(combined), HasNonDeferredRetryable(combined))
	}
	delay, ok := Delay(combined)
	if !ok || delay != 90*time.Second {
		t.Fatalf("Delay() = %s, %v; want 90s, true", delay, ok)
	}
}

func TestDeferredAndTerminalErrorsRemainDistinctWhenWrapped(t *testing.T) {
	deferred := fmt.Errorf("relay: %w", DeferredError(errors.New("queued")))
	if !IsDeferred(deferred) || IsPermanent(deferred) || IsTerminal(deferred) {
		t.Fatalf("deferred classification is not isolated: %v", deferred)
	}
	terminal := fmt.Errorf("relay: %w", TerminalError(errors.New("dead")))
	if !IsTerminal(terminal) || IsPermanent(terminal) || IsDeferred(terminal) {
		t.Fatalf("terminal classification is not isolated: %v", terminal)
	}
}

func TestRetryabilityAcrossMultipleDestinations(t *testing.T) {
	terminal := TerminalError(errors.New("relay dead"))
	deferred := DeferredError(errors.New("relay pending"))
	transient := errors.New("web push timeout")

	if HasRetryable(terminal) || HasNonDeferredRetryable(terminal) {
		t.Fatal("a terminal-only tree must not be retryable")
	}
	mixedDeferred := errors.Join(terminal, fmt.Errorf("second destination: %w", deferred))
	if !HasRetryable(mixedDeferred) || HasNonDeferredRetryable(mixedDeferred) {
		t.Fatal("terminal + deferred must remain open without consuming the local retry budget")
	}
	mixedTransient := errors.Join(terminal, fmt.Errorf("second destination: %w", transient))
	if !HasRetryable(mixedTransient) || !HasNonDeferredRetryable(mixedTransient) {
		t.Fatal("terminal + ordinary transient must remain open and consume a retry attempt")
	}
}
