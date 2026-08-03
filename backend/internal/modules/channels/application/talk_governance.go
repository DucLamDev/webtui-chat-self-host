package application

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const maxTalkIntegrationConfigBytes = 64 * 1024

type TalkGovernanceRepository interface {
	GetRecordingPolicy(ctx context.Context, workspaceID string, channelID string) (RecordingPolicy, error)
	UpsertRecordingPolicy(ctx context.Context, params UpsertRecordingPolicyParams) (RecordingPolicy, error)
	ListRecordings(ctx context.Context, workspaceID string, channelID string) ([]Recording, error)
	CreateRecording(ctx context.Context, params CreateRecordingParams) (Recording, error)
	SetRecordingConsent(ctx context.Context, params SetRecordingConsentParams) (Recording, error)
	TransitionRecording(ctx context.Context, params TransitionRecordingParams) (Recording, error)
	UpdateRecordingResult(ctx context.Context, params UpdateRecordingResultParams) (Recording, error)
	GetTalkIntegration(ctx context.Context, workspaceID string) (TalkIntegration, error)
	UpsertTalkIntegration(ctx context.Context, params UpsertTalkIntegrationParams) (TalkIntegration, error)
	ListFederationInvites(ctx context.Context, workspaceID string, channelID string) ([]FederationInvite, error)
	CreateFederationInvite(ctx context.Context, params CreateFederationInviteParams) (FederationInvite, error)
	TransitionFederationInvite(ctx context.Context, params TransitionFederationInviteParams) (FederationInvite, error)
}

