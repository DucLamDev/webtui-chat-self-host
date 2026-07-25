package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	authdomain "github.com/duclamdev/application-chat/backend/internal/modules/auth/domain"
	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
	"github.com/duclamdev/application-chat/backend/internal/shared/securevalue"
)

const (
	oidcStateTTL          = 10 * time.Minute
	oidcResultTTL         = 2 * time.Minute
	oidcVerifierAADPrefix = "vpsttt-chat:oidc-pkce:v1:"
)

var oidcUsernameSanitizer = regexp.MustCompile(`[^a-z0-9_.-]+`)

var (
	ErrOIDCStateNotFound    = errors.New("oidc state not found")
	ErrOIDCResultNotFound   = errors.New("oidc login result not found")
	ErrOIDCIdentityNotFound = errors.New("oidc identity not found")
	ErrOIDCJITDisabled      = errors.New("oidc jit provisioning disabled")
	ErrOIDCProviderNotReady = errors.New("oidc provider not ready")
)

type OIDCRepository interface {
	ListPublicOIDCProviders(ctx context.Context, domain string) (ZoneAccess, []tenancydomain.OIDCProvider, error)
	ResolveOIDCProvider(ctx context.Context, domain string, providerID string) (ZoneAccess, tenancydomain.OIDCProvider, error)
	CreateOIDCLoginState(ctx context.Context, params CreateOIDCLoginStateParams) error
	ConsumeOIDCLoginState(ctx context.Context, stateHash []byte, now time.Time) (OIDCLoginState, error)
	CreateOIDCLoginResult(ctx context.Context, params CreateOIDCLoginResultParams) error
	ConsumeOIDCLoginResult(ctx context.Context, codeHash []byte, domain string, now time.Time) (OIDCLoginResult, error)
	ResolveOIDCUser(ctx context.Context, params ResolveOIDCUserParams) (authdomain.User, error)
}

type OIDCProtocol interface {
	AuthorizationURL(
		ctx context.Context,
		provider tenancydomain.OIDCProvider,
		clientSecret string,
		redirectURI string,
		state string,
		nonce string,
		codeVerifier string,
	) (string, error)
	ExchangeAndVerify(
		ctx context.Context,
		provider tenancydomain.OIDCProvider,
		clientSecret string,
		redirectURI string,
		code string,
		codeVerifier string,
		nonce string,
	) (map[string]any, error)
}

type OIDCService struct {
	base          *Service
	repo          OIDCRepository
	protocol      OIDCProtocol
	stateSecret   string
	clientSecrets map[string]string
	now           func() time.Time
}

type PublicOIDCProviderDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type OIDCStartInput struct {
	Domain      string
	ProviderID  string
	RedirectURI string
	ReturnTo    string
	DeviceName  string
	IPAddress   string
	UserAgent   string
}

type OIDCStartResult struct {
	AuthorizationURL string `json:"authorization_url"`
	ExpiresAt        string `json:"expires_at"`
}

type OIDCCallbackInput struct {
	State         string
	Code          string
	ProviderError string
}

type OIDCCallbackResult struct {
	RedirectURL string
}

type OIDCCompleteInput struct {
	Code       string
	Domain     string
	DeviceName string
	IPAddress  string
	UserAgent  string
}

type CreateOIDCLoginStateParams struct {
	StateHash             []byte
	ProviderID            string
	ZoneID                string
	Domain                string
	RedirectURI           string
	ReturnTo              string
	CodeVerifierEncrypted string
	Nonce                 string
	DeviceName            string
	IPAddress             string
	UserAgent             string
	ExpiresAt             time.Time
}

type OIDCLoginState struct {
	Provider              tenancydomain.OIDCProvider
	Target                ZoneAccess
	RedirectURI           string
	ReturnTo              string
	CodeVerifierEncrypted string
	Nonce                 string
	DeviceName            string
	IPAddress             string
	UserAgent             string
}

