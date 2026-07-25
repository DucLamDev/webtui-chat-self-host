package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	aptokensapp "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/application"
	outboxdomain "github.com/duclamdev/application-chat/backend/internal/modules/outbox/domain"
	webhooksdomain "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/domain"
	webhooksecurity "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/security"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const maxDeliveryRetries = 5

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type TokenAuthenticator interface {
	Authenticate(ctx context.Context, token string, requiredScope string) (aptokensapp.AuthenticatedTokenDTO, error)
}

type Repository interface {
	CreateIncoming(ctx context.Context, params CreateIncomingParams) (webhooksdomain.IncomingWebhook, error)
	ListIncoming(ctx context.Context, workspaceID string) ([]webhooksdomain.IncomingWebhook, error)
	UpdateIncoming(ctx context.Context, params UpdateIncomingParams) (webhooksdomain.IncomingWebhook, error)
	DeleteIncoming(ctx context.Context, workspaceID string, incomingWebhookID string) error
	CreateOutgoing(ctx context.Context, params CreateOutgoingParams) (webhooksdomain.OutgoingWebhook, error)
	ListOutgoing(ctx context.Context, workspaceID string) ([]webhooksdomain.OutgoingWebhook, error)
	UpdateOutgoing(ctx context.Context, params UpdateOutgoingParams) (webhooksdomain.OutgoingWebhook, error)
	DeleteOutgoing(ctx context.Context, workspaceID string, outgoingWebhookID string) error
	ListDeliveries(ctx context.Context, workspaceID string, outgoingWebhookID string, limit int) ([]webhooksdomain.Delivery, error)
	CreateTestDelivery(ctx context.Context, params TestDeliveryParams) (webhooksdomain.Delivery, error)
	SendIncomingMessage(ctx context.Context, params IncomingMessageParams) (webhooksdomain.IntegrationMessage, error)
	SendIntegrationMessage(ctx context.Context, params IntegrationMessageParams) (webhooksdomain.IntegrationMessage, error)
	CreateDeliveriesForEvent(ctx context.Context, params OutboxDeliveryParams) (int, error)
	ClaimDeliveries(ctx context.Context, limit int) ([]webhooksdomain.Delivery, error)
	MarkDeliverySuccess(ctx context.Context, deliveryID string, responseStatus int, responseBody string) error
	MarkDeliveryFailed(ctx context.Context, deliveryID string, responseStatus int, responseBody string, retryAfter time.Duration, maxRetries int) error
}

type DeliverySender interface {
	Send(ctx context.Context, delivery webhooksdomain.Delivery) (DeliveryResult, error)
}

type Service struct {
	repo                Repository
	checker             PermissionChecker
	tokenAuth           TokenAuthenticator
	sender              DeliverySender
	signingMasterSecret string
}

type CreateIncomingInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	Name        string
}

type CreateIncomingParams struct {
	WorkspaceID string
	ChannelID   string
	Name        string
	SecretHash  string
	CreatedBy   string
}

type UpdateIncomingInput struct {
	ActorUserID string
	WorkspaceID string
	WebhookID   string
	ChannelID   *string
	Name        *string
	Status      *string
}

type UpdateIncomingParams struct {
	WorkspaceID string
	WebhookID   string
	ChannelID   *string
	Name        *string
	Status      *string
}

type CreateOutgoingInput struct {
	ActorUserID string
	WorkspaceID string
	Name        string
	TargetURL   string
	EventTypes  []string
}

type CreateOutgoingParams struct {
	WorkspaceID            string
	Name                   string
	TargetURL              string
	SigningSecretEncrypted string
	EventTypes             []string
	CreatedBy              string
}

type UpdateOutgoingInput struct {
	ActorUserID string
	WorkspaceID string
	WebhookID   string
	Name        *string
	TargetURL   *string
	EventTypes  *[]string
	Status      *string
}

