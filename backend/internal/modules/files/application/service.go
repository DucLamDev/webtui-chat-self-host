package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	filesdomain "github.com/duclamdev/application-chat/backend/internal/modules/files/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const maxUploadSize = 100 << 20

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	CreateFile(ctx context.Context, params CreateFileParams) (filesdomain.File, error)
	FindFile(ctx context.Context, workspaceID string, fileID string) (filesdomain.File, error)
	CanAccessFile(ctx context.Context, workspaceID string, fileID string, userID string) (bool, error)
	ListFiles(ctx context.Context, params ListFilesParams) ([]filesdomain.File, error)
	CreateVersion(ctx context.Context, params CreateVersionParams) (filesdomain.Version, error)
	ListVersions(ctx context.Context, workspaceID string, fileID string) ([]filesdomain.Version, error)
	AttachFile(ctx context.Context, params AttachFileParams) (filesdomain.Attachment, error)
	ListAttachments(ctx context.Context, params ListAttachmentsParams) ([]filesdomain.Attachment, error)
	ListChannelMedia(ctx context.Context, params ListChannelMediaParams) ([]filesdomain.Attachment, error)
	RecordAudit(ctx context.Context, event AuditEvent) error
}

type RealtimePublisher interface {
	Publish(ctx context.Context, event RealtimeEvent) error
}

type RealtimeEvent struct {
	Type        string
	WorkspaceID string
	ChannelID   string
	ActorUserID string
	Payload     map[string]any
}

type AuditEvent struct {
	WorkspaceID string
	ActorUserID string
	Action      string
	FileID      string
	Metadata    map[string]any
}

type ObjectStore interface {
	Put(ctx context.Context, input PutObjectInput) (StoredObject, error)
	Get(ctx context.Context, key string) (StoredObjectReader, error)
	Delete(ctx context.Context, key string) error
}

type PutObjectInput struct {
	Key         string
	Body        io.Reader
	ContentType string
	Size        int64
}

type StoredObject struct {
	Key         string
	ContentType string
	Size        int64
}

type StoredObjectReader struct {
	Info StoredObject
	Body io.ReadCloser
}

type Service struct {
	repo            Repository
	store           ObjectStore
	checker         PermissionChecker
	storageProvider string
	bucket          string
	now             func() time.Time
	realtime        RealtimePublisher
}

type UploadInput struct {
	ActorUserID  string
	WorkspaceID  string
	ChannelID    string
	MessageID    string
	OriginalName string
	MimeType     string
	Size         int64
	SortOrder    int
	Body         io.Reader
	Metadata     json.RawMessage
}

type CreateFileParams struct {
	WorkspaceID     string
	OwnerID         string
	StorageProvider string
	Bucket          string
	ObjectKey       string
	OriginalName    string
	MimeType        string
	ByteSize        int64
	ChecksumSHA256  string
	Metadata        []byte
}

type CreateVersionParams struct {
	WorkspaceID     string
	FileID          string
	CreatedBy       string
	StorageProvider string
	Bucket          string
	ObjectKey       string
	MimeType        string
	ByteSize        int64
	ChecksumSHA256  string
}

type ListFilesInput struct {
	ActorUserID string
	WorkspaceID string
	Limit       int
}

type ListFilesParams struct {
	WorkspaceID string
	ActorUserID string
	Limit       int
}

type DownloadInput struct {
	ActorUserID string
	WorkspaceID string
	FileID      string
}

type DownloadDTO struct {
	File FileDTO
	Body io.ReadCloser
}

type AttachFileInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	MessageID   string
	FileID      string
	SortOrder   int
}

type AttachFileParams struct {
	WorkspaceID string
	ChannelID   string
	MessageID   string
	FileID      string
	ActorUserID string
	SortOrder   int
}

type ListAttachmentsInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	MessageID   string
}

type ListAttachmentsParams struct {
	WorkspaceID string
	ChannelID   string
	MessageID   string
	ActorUserID string
}

type ListChannelMediaInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	Limit       int
}

type ListChannelMediaParams struct {
	WorkspaceID string
	ChannelID   string
	ActorUserID string
	Limit       int
}

