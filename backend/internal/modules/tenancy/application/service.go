package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	webhooksecurity "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/security"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

var fqdnPattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
var slugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)
var oidcSecretAliasPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

const (
	defaultVerificationTTL = 24 * time.Hour
	dnsChallengePrefix     = "vpsttt-chat-verification="
)

type Repository interface {
	ResolveByDomain(ctx context.Context, domain string) (tenancydomain.ResolvedZone, error)
	WorkspaceBelongsToZone(ctx context.Context, workspaceID string, zoneID string) (bool, error)
	ZoneDomainBelongsToActiveZone(ctx context.Context, zoneID string, domain string) (bool, error)
	ZoneDomainBelongsToRecoverableZone(ctx context.Context, zoneID string, domain string) (bool, error)
	CreateDomainClaim(ctx context.Context, params CreateDomainClaimParams) (tenancydomain.DomainClaim, error)
	CreateAdditionalDomainClaim(ctx context.Context, params CreateAdditionalDomainClaimParams) (tenancydomain.DomainClaim, error)
	FindDomainClaim(ctx context.Context, domainID string, actorUserID string) (tenancydomain.DomainClaim, error)
	RecordDomainVerificationFailure(ctx context.Context, domainID string, actorUserID string, checkedAt time.Time, reason string) error
	ProvisionVerifiedDomain(ctx context.Context, params ProvisionVerifiedDomainParams) (tenancydomain.ResolvedZone, error)
	ActivateVerifiedDomain(ctx context.Context, params ProvisionVerifiedDomainParams) (tenancydomain.ResolvedZone, error)
	CanManageZone(ctx context.Context, zoneID string, userID string) (bool, error)
	GetZoneAdminOverview(ctx context.Context, zoneID string) (tenancydomain.ZoneAdminOverview, error)
	UpdateZoneSettings(ctx context.Context, params UpdateZoneSettingsParams) (tenancydomain.ZoneAdminOverview, error)
	SetZoneLifecycle(ctx context.Context, params SetZoneLifecycleParams) error
	SetPrimaryDomain(ctx context.Context, zoneID string, domainID string, actorUserID string) error
	DeleteZoneDomain(ctx context.Context, zoneID string, domainID string, actorUserID string) error
	ListDeploymentRequests(ctx context.Context, zoneID string) ([]tenancydomain.DeploymentRequest, error)
	CreateDeploymentRequest(ctx context.Context, params CreateDeploymentRequestParams) (tenancydomain.DeploymentRequest, error)
	GetZoneQuota(ctx context.Context, zoneID string) (tenancydomain.ZoneQuota, tenancydomain.ZoneUsage, error)
	UpdateZoneQuota(ctx context.Context, params UpdateZoneQuotaParams) (tenancydomain.ZoneQuota, tenancydomain.ZoneUsage, error)
	ListOIDCProviders(ctx context.Context, zoneID string) ([]tenancydomain.OIDCProvider, error)
	CreateOIDCProvider(ctx context.Context, params CreateOIDCProviderParams) (tenancydomain.OIDCProvider, error)
	UpdateOIDCProvider(ctx context.Context, params UpdateOIDCProviderParams) (tenancydomain.OIDCProvider, error)
	DeleteOIDCProvider(ctx context.Context, zoneID string, providerID string, actorUserID string) error
	ListAutomationTemplates(ctx context.Context, zoneID string) ([]tenancydomain.AutomationTemplate, error)
	GetAutomationTemplate(ctx context.Context, zoneID string, templateKey string) (tenancydomain.AutomationTemplate, error)
	ListAutomationInstallations(ctx context.Context, zoneID string) ([]tenancydomain.AutomationInstallation, error)
	CreateAutomationInstallation(ctx context.Context, params CreateAutomationInstallationParams) (tenancydomain.AutomationInstallation, error)
	UpdateAutomationInstallation(ctx context.Context, params UpdateAutomationInstallationParams) (tenancydomain.AutomationInstallation, error)
	DeleteAutomationInstallation(ctx context.Context, zoneID string, installationID string, actorUserID string) error
}

type TXTResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

type Options struct {
	AppName              string
	AppVersion           string
	DefaultLocale        string
	ReleaseChannel       string
	DeploymentMode       string
	RTCICEServers        []map[string]any
	RoutingDNSType       string
	RoutingDNSTarget     string
	VerificationTTL      time.Duration
	TXTResolver          TXTResolver
	Now                  func() time.Time
	WebhookSigningSecret string
	OIDCEnabled          bool
	OIDCClientSecrets    map[string]string
}

type Service struct {
	repo            Repository
	options         Options
	now             func() time.Time
	txtResolver     TXTResolver
	verificationTTL time.Duration
}

type CreateDomainClaimInput struct {
	ActorUserID      string
	Domain           string
	ZoneName         string
	RegistrationMode string
}

type CreateDomainClaimParams struct {
	ActorUserID      string
	Domain           string
	ZoneSlug         string
	ZoneName         string
	RegistrationMode string
	ExpiresAt        time.Time
}

type CreateAdditionalDomainClaimParams struct {
	ActorUserID string
	ZoneID      string
	Domain      string
	Kind        string
	ExpiresAt   time.Time
}

type ProvisionVerifiedDomainParams struct {
	ActorUserID string
	DomainID    string
	VerifiedAt  time.Time
}

type UpdateZoneSettingsParams struct {
	ActorUserID      string
	ZoneID           string
	Name             *string
	RegistrationMode *string
}

type SetZoneLifecycleParams struct {
	ActorUserID string
	ZoneID      string
	Action      string
	Reason      string
}

type CreateDeploymentRequestParams struct {
	ActorUserID           string
	ZoneID                string
	RequestedMode         string
	RequestedDatabaseMode string
	IdempotencyKey        string
}

