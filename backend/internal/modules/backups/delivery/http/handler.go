package http

import (
	"encoding/json"
	nethttp "net/http"
	"strconv"

	backupsapp "github.com/duclamdev/application-chat/backend/internal/modules/backups/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *backupsapp.Service
}

type createJobRequest struct {
	Name       string          `json:"name"`
	Target     string          `json:"target"`
	BackupType string          `json:"backup_type"`
	Schedule   string          `json:"schedule"`
	Status     string          `json:"status"`
	Config     json.RawMessage `json:"config"`
}

func NewHandler(service *backupsapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id/backup-jobs")
	private.Use(authMiddleware)
	private.GET("", h.ListJobs)
	private.POST("", h.CreateJob)
	private.GET("/:backup_job_id/runs", h.ListRuns)
	private.POST("/:backup_job_id/run", h.RunNow)
}

func (h *Handler) ListJobs(c *gin.Context) {
	jobs, err := h.service.ListJobs(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), queryInt(c, "limit"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"backup_jobs": jobs})
}

func (h *Handler) CreateJob(c *gin.Context) {
	var req createJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	job, err := h.service.CreateJob(c.Request.Context(), backupsapp.CreateJobInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Name:        req.Name,
		Target:      req.Target,
		BackupType:  req.BackupType,
		Schedule:    req.Schedule,
		Status:      req.Status,
		Config:      req.Config,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, job)
}

func (h *Handler) ListRuns(c *gin.Context) {
	runs, err := h.service.ListRuns(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("backup_job_id"), queryInt(c, "limit"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"backup_runs": runs})
}

func (h *Handler) RunNow(c *gin.Context) {
	run, err := h.service.RunNow(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("backup_job_id"))
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
