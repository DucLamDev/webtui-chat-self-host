package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	channelsapp "github.com/duclamdev/application-chat/backend/internal/modules/channels/application"
	channelsdomain "github.com/duclamdev/application-chat/backend/internal/modules/channels/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetCollaborationSettings(ctx context.Context, workspaceID string, channelID string) (channelsdomain.CollaborationSettings, error) {
	if _, err := r.pool.Exec(ctx, `
INSERT INTO channel_collaboration_settings (channel_id, workspace_id, created_by)
SELECT c.id, c.workspace_id, c.created_by
FROM channels c
WHERE c.workspace_id = $1::uuid AND c.id = $2::uuid AND c.deleted_at IS NULL
ON CONFLICT (channel_id) DO NOTHING
`, workspaceID, channelID); err != nil {
		return channelsdomain.CollaborationSettings{}, err
	}
	return scanCollaborationSettings(r.pool.QueryRow(ctx, collaborationSettingsSelect+`
WHERE c.workspace_id = $1::uuid AND c.id = $2::uuid AND c.deleted_at IS NULL
`, workspaceID, channelID))
}

func (r *Repository) UpdateCollaborationSettings(ctx context.Context, params channelsapp.UpdateCollaborationSettingsParams) (channelsdomain.CollaborationSettings, error) {
	command, err := r.pool.Exec(ctx, `
INSERT INTO channel_collaboration_settings (
    channel_id, workspace_id, room_mode, meeting_provider, lobby_enabled, chat_locked,
    guest_microphone_enabled, guest_camera_enabled, default_participant_role, created_by
)
SELECT c.id, c.workspace_id, $3, $4, $5, $6, $7, $8, $9, $10::uuid
FROM channels c
WHERE c.workspace_id = $1::uuid AND c.id = $2::uuid AND c.deleted_at IS NULL
ON CONFLICT (channel_id) DO UPDATE
SET room_mode = EXCLUDED.room_mode,
    meeting_provider = EXCLUDED.meeting_provider,
    lobby_enabled = EXCLUDED.lobby_enabled,
    chat_locked = EXCLUDED.chat_locked,
    guest_microphone_enabled = EXCLUDED.guest_microphone_enabled,
    guest_camera_enabled = EXCLUDED.guest_camera_enabled,
    default_participant_role = EXCLUDED.default_participant_role
`, params.WorkspaceID, params.ChannelID, params.RoomMode, params.MeetingProvider, params.LobbyEnabled,
		params.ChatLocked, params.GuestMicrophoneEnabled, params.GuestCameraEnabled,
		params.DefaultParticipantRole, params.ActorUserID)
	if err != nil {
		return channelsdomain.CollaborationSettings{}, err
	}
	if command.RowsAffected() == 0 {
		return channelsdomain.CollaborationSettings{}, channelsdomain.ErrChannelNotFound
	}
	return r.GetCollaborationSettings(ctx, params.WorkspaceID, params.ChannelID)
}

