package bootstrap

import (
	"os"
	"strings"
	"testing"
)

func TestUGCRoutesRequireCurrentLegalAcceptance(t *testing.T) {
	files := map[string][]string{
		"../modules/users/delivery/http/handler.go": {
			`ugc.PATCH("/me", h.UpdateMe)`,
			`ugc.POST("/me/avatar", h.UploadMyAvatar)`,
		},
		"../modules/messages/delivery/http/handler.go": {
			`ugc.POST("/messages/scheduled", h.ScheduleMessage)`,
			`ugc.POST("/channels/:channel_id/messages", h.Send)`,
			`ugc.PATCH("/channels/:channel_id/messages/:message_id", h.Update)`,
			`ugc.POST("/channels/:channel_id/messages/:message_id/forward", h.Forward)`,
			`ugc.POST("/channels/:channel_id/messages/:message_id/pin", h.Pin)`,
			`ugc.PUT("/channels/:channel_id/messages/:message_id/thread/details", h.UpsertThreadDetails)`,
			`ugc.POST("/channels/:channel_id/messages/:message_id/reactions", h.AddReaction)`,
		},
		"../modules/files/delivery/http/handler.go": {
			`ugc.POST("/files", h.Upload)`,
			`ugc.POST("/files/uploads", h.CreateUploadSession)`,
			`ugc.PUT("/files/uploads/:upload_id/parts/:part_number", h.UploadPart)`,
			`ugc.POST("/files/uploads/:upload_id/complete", h.CompleteUpload)`,
		},
		"../modules/calls/delivery/http/handler.go": {
			`ugc.POST("", h.Create)`,
			`ugc.POST("/:call_id/accept", h.Accept)`,
			`ugc.POST("/:call_id/signals", h.Signal)`,
		},
		"../modules/channels/delivery/http/handler.go": {
			`ugc.POST("/channels", h.Create)`,
			`ugc.PATCH("/channels/:channel_id", h.Update)`,
			`ugc.POST("/channels/:channel_id/private-session", h.OpenPrivateSession)`,
			`ugc.POST("/channels/:channel_id/collaboration/promote", h.PromoteConversation)`,
			`ugc.POST("/channels/:channel_id/collaboration/public-link", h.CreatePublicLink)`,
			`ugc.POST("/channels/:channel_id/collaboration/guests/:request_id/approve", h.ApproveGuestRequest)`,
			`ugc.PUT("/channels/:channel_id/collaboration/documents/:kind", h.UpdateCollaborationDocument)`,
			`ugc.POST("/channels/:channel_id/collaboration/tasks", h.CreateChannelTask)`,
			`ugc.PATCH("/channels/:channel_id/collaboration/tasks/:task_id", h.UpdateChannelTask)`,
			`ugc.POST("/channels/:channel_id/collaboration/breakouts", h.CreateBreakoutRoom)`,
			`ugc.PUT("/channels/:channel_id/collaboration/breakouts/setup", h.SetupBreakoutRooms)`,
			`ugc.POST("/channels/:channel_id/collaboration/breakouts/start", h.StartBreakoutRooms)`,
			`ugc.POST("/channels/:channel_id/collaboration/breakouts/:room_id/join", h.JoinBreakoutRoom)`,
			`ugc.POST("/channels/:channel_id/collaboration/breakouts/broadcast", h.BroadcastToBreakouts)`,
			`ugc.POST("/channels/:channel_id/collaboration/meetings", h.CreateMeeting)`,
			`ugc.POST("/channels/:channel_id/collaboration/voice-room/start", h.StartVoiceRoom)`,
			`ugc.POST("/channels/:channel_id/collaboration/ai/summary", h.SummarizeChannel)`,
			`ugc.POST("/channels/:channel_id/collaboration/recordings", h.StartRecording)`,
			`ugc.POST("/channels/:channel_id/collaboration/federation-invites", h.CreateFederationInvite)`,
			`ugc.POST("/direct-conversations", h.CreateDirect)`,
		},
		"../modules/bots/delivery/http/handler.go": {
			`ugc.POST("/bots/:bot_id/messages", h.SendMessage)`,
		},
	}
	for path, required := range files {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, route := range required {
			if !strings.Contains(string(contents), route) {
				t.Fatalf("%s is not protected by the UGC legal gate: %s", path, route)
			}
		}
	}

	routerSource, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	for _, required := range []string{
		"middleware.RequireCurrentLegalAcceptance(authService)",
		"usersHandler.RegisterRoutes(v1.Group(\"/users\"), authMiddleware, legalAcceptanceMiddleware)",
		"channelsHandler.RegisterRoutes(v1, authMiddleware, legalAcceptanceMiddleware)",
		"botsHandler.RegisterRoutes(v1, authMiddleware, legalAcceptanceMiddleware)",
		"callsHandler.RegisterRoutes(v1, authMiddleware, legalAcceptanceMiddleware)",
		"filesHandler.RegisterRoutes(v1, authMiddleware, legalAcceptanceMiddleware)",
		"messagesHandler.RegisterRoutes(v1, authMiddleware, legalAcceptanceMiddleware)",
		"webhooksService.SetLegalDocumentVersions(a.cfg.Legal.TermsVersion, a.cfg.Legal.PrivacyPolicyVersion)",
	} {
		if !strings.Contains(string(routerSource), required) {
			t.Fatalf("production router is missing legal gate wiring %q", required)
		}
	}
}
