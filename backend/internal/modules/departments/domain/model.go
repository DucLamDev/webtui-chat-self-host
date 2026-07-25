package domain

import (
	"errors"
	"time"
)

var (
	ErrDepartmentNotFound = errors.New("không tìm thấy phòng ban")
	ErrDepartmentConflict = errors.New("phòng ban đã tồn tại")
	ErrMemberNotFound     = errors.New("không tìm thấy thành viên phòng ban")
)

type Department struct {
	ID           string
	WorkspaceID  string
	ParentID     *string
	Slug         string
	Name         string
	Description  *string
	CreatedBy    *string
	MemberCount  int
	LeadCount    int
	ChannelCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Member struct {
	DepartmentID string
	UserID       string
	Email        string
	Username     string
	DisplayName  string
	AvatarURL    *string
	Role         string
	CreatedAt    time.Time
}
