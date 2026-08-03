package http

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	nethttp "net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tenancyapp "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/application"
	tenantstorage "github.com/duclamdev/application-chat/backend/internal/platform/storage/tenant"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	brandingStoragePath     string
	storageResolver         *tenantstorage.Resolver
	service                 *tenancyapp.Service
	caddyAskSecret          string
	saasProvisioningEnabled bool
}

const maxBrandingLogoBytes = 4 << 20

var safeBrandingPathSegment = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func NewHandler(service *tenancyapp.Service, caddyAskSecrets ...string) *Handler {
	handler := &Handler{
		service:                 service,
		saasProvisioningEnabled: true,
	}
	if len(caddyAskSecrets) > 0 {
		handler.caddyAskSecret = strings.TrimSpace(caddyAskSecrets[0])
	}
	return handler
}

func (h *Handler) SetSaaSProvisioningEnabled(enabled bool) {
	h.saasProvisioningEnabled = enabled
}

func (h *Handler) SetBrandingStoragePath(path string) {
	h.brandingStoragePath = strings.TrimSpace(path)
}

func (h *Handler) SetStorageResolver(resolver *tenantstorage.Resolver) {
	h.storageResolver = resolver
}

type createDomainClaimRequest struct {
	Domain           string `json:"domain"`
	ZoneName         string `json:"zone_name"`
	RegistrationMode string `json:"registration_mode"`
}

type createAutomationInstallationRequest struct {
	WorkspaceID string         `json:"workspace_id"`
	TemplateKey string         `json:"template_key"`
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	Config      map[string]any `json:"config"`
	SecretRef   string         `json:"secret_ref"`
}

type updateAutomationInstallationRequest struct {
	Name      *string         `json:"name"`
	Status    *string         `json:"status"`
	Config    *map[string]any `json:"config"`
	SecretRef *string         `json:"secret_ref"`
}

type updateZoneSettingsRequest struct {
	Name             *string `json:"name"`
	RegistrationMode *string `json:"registration_mode"`
	LogoURL          *string `json:"logo_url"`
}

type zoneLifecycleRequest struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type createAdditionalDomainRequest struct {
	Domain string `json:"domain"`
	Kind   string `json:"kind"`
}

type createDeploymentRequest struct {
	RequestedMode         string `json:"requested_mode"`
	RequestedDatabaseMode string `json:"requested_database_mode"`
}

type updateZoneQuotaRequest struct {
	MaxWorkspaces              int    `json:"max_workspaces"`
	MaxMembers                 int    `json:"max_members"`
	MaxStorageBytes            int64  `json:"max_storage_bytes"`
	MaxAutomationInstallations int    `json:"max_automation_installations"`
	MaxWebhooks                int    `json:"max_webhooks"`
	EnforcementMode            string `json:"enforcement_mode"`
}

type createOIDCProviderRequest struct {
	Name                 string         `json:"name"`
	IssuerURL            string         `json:"issuer_url"`
	ClientID             string         `json:"client_id"`
	ClientSecretRef      string         `json:"client_secret_ref"`
	Scopes               []string       `json:"scopes"`
	ClaimMapping         map[string]any `json:"claim_mapping"`
	JITProvisioning      bool           `json:"jit_provisioning"`
	RequireVerifiedEmail *bool          `json:"require_verified_email"`
	Status               string         `json:"status"`
}

type updateOIDCProviderRequest struct {
	Name                 *string         `json:"name"`
	IssuerURL            *string         `json:"issuer_url"`
	ClientID             *string         `json:"client_id"`
	ClientSecretRef      *string         `json:"client_secret_ref"`
	Scopes               *[]string       `json:"scopes"`
	ClaimMapping         *map[string]any `json:"claim_mapping"`
	JITProvisioning      *bool           `json:"jit_provisioning"`
	RequireVerifiedEmail *bool           `json:"require_verified_email"`
	Status               *string         `json:"status"`
}

