package pusherror

import (
	"errors"
	"time"
)

type permanentError struct {
	err error
}

type deferredError struct {
	err error
}

type terminalError struct {
	err error
}

type retryAfterError struct {
	err   error
	delay time.Duration
}

func (e permanentError) Error() string {
	return e.err.Error()
}

func (e permanentError) Unwrap() error {
	return e.err
}

func (e permanentError) Permanent() bool {
	return true
}

func (e deferredError) Error() string  { return e.err.Error() }
func (e deferredError) Unwrap() error  { return e.err }
func (e deferredError) Deferred() bool { return true }

func (e terminalError) Error() string  { return e.err.Error() }
func (e terminalError) Unwrap() error  { return e.err }
func (e terminalError) Terminal() bool { return true }

func (e retryAfterError) Error() string             { return e.err.Error() }
func (e retryAfterError) Unwrap() error             { return e.err }
func (e retryAfterError) Deferred() bool            { return true }
func (e retryAfterError) RetryDelay() time.Duration { return e.delay }

func PermanentError(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err: err}
}

func IsPermanent(err error) bool {
	var target interface{ Permanent() bool }
	return errors.As(err, &target) && target.Permanent()
}

// DeferredError marks a durable remote delivery that is still being processed.
// It must be retried soon without consuming the application's own retry budget.
func DeferredError(err error) error {
	if err == nil {
		return nil
	}
	return deferredError{err: err}
}

func IsDeferred(err error) bool {
	var target interface{ Deferred() bool }
	return errors.As(err, &target) && target.Deferred()
}

// RetryAfter marks a deferred relay control-plane response with a parsed delay.
// Callers should pass only a duration, never the raw response header, so queue
// errors and admin dashboards cannot accidentally retain attacker-controlled
// header content.
func RetryAfter(err error, delay time.Duration) error {
	if err == nil {
		return nil
	}
	if delay <= 0 {
		return DeferredError(err)
	}
	return retryAfterError{err: err, delay: delay}
}

// Delay returns the longest requested retry delay in an error tree. Selecting
// the maximum matters for errors.Join across multiple destinations: retrying at
// the shortest provider delay can immediately trigger another throttle.
func Delay(err error) (time.Duration, bool) {
	return retryDelay(err)
}

func retryDelay(err error) (time.Duration, bool) {
	if err == nil {
		return 0, false
	}
	if marker, ok := err.(interface{ RetryDelay() time.Duration }); ok {
		delay := marker.RetryDelay()
		return delay, delay > 0
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var longest time.Duration
		found := false
		for _, child := range joined.Unwrap() {
			if delay, exists := retryDelay(child); exists && (!found || delay > longest) {
				longest = delay
				found = true
			}
		}
		return longest, found
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return retryDelay(wrapped.Unwrap())
	}
	return 0, false
}

// TerminalError marks a destination attempt that is finished and unsuccessful,
// but does not prove that the device token itself is invalid. Unlike a permanent
// provider error, callers must not revoke the token solely because of this type.
func TerminalError(err error) error {
	if err == nil {
		return nil
	}
	return terminalError{err: err}
}

func IsTerminal(err error) bool {
	var target interface{ Terminal() bool }
	return errors.As(err, &target) && target.Terminal()
}

// HasRetryable reports whether an error tree contains a destination failure
// that is not final. It deliberately walks errors.Join trees instead of using
// errors.As: a terminal failure for one device must not hide a retryable
// failure for another device.
func HasRetryable(err error) bool {
	return hasRetryable(err, true)
}

// HasNonDeferredRetryable reports whether retrying should consume the local
// job's attempt budget. A relay acknowledgement that is merely pending is
// retryable but does not consume that budget; an ordinary transport/provider
// error does.
func HasNonDeferredRetryable(err error) bool {
	return hasRetryable(err, false)
}

func hasRetryable(err error, includeDeferred bool) bool {
	if err == nil {
		return false
	}
	if marker, ok := err.(interface{ Deferred() bool }); ok && marker.Deferred() {
		return includeDeferred
	}
	if marker, ok := err.(interface{ Terminal() bool }); ok && marker.Terminal() {
		return false
	}
	if marker, ok := err.(interface{ Permanent() bool }); ok && marker.Permanent() {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if hasRetryable(child, includeDeferred) {
				return true
			}
		}
		return false
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return hasRetryable(wrapped.Unwrap(), includeDeferred)
	}
	return true
}