type UpdateZoneQuotaParams struct {
	ActorUserID                string
	ZoneID                     string
	MaxWorkspaces              int
	MaxMembers                 int
	MaxStorageBytes            int64
	MaxAutomationInstallations int
	MaxWebhooks                int
	EnforcementMode            string
}

type CreateOIDCProviderParams struct {
	ActorUserID          string
	ZoneID               string
	Name                 string
	IssuerURL            string
	ClientID             string
	ClientSecretRef      string
	Scopes               []string
	ClaimMapping         map[string]any
	JITProvisioning      bool
	RequireVerifiedEmail bool
	Status               string
}

type UpdateOIDCProviderParams struct {
	ActorUserID          string
	ZoneID               string
	ProviderID           string
	Name                 *string
	IssuerURL            *string
	ClientID             *string
	ClientSecretRef      *string
	Scopes               *[]string
	ClaimMapping         *map[string]any
	JITProvisioning      *bool
	RequireVerifiedEmail *bool
	Status               *string
}

type CreateAutomationInstallationInput struct {
	ActorUserID string
	ZoneID      string
	WorkspaceID string
	TemplateKey string
	Name        string
	Status      string
	Config      map[string]any
	SecretRef   string
}

type CreateAutomationInstallationParams struct {
	ActorUserID            string
	ZoneID                 string
	WorkspaceID            string
	TemplateKey            string
	Name                   string
	Status                 string
	Config                 map[string]any
	SecretRef              string
	RuntimeTargetURL       string
	RuntimeEventTypes      []string
	RuntimeSecretEncrypted string
}

type UpdateAutomationInstallationInput struct {
	ActorUserID    string
	ZoneID         string
	InstallationID string
	Name           *string
	Status         *string
	Config         *map[string]any
	SecretRef      *string
}

type UpdateAutomationInstallationParams struct {
	ActorUserID    string
	ZoneID         string
	InstallationID string
	Name           *string
	Status         *string
	Config         *map[string]any
	SecretRef      *string
}

type DiscoveryDTO struct {
	Version      string           `json:"version"`
	Domain       string           `json:"domain"`
	Zone         ZoneDTO          `json:"zone"`
	Workspace    *WorkspaceRefDTO `json:"workspace,omitempty"`
	Runtime      RuntimeDTO       `json:"runtime"`
	Capabilities CapabilitiesDTO  `json:"capabilities"`
	Deployment   DeploymentDTO    `json:"deployment"`
}

type ZoneDTO struct {
	ID               string `json:"id"`
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	Status           string `json:"status"`
	RegistrationMode string `json:"registration_mode"`
}

type WorkspaceRefDTO struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type RuntimeDTO struct {
	AppName        string           `json:"app_name"`
	AppVersion     string           `json:"app_version"`
	ReleaseChannel string           `json:"release_channel"`
	Locale         string           `json:"locale"`
	WebBaseURL     string           `json:"web_base_url"`
	APIBaseURL     string           `json:"api_base_url"`
	WSBaseURL      string           `json:"ws_base_url"`
	AdminBaseURL   string           `json:"admin_base_url,omitempty"`
	RTCICEServers  []map[string]any `json:"rtc_ice_servers,omitempty"`
}

type CapabilitiesDTO struct {
	Chat              bool `json:"chat"`
	Files             bool `json:"files"`
	Calls             bool `json:"calls"`
	Bots              bool `json:"bots"`
	Automation        bool `json:"automation"`
	Webhooks          bool `json:"webhooks"`
	Federation        bool `json:"federation"`
	SSO               bool `json:"sso"`
	OIDCConfiguration bool `json:"oidc_configuration"`
	CustomDomain      bool `json:"custom_domain"`
	Dedicated         bool `json:"dedicated"`
	SelfHosted        bool `json:"self_hosted"`
}

type DeploymentDTO struct {
	Mode         string `json:"mode"`
	DatabaseMode string `json:"database_mode"`
	Status       string `json:"status"`
}

type ZoneContextDTO struct {
	ZoneID      string `json:"zone_id"`
	ZoneSlug    string `json:"zone_slug"`
	ZoneKind    string `json:"zone_kind"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Domain      string `json:"domain"`
}

type DomainClaimDTO struct {
	ID                    string           `json:"id"`
	Domain                string           `json:"domain"`
	Status                string           `json:"status"`
	RoutingDNSType        string           `json:"routing_dns_type,omitempty"`
	RoutingDNSName        string           `json:"routing_dns_name,omitempty"`
	RoutingDNSValue       string           `json:"routing_dns_value,omitempty"`
	VerificationMethod    string           `json:"verification_method"`
	VerificationDNSName   string           `json:"verification_dns_name"`
	VerificationDNSValue  string           `json:"verification_dns_value"`
	VerificationExpiresAt *string          `json:"verification_expires_at,omitempty"`
	VerificationAttempts  int              `json:"verification_attempts"`
	LastVerificationError *string          `json:"last_verification_error,omitempty"`
	LastCheckedAt         *string          `json:"last_checked_at,omitempty"`
	Zone                  ZoneDTO          `json:"zone"`
	Workspace             *WorkspaceRefDTO `json:"workspace,omitempty"`
}

type AutomationTemplateDTO struct {
	ID             string         `json:"id"`
	Key            string         `json:"key"`
	Name           string         `json:"name"`
	Description    *string        `json:"description,omitempty"`
	ZoneKind       string         `json:"zone_kind"`
	TemplateType   string         `json:"template_type"`
	RuntimeKind    string         `json:"runtime_kind"`
	ConfigSchema   map[string]any `json:"config_schema"`
	DefaultConfig  map[string]any `json:"default_config"`
	RequiredScopes []string       `json:"required_scopes"`
	Status         string         `json:"status"`
}

type AutomationInstallationDTO struct {
	ID               string         `json:"id"`
	ZoneID           string         `json:"zone_id"`
	WorkspaceID      *string        `json:"workspace_id,omitempty"`
	TemplateID       *string        `json:"template_id,omitempty"`
	TemplateKey      *string        `json:"template_key,omitempty"`
	Name             string         `json:"name"`
	Status           string         `json:"status"`
	Config           map[string]any `json:"config"`
	HasSecretRef     bool           `json:"has_secret_ref"`
	RuntimeWebhookID *string        `json:"runtime_webhook_id,omitempty"`
	RuntimeReady     bool           `json:"runtime_ready"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
}

