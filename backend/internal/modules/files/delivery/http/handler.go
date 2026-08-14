package http

import (
	"encoding/json"
	"errors"
	"mime"
	nethttp "net/http"
	"strconv"
	"strings"

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

type createUploadSessionRequest struct {
	ChannelID    string          `json:"channel_id"`
	MessageID    string          `json:"message_id"`
	OriginalName string          `json:"original_name"`
	MimeType     string          `json:"mime_type"`
	TotalSize    int64           `json:"total_size"`
	ChunkSize    int             `json:"chunk_size"`
	Metadata     json.RawMessage `json:"metadata"`
}

type completeUploadRequest struct {
	ChecksumSHA256 string `json:"checksum_sha256"`
}

func NewHandler(service *filesapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc, legalMiddleware ...gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id")
	private.Use(authMiddleware)
	ugc := router.Group("/workspaces/:workspace_id")
	ugc.Use(authMiddleware)
	if len(legalMiddleware) > 0 && legalMiddleware[0] != nil {
		ugc.Use(legalMiddleware[0])
	}

	private.GET("/files", h.List)
	ugc.POST("/files", h.Upload)
	ugc.POST("/files/uploads", h.CreateUploadSession)
	private.GET("/files/uploads/:upload_id", h.GetUploadSession)
	ugc.PUT("/files/uploads/:upload_id/parts/:part_number", h.UploadPart)
	ugc.POST("/files/uploads/:upload_id/complete", h.CompleteUpload)
	private.DELETE("/files/uploads/:upload_id", h.CancelUpload)
	private.GET("/files/:file_id", h.Get)
	private.GET("/files/:file_id/download", h.Download)
	private.GET("/files/:file_id/versions", h.ListVersions)
	ugc.POST("/files/:file_id/versions", h.CreateVersion)
	private.GET("/channels/:channel_id/messages/:message_id/attachments", h.ListAttachments)
	ugc.POST("/channels/:channel_id/messages/:message_id/attachments", h.AttachFile)
	private.GET("/channels/:channel_id/media", h.ListChannelMedia)
}

func (h *Handler) CreateUploadSession(c *gin.Context) {
	var req createUploadSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	session, err := h.service.CreateUploadSession(c.Request.Context(), filesapp.CreateUploadSessionInput{
		ActorUserID:  middleware.CurrentUserID(c),
		WorkspaceID:  c.Param("workspace_id"),
		ChannelID:    req.ChannelID,
		MessageID:    req.MessageID,
		OriginalName: req.OriginalName,
		MimeType:     req.MimeType,
		TotalSize:    req.TotalSize,
		ChunkSize:    req.ChunkSize,
		Metadata:     req.Metadata,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, session)
}

func (h *Handler) GetUploadSession(c *gin.Context) {
	session, err := h.service.GetUploadSession(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("upload_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, session)
}

func (h *Handler) UploadPart(c *gin.Context) {
	partNumber, err := strconv.Atoi(c.Param("part_number"))
	if err != nil || partNumber < 0 {
		response.Fail(c, nethttp.StatusBadRequest, "VALIDATION_ERROR", "part_number không hợp lệ.", nil)
		return
	}
	if c.Request.ContentLength <= 0 {
		response.Fail(c, nethttp.StatusBadRequest, "VALIDATION_ERROR", "Content-Length của chunk là bắt buộc.", nil)
		return
	}
	session, err := h.service.UploadPart(c.Request.Context(), filesapp.UploadPartInput{
		ActorUserID:    middleware.CurrentUserID(c),
		WorkspaceID:    c.Param("workspace_id"),
		UploadID:       c.Param("upload_id"),
		PartNumber:     partNumber,
		Size:           c.Request.ContentLength,
		ChecksumSHA256: c.GetHeader("X-Chunk-SHA256"),
		Body:           c.Request.Body,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, session)
}

func (h *Handler) CompleteUpload(c *gin.Context) {
	var req completeUploadRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
			return
		}
	}
	file, err := h.service.CompleteUpload(c.Request.Context(), filesapp.CompleteUploadInput{
		ActorUserID:    middleware.CurrentUserID(c),
		WorkspaceID:    c.Param("workspace_id"),
		UploadID:       c.Param("upload_id"),
		ChecksumSHA256: firstNonEmpty(req.ChecksumSHA256, c.GetHeader("X-File-SHA256")),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, file)
}

func (h *Handler) CancelUpload(c *gin.Context) {
	if err := h.service.CancelUpload(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("upload_id"),
	); err != nil {
		response.Error(c, err)
		return
	}
	c.Status(nethttp.StatusNoContent)
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
	requestedRange, err := parseRangeHeader(c.GetHeader("Range"))
	if err != nil {
		c.Header("Content-Range", "bytes */*")
		c.Status(nethttp.StatusRequestedRangeNotSatisfiable)
		return
	}
	download, err := h.service.Download(c.Request.Context(), filesapp.DownloadInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		FileID:      c.Param("file_id"),
		Range:       requestedRange,
	})
	if err != nil {
		var invalidRange filesapp.InvalidRangeError
		if errors.As(err, &invalidRange) {
			c.Header("Content-Range", "bytes */"+strconv.FormatInt(invalidRange.Size, 10))
			c.Status(nethttp.StatusRequestedRangeNotSatisfiable)
			return
		}
		response.Error(c, err)
		return
	}
	defer download.Body.Close()

	disposition := "attachment"
	if isInlineMedia(download.File.MimeType) {
		disposition = "inline"
	}
	headers := map[string]string{
		"Accept-Ranges": "bytes",
		"Cache-Control": "private, no-cache",
		"Content-Disposition": mime.FormatMediaType(disposition, map[string]string{
			"filename": download.File.OriginalName,
		}),
	}
	if download.File.ChecksumSHA256 != nil && *download.File.ChecksumSHA256 != "" {
		headers["ETag"] = `"` + *download.File.ChecksumSHA256 + `"`
	}
	status := nethttp.StatusOK
	if download.Partial {
		status = nethttp.StatusPartialContent
		headers["Content-Range"] = "bytes " +
			strconv.FormatInt(download.RangeStart, 10) + "-" +
			strconv.FormatInt(download.RangeEnd, 10) + "/" +
			strconv.FormatInt(download.File.ByteSize, 10)
	}
	c.DataFromReader(status, download.ContentLength, download.File.MimeType, download.Body, headers)
}

func parseRangeHeader(value string) (*filesapp.DownloadRange, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if !strings.HasPrefix(value, "bytes=") || strings.Contains(value, ",") {
		return nil, errors.New("invalid range")
	}
	parts := strings.SplitN(strings.TrimPrefix(value, "bytes="), "-", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid range")
	}
	if parts[0] == "" {
		suffix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffix <= 0 {
			return nil, errors.New("invalid suffix range")
		}
		return &filesapp.DownloadRange{SuffixLength: &suffix}, nil
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 {
		return nil, errors.New("invalid range start")
	}
	result := &filesapp.DownloadRange{Start: &start}
	if parts[1] != "" {
		end, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start {
			return nil, errors.New("invalid range end")
		}
		result.End = &end
	}
	return result, nil
}

func isInlineMedia(mimeType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(mimeType))
	return strings.HasPrefix(normalized, "image/") ||
		strings.HasPrefix(normalized, "audio/") ||
		strings.HasPrefix(normalized, "video/")
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
