package application

import (
	"context"
	"errors"
	"strings"
	"time"

	ticketsdomain "github.com/duclamdev/application-chat/backend/internal/modules/tickets/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const (
	PermissionTicketManage = "ticket.manage"
	PermissionTicketView   = "ticket.view"
)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	Create(ctx context.Context, params SaveParams) (ticketsdomain.Ticket, error)
	Get(ctx context.Context, workspaceID string, ticketID string) (ticketsdomain.Ticket, error)
	List(ctx context.Context, params ListParams) ([]ticketsdomain.Ticket, error)
	Update(ctx context.Context, params SaveParams) (ticketsdomain.Ticket, error)
}

type Service struct {
	repo    Repository
	checker PermissionChecker
}

type ListInput struct {
	ActorUserID string
	WorkspaceID string
	Status      string
	Limit       int
}

type ListParams struct {
	WorkspaceID string
	Status      string
	Limit       int
}

type CreateInput struct {
	ActorUserID string
	WorkspaceID string
	ChannelID   string
	Title       string
	Description string
	Priority    string
	AssignedTo  string
}

type UpdateInput struct {
	ActorUserID string
	WorkspaceID string
	TicketID    string
	ChannelID   *string
	Title       *string
	Description *string
	Status      *string
	Priority    *string
	AssignedTo  *string
}

type SaveParams struct {
	TicketID    string
	WorkspaceID string
	ChannelID   *string
	Title       *string
	Description *string
	Status      *string
	Priority    *string
	CreatedBy   string
	AssignedTo  *string
}

type TicketDTO struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	ChannelID   *string `json:"channel_id,omitempty"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Priority    string  `json:"priority"`
	CreatedBy   *string `json:"created_by,omitempty"`
	AssignedTo  *string `json:"assigned_to,omitempty"`
	ResolvedAt  *string `json:"resolved_at,omitempty"`
	ClosedAt    *string `json:"closed_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func NewService(repo Repository, checker PermissionChecker) *Service {
	return &Service{repo: repo, checker: checker}
}

func (s *Service) List(ctx context.Context, input ListInput) ([]TicketDTO, error) {
	if err := s.ensureView(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return nil, err
	}
	status := strings.TrimSpace(input.Status)
	if status != "" && !isTicketStatus(status) {
		return nil, apperrors.BadRequest("INVALID_TICKET_STATUS", "Ticket status is invalid.")
	}
	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	tickets, err := s.repo.List(ctx, ListParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		Status:      status,
		Limit:       limit,
	})
	if err != nil {
		return nil, err
	}
	return toDTOs(tickets), nil
}

func (s *Service) Get(ctx context.Context, actorUserID string, workspaceID string, ticketID string) (TicketDTO, error) {
	if err := s.ensureView(ctx, actorUserID, workspaceID); err != nil {
		return TicketDTO{}, err
	}
	ticket, err := s.repo.Get(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(ticketID))
	if err != nil {
		return TicketDTO{}, mapTicketError(err)
	}
	return toDTO(ticket), nil
}

func (s *Service) Create(ctx context.Context, input CreateInput) (TicketDTO, error) {
	if err := s.ensureView(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return TicketDTO{}, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return TicketDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Ticket title is required.")
	}
	priority := strings.TrimSpace(input.Priority)
	if priority == "" {
		priority = "normal"
	}
	if !isTicketPriority(priority) {
		return TicketDTO{}, apperrors.BadRequest("INVALID_TICKET_PRIORITY", "Ticket priority is invalid.")
	}
	channelID := cleanOptionalString(input.ChannelID)
	assignedTo := cleanOptionalString(input.AssignedTo)
	ticket, err := s.repo.Create(ctx, SaveParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   channelID,
		Title:       &title,
		Description: ptr(strings.TrimSpace(input.Description)),
		Status:      ptr("open"),
		Priority:    &priority,
		CreatedBy:   strings.TrimSpace(input.ActorUserID),
		AssignedTo:  assignedTo,
	})
	if err != nil {
		return TicketDTO{}, err
	}
	return toDTO(ticket), nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (TicketDTO, error) {
	if err := s.ensureManage(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return TicketDTO{}, err
	}
	title := cleanOptional(input.Title)
	if title != nil && *title == "" {
		return TicketDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Ticket title is required.")
	}
	description := cleanOptional(input.Description)
	status := cleanOptional(input.Status)
	if status != nil && !isTicketStatus(*status) {
		return TicketDTO{}, apperrors.BadRequest("INVALID_TICKET_STATUS", "Ticket status is invalid.")
	}
	priority := cleanOptional(input.Priority)
	if priority != nil && !isTicketPriority(*priority) {
		return TicketDTO{}, apperrors.BadRequest("INVALID_TICKET_PRIORITY", "Ticket priority is invalid.")
	}
	ticket, err := s.repo.Update(ctx, SaveParams{
		TicketID:    strings.TrimSpace(input.TicketID),
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ChannelID:   cleanOptional(input.ChannelID),
		Title:       title,
		Description: description,
		Status:      status,
		Priority:    priority,
		AssignedTo:  cleanOptional(input.AssignedTo),
	})
	if err != nil {
		return TicketDTO{}, mapTicketError(err)
	}
	return toDTO(ticket), nil
}

func (s *Service) ensureView(ctx context.Context, userID string, workspaceID string) error {
	return s.ensurePermission(ctx, userID, workspaceID, PermissionTicketView)
}

func (s *Service) ensureManage(ctx context.Context, userID string, workspaceID string) error {
	return s.ensurePermission(ctx, userID, workspaceID, PermissionTicketManage)
}

func (s *Service) ensurePermission(ctx context.Context, userID string, workspaceID string, permission string) error {
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), permission)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("You do not have permission to access tickets.")
	}
	return nil
}

func mapTicketError(err error) error {
	if errors.Is(err, ticketsdomain.ErrTicketNotFound) {
		return apperrors.NotFound("TICKET_NOT_FOUND", "Ticket not found.")
	}
	return err
}

func isTicketStatus(status string) bool {
	switch status {
	case "open", "pending", "resolved", "closed":
		return true
	default:
		return false
	}
}

func isTicketPriority(priority string) bool {
	switch priority {
	case "low", "normal", "high", "urgent":
		return true
	default:
		return false
	}
}

func toDTOs(tickets []ticketsdomain.Ticket) []TicketDTO {
	dtos := make([]TicketDTO, 0, len(tickets))
	for _, ticket := range tickets {
		dtos = append(dtos, toDTO(ticket))
	}
	return dtos
}

func toDTO(ticket ticketsdomain.Ticket) TicketDTO {
	return TicketDTO{
		ID:          ticket.ID,
		WorkspaceID: ticket.WorkspaceID,
		ChannelID:   ticket.ChannelID,
		Title:       ticket.Title,
		Description: ticket.Description,
		Status:      ticket.Status,
		Priority:    ticket.Priority,
		CreatedBy:   ticket.CreatedBy,
		AssignedTo:  ticket.AssignedTo,
		ResolvedAt:  formatOptionalTime(ticket.ResolvedAt),
		ClosedAt:    formatOptionalTime(ticket.ClosedAt),
		CreatedAt:   formatTime(ticket.CreatedAt),
		UpdatedAt:   formatTime(ticket.UpdatedAt),
	}
}

func cleanOptional(value *string) *string {
	if value == nil {
		return nil
	}
	clean := strings.TrimSpace(*value)
	return &clean
}

func cleanOptionalString(value string) *string {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return nil
	}
	return &clean
}

func ptr(value string) *string {
	return &value
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
