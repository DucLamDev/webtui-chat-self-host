package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/duclamdev/application-chat/backend/internal/shared/constants"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
	"github.com/gin-gonic/gin"
)

type staticLegalAcceptanceChecker struct {
	accepted    bool
	err         error
	userID      string
	workspaceID string
	zoneID      string
}

func (c *staticLegalAcceptanceChecker) HasCurrentLegalAcceptances(_ context.Context, userID string, workspaceID string, zoneID string) (bool, error) {
	c.userID = userID
	c.workspaceID = workspaceID
	c.zoneID = zoneID
	return c.accepted, c.err
}

func TestRequireCurrentLegalAcceptance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		checker    CurrentLegalAcceptanceChecker
		wantStatus int
		wantCode   string
	}{
		{name: "accepted", checker: &staticLegalAcceptanceChecker{accepted: true}, wantStatus: http.StatusNoContent},
		{name: "missing", checker: &staticLegalAcceptanceChecker{}, wantStatus: http.StatusConflict, wantCode: "LEGAL_ACCEPTANCE_REQUIRED"},
		{name: "store failure", checker: &staticLegalAcceptanceChecker{err: apperrors.New("LEGAL_STORE_FAILED", "failed", 503)}, wantStatus: http.StatusServiceUnavailable, wantCode: "LEGAL_STORE_FAILED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/ugc", func(c *gin.Context) {
				c.Set(constants.ContextUserID, "user-1")
				c.Set(constants.ContextWorkspaceID, "workspace-1")
				c.Set(constants.ContextZoneID, "zone-1")
				c.Next()
			}, RequireCurrentLegalAcceptance(test.checker), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/ugc", nil))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if test.wantCode != "" && !strings.Contains(recorder.Body.String(), test.wantCode) {
				t.Fatalf("body = %s, want code %s", recorder.Body.String(), test.wantCode)
			}
		})
	}
}

func TestRequireCurrentLegalAcceptancePropagatesPlainError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/ugc", func(c *gin.Context) {
		c.Set(constants.ContextUserID, "user-1")
		c.Set(constants.ContextWorkspaceID, "workspace-1")
		c.Set(constants.ContextZoneID, "zone-1")
		c.Next()
	}, RequireCurrentLegalAcceptance(&staticLegalAcceptanceChecker{err: errors.New("database unavailable")}), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/ugc", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
}

func TestRequireCurrentLegalAcceptanceUsesRouteWorkspace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := &staticLegalAcceptanceChecker{accepted: true}
	router := gin.New()
	router.POST("/workspaces/:workspace_id/ugc", func(c *gin.Context) {
		c.Set(constants.ContextUserID, "user-1")
		c.Set(constants.ContextWorkspaceID, "token-workspace")
		c.Set(constants.ContextZoneID, "zone-1")
		c.Next()
	}, RequireCurrentLegalAcceptance(checker), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/workspaces/route-workspace/ugc", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", recorder.Code, recorder.Body.String())
	}
	if checker.userID != "user-1" || checker.workspaceID != "route-workspace" || checker.zoneID != "zone-1" {
		t.Fatalf("checked user/workspace/zone = %q/%q/%q, want user-1/route-workspace/zone-1", checker.userID, checker.workspaceID, checker.zoneID)
	}
}

func TestRequireCurrentLegalAcceptanceRejectsLegacyTokenWithoutZone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/ugc", func(c *gin.Context) {
		c.Set(constants.ContextUserID, "user-1")
		c.Set(constants.ContextWorkspaceID, "workspace-1")
		c.Next()
	}, RequireCurrentLegalAcceptance(&staticLegalAcceptanceChecker{accepted: true}), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/ugc", nil))
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "TOKEN_ZONE_REQUIRED") {
		t.Fatalf("response = %d %s, want 401 TOKEN_ZONE_REQUIRED", recorder.Code, recorder.Body.String())
	}
}
