package postgres

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	authapp "github.com/duclamdev/application-chat/backend/internal/modules/auth/application"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

func TestLegalAcceptancePersistenceIsExplicitAndTransactional(t *testing.T) {
	content, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	source := string(content)
	for _, required := range []string{
		"func (r *Repository) RecordLegalAcceptances",
		"r.pool.Begin(ctx)",
		"tx.Commit(ctx)",
		"func (r *Repository) CanAccessLegalAcceptance",
		"workspace.zone_id = $3::uuid",
		"member.status = 'active'",
		"canAccessLegalAcceptance(ctx, tx",
		"func (r *Repository) ReadCurrentLegalAcceptances",
		"document_type = 'terms' AND document_version = $3",
		"document_type = 'privacy' AND document_version = $4",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("legal acceptance persistence is missing %q", required)
		}
	}
}

func TestOIDCNewUserProvisioningFailsClosedWithoutLegalAcceptanceFlow(t *testing.T) {
	if err := oidcNewUserProvisioningPolicy(false); !errors.Is(err, authapp.ErrOIDCJITDisabled) {
		t.Fatalf("disabled provider policy error = %v", err)
	}
	if err := oidcNewUserProvisioningPolicy(true); !errors.Is(err, authapp.ErrOIDCJITLegalBlocked) {
		t.Fatalf("configured JIT policy error = %v, want legal acceptance blocker", err)
	}
}

func TestResolveImplicitWorkspaceID(t *testing.T) {
	tests := []struct {
		name         string
		workspaceIDs []string
		wantID       string
		wantReason   string
	}{
		{name: "single active workspace", workspaceIDs: []string{"workspace-1"}, wantID: "workspace-1"},
		{name: "no active workspace", wantReason: "no_active_workspace"},
		{name: "ambiguous active workspaces", workspaceIDs: []string{"workspace-1", "workspace-2"}, wantReason: "ambiguous_active_workspaces"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveImplicitWorkspaceID(tt.workspaceIDs)
			if tt.wantReason == "" {
				if err != nil || got != tt.wantID {
					t.Fatalf("resolveImplicitWorkspaceID() = %q, %v; want %q, nil", got, err, tt.wantID)
				}
				return
			}

			var appErr *apperrors.AppError
			if !errors.As(err, &appErr) {
				t.Fatalf("error = %T %v, want *AppError", err, err)
			}
			if got != "" || appErr.Status != http.StatusServiceUnavailable || appErr.Code != "DEFAULT_WORKSPACE_UNAVAILABLE" {
				t.Fatalf("result = %q, status=%d code=%q", got, appErr.Status, appErr.Code)
			}
			if appErr.Details["reason"] != tt.wantReason {
				t.Fatalf("reason = %#v, want %q", appErr.Details["reason"], tt.wantReason)
			}
		})
	}
}
