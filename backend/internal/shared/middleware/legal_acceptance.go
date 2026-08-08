package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type CurrentLegalAcceptanceChecker interface {
	HasCurrentLegalAcceptances(ctx context.Context, userID string, workspaceID string, zoneID string) (bool, error)
}

// RequireCurrentLegalAcceptance gates only UGC-producing routes. Authentication,
// account recovery, reads, reporting/blocking, deletion and the acceptance
// endpoint itself must remain reachable so an existing account can recover.
func RequireCurrentLegalAcceptance(checker CurrentLegalAcceptanceChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if checker == nil {
			response.Fail(c, http.StatusServiceUnavailable, "LEGAL_ACCEPTANCE_UNAVAILABLE", "Legal acceptance status is temporarily unavailable.", nil)
			c.Abort()
			return
		}
		userID := CurrentUserID(c)
		// Workspace-scoped routes may target another workspace in the same zone.
		// Always check consent for the workspace being mutated, not merely the
		// workspace embedded in the access token.
		workspaceID := strings.TrimSpace(c.Param("workspace_id"))
		if workspaceID == "" {
			workspaceID = strings.TrimSpace(c.Query("workspace_id"))
		}
		if workspaceID == "" {
			workspaceID = CurrentWorkspaceID(c)
		}
		zoneID := CurrentZoneID(c)
		if strings.TrimSpace(userID) == "" || strings.TrimSpace(workspaceID) == "" {
			response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "A workspace-scoped session is required.", nil)
			c.Abort()
			return
		}
		if strings.TrimSpace(zoneID) == "" {
			response.Fail(c, http.StatusUnauthorized, "TOKEN_ZONE_REQUIRED", "Sign in again with a zone-scoped session.", nil)
			c.Abort()
			return
		}
		accepted, err := checker.HasCurrentLegalAcceptances(c.Request.Context(), userID, workspaceID, zoneID)
		if err != nil {
			response.Error(c, err)
			c.Abort()
			return
		}
		if !accepted {
			response.Fail(
				c,
				http.StatusConflict,
				"LEGAL_ACCEPTANCE_REQUIRED",
				"Accept the current Terms, Acceptable Use Policy, and Privacy Policy before creating or uploading content.",
				nil,
			)
			c.Abort()
			return
		}
		c.Next()
	}
}
