package application

import (
	"context"
	"time"

	channelsdomain "github.com/duclamdev/application-chat/backend/internal/modules/channels/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type unavailableTalkRepository struct{}

func talkCapabilityUnavailable() error {
	return apperrors.ServiceUnavailable(
		"TALK_REPOSITORY_UNAVAILABLE",
		"Talk features are not available with the configured channels repository.",
	)
}

func (unavailableTalkRepository) ListMeetings(context.Context, string, string, *time.Time, *time.Time) ([]Meeting, error) {
	return nil, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) CreateMeeting(context.Context, CreateMeetingParams) (Meeting, error) {
	return Meeting{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) TransitionMeeting(context.Context, TransitionMeetingParams) (Meeting, error) {
	return Meeting{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) GetVoiceRoom(context.Context, string, string) (VoiceRoom, error) {
	return VoiceRoom{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) SetVoiceRoom(context.Context, SetVoiceRoomParams) (VoiceRoom, error) {
	return VoiceRoom{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) ReplaceBreakoutRooms(context.Context, ReplaceBreakoutRoomsParams) ([]channelsdomain.BreakoutRoom, error) {
	return nil, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) SetBreakoutRoomsStatus(context.Context, string, string, string) ([]channelsdomain.BreakoutRoom, error) {
	return nil, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) JoinBreakoutRoom(context.Context, JoinBreakoutRoomParams) ([]channelsdomain.BreakoutRoom, error) {
	return nil, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) UpdateBreakoutAssignments(context.Context, UpdateBreakoutAssignmentsParams) ([]channelsdomain.BreakoutRoom, error) {
	return nil, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) CreateBreakoutBroadcast(context.Context, CreateBreakoutBroadcastParams) (BreakoutBroadcast, error) {
	return BreakoutBroadcast{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) GetTalkHome(context.Context, string, string, time.Time) (TalkHome, error) {
	return TalkHome{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) ListSharedItems(context.Context, string, string, string, int) ([]SharedItem, error) {
	return nil, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) GetRecordingPolicy(context.Context, string, string) (RecordingPolicy, error) {
	return RecordingPolicy{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) UpsertRecordingPolicy(context.Context, UpsertRecordingPolicyParams) (RecordingPolicy, error) {
	return RecordingPolicy{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) ListRecordings(context.Context, string, string) ([]Recording, error) {
	return nil, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) CreateRecording(context.Context, CreateRecordingParams) (Recording, error) {
	return Recording{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) SetRecordingConsent(context.Context, SetRecordingConsentParams) (Recording, error) {
	return Recording{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) TransitionRecording(context.Context, TransitionRecordingParams) (Recording, error) {
	return Recording{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) UpdateRecordingResult(context.Context, UpdateRecordingResultParams) (Recording, error) {
	return Recording{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) GetTalkIntegration(context.Context, string) (TalkIntegration, error) {
	return TalkIntegration{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) UpsertTalkIntegration(context.Context, UpsertTalkIntegrationParams) (TalkIntegration, error) {
	return TalkIntegration{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) ListFederationInvites(context.Context, string, string) ([]FederationInvite, error) {
	return nil, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) CreateFederationInvite(context.Context, CreateFederationInviteParams) (FederationInvite, error) {
	return FederationInvite{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) TransitionFederationInvite(context.Context, TransitionFederationInviteParams) (FederationInvite, error) {
	return FederationInvite{}, talkCapabilityUnavailable()
}

func (unavailableTalkRepository) ListMessagesForSummary(context.Context, string, string, *time.Time, int) ([]TalkAISummaryMessage, error) {
	return nil, talkCapabilityUnavailable()
}
