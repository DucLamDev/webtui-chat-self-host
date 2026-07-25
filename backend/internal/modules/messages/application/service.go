package application

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	messagesdomain "github.com/duclamdev/application-chat/backend/internal/modules/messages/domain"
	"github.com/duclamdev/application-chat/backend/internal/shared/botauto"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
	"github.com/duclamdev/application-chat/backend/internal/shared/pagination"
)

var (
	mentionPattern = regexp.MustCompile(`<@([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})>`)
	uuidPattern    = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	Send(ctx context.Context, params SendParams) (messagesdomain.Message, error)
	Get(ctx context.Context, params MessageRef) (messagesdomain.Message, error)
	List(ctx context.Context, params ListParams) ([]messagesdomain.Message, error)
	ListThread(ctx context.Context, params ThreadParams) ([]messagesdomain.Message, error)
	Search(ctx context.Context, params SearchParams) ([]messagesdomain.Message, error)
	Forward(ctx context.Context, params ForwardParams) (messagesdomain.Message, error)
	Update(ctx context.Context, params UpdateParams) (messagesdomain.Message, error)
	Delete(ctx context.Context, params DeleteParams) error
	ListPins(ctx context.Context, params ListPinsParams) ([]messagesdomain.Message, error)
	Pin(ctx context.Context, params PinParams) (messagesdomain.Message, error)
	Unpin(ctx context.Context, params PinParams) error
	AddReaction(ctx context.Context, params ReactionParams) (messagesdomain.Message, error)
	RemoveReaction(ctx context.Context, params ReactionParams) (messagesdomain.Message, error)
}

type Service struct {
	repo           Repository
	checker        PermissionChecker
	realtime       RealtimePublisher
	autoResponders []botauto.Responder
}

type SendInput struct {
	ActorUserID      string
	WorkspaceID      string
	ChannelID        string
	ParentID         string
	ClientMessageID  string
	Kind             string
	Body             string
	Metadata         json.RawMessage
	MentionedUserIDs []string
}

type SendParams struct {
	WorkspaceID      string
	ChannelID        string
	SenderID         string
	ParentID         string
	ClientMessageID  string
	Kind             string
	Body             string
	Metadata         []byte
	MentionedUserIDs []string
}

type MessageRef struct {
	WorkspaceID string
	ChannelID   string
	MessageID   string
	ActorUserID string
}

type ListInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	Limit       int
	BeforeID    string
}

type ListParams struct {
	WorkspaceID string
	ChannelID   string
	ActorUserID string
	Limit       int
	BeforeID    string
}

type ThreadInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	MessageID   string
	Limit       int
}

type ThreadParams struct {
	WorkspaceID string
	ChannelID   string
	ActorUserID string
	MessageID   string
	Limit       int
}

type SearchInput struct {
	ActorUserID string
	WorkspaceID string
	Query       string
	ChannelID   string
	SenderID    string
	Kind        string
	DateFrom    string
	DateTo      string
	Limit       int
}

type SearchParams struct {
	WorkspaceID string
	ActorUserID string
	Query       string
	ChannelID   string
	SenderID    string
	Kind        string
	DateFrom    *time.Time
	DateTo      *time.Time
	Limit       int
}

type ForwardInput struct {
	ActorUserID     string
	WorkspaceID     string
	ChannelID       string
	MessageID       string
	TargetChannelID string
}

type ForwardParams struct {
	WorkspaceID     string
	SourceChannelID string
	MessageID       string
	TargetChannelID string
	ActorUserID     string
}

type UpdateInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	MessageID   string
	Body        string
}

type UpdateParams struct {
	WorkspaceID      string
	ChannelID        string
	MessageID        string
	ActorUserID      string
	Body             string
	MentionedUserIDs []string
}

type DeleteInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	MessageID   string
}

type DeleteParams struct {
	WorkspaceID string
	ChannelID   string
	MessageID   string
	ActorUserID string
}

type ListPinsInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
}

type ListPinsParams struct {
	WorkspaceID string
	ChannelID   string
	ActorUserID string
}

type PinInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	MessageID   string
}

type PinParams struct {
	WorkspaceID string
	ChannelID   string
	MessageID   string
	ActorUserID string
}

type ReactionInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	MessageID   string
	Emoji       string
}

type ReactionParams struct {
	WorkspaceID string
	ChannelID   string
	MessageID   string
	ActorUserID string
	Emoji       string
}

