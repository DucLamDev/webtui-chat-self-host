package application

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	departmentsdomain "github.com/duclamdev/application-chat/backend/internal/modules/departments/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

var departmentSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

type PermissionChecker interface {
	HasWorkspacePermission(ctx context.Context, userID string, workspaceID string, permissionCode string) (bool, error)
}

type Repository interface {
	Create(ctx context.Context, params CreateParams) (departmentsdomain.Department, error)
	Find(ctx context.Context, workspaceID string, departmentID string) (departmentsdomain.Department, error)
	List(ctx context.Context, workspaceID string) ([]departmentsdomain.Department, error)
	CanSetParent(ctx context.Context, workspaceID string, departmentID string, parentID string) (bool, error)
	Update(ctx context.Context, params UpdateParams) (departmentsdomain.Department, error)
	Delete(ctx context.Context, workspaceID string, departmentID string) error
	AddMember(ctx context.Context, params AddMemberParams) (departmentsdomain.Member, error)
	RemoveMember(ctx context.Context, workspaceID string, departmentID string, userID string) error
	ListMembers(ctx context.Context, workspaceID string, departmentID string) ([]departmentsdomain.Member, error)
	RecordAudit(ctx context.Context, event AuditEvent) error
}

type Service struct {
	repo    Repository
	checker PermissionChecker
}

type CreateInput struct {
	ActorUserID string
	WorkspaceID string
	ParentID    string
	Slug        string
	Name        string
	Description string
}

type CreateParams struct {
	WorkspaceID string
	ParentID    string
	Slug        string
	Name        string
	Description string
	CreatedBy   string
}

type UpdateInput struct {
	ActorUserID  string
	WorkspaceID  string
	DepartmentID string
	ParentID     *string
	Slug         *string
	Name         *string
	Description  *string
}

type UpdateParams struct {
	WorkspaceID  string
	DepartmentID string
	ParentID     *string
	Slug         *string
	Name         *string
	Description  *string
}

type AddMemberInput struct {
	ActorUserID  string
	WorkspaceID  string
	DepartmentID string
	UserID       string
	Role         string
}

type AddMemberParams struct {
	WorkspaceID  string
	DepartmentID string
	UserID       string
	Role         string
}

type AuditEvent struct {
	ActorUserID string
	WorkspaceID string
	Action      string
	EntityType  string
	EntityID    string
	Metadata    map[string]any
}