type RecordingPolicy struct {
	ChannelID            string
	WorkspaceID          string
	Enabled              bool
	ConsentRequired      bool
	RetentionDays        int
	TranscriptionEnabled bool
	SummaryEnabled       bool
	Provider             string
	UpdatedBy            *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type RecordingPolicyDTO struct {
	ChannelID            string  `json:"channel_id"`
	WorkspaceID          string  `json:"workspace_id"`
	Enabled              bool    `json:"enabled"`
	ConsentRequired      bool    `json:"consent_required"`
	RetentionDays        int     `json:"retention_days"`
	TranscriptionEnabled bool    `json:"transcription_enabled"`
	SummaryEnabled       bool    `json:"summary_enabled"`
	Provider             string  `json:"provider"`
	UpdatedBy            *string `json:"updated_by,omitempty"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

type UpsertRecordingPolicyParams struct {
	WorkspaceID          string
	ChannelID            string
	Enabled              bool
	ConsentRequired      bool
	RetentionDays        int
	TranscriptionEnabled bool
	SummaryEnabled       bool
	Provider             string
	ActorUserID          string
}

type UpdateRecordingPolicyInput struct {
	ActorUserID          string
	WorkspaceID          string
	ChannelID            string
	Enabled              bool
	ConsentRequired      bool
	RetentionDays        int
	TranscriptionEnabled bool
	SummaryEnabled       bool
	Provider             string
}

type Recording struct {
	ID                  string
	WorkspaceID         string
	ChannelID           string
	MeetingID           *string
	Status              string
	Provider            string
	ProviderRecordingID *string
	ParticipantUserIDs  json.RawMessage
	StorageKey          *string
	MimeType            *string
	ByteSize            *int64
	ChecksumSHA256      *string
	StartedBy           *string
	StartedAt           *time.Time
	EndedAt             *time.Time
	ExpiresAt           *time.Time
	TranscriptStatus    string
	Transcript          json.RawMessage
	SummaryStatus       string
	Summary             json.RawMessage
	Error               *string
	ConsentCount        int
	DeclinedCount       int
	ParticipantCount    int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type RecordingDTO struct {
	ID                  string          `json:"id"`
	WorkspaceID         string          `json:"workspace_id"`
	ChannelID           string          `json:"channel_id"`
	MeetingID           *string         `json:"meeting_id,omitempty"`
	Status              string          `json:"status"`
	Provider            string          `json:"provider"`
	ProviderRecordingID *string         `json:"provider_recording_id,omitempty"`
	ParticipantUserIDs  []string        `json:"participant_user_ids"`
	MimeType            *string         `json:"mime_type,omitempty"`
	ByteSize            *int64          `json:"byte_size,omitempty"`
	ChecksumSHA256      *string         `json:"checksum_sha256,omitempty"`
	StartedBy           *string         `json:"started_by,omitempty"`
	StartedAt           *string         `json:"started_at,omitempty"`
	EndedAt             *string         `json:"ended_at,omitempty"`
	ExpiresAt           *string         `json:"expires_at,omitempty"`
	TranscriptStatus    string          `json:"transcript_status"`
	Transcript          json.RawMessage `json:"transcript"`
	SummaryStatus       string          `json:"summary_status"`
	Summary             json.RawMessage `json:"summary"`
	Error               *string         `json:"error,omitempty"`
	ConsentCount        int             `json:"consent_count"`
	DeclinedCount       int             `json:"declined_count"`
	ParticipantCount    int             `json:"participant_count"`
	ReadyToStart        bool            `json:"ready_to_start"`
	CreatedAt           string          `json:"created_at"`
	UpdatedAt           string          `json:"updated_at"`
}

type CreateRecordingParams struct {
	WorkspaceID        string
	ChannelID          string
	MeetingID          string
	Provider           string
	ParticipantUserIDs []string
	ConsentRequired    bool
	RetentionDays      int
	Transcription      bool
	Summary            bool
	ActorUserID        string
}

type StartRecordingInput struct {
	ActorUserID        string
	WorkspaceID        string
	ChannelID          string
	MeetingID          string
	ParticipantUserIDs []string
}

type SetRecordingConsentParams struct {
	WorkspaceID string
	ChannelID   string
	RecordingID string
	UserID      string
	Consented   bool
}

type TransitionRecordingParams struct {
	WorkspaceID string
	ChannelID   string
	RecordingID string
	Action      string
	ActorUserID string
}

type UpdateRecordingResultParams struct {
	WorkspaceID         string
	ChannelID           string
	RecordingID         string
	Status              string
	ProviderRecordingID string
	StorageKey          string
	MimeType            string
	ByteSize            *int64
	ChecksumSHA256      string
	Error               string
}

type RecordingResultInput struct {
	ActorUserID         string
	WorkspaceID         string
	ChannelID           string
	RecordingID         string
	Status              string
	ProviderRecordingID string
	StorageKey          string
	MimeType            string
	ByteSize            *int64
	ChecksumSHA256      string
	Error               string
}

type TalkIntegration struct {
	WorkspaceID           string
	AIEnabled             bool
	AIProvider            string
	TranscriptionProvider string
	FederationEnabled     bool
	E2EECallsEnabled      bool
	SIPEnabled            bool
	BridgeEnabled         bool
	Config                json.RawMessage
	UpdatedBy             *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type TalkIntegrationDTO struct {
	WorkspaceID           string          `json:"workspace_id"`
	AIEnabled             bool            `json:"ai_enabled"`
	AIProvider            string          `json:"ai_provider"`
	TranscriptionProvider string          `json:"transcription_provider"`
	FederationEnabled     bool            `json:"federation_enabled"`
	E2EECallsEnabled      bool            `json:"e2ee_calls_enabled"`
	SIPEnabled            bool            `json:"sip_enabled"`
	BridgeEnabled         bool            `json:"bridge_enabled"`
	Config                json.RawMessage `json:"config"`
	UpdatedBy             *string         `json:"updated_by,omitempty"`
	CreatedAt             string          `json:"created_at"`
	UpdatedAt             string          `json:"updated_at"`
}

type UpsertTalkIntegrationParams struct {
	WorkspaceID           string
	AIEnabled             bool
	AIProvider            string
	TranscriptionProvider string
	FederationEnabled     bool
	E2EECallsEnabled      bool
	SIPEnabled            bool
	BridgeEnabled         bool
	Config                json.RawMessage
	ActorUserID           string
}

type UpdateTalkIntegrationInput struct {
	ActorUserID           string
	WorkspaceID           string
	AIEnabled             bool
	AIProvider            string
	TranscriptionProvider string
	FederationEnabled     bool
	E2EECallsEnabled      bool
	SIPEnabled            bool
	BridgeEnabled         bool
	Config                json.RawMessage
}

type FederationInvite struct {
	ID           string
	WorkspaceID  string
	ChannelID    string
	RemoteServer string
	RemoteUser   string
	Direction    string
	Status       string
	Protocol     string
	Payload      json.RawMessage
	CreatedBy    *string
	RespondedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type FederationInviteDTO struct {
	ID           string          `json:"id"`
	WorkspaceID  string          `json:"workspace_id"`
	ChannelID    string          `json:"channel_id"`
	RemoteServer string          `json:"remote_server"`
	RemoteUser   string          `json:"remote_user"`
	Direction    string          `json:"direction"`
	Status       string          `json:"status"`
	Protocol     string          `json:"protocol"`
	Payload      json.RawMessage `json:"payload"`
	CreatedBy    *string         `json:"created_by,omitempty"`
	RespondedAt  *string         `json:"responded_at,omitempty"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

type CreateFederationInviteParams struct {
	WorkspaceID  string
	ChannelID    string
	RemoteServer string
	RemoteUser   string
	Protocol     string
	Payload      json.RawMessage
	ActorUserID  string
}

type CreateFederationInviteInput struct {
	ActorUserID  string
	WorkspaceID  string
	ChannelID    string
	RemoteServer string
	RemoteUser   string
	Protocol     string
	Payload      json.RawMessage
}

type TransitionFederationInviteParams struct {
	WorkspaceID string
	ChannelID   string
	InviteID    string
	Status      string
	ActorUserID string
}

func (s *Service) GetRecordingPolicy(ctx context.Context, actorUserID string, workspaceID string, channelID string) (RecordingPolicyDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return RecordingPolicyDTO{}, err
	}
	policy, err := s.talkGovernanceRepository().GetRecordingPolicy(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return RecordingPolicyDTO{}, err
	}
	return toRecordingPolicyDTO(policy), nil
}

func (s *Service) UpdateRecordingPolicy(ctx context.Context, input UpdateRecordingPolicyInput) (RecordingPolicyDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return RecordingPolicyDTO{}, err
	}
	if input.RetentionDays == 0 {
		input.RetentionDays = 30
	}
	if input.RetentionDays < 1 || input.RetentionDays > 3650 {
		return RecordingPolicyDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "retention_days must be between 1 and 3650.")
	}
	provider := strings.TrimSpace(input.Provider)
	if provider == "" {
		provider = "jibri"
	}
	switch provider {
	case "jibri", "external", "disabled":
	default:
		return RecordingPolicyDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Unsupported recording provider.")
	}
	policy, err := s.talkGovernanceRepository().UpsertRecordingPolicy(ctx, UpsertRecordingPolicyParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID), ChannelID: strings.TrimSpace(input.ChannelID),
		Enabled: input.Enabled, ConsentRequired: input.ConsentRequired, RetentionDays: input.RetentionDays,
		TranscriptionEnabled: input.TranscriptionEnabled, SummaryEnabled: input.SummaryEnabled,
		Provider: provider, ActorUserID: strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return RecordingPolicyDTO{}, err
	}
	return toRecordingPolicyDTO(policy), nil
}

