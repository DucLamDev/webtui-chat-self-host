package bootstrap

import (
	"context"
	"errors"
	"strings"

	authapp "github.com/duclamdev/application-chat/backend/internal/modules/auth/application"
)

type scheduledLegalAcceptanceReader interface {
	ReadCurrentLegalAcceptances(
		ctx context.Context,
		userID string,
		workspaceID string,
		termsVersion string,
		privacyVersion string,
	) (authapp.LegalAcceptanceRecord, error)
}

// scheduledMessageLegalAcceptanceChecker deliberately reads exact current
// versions without creating or backfilling evidence. Worker delivery has no
// HTTP zone context, but the scheduled row already carries its workspace and
// sender; membership and message.send permission are checked independently.
type scheduledMessageLegalAcceptanceChecker struct {
	reader         scheduledLegalAcceptanceReader
	termsVersion   string
	privacyVersion string
}

func (c scheduledMessageLegalAcceptanceChecker) HasCurrentLegalAcceptances(
	ctx context.Context,
	userID string,
	workspaceID string,
) (bool, error) {
	if c.reader == nil {
		return false, errors.New("scheduled message legal acceptance reader is unavailable")
	}
	userID = strings.TrimSpace(userID)
	workspaceID = strings.TrimSpace(workspaceID)
	termsVersion := strings.TrimSpace(c.termsVersion)
	privacyVersion := strings.TrimSpace(c.privacyVersion)
	if userID == "" || workspaceID == "" || termsVersion == "" || privacyVersion == "" {
		return false, errors.New("scheduled message legal acceptance scope is incomplete")
	}
	record, err := c.reader.ReadCurrentLegalAcceptances(
		ctx,
		userID,
		workspaceID,
		termsVersion,
		privacyVersion,
	)
	if err != nil {
		return false, err
	}
	return record.TermsAcceptedAt != nil && record.PrivacyAcceptedAt != nil, nil
}
