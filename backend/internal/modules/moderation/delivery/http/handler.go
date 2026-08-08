package http

import (
	nethttp "net/http"
	"strconv"

	moderationapp "github.com/duclamdev/application-chat/backend/internal/modules/moderation/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *moderationapp.Service
}

type createReportRequest struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Reason     string `json:"reason"`
	Details    string `json:"details"`
}

type updateReportRequest struct {
	Status         string `json:"status"`
	ResolutionNote string `json:"resolution_note"`
}

type createBlockRequest struct {
	BlockedUserID string `json:"blocked_user_id"`
	Reason        string `json:"reason"`
}

func NewHandler(service *moderationapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id")
	private.Use(authMiddleware)

	private.POST("/moderation/reports", h.CreateReport)
	private.GET("/moderation/reports", h.ListReports)
	private.PATCH("/moderation/reports/:report_id", h.UpdateReport)

	private.GET("/blocks", h.ListBlocks)
	private.POST("/blocks", h.CreateBlock)
	private.DELETE("/blocks/:blocked_user_id", h.DeleteBlock)
}

func (h *Handler) CreateReport(c *gin.Context) {
	var req createReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Request body must be valid JSON.", nil)
		return
	}
	report, err := h.service.CreateReport(c.Request.Context(), moderationapp.CreateReportInput{
		ActorUserID: middleware.CurrentUserID(c), WorkspaceID: c.Param("workspace_id"),
		TargetType: req.TargetType, TargetID: req.TargetID, Reason: req.Reason, Details: req.Details,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, report)
}

func (h *Handler) ListReports(c *gin.Context) {
	reports, err := h.service.ListReports(c.Request.Context(), moderationapp.ListReportsInput{
		ActorUserID: middleware.CurrentUserID(c), WorkspaceID: c.Param("workspace_id"),
		Status: c.Query("status"), TargetType: c.Query("target_type"),
		Limit: queryInt(c, "limit"), Offset: queryInt(c, "offset"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"reports": reports})
}

func (h *Handler) UpdateReport(c *gin.Context) {
	var req updateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Request body must be valid JSON.", nil)
		return
	}
	report, err := h.service.UpdateReport(c.Request.Context(), moderationapp.UpdateReportInput{
		ActorUserID: middleware.CurrentUserID(c), WorkspaceID: c.Param("workspace_id"),
		ReportID: c.Param("report_id"), Status: req.Status, Resolution: req.ResolutionNote,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, report)
}

func (h *Handler) CreateBlock(c *gin.Context) {
	var req createBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Request body must be valid JSON.", nil)
		return
	}
	block, err := h.service.CreateBlock(c.Request.Context(), moderationapp.CreateBlockInput{
		ActorUserID: middleware.CurrentUserID(c), WorkspaceID: c.Param("workspace_id"),
		BlockedUserID: req.BlockedUserID, Reason: req.Reason,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, block)
}

func (h *Handler) ListBlocks(c *gin.Context) {
	blocks, err := h.service.ListBlocks(
		c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"blocks": blocks})
}

func (h *Handler) DeleteBlock(c *gin.Context) {
	if err := h.service.DeleteBlock(
		c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("blocked_user_id"),
	); err != nil {
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
