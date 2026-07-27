package postgres

import (
	"context"
	"errors"
	"strings"

	"github.com/duclamdev/application-chat/backend/internal/platform/storage"
)

type TalkMaintenanceResult struct {
	ExpiredPins       int
	EndedMeetings     int
	ClosedBreakouts   int
	StoppedVoiceRooms int
	FailedRecordings  int
	DeletedRecordings int
	ExpiredUploads    int
}

// MaintainTalk advances time-bound collaboration state and removes expired
// objects. Storage deletion happens before database finalization so a transient
// object-store failure remains retryable on the next worker pass.
func (r *Repository) MaintainTalk(
	ctx context.Context,
	store storage.Store,
	limit int,
) (TalkMaintenanceResult, error) {
	if limit < 1 {
		limit = 100
	}
	result, err := r.advanceTalkState(ctx, limit)
	if err != nil {
		return TalkMaintenanceResult{}, err
	}
	var maintenanceErrors []error
	expiredUploads, uploadErr := r.expireUploadObjects(ctx, store, limit)
	result.ExpiredUploads = expiredUploads
	if uploadErr != nil {
		maintenanceErrors = append(maintenanceErrors, uploadErr)
	}
	deletedRecordings, recordingErr := r.deleteExpiredRecordingObjects(
		ctx,
		store,
		limit,
	)
	result.DeletedRecordings = deletedRecordings
	if recordingErr != nil {
		maintenanceErrors = append(maintenanceErrors, recordingErr)
	}
	return result, errors.Join(maintenanceErrors...)
}

func (r *Repository) advanceTalkState(
	ctx context.Context,
	limit int,
) (TalkMaintenanceResult, error) {
	var result TalkMaintenanceResult
	err := r.pool.QueryRow(ctx, `
WITH expired_pin_ids AS (
    SELECT workspace_id, message_id
    FROM message_pins
    WHERE expires_at IS NOT NULL AND expires_at <= now()
    ORDER BY expires_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
),
expired_pins AS (
    DELETE FROM message_pins pin
    USING expired_pin_ids candidate
    WHERE pin.workspace_id = candidate.workspace_id
      AND pin.message_id = candidate.message_id
    RETURNING pin.message_id
),
due_meeting_ids AS (
    SELECT id
    FROM channel_meetings
    WHERE status IN ('scheduled', 'active')
      AND ends_at IS NOT NULL
      AND ends_at <= now()
    ORDER BY ends_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
),
ended_meetings AS (
    UPDATE channel_meetings meeting
    SET status = 'ended',
        ended_at = COALESCE(ended_at, now())
    FROM due_meeting_ids candidate
    WHERE meeting.id = candidate.id
    RETURNING meeting.channel_id
),
closed_breakouts AS (
    UPDATE channel_breakout_rooms room
    SET status = 'closed',
        closed_at = COALESCE(closed_at, now())
    WHERE room.status IN ('prepared', 'active')
      AND room.channel_id IN (SELECT channel_id FROM ended_meetings)
    RETURNING room.id
),
stale_voice_ids AS (
    SELECT channel_id
    FROM channel_voice_rooms
    WHERE status = 'active'
      AND started_at < now() - interval '12 hours'
    ORDER BY started_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
),
stopped_voice_rooms AS (
    UPDATE channel_voice_rooms room
    SET status = 'inactive',
        ended_at = COALESCE(ended_at, now())
    FROM stale_voice_ids candidate
    WHERE room.channel_id = candidate.channel_id
    RETURNING room.channel_id
),
stale_recording_ids AS (
    SELECT id
    FROM channel_recordings
    WHERE (status = 'pending' AND updated_at < now() - interval '1 hour')
       OR (status = 'recording' AND started_at < now() - interval '12 hours')
       OR (status = 'processing' AND updated_at < now() - interval '24 hours')
    ORDER BY updated_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
),
failed_recordings AS (
    UPDATE channel_recordings recording
    SET status = 'failed',
        ended_at = COALESCE(ended_at, now()),
        error = COALESCE(error, 'Recording timed out in background processing.')
    FROM stale_recording_ids candidate
    WHERE recording.id = candidate.id
    RETURNING recording.id
)
SELECT
    (SELECT count(*) FROM expired_pins),
    (SELECT count(*) FROM ended_meetings),
    (SELECT count(*) FROM closed_breakouts),
    (SELECT count(*) FROM stopped_voice_rooms),
    (SELECT count(*) FROM failed_recordings)
`, limit).Scan(
		&result.ExpiredPins,
		&result.EndedMeetings,
		&result.ClosedBreakouts,
		&result.StoppedVoiceRooms,
		&result.FailedRecordings,
	)
	return result, err
}

