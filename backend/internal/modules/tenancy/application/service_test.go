package application

import (
	"context"
	"errors"
	"testing"
	"time"

	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	webhooksecurity "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/security"
)

type fakeRepo struct {
	resolved            tenancydomain.ResolvedZone
	err                 error
	domain              string
	claim               tenancydomain.DomainClaim
	createdClaimParams  CreateDomainClaimParams
	provisionParams     ProvisionVerifiedDomainParams
	verificationFailure string
	template            tenancydomain.AutomationTemplate
	installation        tenancydomain.AutomationInstallation
	installationParams  CreateAutomationInstallationParams
	updateParams        UpdateAutomationInstallationParams
	deletedInstallation string
	oidcProviders       []tenancydomain.OIDCProvider
	oidcCreateParams    CreateOIDCProviderParams
}

func (r *fakeRepo) ResolveByDomain(_ context.Context, domain string) (tenancydomain.ResolvedZone, error) {
	r.domain = domain
	return r.resolved, r.err
}

func (r *fakeRepo) WorkspaceBelongsToZone(context.Context, string, string) (bool, error) {
	return true, nil
}

func (r *fakeRepo) ZoneDomainBelongsToActiveZone(context.Context, string, string) (bool, error) {
	return true, nil
}

func (r *fakeRepo) ZoneDomainBelongsToRecoverableZone(context.Context, string, string) (bool, error) {
	return true, nil
}

func (r *fakeRepo) CreateDomainClaim(_ context.Context, params CreateDomainClaimParams) (tenancydomain.DomainClaim, error) {
	r.createdClaimParams = params
	return r.claim, r.err
}

func (r *fakeRepo) CreateAdditionalDomainClaim(_ context.Context, _ CreateAdditionalDomainClaimParams) (tenancydomain.DomainClaim, error) {
	return r.claim, r.err
}

func (r *fakeRepo) FindDomainClaim(context.Context, string, string) (tenancydomain.DomainClaim, error) {
	return r.claim, r.err
}

func (r *fakeRepo) RecordDomainVerificationFailure(_ context.Context, _ string, _ string, _ time.Time, reason string) error {
	r.verificationFailure = reason
	return nil
}

func (r *fakeRepo) ProvisionVerifiedDomain(_ context.Context, params ProvisionVerifiedDomainParams) (tenancydomain.ResolvedZone, error) {
	r.provisionParams = params
	return r.resolved, r.err
}

func (r *fakeRepo) ActivateVerifiedDomain(_ context.Context, params ProvisionVerifiedDomainParams) (tenancydomain.ResolvedZone, error) {
	r.provisionParams = params
	return r.resolved, r.err
}

func (r *fakeRepo) CanManageZone(context.Context, string, string) (bool, error) {
	return true, nil
}

func (r *fakeRepo) GetZoneAdminOverview(context.Context, string) (tenancydomain.ZoneAdminOverview, error) {
	return tenancydomain.ZoneAdminOverview{}, r.err
}

func (r *fakeRepo) UpdateZoneSettings(context.Context, UpdateZoneSettingsParams) (tenancydomain.ZoneAdminOverview, error) {
	return tenancydomain.ZoneAdminOverview{}, r.err
}

func (r *fakeRepo) SetZoneLifecycle(context.Context, SetZoneLifecycleParams) error {
	return r.err
}

func (r *fakeRepo) SetPrimaryDomain(context.Context, string, string, string) error {
	return r.err
}

func (r *fakeRepo) DeleteZoneDomain(context.Context, string, string, string) error {
	return r.err
}

func (r *fakeRepo) ListDeploymentRequests(context.Context, string) ([]tenancydomain.DeploymentRequest, error) {
	return nil, r.err
}

func (r *fakeRepo) CreateDeploymentRequest(context.Context, CreateDeploymentRequestParams) (tenancydomain.DeploymentRequest, error) {
	return tenancydomain.DeploymentRequest{}, r.err
}

func (r *fakeRepo) GetZoneQuota(context.Context, string) (tenancydomain.ZoneQuota, tenancydomain.ZoneUsage, error) {
	return tenancydomain.ZoneQuota{}, tenancydomain.ZoneUsage{}, r.err
}

