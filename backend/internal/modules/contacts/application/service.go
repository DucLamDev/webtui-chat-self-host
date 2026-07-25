package application

import (
	"context"
	"errors"
	"strings"
	"time"

	contactsdomain "github.com/duclamdev/application-chat/backend/internal/modules/contacts/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type Repository interface {
	AcceptRequest(ctx context.Context, zoneID string, actorUserID string, requestID string) (contactsdomain.ContactRequest, error)
	CancelRequest(ctx context.Context, zoneID string, actorUserID string, requestID string) (contactsdomain.ContactRequest, error)
	CreateRequest(ctx context.Context, zoneID string, actorUserID string, receiverID string) (contactsdomain.ContactRequest, error)
	ListContacts(ctx context.Context, zoneID string, actorUserID string) ([]contactsdomain.ContactRequest, error)
	ListRequests(ctx context.Context, zoneID string, actorUserID string, status string) ([]contactsdomain.ContactRequest, error)
	RejectRequest(ctx context.Context, zoneID string, actorUserID string, requestID string) (contactsdomain.ContactRequest, error)
}

type RealtimePublisher interface {
	Publish(ctx context.Context, event RealtimeEvent) error
}

type RealtimeEvent struct {
	Type    string
	ZoneID  string
	UserID  string
	Payload map[string]any
}

type Service struct {
	realtime RealtimePublisher
	repo     Repository
}

