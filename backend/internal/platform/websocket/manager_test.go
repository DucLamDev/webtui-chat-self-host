package websocket

import (
	"context"
	"testing"
)

func TestUserRoomIsIsolatedByZone(t *testing.T) {
	manager := NewManager()
	zoneA := &Client{ID: "client-a", UserID: "user-1", ZoneID: "zone-a", Send: make(chan Event, 1)}
	zoneB := &Client{ID: "client-b", UserID: "user-1", ZoneID: "zone-b", Send: make(chan Event, 1)}
	if err := manager.Register(zoneA); err != nil {
		t.Fatal(err)
	}
	if err := manager.Register(zoneB); err != nil {
		t.Fatal(err)
	}
	defer manager.Unregister(zoneA.ID)
	defer manager.Unregister(zoneB.ID)

	if err := manager.Broadcast(context.Background(), UserRoom("zone-a", "user-1"), Event{Type: "Notification"}); err != nil {
		t.Fatal(err)
	}

	select {
	case <-zoneA.Send:
	default:
		t.Fatal("zone A client did not receive its event")
	}
	select {
	case event := <-zoneB.Send:
		t.Fatalf("zone B received cross-zone event: %+v", event)
	default:
	}
}
