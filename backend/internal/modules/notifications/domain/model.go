package domain

import (
	"errors"
	"time"
)

var ErrNotificationNotFound = errors.New("không tìm thấy thông báo")

var ErrNotificationPreferenceUnavailable = errors.New("notification preference unavailable")

var ErrWebPushSubscriptionNotFound = errors.New("web push subscription not found")

var ErrWebPushSubscriptionConflict = errors.New("web push endpoint belongs to another account")

type Notification struct {
	ID          string
	UserID      string
	WorkspaceID *string
	ChannelID   *string
	MessageID   *string
	Type        string
	Title       string
	Body        string
	Data        []byte
	ReadAt      *time.Time
	DeliveredAt *time.Time
	CreatedAt   time.Time
}

type NotificationPreference struct {
	UserID       string
	WorkspaceID  string
	Mode         string
	Preview      bool
	QuietHours   bool
	QuietStart   string
	QuietEnd     string
	Sound        bool
	Vibrate      bool
	CallRinging  bool
	BadgeEnabled bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ChannelPreference struct {
	UserID      string
	WorkspaceID string
	ChannelID   string
	Mode        string
	MutedUntil  *time.Time
	Sensitive   bool
	Important   bool
	Compact     bool
	Tags        []string
	ArchivedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type WebPushSubscription struct {
	ID        string
	UserID    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
