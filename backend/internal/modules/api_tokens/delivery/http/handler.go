package http

import (
	nethttp "net/http"

	aptokensapp "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *aptokensapp.Service
}

type createTokenRequest struct {
	Name        string   `json:"name"`
	Scopes      []string `json:"scopes"`
	ExpiresDays int      `json:"expires_days"`
}

func NewHandler(service *aptokensapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("")
	private.Use(authMiddleware)
	private.GET("/api-scopes", h.ListScopes)
	private.GET("/workspaces/:workspace_id/api-tokens", h.ListTokens)
	private.POST("/workspaces/:workspace_id/api-tokens", h.CreateToken)
	private.DELETE("/workspaces/:workspace_id/api-tokens/:token_id", h.RevokeToken)
}

func (h *Handler) ListScopes(c *gin.Context) {
	scopes, err := h.service.ListScopes(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"scopes": scopes})
}

func (h *Handler) ListTokens(c *gin.Context) {
	tokens, err := h.service.ListTokens(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"api_tokens": tokens})
}

func (h *Handler) CreateToken(c *gin.Context) {
	var req createTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	token, err := h.service.CreateToken(c.Request.Context(), aptokensapp.CreateTokenInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Name:        req.Name,
		ScopeCodes:  req.Scopes,
		ExpiresDays: req.ExpiresDays,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, token)
}

func (h *Handler) RevokeToken(c *gin.Context) {
	if err := h.service.RevokeToken(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("token_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
