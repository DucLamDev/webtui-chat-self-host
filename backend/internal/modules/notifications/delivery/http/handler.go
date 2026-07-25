package http

import (
	nethttp "net/http"
	"strconv"

	notificationsapp "github.com/duclamdev/application-chat/backend/internal/modules/notifications/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *notificationsapp.Service
}

type preferenceRequest struct {
	WorkspaceID  string `json:"workspace_id"`
	Mode         string `json:"mode"`
	Preview      *bool  `json:"preview"`
	QuietHours   *bool  `json:"quiet_hours"`
	QuietStart   string `json:"quiet_start"`
	QuietEnd     string `json:"quiet_end"`
	Sound        *bool  `json:"sound"`
	Vibrate      *bool  `json:"vibrate"`
	CallRinging  *bool  `json:"call_ringing"`
	BadgeEnabled *bool  `json:"badge_enabled"`
}

type channelPreferenceRequest struct {
	WorkspaceID string `json:"workspace_id"`
	ChannelID   string `json:"channel_id"`
	Mode        string `json:"mode"`
	MutedUntil  string `json:"muted_until"`
}

func NewHandler(service *notificationsapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/notifications")
	private.Use(authMiddleware)
	private.GET("", h.ListMine)
	private.GET("/preferences", h.GetPreferences)
	private.PUT("/preferences", h.UpsertPreferences)
	private.GET("/preferences/channels/:channel_id", h.GetChannelPreference)
	private.PUT("/preferences/channels/:channel_id", h.UpsertChannelPreference)
	private.PUT("/:notification_id/read", h.MarkRead)
	private.PUT("/read-all", h.MarkAllRead)
}

func (h *Handler) ListMine(c *gin.Context) {
	notifications, err := h.service.ListMine(c.Request.Context(), notificationsapp.ListParams{
		ZoneID:      middleware.CurrentZoneID(c),
		UserID:      middleware.CurrentUserID(c),
		WorkspaceID: c.Query("workspace_id"),
		Limit:       queryInt(c, "limit"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"notifications": notifications})
}

func (h *Handler) GetPreferences(c *gin.Context) {
	preference, err := h.service.GetPreference(c.Request.Context(), middleware.CurrentZoneID(c), middleware.CurrentUserID(c), c.Query("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, preference)
}

func (h *Handler) UpsertPreferences(c *gin.Context) {
	var req preferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	preference, err := h.service.UpsertPreference(c.Request.Context(), notificationsapp.PreferenceInput{
		ZoneID:       middleware.CurrentZoneID(c),
		UserID:       middleware.CurrentUserID(c),
		WorkspaceID:  requestWorkspaceID(c, req.WorkspaceID),
		Mode:         req.Mode,
		Preview:      req.Preview,
		QuietHours:   req.QuietHours,
		QuietStart:   req.QuietStart,
		QuietEnd:     req.QuietEnd,
		Sound:        req.Sound,
		Vibrate:      req.Vibrate,
		CallRinging:  req.CallRinging,
		BadgeEnabled: req.BadgeEnabled,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, preference)
}

func (h *Handler) GetChannelPreference(c *gin.Context) {
	preference, err := h.service.GetChannelPreference(
		c.Request.Context(),
		middleware.CurrentZoneID(c),
		middleware.CurrentUserID(c),
		c.Query("workspace_id"),
		c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, preference)
}

func (h *Handler) UpsertChannelPreference(c *gin.Context) {
	var req channelPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	channelID := req.ChannelID
	if channelID == "" {
		channelID = c.Param("channel_id")
	}
	preference, err := h.service.UpsertChannelPreference(c.Request.Context(), notificationsapp.ChannelPreferenceInput{
		ZoneID:      middleware.CurrentZoneID(c),
		UserID:      middleware.CurrentUserID(c),
		WorkspaceID: requestWorkspaceID(c, req.WorkspaceID),
		ChannelID:   channelID,
		Mode:        req.Mode,
		MutedUntil:  req.MutedUntil,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, preference)
}

func (h *Handler) MarkRead(c *gin.Context) {
	notification, err := h.service.MarkRead(c.Request.Context(), middleware.CurrentZoneID(c), middleware.CurrentUserID(c), c.Param("notification_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, notification)
}

func (h *Handler) MarkAllRead(c *gin.Context) {
	if err := h.service.MarkAllRead(c.Request.Context(), middleware.CurrentZoneID(c), middleware.CurrentUserID(c), c.Query("workspace_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func queryInt(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return value
}

func requestWorkspaceID(c *gin.Context, bodyWorkspaceID string) string {
	if bodyWorkspaceID != "" {
		return bodyWorkspaceID
	}
	return c.Query("workspace_id")
}
