package application

import (
	"context"
	"errors"
	"testing"

	usersdomain "github.com/duclamdev/application-chat/backend/internal/modules/users/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type accountDeletionRepository struct {
	deletedUserID           string
	ownershipSuccessorEmail string
	deleteOwnErr            error
	workspaceMember         bool
	findInZoneUser          usersdomain.User
	findInZoneErr           error
	findActorUserID         string
	findAllowZoneWide       bool
	listedUsers             []usersdomain.User
	listParams              ListUsersParams
	workspaceMemberCalls    int
	updateUserCalls         int
	deleteUserCalls         int
}

type accountConnectionRevoker struct {
	userID string
}

func (revoker *accountConnectionRevoker) DisconnectUser(_ context.Context, userID string) error {
	revoker.userID = userID
	return nil
}

func (r *accountDeletionRepository) FindByID(context.Context, string) (usersdomain.User, error) {
	return usersdomain.User{}, usersdomain.ErrUserNotFound
}

func (r *accountDeletionRepository) FindByIDInZone(_ context.Context, _ string, _ string, actorUserID string, allowZoneWide bool) (usersdomain.User, error) {
	r.findActorUserID = actorUserID
	r.findAllowZoneWide = allowZoneWide
	return r.findInZoneUser, r.findInZoneErr
}

func (r *accountDeletionRepository) List(_ context.Context, params ListUsersParams) ([]usersdomain.User, error) {
	r.listParams = params
	return r.listedUsers, nil
}

func (r *accountDeletionRepository) UserBelongsToWorkspace(context.Context, string, string) (bool, error) {
	r.workspaceMemberCalls++
	return r.workspaceMember, nil
}

func (r *accountDeletionRepository) UpdateProfile(context.Context, UpdateProfileParams) (usersdomain.User, error) {
	return usersdomain.User{}, nil
}

func (r *accountDeletionRepository) UpdateUser(_ context.Context, params UpdateUserParams) (usersdomain.User, error) {
	r.updateUserCalls++
	return usersdomain.User{ID: params.UserID}, nil
}

func (r *accountDeletionRepository) DeleteUser(context.Context, string) error {
	r.deleteUserCalls++
	return nil
}

func (r *accountDeletionRepository) DeleteOwnAccount(_ context.Context, userID string, ownershipSuccessorEmail string) error {
	r.deletedUserID = userID
	r.ownershipSuccessorEmail = ownershipSuccessorEmail
	return r.deleteOwnErr
}

type userPermissionChecker struct {
	workspaceAllowed bool
	zoneAllowed      bool
}

func (c userPermissionChecker) HasWorkspacePermission(context.Context, string, string, string) (bool, error) {
	return c.workspaceAllowed, nil
}

func (c userPermissionChecker) HasAnyZonePermission(context.Context, string, string, string) (bool, error) {
	return c.zoneAllowed, nil
}

