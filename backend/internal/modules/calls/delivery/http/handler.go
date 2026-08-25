package http

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	nethttp "net/http"
	"strconv"
	"strings"
	"time"

	callsapp "github.com/duclamdev/application-chat/backend/internal/modules/calls/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service           *callsapp.Service
	publicICEServers  []map[string]any
	turnURLs          []string
	turnSharedSecret  string
	turnCredentialTTL time.Duration
	now               func() time.Time
}

type createCallRequest struct {
	ChannelID    string          `json:"channel_id"`
	TargetUserID string          `json:"target_user_id"`
	ClientCallID string          `json:"client_call_id"`
	Mode         string          `json:"mode"`
	Metadata     json.RawMessage `json:"metadata"`
}

type statusRequest struct {
	Reason string `json:"reason"`
}

type signalRequest struct {
	SignalType string          `json:"signal_type"`
	Payload    json.RawMessage `json:"payload"`
}

func NewHandler(service *callsapp.Service) *Handler {
	return &Handler{service: service, turnCredentialTTL: 10 * time.Minute, now: time.Now}
}

func (h *Handler) SetICEConfiguration(publicServers []map[string]any, turnURLs []string, sharedSecret string, ttl time.Duration) {
	h.publicICEServers = publicServers
	h.turnURLs = append([]string(nil), turnURLs...)
	h.turnSharedSecret = strings.TrimSpace(sharedSecret)
	if ttl > 0 {
		h.turnCredentialTTL = ttl
	}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc, legalMiddleware ...gin.HandlerFunc) {
	credentials := router.Group("/calls")
	credentials.Use(authMiddleware)
	credentials.GET("/ice-servers", h.ICEServers)

	private := router.Group("/workspaces/:workspace_id/calls")
	private.Use(authMiddleware)
	ugc := router.Group("/workspaces/:workspace_id/calls")
	ugc.Use(authMiddleware)
	if len(legalMiddleware) > 0 && legalMiddleware[0] != nil {
		ugc.Use(legalMiddleware[0])
	}
	ugc.POST("", h.Create)
	private.GET("/incoming", h.Incoming)
	private.GET("/:call_id", h.Get)
	ugc.POST("/:call_id/accept", h.Accept)
	private.POST("/:call_id/reject", h.Reject)
	private.POST("/:call_id/cancel", h.Cancel)
	private.POST("/:call_id/hangup", h.Hangup)
	private.POST("/:call_id/miss", h.Miss)
	ugc.POST("/:call_id/signals", h.Signal)
}

func (h *Handler) ICEServers(c *gin.Context) {
	servers := cloneICEServers(h.publicICEServers)
	expiresAt := h.now().UTC().Add(h.turnCredentialTTL)
	if h.turnSharedSecret != "" && len(h.turnURLs) > 0 {
		username := strconv.FormatInt(expiresAt.Unix(), 10) + ":" + middleware.CurrentUserID(c)
		mac := hmac.New(sha1.New, []byte(h.turnSharedSecret))
		_, _ = mac.Write([]byte(username))
		servers = append(servers, map[string]any{
			"urls":       append([]string(nil), h.turnURLs...),
			"username":   username,
			"credential": base64.StdEncoding.EncodeToString(mac.Sum(nil)),
		})
	}
	response.OK(c, nethttp.StatusOK, gin.H{
		"ice_servers": servers,
		"expires_at":  expiresAt.Format(time.RFC3339),
	})
}

func cloneICEServers(source []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(source))
	for _, server := range source {
		copy := make(map[string]any, len(server))
		for key, value := range server {
			copy[key] = value
		}
		result = append(result, copy)
	}
	return result
}

func (h *Handler) Incoming(c *gin.Context) {
	call, err := h.service.FindIncomingRinging(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"call": call})
}

func (h *Handler) Create(c *gin.Context) {
	var req createCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	actorUserID := strings.TrimSpace(middleware.CurrentUserID(c))
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	channelID := strings.TrimSpace(req.ChannelID)
	targetUserID := strings.TrimSpace(req.TargetUserID)
	if _, err := uuid.Parse(workspaceID); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "VALIDATION_ERROR", "workspace_id khÃ´ng há»£p lá»‡.", nil)
		return
	}
	if _, err := uuid.Parse(channelID); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "VALIDATION_ERROR", "channel_id cuá»™c gá»i khÃ´ng há»£p lá»‡.", nil)
		return
	}
	if _, err := uuid.Parse(targetUserID); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "VALIDATION_ERROR", "target_user_id cuá»™c gá»i khÃ´ng há»£p lá»‡.", nil)
		return
	}
	if actorUserID != "" && actorUserID == targetUserID {
		response.Fail(c, nethttp.StatusBadRequest, "CALL_TARGET_INVALID", "KhÃ´ng thá»ƒ tá»± gá»i chÃ­nh mÃ¬nh.", nil)
		return
	}
	clientCallID := req.ClientCallID
	if clientCallID == "" {
		clientCallID = c.GetHeader("Idempotency-Key")
	}
	call, err := h.service.Create(c.Request.Context(), callsapp.CreateInput{
		ActorUserID:  actorUserID,
		WorkspaceID:  workspaceID,
		ChannelID:    channelID,
		TargetUserID: targetUserID,
		ClientCallID: clientCallID,
		Mode:         req.Mode,
		Metadata:     req.Metadata,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, call)
}

func (h *Handler) Get(c *gin.Context) {
	call, err := h.service.Get(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("call_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, call)
}

func (h *Handler) Accept(c *gin.Context) {
	h.changeStatus(c, "accept")
}

func (h *Handler) Reject(c *gin.Context) {
	h.changeStatus(c, "reject")
}

func (h *Handler) Cancel(c *gin.Context) {
	h.changeStatus(c, "cancel")
}

func (h *Handler) Hangup(c *gin.Context) {
	h.changeStatus(c, "hangup")
}

func (h *Handler) Miss(c *gin.Context) {
	h.changeStatus(c, "miss")
}

func (h *Handler) changeStatus(c *gin.Context, action string) {
	var req statusRequest
	if c.Request.Body != nil {
		_ = c.ShouldBindJSON(&req)
	}
	call, err := h.service.ChangeStatus(c.Request.Context(), callsapp.StatusInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		CallID:      c.Param("call_id"),
		Action:      action,
		Reason:      req.Reason,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, call)
}

func (h *Handler) Signal(c *gin.Context) {
	var req signalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	signal, err := h.service.SendSignal(c.Request.Context(), callsapp.SignalInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		CallID:      c.Param("call_id"),
		SignalType:  req.SignalType,
		Payload:     req.Payload,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, signal)
}
