package websocket

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

type Event struct {
	Type      string         `json:"type"`
	Room      string         `json:"room,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

type Client struct {
	ID     string
	UserID string
	ZoneID string
	Send   chan Event
}

type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client
	rooms   map[string]map[string]struct{}
}

func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		rooms:   make(map[string]map[string]struct{}),
	}
}

func (m *Manager) Register(client *Client) error {
	if client == nil || client.ID == "" {
		return errors.New("mã client WebSocket là bắt buộc")
	}
	if client.Send == nil {
		client.Send = make(chan Event, 32)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.clients[client.ID] = client
	if room := UserRoom(client.ZoneID, client.UserID); room != "" {
		if _, ok := m.rooms[room]; !ok {
			m.rooms[room] = make(map[string]struct{})
		}
		m.rooms[room][client.ID] = struct{}{}
	}
	return nil
}

func (m *Manager) Unregister(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[clientID]; ok {
		close(client.Send)
		delete(m.clients, clientID)
	}

	for room, members := range m.rooms {
		delete(members, clientID)
		if len(members) == 0 {
			delete(m.rooms, room)
		}
	}
}

func (m *Manager) Join(room string, clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, ok := m.clients[clientID]
	if !ok {
		return
	}
	if strings.HasPrefix(room, "zone:") && strings.Contains(room, ":user:") && room != UserRoom(client.ZoneID, client.UserID) {
		return
	}

	if _, ok := m.rooms[room]; !ok {
		m.rooms[room] = make(map[string]struct{})
	}
	m.rooms[room][clientID] = struct{}{}
}

func UserRoom(zoneID string, userID string) string {
	zoneID = strings.TrimSpace(zoneID)
	userID = strings.TrimSpace(userID)
	if zoneID == "" || userID == "" {
		return ""
	}
	return "zone:" + zoneID + ":user:" + userID
}

func (m *Manager) Leave(room string, clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if members, ok := m.rooms[room]; ok {
		delete(members, clientID)
		if len(members) == 0 {
			delete(m.rooms, room)
		}
	}
}

func (m *Manager) IsMember(room string, clientID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.rooms[room][clientID]
	return ok
}

func (m *Manager) IsUserMember(room string, userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for clientID := range m.rooms[room] {
		if client, ok := m.clients[clientID]; ok && client.UserID == userID {
			return true
		}
	}
	return false
}

func (m *Manager) Broadcast(ctx context.Context, room string, event Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.Room == "" {
		event.Room = room
	}

	m.mu.RLock()
	clientIDs := make([]string, 0, len(m.rooms[room]))
	for clientID := range m.rooms[room] {
		clientIDs = append(clientIDs, clientID)
	}
	clients := make([]*Client, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		if client, ok := m.clients[clientID]; ok {
			clients = append(clients, client)
		}
	}
	m.mu.RUnlock()

	for _, client := range clients {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case client.Send <- event:
		default:
		}
	}

	return nil
}

func (m *Manager) Stats() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]int{
		"clients": len(m.clients),
		"rooms":   len(m.rooms),
	}
}

func (m *Manager) Health(ctx context.Context) error {
	return ctx.Err()
}
