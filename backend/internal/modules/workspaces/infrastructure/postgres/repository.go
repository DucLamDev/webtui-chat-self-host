package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	workspacesapp "github.com/duclamdev/application-chat/backend/internal/modules/workspaces/application"
	workspacesdomain "github.com/duclamdev/application-chat/backend/internal/modules/workspaces/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateWorkspace(ctx context.Context, params workspacesapp.CreateWorkspaceParams) (workspacesdomain.Workspace, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return workspacesdomain.Workspace{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var zoneKind string
	var enforcementMode string
	var maxWorkspaces int
	var workspaceCount int
	if err := tx.QueryRow(ctx, `
SELECT zone.kind,
       quota.enforcement_mode,
       quota.max_workspaces,
       (
           SELECT count(*)
           FROM workspaces workspace
           WHERE workspace.zone_id = zone.id
             AND workspace.deleted_at IS NULL
       )
FROM zones zone
JOIN zone_quotas quota ON quota.zone_id = zone.id
WHERE zone.id = $1::uuid
  AND zone.status = 'active'
  AND zone.deleted_at IS NULL
FOR UPDATE OF quota
`, params.ZoneID).Scan(&zoneKind, &enforcementMode, &maxWorkspaces, &workspaceCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return workspacesdomain.Workspace{}, workspacesdomain.ErrWorkspaceNotFound
		}
		return workspacesdomain.Workspace{}, err
	}
	if enforcementMode == "hard" && workspaceCount >= maxWorkspaces {
		return workspacesdomain.Workspace{}, workspacesdomain.ErrWorkspaceQuotaExceeded
	}
	template := workspacesapp.WorkspaceTemplateForZone(zoneKind)

	row := tx.QueryRow(ctx, `
INSERT INTO workspaces (zone_id, slug, name, description, owner_id)
VALUES ($1::uuid, $2, $3, NULLIF($4, ''), $5::uuid)
RETURNING id::text, zone_id::text, slug::text, name, description, owner_id::text, plan, status, created_at, updated_at
`, params.ZoneID, params.Slug, params.Name, params.Description, params.OwnerID)
	workspace, err := scanWorkspace(row)
	if err != nil {
		if isUniqueViolation(err) {
			return workspacesdomain.Workspace{}, workspacesdomain.ErrWorkspaceConflict
		}
		return workspacesdomain.Workspace{}, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO workspace_members (workspace_id, user_id, status, joined_at)
VALUES ($1::uuid, $2::uuid, 'active', now())
ON CONFLICT (workspace_id, user_id)
DO UPDATE SET status = 'active', joined_at = COALESCE(workspace_members.joined_at, now())
`, workspace.ID, params.OwnerID); err != nil {
		return workspacesdomain.Workspace{}, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
SELECT $1::uuid, $2::uuid, id, $2::uuid
FROM roles
WHERE workspace_id IS NULL AND code = 'workspace_owner' AND deleted_at IS NULL
ON CONFLICT DO NOTHING
`, workspace.ID, params.OwnerID); err != nil {
		return workspacesdomain.Workspace{}, err
	}

	defaultChannelIDs := make(map[string]string, len(template.Channels))
	for _, definition := range template.Channels {
		settings := map[string]any{"system_default": true, "template_key": template.Key}
		if definition.PrivateSession {
			settings["bot_session_mode"] = "private"
		}
		settingsJSON, err := json.Marshal(settings)
		if err != nil {
			return workspacesdomain.Workspace{}, err
		}
		var channelID string
		if err := tx.QueryRow(ctx, `
INSERT INTO channels (workspace_id, slug, name, description, type, created_by, settings)
VALUES ($1::uuid, $2, $3, $4, $5, $6::uuid, $7::jsonb)
RETURNING id::text
`, workspace.ID, definition.Slug, definition.Name, definition.Description, definition.Type, params.OwnerID, string(settingsJSON)).Scan(&channelID); err != nil {
			return workspacesdomain.Workspace{}, err
		}
		defaultChannelIDs[definition.Slug] = channelID
		if !definition.PrivateSession {
			if _, err := tx.Exec(ctx, `
INSERT INTO channel_members (channel_id, user_id, status)
VALUES ($1::uuid, $2::uuid, 'active')
ON CONFLICT (channel_id, user_id) DO UPDATE SET status = 'active'
`, channelID, params.OwnerID); err != nil {
				return workspacesdomain.Workspace{}, err
			}
		}
	}

	for _, definition := range template.Bots {
		channelID, ok := defaultChannelIDs[definition.ChannelSlug]
		if !ok {
			return workspacesdomain.Workspace{}, errors.New("default bot references an unknown channel")
		}
		var botID string
		if err := tx.QueryRow(ctx, `
INSERT INTO bots (workspace_id, slug, name, description, created_by, settings)
VALUES ($1::uuid, $2, $3, $4, $5::uuid, '{"system_default": true}'::jsonb)
RETURNING id::text
`, workspace.ID, definition.Slug, definition.Name, definition.Description, params.OwnerID).Scan(&botID); err != nil {
			return workspacesdomain.Workspace{}, err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO bot_installations (bot_id, workspace_id, channel_id, config)
VALUES ($1::uuid, $2::uuid, $3::uuid, '{"system_default": true}'::jsonb)
`, botID, workspace.ID, channelID); err != nil {
			return workspacesdomain.Workspace{}, err
		}
	}

	if template.Key == "vpsttt_services" {
		if err := ensureDefaultBotGuidePins(ctx, tx, workspace.ID, params.OwnerID); err != nil {
			return workspacesdomain.Workspace{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return workspacesdomain.Workspace{}, err
	}
	return workspace, nil
}

func ensureDefaultBotGuidePins(ctx context.Context, exec commandExecutor, workspaceID string, actorUserID string) error {
	_, err := exec.Exec(ctx, `
WITH guide(slug, body) AS (
    VALUES
        ('gia-han', 'Kiểm tra dịch vụ sắp hết hạn
Email: khach@example.com
Số ngày: 30
Loại dịch vụ: VPS'),
        ('ke-toan', 'Tạo QR nạp ví
Email: khach@example.com
Số tiền: 200000'),
        ('ticket', 'Tạo ticket hỗ trợ cho khach@example.com
VPS #1234 bị mất kết nối, ping timeout và port 22 không truy cập được.'),
        ('server-alert', 'Server: vps-01
Lỗi: Mất ping 3 phút
IP: 192.0.2.10
Mức độ: critical')
),
targets AS (
    SELECT DISTINCT
        c.workspace_id,
        c.id AS channel_id,
        c.id AS source_channel_id,
        c.slug::text AS source_slug,
        guide.body
    FROM channels c
    JOIN guide
      ON guide.slug = c.slug::text
    WHERE c.workspace_id = $1::uuid
      AND c.deleted_at IS NULL
      AND c.status = 'active'
      AND EXISTS (
          SELECT 1
          FROM bot_installations bi
          WHERE bi.workspace_id = c.workspace_id
            AND bi.channel_id = c.id
            AND bi.status = 'active'
      )
),
inserted AS (
    INSERT INTO messages (workspace_id, channel_id, sender_id, kind, body, metadata)
    SELECT
        targets.workspace_id,
        targets.channel_id,
        $2::uuid,
        'text',
        targets.body,
        jsonb_build_object(
            'seed', 'bot_channel_guide',
            'source_channel_id', targets.source_channel_id::text,
            'source_slug', targets.source_slug
        )
    FROM targets
    WHERE NOT EXISTS (
        SELECT 1
        FROM messages m
        WHERE m.workspace_id = targets.workspace_id
          AND m.channel_id = targets.channel_id
          AND m.metadata @> jsonb_build_object(
              'seed', 'bot_channel_guide',
              'source_channel_id', targets.source_channel_id::text
          )
          AND m.deleted_at IS NULL
    )
    RETURNING id, workspace_id, channel_id
),
guide_messages AS (
    SELECT m.id, m.workspace_id, m.channel_id
    FROM targets
    JOIN messages m
      ON m.workspace_id = targets.workspace_id
     AND m.channel_id = targets.channel_id
     AND m.metadata @> jsonb_build_object(
         'seed', 'bot_channel_guide',
         'source_channel_id', targets.source_channel_id::text
     )
     AND m.deleted_at IS NULL
    UNION
    SELECT id, workspace_id, channel_id
    FROM inserted
)
INSERT INTO message_pins (workspace_id, channel_id, message_id, pinned_by)
SELECT workspace_id, channel_id, id, $2::uuid
FROM guide_messages
ON CONFLICT (workspace_id, channel_id, message_id)
DO UPDATE SET pinned_by = EXCLUDED.pinned_by,
              created_at = message_pins.created_at
`, workspaceID, actorUserID)
	return err
}

func (r *Repository) FindWorkspace(ctx context.Context, workspaceID string) (workspacesdomain.Workspace, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, zone_id::text, slug::text, name, description, owner_id::text, plan, status, created_at, updated_at
FROM workspaces
WHERE id = $1::uuid AND deleted_at IS NULL
`, workspaceID)
	return scanWorkspace(row)
}

func (r *Repository) ListWorkspacesForUser(ctx context.Context, userID string, zoneID string) ([]workspacesdomain.Workspace, error) {
	rows, err := r.pool.Query(ctx, `
SELECT DISTINCT w.id::text, w.zone_id::text, w.slug::text, w.name, w.description, w.owner_id::text, w.plan, w.status, w.created_at, w.updated_at
FROM workspaces w
LEFT JOIN workspace_members wm
  ON wm.workspace_id = w.id AND wm.user_id = $1::uuid AND wm.status = 'active'
WHERE w.deleted_at IS NULL
  AND w.zone_id = $2::uuid
  AND (w.owner_id = $1::uuid OR wm.user_id IS NOT NULL)
ORDER BY w.created_at DESC
`, userID, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []workspacesdomain.Workspace
	for rows.Next() {
		workspace, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, workspace)
	}
	return workspaces, rows.Err()
}

func (r *Repository) UpdateWorkspace(ctx context.Context, params workspacesapp.UpdateWorkspaceParams) (workspacesdomain.Workspace, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE workspaces
SET name = COALESCE($2, name),
    description = COALESCE($3, description)
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING id::text, zone_id::text, slug::text, name, description, owner_id::text, plan, status, created_at, updated_at
`, params.WorkspaceID, params.Name, params.Description)
	return scanWorkspace(row)
}

func (r *Repository) ArchiveWorkspace(ctx context.Context, workspaceID string) error {
	command, err := r.pool.Exec(ctx, `
UPDATE workspaces
SET status = 'archived', deleted_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL
`, workspaceID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return workspacesdomain.ErrWorkspaceNotFound
	}
	return nil
}

func (r *Repository) ListMembers(ctx context.Context, workspaceID string) ([]workspacesdomain.Member, error) {
	rows, err := r.pool.Query(ctx, `
SELECT wm.id::text, wm.workspace_id::text, wm.user_id::text, u.email::text, u.username::text, u.display_name, u.avatar_url, u.phone_number,
       wm.status, wm.title, wm.joined_at, wm.created_at, wm.updated_at
FROM workspace_members wm
JOIN users u ON u.id = wm.user_id AND u.deleted_at IS NULL
WHERE wm.workspace_id = $1::uuid
ORDER BY wm.created_at DESC
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []workspacesdomain.Member
	for rows.Next() {
		member, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *Repository) AddMember(ctx context.Context, params workspacesapp.AddMemberParams) (workspacesdomain.Member, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return workspacesdomain.Member{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
INSERT INTO workspace_members (workspace_id, user_id, status, title, joined_at)
VALUES ($1::uuid, $2::uuid, 'active', NULLIF($3, ''), now())
ON CONFLICT (workspace_id, user_id)
DO UPDATE SET status = 'active', title = EXCLUDED.title, joined_at = COALESCE(workspace_members.joined_at, now())
`, params.WorkspaceID, params.UserID, params.Title); err != nil {
		return workspacesdomain.Member{}, err
	}

	command, err := tx.Exec(ctx, `
INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
SELECT $1::uuid, $2::uuid, r.id, $4::uuid
FROM roles r
WHERE r.code = $3
  AND r.deleted_at IS NULL
  AND (r.workspace_id IS NULL OR r.workspace_id = $1::uuid)
ORDER BY (r.workspace_id IS NULL), r.created_at DESC
LIMIT 1
ON CONFLICT (workspace_id, user_id, role_id)
DO UPDATE SET assigned_by = workspace_member_roles.assigned_by
`, params.WorkspaceID, params.UserID, params.RoleCode, params.AssignedBy)
	if err != nil {
		return workspacesdomain.Member{}, err
	}
	if command.RowsAffected() == 0 {
		return workspacesdomain.Member{}, workspacesdomain.ErrRoleNotFound
	}

	// Public channels are visible to workspace members only. Enrol a newly
	// added member so the channel list remains useful without leaking channels
	// to users who are not members of them.
	if _, err := tx.Exec(ctx, `
INSERT INTO channel_members (channel_id, user_id, status)
SELECT c.id, $2::uuid, 'active'
FROM channels c
WHERE c.workspace_id = $1::uuid
  AND c.type = 'public'
  AND c.status = 'active'
  AND c.deleted_at IS NULL
ON CONFLICT (channel_id, user_id) DO UPDATE SET status = 'active'
`, params.WorkspaceID, params.UserID); err != nil {
		return workspacesdomain.Member{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return workspacesdomain.Member{}, err
	}
	return r.member(ctx, params.WorkspaceID, params.UserID)
}

func (r *Repository) UpdateMemberStatus(ctx context.Context, params workspacesapp.UpdateMemberStatusParams) (workspacesdomain.Member, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE workspace_members
SET status = $3
WHERE workspace_id = $1::uuid AND user_id = $2::uuid
RETURNING id::text, workspace_id::text, user_id::text, ''::text, ''::text, ''::text, ''::text, ''::text, status, title, joined_at, created_at, updated_at
`, params.WorkspaceID, params.UserID, params.Status)
	member, err := scanMember(row)
	if err != nil {
		return workspacesdomain.Member{}, err
	}
	return r.member(ctx, member.WorkspaceID, member.UserID)
}

func (r *Repository) UpsertSetting(ctx context.Context, params workspacesapp.UpsertSettingParams) (workspacesdomain.Setting, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO workspace_settings (workspace_id, key, value, value_type, description, updated_by)
VALUES ($1::uuid, $2, $3::jsonb, $4, NULLIF($5, ''), $6::uuid)
ON CONFLICT (workspace_id, key)
DO UPDATE SET value = EXCLUDED.value,
              value_type = EXCLUDED.value_type,
              description = EXCLUDED.description,
              updated_by = EXCLUDED.updated_by
RETURNING workspace_id::text, key, value, value_type, description, updated_by::text, created_at, updated_at
`, params.WorkspaceID, params.Key, string(params.Value), params.ValueType, params.Description, params.UpdatedBy)
	return scanSetting(row)
}

func (r *Repository) ListSettings(ctx context.Context, workspaceID string) ([]workspacesdomain.Setting, error) {
	rows, err := r.pool.Query(ctx, `
SELECT workspace_id::text, key, value, value_type, description, updated_by::text, created_at, updated_at
FROM workspace_settings
WHERE workspace_id = $1::uuid
ORDER BY key
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []workspacesdomain.Setting
	for rows.Next() {
		setting, err := scanSetting(rows)
		if err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}
	return settings, rows.Err()
}

func (r *Repository) CreateInvite(ctx context.Context, params workspacesapp.CreateInviteParams) (workspacesdomain.Invite, error) {
	row := r.pool.QueryRow(ctx, `
WITH selected_role AS (
    SELECT id
    FROM roles
    WHERE code = $3
      AND deleted_at IS NULL
      AND (workspace_id IS NULL OR workspace_id = $1::uuid)
    ORDER BY (workspace_id IS NULL), created_at DESC
    LIMIT 1
)
INSERT INTO workspace_invites (workspace_id, email, role_id, token_hash, invited_by, expires_at)
SELECT
    $1::uuid,
    $2,
    selected_role.id,
    $4,
    $5::uuid,
    $6
FROM selected_role
RETURNING id::text, workspace_id::text, email::text, role_id::text, invited_by::text,
          expires_at, accepted_at, revoked_at, created_at
`, params.WorkspaceID, params.Email, params.RoleCode, params.TokenHash, params.InvitedBy, params.ExpiresAt)
	invite, err := scanInvite(row)
	if errors.Is(err, workspacesdomain.ErrInviteNotFound) {
		return workspacesdomain.Invite{}, workspacesdomain.ErrRoleNotFound
	}
	return invite, err
}

func (r *Repository) ListInvites(ctx context.Context, workspaceID string) ([]workspacesdomain.Invite, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, workspace_id::text, email::text, role_id::text, invited_by::text,
       expires_at, accepted_at, revoked_at, created_at
FROM workspace_invites
WHERE workspace_id = $1::uuid
ORDER BY created_at DESC
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []workspacesdomain.Invite
	for rows.Next() {
		invite, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		invites = append(invites, invite)
	}
	return invites, rows.Err()
}

func (r *Repository) RecordAudit(ctx context.Context, event workspacesapp.AuditEvent) error {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
INSERT INTO audit_logs (workspace_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (NULLIF($1, '')::uuid, NULLIF($2, '')::uuid, $3, $4, NULLIF($5, '')::uuid, $6::jsonb)
`, event.WorkspaceID, event.ActorUserID, event.Action, event.EntityType, event.EntityID, string(metadataBytes))
	return err
}

func (r *Repository) member(ctx context.Context, workspaceID string, userID string) (workspacesdomain.Member, error) {
	row := r.pool.QueryRow(ctx, `
SELECT wm.id::text, wm.workspace_id::text, wm.user_id::text, u.email::text, u.username::text, u.display_name, u.avatar_url, u.phone_number,
       wm.status, wm.title, wm.joined_at, wm.created_at, wm.updated_at
FROM workspace_members wm
JOIN users u ON u.id = wm.user_id AND u.deleted_at IS NULL
WHERE wm.workspace_id = $1::uuid AND wm.user_id = $2::uuid
`, workspaceID, userID)
	return scanMember(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

type commandExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func scanWorkspace(row rowScanner) (workspacesdomain.Workspace, error) {
	var workspace workspacesdomain.Workspace
	var description sql.NullString
	var ownerID sql.NullString
	if err := row.Scan(
		&workspace.ID,
		&workspace.ZoneID,
		&workspace.Slug,
		&workspace.Name,
		&description,
		&ownerID,
		&workspace.Plan,
		&workspace.Status,
		&workspace.CreatedAt,
		&workspace.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return workspacesdomain.Workspace{}, workspacesdomain.ErrWorkspaceNotFound
		}
		return workspacesdomain.Workspace{}, err
	}
	workspace.Description = nullStringPtr(description)
	workspace.OwnerID = nullStringPtr(ownerID)
	return workspace, nil
}

func scanMember(row rowScanner) (workspacesdomain.Member, error) {
	var member workspacesdomain.Member
	var avatarURL sql.NullString
	var phoneNumber sql.NullString
	var title sql.NullString
	var joinedAt sql.NullTime
	if err := row.Scan(
		&member.ID,
		&member.WorkspaceID,
		&member.UserID,
		&member.Email,
		&member.Username,
		&member.DisplayName,
		&avatarURL,
		&phoneNumber,
		&member.Status,
		&title,
		&joinedAt,
		&member.CreatedAt,
		&member.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return workspacesdomain.Member{}, workspacesdomain.ErrMemberNotFound
		}
		return workspacesdomain.Member{}, err
	}
	member.AvatarURL = nullStringPtr(avatarURL)
	member.PhoneNumber = nullStringPtr(phoneNumber)
	member.Title = nullStringPtr(title)
	member.JoinedAt = nullTimePtr(joinedAt)
	return member, nil
}

func scanSetting(row rowScanner) (workspacesdomain.Setting, error) {
	var setting workspacesdomain.Setting
	var value []byte
	var description sql.NullString
	var updatedBy sql.NullString
	if err := row.Scan(
		&setting.WorkspaceID,
		&setting.Key,
		&value,
		&setting.ValueType,
		&description,
		&updatedBy,
		&setting.CreatedAt,
		&setting.UpdatedAt,
	); err != nil {
		return workspacesdomain.Setting{}, err
	}
	setting.Value = value
	setting.Description = nullStringPtr(description)
	setting.UpdatedBy = nullStringPtr(updatedBy)
	return setting, nil
}

func scanInvite(row rowScanner) (workspacesdomain.Invite, error) {
	var invite workspacesdomain.Invite
	var roleID sql.NullString
	var invitedBy sql.NullString
	var acceptedAt sql.NullTime
	var revokedAt sql.NullTime
	if err := row.Scan(
		&invite.ID,
		&invite.WorkspaceID,
		&invite.Email,
		&roleID,
		&invitedBy,
		&invite.ExpiresAt,
		&acceptedAt,
		&revokedAt,
		&invite.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return workspacesdomain.Invite{}, workspacesdomain.ErrInviteNotFound
		}
		return workspacesdomain.Invite{}, err
	}
	invite.RoleID = nullStringPtr(roleID)
	invite.InvitedBy = nullStringPtr(invitedBy)
	invite.AcceptedAt = nullTimePtr(acceptedAt)
	invite.RevokedAt = nullTimePtr(revokedAt)
	return invite, nil
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
