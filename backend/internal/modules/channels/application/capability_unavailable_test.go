package application

import (
	"context"
	"net/http"
	"testing"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

func TestUnavailableTalkRepositoryReturnsServiceUnavailable(t *testing.T) {
	t.Parallel()

	_, err := unavailableTalkRepository{}.GetTalkIntegration(context.Background(), "workspace")
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Status != http.StatusServiceUnavailable || appErr.Code != "TALK_REPOSITORY_UNAVAILABLE" {
		t.Fatalf("expected Talk repository 503, got %#v", err)
	}
}
