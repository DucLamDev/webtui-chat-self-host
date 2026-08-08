package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	channelsdomain "github.com/duclamdev/application-chat/backend/internal/modules/channels/domain"
	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const (
	maxCollaborationDocumentBytes = 512 * 1024
	guestAccessLifetime           = 12 * time.Hour
)

type CollaborationRepository interface {
	GetCollaborationSettings(ctx context.Context, workspaceID string, channelID string) (channelsdomain.CollaborationSettings, error)
	UpdateCollaborationSettings(ctx context.Context, params UpdateCollaborationSettingsParams) (channelsdomain.CollaborationSettings, error)
	PromoteDirectConversation(ctx context.Context, params PromoteDirectConversationParams) (channelsdomain.CollaborationSettings, error)
	SetPublicLink(ctx context.Context, params SetPublicLinkParams) (channelsdomain.CollaborationSettings, error)
	DisablePublicLink(ctx context.Context, workspaceID string, channelID string) (channelsdomain.CollaborationSettings, error)
	FindPublicSettings(ctx context.Context, publicTokenHash string) (channelsdomain.CollaborationSettings, error)
	CreateGuestRequest(ctx context.Context, params CreateGuestRequestParams) (channelsdomain.GuestRequest, error)
	GetGuestRequest(ctx context.Context, channelID string, requestID string, accessTokenHash string) (channelsdomain.GuestRequest, error)
	ListGuestRequests(ctx context.Context, workspaceID string, channelID string) ([]channelsdomain.GuestRequest, error)
	UpdateGuestRequestStatus(ctx context.Context, params UpdateGuestRequestStatusParams) (channelsdomain.GuestRequest, error)
	ListCollaborationRoles(ctx context.Context, workspaceID string, channelID string) ([]channelsdomain.CollaborationRole, error)
	UpsertCollaborationRole(ctx context.Context, params UpsertCollaborationRoleParams) (channelsdomain.CollaborationRole, error)
	GetCollaborationDocument(ctx context.Context, workspaceID string, channelID string, kind string) (channelsdomain.CollaborationDocument, error)
	UpdateCollaborationDocument(ctx context.Context, params UpdateCollaborationDocumentParams) (channelsdomain.CollaborationDocument, error)
	ListChannelTasks(ctx context.Context, workspaceID string, channelID string) ([]channelsdomain.ChannelTask, error)
	CreateChannelTask(ctx context.Context, params CreateChannelTaskParams) (channelsdomain.ChannelTask, error)
	UpdateChannelTask(ctx context.Context, params UpdateChannelTaskParams) (channelsdomain.ChannelTask, error)
	ListBreakoutRooms(ctx context.Context, workspaceID string, channelID string) ([]channelsdomain.BreakoutRoom, error)
	CreateBreakoutRoom(ctx context.Context, params CreateBreakoutRoomParams) (channelsdomain.BreakoutRoom, error)
	CloseBreakoutRooms(ctx context.Context, workspaceID string, channelID string, roomID string) ([]channelsdomain.BreakoutRoom, error)
}

type UpdateCollaborationSettingsParams struct {
	WorkspaceID            string
	ChannelID              string
	RoomMode               string
	MeetingProvider        string
	LobbyEnabled           bool
	ChatLocked             bool
	GuestMicrophoneEnabled bool
	GuestCameraEnabled     bool
	DefaultParticipantRole string
	ActorUserID            string
}

type PromoteDirectConversationParams struct {
	WorkspaceID string
	ChannelID   string
	Name        string
	ActorUserID string
}

type SetPublicLinkParams struct {
	WorkspaceID            string
	ChannelID              string
	RoomMode               string
	PublicTokenHash        string
	PublicTokenPrefix      string
	PasswordHash           string
	LobbyEnabled           bool
	ChatLocked             bool
	GuestMicrophoneEnabled bool
	GuestCameraEnabled     bool
	ActorUserID            string
}

type CreateGuestRequestParams struct {
	ChannelID            string
	DisplayName          string
	Status               string
	AccessTokenHash      string
	TermsVersion         string
	PrivacyPolicyVersion string
	LegalAcceptedAt      time.Time
	LegalIPAddress       string
	LegalUserAgent       string
	ExpiresAt            time.Time
}

type UpdateGuestRequestStatusParams struct {
	WorkspaceID string
	ChannelID   string
	RequestID   string
	Status      string
	ActorUserID string
}

type UpsertCollaborationRoleParams struct {
	WorkspaceID string
	ChannelID   string
	UserID      string
	Role        string
	ActorUserID string
}

type UpdateCollaborationDocumentParams struct {
	WorkspaceID     string
	ChannelID       string
	Kind            string
	Content         []byte
	ExpectedVersion int64
	ActorUserID     string
}

type CreateChannelTaskParams struct {
	WorkspaceID     string
	ChannelID       string
	SourceMessageID string
	Title           string
	Description     string
	AssigneeUserID  string
	DueAt           *time.Time
	ActorUserID     string
}

type UpdateChannelTaskParams struct {
	WorkspaceID    string
	ChannelID      string
	TaskID         string
	Status         string
	AssigneeUserID *string
	DueAt          *time.Time
	ClearDueAt     bool
	ActorUserID    string
}

type CreateBreakoutRoomParams struct {
	WorkspaceID     string
	ChannelID       string
	Name            string
	AssignedUserIDs []string
	AssignmentMode  string
	AllowSelfSelect bool
	Sequence        int
	ActorUserID     string
}

