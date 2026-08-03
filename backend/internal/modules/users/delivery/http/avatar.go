package http

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	nethttp "net/http"
	"path/filepath"
	"regexp"
	"strings"

	platformstorage "github.com/duclamdev/application-chat/backend/internal/platform/storage"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

const maxAvatarBytes = 8 << 20

var safeAvatarSegment = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (h *Handler) UploadMyAvatar(c *gin.Context) {
	if h.storageResolver == nil {
		response.Fail(c, nethttp.StatusServiceUnavailable, "AVATAR_STORAGE_UNAVAILABLE", "Storage ảnh đại diện chưa sẵn sàng.", nil)
		return
	}
	zoneID := strings.TrimSpace(middleware.CurrentZoneID(c))
	userID := strings.TrimSpace(middleware.CurrentUserID(c))
	if !safeAvatarSegment.MatchString(zoneID) || !safeAvatarSegment.MatchString(userID) {
		response.Fail(c, nethttp.StatusBadRequest, "ZONE_REQUIRED", "Không xác định được host hoặc người dùng hiện tại.", nil)
		return
	}

	c.Request.Body = nethttp.MaxBytesReader(c.Writer, c.Request.Body, maxAvatarBytes+(512<<10))
	header, err := c.FormFile("avatar")
	if err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "AVATAR_FILE_REQUIRED", "Hãy chọn ảnh đại diện PNG, JPEG hoặc WebP.", nil)
		return
	}
	if header.Size <= 0 || header.Size > maxAvatarBytes {
		response.Fail(c, nethttp.StatusRequestEntityTooLarge, "AVATAR_TOO_LARGE", "Ảnh đại diện không được vượt quá 8 MB.", nil)
		return
	}
	source, err := header.Open()
	if err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "AVATAR_READ_FAILED", "Không đọc được ảnh đại diện đã chọn.", nil)
		return
	}
	defer source.Close()
	content, err := io.ReadAll(io.LimitReader(source, maxAvatarBytes+1))
	if err != nil || len(content) == 0 {
		response.Fail(c, nethttp.StatusBadRequest, "AVATAR_READ_FAILED", "Không đọc được ảnh đại diện đã chọn.", nil)
		return
	}
	if len(content) > maxAvatarBytes {
		response.Fail(c, nethttp.StatusRequestEntityTooLarge, "AVATAR_TOO_LARGE", "Ảnh đại diện không được vượt quá 8 MB.", nil)
		return
	}
	contentType, extension := avatarContentType(content)
	if extension == "" {
		response.Fail(c, nethttp.StatusUnsupportedMediaType, "AVATAR_TYPE_UNSUPPORTED", "Chỉ hỗ trợ ảnh đại diện PNG, JPEG hoặc WebP.", nil)
		return
	}

	location, err := h.storageResolver.ResolveZone(c.Request.Context(), zoneID)
	if err != nil {
		response.Fail(c, nethttp.StatusServiceUnavailable, "AVATAR_STORAGE_UNAVAILABLE", "Không mở được storage riêng của host.", nil)
		return
	}
	sum := sha256.Sum256(content)
	fileName := "avatar-" + hex.EncodeToString(sum[:8]) + extension
	objectKey := "zones/" + zoneID + "/users/" + userID + "/" + fileName
	object, err := location.Store.Put(c.Request.Context(), platformstorage.PutObjectInput{
		Key: objectKey, Body: bytes.NewReader(content), ContentType: contentType, Size: int64(len(content)),
	})
	if err != nil {
		response.Fail(c, nethttp.StatusBadGateway, "AVATAR_STORE_FAILED", "Không lưu được ảnh đại diện vào storage của host.", nil)
		return
	}
	response.Created(c, gin.H{"avatar": gin.H{
		"content_type": contentType,
		"avatar_path":  "/api/v1/users/avatars/" + zoneID + "/" + userID + "/" + fileName,
		"size":         object.Size,
	}})
}

func (h *Handler) ServeAvatar(c *gin.Context) {
	if h.storageResolver == nil {
		c.Status(nethttp.StatusNotFound)
		return
	}
	zoneID := strings.TrimSpace(c.Param("zone_id"))
	userID := strings.TrimSpace(c.Param("user_id"))
	fileName := strings.TrimSpace(c.Param("file_name"))
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	if !safeAvatarSegment.MatchString(zoneID) || !safeAvatarSegment.MatchString(userID) || !safeAvatarSegment.MatchString(baseName) {
		c.Status(nethttp.StatusNotFound)
		return
	}
	contentType := avatarContentTypeFromExtension(filepath.Ext(fileName))
	if contentType == "" {
		c.Status(nethttp.StatusNotFound)
		return
	}
	location, err := h.storageResolver.ResolveZone(c.Request.Context(), zoneID)
	if err != nil {
		c.Status(nethttp.StatusNotFound)
		return
	}
	object, err := location.Store.Get(c.Request.Context(), "zones/"+zoneID+"/users/"+userID+"/"+fileName)
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

func avatarContentType(content []byte) (string, string) {
	switch nethttp.DetectContentType(content) {
	case "image/png":
		return "image/png", ".png"
	case "image/jpeg":
		return "image/jpeg", ".jpg"
	case "image/webp":
		return "image/webp", ".webp"
	default:
		return "", ""
	}
}

func avatarContentTypeFromExtension(extension string) string {
	switch strings.ToLower(extension) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}
