package http

import (
	nethttp "net/http"
	"strconv"

	auditapp "github.com/duclamdev/application-chat/backend/internal/modules/audit/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *auditapp.Service
}

func NewHandler(service *auditapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id/audit-logs")
	private.Use(authMiddleware)
	private.GET("", h.List)
}

func (h *Handler) List(c *gin.Context) {
	logs, err := h.service.List(c.Request.Context(), auditapp.ListInput{
		ActorUserID:   middleware.CurrentUserID(c),
		WorkspaceID:   c.Param("workspace_id"),
		FilterActorID: c.Query("actor_user_id"),
		Action:        c.Query("action"),
		EntityType:    c.Query("entity_type"),
		EntityID:      c.Query("entity_id"),
		From:          c.Query("from"),
		To:            c.Query("to"),
		Limit:         queryInt(c, "limit"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"audit_logs": logs})
}

func queryInt(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return value
}
