package websocket

import (
	"context"
	"testing"
	"time"

	filesapp "github.com/duclamdev/application-chat/backend/internal/modules/files/application"
	platformws "github.com/duclamdev/application-chat/backend/internal/platform/websocket"
)

func TestPublishBroadcastsAttachmentToConversationRoom(t *testing.T) {
	manager := platformws.NewManager()
	client := &platformws.Client{ID: "client-1", UserID: "user-2", Send: make(chan platformws.Event, 1)}
	if err := manager.Register(client); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	manager.Join("workspace:workspace-1:channel:channel-1", client.ID)

	publisher := NewPublisher(manager)
	err := publisher.Publish(context.Background(), filesapp.RealtimeEvent{
		Type:        "AttachmentCreated",
		WorkspaceID: "workspace-1",
		ChannelID:   "channel-1",
		ActorUserID: "user-1",
		Payload: map[string]any{
			"message_id": "message-1",
		},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case event := <-client.Send:
		if event.Type != "AttachmentCreated" {
			t.Fatalf("event type = %q", event.Type)
		}
		if event.Payload["message_id"] != "message-1" {
			t.Fatalf("message_id = %v", event.Payload["message_id"])
		}
	case <-time.After(time.Second):
		t.Fatal("khong nhan duoc attachment realtime event")
	}
}
