package http

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSelfHostedRoutesDoNotExposeSaaSProvisioning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := NewHandler(nil)
	handler.SetSaaSProvisioningEnabled(false)
	handler.RegisterRoutes(
		engine,
		engine.Group("/api/v1"),
		func(c *gin.Context) { c.Next() },
	)

	routes := map[string]bool{}
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	for _, expected := range []string{
		"GET /.well-known/vpsttt-chat",
		"GET /api/v1/discovery",
		"GET /api/v1/zones/current",
	} {
		if !routes[expected] {
			t.Fatalf("missing self-hosted route %q", expected)
		}
	}
	for _, forbidden := range []string{
		"GET /internal/tenancy/caddy-ask",
		"POST /api/v1/zones/claims",
		"POST /api/v1/zones/current/deployment-requests",
	} {
		if routes[forbidden] {
			t.Fatalf("self-hosted mode exposed SaaS route %q", forbidden)
		}
	}
}
