package domain

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrCallNotFound          = errors.New("không tìm thấy cuộc gọi")
	ErrCallParticipantDenied = errors.New("người dùng không thuộc cuộc gọi")
	ErrCallInvalidTransition = errors.New("trạng thái cuộc gọi không hợp lệ")
)

type Call struct {
	ID              string
	WorkspaceID     string
	ChannelID       string
	InitiatorUserID string
	TargetUserID    string
	ClientCallID    *string
	Mode            string
	Status          string
	Metadata        json.RawMessage
	StartedAt       *time.Time
	EndedAt         *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Signal struct {
	ID           string
	WorkspaceID  string
	CallID       string
	SenderUserID string
	SignalType   string
	Payload      json.RawMessage
	CreatedAt    time.Time
}