func (r *fakeRepo) UpdateZoneQuota(context.Context, UpdateZoneQuotaParams) (tenancydomain.ZoneQuota, tenancydomain.ZoneUsage, error) {
	return tenancydomain.ZoneQuota{}, tenancydomain.ZoneUsage{}, r.err
}

func (r *fakeRepo) ListOIDCProviders(context.Context, string) ([]tenancydomain.OIDCProvider, error) {
	return r.oidcProviders, r.err
}

func (r *fakeRepo) CreateOIDCProvider(_ context.Context, params CreateOIDCProviderParams) (tenancydomain.OIDCProvider, error) {
	r.oidcCreateParams = params
	return tenancydomain.OIDCProvider{}, r.err
}

func (r *fakeRepo) UpdateOIDCProvider(context.Context, UpdateOIDCProviderParams) (tenancydomain.OIDCProvider, error) {
	return tenancydomain.OIDCProvider{}, r.err
}

func (r *fakeRepo) DeleteOIDCProvider(context.Context, string, string, string) error {
	return r.err
}

func (r *fakeRepo) ListAutomationTemplates(context.Context, string) ([]tenancydomain.AutomationTemplate, error) {
	return nil, r.err
}

func (r *fakeRepo) GetAutomationTemplate(context.Context, string, string) (tenancydomain.AutomationTemplate, error) {
	return r.template, r.err
}

func (r *fakeRepo) ListAutomationInstallations(context.Context, string) ([]tenancydomain.AutomationInstallation, error) {
	return nil, r.err
}

func (r *fakeRepo) CreateAutomationInstallation(_ context.Context, params CreateAutomationInstallationParams) (tenancydomain.AutomationInstallation, error) {
	r.installationParams = params
	return r.installation, r.err
}