func (r *Repository) PromoteDirectConversation(ctx context.Context, params channelsapp.PromoteDirectConversationParams) (channelsdomain.CollaborationSettings, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return channelsdomain.CollaborationSettings{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	command, err := tx.Exec(ctx, `
UPDATE channels
SET type = CASE WHEN type = 'direct' THEN 'group' ELSE type END,
    name = $3,
    slug = NULL
WHERE workspace_id = $1::uuid AND id = $2::uuid AND deleted_at IS NULL
  AND type = 'direct'
`, params.WorkspaceID, params.ChannelID, params.Name)
	if err != nil {
		return channelsdomain.CollaborationSettings{}, err
	}
	if command.RowsAffected() == 0 {
		return channelsdomain.CollaborationSettings{}, channelsdomain.ErrChannelNotFound
	}
	if _, err := tx.Exec(ctx, `
UPDATE direct_conversations
SET archived_at = COALESCE(archived_at, now())
WHERE workspace_id = $1::uuid AND channel_id = $2::uuid
`, params.WorkspaceID, params.ChannelID); err != nil {
		return channelsdomain.CollaborationSettings{}, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO channel_collaboration_settings (channel_id, workspace_id, room_mode, meeting_provider, created_by)
VALUES ($2::uuid, $1::uuid, 'internal', 'jitsi', $3::uuid)
ON CONFLICT (channel_id) DO UPDATE
SET room_mode = 'internal', meeting_provider = 'jitsi'
`, params.WorkspaceID, params.ChannelID, params.ActorUserID); err != nil {
		return channelsdomain.CollaborationSettings{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return channelsdomain.CollaborationSettings{}, err
	}
	return r.GetCollaborationSettings(ctx, params.WorkspaceID, params.ChannelID)
}

func (r *Repository) SetPublicLink(ctx context.Context, params channelsapp.SetPublicLinkParams) (channelsdomain.CollaborationSettings, error) {
	command, err := r.pool.Exec(ctx, `
INSERT INTO channel_collaboration_settings (
    channel_id, workspace_id, room_mode, meeting_provider, public_access_enabled,
    public_token_hash, public_token_prefix, password_hash, lobby_enabled, chat_locked,
    guest_microphone_enabled, guest_camera_enabled, created_by
)
SELECT c.id, c.workspace_id, $3, 'jitsi', true, $4, $5, NULLIF($6, ''), $7, $8, $9, $10, $11::uuid
FROM channels c
WHERE c.workspace_id = $1::uuid AND c.id = $2::uuid AND c.deleted_at IS NULL AND c.type <> 'direct'
ON CONFLICT (channel_id) DO UPDATE
SET room_mode = EXCLUDED.room_mode,
    meeting_provider = 'jitsi',
    meeting_room_key = encode(gen_random_bytes(18), 'hex'),
    public_access_enabled = true,
    public_token_hash = EXCLUDED.public_token_hash,
    public_token_prefix = EXCLUDED.public_token_prefix,
    password_hash = EXCLUDED.password_hash,
    lobby_enabled = EXCLUDED.lobby_enabled,
    chat_locked = EXCLUDED.chat_locked,
    guest_microphone_enabled = EXCLUDED.guest_microphone_enabled,
    guest_camera_enabled = EXCLUDED.guest_camera_enabled
`, params.WorkspaceID, params.ChannelID, params.RoomMode, params.PublicTokenHash, params.PublicTokenPrefix,
		params.PasswordHash, params.LobbyEnabled, params.ChatLocked, params.GuestMicrophoneEnabled,
		params.GuestCameraEnabled, params.ActorUserID)
	if err != nil {
		return channelsdomain.CollaborationSettings{}, err
	}
	if command.RowsAffected() == 0 {
		return channelsdomain.CollaborationSettings{}, channelsdomain.ErrChannelNotFound
	}
	return r.GetCollaborationSettings(ctx, params.WorkspaceID, params.ChannelID)
}

func (r *Repository) DisablePublicLink(ctx context.Context, workspaceID string, channelID string) (channelsdomain.CollaborationSettings, error) {
	command, err := r.pool.Exec(ctx, `
UPDATE channel_collaboration_settings s
SET public_access_enabled = false,
    public_token_hash = NULL,
    public_token_prefix = NULL,
    password_hash = NULL,
    room_mode = 'internal',
    meeting_room_key = encode(gen_random_bytes(18), 'hex')
FROM channels c
WHERE c.id = s.channel_id
  AND c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND c.deleted_at IS NULL
`, workspaceID, channelID)
	if err != nil {
		return channelsdomain.CollaborationSettings{}, err
	}
	if command.RowsAffected() == 0 {
		return channelsdomain.CollaborationSettings{}, channelsdomain.ErrChannelNotFound
	}
	return r.GetCollaborationSettings(ctx, workspaceID, channelID)
}

func (r *Repository) FindPublicSettings(ctx context.Context, publicTokenHash string) (channelsdomain.CollaborationSettings, error) {
	return scanCollaborationSettings(r.pool.QueryRow(ctx, collaborationSettingsSelect+`
WHERE s.public_access_enabled = true
  AND s.public_token_hash = $1
  AND c.deleted_at IS NULL
  AND c.status = 'active'
`, publicTokenHash))
}

func (r *Repository) CreateGuestRequest(ctx context.Context, params channelsapp.CreateGuestRequestParams) (channelsdomain.GuestRequest, error) {
	return scanGuestRequest(r.pool.QueryRow(ctx, `
INSERT INTO channel_guest_requests (
    channel_id, display_name, status, access_token_hash, expires_at
)
VALUES ($1::uuid, $2, $3, $4, $5)
RETURNING id::text, channel_id::text, display_name, status, reviewed_by::text,
          reviewed_at, expires_at, created_at, updated_at
`, params.ChannelID, params.DisplayName, params.Status, params.AccessTokenHash, params.ExpiresAt))
}

func (r *Repository) GetGuestRequest(ctx context.Context, channelID string, requestID string, accessTokenHash string) (channelsdomain.GuestRequest, error) {
	return scanGuestRequest(r.pool.QueryRow(ctx, `
UPDATE channel_guest_requests
SET status = CASE WHEN expires_at <= now() AND status IN ('waiting', 'approved') THEN 'expired' ELSE status END
WHERE channel_id = $1::uuid AND id = $2::uuid AND access_token_hash = $3
RETURNING id::text, channel_id::text, display_name, status, reviewed_by::text,
          reviewed_at, expires_at, created_at, updated_at
`, channelID, requestID, accessTokenHash))
}

func (r *Repository) ListGuestRequests(ctx context.Context, workspaceID string, channelID string) ([]channelsdomain.GuestRequest, error) {
	rows, err := r.pool.Query(ctx, `
SELECT g.id::text, g.channel_id::text, g.display_name,
       CASE WHEN g.expires_at <= now() AND g.status IN ('waiting', 'approved') THEN 'expired' ELSE g.status END,
       g.reviewed_by::text, g.reviewed_at, g.expires_at, g.created_at, g.updated_at
FROM channel_guest_requests g
JOIN channels c ON c.id = g.channel_id AND c.deleted_at IS NULL
WHERE c.workspace_id = $1::uuid AND c.id = $2::uuid
ORDER BY (g.status = 'waiting') DESC, g.created_at DESC
`, workspaceID, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]channelsdomain.GuestRequest, 0)
	for rows.Next() {
		guest, err := scanGuestRequest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, guest)
	}
	return result, rows.Err()
}

func (r *Repository) UpdateGuestRequestStatus(ctx context.Context, params channelsapp.UpdateGuestRequestStatusParams) (channelsdomain.GuestRequest, error) {
	return scanGuestRequest(r.pool.QueryRow(ctx, `
UPDATE channel_guest_requests g
SET status = $4,
    reviewed_by = $5::uuid,
    reviewed_at = now()
FROM channels c
WHERE c.id = g.channel_id
  AND c.workspace_id = $1::uuid
  AND g.channel_id = $2::uuid
  AND g.id = $3::uuid
  AND g.status = 'waiting'
  AND g.expires_at > now()
RETURNING g.id::text, g.channel_id::text, g.display_name, g.status, g.reviewed_by::text,
          g.reviewed_at, g.expires_at, g.created_at, g.updated_at
`, params.WorkspaceID, params.ChannelID, params.RequestID, params.Status, params.ActorUserID))
}

func (r *Repository) ListCollaborationRoles(ctx context.Context, workspaceID string, channelID string) ([]channelsdomain.CollaborationRole, error) {
	rows, err := r.pool.Query(ctx, `
SELECT cm.channel_id::text, cm.user_id::text, u.display_name, u.username::text, u.avatar_url,
       COALESCE(cr.role,
           CASE WHEN c.created_by = cm.user_id THEN 'moderator'
                ELSE COALESCE(s.default_participant_role, 'member') END
       ) AS role,
       COALESCE(cr.updated_at, cm.updated_at)
FROM channel_members cm
JOIN channels c ON c.id = cm.channel_id AND c.deleted_at IS NULL
JOIN users u ON u.id = cm.user_id AND u.deleted_at IS NULL
LEFT JOIN channel_collaboration_settings s ON s.channel_id = c.id
LEFT JOIN channel_collaboration_roles cr ON cr.channel_id = cm.channel_id AND cr.user_id = cm.user_id
WHERE c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND cm.status IN ('active', 'muted')
ORDER BY (COALESCE(cr.role, CASE WHEN c.created_by = cm.user_id THEN 'moderator' ELSE 'member' END) = 'moderator') DESC,
         u.display_name
`, workspaceID, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]channelsdomain.CollaborationRole, 0)
	for rows.Next() {
		role, err := scanCollaborationRole(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, role)
	}
	return result, rows.Err()
}

func (r *Repository) UpsertCollaborationRole(ctx context.Context, params channelsapp.UpsertCollaborationRoleParams) (channelsdomain.CollaborationRole, error) {
	if _, err := r.pool.Exec(ctx, `
INSERT INTO channel_collaboration_roles (channel_id, user_id, role, assigned_by)
SELECT cm.channel_id, cm.user_id, $4, $5::uuid
FROM channel_members cm
JOIN channels c ON c.id = cm.channel_id AND c.deleted_at IS NULL
WHERE c.workspace_id = $1::uuid
  AND cm.channel_id = $2::uuid
  AND cm.user_id = $3::uuid
  AND cm.status IN ('active', 'muted')
ON CONFLICT (channel_id, user_id) DO UPDATE
SET role = EXCLUDED.role, assigned_by = EXCLUDED.assigned_by
`, params.WorkspaceID, params.ChannelID, params.UserID, params.Role, params.ActorUserID); err != nil {
		return channelsdomain.CollaborationRole{}, err
	}
	return scanCollaborationRole(r.pool.QueryRow(ctx, `
SELECT cm.channel_id::text, cm.user_id::text, u.display_name, u.username::text, u.avatar_url,
       cr.role, cr.updated_at
FROM channel_collaboration_roles cr
JOIN channel_members cm ON cm.channel_id = cr.channel_id AND cm.user_id = cr.user_id
JOIN users u ON u.id = cr.user_id AND u.deleted_at IS NULL
WHERE cr.channel_id = $1::uuid AND cr.user_id = $2::uuid
`, params.ChannelID, params.UserID))
}

func (r *Repository) GetCollaborationDocument(ctx context.Context, workspaceID string, channelID string, kind string) (channelsdomain.CollaborationDocument, error) {
	command, err := r.pool.Exec(ctx, `
INSERT INTO channel_collaboration_documents (channel_id, kind, content)
SELECT c.id, $3, '{}'::jsonb
FROM channels c
WHERE c.workspace_id = $1::uuid AND c.id = $2::uuid AND c.deleted_at IS NULL
ON CONFLICT (channel_id, kind) DO NOTHING
`, workspaceID, channelID, kind)
	if err != nil {
		return channelsdomain.CollaborationDocument{}, err
	}
	if command.RowsAffected() == 0 {
		var exists bool
		if err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1 FROM channels
    WHERE workspace_id = $1::uuid AND id = $2::uuid AND deleted_at IS NULL
)
`, workspaceID, channelID).Scan(&exists); err != nil {
			return channelsdomain.CollaborationDocument{}, err
		}
		if !exists {
			return channelsdomain.CollaborationDocument{}, channelsdomain.ErrChannelNotFound
		}
	}
	return scanCollaborationDocument(r.pool.QueryRow(ctx, `
SELECT d.channel_id::text, d.kind, d.content, d.version, d.updated_by::text, d.created_at, d.updated_at
FROM channel_collaboration_documents d
JOIN channels c ON c.id = d.channel_id AND c.deleted_at IS NULL
WHERE c.workspace_id = $1::uuid AND d.channel_id = $2::uuid AND d.kind = $3
`, workspaceID, channelID, kind))
}