type CreateOIDCLoginResultParams struct {
	CodeHash   []byte
	ProviderID string
	ZoneID     string
	Domain     string
	Claims     map[string]any
	DeviceName string
	IPAddress  string
	UserAgent  string
	ExpiresAt  time.Time
}

type OIDCLoginResult struct {
	Provider   tenancydomain.OIDCProvider
	Target     ZoneAccess
	Claims     map[string]any
	DeviceName string
	IPAddress  string
	UserAgent  string
}

type ResolveOIDCUserParams struct {
	Provider      tenancydomain.OIDCProvider
	Target        ZoneAccess
	Subject       string
	Email         string
	EmailVerified bool
	Username      string
	DisplayName   string
	PasswordHash  string
	Claims        map[string]any
	SeenAt        time.Time
	DeviceName    string
	IPAddress     string
	UserAgent     string
}

func NewOIDCService(
	base *Service,
	repo OIDCRepository,
	protocol OIDCProtocol,
	stateSecret string,
	clientSecrets map[string]string,
) *OIDCService {
	secrets := make(map[string]string, len(clientSecrets))
	for name, value := range clientSecrets {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" && value != "" {
			secrets[name] = value
		}
	}
	return &OIDCService{
		base:          base,
		repo:          repo,
		protocol:      protocol,
		stateSecret:   strings.TrimSpace(stateSecret),
		clientSecrets: secrets,
		now:           time.Now,
	}
}

func (s *OIDCService) Enabled() bool {
	return s != nil && s.base != nil && s.repo != nil && s.protocol != nil &&
		len(s.stateSecret) >= 32
}

func (s *OIDCService) ListProviders(ctx context.Context, domain string) ([]PublicOIDCProviderDTO, error) {
	if !s.Enabled() {
		return []PublicOIDCProviderDTO{}, nil
	}
	_, providers, err := s.repo.ListPublicOIDCProviders(ctx, normalizeOIDCDomain(domain))
	if err != nil {
		return nil, mapOIDCError(err)
	}
	result := make([]PublicOIDCProviderDTO, 0, len(providers))
	for _, provider := range providers {
		if _, err := s.resolveClientSecret(provider.ClientSecretRef); err != nil {
			continue
		}
		result = append(result, PublicOIDCProviderDTO{ID: provider.ID, Name: provider.Name})
	}
	return result, nil
}

