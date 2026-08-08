package http

import (
	nethttp "net/http"
	"strconv"

	usersapp "github.com/duclamdev/application-chat/backend/internal/modules/users/application"
	tenantstorage "github.com/duclamdev/application-chat/backend/internal/platform/storage/tenant"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service         *usersapp.Service
	storageResolver *tenantstorage.Resolver
}

type updateMeRequest struct {
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
	PhoneNumber *string `json:"phone_number"`
	Locale      *string `json:"locale"`
	Timezone    *string `json:"timezone"`
}

type updateUserRequest struct {
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
	PhoneNumber *string `json:"phone_number"`
	Locale      *string `json:"locale"`
	Timezone    *string `json:"timezone"`
	Status      *string `json:"status"`
	WorkspaceID *string `json:"workspace_id"`
}

type deleteOwnAccountRequest struct {
	Confirmation            string `json:"confirmation"`
	OwnershipSuccessorEmail string `json:"ownership_successor_email"`
}

func NewHandler(service *usersapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) SetStorageResolver(resolver *tenantstorage.Resolver) {
	h.storageResolver = resolver
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc, legalAcceptanceMiddleware gin.HandlerFunc) {
	router.GET("/avatars/:zone_id/:user_id/:file_name", h.ServeAvatar)
	private := router.Group("")
	private.Use(authMiddleware)
	private.GET("/me", h.Me)
	private.DELETE("/me", h.DeleteMe)
	private.GET("", h.List)
	private.GET("/:user_id", h.Get)
	private.PATCH("/:user_id", h.Update)
	private.DELETE("/:user_id", h.Delete)

	// Display names and avatars are user-generated content visible to other
	// workspace members. Keep account deletion and admin moderation reachable,
	// but require current legal acceptance for self-authored profile content.
	ugc := private.Group("")
	ugc.Use(legalAcceptanceMiddleware)
	ugc.PATCH("/me", h.UpdateMe)
	ugc.POST("/me/avatar", h.UploadMyAvatar)
}

func (h *Handler) DeleteMe(c *gin.Context) {
	var req deleteOwnAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	if err := h.service.DeleteOwnAccount(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		req.Confirmation,
		req.OwnershipSuccessorEmail,
	); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) Me(c *gin.Context) {
	user, err := h.service.Me(c.Request.Context(), middleware.CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, user)
}

func (h *Handler) UpdateMe(c *gin.Context) {
	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}

	user, err := h.service.UpdateMe(c.Request.Context(), usersapp.UpdateProfileInput{
		UserID:      middleware.CurrentUserID(c),
		DisplayName: req.DisplayName,
		AvatarURL:   req.AvatarURL,
		PhoneNumber: req.PhoneNumber,
		Locale:      req.Locale,
		Timezone:    req.Timezone,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, user)
}

func (h *Handler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	users, meta, err := h.service.List(c.Request.Context(), usersapp.ListUsersParams{
		ActorUserID: middleware.CurrentUserID(c),
		Query:       c.Query("q"),
		Status:      c.Query("status"),
		ZoneID:      middleware.CurrentZoneID(c),
		Limit:       limit,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OKWithMeta(c, nethttp.StatusOK, gin.H{"users": users}, meta)
}

func (h *Handler) Get(c *gin.Context) {
	user, err := h.service.Get(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("user_id"),
		middleware.CurrentZoneID(c),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, user)
}

func (h *Handler) Update(c *gin.Context) {
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	user, err := h.service.Update(c.Request.Context(), usersapp.UpdateUserInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: requestWorkspaceID(c, req.WorkspaceID),
		UserID:      c.Param("user_id"),
		DisplayName: req.DisplayName,
		AvatarURL:   req.AvatarURL,
		PhoneNumber: req.PhoneNumber,
		Locale:      req.Locale,
		Timezone:    req.Timezone,
		Status:      req.Status,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, user)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), middleware.CurrentUserID(c), c.Query("workspace_id"), c.Param("user_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func requestWorkspaceID(c *gin.Context, bodyWorkspaceID *string) string {
	if bodyWorkspaceID != nil {
		return *bodyWorkspaceID
	}
	return c.Query("workspace_id")
}
