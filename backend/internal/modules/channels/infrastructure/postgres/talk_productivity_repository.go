package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	channelsapp "github.com/duclamdev/application-chat/backend/internal/modules/channels/application"
	channelsdomain "github.com/duclamdev/application-chat/backend/internal/modules/channels/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListMeetings(ctx context.Context, workspaceID string, channelID string, from *time.Time, to *time.Time) ([]channelsapp.Meeting, error) {
	rows, err := r.pool.Query(ctx, meetingSelect+`
WHERE workspace_id = $1::uuid
  AND channel_id = $2::uuid
  AND ($3::timestamptz IS NULL OR starts_at >= $3)
  AND ($4::timestamptz IS NULL OR starts_at <= $4)
ORDER BY starts_at DESC
`, workspaceID, channelID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]channelsapp.Meeting, 0)
	for rows.Next() {
		meeting, scanErr := scanMeeting(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, meeting)
	}
	return result, rows.Err()
}

func (r *Repository) CreateMeeting(ctx context.Context, params channelsapp.CreateMeetingParams) (channelsapp.Meeting, error) {
	return scanMeeting(r.pool.QueryRow(ctx, `
INSERT INTO channel_meetings (
    workspace_id, channel_id, title, description, starts_at, ends_at,
    lobby_opens_at, room_policy, cleanup_after, created_by
)
SELECT c.workspace_id, c.id, $3, $4, $5, $6, $7, $8, $9, $10::uuid
FROM channels c
WHERE c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND c.deleted_at IS NULL
RETURNING id::text, workspace_id::text, channel_id::text, title, description,
          starts_at, ends_at, lobby_opens_at, status, room_policy, cleanup_after,
          created_by::text, started_at, ended_at, created_at, updated_at
`, params.WorkspaceID, params.ChannelID, params.Title, params.Description,
		params.StartsAt, params.EndsAt, params.LobbyOpensAt, params.RoomPolicy,
		params.CleanupAfter, params.ActorUserID))
}

func (r *Repository) TransitionMeeting(ctx context.Context, params channelsapp.TransitionMeetingParams) (channelsapp.Meeting, error) {
	return scanMeeting(r.pool.QueryRow(ctx, `
UPDATE channel_meetings meeting
SET status = CASE $4
        WHEN 'start' THEN 'active'
        WHEN 'end' THEN 'ended'
        WHEN 'cancel' THEN 'cancelled'
    END,
    started_at = CASE WHEN $4 = 'start' THEN COALESCE(started_at, now()) ELSE started_at END,
    ended_at = CASE WHEN $4 IN ('end', 'cancel') THEN COALESCE(ended_at, now()) ELSE ended_at END
FROM channels channel
WHERE channel.id = meeting.channel_id
  AND channel.workspace_id = $1::uuid
  AND meeting.channel_id = $2::uuid
  AND meeting.id = $3::uuid
  AND (
      ($4 = 'start' AND meeting.status = 'scheduled')
      OR ($4 = 'end' AND meeting.status = 'active')
      OR ($4 = 'cancel' AND meeting.status = 'scheduled')
  )
RETURNING meeting.id::text, meeting.workspace_id::text, meeting.channel_id::text,
          meeting.title, meeting.description, meeting.starts_at, meeting.ends_at,
          meeting.lobby_opens_at, meeting.status, meeting.room_policy,
          meeting.cleanup_after, meeting.created_by::text, meeting.started_at,
          meeting.ended_at, meeting.created_at, meeting.updated_at
`, params.WorkspaceID, params.ChannelID, params.MeetingID, params.Action))
}

