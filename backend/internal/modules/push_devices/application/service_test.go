package application

import (
	"context"
	"testing"
	"time"

	devicesdomain "github.com/duclamdev/application-chat/backend/internal/modules/push_devices/domain"
)

type fakeDeviceRepo struct {
	upsertCalled bool
	params       UpsertParams
}

func (r *fakeDeviceRepo) Upsert(_ context.Context, params UpsertParams) (devicesdomain.Device, error) {
	r.upsertCalled = true
	r.params = params
	now := time.Now()
	token := params.PushToken
	workspaceID := params.WorkspaceID
	return devicesdomain.Device{
		ID:                     "device-row-1",
		UserID:                 params.UserID,
		WorkspaceID:            &workspaceID,
		DeviceID:               params.DeviceID,
		Platform:               params.Platform,
		PushProvider:           params.PushProvider,
		PushToken:              &token,
		NotificationPermission: params.NotificationPermission,
		Status:                 "active",
		LastSeenAt:             now,
		CreatedAt:              now,
		UpdatedAt:              now,
	}, nil
}

func (r *fakeDeviceRepo) ListMine(context.Context, string, string) ([]devicesdomain.Device, error) {
	return nil, nil
}

func (r *fakeDeviceRepo) Delete(context.Context, string, string, string) error {
	return nil
}

type fakeDeviceChecker struct {
	allowed bool
	matches bool
	called  bool
}

func (c *fakeDeviceChecker) HasWorkspacePermission(context.Context, string, string, string) (bool, error) {
	c.called = true
	return c.allowed, nil
}

func (c *fakeDeviceChecker) WorkspaceBelongsToZone(context.Context, string, string) (bool, error) {
	return c.matches, nil
}

func TestRegisterOrUpdateDefaultsAndroidToFCM(t *testing.T) {
	repo := &fakeDeviceRepo{}
	checker := &fakeDeviceChecker{allowed: true, matches: true}
	service := NewService(repo, checker)

	device, err := service.RegisterOrUpdate(context.Background(), UpsertInput{
		ActorUserID:            "user-1",
		ZoneID:                 "zone-1",
		WorkspaceID:            "workspace-1",
		DeviceID:               "android-device-1",
		Platform:               "android",
		PushToken:              "push-token",
		NotificationPermission: "granted",
	})
	if err != nil {
		t.Fatalf("RegisterOrUpdate() error = %v", err)
	}
	if !checker.called || !repo.upsertCalled {
		t.Fatal("RegisterOrUpdate() phải kiểm tra workspace và lưu device")
	}
	if repo.params.PushProvider != "fcm" || device.PushProvider != "fcm" {
		t.Fatalf("push provider không đúng: params=%q dto=%q", repo.params.PushProvider, device.PushProvider)
	}
	if !device.HasPushToken {
		t.Fatal("DeviceDTO phải báo có push token nhưng không trả token thô")
	}
}

func TestRegisterOrUpdateRejectsWorkspaceWhenNotMember(t *testing.T) {
	repo := &fakeDeviceRepo{}
	service := NewService(repo, &fakeDeviceChecker{allowed: false, matches: true})

	_, err := service.RegisterOrUpdate(context.Background(), UpsertInput{
		ActorUserID: "user-1",
		ZoneID:      "zone-1",
		WorkspaceID: "workspace-1",
		DeviceID:    "android-device-1",
		Platform:    "android",
	})
	if err == nil {
		t.Fatal("RegisterOrUpdate() phải trả lỗi khi user không thuộc workspace")
	}
	if repo.upsertCalled {
		t.Fatal("RegisterOrUpdate() không được lưu device khi permission bị từ chối")
	}
}

func TestRegisterOrUpdateRejectsUnknownPlatform(t *testing.T) {
	repo := &fakeDeviceRepo{}
	service := NewService(repo, &fakeDeviceChecker{allowed: true, matches: true})

	_, err := service.RegisterOrUpdate(context.Background(), UpsertInput{
		ActorUserID: "user-1",
		ZoneID:      "zone-1",
		WorkspaceID: "workspace-1",
		DeviceID:    "device-1",
		Platform:    "watch",
	})
	if err == nil {
		t.Fatal("RegisterOrUpdate() phải trả lỗi khi platform không hợp lệ")
	}
	if repo.upsertCalled {
		t.Fatal("RegisterOrUpdate() không được lưu device khi validation lỗi")
	}
}
