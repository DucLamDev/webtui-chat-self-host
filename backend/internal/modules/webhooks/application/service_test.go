package application

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	aptokensapp "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/application"
	outboxdomain "github.com/duclamdev/application-chat/backend/internal/modules/outbox/domain"
	webhooksdomain "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/domain"
	webhooksecurity "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/security"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type fakeWebhookRepo struct {
	deliveryParams       OutboxDeliveryParams
	deliveryCalled       bool
	createOutgoingParams CreateOutgoingParams
	incomingParams       IncomingMessageParams
	integrationParams    IntegrationMessageParams
}

func (r *fakeWebhookRepo) CreateIncoming(context.Context, CreateIncomingParams) (webhooksdomain.IncomingWebhook, error) {
	return webhooksdomain.IncomingWebhook{}, nil
}

func (r *fakeWebhookRepo) ListIncoming(context.Context, string) ([]webhooksdomain.IncomingWebhook, error) {
	return nil, nil
}

func (r *fakeWebhookRepo) UpdateIncoming(context.Context, UpdateIncomingParams) (webhooksdomain.IncomingWebhook, error) {
	return webhooksdomain.IncomingWebhook{}, nil
}

func (r *fakeWebhookRepo) DeleteIncoming(context.Context, string, string) error {
	return nil
}

func (r *fakeWebhookRepo) CreateOutgoing(_ context.Context, params CreateOutgoingParams) (webhooksdomain.OutgoingWebhook, error) {
	r.createOutgoingParams = params
	return webhooksdomain.OutgoingWebhook{
		ID:                     "outgoing-1",
		WorkspaceID:            params.WorkspaceID,
		Name:                   params.Name,
		TargetURL:              params.TargetURL,
		SigningSecretEncrypted: params.SigningSecretEncrypted,
		EventTypes:             params.EventTypes,
		Status:                 "active",
	}, nil
}

func (r *fakeWebhookRepo) ListOutgoing(context.Context, string) ([]webhooksdomain.OutgoingWebhook, error) {
	return nil, nil
}

func (r *fakeWebhookRepo) UpdateOutgoing(context.Context, UpdateOutgoingParams) (webhooksdomain.OutgoingWebhook, error) {
	return webhooksdomain.OutgoingWebhook{}, nil
}

func (r *fakeWebhookRepo) DeleteOutgoing(context.Context, string, string) error {
	return nil
}

func (r *fakeWebhookRepo) ListDeliveries(context.Context, string, string, int) ([]webhooksdomain.Delivery, error) {
	return nil, nil
}

func (r *fakeWebhookRepo) CreateTestDelivery(context.Context, TestDeliveryParams) (webhooksdomain.Delivery, error) {
	return webhooksdomain.Delivery{}, nil
}

func (r *fakeWebhookRepo) SendIncomingMessage(_ context.Context, params IncomingMessageParams) (webhooksdomain.IntegrationMessage, error) {
	r.incomingParams = params
	return webhooksdomain.IntegrationMessage{ID: "message-1"}, nil
}

func (r *fakeWebhookRepo) SendIntegrationMessage(_ context.Context, params IntegrationMessageParams) (webhooksdomain.IntegrationMessage, error) {
	r.integrationParams = params
	return webhooksdomain.IntegrationMessage{ID: "message-1"}, nil
}

func (r *fakeWebhookRepo) CreateDeliveriesForEvent(_ context.Context, params OutboxDeliveryParams) (int, error) {
	r.deliveryParams = params
	r.deliveryCalled = true
	return 1, nil
}

func (r *fakeWebhookRepo) ClaimDeliveries(context.Context, int) ([]webhooksdomain.Delivery, error) {
	return nil, nil
}

func (r *fakeWebhookRepo) MarkDeliverySuccess(context.Context, string, int, string) error {
	return nil
}

func (r *fakeWebhookRepo) MarkDeliveryFailed(context.Context, string, int, string, time.Duration, int) error {
	return nil
}

type fakeTokenAuth struct {
	result aptokensapp.AuthenticatedTokenDTO
	err    error
}

func (f fakeTokenAuth) Authenticate(context.Context, string, string) (aptokensapp.AuthenticatedTokenDTO, error) {
	if f.err != nil {
		return aptokensapp.AuthenticatedTokenDTO{}, f.err
	}
	if f.result.ZoneID == "" && f.result.WorkspaceID == "" {
		return aptokensapp.AuthenticatedTokenDTO{ZoneID: "zone-1", WorkspaceID: "workspace-1"}, nil
	}
	return f.result, nil
}