type MessageDTO struct {
	ID           string               `json:"id"`
	WorkspaceID  string               `json:"workspace_id"`
	ChannelID    string               `json:"channel_id"`
	SenderID     *string              `json:"sender_id,omitempty"`
	ParentID     *string              `json:"parent_id,omitempty"`
	ThreadRootID *string              `json:"thread_root_id,omitempty"`
	Kind         string               `json:"kind"`
	Body         string               `json:"body"`
	Metadata     json.RawMessage      `json:"metadata"`
	EditedAt     *string              `json:"edited_at,omitempty"`
	DeletedAt    *string              `json:"deleted_at,omitempty"`
	CreatedAt    string               `json:"created_at"`
	UpdatedAt    string               `json:"updated_at"`
	Mentions     []string             `json:"mentions"`
	Reactions    []ReactionSummaryDTO `json:"reactions"`
}

type ReactionSummaryDTO struct {
	Emoji       string `json:"emoji"`
	Count       int    `json:"count"`
	ReactedByMe bool   `json:"reacted_by_me"`
}

type RealtimePublisher interface {
	Publish(ctx context.Context, event RealtimeEvent) error
}

type RealtimeEvent struct {
	Type        string
	WorkspaceID string
	ChannelID   string
	MessageID   string
	Payload     map[string]any
}

func NewService(repo Repository, checker PermissionChecker, realtime ...RealtimePublisher) *Service {
	service := &Service{repo: repo, checker: checker}
	if len(realtime) > 0 {
		service.realtime = realtime[0]
	}
	return service
}

func (s *Service) SetAutoResponders(responders ...botauto.Responder) {
	s.autoResponders = responders
	activeCount := 0
	for _, responder := range responders {
		if responder != nil {
			activeCount++
		}
	}
	slog.Info("Da cau hinh auto responder cho message service",
		"count", len(responders),
		"active_count", activeCount,
	)
}

func (s *Service) Send(ctx context.Context, input SendInput) (MessageDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return MessageDTO{}, err
	}

	kind := strings.TrimSpace(input.Kind)
	if kind == "" {
		kind = "text"
	}
	if kind != "text" && kind != "file" && kind != "event" {
		return MessageDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Loại tin nhắn chỉ được là text hoặc file/media.")
	}

	body := strings.TrimSpace(input.Body)
	if body == "" || len([]rune(body)) > 8000 {
		return MessageDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Nội dung tin nhắn phải dài từ 1 đến 8000 ký tự.")
	}

	metadata, err := normalizeMetadata(input.Metadata)
	if err != nil {
		return MessageDTO{}, err
	}
	clientMessageID := normalizeClientMessageID(input.ClientMessageID)
	if clientMessageID != "" {
		metadata, err = withClientMessageID(metadata, clientMessageID)
		if err != nil {
			return MessageDTO{}, err
		}
	}

	sendStartedAt := time.Now().UTC()
	message, err := s.repo.Send(ctx, SendParams{
		WorkspaceID:      strings.TrimSpace(input.WorkspaceID),
		ChannelID:        strings.TrimSpace(input.ChannelID),
		SenderID:         strings.TrimSpace(input.ActorUserID),
		ParentID:         strings.TrimSpace(input.ParentID),
		ClientMessageID:  clientMessageID,
		Kind:             kind,
		Body:             body,
		Metadata:         metadata,
		MentionedUserIDs: normalizeMentions(body, input.MentionedUserIDs),
	})
	if err != nil {
		return MessageDTO{}, mapMessageError(err)
	}
	dto := toMessageDTO(message)
	isDuplicateRetry := clientMessageID != "" && message.CreatedAt.Before(sendStartedAt)
	slog.Info("Message service da luu tin nhan nguoi dung",
		"workspace_id", dto.WorkspaceID,
		"channel_id", dto.ChannelID,
		"message_id", dto.ID,
		"actor_user_id", strings.TrimSpace(input.ActorUserID),
		"kind", dto.Kind,
		"body_len", len([]rune(body)),
		"auto_responder_count", len(s.autoResponders),
	)
	if isDuplicateRetry {
		return dto, nil
	}
	s.publishRealtime(ctx, "MessageCreated", dto)
	if kind == "text" {
		s.runAutoResponders(ctx, botauto.MessageInput{
			ActorUserID: strings.TrimSpace(input.ActorUserID),
			WorkspaceID: dto.WorkspaceID,
			ChannelID:   dto.ChannelID,
			MessageID:   dto.ID,
			Body:        body,
		})
	}
	return dto, nil
}

