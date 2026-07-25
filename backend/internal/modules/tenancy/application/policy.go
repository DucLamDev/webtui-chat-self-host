package application

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"

	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type UpdateZoneQuotaInput struct {
	ActorUserID                string
	ZoneID                     string
	MaxWorkspaces              int
	MaxMembers                 int
	MaxStorageBytes            int64
	MaxAutomationInstallations int
	MaxWebhooks                int
	EnforcementMode            string
}

type CreateOIDCProviderInput struct {
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

type UpdateOIDCProviderInput struct {
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

type ZoneQuotaDTO struct {
	MaxWorkspaces              int    `json:"max_workspaces"`
	MaxMembers                 int    `json:"max_members"`
	MaxStorageBytes            int64  `json:"max_storage_bytes"`
	MaxAutomationInstallations int    `json:"max_automation_installations"`
	MaxWebhooks                int    `json:"max_webhooks"`
	EnforcementMode            string `json:"enforcement_mode"`
	UpdatedAt                  string `json:"updated_at"`
}

type ZoneUsageDTO struct {
	Workspaces              int   `json:"workspaces"`
	Members                 int   `json:"members"`
	StorageBytes            int64 `json:"storage_bytes"`
	AutomationInstallations int   `json:"automation_installations"`
	Webhooks                int   `json:"webhooks"`
}

type ZoneQuotaOverviewDTO struct {
	Quota ZoneQuotaDTO `json:"quota"`
	Usage ZoneUsageDTO `json:"usage"`
}

type OIDCProviderDTO struct {
	ID                   string         `json:"id"`
	ZoneID               string         `json:"zone_id"`
	Name                 string         `json:"name"`
	IssuerURL            string         `json:"issuer_url"`
	ClientID             string         `json:"client_id"`
	HasClientSecretRef   bool           `json:"has_client_secret_ref"`
	Scopes               []string       `json:"scopes"`
	ClaimMapping         map[string]any `json:"claim_mapping"`
	JITProvisioning      bool           `json:"jit_provisioning"`
	RequireVerifiedEmail bool           `json:"require_verified_email"`
	Status               string         `json:"status"`
	CreatedAt            string         `json:"created_at"`
	UpdatedAt            string         `json:"updated_at"`
}

func (s *Service) GetZoneQuota(
	ctx context.Context,
	actorUserID string,
	zoneID string,
) (ZoneQuotaOverviewDTO, error) {
	if err := s.ensureZoneManager(ctx, actorUserID, zoneID); err != nil {
		return ZoneQuotaOverviewDTO{}, err
	}
	quota, usage, err := s.repo.GetZoneQuota(ctx, strings.TrimSpace(zoneID))
	if err != nil {
		return ZoneQuotaOverviewDTO{}, mapPolicyError(err)
	}
	return toZoneQuotaOverviewDTO(quota, usage), nil
}

func (s *Service) UpdateZoneQuota(
	ctx context.Context,
	input UpdateZoneQuotaInput,
) (ZoneQuotaOverviewDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	input.EnforcementMode = strings.ToLower(strings.TrimSpace(input.EnforcementMode))
	if err := s.ensureZoneManager(ctx, input.ActorUserID, input.ZoneID); err != nil {
		return ZoneQuotaOverviewDTO{}, err
	}
	if input.MaxWorkspaces <= 0 || input.MaxWorkspaces > 100000 ||
		input.MaxMembers <= 0 || input.MaxMembers > 10000000 ||
		input.MaxStorageBytes <= 0 ||
		input.MaxAutomationInstallations <= 0 || input.MaxAutomationInstallations > 1000000 ||
		input.MaxWebhooks <= 0 || input.MaxWebhooks > 1000000 {
		return ZoneQuotaOverviewDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Gia tri quota khong hop le.")
	}
	if input.EnforcementMode != "monitor" && input.EnforcementMode != "hard" {
		return ZoneQuotaOverviewDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "enforcement_mode khong hop le.")
	}
	quota, usage, err := s.repo.UpdateZoneQuota(ctx, UpdateZoneQuotaParams{
		ActorUserID:                input.ActorUserID,
		ZoneID:                     input.ZoneID,
		MaxWorkspaces:              input.MaxWorkspaces,
		MaxMembers:                 input.MaxMembers,
		MaxStorageBytes:            input.MaxStorageBytes,
		MaxAutomationInstallations: input.MaxAutomationInstallations,
		MaxWebhooks:                input.MaxWebhooks,
		EnforcementMode:            input.EnforcementMode,
	})
	if err != nil {
		return ZoneQuotaOverviewDTO{}, mapPolicyError(err)
	}
	return toZoneQuotaOverviewDTO(quota, usage), nil
}

func (s *Service) ListOIDCProviders(
	ctx context.Context,
	actorUserID string,
	zoneID string,
) ([]OIDCProviderDTO, error) {
	if err := s.ensureZoneManager(ctx, actorUserID, zoneID); err != nil {
		return nil, err
	}
	providers, err := s.repo.ListOIDCProviders(ctx, strings.TrimSpace(zoneID))
	if err != nil {
		return nil, err
	}
	result := make([]OIDCProviderDTO, 0, len(providers))
	for _, provider := range providers {
		result = append(result, toOIDCProviderDTO(provider))
	}
	return result, nil
}

func (s *Service) CreateOIDCProvider(
	ctx context.Context,
	input CreateOIDCProviderInput,
) (OIDCProviderDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	input.Name = strings.TrimSpace(input.Name)
	input.IssuerURL = strings.TrimRight(strings.TrimSpace(input.IssuerURL), "/")
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.ClientSecretRef = strings.TrimSpace(input.ClientSecretRef)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = "configured"
	}
	if err := s.ensureZoneManager(ctx, input.ActorUserID, input.ZoneID); err != nil {
		return OIDCProviderDTO{}, err
	}
	if err := validateOIDCProvider(
		input.Name,
		input.IssuerURL,
		input.ClientID,
		input.ClientSecretRef,
		input.Status,
	); err != nil {
		return OIDCProviderDTO{}, err
	}
	scopes := normalizeOIDCScopes(input.Scopes)
	if len(scopes) == 0 {
		scopes = []string{"email", "openid", "profile"}
	}
	claimMapping := cloneMap(input.ClaimMapping)
	if len(claimMapping) == 0 {
		claimMapping = map[string]any{
			"subject":        "sub",
			"email":          "email",
			"email_verified": "email_verified",
			"username":       "preferred_username",
			"display_name":   "name",
			"groups":         "groups",
		}
	}
	if err := validateOIDCClaimMapping(claimMapping); err != nil {
		return OIDCProviderDTO{}, err
	}
	provider, err := s.repo.CreateOIDCProvider(ctx, CreateOIDCProviderParams{
		ActorUserID:          input.ActorUserID,
		ZoneID:               input.ZoneID,
		Name:                 input.Name,
		IssuerURL:            input.IssuerURL,
		ClientID:             input.ClientID,
		ClientSecretRef:      input.ClientSecretRef,
		Scopes:               scopes,
		ClaimMapping:         claimMapping,
		JITProvisioning:      input.JITProvisioning,
		RequireVerifiedEmail: input.RequireVerifiedEmail,
		Status:               input.Status,
	})
	if err != nil {
		return OIDCProviderDTO{}, mapPolicyError(err)
	}
	return toOIDCProviderDTO(provider), nil
}

