package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	channelsdomain "github.com/duclamdev/application-chat/backend/internal/modules/channels/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type collaborationMutationRepository struct {
	CollaborationRepository
	documentWrites int
}

type guestLegalRepository struct {
	CollaborationRepository
	created *CreateGuestRequestParams
	guest   channelsdomain.GuestRequest
}

func (r *guestLegalRepository) FindPublicSettings(_ context.Context, _ string) (channelsdomain.CollaborationSettings, error) {
	return channelsdomain.CollaborationSettings{
		ChannelID:           "channel-1",
		ChannelName:         "Public workshop",
		RoomMode:            "public",
		MeetingProvider:     "jitsi",
		MeetingRoomKey:      "secret-room-key",
		PublicAccessEnabled: true,
	}, nil
}

func (r *guestLegalRepository) CreateGuestRequest(_ context.Context, params CreateGuestRequestParams) (channelsdomain.GuestRequest, error) {
	r.created = &params
	termsVersion := params.TermsVersion
	privacyVersion := params.PrivacyPolicyVersion
	acceptedAt := params.LegalAcceptedAt
	return channelsdomain.GuestRequest{
		ID:                   "guest-1",
		ChannelID:            params.ChannelID,
		DisplayName:          params.DisplayName,
		Status:               params.Status,
		TermsVersion:         &termsVersion,
		PrivacyPolicyVersion: &privacyVersion,
		LegalAcceptedAt:      &acceptedAt,
		ExpiresAt:            params.ExpiresAt,
		CreatedAt:            acceptedAt,
		UpdatedAt:            acceptedAt,
	}, nil
}

func (r *guestLegalRepository) GetGuestRequest(_ context.Context, _ string, _ string, _ string) (channelsdomain.GuestRequest, error) {
	return r.guest, nil
}