func (s *Service) Get(ctx context.Context, input MessageRef) (MessageDTO, error) {
	message, err := s.repo.Get(ctx, cleanMessageRef(input))
	if err != nil {
		return MessageDTO{}, mapMessageError(err)
	}
	return toMessageDTO(message), nil
}

func (s *Service) List(ctx context.Context, input ListInput) ([]MessageDTO, pagination.Meta, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return nil, pagination.Meta{}, err
	}

	limit := pagination.NormalizeLimit(input.Limit)
	messages, err := s.repo.List(ctx, ListParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
		Limit:       limit + 1,
		BeforeID:    strings.TrimSpace(input.BeforeID),
	})
	if err != nil {
		return nil, pagination.Meta{}, mapMessageError(err)
	}

	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	meta := pagination.Meta{HasMore: hasMore}
	if hasMore && len(messages) > 0 {
		meta.NextCursor = messages[len(messages)-1].ID
	}
	return toMessageDTOs(messages), meta, nil
}

func (s *Service) ListThread(ctx context.Context, input ThreadInput) ([]MessageDTO, pagination.Meta, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return nil, pagination.Meta{}, err
	}

	limit := pagination.NormalizeLimit(input.Limit)
	messages, err := s.repo.ListThread(ctx, ThreadParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
		MessageID:   strings.TrimSpace(input.MessageID),
		Limit:       limit + 1,
	})
	if err != nil {
		return nil, pagination.Meta{}, mapMessageError(err)
	}

	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	return toMessageDTOs(messages), pagination.Meta{HasMore: hasMore}, nil
}

func (s *Service) Search(ctx context.Context, input SearchInput) ([]MessageDTO, pagination.Meta, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return nil, pagination.Meta{}, err
	}

	query := strings.TrimSpace(input.Query)
	if query == "" {
		return nil, pagination.Meta{}, apperrors.BadRequest("VALIDATION_ERROR", "Từ khóa tìm kiếm không được để trống.")
	}
	kind := strings.ToLower(strings.TrimSpace(input.Kind))
	if kind != "" && kind != "text" && kind != "file" && kind != "system" && kind != "bot" && kind != "event" {
		return nil, pagination.Meta{}, apperrors.BadRequest("VALIDATION_ERROR", "Loại nội dung tìm kiếm không hợp lệ.")
	}
	dateFrom, err := parseSearchDate(input.DateFrom, false)
	if err != nil {
		return nil, pagination.Meta{}, err
	}
	dateTo, err := parseSearchDate(input.DateTo, true)
	if err != nil {
		return nil, pagination.Meta{}, err
	}
	if dateFrom != nil && dateTo != nil && !dateFrom.Before(*dateTo) {
		return nil, pagination.Meta{}, apperrors.BadRequest("VALIDATION_ERROR", "Khoảng ngày tìm kiếm không hợp lệ.")
	}

	limit := pagination.NormalizeLimit(input.Limit)
	messages, err := s.repo.Search(ctx, SearchParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
		Query:       query,
		ChannelID:   strings.TrimSpace(input.ChannelID),
		SenderID:    strings.TrimSpace(input.SenderID),
		Kind:        kind,
		DateFrom:    dateFrom,
		DateTo:      dateTo,
		Limit:       limit + 1,
	})
	if err != nil {
		return nil, pagination.Meta{}, mapMessageError(err)
	}

	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	return toMessageDTOs(messages), pagination.Meta{HasMore: hasMore}, nil
}

func (s *Service) Forward(ctx context.Context, input ForwardInput) (MessageDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return MessageDTO{}, err
	}
	params := ForwardParams{
		WorkspaceID:     strings.TrimSpace(input.WorkspaceID),
		SourceChannelID: strings.TrimSpace(input.ChannelID),
		MessageID:       strings.TrimSpace(input.MessageID),
		TargetChannelID: strings.TrimSpace(input.TargetChannelID),
		ActorUserID:     strings.TrimSpace(input.ActorUserID),
	}
	if params.TargetChannelID == "" {
		return MessageDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Bạn cần chọn kênh nhận tin nhắn chuyển tiếp.")
	}
	message, err := s.repo.Forward(ctx, params)
	if err != nil {
		return MessageDTO{}, mapMessageError(err)
	}
	dto := toMessageDTO(message)
	s.publishRealtime(ctx, "MessageCreated", dto)
	return dto, nil
}