type FileDTO struct {
	ID             string          `json:"id"`
	WorkspaceID    *string         `json:"workspace_id,omitempty"`
	OwnerID        *string         `json:"owner_id,omitempty"`
	OriginalName   string          `json:"original_name"`
	MimeType       string          `json:"mime_type"`
	ByteSize       int64           `json:"byte_size"`
	ChecksumSHA256 *string         `json:"checksum_sha256,omitempty"`
	Status         string          `json:"status"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}

type VersionDTO struct {
	ID             string  `json:"id"`
	FileID         string  `json:"file_id"`
	VersionNumber  int     `json:"version_number"`
	MimeType       string  `json:"mime_type"`
	ByteSize       int64   `json:"byte_size"`
	ChecksumSHA256 *string `json:"checksum_sha256,omitempty"`
	CreatedBy      *string `json:"created_by,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

type AttachmentDTO struct {
	WorkspaceID string  `json:"workspace_id"`
	MessageID   string  `json:"message_id"`
	FileID      string  `json:"file_id"`
	SortOrder   int     `json:"sort_order"`
	CreatedAt   string  `json:"created_at"`
	File        FileDTO `json:"file"`
}

func NewService(repo Repository, store ObjectStore, checker PermissionChecker, storageOptions ...string) *Service {
	provider := "local"
	bucket := ""
	if len(storageOptions) > 0 && strings.TrimSpace(storageOptions[0]) != "" {
		provider = strings.ToLower(strings.TrimSpace(storageOptions[0]))
	}
	if len(storageOptions) > 1 {
		bucket = strings.TrimSpace(storageOptions[1])
	}
	return &Service{repo: repo, store: store, checker: checker, storageProvider: provider, bucket: bucket, now: time.Now}
}

func (s *Service) SetRealtimePublisher(publisher RealtimePublisher) {
	s.realtime = publisher
}

func (s *Service) Upload(ctx context.Context, input UploadInput) (FileDTO, error) {
	attachedUpload := strings.TrimSpace(input.ChannelID) != "" && strings.TrimSpace(input.MessageID) != ""
	if attachedUpload {
		if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
			return FileDTO{}, err
		}
	} else {
		if err := s.ensureAnyPermission(ctx, input.ActorUserID, input.WorkspaceID, "file.upload", "message.send"); err != nil {
			return FileDTO{}, err
		}
	}
	originalName, mimeType, metadata, err := validateUpload(input.OriginalName, input.MimeType, input.Size, input.Metadata)
	if err != nil {
		return FileDTO{}, err
	}
	if input.Body == nil {
		return FileDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Nội dung file không được để trống.")
	}

	objectKey, err := newObjectKey(input.WorkspaceID, originalName, "files", s.now)
	if err != nil {
		return FileDTO{}, apperrors.Internal("Không tạo được khóa lưu trữ file.")
	}
	hash := sha256.New()
	reader := io.TeeReader(input.Body, hash)
	object, err := s.store.Put(ctx, PutObjectInput{
		Key:         objectKey,
		Body:        reader,
		ContentType: mimeType,
		Size:        input.Size,
	})
	if err != nil {
		return FileDTO{}, err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))

	file, err := s.repo.CreateFile(ctx, CreateFileParams{
		WorkspaceID:     strings.TrimSpace(input.WorkspaceID),
		OwnerID:         strings.TrimSpace(input.ActorUserID),
		StorageProvider: s.storageProvider,
		Bucket:          s.bucket,
		ObjectKey:       object.Key,
		OriginalName:    originalName,
		MimeType:        mimeType,
		ByteSize:        object.Size,
		ChecksumSHA256:  checksum,
		Metadata:        metadata,
	})
	if err != nil {
		_ = s.store.Delete(ctx, object.Key)
		return FileDTO{}, err
	}
	if attachedUpload {
		attachment, err := s.repo.AttachFile(ctx, AttachFileParams{
			WorkspaceID: strings.TrimSpace(input.WorkspaceID),
			ChannelID:   strings.TrimSpace(input.ChannelID),
			MessageID:   strings.TrimSpace(input.MessageID),
			FileID:      file.ID,
			ActorUserID: strings.TrimSpace(input.ActorUserID),
			SortOrder:   input.SortOrder,
		})
		if err != nil {
			_ = s.store.Delete(ctx, object.Key)
			return FileDTO{}, mapFileError(err)
		}
		s.publishAttachmentCreated(ctx, strings.TrimSpace(input.ActorUserID), strings.TrimSpace(input.ChannelID), attachment)
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		WorkspaceID: fileWorkspaceID(file),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
		Action:      "file.upload",
		FileID:      file.ID,
		Metadata: map[string]any{
			"original_name": file.OriginalName,
			"mime_type":     file.MimeType,
			"byte_size":     file.ByteSize,
		},
	})
	return toFileDTO(file), nil
}

