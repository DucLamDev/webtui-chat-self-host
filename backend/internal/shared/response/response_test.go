package response

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/puddle/v2"
)

func TestErrorMapsExpectedInfrastructureFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name           string
		err            error
		wantStatus     int
		wantCode       string
		wantRetryAfter string
	}{
		{
			name:       "zone quota",
			err:        &pgconn.PgError{Code: "P0001", Message: "ZONE_QUOTA_EXCEEDED:members"},
			wantStatus: http.StatusConflict,
			wantCode:   "ZONE_QUOTA_EXCEEDED",
		},
		{
			name:       "deadline",
			err:        context.DeadlineExceeded,
			wantStatus: http.StatusGatewayTimeout,
			wantCode:   "REQUEST_TIMEOUT",
		},
		{
			name:       "invalid uuid",
			err:        &pgconn.PgError{Code: "22P02"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_IDENTIFIER",
		},
		{
			name:       "unique conflict",
			err:        &pgconn.PgError{Code: "23505"},
			wantStatus: http.StatusConflict,
			wantCode:   "RESOURCE_ALREADY_EXISTS",
		},
		{
			name:           "deadlock",
			err:            &pgconn.PgError{Code: "40P01"},
			wantStatus:     http.StatusServiceUnavailable,
			wantCode:       "DATABASE_RETRY_REQUIRED",
			wantRetryAfter: "1",
		},
		{
			name:           "lock unavailable",
			err:            &pgconn.PgError{Code: "55P03"},
			wantStatus:     http.StatusServiceUnavailable,
			wantCode:       "DATABASE_RETRY_REQUIRED",
			wantRetryAfter: "1",
		},
		{
			name:       "cardinality violation",
			err:        &pgconn.PgError{Code: "21000"},
			wantStatus: http.StatusConflict,
			wantCode:   "DATA_CONSISTENCY_CONFLICT",
		},
		{
			name:           "connection exception",
			err:            &pgconn.PgError{Code: "08006"},
			wantStatus:     http.StatusServiceUnavailable,
			wantCode:       "DATABASE_UNAVAILABLE",
			wantRetryAfter: "2",
		},
		{
			name:           "database shutdown",
			err:            &pgconn.PgError{Code: "57P01"},
			wantStatus:     http.StatusServiceUnavailable,
			wantCode:       "DATABASE_UNAVAILABLE",
			wantRetryAfter: "2",
		},
		{
			name:       "schema mismatch",
			err:        &pgconn.PgError{Code: "42703"},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "DATABASE_SCHEMA_MISMATCH",
		},
		{
			name:       "unclassified postgres error",
			err:        &pgconn.PgError{Code: "XX000"},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "DATABASE_ERROR",
		},
		{
			name:           "query cancelled by timeout",
			err:            &pgconn.PgError{Code: "57014"},
			wantStatus:     http.StatusGatewayTimeout,
			wantCode:       "DATABASE_TIMEOUT",
			wantRetryAfter: "1",
		},
		{
			name:           "closed connection",
			err:            pgconn.ErrConnClosed,
			wantStatus:     http.StatusServiceUnavailable,
			wantCode:       "DATABASE_UNAVAILABLE",
			wantRetryAfter: "2",
		},
		{
			name:           "closed connection pool",
			err:            puddle.ErrClosedPool,
			wantStatus:     http.StatusServiceUnavailable,
			wantCode:       "DATABASE_UNAVAILABLE",
			wantRetryAfter: "2",
		},
		{
			name:       "unknown",
			err:        errors.New("unexpected"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(writer)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/test", nil)

			Error(ctx, tt.err)

			if writer.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", writer.Code, tt.wantStatus)
			}
			var body Body
			if err := json.Unmarshal(writer.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error == nil || body.Error.Code != tt.wantCode {
				t.Fatalf("code = %#v, want %s", body.Error, tt.wantCode)
			}
			if got := writer.Header().Get("Retry-After"); got != tt.wantRetryAfter {
				t.Fatalf("Retry-After = %q, want %q", got, tt.wantRetryAfter)
			}
		})
	}
}

func TestErrorLogsServerSideAppError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	ctx.Set("request_id", "request-123")

	Error(ctx, apperrors.New("DEFAULT_WORKSPACE_UNAVAILABLE", "Workspace chưa sẵn sàng.", http.StatusServiceUnavailable))

	if writer.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", writer.Code, http.StatusServiceUnavailable)
	}
	for _, expected := range []string{"DEFAULT_WORKSPACE_UNAVAILABLE", "status=503", "request_id=request-123", "path=/auth/register"} {
		if !strings.Contains(logs.String(), expected) {
			t.Fatalf("logs = %q, missing %q", logs.String(), expected)
		}
	}
}
