package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	cronjobsdomain "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const (
	PermissionManageCronJob = "cronjob.manage"
	defaultCronLimit        = 20
	cronLockTTL             = 5 * time.Minute
)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	List(ctx context.Context, status string, limit int) ([]cronjobsdomain.CronJob, error)
	Get(ctx context.Context, jobID string) (cronjobsdomain.CronJob, error)
	Create(ctx context.Context, params SaveCronJobParams) (cronjobsdomain.CronJob, error)
	Update(ctx context.Context, params SaveCronJobParams) (cronjobsdomain.CronJob, error)
	Delete(ctx context.Context, jobID string) error
	ListRuns(ctx context.Context, jobID string, limit int) ([]cronjobsdomain.CronJobRun, error)
	ClaimDue(ctx context.Context, limit int, nodeID string, lockTTL time.Duration) ([]cronjobsdomain.CronJob, error)
	StartRun(ctx context.Context, jobID string) (cronjobsdomain.CronJobRun, error)
	FinishRun(ctx context.Context, params FinishRunParams) error
	CleanupExpiredSessions(ctx context.Context, olderThan time.Duration) (int, error)
	CleanupDeadOutbox(ctx context.Context, olderThan time.Duration) (int, error)
	CleanupFailedUploads(ctx context.Context, olderThan time.Duration) (int, error)
	CleanupOldPresence(ctx context.Context, olderThan time.Duration) (int, error)
}

type Service struct {
	repo            Repository
	checker         PermissionChecker
	now             func() time.Time
	scriptAllowlist map[string]string
}

type Option func(*Service)

type ListInput struct {
	ActorUserID string
	WorkspaceID string
	Status      string
	Limit       int
}

type CreateInput struct {
	ActorUserID string
	WorkspaceID string
	Name        string
	Description string
	Schedule    string
	Runner      string
	Status      string
	Payload     json.RawMessage
}

type UpdateInput struct {
	ActorUserID string
	WorkspaceID string
	JobID       string
	Name        string
	Description string
	Schedule    string
	Runner      string
	Status      string
	Payload     json.RawMessage
}

type SaveCronJobParams struct {
	ID          string
	Name        string
	Description string
	Schedule    string
	Runner      string
	Status      string
	Payload     []byte
	NextRunAt   *time.Time
}

type FinishRunParams struct {
	RunID     string
	JobID     string
	Status    string
	Log       string
	Error     string
	NextRunAt *time.Time
}

