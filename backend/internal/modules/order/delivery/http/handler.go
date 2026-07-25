package http

import (
	nethttp "net/http"

	orderapp "github.com/duclamdev/application-chat/backend/internal/modules/order/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *orderapp.Service
}

type walletBalanceRequest struct {
	Email         string `json:"email"`
	UserID        int    `json:"user_id"`
	ChannelID     string `json:"channel_id"`
	PostToChannel *bool  `json:"post_to_channel"`
}

type walletDepositQRRequest struct {
	Email          string `json:"email"`
	Amount         int    `json:"amount"`
	ExpiresMinutes int    `json:"expires_minutes"`
	ChannelID      string `json:"channel_id"`
	PostToChannel  *bool  `json:"post_to_channel"`
}

type orderPaymentQRRequest struct {
	IntentID      int    `json:"intent_id"`
	IntentCode    string `json:"intent_code"`
	Reference     string `json:"reference"`
	ChannelID     string `json:"channel_id"`
	PostToChannel *bool  `json:"post_to_channel"`
}

type servicesExpiringRequest struct {
	Email          string `json:"email"`
	UserID         int    `json:"user_id"`
	Days           int    `json:"days"`
	IncludeExpired bool   `json:"include_expired"`
	ServiceType    string `json:"service_type"`
	ChannelID      string `json:"channel_id"`
	PostToChannel  *bool  `json:"post_to_channel"`
}

type renewServiceRequest struct {
	Email          string `json:"email"`
	UserID         int    `json:"user_id"`
	ServiceType    string `json:"service_type"`
	ServiceID      int    `json:"service_id"`
	ServiceName    string `json:"service_name"`
	Months         int    `json:"months"`
	IdempotencyKey string `json:"idempotency_key"`
	ChannelID      string `json:"channel_id"`
	PostToChannel  *bool  `json:"post_to_channel"`
}

func NewHandler(service *orderapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id/order-bot")
	private.Use(authMiddleware)
	private.GET("/status", h.Status)
	private.POST("/wallet/balance", h.WalletBalance)
	private.POST("/wallet/deposit-qr", h.CreateDepositQR)
	private.POST("/payment/order-qr", h.CreateOrderPaymentQR)
	private.POST("/services/expiring", h.ServicesExpiring)
	private.POST("/services/renew", h.RenewService)
}

func (h *Handler) CreateOrderPaymentQR(c *gin.Context) {
	var req orderPaymentQRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	result, err := h.service.CreateOrderPaymentQR(c.Request.Context(), orderapp.OrderPaymentQRInput{
		ActorUserID:   middleware.CurrentUserID(c),
		WorkspaceID:   c.Param("workspace_id"),
		IntentID:      req.IntentID,
		IntentCode:    req.IntentCode,
		Reference:     req.Reference,
		ChannelID:     req.ChannelID,
		PostToChannel: req.PostToChannel,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, result)
}

func (h *Handler) Status(c *gin.Context) {
	status, err := h.service.Status(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, status)
}

func (h *Handler) WalletBalance(c *gin.Context) {
	var req walletBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	result, err := h.service.WalletBalance(c.Request.Context(), orderapp.WalletBalanceInput{
		ActorUserID:   middleware.CurrentUserID(c),
		WorkspaceID:   c.Param("workspace_id"),
		Email:         req.Email,
		UserID:        req.UserID,
		ChannelID:     req.ChannelID,
		PostToChannel: req.PostToChannel,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, result)
}

func (h *Handler) CreateDepositQR(c *gin.Context) {
	var req walletDepositQRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	result, err := h.service.CreateDepositQR(c.Request.Context(), orderapp.WalletDepositQRInput{
		ActorUserID:    middleware.CurrentUserID(c),
		WorkspaceID:    c.Param("workspace_id"),
		Email:          req.Email,
		Amount:         req.Amount,
		ExpiresMinutes: req.ExpiresMinutes,
		ChannelID:      req.ChannelID,
		PostToChannel:  req.PostToChannel,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, result)
}

func (h *Handler) ServicesExpiring(c *gin.Context) {
	var req servicesExpiringRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	result, err := h.service.ServicesExpiring(c.Request.Context(), orderapp.ServicesExpiringInput{
		ActorUserID:    middleware.CurrentUserID(c),
		WorkspaceID:    c.Param("workspace_id"),
		Email:          req.Email,
		UserID:         req.UserID,
		Days:           req.Days,
		IncludeExpired: req.IncludeExpired,
		ServiceType:    req.ServiceType,
		ChannelID:      req.ChannelID,
		PostToChannel:  req.PostToChannel,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, result)
}

func (h *Handler) RenewService(c *gin.Context) {
	var req renewServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	result, err := h.service.RenewService(c.Request.Context(), orderapp.RenewServiceInput{
		ActorUserID:      middleware.CurrentUserID(c),
		WorkspaceID:      c.Param("workspace_id"),
		TriggerMessageID: req.IdempotencyKey,
		Email:            req.Email,
		UserID:           req.UserID,
		ServiceType:      req.ServiceType,
		ServiceID:        req.ServiceID,
		ServiceName:      req.ServiceName,
		Months:           req.Months,
		ChannelID:        req.ChannelID,
		PostToChannel:    req.PostToChannel,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, result)
}
