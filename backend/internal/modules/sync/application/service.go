package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	syncdomain "github.com/duclamdev/application-chat/backend/internal/modules/sync/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	ListEvents(ctx context.Context, params ListParams) ([]syncdomain.Event, error)
	Ack(ctx context.Context, params AckParams) (syncdomain.CursorAck, error)
	GetAckCursor(ctx context.Context, userID string, workspaceID string, deviceID string) (string, error)
}

type Service struct {
	repo    Repository
	checker PermissionChecker
}

type ListInput struct {
	ActorUserID string
	WorkspaceID string
	DeviceID    string
	Cursor      string
	Limit       int
}

type ListParams struct {
	WorkspaceID string
	Cursor      string
	Limit       int
}

type AckInput struct {
	ActorUserID string
	WorkspaceID string
	DeviceID    string
	Cursor      string
}

type AckParams struct {
	UserID      string
	WorkspaceID string
	DeviceID    string
	Cursor      string
}

type EventDTO struct {
	EventID       string          `json:"event_id"`
	WorkspaceID   string          `json:"workspace_id"`
	Type          string          `json:"type"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventVersion  int             `json:"event_version"`
	OccurredAt    string          `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload"`
}

type CatchUpDTO struct {
	Events     []EventDTO `json:"events"`
	NextCursor string     `json:"next_cursor,omitempty"`
	HasMore    bool       `json:"has_more"`
	ServerTime string     `json:"server_time"`
}

type AckDTO struct {
	UserID          string  `json:"user_id"`
	WorkspaceID     string  `json:"workspace_id"`
	DeviceID        string  `json:"device_id"`
	CursorEventID   *string `json:"cursor_event_id,omitempty"`
	CursorCreatedAt *string `json:"cursor_created_at,omitempty"`
	AckedAt         string  `json:"acked_at"`
}

func NewService(repo Repository, checker PermissionChecker) *Service {
	return &Service{repo: repo, checker: checker}
}

func (s *Service) CatchUp(ctx context.Context, input ListInput) (CatchUpDTO, error) {
	userID := strings.TrimSpace(input.ActorUserID)
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if err := s.ensureWorkspaceMember(ctx, userID, workspaceID); err != nil {
		return CatchUpDTO{}, err
	}
	limit := input.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	cursor := strings.TrimSpace(input.Cursor)
	if cursor == "" && strings.TrimSpace(input.DeviceID) != "" {
		stored, err := s.repo.GetAckCursor(ctx, userID, workspaceID, strings.TrimSpace(input.DeviceID))
		if err != nil {
			return CatchUpDTO{}, err
		}
		cursor = stored
	}
	events, err := s.repo.ListEvents(ctx, ListParams{
		WorkspaceID: workspaceID,
		Cursor:      cursor,
		Limit:       limit + 1,
	})
	if err != nil {
		return CatchUpDTO{}, err
	}
	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}
	nextCursor := ""
	if len(events) > 0 {
		nextCursor = events[len(events)-1].ID
	}
	return CatchUpDTO{
		Events:     toEventDTOs(workspaceID, events),
		NextCursor: nextCursor,
		HasMore:    hasMore,
		ServerTime: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *Service) Ack(ctx context.Context, input AckInput) (AckDTO, error) {
	userID := strings.TrimSpace(input.ActorUserID)
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if err := s.ensureWorkspaceMember(ctx, userID, workspaceID); err != nil {
		return AckDTO{}, err
	}
	deviceID := strings.TrimSpace(input.DeviceID)
	cursor := strings.TrimSpace(input.Cursor)
	if deviceID == "" || cursor == "" {
		return AckDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "device_id và cursor là bắt buộc.")
	}
	ack, err := s.repo.Ack(ctx, AckParams{
		UserID:      userID,
		WorkspaceID: workspaceID,
		DeviceID:    deviceID,
		Cursor:      cursor,
	})
	if err != nil {
		return AckDTO{}, err
	}
	return toAckDTO(ack), nil
}

func (s *Service) ensureWorkspaceMember(ctx context.Context, userID string, workspaceID string) error {
	if workspaceID == "" {
		return apperrors.BadRequest("WORKSPACE_REQUIRED", "workspace_id là bắt buộc.")
	}
	if s.checker == nil {
		return nil
	}
	allowed, err := s.checker.HasWorkspacePermission(ctx, userID, workspaceID, "workspace.view_members")
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không thuộc workspace này.")
	}
	return nil
}

func toEventDTOs(workspaceID string, events []syncdomain.Event) []EventDTO {
	dtos := make([]EventDTO, 0, len(events))
	for _, event := range events {
		payload := event.Payload
		if len(payload) == 0 || !json.Valid(payload) {
			payload = json.RawMessage(`{}`)
		}
		dtos = append(dtos, EventDTO{
			EventID:       event.ID,
			WorkspaceID:   workspaceID,
			Type:          event.EventType,
			AggregateType: event.AggregateType,
			AggregateID:   event.AggregateID,
			EventVersion:  event.EventVersion,
			OccurredAt:    event.CreatedAt.UTC().Format(time.RFC3339),
			Payload:       payload,
		})
	}
	return dtos
}

func toAckDTO(ack syncdomain.CursorAck) AckDTO {
	return AckDTO{
		UserID:          ack.UserID,
		WorkspaceID:     ack.WorkspaceID,
		DeviceID:        ack.DeviceID,
		CursorEventID:   ack.CursorEventID,
		CursorCreatedAt: formatOptionalTime(ack.CursorCreatedAt),
		AckedAt:         ack.AckedAt.UTC().Format(time.RFC3339),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
