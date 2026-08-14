package application

import (
	"context"
	"strings"
	"time"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const (
	PermissionViewAdmin          = "admin.view"
	PermissionManageMessage      = "message.manage"
	PermissionManageNotification = "notification.manage"
)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	DashboardStats(ctx context.Context, workspaceID string) (DashboardStats, error)
	ListChannels(ctx context.Context, workspaceID string) ([]ChannelOverview, error)
	ListRecentMessages(ctx context.Context, workspaceID string, limit int) ([]MessageOverview, error)
	PushQueue(ctx context.Context, workspaceID string, deadLetterLimit int) (PushQueueOverview, error)
	ReplayDeadPushJob(ctx context.Context, workspaceID string, jobID string, actorUserID string) (PushReplayResult, error)
}

type Service struct {
	repo    Repository
	checker PermissionChecker
	now     func() time.Time
}

type DashboardStats struct {
	WorkspaceID      string
	ActiveMembers    int64
	Channels         int64
	Messages         int64
	Files            int64
	Bots             int64
	IncomingWebhooks int64
	OutgoingWebhooks int64
	AuditLogs        int64
	BackupJobs       int64
	StorageBytes     int64
	Activity         []ActivityPoint
	ChannelRanks     []ChannelRank
}

type ActivityPoint struct {
	Date     string `json:"date"`
	Messages int64  `json:"messages"`
	Users    int64  `json:"users"`
}

type ChannelRank struct {
	ChannelID     string `json:"channel_id"`
	Name          string `json:"name"`
	MessagesCount int64  `json:"messages_count"`
}

type ChannelOverview struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Slug               string `json:"slug,omitempty"`
	Type               string `json:"type"`
	Status             string `json:"status"`
	MemberCount        int64  `json:"member_count"`
	MessageCount       int64  `json:"message_count"`
	PrivateSessionMode bool   `json:"private_session_mode"`
	UpdatedAt          string `json:"updated_at"`
}

