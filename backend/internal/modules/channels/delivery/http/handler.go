package http

import (
	nethttp "net/http"

	channelsapp "github.com/duclamdev/application-chat/backend/internal/modules/channels/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *channelsapp.Service
}

type createChannelRequest struct {
	DepartmentID string `json:"department_id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Type         string `json:"type"`
}

type updateChannelRequest struct {
	DepartmentID *string `json:"department_id"`
	Name         *string `json:"name"`
	Description  *string `json:"description"`
}

type addChannelMemberRequest struct {
	UserID string `json:"user_id"`
}

type updateChannelMemberStatusRequest struct {
	Status string `json:"status"`
}

type updateReadStateRequest struct {
	LastReadMessageID string `json:"last_read_message_id"`
}

type createDirectRequest struct {
	ParticipantIDs []string `json:"participant_ids"`
}

func NewHandler(service *channelsapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id")
	private.Use(authMiddleware)
	private.GET("/channels", h.List)
	private.POST("/channels", h.Create)
	private.GET("/channels/:channel_id", h.Get)
	private.PATCH("/channels/:channel_id", h.Update)
	private.DELETE("/channels/:channel_id", h.Archive)
	private.GET("/channels/:channel_id/members", h.ListMembers)
	private.POST("/channels/:channel_id/members", h.AddMember)
	private.PATCH("/channels/:channel_id/members/:user_id", h.UpdateMemberStatus)
	private.POST("/channels/:channel_id/join-requests", h.RequestJoin)
	private.GET("/channels/:channel_id/join-requests", h.ListJoinRequests)
	private.POST("/channels/:channel_id/join-requests/:user_id/approve", h.ApproveJoinRequest)
	private.DELETE("/channels/:channel_id/join-requests/:user_id", h.RejectJoinRequest)
	private.PUT("/channels/:channel_id/read-state", h.UpdateReadState)
	private.POST("/channels/:channel_id/private-session", h.OpenPrivateSession)
	private.GET("/direct-conversations", h.ListDirects)
	private.POST("/direct-conversations", h.CreateDirect)
}

func (h *Handler) OpenPrivateSession(c *gin.Context) {
	channel, err := h.service.OpenPrivateSession(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, channel)
}

func (h *Handler) Create(c *gin.Context) {
	var req createChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	channel, err := h.service.Create(c.Request.Context(), channelsapp.CreateChannelInput{
		ActorUserID:  middleware.CurrentUserID(c),
		WorkspaceID:  c.Param("workspace_id"),
		DepartmentID: req.DepartmentID,
		Slug:         req.Slug,
		Name:         req.Name,
		Description:  req.Description,
		Type:         req.Type,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, channel)
}

func (h *Handler) List(c *gin.Context) {
	channels, err := h.service.List(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"channels": channels})
}

func (h *Handler) Get(c *gin.Context) {
	channel, err := h.service.Get(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, channel)
}

func (h *Handler) Update(c *gin.Context) {
	var req updateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	channel, err := h.service.Update(c.Request.Context(), channelsapp.UpdateChannelInput{
		ActorUserID:  middleware.CurrentUserID(c),
		WorkspaceID:  c.Param("workspace_id"),
		ChannelID:    c.Param("channel_id"),
		DepartmentID: req.DepartmentID,
		Name:         req.Name,
		Description:  req.Description,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, channel)
}

func (h *Handler) Archive(c *gin.Context) {
	if err := h.service.Archive(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) ListMembers(c *gin.Context) {
	members, err := h.service.ListMembers(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"members": members})
}

func (h *Handler) AddMember(c *gin.Context) {
	var req addChannelMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	member, err := h.service.AddMember(c.Request.Context(), channelsapp.AddMemberInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		UserID:      req.UserID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, member)
}

func (h *Handler) UpdateMemberStatus(c *gin.Context) {
	var req updateChannelMemberStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	member, err := h.service.UpdateMemberStatus(c.Request.Context(), channelsapp.UpdateMemberStatusInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		UserID:      c.Param("user_id"),
		Status:      req.Status,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, member)
}

func (h *Handler) RequestJoin(c *gin.Context) {
	member, err := h.service.RequestJoin(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, member)
}

func (h *Handler) ListJoinRequests(c *gin.Context) {
	members, err := h.service.ListJoinRequests(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"join_requests": members})
}

func (h *Handler) ApproveJoinRequest(c *gin.Context) {
	member, err := h.service.ApproveJoinRequest(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id"), c.Param("user_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, member)
}

func (h *Handler) RejectJoinRequest(c *gin.Context) {
	if err := h.service.RejectJoinRequest(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id"), c.Param("user_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) UpdateReadState(c *gin.Context) {
	var req updateReadStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	member, err := h.service.UpdateReadState(c.Request.Context(), channelsapp.UpdateReadStateInput{
		ActorUserID:       middleware.CurrentUserID(c),
		WorkspaceID:       c.Param("workspace_id"),
		ChannelID:         c.Param("channel_id"),
		LastReadMessageID: req.LastReadMessageID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, member)
}

func (h *Handler) CreateDirect(c *gin.Context) {
	var req createDirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	conversation, err := h.service.CreateDirect(c.Request.Context(), channelsapp.CreateDirectInput{
		ActorUserID:    middleware.CurrentUserID(c),
		WorkspaceID:    c.Param("workspace_id"),
		ParticipantIDs: req.ParticipantIDs,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, conversation)
}

func (h *Handler) ListDirects(c *gin.Context) {
	conversations, err := h.service.ListDirects(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"direct_conversations": conversations})
}
