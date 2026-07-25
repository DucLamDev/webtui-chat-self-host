package application

import (
	"context"
	"errors"
	"net/http"
	"testing"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type renewalPermissionChecker map[string]bool

func (checker renewalPermissionChecker) HasWorkspacePermission(_ context.Context, _ string, _ string, permissionCode string) (bool, error) {
	return checker[permissionCode], nil
}

type renewalPermissionRepo struct {
	email string
}

func (repo renewalPermissionRepo) ChannelByID(context.Context, string, string) (ChannelDTO, error) {
	return ChannelDTO{}, nil
}

func (repo renewalPermissionRepo) SendBotMessage(context.Context, SendBotMessageParams) (BotMessageDTO, error) {
	return BotMessageDTO{}, nil
}

func (repo renewalPermissionRepo) UserEmailByID(context.Context, string) (string, error) {
	return repo.email, nil
}

func (repo renewalPermissionRepo) WorkspaceSupportsOrderBot(context.Context, string) (bool, error) {
	return true, nil
}

func TestEnsureRenewalPermissionAllowsMemberForMatchingEmail(t *testing.T) {
	service := NewService(nil, renewalPermissionRepo{email: "khach@example.com"}, renewalPermissionChecker{
		PermissionOrderPaymentRequest: true,
	})

	if err := service.ensureRenewalPermission(context.Background(), "user-id", "workspace-id", "KHACH@example.com"); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureRenewalPermissionRejectsMemberForDifferentEmail(t *testing.T) {
	service := NewService(nil, renewalPermissionRepo{email: "member@example.com"}, renewalPermissionChecker{
		PermissionOrderPaymentRequest: true,
	})

	err := service.ensureRenewalPermission(context.Background(), "user-id", "workspace-id", "customer@example.com")
	if err == nil {
		t.Fatal("expected permission error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Status != http.StatusForbidden {
		t.Fatalf("error = %#v", err)
	}
}

func TestEnsureRenewalPermissionAllowsBillingStaffForCustomerEmail(t *testing.T) {
	service := NewService(nil, renewalPermissionRepo{email: "staff@example.com"}, renewalPermissionChecker{
		PermissionOrderBilling: true,
	})

	if err := service.ensureRenewalPermission(context.Background(), "user-id", "workspace-id", "customer@example.com"); err != nil {
		t.Fatal(err)
	}
}