func (s *Service) ListRecordings(ctx context.Context, actorUserID string, workspaceID string, channelID string) ([]RecordingDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	recordings, err := s.talkGovernanceRepository().ListRecordings(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return nil, err
	}
	return toRecordingDTOs(recordings), nil
}

func (s *Service) StartRecording(ctx context.Context, input StartRecordingInput) (RecordingDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return RecordingDTO{}, err
	}
	policy, err := s.talkGovernanceRepository().GetRecordingPolicy(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.ChannelID))
	if err != nil {
		return RecordingDTO{}, err
	}
	if !policy.Enabled || policy.Provider == "disabled" {
		return RecordingDTO{}, apperrors.Conflict("RECORDING_DISABLED", "Recording is disabled for this conversation.")
	}
	participants := normalizeStringIDs(input.ParticipantUserIDs)
	if len(participants) == 0 {
		members, listErr := s.repo.ListMembers(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.ChannelID))
		if listErr != nil {
			return RecordingDTO{}, listErr
		}
		for _, member := range members {
			if member.Status == "active" || member.Status == "muted" {
				participants = append(participants, member.UserID)
			}
		}
	}
	if len(participants) > 250 {
		return RecordingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "A recording supports at most 250 participants.")
	}
	for _, userID := range participants {
		member, findErr := s.repo.FindMember(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.ChannelID), userID)
		if findErr != nil || (member.Status != "active" && member.Status != "muted") {
			return RecordingDTO{}, apperrors.BadRequest("INVALID_PARTICIPANTS", "A recording participant is not an active channel member.")
		}
	}
	recording, err := s.talkGovernanceRepository().CreateRecording(ctx, CreateRecordingParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID), ChannelID: strings.TrimSpace(input.ChannelID),
		MeetingID: strings.TrimSpace(input.MeetingID), Provider: policy.Provider,
		ParticipantUserIDs: participants, ConsentRequired: policy.ConsentRequired,
		RetentionDays: policy.RetentionDays, Transcription: policy.TranscriptionEnabled,
		Summary: policy.SummaryEnabled, ActorUserID: strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return RecordingDTO{}, err
	}
	return toRecordingDTO(recording), nil
}