type UpdateOutgoingParams struct {
	WorkspaceID string
	WebhookID   string
	Name        *string
	TargetURL   *string
	EventTypes  *[]string
	Status      *string
}

type TestOutgoingInput struct {
	ActorUserID string
	WorkspaceID string
	WebhookID   string
	EventType   string
	Payload     json.RawMessage
}

type TestDeliveryParams struct {
	WorkspaceID       string
	OutgoingWebhookID string
	EventType         string
	RequestBody       []byte
}

type IncomingMessageInput struct {
	ExpectedZoneID string
	WebhookID      string
	Secret         string
	ChannelID      string
	Body           string
	Metadata       json.RawMessage
}

type IncomingMessageParams struct {
	ExpectedZoneID string
	WebhookID      string
	SecretHash     string
	ChannelID      string
	Body           string
	Metadata       []byte
}

type TokenMessageInput struct {
	ExpectedZoneID string
	Token          string
	ChannelID      string
	Body           string
	Metadata       json.RawMessage
}

type IntegrationMessageParams struct {
	WorkspaceID string
	ChannelID   string
	Kind        string
	Body        string
	Metadata    []byte
}

type OutboxDeliveryParams struct {
	EventID     string
	EventType   string
	WorkspaceID string
	RequestBody []byte
}

type DeliveryResult struct {
	StatusCode int
	Body       string
}

type IncomingWebhookDTO struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	ChannelID   *string `json:"channel_id,omitempty"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	CreatedBy   *string `json:"created_by,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	LastUsedAt  *string `json:"last_used_at,omitempty"`
}

type CreatedIncomingWebhookDTO struct {
	IncomingWebhookDTO
	Secret string `json:"secret"`
	URL    string `json:"url"`
}

