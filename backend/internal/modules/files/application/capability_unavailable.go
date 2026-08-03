package application

import (
	"context"

	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type unavailableResumableRepository struct{}

func resumableCapabilityUnavailable() error {
	return apperrors.ServiceUnavailable(
		"RESUMABLE_UPLOADS_UNAVAILABLE",
		"Resumable uploads are not available with the configured files repository.",
	)
}

func (unavailableResumableRepository) CreateUploadSession(context.Context, CreateUploadSessionParams) (UploadSession, error) {
	return UploadSession{}, resumableCapabilityUnavailable()
}

func (unavailableResumableRepository) GetUploadSession(context.Context, string, string, string) (UploadSession, error) {
	return UploadSession{}, resumableCapabilityUnavailable()
}

func (unavailableResumableRepository) UpsertUploadPart(context.Context, UpsertUploadPartParams) (UploadSession, error) {
	return UploadSession{}, resumableCapabilityUnavailable()
}

func (unavailableResumableRepository) ListUploadParts(context.Context, string, string, string) ([]UploadPart, error) {
	return nil, resumableCapabilityUnavailable()
}

func (unavailableResumableRepository) MarkUploadCompleting(context.Context, string, string, string) (UploadSession, error) {
	return UploadSession{}, resumableCapabilityUnavailable()
}

func (unavailableResumableRepository) FailUploadSession(context.Context, string, string, string) error {
	return resumableCapabilityUnavailable()
}

func (unavailableResumableRepository) CompleteUploadSession(context.Context, CompleteUploadSessionParams) (UploadSession, error) {
	return UploadSession{}, resumableCapabilityUnavailable()
}

func (unavailableResumableRepository) CancelUploadSession(context.Context, string, string, string) error {
	return resumableCapabilityUnavailable()
}