func (s *Service) SetRecordingConsent(ctx context.Context, actorUserID string, workspaceID string, channelID string, recordingID string, consented bool) (RecordingDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return RecordingDTO{}, err
	}
	recording, err := s.talkGovernanceRepository().SetRecordingConsent(ctx, SetRecordingConsentParams{
		WorkspaceID: strings.TrimSpace(workspaceID), ChannelID: strings.TrimSpace(channelID),
		RecordingID: strings.TrimSpace(recordingID), UserID: strings.TrimSpace(actorUserID),
		Consented: consented,
	})
	if err != nil {
		return RecordingDTO{}, err
	}
	return toRecordingDTO(recording), nil
}

func (s *Service) StopRecording(ctx context.Context, actorUserID string, workspaceID string, channelID string, recordingID string) (RecordingDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return RecordingDTO{}, err
	}
	recording, err := s.talkGovernanceRepository().TransitionRecording(ctx, TransitionRecordingParams{
		WorkspaceID: strings.TrimSpace(workspaceID), ChannelID: strings.TrimSpace(channelID),
		RecordingID: strings.TrimSpace(recordingID), Action: "stop", ActorUserID: strings.TrimSpace(actorUserID),
	})
	if err != nil {
		return RecordingDTO{}, err
	}
	return toRecordingDTO(recording), nil
}

func (s *Service) UpdateRecordingResult(ctx context.Context, input RecordingResultInput) (RecordingDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return RecordingDTO{}, err
	}
	status := strings.TrimSpace(input.Status)
	if status != "processing" && status != "ready" && status != "failed" {
		return RecordingDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Recording result status must be processing, ready or failed.")
	}
	recording, err := s.talkGovernanceRepository().UpdateRecordingResult(ctx, UpdateRecordingResultParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID), ChannelID: strings.TrimSpace(input.ChannelID),
		RecordingID: strings.TrimSpace(input.RecordingID), Status: status,
		ProviderRecordingID: strings.TrimSpace(input.ProviderRecordingID), StorageKey: strings.TrimSpace(input.StorageKey),
		MimeType: strings.TrimSpace(input.MimeType), ByteSize: input.ByteSize,
		ChecksumSHA256: strings.TrimSpace(input.ChecksumSHA256), Error: strings.TrimSpace(input.Error),
	})
	if err != nil {
		return RecordingDTO{}, err
	}
	return toRecordingDTO(recording), nil
}

func (s *Service) GetTalkIntegration(ctx context.Context, actorUserID string, workspaceID string) (TalkIntegrationDTO, error) {
	if err := s.ensureWorkspaceAccess(ctx, actorUserID, workspaceID); err != nil {
		return TalkIntegrationDTO{}, err
	}
	integration, err := s.talkGovernanceRepository().GetTalkIntegration(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return TalkIntegrationDTO{}, err
	}
	return toTalkIntegrationDTO(integration), nil
}

