package http

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	nethttp "net/http"
	"path/filepath"
	"strings"

	tenancyapp "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/application"
	platformstorage "github.com/duclamdev/application-chat/backend/internal/platform/storage"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type updateZoneStorageRequest struct {
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

func (h *Handler) GetCurrentZoneStorage(c *gin.Context) {
	settings, err := h.service.GetZoneStorageSettings(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"storage": settings})
}

func (h *Handler) UpdateCurrentZoneStorage(c *gin.Context) {
	var req updateZoneStorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	settings, err := h.service.UpdateZoneStorageSettings(c.Request.Context(), tenancyapp.UpdateZoneStorageInput{
		ActorUserID:     middleware.CurrentUserID(c),
		ZoneID:          middleware.CurrentZoneID(c),
		Provider:        req.Provider,
		Endpoint:        req.Endpoint,
		Region:          req.Region,
		Bucket:          req.Bucket,
		AccessKeyID:     req.AccessKeyID,
		SecretAccessKey: req.SecretAccessKey,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	if h.storageResolver != nil {
		h.storageResolver.Invalidate(middleware.CurrentZoneID(c))
	}
	response.OK(c, nethttp.StatusOK, gin.H{"storage": settings})
}

func (h *Handler) uploadCurrentZoneLogoToObjectStorage(c *gin.Context) {
	zoneID := strings.TrimSpace(middleware.CurrentZoneID(c))
	if !safeBrandingPathSegment.MatchString(zoneID) {
		response.Fail(c, nethttp.StatusBadRequest, "ZONE_REQUIRED", "Không xác định được host hiện tại.", nil)
		return
	}
	if _, err := h.service.GetZoneAdminOverview(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		zoneID,
	); err != nil {
		response.Error(c, err)
		return
	}

	c.Request.Body = nethttp.MaxBytesReader(c.Writer, c.Request.Body, maxBrandingLogoBytes+(512<<10))
	header, err := c.FormFile("logo")
	if err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "LOGO_FILE_REQUIRED", "Hãy chọn một file logo PNG, JPEG hoặc WebP.", nil)
		return
	}
	if header.Size <= 0 || header.Size > maxBrandingLogoBytes {
		response.Fail(c, nethttp.StatusRequestEntityTooLarge, "LOGO_TOO_LARGE", "Logo không được vượt quá 4 MB.", nil)
		return
	}
	source, err := header.Open()
	if err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "LOGO_READ_FAILED", "Không đọc được file logo đã chọn.", nil)
		return
	}
	defer source.Close()
	content, err := io.ReadAll(io.LimitReader(source, maxBrandingLogoBytes+1))
	if err != nil || len(content) == 0 {
		response.Fail(c, nethttp.StatusBadRequest, "LOGO_READ_FAILED", "Không đọc được file logo đã chọn.", nil)
		return
	}
	if len(content) > maxBrandingLogoBytes {
		response.Fail(c, nethttp.StatusRequestEntityTooLarge, "LOGO_TOO_LARGE", "Logo không được vượt quá 4 MB.", nil)
		return
	}
	contentType, extension := brandingLogoType(content)
	if extension == "" {
		response.Fail(c, nethttp.StatusUnsupportedMediaType, "LOGO_TYPE_UNSUPPORTED", "Chỉ hỗ trợ logo PNG, JPEG hoặc WebP.", nil)
		return
	}

	location, err := h.storageResolver.ResolveZone(c.Request.Context(), zoneID)
	if err != nil {
		response.Fail(c, nethttp.StatusServiceUnavailable, "BRANDING_STORAGE_UNAVAILABLE", "Không mở được storage riêng của host.", nil)
		return
	}
	sum := sha256.Sum256(content)
	fileName := "logo-" + hex.EncodeToString(sum[:6]) + extension
	objectKey := "zones/" + zoneID + "/branding/" + fileName
	object, err := location.Store.Put(c.Request.Context(), platformstorage.PutObjectInput{
		Key: objectKey, Body: bytes.NewReader(content), ContentType: contentType, Size: int64(len(content)),
	})
	if err != nil {
		response.Fail(c, nethttp.StatusBadGateway, "LOGO_STORE_FAILED", "Không lưu được logo vào storage của host.", nil)
		return
	}
	response.Created(c, gin.H{"logo": gin.H{
		"content_type": contentType,
		"logo_path":    "/api/v1/branding/" + zoneID + "/" + fileName,
		"size":         object.Size,
	}})
}

func (h *Handler) ServeZoneBrandingLogo(c *gin.Context) {
	if h.storageResolver == nil {
		c.Status(nethttp.StatusNotFound)
		return
	}
	zoneID := strings.TrimSpace(c.Param("zone_id"))
	fileName := strings.TrimSpace(c.Param("file_name"))
	if !safeBrandingPathSegment.MatchString(zoneID) || !safeBrandingPathSegment.MatchString(strings.TrimSuffix(fileName, filepath.Ext(fileName))) {
		c.Status(nethttp.StatusNotFound)
		return
	}
	extension := strings.ToLower(filepath.Ext(fileName))
	contentType := ""
	switch extension {
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".webp":
		contentType = "image/webp"
	default:
		c.Status(nethttp.StatusNotFound)
		return
	}
	location, err := h.storageResolver.ResolveZone(c.Request.Context(), zoneID)
	if err != nil {
		c.Status(nethttp.StatusNotFound)
		return
	}
	object, err := location.Store.Get(c.Request.Context(), "zones/"+zoneID+"/branding/"+fileName)
	if err != nil {
		c.Status(nethttp.StatusNotFound)
		return
	}
	defer object.Body.Close()
	if object.Info.ContentType != "" {
		contentType = object.Info.ContentType
	}
	c.Header("Cache-Control", "public, max-age=86400, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(nethttp.StatusOK, object.Info.Size, contentType, object.Body, nil)
}
