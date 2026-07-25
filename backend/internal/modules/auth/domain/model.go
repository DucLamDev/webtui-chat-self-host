package domain

import (
	"errors"
	"time"
)

var (
	ErrUserNotFound       = errors.New("không tìm thấy người dùng")
	ErrUserAlreadyExists  = errors.New("email hoặc username đã tồn tại")
	ErrInvalidCredentials = errors.New("thông tin đăng nhập không hợp lệ")
	ErrSessionNotFound    = errors.New("không tìm thấy phiên đăng nhập")
	ErrSessionExpired     = errors.New("phiên đăng nhập đã hết hạn")
	ErrSessionRevoked     = errors.New("phiên đăng nhập đã bị thu hồi")
	ErrZoneNotFound       = errors.New("khong tim thay auth zone")
	ErrZoneAccessDenied   = errors.New("khong co quyen truy cap auth zone")
	ErrRegistrationClosed = errors.New("zone khong cho phep dang ky")
	ErrInviteRequired     = errors.New("can invite hop le de dang ky")
)

type User struct {
	ID              string
	Email           string
	Username        string
	DisplayName     string
	PasswordHash    string
	AvatarURL       *string
	Status          string
	Locale          string
	Timezone        string
	EmailVerifiedAt *time.Time
	LastSeenAt      *time.Time
	RegistrationIP  *string
	RegistrationDev *string
	LastIPAddress   *string
	DeviceName      *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Session struct {
	ID               string
	UserID           string
	ZoneID           string
	WorkspaceID      string
	Domain           string
	RefreshTokenHash string
	DeviceName       *string
	IPAddress        *string
	UserAgent        *string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (s Session) Active(now time.Time) error {
	if s.ID == "" {
		return ErrSessionNotFound
	}
	if s.RevokedAt != nil {
		return ErrSessionRevoked
	}
	if !s.ExpiresAt.After(now) {
		return ErrSessionExpired
	}
	return nil
}