func (r *Repository) GetVoiceRoom(ctx context.Context, workspaceID string, channelID string) (channelsapp.VoiceRoom, error) {
	return scanVoiceRoom(r.pool.QueryRow(ctx, `
INSERT INTO channel_voice_rooms (channel_id, workspace_id)
SELECT c.id, c.workspace_id
FROM channels c
WHERE c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND c.deleted_at IS NULL
ON CONFLICT (channel_id) DO UPDATE SET channel_id = EXCLUDED.channel_id
RETURNING channel_id::text, workspace_id::text, status, started_by::text,
          started_at, ended_at, created_at, updated_at
`, workspaceID, channelID))
}

func (r *Repository) SetVoiceRoom(ctx context.Context, params channelsapp.SetVoiceRoomParams) (channelsapp.VoiceRoom, error) {
	return scanVoiceRoom(r.pool.QueryRow(ctx, `
INSERT INTO channel_voice_rooms (
    channel_id, workspace_id, status, started_by, started_at, ended_at
)
SELECT c.id, c.workspace_id, $3, $4::uuid,
       CASE WHEN $3 = 'active' THEN now() ELSE NULL END,
       CASE WHEN $3 = 'inactive' THEN now() ELSE NULL END
FROM channels c
WHERE c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND c.deleted_at IS NULL
ON CONFLICT (channel_id) DO UPDATE SET
    status = EXCLUDED.status,
    started_by = CASE
        WHEN EXCLUDED.status = 'active' THEN EXCLUDED.started_by
        ELSE channel_voice_rooms.started_by
    END,
    started_at = CASE
        WHEN EXCLUDED.status = 'active' AND channel_voice_rooms.status <> 'active' THEN now()
        ELSE channel_voice_rooms.started_at
    END,
    ended_at = CASE WHEN EXCLUDED.status = 'inactive' THEN now() ELSE NULL END
RETURNING channel_id::text, workspace_id::text, status, started_by::text,
          started_at, ended_at, created_at, updated_at
`, params.WorkspaceID, params.ChannelID, params.Status, params.ActorUserID))
}

func (r *Repository) ReplaceBreakoutRooms(ctx context.Context, params channelsapp.ReplaceBreakoutRoomsParams) ([]channelsdomain.BreakoutRoom, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
UPDATE channel_breakout_rooms room
SET status = 'closed', closed_at = COALESCE(closed_at, now())
FROM channels channel
WHERE channel.id = room.channel_id
  AND channel.workspace_id = $1::uuid
  AND room.channel_id = $2::uuid
  AND room.status IN ('prepared', 'active')
`, params.WorkspaceID, params.ChannelID); err != nil {
		return nil, err
	}
	for _, spec := range params.Rooms {
		assigned, marshalErr := json.Marshal(spec.AssignedUserIDs)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO channel_breakout_rooms (
    channel_id, name, assigned_user_ids, assignment_mode,
    allow_self_select, sequence, created_by
)
SELECT c.id, $3, $4::jsonb, $5, $6, $7, $8::uuid
FROM channels c
WHERE c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND c.deleted_at IS NULL
`, params.WorkspaceID, params.ChannelID, spec.Name, assigned, params.AssignmentMode,
			params.AllowSelfSelect, spec.Sequence, params.ActorUserID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.ListBreakoutRooms(ctx, params.WorkspaceID, params.ChannelID)
}

func (r *Repository) SetBreakoutRoomsStatus(ctx context.Context, workspaceID string, channelID string, status string) ([]channelsdomain.BreakoutRoom, error) {
	command, err := r.pool.Exec(ctx, `
UPDATE channel_breakout_rooms room
SET status = $3,
    started_at = CASE WHEN $3 = 'active' THEN COALESCE(started_at, now()) ELSE started_at END,
    closed_at = CASE WHEN $3 = 'closed' THEN COALESCE(closed_at, now()) ELSE NULL END
FROM channels channel
WHERE channel.id = room.channel_id
  AND channel.workspace_id = $1::uuid
  AND room.channel_id = $2::uuid
  AND (
      ($3 = 'active' AND room.status = 'prepared')
      OR ($3 = 'closed' AND room.status IN ('prepared', 'active'))
  )
`, workspaceID, channelID, status)
	if err != nil {
		return nil, err
	}
	if command.RowsAffected() == 0 {
		return nil, channelsdomain.ErrChannelNotFound
	}
	return r.ListBreakoutRooms(ctx, workspaceID, channelID)
}

