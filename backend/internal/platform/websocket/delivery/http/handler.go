package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	nethttp "net/http"
	"strings"
	"time"

	platformws "github.com/duclamdev/application-chat/backend/internal/platform/websocket"
	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
	"github.com/duclamdev/application-chat/backend/internal/shared/constants"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	xwebsocket "golang.org/x/net/websocket"
)

type Handler struct {
	manager     *platformws.Manager
	tokens      *sharedauth.Manager
	authorizer  RoomAuthorizer
	zoneChecker WorkspaceZoneChecker
}

type RoomAuthorizer interface {
	CanJoinRoom(ctx context.Context, userID string, room string) bool
}

type WorkspaceZoneChecker interface {
	WorkspaceBelongsToZone(ctx context.Context, workspaceID string, zoneID string) (bool, error)
}

type clientCommand struct {
	Type    string         `json:"type"`
	Room    string         `json:"room"`
	Payload map[string]any `json:"payload"`
}

func NewHandler(manager *platformws.Manager, tokens *sharedauth.Manager, authorizers ...RoomAuthorizer) *Handler {
	handler := &Handler{manager: manager, tokens: tokens}
	if len(authorizers) > 0 {
		handler.authorizer = authorizers[0]
	}
	return handler
}

func (h *Handler) SetWorkspaceZoneChecker(checker WorkspaceZoneChecker) {
	h.zoneChecker = checker
}

func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.GET("/ws", h.Connect)
}

func (h *Handler) RegisterPublicRoute(router gin.IRouter) {
	router.GET("/ws", h.Connect)
}

func (h *Handler) Connect(c *gin.Context) {
	if h.manager == nil {
		response.Fail(c, nethttp.StatusServiceUnavailable, "WEBSOCKET_DISABLED", "WebSocket chưa sẵn sàng.", nil)
		return
	}

	claims, err := h.authenticateRequest(c.Request)
	if err != nil {
		h.failAuth(c, err)
		return
	}
	resolvedZoneID := contextString(c, constants.ContextResolvedZoneID)
	if claims.ZoneID == "" || resolvedZoneID == "" || claims.ZoneID != resolvedZoneID {
		response.Fail(c, nethttp.StatusForbidden, "ZONE_TOKEN_MISMATCH", "Phiên đăng nhập không thuộc domain hiện tại.", nil)
		return
	}

	xwebsocket.Handler(func(conn *xwebsocket.Conn) {
		h.serve(conn, claims.Subject, claims.ZoneID)
	}).ServeHTTP(c.Writer, c.Request)
}

func (h *Handler) authenticateRequest(req *nethttp.Request) (*sharedauth.AccessClaims, error) {
	if h.tokens == nil {
		return nil, sharedauth.ErrInvalidToken
	}

	token := accessTokenFromRequest(req)
	if token == "" {
		return nil, sharedauth.ErrInvalidToken
	}

	claims, err := h.tokens.VerifyAccessToken(token)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func (h *Handler) failAuth(c *gin.Context, err error) {
	message := "Token không hợp lệ."
	if errors.Is(err, sharedauth.ErrExpiredToken) {
		message = "Token đã hết hạn."
	}
	response.Fail(c, nethttp.StatusUnauthorized, "UNAUTHORIZED", message, nil)
}

func accessTokenFromRequest(req *nethttp.Request) string {
	if req == nil {
		return ""
	}
	if token := bearerToken(req.Header.Get("Authorization")); token != "" {
		return token
	}
	if req.URL == nil {
		return tokenFromSubprotocol(req.Header.Get("Sec-WebSocket-Protocol"))
	}
	if token := strings.TrimSpace(req.URL.Query().Get("access_token")); token != "" {
		return token
	}
	if token := strings.TrimSpace(req.URL.Query().Get("token")); token != "" {
		return token
	}
	return tokenFromSubprotocol(req.Header.Get("Sec-WebSocket-Protocol"))
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

func tokenFromSubprotocol(header string) string {
	parts := strings.Split(header, ",")
	for index, part := range parts {
		protocol := strings.TrimSpace(part)
		switch strings.ToLower(protocol) {
		case "webtui.jwt", "webtui.token":
			if index+1 < len(parts) {
				return strings.TrimSpace(parts[index+1])
			}
		default:
			if strings.HasPrefix(protocol, "webtui.jwt.") {
				return strings.TrimSpace(strings.TrimPrefix(protocol, "webtui.jwt."))
			}
			if strings.HasPrefix(protocol, "webtui.token.") {
				return strings.TrimSpace(strings.TrimPrefix(protocol, "webtui.token."))
			}
		}
	}
	return ""
}

func (h *Handler) serve(conn *xwebsocket.Conn, userID string, zoneID string) {
	client := &platformws.Client{
		ID:     newClientID(),
		UserID: userID,
		ZoneID: zoneID,
		Send:   make(chan platformws.Event, 64),
	}
	if err := h.manager.Register(client); err != nil {
		_ = xwebsocket.JSON.Send(conn, map[string]string{
			"type":    "error",
			"message": err.Error(),
		})
		return
	}
	defer h.manager.Unregister(client.ID)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for event := range client.Send {
			if err := xwebsocket.JSON.Send(conn, event); err != nil {
				return
			}
		}
	}()

	for {
		var command clientCommand
		if err := xwebsocket.JSON.Receive(conn, &command); err != nil {
			if !errors.Is(err, nethttp.ErrAbortHandler) {
				return
			}
			return
		}
		h.handleCommand(client, command)
		select {
		case <-done:
			return
		default:
		}
	}
}

func (h *Handler) handleCommand(client *platformws.Client, command clientCommand) {
	if client == nil {
		return
	}
	room := strings.TrimSpace(command.Room)
	if room == "" {
		return
	}
	switch strings.TrimSpace(command.Type) {
	case "join":
		if !h.roomBelongsToZone(client.ZoneID, room) {
			return
		}
		if h.authorizer != nil && !h.authorizer.CanJoinRoom(context.Background(), client.UserID, room) {
			return
		}
		h.manager.Join(room, client.ID)
	case "leave":
		h.manager.Leave(room, client.ID)
	case "TypingStarted", "TypingStopped":
		if !h.manager.IsMember(room, client.ID) {
			return
		}
		_ = h.manager.Broadcast(context.Background(), room, platformws.Event{
			Type:   strings.TrimSpace(command.Type),
			Room:   room,
			UserID: client.UserID,
			Payload: map[string]any{
				"user_id": client.UserID,
			},
		})
	}
}

func (h *Handler) roomBelongsToZone(zoneID string, room string) bool {
	parts := strings.Split(strings.TrimSpace(room), ":")
	if len(parts) != 4 || parts[0] != "workspace" || parts[2] != "channel" {
		return false
	}
	if h.zoneChecker == nil {
		return strings.TrimSpace(zoneID) != ""
	}
	matches, err := h.zoneChecker.WorkspaceBelongsToZone(context.Background(), parts[1], strings.TrimSpace(zoneID))
	return err == nil && matches
}

func contextString(c *gin.Context, key string) string {
	value, _ := c.Get(key)
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func newClientID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(random[:])
}
