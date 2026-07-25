package http

import (
	nethttp "net/http"

	rbacapp "github.com/duclamdev/application-chat/backend/internal/modules/rbac/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *rbacapp.Service
}

type createRoleRequest struct {
	WorkspaceID     string   `json:"workspace_id"`
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	PermissionCodes []string `json:"permission_codes"`
}

type assignRoleRequest struct {
	RoleID string `json:"role_id"`
}

func NewHandler(service *rbacapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("")
	private.Use(authMiddleware)
	private.GET("/permissions", h.ListPermissions)
	private.GET("/roles", h.ListRoles)
	private.POST("/roles", h.CreateRole)
	private.GET("/me", h.MyPermissions)
	private.GET("/check", h.Check)
	private.GET("/workspaces/:workspace_id/members/:user_id/roles", h.ListMemberRoles)
	private.POST("/workspaces/:workspace_id/members/:user_id/roles", h.AssignMemberRole)
	private.DELETE("/workspaces/:workspace_id/members/:user_id/roles/:role_id", h.RevokeMemberRole)
}

func (h *Handler) ListPermissions(c *gin.Context) {
	permissions, err := h.service.ListPermissions(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"permissions": permissions})
}

func (h *Handler) ListRoles(c *gin.Context) {
	roles, err := h.service.ListRoles(c.Request.Context(), c.Query("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"roles": roles})
}

func (h *Handler) CreateRole(c *gin.Context) {
	var req createRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}

	role, err := h.service.CreateRole(c.Request.Context(), rbacapp.CreateRoleInput{
		ActorUserID:     middleware.CurrentUserID(c),
		ZoneID:          middleware.CurrentZoneID(c),
		WorkspaceID:     req.WorkspaceID,
		Code:            req.Code,
		Name:            req.Name,
		Description:     req.Description,
		PermissionCodes: req.PermissionCodes,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, role)
}

func (h *Handler) MyPermissions(c *gin.Context) {
	permissions, err := h.service.ListMyWorkspacePermissions(c.Request.Context(), middleware.CurrentUserID(c), c.Query("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"permissions": permissions})
}

func (h *Handler) Check(c *gin.Context) {
	allowed, err := h.service.HasWorkspacePermission(c.Request.Context(), middleware.CurrentUserID(c), c.Query("workspace_id"), c.Query("permission"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"allowed": allowed})
}

func (h *Handler) ListMemberRoles(c *gin.Context) {
	roles, err := h.service.ListWorkspaceMemberRoles(c.Request.Context(), c.Param("workspace_id"), c.Param("user_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"roles": roles})
}

func (h *Handler) AssignMemberRole(c *gin.Context) {
	var req assignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}

	if err := h.service.AssignWorkspaceRole(c.Request.Context(), rbacapp.AssignWorkspaceRoleInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		UserID:      c.Param("user_id"),
		RoleID:      req.RoleID,
	}); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) RevokeMemberRole(c *gin.Context) {
	if err := h.service.RevokeWorkspaceRole(c.Request.Context(), rbacapp.AssignWorkspaceRoleInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		UserID:      c.Param("user_id"),
		RoleID:      c.Param("role_id"),
	}); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
