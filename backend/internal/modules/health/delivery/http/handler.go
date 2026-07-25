package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type BuildInfo struct {
	Name                      string
	Env                       string
	Version                   string
	DesktopMinimumVersion     string
	DesktopRecommendedVersion string
	DesktopReleaseManifestDir string
	DesktopUpdateURL          string
	MobileMinimumVersion      string
	MobileRecommendedVersion  string
	MobileReleaseManifestDir  string
	MobileDownloadURL         string
	MobileStoreURL            string
	DownloadManifestDir       string
	StartedAt                 time.Time
	Now                       func() time.Time
	Checks                    map[string]CheckFunc
}

type CheckFunc func(context.Context) error

var desktopReleasePathPartPattern = regexp.MustCompile(`^[A-Za-z0-9._+-]+$`)

type Handler struct {
	build BuildInfo
}

func NewHandler(build BuildInfo) *Handler {
	if build.Now == nil {
		build.Now = time.Now
	}
	return &Handler{build: build}
}

func (h *Handler) Register(router gin.IRouter) {
	router.GET("/health", h.Health)
	router.GET("/ready", h.Ready)
	router.GET("/version", h.Version)
	router.GET("/desktop/releases/:channel/:target/:arch/:current_version", h.DesktopRelease)
	router.GET("/mobile/releases/:platform/:channel/:current_version", h.MobileRelease)
	router.GET("/downloads/manifest/:channel", h.DownloadManifest)
}

func (h *Handler) Health(c *gin.Context) {
	response.OK(c, nethttp.StatusOK, gin.H{
		"status": "ok",
		"app":    h.build.Name,
		"env":    h.build.Env,
		"uptime": h.build.Now().UTC().Sub(h.build.StartedAt).String(),
	})
}

func (h *Handler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	status := "ready"
	code := nethttp.StatusOK
	checks := gin.H{"api": "ok"}

	for name, check := range h.build.Checks {
		if check == nil {
			continue
		}
		if err := check(ctx); err != nil {
			status = "not_ready"
			code = nethttp.StatusServiceUnavailable
			checks[name] = err.Error()
			continue
		}
		checks[name] = "ok"
	}

	response.OK(c, code, gin.H{
		"status": status,
		"checks": checks,
	})
}

func (h *Handler) Version(c *gin.Context) {
	response.OK(c, nethttp.StatusOK, gin.H{
		"app": h.build.Name,
		"clients": gin.H{
			"desktop": gin.H{
				"minimum_version":     h.build.DesktopMinimumVersion,
				"recommended_version": h.build.DesktopRecommendedVersion,
				"update_url":          h.build.DesktopUpdateURL,
			},
			"mobile": gin.H{
				"minimum_version":     h.build.MobileMinimumVersion,
				"recommended_version": h.build.MobileRecommendedVersion,
				"download_url":        h.build.MobileDownloadURL,
				"store_url":           h.build.MobileStoreURL,
			},
		},
		"env":     h.build.Env,
		"version": h.build.Version,
	})
}

func (h *Handler) MobileRelease(c *gin.Context) {
	platform := strings.ToLower(strings.TrimSpace(c.Param("platform")))
	channel := strings.ToLower(strings.TrimSpace(c.Param("channel")))
	currentVersion := strings.TrimSpace(c.Param("current_version"))
	if platform != "android" && platform != "ios" {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_RELEASE_PLATFORM", "Nền tảng mobile không hợp lệ.", nil)
		return
	}
	if channel != "stable" && channel != "beta" && channel != "internal" {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_RELEASE_CHANNEL", "Mobile release channel không hợp lệ.", nil)
		return
	}
	if !safeDesktopReleasePart(currentVersion) {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_RELEASE_TARGET", "Phiên bản mobile không hợp lệ.", nil)
		return
	}
	if root := strings.TrimSpace(h.build.MobileReleaseManifestDir); root != "" {
		manifestPath, ok := safeManifestPath(root, channel, platform, "")
		if !ok {
			response.Fail(c, nethttp.StatusBadRequest, "INVALID_RELEASE_TARGET", "Mobile release target không hợp lệ.", nil)
			return
		}
		h.serveJSONManifest(c, manifestPath, "MOBILE_RELEASE")
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{
		"platform":            platform,
		"channel":             channel,
		"current_version":     currentVersion,
		"minimum_version":     h.build.MobileMinimumVersion,
		"recommended_version": h.build.MobileRecommendedVersion,
		"download_url":        h.build.MobileDownloadURL,
		"store_url":           h.build.MobileStoreURL,
		"required":            false,
	})
}

