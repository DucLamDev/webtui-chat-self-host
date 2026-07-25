package postgres

import (
	"errors"
	"net/http"
	"testing"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

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
