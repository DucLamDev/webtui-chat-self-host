package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"time"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const (
	defaultUploadChunkSize = 5 << 20
	minUploadChunkSize     = 256 << 10
	maxUploadChunkSize     = 16 << 20
	uploadSessionLifetime  = 24 * time.Hour
)

type ResumableRepository interface {
	CreateUploadSession(ctx context.Context, params CreateUploadSessionParams) (UploadSession, error)
	GetUploadSession(ctx context.Context, workspaceID string, uploadID string, ownerID string) (UploadSession, error)
	UpsertUploadPart(ctx context.Context, params UpsertUploadPartParams) (UploadSession, error)
	ListUploadParts(ctx context.Context, workspaceID string, uploadID string, ownerID string) ([]UploadPart, error)
	MarkUploadCompleting(ctx context.Context, workspaceID string, uploadID string, ownerID string) (UploadSession, error)
	FailUploadSession(ctx context.Context, workspaceID string, uploadID string, ownerID string) error
	CompleteUploadSession(ctx context.Context, params CompleteUploadSessionParams) (UploadSession, error)
	CancelUploadSession(ctx context.Context, workspaceID string, uploadID string, ownerID string) error
}

type UploadSession struct {
	ID             string
	WorkspaceID    string
	OwnerID        string
	ChannelID      *string
	MessageID      *string
	OriginalName   string
	MimeType       string
	TotalSize      int64
	ChunkSize      int
	TotalChunks    int
	ReceivedBytes  int64
	Metadata       []byte
	Status         string
	FileID         *string
	ChecksumSHA256 *string
	ExpiresAt      time.Time
	CompletedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UploadPart struct {
	UploadID       string
	PartNumber     int
	ObjectKey      string
	ByteSize       int
	ChecksumSHA256 string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateUploadSessionInput struct {
	ActorUserID  string
	WorkspaceID  string
	ChannelID    string
	MessageID    string
	OriginalName string
	MimeType     string
	TotalSize    int64
	ChunkSize    int
	Metadata     json.RawMessage
}

type CreateUploadSessionParams struct {
	WorkspaceID  string
	OwnerID      string
	ChannelID    string
	MessageID    string
	OriginalName string
	MimeType     string
	TotalSize    int64
	ChunkSize    int
	TotalChunks  int
	Metadata     []byte
	ExpiresAt    time.Time
}

type UpsertUploadPartParams struct {
	WorkspaceID    string
	UploadID       string
	OwnerID        string
	PartNumber     int
	ObjectKey      string
	ByteSize       int
	ChecksumSHA256 string
}

type CompleteUploadSessionParams struct {
	WorkspaceID    string
	UploadID       string
	OwnerID        string
	FileID         string
	ChecksumSHA256 string
}

type UploadPartInput struct {
	ActorUserID    string
	WorkspaceID    string
	UploadID       string
	PartNumber     int
	Size           int64
	ChecksumSHA256 string
	Body           io.Reader
}

type CompleteUploadInput struct {
	ActorUserID    string
	WorkspaceID    string
	UploadID       string
	ChecksumSHA256 string
}

type UploadPartDTO struct {
	PartNumber     int    `json:"part_number"`
	ByteSize       int    `json:"byte_size"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	CreatedAt      string `json:"created_at"`
}

type UploadSessionDTO struct {
	ID             string          `json:"id"`
	WorkspaceID    string          `json:"workspace_id"`
	OwnerID        string          `json:"owner_id"`
	ChannelID      *string         `json:"channel_id,omitempty"`
	MessageID      *string         `json:"message_id,omitempty"`
	OriginalName   string          `json:"original_name"`
	MimeType       string          `json:"mime_type"`
	TotalSize      int64           `json:"total_size"`
	ChunkSize      int             `json:"chunk_size"`
	TotalChunks    int             `json:"total_chunks"`
	ReceivedBytes  int64           `json:"received_bytes"`
	UploadedParts  []UploadPartDTO `json:"uploaded_parts"`
	Status         string          `json:"status"`
	FileID         *string         `json:"file_id,omitempty"`
	ChecksumSHA256 *string         `json:"checksum_sha256,omitempty"`
	ExpiresAt      string          `json:"expires_at"`
	CompletedAt    *string         `json:"completed_at,omitempty"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}

func (s *Service) CreateUploadSession(ctx context.Context, input CreateUploadSessionInput) (UploadSessionDTO, error) {
	attached := strings.TrimSpace(input.ChannelID) != ""
	if attached {
		if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
			return UploadSessionDTO{}, err
		}
		if err := s.ensureDirectInteractionAllowed(ctx, input.WorkspaceID, input.ChannelID, input.ActorUserID); err != nil {
			return UploadSessionDTO{}, err
		}
	} else if err := s.ensureAnyPermission(ctx, input.ActorUserID, input.WorkspaceID, "file.upload", "message.send"); err != nil {
		return UploadSessionDTO{}, err
	}
	originalName, mimeType, metadata, err := validateUploadDescriptor(
		input.OriginalName, input.MimeType, input.TotalSize, input.Metadata, maxResumableUploadSize,
	)
	if err != nil {
		return UploadSessionDTO{}, err
	}
	chunkSize := input.ChunkSize
	if chunkSize == 0 {
		chunkSize = defaultUploadChunkSize
	}
	if chunkSize < minUploadChunkSize || chunkSize > maxUploadChunkSize {
		return UploadSessionDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "chunk_size phải từ 256 KiB đến 16 MiB.")
	}
	totalChunks := int(math.Ceil(float64(input.TotalSize) / float64(chunkSize)))
	session, err := s.resumableRepository().CreateUploadSession(ctx, CreateUploadSessionParams{
		WorkspaceID:  strings.TrimSpace(input.WorkspaceID),
		OwnerID:      strings.TrimSpace(input.ActorUserID),
		ChannelID:    strings.TrimSpace(input.ChannelID),
		MessageID:    strings.TrimSpace(input.MessageID),
		OriginalName: originalName,
		MimeType:     mimeType,
		TotalSize:    input.TotalSize,
		ChunkSize:    chunkSize,
		TotalChunks:  totalChunks,
		Metadata:     metadata,
		ExpiresAt:    time.Now().UTC().Add(uploadSessionLifetime),
	})
	if err != nil {
		return UploadSessionDTO{}, err
	}
	return toUploadSessionDTO(session, nil), nil
}

