package bootstrap

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSelfHostedOnlyOfficeIngressPreservesPublicPort(t *testing.T) {
	content := readSelfHostedCaddyfileForTest(t)
	block := caddySiteBlockForTest(t, content, "https://{$INSTANCE_DOMAIN}:8444 {")

	for _, required := range []string{
		"reverse_proxy onlyoffice-document-server:80",
		"header_up Host {hostport}",
		"header_up X-Forwarded-Proto https",
		"header_up X-Forwarded-Host {hostport}",
		"header_up X-Forwarded-Port 8444",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("ONLYOFFICE ingress is missing %q", required)
		}
	}
	if strings.Contains(block, "header_up X-Forwarded-Host {host}") {
		t.Fatal("ONLYOFFICE ingress must not strip :8444 from X-Forwarded-Host")
	}
}

func TestSelfHostedOnlyOfficeCacheFallbackAllowsEditorOrigin(t *testing.T) {
	content := readSelfHostedCaddyfileForTest(t)
	mainBlock := caddySiteBlockForTest(t, content, "{$INSTANCE_DOMAIN} {")

	for _, required := range []string{
		"@onlyOfficeCachePreflight",
		"path /cache/files/*",
		"Access-Control-Allow-Origin \"https://{$INSTANCE_DOMAIN}:8444\"",
		"Access-Control-Allow-Credentials \"true\"",
		"Access-Control-Allow-Methods \"GET,HEAD,OPTIONS\"",
		"Access-Control-Allow-Headers \"Content-Type,Range\"",
		"@onlyOfficeCache path /cache/files/*",
		"reverse_proxy onlyoffice-document-server:80",
	} {
		if !strings.Contains(mainBlock, required) {
			t.Fatalf("main ingress ONLYOFFICE cache fallback is missing %q", required)
		}
	}

	cacheIndex := strings.Index(mainBlock, "handle @onlyOfficeCache")
	backendIndex := strings.Index(mainBlock, "handle @backend")
	if cacheIndex < 0 || backendIndex < 0 || cacheIndex > backendIndex {
		t.Fatal("ONLYOFFICE cache fallback must run before the generic backend/web routes")
	}
}

func readSelfHostedCaddyfileForTest(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() did not return the test file path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	contentBytes, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "self-hosted", "Caddyfile"))
	if err != nil {
		t.Fatalf("read Caddyfile: %v", err)
	}
	return string(contentBytes)
}

func caddySiteBlockForTest(t *testing.T, content string, start string) string {
	t.Helper()

	startIndex := strings.Index(content, start)
	if startIndex < 0 {
		t.Fatalf("Caddyfile is missing site block %q", start)
	}
	bodyStart := startIndex + len(start)
	depth := 1
	for index := bodyStart; index < len(content); index++ {
		switch content[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[bodyStart:index]
			}
		}
	}
	t.Fatalf("Caddyfile site block %q is not closed", start)
	return ""
}
