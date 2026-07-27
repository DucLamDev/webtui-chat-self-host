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

func (r *Repository) GetRecordingPolicy(ctx context.Context, workspaceID string, channelID string) (channelsapp.RecordingPolicy, error) {
	return scanRecordingPolicy(r.pool.QueryRow(ctx, `
INSERT INTO channel_recording_policies (channel_id, workspace_id)
SELECT channel.id, channel.workspace_id
FROM channels channel
WHERE channel.workspace_id = $1::uuid
  AND channel.id = $2::uuid
  AND channel.deleted_at IS NULL
ON CONFLICT (channel_id) DO UPDATE SET channel_id = EXCLUDED.channel_id
RETURNING channel_id::text, workspace_id::text, enabled, consent_required,
          retention_days, transcription_enabled, summary_enabled, provider,
          updated_by::text, created_at, updated_at
`, workspaceID, channelID))
}

func (r *Repository) UpsertRecordingPolicy(ctx context.Context, params channelsapp.UpsertRecordingPolicyParams) (channelsapp.RecordingPolicy, error) {
	return scanRecordingPolicy(r.pool.QueryRow(ctx, `
INSERT INTO channel_recording_policies (
    channel_id, workspace_id, enabled, consent_required, retention_days,
    transcription_enabled, summary_enabled, provider, updated_by
)
SELECT channel.id, channel.workspace_id, $3, $4, $5, $6, $7, $8, $9::uuid
FROM channels channel
WHERE channel.workspace_id = $1::uuid
  AND channel.id = $2::uuid
  AND channel.deleted_at IS NULL
ON CONFLICT (channel_id) DO UPDATE SET
    enabled = EXCLUDED.enabled,
    consent_required = EXCLUDED.consent_required,
    retention_days = EXCLUDED.retention_days,
    transcription_enabled = EXCLUDED.transcription_enabled,
    summary_enabled = EXCLUDED.summary_enabled,
    provider = EXCLUDED.provider,
    updated_by = EXCLUDED.updated_by
RETURNING channel_id::text, workspace_id::text, enabled, consent_required,
          retention_days, transcription_enabled, summary_enabled, provider,
          updated_by::text, created_at, updated_at
`, params.WorkspaceID, params.ChannelID, params.Enabled, params.ConsentRequired,
		params.RetentionDays, params.TranscriptionEnabled, params.SummaryEnabled,
		params.Provider, params.ActorUserID))
}

func (r *Repository) ListRecordings(ctx context.Context, workspaceID string, channelID string) ([]channelsapp.Recording, error) {
	rows, err := r.pool.Query(ctx, recordingSelect+`
WHERE recording.workspace_id = $1::uuid
  AND recording.channel_id = $2::uuid
  AND recording.status <> 'deleted'
`+recordingGroupBy+`
ORDER BY recording.created_at DESC
`, workspaceID, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]channelsapp.Recording, 0)
	for rows.Next() {
		recording, scanErr := scanRecording(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, recording)
	}
	return result, rows.Err()
}

func (r *Repository) CreateRecording(ctx context.Context, params channelsapp.CreateRecordingParams) (channelsapp.Recording, error) {
	participants, err := json.Marshal(params.ParticipantUserIDs)
	if err != nil {
		return channelsapp.Recording{}, err
	}
	return scanRecording(r.pool.QueryRow(ctx, `
WITH inserted AS (
    INSERT INTO channel_recordings (
        workspace_id, channel_id, meeting_id, status, provider,
        participant_user_ids, started_by, started_at, expires_at,
        transcript_status, summary_status
    )
    SELECT channel.workspace_id, channel.id, NULLIF($3, '')::uuid,
           CASE WHEN $6 AND jsonb_array_length($5::jsonb) > 0 THEN 'pending' ELSE 'recording' END,
           $4, $5::jsonb, $10::uuid,
           CASE WHEN $6 AND jsonb_array_length($5::jsonb) > 0 THEN NULL ELSE now() END,
           now() + make_interval(days => $7),
           CASE WHEN $8 THEN 'pending' ELSE 'disabled' END,
           CASE WHEN $9 THEN 'pending' ELSE 'disabled' END
    FROM channels channel
    WHERE channel.workspace_id = $1::uuid
      AND channel.id = $2::uuid
      AND channel.deleted_at IS NULL
    RETURNING *
)
SELECT inserted.id::text, inserted.workspace_id::text, inserted.channel_id::text,
       inserted.meeting_id::text, inserted.status, inserted.provider,
       inserted.provider_recording_id, inserted.participant_user_ids::text,
       inserted.storage_key, inserted.mime_type, inserted.byte_size,
       inserted.checksum_sha256, inserted.started_by::text, inserted.started_at,
       inserted.ended_at, inserted.expires_at, inserted.transcript_status,
       inserted.transcript::text, inserted.summary_status, inserted.summary::text,
       inserted.error,
       0::integer AS consent_count,
       0::integer AS declined_count,
       jsonb_array_length(inserted.participant_user_ids)::integer AS participant_count,
       inserted.created_at, inserted.updated_at
FROM inserted
`, params.WorkspaceID, params.ChannelID, params.MeetingID, params.Provider,
		participants, params.ConsentRequired, params.RetentionDays, params.Transcription,
		params.Summary, params.ActorUserID))
}

