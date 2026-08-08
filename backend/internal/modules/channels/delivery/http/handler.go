package http

import (
	"bytes"
	"encoding/json"
	"io"
	nethttp "net/http"
	"strings"

	channelsapp "github.com/duclamdev/application-chat/backend/internal/modules/channels/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *channelsapp.Service
}

type createChannelRequest struct {
	DepartmentID string `json:"department_id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Type         string `json:"type"`
}

type updateChannelRequest struct {
	DepartmentID *string `json:"department_id"`
	Name         *string `json:"name"`
	Description  *string `json:"description"`
}

type addChannelMemberRequest struct {
	UserID string `json:"user_id"`
}

type updateChannelMemberStatusRequest struct {
	Status string `json:"status"`
}

type updateReadStateRequest struct {
	LastReadMessageID string `json:"last_read_message_id"`
}

type createDirectRequest struct {
	ParticipantIDs  []string `json:"participant_ids"`
	SourceChannelID string   `json:"source_channel_id"`
}

func NewHandler(service *channelsapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc, legalMiddleware ...gin.HandlerFunc) {
	private := router.Group("/workspaces/:workspace_id")
	private.Use(authMiddleware)
	ugc := router.Group("/workspaces/:workspace_id")
	ugc.Use(authMiddleware)
	if len(legalMiddleware) > 0 && legalMiddleware[0] != nil {
		ugc.Use(legalMiddleware[0])
	}
	private.GET("/channels", h.List)
	ugc.POST("/channels", h.Create)
	private.GET("/channels/:channel_id", h.Get)
	ugc.PATCH("/channels/:channel_id", h.Update)
	private.DELETE("/channels/:channel_id", h.Archive)
	private.GET("/channels/:channel_id/members", h.ListMembers)
	private.POST("/channels/:channel_id/members", h.AddMember)
	private.PATCH("/channels/:channel_id/members/:user_id", h.UpdateMemberStatus)
	private.POST("/channels/:channel_id/join-requests", h.RequestJoin)
	private.GET("/channels/:channel_id/join-requests", h.ListJoinRequests)
	private.POST("/channels/:channel_id/join-requests/:user_id/approve", h.ApproveJoinRequest)
	private.DELETE("/channels/:channel_id/join-requests/:user_id", h.RejectJoinRequest)
	private.PUT("/channels/:channel_id/read-state", h.UpdateReadState)
	ugc.POST("/channels/:channel_id/private-session", h.OpenPrivateSession)
	private.GET("/channels/:channel_id/collaboration", h.GetCollaborationSettings)
	private.PUT(
		"/channels/:channel_id/collaboration",
		requireLegalForJSON(firstMiddleware(legalMiddleware), collaborationSettingsNeedsLegalAcceptance),
		h.UpdateCollaborationSettings,
	)
	ugc.POST("/channels/:channel_id/collaboration/promote", h.PromoteConversation)
	ugc.POST("/channels/:channel_id/collaboration/public-link", h.CreatePublicLink)
	private.DELETE("/channels/:channel_id/collaboration/public-link", h.DisablePublicLink)
	private.GET("/channels/:channel_id/collaboration/guests", h.ListGuestRequests)
	ugc.POST("/channels/:channel_id/collaboration/guests/:request_id/approve", h.ApproveGuestRequest)
	private.POST("/channels/:channel_id/collaboration/guests/:request_id/reject", h.RejectGuestRequest)
	private.GET("/channels/:channel_id/collaboration/roles", h.ListCollaborationRoles)
	private.PATCH(
		"/channels/:channel_id/collaboration/roles/:user_id",
		requireLegalForJSON(firstMiddleware(legalMiddleware), collaborationRoleNeedsLegalAcceptance),
		h.UpdateCollaborationRole,
	)
	private.GET("/channels/:channel_id/collaboration/documents/:kind", h.GetCollaborationDocument)
	ugc.PUT("/channels/:channel_id/collaboration/documents/:kind", h.UpdateCollaborationDocument)
	private.GET("/channels/:channel_id/collaboration/tasks", h.ListChannelTasks)
	ugc.POST("/channels/:channel_id/collaboration/tasks", h.CreateChannelTask)
	ugc.PATCH("/channels/:channel_id/collaboration/tasks/:task_id", h.UpdateChannelTask)
	private.GET("/channels/:channel_id/collaboration/breakouts", h.ListBreakoutRooms)
	ugc.POST("/channels/:channel_id/collaboration/breakouts", h.CreateBreakoutRoom)
	private.POST("/channels/:channel_id/collaboration/breakouts/return", h.ReturnBreakoutRooms)
	private.POST("/channels/:channel_id/collaboration/breakouts/:room_id/close", h.CloseBreakoutRoom)
	ugc.PUT("/channels/:channel_id/collaboration/breakouts/setup", h.SetupBreakoutRooms)
	ugc.POST("/channels/:channel_id/collaboration/breakouts/start", h.StartBreakoutRooms)
	ugc.POST("/channels/:channel_id/collaboration/breakouts/:room_id/join", h.JoinBreakoutRoom)
	private.PUT(
		"/channels/:channel_id/collaboration/breakouts/:room_id/assignments",
		requireLegalForJSON(firstMiddleware(legalMiddleware), breakoutAssignmentsNeedLegalAcceptance),
		h.UpdateBreakoutAssignments,
	)
	ugc.POST("/channels/:channel_id/collaboration/breakouts/broadcast", h.BroadcastToBreakouts)
	private.GET("/channels/:channel_id/collaboration/meetings", h.ListMeetings)
	ugc.POST("/channels/:channel_id/collaboration/meetings", h.CreateMeeting)
	private.POST(
		"/channels/:channel_id/collaboration/meetings/:meeting_id/:action",
		requireLegalForPathValue(firstMiddleware(legalMiddleware), "action", "start"),
		h.TransitionMeeting,
	)
	private.GET("/channels/:channel_id/collaboration/voice-room", h.GetVoiceRoom)
	ugc.POST("/channels/:channel_id/collaboration/voice-room/start", h.StartVoiceRoom)
	private.POST("/channels/:channel_id/collaboration/voice-room/stop", h.StopVoiceRoom)
	private.GET("/channels/:channel_id/collaboration/shared-items", h.ListSharedItems)
	ugc.POST("/channels/:channel_id/collaboration/ai/summary", h.SummarizeChannel)
	private.GET("/channels/:channel_id/collaboration/recording-policy", h.GetRecordingPolicy)
	private.PUT(
		"/channels/:channel_id/collaboration/recording-policy",
		requireLegalForJSON(firstMiddleware(legalMiddleware), recordingPolicyNeedsLegalAcceptance),
		h.UpdateRecordingPolicy,
	)
	private.GET("/channels/:channel_id/collaboration/recordings", h.ListRecordings)
	ugc.POST("/channels/:channel_id/collaboration/recordings", h.StartRecording)
	private.PUT(
		"/channels/:channel_id/collaboration/recordings/:recording_id/consent",
		requireLegalForJSON(firstMiddleware(legalMiddleware), recordingConsentNeedsLegalAcceptance),
		h.SetRecordingConsent,
	)
	private.POST("/channels/:channel_id/collaboration/recordings/:recording_id/stop", h.StopRecording)
	private.PUT("/channels/:channel_id/collaboration/recordings/:recording_id/result", h.UpdateRecordingResult)
	private.GET("/channels/:channel_id/collaboration/federation-invites", h.ListFederationInvites)
	ugc.POST("/channels/:channel_id/collaboration/federation-invites", h.CreateFederationInvite)
	private.POST(
		"/channels/:channel_id/collaboration/federation-invites/:invite_id/:status",
		requireLegalForPathValue(firstMiddleware(legalMiddleware), "status", "accepted"),
		h.TransitionFederationInvite,
	)
	private.GET("/talk/home", h.GetTalkHome)
	private.GET("/talk/integrations", h.GetTalkIntegration)
	private.PUT("/talk/integrations", h.UpdateTalkIntegration)
	private.GET("/direct-conversations", h.ListDirects)
	ugc.POST("/direct-conversations", h.CreateDirect)

	router.GET("/public/conversations/:public_token", h.GetPublicRoom)
	router.POST("/public/conversations/:public_token/join", h.JoinPublicRoom)
	router.GET("/public/conversations/:public_token/join/:request_id", h.GetPublicJoinStatus)
}

func firstMiddleware(values []gin.HandlerFunc) gin.HandlerFunc {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

// requireLegalForPathValue preserves terminal/cleanup actions on a shared
// endpoint while gating the values that start or accept new collaboration.
func requireLegalForPathValue(legal gin.HandlerFunc, parameter string, gatedValues ...string) gin.HandlerFunc {
	gated := make(map[string]struct{}, len(gatedValues))
	for _, value := range gatedValues {
		gated[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	return func(c *gin.Context) {
		value := strings.ToLower(strings.TrimSpace(c.Param(parameter)))
		if _, required := gated[value]; required {
			if legal == nil {
				response.Fail(c, nethttp.StatusServiceUnavailable, "LEGAL_ACCEPTANCE_UNAVAILABLE", "Legal acceptance status is temporarily unavailable.", nil)
				c.Abort()
				return
			}
			legal(c)
			return
		}
		c.Next()
	}
}

const maxLegalDecisionBodyBytes = 64 * 1024

// requireLegalForJSON keeps restrictive and terminal mutations available
// during a policy rollover while still requiring current acceptance for any
// mutation that expands collaboration, capture, privileges, or participation.
func requireLegalForJSON(legal gin.HandlerFunc, predicate func(map[string]json.RawMessage) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil {
			requireLegalDecision(c, legal)
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxLegalDecisionBodyBytes+1))
		if err != nil {
			response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		if len(body) > maxLegalDecisionBodyBytes {
			response.Fail(c, nethttp.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "Body JSON vượt quá giới hạn cho phép.", nil)
			c.Abort()
			return
		}
		var payload map[string]json.RawMessage
		if json.Unmarshal(body, &payload) != nil {
			requireLegalDecision(c, legal)
			return
		}
		if predicate(payload) {
			requireLegalDecision(c, legal)
			return
		}
		c.Next()
	}
}

func requireLegalDecision(c *gin.Context, legal gin.HandlerFunc) {
	if legal == nil {
		response.Fail(c, nethttp.StatusServiceUnavailable, "LEGAL_ACCEPTANCE_UNAVAILABLE", "Legal acceptance status is temporarily unavailable.", nil)
		c.Abort()
		return
	}
	legal(c)
}

func collaborationSettingsNeedsLegalAcceptance(payload map[string]json.RawMessage) bool {
	roomMode, roomModeOK := jsonStringValue(payload, "room_mode")
	lobbyEnabled, lobbyOK := jsonBoolValue(payload, "lobby_enabled")
	chatLocked, chatOK := jsonBoolValue(payload, "chat_locked")
	guestMicrophoneEnabled, microphoneOK := jsonBoolValue(payload, "guest_microphone_enabled")
	guestCameraEnabled, cameraOK := jsonBoolValue(payload, "guest_camera_enabled")
	defaultRole, roleOK := jsonStringValue(payload, "default_participant_role")
	if !roomModeOK || !lobbyOK || !chatOK || !microphoneOK || !cameraOK || !roleOK {
		return true
	}
	fullSafetyLockdown := roomMode == "internal" && lobbyEnabled && chatLocked &&
		!guestMicrophoneEnabled && !guestCameraEnabled && defaultRole == "listener"
	return !fullSafetyLockdown
}

func collaborationRoleNeedsLegalAcceptance(payload map[string]json.RawMessage) bool {
	role, ok := jsonStringValue(payload, "role")
	return !ok || role != "listener"
}

func breakoutAssignmentsNeedLegalAcceptance(payload map[string]json.RawMessage) bool {
	var userIDs []string
	raw, ok := payload["assigned_user_ids"]
	return !ok || json.Unmarshal(raw, &userIDs) != nil || len(userIDs) > 0
}

func recordingPolicyNeedsLegalAcceptance(payload map[string]json.RawMessage) bool {
	enabled, ok := jsonBoolValue(payload, "enabled")
	return !ok || enabled
}

func recordingConsentNeedsLegalAcceptance(payload map[string]json.RawMessage) bool {
	consented, ok := jsonBoolValue(payload, "consented")
	return !ok || consented
}

func jsonStringValue(payload map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := payload[key]
	if !ok {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(value)), true
}

func jsonBoolValue(payload map[string]json.RawMessage, key string) (bool, bool) {
	raw, ok := payload[key]
	if !ok {
		return false, false
	}
	var value bool
	if json.Unmarshal(raw, &value) != nil {
		return false, false
	}
	return value, true
}

func (h *Handler) OpenPrivateSession(c *gin.Context) {
	channel, err := h.service.OpenPrivateSession(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, channel)
}

func (h *Handler) Create(c *gin.Context) {
	var req createChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	channel, err := h.service.Create(c.Request.Context(), channelsapp.CreateChannelInput{
		ActorUserID:  middleware.CurrentUserID(c),
		WorkspaceID:  c.Param("workspace_id"),
		DepartmentID: req.DepartmentID,
		Slug:         req.Slug,
		Name:         req.Name,
		Description:  req.Description,
		Type:         req.Type,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, channel)
}

func (h *Handler) List(c *gin.Context) {
	channels, err := h.service.List(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"channels": channels})
}

func (h *Handler) Get(c *gin.Context) {
	channel, err := h.service.Get(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, channel)
}

func (h *Handler) Update(c *gin.Context) {
	var req updateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	channel, err := h.service.Update(c.Request.Context(), channelsapp.UpdateChannelInput{
		ActorUserID:  middleware.CurrentUserID(c),
		WorkspaceID:  c.Param("workspace_id"),
		ChannelID:    c.Param("channel_id"),
		DepartmentID: req.DepartmentID,
		Name:         req.Name,
		Description:  req.Description,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, channel)
}

func (h *Handler) Archive(c *gin.Context) {
	if err := h.service.Archive(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) ListMembers(c *gin.Context) {
	members, err := h.service.ListMembers(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"members": members})
}

func (h *Handler) AddMember(c *gin.Context) {
	var req addChannelMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	member, err := h.service.AddMember(c.Request.Context(), channelsapp.AddMemberInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		UserID:      req.UserID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, member)
}

func (h *Handler) UpdateMemberStatus(c *gin.Context) {
	var req updateChannelMemberStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	member, err := h.service.UpdateMemberStatus(c.Request.Context(), channelsapp.UpdateMemberStatusInput{
		ActorUserID: middleware.CurrentUserID(c),
		WorkspaceID: c.Param("workspace_id"),
		ChannelID:   c.Param("channel_id"),
		UserID:      c.Param("user_id"),
		Status:      req.Status,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, member)
}

func (h *Handler) RequestJoin(c *gin.Context) {
	member, err := h.service.RequestJoin(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, member)
}

func (h *Handler) ListJoinRequests(c *gin.Context) {
	members, err := h.service.ListJoinRequests(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"join_requests": members})
}

func (h *Handler) ApproveJoinRequest(c *gin.Context) {
	member, err := h.service.ApproveJoinRequest(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id"), c.Param("user_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, member)
}

func (h *Handler) RejectJoinRequest(c *gin.Context) {
	if err := h.service.RejectJoinRequest(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"), c.Param("channel_id"), c.Param("user_id")); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) UpdateReadState(c *gin.Context) {
	var req updateReadStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	member, err := h.service.UpdateReadState(c.Request.Context(), channelsapp.UpdateReadStateInput{
		ActorUserID:       middleware.CurrentUserID(c),
		WorkspaceID:       c.Param("workspace_id"),
		ChannelID:         c.Param("channel_id"),
		LastReadMessageID: req.LastReadMessageID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, member)
}

func (h *Handler) CreateDirect(c *gin.Context) {
	var req createDirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	conversation, err := h.service.CreateDirect(c.Request.Context(), channelsapp.CreateDirectInput{
		ActorUserID:     middleware.CurrentUserID(c),
		WorkspaceID:     c.Param("workspace_id"),
		ParticipantIDs:  req.ParticipantIDs,
		SourceChannelID: req.SourceChannelID,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, conversation)
}

func (h *Handler) ListDirects(c *gin.Context) {
	conversations, err := h.service.ListDirects(c.Request.Context(), middleware.CurrentUserID(c), c.Param("workspace_id"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"direct_conversations": conversations})
}
