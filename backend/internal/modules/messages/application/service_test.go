package application

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	messagesdomain "github.com/duclamdev/application-chat/backend/internal/modules/messages/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

func TestFormatTimePreservesSubsecondPrecision(t *testing.T) {
	value := time.Date(2026, time.July, 12, 12, 9, 52, 493177000, time.UTC)

	if got, want := formatTime(value), "2026-07-12T12:09:52.493177Z"; got != want {
		t.Fatalf("formatTime() = %q, want %q", got, want)
	}
}

type testPermissionChecker struct {
	allowed bool
}

func (c testPermissionChecker) HasWorkspacePermission(context.Context, string, string, string) (bool, error) {
	return c.allowed, nil
}

type emptyMessageRepo struct{}

func (r emptyMessageRepo) Send(context.Context, SendParams) (messagesdomain.Message, error) {
	panic("không được gọi")
}

func (r emptyMessageRepo) Get(context.Context, MessageRef) (messagesdomain.Message, error) {
	panic("không được gọi")
}

func (r emptyMessageRepo) List(context.Context, ListParams) ([]messagesdomain.Message, error) {
	panic("không được gọi")
}

func (r emptyMessageRepo) ListThread(context.Context, ThreadParams) ([]messagesdomain.Message, error) {
	panic("không được gọi")
}

func (r emptyMessageRepo) Search(context.Context, SearchParams) ([]messagesdomain.Message, error) {
	panic("không được gọi")
}

func (r emptyMessageRepo) Forward(context.Context, ForwardParams) (messagesdomain.Message, error) {
	panic("không được gọi")
}

func (r emptyMessageRepo) Update(context.Context, UpdateParams) (messagesdomain.Message, error) {
	panic("không được gọi")
}

func (r emptyMessageRepo) Delete(context.Context, DeleteParams) error {
	panic("không được gọi")
}

func (r emptyMessageRepo) ListPins(context.Context, ListPinsParams) ([]messagesdomain.Message, error) {
	panic("không được gọi")
}

func (r emptyMessageRepo) Pin(context.Context, PinParams) (messagesdomain.Message, error) {
	panic("không được gọi")
}

func (r emptyMessageRepo) Unpin(context.Context, PinParams) error {
	panic("không được gọi")
}

func (r emptyMessageRepo) AddReaction(context.Context, ReactionParams) (messagesdomain.Message, error) {
	panic("không được gọi")
}

func (r emptyMessageRepo) RemoveReaction(context.Context, ReactionParams) (messagesdomain.Message, error) {
	panic("không được gọi")
}

type otherUserMessageRepo struct {
	emptyMessageRepo
}

type captureSendRepo struct {
	emptyMessageRepo
	sent SendParams
}

