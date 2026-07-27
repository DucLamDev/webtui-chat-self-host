package application

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type UpdateZoneSettingsInput struct {
	ActorUserID      string
	ZoneID           string
	Name             *string
	RegistrationMode *string
	LogoURL          *string
}

type ZoneLifecycleInput struct {
	ActorUserID string
	ZoneID      string
	Action      string
	Reason      string
}

type CreateAdditionalDomainInput struct {
	ActorUserID string
	ZoneID      string
	Domain      string
	Kind        string
}

type CreateDeploymentRequestInput struct {
	ActorUserID           string
	ZoneID                string
	RequestedMode         string
	RequestedDatabaseMode string
	IdempotencyKey        string
}

type ZoneAdminOverviewDTO struct {
	Zone        ZoneDTO          `json:"zone"`
	Domains     []DomainAdminDTO `json:"domains"`
	Deployments []DeploymentDTO  `json:"deployments"`
}

type DomainAdminDTO struct {
	ID                    string  `json:"id"`
	Domain                string  `json:"domain"`
	Kind                  string  `json:"kind"`
	Status                string  `json:"status"`
	TLSStatus             string  `json:"tls_status"`
	VerificationMethod    string  `json:"verification_method"`
	VerificationDNSName   string  `json:"verification_dns_name"`
	VerificationDNSValue  string  `json:"verification_dns_value,omitempty"`
	VerificationExpiresAt *string `json:"verification_expires_at,omitempty"`
	VerifiedAt            *string `json:"verified_at,omitempty"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
}

type DeploymentRequestDTO struct {
	ID                    string         `json:"id"`
	ZoneID                string         `json:"zone_id"`
	RequestedMode         string         `json:"requested_mode"`
	RequestedDatabaseMode string         `json:"requested_database_mode"`
	Status                string         `json:"status"`
	IdempotencyKey        string         `json:"idempotency_key"`
	FailureReason         *string        `json:"failure_reason,omitempty"`
	Metadata              map[string]any `json:"metadata"`
	CreatedAt             string         `json:"created_at"`
	UpdatedAt             string         `json:"updated_at"`
	CompletedAt           *string        `json:"completed_at,omitempty"`
}

func (s *Service) GetZoneAdminOverview(
	ctx context.Context,
	actorUserID string,
	zoneID string,
) (ZoneAdminOverviewDTO, error) {
	if err := s.ensureZoneManager(ctx, actorUserID, zoneID); err != nil {
		return ZoneAdminOverviewDTO{}, err
	}
	overview, err := s.repo.GetZoneAdminOverview(ctx, strings.TrimSpace(zoneID))
	if err != nil {
		return ZoneAdminOverviewDTO{}, mapClaimError(err)
	}
	return toZoneAdminOverviewDTO(overview), nil
}

func (s *Service) UpdateZoneSettings(
	ctx context.Context,
	input UpdateZoneSettingsInput,
) (ZoneAdminOverviewDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	if err := s.ensureZoneManager(ctx, input.ActorUserID, input.ZoneID); err != nil {
		return ZoneAdminOverviewDTO{}, err
	}
	if input.Name == nil && input.RegistrationMode == nil && input.LogoURL == nil {
		return ZoneAdminOverviewDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Can cung cap it nhat mot truong de cap nhat.")
	}
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if value == "" || len([]rune(value)) > 120 {
			return ZoneAdminOverviewDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Tên vùng máy chủ phải dài từ 1 đến 120 ký tự.")
		}
		input.Name = &value
	}
	if input.RegistrationMode != nil {
		value := strings.ToLower(strings.TrimSpace(*input.RegistrationMode))
		if value != "open" && value != "invite_only" && value != "closed" {
			return ZoneAdminOverviewDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Chế độ đăng ký không hợp lệ.")
		}
		input.RegistrationMode = &value
	}
	if input.LogoURL != nil {
		value := strings.TrimSpace(*input.LogoURL)
		if len(value) > 2048 {
			return ZoneAdminOverviewDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "URL logo quá dài.")
		}
		if value != "" {
			parsed, err := url.Parse(value)
			if err != nil ||
				(parsed.IsAbs() && (parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil)) ||
				(!parsed.IsAbs() && (!strings.HasPrefix(value, "/") || parsed.Host != "")) {
				return ZoneAdminOverviewDTO{}, apperrors.BadRequest(
					"VALIDATION_ERROR",
					"Logo phải là URL HTTPS hoặc đường dẫn bắt đầu bằng /.",
				)
			}
		}
		input.LogoURL = &value
	}
	overview, err := s.repo.UpdateZoneSettings(ctx, UpdateZoneSettingsParams{
		ActorUserID:      input.ActorUserID,
		ZoneID:           input.ZoneID,
		Name:             input.Name,
		RegistrationMode: input.RegistrationMode,
		LogoURL:          input.LogoURL,
	})
	if err != nil {
		return ZoneAdminOverviewDTO{}, mapClaimError(err)
	}
	return toZoneAdminOverviewDTO(overview), nil
}

func (s *Service) SetZoneLifecycle(ctx context.Context, input ZoneLifecycleInput) error {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	input.Reason = strings.TrimSpace(input.Reason)
	if err := s.ensureZoneManager(ctx, input.ActorUserID, input.ZoneID); err != nil {
		return err
	}
	switch input.Action {
	case "suspend", "resume", "archive":
	default:
		return apperrors.BadRequest("VALIDATION_ERROR", "Thao tác vòng đời không hợp lệ.")
	}
	if input.Action != "resume" && (input.Reason == "" || len([]rune(input.Reason)) > 500) {
		return apperrors.BadRequest("VALIDATION_ERROR", "Lý do phải dài từ 1 đến 500 ký tự.")
	}
	if err := s.repo.SetZoneLifecycle(ctx, SetZoneLifecycleParams{
		ActorUserID: input.ActorUserID,
		ZoneID:      input.ZoneID,
		Action:      input.Action,
		Reason:      input.Reason,
	}); err != nil {
		return mapClaimError(err)
	}
	return nil
}

func (s *Service) CreateAdditionalDomain(
	ctx context.Context,
	input CreateAdditionalDomainInput,
) (DomainClaimDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	if err := s.ensureZoneManager(ctx, input.ActorUserID, input.ZoneID); err != nil {
		return DomainClaimDTO{}, err
	}
	domain, err := NormalizeDomain(input.Domain)
	if err != nil || !fqdnPattern.MatchString(domain) {
		return DomainClaimDTO{}, apperrors.BadRequest("INVALID_DOMAIN", "Chỉ hỗ trợ tên miền công khai hợp lệ.")
	}
	if input.Kind == "" {
		input.Kind = "alias"
	}
	if input.Kind != "alias" && input.Kind != "api" && input.Kind != "web" {
		return DomainClaimDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Loại tên miền phụ không hợp lệ.")
	}
	claim, err := s.repo.CreateAdditionalDomainClaim(ctx, CreateAdditionalDomainClaimParams{
		ActorUserID: input.ActorUserID,
		ZoneID:      input.ZoneID,
		Domain:      domain,
		Kind:        input.Kind,
		ExpiresAt:   s.now().UTC().Add(s.verificationTTL),
	})
	if err != nil {
		if errors.Is(err, tenancydomain.ErrDomainAlreadyClaimed) {
			return DomainClaimDTO{}, apperrors.Conflict("DOMAIN_ALREADY_CLAIMED", "Tên miền đã được một vùng máy chủ xác nhận quyền sở hữu.")
		}
		return DomainClaimDTO{}, mapClaimError(err)
	}
	return toDomainClaimDTO(claim, s.options), nil
}

func (s *Service) SetPrimaryDomain(
	ctx context.Context,
	actorUserID string,
	zoneID string,
	domainID string,
) error {
	if err := s.ensureZoneManager(ctx, actorUserID, zoneID); err != nil {
		return err
	}
	if err := s.repo.SetPrimaryDomain(
		ctx,
		strings.TrimSpace(zoneID),
		strings.TrimSpace(domainID),
		strings.TrimSpace(actorUserID),
	); err != nil {
		return mapClaimError(err)
	}
	return nil
}

func (s *Service) DeleteZoneDomain(
	ctx context.Context,
	actorUserID string,
	zoneID string,
	domainID string,
) error {
	if err := s.ensureZoneManager(ctx, actorUserID, zoneID); err != nil {
		return err
	}
	err := s.repo.DeleteZoneDomain(
		ctx,
		strings.TrimSpace(zoneID),
		strings.TrimSpace(domainID),
		strings.TrimSpace(actorUserID),
	)
	if errors.Is(err, tenancydomain.ErrDomainIsPrimary) {
		return apperrors.Conflict("PRIMARY_DOMAIN_REQUIRED", "Hay chuyen primary domain truoc khi xoa domain nay.")
	}
	return mapClaimError(err)
}

func (s *Service) ListDeploymentRequests(
	ctx context.Context,
	actorUserID string,
	zoneID string,
) ([]DeploymentRequestDTO, error) {
	if err := s.ensureZoneManager(ctx, actorUserID, zoneID); err != nil {
		return nil, err
	}
	requests, err := s.repo.ListDeploymentRequests(ctx, strings.TrimSpace(zoneID))
	if err != nil {
		return nil, err
	}
	result := make([]DeploymentRequestDTO, 0, len(requests))
	for _, request := range requests {
		result = append(result, toDeploymentRequestDTO(request))
	}
	return result, nil
}

func (s *Service) CreateDeploymentRequest(
	ctx context.Context,
	input CreateDeploymentRequestInput,
) (DeploymentRequestDTO, error) {
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	input.RequestedMode = strings.ToLower(strings.TrimSpace(input.RequestedMode))
	input.RequestedDatabaseMode = strings.ToLower(strings.TrimSpace(input.RequestedDatabaseMode))
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if err := s.ensureZoneManager(ctx, input.ActorUserID, input.ZoneID); err != nil {
		return DeploymentRequestDTO{}, err
	}
	switch input.RequestedMode {
	case "shared", "dedicated_compose", "dedicated_k8s":
	default:
		return DeploymentRequestDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Chế độ triển khai được yêu cầu không hợp lệ.")
	}
	switch input.RequestedDatabaseMode {
	case "shared_schema", "dedicated_schema", "dedicated_database":
	default:
		return DeploymentRequestDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Chế độ cơ sở dữ liệu được yêu cầu không hợp lệ.")
	}
	if input.IdempotencyKey == "" || len(input.IdempotencyKey) > 120 {
		return DeploymentRequestDTO{}, apperrors.BadRequest("VALIDATION_ERROR", "Idempotency-Key phải dài từ 1 đến 120 ký tự.")
	}
	request, err := s.repo.CreateDeploymentRequest(ctx, CreateDeploymentRequestParams{
		ActorUserID:           input.ActorUserID,
		ZoneID:                input.ZoneID,
		RequestedMode:         input.RequestedMode,
		RequestedDatabaseMode: input.RequestedDatabaseMode,
		IdempotencyKey:        input.IdempotencyKey,
	})
	if errors.Is(err, tenancydomain.ErrDeploymentRequestConflict) {
		return DeploymentRequestDTO{}, apperrors.Conflict("DEPLOYMENT_REQUEST_CONFLICT", "Idempotency-Key đã được dùng với payload khác.")
	}
	if err != nil {
		return DeploymentRequestDTO{}, err
	}
	return toDeploymentRequestDTO(request), nil
}

func toZoneAdminOverviewDTO(overview tenancydomain.ZoneAdminOverview) ZoneAdminOverviewDTO {
	domains := make([]DomainAdminDTO, 0, len(overview.Domains))
	for _, domain := range overview.Domains {
		value := verificationDNSValue(domain.VerificationToken)
		if domain.Status != "pending" {
			value = ""
		}
		domains = append(domains, DomainAdminDTO{
			ID:                    domain.ID,
			Domain:                domain.Domain,
			Kind:                  domain.Kind,
			Status:                domain.Status,
			TLSStatus:             domain.TLSStatus,
			VerificationMethod:    domain.VerificationMethod,
			VerificationDNSName:   verificationDNSName(domain.Domain),
			VerificationDNSValue:  value,
			VerificationExpiresAt: formatOptionalTime(domain.VerificationExpiresAt),
			VerifiedAt:            formatOptionalTime(domain.VerifiedAt),
			CreatedAt:             domain.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:             domain.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	deployments := make([]DeploymentDTO, 0, len(overview.Deployments))
	for _, deployment := range overview.Deployments {
		deployments = append(deployments, DeploymentDTO{
			Mode:         deployment.Mode,
			DatabaseMode: deployment.DatabaseMode,
			Status:       deployment.Status,
		})
	}
	return ZoneAdminOverviewDTO{
		Zone: ZoneDTO{
			ID:               overview.Zone.ID,
			Slug:             overview.Zone.Slug,
			Name:             overview.Zone.Name,
			LogoURL:          brandingLogoURL(overview.Zone.Metadata),
			Kind:             overview.Zone.Kind,
			Status:           overview.Zone.Status,
			RegistrationMode: overview.Zone.RegistrationMode,
		},
		Domains:     domains,
		Deployments: deployments,
	}
}

func toDeploymentRequestDTO(request tenancydomain.DeploymentRequest) DeploymentRequestDTO {
	return DeploymentRequestDTO{
		ID:                    request.ID,
		ZoneID:                request.ZoneID,
		RequestedMode:         request.RequestedMode,
		RequestedDatabaseMode: request.RequestedDatabaseMode,
		Status:                request.Status,
		IdempotencyKey:        request.IdempotencyKey,
		FailureReason:         request.FailureReason,
		Metadata:              cloneMap(request.Metadata),
		CreatedAt:             request.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:             request.UpdatedAt.UTC().Format(time.RFC3339),
		CompletedAt:           formatOptionalTime(request.CompletedAt),
	}
}