func (r *fakeRepo) UpdateAutomationInstallation(_ context.Context, params UpdateAutomationInstallationParams) (tenancydomain.AutomationInstallation, error) {
	r.updateParams = params
	return tenancydomain.AutomationInstallation{
		ID:        params.InstallationID,
		ZoneID:    params.ZoneID,
		Name:      valueOrEmpty(params.Name),
		Status:    valueOrEmpty(params.Status),
		Config:    mapValueOrEmpty(params.Config),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, r.err
}

func (r *fakeRepo) DeleteAutomationInstallation(_ context.Context, _ string, installationID string, _ string) error {
	r.deletedInstallation = installationID
	return r.err
}

type fakeTXTResolver struct {
	records []string
	err     error
	name    string
}

func (r *fakeTXTResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	r.name = name
	return r.records, r.err
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "scheme path and port", raw: " https://Chat.ABC.com:443/path ", want: "chat.abc.com"},
		{name: "plain host", raw: "CHAT.Example.vn", want: "chat.example.vn"},
		{name: "trailing dot", raw: "chat.example.com.", want: "chat.example.com"},
		{name: "localhost dev", raw: "localhost:8080", want: "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeDomain(tt.raw)
			if err != nil {
				t.Fatalf("NormalizeDomain() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeDomain() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeDomainRejectsInvalidInput(t *testing.T) {
	if _, err := NormalizeDomain("not a domain"); err == nil {
		t.Fatal("NormalizeDomain() expected error")
	}
}

func TestDiscoverBuildsRuntimeFromDeployment(t *testing.T) {
	repo := &fakeRepo{
		resolved: tenancydomain.ResolvedZone{
			Zone: tenancydomain.Zone{
				ID:       "zone-1",
				Slug:     "abc",
				Name:     "ABC",
				Kind:     "customer_dedicated",
				Status:   "active",
				Metadata: map[string]any{"capabilities": map[string]any{"federation": true}},
			},
			Deployment: &tenancydomain.Deployment{
				Mode:         "dedicated_compose",
				WebBaseURL:   "https://chat.abc.com",
				APIBaseURL:   "https://api.chat.abc.com",
				WSBaseURL:    "wss://api.chat.abc.com/ws",
				DatabaseMode: "dedicated_database",
				Status:       "ready",
			},
			Workspace: &tenancydomain.WorkspaceRef{ID: "workspace-1", Slug: "abc", Name: "ABC Chat"},
		},
	}
	service := NewService(repo, Options{
		AppName:        "VPSTTT Chat",
		AppVersion:     "0.9.0",
		DeploymentMode: "self_hosted",
	})

	discovery, err := service.Discover(context.Background(), "HTTPS://CHAT.ABC.COM")
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if repo.domain != "chat.abc.com" {
		t.Fatalf("repo domain = %q", repo.domain)
	}
	if discovery.Runtime.APIBaseURL != "https://api.chat.abc.com" {
		t.Fatalf("api base = %q", discovery.Runtime.APIBaseURL)
	}
	if discovery.Runtime.AppName != "ABC" {
		t.Fatalf("app name = %q, want zone name", discovery.Runtime.AppName)
	}
	if discovery.Capabilities.Federation || !discovery.Capabilities.Dedicated ||
		!discovery.Capabilities.OIDCConfiguration || !discovery.Capabilities.SelfHosted ||
		discovery.Capabilities.CustomDomain {
		t.Fatalf("capabilities = %+v", discovery.Capabilities)
	}
	if discovery.Workspace == nil || discovery.Workspace.ID != "workspace-1" {
		t.Fatalf("workspace = %+v", discovery.Workspace)
	}
}

func TestDiscoverMapsMissingZone(t *testing.T) {
	service := NewService(&fakeRepo{err: tenancydomain.ErrZoneNotFound}, Options{})
	_, err := service.Discover(context.Background(), "chat.abc.com")
	if err == nil {
		t.Fatal("Discover() expected error")
	}
	if errors.Is(err, tenancydomain.ErrZoneNotFound) {
		t.Fatal("Discover() should map domain error to application error")
	}
}

func TestDiscoverEnablesSSOOnlyForRuntimeReadyProvider(t *testing.T) {
	secretRef := "env://company"
	repo := &fakeRepo{
		resolved: tenancydomain.ResolvedZone{
			Zone: tenancydomain.Zone{
				ID:     "zone-1",
				Status: "active",
			},
		},
		oidcProviders: []tenancydomain.OIDCProvider{{
			ClientSecretRef: &secretRef,
			Status:          "configured",
		}},
	}
	service := NewService(repo, Options{
		OIDCEnabled:       true,
		OIDCClientSecrets: map[string]string{"company": "secret"},
	})

	discovery, err := service.Discover(context.Background(), "chat.company.example")
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if !discovery.Capabilities.SSO {
		t.Fatalf("SSO capability = false, want true")
	}
}

func TestCreateOIDCProviderValidatesAndNormalizesClaimMapping(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo, Options{})

	_, err := service.CreateOIDCProvider(context.Background(), CreateOIDCProviderInput{
		ActorUserID: "user-1",
		ZoneID:      "zone-1",
		Name:        "Company",
		IssuerURL:   "https://id.company.example",
		ClientID:    "vpsttt-chat",
		ClaimMapping: map[string]any{
			"email": 42,
		},
	})
	if err == nil {
		t.Fatal("CreateOIDCProvider() expected invalid claim mapping error")
	}

	_, err = service.CreateOIDCProvider(context.Background(), CreateOIDCProviderInput{
		ActorUserID: "user-1",
		ZoneID:      "zone-1",
		Name:        "Company",
		IssuerURL:   "https://id.company.example",
		ClientID:    "vpsttt-chat",
	})
	if err != nil {
		t.Fatalf("CreateOIDCProvider() error = %v", err)
	}
	if repo.oidcCreateParams.ClaimMapping["email_verified"] != "email_verified" {
		t.Fatalf("default claim mapping = %#v", repo.oidcCreateParams.ClaimMapping)
	}
}

func TestCreateOIDCProviderRejectsInvalidEnvironmentSecretAlias(t *testing.T) {
	service := NewService(&fakeRepo{}, Options{})

	_, err := service.CreateOIDCProvider(context.Background(), CreateOIDCProviderInput{
		ActorUserID:     "user-1",
		ZoneID:          "zone-1",
		Name:            "Company",
		IssuerURL:       "https://id.company.example",
		ClientID:        "vpsttt-chat",
		ClientSecretRef: "env://../../system",
	})
	if err == nil {
		t.Fatal("CreateOIDCProvider() expected invalid client secret alias error")
	}
}

func TestCreateDomainClaimNormalizesAndSetsExpiry(t *testing.T) {
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		claim: tenancydomain.DomainClaim{
			Zone:   tenancydomain.Zone{ID: "zone-1"},
			Domain: tenancydomain.Domain{ID: "domain-1"},
		},
	}
	service := NewService(repo, Options{
		Now:             func() time.Time { return now },
		VerificationTTL: 2 * time.Hour,
	})

	claim, err := service.CreateDomainClaim(context.Background(), CreateDomainClaimInput{
		ActorUserID: "user-1",
		Domain:      "HTTPS://CHAT.Customer.Example/path",
		ZoneName:    "Customer Chat",
	})
	if err != nil {
		t.Fatalf("CreateDomainClaim() error = %v", err)
	}
	if repo.createdClaimParams.Domain != "chat.customer.example" {
		t.Fatalf("domain = %q", repo.createdClaimParams.Domain)
	}
	if repo.createdClaimParams.RegistrationMode != "invite_only" {
		t.Fatalf("registration mode = %q", repo.createdClaimParams.RegistrationMode)
	}
	if !repo.createdClaimParams.ExpiresAt.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("expires at = %s", repo.createdClaimParams.ExpiresAt)
	}
	if claim.RoutingDNSType != "" || claim.RoutingDNSName != "" || claim.RoutingDNSValue != "" {
		t.Fatalf("routing DNS should be omitted when not configured: %+v", claim)
	}
}