func (r *Repository) expireUploadObjects(
	ctx context.Context,
	store storage.Store,
	limit int,
) (int, error) {
	rows, err := r.pool.Query(ctx, `
SELECT session.id::text, COALESCE(array_agg(part.object_key)
    FILTER (WHERE part.object_key IS NOT NULL), ARRAY[]::text[])
FROM file_upload_sessions session
LEFT JOIN file_upload_parts part ON part.upload_id = session.id
WHERE session.status IN ('uploading', 'completing', 'failed', 'cancelled')
  AND session.expires_at <= now()
GROUP BY session.id, session.expires_at
ORDER BY session.expires_at
LIMIT $1
`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type candidate struct {
		id   string
		keys []string
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.keys); err != nil {
			return 0, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	rows.Close()
	processed := 0
	var cleanupErrors []error
	for _, item := range candidates {
		deleted := true
		for _, key := range item.keys {
			if err := store.Delete(ctx, key); err != nil {
				deleted = false
				cleanupErrors = append(cleanupErrors, err)
			}
		}
		if !deleted {
			continue
		}
		command, err := r.pool.Exec(ctx, `
WITH expired AS (
    UPDATE file_upload_sessions
    SET status = 'expired'
    WHERE id = $1::uuid
      AND status IN ('uploading', 'completing', 'failed', 'cancelled')
      AND expires_at <= now()
    RETURNING id
)
DELETE FROM file_upload_parts
WHERE upload_id IN (SELECT id FROM expired)
`, item.id)
		if err != nil {
			cleanupErrors = append(cleanupErrors, err)
			continue
		}
		if command.RowsAffected() > 0 || len(item.keys) == 0 {
			processed++
		}
	}
	return processed, errors.Join(cleanupErrors...)
}

func (r *Repository) deleteExpiredRecordingObjects(
	ctx context.Context,
	store storage.Store,
	limit int,
) (int, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, COALESCE(storage_key, '')
FROM channel_recordings
WHERE status = 'ready'
  AND expires_at IS NOT NULL
  AND expires_at <= now()
ORDER BY expires_at
LIMIT $1
`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type candidate struct {
		id  string
		key string
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.key); err != nil {
			return 0, err
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	rows.Close()
	processed := 0
	var cleanupErrors []error
	for _, item := range candidates {
		// External providers own URL-shaped recordings and enforce retention
		// through their adapter. App-managed object keys are deleted locally.
		if item.key != "" && !strings.Contains(item.key, "://") {
			if err := store.Delete(ctx, item.key); err != nil {
				cleanupErrors = append(cleanupErrors, err)
				continue
			}
		}
		command, err := r.pool.Exec(ctx, `
UPDATE channel_recordings
SET status = 'deleted',
    transcript = '{}'::jsonb,
    summary = '{}'::jsonb,
    storage_key = NULL
WHERE id = $1::uuid
  AND status = 'ready'
  AND expires_at <= now()
`, item.id)
		if err != nil {
			cleanupErrors = append(cleanupErrors, err)
			continue
		}
		processed += int(command.RowsAffected())
	}
	return processed, errors.Join(cleanupErrors...)
}
