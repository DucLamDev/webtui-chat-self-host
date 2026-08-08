package postgres

import (
	"errors"
	"os"
	"strings"
	"testing"

	webhooksdomain "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/domain"
	"github.com/jackc/pgx/v5"
)

func TestIntegrationLegalVersionsFailClosed(t *testing.T) {
	for _, test := range []struct {
		name       string
		terms      string
		privacy    string
		wantTerms  string
		wantPolicy string
		wantErr    error
	}{
		{name: "missing terms", privacy: "privacy-v3", wantErr: webhooksdomain.ErrIntegrationLegalUnavailable},
		{name: "missing privacy", terms: "terms-v2", wantErr: webhooksdomain.ErrIntegrationLegalUnavailable},
		{name: "current exact versions", terms: " terms-v2 ", privacy: " privacy-v3 ", wantTerms: "terms-v2", wantPolicy: "privacy-v3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			terms, privacy, err := integrationLegalVersions(test.terms, test.privacy)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("integrationLegalVersions() error = %v, want %v", err, test.wantErr)
			}
			if terms != test.wantTerms || privacy != test.wantPolicy {
				t.Fatalf("integrationLegalVersions() = (%q, %q)", terms, privacy)
			}
		})
	}
}

func TestIntegrationPolicyRowResultKeepsDatabaseErrorsRetryable(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	rejected := webhooksdomain.ErrIntegrationCredentialStale
	if err := integrationPolicyRowResult(false, databaseErr, rejected, "policy lookup"); !errors.Is(err, databaseErr) || errors.Is(err, rejected) {
		t.Fatalf("generic database error was misclassified: %v", err)
	}
	if err := integrationPolicyRowResult(false, pgx.ErrNoRows, rejected, "policy lookup"); !errors.Is(err, rejected) {
		t.Fatalf("no-row result = %v, want %v", err, rejected)
	}
	if err := integrationPolicyRowResult(true, nil, rejected, "policy lookup"); err != nil {
		t.Fatalf("allowed policy result = %v", err)
	}
}

func TestIntegrationMessagesRecheckOwnerCredentialAndLegalPolicyInsideTransaction(t *testing.T) {
	sourceBytes, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	source := string(sourceBytes)
	incoming := functionSection(t, source, "func (r *Repository) SendIncomingMessage", "func (r *Repository) SendIntegrationMessage")
	token := functionSection(t, source, "func (r *Repository) SendIntegrationMessage", "func (r *Repository) CreateDeliveriesForEvent")

	assertOrdered(t, incoming,
		"COALESCE(iw.created_by::text, '')",
		"ensureIntegrationProducerEligibility(",
		"ensureIntegrationChannel(",
		"ensureIncomingWebhookCredentialCurrent(",
		"ensureIntegrationLegalAcceptance(",
		"insertIntegrationMessage(",
		"tx.Commit(ctx)",
	)
	if strings.Contains(incoming, "FOR SHARE OF iw") {
		t.Fatal("incoming dispatch must not upgrade a shared webhook lock before updating last_used_at")
	}
	assertOrdered(t, token,
		"ensureIntegrationProducerEligibility(",
		"ensureIntegrationChannel(",
		"ensureAPITokenCredentialCurrent(",
		"ensureIntegrationLegalAcceptance(",
		"insertIntegrationMessage(",
		"tx.Commit(ctx)",
	)

	for _, required := range []string{
		"producer.status = 'active'",
		"producer.deleted_at IS NULL",
		"member.status = 'active'",
		"workspace.status = 'active'",
		"zone.status = 'active'",
		"channel.status = 'active'",
		"FOR UPDATE OF webhook",
		"webhook.created_by = $3::uuid",
		"token.owner_id = $3::uuid",
		"token.status = 'active'",
		"token.expires_at IS NULL OR token.expires_at > now()",
		"scope.code = 'message.write'",
		"FROM user_legal_acceptances acceptance",
		"acceptance.document_type = 'terms' AND acceptance.document_version = $3",
		"acceptance.document_type = 'privacy' AND acceptance.document_version = $4",
		"FOR SHARE OF acceptance",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("transactional integration policy is missing %q", required)
		}
	}
}

func functionSection(t *testing.T, source string, startMarker string, endMarker string) string {
	t.Helper()
	start := strings.Index(source, startMarker)
	end := strings.Index(source, endMarker)
	if start < 0 || end < 0 || start >= end {
		t.Fatalf("cannot isolate function between %q and %q", startMarker, endMarker)
	}
	return source[start:end]
}

func assertOrdered(t *testing.T, source string, markers ...string) {
	t.Helper()
	previous := -1
	for _, marker := range markers {
		index := strings.Index(source, marker)
		if index < 0 {
			t.Fatalf("source is missing %q", marker)
		}
		if index <= previous {
			t.Fatalf("%q must appear after the previous policy step", marker)
		}
		previous = index
	}
}