func (h *Handler) DownloadManifest(c *gin.Context) {
	root := strings.TrimSpace(h.build.DownloadManifestDir)
	if root == "" {
		response.NoContent(c)
		return
	}
	channel := strings.ToLower(strings.TrimSpace(c.Param("channel")))
	if channel != "stable" && channel != "beta" && channel != "internal" {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_RELEASE_CHANNEL", "Download manifest channel không hợp lệ.", nil)
		return
	}
	manifestPath, ok := safeDownloadManifestPath(root, channel)
	if !ok {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_RELEASE_TARGET", "Download manifest target không hợp lệ.", nil)
		return
	}
	h.serveJSONManifest(c, manifestPath, "DOWNLOAD_MANIFEST")
}

func (h *Handler) DesktopRelease(c *gin.Context) {
	root := strings.TrimSpace(h.build.DesktopReleaseManifestDir)
	if root == "" {
		response.NoContent(c)
		return
	}

	channel := strings.ToLower(strings.TrimSpace(c.Param("channel")))
	target := strings.TrimSpace(c.Param("target"))
	arch := strings.TrimSpace(c.Param("arch"))
	currentVersion := strings.TrimSpace(c.Param("current_version"))
	if channel != "stable" && channel != "beta" {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_RELEASE_CHANNEL", "Desktop release channel không hợp lệ.", nil)
		return
	}
	if !safeDesktopReleasePart(target) || !safeDesktopReleasePart(arch) || !safeDesktopReleasePart(currentVersion) {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_RELEASE_TARGET", "Desktop release target không hợp lệ.", nil)
		return
	}

	manifestPath, ok := safeManifestPath(root, channel, target, arch)
	if !ok {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_RELEASE_TARGET", "Desktop release target không hợp lệ.", nil)
		return
	}
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			response.NoContent(c)
			return
		}
		response.Fail(c, nethttp.StatusInternalServerError, "DESKTOP_RELEASE_UNAVAILABLE", "Không đọc được desktop release manifest.", nil)
		return
	}
	if !json.Valid(content) {
		response.Fail(c, nethttp.StatusInternalServerError, "DESKTOP_RELEASE_INVALID", "Desktop release manifest không phải JSON hợp lệ.", nil)
		return
	}

	c.Data(nethttp.StatusOK, "application/json; charset=utf-8", content)
}

func (h *Handler) serveJSONManifest(c *gin.Context, manifestPath string, codePrefix string) {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			response.NoContent(c)
			return
		}
		response.Fail(c, nethttp.StatusInternalServerError, codePrefix+"_UNAVAILABLE", "Không đọc được manifest phát hành.", nil)
		return
	}
	if !json.Valid(content) {
		response.Fail(c, nethttp.StatusInternalServerError, codePrefix+"_INVALID", "Manifest phát hành không phải JSON hợp lệ.", nil)
		return
	}
	c.Data(nethttp.StatusOK, "application/json; charset=utf-8", content)
}

func safeDesktopReleasePart(value string) bool {
	return value != "" &&
		value != "." &&
		value != ".." &&
		!strings.Contains(value, "..") &&
		desktopReleasePathPartPattern.MatchString(value)
}

func safeManifestPath(root string, channel string, target string, arch string) (string, bool) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	parts := []string{absRoot, channel, target}
	if strings.TrimSpace(arch) != "" {
		parts = append(parts, arch)
	}
	parts = append(parts, "latest.json")
	manifestPath := filepath.Join(parts...)
	absManifest, err := filepath.Abs(manifestPath)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(absRoot, absManifest)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", false
	}
	return absManifest, true
}

func safeDownloadManifestPath(root string, channel string) (string, bool) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	manifestPath := filepath.Join(absRoot, channel, "manifest.json")
	absManifest, err := filepath.Abs(manifestPath)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(absRoot, absManifest)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", false
	}
	return absManifest, true
}
