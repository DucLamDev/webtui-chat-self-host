package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	filesapp "github.com/duclamdev/application-chat/backend/internal/modules/files/application"
	filesdomain "github.com/duclamdev/application-chat/backend/internal/modules/files/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

const attachFileQuery = `
WITH target_message AS (
    SELECT m.workspace_id, m.id AS message_id, f.id AS file_id
    FROM messages m
    JOIN channel_members cm
      ON cm.channel_id = m.channel_id
     AND cm.user_id = $5::uuid
     AND cm.status IN ('active', 'muted')
    JOIN files f
      ON f.id = $4::uuid
     AND f.workspace_id = m.workspace_id
     AND f.deleted_at IS NULL
    WHERE m.workspace_id = $1::uuid
      AND m.channel_id = $2::uuid
      AND m.id = $3::uuid
      AND m.deleted_at IS NULL
), attached AS (
    INSERT INTO message_attachments (workspace_id, message_id, file_id, sort_order)
    SELECT workspace_id, message_id, file_id, $6
    FROM target_message
    ON CONFLICT (workspace_id, message_id, file_id)
    DO UPDATE SET sort_order = EXCLUDED.sort_order
    RETURNING workspace_id, message_id
)
UPDATE messages AS m
SET metadata = jsonb_set(
        COALESCE(m.metadata, '{}'::jsonb),
        '{has_attachments}',
        'true'::jsonb,
        true
    ),
    updated_at = now()
FROM attached
WHERE m.workspace_id = attached.workspace_id
  AND m.id = attached.message_id
`

func (r *Repository) RecordAudit(ctx context.Context, event filesapp.AuditEvent) error {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
INSERT INTO audit_logs (workspace_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (NULLIF($1, '')::uuid, NULLIF($2, '')::uuid, $3, 'file', NULLIF($4, '')::uuid, $5::jsonb)
`, event.WorkspaceID, event.ActorUserID, event.Action, event.FileID, string(encoded))
	return err
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateFile(ctx context.Context, params filesapp.CreateFileParams) (filesdomain.File, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return filesdomain.File{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	row := tx.QueryRow(ctx, `
INSERT INTO files (workspace_id, owner_id, storage_provider, bucket, object_key, original_name, mime_type, byte_size, checksum_sha256, status, metadata)
VALUES ($1::uuid, $2::uuid, $3, NULLIF($4, ''), $5, $6, $7, $8, $9, 'ready', $10::jsonb)
RETURNING id::text, workspace_id::text, owner_id::text, storage_provider, bucket, object_key,
          original_name, mime_type, byte_size, checksum_sha256, status, metadata::text, created_at, updated_at
`, params.WorkspaceID, params.OwnerID, params.StorageProvider, params.Bucket, params.ObjectKey, params.OriginalName, params.MimeType, params.ByteSize, params.ChecksumSHA256, string(params.Metadata))
	file, err := scanFile(row)
	if err != nil {
		return filesdomain.File{}, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO file_versions (file_id, version_number, storage_provider, bucket, object_key, mime_type, byte_size, checksum_sha256, created_by)
VALUES ($1::uuid, 1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8::uuid)
`, file.ID, params.StorageProvider, params.Bucket, params.ObjectKey, params.MimeType, params.ByteSize, params.ChecksumSHA256, params.OwnerID); err != nil {
		return filesdomain.File{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return filesdomain.File{}, err
	}
	return file, nil
}

func (r *Repository) FindFile(ctx context.Context, workspaceID string, fileID string) (filesdomain.File, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, workspace_id::text, owner_id::text, storage_provider, bucket, object_key,
       original_name, mime_type, byte_size, checksum_sha256, status, metadata::text, created_at, updated_at
FROM files
WHERE workspace_id = $1::uuid AND id = $2::uuid AND deleted_at IS NULL
`, workspaceID, fileID)
	return scanFile(row)
}

func (r *Repository) CanAccessFile(ctx context.Context, workspaceID string, fileID string, userID string) (bool, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM files f
    WHERE f.workspace_id = $1::uuid
      AND f.id = $2::uuid
      AND f.deleted_at IS NULL
      AND (
          f.owner_id = $3::uuid
          OR EXISTS (
              SELECT 1
              FROM message_attachments ma
              JOIN messages m ON m.workspace_id = ma.workspace_id AND m.id = ma.message_id AND m.deleted_at IS NULL
              JOIN channel_members cm ON cm.channel_id = m.channel_id AND cm.user_id = $3::uuid AND cm.status IN ('active', 'muted')
              WHERE ma.workspace_id = f.workspace_id AND ma.file_id = f.id
          )
      )
)
`, workspaceID, fileID, userID).Scan(&allowed)
	return allowed, err
}