type allowWebhookPermission struct{}

func (allowWebhookPermission) HasWorkspacePermission(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func TestHandleCreatesOutgoingDeliveriesFromOutboxEvent(t *testing.T) {
	repo := &fakeWebhookRepo{}
	service := NewService(repo, nil, fakeTokenAuth{}, nil, "")
	payload := map[string]any{
		"workspace_id": "workspace-1",
		"channel_id":   "channel-1",
		"message_id":   "message-1",
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() trả lỗi: %v", err)
	}

	err = service.Handle(context.Background(), outboxdomain.Event{
		ID:            "11111111-1111-1111-1111-111111111111",
		AggregateType: "message",
		AggregateID:   "message-1",
		EventType:     "MessageCreated",
		EventVersion:  1,
		Payload:       payloadBytes,
		CreatedAt:     time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Handle() trả lỗi: %v", err)
	}
	if !repo.deliveryCalled {
		t.Fatal("Handle() phải tạo webhook delivery")
	}
	if repo.deliveryParams.WorkspaceID != "workspace-1" || repo.deliveryParams.EventType != "MessageCreated" {
		t.Fatalf("delivery params không đúng: %#v", repo.deliveryParams)
	}
}

func TestDispatchIncomingRequiresAndForwardsExpectedZone(t *testing.T) {
	repo := &fakeWebhookRepo{}
	service := NewService(repo, nil, fakeTokenAuth{}, nil, "")
	service.SetLegalDocumentVersions("terms-v2", "privacy-v3")

	if _, err := service.DispatchIncoming(context.Background(), IncomingMessageInput{
		WebhookID: "webhook-1",
		Secret:    "secret",
		Body:      "hello",
	}); err == nil {
		t.Fatal("DispatchIncoming() expected missing zone error")
	}

	_, err := service.DispatchIncoming(context.Background(), IncomingMessageInput{
		ExpectedZoneID: "zone-1",
		WebhookID:      "webhook-1",
		Secret:         "secret",
		Body:           "hello",
	})
	if err != nil {
		t.Fatalf("DispatchIncoming() error = %v", err)
	}
	if repo.incomingParams.ExpectedZoneID != "zone-1" {
		t.Fatalf("expected zone = %q", repo.incomingParams.ExpectedZoneID)
	}
	if repo.incomingParams.TermsVersion != "terms-v2" || repo.incomingParams.PrivacyVersion != "privacy-v3" {
		t.Fatalf("legal versions = (%q, %q)", repo.incomingParams.TermsVersion, repo.incomingParams.PrivacyVersion)
	}
}

func TestSendTokenMessageRejectsAnotherZone(t *testing.T) {
	repo := &fakeWebhookRepo{}
	service := NewService(repo, nil, fakeTokenAuth{}, nil, "")

	if _, err := service.SendTokenMessage(context.Background(), TokenMessageInput{
		ExpectedZoneID: "zone-2",
		Token:          "token",
		ChannelID:      "channel-1",
		Body:           "hello",
	}); err == nil {
		t.Fatal("SendTokenMessage() expected zone mismatch error")
	}
	if repo.integrationParams.WorkspaceID != "" {
		t.Fatal("repository must not receive cross-zone token message")
	}
}

func TestSendTokenMessageRequiresOwnerAndForwardsTransactionalPolicy(t *testing.T) {
	repo := &fakeWebhookRepo{}
	service := NewService(repo, nil, fakeTokenAuth{
		result: aptokensapp.AuthenticatedTokenDTO{
			TokenID:     "token-1",
			ZoneID:      "zone-1",
			WorkspaceID: "workspace-1",
		},
	}, nil, "")
	service.SetLegalDocumentVersions("terms-v2", "privacy-v3")

	_, err := service.SendTokenMessage(context.Background(), TokenMessageInput{
		ExpectedZoneID: "zone-1",
		Token:          "token",
		ChannelID:      "channel-1",
		Body:           "hello",
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "UNAUTHORIZED" || appErr.Status != http.StatusUnauthorized {
		t.Fatalf("ownerless token error = %#v, want UNAUTHORIZED/401", err)
	}
	if repo.integrationParams.WorkspaceID != "" {
		t.Fatal("ownerless token must not reach message persistence")
	}

	ownerID := "owner-1"
	service.tokenAuth = fakeTokenAuth{
		result: aptokensapp.AuthenticatedTokenDTO{
			TokenID:     "token-1",
			ZoneID:      "zone-1",
			WorkspaceID: "workspace-1",
			OwnerID:     &ownerID,
		},
	}
	if _, err := service.SendTokenMessage(context.Background(), TokenMessageInput{
		ExpectedZoneID: "zone-1",
		Token:          "token",
		ChannelID:      "channel-1",
		Body:           "hello",
	}); err != nil {
		t.Fatalf("SendTokenMessage() error = %v", err)
	}
	got := repo.integrationParams
	if got.ExpectedZoneID != "zone-1" || got.TokenID != "token-1" || got.OwnerUserID != ownerID || got.WorkspaceID != "workspace-1" {
		t.Fatalf("credential policy params = %#v", got)
	}
	if got.TermsVersion != "terms-v2" || got.PrivacyVersion != "privacy-v3" {
		t.Fatalf("legal versions = (%q, %q)", got.TermsVersion, got.PrivacyVersion)
	}
}

func TestIntegrationLegalPolicyErrorsHaveStablePublicContracts(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   string
		wantStatus int
	}{
		{name: "missing or stale acceptance", err: webhooksdomain.ErrIntegrationLegalRequired, wantCode: "LEGAL_ACCEPTANCE_REQUIRED", wantStatus: http.StatusConflict},
		{name: "policy unavailable", err: webhooksdomain.ErrIntegrationLegalUnavailable, wantCode: "LEGAL_ACCEPTANCE_UNAVAILABLE", wantStatus: http.StatusServiceUnavailable},
		{name: "credential revoked after authentication", err: webhooksdomain.ErrIntegrationCredentialStale, wantCode: "UNAUTHORIZED", wantStatus: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var appErr *apperrors.AppError
			if err := mapWebhookError(test.err); !errors.As(err, &appErr) || appErr.Code != test.wantCode || appErr.Status != test.wantStatus {
				t.Fatalf("mapWebhookError() = %#v, want %s/%d", err, test.wantCode, test.wantStatus)
			}
		})
	}
}

func TestIntegrationMessageFailsClosedWithoutConfiguredLegalVersions(t *testing.T) {
	repo := &fakeWebhookRepo{}
	service := NewService(repo, nil, fakeTokenAuth{}, nil, "")
	_, err := service.DispatchIncoming(context.Background(), IncomingMessageInput{
		ExpectedZoneID: "zone-1",
		WebhookID:      "webhook-1",
		Secret:         "secret",
		Body:           "hello",
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "LEGAL_ACCEPTANCE_UNAVAILABLE" || appErr.Status != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured integration legal policy error = %#v", err)
	}
	if repo.incomingParams.WebhookID != "" {
		t.Fatal("unconfigured legal policy must fail before message persistence")
	}
}

func TestHandleIgnoresEventWithoutWorkspace(t *testing.T) {
	repo := &fakeWebhookRepo{}
	service := NewService(repo, nil, nil, nil, "")

	err := service.Handle(context.Background(), outboxdomain.Event{
		ID:        "event-1",
		EventType: "SystemEvent",
		Payload:   []byte(`{"message_id":"message-1"}`),
	})
	if err != nil {
		t.Fatalf("Handle() trả lỗi: %v", err)
	}
	if repo.deliveryCalled {
		t.Fatal("Handle() không được tạo delivery khi event thiếu workspace_id")
	}
}

func TestCreateOutgoingEncryptsCustomerVisibleSigningSecret(t *testing.T) {
	const masterSecret = "test-master-secret-with-at-least-32-characters"
	repo := &fakeWebhookRepo{}
	service := NewService(repo, allowWebhookPermission{}, nil, nil, masterSecret)

	created, err := service.CreateOutgoing(context.Background(), CreateOutgoingInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		Name:        "Customer automation",
		TargetURL:   "https://hooks.customer.example/events",
		EventTypes:  []string{"MessageCreated"},
	})
	if err != nil {
		t.Fatalf("CreateOutgoing() error = %v", err)
	}
	if created.Secret == "" {
		t.Fatal("CreateOutgoing() must return the signing secret once")
	}
	if created.Secret == repo.createOutgoingParams.SigningSecretEncrypted {
		t.Fatal("CreateOutgoing() stored the customer-visible secret in plaintext")
	}
	decrypted, err := webhooksecurity.DecryptSecret(masterSecret, repo.createOutgoingParams.SigningSecretEncrypted)
	if err != nil {
		t.Fatalf("DecryptSecret() error = %v", err)
	}
	if decrypted != created.Secret {
		t.Fatalf("stored secret decrypts to %q, want returned secret %q", decrypted, created.Secret)
	}
}
