package http

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBrandingLogoTypeRejectsActiveContent(t *testing.T) {
	contentType, extension := brandingLogoType([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	if contentType != "" || extension != "" {
		t.Fatalf("brandingLogoType(svg) = %q, %q; want rejection", contentType, extension)
	}

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	contentType, extension = brandingLogoType(png)
	if contentType != "image/png" || extension != ".png" {
		t.Fatalf("brandingLogoType(png) = %q, %q", contentType, extension)
	}
}

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