func (s *Service) UpdateOIDCProvider(
	ctx context.Context,
	input UpdateOIDCProviderInput,
) (OIDCProviderDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	input.ProviderID = strings.TrimSpace(input.ProviderID)
	if err := s.ensureZoneManager(ctx, input.ActorUserID, input.ZoneID); err != nil {
		return OIDCProviderDTO{}, err
	}
	if input.ProviderID == "" {
		return OIDCProviderDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "provider_id la bat buoc.")
	}
	if input.Name == nil && input.IssuerURL == nil && input.ClientID == nil &&
		input.ClientSecretRef == nil && input.Scopes == nil &&
		input.ClaimMapping == nil && input.JITProvisioning == nil &&
		input.RequireVerifiedEmail == nil && input.Status == nil {
		return OIDCProviderDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Can cung cap it nhat mot truong de cap nhat.")
	}
	normalizeOptionalString(input.Name, false)
	normalizeOptionalString(input.ClientID, false)
	normalizeOptionalString(input.ClientSecretRef, false)
	if input.IssuerURL != nil {
		value := strings.TrimRight(strings.TrimSpace(*input.IssuerURL), "/")
		input.IssuerURL = &value
	}
	if input.Status != nil {
		value := strings.ToLower(strings.TrimSpace(*input.Status))
		input.Status = &value
	}
	if err := validateOptionalOIDCProvider(input); err != nil {
		return OIDCProviderDTO{}, err
	}
	if input.Scopes != nil {
		scopes := normalizeOIDCScopes(*input.Scopes)
		if len(scopes) == 0 {
			return OIDCProviderDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "OIDC scopes khong duoc rong.")
		}
		input.Scopes = &scopes
	}
	if input.ClaimMapping != nil {
		mapping := cloneMap(*input.ClaimMapping)
		if err := validateOIDCClaimMapping(mapping); err != nil {
			return OIDCProviderDTO{}, err
		}
		input.ClaimMapping = &mapping
	}
	provider, err := s.repo.UpdateOIDCProvider(ctx, UpdateOIDCProviderParams{
		ActorUserID:          input.ActorUserID,
		ZoneID:               input.ZoneID,
		ProviderID:           input.ProviderID,
		Name:                 input.Name,
		IssuerURL:            input.IssuerURL,
		ClientID:             input.ClientID,
		ClientSecretRef:      input.ClientSecretRef,
		Scopes:               input.Scopes,
		ClaimMapping:         input.ClaimMapping,
		JITProvisioning:      input.JITProvisioning,
		RequireVerifiedEmail: input.RequireVerifiedEmail,
		Status:               input.Status,
	})
	if err != nil {
		return OIDCProviderDTO{}, mapPolicyError(err)
	}
	return toOIDCProviderDTO(provider), nil
}

