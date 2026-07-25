package application

import (
	"context"
	"errors"
	"strings"
	"time"

	devicesdomain "github.com/duclamdev/application-chat/backend/internal/modules/push_devices/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
	WorkspaceBelongsToZone(ctx context.Context, workspaceID string, zoneID string) (bool, error)
}

type Repository interface {
	Upsert(ctx context.Context, params UpsertParams) (devicesdomain.Device, error)
	ListMine(ctx context.Context, zoneID string, userID string) ([]devicesdomain.Device, error)
	Delete(ctx context.Context, zoneID string, userID string, deviceID string) error
}

type Service struct {
	repo    Repository
	checker PermissionChecker
}

type UpsertInput struct {
	ActorUserID            string
	ZoneID                 string
	WorkspaceID            string
	DeviceID               string
	Platform               string
	PushProvider           string
	PushToken              string
	NotificationPermission string
	AppVersion             string
	BuildNumber            string
	ReleaseChannel         string
	Locale                 string
	Timezone               string
}

type UpsertParams struct {
	UserID                 string
	ZoneID                 string
	WorkspaceID            string
	DeviceID               string
	Platform               string
	PushProvider           string
	PushToken              string
	NotificationPermission string
	AppVersion             string
	BuildNumber            string
	ReleaseChannel         string
	Locale                 string
	Timezone               string
}

type DeviceDTO struct {
	ID                     string  `json:"id"`
	UserID                 string  `json:"user_id"`
	WorkspaceID            *string `json:"workspace_id,omitempty"`
	DeviceID               string  `json:"device_id"`
	Platform               string  `json:"platform"`
	PushProvider           string  `json:"push_provider"`
	HasPushToken           bool    `json:"has_push_token"`
	NotificationPermission string  `json:"notification_permission"`
	AppVersion             *string `json:"app_version,omitempty"`
	BuildNumber            *string `json:"build_number,omitempty"`
	ReleaseChannel         *string `json:"release_channel,omitempty"`
	Locale                 *string `json:"locale,omitempty"`
	Timezone               *string `json:"timezone,omitempty"`
	Status                 string  `json:"status"`
	LastSeenAt             string  `json:"last_seen_at"`
	RevokedAt              *string `json:"revoked_at,omitempty"`
	CreatedAt              string  `json:"created_at"`
	UpdatedAt              string  `json:"updated_at"`
}

func NewService(repo Repository, checker PermissionChecker) *Service {
	return &Service{repo: repo, checker: checker}
}

func (s *Service) RegisterOrUpdate(ctx context.Context, input UpsertInput) (DeviceDTO, error) {
	userID := strings.TrimSpace(input.ActorUserID)
	zoneID := strings.TrimSpace(input.ZoneID)
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if workspaceID == "" || zoneID == "" {
		return DeviceDTO{}, apperrors.BadRequest("WORKSPACE_REQUIRED", "Workspace của zone hiện tại là bắt buộc.")
	}
	if err := s.ensureWorkspaceMember(ctx, zoneID, userID, workspaceID); err != nil {
		return DeviceDTO{}, err
	}
	params, err := normalizeUpsert(input)
	if err != nil {
		return DeviceDTO{}, err
	}
	params.UserID = userID
	params.ZoneID = zoneID
	params.WorkspaceID = workspaceID
	device, err := s.repo.Upsert(ctx, params)
	if err != nil {
		return DeviceDTO{}, mapDeviceError(err)
	}
	return toDTO(device), nil
}

func (s *Service) ListMine(ctx context.Context, zoneID string, actorUserID string) ([]DeviceDTO, error) {
	devices, err := s.repo.ListMine(ctx, strings.TrimSpace(zoneID), strings.TrimSpace(actorUserID))
	if err != nil {
		return nil, err
	}
	return toDTOs(devices), nil
}

func (s *Service) Delete(ctx context.Context, zoneID string, actorUserID string, deviceID string) error {
	if strings.TrimSpace(deviceID) == "" {
		return apperrors.BadRequest("VALIDATION_ERROR", "Device ID không được để trống.")
	}
	if err := s.repo.Delete(ctx, strings.TrimSpace(zoneID), strings.TrimSpace(actorUserID), strings.TrimSpace(deviceID)); err != nil {
		return mapDeviceError(err)
	}
	return nil
}