func (s *OIDCService) Start(ctx context.Context, input OIDCStartInput) (OIDCStartResult, error) {
	if !s.Enabled() {
		return OIDCStartResult{}, apperrors.ServiceUnavailable("OIDC_NOT_CONFIGURED", "OIDC SSO runtime chua duoc cau hinh.")
	}
	input.Domain = normalizeOIDCDomain(input.Domain)
	input.ProviderID = strings.TrimSpace(input.ProviderID)
	input.DeviceName, input.IPAddress, input.UserAgent = normalizeClientInfo(input.DeviceName, input.IPAddress, input.UserAgent)
	returnTo, err := normalizeOIDCReturnTo(input.ReturnTo, input.Domain)
	if err != nil {
		return OIDCStartResult{}, err
	}
	if err := validateOIDCRedirectURI(input.RedirectURI, input.Domain); err != nil {
		return OIDCStartResult{}, err
	}
	target, provider, err := s.repo.ResolveOIDCProvider(ctx, input.Domain, input.ProviderID)
	if err != nil {
		return OIDCStartResult{}, mapOIDCError(err)
	}
	clientSecret, err := s.resolveClientSecret(provider.ClientSecretRef)
	if err != nil {
		return OIDCStartResult{}, err
	}
	state, err := randomOIDCToken()
	if err != nil {
		return OIDCStartResult{}, apperrors.Internal("Khong tao duoc OIDC state.")
	}
	nonce, err := randomOIDCToken()
	if err != nil {
		return OIDCStartResult{}, apperrors.Internal("Khong tao duoc OIDC nonce.")
	}
	codeVerifier, err := randomOIDCToken()
	if err != nil {
		return OIDCStartResult{}, apperrors.Internal("Khong tao duoc PKCE verifier.")
	}
	authorizationURL, err := s.protocol.AuthorizationURL(
		ctx, provider, clientSecret, input.RedirectURI, state, nonce, codeVerifier,
	)
	if err != nil {
		return OIDCStartResult{}, apperrors.ServiceUnavailable("OIDC_PROVIDER_UNAVAILABLE", "Khong ket noi duoc OIDC provider.")
	}
	encryptedVerifier, err := securevalue.Encrypt(
		s.stateSecret,
		codeVerifier,
		oidcVerifierAADPrefix+provider.ID,
	)
	if err != nil {
		return OIDCStartResult{}, apperrors.Internal("Khong bao ve duoc PKCE verifier.")
	}
	expiresAt := s.now().UTC().Add(oidcStateTTL)
	if err := s.repo.CreateOIDCLoginState(ctx, CreateOIDCLoginStateParams{
		StateHash:             oidcTokenHash(state),
		ProviderID:            provider.ID,
		ZoneID:                target.ZoneID,
		Domain:                target.Domain,
		RedirectURI:           input.RedirectURI,
		ReturnTo:              returnTo,
		CodeVerifierEncrypted: encryptedVerifier,
		Nonce:                 nonce,
		DeviceName:            input.DeviceName,
		IPAddress:             input.IPAddress,
		UserAgent:             input.UserAgent,
		ExpiresAt:             expiresAt,
	}); err != nil {
		return OIDCStartResult{}, err
	}
	return OIDCStartResult{
		AuthorizationURL: authorizationURL,
		ExpiresAt:        expiresAt.Format(time.RFC3339),
	}, nil
}

func (s *OIDCService) Callback(ctx context.Context, input OIDCCallbackInput) (OIDCCallbackResult, error) {
	if !s.Enabled() {
		return OIDCCallbackResult{}, apperrors.ServiceUnavailable("OIDC_NOT_CONFIGURED", "OIDC SSO runtime chua duoc cau hinh.")
	}
	stateToken := strings.TrimSpace(input.State)
	if stateToken == "" {
		return OIDCCallbackResult{}, apperrors.BadRequest("INVALID_OIDC_STATE", "OIDC state khong hop le.")
	}
	state, err := s.repo.ConsumeOIDCLoginState(ctx, oidcTokenHash(stateToken), s.now().UTC())
	if err != nil {
		return OIDCCallbackResult{}, mapOIDCError(err)
	}
	if strings.TrimSpace(input.ProviderError) != "" {
		return OIDCCallbackResult{
			RedirectURL: appendOIDCQuery(state.ReturnTo, "oidc_error", "provider_denied"),
		}, nil
	}
	if strings.TrimSpace(input.Code) == "" {
		return OIDCCallbackResult{
			RedirectURL: appendOIDCQuery(state.ReturnTo, "oidc_error", "missing_code"),
		}, nil
	}
	verifier, err := securevalue.Decrypt(
		s.stateSecret,
		state.CodeVerifierEncrypted,
		oidcVerifierAADPrefix+state.Provider.ID,
	)
	if err != nil {
		return OIDCCallbackResult{}, apperrors.Unauthorized("OIDC state khong con hop le.")
	}
	clientSecret, err := s.resolveClientSecret(state.Provider.ClientSecretRef)
	if err != nil {
		return OIDCCallbackResult{}, err
	}
	claims, err := s.protocol.ExchangeAndVerify(
		ctx,
		state.Provider,
		clientSecret,
		state.RedirectURI,
		strings.TrimSpace(input.Code),
		verifier,
		state.Nonce,
	)
	if err != nil {
		return OIDCCallbackResult{
			RedirectURL: appendOIDCQuery(state.ReturnTo, "oidc_error", "verification_failed"),
		}, nil
	}
	if _, err := mappedOIDCProfile(state.Provider, claims); err != nil {
		return OIDCCallbackResult{
			RedirectURL: appendOIDCQuery(state.ReturnTo, "oidc_error", "claims_rejected"),
		}, nil
	}
	encodedClaims, err := json.Marshal(claims)
	if err != nil || len(encodedClaims) > 64*1024 {
		return OIDCCallbackResult{}, apperrors.BadRequest("INVALID_OIDC_CLAIMS", "OIDC claims khong hop le.")
	}
	loginCode, err := randomOIDCToken()
	if err != nil {
		return OIDCCallbackResult{}, apperrors.Internal("Khong tao duoc OIDC completion code.")
	}
	if err := s.repo.CreateOIDCLoginResult(ctx, CreateOIDCLoginResultParams{
		CodeHash:   oidcTokenHash(loginCode),
		ProviderID: state.Provider.ID,
		ZoneID:     state.Target.ZoneID,
		Domain:     state.Target.Domain,
		Claims:     claims,
		DeviceName: state.DeviceName,
		IPAddress:  state.IPAddress,
		UserAgent:  state.UserAgent,
		ExpiresAt:  s.now().UTC().Add(oidcResultTTL),
	}); err != nil {
		return OIDCCallbackResult{}, err
	}
	return OIDCCallbackResult{
		RedirectURL: appendOIDCQuery(state.ReturnTo, "oidc_code", loginCode),
	}, nil
}

