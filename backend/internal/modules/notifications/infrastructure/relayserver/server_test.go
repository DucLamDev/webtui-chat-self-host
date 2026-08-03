package relayserver

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/pusherror"
)

type fakeStore struct {
	job           Job
	hash          []byte
	enqueueCalls  int
	claimErr      error
	markSent      bool
	markPermanent bool
	markReason    string
	crashedFinal  bool
	reapCalls     int
	stats         QueueStats
	statsErr      error
}

func (s *fakeStore) Enqueue(_ context.Context, input EnqueueInput) (Job, bool, error) {
	s.enqueueCalls++
	if len(s.hash) > 0 {
		if len(s.hash) != len(input.RequestHash) || subtle.ConstantTimeCompare(s.hash, input.RequestHash) != 1 {
			return Job{}, false, ErrIdempotencyConflict
		}
		return s.job, true, nil
	}
	s.hash = append([]byte(nil), input.RequestHash...)
	s.job = Job{
		ID: "4d9b657a-3a54-45e1-a57e-ea8079ba87bd", PublisherID: input.PublisherID,
		Provider: input.Provider, DeviceToken: input.DeviceToken,
		Payload: append([]byte(nil), input.Payload...), Status: "pending", MaxAttempts: input.MaxAttempts,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	return s.job, false, nil
}

func (s *fakeStore) Get(_ context.Context, publisherID string, jobID string) (Job, error) {
	if s.job.ID != jobID || s.job.PublisherID != publisherID {
		return Job{}, ErrJobNotFound
	}
	return s.job, nil
}

func (s *fakeStore) Claim(context.Context) (Job, error) {
	if s.claimErr != nil {
		return Job{}, s.claimErr
	}
	return s.job, nil
}

func (s *fakeStore) MarkSent(context.Context, string) error {
	s.markSent = true
	return nil
}

func (s *fakeStore) MarkFailed(_ context.Context, _ Job, permanent bool, reason string, _ time.Time) error {
	s.markPermanent = permanent
	s.markReason = reason
	return nil
}

func (s *fakeStore) ReapExpiredProcessing(context.Context, time.Time) (int64, error) {
	s.reapCalls++
	if !s.crashedFinal {
		return 0, nil
	}
	s.crashedFinal = false
	s.job.Status = "dead"
	s.job.DeviceToken = "[discarded]"
	s.job.Payload = []byte(`{}`)
	return 1, nil
}

func (s *fakeStore) Stats(context.Context, time.Time) (QueueStats, error) {
	return s.stats, s.statsErr
}

func (*fakeStore) Ping(context.Context) error                      { return nil }
func (*fakeStore) Purge(context.Context, time.Time) (int64, error) { return 0, nil }

type fakeSender struct {
	provider string
	err      error
	received map[string]any
}

func (s fakeSender) Provider() string { return s.provider }
func (s fakeSender) Enabled() bool    { return true }
func (s fakeSender) Send(_ context.Context, _ string, payload map[string]any) error {
	for key, value := range payload {
		if s.received != nil {
			s.received[key] = value
		}
	}
	return s.err
}

func TestDeliveryEndpointAuthenticatesAndDeduplicates(t *testing.T) {
	store := &fakeStore{}
	server := newTestServer(t, store, fakeSender{provider: "fcm"})
	body := []byte(`{"provider":"fcm","device_token":"device-token-123","instance_id":"instance-1","payload":{"event_id":"event-1","event_type":"message","title":"Hello"}}`)

	first := performDelivery(server.Handler(), body, "publisher-token-that-is-at-least-32-characters", "delivery-key-0001")
	if first.Code != http.StatusAccepted || !strings.Contains(first.Body.String(), `"deduplicated":false`) {
		t.Fatalf("first response = %d %s", first.Code, first.Body.String())
	}
	second := performDelivery(server.Handler(), body, "publisher-token-that-is-at-least-32-characters", "delivery-key-0001")
	if second.Code != http.StatusAccepted || !strings.Contains(second.Body.String(), `"deduplicated":true`) {
		t.Fatalf("second response = %d %s", second.Code, second.Body.String())
	}
	if strings.Contains(first.Body.String(), "device-token-123") {
		t.Fatalf("response leaked a device token: %s", first.Body.String())
	}

	changed := bytes.Replace(body, []byte("event-1"), []byte("event-2"), 1)
	conflict := performDelivery(server.Handler(), changed, "publisher-token-that-is-at-least-32-characters", "delivery-key-0001")
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict response = %d %s", conflict.Code, conflict.Body.String())
	}
}

