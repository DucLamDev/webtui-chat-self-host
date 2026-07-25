package application

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	callsapp "github.com/duclamdev/application-chat/backend/internal/modules/calls/application"
	notificationsdomain "github.com/duclamdev/application-chat/backend/internal/modules/notifications/domain"
	outboxdomain "github.com/duclamdev/application-chat/backend/internal/modules/outbox/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type Repository interface {
	CreateMentionNotifications(ctx context.Context, params MentionParams) error
	CreateMessageNotifications(ctx context.Context, params MessageNotificationParams) error
	CreateIncomingCallNotification(ctx context.Context, params CallNotificationParams) error
	UpdateCallNotification(ctx context.Context, params CallNotificationParams) error
	GetPreference(ctx context.Context, zoneID string, userID string, workspaceID string) (notificationsdomain.NotificationPreference, error)
	ListForUser(ctx context.Context, params ListParams) ([]notificationsdomain.Notification, error)
	MarkRead(ctx context.Context, zoneID string, userID string, notificationID string) (notificationsdomain.Notification, error)
	MarkAllRead(ctx context.Context, zoneID string, userID string, workspaceID string) error
	ProcessPendingJobs(ctx context.Context, limit int) (int, error)
	UpsertPreference(ctx context.Context, zoneID string, preference notificationsdomain.NotificationPreference) (notificationsdomain.NotificationPreference, error)
	GetChannelPreference(ctx context.Context, zoneID string, userID string, workspaceID string, channelID string) (notificationsdomain.ChannelPreference, error)
	UpsertChannelPreference(ctx context.Context, zoneID string, preference notificationsdomain.ChannelPreference) (notificationsdomain.ChannelPreference, error)
}

type Service struct {
	repo Repository
}

type MentionParams struct {
	EventID          string
	WorkspaceID      string
	ChannelID        string
	MessageID        string
	SenderID         string
	MentionedUserIDs []string
}

type MessageNotificationParams struct {
	EventID          string
	WorkspaceID      string
	ChannelID        string
	MessageID        string
	SenderID         string
	MentionedUserIDs []string
}

type CallNotificationParams struct {
	CallID          string
	WorkspaceID     string
	ChannelID       string
	InitiatorUserID string
	TargetUserID    string
	Mode            string
	Status          string
}

type ListParams struct {
	ZoneID      string
	UserID      string
	WorkspaceID string
	Limit       int
}

type NotificationDTO struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id"`
	WorkspaceID *string         `json:"workspace_id,omitempty"`
	ChannelID   *string         `json:"channel_id,omitempty"`
	MessageID   *string         `json:"message_id,omitempty"`
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	Data        json.RawMessage `json:"data"`
	ReadAt      *string         `json:"read_at,omitempty"`
	DeliveredAt *string         `json:"delivered_at,omitempty"`
	CreatedAt   string          `json:"created_at"`
}

type PreferenceInput struct {
	ZoneID       string
	UserID       string
	WorkspaceID  string
	Mode         string
	Preview      *bool
	QuietHours   *bool
	QuietStart   string
	QuietEnd     string
	Sound        *bool
	Vibrate      *bool
	CallRinging  *bool
	BadgeEnabled *bool
}

type NotificationPreferenceDTO struct {
	UserID       string `json:"user_id"`
	WorkspaceID  string `json:"workspace_id"`
	Mode         string `json:"mode"`
	Preview      bool   `json:"preview"`
	QuietHours   bool   `json:"quiet_hours"`
	QuietStart   string `json:"quiet_start"`
	QuietEnd     string `json:"quiet_end"`
	Sound        bool   `json:"sound"`
	Vibrate      bool   `json:"vibrate"`
	CallRinging  bool   `json:"call_ringing"`
	BadgeEnabled bool   `json:"badge_enabled"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type ChannelPreferenceInput struct {
	ZoneID      string
	UserID      string
	WorkspaceID string
	ChannelID   string
	Mode        string
	MutedUntil  string
}

type ChannelPreferenceDTO struct {
	UserID      string  `json:"user_id"`
	WorkspaceID string  `json:"workspace_id"`
	ChannelID   string  `json:"channel_id"`
	Mode        string  `json:"mode"`
	MutedUntil  *string `json:"muted_until,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListMine(ctx context.Context, params ListParams) ([]NotificationDTO, error) {
	params.UserID = strings.TrimSpace(params.UserID)
	params.ZoneID = strings.TrimSpace(params.ZoneID)
	params.WorkspaceID = strings.TrimSpace(params.WorkspaceID)
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 50
	}
	notifications, err := s.repo.ListForUser(ctx, params)
	if err != nil {
		return nil, err
	}
	return toDTOs(notifications), nil
}