func (r *Repository) JoinBreakoutRoom(ctx context.Context, params channelsapp.JoinBreakoutRoomParams) ([]channelsdomain.BreakoutRoom, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
UPDATE channel_breakout_rooms
SET assigned_user_ids = assigned_user_ids - $3
WHERE channel_id = $1::uuid
  AND status IN ('prepared', 'active')
  AND allow_self_select = true
  AND EXISTS (
      SELECT 1 FROM channels
      WHERE id = channel_breakout_rooms.channel_id
        AND workspace_id = $2::uuid
  )
`, params.ChannelID, params.WorkspaceID, params.UserID); err != nil {
		return nil, err
	}
	command, err := tx.Exec(ctx, `
UPDATE channel_breakout_rooms
SET assigned_user_ids = assigned_user_ids || jsonb_build_array($4)
WHERE channel_id = $2::uuid
  AND id = $3::uuid
  AND status IN ('prepared', 'active')
  AND allow_self_select = true
  AND EXISTS (
      SELECT 1 FROM channels
      WHERE id = channel_breakout_rooms.channel_id
        AND workspace_id = $1::uuid
  )
  AND NOT assigned_user_ids ? $4
`, params.WorkspaceID, params.ChannelID, params.RoomID, params.UserID)
	if err != nil {
		return nil, err
	}
	if command.RowsAffected() == 0 {
		return nil, channelsdomain.ErrChannelNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.ListBreakoutRooms(ctx, params.WorkspaceID, params.ChannelID)
}

func (r *Repository) UpdateBreakoutAssignments(ctx context.Context, params channelsapp.UpdateBreakoutAssignmentsParams) ([]channelsdomain.BreakoutRoom, error) {
	assigned, err := json.Marshal(params.AssignedUserIDs)
	if err != nil {
		return nil, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
UPDATE channel_breakout_rooms
SET assigned_user_ids = assigned_user_ids - member_id
FROM jsonb_array_elements_text($4::jsonb) member(member_id)
WHERE channel_id = $2::uuid
  AND id <> $3::uuid
  AND status IN ('prepared', 'active')
  AND EXISTS (
      SELECT 1 FROM channels
      WHERE id = channel_breakout_rooms.channel_id
        AND workspace_id = $1::uuid
  )
`, params.WorkspaceID, params.ChannelID, params.RoomID, assigned); err != nil {
		return nil, err
	}
	command, err := tx.Exec(ctx, `
UPDATE channel_breakout_rooms
SET assigned_user_ids = $4::jsonb
WHERE channel_id = $2::uuid
  AND id = $3::uuid
  AND status IN ('prepared', 'active')
  AND EXISTS (
      SELECT 1 FROM channels
      WHERE id = channel_breakout_rooms.channel_id
        AND workspace_id = $1::uuid
  )
`, params.WorkspaceID, params.ChannelID, params.RoomID, assigned)
	if err != nil {
		return nil, err
	}
	if command.RowsAffected() == 0 {
		return nil, channelsdomain.ErrChannelNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.ListBreakoutRooms(ctx, params.WorkspaceID, params.ChannelID)
}

