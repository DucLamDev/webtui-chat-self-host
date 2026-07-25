package domain

import (
	"errors"
	"time"
)

var (
	ErrMessageNotFound  = errors.New("không tìm thấy tin nhắn")
	ErrChannelNotFound  = errors.New("không tìm thấy kênh hoặc bạn chưa thuộc kênh")
	ErrMentionNotFound  = errors.New("người được nhắc chưa thuộc kênh")
	ErrReactionNotFound = errors.New("không tìm thấy reaction")
)

var ErrPinNotFound = errors.New("không tìm thấy tin ghim")

type Message struct {
	ID           string
	WorkspaceID  string
	ChannelID    string
	SenderID     *string
	ParentID     *string
	ThreadRootID *string
	Kind         string
	Body         string
	Metadata     []byte
	EditedAt     *time.Time
	DeletedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Mentions     []string
	Reactions    []ReactionSummary
}

type ReactionSummary struct {
	Emoji       string
	Count       int
	ReactedByMe bool
}