func (s *Service) UpdateTalkIntegration(ctx context.Context, input UpdateTalkIntegrationInput) (TalkIntegrationDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "workspace.manage"); err != nil {
		return TalkIntegrationDTO{}, err
	}
	if input.E2EECallsEnabled {
		return TalkIntegrationDTO{}, apperrors.Conflict(
			"E2EE_CALLS_NOT_AVAILABLE",
			"E2EE cuộc gọi chưa có key agreement và xác minh thiết bị; không thể bật cờ bảo mật gây hiểu nhầm.",
		)
	}
	config := input.Config
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	if len(config) > maxTalkIntegrationConfigBytes || !json.Valid(config) || config[0] != '{' {
		return TalkIntegrationDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "config must be a JSON object up to 64 KiB.")
	}
	aiProvider := strings.TrimSpace(input.AIProvider)
	if aiProvider == "" {
		aiProvider = "ollama"
	}
	transcriptionProvider := strings.TrimSpace(input.TranscriptionProvider)
	if transcriptionProvider == "" {
		transcriptionProvider = "faster_whisper"
	}
	integration, err := s.talkGovernanceRepository().UpsertTalkIntegration(ctx, UpsertTalkIntegrationParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID), AIEnabled: input.AIEnabled,
		AIProvider: aiProvider, TranscriptionProvider: transcriptionProvider,
		FederationEnabled: input.FederationEnabled, E2EECallsEnabled: input.E2EECallsEnabled,
		SIPEnabled: input.SIPEnabled, BridgeEnabled: input.BridgeEnabled,
		Config: config, ActorUserID: strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return TalkIntegrationDTO{}, err
	}
	return toTalkIntegrationDTO(integration), nil
}

func (s *Service) ListFederationInvites(ctx context.Context, actorUserID string, workspaceID string, channelID string) ([]FederationInviteDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	integration, err := s.talkGovernanceRepository().GetTalkIntegration(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	if !integration.FederationEnabled {
		return nil, apperrors.Conflict("FEDERATION_DISABLED", "Federation is disabled for this workspace.")
	}
	invites, err := s.talkGovernanceRepository().ListFederationInvites(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return nil, err
	}
	return toFederationInviteDTOs(invites), nil
}

func (s *Service) CreateFederationInvite(ctx context.Context, input CreateFederationInviteInput) (FederationInviteDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return FederationInviteDTO{}, err
	}
	integration, err := s.talkGovernanceRepository().GetTalkIntegration(ctx, strings.TrimSpace(input.WorkspaceID))
	if err != nil {
		return FederationInviteDTO{}, err
	}
	if !integration.FederationEnabled {
		return FederationInviteDTO{}, apperrors.Conflict("FEDERATION_DISABLED", "Federation is disabled for this workspace.")
	}
	remoteServer := strings.TrimRight(strings.TrimSpace(input.RemoteServer), "/")
	parsed, err := url.Parse(remoteServer)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return FederationInviteDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "remote_server must be an HTTPS origin.")
	}
	remoteUser := strings.TrimSpace(input.RemoteUser)
	if remoteUser == "" || len([]rune(remoteUser)) > 255 {
		return FederationInviteDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "remote_user is required.")
	}
	protocol := strings.TrimSpace(input.Protocol)
	if protocol == "" {
		protocol = "open_cloud_mesh"
	}
	if protocol != "open_cloud_mesh" && protocol != "talk_federation" {
		return FederationInviteDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Unsupported federation protocol.")
	}
	payload := input.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if !json.Valid(payload) || len(payload) > maxTalkIntegrationConfigBytes {
		return FederationInviteDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "payload must be valid JSON up to 64 KiB.")
	}
	invite, err := s.talkGovernanceRepository().CreateFederationInvite(ctx, CreateFederationInviteParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID), ChannelID: strings.TrimSpace(input.ChannelID),
		RemoteServer: remoteServer, RemoteUser: remoteUser, Protocol: protocol,
		Payload: payload, ActorUserID: strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return FederationInviteDTO{}, err
	}
	return toFederationInviteDTO(invite), nil
}

func (s *Service) TransitionFederationInvite(ctx context.Context, actorUserID string, workspaceID string, channelID string, inviteID string, status string) (FederationInviteDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return FederationInviteDTO{}, err
	}
	status = strings.TrimSpace(status)
	if status != "accepted" && status != "declined" && status != "revoked" && status != "failed" {
		return FederationInviteDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Unsupported federation invite status.")
	}
	invite, err := s.talkGovernanceRepository().TransitionFederationInvite(ctx, TransitionFederationInviteParams{
		WorkspaceID: strings.TrimSpace(workspaceID), ChannelID: strings.TrimSpace(channelID),
		InviteID: strings.TrimSpace(inviteID), Status: status, ActorUserID: strings.TrimSpace(actorUserID),
	})
	if err != nil {
		return FederationInviteDTO{}, err
	}
	return toFederationInviteDTO(invite), nil
}

