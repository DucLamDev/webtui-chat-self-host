package application

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type pushTestRepository struct {
	overview       PushQueueOverview
	replay         PushReplayResult
	requestedLimit int
	replayJobID    string
	recentMessages []MessageOverview
	recentCalls    int
}

func (r *pushTestRepository) DashboardStats(context.Context, string) (DashboardStats, error) {
	return DashboardStats{}, nil
}

func (r *pushTestRepository) ListChannels(context.Context, string) ([]ChannelOverview, error) {
	return nil, nil
}

func (r *pushTestRepository) ListRecentMessages(context.Context, string, int) ([]MessageOverview, error) {
	r.recentCalls++
	return r.recentMessages, nil
}

func (r *pushTestRepository) PushQueue(_ context.Context, _ string, limit int) (PushQueueOverview, error) {
	r.requestedLimit = limit
	return r.overview, nil
}

func (r *pushTestRepository) ReplayDeadPushJob(_ context.Context, _ string, jobID string, _ string) (PushReplayResult, error) {
	r.replayJobID = jobID
	return r.replay, nil
}

type pushTestPermissionChecker struct {
	permissions map[string]bool
}

func (c pushTestPermissionChecker) HasWorkspacePermission(_ context.Context, _ string, _ string, code string) (bool, error) {
	return c.permissions[code], nil
}

func TestAdminPermissionCheckFailsClosedWithoutChecker(t *testing.T) {
	repo := &pushTestRepository{}
	service := NewService(repo, nil)

	_, err := service.RecentMessages(context.Background(), "user", "workspace", 100)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Status != 503 || appErr.Code != "RBAC_CHECKER_UNAVAILABLE" {
		t.Fatalf("RecentMessages() error = %#v, want RBAC_CHECKER_UNAVAILABLE (503)", err)
	}
	if repo.recentCalls != 0 {
		t.Fatalf("repository calls = %d, want 0", repo.recentCalls)
	}
}

func TestPushQueueMapsOperationalSummary(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	oldest := now.Add(-90 * time.Second)
	repo := &pushTestRepository{overview: PushQueueOverview{
		Pending: 2, Processing: 1, Failed: 3, Sent24Hours: 90, Skipped24Hours: 7, Dead24Hours: 10,
		OldestQueuedAt:     &oldest,
		ProviderDeliveries: []PushProviderDelivery{{Provider: "fcm", Count: 88}},
		HourlyActivity:     []PushHourlyActivity{{Hour: "2026-08-03T12:00:00Z", Sent: 5}},
		DeadLetters: []PushDeadLetter{{
			ID: "job-dead", AttemptCount: 5, Error: "provider unavailable",
			CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute),
		}},
	}}
	service := NewService(repo, pushTestPermissionChecker{permissions: map[string]bool{PermissionViewAdmin: true}})
	service.now = func() time.Time { return now }

	result, err := service.PushQueue(context.Background(), "user", "workspace", 1000)
	if err != nil {
		t.Fatalf("PushQueue() error = %v", err)
	}
	if result.QueueDepth != 6 || repo.requestedLimit != 50 {
		t.Fatalf("queue depth/limit = %d/%d, want 6/50", result.QueueDepth, repo.requestedLimit)
	}
	if result.DeliveryRatePercent24H == nil || *result.DeliveryRatePercent24H != 90 {
		t.Fatalf("delivery rate = %v, want 90", result.DeliveryRatePercent24H)
	}
	if result.Skipped24Hours != 7 {
		t.Fatalf("skipped jobs = %d, want 7", result.Skipped24Hours)
	}
	if result.OldestQueueAgeSeconds == nil || *result.OldestQueueAgeSeconds != 90 {
		t.Fatalf("oldest queue age = %v, want 90", result.OldestQueueAgeSeconds)
	}
	if len(result.DeadLetters) != 1 || result.DeadLetters[0].ID != "job-dead" {
		t.Fatalf("dead letters = %#v", result.DeadLetters)
	}
}

func TestReplayDeadPushJobRequiresNotificationManage(t *testing.T) {
	repo := &pushTestRepository{replay: PushReplayResult{JobID: "replay", Created: true, Found: true}}
	service := NewService(repo, pushTestPermissionChecker{permissions: map[string]bool{
		PermissionViewAdmin: true,
	}})

	if _, err := service.ReplayDeadPushJob(context.Background(), "user", "workspace", "dead"); err == nil {
		t.Fatal("ReplayDeadPushJob() error = nil, want forbidden")
	}
	if repo.replayJobID != "" {
		t.Fatal("repository must not be called without notification.manage")
	}
}

func TestReplayDeadPushJobReturnsIdempotentReplay(t *testing.T) {
	repo := &pushTestRepository{replay: PushReplayResult{JobID: "replay", Created: false, Found: true}}
	service := NewService(repo, pushTestPermissionChecker{permissions: map[string]bool{
		PermissionManageNotification: true,
	}})

	result, err := service.ReplayDeadPushJob(context.Background(), "user", "workspace", " dead ")
	if err != nil {
		t.Fatalf("ReplayDeadPushJob() error = %v", err)
	}
	if result.ReplayJobID != "replay" || result.Created || result.OriginalJobID != "dead" {
		t.Fatalf("result = %#v", result)
	}
	if repo.replayJobID != "dead" {
		t.Fatalf("repository job id = %q, want dead", repo.replayJobID)
	}
}

func TestRecentMessagesRequiresAdminViewAndMessageManage(t *testing.T) {
	tests := []struct {
		name        string
		permissions map[string]bool
	}{
		{name: "admin view only", permissions: map[string]bool{PermissionViewAdmin: true}},
		{name: "message manage only", permissions: map[string]bool{PermissionManageMessage: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &pushTestRepository{}
			service := NewService(repo, pushTestPermissionChecker{permissions: test.permissions})

			if _, err := service.RecentMessages(context.Background(), "user", "workspace", 100); err == nil {
				t.Fatal("RecentMessages() error = nil, want forbidden")
			}
			if repo.recentCalls != 0 {
				t.Fatalf("repository calls = %d, want 0", repo.recentCalls)
			}
		})
	}
}

func TestRecentMessagesAllowsAdminMessageModerator(t *testing.T) {
	repo := &pushTestRepository{recentMessages: []MessageOverview{{ID: "message-1"}}}
	service := NewService(repo, pushTestPermissionChecker{permissions: map[string]bool{
		PermissionViewAdmin:     true,
		PermissionManageMessage: true,
	}})

	messages, err := service.RecentMessages(context.Background(), "user", "workspace", 100)
	if err != nil {
		t.Fatalf("RecentMessages() error = %v", err)
	}
	if repo.recentCalls != 1 || len(messages) != 1 || messages[0].ID != "message-1" {
		t.Fatalf("messages/calls = %#v/%d", messages, repo.recentCalls)
	}
}
