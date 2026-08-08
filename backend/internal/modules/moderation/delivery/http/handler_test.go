package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	moderationapp "github.com/duclamdev/application-chat/backend/internal/modules/moderation/application"
	moderationdomain "github.com/duclamdev/application-chat/backend/internal/modules/moderation/domain"
	"github.com/duclamdev/application-chat/backend/internal/shared/constants"
	"github.com/gin-gonic/gin"
)

const (
	handlerWorkspaceID = "11111111-1111-1111-1111-111111111111"
	handlerReporterID  = "22222222-2222-2222-2222-222222222222"
	handlerTargetID    = "33333333-3333-3333-3333-333333333333"
	handlerReportID    = "44444444-4444-4444-4444-444444444444"
)

type handlerPermissionChecker struct{}

func (handlerPermissionChecker) HasWorkspacePermission(context.Context, string, string, string) (bool, error) {
	return true, nil
}

type handlerRepository struct{}

func (handlerRepository) IsActiveWorkspaceMember(context.Context, string, string) (bool, error) {
	return true, nil
}

func (handlerRepository) ResolveReportTarget(context.Context, string, string, string, string) (moderationdomain.ReportTarget, error) {
	return moderationdomain.ReportTarget{UserID: handlerTargetID}, nil
}

func (handlerRepository) CountReportsSince(context.Context, string, time.Time) (int, error) {
	return 0, nil
}

func (handlerRepository) CreateReport(_ context.Context, params moderationapp.CreateReportParams) (moderationdomain.Report, error) {
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	reporterID, targetID := params.ReporterUserID, params.TargetUserID
	return moderationdomain.Report{
		ID: handlerReportID, WorkspaceID: params.WorkspaceID, ReporterUserID: &reporterID,
		TargetType: params.TargetType, TargetID: params.TargetID, TargetUserID: &targetID,
		Reason: params.Reason, Status: "pending", CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (handlerRepository) ListReports(context.Context, moderationapp.ListReportsParams) ([]moderationdomain.Report, error) {
	return []moderationdomain.Report{}, nil
}

func (handlerRepository) UpdateReport(context.Context, moderationapp.UpdateReportParams) (moderationdomain.Report, error) {
	return moderationdomain.Report{}, nil
}

func (handlerRepository) CreateBlock(context.Context, moderationapp.CreateBlockParams) (moderationdomain.UserBlock, error) {
	return moderationdomain.UserBlock{}, nil
}

func (handlerRepository) DeleteBlock(context.Context, string, string, string) error { return nil }

func (handlerRepository) ListBlocks(context.Context, string, string) ([]moderationdomain.UserBlock, error) {
	return []moderationdomain.UserBlock{}, nil
}

func (handlerRepository) IsInteractionBlocked(context.Context, string, string, string) (bool, error) {
	return false, nil
}

func (handlerRepository) IsDirectChannelBlocked(context.Context, string, string, string) (bool, error) {
	return false, nil
}

func (handlerRepository) RecordAudit(context.Context, moderationapp.AuditEvent) error { return nil }

func TestCreateReportRouteUsesAuthenticatedActorAndWorkspace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := moderationapp.NewService(handlerRepository{}, handlerPermissionChecker{})
	handler := NewHandler(service)
	router := gin.New()
	handler.RegisterRoutes(router.Group("/api/v1"), func(c *gin.Context) {
		c.Set(constants.ContextUserID, handlerReporterID)
		c.Next()
	})

	body := `{"target_type":"user","target_id":"` + handlerTargetID + `","reason":"spam","details":"repeat abuse"}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/workspaces/"+handlerWorkspaceID+"/moderation/reports",
		bytes.NewBufferString(body),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	for _, expected := range []string{handlerReportID, `"target_type":"user"`, `"reason":"spam"`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("response body missing %q: %s", expected, recorder.Body.String())
		}
	}
}
