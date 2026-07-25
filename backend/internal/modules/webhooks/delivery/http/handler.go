package http

import (
	"encoding/json"
	nethttp "net/http"
	"strconv"
	"strings"

	tenancyhttp "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/delivery/http"
	webhooksapp "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *webhooksapp.Service
	appURL  string
}

type createIncomingRequest struct {
	ChannelID string `json:"channel_id"`
	Name      string `json:"name"`
}

type updateIncomingRequest struct {
	ChannelID *string `json:"channel_id"`
	Name      *string `json:"name"`
	Status    *string `json:"status"`
}

type createOutgoingRequest struct {
	Name       string   `json:"name"`
	TargetURL  string   `json:"target_url"`
	EventTypes []string `json:"event_types"`
}

type updateOutgoingRequest struct {
	Name       *string   `json:"name"`
	TargetURL  *string   `json:"target_url"`
	EventTypes *[]string `json:"event_types"`
	Status     *string   `json:"status"`
}

type testOutgoingRequest struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

type dispatchIncomingRequest struct {
	Secret    string          `json:"secret"`
	ChannelID string          `json:"channel_id"`
	Body      string          `json:"body"`
	Metadata  json.RawMessage `json:"metadata"`
}

type tokenMessageRequest struct {
	ChannelID string          `json:"channel_id"`
	Body      string          `json:"body"`
	Metadata  json.RawMessage `json:"metadata"`
}

func NewHandler(service *webhooksapp.Service, appURL string) *Handler {
	return &Handler{service: service, appURL: appURL}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	router.POST("/hooks/incoming/:webhook_id", h.DispatchIncoming)
	router.POST("/integrations/messages", h.SendTokenMessage)

	private := router.Group("/workspaces/:workspace_id")
	private.Use(authMiddleware)
	private.GET("/incoming-webhooks", h.ListIncoming)
	private.POST("/incoming-webhooks", h.CreateIncoming)
	private.PATCH("/incoming-webhooks/:webhook_id", h.UpdateIncoming)
	private.DELETE("/incoming-webhooks/:webhook_id", h.DeleteIncoming)
	private.GET("/outgoing-webhooks", h.ListOutgoing)
	private.POST("/outgoing-webhooks", h.CreateOutgoing)
	private.PATCH("/outgoing-webhooks/:webhook_id", h.UpdateOutgoing)
	private.DELETE("/outgoing-webhooks/:webhook_id", h.DeleteOutgoing)
	private.GET("/outgoing-webhooks/:webhook_id/deliveries", h.ListDeliveries)
	private.POST("/outgoing-webhooks/:webhook_id/test", h.TestOutgoing)
}

func (h *Handler) CreateIncoming(c *gin.Context) {
	var req createIncomingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	webhook, err := h.service.CreateIncoming(c.Request.Context(), webhooksapp.CreateIncomingInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   req.ChannelID,
		Name:        req.Name,
	}, h.appURL)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, webhook)
}

func (h *Handler) ListIncoming(c *gin.Context) {
	webhooks, err := h.service.ListIncoming(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"incoming_webhooks": webhooks})
}

func (h *Handler) UpdateIncoming(c *gin.Context) {
	var req updateIncomingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	webhook, err := h.service.UpdateIncoming(c.Request.Context(), webhooksapp.UpdateIncomingInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		WebhookID:   c.Param("webhook_id"),
		ChannelID:   req.ChannelID,
		Name:        req.Name,
		Status:      req.Status,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, webhook)
}

func (h *Handler) DeleteIncoming(c *gin.Context) {
	if err := h.service.DeleteIncoming(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("webhook_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) CreateOutgoing(c *gin.Context) {
	var req createOutgoingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	webhook, err := h.service.CreateOutgoing(c.Request.Context(), webhooksapp.CreateOutgoingInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Name:        req.Name,
		TargetURL:   req.TargetURL,
		EventTypes:  req.EventTypes,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, webhook)
}

func (h *Handler) ListOutgoing(c *gin.Context) {
	webhooks, err := h.service.ListOutgoing(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"outgoing_webhooks": webhooks})
}

func (h *Handler) UpdateOutgoing(c *gin.Context) {
	var req updateOutgoingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	webhook, err := h.service.UpdateOutgoing(c.Request.Context(), webhooksapp.UpdateOutgoingInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		WebhookID:   c.Param("webhook_id"),
		Name:        req.Name,
		TargetURL:   req.TargetURL,
		EventTypes:  req.EventTypes,
		Status:      req.Status,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, webhook)
}

func (h *Handler) DeleteOutgoing(c *gin.Context) {
	if err := h.service.DeleteOutgoing(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("webhook_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) ListDeliveries(c *gin.Context) {
	deliveries, err := h.service.ListDeliveries(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("webhook_id"), queryInt(c, "limit"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"deliveries": deliveries})
}

func (h *Handler) TestOutgoing(c *gin.Context) {
	var req testOutgoingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	delivery, err := h.service.TestOutgoing(c.Request.Context(), webhooksapp.TestOutgoingInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		WebhookID:   c.Param("webhook_id"),
		EventType:   req.EventType,
		Payload:     req.Payload,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, delivery)
}

func (h *Handler) DispatchIncoming(c *gin.Context) {
	var req dispatchIncomingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	if req.Secret == "" {
		req.Secret = c.GetHeader("X-WebTui-Webhook-Secret")
	}
	message, err := h.service.DispatchIncoming(c.Request.Context(), webhooksapp.IncomingMessageInput{
		ExpectedZoneID: tenancyhttp.CurrentZoneID(c),
		WebhookID:      c.Param("webhook_id"),
		Secret:         req.Secret,
		ChannelID:      req.ChannelID,
		Body:           req.Body,
		Metadata:       req.Metadata,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, message)
}

func (h *Handler) SendTokenMessage(c *gin.Context) {
	var req tokenMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	message, err := h.service.SendTokenMessage(c.Request.Context(), webhooksapp.TokenMessageInput{
		ExpectedZoneID: tenancyhttp.CurrentZoneID(c),
		Token:          bearerToken(c.GetHeader("Authorization")),
		ChannelID:      req.ChannelID,
		Body:           req.Body,
		Metadata:       req.Metadata,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, message)
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return header
}

func queryInt(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return value
}