func TestCreateDomainClaimReturnsVPSRoutingRecord(t *testing.T) {
	repo := &fakeRepo{
		claim: tenancydomain.DomainClaim{
			Zone: tenancydomain.Zone{ID: "zone-1", Name: "Customer Chat"},
			Domain: tenancydomain.Domain{
				ID:                "domain-1",
				Domain:            "duclam.com",
				VerificationToken: "token-123",
			},
		},
	}
	service := NewService(repo, Options{
		RoutingDNSType:   "A",
		RoutingDNSTarget: "160.191.55.144",
	})

	claim, err := service.CreateDomainClaim(context.Background(), CreateDomainClaimInput{
		ActorUserID: "user-1",
		Domain:      "duclam.com",
		ZoneName:    "Duclam Chat",
	})
	if err != nil {
		t.Fatalf("CreateDomainClaim() error = %v", err)
	}
	if claim.RoutingDNSType != "A" ||
		claim.RoutingDNSName != "duclam.com" ||
		claim.RoutingDNSValue != "160.191.55.144" {
		t.Fatalf("routing DNS = %s %s %s", claim.RoutingDNSType, claim.RoutingDNSName, claim.RoutingDNSValue)
	}
}

func TestVerifyDomainClaimProvisionsOnlyAfterMatchingTXT(t *testing.T) {
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	repo := &fakeRepo{
		claim: tenancydomain.DomainClaim{
			Zone: tenancydomain.Zone{ID: "zone-1", Status: "provisioning"},
			Domain: tenancydomain.Domain{
				ID:                    "domain-1",
				Domain:                "chat.customer.example",
				Status:                "pending",
				VerificationToken:     "token-123",
				VerificationExpiresAt: &expiresAt,
			},
		},
		resolved: tenancydomain.ResolvedZone{
			Zone: tenancydomain.Zone{ID: "zone-1", Status: "active"},
			Deployment: &tenancydomain.Deployment{
				Mode: "shared", DatabaseMode: "shared_schema", Status: "ready",
			},
		},
	}
	resolver := &fakeTXTResolver{
		records: []string{"vpsttt-chat-verification=token-123"},
	}
	service := NewService(repo, Options{
		Now:         func() time.Time { return now },
		TXTResolver: resolver,
	})

	discovery, err := service.VerifyDomainClaim(context.Background(), "user-1", "domain-1")
	if err != nil {
		t.Fatalf("VerifyDomainClaim() error = %v", err)
	}
	if resolver.name != "_vpsttt-chat.chat.customer.example" {
		t.Fatalf("TXT name = %q", resolver.name)
	}
	if repo.provisionParams.DomainID != "domain-1" || !repo.provisionParams.VerifiedAt.Equal(now) {
		t.Fatalf("provision params = %+v", repo.provisionParams)
	}
	if discovery.Deployment.Status != "ready" {
		t.Fatalf("deployment = %+v", discovery.Deployment)
	}
}

