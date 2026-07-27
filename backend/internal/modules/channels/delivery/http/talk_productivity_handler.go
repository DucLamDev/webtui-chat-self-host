package http

import (
	"encoding/json"
	nethttp "net/http"
	"strconv"

	channelsapp "github.com/duclamdev/application-chat/backend/internal/modules/channels/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type createMeetingRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	StartsAt     string `json:"starts_at"`
	EndsAt       string `json:"ends_at"`
	LobbyOpensAt string `json:"lobby_opens_at"`
	RoomPolicy   string `json:"room_policy"`
	CleanupAfter string `json:"cleanup_after"`
}

type setupBreakoutsRequest struct {
	AssignmentMode  string                         `json:"assignment_mode"`
	RoomCount       int                            `json:"room_count"`
	AllowSelfSelect bool                           `json:"allow_self_select"`
	Rooms           []channelsapp.BreakoutRoomSpec `json:"rooms"`
}

type breakoutAssignmentsRequest struct {
	AssignedUserIDs []string `json:"assigned_user_ids"`
}

type breakoutBroadcastRequest struct {
	Body string `json:"body"`
}

type summarizeChannelRequest struct {
	Since    string `json:"since"`
	Language string `json:"language"`
}

type updateRecordingPolicyRequest struct {
	Enabled              bool   `json:"enabled"`
	ConsentRequired      bool   `json:"consent_required"`
	RetentionDays        int    `json:"retention_days"`
	TranscriptionEnabled bool   `json:"transcription_enabled"`
	SummaryEnabled       bool   `json:"summary_enabled"`
	Provider             string `json:"provider"`
}

type startRecordingRequest struct {
	MeetingID          string   `json:"meeting_id"`
	ParticipantUserIDs []string `json:"participant_user_ids"`
}

type recordingConsentRequest struct {
	Consented bool `json:"consented"`
}

type recordingResultRequest struct {
	Status              string `json:"status"`
	ProviderRecordingID string `json:"provider_recording_id"`
	StorageKey          string `json:"storage_key"`
	MimeType            string `json:"mime_type"`
	ByteSize            *int64 `json:"byte_size"`
	ChecksumSHA256      string `json:"checksum_sha256"`
	Error               string `json:"error"`
}

type updateTalkIntegrationRequest struct {
	AIEnabled             bool            `json:"ai_enabled"`
	AIProvider            string          `json:"ai_provider"`
	TranscriptionProvider string          `json:"transcription_provider"`
	FederationEnabled     bool            `json:"federation_enabled"`
	E2EECallsEnabled      bool            `json:"e2ee_calls_enabled"`
	SIPEnabled            bool            `json:"sip_enabled"`
	BridgeEnabled         bool            `json:"bridge_enabled"`
	Config                json.RawMessage `json:"config"`
}

type createFederationInviteRequest struct {
	RemoteServer string          `json:"remote_server"`
	RemoteUser   string          `json:"remote_user"`
	Protocol     string          `json:"protocol"`
	Payload      json.RawMessage `json:"payload"`
}

func (h *Handler) ListMeetings(c *gin.Context) {
	meetings, err := h.service.ListMeetings(
		c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"),
		c.Param("channel_id"), c.Query("from"), c.Query("to"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"meetings": meetings})
}

func (h *Handler) CreateMeeting(c *gin.Context) {
	var req createMeetingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON is invalid.", nil)
		return
	}
	meeting, err := h.service.CreateMeeting(c.Request.Context(), channelsapp.CreateMeetingInput{
		ActorUserID: middleware.CurrentUserID(c), WorkspaceID: c.Param("workspace_id"),
		ChannelID: c.Param("channel_id"), Title: req.Title, Description: req.Description,
		StartsAt: req.StartsAt, EndsAt: req.EndsAt, LobbyOpensAt: req.LobbyOpensAt,
		RoomPolicy: req.RoomPolicy, CleanupAfter: req.CleanupAfter,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, meeting)
}