func (s *OIDCService) Complete(ctx context.Context, input OIDCCompleteInput) (AuthResult, error) {
	if !s.Enabled() {
		return AuthResult{}, apperrors.ServiceUnavailable("OIDC_NOT_CONFIGURED", "OIDC SSO runtime chua duoc cau hinh.")
	}
	input.Code = strings.TrimSpace(input.Code)
	input.Domain = normalizeOIDCDomain(input.Domain)
	if input.Code == "" || input.Domain == "" {
		return AuthResult{}, apperrors.BadRequest("VALIDATION_ERROR", "OIDC completion code va domain la bat buoc.")
	}
	result, err := s.repo.ConsumeOIDCLoginResult(ctx, oidcTokenHash(input.Code), input.Domain, s.now().UTC())
	if err != nil {
		return AuthResult{}, mapOIDCError(err)
	}
	profile, err := mappedOIDCProfile(result.Provider, result.Claims)
	if err != nil {
		return AuthResult{}, err
	}
	password, err := randomOIDCToken()
	if err != nil {
		return AuthResult{}, apperrors.Internal("Khong tao duoc tai khoan OIDC.")
	}
	passwordHash, err := sharedauth.HashPassword(password)
	if err != nil {
		return AuthResult{}, apperrors.Internal("Khong tao duoc tai khoan OIDC.")
	}
	deviceName := firstNonEmpty(strings.TrimSpace(input.DeviceName), result.DeviceName)
	ipAddress := firstNonEmpty(strings.TrimSpace(input.IPAddress), result.IPAddress)
	userAgent := firstNonEmpty(strings.TrimSpace(input.UserAgent), result.UserAgent)
	user, err := s.repo.ResolveOIDCUser(ctx, ResolveOIDCUserParams{
		Provider:      result.Provider,
		Target:        result.Target,
		Subject:       profile.Subject,
		Email:         profile.Email,
		EmailVerified: profile.EmailVerified,
		Username:      oidcUsername(profile.Username, profile.Email, result.Provider.ID, profile.Subject),
		DisplayName:   profile.DisplayName,
		PasswordHash:  passwordHash,
		Claims:        result.Claims,
		SeenAt:        s.now().UTC(),
		DeviceName:    deviceName,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
	})
	if err != nil {
		return AuthResult{}, mapOIDCError(err)
	}
	if user.Status != "active" {
		return AuthResult{}, apperrors.Forbidden("Tai khoan chua san sang hoac da bi khoa.")
	}
	authResult, err := s.base.issueTokens(ctx, user, result.Target, deviceName, ipAddress, userAgent)
	if err != nil {
		return AuthResult{}, err
	}
	_ = s.base.repo.UpdateLastLoginInfo(ctx, UpdateLastLoginInfoParams{
		UserID: user.ID, SeenAt: s.now().UTC(), DeviceName: deviceName,
		IPAddress: ipAddress, UserAgent: userAgent,
	})
	_ = s.base.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: user.ID,
		ZoneID:      result.Target.ZoneID,
		WorkspaceID: result.Target.WorkspaceID,
		Action:      "auth.oidc_login",
		EntityType:  "user",
		EntityID:    user.ID,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Metadata: map[string]any{
			"provider_id": result.Provider.ID,
			"domain":      result.Target.Domain,
		},
	})
	return authResult, nil
}