func TestVerifyDomainClaimRecordsFailedTXTLookup(t *testing.T) {
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	repo := &fakeRepo{
		claim: tenancydomain.DomainClaim{
			Zone: tenancydomain.Zone{ID: "zone-1"},
			Domain: tenancydomain.Domain{
				ID:                    "domain-1",
				Domain:                "chat.customer.example",
				Status:                "pending",
				VerificationToken:     "token-123",
				VerificationExpiresAt: &expiresAt,
			},
		},
	}
	service := NewService(repo, Options{
		Now:         func() time.Time { return now },
		TXTResolver: &fakeTXTResolver{},
	})

	if _, err := service.VerifyDomainClaim(context.Background(), "user-1", "domain-1"); err == nil {
		t.Fatal("VerifyDomainClaim() expected error")
	}
	if repo.verificationFailure != "dns_txt_not_found" {
		t.Fatalf("verification failure = %q", repo.verificationFailure)
	}
	if repo.provisionParams.DomainID != "" {
		t.Fatalf("unexpected provision params = %+v", repo.provisionParams)
	}
}

func TestCreateAutomationInstallationRejectsInlineSecret(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo, Options{})

	if _, err := service.CreateAutomationInstallation(context.Background(), CreateAutomationInstallationInput{
		ActorUserID: "user-1",
		ZoneID:      "zone-1",
		TemplateKey: "customer-basic-webhook-bot",
		Name:        "Support workflow",
		Config:      map[string]any{"api_key": "plaintext-secret"},
	}); err == nil {
		t.Fatal("CreateAutomationInstallation() expected inline secret error")
	}
	if repo.installationParams.TemplateKey != "" {
		t.Fatalf("repository should not be called: %+v", repo.installationParams)
	}
}

func TestCreateAutomationInstallationRejectsInlineSecretInsideArray(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo, Options{})

	if _, err := service.CreateAutomationInstallation(context.Background(), CreateAutomationInstallationInput{
		ActorUserID: "user-1",
		ZoneID:      "zone-1",
		TemplateKey: "customer-basic-webhook-bot",
		Name:        "Unsafe nested config",
		Config: map[string]any{
			"connectors": []any{
				map[string]any{"credentials": map[string]any{"api_token": "plaintext-secret"}},
			},
		},
	}); err == nil {
		t.Fatal("CreateAutomationInstallation() expected nested inline secret error")
	}
}

func TestValidateAutomationConfigEnforcesSchemaAndBlocksRemoteReferences(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []any{"endpoint_url", "event_types"},
		"properties": map[string]any{
			"endpoint_url": map[string]any{"type": "string", "format": "uri"},
			"event_types": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}
	if err := ValidateAutomationConfig(schema, map[string]any{
		"endpoint_url": 42,
		"event_types":  []any{"MessageCreated"},
	}); err == nil {
		t.Fatal("ValidateAutomationConfig() expected type validation error")
	}
	if err := ValidateAutomationConfig(schema, map[string]any{
		"endpoint_url": "https://bot.example.com/events",
		"event_types":  []any{"MessageCreated"},
	}); err != nil {
		t.Fatalf("ValidateAutomationConfig() valid config error = %v", err)
	}
	if err := ValidateAutomationConfig(map[string]any{
		"$ref": "https://schemas.example.com/automation.json",
	}, map[string]any{}); err == nil {
		t.Fatal("ValidateAutomationConfig() expected external reference error")
	}
}

