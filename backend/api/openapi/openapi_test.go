package openapi

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPIParsesAndContainsMobileP0Paths(t *testing.T) {
	content, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatalf("ReadFile(openapi.yaml) error = %v", err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(content, &document); err != nil {
		t.Fatalf("openapi.yaml không phải YAML hợp lệ: %v", err)
	}
	paths, ok := document["paths"].(map[string]any)
	if !ok {
		t.Fatal("openapi.yaml thiếu paths")
	}
	for _, path := range []string{
		"/.well-known/vpsttt-chat",
		"/api/v1/discovery",
		"/api/v1/capabilities",
		"/api/v1/zones/claims",
		"/api/v1/zones/claims/{domain_id}",
		"/api/v1/zones/claims/{domain_id}/verify",
		"/api/v1/zones/current",
		"/api/v1/zones/current/lifecycle",
		"/api/v1/zones/current/domains",
		"/api/v1/zones/current/domains/{domain_id}",
		"/api/v1/zones/current/domains/{domain_id}/primary",
		"/api/v1/zones/current/deployment-requests",
		"/api/v1/zones/current/quota",
		"/api/v1/zones/current/oidc-providers",
		"/api/v1/zones/current/oidc-providers/{provider_id}",
		"/api/v1/zones/current/automation-templates",
		"/api/v1/zones/current/automation-installations",
		"/api/v1/auth/oidc/providers",
		"/api/v1/auth/oidc/start",
		"/api/v1/auth/oidc/callback",
		"/api/v1/auth/oidc/complete",
		"/mobile/releases/{platform}/{channel}/{current_version}",
		"/downloads/manifest/{channel}",
		"/api/v1/mobile/devices",
		"/api/v1/workspaces/{workspace_id}/sync",
		"/api/v1/workspaces/{workspace_id}/calls",
		"/api/v1/workspaces/{workspace_id}/channels/{channel_id}/media",
		"/api/v1/workspaces/{workspace_id}/bots/{bot_id}/ai-config",
		"/api/v1/workspaces/{workspace_id}/bots/{bot_id}/flows",
	} {
		if _, ok := paths[path]; !ok {
			t.Fatalf("openapi.yaml thiếu path %s", path)
		}
	}
}
