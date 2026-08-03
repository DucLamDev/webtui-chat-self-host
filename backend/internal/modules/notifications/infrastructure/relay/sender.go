package relay

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/pusherror"
)

type Config struct {
	URL        string
	Token      string
	InstanceID string
	Provider   string
	HTTPClient *http.Client
}

type Sender struct {
	url        string
	token      string
	instanceID string
	provider   string
	client     *http.Client
}

type deliveryAcknowledgement struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func NewSender(config Config) *Sender {
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	} else {
		// Do not mutate the caller's client when applying the relay security
		// policy. A relay credential must never be forwarded to a redirect
		// target, even when that target shares a parent domain.
		clone := *client
		client = &clone
	}
	if client.CheckRedirect == nil {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return &Sender{
		url:        strings.TrimSpace(config.URL),
		token:      strings.TrimSpace(config.Token),
		instanceID: strings.TrimSpace(config.InstanceID),
		provider:   strings.ToLower(strings.TrimSpace(config.Provider)),
		client:     client,
	}
}

func (s *Sender) Provider() string {
	if s == nil {
		return ""
	}
	return s.provider
}

func (s *Sender) Enabled() bool {
	return s != nil && s.url != "" && s.token != "" && s.instanceID != "" &&
		(s.provider == "fcm" || s.provider == "apns")
}

func (s *Sender) Send(ctx context.Context, deviceToken string, payload map[string]any) error {
	if !s.Enabled() {
		return errors.New("push relay is not configured")
	}
	deviceToken = strings.TrimSpace(deviceToken)
	if deviceToken == "" {
		return errors.New("push device token is empty")
	}
	// APNs destinations stored by the mobile client are PushKit VoIP tokens.
	// Apple requires those tokens to be used only for an incoming call.
	if s.provider == "apns" && !strings.EqualFold(stringValue(payload["event_type"]), "call_invite") {
		return nil
	}
	body, err := json.Marshal(map[string]any{
		"provider":     s.provider,
		"device_token": deviceToken,
		"instance_id":  s.instanceID,
		"payload":      payload,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", relayIdempotencyKey(body))
	req.Header.Set("User-Agent", "webtui-chat-self-host/push-relay")
	response, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		raw, readErr := io.ReadAll(io.LimitReader(response.Body, 4097))
		if readErr != nil {
			return fmt.Errorf("read push relay acknowledgement: %w", readErr)
		}
		if len(raw) > 4096 {
			return errors.New("push relay acknowledgement is too large")
		}
		var acknowledgement deliveryAcknowledgement
		if err := json.Unmarshal(raw, &acknowledgement); err != nil || strings.TrimSpace(acknowledgement.ID) == "" {
			return errors.New("push relay returned an invalid acknowledgement")
		}
		switch strings.ToLower(strings.TrimSpace(acknowledgement.Status)) {
		case "sent":
			return nil
		case "pending", "processing", "retry":
			return pusherror.DeferredError(errors.New("push relay accepted delivery and is awaiting provider completion"))
		case "dead":
			return pusherror.TerminalError(errors.New("push relay delivery reached dead-letter"))
		default:
			return errors.New("push relay returned an unknown delivery status")
		}
	}
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
	detail := sanitizeRelayErrorDetail(string(raw), deviceToken, s.token)
	deliveryErr := fmt.Errorf("push relay returned %s: %s", response.Status, detail)
	reason := strings.ToLower(string(raw))
	if response.StatusCode == http.StatusTooEarly || response.StatusCode == http.StatusTooManyRequests {
		// These are relay admission/control-plane responses, not a provider
		// delivery failure. Keep the local durable job open without consuming its
		// attempt budget. Preserve only a parsed duration from Retry-After; never
		// retain the raw, remotely controlled header in a queue error.
		if delay := parseRetryAfter(response.Header.Get("Retry-After"), time.Now().UTC()); delay > 0 {
			return pusherror.RetryAfter(deliveryErr, delay)
		}
		return pusherror.DeferredError(deliveryErr)
	}
	if response.StatusCode == http.StatusGone ||
		strings.Contains(reason, "invalid_device_token") ||
		strings.Contains(reason, "unregistered") {
		return pusherror.PermanentError(deliveryErr)
	}
	if response.StatusCode >= 400 && response.StatusCode < 500 &&
		response.StatusCode != http.StatusRequestTimeout {
		return pusherror.TerminalError(deliveryErr)
	}
	return deliveryErr
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds <= 0 {
			return 0
		}
		const maxDurationSeconds = int64(^uint64(0)>>1) / int64(time.Second)
		if seconds > maxDurationSeconds {
			seconds = maxDurationSeconds
		}
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil || !when.After(now) {
		return 0
	}
	return when.Sub(now)
}

func sanitizeRelayErrorDetail(detail string, deviceToken string, relayToken string) string {
	for _, secret := range []string{deviceToken, relayToken} {
		if strings.TrimSpace(secret) != "" {
			detail = strings.ReplaceAll(detail, secret, "[REDACTED]")
		}
	}
	detail = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, detail)
	detail = strings.Join(strings.Fields(detail), " ")
	if detail == "" {
		return "relay returned no error detail"
	}
	if utf8.RuneCountInString(detail) > 512 {
		return string([]rune(detail)[:512])
	}
	return detail
}

func relayIdempotencyKey(body []byte) string {
	digest := sha256.Sum256(body)
	return "push-" + base64.RawURLEncoding.EncodeToString(digest[:])
}

func stringValue(value any) string {
	if typed, ok := value.(string); ok {
		return strings.TrimSpace(typed)
	}
	return ""
}