func (s *Service) MarkRead(ctx context.Context, zoneID string, userID string, notificationID string) (NotificationDTO, error) {
	notification, err := s.repo.MarkRead(ctx, strings.TrimSpace(zoneID), strings.TrimSpace(userID), strings.TrimSpace(notificationID))
	if err != nil {
		return NotificationDTO{}, mapNotificationError(err)
	}
	return toDTO(notification), nil
}

func (s *Service) MarkAllRead(ctx context.Context, zoneID string, userID string, workspaceID string) error {
	return s.repo.MarkAllRead(ctx, strings.TrimSpace(zoneID), strings.TrimSpace(userID), strings.TrimSpace(workspaceID))
}

func (s *Service) GetPreference(ctx context.Context, zoneID string, userID string, workspaceID string) (NotificationPreferenceDTO, error) {
	zoneID = strings.TrimSpace(zoneID)
	userID = strings.TrimSpace(userID)
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return NotificationPreferenceDTO{}, apperrors.BadRequest("WORKSPACE_REQUIRED", "workspace_id is required.")
	}
	preference, err := s.repo.GetPreference(ctx, zoneID, userID, workspaceID)
	if err != nil {
		return NotificationPreferenceDTO{}, mapNotificationError(err)
	}
	return toPreferenceDTO(preference), nil
}

func (s *Service) UpsertPreference(ctx context.Context, input PreferenceInput) (NotificationPreferenceDTO, error) {
	preference, err := normalizePreferenceInput(input)
	if err != nil {
		return NotificationPreferenceDTO{}, err
	}
	stored, err := s.repo.UpsertPreference(ctx, strings.TrimSpace(input.ZoneID), preference)
	if err != nil {
		return NotificationPreferenceDTO{}, mapNotificationError(err)
	}
	return toPreferenceDTO(stored), nil
}

func (s *Service) GetChannelPreference(ctx context.Context, zoneID string, userID string, workspaceID string, channelID string) (ChannelPreferenceDTO, error) {
	zoneID = strings.TrimSpace(zoneID)
	userID = strings.TrimSpace(userID)
	workspaceID = strings.TrimSpace(workspaceID)
	channelID = strings.TrimSpace(channelID)
	if workspaceID == "" || channelID == "" {
		return ChannelPreferenceDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "workspace_id và channel_id là bắt buộc.")
	}
	preference, err := s.repo.GetChannelPreference(ctx, zoneID, userID, workspaceID, channelID)
	if err != nil {
		return ChannelPreferenceDTO{}, mapNotificationError(err)
	}
	return toChannelPreferenceDTO(preference), nil
}

func (s *Service) UpsertChannelPreference(ctx context.Context, input ChannelPreferenceInput) (ChannelPreferenceDTO, error) {
	preference, err := normalizeChannelPreferenceInput(input)
	if err != nil {
		return ChannelPreferenceDTO{}, err
	}
	stored, err := s.repo.UpsertChannelPreference(ctx, strings.TrimSpace(input.ZoneID), preference)
	if err != nil {
		return ChannelPreferenceDTO{}, mapNotificationError(err)
	}
	return toChannelPreferenceDTO(stored), nil
}