func (r *Repository) SetRecordingConsent(ctx context.Context, params channelsapp.SetRecordingConsentParams) (channelsapp.Recording, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return channelsapp.Recording{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	command, err := tx.Exec(ctx, `
INSERT INTO channel_recording_consents (recording_id, user_id, consented)
SELECT recording.id, $4::uuid, $5
FROM channel_recordings recording
WHERE recording.workspace_id = $1::uuid
  AND recording.channel_id = $2::uuid
  AND recording.id = $3::uuid
  AND recording.status = 'pending'
  AND recording.participant_user_ids ? $4
ON CONFLICT (recording_id, user_id) DO UPDATE SET
    consented = EXCLUDED.consented,
    consented_at = now()
`, params.WorkspaceID, params.ChannelID, params.RecordingID, params.UserID, params.Consented)
	if err != nil {
		return channelsapp.Recording{}, err
	}
	if command.RowsAffected() == 0 {
		return channelsapp.Recording{}, channelsdomain.ErrChannelNotFound
	}
	if _, err := tx.Exec(ctx, `
UPDATE channel_recordings recording
SET status = 'recording',
    started_at = COALESCE(started_at, now())
WHERE recording.workspace_id = $1::uuid
  AND recording.channel_id = $2::uuid
  AND recording.id = $3::uuid
  AND recording.status = 'pending'
  AND jsonb_array_length(recording.participant_user_ids) > 0
  AND NOT EXISTS (
      SELECT 1
      FROM jsonb_array_elements_text(recording.participant_user_ids) participant(user_id)
      LEFT JOIN channel_recording_consents consent
        ON consent.recording_id = recording.id
       AND consent.user_id::text = participant.user_id
      WHERE consent.consented IS DISTINCT FROM true
  )
`, params.WorkspaceID, params.ChannelID, params.RecordingID); err != nil {
		return channelsapp.Recording{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return channelsapp.Recording{}, err
	}
	return r.getRecording(ctx, params.WorkspaceID, params.ChannelID, params.RecordingID)
}

func (r *Repository) TransitionRecording(ctx context.Context, params channelsapp.TransitionRecordingParams) (channelsapp.Recording, error) {
	if params.Action != "stop" {
		return channelsapp.Recording{}, channelsdomain.ErrChannelNotFound
	}
	recording, err := scanRecording(r.pool.QueryRow(ctx, recordingUpdateSelect(`
UPDATE channel_recordings recording
SET status = 'processing',
    ended_at = COALESCE(ended_at, now())
WHERE recording.workspace_id = $1::uuid
  AND recording.channel_id = $2::uuid
  AND recording.id = $3::uuid
  AND recording.status = 'recording'
RETURNING recording.*
`), params.WorkspaceID, params.ChannelID, params.RecordingID))
	if errors.Is(err, pgx.ErrNoRows) {
		return channelsapp.Recording{}, channelsdomain.ErrChannelNotFound
	}
	return recording, err
}

func (r *Repository) UpdateRecordingResult(ctx context.Context, params channelsapp.UpdateRecordingResultParams) (channelsapp.Recording, error) {
	recording, err := scanRecording(r.pool.QueryRow(ctx, recordingUpdateSelect(`
UPDATE channel_recordings recording
SET status = $4,
    provider_recording_id = COALESCE(NULLIF($5, ''), provider_recording_id),
    storage_key = COALESCE(NULLIF($6, ''), storage_key),
    mime_type = COALESCE(NULLIF($7, ''), mime_type),
    byte_size = COALESCE($8, byte_size),
    checksum_sha256 = COALESCE(NULLIF($9, ''), checksum_sha256),
    error = NULLIF($10, ''),
    ended_at = CASE WHEN $4 IN ('ready', 'failed') THEN COALESCE(ended_at, now()) ELSE ended_at END
WHERE recording.workspace_id = $1::uuid
  AND recording.channel_id = $2::uuid
  AND recording.id = $3::uuid
  AND recording.status IN ('recording', 'processing')
RETURNING recording.*
`), params.WorkspaceID, params.ChannelID, params.RecordingID, params.Status,
		params.ProviderRecordingID, params.StorageKey, params.MimeType,
		params.ByteSize, params.ChecksumSHA256, params.Error))
	if errors.Is(err, pgx.ErrNoRows) {
		return channelsapp.Recording{}, channelsdomain.ErrChannelNotFound
	}
	return recording, err
}

func (r *Repository) GetTalkIntegration(ctx context.Context, workspaceID string) (channelsapp.TalkIntegration, error) {
	return scanTalkIntegration(r.pool.QueryRow(ctx, `
INSERT INTO workspace_talk_integrations (workspace_id)
SELECT id FROM workspaces WHERE id = $1::uuid AND deleted_at IS NULL
ON CONFLICT (workspace_id) DO UPDATE SET workspace_id = EXCLUDED.workspace_id
RETURNING workspace_id::text, ai_enabled, ai_provider, transcription_provider,
          federation_enabled, e2ee_calls_enabled, sip_enabled, bridge_enabled,
          config::text, updated_by::text, created_at, updated_at
`, workspaceID))
}

func (r *Repository) UpsertTalkIntegration(ctx context.Context, params channelsapp.UpsertTalkIntegrationParams) (channelsapp.TalkIntegration, error) {
	return scanTalkIntegration(r.pool.QueryRow(ctx, `
INSERT INTO workspace_talk_integrations (
    workspace_id, ai_enabled, ai_provider, transcription_provider,
    federation_enabled, e2ee_calls_enabled, sip_enabled, bridge_enabled,
    config, updated_by
)
SELECT workspace.id, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::uuid
FROM workspaces workspace
WHERE workspace.id = $1::uuid AND workspace.deleted_at IS NULL
ON CONFLICT (workspace_id) DO UPDATE SET
    ai_enabled = EXCLUDED.ai_enabled,
    ai_provider = EXCLUDED.ai_provider,
    transcription_provider = EXCLUDED.transcription_provider,
    federation_enabled = EXCLUDED.federation_enabled,
    e2ee_calls_enabled = EXCLUDED.e2ee_calls_enabled,
    sip_enabled = EXCLUDED.sip_enabled,
    bridge_enabled = EXCLUDED.bridge_enabled,
    config = EXCLUDED.config,
    updated_by = EXCLUDED.updated_by
RETURNING workspace_id::text, ai_enabled, ai_provider, transcription_provider,
          federation_enabled, e2ee_calls_enabled, sip_enabled, bridge_enabled,
          config::text, updated_by::text, created_at, updated_at
`, params.WorkspaceID, params.AIEnabled, params.AIProvider,
		params.TranscriptionProvider, params.FederationEnabled,
		params.E2EECallsEnabled, params.SIPEnabled, params.BridgeEnabled,
		params.Config, params.ActorUserID))
}

func (r *Repository) ListFederationInvites(ctx context.Context, workspaceID string, channelID string) ([]channelsapp.FederationInvite, error) {
	rows, err := r.pool.Query(ctx, federationInviteSelect+`
WHERE workspace_id = $1::uuid AND channel_id = $2::uuid
ORDER BY created_at DESC
`, workspaceID, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]channelsapp.FederationInvite, 0)
	for rows.Next() {
		invite, scanErr := scanFederationInvite(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, invite)
	}
	return result, rows.Err()
}

func (r *Repository) CreateFederationInvite(ctx context.Context, params channelsapp.CreateFederationInviteParams) (channelsapp.FederationInvite, error) {
	return scanFederationInvite(r.pool.QueryRow(ctx, `
INSERT INTO federated_conversation_invites (
    workspace_id, channel_id, remote_server, remote_user,
    direction, protocol, payload, created_by
)
SELECT channel.workspace_id, channel.id, $3, $4, 'outbound', $5, $6::jsonb, $7::uuid
FROM channels channel
WHERE channel.workspace_id = $1::uuid
  AND channel.id = $2::uuid
  AND channel.deleted_at IS NULL
RETURNING id::text, workspace_id::text, channel_id::text, remote_server,
          remote_user, direction, status, protocol, payload::text,
          created_by::text, responded_at, created_at, updated_at
`, params.WorkspaceID, params.ChannelID, params.RemoteServer, params.RemoteUser,
		params.Protocol, params.Payload, params.ActorUserID))
}

func (r *Repository) TransitionFederationInvite(ctx context.Context, params channelsapp.TransitionFederationInviteParams) (channelsapp.FederationInvite, error) {
	invite, err := scanFederationInvite(r.pool.QueryRow(ctx, `
UPDATE federated_conversation_invites
SET status = $4,
    responded_at = CASE WHEN $4 IN ('accepted', 'declined') THEN now() ELSE responded_at END
WHERE workspace_id = $1::uuid
  AND channel_id = $2::uuid
  AND id = $3::uuid
  AND status = 'pending'
RETURNING id::text, workspace_id::text, channel_id::text, remote_server,
          remote_user, direction, status, protocol, payload::text,
          created_by::text, responded_at, created_at, updated_at
`, params.WorkspaceID, params.ChannelID, params.InviteID, params.Status))
	if errors.Is(err, pgx.ErrNoRows) {
		return channelsapp.FederationInvite{}, channelsdomain.ErrChannelNotFound
	}
	return invite, err
}

const recordingSelect = `
SELECT recording.id::text, recording.workspace_id::text, recording.channel_id::text,
       recording.meeting_id::text, recording.status, recording.provider,
       recording.provider_recording_id, recording.participant_user_ids::text,
       recording.storage_key, recording.mime_type, recording.byte_size,
       recording.checksum_sha256, recording.started_by::text, recording.started_at,
       recording.ended_at, recording.expires_at, recording.transcript_status,
       recording.transcript::text, recording.summary_status, recording.summary::text,
       recording.error,
       count(consent.user_id) FILTER (WHERE consent.consented = true)::integer AS consent_count,
       count(consent.user_id) FILTER (WHERE consent.consented = false)::integer AS declined_count,
       jsonb_array_length(recording.participant_user_ids)::integer AS participant_count,
       recording.created_at, recording.updated_at
FROM channel_recordings recording
LEFT JOIN channel_recording_consents consent ON consent.recording_id = recording.id
`

const recordingGroupBy = `
GROUP BY recording.id
`

func recordingUpdateSelect(updateQuery string) string {
	return `
WITH updated AS (
` + updateQuery + `
)
SELECT updated.id::text, updated.workspace_id::text, updated.channel_id::text,
       updated.meeting_id::text, updated.status, updated.provider,
       updated.provider_recording_id, updated.participant_user_ids::text,
       updated.storage_key, updated.mime_type, updated.byte_size,
       updated.checksum_sha256, updated.started_by::text, updated.started_at,
       updated.ended_at, updated.expires_at, updated.transcript_status,
       updated.transcript::text, updated.summary_status, updated.summary::text,
       updated.error,
       count(consent.user_id) FILTER (WHERE consent.consented = true)::integer,
       count(consent.user_id) FILTER (WHERE consent.consented = false)::integer,
       jsonb_array_length(updated.participant_user_ids)::integer,
       updated.created_at, updated.updated_at
FROM updated
LEFT JOIN channel_recording_consents consent ON consent.recording_id = updated.id
GROUP BY updated.id, updated.workspace_id, updated.channel_id, updated.meeting_id,
         updated.status, updated.provider, updated.provider_recording_id,
         updated.participant_user_ids, updated.storage_key, updated.mime_type,
         updated.byte_size, updated.checksum_sha256, updated.started_by,
         updated.started_at, updated.ended_at, updated.expires_at,
         updated.transcript_status, updated.transcript, updated.summary_status,
         updated.summary, updated.error, updated.created_at, updated.updated_at
`
}

func (r *Repository) getRecording(ctx context.Context, workspaceID string, channelID string, recordingID string) (channelsapp.Recording, error) {
	return scanRecording(r.pool.QueryRow(ctx, recordingSelect+`
WHERE recording.workspace_id = $1::uuid
  AND recording.channel_id = $2::uuid
  AND recording.id = $3::uuid
`+recordingGroupBy, workspaceID, channelID, recordingID))
}

func scanRecordingPolicy(row rowScanner) (channelsapp.RecordingPolicy, error) {
	var policy channelsapp.RecordingPolicy
	var updatedBy sql.NullString
	if err := row.Scan(
		&policy.ChannelID, &policy.WorkspaceID, &policy.Enabled,
		&policy.ConsentRequired, &policy.RetentionDays,
		&policy.TranscriptionEnabled, &policy.SummaryEnabled, &policy.Provider,
		&updatedBy, &policy.CreatedAt, &policy.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsapp.RecordingPolicy{}, channelsdomain.ErrChannelNotFound
		}
		return channelsapp.RecordingPolicy{}, err
	}
	policy.UpdatedBy = nullStringPtr(updatedBy)
	return policy, nil
}

