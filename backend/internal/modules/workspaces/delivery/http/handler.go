package http

import (
	"encoding/json"
	nethttp "net/http"

	workspacesapp "github.com/duclamdev/application-chat/backend/internal/modules/workspaces/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *workspacesapp.Service
}

type createWorkspaceRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateWorkspaceRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type addMemberRequest struct {
	UserID   string `json:"user_id"`
	Title    string `json:"title"`
	RoleCode string `json:"role_code"`
}

type updateMemberStatusRequest struct {
	Status string `json:"status"`
}

type upsertSettingRequest struct {
	Value       json.RawMessage `json:"value"`
	ValueType   string          `json:"value_type"`
	Description string          `json:"description"`
}

type createInviteRequest struct {
	Email       string `json:"email"`
	RoleCode    string `json:"role_code"`
	ExpiresDays int    `json:"expires_days"`
}

func NewHandler(service *workspacesapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("")
	private.Use(authMiddleware)
	private.GET("", h.ListMine)
	private.POST("", h.Create)
	private.GET("/:workspace_id", h.Get)
	private.PATCH("/:workspace_id", h.Update)
	private.DELETE("/:workspace_id", h.Archive)
	private.GET("/:workspace_id/members", h.ListMembers)
	private.POST("/:workspace_id/members", h.AddMember)
	private.PATCH("/:workspace_id/members/:user_id", h.UpdateMemberStatus)
	private.GET("/:workspace_id/settings", h.ListSettings)
	private.PUT("/:workspace_id/settings/:key", h.UpsertSetting)
	private.GET("/:workspace_id/invites", h.ListInvites)
	private.POST("/:workspace_id/invites", h.CreateInvite)
}

func (h *Handler) Create(c *gin.Context) {
	var req createWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	workspace, err := h.service.Create(c.Request.Context(), workspacesapp.CreateWorkspaceInput{
		ActorUserID: middleware.CurrentUserID(c),
		ZoneID:      middleware.CurrentZoneID(c),
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, workspace)
}

func (h *Handler) ListMine(c *gin.Context) {
	workspaces, err := h.service.ListMine(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"workspaces": workspaces})
}

func (h *Handler) Get(c *gin.Context) {
	workspace, err := h.service.Get(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, workspace)
}

func (h *Handler) Update(c *gin.Context) {
	var req updateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	workspace, err := h.service.Update(c.Request.Context(), workspacesapp.UpdateWorkspaceInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, workspace)
}

func (h *Handler) Archive(c *gin.Context) {
	if err := h.service.Archive(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) ListMembers(c *gin.Context) {
	members, err := h.service.ListMembers(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"members": members})
}

func (h *Handler) AddMember(c *gin.Context) {
	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	member, err := h.service.AddMember(c.Request.Context(), workspacesapp.AddMemberInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		UserID:      req.UserID,
		Title:       req.Title,
		RoleCode:    req.RoleCode,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, member)
}

func (h *Handler) UpdateMemberStatus(c *gin.Context) {
	var req updateMemberStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	member, err := h.service.UpdateMemberStatus(c.Request.Context(), workspacesapp.UpdateMemberStatusInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		UserID:      c.Param("user_id"),
		Status:      req.Status,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, member)
}

func (h *Handler) ListSettings(c *gin.Context) {
	settings, err := h.service.ListSettings(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"settings": settings})
}

func (h *Handler) UpsertSetting(c *gin.Context) {
	var req upsertSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	setting, err := h.service.UpsertSetting(c.Request.Context(), workspacesapp.UpsertSettingInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Key:         c.Param("key"),
		Value:       req.Value,
		ValueType:   req.ValueType,
		Description: req.Description,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, setting)
}

func (h *Handler) ListInvites(c *gin.Context) {
	invites, err := h.service.ListInvites(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"invites": invites})
}

func (h *Handler) CreateInvite(c *gin.Context) {
	var req createInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	invite, err := h.service.CreateInvite(c.Request.Context(), workspacesapp.CreateInviteInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Email:       req.Email,
		RoleCode:    req.RoleCode,
		ExpiresDays: req.ExpiresDays,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, invite)
}
