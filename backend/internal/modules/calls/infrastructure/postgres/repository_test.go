package postgres

import (
	"errors"
	"testing"

	callsdomain "github.com/duclamdev/application-chat/backend/internal/modules/calls/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapCreateCallPostgresErrorSelfTargetCheck(t *testing.T) {
	err := &pgconn.PgError{
		Code:           "23514",
		ConstraintName: "call_sessions_check",
		Message:        `new row for relation "call_sessions" violates check constraint "call_sessions_check"`,
		Detail:         "Failing row contains the same initiator_user_id and target_user_id.",
	}

	if got := mapCreateCallPostgresError(err); !errors.Is(got, callsdomain.ErrCallInvalidTarget) {
		t.Fatalf("mapped error = %#v, want ErrCallInvalidTarget", got)
	}
}

func TestMapCreateCallPostgresErrorNotNullPayload(t *testing.T) {
	err := &pgconn.PgError{
		Code:           "23502",
		ConstraintName: "call_sessions_metadata_not_null",
		Message:        `null value in column "metadata" violates not-null constraint`,
	}

	if got := mapCreateCallPostgresError(err); !errors.Is(got, callsdomain.ErrCallInvalidPayload) {
		t.Fatalf("mapped error = %#v, want ErrCallInvalidPayload", got)
	}
}