func scanRecording(row rowScanner) (channelsapp.Recording, error) {
	var recording channelsapp.Recording
	var meetingID, providerRecordingID, storageKey, mimeType, checksum sql.NullString
	var startedBy, errorText sql.NullString
	var byteSize sql.NullInt64
	var startedAt, endedAt, expiresAt sql.NullTime
	var participants, transcript, summary string
	if err := row.Scan(
		&recording.ID, &recording.WorkspaceID, &recording.ChannelID, &meetingID,
		&recording.Status, &recording.Provider, &providerRecordingID, &participants,
		&storageKey, &mimeType, &byteSize, &checksum, &startedBy, &startedAt,
		&endedAt, &expiresAt, &recording.TranscriptStatus, &transcript,
		&recording.SummaryStatus, &summary, &errorText, &recording.ConsentCount,
		&recording.DeclinedCount, &recording.ParticipantCount,
		&recording.CreatedAt, &recording.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsapp.Recording{}, channelsdomain.ErrChannelNotFound
		}
		return channelsapp.Recording{}, err
	}
	recording.MeetingID = nullStringPtr(meetingID)
	recording.ProviderRecordingID = nullStringPtr(providerRecordingID)
	recording.ParticipantUserIDs = json.RawMessage(participants)
	recording.StorageKey = nullStringPtr(storageKey)
	recording.MimeType = nullStringPtr(mimeType)
	if byteSize.Valid {
		value := byteSize.Int64
		recording.ByteSize = &value
	}
	recording.ChecksumSHA256 = nullStringPtr(checksum)
	recording.StartedBy = nullStringPtr(startedBy)
	recording.StartedAt = nullTimePtr(startedAt)
	recording.EndedAt = nullTimePtr(endedAt)
	recording.ExpiresAt = nullTimePtr(expiresAt)
	recording.Transcript = json.RawMessage(transcript)
	recording.Summary = json.RawMessage(summary)
	recording.Error = nullStringPtr(errorText)
	return recording, nil
}

