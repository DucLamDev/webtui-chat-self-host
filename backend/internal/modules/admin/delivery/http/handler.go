package http

import (
	"context"
	nethttp "net/http"
	"strconv"
	"time"

	adminapp "github.com/duclamdev/application-chat/backend/internal/modules/admin/application"
	healthhttp "github.com/duclamdev/application-chat/backend/internal/modules/health/delivery/http"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *adminapp.Service
	checks  map[string]healthhttp.CheckFunc
}

func NewHandler(service *adminapp.Service, checks map[string]healthhttp.CheckFunc) *Handler {
	return &Handler{service: service, checks: checks}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id/admin")
	private.Use(authMiddleware)
	private.GET("/stats", h.Stats)
	private.GET("/health", h.Health)
	private.GET("/channels", h.Channels)
	private.GET("/messages", h.Messages)
}

func (h *Handler) Channels(c *gin.Context) {
	channels, err := h.service.Channels(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"channels": channels})
}

func (h *Handler) Messages(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	messages, err := h.service.RecentMessages(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), limit)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"messages": messages})
}

func (h *Handler) Stats(c *gin.Context) {
	stats, err := h.service.Dashboard(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, stats)
}

func (h *Handler) Health(c *gin.Context) {
	if err := h.service.EnsureAdminPermission(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id")); err != nil {
		response.Error(c, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	status := "ready"
	code := nethttp.StatusOK
	checks := gin.H{"api": "ok"}
	for name, check := range h.checks {
		if check == nil {
			continue
		}
		if err := check(ctx); err != nil {
			status = "not_ready"
			code = nethttp.StatusServiceUnavailable
			checks[name] = err.Error()
			continue
		}
		checks[name] = "ok"
	}
	response.OK(c, code, gin.H{
		"status": status,
		"checks": checks,
	})
}
