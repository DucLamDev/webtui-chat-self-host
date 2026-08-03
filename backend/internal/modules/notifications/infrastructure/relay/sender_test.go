package relay

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/pusherror"
)

func TestSenderForwardsPushWithoutLeakingCredentialIntoBody(t *testing.T) {
	var body map[string]any
	var idempotencyKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer relay-secret" {
			t.Fatalf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		idempotencyKey = r.Header.Get("Idempotency-Key")
		writeRelayAcknowledgement(t, w, "sent")
	}))
	defer server.Close()

	sender := NewSender(Config{
		URL: server.URL, Token: "relay-secret", InstanceID: "instance-1", Provider: "fcm",
	})
	if err := sender.Send(context.Background(), "device-token", map[string]any{"event_type": "message_created"}); err != nil {
		t.Fatal(err)
	}
	if body["provider"] != "fcm" || body["device_token"] != "device-token" || body["instance_id"] != "instance-1" {
		t.Fatalf("unexpected relay body: %#v", body)
	}
	if _, exists := body["token"]; exists {
		t.Fatalf("relay credential must not be included in body: %#v", body)
	}
	if !strings.HasPrefix(idempotencyKey, "push-") {
		t.Fatalf("Idempotency-Key = %q", idempotencyKey)
	}
}

func TestSenderKeepsLocalJobOpenUntilRelayProviderCompletion(t *testing.T) {
	for _, test := range []struct {
		name     string
		status   string
		deferred bool
		terminal bool
	}{
		{name: "pending", status: "pending", deferred: true},
		{name: "retry", status: "retry", deferred: true},
		{name: "processing", status: "processing", deferred: true},
		{name: "dead", status: "dead", terminal: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRelayAcknowledgement(t, w, test.status)
			}))
			defer server.Close()
			sender := NewSender(Config{URL: server.URL, Token: "relay-secret", InstanceID: "instance-1", Provider: "fcm"})
			err := sender.Send(context.Background(), "device-token", map[string]any{"event_type": "message_created"})
			if err == nil || pusherror.IsDeferred(err) != test.deferred || pusherror.IsTerminal(err) != test.terminal || pusherror.IsPermanent(err) {
				t.Fatalf("Send() classification = %v, deferred=%v terminal=%v", err, pusherror.IsDeferred(err), pusherror.IsTerminal(err))
			}
		})
	}
}

func TestSenderTreatsRelayAdmissionThrottleAsDeferred(t *testing.T) {
	for _, status := range []int{http.StatusTooEarly, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"error":{"code":"RATE_LIMITED","message":"retry later"}}`)
			}))
			defer server.Close()

			sender := NewSender(Config{URL: server.URL, Token: "relay-secret", InstanceID: "instance-1", Provider: "fcm"})
			err := sender.Send(context.Background(), "device-token", map[string]any{"event_type": "message_created"})
			if err == nil || !pusherror.IsDeferred(err) || pusherror.IsTerminal(err) || pusherror.IsPermanent(err) {
				t.Fatalf("status %d classification = %v", status, err)
			}
			if delay, ok := pusherror.Delay(err); !ok || delay != 60*time.Second {
				t.Fatalf("status %d Retry-After delay = %s, %v", status, delay, ok)
			}
		})
	}
}

func TestParseRetryAfterSupportsHTTPDateWithoutRetainingRawValue(t *testing.T) {
	now := time.Date(2026, time.August, 3, 10, 0, 0, 0, time.UTC)
	if delay := parseRetryAfter(now.Add(75*time.Second).Format(http.TimeFormat), now); delay != 75*time.Second {
		t.Fatalf("HTTP-date delay = %s", delay)
	}
	if delay := parseRetryAfter("not-a-delay secret-header", now); delay != 0 {
		t.Fatalf("invalid header delay = %s", delay)
	}
}

func TestSenderSkipsNonCallPayloadForAPNSVoIPToken(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sender := NewSender(Config{
		URL: server.URL, Token: "relay-secret", InstanceID: "instance-1", Provider: "apns",
	})
	if err := sender.Send(context.Background(), "voip-token", map[string]any{"event_type": "message_created"}); err != nil {
		t.Fatal(err)
	}
	if requests != 0 {
		t.Fatalf("APNs VoIP sender made %d request(s) for a message notification", requests)
	}
}

func TestSenderDoesNotForwardCredentialAcrossRedirect(t *testing.T) {
	credentialSeen := ""
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		credentialSeen = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer target.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	sender := NewSender(Config{
		URL: redirect.URL, Token: "relay-secret", InstanceID: "instance-1", Provider: "fcm",
	})
	err := sender.Send(context.Background(), "device-token", map[string]any{"event_type": "message_created"})
	if err == nil || !strings.Contains(err.Error(), "307") {
		t.Fatalf("Send() error = %v, want redirect response", err)
	}
	if credentialSeen != "" {
		t.Fatalf("relay credential leaked to redirect target: %q", credentialSeen)
	}
}

func TestSenderRedactsDeviceTokenFromRelayError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"bad device-token-secret"}`)
	}))
	defer server.Close()

	sender := NewSender(Config{
		URL: server.URL, Token: "relay-secret", InstanceID: "instance-1", Provider: "fcm",
	})
	err := sender.Send(context.Background(), "device-token-secret", map[string]any{"event_type": "message_created"})
	if err == nil || strings.Contains(err.Error(), "device-token-secret") {
		t.Fatalf("Send() error did not redact the device token: %v", err)
	}
}

func TestSenderRedactsRelayCredentialAndControlsFromError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "proxy echoed relay-secret\r\nforbidden")
	}))
	defer server.Close()

	sender := NewSender(Config{
		URL: server.URL, Token: "relay-secret", InstanceID: "instance-1", Provider: "fcm",
	})
	err := sender.Send(context.Background(), "device-token", map[string]any{"event_type": "message_created"})
	if err == nil || strings.Contains(err.Error(), "relay-secret") || strings.ContainsAny(err.Error(), "\r\n") {
		t.Fatalf("Send() retained a secret or control character: %v", err)
	}
}

func writeRelayAcknowledgement(t *testing.T, w http.ResponseWriter, status string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"id": "4d9b657a-3a54-45e1-a57e-ea8079ba87bd", "status": status, "deduplicated": false,
	}); err != nil {
		t.Fatal(err)
	}
}