func (h *Handler) TransitionMeeting(c *gin.Context) {
	meeting, err := h.service.TransitionMeeting(
		c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"),
		c.Param("channel_id"), c.Param("meeting_id"), c.Param("action"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, meeting)
}

func (h *Handler) GetVoiceRoom(c *gin.Context) {
	room, err := h.service.GetVoiceRoom(
		c.Request.Context(), middleware.CurrentUserID(c),
		c.Param("workspace_id"), c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, room)
}

func (h *Handler) StartVoiceRoom(c *gin.Context) {
	room, err := h.service.StartVoiceRoom(
		c.Request.Context(), middleware.CurrentUserID(c),
		c.Param("workspace_id"), c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, room)
}

func (h *Handler) StopVoiceRoom(c *gin.Context) {
	room, err := h.service.StopVoiceRoom(
		c.Request.Context(), middleware.CurrentUserID(c),
		c.Param("workspace_id"), c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, room)
}

func (h *Handler) SetupBreakoutRooms(c *gin.Context) {
	var req setupBreakoutsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON is invalid.", nil)
		return
	}
	rooms, err := h.service.SetupBreakoutRooms(c.Request.Context(), channelsapp.SetupBreakoutsInput{
		ActorUserID: middleware.CurrentUserID(c), WorkspaceID: c.Param("workspace_id"),
		ChannelID: c.Param("channel_id"), AssignmentMode: req.AssignmentMode,
		RoomCount: req.RoomCount, AllowSelfSelect: req.AllowSelfSelect, Rooms: req.Rooms,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"breakout_rooms": rooms})
}

func (h *Handler) StartBreakoutRooms(c *gin.Context) {
	rooms, err := h.service.StartBreakoutRooms(
		c.Request.Context(), middleware.CurrentUserID(c),
		c.Param("workspace_id"), c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"breakout_rooms": rooms})
}

func (h *Handler) JoinBreakoutRoom(c *gin.Context) {
	rooms, err := h.service.JoinBreakoutRoom(
		c.Request.Context(), middleware.CurrentUserID(c),
		c.Param("workspace_id"), c.Param("channel_id"), c.Param("room_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"breakout_rooms": rooms})
}

func (h *Handler) UpdateBreakoutAssignments(c *gin.Context) {
	var req breakoutAssignmentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON is invalid.", nil)
		return
	}
	rooms, err := h.service.UpdateBreakoutAssignments(
		c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"),
		c.Param("channel_id"), c.Param("room_id"), req.AssignedUserIDs,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"breakout_rooms": rooms})
}

func (h *Handler) BroadcastToBreakouts(c *gin.Context) {
	var req breakoutBroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON is invalid.", nil)
		return
	}
	broadcast, err := h.service.BroadcastToBreakouts(
		c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"),
		c.Param("channel_id"), req.Body,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, broadcast)
}

func (h *Handler) GetTalkHome(c *gin.Context) {
	home, err := h.service.GetTalkHome(
		c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, home)
}

func (h *Handler) ListSharedItems(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, err := h.service.ListSharedItems(
		c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"),
		c.Param("channel_id"), c.Query("kind"), limit,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"items": items})
}

func (h *Handler) SummarizeChannel(c *gin.Context) {
	var req summarizeChannelRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON is invalid.", nil)
			return
		}
	}
	summary, err := h.service.SummarizeChannel(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
		req.Since,
		req.Language,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, summary)
}

func (h *Handler) GetRecordingPolicy(c *gin.Context) {
	policy, err := h.service.GetRecordingPolicy(
		c.Request.Context(), middleware.CurrentUserID(c),
		c.Param("workspace_id"), c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, policy)
}

func (h *Handler) UpdateRecordingPolicy(c *gin.Context) {
	var req updateRecordingPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON is invalid.", nil)
		return
	}
	policy, err := h.service.UpdateRecordingPolicy(c.Request.Context(), channelsapp.UpdateRecordingPolicyInput{
		ActorUserID: middleware.CurrentUserID(c), WorkspaceID: c.Param("workspace_id"),
		ChannelID: c.Param("channel_id"), Enabled: req.Enabled,
		ConsentRequired: req.ConsentRequired, RetentionDays: req.RetentionDays,
		TranscriptionEnabled: req.TranscriptionEnabled, SummaryEnabled: req.SummaryEnabled,
		Provider: req.Provider,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, policy)
}