func (s *Service) CreateVersion(ctx context.Context, input UploadInput, fileID string) (VersionDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "file.upload"); err != nil {
		return VersionDTO{}, err
	}
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return VersionDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Mã file không được để trống.")
	}
	originalName, mimeType, _, err := validateUpload(input.OriginalName, input.MimeType, input.Size, nil)
	if err != nil {
		return VersionDTO{}, err
	}
	if input.Body == nil {
		return VersionDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Nội dung file không được để trống.")
	}

	objectKey, err := newObjectKey(input.WorkspaceID, originalName, "versions/"+fileID, s.now)
	if err != nil {
		return VersionDTO{}, apperrors.Internal("Không tạo được khóa lưu trữ file.")
	}
	hash := sha256.New()
	object, err := s.store.Put(ctx, PutObjectInput{
		Key:         objectKey,
		Body:        io.TeeReader(input.Body, hash),
		ContentType: mimeType,
		Size:        input.Size,
	})
	if err != nil {
		return VersionDTO{}, err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))

	version, err := s.repo.CreateVersion(ctx, CreateVersionParams{
		WorkspaceID:     strings.TrimSpace(input.WorkspaceID),
		FileID:          fileID,
		CreatedBy:       strings.TrimSpace(input.ActorUserID),
		StorageProvider: s.storageProvider,
		Bucket:          s.bucket,
		ObjectKey:       object.Key,
		MimeType:        mimeType,
		ByteSize:        object.Size,
		ChecksumSHA256:  checksum,
	})
	if err != nil {
		_ = s.store.Delete(ctx, object.Key)
		return VersionDTO{}, mapFileError(err)
	}
	return toVersionDTO(version), nil
}

func (s *Service) Get(ctx context.Context, actorUserID string, workspaceID string, fileID string) (FileDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID, "message.send"); err != nil {
		return FileDTO{}, err
	}
	if err := s.ensureFileAccess(ctx, actorUserID, workspaceID, fileID); err != nil {
		return FileDTO{}, err
	}
	file, err := s.repo.FindFile(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(fileID))
	if err != nil {
		return FileDTO{}, mapFileError(err)
	}
	return toFileDTO(file), nil
}

func (s *Service) List(ctx context.Context, input ListFilesInput) ([]FileDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	files, err := s.repo.ListFiles(ctx, ListFilesParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}
	return toFileDTOs(files), nil
}

func (s *Service) Download(ctx context.Context, input DownloadInput) (DownloadDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return DownloadDTO{}, err
	}
	if err := s.ensureFileAccess(ctx, input.ActorUserID, input.WorkspaceID, input.FileID); err != nil {
		return DownloadDTO{}, err
	}
	file, err := s.repo.FindFile(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.FileID))
	if err != nil {
		return DownloadDTO{}, mapFileError(err)
	}
	object, err := s.store.Get(ctx, file.ObjectKey)
	if err != nil {
		return DownloadDTO{}, err
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
		Action:      "file.download",
		FileID:      file.ID,
		Metadata: map[string]any{
			"original_name": file.OriginalName,
			"mime_type":     file.MimeType,
			"byte_size":     file.ByteSize,
		},
	})
	return DownloadDTO{File: toFileDTO(file), Body: object.Body}, nil
}

func fileWorkspaceID(file filesdomain.File) string {
	if file.WorkspaceID == nil {
		return ""
	}
	return *file.WorkspaceID
}

func (s *Service) ListVersions(ctx context.Context, actorUserID string, workspaceID string, fileID string) ([]VersionDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID, "message.send"); err != nil {
		return nil, err
	}
	if err := s.ensureFileAccess(ctx, actorUserID, workspaceID, fileID); err != nil {
		return nil, err
	}
	versions, err := s.repo.ListVersions(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(fileID))
	if err != nil {
		return nil, mapFileError(err)
	}
	return toVersionDTOs(versions), nil
}

