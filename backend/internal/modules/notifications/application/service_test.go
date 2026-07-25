package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	notificationsdomain "github.com/duclamdev/application-chat/backend/internal/modules/notifications/domain"
	outboxdomain "github.com/duclamdev/application-chat/backend/internal/modules/outbox/domain"
)

type fakeNotificationRepo struct {
	mentionParams       MentionParams
	mentionCalled       bool
	messageParams       MessageNotificationParams
	messageCalled       bool
	preference          notificationsdomain.NotificationPreference
	upsertCalled        bool
	channelPreference   notificationsdomain.ChannelPreference
	channelUpsertCalled bool
}

func (r *fakeNotificationRepo) CreateMentionNotifications(_ context.Context, params MentionParams) error {
	r.mentionParams = params
	r.mentionCalled = true
	return nil
}

func (r *fakeNotificationRepo) CreateMessageNotifications(_ context.Context, params MessageNotificationParams) error {
	r.messageParams = params
	r.messageCalled = true
	return nil
}

func (r *fakeNotificationRepo) CreateIncomingCallNotification(context.Context, CallNotificationParams) error {
	return nil
}

func (r *fakeNotificationRepo) UpdateCallNotification(context.Context, CallNotificationParams) error {
	return nil
}

func (r *fakeNotificationRepo) ListForUser(context.Context, ListParams) ([]notificationsdomain.Notification, error) {
	return nil, nil
}

func (r *fakeNotificationRepo) GetPreference(context.Context, string, string, string) (notificationsdomain.NotificationPreference, error) {
	if r.preference.CreatedAt.IsZero() {
		r.preference.CreatedAt = time.Now()
	}
	if r.preference.UpdatedAt.IsZero() {
		r.preference.UpdatedAt = r.preference.CreatedAt
	}
	return r.preference, nil
}

func (r *fakeNotificationRepo) MarkRead(context.Context, string, string, string) (notificationsdomain.Notification, error) {
	return notificationsdomain.Notification{}, nil
}

func (r *fakeNotificationRepo) MarkAllRead(context.Context, string, string, string) error {
	return nil
}

func (r *fakeNotificationRepo) ProcessPendingJobs(context.Context, int) (int, error) {
	return 0, nil
}

func (r *fakeNotificationRepo) UpsertPreference(_ context.Context, _ string, preference notificationsdomain.NotificationPreference) (notificationsdomain.NotificationPreference, error) {
	r.preference = preference
	r.upsertCalled = true
	if r.preference.CreatedAt.IsZero() {
		r.preference.CreatedAt = time.Now()
	}
	if r.preference.UpdatedAt.IsZero() {
		r.preference.UpdatedAt = r.preference.CreatedAt
	}
	return r.preference, nil
}

func (r *fakeNotificationRepo) GetChannelPreference(context.Context, string, string, string, string) (notificationsdomain.ChannelPreference, error) {
	if r.channelPreference.CreatedAt.IsZero() {
		r.channelPreference.CreatedAt = time.Now()
	}
	if r.channelPreference.UpdatedAt.IsZero() {
		r.channelPreference.UpdatedAt = r.channelPreference.CreatedAt
	}
	return r.channelPreference, nil
}

func (r *fakeNotificationRepo) UpsertChannelPreference(_ context.Context, _ string, preference notificationsdomain.ChannelPreference) (notificationsdomain.ChannelPreference, error) {
	r.channelPreference = preference
	r.channelUpsertCalled = true
	if r.channelPreference.CreatedAt.IsZero() {
		r.channelPreference.CreatedAt = time.Now()
	}
	if r.channelPreference.UpdatedAt.IsZero() {
		r.channelPreference.UpdatedAt = r.channelPreference.CreatedAt
	}
	return r.channelPreference, nil
}

