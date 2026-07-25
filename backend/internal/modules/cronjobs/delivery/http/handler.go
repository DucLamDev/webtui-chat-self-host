package http

import (
	"encoding/json"
	nethttp "net/http"
	"strconv"

	cronjobsapp "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *cronjobsapp.Service
	nodeID  string
}

type saveCronJobRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Schedule    string          `json:"schedule"`
	Runner      string          `json:"runner"`
	Status      string          `json:"status"`
	Payload     json.RawMessage `json:"payload"`
}

func NewHandler(service *cronjobsapp.Service, nodeID string) *Handler {
	return &Handler{service: service, nodeID: nodeID}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id/cronjobs")
	private.Use(authMiddleware)
	private.GET("", h.List)
	private.POST("", h.Create)
	private.PATCH("/:cronjob_id", h.Update)
	private.DELETE("/:cronjob_id", h.Delete)
	private.GET("/:cronjob_id/runs", h.ListRuns)
	private.POST("/:cronjob_id/run", h.RunNow)
}

func (h *Handler) List(c *gin.Context) {
	jobs, err := h.service.List(c.Request.Context(), cronjobsapp.ListInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Status:      c.Query("status"),
		Limit:       queryInt(c, "limit"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"cronjobs": jobs})
}

func (h *Handler) Create(c *gin.Context) {
	var req saveCronJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	job, err := h.service.Create(c.Request.Context(), cronjobsapp.CreateInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Name:        req.Name,
		Description: req.Description,
		Schedule:    req.Schedule,
		Runner:      req.Runner,
		Status:      req.Status,
		Payload:     req.Payload,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, job)
}

func (h *Handler) Update(c *gin.Context) {
	var req saveCronJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	job, err := h.service.Update(c.Request.Context(), cronjobsapp.UpdateInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		JobID:       c.Param("cronjob_id"),
		Name:        req.Name,
		Description: req.Description,
		Schedule:    req.Schedule,
		Runner:      req.Runner,
		Status:      req.Status,
		Payload:     req.Payload,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, job)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("cronjob_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) ListRuns(c *gin.Context) {
	runs, err := h.service.ListRuns(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("cronjob_id"), queryInt(c, "limit"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"runs": runs})
}

func (h *Handler) RunNow(c *gin.Context) {
	run, err := h.service.RunNow(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("cronjob_id"), h.nodeID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, run)
}

func queryInt(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return value
}