type MessageOverview struct {
	ID          string `json:"id"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	SenderName  string `json:"sender_name"`
	Kind        string `json:"kind"`
	Body        string `json:"body"`
	CreatedAt   string `json:"created_at"`
}

type PushQueueOverview struct {
	Pending            int64
	Processing         int64
	Failed             int64
	Sent24Hours        int64
	Skipped24Hours     int64
	Dead24Hours        int64
	OldestQueuedAt     *time.Time
	ProviderDeliveries []PushProviderDelivery
	HourlyActivity     []PushHourlyActivity
	DeadLetters        []PushDeadLetter
}

type PushProviderDelivery struct {
	Provider string `json:"provider"`
	Count    int64  `json:"count"`
}

type PushHourlyActivity struct {
	Hour string `json:"hour"`
	Sent int64  `json:"sent"`
	Dead int64  `json:"dead"`
}

type PushDeadLetter struct {
	ID           string    `json:"id"`
	AttemptCount int       `json:"attempt_count"`
	Error        string    `json:"error"`
	CreatedAt    time.Time `json:"-"`
	UpdatedAt    time.Time `json:"-"`
}

type PushReplayResult struct {
	JobID   string
	Created bool
	Found   bool
}

type PushDeadLetterDTO struct {
	ID           string `json:"id"`
	AttemptCount int    `json:"attempt_count"`
	Error        string `json:"error"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type PushQueueDTO struct {
	WorkspaceID            string                 `json:"workspace_id"`
	QueueDepth             int64                  `json:"queue_depth"`
	Pending                int64                  `json:"pending"`
	Processing             int64                  `json:"processing"`
	Failed                 int64                  `json:"failed"`
	Sent24Hours            int64                  `json:"sent_24h"`
	Skipped24Hours         int64                  `json:"skipped_24h"`
	Dead24Hours            int64                  `json:"dead_24h"`
	DeliveryRatePercent24H *float64               `json:"delivery_rate_percent_24h"`
	OldestQueuedAt         *string                `json:"oldest_queued_at,omitempty"`
	OldestQueueAgeSeconds  *int64                 `json:"oldest_queue_age_seconds,omitempty"`
	ProviderDeliveries24H  []PushProviderDelivery `json:"provider_deliveries_24h"`
	HourlyActivity         []PushHourlyActivity   `json:"hourly_activity"`
	DeadLetters            []PushDeadLetterDTO    `json:"dead_letters"`
	GeneratedAt            string                 `json:"generated_at"`
}

type PushReplayDTO struct {
	OriginalJobID string `json:"original_job_id"`
	ReplayJobID   string `json:"replay_job_id"`
	Created       bool   `json:"created"`
}

type DashboardStatsDTO struct {
	WorkspaceID      string          `json:"workspace_id"`
	ActiveMembers    int64           `json:"active_members"`
	Channels         int64           `json:"channels"`
	Messages         int64           `json:"messages"`
	Files            int64           `json:"files"`
	Bots             int64           `json:"bots"`
	IncomingWebhooks int64           `json:"incoming_webhooks"`
	OutgoingWebhooks int64           `json:"outgoing_webhooks"`
	AuditLogs        int64           `json:"audit_logs"`
	BackupJobs       int64           `json:"backup_jobs"`
	StorageBytes     int64           `json:"storage_bytes"`
	Activity         []ActivityPoint `json:"activity"`
	ChannelRanks     []ChannelRank   `json:"channel_ranks"`
	GeneratedAt      string          `json:"generated_at"`
}

func NewService(repo Repository, checker PermissionChecker) *Service {
	return &Service{repo: repo, checker: checker, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Dashboard(ctx context.Context, actorUserID string, workspaceID string) (DashboardStatsDTO, error) {
	if err := s.EnsureAdminPermission(ctx, actorUserID, workspaceID); err != nil {
		return DashboardStatsDTO{}, err
	}
	stats, err := s.repo.DashboardStats(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return DashboardStatsDTO{}, err
	}
	return DashboardStatsDTO{
		WorkspaceID:      stats.WorkspaceID,
		ActiveMembers:    stats.ActiveMembers,
		Channels:         stats.Channels,
		Messages:         stats.Messages,
		Files:            stats.Files,
		Bots:             stats.Bots,
		IncomingWebhooks: stats.IncomingWebhooks,
		OutgoingWebhooks: stats.OutgoingWebhooks,
		AuditLogs:        stats.AuditLogs,
		BackupJobs:       stats.BackupJobs,
		StorageBytes:     stats.StorageBytes,
		Activity:         stats.Activity,
		ChannelRanks:     stats.ChannelRanks,
		GeneratedAt:      s.now().Format(time.RFC3339),
	}, nil
}

func (s *Service) Channels(ctx context.Context, actorUserID string, workspaceID string) ([]ChannelOverview, error) {
	if err := s.EnsureAdminPermission(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	return s.repo.ListChannels(ctx, strings.TrimSpace(workspaceID))
}

func (s *Service) RecentMessages(ctx context.Context, actorUserID string, workspaceID string, limit int) ([]MessageOverview, error) {
	if err := s.EnsureAdminPermission(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	allowed, err := s.checkPermission(ctx, actorUserID, workspaceID, PermissionManageMessage)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, apperrors.Forbidden("Bạn không có quyền kiểm duyệt tin nhắn.")
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.repo.ListRecentMessages(ctx, strings.TrimSpace(workspaceID), limit)
}

func (s *Service) PushQueue(ctx context.Context, actorUserID string, workspaceID string, deadLetterLimit int) (PushQueueDTO, error) {
	if err := s.EnsureAdminPermission(ctx, actorUserID, workspaceID); err != nil {
		return PushQueueDTO{}, err
	}
	if deadLetterLimit <= 0 || deadLetterLimit > 100 {
		deadLetterLimit = 50
	}
	workspaceID = strings.TrimSpace(workspaceID)
	overview, err := s.repo.PushQueue(ctx, workspaceID, deadLetterLimit)
	if err != nil {
		return PushQueueDTO{}, err
	}
	now := s.now()
	completed := overview.Sent24Hours + overview.Dead24Hours
	var deliveryRate *float64
	if completed > 0 {
		value := float64(overview.Sent24Hours) / float64(completed) * 100
		deliveryRate = &value
	}
	var oldestQueuedAt *string
	var oldestQueueAgeSeconds *int64
	if overview.OldestQueuedAt != nil {
		formatted := overview.OldestQueuedAt.UTC().Format(time.RFC3339)
		oldestQueuedAt = &formatted
		age := int64(now.Sub(overview.OldestQueuedAt.UTC()).Seconds())
		if age < 0 {
			age = 0
		}
		oldestQueueAgeSeconds = &age
	}
	deadLetters := make([]PushDeadLetterDTO, 0, len(overview.DeadLetters))
	for _, item := range overview.DeadLetters {
		deadLetters = append(deadLetters, PushDeadLetterDTO{
			ID: item.ID, AttemptCount: item.AttemptCount, Error: item.Error,
			CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	providerDeliveries := make([]PushProviderDelivery, 0, len(overview.ProviderDeliveries))
	providerDeliveries = append(providerDeliveries, overview.ProviderDeliveries...)
	hourlyActivity := make([]PushHourlyActivity, 0, len(overview.HourlyActivity))
	hourlyActivity = append(hourlyActivity, overview.HourlyActivity...)
	return PushQueueDTO{
		WorkspaceID:            workspaceID,
		QueueDepth:             overview.Pending + overview.Processing + overview.Failed,
		Pending:                overview.Pending,
		Processing:             overview.Processing,
		Failed:                 overview.Failed,
		Sent24Hours:            overview.Sent24Hours,
		Skipped24Hours:         overview.Skipped24Hours,
		Dead24Hours:            overview.Dead24Hours,
		DeliveryRatePercent24H: deliveryRate,
		OldestQueuedAt:         oldestQueuedAt,
		OldestQueueAgeSeconds:  oldestQueueAgeSeconds,
		ProviderDeliveries24H:  providerDeliveries,
		HourlyActivity:         hourlyActivity,
		DeadLetters:            deadLetters,
		GeneratedAt:            now.Format(time.RFC3339),
	}, nil
}

func (s *Service) ReplayDeadPushJob(ctx context.Context, actorUserID string, workspaceID string, jobID string) (PushReplayDTO, error) {
	allowed, err := s.checkPermission(ctx, actorUserID, workspaceID, PermissionManageNotification)
	if err != nil {
		return PushReplayDTO{}, err
	}
	if !allowed {
		return PushReplayDTO{}, apperrors.Forbidden("Bạn không có quyền xử lý dead-letter notification.")
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return PushReplayDTO{}, apperrors.BadRequest("PUSH_JOB_ID_REQUIRED", "Thiếu push job cần chạy lại.")
	}
	result, err := s.repo.ReplayDeadPushJob(ctx, strings.TrimSpace(workspaceID), jobID, strings.TrimSpace(actorUserID))
	if err != nil {
		return PushReplayDTO{}, err
	}
	if !result.Found {
		return PushReplayDTO{}, apperrors.NotFound("PUSH_DEAD_LETTER_NOT_FOUND", "Không tìm thấy push dead-letter trong workspace này.")
	}
	return PushReplayDTO{OriginalJobID: jobID, ReplayJobID: result.JobID, Created: result.Created}, nil
}

func (s *Service) EnsureAdminPermission(ctx context.Context, userID string, workspaceID string) error {
	allowed, err := s.checkPermission(ctx, userID, workspaceID, PermissionViewAdmin)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền xem trang quản trị.")
	}
	return nil
}

func (s *Service) checkPermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error) {
	if s.checker == nil {
		return false, apperrors.ServiceUnavailable(
			"RBAC_CHECKER_UNAVAILABLE",
			"Dịch vụ phân quyền chưa sẵn sàng.",
		)
	}
	return s.checker.HasWorkspacePermission(
		ctx,
		strings.TrimSpace(userID),
		strings.TrimSpace(workspaceID),
		permissionCode,
	)
}
