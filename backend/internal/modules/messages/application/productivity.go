package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const (
	maxScheduledMessageDelay = 365 * 24 * time.Hour
	maxReminderDelay         = 365 * 24 * time.Hour
)

type ProductivityRepository interface {
	ScheduleMessage(ctx context.Context, params ScheduleMessageParams) (ScheduledMessageDTO, error)
	ListScheduledMessages(ctx context.Context, params UserProductivityParams) ([]ScheduledMessageDTO, error)
	CancelScheduledMessage(ctx context.Context, params ScheduledMessageRef) error
	ProcessDueScheduledMessages(ctx context.Context, limit int, authorizer ScheduledMessageDeliveryAuthorizer) (int, error)
	CreateReminder(ctx context.Context, params CreateReminderParams) (MessageReminderDTO, error)
	ListReminders(ctx context.Context, params UserProductivityParams) ([]MessageReminderDTO, error)
	CancelReminder(ctx context.Context, params ReminderRef) error
	ProcessDueReminders(ctx context.Context, limit int) (int, error)
	GetThreadDetails(ctx context.Context, params ThreadDetailsParams) (ThreadDetailsDTO, error)
	UpsertThreadDetails(ctx context.Context, params UpsertThreadDetailsParams) (ThreadDetailsDTO, error)
	ListThreadDetails(ctx context.Context, params ListThreadDetailsParams) ([]ThreadDetailsDTO, error)
	SetThreadSubscription(ctx context.Context, params ThreadSubscriptionParams) (ThreadDetailsDTO, error)
	MarkThreadRead(ctx context.Context, params ThreadReadParams) (ThreadDetailsDTO, error)
}

type ScheduledMessageDelivery struct {
	WorkspaceID string
	ChannelID   string
	SenderID    string
}

type ScheduledMessageDeliveryAuthorizer interface {
	AuthorizeScheduledMessageDelivery(ctx context.Context, delivery ScheduledMessageDelivery) error
}

type ScheduleMessageInput struct {
	ActorUserID      string
	WorkspaceID      string
	ChannelID        string
	ParentID         string
	ClientMessageID  string
	Kind             string
	Body             string
	Metadata         json.RawMessage
	MentionedUserIDs []string
	ScheduledFor     string
	Silent           bool
}

type ScheduleMessageParams struct {
	WorkspaceID      string
	ChannelID        string
	SenderID         string
	ParentID         string
	ClientMessageID  string
	Kind             string
	Body             string
	Metadata         []byte
	MentionedUserIDs []string
	ScheduledFor     time.Time
}

type ScheduledMessageRef struct {
	WorkspaceID string
	SenderID    string
	ID          string
}

type UserProductivityParams struct {
	WorkspaceID string
	UserID      string
	ChannelID   string
	Limit       int
}

