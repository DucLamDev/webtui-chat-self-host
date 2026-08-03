package application

import (
	"context"
	"errors"
	"testing"

	rbacdomain "github.com/duclamdev/application-chat/backend/internal/modules/rbac/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type readRBACRepository struct {
	permissions          []rbacdomain.Permission
	roles                []rbacdomain.Role
	memberRoles          []rbacdomain.Role
	myPermissions        []rbacdomain.Permission
	workspacePermissions map[string]bool
	zonePermissions      map[string]bool
	workspaceMatchesZone bool
	listPermissionCalls  int
	listRoleCalls        int
	listMemberRoleCalls  int
	listMyCalls          int
}

func (r *readRBACRepository) ListPermissions(context.Context) ([]rbacdomain.Permission, error) {
	r.listPermissionCalls++
	return r.permissions, nil
}

func (r *readRBACRepository) ListRoles(context.Context, string) ([]rbacdomain.Role, error) {
	r.listRoleCalls++
	return r.roles, nil
}

func (r *readRBACRepository) CreateRole(context.Context, CreateRoleParams) (rbacdomain.Role, error) {
	return rbacdomain.Role{}, nil
}

func (r *readRBACRepository) ListWorkspaceMemberRoles(context.Context, string, string) ([]rbacdomain.Role, error) {
	r.listMemberRoleCalls++
	return r.memberRoles, nil
}

func (r *readRBACRepository) AssignWorkspaceRole(context.Context, AssignWorkspaceRoleParams) error {
	return nil
}

func (r *readRBACRepository) RevokeWorkspaceRole(context.Context, RevokeWorkspaceRoleParams) error {
	return nil
}

func (r *readRBACRepository) ListUserWorkspacePermissions(context.Context, string, string) ([]rbacdomain.Permission, error) {
	r.listMyCalls++
	return r.myPermissions, nil
}

func (r *readRBACRepository) HasWorkspacePermission(_ context.Context, _ string, _ string, permissionCode string) (bool, error) {
	return r.workspacePermissions[permissionCode], nil
}

func (r *readRBACRepository) HasAnyWorkspacePermission(context.Context, string, string) (bool, error) {
	return false, nil
}

func (r *readRBACRepository) HasAnyZonePermission(_ context.Context, _ string, _ string, permissionCode string) (bool, error) {
	return r.zonePermissions[permissionCode], nil
}

func (r *readRBACRepository) WorkspaceBelongsToZone(context.Context, string, string) (bool, error) {
	return r.workspaceMatchesZone, nil
}

func (r *readRBACRepository) RecordAudit(context.Context, AuditEvent) error {
	return nil
}

func TestListPermissionsRequiresAuthorizedZoneReader(t *testing.T) {
	for _, permission := range []string{PermissionViewAdmin, PermissionManageRole} {
		t.Run(permission, func(t *testing.T) {
			repo := &readRBACRepository{
				permissions:     []rbacdomain.Permission{{Code: "role.manage"}},
				zonePermissions: map[string]bool{permission: true},
			}
			service := NewService(repo)

			permissions, err := service.ListPermissions(context.Background(), "actor", "zone")
			if err != nil {
				t.Fatalf("ListPermissions() error = %v", err)
			}
			if len(permissions) != 1 || repo.listPermissionCalls != 1 {
				t.Fatalf("permissions/calls = %#v/%d", permissions, repo.listPermissionCalls)
			}
		})
	}

	repo := &readRBACRepository{zonePermissions: map[string]bool{}}
	service := NewService(repo)
	_, err := service.ListPermissions(context.Background(), "actor", "zone")
	assertAppStatus(t, err, 403)
	if repo.listPermissionCalls != 0 {
		t.Fatal("permission catalog repository must not run for an unauthorized actor")
	}
}

func TestListRolesRequiresExactWorkspaceZoneAndReadPermission(t *testing.T) {
	repo := &readRBACRepository{workspaceMatchesZone: false, workspacePermissions: map[string]bool{
		PermissionViewAdmin: true,
	}}
	service := NewService(repo)

	_, err := service.ListRoles(context.Background(), "actor", "zone", "workspace")
	assertAppStatus(t, err, 403)
	if repo.listRoleCalls != 0 {
		t.Fatal("role repository must not run for a workspace outside the actor zone")
	}

	repo.workspaceMatchesZone = true
	roles, err := service.ListRoles(context.Background(), "actor", "zone", "workspace")
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) != 0 || repo.listRoleCalls != 1 {
		t.Fatalf("roles/calls = %#v/%d", roles, repo.listRoleCalls)
	}
}

func TestListRolesAllowsRoleManagerWithoutAdminView(t *testing.T) {
	repo := &readRBACRepository{
		workspaceMatchesZone: true,
		workspacePermissions: map[string]bool{PermissionManageRole: true},
		roles:                []rbacdomain.Role{{ID: "role-1", Code: "custom"}},
	}
	service := NewService(repo)

	roles, err := service.ListRoles(context.Background(), "actor", "zone", "workspace")
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) != 1 || roles[0].ID != "role-1" {
		t.Fatalf("roles = %#v", roles)
	}
}

func TestListMemberRolesUsesSameWorkspaceReadGuard(t *testing.T) {
	repo := &readRBACRepository{
		workspaceMatchesZone: true,
		workspacePermissions: map[string]bool{},
	}
	service := NewService(repo)

	_, err := service.ListWorkspaceMemberRoles(context.Background(), "actor", "zone", "workspace", "member")
	assertAppStatus(t, err, 403)
	if repo.listMemberRoleCalls != 0 {
		t.Fatal("member role repository must not run for an unauthorized actor")
	}

	repo.workspacePermissions[PermissionViewAdmin] = true
	if _, err := service.ListWorkspaceMemberRoles(context.Background(), "actor", "zone", "workspace", "member"); err != nil {
		t.Fatalf("ListWorkspaceMemberRoles() error = %v", err)
	}
	if repo.listMemberRoleCalls != 1 {
		t.Fatalf("member role calls = %d, want 1", repo.listMemberRoleCalls)
	}
}

func TestListRolesRequiresWorkspaceID(t *testing.T) {
	repo := &readRBACRepository{workspaceMatchesZone: true}
	service := NewService(repo)

	_, err := service.ListRoles(context.Background(), "actor", "zone", "")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "WORKSPACE_REQUIRED" {
		t.Fatalf("error = %#v, want WORKSPACE_REQUIRED", err)
	}
}

func TestListMyPermissionsRemainsSelfScopedWithoutAdminReadPermission(t *testing.T) {
	repo := &readRBACRepository{myPermissions: []rbacdomain.Permission{{Code: "message.send"}}}
	service := NewService(repo)

	permissions, err := service.ListMyWorkspacePermissions(context.Background(), "actor", "workspace")
	if err != nil {
		t.Fatalf("ListMyWorkspacePermissions() error = %v", err)
	}
	if len(permissions) != 1 || permissions[0].Code != "message.send" || repo.listMyCalls != 1 {
		t.Fatalf("permissions/calls = %#v/%d", permissions, repo.listMyCalls)
	}
}

func assertAppStatus(t *testing.T, err error, status int) {
	t.Helper()
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Status != status {
		t.Fatalf("error = %#v, want status %d", err, status)
	}
}