func (h *Handler) RegisterRoutes(
	engine *gin.Engine,
	v1 gin.IRouter,
	authMiddleware gin.HandlerFunc,
	recoveryAuthMiddleware ...gin.HandlerFunc,
) {
	engine.GET("/.well-known/vpsttt-chat", h.WellKnown)
	if h.saasProvisioningEnabled {
		engine.GET("/internal/tenancy/caddy-ask", h.CaddyAsk)
	}
	v1.GET("/discovery", h.Discovery)
	v1.GET("/capabilities", h.Capabilities)
	v1.GET("/branding/:zone_id/:file_name", h.ServeZoneBrandingLogo)

	private := v1.Group("/zones")
	private.Use(authMiddleware)
	private.GET("/current", h.GetCurrentZone)
	private.PATCH("/current", h.UpdateCurrentZone)
	private.POST("/current/logo", h.UploadCurrentZoneLogo)
	private.GET("/current/storage", h.GetCurrentZoneStorage)
	private.PUT("/current/storage", h.UpdateCurrentZoneStorage)
	private.GET("/current/quota", h.GetZoneQuota)
	private.PUT("/current/quota", h.UpdateZoneQuota)
	private.GET("/current/oidc-providers", h.ListOIDCProviders)
	private.POST("/current/oidc-providers", h.CreateOIDCProvider)
	private.PATCH("/current/oidc-providers/:provider_id", h.UpdateOIDCProvider)
	private.DELETE("/current/oidc-providers/:provider_id", h.DeleteOIDCProvider)
	private.GET("/current/automation-templates", h.ListAutomationTemplates)
	private.GET("/current/automation-installations", h.ListAutomationInstallations)
	private.POST("/current/automation-installations", h.CreateAutomationInstallation)
	private.PATCH("/current/automation-installations/:installation_id", h.UpdateAutomationInstallation)
	private.DELETE("/current/automation-installations/:installation_id", h.DeleteAutomationInstallation)
	if h.saasProvisioningEnabled {
		private.POST("/claims", h.CreateDomainClaim)
		private.GET("/claims/:domain_id", h.GetDomainClaim)
		private.POST("/claims/:domain_id/verify", h.VerifyDomainClaim)
		private.POST("/current/domains", h.CreateAdditionalDomain)
		private.POST("/current/domains/:domain_id/primary", h.SetPrimaryDomain)
		private.DELETE("/current/domains/:domain_id", h.DeleteCurrentZoneDomain)
		private.GET("/current/deployment-requests", h.ListDeploymentRequests)
		private.POST("/current/deployment-requests", h.CreateDeploymentRequest)
	}

	recoveryAuth := authMiddleware
	if len(recoveryAuthMiddleware) > 0 && recoveryAuthMiddleware[0] != nil {
		recoveryAuth = recoveryAuthMiddleware[0]
	}
	recovery := v1.Group("/zones")
	recovery.Use(recoveryAuth)
	recovery.POST("/current/lifecycle", h.SetCurrentZoneLifecycle)
}