type ScheduledMessageDTO struct {
	ID              string          `json:"id"`
	WorkspaceID     string          `json:"workspace_id"`
	ChannelID       string          `json:"channel_id"`
	SenderID        string          `json:"sender_id"`
	ParentID        *string         `json:"parent_id,omitempty"`
	Kind            string          `json:"kind"`
	Body            string          `json:"body"`
	Metadata        json.RawMessage `json:"metadata"`
	ScheduledFor    string          `json:"scheduled_for"`
	Status          string          `json:"status"`
	SentMessageID   *string         `json:"sent_message_id,omitempty"`
	AttemptCount    int             `json:"attempt_count"`
	LastError       *string         `json:"last_error,omitempty"`
	ClientMessageID *string         `json:"client_message_id,omitempty"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
}

type CreateReminderInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	MessageID   string
	RemindAt    string
	Note        string
}

type CreateReminderParams struct {
	WorkspaceID string
	ChannelID   string
	MessageID   string
	UserID      string
	RemindAt    time.Time
	Note        string
}

type ReminderRef struct {
	WorkspaceID string
	UserID      string
	ID          string
}

type MessageReminderDTO struct {
	ID             string  `json:"id"`
	WorkspaceID    string  `json:"workspace_id"`
	ChannelID      string  `json:"channel_id"`
	MessageID      string  `json:"message_id"`
	UserID         string  `json:"user_id"`
	RemindAt       string  `json:"remind_at"`
	Note           *string `json:"note,omitempty"`
	Status         string  `json:"status"`
	NotificationID *string `json:"notification_id,omitempty"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type ThreadDetailsParams struct {
	WorkspaceID string
	ChannelID   string
	RootID      string
	UserID      string
}

type UpsertThreadDetailsParams struct {
	ThreadDetailsParams
	Title       string
	Description string
	Status      string
}

type ListThreadDetailsParams struct {
	WorkspaceID    string
	ChannelID      string
	UserID         string
	SubscribedOnly bool
	Limit          int
}

type ThreadSubscriptionParams struct {
	ThreadDetailsParams
	Subscribed bool
}

type ThreadReadParams struct {
	ThreadDetailsParams
	LastReadMessageID string
}

type ThreadDetailsDTO struct {
	WorkspaceID   string  `json:"workspace_id"`
	ChannelID     string  `json:"channel_id"`
	RootMessageID string  `json:"root_message_id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Status        string  `json:"status"`
	Subscribed    bool    `json:"subscribed"`
	ReplyCount    int     `json:"reply_count"`
	UnreadCount   int     `json:"unread_count"`
	LastReplyAt   *string `json:"last_reply_at,omitempty"`
	LastReadAt    *string `json:"last_read_at,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

func (s *Service) ScheduleMessage(ctx context.Context, input ScheduleMessageInput) (ScheduledMessageDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return ScheduledMessageDTO{}, err
	}
	if err := s.ensureDirectInteractionAllowed(ctx, input.WorkspaceID, input.ChannelID, input.ActorUserID); err != nil {
		return ScheduledMessageDTO{}, err
	}
	kind := strings.ToLower(strings.TrimSpace(input.Kind))
	if kind == "" {
		kind = "text"
	}
	if kind != "text" && kind != "file" && kind != "event" {
		return ScheduledMessageDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Loại tin nhắn không hợp lệ.")
	}
	body := strings.TrimSpace(input.Body)
	if body == "" || len([]rune(body)) > 8000 {
		return ScheduledMessageDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Nội dung tin nhắn phải dài từ 1 đến 8000 ký tự.")
	}
	scheduledFor, err := time.Parse(time.RFC3339, strings.TrimSpace(input.ScheduledFor))
	if err != nil {
		return ScheduledMessageDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "scheduled_for phải dùng định dạng RFC3339.")
	}
	now := time.Now().UTC()
	scheduledFor = scheduledFor.UTC()
	if scheduledFor.Before(now.Add(15*time.Second)) || scheduledFor.After(now.Add(maxScheduledMessageDelay)) {
		return ScheduledMessageDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Thời gian gửi phải sau hiện tại ít nhất 15 giây và không quá 365 ngày.")
	}
	metadata, err := normalizeMetadata(input.Metadata)
	if err != nil {
		return ScheduledMessageDTO{}, err
	}
	if input.Silent {
		metadata, err = withMetadataValue(metadata, "silent", true)
		if err != nil {
			return ScheduledMessageDTO{}, err
		}
	}
	clientMessageID := normalizeClientMessageID(input.ClientMessageID)
	if clientMessageID != "" {
		metadata, err = withClientMessageID(metadata, clientMessageID)
		if err != nil {
			return ScheduledMessageDTO{}, err
		}
	}
	return s.productivityRepository().ScheduleMessage(ctx, ScheduleMessageParams{
		WorkspaceID:      strings.TrimSpace(input.WorkspaceID),
		ChannelID:        strings.TrimSpace(input.ChannelID),
		SenderID:         strings.TrimSpace(input.ActorUserID),
		ParentID:         strings.TrimSpace(input.ParentID),
		ClientMessageID:  clientMessageID,
		Kind:             kind,
		Body:             body,
		Metadata:         metadata,
		MentionedUserIDs: normalizeMentions(body, input.MentionedUserIDs),
		ScheduledFor:     scheduledFor,
	})
}