func (s *Service) AttachFile(ctx context.Context, input AttachFileInput) (AttachmentDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return AttachmentDTO{}, err
	}
	attachment, err := s.repo.AttachFile(ctx, AttachFileParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		MessageID:   strings.TrimSpace(input.MessageID),
		FileID:      strings.TrimSpace(input.FileID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
		SortOrder:   input.SortOrder,
	})
	if err != nil {
		return AttachmentDTO{}, mapFileError(err)
	}
	s.publishAttachmentCreated(ctx, strings.TrimSpace(input.ActorUserID), strings.TrimSpace(input.ChannelID), attachment)
	return toAttachmentDTO(attachment), nil
}

func (s *Service) ListAttachments(ctx context.Context, input ListAttachmentsInput) ([]AttachmentDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return nil, err
	}
	attachments, err := s.repo.ListAttachments(ctx, ListAttachmentsParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		MessageID:   strings.TrimSpace(input.MessageID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return nil, mapFileError(err)
	}
	dtos := make([]AttachmentDTO, 0, len(attachments))
	for _, attachment := range attachments {
		dtos = append(dtos, toAttachmentDTO(attachment))
	}
	return dtos, nil
}

func (s *Service) ListChannelMedia(ctx context.Context, input ListChannelMediaInput) ([]AttachmentDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	attachments, err := s.repo.ListChannelMedia(ctx, ListChannelMediaParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
		Limit:       limit,
	})
	if err != nil {
		return nil, mapFileError(err)
	}
	dtos := make([]AttachmentDTO, 0, len(attachments))
	for _, attachment := range attachments {
		dtos = append(dtos, toAttachmentDTO(attachment))
	}
	return dtos, nil
}

func (s *Service) publishAttachmentCreated(ctx context.Context, actorUserID string, channelID string, attachment filesdomain.Attachment) {
	if s.realtime == nil {
		return
	}
	dto := toAttachmentDTO(attachment)
	if err := s.realtime.Publish(ctx, RealtimeEvent{
		Type:        "AttachmentCreated",
		WorkspaceID: attachment.WorkspaceID,
		ChannelID:   channelID,
		ActorUserID: actorUserID,
		Payload: map[string]any{
			"attachment": dto,
			"channel_id": channelID,
			"message_id": attachment.MessageID,
		},
	}); err != nil {
		slog.Warn("Khong publish duoc su kien attachment realtime",
			"workspace_id", attachment.WorkspaceID,
			"channel_id", channelID,
			"message_id", attachment.MessageID,
			"file_id", attachment.FileID,
			"error", err,
		)
	}
}

func (s *Service) ensurePermission(ctx context.Context, userID string, workspaceID string, permission string) error {
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), permission)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền thực hiện thao tác này.")
	}
	return nil
}

func (s *Service) ensureAnyPermission(ctx context.Context, userID string, workspaceID string, permissions ...string) error {
	for _, permission := range permissions {
		allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), permission)
		if err != nil {
			return err
		}
		if allowed {
			return nil
		}
	}
	return apperrors.Forbidden("Bạn không có quyền thực hiện thao tác này.")
}

func (s *Service) ensureFileAccess(ctx context.Context, userID string, workspaceID string, fileID string) error {
	allowed, err := s.repo.CanAccessFile(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(fileID), strings.TrimSpace(userID))
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền xem file này.")
	}
	return nil
}

func validateUpload(originalName string, mimeType string, size int64, metadataInput json.RawMessage) (string, string, []byte, error) {
	originalName = strings.TrimSpace(originalName)
	if originalName == "" {
		return "", "", nil, apperrors.BadRequest("VALIDATION_ERROR", "Tên file không được để trống.")
	}
	if len([]rune(originalName)) > 255 {
		return "", "", nil, apperrors.BadRequest("VALIDATION_ERROR", "Tên file không được dài quá 255 ký tự.")
	}
	if size <= 0 || size > maxUploadSize {
		return "", "", nil, apperrors.BadRequest("VALIDATION_ERROR", "Dung lượng file phải lớn hơn 0 và không vượt quá 100MB.")
	}
	mimeType = strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	if !isAllowedMimeType(mimeType) {
		return "", "", nil, apperrors.BadRequest("VALIDATION_ERROR", "Định dạng file không được hỗ trợ.")
	}
	metadata, err := normalizeMetadata(metadataInput)
	if err != nil {
		return "", "", nil, err
	}
	return originalName, mimeType, metadata, nil
}