type CollaborationSettingsDTO struct {
	ChannelID              string  `json:"channel_id"`
	WorkspaceID            string  `json:"workspace_id"`
	ChannelName            string  `json:"channel_name"`
	ChannelType            string  `json:"channel_type"`
	RoomMode               string  `json:"room_mode"`
	MeetingProvider        string  `json:"meeting_provider"`
	MeetingBaseURL         string  `json:"meeting_base_url,omitempty"`
	MeetingRoomKey         string  `json:"meeting_room_key,omitempty"`
	PublicAccessEnabled    bool    `json:"public_access_enabled"`
	PublicTokenPrefix      *string `json:"public_token_prefix,omitempty"`
	HasPassword            bool    `json:"has_password"`
	LobbyEnabled           bool    `json:"lobby_enabled"`
	ChatLocked             bool    `json:"chat_locked"`
	GuestMicrophoneEnabled bool    `json:"guest_microphone_enabled"`
	GuestCameraEnabled     bool    `json:"guest_camera_enabled"`
	DefaultParticipantRole string  `json:"default_participant_role"`
	CreatedAt              string  `json:"created_at"`
	UpdatedAt              string  `json:"updated_at"`
}

type PublicLinkDTO struct {
	CollaborationSettingsDTO
	Token string `json:"token"`
}

type PublicRoomDTO struct {
	ChannelID              string `json:"channel_id"`
	ChannelName            string `json:"channel_name"`
	RoomMode               string `json:"room_mode"`
	MeetingProvider        string `json:"meeting_provider"`
	MeetingBaseURL         string `json:"meeting_base_url,omitempty"`
	MeetingRoomKey         string `json:"meeting_room_key,omitempty"`
	HasPassword            bool   `json:"has_password"`
	LobbyEnabled           bool   `json:"lobby_enabled"`
	ChatLocked             bool   `json:"chat_locked"`
	GuestMicrophoneEnabled bool   `json:"guest_microphone_enabled"`
	GuestCameraEnabled     bool   `json:"guest_camera_enabled"`
}

type GuestRequestDTO struct {
	ID               string         `json:"id"`
	ChannelID        string         `json:"channel_id"`
	DisplayName      string         `json:"display_name"`
	Status           string         `json:"status"`
	GuestAccessToken string         `json:"guest_access_token,omitempty"`
	Room             *PublicRoomDTO `json:"room,omitempty"`
	ExpiresAt        string         `json:"expires_at"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
}

type CollaborationRoleDTO struct {
	ChannelID   string  `json:"channel_id"`
	UserID      string  `json:"user_id"`
	DisplayName string  `json:"display_name"`
	Username    string  `json:"username"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	Role        string  `json:"role"`
	UpdatedAt   string  `json:"updated_at"`
}

