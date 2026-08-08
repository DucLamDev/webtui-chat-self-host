package bootstrap

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	authapp "github.com/duclamdev/application-chat/backend/internal/modules/auth/application"
)

type recordingScheduledLegalReader struct {
	record         authapp.LegalAcceptanceRecord
	err            error
	userID         string
	workspaceID    string
	termsVersion   string
	privacyVersion string
}

func (r *recordingScheduledLegalReader) ReadCurrentLegalAcceptances(
	_ context.Context,
	userID string,
	workspaceID string,
	termsVersion string,
	privacyVersion string,
) (authapp.LegalAcceptanceRecord, error) {
	r.userID = userID
	r.workspaceID = workspaceID
	r.termsVersion = termsVersion
	r.privacyVersion = privacyVersion
	return r.record, r.err
}

func TestScheduledMessageLegalAcceptanceCheckerRequiresBothCurrentDocuments(t *testing.T) {
	now := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)
	reader := &recordingScheduledLegalReader{record: authapp.LegalAcceptanceRecord{
		TermsAcceptedAt:   &now,
		PrivacyAcceptedAt: &now,
	}}
	checker := scheduledMessageLegalAcceptanceChecker{
		reader:         reader,
		termsVersion:   "2026-08-07",
		privacyVersion: "2026-08-07",
	}

	accepted, err := checker.HasCurrentLegalAcceptances(context.Background(), " user-1 ", " workspace-1 ")
	if err != nil || !accepted {
		t.Fatalf("HasCurrentLegalAcceptances() = (%t, %v), want (true, nil)", accepted, err)
	}
	if reader.userID != "user-1" || reader.workspaceID != "workspace-1" ||
		reader.termsVersion != "2026-08-07" || reader.privacyVersion != "2026-08-07" {
		t.Fatalf("reader scope = %#v", reader)
	}

	reader.record.PrivacyAcceptedAt = nil
	accepted, err = checker.HasCurrentLegalAcceptances(context.Background(), "user-1", "workspace-1")
	if err != nil || accepted {
		t.Fatalf("HasCurrentLegalAcceptances() with missing privacy = (%t, %v), want (false, nil)", accepted, err)
	}
}

func TestScheduledMessageLegalAcceptanceCheckerFailsClosed(t *testing.T) {
	checker := scheduledMessageLegalAcceptanceChecker{}
	if accepted, err := checker.HasCurrentLegalAcceptances(context.Background(), "user-1", "workspace-1"); err == nil || accepted {
		t.Fatalf("missing reader = (%t, %v), want fail closed", accepted, err)
	}

	readerErr := errors.New("database unavailable")
	checker = scheduledMessageLegalAcceptanceChecker{
		reader:         &recordingScheduledLegalReader{err: readerErr},
		termsVersion:   "2026-08-07",
		privacyVersion: "2026-08-07",
	}
	if accepted, err := checker.HasCurrentLegalAcceptances(context.Background(), "user-1", "workspace-1"); !errors.Is(err, readerErr) || accepted {
		t.Fatalf("reader error = (%t, %v), want (false, %v)", accepted, err, readerErr)
	}
}

func TestWorkerWiresScheduledMessageDeliveryAuthorization(t *testing.T) {
	source, err := os.ReadFile("worker.go")
	if err != nil {
		t.Fatalf("read worker.go: %v", err)
	}
	for _, required := range []string{
		"messagesRepo.SetScheduledDeliveryLegalVersions",
		"messagesapp.NewService(messagesRepo, rbacService)",
		"messagesService.SetBlockChecker(moderationRepo)",
		"messagesService.SetCurrentLegalAcceptanceChecker",
		"termsVersion:   w.cfg.Legal.TermsVersion",
		"privacyVersion: w.cfg.Legal.PrivacyPolicyVersion",
	} {
		if !strings.Contains(string(source), required) {
			t.Fatalf("worker is missing scheduled delivery policy wiring %q", required)
		}
	}
}