func (r *captureSendRepo) Send(_ context.Context, params SendParams) (messagesdomain.Message, error) {
	r.sent = params
	senderID := params.SenderID
	now := time.Now().UTC()
	return messagesdomain.Message{
		ID:          "message-voice",
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
		SenderID:    &senderID,
		Kind:        params.Kind,
		Body:        params.Body,
		Metadata:    params.Metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

type staleSendRepo struct {
	emptyMessageRepo
}

func (r staleSendRepo) Send(_ context.Context, params SendParams) (messagesdomain.Message, error) {
	senderID := params.SenderID
	return messagesdomain.Message{
		ID:          "message-existing",
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
		SenderID:    &senderID,
		Kind:        params.Kind,
		Body:        params.Body,
		Metadata:    params.Metadata,
		CreatedAt:   time.Now().UTC().Add(-time.Minute),
		UpdatedAt:   time.Now().UTC().Add(-time.Minute),
	}, nil
}

type captureRealtimePublisher struct {
	count int
}

func (p *captureRealtimePublisher) Publish(context.Context, RealtimeEvent) error {
	p.count++
	return nil
}

func (r otherUserMessageRepo) Get(context.Context, MessageRef) (messagesdomain.Message, error) {
	senderID := "user-a"
	return messagesdomain.Message{
		ID:          "message-1",
		WorkspaceID: "workspace-1",
		ChannelID:   "channel-1",
		SenderID:    &senderID,
		Body:        "Tin nhắn của A",
	}, nil
}

func (r otherUserMessageRepo) Update(context.Context, UpdateParams) (messagesdomain.Message, error) {
	panic("không được sửa tin nhắn của người khác")
}

func TestNormalizeMentionsDeduplicatesExplicitAndBodyMentions(t *testing.T) {
	body := "Chào <@22222222-2222-2222-2222-222222222222> và <@11111111-1111-1111-1111-111111111111>"
	got := normalizeMentions(body, []string{
		" 22222222-2222-2222-2222-222222222222 ",
		"33333333-3333-3333-3333-333333333333",
	})

	want := []string{
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
		"33333333-3333-3333-3333-333333333333",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeMentions() = %#v, muốn %#v", got, want)
	}
}

func TestSendAddsClientMessageIDToMetadata(t *testing.T) {
	repo := &captureSendRepo{}
	service := NewService(repo, testPermissionChecker{allowed: true})

	_, err := service.Send(context.Background(), SendInput{
		ActorUserID:     "11111111-1111-1111-1111-111111111111",
		Body:            "Hello",
		ChannelID:       "22222222-2222-2222-2222-222222222222",
		ClientMessageID: "client-123",
		Metadata:        json.RawMessage(`{"source":"desktop"}`),
		WorkspaceID:     "33333333-3333-3333-3333-333333333333",
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if repo.sent.ClientMessageID != "client-123" {
		t.Fatalf("ClientMessageID = %q, want client-123", repo.sent.ClientMessageID)
	}
	var metadata map[string]string
	if err := json.Unmarshal(repo.sent.Metadata, &metadata); err != nil {
		t.Fatalf("metadata is not object: %v", err)
	}
	if metadata["client_message_id"] != "client-123" || metadata["source"] != "desktop" {
		t.Fatalf("metadata = %#v, want client_message_id and source preserved", metadata)
	}
}

func TestSendDoesNotRepublishDuplicateClientMessage(t *testing.T) {
	realtime := &captureRealtimePublisher{}
	service := NewService(staleSendRepo{}, testPermissionChecker{allowed: true}, realtime)

	_, err := service.Send(context.Background(), SendInput{
		ActorUserID:     "11111111-1111-1111-1111-111111111111",
		Body:            "Hello",
		ChannelID:       "22222222-2222-2222-2222-222222222222",
		ClientMessageID: "client-123",
		WorkspaceID:     "33333333-3333-3333-3333-333333333333",
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if realtime.count != 0 {
		t.Fatalf("Publish count = %d, want 0 for duplicate retry", realtime.count)
	}
}

func TestUpdateRejectsEditingOtherUsersMessage(t *testing.T) {
	service := NewService(otherUserMessageRepo{}, testPermissionChecker{allowed: true})

	_, err := service.Update(context.Background(), UpdateInput{
		ActorUserID: "user-b",
		WorkspaceID: "workspace-1",
		ChannelID:   "channel-1",
		MessageID:   "message-1",
		Body:        "B sửa tin nhắn của A",
	})
	if err == nil {
		t.Fatal("Update() phải từ chối khi người dùng sửa tin nhắn của người khác")
	}

	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("lỗi = %T, muốn AppError", err)
	}
	if appErr.Code != "FORBIDDEN" {
		t.Fatalf("mã lỗi = %q, muốn FORBIDDEN", appErr.Code)
	}
}

func TestDeleteRejectsInvalidMessageIDBeforeRepository(t *testing.T) {
	service := NewService(emptyMessageRepo{}, testPermissionChecker{allowed: true})

	err := service.Delete(context.Background(), DeleteInput{
		ActorUserID: "11111111-1111-1111-1111-111111111111",
		WorkspaceID: "22222222-2222-2222-2222-222222222222",
		ChannelID:   "33333333-3333-3333-3333-333333333333",
		MessageID:   "local-voice-message",
	})
	if err == nil {
		t.Fatal("Delete() phai tra loi khi message id khong hop le")
	}

	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("loi = %T, muon AppError", err)
	}
	if appErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("ma loi = %q, muon VALIDATION_ERROR", appErr.Code)
	}
}

func TestSendRejectsEmptyTextMessage(t *testing.T) {
	service := NewService(emptyMessageRepo{}, testPermissionChecker{allowed: true})

	_, err := service.Send(context.Background(), SendInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		ChannelID:   "channel-1",
		Body:        "   ",
	})
	if err == nil {
		t.Fatal("Send() phải trả lỗi khi nội dung rỗng")
	}

	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("lỗi = %T, muốn AppError", err)
	}
	if appErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("mã lỗi = %q", appErr.Code)
	}
}

func TestSendAcceptsVoiceMediaMessage(t *testing.T) {
	repo := &captureSendRepo{}
	service := NewService(repo, testPermissionChecker{allowed: true})

	message, err := service.Send(context.Background(), SendInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		ChannelID:   "channel-1",
		Kind:        "file",
		Body:        "Đã gửi tin nhắn thoại",
		Metadata:    []byte(`{"message_type":"voice"}`),
	})
	if err != nil {
		t.Fatalf("Send() trả lỗi: %v", err)
	}
	if message.Kind != "file" || repo.sent.Kind != "file" {
		t.Fatalf("kind = %q, muốn file", message.Kind)
	}
}

func TestSendAcceptsCallEventMessage(t *testing.T) {
	repo := &captureSendRepo{}
	service := NewService(repo, testPermissionChecker{allowed: true})

	message, err := service.Send(context.Background(), SendInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		ChannelID:   "channel-1",
		Kind:        "event",
		Body:        "Cuoc goi thoai",
		Metadata:    []byte(`{"message_type":"call","call_status":"completed"}`),
	})
	if err != nil {
		t.Fatalf("Send() tra loi: %v", err)
	}
	if message.Kind != "event" || repo.sent.Kind != "event" {
		t.Fatalf("kind = %q, muon event", message.Kind)
	}
}

func TestParseSearchDateUsesExclusiveEndDate(t *testing.T) {
	got, err := parseSearchDate("2026-07-10", true)
	if err != nil {
		t.Fatalf("parseSearchDate() trả lỗi: %v", err)
	}
	want := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	if got == nil || !got.Equal(want) {
		t.Fatalf("parseSearchDate() = %v, muốn %v", got, want)
	}
}

func TestForwardRequiresTargetChannel(t *testing.T) {
	service := NewService(emptyMessageRepo{}, testPermissionChecker{allowed: true})
	_, err := service.Forward(context.Background(), ForwardInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		ChannelID:   "channel-1",
		MessageID:   "message-1",
	})
	if err == nil {
		t.Fatal("Forward() phải yêu cầu target channel")
	}
}
