package bootstrap

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSelfHostedIngressProxiesMobileAssociationsWithoutRedirect(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() did not return the test file path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	caddyPath := filepath.Join(repositoryRoot, "deploy", "self-hosted", "Caddyfile")
	contentBytes, err := os.ReadFile(caddyPath)
	if err != nil {
		t.Fatalf("read Caddyfile: %v", err)
	}
	content := string(contentBytes)
	for _, required := range []string{
		"host {$WEBTUI_APP_LINK_HOST}",
		"/.well-known/assetlinks.json",
		"/.well-known/apple-app-site-association",
		"reverse_proxy https://{$PORTAL_DOMAIN}",
		"header_up Host {$PORTAL_DOMAIN}",
		"tls_server_name {$PORTAL_DOMAIN}",
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("Caddy app association route is missing %q", required)
		}
	}
	associationIndex := strings.Index(content, "handle @appAssociations")
	backendIndex := strings.Index(content, "handle @backend")
	if associationIndex < 0 || backendIndex < 0 || associationIndex > backendIndex {
		t.Fatal("app association route must run before the generic /.well-known backend route")
	}
	associationBlock := content[associationIndex:backendIndex]
	if strings.Contains(associationBlock, "redir") {
		t.Fatal("app association routes must proxy their response body and never redirect")
	}

	composeBytes, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "self-hosted", "compose.yml"))
	if err != nil {
		t.Fatalf("read self-host compose: %v", err)
	}
	compose := string(composeBytes)
	for _, required := range []string{
		"PORTAL_DOMAIN: ${PORTAL_DOMAIN:?",
		"WEBTUI_APP_LINK_HOST: ${WEBTUI_APP_LINK_HOST:?",
	} {
		if !strings.Contains(compose, required) {
			t.Fatalf("compose must fail closed when app association config is missing: %q", required)
		}
	}
}