func TestListUsersScopesRegularDirectoryAndRedactsSecurityMetadata(t *testing.T) {
	ip := "203.0.113.10"
	device := "private-device"
	repo := &accountDeletionRepository{listedUsers: []usersdomain.User{{
		ID: "user-2", Email: "user@example.com", RegistrationIP: &ip, DeviceName: &device,
	}}}
	service := NewService(repo, userPermissionChecker{})

	users, _, err := service.List(context.Background(), ListUsersParams{
		ActorUserID: " actor-1 ", ZoneID: " zone-1 ", Query: " user ", Limit: 25,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repo.listParams.ActorUserID != "actor-1" || repo.listParams.ZoneID != "zone-1" || repo.listParams.AllowZoneWide {
		t.Fatalf("directory scope = %#v", repo.listParams)
	}
	if len(users) != 1 || users[0].RegistrationIP != nil || users[0].DeviceName != nil {
		t.Fatalf("regular directory result leaked security metadata: %#v", users)
	}
}

func TestListUsersAllowsZoneUserManagerAndKeepsSecurityMetadata(t *testing.T) {
	ip := "203.0.113.10"
	repo := &accountDeletionRepository{listedUsers: []usersdomain.User{{ID: "user-2", RegistrationIP: &ip}}}
	service := NewService(repo, userPermissionChecker{zoneAllowed: true})

	users, _, err := service.List(context.Background(), ListUsersParams{ActorUserID: "admin-1", ZoneID: "zone-1"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !repo.listParams.AllowZoneWide || len(users) != 1 || users[0].RegistrationIP == nil {
		t.Fatalf("managed directory result/scope = %#v / %#v", users, repo.listParams)
	}
}

func TestListUsersRejectsMissingActor(t *testing.T) {
	repo := &accountDeletionRepository{}
	service := NewService(repo, userPermissionChecker{})

	_, _, err := service.List(context.Background(), ListUsersParams{ZoneID: "zone-1"})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Status != 401 {
		t.Fatalf("error = %#v, want unauthorized", err)
	}
	if repo.listParams.ZoneID != "" {
		t.Fatal("repository must not be called without an authenticated actor")
	}
}

func TestGetUserUsesScopedVisibilityAndRedactsSecurityMetadata(t *testing.T) {
	ip := "203.0.113.10"
	repo := &accountDeletionRepository{findInZoneUser: usersdomain.User{ID: "user-2", LastIPAddress: &ip}}
	service := NewService(repo, userPermissionChecker{})

	user, err := service.Get(context.Background(), "actor-1", "user-2", "zone-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if repo.findActorUserID != "actor-1" || repo.findAllowZoneWide || user.LastIPAddress != nil {
		t.Fatalf("scoped get result = %#v", user)
	}
}

func TestDeleteOwnAccountRequiresExplicitConfirmation(t *testing.T) {
	repo := &accountDeletionRepository{}
	service := NewService(repo)

	err := service.DeleteOwnAccount(context.Background(), "user-1", "delete", "")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "ACCOUNT_DELETION_CONFIRMATION_REQUIRED" {
		t.Fatalf("error = %#v, want confirmation error", err)
	}
	if repo.deletedUserID != "" {
		t.Fatalf("repository called for unconfirmed deletion: %q", repo.deletedUserID)
	}
}

func TestDeleteOwnAccountPermanentlyDeletesAuthenticatedUser(t *testing.T) {
	repo := &accountDeletionRepository{}
	service := NewService(repo)
	revoker := &accountConnectionRevoker{}
	service.SetAccountConnectionRevoker(revoker)

	if err := service.DeleteOwnAccount(context.Background(), " user-1 ", " DELETE ", " owner@example.com "); err != nil {
		t.Fatalf("DeleteOwnAccount() error = %v", err)
	}
	if repo.deletedUserID != "user-1" {
		t.Fatalf("deleted user = %q, want user-1", repo.deletedUserID)
	}
	if repo.ownershipSuccessorEmail != "owner@example.com" {
		t.Fatalf("successor email = %q, want owner@example.com", repo.ownershipSuccessorEmail)
	}
	if revoker.userID != "user-1" {
		t.Fatalf("disconnected user = %q, want user-1", revoker.userID)
	}
}

func TestDeleteOwnAccountMapsMissingUser(t *testing.T) {
	repo := &accountDeletionRepository{deleteOwnErr: usersdomain.ErrUserNotFound}
	service := NewService(repo)

	err := service.DeleteOwnAccount(context.Background(), "user-1", "DELETE", "")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "USER_NOT_FOUND" {
		t.Fatalf("error = %#v, want USER_NOT_FOUND", err)
	}
}

func TestDeleteOwnAccountRejectsWorkspaceOwner(t *testing.T) {
	repo := &accountDeletionRepository{deleteOwnErr: usersdomain.ErrUserOwnsWorkspace}
	service := NewService(repo)

	err := service.DeleteOwnAccount(context.Background(), "user-1", "DELETE", "")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "ACCOUNT_OWNERSHIP_TRANSFER_REQUIRED" {
		t.Fatalf("error = %#v, want ACCOUNT_OWNERSHIP_TRANSFER_REQUIRED", err)
	}
}

func TestDeleteOwnAccountRejectsInvalidOwnershipSuccessor(t *testing.T) {
	repo := &accountDeletionRepository{deleteOwnErr: usersdomain.ErrOwnershipSuccessorNotEligible}
	service := NewService(repo)

	err := service.DeleteOwnAccount(context.Background(), "user-1", "DELETE", "missing@example.com")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "ACCOUNT_OWNERSHIP_SUCCESSOR_INVALID" {
		t.Fatalf("error = %#v, want ACCOUNT_OWNERSHIP_SUCCESSOR_INVALID", err)
	}
}

func TestAdministrativeDisableDisconnectsRealtimeUser(t *testing.T) {
	repo := &accountDeletionRepository{workspaceMember: true}
	service := NewService(repo, userPermissionChecker{workspaceAllowed: true})
	revoker := &accountConnectionRevoker{}
	service.SetAccountConnectionRevoker(revoker)
	status := "disabled"

	if _, err := service.Update(context.Background(), UpdateUserInput{
		ActorUserID: "admin-1",
		WorkspaceID: "workspace-1",
		UserID:      "user-1",
		Status:      &status,
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if revoker.userID != "user-1" {
		t.Fatalf("disconnected user = %q, want user-1", revoker.userID)
	}
}

func TestAdministrativeDeleteDisconnectsRealtimeUser(t *testing.T) {
	repo := &accountDeletionRepository{workspaceMember: true}
	service := NewService(repo, userPermissionChecker{workspaceAllowed: true})
	revoker := &accountConnectionRevoker{}
	service.SetAccountConnectionRevoker(revoker)

	if err := service.Delete(context.Background(), "admin-1", "workspace-1", "user-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if revoker.userID != "user-1" {
		t.Fatalf("disconnected user = %q, want user-1", revoker.userID)
	}
}

func TestAdministrativeUpdateFailsClosedWithoutPermissionChecker(t *testing.T) {
	repo := &accountDeletionRepository{workspaceMember: true}
	service := NewService(repo)
	status := "disabled"

	_, err := service.Update(context.Background(), UpdateUserInput{
		ActorUserID: "admin-1",
		WorkspaceID: "workspace-1",
		UserID:      "user-1",
		Status:      &status,
	})
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Status != 503 || appErr.Code != "RBAC_CHECKER_UNAVAILABLE" {
		t.Fatalf("Update() error = %#v, want RBAC_CHECKER_UNAVAILABLE (503)", err)
	}
	if repo.workspaceMemberCalls != 0 || repo.updateUserCalls != 0 {
		t.Fatalf(
			"repository membership/update calls = %d/%d, want 0/0",
			repo.workspaceMemberCalls,
			repo.updateUserCalls,
		)
	}
}

func TestAdministrativeDeleteFailsClosedWithoutPermissionChecker(t *testing.T) {
	repo := &accountDeletionRepository{workspaceMember: true}
	service := NewService(repo)

	err := service.Delete(context.Background(), "admin-1", "workspace-1", "user-1")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Status != 503 || appErr.Code != "RBAC_CHECKER_UNAVAILABLE" {
		t.Fatalf("Delete() error = %#v, want RBAC_CHECKER_UNAVAILABLE (503)", err)
	}
	if repo.workspaceMemberCalls != 0 || repo.deleteUserCalls != 0 {
		t.Fatalf(
			"repository membership/delete calls = %d/%d, want 0/0",
			repo.workspaceMemberCalls,
			repo.deleteUserCalls,
		)
	}
}