func (s *Service) Handle(ctx context.Context, event outboxdomain.Event) error {
	if event.EventType != "MessageCreated" {
		return nil
	}
	var payload struct {
		WorkspaceID      string   `json:"workspace_id"`
		ChannelID        string   `json:"channel_id"`
		MessageID        string   `json:"message_id"`
		SenderID         string   `json:"sender_id"`
		MentionedUserIDs []string `json:"mentioned_user_ids"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	return s.repo.CreateMessageNotifications(ctx, MessageNotificationParams{
		EventID:          event.ID,
		WorkspaceID:      payload.WorkspaceID,
		ChannelID:        payload.ChannelID,
		MessageID:        payload.MessageID,
		SenderID:         payload.SenderID,
		MentionedUserIDs: payload.MentionedUserIDs,
	})
}

func (s *Service) NotifyIncomingCall(ctx context.Context, call callsapp.CallNotification) error {
	return s.repo.CreateIncomingCallNotification(ctx, callNotificationParams(call))
}

func (s *Service) NotifyCallTerminal(ctx context.Context, call callsapp.CallNotification) error {
	return s.repo.UpdateCallNotification(ctx, callNotificationParams(call))
}

func callNotificationParams(call callsapp.CallNotification) CallNotificationParams {
	return CallNotificationParams{
		CallID:          call.ID,
		WorkspaceID:     call.WorkspaceID,
		ChannelID:       call.ChannelID,
		InitiatorUserID: call.InitiatorUserID,
		TargetUserID:    call.TargetUserID,
		Mode:            call.Mode,
		Status:          call.Status,
	}
}

func (s *Service) ProcessJobs(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.ProcessPendingJobs(ctx, limit)
}

func mapNotificationError(err error) error {
	if errors.Is(err, notificationsdomain.ErrNotificationPreferenceUnavailable) {
		return apperrors.Forbidden("Workspace notification preference is unavailable.")
	}
	if errors.Is(err, notificationsdomain.ErrNotificationNotFound) {
		return apperrors.NotFound("NOTIFICATION_NOT_FOUND", "Không tìm thấy thông báo.")
	}
	return err
}

func normalizePreferenceInput(input PreferenceInput) (notificationsdomain.NotificationPreference, error) {
	userID := strings.TrimSpace(input.UserID)
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if workspaceID == "" {
		return notificationsdomain.NotificationPreference{}, apperrors.BadRequest("WORKSPACE_REQUIRED", "workspace_id is required.")
	}

	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = "all"
	}
	if mode != "all" && mode != "mentions" && mode != "muted" {
		return notificationsdomain.NotificationPreference{}, apperrors.BadRequest("INVALID_NOTIFICATION_MODE", "Notification mode must be all, mentions, or muted.")
	}

	preview := true
	if input.Preview != nil {
		preview = *input.Preview
	}
	quietHours := false
	if input.QuietHours != nil {
		quietHours = *input.QuietHours
	}

	quietStart := strings.TrimSpace(input.QuietStart)
	if quietStart == "" {
		quietStart = "22:00"
	}
	quietEnd := strings.TrimSpace(input.QuietEnd)
	if quietEnd == "" {
		quietEnd = "07:00"
	}
	if !isHHMM(quietStart) || !isHHMM(quietEnd) {
		return notificationsdomain.NotificationPreference{}, apperrors.BadRequest("INVALID_QUIET_HOURS", "Quiet hours must use HH:MM.")
	}
	sound := true
	if input.Sound != nil {
		sound = *input.Sound
	}
	vibrate := true
	if input.Vibrate != nil {
		vibrate = *input.Vibrate
	}
	callRinging := true
	if input.CallRinging != nil {
		callRinging = *input.CallRinging
	}
	badgeEnabled := true
	if input.BadgeEnabled != nil {
		badgeEnabled = *input.BadgeEnabled
	}

	return notificationsdomain.NotificationPreference{
		UserID:       userID,
		WorkspaceID:  workspaceID,
		Mode:         mode,
		Preview:      preview,
		QuietHours:   quietHours,
		QuietStart:   quietStart,
		QuietEnd:     quietEnd,
		Sound:        sound,
		Vibrate:      vibrate,
		CallRinging:  callRinging,
		BadgeEnabled: badgeEnabled,
	}, nil
}

func normalizeChannelPreferenceInput(input ChannelPreferenceInput) (notificationsdomain.ChannelPreference, error) {
	userID := strings.TrimSpace(input.UserID)
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	channelID := strings.TrimSpace(input.ChannelID)
	if workspaceID == "" || channelID == "" {
		return notificationsdomain.ChannelPreference{}, apperrors.BadRequest("VALIDATION_ERROR", "workspace_id và channel_id là bắt buộc.")
	}
	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = "all"
	}
	if mode != "all" && mode != "mentions" && mode != "muted" {
		return notificationsdomain.ChannelPreference{}, apperrors.BadRequest("INVALID_NOTIFICATION_MODE", "Notification mode must be all, mentions, or muted.")
	}
	var mutedUntil *time.Time
	if value := strings.TrimSpace(input.MutedUntil); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return notificationsdomain.ChannelPreference{}, apperrors.BadRequest("VALIDATION_ERROR", "muted_until phải dùng định dạng RFC3339.")
		}
		mutedUntil = &parsed
	}
	return notificationsdomain.ChannelPreference{
		UserID:      userID,
		WorkspaceID: workspaceID,
		ChannelID:   channelID,
		Mode:        mode,
		MutedUntil:  mutedUntil,
	}, nil
}

func toDTOs(notifications []notificationsdomain.Notification) []NotificationDTO {
	dtos := make([]NotificationDTO, 0, len(notifications))
	for _, notification := range notifications {
		dtos = append(dtos, toDTO(notification))
	}
	return dtos
}

func toDTO(notification notificationsdomain.Notification) NotificationDTO {
	data := json.RawMessage(notification.Data)
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	return NotificationDTO{
		ID:          notification.ID,
		UserID:      notification.UserID,
		WorkspaceID: notification.WorkspaceID,
		ChannelID:   notification.ChannelID,
		MessageID:   notification.MessageID,
		Type:        notification.Type,
		Title:       notification.Title,
		Body:        notification.Body,
		Data:        data,
		ReadAt:      formatOptionalTime(notification.ReadAt),
		DeliveredAt: formatOptionalTime(notification.DeliveredAt),
		CreatedAt:   formatTime(notification.CreatedAt),
	}
}

func toPreferenceDTO(preference notificationsdomain.NotificationPreference) NotificationPreferenceDTO {
	return NotificationPreferenceDTO{
		UserID:       preference.UserID,
		WorkspaceID:  preference.WorkspaceID,
		Mode:         preference.Mode,
		Preview:      preference.Preview,
		QuietHours:   preference.QuietHours,
		QuietStart:   preference.QuietStart,
		QuietEnd:     preference.QuietEnd,
		Sound:        preference.Sound,
		Vibrate:      preference.Vibrate,
		CallRinging:  preference.CallRinging,
		BadgeEnabled: preference.BadgeEnabled,
		CreatedAt:    formatTime(preference.CreatedAt),
		UpdatedAt:    formatTime(preference.UpdatedAt),
	}
}

func toChannelPreferenceDTO(preference notificationsdomain.ChannelPreference) ChannelPreferenceDTO {
	return ChannelPreferenceDTO{
		UserID:      preference.UserID,
		WorkspaceID: preference.WorkspaceID,
		ChannelID:   preference.ChannelID,
		Mode:        preference.Mode,
		MutedUntil:  formatOptionalTime(preference.MutedUntil),
		CreatedAt:   formatTime(preference.CreatedAt),
		UpdatedAt:   formatTime(preference.UpdatedAt),
	}
}

func isHHMM(value string) bool {
	if len(value) != 5 || value[2] != ':' {
		return false
	}
	_, err := time.Parse("15:04", value)
	return err == nil
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}