func TestDeliveryEndpointNormalizesMultilineDisplayText(t *testing.T) {
	store := &fakeStore{}
	server := newTestServer(t, store, fakeSender{provider: "fcm"})
	body := []byte(`{"provider":"fcm","device_token":"device-token-123","instance_id":"instance-1","payload":{"event_id":"event-1","event_type":"message","title":"Hello\tthere","body":"line 1\nline 2"}}`)

	response := performDelivery(server.Handler(), body, "publisher-token-that-is-at-least-32-characters", "delivery-key-text-0001")
	if response.Code != http.StatusAccepted {
		t.Fatalf("multiline display payload = %d %s", response.Code, response.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(store.job.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["title"] != "Hello there" || payload["body"] != "line 1 line 2" {
		t.Fatalf("stored normalized payload = %#v", payload)
	}
}

func TestDeliveryEndpointRejectsInvalidAuthenticationBeforeQueueing(t *testing.T) {
	store := &fakeStore{}
	server := newTestServer(t, store, fakeSender{provider: "fcm"})
	body := []byte(`{"provider":"fcm","device_token":"device-token-123","instance_id":"instance-1","payload":{"event_id":"event-1","event_type":"message"}}`)
	response := performDelivery(server.Handler(), body, "wrong-token", "delivery-key-0001")
	if response.Code != http.StatusUnauthorized || store.enqueueCalls != 0 {
		t.Fatalf("response = %d, enqueue calls = %d", response.Code, store.enqueueCalls)
	}
}

func TestDeliveryEndpointRejectsOversizedBody(t *testing.T) {
	store := &fakeStore{}
	server := newTestServer(t, store, fakeSender{provider: "fcm"})
	server.maxBody = 64
	body := []byte(`{"provider":"fcm","device_token":"device-token-123","instance_id":"instance-1","payload":{"event_id":"event-1","event_type":"message"}}`)
	response := performDelivery(server.Handler(), body, "publisher-token-that-is-at-least-32-characters", "delivery-key-0001")
	if response.Code != http.StatusRequestEntityTooLarge || store.enqueueCalls != 0 {
		t.Fatalf("response = %d, enqueue calls = %d", response.Code, store.enqueueCalls)
	}
}

func TestDeliveryEndpointRejectsNonCallAPNSPushKitPayload(t *testing.T) {
	store := &fakeStore{}
	server := newTestServer(t, store, fakeSender{provider: "apns"})
	body := []byte(`{"provider":"apns","device_token":"voip-token-123","instance_id":"instance-1","payload":{"event_id":"event-1","event_type":"message_created"}}`)

	response := performDelivery(server.Handler(), body, "publisher-token-that-is-at-least-32-characters", "delivery-key-apns-0001")
	if response.Code != http.StatusUnprocessableEntity || store.enqueueCalls != 0 {
		t.Fatalf("response = %d %s, enqueue calls = %d", response.Code, response.Body.String(), store.enqueueCalls)
	}
}

func TestMaintenanceMovesCrashAtFinalAttemptToDeadAndScrubsDeliveryData(t *testing.T) {
	store := &fakeStore{
		crashedFinal: true,
		job: Job{
			Status: "processing", AttemptCount: 8, MaxAttempts: 8,
			DeviceToken: "device-token-secret", Payload: []byte(`{"body":"private"}`),
		},
	}
	server := newTestServer(t, store, fakeSender{provider: "fcm"})

	server.reapExpiredProcessing(context.Background())

	if store.reapCalls != 1 || store.job.Status != "dead" || store.job.DeviceToken != "[discarded]" || string(store.job.Payload) != `{}` {
		t.Fatalf("expired final attempt was not safely reaped: calls=%d job=%+v payload=%s", store.reapCalls, store.job, store.job.Payload)
	}
}

func TestMetricsEndpointExposesOnlyAggregateQueueState(t *testing.T) {
	store := &fakeStore{
		job: Job{PublisherID: "private-publisher", DeviceToken: "device-token-secret", Payload: []byte(`{"body":"private"}`)},
		stats: QueueStats{
			Pending: 2, Processing: 3, Retry: 4, Sent: 5, Dead: 6,
			Sent24Hours: 9, Dead24Hours: 1, OldestQueuedAgeSeconds: 42.5,
		},
	}
	server := newTestServer(t, store, fakeSender{provider: "fcm"})
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	body := response.Body.String()
	for _, expected := range []string{
		`webtui_push_relay_jobs{status="pending"} 2.000000`,
		`webtui_push_relay_jobs{status="processing"} 3.000000`,
		`webtui_push_relay_jobs{status="retry"} 4.000000`,
		`webtui_push_relay_jobs{status="sent"} 5.000000`,
		`webtui_push_relay_jobs{status="dead"} 6.000000`,
		`webtui_push_relay_sent_jobs_24h 9.000000`,
		`webtui_push_relay_dead_jobs_24h 1.000000`,
		`webtui_push_relay_oldest_queued_age_seconds 42.500000`,
		`webtui_push_relay_delivery_rate_ratio_24h 0.900000`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metrics missing %q:\n%s", expected, body)
		}
	}
	if response.Code != http.StatusOK || strings.Contains(body, "private-publisher") || strings.Contains(body, "device-token-secret") || strings.Contains(body, "private") {
		t.Fatalf("metrics response leaked private state or returned a bad status: %d\n%s", response.Code, body)
	}
	if count := strings.Count(body, "# HELP webtui_push_relay_jobs "); count != 1 {
		t.Fatalf("queue metric family has %d HELP declarations:\n%s", count, body)
	}
}

func TestMetricsEndpointReportsCollectionFailureWithoutStaleQueueValues(t *testing.T) {
	store := &fakeStore{statsErr: errors.New("database unavailable")}
	server := newTestServer(t, store, fakeSender{provider: "fcm"})
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "webtui_push_relay_metrics_collection_success 0.000000") || strings.Contains(body, "webtui_push_relay_jobs{") {
		t.Fatalf("unexpected failed metrics snapshot: %d\n%s", response.Code, body)
	}
}

func TestProcessOneRedactsPermanentProviderFailure(t *testing.T) {
	store := &fakeStore{job: Job{
		ID: "4d9b657a-3a54-45e1-a57e-ea8079ba87bd", PublisherID: "instance-1",
		Provider: "fcm", DeviceToken: "device-token-secret",
		Payload: []byte(`{"event_id":"event-1","event_type":"message"}`),
		Status:  "processing", AttemptCount: 1, MaxAttempts: 8,
	}}
	server := newTestServer(t, store, fakeSender{
		provider: "fcm", err: pusherror.PermanentError(errors.New("unregistered device-token-secret\nprovider detail")),
	})
	if err := server.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !store.markPermanent || strings.Contains(store.markReason, "device-token-secret") || strings.Contains(store.markReason, "\n") {
		t.Fatalf("permanent = %v, reason = %q", store.markPermanent, store.markReason)
	}
}

func TestManualReplayMetadataIsAcceptedButNotForwardedToProvider(t *testing.T) {
	received := map[string]any{}
	store := &fakeStore{job: Job{
		ID: "4d9b657a-3a54-45e1-a57e-ea8079ba87bd", PublisherID: "instance-1",
		Provider: "fcm", DeviceToken: "device-token-secret",
		Payload: []byte(`{"event_id":"event-1","event_type":"message","manual_replay_of":"2f08fbeb-c565-45b0-a1be-61798e55d2aa","manual_replay_at":"2026-08-03T00:00:00Z"}`),
		Status:  "processing", AttemptCount: 1, MaxAttempts: 8,
	}}
	server := newTestServer(t, store, fakeSender{provider: "fcm", received: received})
	request := deliveryRequest{
		Provider: "fcm", DeviceToken: "device-token-secret", InstanceID: "instance-1",
		Payload: map[string]any{
			"event_id": "event-1", "event_type": "message",
			"manual_replay_of": "2f08fbeb-c565-45b0-a1be-61798e55d2aa",
			"manual_replay_at": "2026-08-03T00:00:00Z",
		},
	}
	if err := server.validateRequest(request); err != nil {
		t.Fatalf("validateRequest() rejected replay metadata: %v", err)
	}
	if err := server.ProcessOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !store.markSent {
		t.Fatal("replayed relay job was not marked sent")
	}
	if received["event_id"] != "event-1" || received["manual_replay_of"] != nil || received["manual_replay_at"] != nil {
		t.Fatalf("provider payload contains internal replay metadata: %#v", received)
	}
}

func TestValidateRequestRejectsUnsafeDeepLink(t *testing.T) {
	server := newTestServer(t, &fakeStore{}, fakeSender{provider: "fcm"})
	err := server.validateRequest(deliveryRequest{
		Provider: "fcm", DeviceToken: "device-token-123", InstanceID: "instance-1",
		Payload: map[string]any{"event_id": "event-1", "event_type": "message", "deep_link": "https://evil.example"},
	})
	if err == nil {
		t.Fatal("validateRequest() accepted an external deep link")
	}
}

func TestPublisherLimiterEnforcesBurstAndRefills(t *testing.T) {
	limiter := newPublisherLimiter(60, 2)
	now := time.Now()
	if !limiter.Allow("instance-1", now) || !limiter.Allow("instance-1", now) {
		t.Fatal("limiter rejected requests inside the configured burst")
	}
	if limiter.Allow("instance-1", now) {
		t.Fatal("limiter allowed a request above the configured burst")
	}
	if !limiter.Allow("instance-1", now.Add(time.Second)) {
		t.Fatal("limiter did not refill at the configured per-minute rate")
	}
}

func newTestServer(t *testing.T, store Store, sender PushSender) *Server {
	t.Helper()
	server, err := New(store, Config{
		Publishers:   map[string]string{"instance-1": "publisher-token-that-is-at-least-32-characters"},
		MaxBodyBytes: 4096, RateLimitPerMinute: 1000, RateLimitBurst: 100,
	}, sender)
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func performDelivery(handler http.Handler, body []byte, token string, idempotencyKey string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/v1/deliveries", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Idempotency-Key", idempotencyKey)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestJobResponseDoesNotExposePrivateFields(t *testing.T) {
	encoded, err := json.Marshal(Job{ID: "id", PublisherID: "publisher", DeviceToken: "secret", Payload: []byte("secret")})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "publisher") || strings.Contains(string(encoded), "secret") {
		t.Fatalf("Job JSON exposed a private field: %s", encoded)
	}
}
