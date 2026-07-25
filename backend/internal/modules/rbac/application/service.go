package application

import (
	"context"
	"errors"
	"strings"

	rbacdomain "github.com/duclamdev/application-chat/backend/internal/modules/rbac/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type Repository interface {
	ListPermissions(ctx context.Context) ([]rbacdomain.Permission, error)
	ListRoles(ctx context.Context, workspaceID string) ([]rbacdomain.Role, error)
	CreateRole(ctx context.Context, params CreateRoleParams) (rbacdomain.Role, error)
	ListWorkspaceMemberRoles(ctx context.Context, workspaceID string, userID string) ([]rbacdomain.Role, error)
	AssignWorkspaceRole(ctx context.Context, params AssignWorkspaceRoleParams) error
	RevokeWorkspaceRole(ctx context.Context, params RevokeWorkspaceRoleParams) error
	ListUserWorkspacePermissions(ctx context.Context, userID string, workspaceID string) ([]rbacdomain.Permission, error)
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
	HasAnyWorkspacePermission(ctx context.Context, userID string, permissionCode string) (bool, error)
	HasAnyZonePermission(ctx context.Context, userID string, zoneID string, permissionCode string) (bool, error)
	WorkspaceBelongsToZone(ctx context.Context, workspaceID string, zoneID string) (bool, error)
	RecordAudit(ctx context.Context, event AuditEvent) error
}

type Service struct {
	repo Repository
}

