package application

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	moderationdomain "github.com/duclamdev/application-chat/backend/internal/modules/moderation/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const (
	testWorkspaceID = "11111111-1111-1111-1111-111111111111"
	testReporterID  = "22222222-2222-2222-2222-222222222222"
	testTargetID    = "33333333-3333-3333-3333-333333333333"
	testMessageID   = "44444444-4444-4444-4444-444444444444"
	testReportID    = "55555555-5555-5555-5555-555555555555"
)

type fakePermissionChecker struct {
	allowed bool
}

func (f fakePermissionChecker) HasWorkspacePermission(context.Context, string, string, string) (bool, error) {
	return f.allowed, nil
}

type fakeRepository struct {
	members          map[string]bool
	targetUserID     string
	reportCount      int
	createReportErr  error
	createdReport    CreateReportParams
	createdBlock     CreateBlockParams
	deletedBlockID   string
	reports          []moderationdomain.Report
	blocks           []moderationdomain.UserBlock
	senderlessTarget bool
}

func (f *fakeRepository) IsActiveWorkspaceMember(_ context.Context, _ string, userID string) (bool, error) {
	return f.members[userID], nil
}

func (f *fakeRepository) ResolveReportTarget(context.Context, string, string, string, string) (moderationdomain.ReportTarget, error) {
	if f.targetUserID == "" && !f.senderlessTarget {
		return moderationdomain.ReportTarget{}, moderationdomain.ErrReportTarget
	}
	return moderationdomain.ReportTarget{UserID: f.targetUserID, Snapshot: []byte(`{"kind":"text","body_excerpt":"evidence"}`)}, nil
}

func (f *fakeRepository) CreateReport(_ context.Context, params CreateReportParams) (moderationdomain.Report, error) {
	if f.reportCount >= params.MaxReports {
		return moderationdomain.Report{}, moderationdomain.ErrReportRateLimit
	}
	if f.createReportErr != nil {
		return moderationdomain.Report{}, f.createReportErr
	}
	f.createdReport = params
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	reporterID := params.ReporterUserID
	var targetUserID *string
	if params.TargetUserID != "" {
		targetUserID = &params.TargetUserID
	}
	return moderationdomain.Report{
		ID: testReportID, WorkspaceID: params.WorkspaceID, ReporterUserID: &reporterID,
		TargetType: params.TargetType, TargetID: params.TargetID, TargetUserID: targetUserID,
		TargetSnapshot: params.TargetSnapshot,
		Reason:         params.Reason, Status: "pending", CreatedAt: now, UpdatedAt: now,
	}, nil
}

func TestCreateReportAllowsVisibleSenderlessMessage(t *testing.T) {
	repo := &fakeRepository{
		members:          map[string]bool{testReporterID: true},
		senderlessTarget: true,
	}
	service := NewService(repo, fakePermissionChecker{allowed: true})
	report, err := service.CreateReport(context.Background(), CreateReportInput{
		ActorUserID: testReporterID,
		WorkspaceID: testWorkspaceID,
		TargetType:  "message",
		TargetID:    testMessageID,
		Reason:      "spam",
	})
	if err != nil {
		t.Fatalf("CreateReport() senderless message error = %v", err)
	}
	if repo.createdReport.TargetUserID != "" || report.TargetUserID != nil {
		t.Fatalf("senderless target attribution = params:%q dto:%v", repo.createdReport.TargetUserID, report.TargetUserID)
	}
}

func (f *fakeRepository) ListReports(context.Context, ListReportsParams) ([]moderationdomain.Report, error) {
	return f.reports, nil
}