func parseSearchDate(value string, endExclusive bool) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", "Ngày tìm kiếm phải có định dạng YYYY-MM-DD.")
	}
	parsed = parsed.UTC()
	if endExclusive {
		parsed = parsed.AddDate(0, 0, 1)
	}
	return &parsed, nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (MessageDTO, error) {
	ref := cleanMessageRef(MessageRef{
		WorkspaceID: input.WorkspaceID,
		ChannelID:   input.ChannelID,
		MessageID:   input.MessageID,
		ActorUserID: input.ActorUserID,
	})
	message, err := s.repo.Get(ctx, ref)
	if err != nil {
		return MessageDTO{}, mapMessageError(err)
	}
	if !messageOwnedBy(message, ref.ActorUserID) {
		return MessageDTO{}, apperrors.Forbidden("Bạn chỉ có thể sửa tin nhắn của chính mình.")
	}

	body := strings.TrimSpace(input.Body)
	if body == "" || len([]rune(body)) > 8000 {
		return MessageDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Nội dung tin nhắn phải dài từ 1 đến 8000 ký tự.")
	}

	updated, err := s.repo.Update(ctx, UpdateParams{
		WorkspaceID:      ref.WorkspaceID,
		ChannelID:        ref.ChannelID,
		MessageID:        ref.MessageID,
		ActorUserID:      ref.ActorUserID,
		Body:             body,
		MentionedUserIDs: normalizeMentions(body, nil),
	})
	if err != nil {
		return MessageDTO{}, mapMessageError(err)
	}
	dto := toMessageDTO(updated)
	s.publishRealtime(ctx, "MessageUpdated", dto)
	return dto, nil
}

func (s *Service) Delete(ctx context.Context, input DeleteInput) error {
	ref := cleanMessageRef(MessageRef{
		WorkspaceID: input.WorkspaceID,
		ChannelID:   input.ChannelID,
		MessageID:   input.MessageID,
		ActorUserID: input.ActorUserID,
	})
	if !isUUID(ref.WorkspaceID) || !isUUID(ref.ChannelID) || !isUUID(ref.MessageID) || !isUUID(ref.ActorUserID) {
		return apperrors.BadRequest("VALIDATION_ERROR", "Ma dinh danh tin nhan khong hop le.")
	}
	message, err := s.repo.Get(ctx, ref)
	if err != nil {
		return mapMessageError(err)
	}
	if !messageOwnedBy(message, ref.ActorUserID) {
		if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.manage"); err != nil {
			return err
		}
	}

	if err := s.repo.Delete(ctx, DeleteParams{
		WorkspaceID: ref.WorkspaceID,
		ChannelID:   ref.ChannelID,
		MessageID:   ref.MessageID,
		ActorUserID: ref.ActorUserID,
	}); err != nil {
		return mapMessageError(err)
	}
	s.publishRealtime(ctx, "MessageDeleted", MessageDTO{
		ID:          ref.MessageID,
		WorkspaceID: ref.WorkspaceID,
		ChannelID:   ref.ChannelID,
	})
	return nil
}

func (s *Service) ListPins(ctx context.Context, input ListPinsInput) ([]MessageDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return nil, err
	}
	messages, err := s.repo.ListPins(ctx, ListPinsParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		return nil, mapMessageError(err)
	}
	return toMessageDTOs(messages), nil
}

func (s *Service) Pin(ctx context.Context, input PinInput) (MessageDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return MessageDTO{}, err
	}
	params := PinParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		MessageID:   strings.TrimSpace(input.MessageID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
	}
	message, err := s.repo.Pin(ctx, params)
	if err != nil {
		return MessageDTO{}, mapMessageError(err)
	}
	dto := toMessageDTO(message)
	s.publishRealtime(ctx, "MessagePinned", dto)
	return dto, nil
}

func (s *Service) Unpin(ctx context.Context, input PinInput) error {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return err
	}
	params := PinParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		MessageID:   strings.TrimSpace(input.MessageID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
	}
	if err := s.repo.Unpin(ctx, params); err != nil {
		return mapMessageError(err)
	}
	s.publishRealtime(ctx, "MessageUnpinned", MessageDTO{
		ID:          params.MessageID,
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
	})
	return nil
}

