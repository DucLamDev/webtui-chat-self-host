package application

import (
	"context"
	"net/http"
	"testing"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

func TestUnavailableProductivityRepositoryReturnsServiceUnavailable(t *testing.T) {
	t.Parallel()

	_, err := unavailableProductivityRepository{}.CreateReminder(context.Background(), CreateReminderParams{})
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Status != http.StatusServiceUnavailable || appErr.Code != "MESSAGE_PRODUCTIVITY_UNAVAILABLE" {
		t.Fatalf("expected message productivity 503, got %#v", err)
	}
}