type PermissionDTO struct {
	ID          string  `json:"id"`
	Code        string  `json:"code"`
	Module      string  `json:"module"`
	Action      string  `json:"action"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type RoleDTO struct {
	ID          string          `json:"id"`
	WorkspaceID *string         `json:"workspace_id,omitempty"`
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	IsSystem    bool            `json:"is_system"`
	CreatedBy   *string         `json:"created_by,omitempty"`
	Permissions []PermissionDTO `json:"permissions,omitempty"`
}

type CreateRoleInput struct {
	ActorUserID     string
	ZoneID          string
	WorkspaceID     string
	Code            string
	Name            string
	Description     string
	PermissionCodes []string
}

type CreateRoleParams struct {
	WorkspaceID     string
	Code            string
	Name            string
	Description     string
	PermissionCodes []string
	CreatedBy       string
}

type AssignWorkspaceRoleInput struct {
	ActorUserID string
	WorkspaceID string
	UserID      string
	RoleID      string
}

type AssignWorkspaceRoleParams struct {
	WorkspaceID string
	UserID      string
	RoleID      string
	AssignedBy  string
}

type RevokeWorkspaceRoleParams struct {
	WorkspaceID string
	UserID      string
	RoleID      string
}

type AuditEvent struct {
	ActorUserID string
	WorkspaceID string
	Action      string
	EntityType  string
	EntityID    string
	Metadata    map[string]any
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListPermissions(ctx context.Context) ([]PermissionDTO, error) {
	permissions, err := s.repo.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	return toDTOs(permissions), nil
}

func (s *Service) ListRoles(ctx context.Context, workspaceID string) ([]RoleDTO, error) {
	roles, err := s.repo.ListRoles(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	return toRoleDTOs(roles), nil
}

func (s *Service) CreateRole(ctx context.Context, input CreateRoleInput) (RoleDTO, error) {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	if input.WorkspaceID == "" {
		return RoleDTO{}, apperrors.BadRequest("WORKSPACE_REQUIRED", "Thiếu workspace_id để tạo role.")
	}
	matchesZone, err := s.repo.WorkspaceBelongsToZone(ctx, input.WorkspaceID, input.ZoneID)
	if err != nil {
		return RoleDTO{}, err
	}
	if !matchesZone {
		return RoleDTO{}, apperrors.Forbidden("Workspace không thuộc zone của phiên đăng nhập.")
	}
	if input.Code == "" || input.Name == "" {
		return RoleDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Mã role và tên role không được để trống.")
	}
	if allowed, err := s.HasWorkspacePermission(ctx, input.ActorUserID, input.WorkspaceID, "role.manage"); err != nil {
		return RoleDTO{}, err
	} else if !allowed {
		return RoleDTO{}, apperrors.Forbidden("Bạn không có quyền quản lý role.")
	}

	role, err := s.repo.CreateRole(ctx, CreateRoleParams{
		WorkspaceID:     input.WorkspaceID,
		Code:            input.Code,
		Name:            input.Name,
		Description:     input.Description,
		PermissionCodes: normalizePermissionCodes(input.PermissionCodes),
		CreatedBy:       input.ActorUserID,
	})
	if err != nil {
		if errors.Is(err, rbacdomain.ErrRoleAlreadyExists) {
			return RoleDTO{}, apperrors.Conflict("ROLE_ALREADY_EXISTS", "Mã role đã tồn tại trong workspace.")
		}
		return RoleDTO{}, err
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: input.ActorUserID,
		WorkspaceID: input.WorkspaceID,
		Action:      "role.create",
		EntityType:  "role",
		EntityID:    role.ID,
	})
	return toRoleDTO(role), nil
}

func (s *Service) ListWorkspaceMemberRoles(ctx context.Context, workspaceID string, userID string) ([]RoleDTO, error) {
	roles, err := s.repo.ListWorkspaceMemberRoles(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	return toRoleDTOs(roles), nil
}

func (s *Service) AssignWorkspaceRole(ctx context.Context, input AssignWorkspaceRoleInput) error {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.RoleID = strings.TrimSpace(input.RoleID)
	if input.WorkspaceID == "" || input.UserID == "" || input.RoleID == "" {
		return apperrors.BadRequest("VALIDATION_ERROR", "Thiếu workspace_id, user_id hoặc role_id.")
	}
	if allowed, err := s.HasWorkspacePermission(ctx, input.ActorUserID, input.WorkspaceID, "role.manage"); err != nil {
		return err
	} else if !allowed {
		return apperrors.Forbidden("Bạn không có quyền gán role.")
	}
	if err := s.repo.AssignWorkspaceRole(ctx, AssignWorkspaceRoleParams{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		RoleID:      input.RoleID,
		AssignedBy:  input.ActorUserID,
	}); err != nil {
		if errors.Is(err, rbacdomain.ErrRoleNotFound) {
			return apperrors.NotFound("ROLE_NOT_FOUND", "Không tìm thấy role hoặc thành viên workspace.")
		}
		return err
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: input.ActorUserID,
		WorkspaceID: input.WorkspaceID,
		Action:      "role.assign",
		EntityType:  "workspace_member",
		EntityID:    input.UserID,
		Metadata: map[string]any{
			"role_id": input.RoleID,
		},
	})
	return nil
}

func (s *Service) RevokeWorkspaceRole(ctx context.Context, input AssignWorkspaceRoleInput) error {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.RoleID = strings.TrimSpace(input.RoleID)
	if input.WorkspaceID == "" || input.UserID == "" || input.RoleID == "" {
		return apperrors.BadRequest("VALIDATION_ERROR", "Thiếu workspace_id, user_id hoặc role_id.")
	}
	if allowed, err := s.HasWorkspacePermission(ctx, input.ActorUserID, input.WorkspaceID, "role.manage"); err != nil {
		return err
	} else if !allowed {
		return apperrors.Forbidden("Bạn không có quyền gỡ role.")
	}
	if err := s.repo.RevokeWorkspaceRole(ctx, RevokeWorkspaceRoleParams{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		RoleID:      input.RoleID,
	}); err != nil {
		return err
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: input.ActorUserID,
		WorkspaceID: input.WorkspaceID,
		Action:      "role.revoke",
		EntityType:  "workspace_member",
		EntityID:    input.UserID,
		Metadata: map[string]any{
			"role_id": input.RoleID,
		},
	})
	return nil
}

func (s *Service) ListMyWorkspacePermissions(ctx context.Context, userID string, workspaceID string) ([]PermissionDTO, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, apperrors.BadRequest("WORKSPACE_REQUIRED", "Thiếu workspace_id để kiểm tra quyền.")
	}

	permissions, err := s.repo.ListUserWorkspacePermissions(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	return toDTOs(permissions), nil
}

func (s *Service) HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	permissionCode = strings.TrimSpace(permissionCode)
	if workspaceID == "" {
		return false, apperrors.BadRequest("WORKSPACE_REQUIRED", "Thiếu workspace_id để kiểm tra quyền.")
	}
	if permissionCode == "" {
		return false, apperrors.BadRequest("PERMISSION_REQUIRED", "Thiếu permission để kiểm tra quyền.")
	}
	return s.repo.HasWorkspacePermission(ctx, userID, workspaceID, permissionCode)
}

// HasAnyWorkspacePermission is reserved for operations that do not yet have a
// target workspace (for example creating another workspace). A brand-new user
// therefore cannot bootstrap themselves into workspace_owner.
func (s *Service) HasAnyWorkspacePermission(ctx context.Context, userID string, permissionCode string) (bool, error) {
	userID = strings.TrimSpace(userID)
	permissionCode = strings.TrimSpace(permissionCode)
	if userID == "" {
		return false, apperrors.Unauthorized("Phiên đăng nhập không hợp lệ.")
	}
	if permissionCode == "" {
		return false, apperrors.BadRequest("PERMISSION_REQUIRED", "Thiếu permission để kiểm tra quyền.")
	}
	return s.repo.HasAnyWorkspacePermission(ctx, userID, permissionCode)
}

func (s *Service) HasAnyZonePermission(ctx context.Context, userID string, zoneID string, permissionCode string) (bool, error) {
	userID = strings.TrimSpace(userID)
	zoneID = strings.TrimSpace(zoneID)
	permissionCode = strings.ToLower(strings.TrimSpace(permissionCode))
	if userID == "" || zoneID == "" || permissionCode == "" {
		return false, nil
	}
	return s.repo.HasAnyZonePermission(ctx, userID, zoneID, permissionCode)
}

func (s *Service) WorkspaceBelongsToZone(ctx context.Context, workspaceID string, zoneID string) (bool, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	zoneID = strings.TrimSpace(zoneID)
	if workspaceID == "" || zoneID == "" {
		return false, nil
	}
	return s.repo.WorkspaceBelongsToZone(ctx, workspaceID, zoneID)
}

func toDTOs(permissions []rbacdomain.Permission) []PermissionDTO {
	dtos := make([]PermissionDTO, 0, len(permissions))
	for _, permission := range permissions {
		dtos = append(dtos, PermissionDTO{
			ID:          permission.ID,
			Code:        permission.Code,
			Module:      permission.Module,
			Action:      permission.Action,
			Name:        permission.Name,
			Description: permission.Description,
		})
	}
	return dtos
}

func toRoleDTOs(roles []rbacdomain.Role) []RoleDTO {
	dtos := make([]RoleDTO, 0, len(roles))
	for _, role := range roles {
		dtos = append(dtos, toRoleDTO(role))
	}
	return dtos
}

func toRoleDTO(role rbacdomain.Role) RoleDTO {
	return RoleDTO{
		ID:          role.ID,
		WorkspaceID: role.WorkspaceID,
		Code:        role.Code,
		Name:        role.Name,
		Description: role.Description,
		IsSystem:    role.IsSystem,
		CreatedBy:   role.CreatedBy,
		Permissions: toDTOs(role.Permissions),
	}
}

func normalizePermissionCodes(codes []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.ToLower(strings.TrimSpace(code))
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		result = append(result, code)
	}
	return result
}
