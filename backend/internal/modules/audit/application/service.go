package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	auditdomain "github.com/duclamdev/application-chat/backend/internal/modules/audit/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

const PermissionViewAudit = "audit.view"

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	List(ctx context.Context, params ListParams) ([]auditdomain.Log, error)
}

type Service struct {
	repo    Repository
	checker PermissionChecker
}

type ListInput struct {
	ActorUserID   string
	WorkspaceID   string
	FilterActorID string
	Action        string
	EntityType    string
	EntityID      string
	From          string
	To            string
	Limit         int
}

type ListParams struct {
	WorkspaceID   string
	FilterActorID string
	Action        string
	EntityType    string
	EntityID      string
	From          *time.Time
	To            *time.Time
	Limit         int
}

type LogDTO struct {
	ID          string          `json:"id"`
	WorkspaceID *string         `json:"workspace_id,omitempty"`
	ActorUserID *string         `json:"actor_user_id,omitempty"`
	Action      string          `json:"action"`
	EntityType  string          `json:"entity_type"`
	EntityID    *string         `json:"entity_id,omitempty"`
	IPAddress   *string         `json:"ip_address,omitempty"`
	UserAgent   *string         `json:"user_agent,omitempty"`
	BeforeData  json.RawMessage `json:"before_data,omitempty"`
	AfterData   json.RawMessage `json:"after_data,omitempty"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   string          `json:"created_at"`
}

func NewService(repo Repository, checker PermissionChecker) *Service {
	return &Service{repo: repo, checker: checker}
}

func (s *Service) List(ctx context.Context, input ListInput) ([]LogDTO, error) {
	if err := s.ensurePermission(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return nil, err
	}
	from, err := parseOptionalTime(input.From, "from")
	if err != nil {
		return nil, err
	}
	to, err := parseOptionalTime(input.To, "to")
	if err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	logs, err := s.repo.List(ctx, ListParams{
		WorkspaceID:   strings.TrimSpace(input.WorkspaceID),
		FilterActorID: strings.TrimSpace(input.FilterActorID),
		Action:        strings.TrimSpace(input.Action),
		EntityType:    strings.TrimSpace(input.EntityType),
		EntityID:      strings.TrimSpace(input.EntityID),
		From:          from,
		To:            to,
		Limit:         limit,
	})
	if err != nil {
		return nil, err
	}
	return toLogDTOs(logs), nil
}

func (s *Service) ensurePermission(ctx context.Context, userID string, workspaceID string) error {
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), PermissionViewAudit)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền xem audit log.")
	}
	return nil
}

func parseOptionalTime(value string, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", "Tham số "+field+" phải theo định dạng RFC3339.")
	}
	return &parsed, nil
}

func toLogDTOs(logs []auditdomain.Log) []LogDTO {
	dtos := make([]LogDTO, 0, len(logs))
	for _, log := range logs {
		dtos = append(dtos, toLogDTO(log))
	}
	return dtos
}

func toLogDTO(log auditdomain.Log) LogDTO {
	metadata := json.RawMessage(log.Metadata)
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	return LogDTO{
		ID:          log.ID,
		WorkspaceID: log.WorkspaceID,
		ActorUserID: log.ActorUserID,
		Action:      log.Action,
		EntityType:  log.EntityType,
		EntityID:    log.EntityID,
		IPAddress:   log.IPAddress,
		UserAgent:   log.UserAgent,
		BeforeData:  optionalJSON(log.BeforeData),
		AfterData:   optionalJSON(log.AfterData),
		Metadata:    metadata,
		CreatedAt:   log.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func optionalJSON(value []byte) json.RawMessage {
	if len(value) == 0 || strings.TrimSpace(string(value)) == "" || strings.TrimSpace(string(value)) == "null" {
		return nil
	}
	return json.RawMessage(value)
}