type OutgoingWebhookDTO struct {
	ID          string   `json:"id"`
	WorkspaceID string   `json:"workspace_id"`
	Name        string   `json:"name"`
	TargetURL   string   `json:"target_url"`
	EventTypes  []string `json:"event_types"`
	Status      string   `json:"status"`
	CreatedBy   *string  `json:"created_by,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type CreatedOutgoingWebhookDTO struct {
	OutgoingWebhookDTO
	Secret string `json:"secret"`
}

type DeliveryDTO struct {
	ID                string          `json:"id"`
	OutgoingWebhookID string          `json:"outgoing_webhook_id"`
	EventID           *string         `json:"event_id,omitempty"`
	EventType         string          `json:"event_type"`
	RequestBody       json.RawMessage `json:"request_body"`
	ResponseStatus    *int            `json:"response_status,omitempty"`
	ResponseBody      *string         `json:"response_body,omitempty"`
	Status            string          `json:"status"`
	AttemptCount      int             `json:"attempt_count"`
	NextAttemptAt     *string         `json:"next_attempt_at,omitempty"`
	DeliveredAt       *string         `json:"delivered_at,omitempty"`
	CreatedAt         string          `json:"created_at"`
	UpdatedAt         string          `json:"updated_at"`
}

type IntegrationMessageDTO struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspace_id"`
	ChannelID   string          `json:"channel_id"`
	Kind        string          `json:"kind"`
	Body        string          `json:"body"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   string          `json:"created_at"`
}

func NewService(
	repo Repository,
	checker PermissionChecker,
	tokenAuth TokenAuthenticator,
	sender DeliverySender,
	signingMasterSecret string,
) *Service {
	return &Service{
		repo:                repo,
		checker:             checker,
		tokenAuth:           tokenAuth,
		sender:              sender,
		signingMasterSecret: signingMasterSecret,
	}
}

func (s *Service) CreateIncoming(ctx context.Context, input CreateIncomingInput, appURL string) (CreatedIncomingWebhookDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return CreatedIncomingWebhookDTO{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 120 {
		return CreatedIncomingWebhookDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên incoming webhook phải dài từ 1 đến 120 ký tự.")
	}
	secret, secretHash, err := newSecret("wtiw")
	if err != nil {
		return CreatedIncomingWebhookDTO{}, apperrors.Internal("Không tạo được secret webhook.")
	}
	webhook, err := s.repo.CreateIncoming(ctx, CreateIncomingParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		Name:        name,
		SecretHash:  secretHash,
		CreatedBy:   strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return CreatedIncomingWebhookDTO{}, mapWebhookError(err)
	}
	dto := CreatedIncomingWebhookDTO{
		IncomingWebhookDTO: toIncomingDTO(webhook),
		Secret:             secret,
		URL:                strings.TrimRight(appURL, "/") + "/api/v1/hooks/incoming/" + webhook.ID,
	}
	return dto, nil
}

func (s *Service) ListIncoming(ctx context.Context, actorUserID string, workspaceID string) ([]IncomingWebhookDTO, error) {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	webhooks, err := s.repo.ListIncoming(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	return toIncomingDTOs(webhooks), nil
}

func (s *Service) UpdateIncoming(ctx context.Context, input UpdateIncomingInput) (IncomingWebhookDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return IncomingWebhookDTO{}, err
	}
	if strings.TrimSpace(input.WebhookID) == "" {
		return IncomingWebhookDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Thieu incoming webhook_id.")
	}
	name := cleanStringPtr(input.Name)
	if name != nil && (*name == "" || len([]rune(*name)) > 120) {
		return IncomingWebhookDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Ten incoming webhook phai dai tu 1 den 120 ky tu.")
	}
	status := cleanStringPtr(input.Status)
	if status != nil && !validWebhookStatus(*status) {
		return IncomingWebhookDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Trang thai incoming webhook khong hop le.")
	}
	webhook, err := s.repo.UpdateIncoming(ctx, UpdateIncomingParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		WebhookID:   strings.TrimSpace(input.WebhookID),
		ChannelID:   cleanStringPtr(input.ChannelID),
		Name:        name,
		Status:      status,
	})
	if err != nil {
		return IncomingWebhookDTO{}, mapWebhookError(err)
	}
	return toIncomingDTO(webhook), nil
}

func (s *Service) DeleteIncoming(ctx context.Context, actorUserID string, workspaceID string, incomingWebhookID string) error {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return err
	}
	if err := s.repo.DeleteIncoming(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(incomingWebhookID)); err != nil {
		return mapWebhookError(err)
	}
	return nil
}

func (s *Service) CreateOutgoing(ctx context.Context, input CreateOutgoingInput) (CreatedOutgoingWebhookDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return CreatedOutgoingWebhookDTO{}, err
	}
	name := strings.TrimSpace(input.Name)
	targetURL := strings.TrimSpace(input.TargetURL)
	if name == "" || len([]rune(name)) > 120 {
		return CreatedOutgoingWebhookDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên outgoing webhook phải dài từ 1 đến 120 ký tự.")
	}
	if !validHTTPURL(targetURL) {
		return CreatedOutgoingWebhookDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Target URL của outgoing webhook không hợp lệ.")
	}
	secret, _, err := newSecret("wtow")
	if err != nil {
		return CreatedOutgoingWebhookDTO{}, apperrors.Internal("Không tạo được secret webhook.")
	}
	secretEncrypted, err := webhooksecurity.EncryptSecret(s.signingMasterSecret, secret)
	if err != nil {
		return CreatedOutgoingWebhookDTO{}, apperrors.Internal("Không mã hóa được secret webhook. Kiểm tra WEBHOOK_SIGNING_SECRET.")
	}
	webhook, err := s.repo.CreateOutgoing(ctx, CreateOutgoingParams{
		WorkspaceID:            strings.TrimSpace(input.WorkspaceID),
		Name:                   name,
		TargetURL:              targetURL,
		SigningSecretEncrypted: secretEncrypted,
		EventTypes:             normalizeEventTypes(input.EventTypes),
		CreatedBy:              strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return CreatedOutgoingWebhookDTO{}, mapWebhookError(err)
	}
	return CreatedOutgoingWebhookDTO{OutgoingWebhookDTO: toOutgoingDTO(webhook), Secret: secret}, nil
}

func (s *Service) ListOutgoing(ctx context.Context, actorUserID string, workspaceID string) ([]OutgoingWebhookDTO, error) {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	webhooks, err := s.repo.ListOutgoing(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	return toOutgoingDTOs(webhooks), nil
}

func (s *Service) UpdateOutgoing(ctx context.Context, input UpdateOutgoingInput) (OutgoingWebhookDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return OutgoingWebhookDTO{}, err
	}
	if strings.TrimSpace(input.WebhookID) == "" {
		return OutgoingWebhookDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Thieu outgoing webhook_id.")
	}
	name := cleanStringPtr(input.Name)
	if name != nil && (*name == "" || len([]rune(*name)) > 120) {
		return OutgoingWebhookDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Ten outgoing webhook phai dai tu 1 den 120 ky tu.")
	}
	targetURL := cleanStringPtr(input.TargetURL)
	if targetURL != nil && !validHTTPURL(*targetURL) {
		return OutgoingWebhookDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Target URL cua outgoing webhook khong hop le.")
	}
	status := cleanStringPtr(input.Status)
	if status != nil && !validWebhookStatus(*status) {
		return OutgoingWebhookDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Trang thai outgoing webhook khong hop le.")
	}
	eventTypes := input.EventTypes
	if eventTypes != nil {
		normalized := normalizeEventTypes(*eventTypes)
		eventTypes = &normalized
	}
	webhook, err := s.repo.UpdateOutgoing(ctx, UpdateOutgoingParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		WebhookID:   strings.TrimSpace(input.WebhookID),
		Name:        name,
		TargetURL:   targetURL,
		EventTypes:  eventTypes,
		Status:      status,
	})
	if err != nil {
		return OutgoingWebhookDTO{}, mapWebhookError(err)
	}
	return toOutgoingDTO(webhook), nil
}

func (s *Service) DeleteOutgoing(ctx context.Context, actorUserID string, workspaceID string, outgoingWebhookID string) error {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return err
	}
	if err := s.repo.DeleteOutgoing(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(outgoingWebhookID)); err != nil {
		return mapWebhookError(err)
	}
	return nil
}

func (s *Service) ListDeliveries(ctx context.Context, actorUserID string, workspaceID string, outgoingWebhookID string, limit int) ([]DeliveryDTO, error) {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	deliveries, err := s.repo.ListDeliveries(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(outgoingWebhookID), limit)
	if err != nil {
		return nil, err
	}
	return toDeliveryDTOs(deliveries), nil
}

func (s *Service) TestOutgoing(ctx context.Context, input TestOutgoingInput) (DeliveryDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return DeliveryDTO{}, err
	}
	eventType := strings.TrimSpace(input.EventType)
	if eventType == "" {
		eventType = "webhook.test"
	}
	requestBody, err := normalizeJSON(input.Payload, "Payload test webhook khong phai JSON hop le.")
	if err != nil {
		return DeliveryDTO{}, err
	}
	delivery, err := s.repo.CreateTestDelivery(ctx, TestDeliveryParams{
		WorkspaceID:       strings.TrimSpace(input.WorkspaceID),
		OutgoingWebhookID: strings.TrimSpace(input.WebhookID),
		EventType:         eventType,
		RequestBody:       requestBody,
	})
	if err != nil {
		return DeliveryDTO{}, mapWebhookError(err)
	}
	if s.sender == nil {
		return toDeliveryDTO(delivery), nil
	}
	result, sendErr := s.sender.Send(ctx, delivery)
	now := time.Now().UTC()
	if sendErr != nil {
		if err := s.repo.MarkDeliveryFailed(ctx, delivery.ID, result.StatusCode, result.Body, retryDelay(delivery.AttemptCount), maxDeliveryRetries); err != nil {
			return DeliveryDTO{}, err
		}
		delivery.Status = "failed"
		delivery.AttemptCount++
		if result.StatusCode > 0 {
			responseStatus := result.StatusCode
			delivery.ResponseStatus = &responseStatus
		}
		delivery.ResponseBody = stringPtr(trimResponsePreview(result.Body))
		nextAttempt := now.Add(retryDelay(delivery.AttemptCount - 1))
		delivery.NextAttemptAt = &nextAttempt
		delivery.UpdatedAt = now
		return toDeliveryDTO(delivery), nil
	}
	if err := s.repo.MarkDeliverySuccess(ctx, delivery.ID, result.StatusCode, result.Body); err != nil {
		return DeliveryDTO{}, err
	}
	delivery.Status = "success"
	delivery.AttemptCount++
	responseStatus := result.StatusCode
	delivery.ResponseStatus = &responseStatus
	delivery.ResponseBody = stringPtr(trimResponsePreview(result.Body))
	delivery.DeliveredAt = &now
	delivery.UpdatedAt = now
	return toDeliveryDTO(delivery), nil
}

func (s *Service) DispatchIncoming(ctx context.Context, input IncomingMessageInput) (IntegrationMessageDTO, error) {
	input.ExpectedZoneID = strings.TrimSpace(input.ExpectedZoneID)
	if input.ExpectedZoneID == "" {
		return IntegrationMessageDTO{}, apperrors.NotFound("ZONE_NOT_FOUND", "Domain hien tai khong thuoc zone chat dang hoat dong.")
	}
	body := strings.TrimSpace(input.Body)
	if body == "" || len([]rune(body)) > 8000 {
		return IntegrationMessageDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Nội dung incoming webhook phải dài từ 1 đến 8000 ký tự.")
	}
	metadata, err := normalizeJSON(input.Metadata, "Metadata incoming webhook không phải JSON hợp lệ.")
	if err != nil {
		return IntegrationMessageDTO{}, err
	}
	message, err := s.repo.SendIncomingMessage(ctx, IncomingMessageParams{
		ExpectedZoneID: input.ExpectedZoneID,
		WebhookID:      strings.TrimSpace(input.WebhookID),
		SecretHash:     hashSecret(strings.TrimSpace(input.Secret)),
		ChannelID:      strings.TrimSpace(input.ChannelID),
		Body:           body,
		Metadata:       metadata,
	})
	if err != nil {
		return IntegrationMessageDTO{}, mapWebhookError(err)
	}
	return toIntegrationMessageDTO(message), nil
}

func (s *Service) SendTokenMessage(ctx context.Context, input TokenMessageInput) (IntegrationMessageDTO, error) {
	input.ExpectedZoneID = strings.TrimSpace(input.ExpectedZoneID)
	if input.ExpectedZoneID == "" {
		return IntegrationMessageDTO{}, apperrors.NotFound("ZONE_NOT_FOUND", "Domain hien tai khong thuoc zone chat dang hoat dong.")
	}
	if s.tokenAuth == nil {
		return IntegrationMessageDTO{}, apperrors.Unauthorized("API token chưa được cấu hình.")
	}
	authenticated, err := s.tokenAuth.Authenticate(ctx, input.Token, "message.write")
	if err != nil {
		return IntegrationMessageDTO{}, err
	}
	if authenticated.ZoneID == "" || authenticated.ZoneID != input.ExpectedZoneID {
		return IntegrationMessageDTO{}, apperrors.Forbidden("API token khong thuoc domain/zone hien tai.")
	}
	body := strings.TrimSpace(input.Body)
	if body == "" || len([]rune(body)) > 8000 {
		return IntegrationMessageDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Nội dung API token message phải dài từ 1 đến 8000 ký tự.")
	}
	metadata, err := normalizeJSON(input.Metadata, "Metadata API token message không phải JSON hợp lệ.")
	if err != nil {
		return IntegrationMessageDTO{}, err
	}
	message, err := s.repo.SendIntegrationMessage(ctx, IntegrationMessageParams{
		WorkspaceID: authenticated.WorkspaceID,
		ChannelID:   strings.TrimSpace(input.ChannelID),
		Kind:        "event",
		Body:        body,
		Metadata:    metadata,
	})
	if err != nil {
		return IntegrationMessageDTO{}, mapWebhookError(err)
	}
	return toIntegrationMessageDTO(message), nil
}

func (s *Service) Handle(ctx context.Context, event outboxdomain.Event) error {
	var payload map[string]any
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	workspaceID, _ := payload["workspace_id"].(string)
	if strings.TrimSpace(workspaceID) == "" {
		return nil
	}
	body, err := json.Marshal(map[string]any{
		"event_id":       event.ID,
		"event_type":     event.EventType,
		"event_version":  event.EventVersion,
		"aggregate_type": event.AggregateType,
		"aggregate_id":   event.AggregateID,
		"payload":        payload,
		"occurred_at":    event.CreatedAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	_, err = s.repo.CreateDeliveriesForEvent(ctx, OutboxDeliveryParams{
		EventID:     event.ID,
		EventType:   event.EventType,
		WorkspaceID: workspaceID,
		RequestBody: body,
	})
	return err
}

func (s *Service) ProcessDeliveries(ctx context.Context, limit int) (int, error) {
	if s.sender == nil {
		return 0, nil
	}
	if limit <= 0 {
		limit = 50
	}
	deliveries, err := s.repo.ClaimDeliveries(ctx, limit)
	if err != nil {
		return 0, err
	}
	for _, delivery := range deliveries {
		result, err := s.sender.Send(ctx, delivery)
		if err != nil {
			_ = s.repo.MarkDeliveryFailed(ctx, delivery.ID, result.StatusCode, result.Body, retryDelay(delivery.AttemptCount), maxDeliveryRetries)
			continue
		}
		if err := s.repo.MarkDeliverySuccess(ctx, delivery.ID, result.StatusCode, result.Body); err != nil {
			return 0, err
		}
	}
	return len(deliveries), nil
}

func (s *Service) ensureManagePermission(ctx context.Context, userID string, workspaceID string) error {
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), "webhook.manage")
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền quản lý webhook.")
	}
	return nil
}

func newSecret(prefix string) (string, string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	secret := prefix + "_" + base64.RawURLEncoding.EncodeToString(b[:])
	return secret, hashSecret(secret), nil
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func normalizeEventTypes(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func cleanStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	return &cleaned
}

func validWebhookStatus(status string) bool {
	return status == "active" || status == "disabled"
}

func normalizeJSON(value json.RawMessage, message string) ([]byte, error) {
	if len(value) == 0 || strings.TrimSpace(string(value)) == "" || strings.TrimSpace(string(value)) == "null" {
		return []byte(`{}`), nil
	}
	if !json.Valid(value) {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", message)
	}
	return []byte(value), nil
}

func validHTTPURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func retryDelay(attemptCount int) time.Duration {
	delay := time.Duration(attemptCount+1) * 30 * time.Second
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

func trimResponsePreview(value string) string {
	if len(value) > 4096 {
		return value[:4096]
	}
	return value
}

func stringPtr(value string) *string {
	return &value
}

func mapWebhookError(err error) error {
	if errors.Is(err, webhooksdomain.ErrWebhookQuotaExceeded) {
		return apperrors.Conflict("ZONE_QUOTA_EXCEEDED", "Zone da dat gioi han webhook.")
	}
	if errors.Is(err, webhooksdomain.ErrIncomingWebhookNotFound) {
		return apperrors.NotFound("INCOMING_WEBHOOK_NOT_FOUND", "Không tìm thấy incoming webhook hoặc secret không hợp lệ.")
	}
	if errors.Is(err, webhooksdomain.ErrOutgoingWebhookNotFound) {
		return apperrors.NotFound("OUTGOING_WEBHOOK_NOT_FOUND", "Không tìm thấy outgoing webhook.")
	}
	if errors.Is(err, webhooksdomain.ErrIntegrationChannelNotFound) {
		return apperrors.BadRequest("CHANNEL_NOT_FOUND", "Không tìm thấy kênh để gửi message tích hợp.")
	}
	return err
}

func toIncomingDTOs(webhooks []webhooksdomain.IncomingWebhook) []IncomingWebhookDTO {
	dtos := make([]IncomingWebhookDTO, 0, len(webhooks))
	for _, webhook := range webhooks {
		dtos = append(dtos, toIncomingDTO(webhook))
	}
	return dtos
}

func toIncomingDTO(webhook webhooksdomain.IncomingWebhook) IncomingWebhookDTO {
	return IncomingWebhookDTO{
		ID:          webhook.ID,
		WorkspaceID: webhook.WorkspaceID,
		ChannelID:   webhook.ChannelID,
		Name:        webhook.Name,
		Status:      webhook.Status,
		CreatedBy:   webhook.CreatedBy,
		CreatedAt:   formatTime(webhook.CreatedAt),
		UpdatedAt:   formatTime(webhook.UpdatedAt),
		LastUsedAt:  formatOptionalTime(webhook.LastUsedAt),
	}
}

func toOutgoingDTOs(webhooks []webhooksdomain.OutgoingWebhook) []OutgoingWebhookDTO {
	dtos := make([]OutgoingWebhookDTO, 0, len(webhooks))
	for _, webhook := range webhooks {
		dtos = append(dtos, toOutgoingDTO(webhook))
	}
	return dtos
}

func toOutgoingDTO(webhook webhooksdomain.OutgoingWebhook) OutgoingWebhookDTO {
	return OutgoingWebhookDTO{
		ID:          webhook.ID,
		WorkspaceID: webhook.WorkspaceID,
		Name:        webhook.Name,
		TargetURL:   webhook.TargetURL,
		EventTypes:  webhook.EventTypes,
		Status:      webhook.Status,
		CreatedBy:   webhook.CreatedBy,
		CreatedAt:   formatTime(webhook.CreatedAt),
		UpdatedAt:   formatTime(webhook.UpdatedAt),
	}
}

func toDeliveryDTOs(deliveries []webhooksdomain.Delivery) []DeliveryDTO {
	dtos := make([]DeliveryDTO, 0, len(deliveries))
	for _, delivery := range deliveries {
		dtos = append(dtos, toDeliveryDTO(delivery))
	}
	return dtos
}

func toDeliveryDTO(delivery webhooksdomain.Delivery) DeliveryDTO {
	body := json.RawMessage(delivery.RequestBody)
	if len(body) == 0 {
		body = json.RawMessage(`{}`)
	}
	return DeliveryDTO{
		ID:                delivery.ID,
		OutgoingWebhookID: delivery.OutgoingWebhookID,
		EventID:           delivery.EventID,
		EventType:         delivery.EventType,
		RequestBody:       body,
		ResponseStatus:    delivery.ResponseStatus,
		ResponseBody:      delivery.ResponseBody,
		Status:            delivery.Status,
		AttemptCount:      delivery.AttemptCount,
		NextAttemptAt:     formatOptionalTime(delivery.NextAttemptAt),
		DeliveredAt:       formatOptionalTime(delivery.DeliveredAt),
		CreatedAt:         formatTime(delivery.CreatedAt),
		UpdatedAt:         formatTime(delivery.UpdatedAt),
	}
}

func toIntegrationMessageDTO(message webhooksdomain.IntegrationMessage) IntegrationMessageDTO {
	metadata := json.RawMessage(message.Metadata)
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	return IntegrationMessageDTO{
		ID:          message.ID,
		WorkspaceID: message.WorkspaceID,
		ChannelID:   message.ChannelID,
		Kind:        message.Kind,
		Body:        message.Body,
		Metadata:    metadata,
		CreatedAt:   formatTime(message.CreatedAt),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}
