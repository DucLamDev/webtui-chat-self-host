package application

import (
	"context"
	"errors"
	"testing"
	"time"

	ticketsdomain "github.com/duclamdev/application-chat/backend/internal/modules/tickets/domain"
)

type fakeTicketRepo struct {
	createParams SaveParams
	updateParams SaveParams
}

func (r *fakeTicketRepo) Create(_ context.Context, params SaveParams) (ticketsdomain.Ticket, error) {
	r.createParams = params
	return sampleTicket(params), nil
}

func (r *fakeTicketRepo) Get(_ context.Context, workspaceID string, ticketID string) (ticketsdomain.Ticket, error) {
	if ticketID == "missing" {
		return ticketsdomain.Ticket{}, ticketsdomain.ErrTicketNotFound
	}
	return sampleTicket(SaveParams{TicketID: ticketID, WorkspaceID: workspaceID, Title: ptr("Ticket"), Status: ptr("open"), Priority: ptr("normal")}), nil
}

func (r *fakeTicketRepo) List(_ context.Context, params ListParams) ([]ticketsdomain.Ticket, error) {
	return []ticketsdomain.Ticket{sampleTicket(SaveParams{WorkspaceID: params.WorkspaceID, Title: ptr("Ticket"), Status: ptr("open"), Priority: ptr("normal")})}, nil
}

func (r *fakeTicketRepo) Update(_ context.Context, params SaveParams) (ticketsdomain.Ticket, error) {
	r.updateParams = params
	return sampleTicket(params), nil
}

type fakeTicketPermissionChecker struct {
	allowed map[string]bool
	err     error
	seen    []string
}

func (c *fakeTicketPermissionChecker) HasWorkspacePermission(_ context.Context, _ string, _ string, permissionCode string) (bool, error) {
	c.seen = append(c.seen, permissionCode)
	if c.err != nil {
		return false, c.err
	}
	return c.allowed[permissionCode], nil
}

func TestTicketServiceCreateDefaultsAndRequiresView(t *testing.T) {
	repo := &fakeTicketRepo{}
	checker := &fakeTicketPermissionChecker{allowed: map[string]bool{PermissionTicketView: true}}
	service := NewService(repo, checker)

	dto, err := service.Create(context.Background(), CreateInput{
		ActorUserID: "user-1",
		Description: "  Need help  ",
		Priority:    "",
		Title:       "  VPS expired  ",
		WorkspaceID: "workspace-1",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if dto.Status != "open" || dto.Priority != "normal" {
		t.Fatalf("Create defaults = (%q, %q), want open/normal", dto.Status, dto.Priority)
	}
	if repo.createParams.Title == nil || *repo.createParams.Title != "VPS expired" {
		t.Fatalf("Create title was not trimmed: %#v", repo.createParams.Title)
	}
	if len(checker.seen) != 1 || checker.seen[0] != PermissionTicketView {
		t.Fatalf("Create checked permissions %#v, want %q", checker.seen, PermissionTicketView)
	}
}

func TestTicketServiceCreateValidation(t *testing.T) {
	service := NewService(&fakeTicketRepo{}, &fakeTicketPermissionChecker{allowed: map[string]bool{PermissionTicketView: true}})

	if _, err := service.Create(context.Background(), CreateInput{ActorUserID: "user-1", WorkspaceID: "workspace-1"}); err == nil {
		t.Fatal("Create with empty title returned nil error")
	}
	if _, err := service.Create(context.Background(), CreateInput{ActorUserID: "user-1", WorkspaceID: "workspace-1", Title: "Ticket", Priority: "invalid"}); err == nil {
		t.Fatal("Create with invalid priority returned nil error")
	}
}

func TestTicketServiceUpdateRequiresManageAndValidatesStatus(t *testing.T) {
	repo := &fakeTicketRepo{}
	checker := &fakeTicketPermissionChecker{allowed: map[string]bool{PermissionTicketManage: true}}
	service := NewService(repo, checker)
	status := "resolved"
	assignedTo := ""

	dto, err := service.Update(context.Background(), UpdateInput{
		ActorUserID: "user-1",
		AssignedTo:  &assignedTo,
		Status:      &status,
		TicketID:    "ticket-1",
		WorkspaceID: "workspace-1",
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if dto.Status != "resolved" {
		t.Fatalf("Update status = %q, want resolved", dto.Status)
	}
	if repo.updateParams.AssignedTo == nil || *repo.updateParams.AssignedTo != "" {
		t.Fatalf("Update assigned_to should preserve explicit empty string clear, got %#v", repo.updateParams.AssignedTo)
	}
	if len(checker.seen) != 1 || checker.seen[0] != PermissionTicketManage {
		t.Fatalf("Update checked permissions %#v, want %q", checker.seen, PermissionTicketManage)
	}

	invalidStatus := "invalid"
	if _, err := service.Update(context.Background(), UpdateInput{ActorUserID: "user-1", Status: &invalidStatus, TicketID: "ticket-1", WorkspaceID: "workspace-1"}); err == nil {
		t.Fatal("Update with invalid status returned nil error")
	}
}

func TestTicketServicePropagatesPermissionErrors(t *testing.T) {
	boom := errors.New("rbac unavailable")
	service := NewService(&fakeTicketRepo{}, &fakeTicketPermissionChecker{err: boom})

	_, err := service.List(context.Background(), ListInput{ActorUserID: "user-1", WorkspaceID: "workspace-1"})
	if !errors.Is(err, boom) {
		t.Fatalf("List error = %v, want %v", err, boom)
	}
}

func sampleTicket(params SaveParams) ticketsdomain.Ticket {
	now := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	title := testRequiredString(params.Title)
	if title == "" {
		title = "Ticket"
	}
	status := testRequiredString(params.Status)
	if status == "" {
		status = "open"
	}
	priority := testRequiredString(params.Priority)
	if priority == "" {
		priority = "normal"
	}
	id := params.TicketID
	if id == "" {
		id = "ticket-1"
	}
	return ticketsdomain.Ticket{
		ID:          id,
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
		Title:       title,
		Description: testRequiredString(params.Description),
		Status:      status,
		Priority:    priority,
		CreatedBy:   &params.CreatedBy,
		AssignedTo:  params.AssignedTo,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func testRequiredString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