func TestCreateAutomationInstallationBuildsExecutableWebhookRuntime(t *testing.T) {
	runtimeWebhookID := "webhook-1"
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		template: tenancydomain.AutomationTemplate{
			Key:           "customer-basic-webhook-bot",
			RuntimeKind:   "outgoing_webhook",
			DefaultConfig: map[string]any{"event_types": []any{"MessageCreated"}},
		},
		installation: tenancydomain.AutomationInstallation{
			ID:               "installation-1",
			ZoneID:           "zone-1",
			Name:             "Support workflow",
			Status:           "enabled",
			Config:           map[string]any{"endpoint_url": "https://bot.example.com/events"},
			RuntimeWebhookID: &runtimeWebhookID,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
	const masterSecret = "test-webhook-master-secret"
	service := NewService(repo, Options{WebhookSigningSecret: masterSecret})

	created, err := service.CreateAutomationInstallation(context.Background(), CreateAutomationInstallationInput{
		ActorUserID: "user-1",
		ZoneID:      "zone-1",
		TemplateKey: "customer-basic-webhook-bot",
		Name:        "Support workflow",
		Status:      "enabled",
		Config: map[string]any{
			"endpoint_url": "https://bot.example.com/events",
			"channel_slug": "support",
			"event_types":  []any{"MessageCreated", "MessageCreated"},
		},
	})
	if err != nil {
		t.Fatalf("CreateAutomationInstallation() error = %v", err)
	}
	if created.RuntimeSecret == "" || !created.RuntimeReady {
		t.Fatalf("runtime response = %+v", created)
	}
	decrypted, err := webhooksecurity.DecryptSecret(masterSecret, repo.installationParams.RuntimeSecretEncrypted)
	if err != nil {
		t.Fatalf("DecryptSecret() error = %v", err)
	}
	if decrypted != created.RuntimeSecret {
		t.Fatal("returned runtime secret does not match encrypted runtime secret")
	}
	if repo.installationParams.RuntimeTargetURL != "https://bot.example.com/events" {
		t.Fatalf("runtime target = %q", repo.installationParams.RuntimeTargetURL)
	}
	if len(repo.installationParams.RuntimeEventTypes) != 1 ||
		repo.installationParams.RuntimeEventTypes[0] != "MessageCreated" {
		t.Fatalf("runtime event types = %#v", repo.installationParams.RuntimeEventTypes)
	}
}

func TestUpdateAutomationInstallationNormalizesAndAllowsClearingSecretRef(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo, Options{})
	name := "  Alert workflow  "
	status := " ENABLED "
	secretRef := " "
	config := map[string]any{"channel": "alerts"}

	_, err := service.UpdateAutomationInstallation(context.Background(), UpdateAutomationInstallationInput{
		ActorUserID:    "user-1",
		ZoneID:         "zone-1",
		InstallationID: "installation-1",
		Name:           &name,
		Status:         &status,
		Config:         &config,
		SecretRef:      &secretRef,
	})
	if err != nil {
		t.Fatalf("UpdateAutomationInstallation() error = %v", err)
	}
	if repo.updateParams.Name == nil || *repo.updateParams.Name != "Alert workflow" {
		t.Fatalf("normalized name = %#v", repo.updateParams.Name)
	}
	if repo.updateParams.Status == nil || *repo.updateParams.Status != "enabled" {
		t.Fatalf("normalized status = %#v", repo.updateParams.Status)
	}
	if repo.updateParams.SecretRef == nil || *repo.updateParams.SecretRef != "" {
		t.Fatalf("cleared secret_ref = %#v", repo.updateParams.SecretRef)
	}
}

func TestDeleteAutomationInstallationUsesCurrentZone(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo, Options{})
	if err := service.DeleteAutomationInstallation(
		context.Background(),
		"user-1",
		"zone-1",
		"installation-1",
	); err != nil {
		t.Fatalf("DeleteAutomationInstallation() error = %v", err)
	}
	if repo.deletedInstallation != "installation-1" {
		t.Fatalf("deleted installation = %q", repo.deletedInstallation)
	}
}

func TestCertificateDomainAllowedRequiresResolvablePublicDomain(t *testing.T) {
	repo := &fakeRepo{resolved: tenancydomain.ResolvedZone{
		Zone:   tenancydomain.Zone{ID: "zone-1", Status: "active"},
		Domain: tenancydomain.Domain{Status: "active"},
	}}
	service := NewService(repo, Options{})

	if !service.CertificateDomainAllowed(context.Background(), "chat.customer.example") {
		t.Fatal("expected active customer domain to be allowed")
	}
	if service.CertificateDomainAllowed(context.Background(), "localhost") {
		t.Fatal("localhost must never be authorized for public ACME")
	}
	repo.resolved.Zone.Status = "suspended"
	if service.CertificateDomainAllowed(context.Background(), "chat.customer.example") {
		t.Fatal("suspended zone must not be authorized for public ACME")
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func mapValueOrEmpty(value *map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return *value
}
