package websocket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	platformredis "github.com/duclamdev/application-chat/backend/internal/platform/redis"
)

const (
	redisFanoutChannel  = "webtui:realtime:v1"
	redisControlChannel = "webtui:realtime:control:v1"
)

type Event struct {
	Type      string         `json:"type"`
	Room      string         `json:"room,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

type Client struct {
	ID         string
	UserID     string
	ZoneID     string
	Send       chan Event
	Disconnect func()
	mu         sync.Mutex
	closed     bool
}

type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client
	rooms   map[string]map[string]struct{}
	redis   *platformredis.Client
	nodeID  string
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	dropped atomic.Int64
}

type fanoutEnvelope struct {
	NodeID string `json:"node_id"`
	Room   string `json:"room"`
	Event  Event  `json:"event"`
}

type controlEnvelope struct {
	NodeID string `json:"node_id"`
	Type   string `json:"type"`
	UserID string `json:"user_id"`
}

func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		rooms:   make(map[string]map[string]struct{}),
	}
}

func NewManagerWithRedis(client *platformredis.Client) *Manager {
	manager := NewManager()
	if client == nil || client.Raw() == nil {
		return manager
	}
	manager.redis = client
	manager.nodeID = randomNodeID()
	ctx, cancel := context.WithCancel(context.Background())
	manager.cancel = cancel
	manager.wg.Add(1)
	go manager.consumeFanout(ctx)
	return manager
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
	client := m.clients[clientID]
	delete(m.clients, clientID)

	for room, members := range m.rooms {
		delete(members, clientID)
		if len(members) == 0 {
			delete(m.rooms, room)
		}
	}
	m.mu.Unlock()

	// Network close can block behind an in-flight writer. Never perform it while
	// holding the manager lock or one slow socket can stall all realtime rooms.
	if client != nil {
		client.close()
	}
}

// DisconnectUser closes every local connection for a deleted/disabled user and
// publishes the revocation so other API nodes do the same. A periodic active
// user check in the HTTP handler remains the fallback if Redis is unavailable.
func (m *Manager) DisconnectUser(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	m.disconnectUserLocal(userID)
	if m.redis == nil || m.redis.Raw() == nil {
		return ctx.Err()
	}
	payload, err := json.Marshal(controlEnvelope{
		NodeID: m.nodeID,
		Type:   "disconnect_user",
		UserID: userID,
	})
	if err != nil {
		return err
	}
	return m.redis.Raw().Publish(ctx, redisControlChannel, payload).Err()
}

func (m *Manager) disconnectUserLocal(userID string) {
	m.mu.RLock()
	clientIDs := make([]string, 0)
	for clientID, client := range m.clients {
		if client.UserID == userID {
			clientIDs = append(clientIDs, clientID)
		}
	}
	m.mu.RUnlock()
	for _, clientID := range clientIDs {
		m.Unregister(clientID)
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

	m.broadcastLocal(ctx, room, event)
	if m.redis == nil || m.redis.Raw() == nil {
		return ctx.Err()
	}
	payload, err := json.Marshal(fanoutEnvelope{NodeID: m.nodeID, Room: room, Event: event})
	if err != nil {
		return err
	}
	return m.redis.Raw().Publish(ctx, redisFanoutChannel, payload).Err()
}

func (m *Manager) broadcastLocal(ctx context.Context, room string, event Event) {
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

	slowClientIDs := make([]string, 0)
	for _, client := range clients {
		select {
		case <-ctx.Done():
			return
		default:
			if !client.deliver(event) {
				m.dropped.Add(1)
				slowClientIDs = append(slowClientIDs, client.ID)
			}
		}
	}
	for _, clientID := range slowClientIDs {
		m.Unregister(clientID)
	}
}

func (m *Manager) Stats() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]int{
		"clients":        len(m.clients),
		"rooms":          len(m.rooms),
		"dropped_events": int(m.dropped.Load()),
	}
}

func (m *Manager) Health(ctx context.Context) error {
	return ctx.Err()
}

func (m *Manager) Close() {
	if m == nil {
		return
	}
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
}

func (m *Manager) consumeFanout(ctx context.Context) {
	defer m.wg.Done()
	pubsub := m.redis.Raw().Subscribe(ctx, redisFanoutChannel, redisControlChannel)
	defer pubsub.Close()
	if _, err := pubsub.Receive(ctx); err != nil {
		if ctx.Err() == nil {
			slog.Error("WebSocket Redis subscription failed", "error", err)
		}
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-pubsub.Channel():
			if !ok {
				return
			}
			if message.Channel == redisControlChannel {
				var control controlEnvelope
				if err := json.Unmarshal([]byte(message.Payload), &control); err != nil {
					slog.Warn("Ignored invalid WebSocket control event", "error", err)
					continue
				}
				if control.NodeID != m.nodeID && control.Type == "disconnect_user" {
					m.disconnectUserLocal(strings.TrimSpace(control.UserID))
				}
				continue
			}
			var envelope fanoutEnvelope
			if err := json.Unmarshal([]byte(message.Payload), &envelope); err != nil {
				slog.Warn("Ignored invalid WebSocket Redis event", "error", err)
				continue
			}
			if envelope.NodeID == m.nodeID || strings.TrimSpace(envelope.Room) == "" {
				continue
			}
			m.broadcastLocal(ctx, envelope.Room, envelope.Event)
		}
	}
}

func (c *Client) deliver(event Event) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	select {
	case c.Send <- event:
		return true
	default:
		return false
	}
}

func (c *Client) close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.Send)
	disconnect := c.Disconnect
	c.mu.Unlock()
	if disconnect != nil {
		disconnect()
	}
}

func randomNodeID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(bytes)
}
