package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	filesapp "github.com/duclamdev/application-chat/backend/internal/modules/files/application"
	filesdomain "github.com/duclamdev/application-chat/backend/internal/modules/files/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateUploadSession(ctx context.Context, params filesapp.CreateUploadSessionParams) (filesapp.UploadSession, error) {
	return scanUploadSession(r.pool.QueryRow(ctx, `
INSERT INTO file_upload_sessions (
    workspace_id, owner_id, channel_id, message_id, original_name, mime_type,
    total_size, chunk_size, total_chunks, metadata, expires_at
)
SELECT $1::uuid, $2::uuid, NULLIF($3, '')::uuid, NULLIF($4, '')::uuid,
       $5, $6, $7, $8, $9, $10::jsonb, $11
WHERE (
    $3 = ''
    OR EXISTS (
        SELECT 1
        FROM channels c
        JOIN channel_members cm
          ON cm.channel_id = c.id
         AND cm.user_id = $2::uuid
         AND cm.status IN ('active', 'muted')
        WHERE c.workspace_id = $1::uuid
          AND c.id = $3::uuid
          AND c.deleted_at IS NULL
          AND (
              $4 = ''
              OR EXISTS (
                  SELECT 1 FROM messages m
                  WHERE m.workspace_id = c.workspace_id
                    AND m.channel_id = c.id
                    AND m.id = $4::uuid
                    AND m.deleted_at IS NULL
              )
          )
    )
)
RETURNING id::text, workspace_id::text, owner_id::text, channel_id::text,
          message_id::text, original_name, mime_type, total_size, chunk_size,
          total_chunks, received_bytes, metadata::text, status, file_id::text,
          checksum_sha256, expires_at, completed_at, created_at, updated_at
`, params.WorkspaceID, params.OwnerID, params.ChannelID, params.MessageID,
		params.OriginalName, params.MimeType, params.TotalSize, params.ChunkSize,
		params.TotalChunks, string(params.Metadata), params.ExpiresAt))
}

