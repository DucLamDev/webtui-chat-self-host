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
		"/api/v1/auth/legal-documents",
		"/api/v1/auth/legal-acceptance",
		"/api/v1/workspaces/{workspace_id}/moderation/reports",
		"/api/v1/workspaces/{workspace_id}/moderation/reports/{report_id}",
		"/api/v1/workspaces/{workspace_id}/blocks",
		"/api/v1/workspaces/{workspace_id}/blocks/{blocked_user_id}",
		"/api/v1/calls/ice-servers",
		"/mobile/releases/{platform}/{channel}/{current_version}",
		"/downloads/manifest/{channel}",
		"/api/v1/mobile/devices",
		"/api/v1/notifications/web-push/config",
		"/api/v1/notifications/web-push/subscriptions",
		"/api/v1/notifications/web-push/subscriptions/{subscription_id}",
		"/api/v1/workspaces/{workspace_id}/admin/push",
		"/api/v1/workspaces/{workspace_id}/admin/messages",
		"/api/v1/workspaces/{workspace_id}/admin/push/dead-letters/{job_id}/replay",
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
	channelPreference, ok := paths["/api/v1/notifications/preferences/channels/{channel_id}"].(map[string]any)
	if !ok || channelPreference["get"] == nil || channelPreference["put"] == nil {
		t.Fatal("channel notification preference contract must contain GET and PUT")
	}
	webPushSubscription, ok := paths["/api/v1/notifications/web-push/subscriptions/{subscription_id}"].(map[string]any)
	if !ok || webPushSubscription["delete"] == nil || webPushSubscription["put"] != nil {
		t.Fatal("Web Push subscription item contract must contain only the expected DELETE operation")
	}
	components, ok := document["components"].(map[string]any)
	if !ok {
		t.Fatal("openapi.yaml thiếu components")
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("openapi.yaml thiếu component schemas")
	}
	register, ok := schemas["RegisterRequest"].(map[string]any)
	if !ok {
		t.Fatal("openapi.yaml thiếu RegisterRequest")
	}
	requiredValues, ok := register["required"].([]any)
	if !ok {
		t.Fatal("RegisterRequest.required phải là array")
	}
	required := make(map[string]bool, len(requiredValues))
	for _, value := range requiredValues {
		required[value.(string)] = true
	}
	for _, field := range []string{"terms_accepted", "terms_version", "privacy_accepted", "privacy_version"} {
		if !required[field] {
			t.Fatalf("RegisterRequest.required thiếu %s", field)
		}
	}
	properties := register["properties"].(map[string]any)
	for _, field := range []string{"terms_accepted", "privacy_accepted"} {
		property := properties[field].(map[string]any)
		if value, ok := property["const"].(bool); !ok || !value {
			t.Fatalf("RegisterRequest.%s phải const=true", field)
		}
	}
	legalAcceptance, ok := schemas["LegalAcceptanceRequest"].(map[string]any)
	if !ok {
		t.Fatal("openapi.yaml is missing LegalAcceptanceRequest")
	}
	legalRequiredValues, ok := legalAcceptance["required"].([]any)
	if !ok || len(legalRequiredValues) != 4 {
		t.Fatal("LegalAcceptanceRequest must require all four legal fields")
	}
	legalProperties := legalAcceptance["properties"].(map[string]any)
	if legalProperties["workspace_id"] == nil {
		t.Fatal("LegalAcceptanceRequest must support explicit workspace_id")
	}
	for _, field := range []string{"terms_accepted", "privacy_accepted"} {
		property := legalProperties[field].(map[string]any)
		if value, ok := property["const"].(bool); !ok || !value {
			t.Fatalf("LegalAcceptanceRequest.%s must be const=true", field)
		}
	}
	legalPath := paths["/api/v1/auth/legal-acceptance"].(map[string]any)
	legalGet := legalPath["get"].(map[string]any)
	parameters, ok := legalGet["parameters"].([]any)
	if !ok || len(parameters) == 0 || parameters[0].(map[string]any)["name"] != "workspace_id" {
		t.Fatal("GET legal acceptance must expose optional workspace_id query scope")
	}
	currentLegal := schemas["CurrentLegalAcceptance"].(map[string]any)
	currentProperties := currentLegal["properties"].(map[string]any)
	if currentProperties["workspace_id"] == nil {
		t.Fatal("CurrentLegalAcceptance response must echo workspace_id")
	}
	currentRequiredValues, ok := currentLegal["required"].([]any)
	if !ok {
		t.Fatal("CurrentLegalAcceptance.required must be an array")
	}
	currentRequired := make(map[string]bool, len(currentRequiredValues))
	for _, value := range currentRequiredValues {
		currentRequired[value.(string)] = true
	}
	if !currentRequired["workspace_id"] {
		t.Fatal("CurrentLegalAcceptance.workspace_id must be required")
	}
	for _, path := range []string{"/api/v1/auth/google", "/api/v1/auth/oidc/complete"} {
		operation := paths[path].(map[string]any)["post"].(map[string]any)
		responses := operation["responses"].(map[string]any)
		if responses["409"] == nil {
			t.Fatalf("%s thiếu response 409 legal/JIT contract", path)
		}
	}
	for _, path := range []string{"/api/v1/integrations/messages", "/api/v1/hooks/incoming/{webhook_id}"} {
		operation := paths[path].(map[string]any)["post"].(map[string]any)
		responses := operation["responses"].(map[string]any)
		for _, status := range []string{"401", "409", "503"} {
			if responses[status] == nil {
				t.Fatalf("%s thiếu response %s cho integration owner/legal fail-closed contract", path, status)
			}
		}
	}
	relayContent, err := os.ReadFile("push-relay.yaml")
	if err != nil {
		t.Fatalf("push relay OpenAPI contract is missing: %v", err)
	}
	var relayDocument map[string]any
	if err := yaml.Unmarshal(relayContent, &relayDocument); err != nil {
		t.Fatalf("push-relay.yaml is not valid YAML: %v", err)
	}
	relayPaths, ok := relayDocument["paths"].(map[string]any)
	if !ok || relayPaths["/v1/deliveries"] == nil || relayPaths["/ready"] == nil {
		t.Fatal("push-relay.yaml is missing required relay paths")
	}
}