func (r *Repository) UpdateCollaborationDocument(ctx context.Context, params channelsapp.UpdateCollaborationDocumentParams) (channelsdomain.CollaborationDocument, error) {
	if _, err := r.GetCollaborationDocument(ctx, params.WorkspaceID, params.ChannelID, params.Kind); err != nil {
		return channelsdomain.CollaborationDocument{}, err
	}
	document, err := scanCollaborationDocument(r.pool.QueryRow(ctx, `
UPDATE channel_collaboration_documents d
SET content = $4::jsonb,
    version = version + 1,
    updated_by = $5::uuid
FROM channels c
WHERE c.id = d.channel_id
  AND c.workspace_id = $1::uuid
  AND d.channel_id = $2::uuid
  AND d.kind = $3
  AND d.version = $6
RETURNING d.channel_id::text, d.kind, d.content, d.version, d.updated_by::text, d.created_at, d.updated_at
`, params.WorkspaceID, params.ChannelID, params.Kind, params.Content, params.ActorUserID, params.ExpectedVersion))
	if errors.Is(err, channelsdomain.ErrChannelNotFound) {
		return channelsdomain.CollaborationDocument{}, channelsdomain.ErrVersionConflict
	}
	return document, err
}

func (r *Repository) ListChannelTasks(ctx context.Context, workspaceID string, channelID string) ([]channelsdomain.ChannelTask, error) {
	rows, err := r.pool.Query(ctx, `
SELECT t.id::text, t.workspace_id::text, t.channel_id::text, t.source_message_id::text,
       t.title, t.description, t.status, t.assignee_user_id::text, t.due_at,
       t.created_by::text, t.completed_at, t.created_at, t.updated_at
FROM channel_tasks t
JOIN channels c ON c.id = t.channel_id AND c.deleted_at IS NULL
WHERE c.workspace_id = $1::uuid AND t.channel_id = $2::uuid
ORDER BY (t.status IN ('open', 'in_progress')) DESC, t.created_at DESC
`, workspaceID, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]channelsdomain.ChannelTask, 0)
	for rows.Next() {
		task, err := scanChannelTask(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, task)
	}
	return result, rows.Err()
}

