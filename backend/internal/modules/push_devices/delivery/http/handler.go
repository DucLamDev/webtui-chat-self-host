package http

import (
	nethttp "net/http"

	devicesapp "github.com/duclamdev/application-chat/backend/internal/modules/push_devices/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *devicesapp.Service
}

type upsertDeviceRequest struct {
	WorkspaceID            string `json:"workspace_id"`
	DeviceID               string `json:"device_id"`
	Platform               string `json:"platform"`
	PushProvider           string `json:"push_provider"`
	PushToken              string `json:"push_token"`
	NotificationPermission string `json:"notification_permission"`
	AppVersion             string `json:"app_version"`
	BuildNumber            string `json:"build_number"`
	ReleaseChannel         string `json:"release_channel"`
	Locale                 string `json:"locale"`
	Timezone               string `json:"timezone"`
}

func NewHandler(service *devicesapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/mobile/devices")
	private.Use(authMiddleware)
	private.GET("", h.ListMine)
	private.POST("", h.RegisterOrUpdate)
	private.PATCH("/:device_id", h.Update)
	private.DELETE("/:device_id", h.Delete)
}

func (h *Handler) ListMine(c *gin.Context) {
	devices, err := h.service.ListMine(c.Request.Context(), middleware.CurrentZoneID(c), middleware.CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"devices": devices})
}

func (h *Handler) RegisterOrUpdate(c *gin.Context) {
	var req upsertDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	device, err := h.service.RegisterOrUpdate(c.Request.Context(), toInput(c, req, req.DeviceID))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, device)
}

func (h *Handler) Update(c *gin.Context) {
	var req upsertDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	device, err := h.service.RegisterOrUpdate(c.Request.Context(), toInput(c, req, c.Param("device_id")))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, device)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), middleware.CurrentZoneID(c), middleware.CurrentUserID(c), c.Param("device_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func toInput(c *gin.Context, req upsertDeviceRequest, deviceID string) devicesapp.UpsertInput {
	return devicesapp.UpsertInput{
		ActorUserID:            middleware.CurrentUserID(c),
		ZoneID:                 middleware.CurrentZoneID(c),
		WorkspaceID:            firstNonEmpty(req.WorkspaceID, middleware.CurrentWorkspaceID(c)),
		DeviceID:               deviceID,
		Platform:               req.Platform,
		PushProvider:           req.PushProvider,
		PushToken:              req.PushToken,
		NotificationPermission: req.NotificationPermission,
		AppVersion:             req.AppVersion,
		BuildNumber:            req.BuildNumber,
		ReleaseChannel:         req.ReleaseChannel,
		Locale:                 req.Locale,
		Timezone:               req.Timezone,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