func (h *Handler) UploadCurrentZoneLogo(c *gin.Context) {
	if h.storageResolver != nil {
		h.uploadCurrentZoneLogoToObjectStorage(c)
		return
	}
	zoneID := strings.TrimSpace(middleware.CurrentZoneID(c))
	if h.brandingStoragePath == "" {
		response.Fail(c, nethttp.StatusServiceUnavailable, "BRANDING_STORAGE_UNAVAILABLE", "Máy chủ chưa bật storage local cho logo.", nil)
		return
	}
	if !safeBrandingPathSegment.MatchString(zoneID) {
		response.Fail(c, nethttp.StatusBadRequest, "ZONE_REQUIRED", "Không xác định được vùng máy chủ hiện tại.", nil)
		return
	}
	if _, err := h.service.GetZoneAdminOverview(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		zoneID,
	); err != nil {
		response.Error(c, err)
		return
	}

	c.Request.Body = nethttp.MaxBytesReader(c.Writer, c.Request.Body, maxBrandingLogoBytes+(512<<10))
	header, err := c.FormFile("logo")
	if err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "LOGO_FILE_REQUIRED", "Hãy chọn một file logo PNG, JPEG hoặc WebP.", nil)
		return
	}
	if header.Size <= 0 || header.Size > maxBrandingLogoBytes {
		response.Fail(c, nethttp.StatusRequestEntityTooLarge, "LOGO_TOO_LARGE", "Logo không được vượt quá 4 MB.", nil)
		return
	}
	source, err := header.Open()
	if err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "LOGO_READ_FAILED", "Không đọc được file logo đã chọn.", nil)
		return
	}
	defer source.Close()
	content, err := io.ReadAll(io.LimitReader(source, maxBrandingLogoBytes+1))
	if err != nil || len(content) == 0 {
		response.Fail(c, nethttp.StatusBadRequest, "LOGO_READ_FAILED", "Không đọc được file logo đã chọn.", nil)
		return
	}
	if len(content) > maxBrandingLogoBytes {
		response.Fail(c, nethttp.StatusRequestEntityTooLarge, "LOGO_TOO_LARGE", "Logo không được vượt quá 4 MB.", nil)
		return
	}
	contentType, extension := brandingLogoType(content)
	if extension == "" {
		response.Fail(c, nethttp.StatusUnsupportedMediaType, "LOGO_TYPE_UNSUPPORTED", "Chỉ hỗ trợ logo PNG, JPEG hoặc WebP.", nil)
		return
	}

	sum := sha256.Sum256(content)
	fileName := "logo-" + hex.EncodeToString(sum[:6]) + extension
	targetDir := filepath.Join(h.brandingStoragePath, "public", "branding", zoneID)
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		response.Fail(c, nethttp.StatusInternalServerError, "LOGO_STORE_FAILED", "Không tạo được thư mục lưu logo.", nil)
		return
	}
	tempFile, err := os.CreateTemp(targetDir, ".logo-upload-*")
	if err != nil {
		response.Fail(c, nethttp.StatusInternalServerError, "LOGO_STORE_FAILED", "Không lưu được logo.", nil)
		return
	}
	tempName := tempFile.Name()
	defer os.Remove(tempName)
	if err := tempFile.Chmod(0o644); err != nil {
		tempFile.Close()
		response.Fail(c, nethttp.StatusInternalServerError, "LOGO_STORE_FAILED", "Không thiết lập được quyền đọc logo.", nil)
		return
	}
	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		response.Fail(c, nethttp.StatusInternalServerError, "LOGO_STORE_FAILED", "Không ghi được file logo.", nil)
		return
	}
	if err := tempFile.Close(); err != nil {
		response.Fail(c, nethttp.StatusInternalServerError, "LOGO_STORE_FAILED", "Không hoàn tất được file logo.", nil)
		return
	}
	targetPath := filepath.Join(targetDir, fileName)
	if err := os.Rename(tempName, targetPath); err != nil && !os.IsExist(err) {
		response.Fail(c, nethttp.StatusInternalServerError, "LOGO_STORE_FAILED", "Không hoàn tất được file logo.", nil)
		return
	}

	response.Created(c, gin.H{"logo": gin.H{
		"content_type": contentType,
		"logo_path":    "/branding/" + zoneID + "/" + fileName,
		"size":         len(content),
	}})
}

func brandingLogoType(content []byte) (string, string) {
	switch nethttp.DetectContentType(content) {
	case "image/png":
		return "image/png", ".png"
	case "image/jpeg":
		return "image/jpeg", ".jpg"
	case "image/webp":
		return "image/webp", ".webp"
	default:
		return "", ""
	}
}

func (h *Handler) CaddyAsk(c *gin.Context) {
	provided := strings.TrimSpace(c.Query("token"))
	if h.caddyAskSecret == "" ||
		subtle.ConstantTimeCompare([]byte(provided), []byte(h.caddyAskSecret)) != 1 {
		c.Status(nethttp.StatusForbidden)
		return
	}
	if !h.service.CertificateDomainAllowed(c.Request.Context(), c.Query("domain")) {
		c.Status(nethttp.StatusNotFound)
		return
	}
	c.Status(nethttp.StatusNoContent)
}

func (h *Handler) WellKnown(c *gin.Context) {
	discovery, err := h.service.Discover(c.Request.Context(), requestDomain(c))
	if err != nil {
		response.Error(c, err)
		return
	}

	c.JSON(nethttp.StatusOK, discovery)
}

func (h *Handler) Discovery(c *gin.Context) {
	discovery, err := h.service.Discover(c.Request.Context(), requestDomain(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"discovery": discovery})
}