func (r *Repository) CreateBreakoutBroadcast(ctx context.Context, params channelsapp.CreateBreakoutBroadcastParams) (channelsapp.BreakoutBroadcast, error) {
	var broadcast channelsapp.BreakoutBroadcast
	var createdBy sql.NullString
	err := r.pool.QueryRow(ctx, `
INSERT INTO channel_breakout_broadcasts (channel_id, body, created_by)
SELECT c.id, $3, $4::uuid
FROM channels c
WHERE c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND c.deleted_at IS NULL
  AND EXISTS (
      SELECT 1 FROM channel_breakout_rooms room
      WHERE room.channel_id = c.id AND room.status = 'active'
  )
RETURNING id::text, channel_id::text, body, created_by::text, created_at
`, params.WorkspaceID, params.ChannelID, params.Body, params.ActorUserID).Scan(
		&broadcast.ID, &broadcast.ChannelID, &broadcast.Body, &createdBy, &broadcast.CreatedAt,
	)
	if err != nil {
		return channelsapp.BreakoutBroadcast{}, err
	}
	broadcast.CreatedBy = nullStringPtr(createdBy)
	return broadcast, nil
}

func (r *Repository) GetTalkHome(ctx context.Context, workspaceID string, userID string, now time.Time) (channelsapp.TalkHome, error) {
	home := channelsapp.TalkHome{
		UpcomingMeetings: make([]channelsapp.Meeting, 0),
		ActiveVoiceRooms: make([]channelsapp.VoiceRoom, 0),
		OpenTasks:        make([]channelsdomain.ChannelTask, 0),
	}
	meetingRows, err := r.pool.Query(ctx, meetingSelect+`
WHERE workspace_id = $1::uuid
  AND status IN ('scheduled', 'active')
  AND starts_at >= $3::timestamptz - interval '6 hours'
  AND channel_id IN (
      SELECT channel_id FROM channel_members
      WHERE user_id = $2::uuid AND status IN ('active', 'muted')
  )
ORDER BY starts_at
LIMIT 20
`, workspaceID, userID, now)
	if err != nil {
		return home, err
	}
	for meetingRows.Next() {
		meeting, scanErr := scanMeeting(meetingRows)
		if scanErr != nil {
			meetingRows.Close()
			return home, scanErr
		}
		home.UpcomingMeetings = append(home.UpcomingMeetings, meeting)
	}
	if err := meetingRows.Err(); err != nil {
		meetingRows.Close()
		return home, err
	}
	meetingRows.Close()

	voiceRows, err := r.pool.Query(ctx, `
SELECT voice.channel_id::text, voice.workspace_id::text, voice.status,
       voice.started_by::text, voice.started_at, voice.ended_at,
       voice.created_at, voice.updated_at
FROM channel_voice_rooms voice
JOIN channel_members member
  ON member.channel_id = voice.channel_id
 AND member.user_id = $2::uuid
 AND member.status IN ('active', 'muted')
WHERE voice.workspace_id = $1::uuid AND voice.status = 'active'
ORDER BY voice.started_at DESC
LIMIT 20
`, workspaceID, userID)
	if err != nil {
		return home, err
	}
	for voiceRows.Next() {
		room, scanErr := scanVoiceRoom(voiceRows)
		if scanErr != nil {
			voiceRows.Close()
			return home, scanErr
		}
		home.ActiveVoiceRooms = append(home.ActiveVoiceRooms, room)
	}
	if err := voiceRows.Err(); err != nil {
		voiceRows.Close()
		return home, err
	}
	voiceRows.Close()

	taskRows, err := r.pool.Query(ctx, `
SELECT task.id::text, task.workspace_id::text, task.channel_id::text,
       task.source_message_id::text, task.title, task.description, task.status,
       task.assignee_user_id::text, task.due_at, task.created_by::text,
       task.completed_at, task.created_at, task.updated_at
FROM channel_tasks task
JOIN channel_members member
  ON member.channel_id = task.channel_id
 AND member.user_id = $2::uuid
 AND member.status IN ('active', 'muted')
WHERE task.workspace_id = $1::uuid
  AND task.status IN ('open', 'in_progress')
  AND (task.assignee_user_id IS NULL OR task.assignee_user_id = $2::uuid)
ORDER BY task.due_at NULLS LAST, task.updated_at DESC
LIMIT 30
`, workspaceID, userID)
	if err != nil {
		return home, err
	}
	for taskRows.Next() {
		task, scanErr := scanChannelTask(taskRows)
		if scanErr != nil {
			taskRows.Close()
			return home, scanErr
		}
		home.OpenTasks = append(home.OpenTasks, task)
	}
	if err := taskRows.Err(); err != nil {
		taskRows.Close()
		return home, err
	}
	taskRows.Close()

	err = r.pool.QueryRow(ctx, `
SELECT
    (
        SELECT count(*)
        FROM message_mentions mention
        JOIN messages message
          ON message.workspace_id = mention.workspace_id
         AND message.id = mention.message_id
        JOIN channel_members member
          ON member.channel_id = message.channel_id
         AND member.user_id = $2::uuid
        WHERE mention.workspace_id = $1::uuid
          AND mention.mentioned_user_id = $2::uuid
          AND message.deleted_at IS NULL
          AND message.created_at > COALESCE(member.last_read_at, member.joined_at)
    ),
    (
        SELECT count(*)
        FROM message_reminders reminder
        WHERE reminder.workspace_id = $1::uuid
          AND reminder.user_id = $2::uuid
          AND reminder.status = 'pending'
    ),
    (
        SELECT count(*)
        FROM call_sessions call
        WHERE call.workspace_id = $1::uuid
          AND call.target_user_id = $2::uuid
          AND call.status = 'missed'
          AND call.updated_at >= now() - interval '30 days'
    )
`, workspaceID, userID).Scan(&home.UnreadMentions, &home.PendingReminders, &home.MissedCalls)
	return home, err
}

