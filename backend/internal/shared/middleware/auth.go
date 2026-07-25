package middleware

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
	"github.com/duclamdev/application-chat/backend/internal/shared/constants"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type WorkspaceZoneChecker interface {
	WorkspaceBelongsToZone(ctx context.Context, workspaceID string, zoneID string) (bool, error)
	ZoneDomainBelongsToActiveZone(ctx context.Context, zoneID string, domain string) (bool, error)
	ZoneDomainBelongsToRecoverableZone(ctx context.Context, zoneID string, domain string) (bool, error)
}

func Auth(tokens *sharedauth.Manager, zoneCheckers ...WorkspaceZoneChecker) gin.HandlerFunc {
	return authWithZonePolicy(tokens, false, zoneCheckers...)
}

func AuthForZoneRecovery(tokens *sharedauth.Manager, zoneCheckers ...WorkspaceZoneChecker) gin.HandlerFunc {
	return authWithZonePolicy(tokens, true, zoneCheckers...)
}

func authWithZonePolicy(
	tokens *sharedauth.Manager,
	allowSuspendedZone bool,
	zoneCheckers ...WorkspaceZoneChecker,
) gin.HandlerFunc {
	var zoneChecker WorkspaceZoneChecker
	if len(zoneCheckers) > 0 {
		zoneChecker = zoneCheckers[0]
	}
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader("Authorization"))
		if raw == "" {
			response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "Bạn cần đăng nhập để tiếp tục.", nil)
			c.Abort()
			return
		}

		prefix := "Bearer "
		if !strings.HasPrefix(raw, prefix) {
			response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "Header Authorization không hợp lệ.", nil)
			c.Abort()
			return
		}

		claims, err := tokens.VerifyAccessToken(strings.TrimSpace(strings.TrimPrefix(raw, prefix)))
		if err != nil {
			message := "Token không hợp lệ."
			if errors.Is(err, sharedauth.ErrExpiredToken) {
				message = "Token đã hết hạn."
			}
			response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", message, nil)
			c.Abort()
			return
		}

		resolvedZoneID := contextString(c, constants.ContextResolvedZoneID)
		if resolvedZoneID != "" && claims.ZoneID == "" {
			response.Fail(c, http.StatusUnauthorized, "TOKEN_ZONE_REQUIRED", "Phien dang nhap cu khong co zone. Vui long dang nhap lai.", nil)
			c.Abort()
			return
		}
		if resolvedZoneID != "" && claims.ZoneID != resolvedZoneID {
			response.Fail(c, http.StatusForbidden, "ZONE_TOKEN_MISMATCH", "Phien dang nhap khong thuoc domain hien tai.", nil)
			c.Abort()
			return
		}
		if zoneChecker != nil && claims.ZoneID != "" && !isLocalDevelopmentHost(c.Request.Host) {
			matches, checkErr := zoneChecker.ZoneDomainBelongsToActiveZone(
				c.Request.Context(),
				claims.ZoneID,
				c.Request.Host,
			)
			if checkErr != nil {
				response.Error(c, checkErr)
				c.Abort()
				return
			}
			if !matches && allowSuspendedZone {
				matches, checkErr = zoneChecker.ZoneDomainBelongsToRecoverableZone(
					c.Request.Context(),
					claims.ZoneID,
					c.Request.Host,
				)
				if checkErr != nil {
					response.Error(c, checkErr)
					c.Abort()
					return
				}
			}
			if !matches {
				response.Fail(
					c,
					http.StatusForbidden,
					"ZONE_DOMAIN_MISMATCH",
					"Domain hien tai khong hoat dong trong zone cua phien dang nhap.",
					nil,
				)
				c.Abort()
				return
			}
		}
		workspaceID := strings.TrimSpace(c.Param("workspace_id"))
		if workspaceID == "" {
			workspaceID = strings.TrimSpace(c.Query("workspace_id"))
		}
		if zoneChecker != nil && claims.ZoneID != "" && workspaceID != "" {
			matches, checkErr := zoneChecker.WorkspaceBelongsToZone(c.Request.Context(), workspaceID, claims.ZoneID)
			if checkErr != nil {
				response.Error(c, checkErr)
				c.Abort()
				return
			}
			if !matches {
				response.Fail(c, http.StatusForbidden, "WORKSPACE_ZONE_MISMATCH", "Workspace khong thuoc zone cua phien dang nhap.", nil)
				c.Abort()
				return
			}
		}

		c.Set(constants.ContextUserID, claims.Subject)
		c.Set(constants.ContextEmail, claims.Email)
		c.Set(constants.ContextUsername, claims.Username)
		c.Set(constants.ContextZoneID, claims.ZoneID)
		c.Set(constants.ContextWorkspaceID, claims.WorkspaceID)
		c.Set(constants.ContextZoneDomain, claims.Domain)
		c.Next()
	}
}

func isLocalDevelopmentHost(rawHost string) bool {
	host := strings.ToLower(strings.TrimSpace(rawHost))
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func contextString(c *gin.Context, key string) string {
	value, _ := c.Get(key)
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func CurrentUserID(c *gin.Context) string {
	value, ok := c.Get(constants.ContextUserID)
	if !ok {
		return ""
	}
	userID, _ := value.(string)
	return userID
}

func CurrentZoneID(c *gin.Context) string {
	value, ok := c.Get(constants.ContextZoneID)
	if !ok {
		return ""
	}
	zoneID, _ := value.(string)
	return zoneID
}

func CurrentWorkspaceID(c *gin.Context) string {
	value, ok := c.Get(constants.ContextWorkspaceID)
	if !ok {
		return ""
	}
	workspaceID, _ := value.(string)
	return workspaceID
}

func CurrentZoneDomain(c *gin.Context) string {
	value, ok := c.Get(constants.ContextZoneDomain)
	if !ok {
		return ""
	}
	domain, _ := value.(string)
	return domain
}
