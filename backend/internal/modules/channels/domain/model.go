package domain

import (
	"errors"
	"time"
)

var (
	ErrChannelNotFound = errors.New("không tìm thấy kênh")
	ErrChannelConflict = errors.New("kênh đã tồn tại")
	ErrMemberNotFound  = errors.New("không tìm thấy thành viên kênh")
	ErrContactRequired = errors.New("cần kết bạn trước khi tạo hội thoại riêng")
)

type Channel struct {
	ID                 string
	WorkspaceID        string
	DepartmentID       *string
	Slug               *string
	Name               string
	Description        *string
	Type               string
	Status             string
	CreatedBy          *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ArchivedAt         *time.Time
	PrivateSessionMode bool
	MemberCount        int
}

type Member struct {
	ChannelID         string
	UserID            string
	Email             string
	Username          string
	DisplayName       string
	AvatarURL         *string
	Status            string
	LastReadAt        *time.Time
	LastReadMessageID *string
	JoinedAt          time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type DirectConversation struct {
	ID               string
	WorkspaceID      string
	ChannelID        string
	ParticipantKey   string
	ConversationType string
	CreatedBy        *string
	ParticipantIDs   []string
	Participants     []Member
	LastMessage      *MessageSummary
	UnreadCount      int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type MessageSummary struct {
	ID          string
	WorkspaceID string
	ChannelID   string
	SenderID    *string
	Kind        string
	Body        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