func (f *fakeRepository) UpdateReport(_ context.Context, params UpdateReportParams) (moderationdomain.Report, error) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	return moderationdomain.Report{
		ID: params.ReportID, WorkspaceID: params.WorkspaceID, TargetType: "user", TargetID: testTargetID,
		Reason: "spam", Status: params.Status, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (f *fakeRepository) CreateBlock(_ context.Context, params CreateBlockParams) (moderationdomain.UserBlock, error) {
	f.createdBlock = params
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	return moderationdomain.UserBlock{
		ID: testReportID, WorkspaceID: params.WorkspaceID, BlockerUserID: params.BlockerUserID,
		BlockedUserID: params.BlockedUserID, BlockedUsername: "blocked", BlockedDisplayName: "Blocked User",
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (f *fakeRepository) DeleteBlock(_ context.Context, _ string, _ string, blockedUserID string) error {
	f.deletedBlockID = blockedUserID
	return nil
}

func (f *fakeRepository) ListBlocks(context.Context, string, string) ([]moderationdomain.UserBlock, error) {
	return f.blocks, nil
}

func (f *fakeRepository) IsInteractionBlocked(context.Context, string, string, string) (bool, error) {
	return false, nil
}

func (f *fakeRepository) IsDirectChannelBlocked(context.Context, string, string, string) (bool, error) {
	return false, nil
}

func (f *fakeRepository) RecordAudit(context.Context, AuditEvent) error { return nil }

func TestCreateReportValidatesAndPersistsNormalizedInput(t *testing.T) {
	repo := &fakeRepository{
		members:      map[string]bool{testReporterID: true},
		targetUserID: testTargetID,
	}
	service := NewService(repo, fakePermissionChecker{allowed: true})
	service.now = func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) }

	report, err := service.CreateReport(context.Background(), CreateReportInput{
		ActorUserID: testReporterID, WorkspaceID: testWorkspaceID,
		TargetType: " MESSAGE ", TargetID: testMessageID, Reason: " HARASSMENT ", Details: " details ",
	})
	if err != nil {
		t.Fatalf("CreateReport() error = %v", err)
	}
	if report.ID != testReportID || repo.createdReport.TargetType != "message" || repo.createdReport.Reason != "harassment" || repo.createdReport.Details != "details" {
		t.Fatalf("report/params not normalized: report=%#v params=%#v", report, repo.createdReport)
	}
	if len(report.TargetSnapshot) != 0 {
		t.Fatalf("reporter receipt exposed moderation snapshot: %s", report.TargetSnapshot)
	}
	if string(repo.createdReport.TargetSnapshot) != `{"kind":"text","body_excerpt":"evidence"}` {
		t.Fatalf("persisted target snapshot = %s", repo.createdReport.TargetSnapshot)
	}
}

func TestCreateReportRejectsSelfDuplicateAndRateAbuse(t *testing.T) {
	tests := []struct {
		name       string
		repo       *fakeRepository
		wantCode   string
		wantStatus int
	}{
		{
			name:     "self report",
			repo:     &fakeRepository{members: map[string]bool{testReporterID: true}, targetUserID: testReporterID},
			wantCode: "SELF_REPORT_NOT_ALLOWED", wantStatus: http.StatusBadRequest,
		},
		{
			name:     "duplicate active report",
			repo:     &fakeRepository{members: map[string]bool{testReporterID: true}, targetUserID: testTargetID, createReportErr: moderationdomain.ErrReportDuplicate},
			wantCode: "REPORT_ALREADY_OPEN", wantStatus: http.StatusConflict,
		},
		{
			name:     "daily abuse limit",
			repo:     &fakeRepository{members: map[string]bool{testReporterID: true}, targetUserID: testTargetID, reportCount: MaxReportsPer24Hours},
			wantCode: "REPORT_RATE_LIMITED", wantStatus: http.StatusTooManyRequests,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(tt.repo, fakePermissionChecker{allowed: true})
			_, err := service.CreateReport(context.Background(), CreateReportInput{
				ActorUserID: testReporterID, WorkspaceID: testWorkspaceID,
				TargetType: "user", TargetID: testTargetID, Reason: "spam",
			})
			var appErr *apperrors.AppError
			if !errors.As(err, &appErr) || appErr.Code != tt.wantCode || appErr.Status != tt.wantStatus {
				t.Fatalf("error = %#v, want code=%s status=%d", err, tt.wantCode, tt.wantStatus)
			}
		})
	}
}

func TestModerationQueueRequiresPermissionAndResolution(t *testing.T) {
	repo := &fakeRepository{members: map[string]bool{testReporterID: true}}
	service := NewService(repo, fakePermissionChecker{allowed: false})
	if _, err := service.ListReports(context.Background(), ListReportsInput{
		ActorUserID: testReporterID, WorkspaceID: testWorkspaceID,
	}); err == nil {
		t.Fatal("ListReports() allowed a non-moderator")
	}

	service = NewService(repo, fakePermissionChecker{allowed: true})
	_, err := service.UpdateReport(context.Background(), UpdateReportInput{
		ActorUserID: testReporterID, WorkspaceID: testWorkspaceID,
		ReportID: testReportID, Status: "resolved",
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "RESOLUTION_REQUIRED" {
		t.Fatalf("UpdateReport() error = %#v, want RESOLUTION_REQUIRED", err)
	}

	inactiveService := NewService(&fakeRepository{members: map[string]bool{}}, fakePermissionChecker{allowed: true})
	if _, err := inactiveService.ListReports(context.Background(), ListReportsInput{
		ActorUserID: testReporterID, WorkspaceID: testWorkspaceID,
	}); err == nil {
		t.Fatal("ListReports() allowed a moderator without active membership")
	}
}

func TestBlockLifecycleRequiresActiveMembers(t *testing.T) {
	repo := &fakeRepository{members: map[string]bool{testReporterID: true, testTargetID: true}}
	service := NewService(repo, fakePermissionChecker{allowed: true})
	block, err := service.CreateBlock(context.Background(), CreateBlockInput{
		ActorUserID: testReporterID, WorkspaceID: testWorkspaceID, BlockedUserID: testTargetID, Reason: " safety ",
	})
	if err != nil {
		t.Fatalf("CreateBlock() error = %v", err)
	}
	if block.BlockedUserID != testTargetID || repo.createdBlock.Reason != "safety" {
		t.Fatalf("block = %#v, params = %#v", block, repo.createdBlock)
	}
	if err := service.DeleteBlock(context.Background(), testReporterID, testWorkspaceID, testTargetID); err != nil {
		t.Fatalf("DeleteBlock() error = %v", err)
	}
	if repo.deletedBlockID != testTargetID {
		t.Fatalf("deleted block user = %q", repo.deletedBlockID)
	}
}
