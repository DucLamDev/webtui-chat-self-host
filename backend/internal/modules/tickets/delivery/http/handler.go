package http

import (
	nethttp "net/http"
	"strconv"

	ticketsapp "github.com/duclamdev/application-chat/backend/internal/modules/tickets/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *ticketsapp.Service
}

type createTicketRequest struct {
	ChannelID   string `json:"channel_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	AssignedTo  string `json:"assigned_to"`
}

type updateTicketRequest struct {
	ChannelID   *string `json:"channel_id"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	AssignedTo  *string `json:"assigned_to"`
}

func NewHandler(service *ticketsapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id/tickets")
	private.Use(authMiddleware)
	private.GET("", h.List)
	private.POST("", h.Create)
	private.GET("/:ticket_id", h.Get)
	private.PATCH("/:ticket_id", h.Update)
}

func (h *Handler) List(c *gin.Context) {
	tickets, err := h.service.List(c.Request.Context(), ticketsapp.ListInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Status:      c.Query("status"),
		Limit:       queryInt(c, "limit"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"tickets": tickets})
}

func (h *Handler) Create(c *gin.Context) {
	var req createTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON khong hop le.", nil)
		return
	}
	ticket, err := h.service.Create(c.Request.Context(), ticketsapp.CreateInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   req.ChannelID,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		AssignedTo:  req.AssignedTo,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, ticket)
}

func (h *Handler) Get(c *gin.Context) {
	ticket, err := h.service.Get(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("ticket_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, ticket)
}

func (h *Handler) Update(c *gin.Context) {
	var req updateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON khong hop le.", nil)
		return
	}
	ticket, err := h.service.Update(c.Request.Context(), ticketsapp.UpdateInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		TicketID:    c.Param("ticket_id"),
		ChannelID:   req.ChannelID,
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Priority:    req.Priority,
		AssignedTo:  req.AssignedTo,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, ticket)
}

func queryInt(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return value
}