type ContactRequestDTO struct {
	ID          string  `json:"id"`
	Direction   string  `json:"direction"`
	RequesterID string  `json:"requester_id"`
	ReceiverID  string  `json:"receiver_id"`
	Status      string  `json:"status"`
	User        UserDTO `json:"user"`
	RequestedAt string  `json:"requested_at"`
	RespondedAt *string `json:"responded_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type UserDTO struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	Status      string  `json:"status"`
}

func NewService(repo Repository, realtime ...RealtimePublisher) *Service {
	service := &Service{repo: repo}
	if len(realtime) > 0 {
		service.realtime = realtime[0]
	}
	return service
}

func (s *Service) ListContacts(ctx context.Context, zoneID string, actorUserID string) ([]ContactRequestDTO, error) {
	items, err := s.repo.ListContacts(ctx, strings.TrimSpace(zoneID), strings.TrimSpace(actorUserID))
	if err != nil {
		return nil, mapContactError(err)
	}
	return toDTOs(actorUserID, items), nil
}

func (s *Service) ListRequests(ctx context.Context, zoneID string, actorUserID string, status string) ([]ContactRequestDTO, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		status = "pending"
	}
	items, err := s.repo.ListRequests(ctx, strings.TrimSpace(zoneID), strings.TrimSpace(actorUserID), status)
	if err != nil {
		return nil, mapContactError(err)
	}
	return toDTOs(actorUserID, items), nil
}

func (s *Service) SendRequest(ctx context.Context, zoneID string, actorUserID string, receiverID string) (ContactRequestDTO, error) {
	zoneID = strings.TrimSpace(zoneID)
	item, err := s.repo.CreateRequest(ctx, zoneID, strings.TrimSpace(actorUserID), strings.TrimSpace(receiverID))
	if err != nil {
		return ContactRequestDTO{}, mapContactError(err)
	}
	dto := toDTO(actorUserID, item)
	s.publishContactRealtime(ctx, zoneID, "ContactRequestCreated", item.ReceiverID, dto)
	return dto, nil
}

func (s *Service) AcceptRequest(ctx context.Context, zoneID string, actorUserID string, requestID string) (ContactRequestDTO, error) {
	zoneID = strings.TrimSpace(zoneID)
	item, err := s.repo.AcceptRequest(ctx, zoneID, strings.TrimSpace(actorUserID), strings.TrimSpace(requestID))
	if err != nil {
		return ContactRequestDTO{}, mapContactError(err)
	}
	dto := toDTO(actorUserID, item)
	s.publishContactRealtime(ctx, zoneID, "ContactRequestUpdated", item.RequesterID, dto)
	s.publishContactRealtime(ctx, zoneID, "ContactRequestUpdated", item.ReceiverID, dto)
	return dto, nil
}

func (s *Service) RejectRequest(ctx context.Context, zoneID string, actorUserID string, requestID string) (ContactRequestDTO, error) {
	zoneID = strings.TrimSpace(zoneID)
	item, err := s.repo.RejectRequest(ctx, zoneID, strings.TrimSpace(actorUserID), strings.TrimSpace(requestID))
	if err != nil {
		return ContactRequestDTO{}, mapContactError(err)
	}
	dto := toDTO(actorUserID, item)
	s.publishContactRealtime(ctx, zoneID, "ContactRequestUpdated", item.RequesterID, dto)
	s.publishContactRealtime(ctx, zoneID, "ContactRequestUpdated", item.ReceiverID, dto)
	return dto, nil
}

func (s *Service) CancelRequest(ctx context.Context, zoneID string, actorUserID string, requestID string) error {
	zoneID = strings.TrimSpace(zoneID)
	item, err := s.repo.CancelRequest(ctx, zoneID, strings.TrimSpace(actorUserID), strings.TrimSpace(requestID))
	if err != nil {
		return mapContactError(err)
	}
	dto := toDTO(actorUserID, item)
	s.publishContactRealtime(ctx, zoneID, "ContactRequestCancelled", item.RequesterID, dto)
	s.publishContactRealtime(ctx, zoneID, "ContactRequestCancelled", item.ReceiverID, dto)
	return nil
}

func (s *Service) publishContactRealtime(ctx context.Context, zoneID string, eventType string, userID string, request ContactRequestDTO) {
	if s.realtime == nil || strings.TrimSpace(userID) == "" {
		return
	}
	_ = s.realtime.Publish(ctx, RealtimeEvent{
		Type:   eventType,
		ZoneID: strings.TrimSpace(zoneID),
		UserID: strings.TrimSpace(userID),
		Payload: map[string]any{
			"contact_request": request,
		},
	})
}

func mapContactError(err error) error {
	if errors.Is(err, contactsdomain.ErrCannotContactSelf) {
		return apperrors.BadRequest("CANNOT_CONTACT_SELF", "Bạn không thể gửi lời mời kết bạn cho chính mình.")
	}
	if errors.Is(err, contactsdomain.ErrUserNotFound) {
		return apperrors.NotFound("USER_NOT_FOUND", "Không tìm thấy người dùng.")
	}
	if errors.Is(err, contactsdomain.ErrContactRequestConflict) {
		return apperrors.Conflict("CONTACT_REQUEST_EXISTS", "Hai tài khoản đã có lời mời hoặc đã là bạn bè.")
	}
	if errors.Is(err, contactsdomain.ErrContactRequestNotFound) {
		return apperrors.NotFound("CONTACT_REQUEST_NOT_FOUND", "Không tìm thấy lời mời kết bạn.")
	}
	return err
}

func toDTOs(actorUserID string, items []contactsdomain.ContactRequest) []ContactRequestDTO {
	dtos := make([]ContactRequestDTO, 0, len(items))
	for _, item := range items {
		dtos = append(dtos, toDTO(actorUserID, item))
	}
	return dtos
}

func toDTO(actorUserID string, item contactsdomain.ContactRequest) ContactRequestDTO {
	direction := "outgoing"
	if item.ReceiverID == strings.TrimSpace(actorUserID) {
		direction = "incoming"
	}
	return ContactRequestDTO{
		ID:          item.ID,
		Direction:   direction,
		RequesterID: item.RequesterID,
		ReceiverID:  item.ReceiverID,
		Status:      item.Status,
		User:        toUserDTO(item.User),
		RequestedAt: formatTime(item.RequestedAt),
		RespondedAt: formatOptionalTime(item.RespondedAt),
		CreatedAt:   formatTime(item.CreatedAt),
		UpdatedAt:   formatTime(item.UpdatedAt),
	}
}

func toUserDTO(user contactsdomain.UserSummary) UserDTO {
	return UserDTO{
		ID:          user.ID,
		Email:       user.Email,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		PhoneNumber: user.PhoneNumber,
		Status:      user.Status,
	}
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