func (r *Repository) CreateChannelTask(ctx context.Context, params channelsapp.CreateChannelTaskParams) (channelsdomain.ChannelTask, error) {
	task, err := scanChannelTask(r.pool.QueryRow(ctx, `
INSERT INTO channel_tasks (
    workspace_id, channel_id, source_message_id, title, description,
    assignee_user_id, due_at, created_by
)
SELECT c.workspace_id, c.id, NULLIF($3, '')::uuid, $4, NULLIF($5, ''),
       NULLIF($6, '')::uuid, $7, $8::uuid
FROM channels c
WHERE c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND c.deleted_at IS NULL
  AND (
      $3 = '' OR EXISTS (
          SELECT 1 FROM messages m
          WHERE m.workspace_id = c.workspace_id AND m.channel_id = c.id
            AND m.id = $3::uuid AND m.deleted_at IS NULL
      )
  )
  AND (
      $6 = '' OR EXISTS (
          SELECT 1 FROM channel_members cm
          WHERE cm.channel_id = c.id AND cm.user_id = $6::uuid
            AND cm.status IN ('active', 'muted')
      )
  )
RETURNING id::text, workspace_id::text, channel_id::text, source_message_id::text,
          title, description, status, assignee_user_id::text, due_at,
          created_by::text, completed_at, created_at, updated_at
`, params.WorkspaceID, params.ChannelID, params.SourceMessageID, params.Title, params.Description,
		params.AssigneeUserID, params.DueAt, params.ActorUserID))
	if err != nil && isUniqueViolation(err) {
		return channelsdomain.ChannelTask{}, channelsdomain.ErrTaskConflict
	}
	return task, err
}

