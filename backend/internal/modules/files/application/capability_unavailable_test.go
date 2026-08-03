package application

import (
	"context"
	"net/http"
	"testing"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

func TestUnavailableResumableRepositoryReturnsServiceUnavailable(t *testing.T) {
	t.Parallel()

	_, err := unavailableResumableRepository{}.CreateUploadSession(context.Background(), CreateUploadSessionParams{})
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Status != http.StatusServiceUnavailable || appErr.Code != "RESUMABLE_UPLOADS_UNAVAILABLE" {
		t.Fatalf("expected resumable uploads 503, got %#v", err)
	}
}