func (s *Service) DeleteOIDCProvider(
	ctx context.Context,
	actorUserID string,
	zoneID string,
	providerID string,
) error {
	if err := s.ensureZoneManager(ctx, actorUserID, zoneID); err != nil {
		return err
	}
	err := s.repo.DeleteOIDCProvider(
		ctx,
		strings.TrimSpace(zoneID),
		strings.TrimSpace(providerID),
		strings.TrimSpace(actorUserID),
	)
	return mapPolicyError(err)
}

func validateOIDCProvider(name string, issuerURL string, clientID string, secretRef string, status string) error {
	if name == "" || len([]rune(name)) > 120 || clientID == "" || len(clientID) > 500 {
		return apperrors.BadRequest("VALIDATION_ERROR", "name va client_id OIDC khong hop le.")
	}
	parsed, err := url.Parse(issuerURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return apperrors.BadRequest("VALIDATION_ERROR", "issuer_url phai la HTTPS URL khong chua user, query hoac fragment.")
	}
	if secretRef != "" && !validOIDCSecretRef(secretRef) {
		return apperrors.BadRequest("INVALID_SECRET_REF", "client_secret_ref phai dung provider secret duoc ho tro.")
	}
	if status == "" {
		status = "configured"
	}
	if status != "configured" && status != "disabled" {
		return apperrors.BadRequest("VALIDATION_ERROR", "status OIDC khong hop le.")
	}
	return nil
}

func validateOptionalOIDCProvider(input UpdateOIDCProviderInput) error {
	if input.Name != nil && (*input.Name == "" || len([]rune(*input.Name)) > 120) {
		return apperrors.BadRequest("VALIDATION_ERROR", "name OIDC khong hop le.")
	}
	if input.ClientID != nil && (*input.ClientID == "" || len(*input.ClientID) > 500) {
		return apperrors.BadRequest("VALIDATION_ERROR", "client_id OIDC khong hop le.")
	}
	if input.IssuerURL != nil {
		if err := validateOIDCProvider("provider", *input.IssuerURL, "client", "", "configured"); err != nil {
			return err
		}
	}
	if input.ClientSecretRef != nil && *input.ClientSecretRef != "" && !validOIDCSecretRef(*input.ClientSecretRef) {
		return apperrors.BadRequest("INVALID_SECRET_REF", "client_secret_ref phai dung provider secret duoc ho tro.")
	}
	if input.Status != nil && *input.Status != "configured" && *input.Status != "disabled" {
		return apperrors.BadRequest("VALIDATION_ERROR", "status OIDC khong hop le.")
	}
	return nil
}