func (s *Service) ListScheduledMessages(ctx context.Context, actorUserID string, workspaceID string, channelID string, limit int) ([]ScheduledMessageDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID, "message.send"); err != nil {
		return nil, err
	}
	return s.productivityRepository().ListScheduledMessages(ctx, UserProductivityParams{
		WorkspaceID: strings.TrimSpace(workspaceID),
		UserID:      strings.TrimSpace(actorUserID),
		ChannelID:   strings.TrimSpace(channelID),
		Limit:       normalizeProductivityLimit(limit),
	})
}

func (s *Service) CancelScheduledMessage(ctx context.Context, actorUserID string, workspaceID string, id string) error {
	return s.productivityRepository().CancelScheduledMessage(ctx, ScheduledMessageRef{
		WorkspaceID: strings.TrimSpace(workspaceID),
		SenderID:    strings.TrimSpace(actorUserID),
		ID:          strings.TrimSpace(id),
	})
}

func (s *Service) CreateReminder(ctx context.Context, input CreateReminderInput) (MessageReminderDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return MessageReminderDTO{}, err
	}
	remindAt, err := time.Parse(time.RFC3339, strings.TrimSpace(input.RemindAt))
	if err != nil {
		return MessageReminderDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "remind_at phải dùng định dạng RFC3339.")
	}
	now := time.Now().UTC()
	remindAt = remindAt.UTC()
	if remindAt.Before(now.Add(15*time.Second)) || remindAt.After(now.Add(maxReminderDelay)) {
		return MessageReminderDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Lời nhắc phải sau hiện tại ít nhất 15 giây và không quá 365 ngày.")
	}
	note := strings.TrimSpace(input.Note)
	if len([]rune(note)) > 500 {
		return MessageReminderDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Ghi chú lời nhắc không được quá 500 ký tự.")
	}
	if _, err := s.repo.Get(ctx, MessageRef{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		MessageID:   strings.TrimSpace(input.MessageID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
	}); err != nil {
		return MessageReminderDTO{}, mapMessageError(err)
	}
	return s.productivityRepository().CreateReminder(ctx, CreateReminderParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		MessageID:   strings.TrimSpace(input.MessageID),
		UserID:      strings.TrimSpace(input.ActorUserID),
		RemindAt:    remindAt,
		Note:        note,
	})
}

func (s *Service) ListReminders(ctx context.Context, actorUserID string, workspaceID string, channelID string, limit int) ([]MessageReminderDTO, error) {
	return s.productivityRepository().ListReminders(ctx, UserProductivityParams{
		WorkspaceID: strings.TrimSpace(workspaceID),
		UserID:      strings.TrimSpace(actorUserID),
		ChannelID:   strings.TrimSpace(channelID),
		Limit:       normalizeProductivityLimit(limit),
	})
}

func (s *Service) CancelReminder(ctx context.Context, actorUserID string, workspaceID string, id string) error {
	return s.productivityRepository().CancelReminder(ctx, ReminderRef{
		WorkspaceID: strings.TrimSpace(workspaceID),
		UserID:      strings.TrimSpace(actorUserID),
		ID:          strings.TrimSpace(id),
	})
}

func (s *Service) GetThreadDetails(ctx context.Context, actorUserID string, workspaceID string, channelID string, rootID string) (ThreadDetailsDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID, "message.send"); err != nil {
		return ThreadDetailsDTO{}, err
	}
	return s.productivityRepository().GetThreadDetails(ctx, ThreadDetailsParams{
		WorkspaceID: strings.TrimSpace(workspaceID),
		ChannelID:   strings.TrimSpace(channelID),
		RootID:      strings.TrimSpace(rootID),
		UserID:      strings.TrimSpace(actorUserID),
	})
}