func (s *Service) talkGovernanceRepository() TalkGovernanceRepository {
	repository, ok := s.collab.(TalkGovernanceRepository)
	if !ok {
		return unavailableTalkRepository{}
	}
	return repository
}

func toRecordingPolicyDTO(policy RecordingPolicy) RecordingPolicyDTO {
	return RecordingPolicyDTO{
		ChannelID: policy.ChannelID, WorkspaceID: policy.WorkspaceID, Enabled: policy.Enabled,
		ConsentRequired: policy.ConsentRequired, RetentionDays: policy.RetentionDays,
		TranscriptionEnabled: policy.TranscriptionEnabled, SummaryEnabled: policy.SummaryEnabled,
		Provider: policy.Provider, UpdatedBy: policy.UpdatedBy,
		CreatedAt: formatTime(policy.CreatedAt), UpdatedAt: formatTime(policy.UpdatedAt),
	}
}

func toRecordingDTOs(recordings []Recording) []RecordingDTO {
	result := make([]RecordingDTO, 0, len(recordings))
	for _, recording := range recordings {
		result = append(result, toRecordingDTO(recording))
	}
	return result
}

func toRecordingDTO(recording Recording) RecordingDTO {
	var participants []string
	_ = json.Unmarshal(recording.ParticipantUserIDs, &participants)
	return RecordingDTO{
		ID: recording.ID, WorkspaceID: recording.WorkspaceID, ChannelID: recording.ChannelID,
		MeetingID: recording.MeetingID, Status: recording.Status, Provider: recording.Provider,
		ProviderRecordingID: recording.ProviderRecordingID, ParticipantUserIDs: participants,
		MimeType: recording.MimeType, ByteSize: recording.ByteSize, ChecksumSHA256: recording.ChecksumSHA256,
		StartedBy: recording.StartedBy, StartedAt: formatOptionalTime(recording.StartedAt),
		EndedAt: formatOptionalTime(recording.EndedAt), ExpiresAt: formatOptionalTime(recording.ExpiresAt),
		TranscriptStatus: recording.TranscriptStatus, Transcript: recording.Transcript,
		SummaryStatus: recording.SummaryStatus, Summary: recording.Summary, Error: recording.Error,
		ConsentCount: recording.ConsentCount, DeclinedCount: recording.DeclinedCount,
		ParticipantCount: recording.ParticipantCount,
		ReadyToStart:     recording.ParticipantCount > 0 && recording.ConsentCount == recording.ParticipantCount && recording.DeclinedCount == 0,
		CreatedAt:        formatTime(recording.CreatedAt), UpdatedAt: formatTime(recording.UpdatedAt),
	}
}

func toTalkIntegrationDTO(integration TalkIntegration) TalkIntegrationDTO {
	return TalkIntegrationDTO{
		WorkspaceID: integration.WorkspaceID, AIEnabled: integration.AIEnabled,
		AIProvider: integration.AIProvider, TranscriptionProvider: integration.TranscriptionProvider,
		FederationEnabled: integration.FederationEnabled, E2EECallsEnabled: integration.E2EECallsEnabled,
		SIPEnabled: integration.SIPEnabled, BridgeEnabled: integration.BridgeEnabled,
		Config: integration.Config, UpdatedBy: integration.UpdatedBy,
		CreatedAt: formatTime(integration.CreatedAt), UpdatedAt: formatTime(integration.UpdatedAt),
	}
}

func toFederationInviteDTOs(invites []FederationInvite) []FederationInviteDTO {
	result := make([]FederationInviteDTO, 0, len(invites))
	for _, invite := range invites {
		result = append(result, toFederationInviteDTO(invite))
	}
	return result
}

func toFederationInviteDTO(invite FederationInvite) FederationInviteDTO {
	return FederationInviteDTO{
		ID: invite.ID, WorkspaceID: invite.WorkspaceID, ChannelID: invite.ChannelID,
		RemoteServer: invite.RemoteServer, RemoteUser: invite.RemoteUser,
		Direction: invite.Direction, Status: invite.Status, Protocol: invite.Protocol,
		Payload: invite.Payload, CreatedBy: invite.CreatedBy,
		RespondedAt: formatOptionalTime(invite.RespondedAt),
		CreatedAt:   formatTime(invite.CreatedAt), UpdatedAt: formatTime(invite.UpdatedAt),
	}
}
