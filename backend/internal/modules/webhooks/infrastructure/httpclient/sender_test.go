package httpclient

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	webhooksdomain "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/domain"
	webhooksecurity "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/security"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestSenderSignsWithCustomerVisibleSecret(t *testing.T) {
	const masterSecret = "test-master-secret-with-at-least-32-characters"
	const signingSecret = "wtow_customer-visible-secret"
	body := []byte(`{"event":"MessageCreated"}`)
	envelope, err := webhooksecurity.EncryptSecret(masterSecret, signingSecret)
	if err != nil {
		t.Fatalf("EncryptSecret() error = %v", err)
	}

	sender := NewSender(masterSecret)
	sender.now = func() time.Time {
		return time.Unix(1_721_000_000, 0)
	}
	sender.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestBody, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Fatalf("read request body: %v", readErr)
		}
		timestamp := request.Header.Get("X-WebTui-Timestamp")
		mac := hmac.New(sha256.New, []byte(signingSecret))
		_, _ = mac.Write([]byte(timestamp + "."))
		_, _ = mac.Write(requestBody)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if actual := request.Header.Get("X-WebTui-Signature"); actual != expected {
			t.Fatalf("X-WebTui-Signature = %q, want %q", actual, expected)
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})

	_, err = sender.Send(context.Background(), webhooksdomain.Delivery{
		TargetURL:              "https://hooks.customer.example/events",
		SigningSecretEncrypted: envelope,
		EventType:              "MessageCreated",
		RequestBody:            body,
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

func TestPublicOutboundIPPolicy(t *testing.T) {
	tests := []struct {
		address string
		allowed bool
	}{
		{address: "8.8.8.8", allowed: true},
		{address: "2606:4700:4700::1111", allowed: true},
		{address: "127.0.0.1", allowed: false},
		{address: "10.0.0.1", allowed: false},
		{address: "169.254.169.254", allowed: false},
		{address: "100.64.0.1", allowed: false},
		{address: "198.51.100.20", allowed: false},
		{address: "::1", allowed: false},
		{address: "fd00::1", allowed: false},
		{address: "2001:db8::1", allowed: false},
	}
	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			if actual := isPublicOutboundIP(netip.MustParseAddr(tt.address)); actual != tt.allowed {
				t.Fatalf("isPublicOutboundIP(%s) = %t, want %t", tt.address, actual, tt.allowed)
			}
		})
	}
}
