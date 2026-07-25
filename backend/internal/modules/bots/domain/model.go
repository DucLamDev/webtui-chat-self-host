package domain

import (
	"errors"
	"time"
)

var (
	ErrBotNotFound         = errors.New("không tìm thấy bot")
	ErrBotAlreadyExists    = errors.New("bot đã tồn tại")
	ErrBotAlreadyInstalled = errors.New("bot đã được cài đặt")
	ErrBotNotInstalled     = errors.New("bot chưa được cài đặt vào kênh")
)

type Bot struct {
	ID          string
	WorkspaceID string
	Slug        string
	Name        string
	Description *string
	AvatarURL   *string
	Status      string
	CreatedBy   *string
	Settings    []byte
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Installation struct {
	ID          string
	BotID       string
	WorkspaceID string
	ChannelID   *string
	Status      string
	Config      []byte
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type BotMessage struct {
	ID          string
	WorkspaceID string
	ChannelID   string
	BotID       string
	Kind        string
	Body        string
	Metadata    []byte
	CreatedAt   time.Time
}

type AIConfig struct {
	WorkspaceID string
	BotID       string
	Provider    string
	Model       string
	SecretRef   *string
	Settings    []byte
	CreatedBy   *string
	UpdatedBy   *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Flow struct {
	ID              string
	WorkspaceID     string
	BotID           string
	Version         int
	Status          string
	Name            string
	Prompt          string
	TriggerConfig   []byte
	ToolConfig      []byte
	KnowledgeConfig []byte
	CreatedBy       *string
	UpdatedBy       *string
	PublishedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type FlowRun struct {
	ID          string
	WorkspaceID string
	BotID       string
	FlowID      string
	Input       []byte
	Transcript  []byte
	Status      string
	Error       *string
	CreatedBy   *string
	CreatedAt   time.Time
}
