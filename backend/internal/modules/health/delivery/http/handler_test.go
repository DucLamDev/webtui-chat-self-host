package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDesktopReleaseServesManifest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	manifestDir := filepath.Join(root, "stable", "windows-x86_64", "x86_64")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	manifest := []byte(`{"version":"0.2.0","notes":"signed build","pub_date":"2026-07-14T00:00:00Z","platforms":{"windows-x86_64":{"signature":"sig","url":"https://downloads.vpsttt.com/webtui-chat-0.2.0.msi"}}}`)
	if err := os.WriteFile(filepath.Join(manifestDir, "latest.json"), manifest, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	router := gin.New()
	NewHandler(BuildInfo{DesktopReleaseManifestDir: root}).Register(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/desktop/releases/stable/windows-x86_64/x86_64/0.1.0", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder.Body.String() != string(manifest) {
		t.Fatalf("body = %q, want manifest", recorder.Body.String())
	}
}

func TestDesktopReleaseReturnsNoContentWhenManifestNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(BuildInfo{}).Register(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/desktop/releases/stable/windows-x86_64/x86_64/0.1.0", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestDesktopReleaseRejectsUnsafePathParts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(BuildInfo{DesktopReleaseManifestDir: t.TempDir()}).Register(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/desktop/releases/stable/bad..target/x86_64/0.1.0", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
