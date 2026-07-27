package http

import (
	"encoding/json"
	nethttp "net/http"
	"strings"

	channelsapp "github.com/duclamdev/application-chat/backend/internal/modules/channels/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type updateCollaborationSettingsRequest struct {
	RoomMode               string `json:"room_mode"`
	MeetingProvider        string `json:"meeting_provider"`
	LobbyEnabled           bool   `json:"lobby_enabled"`
	ChatLocked             bool   `json:"chat_locked"`
	GuestMicrophoneEnabled bool   `json:"guest_microphone_enabled"`
	GuestCameraEnabled     bool   `json:"guest_camera_enabled"`
	DefaultParticipantRole string `json:"default_participant_role"`
}

type promoteConversationRequest struct {
	Name string `json:"name"`
}

type createPublicLinkRequest struct {
	RoomMode               string `json:"room_mode"`
	Password               string `json:"password"`
	LobbyEnabled           bool   `json:"lobby_enabled"`
	ChatLocked             bool   `json:"chat_locked"`
	GuestMicrophoneEnabled bool   `json:"guest_microphone_enabled"`
	GuestCameraEnabled     bool   `json:"guest_camera_enabled"`
}

type joinPublicRoomRequest struct {
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type updateCollaborationRoleRequest struct {
	Role string `json:"role"`
}

type updateCollaborationDocumentRequest struct {
	Content         json.RawMessage `json:"content"`
	ExpectedVersion int64           `json:"expected_version"`
}

type createChannelTaskRequest struct {
	SourceMessageID string `json:"source_message_id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	AssigneeUserID  string `json:"assignee_user_id"`
	DueAt           string `json:"due_at"`
}

type updateChannelTaskRequest struct {
	Status         string  `json:"status"`
	AssigneeUserID *string `json:"assignee_user_id"`
	DueAt          *string `json:"due_at"`
}

type createBreakoutRoomRequest struct {
	Name            string   `json:"name"`
	AssignedUserIDs []string `json:"assigned_user_ids"`
}

func (h *Handler) GetCollaborationSettings(c *gin.Context) {
	settings, err := h.service.GetCollaborationSettings(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, settings)
}

func (h *Handler) UpdateCollaborationSettings(c *gin.Context) {
	var req updateCollaborationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	settings, err := h.service.UpdateCollaborationSettings(c.Request.Context(), channelsapp.UpdateCollaborationSettingsInput{
		ActorUserID:            middleware.CurrentUserID(c),
		WorkspaceID:            c.Param("workspace_id"),
		ChannelID:              c.Param("channel_id"),
		RoomMode:               req.RoomMode,
		MeetingProvider:        req.MeetingProvider,
		LobbyEnabled:           req.LobbyEnabled,
		ChatLocked:             req.ChatLocked,
		GuestMicrophoneEnabled: req.GuestMicrophoneEnabled,
		GuestCameraEnabled:     req.GuestCameraEnabled,
		DefaultParticipantRole: req.DefaultParticipantRole,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, settings)
}

func (h *Handler) PromoteConversation(c *gin.Context) {
	var req promoteConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	settings, err := h.service.PromoteDirectConversation(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
		req.Name,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, settings)
}

func (h *Handler) CreatePublicLink(c *gin.Context) {
	var req createPublicLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	link, err := h.service.CreatePublicLink(c.Request.Context(), channelsapp.CreatePublicLinkInput{
		ActorUserID:            middleware.CurrentUserID(c),
		WorkspaceID:            c.Param("workspace_id"),
		ChannelID:              c.Param("channel_id"),
		RoomMode:               req.RoomMode,
		Password:               req.Password,
		LobbyEnabled:           req.LobbyEnabled,
		ChatLocked:             req.ChatLocked,
		GuestMicrophoneEnabled: req.GuestMicrophoneEnabled,
		GuestCameraEnabled:     req.GuestCameraEnabled,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, link)
}

func (h *Handler) DisablePublicLink(c *gin.Context) {
	settings, err := h.service.DisablePublicLink(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, settings)
}

func (h *Handler) GetPublicRoom(c *gin.Context) {
	room, err := h.service.GetPublicRoom(c.Request.Context(), c.Param("public_token"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, room)
}

func (h *Handler) JoinPublicRoom(c *gin.Context) {
	var req joinPublicRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	guest, err := h.service.JoinPublicRoom(c.Request.Context(), channelsapp.JoinPublicRoomInput{
		PublicToken: c.Param("public_token"),
		DisplayName: req.DisplayName,
		Password:    req.Password,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, guest)
}

func (h *Handler) GetPublicJoinStatus(c *gin.Context) {
	accessToken := strings.TrimSpace(c.Query("access_token"))
	if accessToken == "" {
		const bearerPrefix = "Bearer "
		authorization := c.GetHeader("Authorization")
		if strings.HasPrefix(authorization, bearerPrefix) {
			accessToken = strings.TrimSpace(strings.TrimPrefix(authorization, bearerPrefix))
		}
	}
	guest, err := h.service.GetPublicJoinStatus(
		c.Request.Context(),
		c.Param("public_token"),
		c.Param("request_id"),
		accessToken,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, guest)
}

func (h *Handler) ListGuestRequests(c *gin.Context) {
	guests, err := h.service.ListGuestRequests(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"guests": guests})
}

func (h *Handler) ApproveGuestRequest(c *gin.Context) {
	h.moderateGuestRequest(c, "approved")
}

func (h *Handler) RejectGuestRequest(c *gin.Context) {
	h.moderateGuestRequest(c, "rejected")
}

func (h *Handler) moderateGuestRequest(c *gin.Context, status string) {
	guest, err := h.service.ModerateGuestRequest(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
		c.Param("request_id"),
		status,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, guest)
}

func (h *Handler) ListCollaborationRoles(c *gin.Context) {
	roles, err := h.service.ListCollaborationRoles(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"roles": roles})
}

func (h *Handler) UpdateCollaborationRole(c *gin.Context) {
	var req updateCollaborationRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	role, err := h.service.UpdateCollaborationRole(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
		c.Param("user_id"),
		req.Role,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, role)
}

func (h *Handler) GetCollaborationDocument(c *gin.Context) {
	document, err := h.service.GetCollaborationDocument(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
		c.Param("kind"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, document)
}

func (h *Handler) UpdateCollaborationDocument(c *gin.Context) {
	var req updateCollaborationDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	document, err := h.service.UpdateCollaborationDocument(c.Request.Context(), channelsapp.UpdateCollaborationDocumentInput{
		ActorUserID:     middleware.CurrentUserID(c),
		WorkspaceID:     c.Param("workspace_id"),
		ChannelID:       c.Param("channel_id"),
		Kind:            c.Param("kind"),
		Content:         req.Content,
		ExpectedVersion: req.ExpectedVersion,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, document)
}

func (h *Handler) ListChannelTasks(c *gin.Context) {
	tasks, err := h.service.ListChannelTasks(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"tasks": tasks})
}

func (h *Handler) CreateChannelTask(c *gin.Context) {
	var req createChannelTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	task, err := h.service.CreateChannelTask(c.Request.Context(), channelsapp.CreateChannelTaskInput{
		ActorUserID:     middleware.CurrentUserID(c),
		WorkspaceID:     c.Param("workspace_id"),
		ChannelID:       c.Param("channel_id"),
		SourceMessageID: req.SourceMessageID,
		Title:           req.Title,
		Description:     req.Description,
		AssigneeUserID:  req.AssigneeUserID,
		DueAt:           req.DueAt,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, task)
}

func (h *Handler) UpdateChannelTask(c *gin.Context) {
	var req updateChannelTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	task, err := h.service.UpdateChannelTask(c.Request.Context(), channelsapp.UpdateChannelTaskInput{
		ActorUserID:    middleware.CurrentUserID(c),
		WorkspaceID:    c.Param("workspace_id"),
		ChannelID:      c.Param("channel_id"),
		TaskID:         c.Param("task_id"),
		Status:         req.Status,
		AssigneeUserID: req.AssigneeUserID,
		DueAt:          req.DueAt,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, task)
}

func (h *Handler) ListBreakoutRooms(c *gin.Context) {
	rooms, err := h.service.ListBreakoutRooms(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"breakout_rooms": rooms})
}

func (h *Handler) CreateBreakoutRoom(c *gin.Context) {
	var req createBreakoutRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}
	room, err := h.service.CreateBreakoutRoom(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
		req.Name,
		req.AssignedUserIDs,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, room)
}

func (h *Handler) ReturnBreakoutRooms(c *gin.Context) {
	h.closeBreakoutRooms(c, "")
}

func (h *Handler) CloseBreakoutRoom(c *gin.Context) {
	h.closeBreakoutRooms(c, c.Param("room_id"))
}

func (h *Handler) closeBreakoutRooms(c *gin.Context, roomID string) {
	rooms, err := h.service.CloseBreakoutRooms(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("workspace_id"),
		c.Param("channel_id"),
		roomID,
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"breakout_rooms": rooms})
}
