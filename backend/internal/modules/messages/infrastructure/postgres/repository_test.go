package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	messagesdomain "github.com/duclamdev/application-chat/backend/internal/modules/messages/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

type recordingCommandExecutor struct {
	sql  string
	args []any
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
