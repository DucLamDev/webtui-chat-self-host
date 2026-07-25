package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	presencedomain "github.com/duclamdev/application-chat/backend/internal/modules/presence/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
	"github.com/duclamdev/application-chat/backend/internal/shared/pagination"
)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	Upsert(ctx context.Context, params UpsertParams) (presencedomain.Presence, error)
	List(ctx context.Context, workspaceID string, limit int) ([]presencedomain.Presence, error)
	MarkOfflineStale(ctx context.Context, staleAfter time.Duration) (int, error)
}

type Service struct {
	repo    Repository
	checker PermissionChecker
}

type UpsertInput struct {
	ActorUserID string
	WorkspaceID string
	DeviceID    string
	SocketID    string
	NodeID      string
	Status      string
	Metadata    json.RawMessage
}

type UpsertParams struct {
	UserID      string
	WorkspaceID string
	DeviceID    string
	SocketID    string
	NodeID      string
	Status      string
	Metadata    []byte
}

type ListInput struct {
	ActorUserID string
	WorkspaceID string
	Limit       int
}

type PresenceDTO struct {
	UserID          string          `json:"user_id"`
	WorkspaceID     *string         `json:"workspace_id,omitempty"`
	DeviceID        string          `json:"device_id"`
	SocketID        string          `json:"socket_id"`
	NodeID          string          `json:"node_id"`
	Status          string          `json:"status"`
	LastHeartbeatAt string          `json:"last_heartbeat_at"`
	ConnectedAt     string          `json:"connected_at"`
	Metadata        json.RawMessage `json:"metadata"`
}

func NewService(repo Repository, checker PermissionChecker) *Service {
	return &Service{repo: repo, checker: checker}
}

func (s *Service) Heartbeat(ctx context.Context, input UpsertInput) (PresenceDTO, error) {
	if err := s.ensureWorkspaceMember(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return PresenceDTO{}, err
	}
	deviceID := strings.TrimSpace(input.DeviceID)
	socketID := strings.TrimSpace(input.SocketID)
	nodeID := strings.TrimSpace(input.NodeID)
	if deviceID == "" {
		return PresenceDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Device ID không được để trống.")
	}
	if socketID == "" {
		socketID = deviceID
	}
	if nodeID == "" {
		nodeID = "api-local"
	}
	status := normalizeStatus(input.Status)
	metadata, err := normalizeMetadata(input.Metadata)
	if err != nil {
		return PresenceDTO{}, err
	}
	presence, err := s.repo.Upsert(ctx, UpsertParams{
		UserID:      strings.TrimSpace(input.ActorUserID),
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		DeviceID:    deviceID,
		SocketID:    socketID,
		NodeID:      nodeID,
		Status:      status,
		Metadata:    metadata,
	})
	if err != nil {
		return PresenceDTO{}, err
	}
	return toDTO(presence), nil
}

func (s *Service) List(ctx context.Context, input ListInput) ([]PresenceDTO, pagination.Meta, error) {
	if err := s.ensureWorkspaceMember(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return nil, pagination.Meta{}, err
	}
	limit := pagination.NormalizeLimit(input.Limit)
	presences, err := s.repo.List(ctx, strings.TrimSpace(input.WorkspaceID), limit+1)
	if err != nil {
		return nil, pagination.Meta{}, err
	}
	hasMore := len(presences) > limit
	if hasMore {
		presences = presences[:limit]
	}
	return toDTOs(presences), pagination.Meta{HasMore: hasMore}, nil
}

func (s *Service) CleanupStale(ctx context.Context, staleAfter time.Duration) (int, error) {
	if staleAfter <= 0 {
		staleAfter = 90 * time.Second
	}
	return s.repo.MarkOfflineStale(ctx, staleAfter)
}

func (s *Service) ensureWorkspaceMember(ctx context.Context, userID string, workspaceID string) error {
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), "workspace.view_members")
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không thuộc workspace này.")
	}
	return nil
}

func normalizeStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "away":
		return "away"
	case "offline":
		return "offline"
	default:
		return "online"
	}
}

func normalizeMetadata(value json.RawMessage) ([]byte, error) {
	if len(value) == 0 || strings.TrimSpace(string(value)) == "" || strings.TrimSpace(string(value)) == "null" {
		return []byte(`{}`), nil
	}
	if !json.Valid(value) {
		return nil, apperrors.BadRequest("VALIDATION_ERROR", "Metadata presence không phải JSON hợp lệ.")
	}
	return []byte(value), nil
}

func toDTOs(presences []presencedomain.Presence) []PresenceDTO {
	dtos := make([]PresenceDTO, 0, len(presences))
	for _, presence := range presences {
		dtos = append(dtos, toDTO(presence))
	}
	return dtos
}

func toDTO(presence presencedomain.Presence) PresenceDTO {
	metadata := json.RawMessage(presence.Metadata)
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	return PresenceDTO{
		UserID:          presence.UserID,
		WorkspaceID:     presence.WorkspaceID,
		DeviceID:        presence.DeviceID,
		SocketID:        presence.SocketID,
		NodeID:          presence.NodeID,
		Status:          presence.Status,
		LastHeartbeatAt: formatTime(presence.LastHeartbeatAt),
		ConnectedAt:     formatTime(presence.ConnectedAt),
		Metadata:        metadata,
	}
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}
