package application

import (
	"context"
	"strings"
	"time"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const PermissionViewAdmin = "admin.view"

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	DashboardStats(ctx context.Context, workspaceID string) (DashboardStats, error)
	ListChannels(ctx context.Context, workspaceID string) ([]ChannelOverview, error)
	ListRecentMessages(ctx context.Context, workspaceID string, limit int) ([]MessageOverview, error)
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
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.repo.ListRecentMessages(ctx, strings.TrimSpace(workspaceID), limit)
}

func (s *Service) EnsureAdminPermission(ctx context.Context, userID string, workspaceID string) error {
	if s.checker == nil {
		return nil
	}
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), PermissionViewAdmin)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền xem trang quản trị.")
	}
	return nil
}