type CreatedAutomationInstallationDTO struct {
	AutomationInstallationDTO
	RuntimeSecret string `json:"runtime_secret,omitempty"`
}

func NewService(repo Repository, options Options) *Service {
	if strings.TrimSpace(options.AppName) == "" {
		options.AppName = "WebTui Chat"
	}
	if strings.TrimSpace(options.AppVersion) == "" {
		options.AppVersion = "dev"
	}
	if strings.TrimSpace(options.DefaultLocale) == "" {
		options.DefaultLocale = "vi-VN"
	}
	if strings.TrimSpace(options.ReleaseChannel) == "" {
		options.ReleaseChannel = "stable"
	}
	if options.VerificationTTL <= 0 {
		options.VerificationTTL = defaultVerificationTTL
	}
	if options.TXTResolver == nil {
		options.TXTResolver = net.DefaultResolver
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Service{
		repo:            repo,
		options:         options,
		now:             options.Now,
		txtResolver:     options.TXTResolver,
		verificationTTL: options.VerificationTTL,
	}
}

func (s *Service) Discover(ctx context.Context, rawDomain string) (DiscoveryDTO, error) {
	domain, err := NormalizeDomain(rawDomain)
	if err != nil {
		return DiscoveryDTO{}, apperrors.BadRequest("INVALID_DOMAIN", "Domain khong dung dinh dang.")
	}
	resolved, err := s.repo.ResolveByDomain(ctx, domain)
	if err != nil {
		if errors.Is(err, tenancydomain.ErrZoneNotFound) {
			return DiscoveryDTO{}, apperrors.NotFound("ZONE_NOT_FOUND", "Domain chua duoc cau hinh thanh zone chat.")
		}
		return DiscoveryDTO{}, err
	}
	return s.toDiscovery(ctx, domain, resolved), nil
}

func (s *Service) CertificateDomainAllowed(ctx context.Context, rawDomain string) bool {
	domain, err := NormalizeDomain(rawDomain)
	if err != nil || net.ParseIP(domain) != nil || !fqdnPattern.MatchString(domain) {
		return false
	}
	resolved, err := s.repo.ResolveByDomain(ctx, domain)
	return err == nil && resolved.Zone.Status == "active" && resolved.Domain.Status == "active"
}

func (s *Service) ResolveContext(ctx context.Context, rawHost string) (ZoneContextDTO, error) {
	discovery, err := s.Discover(ctx, rawHost)
	if err != nil {
		return ZoneContextDTO{}, err
	}
	workspaceID := ""
	if discovery.Workspace != nil {
		workspaceID = discovery.Workspace.ID
	}
	return ZoneContextDTO{
		ZoneID:      discovery.Zone.ID,
		ZoneSlug:    discovery.Zone.Slug,
		ZoneKind:    discovery.Zone.Kind,
		WorkspaceID: workspaceID,
		Domain:      discovery.Domain,
	}, nil
}

func (s *Service) WorkspaceBelongsToZone(ctx context.Context, workspaceID string, zoneID string) (bool, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	zoneID = strings.TrimSpace(zoneID)
	if workspaceID == "" || zoneID == "" {
		return false, nil
	}
	return s.repo.WorkspaceBelongsToZone(ctx, workspaceID, zoneID)
}

func (s *Service) ZoneDomainBelongsToActiveZone(ctx context.Context, zoneID string, rawDomain string) (bool, error) {
	zoneID = strings.TrimSpace(zoneID)
	domain, err := NormalizeDomain(rawDomain)
	if err != nil || zoneID == "" {
		return false, nil
	}
	return s.repo.ZoneDomainBelongsToActiveZone(ctx, zoneID, domain)
}

func (s *Service) ZoneDomainBelongsToRecoverableZone(ctx context.Context, zoneID string, rawDomain string) (bool, error) {
	zoneID = strings.TrimSpace(zoneID)
	domain, err := NormalizeDomain(rawDomain)
	if err != nil || zoneID == "" {
		return false, nil
	}
	return s.repo.ZoneDomainBelongsToRecoverableZone(ctx, zoneID, domain)
}

func (s *Service) CreateDomainClaim(ctx context.Context, input CreateDomainClaimInput) (DomainClaimDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneName = strings.TrimSpace(input.ZoneName)
	input.RegistrationMode = strings.ToLower(strings.TrimSpace(input.RegistrationMode))
	if input.ActorUserID == "" {
		return DomainClaimDTO{}, apperrors.Unauthorized("Ban can dang nhap de claim domain.")
	}
	domain, err := NormalizeDomain(input.Domain)
	if err != nil || !fqdnPattern.MatchString(domain) {
		return DomainClaimDTO{}, apperrors.BadRequest("INVALID_DOMAIN", "Chi ho tro domain cong khai hop le de tao zone.")
	}
	if input.ZoneName == "" || len([]rune(input.ZoneName)) > 120 {
		return DomainClaimDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Ten zone phai dai tu 1 den 120 ky tu.")
	}
	if input.RegistrationMode == "" {
		input.RegistrationMode = "invite_only"
	}
	switch input.RegistrationMode {
	case "open", "invite_only", "closed":
	default:
		return DomainClaimDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "registration_mode khong hop le.")
	}

	claim, err := s.repo.CreateDomainClaim(ctx, CreateDomainClaimParams{
		ActorUserID:      input.ActorUserID,
		Domain:           domain,
		ZoneSlug:         zoneSlug(domain),
		ZoneName:         input.ZoneName,
		RegistrationMode: input.RegistrationMode,
		ExpiresAt:        s.now().UTC().Add(s.verificationTTL),
	})
	if err != nil {
		if errors.Is(err, tenancydomain.ErrDomainAlreadyClaimed) {
			return DomainClaimDTO{}, apperrors.Conflict("DOMAIN_ALREADY_CLAIMED", "Domain da duoc claim boi mot zone khac.")
		}
		return DomainClaimDTO{}, err
	}
	return toDomainClaimDTO(claim, s.options), nil
}

