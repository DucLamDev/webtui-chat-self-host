package websocket

import (
	"context"
	"fmt"

	filesapp "github.com/duclamdev/application-chat/backend/internal/modules/files/application"
	platformws "github.com/duclamdev/application-chat/backend/internal/platform/websocket"
)

type Publisher struct {
	manager *platformws.Manager
}

func NewPublisher(manager *platformws.Manager) *Publisher {
	return &Publisher{manager: manager}
}

func (p *Publisher) Publish(ctx context.Context, event filesapp.RealtimeEvent) error {
	if p == nil || p.manager == nil {
		return nil
	}
	room := fmt.Sprintf("workspace:%s:channel:%s", event.WorkspaceID, event.ChannelID)
	return p.manager.Broadcast(ctx, room, platformws.Event{
		Type:    event.Type,
		Room:    room,
		UserID:  event.ActorUserID,
		Payload: event.Payload,
	})
}
