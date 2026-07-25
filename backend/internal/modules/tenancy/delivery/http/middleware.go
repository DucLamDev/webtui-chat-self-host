package http

import (
	"strings"

	tenancyapp "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/constants"
	"github.com/gin-gonic/gin"
)

const (
	ContextZoneIDKey      = constants.ContextResolvedZoneID
	ContextZoneSlugKey    = constants.ContextResolvedZoneSlug
	ContextZoneKindKey    = constants.ContextResolvedZoneKind
	ContextZoneDomainKey  = constants.ContextResolvedZoneDomain
	ContextWorkspaceIDKey = constants.ContextResolvedWorkspaceID
)

func OptionalZoneContext(service *tenancyapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := strings.TrimSpace(c.Request.Host)
		if host == "" {
			c.Next()
			return
		}

		zoneContext, err := service.ResolveContext(c.Request.Context(), host)
		if err == nil {
			c.Set(ContextZoneIDKey, zoneContext.ZoneID)
			c.Set(ContextZoneSlugKey, zoneContext.ZoneSlug)
			c.Set(ContextZoneKindKey, zoneContext.ZoneKind)
			c.Set(ContextZoneDomainKey, zoneContext.Domain)
			if zoneContext.WorkspaceID != "" {
				c.Set(ContextWorkspaceIDKey, zoneContext.WorkspaceID)
			}
		}

		c.Next()
	}
}

func CurrentZoneID(c *gin.Context) string {
	value, _ := c.Get(ContextZoneIDKey)
	zoneID, _ := value.(string)
	return zoneID
}