func TestHandleCreatesMentionNotificationsFromMessageCreatedEvent(t *testing.T) {
	repo := &fakeNotificationRepo{}
	service := NewService(repo)
	payload := map[string]any{
		"workspace_id":       "workspace-1",
		"channel_id":         "channel-1",
		"message_id":         "message-1",
		"sender_id":          "sender-1",
		"mentioned_user_ids": []string{"user-1", "user-2"},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() trả lỗi: %v", err)
	}

	err = service.Handle(context.Background(), outboxdomain.Event{
		ID:           "event-1",
		EventType:    "MessageCreated",
		Payload:      payloadBytes,
		EventVersion: 1,
		CreatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("Handle() trả lỗi: %v", err)
	}
	if !repo.messageCalled {
		t.Fatal("Handle() phải tạo notification khi message có mention")
	}
	if repo.messageParams.EventID != "event-1" || repo.messageParams.MessageID != "message-1" {
		t.Fatalf("mention params không đúng: %#v", repo.mentionParams)
	}
	if len(repo.messageParams.MentionedUserIDs) != 2 {
		t.Fatalf("mentioned_user_ids = %#v", repo.mentionParams.MentionedUserIDs)
	}
}

func TestUpsertPreferenceValidatesAndStoresDesktopPolicy(t *testing.T) {
	repo := &fakeNotificationRepo{}
	service := NewService(repo)
	preview := false
	quietHours := true

	preference, err := service.UpsertPreference(context.Background(), PreferenceInput{
		UserID:      "user-1",
		WorkspaceID: "workspace-1",
		Mode:        "mentions",
		Preview:     &preview,
		QuietHours:  &quietHours,
		QuietStart:  "21:30",
		QuietEnd:    "06:45",
	})
	if err != nil {
		t.Fatalf("UpsertPreference() error = %v", err)
	}
	if !repo.upsertCalled {
		t.Fatal("UpsertPreference() phải lưu preference")
	}
	if preference.Mode != "mentions" || preference.Preview || !preference.QuietHours {
		t.Fatalf("preference không đúng: %#v", preference)
	}
	if repo.preference.QuietStart != "21:30" || repo.preference.QuietEnd != "06:45" {
		t.Fatalf("quiet hours không đúng: %#v", repo.preference)
	}
}

func TestUpsertPreferenceRejectsInvalidMode(t *testing.T) {
	repo := &fakeNotificationRepo{}
	service := NewService(repo)

	_, err := service.UpsertPreference(context.Background(), PreferenceInput{
		UserID:      "user-1",
		WorkspaceID: "workspace-1",
		Mode:        "everything",
	})
	if err == nil {
		t.Fatal("UpsertPreference() phải trả lỗi với mode không hợp lệ")
	}
	if repo.upsertCalled {
		t.Fatal("UpsertPreference() không được ghi repo khi input sai")
	}
}

func TestUpsertPreferenceStoresMobileToggles(t *testing.T) {
	repo := &fakeNotificationRepo{}
	service := NewService(repo)
	sound := false
	vibrate := false
	callRinging := false
	badgeEnabled := false

	preference, err := service.UpsertPreference(context.Background(), PreferenceInput{
		UserID:       "user-1",
		WorkspaceID:  "workspace-1",
		Mode:         "all",
		Sound:        &sound,
		Vibrate:      &vibrate,
		CallRinging:  &callRinging,
		BadgeEnabled: &badgeEnabled,
	})
	if err != nil {
		t.Fatalf("UpsertPreference() error = %v", err)
	}
	if preference.Sound || preference.Vibrate || preference.CallRinging || preference.BadgeEnabled {
		t.Fatalf("mobile toggles không đúng: %#v", preference)
	}
}

func TestUpsertChannelPreferenceStoresMute(t *testing.T) {
	repo := &fakeNotificationRepo{}
	service := NewService(repo)

	preference, err := service.UpsertChannelPreference(context.Background(), ChannelPreferenceInput{
		UserID:      "user-1",
		WorkspaceID: "workspace-1",
		ChannelID:   "channel-1",
		Mode:        "muted",
		MutedUntil:  "2026-07-15T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("UpsertChannelPreference() error = %v", err)
	}
	if !repo.channelUpsertCalled {
		t.Fatal("UpsertChannelPreference() phải lưu preference theo kênh")
	}
	if preference.Mode != "muted" || preference.MutedUntil == nil {
		t.Fatalf("channel preference không đúng: %#v", preference)
	}
}

func TestHandleIgnoresMessageWithoutMentions(t *testing.T) {
	repo := &fakeNotificationRepo{}
	service := NewService(repo)

	err := service.Handle(context.Background(), outboxdomain.Event{
		ID:        "event-1",
		EventType: "MessageCreated",
		Payload:   []byte(`{"workspace_id":"workspace-1","mentioned_user_ids":[]}`),
	})
	if err != nil {
		t.Fatalf("Handle() trả lỗi: %v", err)
	}
	if repo.mentionCalled {
		t.Fatal("Handle() không được tạo notification khi không có mention")
	}
}
