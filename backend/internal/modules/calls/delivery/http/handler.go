package http

import (
	"encoding/json"
	nethttp "net/http"

	callsapp "github.com/duclamdev/application-chat/backend/internal/modules/calls/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *callsapp.Service
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
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id/calls")
	private.Use(authMiddleware)
	private.POST("", h.Create)
	private.GET("/:call_id", h.Get)
	private.POST("/:call_id/accept", h.Accept)
	private.POST("/:call_id/reject", h.Reject)
	private.POST("/:call_id/cancel", h.Cancel)
	private.POST("/:call_id/hangup", h.Hangup)
	private.POST("/:call_id/miss", h.Miss)
	private.POST("/:call_id/signals", h.Signal)
}

func (h *Handler) Create(c *gin.Context) {
	var req createCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	clientCallID := req.ClientCallID
	if clientCallID == "" {
		clientCallID = c.GetHeader("Idempotency-Key")
	}
	call, err := h.service.Create(c.Request.Context(), callsapp.CreateInput{
		ActorUserID:  middleware.CurrentUserID(c),
		WorkspaceID:  c.Param("workspace_id"),
		ChannelID:    req.ChannelID,
		TargetUserID: req.TargetUserID,
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