type DepartmentDTO struct {
	ID           string  `json:"id"`
	WorkspaceID  string  `json:"workspace_id"`
	ParentID     *string `json:"parent_id,omitempty"`
	Slug         string  `json:"slug"`
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	CreatedBy    *string `json:"created_by,omitempty"`
	MemberCount  int     `json:"member_count"`
	LeadCount    int     `json:"lead_count"`
	ChannelCount int     `json:"channel_count"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type MemberDTO struct {
	DepartmentID string `json:"department_id"`
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	AvatarURL    *string `json:"avatar_url,omitempty"`
	Role         string `json:"role"`
	CreatedAt    string `json:"created_at"`
}

func NewService(repo Repository, checker PermissionChecker) *Service {
	return &Service{repo: repo, checker: checker}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (DepartmentDTO, error) {
	if err := s.ensureManage(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return DepartmentDTO{}, err
	}
	input.Slug = strings.ToLower(strings.TrimSpace(input.Slug))
	input.Name = strings.TrimSpace(input.Name)
	if !departmentSlugPattern.MatchString(input.Slug) {
		return DepartmentDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Slug phòng ban không hợp lệ.")
	}
	if input.Name == "" {
		return DepartmentDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên phòng ban không được để trống.")
	}
	parentID := strings.TrimSpace(input.ParentID)
	if parentID != "" {
		allowed, err := s.repo.CanSetParent(ctx, strings.TrimSpace(input.WorkspaceID), "", parentID)
		if err != nil {
			return DepartmentDTO{}, err
		}
		if !allowed {
			return DepartmentDTO{}, apperrors.BadRequest("DEPARTMENT_PARENT_INVALID", "Phòng ban cha không hợp lệ hoặc không thuộc workspace hiện tại.")
		}
	}
	department, err := s.repo.Create(ctx, CreateParams{
		WorkspaceID: strings.TrimSpace(input.WorkspaceID),
		ParentID:    parentID,
		Slug:        input.Slug,
		Name:        input.Name,
		Description: strings.TrimSpace(input.Description),
		CreatedBy:   strings.TrimSpace(input.ActorUserID),
	})
	if err != nil {
		if errors.Is(err, departmentsdomain.ErrDepartmentConflict) {
			return DepartmentDTO{}, apperrors.Conflict("DEPARTMENT_ALREADY_EXISTS", "Slug phòng ban đã tồn tại.")
		}
		return DepartmentDTO{}, err
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{ActorUserID: input.ActorUserID, WorkspaceID: input.WorkspaceID, Action: "department.create", EntityType: "department", EntityID: department.ID})
	return toDepartmentDTO(department), nil
}

func (s *Service) Get(ctx context.Context, actorUserID string, workspaceID string, departmentID string) (DepartmentDTO, error) {
	if err := s.ensureManage(ctx, actorUserID, workspaceID); err != nil {
		return DepartmentDTO{}, err
	}
	department, err := s.repo.Find(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(departmentID))
	if err != nil {
		return DepartmentDTO{}, mapDepartmentError(err)
	}
	return toDepartmentDTO(department), nil
}

func (s *Service) List(ctx context.Context, actorUserID string, workspaceID string) ([]DepartmentDTO, error) {
	if err := s.ensureManage(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	departments, err := s.repo.List(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, err
	}
	dtos := make([]DepartmentDTO, 0, len(departments))
	for _, department := range departments {
		dtos = append(dtos, toDepartmentDTO(department))
	}
	return dtos, nil
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (DepartmentDTO, error) {
	if err := s.ensureManage(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return DepartmentDTO{}, err
	}
	slug := cleanOptional(input.Slug)
	if slug != nil {
		*slug = strings.ToLower(*slug)
		if !departmentSlugPattern.MatchString(*slug) {
			return DepartmentDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Slug phòng ban không hợp lệ.")
		}
	}
	name := cleanOptional(input.Name)
	if name != nil && *name == "" {
		return DepartmentDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên phòng ban không được để trống.")
	}
	parentID := cleanOptional(input.ParentID)
	if parentID != nil && *parentID != "" {
		allowed, err := s.repo.CanSetParent(
			ctx,
			strings.TrimSpace(input.WorkspaceID),
			strings.TrimSpace(input.DepartmentID),
			*parentID,
		)
		if err != nil {
			return DepartmentDTO{}, err
		}
		if !allowed {
			return DepartmentDTO{}, apperrors.BadRequest("DEPARTMENT_PARENT_INVALID", "Không thể chọn chính phòng ban này, phòng ban con hoặc phòng ban ngoài workspace làm cấp cha.")
		}
	}
	department, err := s.repo.Update(ctx, UpdateParams{
		WorkspaceID:  strings.TrimSpace(input.WorkspaceID),
		DepartmentID: strings.TrimSpace(input.DepartmentID),
		ParentID:     parentID,
		Slug:         slug,
		Name:         name,
		Description:  cleanOptional(input.Description),
	})
	if err != nil {
		return DepartmentDTO{}, mapDepartmentError(err)
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{ActorUserID: input.ActorUserID, WorkspaceID: input.WorkspaceID, Action: "department.update", EntityType: "department", EntityID: input.DepartmentID})
	return toDepartmentDTO(department), nil
}

func (s *Service) Delete(ctx context.Context, actorUserID string, workspaceID string, departmentID string) error {
	if err := s.ensureManage(ctx, actorUserID, workspaceID); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(departmentID)); err != nil {
		return mapDepartmentError(err)
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{ActorUserID: actorUserID, WorkspaceID: workspaceID, Action: "department.delete", EntityType: "department", EntityID: departmentID})
	return nil
}

func (s *Service) AddMember(ctx context.Context, input AddMemberInput) (MemberDTO, error) {
	if err := s.ensureManage(ctx, input.ActorUserID, input.WorkspaceID); err != nil {
		return MemberDTO{}, err
	}
	role := strings.TrimSpace(input.Role)
	if role == "" {
		role = "member"
	}
	if role != "lead" && role != "member" {
		return MemberDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Vai trò phòng ban không hợp lệ.")
	}
	member, err := s.repo.AddMember(ctx, AddMemberParams{
		WorkspaceID:  strings.TrimSpace(input.WorkspaceID),
		DepartmentID: strings.TrimSpace(input.DepartmentID),
		UserID:       strings.TrimSpace(input.UserID),
		Role:         role,
	})
	if err != nil {
		return MemberDTO{}, mapDepartmentError(err)
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: input.ActorUserID,
		WorkspaceID: input.WorkspaceID,
		Action:      "department.member.upsert",
		EntityType:  "department",
		EntityID:    input.DepartmentID,
		Metadata:    map[string]any{"user_id": input.UserID, "role": role},
	})
	return toMemberDTO(member), nil
}

func (s *Service) RemoveMember(ctx context.Context, actorUserID string, workspaceID string, departmentID string, userID string) error {
	if err := s.ensureManage(ctx, actorUserID, workspaceID); err != nil {
		return err
	}
	if err := s.repo.RemoveMember(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(departmentID), strings.TrimSpace(userID)); err != nil {
		return mapDepartmentError(err)
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: actorUserID,
		WorkspaceID: workspaceID,
		Action:      "department.member.remove",
		EntityType:  "department",
		EntityID:    departmentID,
		Metadata:    map[string]any{"user_id": userID},
	})
	return nil
}

func (s *Service) ListMembers(ctx context.Context, actorUserID string, workspaceID string, departmentID string) ([]MemberDTO, error) {
	if err := s.ensureManage(ctx, actorUserID, workspaceID); err != nil {
		return nil, err
	}
	members, err := s.repo.ListMembers(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(departmentID))
	if err != nil {
		return nil, err
	}
	dtos := make([]MemberDTO, 0, len(members))
	for _, member := range members {
		dtos = append(dtos, toMemberDTO(member))
	}
	return dtos, nil
}

func (s *Service) ensureManage(ctx context.Context, userID string, workspaceID string) error {
	allowed, err := s.checker.HasWorkspacePermission(ctx, strings.TrimSpace(userID), strings.TrimSpace(workspaceID), "workspace.manage")
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Bạn không có quyền quản lý phòng ban.")
	}
	return nil
}

func mapDepartmentError(err error) error {
	if errors.Is(err, departmentsdomain.ErrDepartmentNotFound) {
		return apperrors.NotFound("DEPARTMENT_NOT_FOUND", "Không tìm thấy phòng ban.")
	}
	if errors.Is(err, departmentsdomain.ErrMemberNotFound) {
		return apperrors.NotFound("DEPARTMENT_MEMBER_NOT_FOUND", "Không tìm thấy thành viên phòng ban hoặc user chưa thuộc workspace.")
	}
	if errors.Is(err, departmentsdomain.ErrDepartmentConflict) {
		return apperrors.Conflict("DEPARTMENT_ALREADY_EXISTS", "Slug phòng ban đã tồn tại.")
	}
	return err
}

func cleanOptional(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	return &cleaned
}

func toDepartmentDTO(department departmentsdomain.Department) DepartmentDTO {
	return DepartmentDTO{
		ID:           department.ID,
		WorkspaceID:  department.WorkspaceID,
		ParentID:     department.ParentID,
		Slug:         department.Slug,
		Name:         department.Name,
		Description:  department.Description,
		CreatedBy:    department.CreatedBy,
		MemberCount:  department.MemberCount,
		LeadCount:    department.LeadCount,
		ChannelCount: department.ChannelCount,
		CreatedAt:    formatTime(department.CreatedAt),
		UpdatedAt:    formatTime(department.UpdatedAt),
	}
}

func toMemberDTO(member departmentsdomain.Member) MemberDTO {
	return MemberDTO{
		DepartmentID: member.DepartmentID,
		UserID:       member.UserID,
		Email:        member.Email,
		Username:     member.Username,
		DisplayName:  member.DisplayName,
		AvatarURL:    member.AvatarURL,
		Role:         member.Role,
		CreatedAt:    formatTime(member.CreatedAt),
	}
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}
