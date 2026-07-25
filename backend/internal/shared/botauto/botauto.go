package botauto

import (
	"context"
	"encoding/json"
)

type MessageInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	MessageID   string
	Body        string
}

type BotMessage struct {
	ID          string
	WorkspaceID string
	ChannelID   string
	BotID       string
	Kind        string
	Body        string
	Metadata    json.RawMessage
	CreatedAt   string
}

type Responder interface {
	HandleMessage(ctx context.Context, input MessageInput) ([]BotMessage, error)
}