func (s *Service) UpsertThreadDetails(ctx context.Context, actorUserID string, workspaceID string, channelID string, rootID string, title string, description string, status string) (ThreadDetailsDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID, "message.send"); err != nil {
		return ThreadDetailsDTO{}, err
	}
	if err := s.ensureDirectInteractionAllowed(ctx, workspaceID, channelID, actorUserID); err != nil {
		return ThreadDetailsDTO{}, err
	}
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	status = strings.ToLower(strings.TrimSpace(status))
	if len([]rune(title)) > 160 || len([]rune(description)) > 1000 {
		return ThreadDetailsDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tiêu đề hoặc mô tả thread quá dài.")
	}
	if status == "" {
		status = "open"
	}
	if status != "open" && status != "resolved" {
		return ThreadDetailsDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Trạng thái thread không hợp lệ.")
	}
	return s.productivityRepository().UpsertThreadDetails(ctx, UpsertThreadDetailsParams{
		ThreadDetailsParams: ThreadDetailsParams{
			WorkspaceID: strings.TrimSpace(workspaceID),
			ChannelID:   strings.TrimSpace(channelID),
			RootID:      strings.TrimSpace(rootID),
			UserID:      strings.TrimSpace(actorUserID),
		},
		Title: title, Description: description, Status: status,
	})
}

func (s *Service) ListThreadDetails(ctx context.Context, actorUserID string, workspaceID string, channelID string, subscribedOnly bool, limit int) ([]ThreadDetailsDTO, error) {
	if err := s.ensurePermission(ctx, actorUserID, workspaceID, "message.send"); err != nil {
		return nil, err
	}
	return s.productivityRepository().ListThreadDetails(ctx, ListThreadDetailsParams{
		WorkspaceID:    strings.TrimSpace(workspaceID),
		ChannelID:      strings.TrimSpace(channelID),
		UserID:         strings.TrimSpace(actorUserID),
		SubscribedOnly: subscribedOnly,
		Limit:          normalizeProductivityLimit(limit),
	})
}

func (s *Service) SetThreadSubscription(ctx context.Context, actorUserID string, workspaceID string, channelID string, rootID string, subscribed bool) (ThreadDetailsDTO, error) {
	return s.productivityRepository().SetThreadSubscription(ctx, ThreadSubscriptionParams{
		ThreadDetailsParams: ThreadDetailsParams{
			WorkspaceID: strings.TrimSpace(workspaceID),
			ChannelID:   strings.TrimSpace(channelID),
			RootID:      strings.TrimSpace(rootID),
			UserID:      strings.TrimSpace(actorUserID),
		},
		Subscribed: subscribed,
	})
}

func (s *Service) MarkThreadRead(ctx context.Context, actorUserID string, workspaceID string, channelID string, rootID string, lastReadMessageID string) (ThreadDetailsDTO, error) {
	return s.productivityRepository().MarkThreadRead(ctx, ThreadReadParams{
		ThreadDetailsParams: ThreadDetailsParams{
			WorkspaceID: strings.TrimSpace(workspaceID),
			ChannelID:   strings.TrimSpace(channelID),
			RootID:      strings.TrimSpace(rootID),
			UserID:      strings.TrimSpace(actorUserID),
		},
		LastReadMessageID: strings.TrimSpace(lastReadMessageID),
	})
}

func (s *Service) ProcessDueScheduledMessages(ctx context.Context, limit int) (int, error) {
	return s.productivityRepository().ProcessDueScheduledMessages(ctx, normalizeWorkerLimit(limit), s)
}

func (s *Service) ProcessDueReminders(ctx context.Context, limit int) (int, error) {
	return s.productivityRepository().ProcessDueReminders(ctx, normalizeWorkerLimit(limit))
}

func (s *Service) productivityRepository() ProductivityRepository {
	repository, ok := s.repo.(ProductivityRepository)
	if !ok {
		return unavailableProductivityRepository{}
	}
	return repository
}

func normalizeProductivityLimit(limit int) int {
	if limit <= 0 || limit > 200 {
		return 50
	}
	return limit
}

func normalizeWorkerLimit(limit int) int {
	if limit <= 0 || limit > 1000 {
		return 100
	}
	return limit
}