func (s *Service) GetDomainClaim(ctx context.Context, actorUserID string, domainID string) (DomainClaimDTO, error) {
	claim, err := s.repo.FindDomainClaim(ctx, strings.TrimSpace(domainID), strings.TrimSpace(actorUserID))
	if err != nil {
		return DomainClaimDTO{}, mapClaimError(err)
	}
	return toDomainClaimDTO(claim, s.options), nil
}

func (s *Service) VerifyDomainClaim(ctx context.Context, actorUserID string, domainID string) (DiscoveryDTO, error) {
	actorUserID = strings.TrimSpace(actorUserID)
	domainID = strings.TrimSpace(domainID)
	claim, err := s.repo.FindDomainClaim(ctx, domainID, actorUserID)
	if err != nil {
		return DiscoveryDTO{}, mapClaimError(err)
	}
	if claim.Domain.Status == "active" || claim.Domain.Status == "verified" {
		resolved, resolveErr := s.repo.ResolveByDomain(ctx, claim.Domain.Domain)
		if resolveErr != nil {
			return DiscoveryDTO{}, resolveErr
		}
		return s.toDiscovery(ctx, claim.Domain.Domain, resolved), nil
	}

	now := s.now().UTC()
	if claim.Domain.VerificationExpiresAt == nil || !claim.Domain.VerificationExpiresAt.After(now) {
		return DiscoveryDTO{}, apperrors.Conflict("DOMAIN_VERIFICATION_EXPIRED", "Ma xac minh domain da het han. Hay claim lai domain.")
	}

	txtName := verificationDNSName(claim.Domain.Domain)
	expected := verificationDNSValue(claim.Domain.VerificationToken)
	records, lookupErr := s.txtResolver.LookupTXT(ctx, txtName)
	if lookupErr != nil || !containsTXTRecord(records, expected) {
		reason := "dns_txt_not_found"
		if lookupErr != nil {
			reason = "dns_lookup_failed"
		}
		_ = s.repo.RecordDomainVerificationFailure(ctx, domainID, actorUserID, now, reason)
		err := apperrors.Conflict("DOMAIN_VERIFICATION_FAILED", "Chua tim thay DNS TXT xac minh domain.")
		err.Details = map[string]any{
			"dns_name":  txtName,
			"dns_value": expected,
			"reason":    reason,
		}
		return DiscoveryDTO{}, err
	}

	params := ProvisionVerifiedDomainParams{
		ActorUserID: actorUserID,
		DomainID:    domainID,
		VerifiedAt:  now,
	}
	var resolved tenancydomain.ResolvedZone
	if claim.Zone.Status == "active" && claim.Workspace != nil {
		resolved, err = s.repo.ActivateVerifiedDomain(ctx, params)
	} else {
		resolved, err = s.repo.ProvisionVerifiedDomain(ctx, params)
	}
	if err != nil {
		return DiscoveryDTO{}, mapClaimError(err)
	}
	return s.toDiscovery(ctx, claim.Domain.Domain, resolved), nil
}

func (s *Service) ListAutomationTemplates(ctx context.Context, actorUserID string, zoneID string) ([]AutomationTemplateDTO, error) {
	if err := s.ensureZoneManager(ctx, actorUserID, zoneID); err != nil {
		return nil, err
	}
	templates, err := s.repo.ListAutomationTemplates(ctx, strings.TrimSpace(zoneID))
	if err != nil {
		return nil, err
	}
	result := make([]AutomationTemplateDTO, 0, len(templates))
	for _, template := range templates {
		result = append(result, toAutomationTemplateDTO(template))
	}
	return result, nil
}

func (s *Service) ListAutomationInstallations(ctx context.Context, actorUserID string, zoneID string) ([]AutomationInstallationDTO, error) {
	if err := s.ensureZoneManager(ctx, actorUserID, zoneID); err != nil {
		return nil, err
	}
	installations, err := s.repo.ListAutomationInstallations(ctx, strings.TrimSpace(zoneID))
	if err != nil {
		return nil, err
	}
	result := make([]AutomationInstallationDTO, 0, len(installations))
	for _, installation := range installations {
		result = append(result, toAutomationInstallationDTO(installation))
	}
	return result, nil
}