type oidcProfile struct {
	Subject       string
	Email         string
	EmailVerified bool
	Username      string
	DisplayName   string
}

func mappedOIDCProfile(provider tenancydomain.OIDCProvider, claims map[string]any) (oidcProfile, error) {
	mapping := provider.ClaimMapping
	profile := oidcProfile{
		Subject:     oidcClaimString(claims, mapping, "subject", "sub"),
		Email:       strings.ToLower(oidcClaimString(claims, mapping, "email", "email")),
		Username:    oidcClaimString(claims, mapping, "username", "preferred_username"),
		DisplayName: oidcClaimString(claims, mapping, "display_name", "name"),
	}
	if profile.Subject == "" || len(profile.Subject) > 1000 {
		return oidcProfile{}, apperrors.BadRequest("INVALID_OIDC_CLAIMS", "OIDC subject khong hop le.")
	}
	if _, err := mail.ParseAddress(profile.Email); err != nil {
		return oidcProfile{}, apperrors.BadRequest("INVALID_OIDC_CLAIMS", "OIDC provider khong tra ve email hop le.")
	}
	profile.EmailVerified = oidcClaimBool(claims, mapping, "email_verified", "email_verified")
	if provider.RequireVerifiedEmail && !profile.EmailVerified {
		return oidcProfile{}, apperrors.Unauthorized("OIDC provider chua xac minh email.")
	}
	if profile.DisplayName == "" {
		profile.DisplayName = strings.Split(profile.Email, "@")[0]
	}
	return profile, nil
}

func oidcClaimString(claims map[string]any, mapping map[string]any, mappingKey string, fallback string) string {
	claimName := fallback
	if configured, ok := mapping[mappingKey].(string); ok && strings.TrimSpace(configured) != "" {
		claimName = strings.TrimSpace(configured)
	}
	value, _ := claims[claimName].(string)
	return strings.TrimSpace(value)
}

func oidcClaimBool(claims map[string]any, mapping map[string]any, mappingKey string, fallback string) bool {
	claimName := fallback
	if configured, ok := mapping[mappingKey].(string); ok && strings.TrimSpace(configured) != "" {
		claimName = strings.TrimSpace(configured)
	}
	value, _ := claims[claimName].(bool)
	return value
}

func (s *OIDCService) resolveClientSecret(ref *string) (string, error) {
	if ref == nil || strings.TrimSpace(*ref) == "" {
		return "", nil
	}
	value := strings.TrimSpace(*ref)
	if !strings.HasPrefix(value, "env://") {
		return "", apperrors.ServiceUnavailable("OIDC_SECRET_UNAVAILABLE", "OIDC client secret provider chua co runtime adapter.")
	}
	alias := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "env://")))
	secret := s.clientSecrets[alias]
	if alias == "" || secret == "" {
		return "", apperrors.ServiceUnavailable("OIDC_SECRET_UNAVAILABLE", "OIDC client secret alias khong nam trong allowlist.")
	}
	return secret, nil
}

