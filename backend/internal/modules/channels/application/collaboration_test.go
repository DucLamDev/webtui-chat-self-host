package application

import (
	"context"
	"testing"

	channelsdomain "github.com/duclamdev/application-chat/backend/internal/modules/channels/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

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
