package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	backupsdomain "github.com/duclamdev/application-chat/backend/internal/modules/backups/domain"
	cronjobsapp "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/application"
	platformstorage "github.com/duclamdev/application-chat/backend/internal/platform/storage"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const (
	PermissionManageBackup = "backup.manage"
	defaultBackupLimit     = 20
	backupLockTTL          = 15 * time.Minute
)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	ListJobs(ctx context.Context, workspaceID string, limit int) ([]backupsdomain.Job, error)
	GetJob(ctx context.Context, jobID string) (backupsdomain.Job, error)
	CreateJob(ctx context.Context, params SaveJobParams) (backupsdomain.Job, error)
	ListRuns(ctx context.Context, jobID string, limit int) ([]backupsdomain.Run, error)
	ClaimDue(ctx context.Context, limit int, nodeID string, lockTTL time.Duration) ([]backupsdomain.Job, error)
	StartRun(ctx context.Context, jobID string, backupType string) (backupsdomain.Run, error)
	FinishRun(ctx context.Context, params FinishRunParams) error
}

type Options struct {
	DatabaseURL     string
	PGDumpPath      string
	Timeout         time.Duration
	StorageProvider string
}

type Service struct {
	repo    Repository
	store   platformstorage.Store
	checker PermissionChecker
	options Options
	now     func() time.Time
}

type CreateJobInput struct {
	ActorUserID string
	WorkspaceID string
	Name        string
	Target      string
	BackupType  string
	Schedule    string
	Status      string
	Config      json.RawMessage
}

type SaveJobParams struct {
	WorkspaceID string
	Name        string
	Target      string
	BackupType  string
	Schedule    string
	Status      string
	Config      []byte
	NextRunAt   *time.Time
}

type FinishRunParams struct {
	RunID     string
	JobID     string
	Status    string
	ObjectKey string
	ByteSize  *int64
	Checksum  string
	Error     string
	NextRunAt *time.Time
}

