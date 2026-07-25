package application

import (
	"context"
	"crypto/subtle"
	"net/url"
	"strings"
	"testing"
	"time"

	authdomain "github.com/duclamdev/application-chat/backend/internal/modules/auth/domain"
	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
)

type oidcMemoryRepository struct {
	target         ZoneAccess
	provider       tenancydomain.OIDCProvider
	stateParams    CreateOIDCLoginStateParams
	resultParams   CreateOIDCLoginResultParams
	stateConsumed  bool
	resultConsumed bool
	resolvedUser   authdomain.User
	resolveParams  ResolveOIDCUserParams
}

func (r *oidcMemoryRepository) ListPublicOIDCProviders(
	context.Context,
	string,
) (ZoneAccess, []tenancydomain.OIDCProvider, error) {
	return r.target, []tenancydomain.OIDCProvider{r.provider}, nil
}

func (r *oidcMemoryRepository) ResolveOIDCProvider(
	context.Context,
	string,
	string,
) (ZoneAccess, tenancydomain.OIDCProvider, error) {
	return r.target, r.provider, nil
}

func (r *oidcMemoryRepository) CreateOIDCLoginState(
	_ context.Context,
	params CreateOIDCLoginStateParams,
) error {
	r.stateParams = params
	return nil
}

func (r *oidcMemoryRepository) ConsumeOIDCLoginState(
	_ context.Context,
	stateHash []byte,
	_ time.Time,
) (OIDCLoginState, error) {
	if r.stateConsumed || subtle.ConstantTimeCompare(stateHash, r.stateParams.StateHash) != 1 {
		return OIDCLoginState{}, ErrOIDCStateNotFound
	}
	r.stateConsumed = true
	return OIDCLoginState{
		Provider:              r.provider,
		Target:                r.target,
		RedirectURI:           r.stateParams.RedirectURI,
		ReturnTo:              r.stateParams.ReturnTo,
		CodeVerifierEncrypted: r.stateParams.CodeVerifierEncrypted,
		Nonce:                 r.stateParams.Nonce,
		DeviceName:            r.stateParams.DeviceName,
		IPAddress:             r.stateParams.IPAddress,
		UserAgent:             r.stateParams.UserAgent,
	}, nil
}

func (r *oidcMemoryRepository) CreateOIDCLoginResult(
	_ context.Context,
	params CreateOIDCLoginResultParams,
) error {
	r.resultParams = params
	return nil
}

func (r *oidcMemoryRepository) ConsumeOIDCLoginResult(
	_ context.Context,
	codeHash []byte,
	_ string,
	_ time.Time,
) (OIDCLoginResult, error) {
	if r.resultConsumed || subtle.ConstantTimeCompare(codeHash, r.resultParams.CodeHash) != 1 {
		return OIDCLoginResult{}, ErrOIDCResultNotFound
	}
	r.resultConsumed = true
	return OIDCLoginResult{
		Provider:   r.provider,
		Target:     r.target,
		Claims:     r.resultParams.Claims,
		DeviceName: r.resultParams.DeviceName,
		IPAddress:  r.resultParams.IPAddress,
		UserAgent:  r.resultParams.UserAgent,
	}, nil
}

func (r *oidcMemoryRepository) ResolveOIDCUser(
	_ context.Context,
	params ResolveOIDCUserParams,
) (authdomain.User, error) {
	r.resolveParams = params
	return r.resolvedUser, nil
}

type oidcProtocolFake struct {
	verifier string
	nonce    string
}

func (p *oidcProtocolFake) AuthorizationURL(
	_ context.Context,
	_ tenancydomain.OIDCProvider,
	_ string,
	_ string,
	state string,
	nonce string,
	codeVerifier string,
) (string, error) {
	p.verifier = codeVerifier
	p.nonce = nonce
	return "https://identity.example/authorize?state=" + url.QueryEscape(state), nil
}

func (p *oidcProtocolFake) ExchangeAndVerify(
	_ context.Context,
	_ tenancydomain.OIDCProvider,
	_ string,
	_ string,
	_ string,
	codeVerifier string,
	nonce string,
) (map[string]any, error) {
	if codeVerifier != p.verifier || nonce != p.nonce {
		return nil, ErrOIDCStateNotFound
	}
	return map[string]any{
		"sub":                "subject-123",
		"email":              "member@example.com",
		"email_verified":     true,
		"preferred_username": "member",
		"name":               "Member Example",
	}, nil
}

