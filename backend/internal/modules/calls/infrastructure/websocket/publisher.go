package websocket

import (
	"context"
	"strings"

	callsapp "github.com/duclamdev/application-chat/backend/internal/modules/calls/application"
	platformws "github.com/duclamdev/application-chat/backend/internal/platform/websocket"
)

type Publisher struct {
	manager *platformws.Manager
}

func NewPublisher(manager *platformws.Manager) *Publisher {
	return &Publisher{manager: manager}
}

func (p *Publisher) Publish(ctx context.Context, event callsapp.RealtimeEvent) error {
	if p == nil || p.manager == nil {
		return nil
	}
	participants := []string{
		payloadString(event.Payload, "initiator_user_id"),
		payloadString(event.Payload, "target_user_id"),
	}
	seen := make(map[string]struct{}, len(participants))
	for _, userID := range participants {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		room := platformws.UserRoom(event.ZoneID, userID)
		if room == "" {
			continue
		}
		if err := p.manager.Broadcast(ctx, room, platformws.Event{
			Type:    event.Type,
			Room:    room,
			UserID:  event.ActorUserID,
			Payload: event.Payload,
		}); err != nil {
			return err
		}
	}
	return nil
}

func payloadString(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}
