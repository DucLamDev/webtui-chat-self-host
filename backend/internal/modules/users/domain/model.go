package domain

import (
	"errors"
	"time"
)

var (
	ErrUserNotFound                  = errors.New("không tìm thấy người dùng")
	ErrUserOwnsWorkspace             = errors.New("người dùng vẫn đang sở hữu workspace")
	ErrOwnershipSuccessorNotEligible = errors.New("người kế nhiệm không hợp lệ")
)

type User struct {
	ID              string
	Email           string
	Username        string
	DisplayName     string
	AvatarURL       *string
	PhoneNumber     *string
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
