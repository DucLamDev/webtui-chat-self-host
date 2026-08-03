package webpush

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	push "github.com/SherClockHolmes/webpush-go"
	"github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/pusherror"
	"github.com/duclamdev/application-chat/backend/internal/shared/outboundhttp"
)

var safeEventType = regexp.MustCompile(`^[a-z0-9_.:-]{1,64}$`)

type Config struct {
	Enabled    bool
	PublicKey  string
	PrivateKey string
	Subject    string
	TTL        int
	HTTPClient *http.Client
}

type Sender struct {
	enabled    bool
	publicKey  string
	privateKey string
	subject    string
	ttl        int
	client     *http.Client
	send       func(context.Context, []byte, *push.Subscription, *push.Options) (*http.Response, error)
}

func NewSender(config Config) *Sender {
	client := config.HTTPClient
	if client == nil {
		// Subscription endpoints are browser-controlled URLs. Resolve and dial
		// only public IPs, disable proxies and refuse redirects so Web Push cannot
		// be used to reach instance metadata or private services.
		client = outboundhttp.NewPublicClient(15*time.Second, false)
	}
	return &Sender{
		enabled: config.Enabled, publicKey: strings.TrimSpace(config.PublicKey),
		privateKey: strings.TrimSpace(config.PrivateKey), subject: strings.TrimSpace(config.Subject),
		ttl: config.TTL, client: client, send: push.SendNotificationWithContext,
	}
}

func (s *Sender) Enabled() bool {
	return s != nil && s.enabled && s.publicKey != "" && s.privateKey != "" && s.subject != ""
}

func (s *Sender) Send(ctx context.Context, endpoint string, p256dh string, auth string, payload map[string]any) error {
	if !s.Enabled() {
		return errors.New("Web Push is not configured")
	}
	message, topic, urgency, err := buildPayload(payload)
	if err != nil {
		return err
	}
	response, err := s.send(ctx, message, &push.Subscription{
		Endpoint: strings.TrimSpace(endpoint),
		Keys:     push.Keys{P256dh: strings.TrimSpace(p256dh), Auth: strings.TrimSpace(auth)},
	}, &push.Options{
		HTTPClient: s.client, Subscriber: s.subject, TTL: s.ttl,
		VAPIDPublicKey: s.publicKey, VAPIDPrivateKey: s.privateKey,
		Topic: topic, Urgency: urgency,
	})
	if response != nil {
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	}
	if err == nil && response != nil && response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	if err == nil && response == nil {
		err = errors.New("Web Push endpoint returned no response")
	}
	if response != nil {
		deliveryErr := fmt.Errorf("Web Push endpoint returned %s", response.Status)
		if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusGone {
			return pusherror.PermanentError(deliveryErr)
		}
		return deliveryErr
	}
	return err
}

func buildPayload(payload map[string]any) ([]byte, string, push.Urgency, error) {
	eventID := cleanText(firstString(payload["event_id"], payload["notification_id"]), 128)
	if eventID == "" {
		return nil, "", "", errors.New("Web Push payload is missing an event ID")
	}
	eventType := strings.ToLower(cleanText(firstString(payload["event_type"], "notification"), 64))
	if !safeEventType.MatchString(eventType) {
		eventType = "notification"
	}
	title := cleanText(firstString(payload["title"], "WebTui Chat"), 120)
	body := cleanText(firstString(payload["body"], "You have a new notification."), 240)
	workspaceID := cleanIdentifier(payload["workspace_id"])
	channelID := cleanIdentifier(payload["channel_id"])
	messageID := cleanIdentifier(payload["message_id"])
	callID := cleanIdentifier(payload["call_id"])
	tag := cleanText(firstString(payload["tag"], eventType+"-"+eventID), 64)

	data := map[string]string{}
	for key, value := range map[string]string{
		"workspace_id": workspaceID, "channel_id": channelID,
		"message_id": messageID, "call_id": callID,
	} {
		if value != "" {
			data[key] = value
		}
	}
	if target := browserTarget(workspaceID, channelID, messageID); target != "" {
		data["url"] = target
	}
	envelope := map[string]any{
		"version": 1,
		"id":      eventID,
		"type":    eventType,
		"title":   title,
		"body":    body,
		"tag":     tag,
		"data":    data,
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, "", "", err
	}
	if len(encoded) > 3072 {
		return nil, "", "", errors.New("Web Push payload exceeds the safe encrypted payload limit")
	}
	urgency := push.UrgencyNormal
	if strings.HasPrefix(eventType, "call_") || eventType == "mention" {
		urgency = push.UrgencyHigh
	}
	return encoded, topicFor(eventID), urgency, nil
}

func browserTarget(workspaceID string, channelID string, messageID string) string {
	if workspaceID == "" {
		return ""
	}
	path := "/chat/" + url.PathEscape(workspaceID)
	if channelID != "" {
		path += "/channel/" + url.PathEscape(channelID)
	}
	if messageID != "" {
		query := url.Values{"message": []string{messageID}}
		path += "?" + query.Encode()
	}
	return path
}

func topicFor(value string) string {
	digest := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(digest[:18])
}

func cleanIdentifier(value any) string {
	valueString := strings.TrimSpace(firstString(value))
	if len(valueString) > 128 || !safeEventType.MatchString(valueString) {
		return ""
	}
	return valueString
}

func cleanText(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if r == '\x00' || r == '\r' || r == '\n' || r == '\t' {
			return ' '
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(value))
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit])
}

func firstString(values ...any) string {
	for _, value := range values {
		if typed, ok := value.(string); ok && strings.TrimSpace(typed) != "" {
			return typed
		}
	}
	return ""
}