func (s *Service) CreateAutomationInstallation(ctx context.Context, input CreateAutomationInstallationInput) (CreatedAutomationInstallationDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.TemplateKey = strings.ToLower(strings.TrimSpace(input.TemplateKey))
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.SecretRef = strings.TrimSpace(input.SecretRef)
	if err := s.ensureZoneManager(ctx, input.ActorUserID, input.ZoneID); err != nil {
		return CreatedAutomationInstallationDTO{}, err
	}
	if input.TemplateKey == "" || input.Name == "" || len([]rune(input.Name)) > 120 {
		return CreatedAutomationInstallationDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "template_key va name hop le la bat buoc.")
	}
	if input.Status == "" {
		input.Status = "disabled"
	}
	if input.Status != "enabled" && input.Status != "disabled" {
		return CreatedAutomationInstallationDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "status automation khong hop le.")
	}
	if containsInlineSecret(input.Config) {
		return CreatedAutomationInstallationDTO{}, apperrors.BadRequest(
			"INLINE_SECRET_FORBIDDEN",
			"Khong luu secret, token, password hoac API key truc tiep trong config. Hay dung secret_ref.",
		)
	}
	if input.SecretRef != "" && !validSecretRef(input.SecretRef) {
		return CreatedAutomationInstallationDTO{}, apperrors.BadRequest(
			"INVALID_SECRET_REF",
			"secret_ref phai dung mot provider duoc ho tro.",
		)
	}

	template, err := s.repo.GetAutomationTemplate(ctx, input.ZoneID, input.TemplateKey)
	if err != nil {
		return CreatedAutomationInstallationDTO{}, mapAutomationError(err)
	}
	runtimeSecret := ""
	runtimeTargetURL := ""
	runtimeEventTypes := []string{}
	runtimeSecretEncrypted := ""
	if template.RuntimeKind == "outgoing_webhook" {
		runtimeTargetURL, runtimeEventTypes, err = automationWebhookRuntime(input.Config, template.DefaultConfig)
		if err != nil {
			return CreatedAutomationInstallationDTO{}, err
		}
		runtimeSecret, err = newAutomationSecret()
		if err != nil {
			return CreatedAutomationInstallationDTO{}, apperrors.Internal("Khong tao duoc signing secret cho automation.")
		}
		runtimeSecretEncrypted, err = webhooksecurity.EncryptSecret(s.options.WebhookSigningSecret, runtimeSecret)
		if err != nil {
			return CreatedAutomationInstallationDTO{}, apperrors.Internal(
				"Khong ma hoa duoc signing secret automation. Kiem tra WEBHOOK_SIGNING_SECRET.",
			)
		}
	}

	installation, err := s.repo.CreateAutomationInstallation(ctx, CreateAutomationInstallationParams{
		ActorUserID:            input.ActorUserID,
		ZoneID:                 input.ZoneID,
		WorkspaceID:            input.WorkspaceID,
		TemplateKey:            input.TemplateKey,
		Name:                   input.Name,
		Status:                 input.Status,
		Config:                 cloneMap(input.Config),
		SecretRef:              input.SecretRef,
		RuntimeTargetURL:       runtimeTargetURL,
		RuntimeEventTypes:      runtimeEventTypes,
		RuntimeSecretEncrypted: runtimeSecretEncrypted,
	})
	if err != nil {
		return CreatedAutomationInstallationDTO{}, mapAutomationError(err)
	}
	return CreatedAutomationInstallationDTO{
		AutomationInstallationDTO: toAutomationInstallationDTO(installation),
		RuntimeSecret:             runtimeSecret,
	}, nil
}

func (s *Service) UpdateAutomationInstallation(ctx context.Context, input UpdateAutomationInstallationInput) (AutomationInstallationDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	input.InstallationID = strings.TrimSpace(input.InstallationID)
	if err := s.ensureZoneManager(ctx, input.ActorUserID, input.ZoneID); err != nil {
		return AutomationInstallationDTO{}, err
	}
	if input.InstallationID == "" {
		return AutomationInstallationDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "installation_id la bat buoc.")
	}
	if input.Name == nil && input.Status == nil && input.Config == nil && input.SecretRef == nil {
		return AutomationInstallationDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Can cung cap it nhat mot truong de cap nhat.")
	}
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if value == "" || len([]rune(value)) > 120 {
			return AutomationInstallationDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "name automation phai dai tu 1 den 120 ky tu.")
		}
		input.Name = &value
	}
	if input.Status != nil {
		value := strings.ToLower(strings.TrimSpace(*input.Status))
		if value != "enabled" && value != "disabled" {
			return AutomationInstallationDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "status automation khong hop le.")
		}
		input.Status = &value
	}
	if input.Config != nil {
		config := cloneMap(*input.Config)
		if containsInlineSecret(config) {
			return AutomationInstallationDTO{}, apperrors.BadRequest(
				"INLINE_SECRET_FORBIDDEN",
				"Khong luu secret, token, password hoac API key truc tiep trong config. Hay dung secret_ref.",
			)
		}
		input.Config = &config
	}
	if input.SecretRef != nil {
		value := strings.TrimSpace(*input.SecretRef)
		if value != "" && !validSecretRef(value) {
			return AutomationInstallationDTO{}, apperrors.BadRequest(
				"INVALID_SECRET_REF",
				"secret_ref phai dung mot provider duoc ho tro.",
			)
		}
		input.SecretRef = &value
	}

	installation, err := s.repo.UpdateAutomationInstallation(ctx, UpdateAutomationInstallationParams{
		ActorUserID:    input.ActorUserID,
		ZoneID:         input.ZoneID,
		InstallationID: input.InstallationID,
		Name:           input.Name,
		Status:         input.Status,
		Config:         input.Config,
		SecretRef:      input.SecretRef,
	})
	if err != nil {
		return AutomationInstallationDTO{}, mapAutomationError(err)
	}
	return toAutomationInstallationDTO(installation), nil
}

func (s *Service) DeleteAutomationInstallation(
	ctx context.Context,
	actorUserID string,
	zoneID string,
	installationID string,
) error {
	actorUserID = strings.TrimSpace(actorUserID)
	zoneID = strings.TrimSpace(zoneID)
	installationID = strings.TrimSpace(installationID)
	if err := s.ensureZoneManager(ctx, actorUserID, zoneID); err != nil {
		return err
	}
	if installationID == "" {
		return apperrors.BadRequest("VALIDATION_ERROR", "installation_id la bat buoc.")
	}
	if err := s.repo.DeleteAutomationInstallation(ctx, zoneID, installationID, actorUserID); err != nil {
		return mapAutomationError(err)
	}
	return nil
}