func normalizeOIDCReturnTo(value string, domain string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || !strings.HasPrefix(parsed.Path, "/") ||
		strings.HasPrefix(parsed.Path, "//") {
		return "", apperrors.BadRequest("INVALID_RETURN_TO", "return_to phai la relative path cung origin.")
	}
	if parsed.IsAbs() || parsed.Host != "" {
		if !isLocalOIDCDomain(domain) || parsed.Scheme != "http" ||
			!strings.EqualFold(parsed.Hostname(), domain) {
			return "", apperrors.BadRequest("INVALID_RETURN_TO", "return_to phai la relative path cung origin.")
		}
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func validateOIDCRedirectURI(value string, domain string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" ||
		parsed.Fragment != "" || parsed.Path != "/api/v1/auth/oidc/callback" {
		return apperrors.BadRequest("INVALID_REDIRECT_URI", "OIDC redirect URI khong hop le.")
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLocalOIDCDomain(domain)) {
		return apperrors.BadRequest("INVALID_REDIRECT_URI", "OIDC redirect URI phai dung HTTPS.")
	}
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(parsed.Hostname())), []byte(domain)) != 1 &&
		!isLocalOIDCDomain(domain) {
		return apperrors.BadRequest("INVALID_REDIRECT_URI", "OIDC redirect URI khong khop domain.")
	}
	return nil
}

func normalizeOIDCDomain(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if parsed, err := url.Parse("//" + value); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return strings.TrimSuffix(value, ".")
}

func isLocalOIDCDomain(domain string) bool {
	return domain == "localhost" || strings.HasSuffix(domain, ".localhost") ||
		domain == "127.0.0.1" || domain == "::1"
}

func randomOIDCToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func oidcTokenHash(value string) []byte {
	hash := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hash[:]
}

func oidcUsername(configured string, email string, providerID string, subject string) string {
	base := strings.ToLower(strings.TrimSpace(configured))
	if base == "" {
		base = strings.Split(strings.ToLower(email), "@")[0]
	}
	base = strings.Trim(oidcUsernameSanitizer.ReplaceAllString(base, "-"), "._-")
	if len(base) < 3 {
		base = "oidc-user"
	}
	if len(base) > 28 {
		base = strings.Trim(base[:28], "._-")
	}
	digest := sha256.Sum256([]byte(providerID + ":" + subject))
	return base + "-" + base64.RawURLEncoding.EncodeToString(digest[:6])
}

func appendOIDCQuery(returnTo string, key string, value string) string {
	parsed, err := url.Parse(returnTo)
	if err != nil {
		return "/"
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mapOIDCError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrOIDCStateNotFound):
		return apperrors.BadRequest("INVALID_OIDC_STATE", "OIDC state da het han hoac da duoc su dung.")
	case errors.Is(err, ErrOIDCResultNotFound):
		return apperrors.BadRequest("INVALID_OIDC_CODE", "OIDC completion code da het han hoac da duoc su dung.")
	case errors.Is(err, ErrOIDCJITDisabled):
		return apperrors.Forbidden("OIDC provider khong cho phep tao thanh vien moi.")
	case errors.Is(err, ErrOIDCProviderNotReady), errors.Is(err, tenancydomain.ErrOIDCProviderNotFound):
		return apperrors.NotFound("OIDC_PROVIDER_NOT_FOUND", "Khong tim thay OIDC provider san sang cho domain.")
	case errors.Is(err, authdomain.ErrUserAlreadyExists):
		return apperrors.Conflict("OIDC_IDENTITY_CONFLICT", "OIDC identity xung dot voi tai khoan hien co.")
	case errors.Is(err, authdomain.ErrZoneAccessDenied):
		return apperrors.Forbidden("Tai khoan khong co quyen truy cap zone nay.")
	default:
		return err
	}
}
