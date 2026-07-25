package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	presencedomain "github.com/duclamdev/application-chat/backend/internal/modules/presence/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type fakePresenceChecker struct {
	allowed bool
}

func (c fakePresenceChecker) HasWorkspacePermission(context.Context, string, string, string) (bool, error) {
	return c.allowed, nil
}

type fakePresenceRepo struct {
	upsertParams UpsertParams
	upsertCalled bool
}

func (r *fakePresenceRepo) Upsert(_ context.Context, params UpsertParams) (presencedomain.Presence, error) {
	r.upsertParams = params
	r.upsertCalled = true
	workspaceID := params.WorkspaceID
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	return presencedomain.Presence{
		UserID:          params.UserID,
		WorkspaceID:     &workspaceID,
		DeviceID:        params.DeviceID,
		SocketID:        params.SocketID,
		NodeID:          params.NodeID,
		Status:          params.Status,
		LastHeartbeatAt: now,
		ConnectedAt:     now,
		Metadata:        params.Metadata,
	}, nil
}

func (r *fakePresenceRepo) List(context.Context, string, int) ([]presencedomain.Presence, error) {
	return nil, nil
}

func (r *fakePresenceRepo) MarkOfflineStale(context.Context, time.Duration) (int, error) {
	return 0, nil
}

func TestHeartbeatDefaultsSocketNodeAndStatus(t *testing.T) {
	repo := &fakePresenceRepo{}
	service := NewService(repo, fakePresenceChecker{allowed: true})

	dto, err := service.Heartbeat(context.Background(), UpsertInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		DeviceID:    "device-1",
		Metadata:    json.RawMessage(`{"platform":"desktop"}`),
	})
	if err != nil {
		t.Fatalf("Heartbeat() trả lỗi: %v", err)
	}
	if !repo.upsertCalled {
		t.Fatal("Heartbeat() phải gọi repository")
	}
	if repo.upsertParams.SocketID != "device-1" || repo.upsertParams.NodeID != "api-local" || repo.upsertParams.Status != "online" {
		t.Fatalf("upsert params không đúng: %#v", repo.upsertParams)
	}
	if dto.Status != "online" || string(dto.Metadata) != `{"platform":"desktop"}` {
		t.Fatalf("dto không đúng: %#v", dto)
	}
}

func TestHeartbeatRejectsUserOutsideWorkspace(t *testing.T) {
	repo := &fakePresenceRepo{}
	service := NewService(repo, fakePresenceChecker{allowed: false})

	_, err := service.Heartbeat(context.Background(), UpsertInput{
		ActorUserID: "user-1",
		WorkspaceID: "workspace-1",
		DeviceID:    "device-1",
	})
	if err == nil {
		t.Fatal("Heartbeat() phải trả lỗi khi user không thuộc workspace")
	}
	if repo.upsertCalled {
		t.Fatal("Heartbeat() không được gọi repository khi chưa đủ quyền")
	}
	if _, ok := err.(*apperrors.AppError); !ok {
		t.Fatalf("lỗi = %T, muốn AppError", err)
	}
}