func (s *Service) AddReaction(ctx context.Context, input ReactionInput) (MessageDTO, error) {
	params, err := s.validateReaction(ctx, input)
	if err != nil {
		return MessageDTO{}, err
	}
	message, err := s.repo.AddReaction(ctx, params)
	if err != nil {
		return MessageDTO{}, mapMessageError(err)
	}
	dto := toMessageDTO(message)
	s.publishRealtime(ctx, "ReactionChanged", dto)
	return dto, nil
}

func (s *Service) RemoveReaction(ctx context.Context, input ReactionInput) (MessageDTO, error) {
	params, err := s.validateReaction(ctx, input)
	if err != nil {
		return MessageDTO{}, err
	}
	message, err := s.repo.RemoveReaction(ctx, params)
	if err != nil {
		return MessageDTO{}, mapMessageError(err)
	}
	dto := toMessageDTO(message)
	s.publishRealtime(ctx, "ReactionChanged", dto)
	return dto, nil
}

func (s *Service) validateReaction(ctx context.Context, input ReactionInput) (ReactionParams, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID, "message.send"); err != nil {
		return ReactionParams{}, err
	}
	emoji := strings.TrimSpace(input.Emoji)
	if emoji == "" || len([]rune(emoji)) > 32 {
		return ReactionParams{}, apperrors.BadRequest("VALIDATION_ERROR", "Reaction không hợp lệ.")
	}
	return ReactionParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		MessageID:   strings.TrimSpace(input.MessageID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
		Emoji:       emoji,
	}, nil
}

func (s *Service) ensurePermission(ctx context.Context, userID string, workspaceID string, permission string) error {
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), permission)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền thực hiện thao tác này.")
	}
	return nil
}

func (s *Service) publishRealtime(ctx context.Context, eventType string, message MessageDTO) {
	if s.realtime == nil {
		return
	}
	_ = s.realtime.Publish(ctx, RealtimeEvent{
		Type:        eventType,
		WorkspaceID: message.WorkspaceID,
		ChannelID:   message.ChannelID,
		MessageID:   message.ID,
		Payload: map[string]any{
			"message": message,
		},
	})
}

func (s *Service) runAutoResponders(ctx context.Context, input botauto.MessageInput) {
	if len(s.autoResponders) == 0 {
		slog.Debug("Khong co auto responder nao duoc cau hinh",
			"workspace_id", input.WorkspaceID,
			"channel_id", input.ChannelID,
			"message_id", input.MessageID,
		)
		return
	}
	for _, responder := range s.autoResponders {
		if responder == nil {
			continue
		}
		slog.Debug("Bat dau chay auto responder",
			"workspace_id", input.WorkspaceID,
			"channel_id", input.ChannelID,
			"message_id", input.MessageID,
			"body_len", len([]rune(input.Body)),
		)
		messages, err := responder.HandleMessage(ctx, input)
		if err != nil {
			slog.Warn("Auto responder xử lý tin nhắn thất bại",
				"workspace_id", input.WorkspaceID,
				"channel_id", input.ChannelID,
				"message_id", input.MessageID,
				"error", err,
			)
			continue
		}
		if len(messages) == 0 {
			slog.Debug("Auto responder khong tao phan hoi",
				"workspace_id", input.WorkspaceID,
				"channel_id", input.ChannelID,
				"message_id", input.MessageID,
			)
			continue
		}
		slog.Info("Auto responder tao phan hoi",
			"workspace_id", input.WorkspaceID,
			"channel_id", input.ChannelID,
			"message_id", input.MessageID,
			"response_count", len(messages),
		)
		for _, message := range messages {
			s.publishRealtime(ctx, "MessageCreated", autoBotMessageDTO(message))
		}
	}
}

func autoBotMessageDTO(message botauto.BotMessage) MessageDTO {
	metadata := message.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	createdAt := message.CreatedAt
	if strings.TrimSpace(createdAt) == "" {
		createdAt = time.Now().UTC().Format(time.RFC3339)
	}
	kind := strings.TrimSpace(message.Kind)
	if kind == "" {
		kind = "bot"
	}
	return MessageDTO{
		ID:          message.ID,
		WorkspaceID: message.WorkspaceID,
		ChannelID:   message.ChannelID,
		Kind:        kind,
		Body:        message.Body,
		Metadata:    metadata,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
		Mentions:    []string{},
		Reactions:   []ReactionSummaryDTO{},
	}
}

