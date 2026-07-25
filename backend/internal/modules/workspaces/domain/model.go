package domain

import (
	"errors"
	"time"
)

var (
	ErrWorkspaceQuotaExceeded = errors.New("zone da vuot workspace quota")
	ErrWorkspaceNotFound      = errors.New("không tìm thấy workspace")
	ErrMemberNotFound         = errors.New("không tìm thấy thành viên workspace")
	ErrInviteNotFound         = errors.New("không tìm thấy lời mời workspace")
	ErrRoleNotFound           = errors.New("không tìm thấy vai trò workspace")
	ErrWorkspaceConflict      = errors.New("workspace đã tồn tại")
)

type Workspace struct {
	ID          string
	ZoneID      string
	Slug        string
	Name        string
	Description *string
	OwnerID     *string
	Plan        string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Member struct {
	ID          string
	WorkspaceID string
	UserID      string
	Email       string
	Username    string
	DisplayName string
	AvatarURL   *string
	PhoneNumber *string
	Status      string
	Title       *string
	JoinedAt    *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Setting struct {
	WorkspaceID string
	Key         string
	Value       []byte
	ValueType   string
	Description *string
	UpdatedBy   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Invite struct {
	ID          string
	WorkspaceID string
	Email       string
	RoleID      *string
	InvitedBy   *string
	ExpiresAt   time.Time
	AcceptedAt  *time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
}