func (r *Repository) ListSharedItems(ctx context.Context, workspaceID string, channelID string, kind string, limit int) ([]channelsapp.SharedItem, error) {
	rows, err := r.pool.Query(ctx, `
WITH shared AS (
    SELECT file.id::text AS id, 'file'::text AS kind, file.original_name AS title,
           file.mime_type AS subtitle,
           '/api/v1/workspaces/' || $1 || '/files/' || file.id::text || '/download' AS url,
           jsonb_build_object(
               'message_id', attachment.message_id,
               'byte_size', file.byte_size,
               'mime_type', file.mime_type
           ) AS metadata,
           file.owner_id::text AS created_by, attachment.created_at
    FROM message_attachments attachment
    JOIN messages message
      ON message.workspace_id = attachment.workspace_id
     AND message.id = attachment.message_id
    JOIN files file ON file.id = attachment.file_id
    WHERE attachment.workspace_id = $1::uuid
      AND message.channel_id = $2::uuid
      AND message.deleted_at IS NULL
      AND file.deleted_at IS NULL

    UNION ALL

    SELECT pin.message_id::text, 'pin', left(message.body, 160), 'Pinned message',
           '', jsonb_build_object('message_id', pin.message_id, 'expires_at', pin.expires_at),
           pin.pinned_by::text, pin.created_at
    FROM message_pins pin
    JOIN messages message
      ON message.workspace_id = pin.workspace_id
     AND message.id = pin.message_id
    WHERE pin.workspace_id = $1::uuid
      AND pin.channel_id = $2::uuid
      AND (pin.expires_at IS NULL OR pin.expires_at > now())
      AND message.deleted_at IS NULL

    UNION ALL

    SELECT task.id::text, 'task', task.title, task.status, '',
           jsonb_build_object(
               'status', task.status,
               'assignee_user_id', task.assignee_user_id,
               'due_at', task.due_at,
               'source_message_id', task.source_message_id
           ),
           task.created_by::text, task.created_at
    FROM channel_tasks task
    WHERE task.workspace_id = $1::uuid
      AND task.channel_id = $2::uuid

    UNION ALL

    SELECT recording.id::text, 'recording', 'Meeting recording', recording.status, '',
           jsonb_build_object(
               'status', recording.status,
               'mime_type', recording.mime_type,
               'byte_size', recording.byte_size,
               'transcript_status', recording.transcript_status,
               'summary_status', recording.summary_status
           ),
           recording.started_by::text, recording.created_at
    FROM channel_recordings recording
    WHERE recording.workspace_id = $1::uuid
      AND recording.channel_id = $2::uuid
      AND recording.status <> 'deleted'

    UNION ALL

    SELECT message.id::text, 'poll',
           COALESCE(NULLIF(message.metadata->>'question', ''), left(message.body, 160)),
           'Poll', '', message.metadata, message.sender_id::text, message.created_at
    FROM messages message
    WHERE message.workspace_id = $1::uuid
      AND message.channel_id = $2::uuid
      AND message.deleted_at IS NULL
      AND (
          message.metadata->>'type' = 'poll'
          OR message.metadata ? 'poll_options'
      )
)
SELECT id, kind, title, subtitle, url, metadata::text, created_by, created_at
FROM shared
WHERE ($3 IN ('', 'all') OR kind = $3)
ORDER BY created_at DESC
LIMIT $4
`, workspaceID, channelID, kind, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]channelsapp.SharedItem, 0)
	for rows.Next() {
		var item channelsapp.SharedItem
		var metadata string
		var createdBy sql.NullString
		if err := rows.Scan(
			&item.ID, &item.Kind, &item.Title, &item.Subtitle, &item.URL,
			&metadata, &createdBy, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.Metadata = json.RawMessage(metadata)
		item.CreatedBy = nullStringPtr(createdBy)
		result = append(result, item)
	}
	return result, rows.Err()
}

