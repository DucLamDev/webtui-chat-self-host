package http

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	platformws "github.com/duclamdev/application-chat/backend/internal/platform/websocket"
	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
	"github.com/duclamdev/application-chat/backend/internal/shared/constants"
	"github.com/gin-gonic/gin"
	xwebsocket "golang.org/x/net/websocket"
)

type fakeActiveUserChecker struct {
	active bool
}

func (checker fakeActiveUserChecker) UserIsActive(context.Context, string) (bool, error) {
	return checker.active, nil
}

func TestRegistersDiscoveryAndAPIV1WebSocketRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(nil, nil)

	handler.RegisterRoutes(router.Group("/api/v1"))
	handler.RegisterPublicRoute(router)

	routes := map[string]bool{}
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{"GET /ws", "GET /api/v1/ws"} {
		if !routes[route] {
			t.Fatalf("missing route %s: %#v", route, routes)
		}
	}
}

func TestRootWebSocketRouteUsesResolvedZoneContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := sharedauth.NewManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour)
	accessToken, _, err := tokens.CreateZoneAccessToken(
		"user-1", "user@example.com", "user", "zone-1", "workspace-1", "chat.example.com",
	)
	if err != nil {
		t.Fatalf("CreateZoneAccessToken() error = %v", err)
	}
	manager := platformws.NewManager()
	handler := NewHandler(manager, tokens)
	handler.SetActiveUserChecker(fakeActiveUserChecker{active: true})
	router := gin.New()
	root := router.Group("")
	root.Use(func(c *gin.Context) {
		c.Set(constants.ContextResolvedZoneID, "zone-1")
		c.Next()
	})
	handler.RegisterPublicRoute(root)
	server := httptest.NewServer(router)
	defer server.Close()

	config, err := xwebsocket.NewConfig(
		strings.Replace(server.URL, "http://", "ws://", 1)+"/ws",
		server.URL,
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}
	config.Header.Set("Authorization", "Bearer "+accessToken)
	connection, err := xwebsocket.DialConfig(config)
	if err != nil {
		t.Fatalf("DialConfig() error = %v", err)
	}
	_ = connection.Close()
}

