package domain

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrChannelNotFound = errors.New("không tìm thấy kênh")
	ErrChannelConflict = errors.New("kênh đã tồn tại")
	ErrMemberNotFound  = errors.New("không tìm thấy thành viên kênh")
	ErrContactRequired = errors.New("cần kết bạn trước khi tạo hội thoại riêng")
	ErrGuestNotFound   = errors.New("không tìm thấy yêu cầu của khách")
	ErrVersionConflict = errors.New("tài liệu đã được cập nhật bởi người khác")
	ErrTaskConflict    = errors.New("tin nhắn đã được chuyển thành công việc")
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

type CollaborationSettings struct {
	ChannelID              string
	WorkspaceID            string
	ChannelName            string
	ChannelType            string
	RoomMode               string
	MeetingProvider        string
	MeetingRoomKey         string
	PublicAccessEnabled    bool
	PublicTokenPrefix      *string
	PasswordHash           *string
	LobbyEnabled           bool
	ChatLocked             bool
	GuestMicrophoneEnabled bool
	GuestCameraEnabled     bool
	DefaultParticipantRole string
	CreatedBy              *string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type CollaborationRole struct {
	ChannelID   string
	UserID      string
	DisplayName string
	Username    string
	AvatarURL   *string
	Role        string
	UpdatedAt   time.Time
}

type GuestRequest struct {
	ID                   string
	ChannelID            string
	DisplayName          string
	Status               string
	ReviewedBy           *string
	ReviewedAt           *time.Time
	TermsVersion         *string
	PrivacyPolicyVersion *string
	LegalAcceptedAt      *time.Time
	ExpiresAt            time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type CollaborationDocument struct {
	ChannelID string
	Kind      string
	Content   json.RawMessage
	Version   int64
	UpdatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ChannelTask struct {
	ID              string
	WorkspaceID     string
	ChannelID       string
	SourceMessageID *string
	Title           string
	Description     *string
	Status          string
	AssigneeUserID  *string
	DueAt           *time.Time
	CreatedBy       *string
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type BreakoutRoom struct {
	ID              string
	ChannelID       string
	Name            string
	RoomKey         string
	AssignedUserIDs json.RawMessage
	Status          string
	AssignmentMode  string
	AllowSelfSelect bool
	StartedAt       *time.Time
	Sequence        int
	CreatedBy       *string
	ClosedAt        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