func (s *Service) GetUploadSession(ctx context.Context, actorUserID string, workspaceID string, uploadID string) (UploadSessionDTO, error) {
	session, err := s.resumableRepository().GetUploadSession(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(uploadID), strings.TrimSpace(actorUserID))
	if err != nil {
		return UploadSessionDTO{}, mapFileError(err)
	}
	parts, err := s.resumableRepository().ListUploadParts(ctx, session.WorkspaceID, session.ID, session.OwnerID)
	if err != nil {
		return UploadSessionDTO{}, err
	}
	return toUploadSessionDTO(session, parts), nil
}

func (s *Service) UploadPart(ctx context.Context, input UploadPartInput) (UploadSessionDTO, error) {
	if input.Body == nil || input.Size <= 0 {
		return UploadSessionDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Chunk upload không có dữ liệu.")
	}
	session, err := s.resumableRepository().GetUploadSession(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.UploadID), strings.TrimSpace(input.ActorUserID))
	if err != nil {
		return UploadSessionDTO{}, mapFileError(err)
	}
	storageLocation, err := s.storageForWorkspace(ctx, session.WorkspaceID)
	if err != nil {
		return UploadSessionDTO{}, err
	}
	if session.Status != "uploading" || session.ExpiresAt.Before(time.Now().UTC()) {
		return UploadSessionDTO{}, apperrors.Conflict("UPLOAD_NOT_ACTIVE", "Phiên upload không còn hoạt động.")
	}
	if input.PartNumber < 0 || input.PartNumber >= session.TotalChunks {
		return UploadSessionDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "part_number nằm ngoài phạm vi.")
	}
	expectedSize := int64(session.ChunkSize)
	if input.PartNumber == session.TotalChunks-1 {
		expectedSize = session.TotalSize - int64(session.ChunkSize*(session.TotalChunks-1))
	}
	if input.Size != expectedSize {
		return UploadSessionDTO{}, apperrors.BadRequest("INVALID_CHUNK_SIZE", "Dung lượng chunk không đúng với phiên upload.")
	}
	objectKey := "uploads/" + session.WorkspaceID + "/" + session.ID + "/parts/" + paddedPartNumber(input.PartNumber)
	hash := sha256.New()
	object, err := storageLocation.Store.Put(ctx, PutObjectInput{
		Key: objectKey, Body: io.TeeReader(input.Body, hash),
		ContentType: "application/octet-stream", Size: input.Size,
	})
	if err != nil {
		return UploadSessionDTO{}, err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	expectedChecksum := strings.ToLower(strings.TrimSpace(input.ChecksumSHA256))
	if expectedChecksum != "" && expectedChecksum != checksum {
		_ = storageLocation.Store.Delete(ctx, object.Key)
		return UploadSessionDTO{}, apperrors.BadRequest("CHECKSUM_MISMATCH", "Checksum của chunk không khớp.")
	}
	session, err = s.resumableRepository().UpsertUploadPart(ctx, UpsertUploadPartParams{
		WorkspaceID: session.WorkspaceID, UploadID: session.ID, OwnerID: session.OwnerID,
		PartNumber: input.PartNumber, ObjectKey: object.Key, ByteSize: int(object.Size),
		ChecksumSHA256: checksum,
	})
	if err != nil {
		_ = storageLocation.Store.Delete(ctx, object.Key)
		return UploadSessionDTO{}, err
	}
	parts, err := s.resumableRepository().ListUploadParts(ctx, session.WorkspaceID, session.ID, session.OwnerID)
	if err != nil {
		return UploadSessionDTO{}, err
	}
	return toUploadSessionDTO(session, parts), nil
}