func (s *Service) ensureWorkspaceMember(ctx context.Context, zoneID string, userID string, workspaceID string) error {
	if s.checker == nil {
		return nil
	}
	matches, err := s.checker.WorkspaceBelongsToZone(ctx, workspaceID, zoneID)
	if err != nil {
		return err
	}
	if !matches {
		return apperrors.Forbidden("Workspace không thuộc zone của phiên đăng nhập.")
	}
	allowed, err := s.checker.HasWorkspacePermission(ctx, userID, workspaceID, "workspace.view_members")
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không thuộc workspace này.")
	}
	return nil
}

func normalizeUpsert(input UpsertInput) (UpsertParams, error) {
	deviceID := strings.TrimSpace(input.DeviceID)
	if deviceID == "" || len(deviceID) > 128 {
		return UpsertParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Device ID phải có độ dài từ 1 đến 128 ký tự.")
	}
	platform := normalizePlatform(input.Platform)
	if platform == "" {
		return UpsertParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Platform thiết bị phải là android, ios, desktop hoặc web.")
	}
	pushProvider := normalizePushProvider(input.PushProvider, platform)
	permission := normalizePermission(input.NotificationPermission)
	return UpsertParams{
		DeviceID:               deviceID,
		Platform:               platform,
		PushProvider:           pushProvider,
		PushToken:              strings.TrimSpace(input.PushToken),
		NotificationPermission: permission,
		AppVersion:             trimMax(input.AppVersion, 64),
		BuildNumber:            trimMax(input.BuildNumber, 64),
		ReleaseChannel:         trimMax(input.ReleaseChannel, 32),
		Locale:                 trimMax(input.Locale, 32),
		Timezone:               trimMax(input.Timezone, 64),
	}, nil
}

func normalizePlatform(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "android", "ios", "desktop", "web":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizePushProvider(value string, platform string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "apns":
		return "apns"
	case "none":
		return "none"
	case "fcm":
		return "fcm"
	default:
		if platform == "ios" {
			return "apns"
		}
		if platform == "desktop" || platform == "web" {
			return "none"
		}
		return "fcm"
	}
}

func normalizePermission(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "granted", "denied", "provisional":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "unknown"
	}
}

func trimMax(value string, max int) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= max {
		return value
	}
	return string([]rune(value)[:max])
}

func mapDeviceError(err error) error {
	if errors.Is(err, devicesdomain.ErrDeviceNotFound) {
		return apperrors.NotFound("PUSH_DEVICE_NOT_FOUND", "Không tìm thấy thiết bị nhận thông báo.")
	}
	return err
}

func toDTOs(devices []devicesdomain.Device) []DeviceDTO {
	dtos := make([]DeviceDTO, 0, len(devices))
	for _, device := range devices {
		dtos = append(dtos, toDTO(device))
	}
	return dtos
}

func toDTO(device devicesdomain.Device) DeviceDTO {
	return DeviceDTO{
		ID:                     device.ID,
		UserID:                 device.UserID,
		WorkspaceID:            device.WorkspaceID,
		DeviceID:               device.DeviceID,
		Platform:               device.Platform,
		PushProvider:           device.PushProvider,
		HasPushToken:           device.PushToken != nil && strings.TrimSpace(*device.PushToken) != "",
		NotificationPermission: device.NotificationPermission,
		AppVersion:             device.AppVersion,
		BuildNumber:            device.BuildNumber,
		ReleaseChannel:         device.ReleaseChannel,
		Locale:                 device.Locale,
		Timezone:               device.Timezone,
		Status:                 device.Status,
		LastSeenAt:             formatTime(device.LastSeenAt),
		RevokedAt:              formatTimePtr(device.RevokedAt),
		CreatedAt:              formatTime(device.CreatedAt),
		UpdatedAt:              formatTime(device.UpdatedAt),
	}
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func formatTimePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}
