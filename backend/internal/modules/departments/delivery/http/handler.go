package http

import (
	nethttp "net/http"

	departmentsapp "github.com/duclamdev/application-chat/backend/internal/modules/departments/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *departmentsapp.Service
}

type createDepartmentRequest struct {
	ParentID    string `json:"parent_id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateDepartmentRequest struct {
	ParentID    *string `json:"parent_id"`
	Slug        *string `json:"slug"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type addDepartmentMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func NewHandler(service *departmentsapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id/departments")
	private.Use(authMiddleware)
	private.GET("", h.List)
	private.POST("", h.Create)
	private.GET("/:department_id", h.Get)
	private.PATCH("/:department_id", h.Update)
	private.DELETE("/:department_id", h.Delete)
	private.GET("/:department_id/members", h.ListMembers)
	private.POST("/:department_id/members", h.AddMember)
	private.DELETE("/:department_id/members/:user_id", h.RemoveMember)
}

func (h *Handler) Create(c *gin.Context) {
	var req createDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	department, err := h.service.Create(c.Request.Context(), departmentsapp.CreateInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ParentID:    req.ParentID,
		Slug:        req.Slug,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, department)
}

func (h *Handler) List(c *gin.Context) {
	departments, err := h.service.List(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"departments": departments})
}

func (h *Handler) Get(c *gin.Context) {
	department, err := h.service.Get(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("department_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, department)
}

func (h *Handler) Update(c *gin.Context) {
	var req updateDepartmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	department, err := h.service.Update(c.Request.Context(), departmentsapp.UpdateInput{
		ActorUserID:  middleware.CurrentUserID(c),
		WorkspaceID:  c.Param("workspace_id"),
		DepartmentID: c.Param("department_id"),
		ParentID:     req.ParentID,
		Slug:         req.Slug,
		Name:         req.Name,
		Description:  req.Description,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, department)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("department_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) ListMembers(c *gin.Context) {
	members, err := h.service.ListMembers(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("department_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"members": members})
}

func (h *Handler) AddMember(c *gin.Context) {
	var req addDepartmentMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	member, err := h.service.AddMember(c.Request.Context(), departmentsapp.AddMemberInput{
		ActorUserID:  middleware.CurrentUserID(c),
		WorkspaceID:  c.Param("workspace_id"),
		DepartmentID: c.Param("department_id"),
		UserID:       req.UserID,
		Role:         req.Role,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, member)
}

func (h *Handler) RemoveMember(c *gin.Context) {
	if err := h.service.RemoveMember(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("department_id"), c.Param("user_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
