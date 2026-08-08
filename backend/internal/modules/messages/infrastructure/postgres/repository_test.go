package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	messagesdomain "github.com/duclamdev/application-chat/backend/internal/modules/messages/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestScheduledDeliveryCancellationReason(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantReason string
		terminal   bool
	}{
		{
			name:       "permission revoked",
			err:        messagesdomain.ErrScheduledDeliveryPermissionRevoked,
			wantReason: "sender permission revoked before delivery",
			terminal:   true,
		},
		{
			name:       "legal acceptance stale",
			err:        messagesdomain.ErrScheduledDeliveryLegalAcceptanceStale,
			wantReason: "current legal documents not accepted before delivery",
			terminal:   true,
		},
		{
			name:       "interaction blocked",
			err:        messagesdomain.ErrInteractionBlocked,
			wantReason: "interaction blocked before delivery",
			terminal:   true,
		},
		{name: "transient database failure", err: errors.New("database unavailable")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reason, terminal := scheduledDeliveryCancellationReason(test.err)
			if reason != test.wantReason || terminal != test.terminal {
				t.Fatalf("scheduledDeliveryCancellationReason() = (%q, %t), want (%q, %t)", reason, terminal, test.wantReason, test.terminal)
			}
		})
	}
}

func TestSetScheduledMessageDeliveryStateSurfacesPersistenceFailure(t *testing.T) {
	if err := setScheduledMessageDeliveryState(
		context.Background(),
		fixedCommandExecutor{tag: pgconn.NewCommandTag("UPDATE 0")},
		"scheduled-1",
		"cancelled",
		"permission revoked",
		"",
	); err == nil || !strings.Contains(err.Error(), "affected 0 rows") {
		t.Fatalf("zero-row state update error = %v", err)
	}

	databaseErr := errors.New("database unavailable")
	if err := setScheduledMessageDeliveryState(
		context.Background(),
		fixedCommandExecutor{err: databaseErr},
		"scheduled-1",
		"failed",
		"temporary failure",
		"",
	); !errors.Is(err, databaseErr) {
		t.Fatalf("state update error = %v, want %v", err, databaseErr)
	}
}

func TestScheduledDeliveryPolicyIsRecheckedInsideSendTransaction(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	text := string(source)
	for _, required := range []string{
		"if params.EnforceScheduledDeliveryPolicy",
		"ensureScheduledDeliveryPolicy(ctx, tx, params)",
		"sender.status = 'active' AND sender.deleted_at IS NULL",
		"FOR SHARE OF sender",
		"AND zone.status = 'active'",
		"AND workspace.status = 'active'",
		"FOR SHARE OF workspace, zone",
		"permission.code = 'message.send'",
		"FROM user_legal_acceptances acceptance",
		"FOR SHARE OF member, assignment, role, role_permission, permission",
		"FOR SHARE OF acceptance",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("scheduled delivery transaction is missing %q", required)
		}
	}
	idempotencyIndex := strings.Index(text, "r.findByClientMessageID(ctx, tx, params)")
	policyIndex := strings.Index(text, "ensureScheduledDeliveryPolicy(ctx, tx, params)")
	if idempotencyIndex < 0 || policyIndex < 0 || idempotencyIndex > policyIndex {
		t.Fatal("idempotent retry must resolve before mutable scheduled-delivery policy")
	}
}

func TestScheduledRoleAuthorizationKeepsTransientErrorsRetryable(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	err := scheduledRoleAuthorizationResult(false, databaseErr)
	if !errors.Is(err, databaseErr) {
		t.Fatalf("scheduledRoleAuthorizationResult() error = %v, want wrapped transient error", err)
	}
	if errors.Is(err, messagesdomain.ErrScheduledDeliveryPermissionRevoked) {
		t.Fatalf("transient error was misclassified as terminal permission revocation: %v", err)
	}
	if err := scheduledRoleAuthorizationResult(false, pgx.ErrNoRows); !errors.Is(err, messagesdomain.ErrScheduledDeliveryPermissionRevoked) {
		t.Fatalf("no-row error = %v, want permission revoked", err)
	}
}

type recordingCommandExecutor struct {
	sql  string
	args []any
}

type fixedCommandExecutor struct {
	tag pgconn.CommandTag
	err error
}

func (e fixedCommandExecutor) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return e.tag, e.err
}

func (e *recordingCommandExecutor) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	e.sql = sql
	e.args = arguments
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func TestTouchDirectConversationLastMessageUpdatesSummaryFields(t *testing.T) {
	createdAt := time.Date(2026, 7, 21, 9, 30, 0, 0, time.UTC)
	exec := &recordingCommandExecutor{}

	err := touchDirectConversationLastMessage(context.Background(), exec, messagesdomain.Message{
		ID:          "11111111-1111-1111-1111-111111111111",
		WorkspaceID: "22222222-2222-2222-2222-222222222222",
		ChannelID:   "33333333-3333-3333-3333-333333333333",
		CreatedAt:   createdAt,
	})
	if err != nil {
		t.Fatalf("touchDirectConversationLastMessage() error = %v", err)
	}

	if !strings.Contains(exec.sql, "UPDATE direct_conversations") ||
		!strings.Contains(exec.sql, "last_message_id = $3::uuid") {
		t.Fatalf("unexpected SQL: %s", exec.sql)
	}
	wantArgs := []any{
		"22222222-2222-2222-2222-222222222222",
		"33333333-3333-3333-3333-333333333333",
		"11111111-1111-1111-1111-111111111111",
		createdAt,
	}
	if len(exec.args) != len(wantArgs) {
		t.Fatalf("args len = %d, want %d", len(exec.args), len(wantArgs))
	}
	for i := range wantArgs {
		if exec.args[i] != wantArgs[i] {
			t.Fatalf("arg[%d] = %#v, want %#v", i, exec.args[i], wantArgs[i])
		}
	}
}