func (r *Repository) GetUploadSession(ctx context.Context, workspaceID string, uploadID string, ownerID string) (filesapp.UploadSession, error) {
	session, err := scanUploadSession(r.pool.QueryRow(ctx, uploadSessionSelect+`
WHERE workspace_id = $1::uuid
  AND id = $2::uuid
  AND owner_id = $3::uuid
`, workspaceID, uploadID, ownerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return filesapp.UploadSession{}, filesdomain.ErrFileNotFound
	}
	return session, err
}

func (r *Repository) UpsertUploadPart(ctx context.Context, params filesapp.UpsertUploadPartParams) (filesapp.UploadSession, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return filesapp.UploadSession{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	command, err := tx.Exec(ctx, `
INSERT INTO file_upload_parts (
    upload_id, part_number, object_key, byte_size, checksum_sha256
)
SELECT fus.id, $4, $5, $6, $7
FROM file_upload_sessions fus
WHERE fus.workspace_id = $1::uuid
  AND fus.id = $2::uuid
  AND fus.owner_id = $3::uuid
  AND fus.status = 'uploading'
  AND fus.expires_at > now()
ON CONFLICT (upload_id, part_number)
DO UPDATE SET
    object_key = EXCLUDED.object_key,
    byte_size = EXCLUDED.byte_size,
    checksum_sha256 = EXCLUDED.checksum_sha256
`, params.WorkspaceID, params.UploadID, params.OwnerID, params.PartNumber,
		params.ObjectKey, params.ByteSize, params.ChecksumSHA256)
	if err != nil {
		return filesapp.UploadSession{}, err
	}
	if command.RowsAffected() == 0 {
		return filesapp.UploadSession{}, filesdomain.ErrFileNotFound
	}
	if _, err := tx.Exec(ctx, `
UPDATE file_upload_sessions
SET received_bytes = COALESCE((
    SELECT sum(part.byte_size)
    FROM file_upload_parts part
    WHERE part.upload_id = file_upload_sessions.id
), 0)
WHERE workspace_id = $1::uuid
  AND id = $2::uuid
  AND owner_id = $3::uuid
`, params.WorkspaceID, params.UploadID, params.OwnerID); err != nil {
		return filesapp.UploadSession{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return filesapp.UploadSession{}, err
	}
	return r.GetUploadSession(ctx, params.WorkspaceID, params.UploadID, params.OwnerID)
}

func (r *Repository) ListUploadParts(ctx context.Context, workspaceID string, uploadID string, ownerID string) ([]filesapp.UploadPart, error) {
	rows, err := r.pool.Query(ctx, `
SELECT part.upload_id::text, part.part_number, part.object_key, part.byte_size,
       part.checksum_sha256, part.created_at, part.updated_at
FROM file_upload_parts part
JOIN file_upload_sessions session ON session.id = part.upload_id
WHERE session.workspace_id = $1::uuid
  AND session.id = $2::uuid
  AND session.owner_id = $3::uuid
ORDER BY part.part_number
`, workspaceID, uploadID, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]filesapp.UploadPart, 0)
	for rows.Next() {
		var part filesapp.UploadPart
		if err := rows.Scan(
			&part.UploadID, &part.PartNumber, &part.ObjectKey, &part.ByteSize,
			&part.ChecksumSHA256, &part.CreatedAt, &part.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, part)
	}
	return result, rows.Err()
}

func (r *Repository) MarkUploadCompleting(ctx context.Context, workspaceID string, uploadID string, ownerID string) (filesapp.UploadSession, error) {
	session, err := scanUploadSession(r.pool.QueryRow(ctx, `
UPDATE file_upload_sessions session
SET status = 'completing'
WHERE session.workspace_id = $1::uuid
  AND session.id = $2::uuid
  AND session.owner_id = $3::uuid
  AND session.status IN ('uploading', 'failed')
  AND session.expires_at > now()
  AND session.received_bytes = session.total_size
  AND (
      SELECT count(*)
      FROM file_upload_parts part
      WHERE part.upload_id = session.id
  ) = session.total_chunks
RETURNING id::text, workspace_id::text, owner_id::text, channel_id::text,
          message_id::text, original_name, mime_type, total_size, chunk_size,
          total_chunks, received_bytes, metadata::text, status, file_id::text,
          checksum_sha256, expires_at, completed_at, created_at, updated_at
`, workspaceID, uploadID, ownerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return filesapp.UploadSession{}, filesdomain.ErrFileNotFound
	}
	return session, err
}

func (r *Repository) CompleteUploadSession(ctx context.Context, params filesapp.CompleteUploadSessionParams) (filesapp.UploadSession, error) {
	session, err := scanUploadSession(r.pool.QueryRow(ctx, `
UPDATE file_upload_sessions
SET status = 'completed',
    file_id = $4::uuid,
    checksum_sha256 = $5,
    completed_at = now()
WHERE workspace_id = $1::uuid
  AND id = $2::uuid
  AND owner_id = $3::uuid
  AND status = 'completing'
RETURNING id::text, workspace_id::text, owner_id::text, channel_id::text,
          message_id::text, original_name, mime_type, total_size, chunk_size,
          total_chunks, received_bytes, metadata::text, status, file_id::text,
          checksum_sha256, expires_at, completed_at, created_at, updated_at
`, params.WorkspaceID, params.UploadID, params.OwnerID, params.FileID, params.ChecksumSHA256))
	if errors.Is(err, pgx.ErrNoRows) {
		return filesapp.UploadSession{}, filesdomain.ErrFileNotFound
	}
	return session, err
}

func (r *Repository) FailUploadSession(ctx context.Context, workspaceID string, uploadID string, ownerID string) error {
	command, err := r.pool.Exec(ctx, `
UPDATE file_upload_sessions
SET status = 'failed'
WHERE workspace_id = $1::uuid
  AND id = $2::uuid
  AND owner_id = $3::uuid
  AND status = 'completing'
`, workspaceID, uploadID, ownerID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return filesdomain.ErrFileNotFound
	}
	return nil
}

func (r *Repository) CancelUploadSession(ctx context.Context, workspaceID string, uploadID string, ownerID string) error {
	command, err := r.pool.Exec(ctx, `
UPDATE file_upload_sessions
SET status = 'cancelled'
WHERE workspace_id = $1::uuid
  AND id = $2::uuid
  AND owner_id = $3::uuid
  AND status IN ('uploading', 'failed')
`, workspaceID, uploadID, ownerID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return filesdomain.ErrFileNotFound
	}
	return nil
}

const uploadSessionSelect = `
SELECT id::text, workspace_id::text, owner_id::text, channel_id::text,
       message_id::text, original_name, mime_type, total_size, chunk_size,
       total_chunks, received_bytes, metadata::text, status, file_id::text,
       checksum_sha256, expires_at, completed_at, created_at, updated_at
FROM file_upload_sessions
`

func scanUploadSession(row rowScanner) (filesapp.UploadSession, error) {
	var session filesapp.UploadSession
	var channelID, messageID, fileID, checksum sql.NullString
	var completedAt sql.NullTime
	var metadata string
	if err := row.Scan(
		&session.ID, &session.WorkspaceID, &session.OwnerID, &channelID,
		&messageID, &session.OriginalName, &session.MimeType, &session.TotalSize,
		&session.ChunkSize, &session.TotalChunks, &session.ReceivedBytes,
		&metadata, &session.Status, &fileID, &checksum, &session.ExpiresAt,
		&completedAt, &session.CreatedAt, &session.UpdatedAt,
	); err != nil {
		return filesapp.UploadSession{}, err
	}
	session.ChannelID = nullStringPtr(channelID)
	session.MessageID = nullStringPtr(messageID)
	session.FileID = nullStringPtr(fileID)
	session.ChecksumSHA256 = nullStringPtr(checksum)
	session.CompletedAt = nullTimePtr(completedAt)
	session.Metadata = []byte(metadata)
	return session, nil
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
