package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	syncdomain "github.com/duclamdev/application-chat/backend/internal/modules/sync/domain"
)

type fakeSyncRepo struct {
	storedCursor string
	listParams   ListParams
	ackParams    AckParams
	events       []syncdomain.Event
}

func (r *fakeSyncRepo) ListEvents(_ context.Context, params ListParams) ([]syncdomain.Event, error) {
	r.listParams = params
	return r.events, nil
}

func (r *fakeSyncRepo) Ack(_ context.Context, params AckParams) (syncdomain.CursorAck, error) {
	r.ackParams = params
	now := time.Now()
	return syncdomain.CursorAck{
		UserID:          params.UserID,
		WorkspaceID:     params.WorkspaceID,
		DeviceID:        params.DeviceID,
		CursorEventID:   &params.Cursor,
		CursorCreatedAt: &now,
		AckedAt:         now,
	}, nil
}

func (r *fakeSyncRepo) GetAckCursor(context.Context, string, string, string) (string, error) {
	return r.storedCursor, nil
}

type fakeSyncChecker struct {
	allowed bool
}

func (c fakeSyncChecker) HasWorkspacePermission(context.Context, string, string, string) (bool, error) {
	return c.allowed, nil
}

func TestCatchUpUsesStoredDeviceCursor(t *testing.T) {
	repo := &fakeSyncRepo{
		storedCursor: "event-1",
		events: []syncdomain.Event{
			{
				ID:            "event-2",
				AggregateType: "message",
				AggregateID:   "message-1",
				EventType:     "MessageCreated",
				EventVersion:  1,
				Payload:       json.RawMessage(`{"workspace_id":"workspace-1"}`),
				CreatedAt:     time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewService(repo, fakeSyncChecker{allowed: true})

	result, err := service.CatchUp(context.Background(), ListInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		DeviceID:    "device-1",
		Limit:       50,
	})
	if err != nil {
		t.Fatalf("CatchUp() error = %v", err)
	}
	if repo.listParams.Cursor != "event-1" {
		t.Fatalf("cursor truyền xuống repo = %q, muốn event-1", repo.listParams.Cursor)
	}
	if result.NextCursor != "event-2" || len(result.Events) != 1 {
		t.Fatalf("catch-up result không đúng: %#v", result)
	}
}

func TestCatchUpMarksHasMoreWhenLimitExceeded(t *testing.T) {
	repo := &fakeSyncRepo{
		events: []syncdomain.Event{
			{ID: "event-1", CreatedAt: time.Now()},
			{ID: "event-2", CreatedAt: time.Now()},
		},
	}
	service := NewService(repo, fakeSyncChecker{allowed: true})

	result, err := service.CatchUp(context.Background(), ListInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("CatchUp() error = %v", err)
	}
	if !result.HasMore || len(result.Events) != 1 || result.NextCursor != "event-1" {
		t.Fatalf("pagination không đúng: %#v", result)
	}
}

func TestAckRequiresDeviceAndCursor(t *testing.T) {
	service := NewService(&fakeSyncRepo{}, fakeSyncChecker{allowed: true})

	_, err := service.Ack(context.Background(), AckInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		DeviceID:    "device-1",
	})
	if err == nil {
		t.Fatal("Ack() phải trả lỗi khi thiếu cursor")
	}
}