func mapAutomationError(err error) error {
	switch {
	case errors.Is(err, tenancydomain.ErrAutomationTemplateNotFound):
		return apperrors.NotFound("AUTOMATION_TEMPLATE_NOT_FOUND", "Khong tim thay template automation phu hop voi zone.")
	case errors.Is(err, tenancydomain.ErrAutomationInstallationNotFound):
		return apperrors.NotFound("AUTOMATION_INSTALLATION_NOT_FOUND", "Khong tim thay automation installation trong zone hien tai.")
	case errors.Is(err, tenancydomain.ErrAutomationInstallationConflict):
		return apperrors.Conflict("AUTOMATION_INSTALLATION_EXISTS", "Ten automation da ton tai trong zone.")
	case errors.Is(err, tenancydomain.ErrAutomationConfigInvalid):
		return apperrors.BadRequest("AUTOMATION_CONFIG_INVALID", err.Error())
	case errors.Is(err, tenancydomain.ErrWorkspaceZoneMismatch):
		return apperrors.BadRequest("WORKSPACE_ZONE_MISMATCH", "Workspace khong thuoc zone hien tai.")
	case errors.Is(err, tenancydomain.ErrZoneQuotaExceeded):
		return apperrors.Conflict("ZONE_QUOTA_EXCEEDED", "Zone da dat gioi han automation installation.")
	default:
		return err
	}
}

func (s *Service) ensureZoneManager(ctx context.Context, actorUserID string, zoneID string) error {
	actorUserID = strings.TrimSpace(actorUserID)
	zoneID = strings.TrimSpace(zoneID)
	if actorUserID == "" {
		return apperrors.Unauthorized("Ban can dang nhap de quan ly zone.")
	}
	if zoneID == "" {
		return apperrors.BadRequest("ZONE_REQUIRED", "Khong xac dinh duoc zone hien tai.")
	}
	allowed, err := s.repo.CanManageZone(ctx, zoneID, actorUserID)
	if err != nil {
		return err
	}
	if !allowed {
		return apperrors.Forbidden("Ban khong co quyen quan ly zone nay.")
	}
	return nil
}

func NormalizeDomain(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", tenancydomain.ErrDomainInvalid
	}

	host := value
	if strings.Contains(host, "://") {
		parsed, err := url.Parse(host)
		if err != nil {
			return "", tenancydomain.ErrDomainInvalid
		}
		host = parsed.Host
	} else if slash := strings.Index(host, "/"); slash >= 0 {
		host = host[:slash]
	}

	if strings.Contains(host, "@") {
		return "", tenancydomain.ErrDomainInvalid
	}
	host = strings.TrimSuffix(host, ".")
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	} else if colon := strings.LastIndex(host, ":"); colon > -1 {
		if _, parseErr := strconv.Atoi(host[colon+1:]); parseErr == nil {
			host = host[:colon]
		}
	}
	host = strings.Trim(host, "[]")

	if host == "localhost" {
		return host, nil
	}
	if net.ParseIP(host) != nil {
		return host, nil
	}
	if len(host) > 253 || strings.ContainsAny(host, " _*") || !fqdnPattern.MatchString(host) {
		return "", tenancydomain.ErrDomainInvalid
	}
	return host, nil
}

func zoneSlug(domain string) string {
	base := strings.Trim(slugSanitizer.ReplaceAllString(strings.ToLower(domain), "-"), "-")
	if len(base) > 50 {
		base = strings.Trim(base[:50], "-")
	}
	digest := sha256.Sum256([]byte(domain))
	return base + "-" + hex.EncodeToString(digest[:4])
}

func verificationDNSName(domain string) string {
	return "_vpsttt-chat." + domain
}

func verificationDNSValue(token string) string {
	return dnsChallengePrefix + strings.TrimSpace(token)
}

func containsTXTRecord(records []string, expected string) bool {
	for _, record := range records {
		if strings.TrimSpace(record) == expected {
			return true
		}
	}
	return false
}

func (s *Service) toDiscovery(
	ctx context.Context,
	domain string,
	resolved tenancydomain.ResolvedZone,
) DiscoveryDTO {
	runtime := s.runtime(domain, resolved.Deployment)
	if name := strings.TrimSpace(resolved.Zone.Name); name != "" {
		runtime.AppName = name
	}
	capabilities := capabilitiesFromMetadata(resolved.Zone.Metadata)
	// Federation remains fail-closed until its runtime protocol is deployed.
	capabilities.Federation = false
	capabilities.SSO = s.zoneHasReadyOIDCProvider(ctx, resolved.Zone)
	capabilities.OIDCConfiguration = true
	if resolved.Zone.Status != "active" {
		capabilities.Chat = false
		capabilities.Files = false
		capabilities.Calls = false
		capabilities.Bots = false
		capabilities.Automation = false
		capabilities.Webhooks = false
	}
	if resolved.Zone.Kind == "customer_dedicated" || deploymentMode(resolved.Deployment) != "shared" {
		capabilities.Dedicated = true
	}
	capabilities.SelfHosted = strings.EqualFold(strings.TrimSpace(s.options.DeploymentMode), "self_hosted")
	capabilities.CustomDomain = !capabilities.SelfHosted

	var workspace *WorkspaceRefDTO
	if resolved.Workspace != nil {
		workspace = &WorkspaceRefDTO{
			ID:   resolved.Workspace.ID,
			Slug: resolved.Workspace.Slug,
			Name: resolved.Workspace.Name,
		}
	}

	return DiscoveryDTO{
		Version:   "1",
		Domain:    domain,
		Workspace: workspace,
		Zone: ZoneDTO{
			ID:               resolved.Zone.ID,
			Slug:             resolved.Zone.Slug,
			Name:             resolved.Zone.Name,
			Kind:             resolved.Zone.Kind,
			Status:           resolved.Zone.Status,
			RegistrationMode: resolved.Zone.RegistrationMode,
		},
		Runtime:      runtime,
		Capabilities: capabilities,
		Deployment: DeploymentDTO{
			Mode:         deploymentMode(resolved.Deployment),
			DatabaseMode: deploymentDatabaseMode(resolved.Deployment),
			Status:       deploymentStatus(resolved.Deployment),
		},
	}
}

