package relayserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/pusherror"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	relayLeaseTimeout      = 2 * time.Minute
	relayRecoveryInterval  = 30 * time.Second
	relayRetentionInterval = 6 * time.Hour
	relayTracerName        = "github.com/duclamdev/application-chat/backend/push-relay"
)

var (
	publisherIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,199}$`)
	uuidPattern        = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	eventTypePattern   = regexp.MustCompile(`^[a-z0-9_.:-]{1,64}$`)
)

var allowedPayloadKeys = map[string]int{
	"event_id": 256, "event_type": 64, "target_type": 64,
	"workspace_id": 128, "channel_id": 128, "message_id": 128, "sender_id": 128,
	"title": 160, "body": 512, "deep_link": 1024, "tag": 128,
	"call_id": 128, "initiator_user_id": 128, "target_user_id": 128,
	"mode": 32, "status": 32, "caller_name": 160, "app_name": 160,
	"logo_url": 1024, "manual_replay_of": 128, "manual_replay_at": 64,
}

var displayPayloadLimits = map[string]int{
	"title": 160, "body": 512, "caller_name": 160, "app_name": 160,
}

type PushSender interface {
	Provider() string
	Enabled() bool
	Send(context.Context, string, map[string]any) error
}

type Config struct {
	Publishers         map[string]string
	MaxBodyBytes       int64
	RateLimitPerMinute int
	RateLimitBurst     int
	WorkerConcurrency  int
	PollInterval       time.Duration
	Retention          time.Duration
	MaxAttempts        int
}

type Server struct {
	store       Store
	publishers  map[string][32]byte
	senders     map[string]PushSender
	maxBody     int64
	concurrency int
	poll        time.Duration
	retention   time.Duration
	maxAttempts int
	limiter     *publisherLimiter
	httpServer  *http.Server
}

type deliveryRequest struct {
	Provider    string         `json:"provider"`
	DeviceToken string         `json:"device_token"`
	InstanceID  string         `json:"instance_id"`
	Payload     map[string]any `json:"payload"`
}

func New(store Store, config Config, senders ...PushSender) (*Server, error) {
	if store == nil {
		return nil, errors.New("relay store is required")
	}
	publishers := make(map[string][32]byte, len(config.Publishers))
	for rawID, rawToken := range config.Publishers {
		id := strings.ToLower(strings.TrimSpace(rawID))
		token := strings.TrimSpace(rawToken)
		if !publisherIDPattern.MatchString(id) || len(token) < 32 {
			return nil, errors.New("relay publisher configuration is invalid")
		}
		publishers[id] = sha256.Sum256([]byte(token))
	}
	if len(publishers) == 0 {
		return nil, errors.New("at least one relay publisher is required")
	}
	providers := make(map[string]PushSender)
	for _, sender := range senders {
		if sender == nil || !sender.Enabled() {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(sender.Provider()))
		if provider == "fcm" || provider == "apns" {
			providers[provider] = sender
		}
	}
	if len(providers) == 0 {
		return nil, errors.New("at least one relay push provider is required")
	}
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = 32768
	}
	if config.WorkerConcurrency <= 0 {
		config.WorkerConcurrency = 4
	}
	if config.PollInterval <= 0 {
		config.PollInterval = time.Second
	}
	if config.Retention <= 0 {
		config.Retention = 7 * 24 * time.Hour
	}
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 8
	}
	return &Server{
		store: store, publishers: publishers, senders: providers,
		maxBody: config.MaxBodyBytes, concurrency: config.WorkerConcurrency,
		poll: config.PollInterval, retention: config.Retention, maxAttempts: config.MaxAttempts,
		limiter: newPublisherLimiter(config.RateLimitPerMinute, config.RateLimitBurst),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("POST /v1/deliveries", s.handleDelivery)
	mux.HandleFunc("GET /v1/deliveries/{job_id}", s.handleJob)
	return s.accessLog(s.traceHTTP(mux))
}

func (s *Server) Run(ctx context.Context, addr string) error {
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	var workers sync.WaitGroup
	for index := 0; index < s.concurrency; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			s.workerLoop(workerCtx)
		}()
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		s.cleanupLoop(workerCtx)
	}()

	s.httpServer = &http.Server{
		Addr: addr, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second,
		IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("push relay listening", "addr", addr, "workers", s.concurrency)
		errCh <- s.httpServer.ListenAndServe()
	}()

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errCh:
		if errors.Is(runErr, http.ErrServerClosed) {
			runErr = nil
		}
	}
	cancelWorkers()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil && runErr == nil {
		runErr = err
	}
	workers.Wait()
	return runErr
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil || len(s.senders) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

func (s *Server) handleDelivery(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeProblem(w, http.StatusUnsupportedMediaType, "CONTENT_TYPE_REQUIRED", "Content-Type must be application/json.")
		return
	}
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if !idempotencyPattern.MatchString(idempotencyKey) {
		writeProblem(w, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "A valid Idempotency-Key header is required.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request deliveryRequest
	if err := decoder.Decode(&request); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeProblem(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Request exceeds the configured size limit.")
			return
		}
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body is invalid.")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeProblem(w, http.StatusBadRequest, "INVALID_REQUEST", "Request body must contain one JSON object.")
		return
	}
	publisherID := strings.ToLower(strings.TrimSpace(request.InstanceID))
	if !s.authenticate(publisherID, r.Header.Get("Authorization")) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="push-relay"`)
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "Publisher authentication failed.")
		return
	}
	if !s.limiter.Allow(publisherID, time.Now()) {
		w.Header().Set("Retry-After", "60")
		writeProblem(w, http.StatusTooManyRequests, "RATE_LIMITED", "Publisher rate limit exceeded.")
		return
	}
	request.Provider = strings.ToLower(strings.TrimSpace(request.Provider))
	request.DeviceToken = strings.TrimSpace(request.DeviceToken)
	request.InstanceID = publisherID
	if err := s.validateRequest(request); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "INVALID_DELIVERY", err.Error())
		return
	}
	normalized, err := json.Marshal(request)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not normalize request.")
		return
	}
	requestHash := sha256.Sum256(normalized)
	payload, _ := json.Marshal(request.Payload)
	job, deduplicated, err := s.store.Enqueue(r.Context(), EnqueueInput{
		PublisherID: publisherID, IdempotencyKey: idempotencyKey, RequestHash: requestHash[:],
		Provider: request.Provider, DeviceToken: request.DeviceToken, Payload: payload,
		MaxAttempts: s.maxAttempts,
	})
	if errors.Is(err, ErrIdempotencyConflict) {
		writeProblem(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Idempotency-Key was already used for another delivery.")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "QUEUE_UNAVAILABLE", "Delivery queue is temporarily unavailable.")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"id": job.ID, "status": job.Status, "deduplicated": deduplicated,
	})
}

