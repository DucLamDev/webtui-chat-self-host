package domain

import (
	"errors"
	"time"
)

var (
	ErrContactRequestNotFound = errors.New("không tìm thấy lời mời kết bạn")
	ErrContactRequestConflict = errors.New("lời mời kết bạn đã tồn tại")
	ErrCannotContactSelf      = errors.New("không thể kết bạn với chính mình")
	ErrUserNotFound           = errors.New("không tìm thấy người dùng")
)

type UserSummary struct {
	ID          string
	Email       string
	Username    string
	DisplayName string
	AvatarURL   *string
	PhoneNumber *string
	Status      string
}

type ContactRequest struct {
	ID          string
	RequesterID string
	ReceiverID  string
	Status      string
	User        UserSummary
	RequestedAt time.Time
	RespondedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