func (h *Handler) Capabilities(c *gin.Context) {
	discovery, err := h.service.Discover(c.Request.Context(), requestDomain(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{
		"capabilities": discovery.Capabilities,
		"domain":       discovery.Domain,
		"zone":         discovery.Zone,
	})
}

func (h *Handler) CreateDomainClaim(c *gin.Context) {
	var req createDomainClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	claim, err := h.service.CreateDomainClaim(c.Request.Context(), tenancyapp.CreateDomainClaimInput{
		ActorUserID:      middleware.CurrentUserID(c),
		Domain:           req.Domain,
		ZoneName:         req.ZoneName,
		RegistrationMode: req.RegistrationMode,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, gin.H{"claim": claim})
}

func (h *Handler) GetDomainClaim(c *gin.Context) {
	claim, err := h.service.GetDomainClaim(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("domain_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"claim": claim})
}

func (h *Handler) VerifyDomainClaim(c *gin.Context) {
	discovery, err := h.service.VerifyDomainClaim(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		c.Param("domain_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"discovery": discovery})
}

func (h *Handler) GetCurrentZone(c *gin.Context) {
	overview, err := h.service.GetZoneAdminOverview(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"zone": overview})
}

func (h *Handler) UpdateCurrentZone(c *gin.Context) {
	var req updateZoneSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	overview, err := h.service.UpdateZoneSettings(c.Request.Context(), tenancyapp.UpdateZoneSettingsInput{
		ActorUserID:      middleware.CurrentUserID(c),
		ZoneID:           middleware.CurrentZoneID(c),
		Name:             req.Name,
		RegistrationMode: req.RegistrationMode,
		LogoURL:          req.LogoURL,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"zone": overview})
}

func (h *Handler) SetCurrentZoneLifecycle(c *gin.Context) {
	var req zoneLifecycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	err := h.service.SetZoneLifecycle(c.Request.Context(), tenancyapp.ZoneLifecycleInput{
		ActorUserID: middleware.CurrentUserID(c),
		ZoneID:      middleware.CurrentZoneID(c),
		Action:      req.Action,
		Reason:      req.Reason,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) CreateAdditionalDomain(c *gin.Context) {
	var req createAdditionalDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	claim, err := h.service.CreateAdditionalDomain(c.Request.Context(), tenancyapp.CreateAdditionalDomainInput{
		ActorUserID: middleware.CurrentUserID(c),
		ZoneID:      middleware.CurrentZoneID(c),
		Domain:      req.Domain,
		Kind:        req.Kind,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, gin.H{"claim": claim})
}

func (h *Handler) SetPrimaryDomain(c *gin.Context) {
	if err := h.service.SetPrimaryDomain(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
		c.Param("domain_id"),
	); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) DeleteCurrentZoneDomain(c *gin.Context) {
	if err := h.service.DeleteZoneDomain(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
		c.Param("domain_id"),
	); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) ListDeploymentRequests(c *gin.Context) {
	requests, err := h.service.ListDeploymentRequests(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"deployment_requests": requests})
}

func (h *Handler) CreateDeploymentRequest(c *gin.Context) {
	var req createDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	request, err := h.service.CreateDeploymentRequest(c.Request.Context(), tenancyapp.CreateDeploymentRequestInput{
		ActorUserID:           middleware.CurrentUserID(c),
		ZoneID:                middleware.CurrentZoneID(c),
		RequestedMode:         req.RequestedMode,
		RequestedDatabaseMode: req.RequestedDatabaseMode,
		IdempotencyKey:        c.GetHeader("Idempotency-Key"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, gin.H{"deployment_request": request})
}

func (h *Handler) GetZoneQuota(c *gin.Context) {
	quota, err := h.service.GetZoneQuota(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, quota)
}

func (h *Handler) UpdateZoneQuota(c *gin.Context) {
	var req updateZoneQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	quota, err := h.service.UpdateZoneQuota(c.Request.Context(), tenancyapp.UpdateZoneQuotaInput{
		ActorUserID:                middleware.CurrentUserID(c),
		ZoneID:                     middleware.CurrentZoneID(c),
		MaxWorkspaces:              req.MaxWorkspaces,
		MaxMembers:                 req.MaxMembers,
		MaxStorageBytes:            req.MaxStorageBytes,
		MaxAutomationInstallations: req.MaxAutomationInstallations,
		MaxWebhooks:                req.MaxWebhooks,
		EnforcementMode:            req.EnforcementMode,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, quota)
}

func (h *Handler) ListOIDCProviders(c *gin.Context) {
	providers, err := h.service.ListOIDCProviders(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"oidc_providers": providers})
}

func (h *Handler) CreateOIDCProvider(c *gin.Context) {
	var req createOIDCProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	provider, err := h.service.CreateOIDCProvider(c.Request.Context(), tenancyapp.CreateOIDCProviderInput{
		ActorUserID:          middleware.CurrentUserID(c),
		ZoneID:               middleware.CurrentZoneID(c),
		Name:                 req.Name,
		IssuerURL:            req.IssuerURL,
		ClientID:             req.ClientID,
		ClientSecretRef:      req.ClientSecretRef,
		Scopes:               req.Scopes,
		ClaimMapping:         req.ClaimMapping,
		JITProvisioning:      req.JITProvisioning,
		RequireVerifiedEmail: req.RequireVerifiedEmail == nil || *req.RequireVerifiedEmail,
		Status:               req.Status,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, gin.H{"oidc_provider": provider})
}

func (h *Handler) UpdateOIDCProvider(c *gin.Context) {
	var req updateOIDCProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	provider, err := h.service.UpdateOIDCProvider(c.Request.Context(), tenancyapp.UpdateOIDCProviderInput{
		ActorUserID:          middleware.CurrentUserID(c),
		ZoneID:               middleware.CurrentZoneID(c),
		ProviderID:           c.Param("provider_id"),
		Name:                 req.Name,
		IssuerURL:            req.IssuerURL,
		ClientID:             req.ClientID,
		ClientSecretRef:      req.ClientSecretRef,
		Scopes:               req.Scopes,
		ClaimMapping:         req.ClaimMapping,
		JITProvisioning:      req.JITProvisioning,
		RequireVerifiedEmail: req.RequireVerifiedEmail,
		Status:               req.Status,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"oidc_provider": provider})
}

func (h *Handler) DeleteOIDCProvider(c *gin.Context) {
	err := h.service.DeleteOIDCProvider(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
		c.Param("provider_id"),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) ListAutomationTemplates(c *gin.Context) {
	templates, err := h.service.ListAutomationTemplates(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"templates": templates})
}

func (h *Handler) ListAutomationInstallations(c *gin.Context) {
	installations, err := h.service.ListAutomationInstallations(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"installations": installations})
}

func (h *Handler) CreateAutomationInstallation(c *gin.Context) {
	var req createAutomationInstallationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	installation, err := h.service.CreateAutomationInstallation(
		c.Request.Context(),
		tenancyapp.CreateAutomationInstallationInput{
			ActorUserID: middleware.CurrentUserID(c),
			ZoneID:      middleware.CurrentZoneID(c),
			WorkspaceID: req.WorkspaceID,
			TemplateKey: req.TemplateKey,
			Name:        req.Name,
			Status:      req.Status,
			Config:      req.Config,
			SecretRef:   req.SecretRef,
		},
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, gin.H{"installation": installation})
}

func (h *Handler) UpdateAutomationInstallation(c *gin.Context) {
	var req updateAutomationInstallationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	installation, err := h.service.UpdateAutomationInstallation(
		c.Request.Context(),
		tenancyapp.UpdateAutomationInstallationInput{
			ActorUserID:    middleware.CurrentUserID(c),
			ZoneID:         middleware.CurrentZoneID(c),
			InstallationID: c.Param("installation_id"),
			Name:           req.Name,
			Status:         req.Status,
			Config:         req.Config,
			SecretRef:      req.SecretRef,
		},
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"installation": installation})
}

func (h *Handler) DeleteAutomationInstallation(c *gin.Context) {
	if err := h.service.DeleteAutomationInstallation(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
		c.Param("installation_id"),
	); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func requestDomain(c *gin.Context) string {
	if value := strings.TrimSpace(c.Query("domain")); value != "" {
		return value
	}
	return c.Request.Host
}