type CronJobDTO struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Schedule    string          `json:"schedule"`
	Runner      string          `json:"runner"`
	Status      string          `json:"status"`
	Payload     json.RawMessage `json:"payload"`
	LastRunAt   *string         `json:"last_run_at,omitempty"`
	NextRunAt   *string         `json:"next_run_at,omitempty"`
	LockedAt    *string         `json:"locked_at,omitempty"`
	LockedBy    *string         `json:"locked_by,omitempty"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

type CronJobRunDTO struct {
	ID         string  `json:"id"`
	CronJobID  string  `json:"cron_job_id"`
	Status     string  `json:"status"`
	StartedAt  string  `json:"started_at"`
	FinishedAt *string `json:"finished_at,omitempty"`
	Log        *string `json:"log,omitempty"`
	Error      *string `json:"error,omitempty"`
	DurationMS *int64  `json:"duration_ms,omitempty"`
}

func NewService(repo Repository, checker PermissionChecker, options ...Option) *Service {
	service := &Service{
		repo:            repo,
		checker:         checker,
		now:             func() time.Time { return time.Now().UTC() },
		scriptAllowlist: map[string]string{},
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func WithScriptAllowlist(allowlist map[string]string) Option {
	return func(service *Service) {
		service.scriptAllowlist = normalizeScriptAllowlist(allowlist)
	}
}

func (s *Service) List(ctx context.Context, input ListInput) ([]CronJobDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return nil, err
	}
	limit := normalizeLimit(input.Limit)
	jobs, err := s.repo.List(ctx, normalizeStatusFilter(input.Status), limit)
	if err != nil {
		return nil, err
	}
	return toCronJobDTOs(jobs), nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (CronJobDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return CronJobDTO{}, err
	}
	params, err := s.normalizeSaveInput(SaveCronJobParams{
		Name:        input.Name,
		Description: input.Description,
		Schedule:    input.Schedule,
		Runner:      input.Runner,
		Status:      input.Status,
		Payload:     input.Payload,
	})
	if err != nil {
		return CronJobDTO{}, err
	}
	job, err := s.repo.Create(ctx, params)
	if err != nil {
		return CronJobDTO{}, mapCronJobError(err)
	}
	return toCronJobDTO(job), nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (CronJobDTO, error) {
	if err := s.ensureManagePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return CronJobDTO{}, err
	}
	if strings.TrimSpace(input.JobID) == "" {
		return CronJobDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Thiếu cronjob_id.")
	}
	params, err := s.normalizeSaveInput(SaveCronJobParams{
		ID:          strings.TrimSpace(input.JobID),
		Name:        input.Name,
		Description: input.Description,
		Schedule:    input.Schedule,
		Runner:      input.Runner,
		Status:      input.Status,
		Payload:     input.Payload,
	})
	if err != nil {
		return CronJobDTO{}, err
	}
	job, err := s.repo.Update(ctx, params)
	if err != nil {
		return CronJobDTO{}, mapCronJobError(err)
	}
	return toCronJobDTO(job), nil
}

func (s *Service) Delete(ctx context.Context, actorUserID string, workspaceID string, jobID string) error {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, strings.TrimSpace(jobID)); err != nil {
		return mapCronJobError(err)
	}
	return nil
}

func (s *Service) ListRuns(ctx context.Context, actorUserID string, workspaceID string, jobID string, limit int) ([]CronJobRunDTO, error) {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	runs, err := s.repo.ListRuns(ctx, strings.TrimSpace(jobID), normalizeLimit(limit))
	if err != nil {
		return nil, err
	}
	return toCronJobRunDTOs(runs), nil
}

func (s *Service) RunNow(ctx context.Context, actorUserID string, workspaceID string, jobID string, nodeID string) (CronJobRunDTO, error) {
	if err := s.ensureManagePermission(ctx, actorUserID, workspaceID); err != nil {
		return CronJobRunDTO{}, err
	}
	job, err := s.repo.Get(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return CronJobRunDTO{}, mapCronJobError(err)
	}
	run, err := s.runJob(ctx, job, nodeID)
	if err != nil {
		return CronJobRunDTO{}, err
	}
	return toCronJobRunDTO(run), nil
}

func (s *Service) ProcessDue(ctx context.Context, limit int, nodeID string) (int, error) {
	if limit <= 0 {
		limit = defaultCronLimit
	}
	if strings.TrimSpace(nodeID) == "" {
		nodeID = "worker"
	}
	jobs, err := s.repo.ClaimDue(ctx, limit, nodeID, cronLockTTL)
	if err != nil {
		return 0, err
	}
	for _, job := range jobs {
		if _, err := s.runJob(ctx, job, nodeID); err != nil {
			return 0, err
		}
	}
	return len(jobs), nil
}

func (s *Service) runJob(ctx context.Context, job cronjobsdomain.CronJob, nodeID string) (cronjobsdomain.CronJobRun, error) {
	run, err := s.repo.StartRun(ctx, job.ID)
	if err != nil {
		return cronjobsdomain.CronJobRun{}, err
	}

	result := s.execute(ctx, job)
	nextRun, nextErr := NextRun(job.Schedule, s.now())
	var nextRunPtr *time.Time
	if nextErr == nil && job.Status == "active" {
		nextRunPtr = &nextRun
	}
	errorText := result.Error
	if nextErr != nil {
		if errorText != "" {
			errorText += "; "
		}
		errorText += nextErr.Error()
	}
	status := result.Status
	if status == "" {
		status = "success"
	}
	if errorText != "" {
		status = "failed"
	}
	if err := s.repo.FinishRun(ctx, FinishRunParams{
		RunID:     run.ID,
		JobID:     job.ID,
		Status:    status,
		Log:       result.Log,
		Error:     errorText,
		NextRunAt: nextRunPtr,
	}); err != nil {
		return cronjobsdomain.CronJobRun{}, err
	}
	return s.lastRunSnapshot(run, status, result.Log, errorText), nil
}

func (s *Service) lastRunSnapshot(run cronjobsdomain.CronJobRun, status string, logText string, errorText string) cronjobsdomain.CronJobRun {
	now := s.now()
	run.Status = status
	run.FinishedAt = &now
	if strings.TrimSpace(logText) != "" {
		run.Log = &logText
	}
	if strings.TrimSpace(errorText) != "" {
		run.Error = &errorText
	}
	return run
}

func (s *Service) normalizeSaveInput(input SaveCronJobParams) (SaveCronJobParams, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.Schedule = strings.TrimSpace(input.Schedule)
	input.Runner = strings.TrimSpace(input.Runner)
	input.Status = strings.TrimSpace(input.Status)
	if input.Runner == "" {
		input.Runner = "builtin_cleanup"
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Name == "" || len([]rune(input.Name)) > 120 {
		return SaveCronJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên cronjob phải dài từ 1 đến 120 ký tự.")
	}
	if input.ID == "" && input.Schedule == "" {
		return SaveCronJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Lịch chạy cronjob không được để trống.")
	}
	if !validCronStatus(input.Status) {
		return SaveCronJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Trạng thái cronjob không hợp lệ.")
	}
	if !validCronRunner(input.Runner) {
		return SaveCronJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Runner cronjob không được hỗ trợ.")
	}
	payload := json.RawMessage(input.Payload)
	if len(payload) == 0 || strings.TrimSpace(string(payload)) == "" || strings.TrimSpace(string(payload)) == "null" {
		payload = json.RawMessage(`{}`)
	}
	if !json.Valid(payload) {
		return SaveCronJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Payload cronjob không phải JSON hợp lệ.")
	}
	if err := s.validateRunnerPayload(input.Runner, payload); err != nil {
		return SaveCronJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", err.Error()+".")
	}
	nextRun, err := NextRun(input.Schedule, s.now())
	if err != nil {
		return SaveCronJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", err.Error()+".")
	}
	if input.Status == "active" {
		input.NextRunAt = &nextRun
	}
	input.Payload = []byte(payload)
	return input, nil
}

func (s *Service) ensureManagePermission(ctx context.Context, userID string, workspaceID string) error {
	if s.checker == nil {
		return nil
	}
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), PermissionManageCronJob)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền quản lý cronjob.")
	}
	return nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 100 {
		return 50
	}
	return limit
}

func normalizeStatusFilter(status string) string {
	status = strings.TrimSpace(status)
	if validCronStatus(status) {
		return status
	}
	return ""
}

func validCronStatus(status string) bool {
	switch status {
	case "active", "paused", "disabled":
		return true
	default:
		return false
	}
}

func validCronRunner(runner string) bool {
	switch runner {
	case "http", "builtin_cleanup", "worker", "script":
		return true
	default:
		return false
	}
}

func mapCronJobError(err error) error {
	if errors.Is(err, cronjobsdomain.ErrCronJobNotFound) {
		return apperrors.NotFound("CRON_JOB_NOT_FOUND", "Không tìm thấy cronjob.")
	}
	if errors.Is(err, cronjobsdomain.ErrCronJobAlreadyExists) {
		return apperrors.Conflict("CRON_JOB_ALREADY_EXISTS", "Tên cronjob đã tồn tại.")
	}
	return err
}

func toCronJobDTOs(jobs []cronjobsdomain.CronJob) []CronJobDTO {
	dtos := make([]CronJobDTO, 0, len(jobs))
	for _, job := range jobs {
		dtos = append(dtos, toCronJobDTO(job))
	}
	return dtos
}

func toCronJobDTO(job cronjobsdomain.CronJob) CronJobDTO {
	payload := json.RawMessage(job.Payload)
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	return CronJobDTO{
		ID:          job.ID,
		Name:        job.Name,
		Description: job.Description,
		Schedule:    job.Schedule,
		Runner:      job.Runner,
		Status:      job.Status,
		Payload:     payload,
		LastRunAt:   formatOptionalTime(job.LastRunAt),
		NextRunAt:   formatOptionalTime(job.NextRunAt),
		LockedAt:    formatOptionalTime(job.LockedAt),
		LockedBy:    job.LockedBy,
		CreatedAt:   formatTime(job.CreatedAt),
		UpdatedAt:   formatTime(job.UpdatedAt),
	}
}

func toCronJobRunDTOs(runs []cronjobsdomain.CronJobRun) []CronJobRunDTO {
	dtos := make([]CronJobRunDTO, 0, len(runs))
	for _, run := range runs {
		dtos = append(dtos, toCronJobRunDTO(run))
	}
	return dtos
}

func toCronJobRunDTO(run cronjobsdomain.CronJobRun) CronJobRunDTO {
	var durationMS *int64
	if run.FinishedAt != nil {
		value := run.FinishedAt.Sub(run.StartedAt).Milliseconds()
		durationMS = &value
	}
	return CronJobRunDTO{
		ID:         run.ID,
		CronJobID:  run.CronJobID,
		Status:     run.Status,
		StartedAt:  formatTime(run.StartedAt),
		FinishedAt: formatOptionalTime(run.FinishedAt),
		Log:        run.Log,
		Error:      run.Error,
		DurationMS: durationMS,
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}