func (s *Service) zoneHasReadyOIDCProvider(ctx context.Context, zone tenancydomain.Zone) bool {
	if !s.options.OIDCEnabled || zone.Status != "active" {
		return false
	}
	providers, err := s.repo.ListOIDCProviders(ctx, zone.ID)
	if err != nil {
		return false
	}
	for _, provider := range providers {
		if provider.Status != "configured" {
			continue
		}
		if provider.ClientSecretRef == nil || strings.TrimSpace(*provider.ClientSecretRef) == "" {
			return true
		}
		ref := strings.TrimSpace(*provider.ClientSecretRef)
		if !strings.HasPrefix(ref, "env://") {
			continue
		}
		alias := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(ref, "env://")))
		if s.options.OIDCClientSecrets[alias] != "" {
			return true
		}
	}
	return false
}

func (s *Service) runtime(domain string, deployment *tenancydomain.Deployment) RuntimeDTO {
	webURL := "https://" + domain
	apiURL := webURL
	wsURL := "wss://" + domain + "/ws"
	adminURL := webURL + "/admin"

	if deployment != nil && deploymentMode(deployment) != "shared" {
		webURL = firstNonEmpty(deployment.WebBaseURL, webURL)
		apiURL = firstNonEmpty(deployment.APIBaseURL, apiURL)
		wsURL = firstNonEmpty(deployment.WSBaseURL, wsURL)
		if deployment.AdminBaseURL != nil && strings.TrimSpace(*deployment.AdminBaseURL) != "" {
			adminURL = strings.TrimRight(strings.TrimSpace(*deployment.AdminBaseURL), "/")
		}
	}

	return RuntimeDTO{
		AppName:        s.options.AppName,
		AppVersion:     s.options.AppVersion,
		ReleaseChannel: s.options.ReleaseChannel,
		Locale:         s.options.DefaultLocale,
		WebBaseURL:     strings.TrimRight(webURL, "/"),
		APIBaseURL:     strings.TrimRight(apiURL, "/"),
		WSBaseURL:      strings.TrimRight(wsURL, "/"),
		AdminBaseURL:   strings.TrimRight(adminURL, "/"),
		RTCICEServers:  cloneJSONMapList(s.options.RTCICEServers),
	}
}

func cloneJSONMapList(source []map[string]any) []map[string]any {
	if len(source) == 0 {
		return nil
	}
	cloned := make([]map[string]any, 0, len(source))
	for _, item := range source {
		copy := make(map[string]any, len(item))
		for key, value := range item {
			copy[key] = value
		}
		cloned = append(cloned, copy)
	}
	return cloned
}

func capabilitiesFromMetadata(metadata map[string]any) CapabilitiesDTO {
	capabilities := CapabilitiesDTO{
		Chat:       true,
		Files:      true,
		Calls:      true,
		Bots:       true,
		Automation: true,
		Webhooks:   true,
	}
	raw, ok := metadata["capabilities"].(map[string]any)
	if !ok {
		return capabilities
	}
	capabilities.Chat = boolValue(raw, "chat", capabilities.Chat)
	capabilities.Files = boolValue(raw, "files", capabilities.Files)
	capabilities.Calls = boolValue(raw, "calls", capabilities.Calls)
	capabilities.Bots = boolValue(raw, "bots", capabilities.Bots)
	capabilities.Automation = boolValue(raw, "automation", capabilities.Automation)
	capabilities.Webhooks = boolValue(raw, "webhooks", capabilities.Webhooks)
	capabilities.Federation = boolValue(raw, "federation", capabilities.Federation)
	capabilities.SSO = boolValue(raw, "sso", capabilities.SSO)
	capabilities.CustomDomain = boolValue(raw, "custom_domain", capabilities.CustomDomain)
	capabilities.Dedicated = boolValue(raw, "dedicated", capabilities.Dedicated)
	return capabilities
}

func boolValue(values map[string]any, key string, fallback bool) bool {
	value, ok := values[key]
	if !ok {
		return fallback
	}
	typed, ok := value.(bool)
	if !ok {
		return fallback
	}
	return typed
}

func deploymentMode(deployment *tenancydomain.Deployment) string {
	if deployment == nil || strings.TrimSpace(deployment.Mode) == "" {
		return "shared"
	}
	return deployment.Mode
}

func deploymentDatabaseMode(deployment *tenancydomain.Deployment) string {
	if deployment == nil || strings.TrimSpace(deployment.DatabaseMode) == "" {
		return "shared_schema"
	}
	return deployment.DatabaseMode
}

func deploymentStatus(deployment *tenancydomain.Deployment) string {
	if deployment == nil || strings.TrimSpace(deployment.Status) == "" {
		return "provisioning"
	}
	return deployment.Status
}

func firstNonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func toDomainClaimDTO(claim tenancydomain.DomainClaim, options Options) DomainClaimDTO {
	var workspace *WorkspaceRefDTO
	if claim.Workspace != nil {
		workspace = &WorkspaceRefDTO{
			ID:   claim.Workspace.ID,
			Slug: claim.Workspace.Slug,
			Name: claim.Workspace.Name,
		}
	}
	return DomainClaimDTO{
		ID:                    claim.Domain.ID,
		Domain:                claim.Domain.Domain,
		Status:                claim.Domain.Status,
		RoutingDNSType:        strings.ToUpper(strings.TrimSpace(options.RoutingDNSType)),
		RoutingDNSName:        routingDNSName(claim.Domain.Domain, options),
		RoutingDNSValue:       strings.TrimSpace(options.RoutingDNSTarget),
		VerificationMethod:    claim.Domain.VerificationMethod,
		VerificationDNSName:   verificationDNSName(claim.Domain.Domain),
		VerificationDNSValue:  verificationDNSValue(claim.Domain.VerificationToken),
		VerificationExpiresAt: formatOptionalTime(claim.Domain.VerificationExpiresAt),
		VerificationAttempts:  claim.Domain.VerificationAttempts,
		LastVerificationError: claim.Domain.LastVerificationError,
		LastCheckedAt:         formatOptionalTime(claim.Domain.LastCheckedAt),
		Zone: ZoneDTO{
			ID:               claim.Zone.ID,
			Slug:             claim.Zone.Slug,
			Name:             claim.Zone.Name,
			Kind:             claim.Zone.Kind,
			Status:           claim.Zone.Status,
			RegistrationMode: claim.Zone.RegistrationMode,
		},
		Workspace: workspace,
	}
}

func routingDNSName(domain string, options Options) string {
	if strings.TrimSpace(options.RoutingDNSTarget) == "" {
		return ""
	}
	return domain
}

func mapClaimError(err error) error {
	switch {
	case errors.Is(err, tenancydomain.ErrDomainClaimNotFound):
		return apperrors.NotFound("DOMAIN_CLAIM_NOT_FOUND", "Khong tim thay domain claim.")
	case errors.Is(err, tenancydomain.ErrDomainVerificationExpired):
		return apperrors.Conflict("DOMAIN_VERIFICATION_EXPIRED", "Ma xac minh domain da het han.")
	case errors.Is(err, tenancydomain.ErrZoneAccessDenied):
		return apperrors.Forbidden("Ban khong co quyen quan ly domain nay.")
	default:
		return err
	}
}

func toAutomationTemplateDTO(template tenancydomain.AutomationTemplate) AutomationTemplateDTO {
	return AutomationTemplateDTO{
		ID:             template.ID,
		Key:            template.Key,
		Name:           template.Name,
		Description:    template.Description,
		ZoneKind:       template.ZoneKind,
		TemplateType:   template.TemplateType,
		RuntimeKind:    template.RuntimeKind,
		ConfigSchema:   cloneMap(template.ConfigSchema),
		DefaultConfig:  cloneMap(template.DefaultConfig),
		RequiredScopes: append([]string{}, template.RequiredScopes...),
		Status:         template.Status,
	}
}

func toAutomationInstallationDTO(installation tenancydomain.AutomationInstallation) AutomationInstallationDTO {
	return AutomationInstallationDTO{
		ID:               installation.ID,
		ZoneID:           installation.ZoneID,
		WorkspaceID:      installation.WorkspaceID,
		TemplateID:       installation.TemplateID,
		TemplateKey:      installation.TemplateKey,
		Name:             installation.Name,
		Status:           installation.Status,
		Config:           cloneMap(installation.Config),
		HasSecretRef:     installation.SecretRef != nil && strings.TrimSpace(*installation.SecretRef) != "",
		RuntimeWebhookID: installation.RuntimeWebhookID,
		RuntimeReady:     installation.RuntimeWebhookID != nil,
		CreatedAt:        installation.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        installation.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func automationWebhookRuntime(config map[string]any, defaults map[string]any) (string, []string, error) {
	merged := cloneMap(defaults)
	for key, value := range config {
		merged[key] = value
	}
	targetURL, _ := merged["endpoint_url"].(string)
	targetURL = strings.TrimSpace(targetURL)
	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return "", nil, apperrors.BadRequest(
			"AUTOMATION_CONFIG_INVALID",
			"endpoint_url cua automation phai la HTTPS URL hop le va khong chua thong tin dang nhap.",
		)
	}
	eventTypes := normalizeAutomationEventTypes(merged["event_types"])
	if len(eventTypes) == 0 {
		eventTypes = []string{"MessageCreated"}
	}
	return targetURL, eventTypes, nil
}

func normalizeAutomationEventTypes(value any) []string {
	values := make([]string, 0)
	switch typed := value.(type) {
	case []string:
		values = append(values, typed...)
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
	case string:
		values = append(values, typed)
	}
	unique := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 120 {
			continue
		}
		if _, exists := unique[value]; exists {
			continue
		}
		unique[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func newAutomationSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "wtar_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func validSecretRef(value string) bool {
	for _, prefix := range []string{"env://", "vault://", "aws-secrets://", "gcp-secrets://", "azure-keyvault://"} {
		if strings.HasPrefix(value, prefix) && len(strings.TrimSpace(strings.TrimPrefix(value, prefix))) > 0 {
			return true
		}
	}
	return false
}

func validOIDCSecretRef(value string) bool {
	if !validSecretRef(value) {
		return false
	}
	if !strings.HasPrefix(value, "env://") {
		return true
	}
	alias := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "env://")))
	return oidcSecretAliasPattern.MatchString(alias)
}

func containsInlineSecret(config map[string]any) bool {
	if len(config) == 0 {
		return false
	}
	sensitive := []string{"secret", "token", "password", "api_key", "apikey", "private_key", "credential"}
	var walk func(any) bool
	walk = func(value any) bool {
		switch values := value.(type) {
		case map[string]any:
			keys := make([]string, 0, len(values))
			for key := range values {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				lowerKey := strings.ToLower(strings.TrimSpace(key))
				for _, marker := range sensitive {
					if strings.Contains(lowerKey, marker) && values[key] != nil && strings.TrimSpace(toString(values[key])) != "" {
						return true
					}
				}
				if walk(values[key]) {
					return true
				}
			}
		case []any:
			for _, item := range values {
				if walk(item) {
					return true
				}
			}
		}
		return false
	}
	return walk(config)
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return "configured"
	}
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}