func (r *Repository) UpdateChannelTask(ctx context.Context, params channelsapp.UpdateChannelTaskParams) (channelsdomain.ChannelTask, error) {
	updateAssignee := params.AssigneeUserID != nil
	assignee := ""
	if params.AssigneeUserID != nil {
		assignee = *params.AssigneeUserID
	}
	return scanChannelTask(r.pool.QueryRow(ctx, `
UPDATE channel_tasks t
SET status = $4,
    assignee_user_id = CASE WHEN $5 THEN NULLIF($6, '')::uuid ELSE assignee_user_id END,
    due_at = CASE WHEN $7 THEN NULL ELSE COALESCE($8, due_at) END,
    completed_at = CASE
        WHEN $4 = 'done' THEN COALESCE(completed_at, now())
        WHEN $4 <> 'done' THEN NULL
        ELSE completed_at
    END
FROM channels c
WHERE c.id = t.channel_id
  AND c.workspace_id = $1::uuid
  AND t.channel_id = $2::uuid
  AND t.id = $3::uuid
RETURNING t.id::text, t.workspace_id::text, t.channel_id::text, t.source_message_id::text,
          t.title, t.description, t.status, t.assignee_user_id::text, t.due_at,
          t.created_by::text, t.completed_at, t.created_at, t.updated_at
`, params.WorkspaceID, params.ChannelID, params.TaskID, params.Status, updateAssignee, assignee,
		params.ClearDueAt, params.DueAt))
}

