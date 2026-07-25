package http

import (
	nethttp "net/http"

	contactsapp "github.com/duclamdev/application-chat/backend/internal/modules/contacts/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *contactsapp.Service
}

type createContactRequest struct {
	UserID string `json:"user_id"`
}

func NewHandler(service *contactsapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("")
	private.Use(authMiddleware)

	private.GET("/contacts", h.ListContacts)
	private.GET("/contact-requests", h.ListRequests)
	private.POST("/contact-requests", h.SendRequest)
	private.POST("/contact-requests/:request_id/accept", h.AcceptRequest)
	private.POST("/contact-requests/:request_id/reject", h.RejectRequest)
	private.DELETE("/contact-requests/:request_id", h.CancelRequest)
}

func (h *Handler) ListContacts(c *gin.Context) {
	contacts, err := h.service.ListContacts(c.Request.Context(), middleware.CurrentZoneID(c), middleware.CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"contacts": contacts})
}

func (h *Handler) ListRequests(c *gin.Context) {
	requests, err := h.service.ListRequests(c.Request.Context(), middleware.CurrentZoneID(c), middleware.CurrentUserID(c), c.Query("status"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"contact_requests": requests})
}

func (h *Handler) SendRequest(c *gin.Context) {
	var req createContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	request, err := h.service.SendRequest(c.Request.Context(), middleware.CurrentZoneID(c), middleware.CurrentUserID(c), req.UserID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, request)
}

func (h *Handler) AcceptRequest(c *gin.Context) {
	request, err := h.service.AcceptRequest(c.Request.Context(), middleware.CurrentZoneID(c), middleware.CurrentUserID(c), c.Param("request_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, request)
}

func (h *Handler) RejectRequest(c *gin.Context) {
	request, err := h.service.RejectRequest(c.Request.Context(), middleware.CurrentZoneID(c), middleware.CurrentUserID(c), c.Param("request_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, request)
}

func (h *Handler) CancelRequest(c *gin.Context) {
	if err := h.service.CancelRequest(c.Request.Context(), middleware.CurrentZoneID(c), middleware.CurrentUserID(c), c.Param("request_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}