func (s *Service) CompleteUpload(ctx context.Context, input CompleteUploadInput) (FileDTO, error) {
	session, err := s.resumableRepository().MarkUploadCompleting(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.UploadID), strings.TrimSpace(input.ActorUserID))
	if err != nil {
		return FileDTO{}, mapFileError(err)
	}
	failSession := func() {
		_ = s.resumableRepository().FailUploadSession(ctx, session.WorkspaceID, session.ID, session.OwnerID)
	}
	if session.ChannelID != nil {
		if err := s.ensureDirectInteractionAllowed(ctx, session.WorkspaceID, *session.ChannelID, session.OwnerID); err != nil {
			failSession()
			return FileDTO{}, err
		}
	}
	storageLocation, err := s.storageForWorkspace(ctx, session.WorkspaceID)
	if err != nil {
		failSession()
		return FileDTO{}, err
	}
	parts, err := s.resumableRepository().ListUploadParts(ctx, session.WorkspaceID, session.ID, session.OwnerID)
	if err != nil {
		failSession()
		return FileDTO{}, err
	}
	if len(parts) != session.TotalChunks {
		return FileDTO{}, apperrors.Conflict("UPLOAD_INCOMPLETE", "Chưa upload đủ tất cả chunk.")
	}
	objectKey, err := newObjectKey(session.WorkspaceID, session.OriginalName, "files", s.now)
	if err != nil {
		return FileDTO{}, apperrors.Internal("Không tạo được khóa lưu trữ file.")
	}
	reader := &uploadPartSequenceReader{ctx: ctx, store: storageLocation.Store, parts: parts}
	hash := sha256.New()
	object, err := storageLocation.Store.Put(ctx, PutObjectInput{
		Key: objectKey, Body: io.TeeReader(reader, hash),
		ContentType: session.MimeType, Size: session.TotalSize,
	})
	closeErr := reader.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		failSession()
		return FileDTO{}, err
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	expectedChecksum := strings.ToLower(strings.TrimSpace(input.ChecksumSHA256))
	if expectedChecksum != "" && expectedChecksum != checksum {
		_ = storageLocation.Store.Delete(ctx, object.Key)
		failSession()
		return FileDTO{}, apperrors.BadRequest("CHECKSUM_MISMATCH", "Checksum của file hoàn chỉnh không khớp.")
	}
	file, err := s.repo.CreateFile(ctx, CreateFileParams{
		WorkspaceID: session.WorkspaceID, OwnerID: session.OwnerID,
		StorageProvider: storageLocation.Provider, Bucket: storageLocation.Bucket, ObjectKey: object.Key,
		OriginalName: session.OriginalName, MimeType: session.MimeType,
		ByteSize: object.Size, ChecksumSHA256: checksum, Metadata: session.Metadata,
	})
	if err != nil {
		_ = storageLocation.Store.Delete(ctx, object.Key)
		failSession()
		return FileDTO{}, err
	}
	if session.ChannelID != nil && session.MessageID != nil {
		attachment, attachErr := s.repo.AttachFile(ctx, AttachFileParams{
			WorkspaceID: session.WorkspaceID, ChannelID: *session.ChannelID,
			MessageID: *session.MessageID, FileID: file.ID, ActorUserID: session.OwnerID,
		})
		if attachErr != nil {
			failSession()
			return FileDTO{}, mapFileError(attachErr)
		}
		s.publishAttachmentCreated(ctx, session.OwnerID, *session.ChannelID, attachment)
	}
	if _, err := s.resumableRepository().CompleteUploadSession(ctx, CompleteUploadSessionParams{
		WorkspaceID: session.WorkspaceID, UploadID: session.ID, OwnerID: session.OwnerID,
		FileID: file.ID, ChecksumSHA256: checksum,
	}); err != nil {
		failSession()
		return FileDTO{}, err
	}
	for _, part := range parts {
		_ = storageLocation.Store.Delete(ctx, part.ObjectKey)
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		WorkspaceID: session.WorkspaceID, ActorUserID: session.OwnerID,
		Action: "file.upload.resumable", FileID: file.ID,
		Metadata: map[string]any{"upload_id": session.ID, "parts": len(parts), "byte_size": file.ByteSize},
	})
	return toFileDTO(file), nil
}

