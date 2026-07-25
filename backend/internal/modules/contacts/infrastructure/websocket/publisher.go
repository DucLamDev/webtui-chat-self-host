package websocket

import (
	"context"
	"strings"

	contactsapp "github.com/duclamdev/application-chat/backend/internal/modules/contacts/application"
	platformws "github.com/duclamdev/application-chat/backend/internal/platform/websocket"
)

type Publisher struct {
	manager *platformws.Manager
}

func NewPublisher(manager *platformws.Manager) *Publisher {
	return &Publisher{manager: manager}
}

func (p *Publisher) Publish(ctx context.Context, event contactsapp.RealtimeEvent) error {
	if p == nil || p.manager == nil {
		return nil
	}
	room := platformws.UserRoom(event.ZoneID, event.UserID)
	if strings.TrimSpace(room) == "" {
		return nil
	}
	return p.manager.Broadcast(ctx, room, platformws.Event{
		Type:    event.Type,
		Room:    room,
		UserID:  event.UserID,
		Payload: event.Payload,
	})
}