func (h *Handler) ListRecordings(c *gin.Context) {
	recordings, err := h.service.ListRecordings(
		c.Request.Context(), middleware.CurrentUserID(c),
		c.Param("workspace_id"), c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"recordings": recordings})
}

func (h *Handler) StartRecording(c *gin.Context) {
	var req startRecordingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON is invalid.", nil)
		return
	}
	recording, err := h.service.StartRecording(c.Request.Context(), channelsapp.StartRecordingInput{
		ActorUserID: middleware.CurrentUserID(c), WorkspaceID: c.Param("workspace_id"),
		ChannelID: c.Param("channel_id"), MeetingID: req.MeetingID,
		ParticipantUserIDs: req.ParticipantUserIDs,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, recording)
}

func (h *Handler) SetRecordingConsent(c *gin.Context) {
	var req recordingConsentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON is invalid.", nil)
		return
	}
	recording, err := h.service.SetRecordingConsent(
		c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"),
		c.Param("channel_id"), c.Param("recording_id"), req.Consented,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, recording)
}

func (h *Handler) StopRecording(c *gin.Context) {
	recording, err := h.service.StopRecording(
		c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"),
		c.Param("channel_id"), c.Param("recording_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, recording)
}

func (h *Handler) UpdateRecordingResult(c *gin.Context) {
	var req recordingResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON is invalid.", nil)
		return
	}
	recording, err := h.service.UpdateRecordingResult(c.Request.Context(), channelsapp.RecordingResultInput{
		ActorUserID: middleware.CurrentUserID(c), WorkspaceID: c.Param("workspace_id"),
		ChannelID: c.Param("channel_id"), RecordingID: c.Param("recording_id"),
		Status: req.Status, ProviderRecordingID: req.ProviderRecordingID,
		StorageKey: req.StorageKey, MimeType: req.MimeType, ByteSize: req.ByteSize,
		ChecksumSHA256: req.ChecksumSHA256, Error: req.Error,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, recording)
}

func (h *Handler) GetTalkIntegration(c *gin.Context) {
	integration, err := h.service.GetTalkIntegration(
		c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, integration)
}

func (h *Handler) UpdateTalkIntegration(c *gin.Context) {
	var req updateTalkIntegrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON is invalid.", nil)
		return
	}
	integration, err := h.service.UpdateTalkIntegration(c.Request.Context(), channelsapp.UpdateTalkIntegrationInput{
		ActorUserID: middleware.CurrentUserID(c), WorkspaceID: c.Param("workspace_id"),
		AIEnabled: req.AIEnabled, AIProvider: req.AIProvider,
		TranscriptionProvider: req.TranscriptionProvider, FederationEnabled: req.FederationEnabled,
		E2EECallsEnabled: req.E2EECallsEnabled, SIPEnabled: req.SIPEnabled,
		BridgeEnabled: req.BridgeEnabled, Config: req.Config,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, integration)
}

func (h *Handler) ListFederationInvites(c *gin.Context) {
	invites, err := h.service.ListFederationInvites(
		c.Request.Context(), middleware.CurrentUserID(c),
		c.Param("workspace_id"), c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"invites": invites})
}

func (h *Handler) CreateFederationInvite(c *gin.Context) {
	var req createFederationInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON is invalid.", nil)
		return
	}
	invite, err := h.service.CreateFederationInvite(c.Request.Context(), channelsapp.CreateFederationInviteInput{
		ActorUserID: middleware.CurrentUserID(c), WorkspaceID: c.Param("workspace_id"),
		ChannelID: c.Param("channel_id"), RemoteServer: req.RemoteServer,
		RemoteUser: req.RemoteUser, Protocol: req.Protocol, Payload: req.Payload,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, invite)
}

func (h *Handler) TransitionFederationInvite(c *gin.Context) {
	invite, err := h.service.TransitionFederationInvite(
		c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"),
		c.Param("channel_id"), c.Param("invite_id"), c.Param("status"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, invite)
}