type CollaborationDocumentDTO struct {
	ChannelID string          `json:"channel_id"`
	Kind      string          `json:"kind"`
	Content   json.RawMessage `json:"content"`
	Version   int64           `json:"version"`
	UpdatedBy *string         `json:"updated_by,omitempty"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

type ChannelTaskDTO struct {
	ID              string  `json:"id"`
	WorkspaceID     string  `json:"workspace_id"`
	ChannelID       string  `json:"channel_id"`
	SourceMessageID *string `json:"source_message_id,omitempty"`
	Title           string  `json:"title"`
	Description     *string `json:"description,omitempty"`
	Status          string  `json:"status"`
	AssigneeUserID  *string `json:"assignee_user_id,omitempty"`
	DueAt           *string `json:"due_at,omitempty"`
	CreatedBy       *string `json:"created_by,omitempty"`
	CompletedAt     *string `json:"completed_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type BreakoutRoomDTO struct {
	ID              string   `json:"id"`
	ChannelID       string   `json:"channel_id"`
	Name            string   `json:"name"`
	RoomKey         string   `json:"room_key"`
	AssignedUserIDs []string `json:"assigned_user_ids"`
	Status          string   `json:"status"`
	AssignmentMode  string   `json:"assignment_mode"`
	AllowSelfSelect bool     `json:"allow_self_select"`
	StartedAt       *string  `json:"started_at,omitempty"`
	Sequence        int      `json:"sequence"`
	CreatedBy       *string  `json:"created_by,omitempty"`
	ClosedAt        *string  `json:"closed_at,omitempty"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

type UpdateCollaborationSettingsInput struct {
	ActorUserID            string
	WorkspaceID            string
	ChannelID              string
	RoomMode               string
	MeetingProvider        string
	LobbyEnabled           bool
	ChatLocked             bool
	GuestMicrophoneEnabled bool
	GuestCameraEnabled     bool
	DefaultParticipantRole string
}

type CreatePublicLinkInput struct {
	ActorUserID            string
	WorkspaceID            string
	ChannelID              string
	RoomMode               string
	Password               string
	LobbyEnabled           bool
	ChatLocked             bool
	GuestMicrophoneEnabled bool
	GuestCameraEnabled     bool
}

type JoinPublicRoomInput struct {
	PublicToken     string
	DisplayName     string
	Password        string
	TermsAccepted   bool
	TermsVersion    string
	PrivacyAccepted bool
	PrivacyVersion  string
	IPAddress       string
	UserAgent       string
}

type UpdateCollaborationDocumentInput struct {
	ActorUserID     string
	WorkspaceID     string
	ChannelID       string
	Kind            string
	Content         json.RawMessage
	ExpectedVersion int64
}

type CreateChannelTaskInput struct {
	ActorUserID     string
	WorkspaceID     string
	ChannelID       string
	SourceMessageID string
	Title           string
	Description     string
	AssigneeUserID  string
	DueAt           string
}

type UpdateChannelTaskInput struct {
	ActorUserID    string
	WorkspaceID    string
	ChannelID      string
	TaskID         string
	Status         string
	AssigneeUserID *string
	DueAt          *string
}

func (s *Service) GetCollaborationSettings(ctx context.Context, actorUserID string, workspaceID string, channelID string) (CollaborationSettingsDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return CollaborationSettingsDTO{}, err
	}
	settings, err := s.collaborationRepository().GetCollaborationSettings(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return CollaborationSettingsDTO{}, mapChannelError(err)
	}
	return toCollaborationSettingsDTO(settings, true, s.meetingBaseURL), nil
}

func (s *Service) UpdateCollaborationSettings(ctx context.Context, input UpdateCollaborationSettingsInput) (CollaborationSettingsDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return CollaborationSettingsDTO{}, err
	}
	if !isCollaborationSafetyLockdown(input) {
		if err := s.ensureDirectInteractionAllowed(ctx, input.WorkspaceID, input.ChannelID, input.ActorUserID); err != nil {
			return CollaborationSettingsDTO{}, err
		}
	}
	channel, err := s.repo.FindChannel(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.ChannelID))
	if err != nil {
		return CollaborationSettingsDTO{}, mapChannelError(err)
	}
	roomMode := normalizeRoomMode(input.RoomMode)
	if roomMode == "" {
		return CollaborationSettingsDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Chế độ phòng không hợp lệ.")
	}
	if channel.Type == "direct" && roomMode != "internal" {
		return CollaborationSettingsDTO{}, apperrors.Conflict("PROMOTION_REQUIRED", "Hãy chuyển cuộc trò chuyện riêng thành phòng nhóm trước khi bật link công khai hoặc webinar.")
	}
	provider := normalizeMeetingProvider(input.MeetingProvider)
	if provider == "" {
		return CollaborationSettingsDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Nhà cung cấp cuộc họp không hợp lệ.")
	}
	role := normalizeCollaborationRole(input.DefaultParticipantRole)
	if role == "" {
		return CollaborationSettingsDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Vai trò mặc định không hợp lệ.")
	}
	settings, err := s.collaborationRepository().UpdateCollaborationSettings(ctx, UpdateCollaborationSettingsParams{
		WorkspaceID:            strings.TrimSpace(input.WorkspaceID),
		ChannelID:              strings.TrimSpace(input.ChannelID),
		RoomMode:               roomMode,
		MeetingProvider:        provider,
		LobbyEnabled:           input.LobbyEnabled,
		ChatLocked:             input.ChatLocked,
		GuestMicrophoneEnabled: input.GuestMicrophoneEnabled,
		GuestCameraEnabled:     input.GuestCameraEnabled,
		DefaultParticipantRole: role,
		ActorUserID:            strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return CollaborationSettingsDTO{}, mapChannelError(err)
	}
	return toCollaborationSettingsDTO(settings, true, s.meetingBaseURL), nil
}

func isCollaborationSafetyLockdown(input UpdateCollaborationSettingsInput) bool {
	return normalizeRoomMode(input.RoomMode) == "internal" && input.LobbyEnabled && input.ChatLocked &&
		!input.GuestMicrophoneEnabled && !input.GuestCameraEnabled &&
		normalizeCollaborationRole(input.DefaultParticipantRole) == "listener"
}

func (s *Service) PromoteDirectConversation(ctx context.Context, actorUserID string, workspaceID string, channelID string, name string) (CollaborationSettingsDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return CollaborationSettingsDTO{}, err
	}
	if err := s.ensureDirectInteractionAllowed(ctx, workspaceID, channelID, actorUserID); err != nil {
		return CollaborationSettingsDTO{}, err
	}
	channel, err := s.repo.FindChannel(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return CollaborationSettingsDTO{}, mapChannelError(err)
	}
	if channel.Type != "direct" {
		return CollaborationSettingsDTO{}, apperrors.Conflict("NOT_DIRECT_CONVERSATION", "Chỉ cuộc trò chuyện riêng 1-1 mới cần chuyển thành phòng nhóm.")
	}
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 2 || len([]rune(name)) > 120 {
		return CollaborationSettingsDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên phòng nhóm phải có từ 2 đến 120 ký tự.")
	}
	settings, err := s.collaborationRepository().PromoteDirectConversation(ctx, PromoteDirectConversationParams{
		WorkspaceID: strings.TrimSpace(workspaceID),
		ChannelID:   strings.TrimSpace(channelID),
		Name:        name,
		ActorUserID: strings.TrimSpace(actorUserID),
	})
	if err != nil {
		return CollaborationSettingsDTO{}, mapChannelError(err)
	}
	return toCollaborationSettingsDTO(settings, true, s.meetingBaseURL), nil
}

func (s *Service) CreatePublicLink(ctx context.Context, input CreatePublicLinkInput) (PublicLinkDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return PublicLinkDTO{}, err
	}
	if err := s.ensureDirectInteractionAllowed(ctx, input.WorkspaceID, input.ChannelID, input.ActorUserID); err != nil {
		return PublicLinkDTO{}, err
	}
	channel, err := s.repo.FindChannel(ctx, strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.ChannelID))
	if err != nil {
		return PublicLinkDTO{}, mapChannelError(err)
	}
	if channel.Type == "direct" {
		return PublicLinkDTO{}, apperrors.Conflict("PROMOTION_REQUIRED", "Cuộc trò chuyện 1-1 phải được chuyển thành phòng nhóm trước khi tạo link công khai.")
	}
	roomMode := normalizeRoomMode(input.RoomMode)
	if roomMode != "public" && roomMode != "webinar" {
		roomMode = "public"
	}
	password := strings.TrimSpace(input.Password)
	if password != "" && (len(password) < 8 || len(password) > 128) {
		return PublicLinkDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Mật khẩu phòng phải có từ 8 đến 128 ký tự.")
	}
	passwordHash := ""
	if password != "" {
		passwordHash, err = sharedauth.HashPassword(password)
		if err != nil {
			return PublicLinkDTO{}, err
		}
	}
	token, err := randomURLToken(32)
	if err != nil {
		return PublicLinkDTO{}, err
	}
	settings, err := s.collaborationRepository().SetPublicLink(ctx, SetPublicLinkParams{
		WorkspaceID:            strings.TrimSpace(input.WorkspaceID),
		ChannelID:              strings.TrimSpace(input.ChannelID),
		RoomMode:               roomMode,
		PublicTokenHash:        hashOpaqueToken(token),
		PublicTokenPrefix:      token[:8],
		PasswordHash:           passwordHash,
		LobbyEnabled:           input.LobbyEnabled,
		ChatLocked:             input.ChatLocked,
		GuestMicrophoneEnabled: input.GuestMicrophoneEnabled,
		GuestCameraEnabled:     input.GuestCameraEnabled,
		ActorUserID:            strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return PublicLinkDTO{}, mapChannelError(err)
	}
	return PublicLinkDTO{CollaborationSettingsDTO: toCollaborationSettingsDTO(settings, true, s.meetingBaseURL), Token: token}, nil
}

func (s *Service) DisablePublicLink(ctx context.Context, actorUserID string, workspaceID string, channelID string) (CollaborationSettingsDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return CollaborationSettingsDTO{}, err
	}
	settings, err := s.collaborationRepository().DisablePublicLink(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return CollaborationSettingsDTO{}, mapChannelError(err)
	}
	return toCollaborationSettingsDTO(settings, true, s.meetingBaseURL), nil
}

func (s *Service) GetPublicRoom(ctx context.Context, publicToken string) (PublicRoomDTO, error) {
	settings, err := s.publicSettings(ctx, publicToken)
	if err != nil {
		return PublicRoomDTO{}, err
	}
	return toPublicRoomDTO(settings, false, s.meetingBaseURL), nil
}

func (s *Service) JoinPublicRoom(ctx context.Context, input JoinPublicRoomInput) (GuestRequestDTO, error) {
	settings, err := s.publicSettings(ctx, input.PublicToken)
	if err != nil {
		return GuestRequestDTO{}, err
	}
	if err := s.validateGuestLegalAcceptance(input); err != nil {
		return GuestRequestDTO{}, err
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if len([]rune(displayName)) < 2 || len([]rune(displayName)) > 80 {
		return GuestRequestDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên khách phải có từ 2 đến 80 ký tự.")
	}
	if settings.PasswordHash != nil && !sharedauth.VerifyPassword(*settings.PasswordHash, input.Password) {
		return GuestRequestDTO{}, apperrors.Unauthorized("Mật khẩu phòng không đúng.")
	}
	accessToken, err := randomURLToken(32)
	if err != nil {
		return GuestRequestDTO{}, err
	}
	status := "approved"
	if settings.LobbyEnabled {
		status = "waiting"
	}
	now := time.Now().UTC()
	guest, err := s.collaborationRepository().CreateGuestRequest(ctx, CreateGuestRequestParams{
		ChannelID:            settings.ChannelID,
		DisplayName:          displayName,
		Status:               status,
		AccessTokenHash:      hashOpaqueToken(accessToken),
		TermsVersion:         strings.TrimSpace(input.TermsVersion),
		PrivacyPolicyVersion: strings.TrimSpace(input.PrivacyVersion),
		LegalAcceptedAt:      now,
		LegalIPAddress:       strings.TrimSpace(input.IPAddress),
		LegalUserAgent:       strings.TrimSpace(input.UserAgent),
		ExpiresAt:            now.Add(guestAccessLifetime),
	})
	if err != nil {
		return GuestRequestDTO{}, err
	}
	dto := toGuestRequestDTO(guest)
	dto.GuestAccessToken = accessToken
	if status == "approved" {
		room := toPublicRoomDTO(settings, true, s.meetingBaseURL)
		dto.Room = &room
	}
	return dto, nil
}

func (s *Service) GetPublicJoinStatus(ctx context.Context, publicToken string, requestID string, accessToken string) (GuestRequestDTO, error) {
	settings, err := s.publicSettings(ctx, publicToken)
	if err != nil {
		return GuestRequestDTO{}, err
	}
	if strings.TrimSpace(accessToken) == "" {
		return GuestRequestDTO{}, apperrors.Unauthorized("Thiếu mã truy cập của khách.")
	}
	guest, err := s.collaborationRepository().GetGuestRequest(
		ctx,
		settings.ChannelID,
		strings.TrimSpace(requestID),
		hashOpaqueToken(accessToken),
	)
	if err != nil {
		if errors.Is(err, channelsdomain.ErrGuestNotFound) {
			return GuestRequestDTO{}, apperrors.NotFound("GUEST_REQUEST_NOT_FOUND", "Không tìm thấy yêu cầu tham gia.")
		}
		return GuestRequestDTO{}, err
	}
	if guest.TermsVersion == nil || guest.PrivacyPolicyVersion == nil || guest.LegalAcceptedAt == nil ||
		*guest.TermsVersion != s.termsVersion || *guest.PrivacyPolicyVersion != s.privacyPolicyVersion {
		return GuestRequestDTO{}, apperrors.Conflict(
			"LEGAL_ACCEPTANCE_REQUIRED",
			"Accept the current Terms, Acceptable Use Policy, and Privacy Policy before joining this public room.",
		)
	}
	dto := toGuestRequestDTO(guest)
	if guest.Status == "approved" && guest.ExpiresAt.After(time.Now().UTC()) {
		room := toPublicRoomDTO(settings, true, s.meetingBaseURL)
		dto.Room = &room
	}
	return dto, nil
}

func (s *Service) validateGuestLegalAcceptance(input JoinPublicRoomInput) error {
	if s.termsVersion == "" || s.privacyPolicyVersion == "" {
		return apperrors.ServiceUnavailable(
			"LEGAL_DOCUMENTS_UNAVAILABLE",
			"The current Terms and Privacy Policy versions are not configured.",
		)
	}
	if !input.TermsAccepted || !input.PrivacyAccepted {
		return apperrors.Conflict(
			"LEGAL_ACCEPTANCE_REQUIRED",
			"Accept the current Terms, Acceptable Use Policy, and Privacy Policy before joining this public room.",
		)
	}
	if strings.TrimSpace(input.TermsVersion) != s.termsVersion {
		return apperrors.BadRequest("TERMS_VERSION_INVALID", "Accept the current Terms and Acceptable Use Policy version before joining.")
	}
	if strings.TrimSpace(input.PrivacyVersion) != s.privacyPolicyVersion {
		return apperrors.BadRequest("PRIVACY_VERSION_INVALID", "Acknowledge the current Privacy Policy version before joining.")
	}
	return nil
}

func (s *Service) ListGuestRequests(ctx context.Context, actorUserID string, workspaceID string, channelID string) ([]GuestRequestDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	guests, err := s.collaborationRepository().ListGuestRequests(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return nil, err
	}
	result := make([]GuestRequestDTO, 0, len(guests))
	for _, guest := range guests {
		result = append(result, toGuestRequestDTO(guest))
	}
	return result, nil
}

func (s *Service) ModerateGuestRequest(ctx context.Context, actorUserID string, workspaceID string, channelID string, requestID string, status string) (GuestRequestDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return GuestRequestDTO{}, err
	}
	if status != "approved" && status != "rejected" {
		return GuestRequestDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Trạng thái duyệt khách không hợp lệ.")
	}
	if status == "approved" {
		if err := s.ensureDirectInteractionAllowed(ctx, workspaceID, channelID, actorUserID); err != nil {
			return GuestRequestDTO{}, err
		}
	}
	guest, err := s.collaborationRepository().UpdateGuestRequestStatus(ctx, UpdateGuestRequestStatusParams{
		WorkspaceID: strings.TrimSpace(workspaceID),
		ChannelID:   strings.TrimSpace(channelID),
		RequestID:   strings.TrimSpace(requestID),
		Status:      status,
		ActorUserID: strings.TrimSpace(actorUserID),
	})
	if err != nil {
		if errors.Is(err, channelsdomain.ErrGuestNotFound) {
			return GuestRequestDTO{}, apperrors.NotFound("GUEST_REQUEST_NOT_FOUND", "Không tìm thấy yêu cầu tham gia.")
		}
		return GuestRequestDTO{}, err
	}
	return toGuestRequestDTO(guest), nil
}

func (s *Service) ListCollaborationRoles(ctx context.Context, actorUserID string, workspaceID string, channelID string) ([]CollaborationRoleDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	roles, err := s.collaborationRepository().ListCollaborationRoles(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return nil, err
	}
	result := make([]CollaborationRoleDTO, 0, len(roles))
	for _, role := range roles {
		result = append(result, toCollaborationRoleDTO(role))
	}
	return result, nil
}

func (s *Service) UpdateCollaborationRole(ctx context.Context, actorUserID string, workspaceID string, channelID string, userID string, role string) (CollaborationRoleDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return CollaborationRoleDTO{}, err
	}
	role = normalizeCollaborationRole(role)
	if role == "" {
		return CollaborationRoleDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Vai trò cuộc họp không hợp lệ.")
	}
	if role != "listener" {
		if err := s.ensureDirectInteractionAllowed(ctx, workspaceID, channelID, actorUserID); err != nil {
			return CollaborationRoleDTO{}, err
		}
	}
	member, err := s.repo.FindMember(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID), strings.TrimSpace(userID))
	if err != nil || (member.Status != "active" && member.Status != "muted") {
		return CollaborationRoleDTO{}, apperrors.NotFound("CHANNEL_MEMBER_NOT_FOUND", "Người dùng chưa phải thành viên hoạt động của phòng.")
	}
	updated, err := s.collaborationRepository().UpsertCollaborationRole(ctx, UpsertCollaborationRoleParams{
		WorkspaceID: strings.TrimSpace(workspaceID),
		ChannelID:   strings.TrimSpace(channelID),
		UserID:      strings.TrimSpace(userID),
		Role:        role,
		ActorUserID: strings.TrimSpace(actorUserID),
	})
	if err != nil {
		return CollaborationRoleDTO{}, err
	}
	return toCollaborationRoleDTO(updated), nil
}

func (s *Service) GetCollaborationDocument(ctx context.Context, actorUserID string, workspaceID string, channelID string, kind string) (CollaborationDocumentDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return CollaborationDocumentDTO{}, err
	}
	kind = normalizeDocumentKind(kind)
	if kind == "" {
		return CollaborationDocumentDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Loại tài liệu cộng tác không hợp lệ.")
	}
	document, err := s.collaborationRepository().GetCollaborationDocument(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID), kind)
	if err != nil {
		return CollaborationDocumentDTO{}, mapChannelError(err)
	}
	return toCollaborationDocumentDTO(document), nil
}

func (s *Service) UpdateCollaborationDocument(ctx context.Context, input UpdateCollaborationDocumentInput) (CollaborationDocumentDTO, error) {
	if err := s.ensureCollaborationMember(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return CollaborationDocumentDTO{}, err
	}
	if err := s.ensureDirectInteractionAllowed(ctx, input.WorkspaceID, input.ChannelID, input.ActorUserID); err != nil {
		return CollaborationDocumentDTO{}, err
	}
	kind := normalizeDocumentKind(input.Kind)
	if kind == "" {
		return CollaborationDocumentDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Loại tài liệu cộng tác không hợp lệ.")
	}
	content := input.Content
	if len(content) == 0 {
		content = json.RawMessage(`{}`)
	}
	if len(content) > maxCollaborationDocumentBytes || !json.Valid(content) {
		return CollaborationDocumentDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Nội dung tài liệu không hợp lệ hoặc vượt quá 512 KB.")
	}
	document, err := s.collaborationRepository().UpdateCollaborationDocument(ctx, UpdateCollaborationDocumentParams{
		WorkspaceID:     strings.TrimSpace(input.WorkspaceID),
		ChannelID:       strings.TrimSpace(input.ChannelID),
		Kind:            kind,
		Content:         content,
		ExpectedVersion: input.ExpectedVersion,
		ActorUserID:     strings.TrimSpace(input.ActorUserID),
	})
	if errors.Is(err, channelsdomain.ErrVersionConflict) {
		return CollaborationDocumentDTO{}, apperrors.Conflict("DOCUMENT_VERSION_CONFLICT", "Tài liệu vừa được người khác cập nhật. Hãy tải lại trước khi lưu.")
	}
	if err != nil {
		return CollaborationDocumentDTO{}, err
	}
	return toCollaborationDocumentDTO(document), nil
}

func (s *Service) ListChannelTasks(ctx context.Context, actorUserID string, workspaceID string, channelID string) ([]ChannelTaskDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	tasks, err := s.collaborationRepository().ListChannelTasks(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return nil, err
	}
	return toChannelTaskDTOs(tasks), nil
}

func (s *Service) CreateChannelTask(ctx context.Context, input CreateChannelTaskInput) (ChannelTaskDTO, error) {
	if err := s.ensureCollaborationMember(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return ChannelTaskDTO{}, err
	}
	if err := s.ensureDirectInteractionAllowed(ctx, input.WorkspaceID, input.ChannelID, input.ActorUserID); err != nil {
		return ChannelTaskDTO{}, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" || len([]rune(title)) > 240 {
		return ChannelTaskDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tiêu đề công việc phải có từ 1 đến 240 ký tự.")
	}
	dueAt, err := parseOptionalRFC3339(input.DueAt)
	if err != nil {
		return ChannelTaskDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Thời hạn công việc không hợp lệ.")
	}
	task, err := s.collaborationRepository().CreateChannelTask(ctx, CreateChannelTaskParams{
		WorkspaceID:     strings.TrimSpace(input.WorkspaceID),
		ChannelID:       strings.TrimSpace(input.ChannelID),
		SourceMessageID: strings.TrimSpace(input.SourceMessageID),
		Title:           title,
		Description:     strings.TrimSpace(input.Description),
		AssigneeUserID:  strings.TrimSpace(input.AssigneeUserID),
		DueAt:           dueAt,
		ActorUserID:     strings.TrimSpace(input.ActorUserID),
	})
	if errors.Is(err, channelsdomain.ErrTaskConflict) {
		return ChannelTaskDTO{}, apperrors.Conflict("TASK_ALREADY_EXISTS", "Tin nhắn này đã được chuyển thành công việc.")
	}
	if err != nil {
		return ChannelTaskDTO{}, err
	}
	return toChannelTaskDTO(task), nil
}

func (s *Service) UpdateChannelTask(ctx context.Context, input UpdateChannelTaskInput) (ChannelTaskDTO, error) {
	if err := s.ensureCollaborationMember(ctx, input.ActorUserID, input.WorkspaceID, input.ChannelID); err != nil {
		return ChannelTaskDTO{}, err
	}
	if err := s.ensureDirectInteractionAllowed(ctx, input.WorkspaceID, input.ChannelID, input.ActorUserID); err != nil {
		return ChannelTaskDTO{}, err
	}
	status := strings.TrimSpace(input.Status)
	switch status {
	case "open", "in_progress", "done", "cancelled":
	default:
		return ChannelTaskDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Trạng thái công việc không hợp lệ.")
	}
	var dueAt *time.Time
	clearDueAt := false
	if input.DueAt != nil {
		if strings.TrimSpace(*input.DueAt) == "" {
			clearDueAt = true
		} else {
			parsed, err := parseOptionalRFC3339(*input.DueAt)
			if err != nil {
				return ChannelTaskDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Thời hạn công việc không hợp lệ.")
			}
			dueAt = parsed
		}
	}
	task, err := s.collaborationRepository().UpdateChannelTask(ctx, UpdateChannelTaskParams{
		WorkspaceID:    strings.TrimSpace(input.WorkspaceID),
		ChannelID:      strings.TrimSpace(input.ChannelID),
		TaskID:         strings.TrimSpace(input.TaskID),
		Status:         status,
		AssigneeUserID: cleanOptional(input.AssigneeUserID),
		DueAt:          dueAt,
		ClearDueAt:     clearDueAt,
		ActorUserID:    strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return ChannelTaskDTO{}, err
	}
	return toChannelTaskDTO(task), nil
}

func (s *Service) ListBreakoutRooms(ctx context.Context, actorUserID string, workspaceID string, channelID string) ([]BreakoutRoomDTO, error) {
	if err := s.ensureCollaborationMember(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	rooms, err := s.collaborationRepository().ListBreakoutRooms(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return nil, err
	}
	result := toBreakoutRoomDTOs(rooms)
	if s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID) != nil {
		filtered := make([]BreakoutRoomDTO, 0, 1)
		for _, room := range result {
			for _, userID := range room.AssignedUserIDs {
				if userID == strings.TrimSpace(actorUserID) {
					filtered = append(filtered, room)
					break
				}
			}
		}
		result = filtered
	}
	return result, nil
}

func (s *Service) CreateBreakoutRoom(ctx context.Context, actorUserID string, workspaceID string, channelID string, name string, assignedUserIDs []string) (BreakoutRoomDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return BreakoutRoomDTO{}, err
	}
	if err := s.ensureDirectInteractionAllowed(ctx, workspaceID, channelID, actorUserID); err != nil {
		return BreakoutRoomDTO{}, err
	}
	channel, err := s.repo.FindChannel(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return BreakoutRoomDTO{}, mapChannelError(err)
	}
	if channel.Type == "direct" {
		return BreakoutRoomDTO{}, apperrors.Conflict("PROMOTION_REQUIRED", "Hãy chuyển cuộc trò chuyện riêng thành phòng nhóm trước khi tạo breakout room.")
	}
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 2 || len([]rune(name)) > 80 {
		return BreakoutRoomDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên phòng nhỏ phải có từ 2 đến 80 ký tự.")
	}
	settings, err := s.collaborationRepository().GetCollaborationSettings(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return BreakoutRoomDTO{}, err
	}
	if settings.PublicAccessEnabled || settings.RoomMode == "public" || settings.RoomMode == "webinar" {
		return BreakoutRoomDTO{}, apperrors.Conflict(
			"BREAKOUT_INTERNAL_ONLY",
			"Breakout rooms are available only in internal group conversations.",
		)
	}
	assignedUserIDs = normalizeStringIDs(assignedUserIDs)
	if len(assignedUserIDs) > 50 {
		return BreakoutRoomDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Mỗi phòng nhỏ hỗ trợ tối đa 50 thành viên.")
	}
	for _, userID := range assignedUserIDs {
		member, err := s.repo.FindMember(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID), userID)
		if err != nil || (member.Status != "active" && member.Status != "muted") {
			return BreakoutRoomDTO{}, apperrors.BadRequest("INVALID_PARTICIPANTS", "Danh sách phòng nhỏ chứa người không thuộc phòng chính.")
		}
	}
	room, err := s.collaborationRepository().CreateBreakoutRoom(ctx, CreateBreakoutRoomParams{
		WorkspaceID:     strings.TrimSpace(workspaceID),
		ChannelID:       strings.TrimSpace(channelID),
		Name:            name,
		AssignedUserIDs: assignedUserIDs,
		AssignmentMode:  "manual",
		ActorUserID:     strings.TrimSpace(actorUserID),
	})
	if err != nil {
		return BreakoutRoomDTO{}, err
	}
	return toBreakoutRoomDTO(room), nil
}

func (s *Service) CloseBreakoutRooms(ctx context.Context, actorUserID string, workspaceID string, channelID string, roomID string) ([]BreakoutRoomDTO, error) {
	if err := s.ensureCanManageCollaboration(ctx, actorUserID, workspaceID, channelID); err != nil {
		return nil, err
	}
	rooms, err := s.collaborationRepository().CloseBreakoutRooms(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID), strings.TrimSpace(roomID))
	if err != nil {
		return nil, err
	}
	return toBreakoutRoomDTOs(rooms), nil
}

func (s *Service) collaborationRepository() CollaborationRepository {
	return s.collab
}

func (s *Service) ensureCollaborationMember(ctx context.Context, userID string, workspaceID string, channelID string) error {
	if s.collab == nil {
		return apperrors.ServiceUnavailable("COLLABORATION_NOT_CONFIGURED", "Dịch vụ cộng tác chưa được cấu hình.")
	}
	if err := s.ensureWorkspaceAccess(ctx, userID, workspaceID); err != nil {
		return err
	}
	member, err := s.repo.FindMember(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID), strings.TrimSpace(userID))
	if err != nil || (member.Status != "active" && member.Status != "muted") {
		return apperrors.Forbidden("Bạn chưa phải thành viên hoạt động của phòng.")
	}
	return nil
}

func (s *Service) ensureCanManageCollaboration(ctx context.Context, userID string, workspaceID string, channelID string) error {
	if s.collab == nil {
		return apperrors.ServiceUnavailable("COLLABORATION_NOT_CONFIGURED", "Dịch vụ cộng tác chưa được cấu hình.")
	}
	channel, err := s.repo.FindChannel(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(channelID))
	if err != nil {
		return mapChannelError(err)
	}
	if channel.CreatedBy != nil && *channel.CreatedBy == strings.TrimSpace(userID) {
		return nil
	}
	return s.ensurePermission(ctx, userID, workspaceID, "channel.manage")
}

func (s *Service) publicSettings(ctx context.Context, publicToken string) (channelsdomain.CollaborationSettings, error) {
	if s.collab == nil {
		return channelsdomain.CollaborationSettings{}, apperrors.ServiceUnavailable("COLLABORATION_NOT_CONFIGURED", "Dịch vụ cộng tác chưa được cấu hình.")
	}
	publicToken = strings.TrimSpace(publicToken)
	if len(publicToken) < 24 {
		return channelsdomain.CollaborationSettings{}, apperrors.NotFound("PUBLIC_ROOM_NOT_FOUND", "Link phòng họp không tồn tại hoặc đã bị thu hồi.")
	}
	settings, err := s.collaborationRepository().FindPublicSettings(ctx, hashOpaqueToken(publicToken))
	if err != nil || !settings.PublicAccessEnabled {
		return channelsdomain.CollaborationSettings{}, apperrors.NotFound("PUBLIC_ROOM_NOT_FOUND", "Link phòng họp không tồn tại hoặc đã bị thu hồi.")
	}
	return settings, nil
}

func normalizeRoomMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "internal", "public", "webinar":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeMeetingProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "jitsi":
		return "jitsi"
	case "webrtc":
		return "webrtc"
	default:
		return ""
	}
}

func normalizeCollaborationRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "moderator", "presenter", "member", "listener":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeDocumentKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "notes", "whiteboard":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func randomURLToken(byteLength int) (string, error) {
	value := make([]byte, byteLength)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func hashOpaqueToken(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func parseOptionalRFC3339(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	utc := parsed.UTC()
	return &utc, nil
}

func normalizeStringIDs(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func toCollaborationSettingsDTO(settings channelsdomain.CollaborationSettings, includeMeetingRoom bool, meetingBaseURL string) CollaborationSettingsDTO {
	dto := CollaborationSettingsDTO{
		ChannelID:              settings.ChannelID,
		WorkspaceID:            settings.WorkspaceID,
		ChannelName:            settings.ChannelName,
		ChannelType:            settings.ChannelType,
		RoomMode:               settings.RoomMode,
		MeetingProvider:        settings.MeetingProvider,
		MeetingBaseURL:         meetingBaseURL,
		PublicAccessEnabled:    settings.PublicAccessEnabled,
		PublicTokenPrefix:      settings.PublicTokenPrefix,
		HasPassword:            settings.PasswordHash != nil && strings.TrimSpace(*settings.PasswordHash) != "",
		LobbyEnabled:           settings.LobbyEnabled,
		ChatLocked:             settings.ChatLocked,
		GuestMicrophoneEnabled: settings.GuestMicrophoneEnabled,
		GuestCameraEnabled:     settings.GuestCameraEnabled,
		DefaultParticipantRole: settings.DefaultParticipantRole,
		CreatedAt:              formatTime(settings.CreatedAt),
		UpdatedAt:              formatTime(settings.UpdatedAt),
	}
	if includeMeetingRoom {
		dto.MeetingRoomKey = settings.MeetingRoomKey
	}
	return dto
}

func toPublicRoomDTO(settings channelsdomain.CollaborationSettings, includeMeetingRoom bool, meetingBaseURL string) PublicRoomDTO {
	dto := PublicRoomDTO{
		ChannelID:              settings.ChannelID,
		ChannelName:            settings.ChannelName,
		RoomMode:               settings.RoomMode,
		MeetingProvider:        settings.MeetingProvider,
		MeetingBaseURL:         meetingBaseURL,
		HasPassword:            settings.PasswordHash != nil && strings.TrimSpace(*settings.PasswordHash) != "",
		LobbyEnabled:           settings.LobbyEnabled,
		ChatLocked:             settings.ChatLocked,
		GuestMicrophoneEnabled: settings.GuestMicrophoneEnabled,
		GuestCameraEnabled:     settings.GuestCameraEnabled,
	}
	if includeMeetingRoom {
		dto.MeetingRoomKey = settings.MeetingRoomKey
	}
	return dto
}

func toGuestRequestDTO(guest channelsdomain.GuestRequest) GuestRequestDTO {
	return GuestRequestDTO{
		ID:          guest.ID,
		ChannelID:   guest.ChannelID,
		DisplayName: guest.DisplayName,
		Status:      guest.Status,
		ExpiresAt:   formatTime(guest.ExpiresAt),
		CreatedAt:   formatTime(guest.CreatedAt),
		UpdatedAt:   formatTime(guest.UpdatedAt),
	}
}

func toCollaborationRoleDTO(role channelsdomain.CollaborationRole) CollaborationRoleDTO {
	return CollaborationRoleDTO{
		ChannelID:   role.ChannelID,
		UserID:      role.UserID,
		DisplayName: role.DisplayName,
		Username:    role.Username,
		AvatarURL:   role.AvatarURL,
		Role:        role.Role,
		UpdatedAt:   formatTime(role.UpdatedAt),
	}
}

func toCollaborationDocumentDTO(document channelsdomain.CollaborationDocument) CollaborationDocumentDTO {
	content := document.Content
	if len(content) == 0 || !json.Valid(content) {
		content = json.RawMessage(`{}`)
	}
	return CollaborationDocumentDTO{
		ChannelID: document.ChannelID,
		Kind:      document.Kind,
		Content:   content,
		Version:   document.Version,
		UpdatedBy: document.UpdatedBy,
		CreatedAt: formatTime(document.CreatedAt),
		UpdatedAt: formatTime(document.UpdatedAt),
	}
}

func toChannelTaskDTOs(tasks []channelsdomain.ChannelTask) []ChannelTaskDTO {
	result := make([]ChannelTaskDTO, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, toChannelTaskDTO(task))
	}
	return result
}

func toChannelTaskDTO(task channelsdomain.ChannelTask) ChannelTaskDTO {
	return ChannelTaskDTO{
		ID:              task.ID,
		WorkspaceID:     task.WorkspaceID,
		ChannelID:       task.ChannelID,
		SourceMessageID: task.SourceMessageID,
		Title:           task.Title,
		Description:     task.Description,
		Status:          task.Status,
		AssigneeUserID:  task.AssigneeUserID,
		DueAt:           formatOptionalTime(task.DueAt),
		CreatedBy:       task.CreatedBy,
		CompletedAt:     formatOptionalTime(task.CompletedAt),
		CreatedAt:       formatTime(task.CreatedAt),
		UpdatedAt:       formatTime(task.UpdatedAt),
	}
}

func toBreakoutRoomDTOs(rooms []channelsdomain.BreakoutRoom) []BreakoutRoomDTO {
	result := make([]BreakoutRoomDTO, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, toBreakoutRoomDTO(room))
	}
	return result
}

func toBreakoutRoomDTO(room channelsdomain.BreakoutRoom) BreakoutRoomDTO {
	var assignedUserIDs []string
	if len(room.AssignedUserIDs) > 0 {
		_ = json.Unmarshal(room.AssignedUserIDs, &assignedUserIDs)
	}
	return BreakoutRoomDTO{
		ID:              room.ID,
		ChannelID:       room.ChannelID,
		Name:            room.Name,
		RoomKey:         room.RoomKey,
		AssignedUserIDs: assignedUserIDs,
		Status:          room.Status,
		AssignmentMode:  room.AssignmentMode,
		AllowSelfSelect: room.AllowSelfSelect,
		StartedAt:       formatOptionalTime(room.StartedAt),
		Sequence:        room.Sequence,
		CreatedBy:       room.CreatedBy,
		ClosedAt:        formatOptionalTime(room.ClosedAt),
		CreatedAt:       formatTime(room.CreatedAt),
		UpdatedAt:       formatTime(room.UpdatedAt),
	}
}
