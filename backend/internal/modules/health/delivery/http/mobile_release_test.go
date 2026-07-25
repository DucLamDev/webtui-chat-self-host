package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMobileReleaseReturnsFallbackMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(BuildInfo{
		MobileMinimumVersion:     "1.0.0",
		MobileRecommendedVersion: "1.1.0",
		MobileDownloadURL:        "https://download.vpsttt.com/mobile/webtui-chat.apk",
		MobileStoreURL:           "https://play.google.com/store/apps/details?id=com.vpsttt.webtui",
	}).Register(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/mobile/releases/android/stable/0.9.0", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"minimum_version":"1.0.0"`) {
		t.Fatalf("body không có minimum_version: %s", recorder.Body.String())
	}
}

func TestMobileReleaseServesManifest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	manifestDir := filepath.Join(root, "stable", "android")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	manifest := []byte(`{"version":"1.1.0","required":false,"checksum_sha256":"abc"}`)
	if err := os.WriteFile(filepath.Join(manifestDir, "latest.json"), manifest, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	router := gin.New()
	NewHandler(BuildInfo{MobileReleaseManifestDir: root}).Register(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/mobile/releases/android/stable/1.0.0", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder.Body.String() != string(manifest) {
		t.Fatalf("body = %q, want manifest", recorder.Body.String())
	}
}

func TestDownloadManifestServesChannelManifest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	manifestDir := filepath.Join(root, "stable")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	manifest := []byte(`{"files":[{"name":"webtui-chat.apk","checksum_sha256":"abc"}]}`)
	if err := os.WriteFile(filepath.Join(manifestDir, "manifest.json"), manifest, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	router := gin.New()
	NewHandler(BuildInfo{DownloadManifestDir: root}).Register(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/downloads/manifest/stable", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder.Body.String() != string(manifest) {
		t.Fatalf("body = %q, want manifest", recorder.Body.String())
	}
}