func (r *Repository) ListBreakoutRooms(ctx context.Context, workspaceID string, channelID string) ([]channelsdomain.BreakoutRoom, error) {
	rows, err := r.pool.Query(ctx, `
SELECT b.id::text, b.channel_id::text, b.name, b.room_key, b.assigned_user_ids,
       b.status, b.assignment_mode, b.allow_self_select, b.started_at, b.sequence,
       b.created_by::text, b.closed_at, b.created_at, b.updated_at
FROM channel_breakout_rooms b
JOIN channels c ON c.id = b.channel_id AND c.deleted_at IS NULL
WHERE c.workspace_id = $1::uuid AND b.channel_id = $2::uuid
ORDER BY (b.status = 'active') DESC, b.sequence, b.created_at
`, workspaceID, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]channelsdomain.BreakoutRoom, 0)
	for rows.Next() {
		room, err := scanBreakoutRoom(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, room)
	}
	return result, rows.Err()
}

func (r *Repository) CreateBreakoutRoom(ctx context.Context, params channelsapp.CreateBreakoutRoomParams) (channelsdomain.BreakoutRoom, error) {
	assigned, err := json.Marshal(params.AssignedUserIDs)
	if err != nil {
		return channelsdomain.BreakoutRoom{}, err
	}
	return scanBreakoutRoom(r.pool.QueryRow(ctx, `
INSERT INTO channel_breakout_rooms (
    channel_id, name, assigned_user_ids, assignment_mode,
    allow_self_select, sequence, created_by
)
SELECT c.id, $3, $4::jsonb, $5, $6, $7, $8::uuid
FROM channels c
WHERE c.workspace_id = $1::uuid AND c.id = $2::uuid AND c.deleted_at IS NULL
RETURNING id::text, channel_id::text, name, room_key, assigned_user_ids,
          status, assignment_mode, allow_self_select, started_at, sequence,
          created_by::text, closed_at, created_at, updated_at
`, params.WorkspaceID, params.ChannelID, params.Name, assigned, params.AssignmentMode,
		params.AllowSelfSelect, params.Sequence, params.ActorUserID))
}

func (r *Repository) CloseBreakoutRooms(ctx context.Context, workspaceID string, channelID string, roomID string) ([]channelsdomain.BreakoutRoom, error) {
	if _, err := r.pool.Exec(ctx, `
UPDATE channel_breakout_rooms b
SET status = 'closed', closed_at = COALESCE(closed_at, now())
FROM channels c
WHERE c.id = b.channel_id
  AND c.workspace_id = $1::uuid
  AND b.channel_id = $2::uuid
  AND ($3 = '' OR b.id = $3::uuid)
  AND b.status IN ('prepared', 'active')
`, workspaceID, channelID, roomID); err != nil {
		return nil, err
	}
	return r.ListBreakoutRooms(ctx, workspaceID, channelID)
}

const collaborationSettingsSelect = `
SELECT c.id::text, c.workspace_id::text, c.name, c.type,
       s.room_mode, s.meeting_provider, s.meeting_room_key,
       s.public_access_enabled, s.public_token_prefix, s.password_hash,
       s.lobby_enabled, s.chat_locked, s.guest_microphone_enabled,
       s.guest_camera_enabled, s.default_participant_role,
       s.created_by::text, s.created_at, s.updated_at
FROM channel_collaboration_settings s
JOIN channels c ON c.id = s.channel_id
`

func scanCollaborationSettings(row rowScanner) (channelsdomain.CollaborationSettings, error) {
	var settings channelsdomain.CollaborationSettings
	var tokenPrefix sql.NullString
	var passwordHash sql.NullString
	var createdBy sql.NullString
	if err := row.Scan(
		&settings.ChannelID,
		&settings.WorkspaceID,
		&settings.ChannelName,
		&settings.ChannelType,
		&settings.RoomMode,
		&settings.MeetingProvider,
		&settings.MeetingRoomKey,
		&settings.PublicAccessEnabled,
		&tokenPrefix,
		&passwordHash,
		&settings.LobbyEnabled,
		&settings.ChatLocked,
		&settings.GuestMicrophoneEnabled,
		&settings.GuestCameraEnabled,
		&settings.DefaultParticipantRole,
		&createdBy,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsdomain.CollaborationSettings{}, channelsdomain.ErrChannelNotFound
		}
		return channelsdomain.CollaborationSettings{}, err
	}
	settings.PublicTokenPrefix = nullStringPtr(tokenPrefix)
	settings.PasswordHash = nullStringPtr(passwordHash)
	settings.CreatedBy = nullStringPtr(createdBy)
	return settings, nil
}