func normalizeOIDCScopes(values []string) []string {
	seen := make(map[string]struct{}, len(values)+1)
	seen["openid"] = struct{}{}
	result := []string{"openid"}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || len(value) > 120 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func validateOIDCClaimMapping(mapping map[string]any) error {
	allowed := map[string]struct{}{
		"subject":        {},
		"email":          {},
		"email_verified": {},
		"username":       {},
		"display_name":   {},
		"groups":         {},
	}
	for field, rawClaimName := range mapping {
		if _, ok := allowed[field]; !ok {
			return apperrors.BadRequest("VALIDATION_ERROR", "claim_mapping chua truong khong duoc ho tro.")
		}
		claimName, ok := rawClaimName.(string)
		claimName = strings.TrimSpace(claimName)
		if !ok || claimName == "" || len(claimName) > 200 {
			return apperrors.BadRequest("VALIDATION_ERROR", "claim_mapping phai anh xa sang ten claim hop le.")
		}
		mapping[field] = claimName
	}
	return nil
}

func normalizeOptionalString(value *string, lower bool) {
	if value == nil {
		return
	}
	normalized := strings.TrimSpace(*value)
	if lower {
		normalized = strings.ToLower(normalized)
	}
	*value = normalized
}

func mapPolicyError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, tenancydomain.ErrOIDCProviderNotFound):
		return apperrors.NotFound("OIDC_PROVIDER_NOT_FOUND", "Khong tim thay OIDC provider trong zone hien tai.")
	case errors.Is(err, tenancydomain.ErrOIDCProviderConflict):
		return apperrors.Conflict("OIDC_PROVIDER_CONFLICT", "Ten OIDC provider da ton tai trong zone.")
	case errors.Is(err, tenancydomain.ErrZoneQuotaExceeded):
		return apperrors.Conflict("ZONE_QUOTA_EXCEEDED", "Zone da dat gioi han quota.")
	default:
		return err
	}
}

func toZoneQuotaOverviewDTO(quota tenancydomain.ZoneQuota, usage tenancydomain.ZoneUsage) ZoneQuotaOverviewDTO {
	return ZoneQuotaOverviewDTO{
		Quota: ZoneQuotaDTO{
			MaxWorkspaces:              quota.MaxWorkspaces,
			MaxMembers:                 quota.MaxMembers,
			MaxStorageBytes:            quota.MaxStorageBytes,
			MaxAutomationInstallations: quota.MaxAutomationInstallations,
			MaxWebhooks:                quota.MaxWebhooks,
			EnforcementMode:            quota.EnforcementMode,
			UpdatedAt:                  quota.UpdatedAt.UTC().Format(time.RFC3339),
		},
		Usage: ZoneUsageDTO{
			Workspaces:              usage.Workspaces,
			Members:                 usage.Members,
			StorageBytes:            usage.StorageBytes,
			AutomationInstallations: usage.AutomationInstallations,
			Webhooks:                usage.Webhooks,
		},
	}
}

func toOIDCProviderDTO(provider tenancydomain.OIDCProvider) OIDCProviderDTO {
	return OIDCProviderDTO{
		ID:                   provider.ID,
		ZoneID:               provider.ZoneID,
		Name:                 provider.Name,
		IssuerURL:            provider.IssuerURL,
		ClientID:             provider.ClientID,
		HasClientSecretRef:   provider.ClientSecretRef != nil && *provider.ClientSecretRef != "",
		Scopes:               append([]string{}, provider.Scopes...),
		ClaimMapping:         cloneMap(provider.ClaimMapping),
		JITProvisioning:      provider.JITProvisioning,
		RequireVerifiedEmail: provider.RequireVerifiedEmail,
		Status:               provider.Status,
		CreatedAt:            provider.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:            provider.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
