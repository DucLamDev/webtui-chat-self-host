package http

import (
	"encoding/json"
	nethttp "net/http"
	"strconv"

	presenceapp "github.com/duclamdev/application-chat/backend/internal/modules/presence/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *presenceapp.Service
}

type heartbeatRequest struct {
	DeviceID string          `json:"device_id"`
	SocketID string          `json:"socket_id"`
	NodeID   string          `json:"node_id"`
	Status   string          `json:"status"`
	Metadata json.RawMessage `json:"metadata"`
}

func NewHandler(service *presenceapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id/presence")
	private.Use(authMiddleware)
	private.GET("", h.List)
	private.PUT("/heartbeat", h.Heartbeat)
}

func (h *Handler) List(c *gin.Context) {
	presences, meta, err := h.service.List(c.Request.Context(), presenceapp.ListInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Limit:       queryInt(c, "limit"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OKWithMeta(c, nethttp.StatusOK, gin.H{"presence": presences}, meta)
}

func (h *Handler) Heartbeat(c *gin.Context) {
	var req heartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	presence, err := h.service.Heartbeat(c.Request.Context(), presenceapp.UpsertInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		DeviceID:    req.DeviceID,
		SocketID:    req.SocketID,
		NodeID:      req.NodeID,
		Status:      req.Status,
		Metadata:    req.Metadata,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, presence)
}

func queryInt(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return value
}
