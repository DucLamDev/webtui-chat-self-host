package bootstrap

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSelfHostedUpdatePreservesMobileReleaseContracts(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() did not return the test file path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	updateBytes, err := os.ReadFile(filepath.Join(repositoryRoot, "deploy", "self-hosted", "update.sh"))
	if err != nil {
		t.Fatalf("read update.sh: %v", err)
	}
	update := string(updateBytes)
	for _, required := range []string{
		"OFFICIAL_APP_LINK_HOST=chat.vpsttt.com",
		`""|"$INSTANCE_DOMAIN")`,
		`write_env_value WEBTUI_APP_LINK_HOST "$OFFICIAL_APP_LINK_HOST"`,
		"PRESERVE_CUSTOM_APP_LINK_HOST",
		`write_env_value ENABLE_IOS_ASSOCIATION "false"`,
		`--profile push-relay build --pull push-relay`,
		`--profile push-relay up -d --no-deps --force-recreate push-relay`,
		`wget -qO- "http://localhost:$PUSH_RELAY_PORT/ready"`,
		`if [ "$PUSH_RELAY_PORT" != "8090" ]; then`,
	} {
		if !strings.Contains(update, required) {
			t.Fatalf("self-host update is missing mobile release guard %q", required)
		}
	}

	for _, relativePath := range []string{
		filepath.Join("deploy", "self-hosted", ".env.example"),
		filepath.Join("deploy", "self-hosted", "install.sh"),
	} {
		contentBytes, err := os.ReadFile(filepath.Join(repositoryRoot, relativePath))
		if err != nil {
			t.Fatalf("read %s: %v", relativePath, err)
		}
		content := string(contentBytes)
		if !strings.Contains(content, "WEBTUI_APP_LINK_HOST=chat.vpsttt.com") &&
			!strings.Contains(content, "WEBTUI_APP_LINK_HOST=$APP_LINK_HOST") {
			t.Fatalf("%s does not provision the official publisher app-link host", relativePath)
		}
		if !strings.Contains(content, "PRESERVE_CUSTOM_APP_LINK_HOST=false") {
			t.Fatalf("%s does not make the custom-branded opt-out explicit", relativePath)
		}
		if !strings.Contains(content, "ENABLE_IOS_ASSOCIATION=false") {
			t.Fatalf("%s does not provision the Play-only AASA default", relativePath)
		}
	}
}