func TestOIDCAuthorizationCodeFlowUsesPKCENonceAndOneTimeCodes(t *testing.T) {
	target := testZoneAccess()
	provider := tenancydomain.OIDCProvider{
		ID:                   "33333333-3333-4333-8333-333333333333",
		ZoneID:               target.ZoneID,
		Name:                 "Company SSO",
		IssuerURL:            "https://identity.example",
		ClientID:             "vpsttt-chat",
		Scopes:               []string{"openid", "email", "profile"},
		ClaimMapping:         map[string]any{},
		JITProvisioning:      true,
		RequireVerifiedEmail: true,
		Status:               "configured",
	}
	repo := &oidcMemoryRepository{
		target:   target,
		provider: provider,
		resolvedUser: authdomain.User{
			ID:          "44444444-4444-4444-8444-444444444444",
			Email:       "member@example.com",
			Username:    "member-oidc",
			DisplayName: "Member Example",
			Status:      "active",
		},
	}
	baseRepo := &workspaceProvisioningRepo{}
	base := NewService(
		baseRepo,
		sharedauth.NewManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour),
	)
	protocol := &oidcProtocolFake{}
	service := NewOIDCService(
		base,
		repo,
		protocol,
		"oidc-state-secret-with-at-least-32-characters",
		nil,
	)
	fixedNow := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	start, err := service.Start(context.Background(), OIDCStartInput{
		Domain:      target.Domain,
		ProviderID:  provider.ID,
		RedirectURI: "https://" + target.Domain + "/api/v1/auth/oidc/callback",
		ReturnTo:    "/",
		DeviceName:  "Browser",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	authorizationURL, _ := url.Parse(start.AuthorizationURL)
	stateToken := authorizationURL.Query().Get("state")
	if stateToken == "" || protocol.verifier == "" || protocol.nonce == "" {
		t.Fatal("Start() did not create state, nonce, and PKCE verifier")
	}
	if strings.Contains(repo.stateParams.CodeVerifierEncrypted, protocol.verifier) {
		t.Fatal("PKCE verifier was stored in plaintext")
	}

	callback, err := service.Callback(context.Background(), OIDCCallbackInput{
		State: stateToken,
		Code:  "authorization-code",
	})
	if err != nil {
		t.Fatalf("Callback() error = %v", err)
	}
	redirectURL, _ := url.Parse(callback.RedirectURL)
	completionCode := redirectURL.Query().Get("oidc_code")
	if completionCode == "" {
		t.Fatal("Callback() did not return a one-time completion code")
	}
	if _, err := service.Callback(context.Background(), OIDCCallbackInput{
		State: stateToken,
		Code:  "authorization-code",
	}); err == nil {
		t.Fatal("Callback() accepted a replayed state")
	}

	result, err := service.Complete(context.Background(), OIDCCompleteInput{
		Code:   completionCode,
		Domain: target.Domain,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if result.User.ID != repo.resolvedUser.ID || result.Tokens.AccessToken == "" {
		t.Fatalf("Complete() result = %#v", result)
	}
	if repo.resolveParams.Subject != "subject-123" || repo.resolveParams.Email != "member@example.com" {
		t.Fatalf("ResolveOIDCUser params = %#v", repo.resolveParams)
	}
	if _, err := service.Complete(context.Background(), OIDCCompleteInput{
		Code:   completionCode,
		Domain: target.Domain,
	}); err == nil {
		t.Fatal("Complete() accepted a replayed completion code")
	}
}

func TestOIDCRejectsUnverifiedEmailByDefault(t *testing.T) {
	_, err := mappedOIDCProfile(tenancydomain.OIDCProvider{
		RequireVerifiedEmail: true,
		ClaimMapping:         map[string]any{},
	}, map[string]any{
		"sub":            "subject",
		"email":          "member@example.com",
		"email_verified": false,
	})
	if err == nil {
		t.Fatal("mappedOIDCProfile() accepted an unverified email")
	}
}

func TestOIDCDoesNotPromoteUnverifiedEmailWhenProviderAllowsIt(t *testing.T) {
	profile, err := mappedOIDCProfile(tenancydomain.OIDCProvider{
		RequireVerifiedEmail: false,
		ClaimMapping:         map[string]any{},
	}, map[string]any{
		"sub":            "subject",
		"email":          "member@example.com",
		"email_verified": false,
	})
	if err != nil {
		t.Fatalf("mappedOIDCProfile() error = %v", err)
	}
	if profile.EmailVerified {
		t.Fatal("mappedOIDCProfile() promoted an unverified email")
	}
}

func TestOIDCRejectsCrossOriginRedirectsAndReturnTargets(t *testing.T) {
	if _, err := normalizeOIDCReturnTo("//evil.example/callback", "chat.company.example"); err == nil {
		t.Fatal("normalizeOIDCReturnTo() accepted a protocol-relative target")
	}
	if target, err := normalizeOIDCReturnTo(
		"http://127.0.0.1:3000/chat",
		"127.0.0.1",
	); err != nil || target != "http://127.0.0.1:3000/chat" {
		t.Fatalf("normalizeOIDCReturnTo(local) = %q, %v", target, err)
	}
	if err := validateOIDCRedirectURI(
		"https://evil.example/api/v1/auth/oidc/callback",
		"chat.company.example",
	); err == nil {
		t.Fatal("validateOIDCRedirectURI() accepted a cross-origin callback")
	}
}

func TestOIDCSecretReferencesUseExplicitAllowlist(t *testing.T) {
	service := NewOIDCService(
		nil,
		nil,
		nil,
		"oidc-state-secret-with-at-least-32-characters",
		map[string]string{"company": "client-secret"},
	)
	allowed := "env://company"
	if secret, err := service.resolveClientSecret(&allowed); err != nil || secret != "client-secret" {
		t.Fatalf("resolveClientSecret(allowed) = %q, %v", secret, err)
	}
	blocked := "env://database_url"
	if _, err := service.resolveClientSecret(&blocked); err == nil {
		t.Fatal("resolveClientSecret() accepted an alias outside the allowlist")
	}
}