func TestAccessTokenFromRequest(t *testing.T) {
	tests := []struct {
		name   string
		req    *nethttp.Request
		expect string
	}{
		{
			name: "authorization bearer",
			req: func() *nethttp.Request {
				req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/ws", nil)
				req.Header.Set("Authorization", "Bearer header-token")
				return req
			}(),
			expect: "header-token",
		},
		{
			name:   "access token query is rejected",
			req:    httptest.NewRequest(nethttp.MethodGet, "/api/v1/ws?access_token=query-token", nil),
			expect: "",
		},
		{
			name:   "token query is rejected",
			req:    httptest.NewRequest(nethttp.MethodGet, "/api/v1/ws?token=query-token", nil),
			expect: "",
		},
		{
			name: "subprotocol pair",
			req: func() *nethttp.Request {
				req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/ws", nil)
				req.Header.Set("Sec-WebSocket-Protocol", "webtui.jwt, protocol-token")
				return req
			}(),
			expect: "protocol-token",
		},
		{
			name: "subprotocol compact",
			req: func() *nethttp.Request {
				req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/ws", nil)
				req.Header.Set("Sec-WebSocket-Protocol", "webtui.jwt.protocol-token")
				return req
			}(),
			expect: "protocol-token",
		},
		{
			name: "subprotocol with nil URL",
			req: func() *nethttp.Request {
				req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/ws", nil)
				req.URL = nil
				req.Header.Set("Sec-WebSocket-Protocol", "webtui.token, protocol-token")
				return req
			}(),
			expect: "protocol-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := accessTokenFromRequest(tt.req); got != tt.expect {
				t.Fatalf("accessTokenFromRequest() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestAuthenticateRequestFromBrowserSubprotocolToken(t *testing.T) {
	tokens := sharedauth.NewManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour)
	accessToken, _, err := tokens.CreateAccessToken("user-1", "user@example.com", "user")
	if err != nil {
		t.Fatalf("CreateAccessToken() error = %v", err)
	}

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "webtui.jwt."+accessToken)
	handler := NewHandler(platformws.NewManager(), tokens)

	claims, err := handler.authenticateRequest(req)
	if err != nil {
		t.Fatalf("authenticateRequest() error = %v", err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("authenticateRequest() userID = %q, want user-1", claims.Subject)
	}
}

func TestAuthenticateRequestRejectsDeletedUser(t *testing.T) {
	tokens := sharedauth.NewManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour)
	accessToken, _, err := tokens.CreateAccessToken("user-1", "user@example.com", "user")
	if err != nil {
		t.Fatalf("CreateAccessToken() error = %v", err)
	}
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/ws", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	handler := NewHandler(platformws.NewManager(), tokens)
	handler.SetActiveUserChecker(fakeActiveUserChecker{active: false})

	if _, err := handler.authenticateRequest(req); err == nil {
		t.Fatal("authenticateRequest() accepted a deleted user")
	}
}

func TestHandleCommandBroadcastsTypingState(t *testing.T) {
	manager := platformws.NewManager()
	handler := NewHandler(manager, nil)
	sender := &platformws.Client{ID: "sender", UserID: "user-a", ZoneID: "zone-1", Send: make(chan platformws.Event, 2)}
	receiver := &platformws.Client{ID: "receiver", UserID: "user-b", ZoneID: "zone-1", Send: make(chan platformws.Event, 2)}
	if err := manager.Register(sender); err != nil {
		t.Fatal(err)
	}
	if err := manager.Register(receiver); err != nil {
		t.Fatal(err)
	}
	defer manager.Unregister(sender.ID)
	defer manager.Unregister(receiver.ID)

	room := "workspace:workspace-1:channel:channel-1"
	manager.Join(room, sender.ID)
	manager.Join(room, receiver.ID)
	handler.handleCommand(sender, clientCommand{Type: "TypingStarted", Room: room})

	select {
	case event := <-receiver.Send:
		if event.Type != "TypingStarted" || event.UserID != sender.UserID || event.Room != room {
			t.Fatalf("unexpected typing event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("typing event was not broadcast")
	}
}

func TestHandleCommandRejectsUnvalidatedCallSignal(t *testing.T) {
	manager := platformws.NewManager()
	handler := NewHandler(manager, nil)
	sender := &platformws.Client{ID: "sender", UserID: "user-a", ZoneID: "zone-1", Send: make(chan platformws.Event, 2)}
	receiver := &platformws.Client{ID: "receiver", UserID: "user-b", ZoneID: "zone-1", Send: make(chan platformws.Event, 2)}
	if err := manager.Register(sender); err != nil {
		t.Fatal(err)
	}
	if err := manager.Register(receiver); err != nil {
		t.Fatal(err)
	}
	defer manager.Unregister(sender.ID)
	defer manager.Unregister(receiver.ID)

	room := "workspace:workspace-1:channel:channel-1"
	manager.Join(room, sender.ID)
	manager.Join(room, receiver.ID)
	handler.handleCommand(sender, clientCommand{
		Type: "CallOffer",
		Room: room,
		Payload: map[string]any{
			"call_id": "call-1",
			"mode":    "video",
		},
	})

	select {
	case event := <-receiver.Send:
		t.Fatalf("unvalidated call signal was broadcast: %#v", event)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHandleCommandRejectsUnvalidatedTargetedCallSignal(t *testing.T) {
	manager := platformws.NewManager()
	handler := NewHandler(manager, nil)
	sender := &platformws.Client{ID: "sender", UserID: "user-a", ZoneID: "zone-1", Send: make(chan platformws.Event, 2)}
	receiver := &platformws.Client{ID: "receiver", UserID: "user-b", ZoneID: "zone-1", Send: make(chan platformws.Event, 2)}
	if err := manager.Register(sender); err != nil {
		t.Fatal(err)
	}
	if err := manager.Register(receiver); err != nil {
		t.Fatal(err)
	}
	defer manager.Unregister(sender.ID)
	defer manager.Unregister(receiver.ID)

	room := "workspace:workspace-1:channel:channel-1"
	manager.Join(room, sender.ID)
	handler.handleCommand(sender, clientCommand{
		Type: "CallOffer",
		Room: room,
		Payload: map[string]any{
			"call_id":        "call-1",
			"mode":           "video",
			"target_user_id": "user-b",
		},
	})

	select {
	case event := <-receiver.Send:
		t.Fatalf("unvalidated targeted call signal was broadcast: %#v", event)
	case <-time.After(50 * time.Millisecond):
	}
}