const meetingSelect = `
SELECT id::text, workspace_id::text, channel_id::text, title, description,
       starts_at, ends_at, lobby_opens_at, status, room_policy, cleanup_after,
       created_by::text, started_at, ended_at, created_at, updated_at
FROM channel_meetings
`

func scanMeeting(row rowScanner) (channelsapp.Meeting, error) {
	var meeting channelsapp.Meeting
	var endsAt, lobbyOpensAt, cleanupAfter, startedAt, endedAt sql.NullTime
	var createdBy sql.NullString
	if err := row.Scan(
		&meeting.ID, &meeting.WorkspaceID, &meeting.ChannelID, &meeting.Title,
		&meeting.Description, &meeting.StartsAt, &endsAt, &lobbyOpensAt,
		&meeting.Status, &meeting.RoomPolicy, &cleanupAfter, &createdBy,
		&startedAt, &endedAt, &meeting.CreatedAt, &meeting.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsapp.Meeting{}, channelsdomain.ErrChannelNotFound
		}
		return channelsapp.Meeting{}, err
	}
	meeting.EndsAt = nullTimePtr(endsAt)
	meeting.LobbyOpensAt = nullTimePtr(lobbyOpensAt)
	meeting.CleanupAfter = nullTimePtr(cleanupAfter)
	meeting.CreatedBy = nullStringPtr(createdBy)
	meeting.StartedAt = nullTimePtr(startedAt)
	meeting.EndedAt = nullTimePtr(endedAt)
	return meeting, nil
}

func scanVoiceRoom(row rowScanner) (channelsapp.VoiceRoom, error) {
	var room channelsapp.VoiceRoom
	var startedBy sql.NullString
	var startedAt, endedAt sql.NullTime
	if err := row.Scan(
		&room.ChannelID, &room.WorkspaceID, &room.Status, &startedBy,
		&startedAt, &endedAt, &room.CreatedAt, &room.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsapp.VoiceRoom{}, channelsdomain.ErrChannelNotFound
		}
		return channelsapp.VoiceRoom{}, err
	}
	room.StartedBy = nullStringPtr(startedBy)
	room.StartedAt = nullTimePtr(startedAt)
	room.EndedAt = nullTimePtr(endedAt)
	return room, nil
}