func (s *Service) CancelUpload(ctx context.Context, actorUserID string, workspaceID string, uploadID string) error {
	session, err := s.resumableRepository().GetUploadSession(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(uploadID), strings.TrimSpace(actorUserID))
	if err != nil {
		return mapFileError(err)
	}
	storageLocation, err := s.storageForWorkspace(ctx, session.WorkspaceID)
	if err != nil {
		return err
	}
	parts, err := s.resumableRepository().ListUploadParts(ctx, session.WorkspaceID, session.ID, session.OwnerID)
	if err != nil {
		return err
	}
	if err := s.resumableRepository().CancelUploadSession(ctx, session.WorkspaceID, session.ID, session.OwnerID); err != nil {
		return err
	}
	for _, part := range parts {
		_ = storageLocation.Store.Delete(ctx, part.ObjectKey)
	}
	return nil
}

func validateUploadDescriptor(originalName string, mimeType string, size int64, metadataInput json.RawMessage, maxSize int64) (string, string, []byte, error) {
	originalName = strings.TrimSpace(originalName)
	if originalName == "" || len([]rune(originalName)) > 255 {
		return "", "", nil, apperrors.BadRequest("VALIDATION_ERROR", "Tên file không hợp lệ.")
	}
	if size <= 0 || size > maxSize {
		return "", "", nil, apperrors.BadRequest("VALIDATION_ERROR", "Dung lượng file vượt giới hạn phiên upload.")
	}
	mimeType = strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	if !isAllowedMimeType(mimeType) {
		return "", "", nil, apperrors.BadRequest("VALIDATION_ERROR", "Định dạng file không được hỗ trợ.")
	}
	metadata, err := normalizeMetadata(metadataInput)
	return originalName, mimeType, metadata, err
}

func paddedPartNumber(partNumber int) string {
	const digits = "00000000"
	value := strconvItoa(partNumber)
	if len(value) >= len(digits) {
		return value
	}
	return digits[:len(digits)-len(value)] + value
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	var buffer [20]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[index:])
}

type uploadPartSequenceReader struct {
	ctx     context.Context
	store   ObjectStore
	parts   []UploadPart
	index   int
	current io.ReadCloser
}

func (r *uploadPartSequenceReader) Read(buffer []byte) (int, error) {
	for {
		if r.current == nil {
			if r.index >= len(r.parts) {
				return 0, io.EOF
			}
			object, err := r.store.Get(r.ctx, r.parts[r.index].ObjectKey)
			if err != nil {
				return 0, err
			}
			r.current = object.Body
		}
		count, err := r.current.Read(buffer)
		if errors.Is(err, io.EOF) {
			_ = r.current.Close()
			r.current = nil
			r.index++
			if count > 0 {
				return count, nil
			}
			continue
		}
		return count, err
	}
}

func (r *uploadPartSequenceReader) Close() error {
	if r.current == nil {
		return nil
	}
	err := r.current.Close()
	r.current = nil
	return err
}

func toUploadSessionDTO(session UploadSession, parts []UploadPart) UploadSessionDTO {
	partDTOs := make([]UploadPartDTO, 0, len(parts))
	for _, part := range parts {
		partDTOs = append(partDTOs, UploadPartDTO{
			PartNumber: part.PartNumber, ByteSize: part.ByteSize,
			ChecksumSHA256: part.ChecksumSHA256,
			CreatedAt:      part.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	return UploadSessionDTO{
		ID: session.ID, WorkspaceID: session.WorkspaceID, OwnerID: session.OwnerID,
		ChannelID: session.ChannelID, MessageID: session.MessageID,
		OriginalName: session.OriginalName, MimeType: session.MimeType,
		TotalSize: session.TotalSize, ChunkSize: session.ChunkSize,
		TotalChunks: session.TotalChunks, ReceivedBytes: session.ReceivedBytes,
		UploadedParts: partDTOs, Status: session.Status, FileID: session.FileID,
		ChecksumSHA256: session.ChecksumSHA256,
		ExpiresAt:      session.ExpiresAt.UTC().Format(time.RFC3339Nano),
		CompletedAt:    formatOptionalUploadTime(session.CompletedAt),
		CreatedAt:      session.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:      session.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func (s *Service) resumableRepository() ResumableRepository {
	repository, ok := s.repo.(ResumableRepository)
	if !ok {
		return unavailableResumableRepository{}
	}
	return repository
}

func formatOptionalUploadTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339Nano)
	return &formatted
}