func scanTalkIntegration(row rowScanner) (channelsapp.TalkIntegration, error) {
	var integration channelsapp.TalkIntegration
	var config string
	var updatedBy sql.NullString
	if err := row.Scan(
		&integration.WorkspaceID, &integration.AIEnabled, &integration.AIProvider,
		&integration.TranscriptionProvider, &integration.FederationEnabled,
		&integration.E2EECallsEnabled, &integration.SIPEnabled,
		&integration.BridgeEnabled, &config, &updatedBy,
		&integration.CreatedAt, &integration.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsapp.TalkIntegration{}, channelsdomain.ErrChannelNotFound
		}
		return channelsapp.TalkIntegration{}, err
	}
	integration.Config = json.RawMessage(config)
	integration.UpdatedBy = nullStringPtr(updatedBy)
	return integration, nil
}

const federationInviteSelect = `
SELECT id::text, workspace_id::text, channel_id::text, remote_server,
       remote_user, direction, status, protocol, payload::text,
       created_by::text, responded_at, created_at, updated_at
FROM federated_conversation_invites
`

func scanFederationInvite(row rowScanner) (channelsapp.FederationInvite, error) {
	var invite channelsapp.FederationInvite
	var payload string
	var createdBy sql.NullString
	var respondedAt sql.NullTime
	if err := row.Scan(
		&invite.ID, &invite.WorkspaceID, &invite.ChannelID, &invite.RemoteServer,
		&invite.RemoteUser, &invite.Direction, &invite.Status, &invite.Protocol,
		&payload, &createdBy, &respondedAt, &invite.CreatedAt, &invite.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsapp.FederationInvite{}, channelsdomain.ErrChannelNotFound
		}
		return channelsapp.FederationInvite{}, err
	}
	invite.Payload = json.RawMessage(payload)
	invite.CreatedBy = nullStringPtr(createdBy)
	invite.RespondedAt = nullTimePtr(respondedAt)
	return invite, nil
}