func (r *collaborationMutationRepository) UpdateCollaborationDocument(_ context.Context, params UpdateCollaborationDocumentParams) (channelsdomain.CollaborationDocument, error) {
	r.documentWrites++
	now := time.Now().UTC()
	actorUserID := params.ActorUserID
	return channelsdomain.CollaborationDocument{
		ChannelID: params.ChannelID,
		Kind:      params.Kind,
		Content:   params.Content,
		Version:   params.ExpectedVersion + 1,
		UpdatedBy: &actorUserID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

type collaborationCleanupRepository struct {
	CollaborationRepository
	TalkProductivityRepository
	TalkGovernanceRepository
	calls int
}

func (r *collaborationCleanupRepository) GetCollaborationDocument(_ context.Context, workspaceID string, channelID string, kind string) (channelsdomain.CollaborationDocument, error) {
	r.calls++
	return channelsdomain.CollaborationDocument{ChannelID: channelID, Kind: kind, Content: json.RawMessage(`{}`)}, nil
}

func (r *collaborationCleanupRepository) DisablePublicLink(_ context.Context, workspaceID string, channelID string) (channelsdomain.CollaborationSettings, error) {
	r.calls++
	return channelsdomain.CollaborationSettings{WorkspaceID: workspaceID, ChannelID: channelID}, nil
}

func (r *collaborationCleanupRepository) UpdateGuestRequestStatus(_ context.Context, params UpdateGuestRequestStatusParams) (channelsdomain.GuestRequest, error) {
	r.calls++
	return channelsdomain.GuestRequest{ID: params.RequestID, ChannelID: params.ChannelID, Status: params.Status}, nil
}

func (r *collaborationCleanupRepository) CloseBreakoutRooms(_ context.Context, workspaceID string, channelID string, roomID string) ([]channelsdomain.BreakoutRoom, error) {
	r.calls++
	return []channelsdomain.BreakoutRoom{{ID: roomID, ChannelID: channelID, Status: "closed"}}, nil
}

func (r *collaborationCleanupRepository) TransitionMeeting(_ context.Context, params TransitionMeetingParams) (Meeting, error) {
	r.calls++
	return Meeting{ID: params.MeetingID, WorkspaceID: params.WorkspaceID, ChannelID: params.ChannelID, Status: params.Action}, nil
}

func (r *collaborationCleanupRepository) GetVoiceRoom(_ context.Context, workspaceID string, channelID string) (VoiceRoom, error) {
	r.calls++
	actorUserID := "user-a"
	return VoiceRoom{WorkspaceID: workspaceID, ChannelID: channelID, Status: "active", StartedBy: &actorUserID}, nil
}

func (r *collaborationCleanupRepository) SetVoiceRoom(_ context.Context, params SetVoiceRoomParams) (VoiceRoom, error) {
	r.calls++
	return VoiceRoom{WorkspaceID: params.WorkspaceID, ChannelID: params.ChannelID, Status: params.Status}, nil
}

func (r *collaborationCleanupRepository) SetRecordingConsent(_ context.Context, params SetRecordingConsentParams) (Recording, error) {
	r.calls++
	return Recording{ID: params.RecordingID, WorkspaceID: params.WorkspaceID, ChannelID: params.ChannelID}, nil
}

func (r *collaborationCleanupRepository) TransitionRecording(_ context.Context, params TransitionRecordingParams) (Recording, error) {
	r.calls++
	return Recording{ID: params.RecordingID, WorkspaceID: params.WorkspaceID, ChannelID: params.ChannelID, Status: params.Action}, nil
}

func (r *collaborationCleanupRepository) UpdateRecordingResult(_ context.Context, params UpdateRecordingResultParams) (Recording, error) {
	r.calls++
	return Recording{ID: params.RecordingID, WorkspaceID: params.WorkspaceID, ChannelID: params.ChannelID, Status: params.Status}, nil
}

func (r *collaborationCleanupRepository) TransitionFederationInvite(_ context.Context, params TransitionFederationInviteParams) (FederationInvite, error) {
	r.calls++
	return FederationInvite{ID: params.InviteID, WorkspaceID: params.WorkspaceID, ChannelID: params.ChannelID, Status: params.Status}, nil
}

func TestPublicRoomHidesMediaKeyUntilGuestIsApproved(t *testing.T) {
	settings := channelsdomain.CollaborationSettings{
		ChannelID:       "channel-id",
		ChannelName:     "Customer workshop",
		RoomMode:        "public",
		MeetingProvider: "jitsi",
		MeetingRoomKey:  "private-media-room",
	}

	beforeApproval := toPublicRoomDTO(settings, false, "https://meet.example.com")
	if beforeApproval.MeetingRoomKey != "" {
		t.Fatal("public room metadata must not expose the media room key before approval")
	}

	afterApproval := toPublicRoomDTO(settings, true, "https://meet.example.com/")
	if afterApproval.MeetingRoomKey != settings.MeetingRoomKey {
		t.Fatalf("approved guest received room key %q, want %q", afterApproval.MeetingRoomKey, settings.MeetingRoomKey)
	}
	if afterApproval.MeetingBaseURL != "https://meet.example.com/" {
		t.Fatalf("approved guest received base URL %q", afterApproval.MeetingBaseURL)
	}
}

func TestPublicGuestJoinRequiresAndPersistsCurrentLegalEvidence(t *testing.T) {
	repository := &guestLegalRepository{}
	service := NewService(nil, nil, repository)
	service.SetLegalDocumentVersions("terms-v2", "privacy-v3")

	_, err := service.JoinPublicRoom(context.Background(), JoinPublicRoomInput{
		PublicToken: "public-token-longer-than-24-characters",
		DisplayName: "Guest User",
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "LEGAL_ACCEPTANCE_REQUIRED" || appErr.Status != 409 {
		t.Fatalf("missing consent error = %#v, want 409 LEGAL_ACCEPTANCE_REQUIRED", err)
	}
	if repository.created != nil {
		t.Fatal("missing consent reached guest persistence")
	}

	_, err = service.JoinPublicRoom(context.Background(), JoinPublicRoomInput{
		PublicToken:     "public-token-longer-than-24-characters",
		DisplayName:     "Guest User",
		TermsAccepted:   true,
		TermsVersion:    "terms-v1",
		PrivacyAccepted: true,
		PrivacyVersion:  "privacy-v3",
	})
	if !errors.As(err, &appErr) || appErr.Code != "TERMS_VERSION_INVALID" || appErr.Status != 400 {
		t.Fatalf("stale consent error = %#v, want 400 TERMS_VERSION_INVALID", err)
	}

	guest, err := service.JoinPublicRoom(context.Background(), JoinPublicRoomInput{
		PublicToken:     "public-token-longer-than-24-characters",
		DisplayName:     " Guest User ",
		TermsAccepted:   true,
		TermsVersion:    "terms-v2",
		PrivacyAccepted: true,
		PrivacyVersion:  "privacy-v3",
		IPAddress:       "203.0.113.10",
		UserAgent:       "release-test-agent",
	})
	if err != nil {
		t.Fatalf("current guest consent rejected: %v", err)
	}
	if repository.created == nil {
		t.Fatal("current guest consent was not persisted")
	}
	if repository.created.TermsVersion != "terms-v2" || repository.created.PrivacyPolicyVersion != "privacy-v3" ||
		repository.created.LegalAcceptedAt.IsZero() || repository.created.LegalIPAddress != "203.0.113.10" ||
		repository.created.LegalUserAgent != "release-test-agent" {
		t.Fatalf("legal evidence = %#v", repository.created)
	}
	if guest.Room == nil || guest.Room.MeetingRoomKey != "secret-room-key" {
		t.Fatalf("approved guest room = %#v", guest.Room)
	}
}

func TestLegacyPublicGuestGrantCannotRevealRoomAfterLegalMigration(t *testing.T) {
	repository := &guestLegalRepository{guest: channelsdomain.GuestRequest{
		ID: "legacy-guest", ChannelID: "channel-1", Status: "approved", ExpiresAt: time.Now().UTC().Add(time.Hour),
	}}
	service := NewService(nil, nil, repository)
	service.SetLegalDocumentVersions("terms-v2", "privacy-v3")

	guest, err := service.GetPublicJoinStatus(context.Background(), "public-token-longer-than-24-characters", "legacy-guest", "access-token")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "LEGAL_ACCEPTANCE_REQUIRED" || appErr.Status != 409 {
		t.Fatalf("legacy grant error = %#v, want 409 LEGAL_ACCEPTANCE_REQUIRED", err)
	}
	if guest.Room != nil {
		t.Fatal("legacy guest without evidence received the private media room")
	}
}

func TestOpaquePublicTokensAreRandomAndStoredAsHashes(t *testing.T) {
	first, err := randomURLToken(32)
	if err != nil {
		t.Fatalf("randomURLToken() error = %v", err)
	}
	second, err := randomURLToken(32)
	if err != nil {
		t.Fatalf("randomURLToken() second error = %v", err)
	}
	if first == second {
		t.Fatal("two public-link tokens must not be equal")
	}
	hash := hashOpaqueToken(first)
	if hash == first || len(hash) != 64 {
		t.Fatalf("hashOpaqueToken() returned an invalid SHA-256 representation: %q", hash)
	}
	if hash != hashOpaqueToken(first) {
		t.Fatal("hashOpaqueToken() must be deterministic")
	}
}

func TestCollaborationEnumNormalizationFailsClosed(t *testing.T) {
	if normalizeRoomMode(" WEBINAR ") != "webinar" {
		t.Fatal("room mode should be normalized case-insensitively")
	}
	if normalizeRoomMode("external") != "" {
		t.Fatal("unknown room mode must be rejected")
	}
	if normalizeCollaborationRole("Presenter") != "presenter" {
		t.Fatal("participant role should be normalized case-insensitively")
	}
	if normalizeDocumentKind("../notes") != "" {
		t.Fatal("unknown document kind must be rejected")
	}
}

func TestBlockedDirectConversationRejectsCollaborationMutations(t *testing.T) {
	actorUserID := "user-a"

	tests := []struct {
		name string
		run  func(*Service) error
	}{
		{name: "settings", run: func(service *Service) error {
			_, err := service.UpdateCollaborationSettings(context.Background(), UpdateCollaborationSettingsInput{
				ActorUserID: actorUserID, WorkspaceID: "workspace-1", ChannelID: "direct-1",
			})
			return err
		}},
		{name: "promotion", run: func(service *Service) error {
			_, err := service.PromoteDirectConversation(context.Background(), actorUserID, "workspace-1", "direct-1", "new group")
			return err
		}},
		{name: "public link", run: func(service *Service) error {
			_, err := service.CreatePublicLink(context.Background(), CreatePublicLinkInput{
				ActorUserID: actorUserID, WorkspaceID: "workspace-1", ChannelID: "direct-1",
			})
			return err
		}},
		{name: "approve guest", run: func(service *Service) error {
			_, err := service.ModerateGuestRequest(context.Background(), actorUserID, "workspace-1", "direct-1", "request-1", "approved")
			return err
		}},
		{name: "role", run: func(service *Service) error {
			_, err := service.UpdateCollaborationRole(context.Background(), actorUserID, "workspace-1", "direct-1", "user-b", "moderator")
			return err
		}},
		{name: "document", run: func(service *Service) error {
			_, err := service.UpdateCollaborationDocument(context.Background(), UpdateCollaborationDocumentInput{
				ActorUserID: actorUserID, WorkspaceID: "workspace-1", ChannelID: "direct-1",
				Kind: "notes", Content: json.RawMessage(`{"body":"blocked"}`),
			})
			return err
		}},
		{name: "create task", run: func(service *Service) error {
			_, err := service.CreateChannelTask(context.Background(), CreateChannelTaskInput{
				ActorUserID: actorUserID, WorkspaceID: "workspace-1", ChannelID: "direct-1", Title: "blocked",
			})
			return err
		}},
		{name: "update task", run: func(service *Service) error {
			_, err := service.UpdateChannelTask(context.Background(), UpdateChannelTaskInput{
				ActorUserID: actorUserID, WorkspaceID: "workspace-1", ChannelID: "direct-1", TaskID: "task-1", Status: "open",
			})
			return err
		}},
		{name: "legacy breakout", run: func(service *Service) error {
			_, err := service.CreateBreakoutRoom(context.Background(), actorUserID, "workspace-1", "direct-1", "Room 1", nil)
			return err
		}},
		{name: "meeting create", run: func(service *Service) error {
			_, err := service.CreateMeeting(context.Background(), CreateMeetingInput{
				ActorUserID: actorUserID, WorkspaceID: "workspace-1", ChannelID: "direct-1",
			})
			return err
		}},
		{name: "meeting start", run: func(service *Service) error {
			_, err := service.TransitionMeeting(context.Background(), actorUserID, "workspace-1", "direct-1", "meeting-1", "start")
			return err
		}},
		{name: "voice start", run: func(service *Service) error {
			_, err := service.StartVoiceRoom(context.Background(), actorUserID, "workspace-1", "direct-1")
			return err
		}},
		{name: "breakout setup", run: func(service *Service) error {
			_, err := service.SetupBreakoutRooms(context.Background(), SetupBreakoutsInput{
				ActorUserID: actorUserID, WorkspaceID: "workspace-1", ChannelID: "direct-1",
			})
			return err
		}},
		{name: "breakout start", run: func(service *Service) error {
			_, err := service.StartBreakoutRooms(context.Background(), actorUserID, "workspace-1", "direct-1")
			return err
		}},
		{name: "breakout join", run: func(service *Service) error {
			_, err := service.JoinBreakoutRoom(context.Background(), actorUserID, "workspace-1", "direct-1", "room-1")
			return err
		}},
		{name: "breakout assignment", run: func(service *Service) error {
			_, err := service.UpdateBreakoutAssignments(context.Background(), actorUserID, "workspace-1", "direct-1", "room-1", []string{"user-a"})
			return err
		}},
		{name: "breakout broadcast", run: func(service *Service) error {
			_, err := service.BroadcastToBreakouts(context.Background(), actorUserID, "workspace-1", "direct-1", "blocked")
			return err
		}},
		{name: "AI summary", run: func(service *Service) error {
			_, err := service.SummarizeChannel(context.Background(), actorUserID, "workspace-1", "direct-1", "", "vi")
			return err
		}},
		{name: "recording policy", run: func(service *Service) error {
			_, err := service.UpdateRecordingPolicy(context.Background(), UpdateRecordingPolicyInput{
				ActorUserID: actorUserID, WorkspaceID: "workspace-1", ChannelID: "direct-1", Enabled: true,
			})
			return err
		}},
		{name: "recording start", run: func(service *Service) error {
			_, err := service.StartRecording(context.Background(), StartRecordingInput{
				ActorUserID: actorUserID, WorkspaceID: "workspace-1", ChannelID: "direct-1",
			})
			return err
		}},
		{name: "recording consent", run: func(service *Service) error {
			_, err := service.SetRecordingConsent(context.Background(), actorUserID, "workspace-1", "direct-1", "recording-1", true)
			return err
		}},
		{name: "federation create", run: func(service *Service) error {
			_, err := service.CreateFederationInvite(context.Background(), CreateFederationInviteInput{
				ActorUserID: actorUserID, WorkspaceID: "workspace-1", ChannelID: "direct-1",
			})
			return err
		}},
		{name: "federation accept", run: func(service *Service) error {
			_, err := service.TransitionFederationInvite(context.Background(), actorUserID, "workspace-1", "direct-1", "invite-1", "accepted")
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &directConversationRepo{
				privateSource: channelsdomain.Channel{
					ID: "direct-1", WorkspaceID: "workspace-1", Type: "direct", CreatedBy: &actorUserID,
				},
				memberStatuses: map[string]string{actorUserID: "active"},
			}
			collaboration := &collaborationMutationRepository{}
			service := NewService(repository, staticPermissionChecker{allowed: true}, collaboration)
			service.SetBlockChecker(staticBlockChecker{blockedChannels: map[string]bool{"direct-1": true}})

			var appErr *apperrors.AppError
			if err := test.run(service); !errors.As(err, &appErr) || appErr.Code != "INTERACTION_BLOCKED" || appErr.Status != 403 {
				t.Fatalf("mutation error = %#v, want 403 INTERACTION_BLOCKED", err)
			}
			if collaboration.documentWrites != 0 {
				t.Fatal("blocked direct mutation reached the collaboration repository")
			}
		})
	}
}

func TestGroupChannelCollaborationMutationIsUnaffectedByDirectBlockPolicy(t *testing.T) {
	actorUserID := "user-a"
	repository := &directConversationRepo{
		privateSource: channelsdomain.Channel{
			ID: "group-1", WorkspaceID: "workspace-1", Type: "private", CreatedBy: &actorUserID,
		},
		memberStatuses: map[string]string{actorUserID: "active"},
	}
	collaboration := &collaborationMutationRepository{}
	service := NewService(repository, staticPermissionChecker{allowed: true}, collaboration)
	service.SetBlockChecker(staticBlockChecker{blockedChannels: map[string]bool{"direct-1": true}})

	document, err := service.UpdateCollaborationDocument(context.Background(), UpdateCollaborationDocumentInput{
		ActorUserID: actorUserID,
		WorkspaceID: "workspace-1",
		ChannelID:   "group-1",
		Kind:        "notes",
		Content:     json.RawMessage(`{"body":"allowed"}`),
	})
	if err != nil {
		t.Fatalf("UpdateCollaborationDocument() error = %v", err)
	}
	if collaboration.documentWrites != 1 || document.ChannelID != "group-1" {
		t.Fatalf("group write count = %d, document = %#v", collaboration.documentWrites, document)
	}
}

func TestBlockedDirectConversationStillAllowsReadsAndCleanup(t *testing.T) {
	actorUserID := "user-a"
	repository := &directConversationRepo{
		privateSource: channelsdomain.Channel{
			ID: "direct-1", WorkspaceID: "workspace-1", Type: "direct", CreatedBy: &actorUserID,
		},
		memberStatuses: map[string]string{actorUserID: "active"},
	}
	collaboration := &collaborationCleanupRepository{}
	service := NewService(repository, staticPermissionChecker{allowed: true}, collaboration)
	service.SetBlockChecker(staticBlockChecker{blockedChannels: map[string]bool{"direct-1": true}})

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "read document", run: func() error {
			_, err := service.GetCollaborationDocument(context.Background(), actorUserID, "workspace-1", "direct-1", "notes")
			return err
		}},
		{name: "disable public link", run: func() error {
			_, err := service.DisablePublicLink(context.Background(), actorUserID, "workspace-1", "direct-1")
			return err
		}},
		{name: "reject guest", run: func() error {
			_, err := service.ModerateGuestRequest(context.Background(), actorUserID, "workspace-1", "direct-1", "request-1", "rejected")
			return err
		}},
		{name: "close breakout", run: func() error {
			_, err := service.CloseBreakoutRooms(context.Background(), actorUserID, "workspace-1", "direct-1", "room-1")
			return err
		}},
		{name: "end meeting", run: func() error {
			_, err := service.TransitionMeeting(context.Background(), actorUserID, "workspace-1", "direct-1", "meeting-1", "end")
			return err
		}},
		{name: "stop voice", run: func() error {
			_, err := service.StopVoiceRoom(context.Background(), actorUserID, "workspace-1", "direct-1")
			return err
		}},
		{name: "withdraw recording consent", run: func() error {
			_, err := service.SetRecordingConsent(context.Background(), actorUserID, "workspace-1", "direct-1", "recording-1", false)
			return err
		}},
		{name: "stop recording", run: func() error {
			_, err := service.StopRecording(context.Background(), actorUserID, "workspace-1", "direct-1", "recording-1")
			return err
		}},
		{name: "finish recording pipeline", run: func() error {
			_, err := service.UpdateRecordingResult(context.Background(), RecordingResultInput{
				ActorUserID: actorUserID, WorkspaceID: "workspace-1", ChannelID: "direct-1",
				RecordingID: "recording-1", Status: "failed",
			})
			return err
		}},
		{name: "revoke federation invite", run: func() error {
			_, err := service.TransitionFederationInvite(context.Background(), actorUserID, "workspace-1", "direct-1", "invite-1", "revoked")
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); err != nil {
				t.Fatalf("cleanup/read operation error = %v", err)
			}
		})
	}
	if collaboration.calls < len(tests) {
		t.Fatalf("repository calls = %d, want at least %d", collaboration.calls, len(tests))
	}
}

func TestUpdateTalkIntegrationRejectsUnimplementedE2EE(t *testing.T) {
	t.Parallel()

	service := NewService(&directConversationRepo{}, staticPermissionChecker{allowed: true})
	_, err := service.UpdateTalkIntegration(context.Background(), UpdateTalkIntegrationInput{
		ActorUserID:      "user-1",
		WorkspaceID:      "workspace-1",
		E2EECallsEnabled: true,
	})
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != "E2EE_CALLS_NOT_AVAILABLE" {
		t.Fatalf("expected fail-closed E2EE conflict, got %#v", err)
	}
}