func (r *Repository) ListFiles(ctx context.Context, params filesapp.ListFilesParams) ([]filesdomain.File, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, workspace_id::text, owner_id::text, storage_provider, bucket, object_key,
       original_name, mime_type, byte_size, checksum_sha256, status, metadata::text, created_at, updated_at
FROM files
WHERE workspace_id = $1::uuid
  AND deleted_at IS NULL
  AND (
      owner_id = $2::uuid
      OR EXISTS (
          SELECT 1
          FROM message_attachments ma
          JOIN messages m ON m.workspace_id = ma.workspace_id AND m.id = ma.message_id AND m.deleted_at IS NULL
          JOIN channel_members cm ON cm.channel_id = m.channel_id AND cm.user_id = $2::uuid AND cm.status IN ('active', 'muted')
          WHERE ma.workspace_id = files.workspace_id AND ma.file_id = files.id
      )
  )
ORDER BY created_at DESC
LIMIT $3
`, params.WorkspaceID, params.ActorUserID, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]filesdomain.File, 0)
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func (r *Repository) CreateVersion(ctx context.Context, params filesapp.CreateVersionParams) (filesdomain.Version, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return filesdomain.Version{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	row := tx.QueryRow(ctx, `
WITH target AS (
    SELECT id
    FROM files
    WHERE workspace_id = $1::uuid AND id = $2::uuid AND deleted_at IS NULL
),
next_version AS (
    SELECT COALESCE(MAX(version_number), 0) + 1 AS version_number
    FROM file_versions
    WHERE file_id = $2::uuid
)
INSERT INTO file_versions (file_id, version_number, storage_provider, bucket, object_key, mime_type, byte_size, checksum_sha256, created_by)
SELECT target.id, next_version.version_number, $4, NULLIF($5, ''), $6, $7, $8, $9, $3::uuid
FROM target, next_version
RETURNING id::text, file_id::text, version_number, storage_provider, bucket, object_key,
          mime_type, byte_size, checksum_sha256, created_by::text, created_at
`, params.WorkspaceID, params.FileID, params.CreatedBy, params.StorageProvider, params.Bucket, params.ObjectKey, params.MimeType, params.ByteSize, params.ChecksumSHA256)
	version, err := scanVersion(row)
	if err != nil {
		return filesdomain.Version{}, err
	}

	command, err := tx.Exec(ctx, `
UPDATE files
SET storage_provider = $3,
    bucket = NULLIF($4, ''),
    object_key = $5,
    mime_type = $6,
    byte_size = $7,
    checksum_sha256 = $8,
    status = 'ready'
WHERE workspace_id = $1::uuid AND id = $2::uuid AND deleted_at IS NULL
`, params.WorkspaceID, params.FileID, params.StorageProvider, params.Bucket, params.ObjectKey, params.MimeType, params.ByteSize, params.ChecksumSHA256)
	if err != nil {
		return filesdomain.Version{}, err
	}
	if command.RowsAffected() == 0 {
		return filesdomain.Version{}, filesdomain.ErrFileNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return filesdomain.Version{}, err
	}
	return version, nil
}

func (r *Repository) ListVersions(ctx context.Context, workspaceID string, fileID string) ([]filesdomain.Version, error) {
	rows, err := r.pool.Query(ctx, `
SELECT fv.id::text, fv.file_id::text, fv.version_number, fv.storage_provider, fv.bucket, fv.object_key,
       fv.mime_type, fv.byte_size, fv.checksum_sha256, fv.created_by::text, fv.created_at
FROM file_versions fv
JOIN files f ON f.id = fv.file_id AND f.deleted_at IS NULL
WHERE f.workspace_id = $1::uuid AND f.id = $2::uuid
ORDER BY fv.version_number DESC
`, workspaceID, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	versions := make([]filesdomain.Version, 0)
	for rows.Next() {
		version, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

func (r *Repository) AttachFile(ctx context.Context, params filesapp.AttachFileParams) (filesdomain.Attachment, error) {
	// Keep the attachment row and its discoverability marker in one PostgreSQL
	// statement. jsonb_set changes only has_attachments, preserving metadata such
	// as message_type=voice and leaving the message kind untouched.
	command, err := r.pool.Exec(ctx, attachFileQuery,
		params.WorkspaceID,
		params.ChannelID,
		params.MessageID,
		params.FileID,
		params.ActorUserID,
		params.SortOrder,
	)
	if err != nil {
		return filesdomain.Attachment{}, err
	}
	if command.RowsAffected() == 0 {
		if _, err := r.FindFile(ctx, params.WorkspaceID, params.FileID); err != nil {
			return filesdomain.Attachment{}, err
		}
		return filesdomain.Attachment{}, filesdomain.ErrMessageNotFound
	}
	return r.attachment(ctx, params.WorkspaceID, params.MessageID, params.FileID)
}

func (r *Repository) ListAttachments(ctx context.Context, params filesapp.ListAttachmentsParams) ([]filesdomain.Attachment, error) {
	rows, err := r.pool.Query(ctx, `
SELECT ma.workspace_id::text, ma.message_id::text, ma.file_id::text, ma.sort_order, ma.created_at,
       f.id::text, f.workspace_id::text, f.owner_id::text, f.storage_provider, f.bucket, f.object_key,
       f.original_name, f.mime_type, f.byte_size, f.checksum_sha256, f.status, f.metadata::text, f.created_at, f.updated_at
FROM message_attachments ma
JOIN messages m
  ON m.workspace_id = ma.workspace_id
 AND m.id = ma.message_id
 AND m.channel_id = $2::uuid
 AND m.deleted_at IS NULL
JOIN channel_members cm
  ON cm.channel_id = m.channel_id
 AND cm.user_id = $4::uuid
 AND cm.status IN ('active', 'muted')
JOIN files f ON f.id = ma.file_id AND f.deleted_at IS NULL
WHERE ma.workspace_id = $1::uuid AND ma.message_id = $3::uuid
ORDER BY ma.sort_order ASC, ma.created_at ASC
`, params.WorkspaceID, params.ChannelID, params.MessageID, params.ActorUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attachments := make([]filesdomain.Attachment, 0)
	for rows.Next() {
		attachment, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	return attachments, rows.Err()
}

func (r *Repository) ListChannelMedia(ctx context.Context, params filesapp.ListChannelMediaParams) ([]filesdomain.Attachment, error) {
	rows, err := r.pool.Query(ctx, `
SELECT ma.workspace_id::text, ma.message_id::text, ma.file_id::text, ma.sort_order, ma.created_at,
       f.id::text, f.workspace_id::text, f.owner_id::text, f.storage_provider, f.bucket, f.object_key,
       f.original_name, f.mime_type, f.byte_size, f.checksum_sha256, f.status, f.metadata::text, f.created_at, f.updated_at
FROM message_attachments ma
JOIN messages m
  ON m.workspace_id = ma.workspace_id
 AND m.id = ma.message_id
 AND m.channel_id = $2::uuid
 AND m.deleted_at IS NULL
JOIN channel_members cm
  ON cm.channel_id = m.channel_id
 AND cm.user_id = $3::uuid
 AND cm.status IN ('active', 'muted')
JOIN files f ON f.id = ma.file_id AND f.deleted_at IS NULL
WHERE ma.workspace_id = $1::uuid
  AND f.mime_type LIKE 'image/%'
ORDER BY ma.created_at DESC, ma.sort_order ASC
LIMIT $4
`, params.WorkspaceID, params.ChannelID, params.ActorUserID, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attachments := make([]filesdomain.Attachment, 0)
	for rows.Next() {
		attachment, err := scanAttachment(rows)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	return attachments, rows.Err()
}

func (r *Repository) attachment(ctx context.Context, workspaceID string, messageID string, fileID string) (filesdomain.Attachment, error) {
	row := r.pool.QueryRow(ctx, `
SELECT ma.workspace_id::text, ma.message_id::text, ma.file_id::text, ma.sort_order, ma.created_at,
       f.id::text, f.workspace_id::text, f.owner_id::text, f.storage_provider, f.bucket, f.object_key,
       f.original_name, f.mime_type, f.byte_size, f.checksum_sha256, f.status, f.metadata::text, f.created_at, f.updated_at
FROM message_attachments ma
JOIN files f ON f.id = ma.file_id AND f.deleted_at IS NULL
WHERE ma.workspace_id = $1::uuid AND ma.message_id = $2::uuid AND ma.file_id = $3::uuid
`, workspaceID, messageID, fileID)
	return scanAttachment(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFile(row rowScanner) (filesdomain.File, error) {
	var file filesdomain.File
	var workspaceID sql.NullString
	var ownerID sql.NullString
	var bucket sql.NullString
	var checksum sql.NullString
	var metadata string
	if err := row.Scan(
		&file.ID,
		&workspaceID,
		&ownerID,
		&file.StorageProvider,
		&bucket,
		&file.ObjectKey,
		&file.OriginalName,
		&file.MimeType,
		&file.ByteSize,
		&checksum,
		&file.Status,
		&metadata,
		&file.CreatedAt,
		&file.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return filesdomain.File{}, filesdomain.ErrFileNotFound
		}
		return filesdomain.File{}, err
	}
	file.WorkspaceID = nullStringPtr(workspaceID)
	file.OwnerID = nullStringPtr(ownerID)
	file.Bucket = nullStringPtr(bucket)
	file.ChecksumSHA256 = nullStringPtr(checksum)
	file.Metadata = []byte(metadata)
	return file, nil
}

func scanVersion(row rowScanner) (filesdomain.Version, error) {
	var version filesdomain.Version
	var bucket sql.NullString
	var checksum sql.NullString
	var createdBy sql.NullString
	if err := row.Scan(
		&version.ID,
		&version.FileID,
		&version.VersionNumber,
		&version.StorageProvider,
		&bucket,
		&version.ObjectKey,
		&version.MimeType,
		&version.ByteSize,
		&checksum,
		&createdBy,
		&version.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return filesdomain.Version{}, filesdomain.ErrFileNotFound
		}
		return filesdomain.Version{}, err
	}
	version.Bucket = nullStringPtr(bucket)
	version.ChecksumSHA256 = nullStringPtr(checksum)
	version.CreatedBy = nullStringPtr(createdBy)
	return version, nil
}

func scanAttachment(row rowScanner) (filesdomain.Attachment, error) {
	var attachment filesdomain.Attachment
	var file filesdomain.File
	var workspaceID sql.NullString
	var ownerID sql.NullString
	var bucket sql.NullString
	var checksum sql.NullString
	var metadata string
	if err := row.Scan(
		&attachment.WorkspaceID,
		&attachment.MessageID,
		&attachment.FileID,
		&attachment.SortOrder,
		&attachment.CreatedAt,
		&file.ID,
		&workspaceID,
		&ownerID,
		&file.StorageProvider,
		&bucket,
		&file.ObjectKey,
		&file.OriginalName,
		&file.MimeType,
		&file.ByteSize,
		&checksum,
		&file.Status,
		&metadata,
		&file.CreatedAt,
		&file.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return filesdomain.Attachment{}, filesdomain.ErrAttachmentNotFound
		}
		return filesdomain.Attachment{}, err
	}
	file.WorkspaceID = nullStringPtr(workspaceID)
	file.OwnerID = nullStringPtr(ownerID)
	file.Bucket = nullStringPtr(bucket)
	file.ChecksumSHA256 = nullStringPtr(checksum)
	file.Metadata = []byte(metadata)
	attachment.File = file
	return attachment, nil
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