func normalizeMetadata(value json.RawMessage) ([]byte, error) {
	if len(value) == 0 || strings.TrimSpace(string(value)) == "" || strings.TrimSpace(string(value)) == "null" {
		return []byte(`{}`), nil
	}
	if !json.Valid(value) {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", "Metadata của file không phải JSON hợp lệ.")
	}
	return []byte(value), nil
}

func isAllowedMimeType(mimeType string) bool {
	if strings.HasPrefix(mimeType, "image/") || strings.HasPrefix(mimeType, "text/") {
		return true
	}
	switch mimeType {
	case "audio/webm",
		"audio/ogg",
		"audio/mp4",
		"audio/mpeg",
		"audio/wav",
		"audio/x-m4a",
		"application/ogg",
		"application/pdf",
		"application/json",
		"application/zip",
		"application/octet-stream",
		"application/msword",
		"application/vnd.ms-excel",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return true
	default:
		return false
	}
}

func newObjectKey(workspaceID string, originalName string, prefix string, now func() time.Time) (string, error) {
	random, err := randomHex(16)
	if err != nil {
		return "", err
	}
	date := now().UTC().Format("2006/01")
	return "workspaces/" + strings.TrimSpace(workspaceID) + "/" + strings.Trim(prefix, "/") + "/" + date + "/" + random + "-" + sanitizeFilename(originalName), nil
}

func randomHex(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func sanitizeFilename(value string) string {
	base := filepath.Base(strings.TrimSpace(value))
	base = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			return r
		case r == '.', r == '-', r == '_':
			return r
		case unicode.IsSpace(r):
			return '_'
		default:
			return '_'
		}
	}, base)
	base = strings.Trim(base, "._-")
	if base == "" {
		return "file"
	}
	return base
}

func mapFileError(err error) error {
	if errors.Is(err, filesdomain.ErrFileNotFound) {
		return apperrors.NotFound("FILE_NOT_FOUND", "Không tìm thấy file.")
	}
	if errors.Is(err, filesdomain.ErrMessageNotFound) {
		return apperrors.NotFound("MESSAGE_NOT_FOUND", "Không tìm thấy tin nhắn.")
	}
	if errors.Is(err, filesdomain.ErrAttachmentNotFound) {
		return apperrors.NotFound("ATTACHMENT_NOT_FOUND", "Không tìm thấy attachment.")
	}
	return err
}

func toFileDTOs(files []filesdomain.File) []FileDTO {
	dtos := make([]FileDTO, 0, len(files))
	for _, file := range files {
		dtos = append(dtos, toFileDTO(file))
	}
	return dtos
}

func toFileDTO(file filesdomain.File) FileDTO {
	metadata := json.RawMessage(file.Metadata)
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	return FileDTO{
		ID:             file.ID,
		WorkspaceID:    file.WorkspaceID,
		OwnerID:        file.OwnerID,
		OriginalName:   file.OriginalName,
		MimeType:       file.MimeType,
		ByteSize:       file.ByteSize,
		ChecksumSHA256: file.ChecksumSHA256,
		Status:         file.Status,
		Metadata:       metadata,
		CreatedAt:      formatTime(file.CreatedAt),
		UpdatedAt:      formatTime(file.UpdatedAt),
	}
}

func toVersionDTOs(versions []filesdomain.Version) []VersionDTO {
	dtos := make([]VersionDTO, 0, len(versions))
	for _, version := range versions {
		dtos = append(dtos, toVersionDTO(version))
	}
	return dtos
}

func toVersionDTO(version filesdomain.Version) VersionDTO {
	return VersionDTO{
		ID:             version.ID,
		FileID:         version.FileID,
		VersionNumber:  version.VersionNumber,
		MimeType:       version.MimeType,
		ByteSize:       version.ByteSize,
		ChecksumSHA256: version.ChecksumSHA256,
		CreatedBy:      version.CreatedBy,
		CreatedAt:      formatTime(version.CreatedAt),
	}
}

func toAttachmentDTO(attachment filesdomain.Attachment) AttachmentDTO {
	return AttachmentDTO{
		WorkspaceID: attachment.WorkspaceID,
		MessageID:   attachment.MessageID,
		FileID:      attachment.FileID,
		SortOrder:   attachment.SortOrder,
		CreatedAt:   formatTime(attachment.CreatedAt),
		File:        toFileDTO(attachment.File),
	}
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}
