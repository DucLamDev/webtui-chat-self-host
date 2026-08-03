package webpush

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	push "github.com/SherClockHolmes/webpush-go"
	"github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/pusherror"
)

func TestBuildPayloadKeepsOnlySafeSameOriginNavigation(t *testing.T) {
	raw, topic, urgency, err := buildPayload(map[string]any{
		"event_id": "event-1", "event_type": "mention", "title": " hello\nworld ",
		"body": "body", "workspace_id": "workspace-1", "channel_id": "channel-1",
		"message_id": "message-1", "deep_link": "https://evil.example/steal",
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Title string            `json:"title"`
		Data  map[string]string `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Title != "hello world" {
		t.Fatalf("title = %q", payload.Title)
	}
	if got := payload.Data["url"]; got != "/chat/workspace-1/channel/channel-1?message=message-1" {
		t.Fatalf("url = %q", got)
	}
	if strings.Contains(string(raw), "evil.example") || topic == "" || urgency != push.UrgencyHigh {
		t.Fatalf("unsafe or incomplete payload: %s", raw)
	}
}

func TestSenderClassifiesGoneSubscriptionAsPermanent(t *testing.T) {
	sender := NewSender(Config{Enabled: true, PublicKey: "public", PrivateKey: "private", Subject: "mailto:test@example.com"})
	sender.send = func(context.Context, []byte, *push.Subscription, *push.Options) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusGone, Status: "410 Gone", Body: io.NopCloser(strings.NewReader("expired"))}, nil
	}
	err := sender.Send(context.Background(), "https://push.example/subscription-secret", "p256dh", "auth", map[string]any{"event_id": "event-1"})
	if !pusherror.IsPermanent(err) || strings.Contains(err.Error(), "subscription-secret") {
		t.Fatalf("Send() error = %v", err)
	}
}
