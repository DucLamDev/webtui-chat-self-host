package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
	"github.com/duclamdev/application-chat/backend/internal/shared/constants"
	"github.com/gin-gonic/gin"
)

type fakeWorkspaceZoneChecker struct {
	matches                  bool
	domainMatches            bool
	recoverableDomainMatches bool
}

func (checker fakeWorkspaceZoneChecker) WorkspaceBelongsToZone(context.Context, string, string) (bool, error) {
	return checker.matches, nil
}

func (checker fakeWorkspaceZoneChecker) ZoneDomainBelongsToActiveZone(context.Context, string, string) (bool, error) {
	return checker.domainMatches, nil
}

func (checker fakeWorkspaceZoneChecker) ZoneDomainBelongsToRecoverableZone(context.Context, string, string) (bool, error) {
	return checker.recoverableDomainMatches, nil
}

func TestAuthRejectsTokenFromAnotherResolvedZone(t *testing.T) {
	tokenManager, token := zoneToken(t, "zone-a")
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constants.ContextResolvedZoneID, "zone-b")
		c.Next()
	})
	router.Use(Auth(tokenManager, fakeWorkspaceZoneChecker{matches: true, domainMatches: true}))
	router.GET("/resource", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/resource", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestAuthRejectsWorkspaceOutsideTokenZone(t *testing.T) {
	tokenManager, token := zoneToken(t, "zone-a")
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(constants.ContextResolvedZoneID, "zone-a")
		c.Next()
	})
	router.Use(Auth(tokenManager, fakeWorkspaceZoneChecker{matches: false, domainMatches: true}))
	router.GET("/workspaces/:workspace_id/resource", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/workspaces/workspace-b/resource", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestAuthRejectsTokenWhenRequestDomainIsNotActiveInZone(t *testing.T) {
	tokenManager, token := zoneToken(t, "zone-a")
	router := gin.New()
	router.Use(Auth(tokenManager, fakeWorkspaceZoneChecker{matches: true, domainMatches: false}))
	router.GET("/resource", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "https://suspended.example/resource", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestRecoveryAuthAllowsOnlyRecoverableZoneDomain(t *testing.T) {
	tokenManager, token := zoneToken(t, "zone-a")
	router := gin.New()
	router.Use(AuthForZoneRecovery(tokenManager, fakeWorkspaceZoneChecker{
		matches:                  true,
		recoverableDomainMatches: true,
	}))
	router.POST("/lifecycle", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "https://suspended.example/lifecycle", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func zoneToken(t *testing.T, zoneID string) (*sharedauth.Manager, string) {
	t.Helper()
	manager := sharedauth.NewManager(
		"access_secret_du_32_ky_tu_de_test",
		"refresh_secret_du_32_ky_tu_de_test",
		time.Minute,
		time.Hour,
	)
	token, _, err := manager.CreateZoneAccessToken(
		"user-1",
		"user@example.com",
		"user",
		zoneID,
		"workspace-1",
		"chat.customer.example",
	)
	if err != nil {
		t.Fatal(err)
	}
	return manager, token
}