func (s *Server) handleJob(w http.ResponseWriter, r *http.Request) {
	publisherID := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Push-Relay-Publisher")))
	if !s.authenticate(publisherID, r.Header.Get("Authorization")) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="push-relay"`)
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "Publisher authentication failed.")
		return
	}
	jobID := strings.TrimSpace(r.PathValue("job_id"))
	if !uuidPattern.MatchString(jobID) {
		writeProblem(w, http.StatusNotFound, "DELIVERY_NOT_FOUND", "Delivery was not found.")
		return
	}
	job, err := s.store.Get(r.Context(), publisherID, jobID)
	if errors.Is(err, ErrJobNotFound) {
		writeProblem(w, http.StatusNotFound, "DELIVERY_NOT_FOUND", "Delivery was not found.")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "QUEUE_UNAVAILABLE", "Delivery queue is temporarily unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) authenticate(publisherID string, authorization string) bool {
	expected, exists := s.publishers[publisherID]
	if !exists || !strings.HasPrefix(authorization, "Bearer ") {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	actual := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(expected[:], actual[:]) == 1
}

func (s *Server) validateRequest(request deliveryRequest) error {
	if !publisherIDPattern.MatchString(request.InstanceID) {
		return errors.New("instance_id is invalid")
	}
	if _, ok := s.senders[request.Provider]; !ok {
		return errors.New("provider is not enabled on this relay")
	}
	if len(request.DeviceToken) < 8 || len(request.DeviceToken) > 4096 || containsUnsafeControl(request.DeviceToken) || strings.IndexFunc(request.DeviceToken, unicode.IsSpace) >= 0 {
		return errors.New("device_token is invalid")
	}
	if len(request.Payload) == 0 {
		return errors.New("payload is required")
	}
	normalizeDisplayPayload(request.Payload)
	for key, value := range request.Payload {
		limit, ok := allowedPayloadKeys[key]
		if !ok {
			return fmt.Errorf("payload field %q is not allowed", key)
		}
		text, ok := value.(string)
		if !ok || utf8.RuneCountInString(text) > limit || containsUnsafeControl(text) {
			return fmt.Errorf("payload field %q is invalid", key)
		}
	}
	eventType, _ := request.Payload["event_type"].(string)
	eventType = strings.ToLower(strings.TrimSpace(eventType))
	if !eventTypePattern.MatchString(eventType) {
		return errors.New("payload event_type is required and invalid")
	}
	if request.Provider == "apns" && eventType != "call_invite" {
		return errors.New("APNs PushKit deliveries only support call_invite")
	}
	if value, ok := request.Payload["deep_link"].(string); ok && value != "" && !strings.HasPrefix(value, "webtui://") {
		return errors.New("payload deep_link must use the webtui scheme")
	}
	if value, ok := request.Payload["logo_url"].(string); ok && value != "" {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return errors.New("payload logo_url must be an HTTPS URL")
		}
	}
	encoded, _ := json.Marshal(request.Payload)
	if len(encoded) > 16384 {
		return errors.New("payload is too large")
	}
	return nil
}

func normalizeDisplayPayload(payload map[string]any) {
	for key, limit := range displayPayloadLimits {
		value, ok := payload[key].(string)
		if !ok {
			continue
		}
		value = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, value)
		value = strings.Join(strings.Fields(value), " ")
		if utf8.RuneCountInString(value) > limit {
			value = string([]rune(value)[:limit])
		}
		payload[key] = value
	}
}

func (s *Server) ProcessOne(ctx context.Context) error {
	job, err := s.store.Claim(ctx)
	if err != nil {
		return err
	}
	ctx, span := otel.Tracer(relayTracerName).Start(ctx, "push-relay deliver",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("push.provider", job.Provider),
			attribute.Int("push.attempt", job.AttemptCount),
			attribute.Int("push.max_attempts", job.MaxAttempts),
		),
	)
	defer span.End()
	var payload map[string]any
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		span.SetStatus(codes.Error, "stored payload is invalid")
		span.SetAttributes(attribute.String("push.outcome", "dead"))
		return s.store.MarkFailed(ctx, job, true, "invalid stored payload", time.Time{})
	}
	// These fields make a manually replayed application job a distinct relay
	// request, but they are internal audit metadata and must not reach FCM/APNs.
	delete(payload, "manual_replay_of")
	delete(payload, "manual_replay_at")
	sender := s.senders[job.Provider]
	if sender == nil || !sender.Enabled() {
		span.SetStatus(codes.Error, "provider is unavailable")
		span.SetAttributes(attribute.String("push.outcome", "dead"))
		return s.store.MarkFailed(ctx, job, true, "provider unavailable", time.Time{})
	}
	deliveryErr := sender.Send(ctx, job.DeviceToken, payload)
	if deliveryErr == nil {
		if err := s.store.MarkSent(ctx, job.ID); err != nil {
			span.SetStatus(codes.Error, "could not persist delivery result")
			return err
		}
		span.SetAttributes(attribute.String("push.outcome", "sent"))
		slog.Debug("push relay delivery completed", "job_id", job.ID, "publisher_id", job.PublisherID, "provider", job.Provider)
		return nil
	}
	permanent := pusherror.IsPermanent(deliveryErr)
	outcome := "retry"
	if permanent || job.AttemptCount >= job.MaxAttempts {
		outcome = "dead"
	}
	span.SetStatus(codes.Error, "provider delivery failed")
	span.SetAttributes(
		attribute.String("push.outcome", outcome),
		attribute.Bool("push.permanent_failure", permanent),
	)
	retryAt := time.Now().UTC().Add(retryDelay(job))
	reason := sanitizeDeliveryError(deliveryErr, job.DeviceToken)
	if err := s.store.MarkFailed(ctx, job, permanent, reason, retryAt); err != nil {
		return err
	}
	slog.Warn("push relay delivery failed", "job_id", job.ID, "publisher_id", job.PublisherID,
		"provider", job.Provider, "permanent", permanent, "attempt", job.AttemptCount)
	return nil
}

func (s *Server) workerLoop(ctx context.Context) {
	for {
		err := s.ProcessOne(ctx)
		if err == nil {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		wait := s.poll
		if !errors.Is(err, ErrNoJobs) {
			slog.Warn("push relay worker could not claim job")
			wait = maxDuration(wait, 2*time.Second)
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Server) cleanupLoop(ctx context.Context) {
	recoveryTicker := time.NewTicker(relayRecoveryInterval)
	retentionTicker := time.NewTicker(relayRetentionInterval)
	defer recoveryTicker.Stop()
	defer retentionTicker.Stop()
	s.reapExpiredProcessing(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-recoveryTicker.C:
			s.reapExpiredProcessing(ctx)
		case <-retentionTicker.C:
			count, err := s.store.Purge(ctx, time.Now().UTC().Add(-s.retention))
			if err != nil {
				slog.Warn("push relay retention cleanup failed")
			} else if count > 0 {
				slog.Info("push relay retention cleanup completed", "deleted", count)
			}
		}
	}
}

func (s *Server) reapExpiredProcessing(ctx context.Context) {
	count, err := s.store.ReapExpiredProcessing(ctx, time.Now().UTC().Add(-relayLeaseTimeout))
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("push relay expired lease recovery failed")
		}
		return
	}
	if count > 0 {
		slog.Warn("push relay moved exhausted expired leases to dead-letter", "count", count)
	}
}

func retryDelay(job Job) time.Duration {
	attempt := job.AttemptCount
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	delay := time.Duration(1<<(attempt-1)) * 5 * time.Second
	digest := sha256.Sum256([]byte(job.ID))
	return delay + time.Duration(digest[0]%5)*time.Second
}

func sanitizeDeliveryError(err error, token string) string {
	if err == nil {
		return "delivery failed"
	}
	value := strings.ReplaceAll(err.Error(), token, "[REDACTED]")
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	if len([]rune(value)) > 500 {
		value = string([]rune(value)[:500])
	}
	return value
}

func containsUnsafeControl(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool { return unicode.IsControl(r) }) >= 0
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *Server) traceHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := relayHTTPRoute(r)
		if route == "/health" || route == "/ready" || route == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := otel.Tracer(relayTracerName).Start(ctx, r.Method+" "+route,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", route),
			),
		)
		defer span.End()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r.WithContext(ctx))
		span.SetAttributes(attribute.Int("http.response.status_code", recorder.status))
		if recorder.status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, "server error")
		}
	})
}

func relayHTTPRoute(r *http.Request) string {
	switch {
	case r.URL.Path == "/health":
		return "/health"
	case r.URL.Path == "/ready":
		return "/ready"
	case r.URL.Path == "/metrics":
		return "/metrics"
	case r.URL.Path == "/v1/deliveries":
		return "/v1/deliveries"
	case strings.HasPrefix(r.URL.Path, "/v1/deliveries/"):
		return "/v1/deliveries/{job_id}"
	default:
		return "unmatched"
	}
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := safeRequestID(r.Header.Get("X-Request-ID"))
		w.Header().Set("X-Request-ID", requestID)
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		slog.Info("push relay request", "method", r.Method, "path", r.URL.Path,
			"status", recorder.status, "duration_ms", time.Since(started).Milliseconds(), "request_id", requestID)
	})
}

func safeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 8 && len(value) <= 128 && !containsUnsafeControl(value) {
		return value
	}
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err == nil {
		return hex.EncodeToString(raw)
	}
	return fmt.Sprintf("relay-%d", time.Now().UnixNano())
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func maxDuration(left time.Duration, right time.Duration) time.Duration {
	if left > right {
		return left
	}
	return right
}

type publisherLimiter struct {
	mu       sync.Mutex
	rate     float64
	capacity float64
	buckets  map[string]*publisherBucket
}

type publisherBucket struct {
	tokens float64
	last   time.Time
}

func newPublisherLimiter(perMinute int, burst int) *publisherLimiter {
	if perMinute <= 0 {
		perMinute = 240
	}
	if burst <= 0 {
		burst = 1
	}
	return &publisherLimiter{
		rate: float64(perMinute) / 60, capacity: float64(burst),
		buckets: make(map[string]*publisherBucket),
	}
}

func (l *publisherLimiter) Allow(publisherID string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket := l.buckets[publisherID]
	if bucket == nil {
		bucket = &publisherBucket{tokens: l.capacity, last: now}
		l.buckets[publisherID] = bucket
	}
	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed > 0 {
		bucket.tokens = minFloat(l.capacity, bucket.tokens+elapsed*l.rate)
		bucket.last = now
	}
	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens--
	return true
}

func minFloat(left float64, right float64) float64 {
	if left < right {
		return left
	}
	return right
}