func scanGuestRequest(row rowScanner) (channelsdomain.GuestRequest, error) {
	var guest channelsdomain.GuestRequest
	var reviewedBy sql.NullString
	var reviewedAt sql.NullTime
	if err := row.Scan(
		&guest.ID,
		&guest.ChannelID,
		&guest.DisplayName,
		&guest.Status,
		&reviewedBy,
		&reviewedAt,
		&guest.ExpiresAt,
		&guest.CreatedAt,
		&guest.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsdomain.GuestRequest{}, channelsdomain.ErrGuestNotFound
		}
		return channelsdomain.GuestRequest{}, err
	}
	guest.ReviewedBy = nullStringPtr(reviewedBy)
	guest.ReviewedAt = nullTimePtr(reviewedAt)
	return guest, nil
}

func scanCollaborationRole(row rowScanner) (channelsdomain.CollaborationRole, error) {
	var role channelsdomain.CollaborationRole
	var avatarURL sql.NullString
	if err := row.Scan(
		&role.ChannelID,
		&role.UserID,
		&role.DisplayName,
		&role.Username,
		&avatarURL,
		&role.Role,
		&role.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsdomain.CollaborationRole{}, channelsdomain.ErrMemberNotFound
		}
		return channelsdomain.CollaborationRole{}, err
	}
	role.AvatarURL = nullStringPtr(avatarURL)
	return role, nil
}

func scanCollaborationDocument(row rowScanner) (channelsdomain.CollaborationDocument, error) {
	var document channelsdomain.CollaborationDocument
	var updatedBy sql.NullString
	if err := row.Scan(
		&document.ChannelID,
		&document.Kind,
		&document.Content,
		&document.Version,
		&updatedBy,
		&document.CreatedAt,
		&document.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsdomain.CollaborationDocument{}, channelsdomain.ErrChannelNotFound
		}
		return channelsdomain.CollaborationDocument{}, err
	}
	document.UpdatedBy = nullStringPtr(updatedBy)
	return document, nil
}

func scanChannelTask(row rowScanner) (channelsdomain.ChannelTask, error) {
	var task channelsdomain.ChannelTask
	var sourceMessageID sql.NullString
	var description sql.NullString
	var assigneeUserID sql.NullString
	var dueAt sql.NullTime
	var createdBy sql.NullString
	var completedAt sql.NullTime
	if err := row.Scan(
		&task.ID,
		&task.WorkspaceID,
		&task.ChannelID,
		&sourceMessageID,
		&task.Title,
		&description,
		&task.Status,
		&assigneeUserID,
		&dueAt,
		&createdBy,
		&completedAt,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsdomain.ChannelTask{}, channelsdomain.ErrChannelNotFound
		}
		return channelsdomain.ChannelTask{}, err
	}
	task.SourceMessageID = nullStringPtr(sourceMessageID)
	task.Description = nullStringPtr(description)
	task.AssigneeUserID = nullStringPtr(assigneeUserID)
	task.DueAt = nullTimePtr(dueAt)
	task.CreatedBy = nullStringPtr(createdBy)
	task.CompletedAt = nullTimePtr(completedAt)
	return task, nil
}

func scanBreakoutRoom(row rowScanner) (channelsdomain.BreakoutRoom, error) {
	var room channelsdomain.BreakoutRoom
	var createdBy sql.NullString
	var startedAt sql.NullTime
	var closedAt sql.NullTime
	if err := row.Scan(
		&room.ID,
		&room.ChannelID,
		&room.Name,
		&room.RoomKey,
		&room.AssignedUserIDs,
		&room.Status,
		&room.AssignmentMode,
		&room.AllowSelfSelect,
		&startedAt,
		&room.Sequence,
		&createdBy,
		&closedAt,
		&room.CreatedAt,
		&room.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsdomain.BreakoutRoom{}, channelsdomain.ErrChannelNotFound
		}
		return channelsdomain.BreakoutRoom{}, err
	}
	room.CreatedBy = nullStringPtr(createdBy)
	room.StartedAt = nullTimePtr(startedAt)
	room.ClosedAt = nullTimePtr(closedAt)
	return room, nil
}
