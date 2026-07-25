package domain

import (
	"errors"
	"time"
)

var (
	ErrDeviceNotFound = errors.New("không tìm thấy thiết bị nhận thông báo")
)

type Device struct {
	ID                     string
	UserID                 string
	WorkspaceID            *string
	DeviceID               string
	Platform               string
	PushProvider           string
	PushToken              *string
	NotificationPermission string
	AppVersion             *string
	BuildNumber            *string
	ReleaseChannel         *string
	Locale                 *string
	Timezone               *string
	Status                 string
	LastSeenAt             time.Time
	RevokedAt              *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}