type JobDTO struct {
	ID          string          `json:"id"`
	WorkspaceID *string         `json:"workspace_id,omitempty"`
	Name        string          `json:"name"`
	Target      string          `json:"target"`
	BackupType  string          `json:"backup_type"`
	Schedule    *string         `json:"schedule,omitempty"`
	Status      string          `json:"status"`
	Config      json.RawMessage `json:"config"`
	LastRunAt   *string         `json:"last_run_at,omitempty"`
	NextRunAt   *string         `json:"next_run_at,omitempty"`
	LockedAt    *string         `json:"locked_at,omitempty"`
	LockedBy    *string         `json:"locked_by,omitempty"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

type RunDTO struct {
	ID          string  `json:"id"`
	BackupJobID *string `json:"backup_job_id,omitempty"`
	Status      string  `json:"status"`
	BackupType  string  `json:"backup_type"`
	ObjectKey   *string `json:"object_key,omitempty"`
	ByteSize    *int64  `json:"byte_size,omitempty"`
	Checksum    *string `json:"checksum_sha256,omitempty"`
	StartedAt   string  `json:"started_at"`
	FinishedAt  *string `json:"finished_at,omitempty"`
	Error       *string `json:"error,omitempty"`
	DurationMS  *int64  `json:"duration_ms,omitempty"`
}

func NewService(repo Repository, store platformstorage.Store, checker PermissionChecker, options Options) *Service {
	if strings.TrimSpace(options.PGDumpPath) == "" {
		options.PGDumpPath = "pg_dump"
	}
	if options.Timeout <= 0 {
		options.Timeout = 10 * time.Minute
	}
	return &Service{repo: repo, store: store, checker: checker, options: options, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) ListJobs(ctx context.Context, actorUserID string, workspaceID string, limit int) ([]JobDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	jobs, err := s.repo.ListJobs(ctx, strings.TrimSpace(workspaceID), normalizeLimit(limit))
	if err != nil {
		return nil, err
	}
	return toJobDTOs(jobs), nil
}

func (s *Service) CreateJob(ctx context.Context, input CreateJobInput) (JobDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return JobDTO{}, err
	}
	params, err := s.normalizeJob(input)
	if err != nil {
		return JobDTO{}, err
	}
	job, err := s.repo.CreateJob(ctx, params)
	if err != nil {
		return JobDTO{}, err
	}
	return toJobDTO(job), nil
}

func (s *Service) ListRuns(ctx context.Context, actorUserID string, workspaceID string, jobID string, limit int) ([]RunDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	job, err := s.repo.GetJob(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return nil, mapBackupError(err)
	}
	if err := ensureJobWorkspace(job, workspaceID); err != nil {
		return nil, err
	}
	runs, err := s.repo.ListRuns(ctx, strings.TrimSpace(jobID), normalizeLimit(limit))
	if err != nil {
		return nil, err
	}
	return toRunDTOs(runs), nil
}

func (s *Service) RunNow(ctx context.Context, actorUserID string, workspaceID string, jobID string) (RunDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID); err != nil {
		return RunDTO{}, err
	}
	job, err := s.repo.GetJob(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return RunDTO{}, mapBackupError(err)
	}
	if err := ensureJobWorkspace(job, workspaceID); err != nil {
		return RunDTO{}, err
	}
	run, err := s.runJob(ctx, job)
	if err != nil {
		return RunDTO{}, err
	}
	return toRunDTO(run), nil
}

func (s *Service) ProcessDue(ctx context.Context, limit int, nodeID string) (int, error) {
	if limit <= 0 {
		limit = defaultBackupLimit
	}
	if strings.TrimSpace(nodeID) == "" {
		nodeID = "worker"
	}
	jobs, err := s.repo.ClaimDue(ctx, limit, nodeID, backupLockTTL)
	if err != nil {
		return 0, err
	}
	for _, job := range jobs {
		if _, err := s.runJob(ctx, job); err != nil {
			return 0, err
		}
	}
	return len(jobs), nil
}

func (s *Service) runJob(ctx context.Context, job backupsdomain.Job) (backupsdomain.Run, error) {
	run, err := s.repo.StartRun(ctx, job.ID, job.BackupType)
	if err != nil {
		return backupsdomain.Run{}, err
	}

	objectKey, size, checksum, runErr := s.executeBackup(ctx, job)
	nextRun := s.nextRun(job)
	status := "success"
	errorText := ""
	if runErr != nil {
		status = "failed"
		errorText = runErr.Error()
	}
	if err := s.repo.FinishRun(ctx, FinishRunParams{
		RunID:     run.ID,
		JobID:     job.ID,
		Status:    status,
		ObjectKey: objectKey,
		ByteSize:  size,
		Checksum:  checksum,
		Error:     errorText,
		NextRunAt: nextRun,
	}); err != nil {
		return backupsdomain.Run{}, err
	}
	finishedAt := s.now()
	run.Status = status
	run.FinishedAt = &finishedAt
	if objectKey != "" {
		run.ObjectKey = &objectKey
	}
	run.ByteSize = size
	if checksum != "" {
		run.Checksum = &checksum
	}
	if errorText != "" {
		run.Error = &errorText
	}
	return run, nil
}

func (s *Service) executeBackup(ctx context.Context, job backupsdomain.Job) (string, *int64, string, error) {
	if job.BackupType != "database" {
		return "", nil, "", fmt.Errorf("MVP hiện chỉ hỗ trợ backup database")
	}
	if s.store == nil {
		return "", nil, "", fmt.Errorf("storage chưa được cấu hình để lưu backup")
	}
	if strings.TrimSpace(s.options.DatabaseURL) == "" {
		return "", nil, "", fmt.Errorf("DATABASE_URL chưa được cấu hình để chạy pg_dump")
	}
	runCtx, cancel := context.WithTimeout(ctx, s.options.Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, s.options.PGDumpPath, "--format=custom", "--no-owner", "--no-privileges", s.options.DatabaseURL)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return "", nil, "", fmt.Errorf("pg_dump chạy quá thời gian cho phép")
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", nil, "", fmt.Errorf("pg_dump thất bại: %s", message)
	}

	sum := sha256.Sum256(stdout.Bytes())
	checksum := hex.EncodeToString(sum[:])
	size := int64(stdout.Len())
	objectKey := backupObjectKey(job, s.now())
	if _, err := s.store.Put(ctx, platformstorage.PutObjectInput{
		Key:         objectKey,
		Body:        bytes.NewReader(stdout.Bytes()),
		ContentType: "application/octet-stream",
		Size:        size,
	}); err != nil {
		return "", nil, "", err
	}
	return objectKey, &size, checksum, nil
}

func (s *Service) normalizeJob(input CreateJobInput) (SaveJobParams, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 120 {
		return SaveJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên backup job phải dài từ 1 đến 120 ký tự.")
	}
	target := strings.ToLower(strings.TrimSpace(input.Target))
	if target == "" {
		target = normalizeTarget(s.options.StorageProvider)
	}
	if !validTarget(target) {
		return SaveJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Target backup không hợp lệ.")
	}
	backupType := strings.ToLower(strings.TrimSpace(input.BackupType))
	if backupType == "" {
		backupType = "database"
	}
	if backupType != "database" {
		return SaveJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", "MVP hiện chỉ hỗ trợ backup database.")
	}
	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status == "" {
		status = "active"
	}
	if !validStatus(status) {
		return SaveJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Trạng thái backup job không hợp lệ.")
	}
	config := json.RawMessage(input.Config)
	if len(config) == 0 || strings.TrimSpace(string(config)) == "" || strings.TrimSpace(string(config)) == "null" {
		config = json.RawMessage(`{}`)
	}
	if !json.Valid(config) {
		return SaveJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Config backup job không phải JSON hợp lệ.")
	}
	schedule := strings.TrimSpace(input.Schedule)
	var nextRunAt *time.Time
	if schedule != "" {
		next, err := cronjobsapp.NextRun(schedule, s.now())
		if err != nil {
			return SaveJobParams{}, apperrors.BadRequest("VALIDATION_ERROR", err.Error()+".")
		}
		if status == "active" {
			nextRunAt = &next
		}
	}
	return SaveJobParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		Name:        name,
		Target:      target,
		BackupType:  backupType,
		Schedule:    schedule,
		Status:      status,
		Config:      []byte(config),
		NextRunAt:   nextRunAt,
	}, nil
}

func (s *Service) nextRun(job backupsdomain.Job) *time.Time {
	if job.Status != "active" || job.Schedule == nil || strings.TrimSpace(*job.Schedule) == "" {
		return nil
	}
	next, err := cronjobsapp.NextRun(*job.Schedule, s.now())
	if err != nil {
		return nil
	}
	return &next
}

func (s *Service) ensurePermission(ctx context.Context, userID string, workspaceID string) error {
	if s.checker == nil {
		return nil
	}
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), PermissionManageBackup)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền quản lý backup.")
	}
	return nil
}

func backupObjectKey(job backupsdomain.Job, now time.Time) string {
	workspace := "system"
	if job.WorkspaceID != nil && strings.TrimSpace(*job.WorkspaceID) != "" {
		workspace = strings.TrimSpace(*job.WorkspaceID)
	}
	return "backups/database/" + workspace + "/" + now.UTC().Format("2006/01/02/150405") + "-" + job.ID + ".dump"
}

func normalizeTarget(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "minio", "s3":
		return provider
	default:
		return "local"
	}
}

func validTarget(target string) bool {
	switch target {
	case "local", "minio", "s3":
		return true
	default:
		return false
	}
}

func validStatus(status string) bool {
	switch status {
	case "active", "paused", "disabled":
		return true
	default:
		return false
	}
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 100 {
		return 50
	}
	return limit
}

func mapBackupError(err error) error {
	if err == nil {
		return nil
	}
	if err == backupsdomain.ErrBackupJobNotFound {
		return apperrors.NotFound("BACKUP_JOB_NOT_FOUND", "Không tìm thấy backup job.")
	}
	return err
}

func ensureJobWorkspace(job backupsdomain.Job, workspaceID string) error {
	if job.WorkspaceID == nil {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(*job.WorkspaceID), strings.TrimSpace(workspaceID)) {
		return nil
	}
	return apperrors.NotFound("BACKUP_JOB_NOT_FOUND", "Không tìm thấy backup job.")
}

func toJobDTOs(jobs []backupsdomain.Job) []JobDTO {
	dtos := make([]JobDTO, 0, len(jobs))
	for _, job := range jobs {
		dtos = append(dtos, toJobDTO(job))
	}
	return dtos
}

func toJobDTO(job backupsdomain.Job) JobDTO {
	config := json.RawMessage(job.Config)
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	return JobDTO{
		ID:          job.ID,
		WorkspaceID: job.WorkspaceID,
		Name:        job.Name,
		Target:      job.Target,
		BackupType:  job.BackupType,
		Schedule:    job.Schedule,
		Status:      job.Status,
		Config:      config,
		LastRunAt:   formatOptionalTime(job.LastRunAt),
		NextRunAt:   formatOptionalTime(job.NextRunAt),
		LockedAt:    formatOptionalTime(job.LockedAt),
		LockedBy:    job.LockedBy,
		CreatedAt:   formatTime(job.CreatedAt),
		UpdatedAt:   formatTime(job.UpdatedAt),
	}
}

func toRunDTOs(runs []backupsdomain.Run) []RunDTO {
	dtos := make([]RunDTO, 0, len(runs))
	for _, run := range runs {
		dtos = append(dtos, toRunDTO(run))
	}
	return dtos
}

func toRunDTO(run backupsdomain.Run) RunDTO {
	var durationMS *int64
	if run.FinishedAt != nil {
		value := run.FinishedAt.Sub(run.StartedAt).Milliseconds()
		durationMS = &value
	}
	return RunDTO{
		ID:          run.ID,
		BackupJobID: run.BackupJobID,
		Status:      run.Status,
		BackupType:  run.BackupType,
		ObjectKey:   run.ObjectKey,
		ByteSize:    run.ByteSize,
		Checksum:    run.Checksum,
		StartedAt:   formatTime(run.StartedAt),
		FinishedAt:  formatOptionalTime(run.FinishedAt),
		Error:       run.Error,
		DurationMS:  durationMS,
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
