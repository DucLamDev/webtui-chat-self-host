package http

import (
	"encoding/json"
	"mime"
	nethttp "net/http"
	"strconv"

	filesapp "github.com/duclamdev/application-chat/backend/internal/modules/files/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *filesapp.Service
}

type attachFileRequest struct {
	FileID    string `json:"file_id"`
	SortOrder int    `json:"sort_order"`
}

func NewHandler(service *filesapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id")
	private.Use(authMiddleware)

	private.GET("/files", h.List)
	private.POST("/files", h.Upload)
	private.GET("/files/:file_id", h.Get)
	private.GET("/files/:file_id/download", h.Download)
	private.GET("/files/:file_id/versions", h.ListVersions)
	private.POST("/files/:file_id/versions", h.CreateVersion)
	private.GET("/channels/:channel_id/messages/:message_id/attachments", h.ListAttachments)
	private.POST("/channels/:channel_id/messages/:message_id/attachments", h.AttachFile)
	private.GET("/channels/:channel_id/media", h.ListChannelMedia)
}

func (h *Handler) Upload(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "VALIDATION_ERROR", "Bạn cần gửi file trong field multipart tên file.", nil)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "VALIDATION_ERROR", "Không mở được file upload.", nil)
		return
	}
	defer file.Close()

	dto, err := h.service.Upload(c.Request.Context(), filesapp.UploadInput{
		ActorUserID:  middleware.CurrentUserID(c),
		WorkspaceID:  c.Param("workspace_id"),
		ChannelID:    c.PostForm("channel_id"),
		MessageID:    c.PostForm("message_id"),
		OriginalName: fileHeader.Filename,
		MimeType:     contentType(fileHeader.Header.Get("Content-Type")),
		Size:         fileHeader.Size,
		SortOrder:    formInt(c, "sort_order"),
		Body:         file,
		Metadata:     metadataFromForm(c),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, dto)
}

func (h *Handler) CreateVersion(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "VALIDATION_ERROR", "Bạn cần gửi file trong field multipart tên file.", nil)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "VALIDATION_ERROR", "Không mở được file upload.", nil)
		return
	}
	defer file.Close()

	version, err := h.service.CreateVersion(c.Request.Context(), filesapp.UploadInput{
		ActorUserID:  middleware.CurrentUserID(c),
		WorkspaceID:  c.Param("workspace_id"),
		OriginalName: fileHeader.Filename,
		MimeType:     contentType(fileHeader.Header.Get("Content-Type")),
		Size:         fileHeader.Size,
		Body:         file,
	}, c.Param("file_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, version)
}

func (h *Handler) List(c *gin.Context) {
	files, err := h.service.List(c.Request.Context(), filesapp.ListFilesInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		Limit:       queryInt(c, "limit"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"files": files})
}

func (h *Handler) Get(c *gin.Context) {
	file, err := h.service.Get(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("file_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, file)
}

func (h *Handler) Download(c *gin.Context) {
	download, err := h.service.Download(c.Request.Context(), filesapp.DownloadInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		FileID:      c.Param("file_id"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	defer download.Body.Close()

	headers := map[string]string{
		"Cache-Control": "private, max-age=3600",
		"Content-Disposition": mime.FormatMediaType("attachment", map[string]string{
			"filename": download.File.OriginalName,
		}),
	}
	if download.File.ChecksumSHA256 != nil && *download.File.ChecksumSHA256 != "" {
		headers["ETag"] = `"` + *download.File.ChecksumSHA256 + `"`
	}
	c.DataFromReader(nethttp.StatusOK, download.File.ByteSize, download.File.MimeType, download.Body, headers)
}

func (h *Handler) ListVersions(c *gin.Context) {
	versions, err := h.service.ListVersions(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("file_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"versions": versions})
}

func (h *Handler) AttachFile(c *gin.Context) {
	var req attachFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	attachment, err := h.service.AttachFile(c.Request.Context(), filesapp.AttachFileInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		MessageID:   c.Param("message_id"),
		FileID:      req.FileID,
		SortOrder:   req.SortOrder,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, attachment)
}

func (h *Handler) ListAttachments(c *gin.Context) {
	attachments, err := h.service.ListAttachments(c.Request.Context(), filesapp.ListAttachmentsInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		MessageID:   c.Param("message_id"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"attachments": attachments})
}

func (h *Handler) ListChannelMedia(c *gin.Context) {
	attachments, err := h.service.ListChannelMedia(c.Request.Context(), filesapp.ListChannelMediaInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		Limit:       queryInt(c, "limit"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"attachments": attachments})
}

func contentType(value string) string {
	if value == "" {
		return "application/octet-stream"
	}
	return value
}

func metadataFromForm(c *gin.Context) json.RawMessage {
	value := c.PostForm("metadata")
	if value == "" {
		return nil
	}
	return json.RawMessage(value)
}

func queryInt(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return value
}

func formInt(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.PostForm(key))
	if err != nil {
		return 0
	}
	return value
}
