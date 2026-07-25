package application

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

func TestMapOrderClientErrorPreservesForbiddenReason(t *testing.T) {
	err := mapOrderClientError(&UpstreamError{
		StatusCode: http.StatusForbidden,
		Message:    "IP không nằm trong whitelist: 160.191.55.144",
	})

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error type = %T, want *errors.AppError", err)
	}
	if appErr.Code != "ORDER_API_FORBIDDEN" {
		t.Fatalf("code = %q, want ORDER_API_FORBIDDEN", appErr.Code)
	}
	if appErr.Status != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", appErr.Status, http.StatusBadGateway)
	}
	if !strings.Contains(appErr.Message, "160.191.55.144") {
		t.Fatalf("message = %q, want whitelist reason", appErr.Message)
	}
}

func TestMapRenewalClientErrorExplainsMissingUpstreamContract(t *testing.T) {
	err := mapRenewalClientError(&UpstreamError{StatusCode: http.StatusNotFound, Message: "Not Found"})

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("error type = %T, want *errors.AppError", err)
	}
	if appErr.Code != "ORDER_RENEWAL_NOT_SUPPORTED" {
		t.Fatalf("code = %q", appErr.Code)
	}
	if !strings.Contains(appErr.Message, "Không có số dư nào bị trừ") {
		t.Fatalf("message = %q", appErr.Message)
	}
}
