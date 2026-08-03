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

func TestDisconnectUserClosesOnlyThatUsersClients(t *testing.T) {
	manager := NewManager()
	disconnected := false
	deletedUser := &Client{
		ID: "deleted-user-client", UserID: "user-1", ZoneID: "zone-a",
		Send: make(chan Event, 1), Disconnect: func() { disconnected = true },
	}
	otherUser := &Client{ID: "other-user-client", UserID: "user-2", ZoneID: "zone-a", Send: make(chan Event, 1)}
	if err := manager.Register(deletedUser); err != nil {
		t.Fatal(err)
	}
	if err := manager.Register(otherUser); err != nil {
		t.Fatal(err)
	}
	defer manager.Unregister(otherUser.ID)

	if err := manager.DisconnectUser(context.Background(), "user-1"); err != nil {
		t.Fatalf("DisconnectUser() error = %v", err)
	}
	if !disconnected {
		t.Fatal("deleted user's network connection was not closed")
	}
	if _, open := <-deletedUser.Send; open {
		t.Fatal("deleted user's send channel is still open")
	}
	if stats := manager.Stats(); stats["clients"] != 1 {
		t.Fatalf("clients = %d, want 1", stats["clients"])
	}
}
