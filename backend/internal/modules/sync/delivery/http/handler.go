package http

import (
	nethttp "net/http"
	"strconv"

	syncapp "github.com/duclamdev/application-chat/backend/internal/modules/sync/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *syncapp.Service
}

type ackRequest struct {
	DeviceID string `json:"device_id"`
	Cursor   string `json:"cursor"`
}

func NewHandler(service *syncapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id/sync")
	private.Use(authMiddleware)
	private.GET("", h.CatchUp)
	private.POST("/ack", h.Ack)
}

func (h *Handler) CatchUp(c *gin.Context) {
	result, err := h.service.CatchUp(c.Request.Context(), syncapp.ListInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		DeviceID:    c.Query("device_id"),
		Cursor:      c.Query("cursor"),
		Limit:       queryInt(c, "limit"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, result)
}

func (h *Handler) Ack(c *gin.Context) {
	var req ackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	ack, err := h.service.Ack(c.Request.Context(), syncapp.AckInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		DeviceID:    req.DeviceID,
		Cursor:      req.Cursor,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, ack)
}

func queryInt(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return value
}