func normalizeMetadata(value json.RawMessage) ([]byte, error) {
	if len(value) == 0 || strings.TrimSpace(string(value)) == "" || strings.TrimSpace(string(value)) == "null" {
		return []byte(`{}`), nil
	}
	if !json.Valid(value) {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", "Metadata của tin nhắn không phải JSON hợp lệ.")
	}
	return []byte(value), nil
}

func normalizeClientMessageID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 128 {
		return value[:128]
	}
	return value
}

func withClientMessageID(metadata []byte, clientMessageID string) ([]byte, error) {
	var payload map[string]any
	if len(metadata) == 0 {
		payload = map[string]any{}
	} else if err := json.Unmarshal(metadata, &payload); err != nil {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", "Metadata cua tin nhan khong phai JSON object hop le.")
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["client_message_id"] = clientMessageID
	return json.Marshal(payload)
}

func normalizeMentions(body string, ids []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(ids))
	add := func(id string) {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		result = append(result, id)
	}

	for _, id := range ids {
		add(id)
	}
	for _, match := range mentionPattern.FindAllStringSubmatch(body, -1) {
		if len(match) == 2 {
			add(match[1])
		}
	}

	sort.Strings(result)
	return result
}

func cleanMessageRef(input MessageRef) MessageRef {
	return MessageRef{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   strings.TrimSpace(input.ChannelID),
		MessageID:   strings.TrimSpace(input.MessageID),
		ActorUserID: strings.TrimSpace(input.ActorUserID),
	}
}

func messageOwnedBy(message messagesdomain.Message, userID string) bool {
	return message.SenderID != nil && *message.SenderID == strings.TrimSpace(userID)
}

func isUUID(value string) bool {
	return uuidPattern.MatchString(strings.TrimSpace(value))
}

func mapMessageError(err error) error {
	if errors.Is(err, messagesdomain.ErrMessageNotFound) {
		return apperrors.NotFound("MESSAGE_NOT_FOUND", "Không tìm thấy tin nhắn.")
	}
	if errors.Is(err, messagesdomain.ErrChannelNotFound) {
		return apperrors.NotFound("CHANNEL_NOT_FOUND", "Không tìm thấy kênh hoặc bạn chưa thuộc kênh.")
	}
	if errors.Is(err, messagesdomain.ErrMentionNotFound) {
		return apperrors.BadRequest("INVALID_MENTION", "Một hoặc nhiều user được nhắc chưa thuộc kênh.")
	}
	if errors.Is(err, messagesdomain.ErrPinNotFound) {
		return apperrors.NotFound("MESSAGE_PIN_NOT_FOUND", "Không tìm thấy tin ghim.")
	}
	if errors.Is(err, messagesdomain.ErrReactionNotFound) {
		return apperrors.NotFound("REACTION_NOT_FOUND", "Không tìm thấy reaction.")
	}
	return err
}

func toMessageDTOs(messages []messagesdomain.Message) []MessageDTO {
	dtos := make([]MessageDTO, 0, len(messages))
	for _, message := range messages {
		dtos = append(dtos, toMessageDTO(message))
	}
	return dtos
}

func toMessageDTO(message messagesdomain.Message) MessageDTO {
	metadata := json.RawMessage(message.Metadata)
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	return MessageDTO{
		ID:           message.ID,
		WorkspaceID:  message.WorkspaceID,
		ChannelID:    message.ChannelID,
		SenderID:     message.SenderID,
		ParentID:     message.ParentID,
		ThreadRootID: message.ThreadRootID,
		Kind:         message.Kind,
		Body:         message.Body,
		Metadata:     metadata,
		EditedAt:     formatOptionalTime(message.EditedAt),
		DeletedAt:    formatOptionalTime(message.DeletedAt),
		CreatedAt:    formatTime(message.CreatedAt),
		UpdatedAt:    formatTime(message.UpdatedAt),
		Mentions:     message.Mentions,
		Reactions:    toReactionDTOs(message.Reactions),
	}
}

func toReactionDTOs(reactions []messagesdomain.ReactionSummary) []ReactionSummaryDTO {
	dtos := make([]ReactionSummaryDTO, 0, len(reactions))
	for _, reaction := range reactions {
		dtos = append(dtos, ReactionSummaryDTO{
			Emoji:       reaction.Emoji,
			Count:       reaction.Count,
			ReactedByMe: reaction.ReactedByMe,
		})
	}
	return dtos
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
