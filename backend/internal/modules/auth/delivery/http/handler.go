package http

import (
	"net"
	nethttp "net/http"
	"strings"

	authapp "github.com/duclamdev/application-chat/backend/internal/modules/auth/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service        *authapp.Service
	oidcService    *authapp.OIDCService
	instanceDomain string
}

type registerRequest struct {
	Email           string `json:"email"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	Domain          string `json:"domain"`
	InviteToken     string `json:"invite_token"`
	Password        string `json:"password"`
	DeviceName      string `json:"device_name"`
	TermsAccepted   bool   `json:"terms_accepted"`
	TermsVersion    string `json:"terms_version"`
	PrivacyAccepted bool   `json:"privacy_accepted"`
	PrivacyVersion  string `json:"privacy_version"`
}

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
	Domain     string `json:"domain"`
	DeviceName string `json:"device_name"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	Domain       string `json:"domain"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type legalAcceptanceRequest struct {
	WorkspaceID     string `json:"workspace_id"`
	TermsAccepted   bool   `json:"terms_accepted"`
	TermsVersion    string `json:"terms_version"`
	PrivacyAccepted bool   `json:"privacy_accepted"`
	PrivacyVersion  string `json:"privacy_version"`
}

type oidcStartRequest struct {
	Domain     string `json:"domain"`
	ProviderID string `json:"provider_id"`
	ReturnTo   string `json:"return_to"`
	DeviceName string `json:"device_name"`
}

type oidcCompleteRequest struct {
	Code       string `json:"code"`
	Domain     string `json:"domain"`
	DeviceName string `json:"device_name"`
}

func NewHandler(service *authapp.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) SetOIDCService(service *authapp.OIDCService) {
	h.oidcService = service
}

func (h *Handler) SetInstanceDomain(domain string) {
	h.instanceDomain = strings.TrimSpace(domain)
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	router.GET("/legal-documents", h.LegalDocuments)
	router.POST("/register", h.Register)
	router.POST("/login", h.Login)
	router.GET("/oidc/providers", h.ListOIDCProviders)
	router.POST("/oidc/start", h.StartOIDC)
	router.GET("/oidc/callback", h.OIDCCallback)
	router.POST("/oidc/complete", h.CompleteOIDC)
	router.POST("/refresh", h.Refresh)
	router.POST("/logout", h.Logout)

	private := router.Group("")
	private.Use(authMiddleware)
	private.GET("/legal-acceptance", h.GetLegalAcceptance)
	private.POST("/legal-acceptance", h.AcceptLegalDocuments)
	private.GET("/me", h.Me)
	private.GET("/sessions", h.ListSessions)
	private.DELETE("/sessions/:session_id", h.RevokeSession)
	private.DELETE("/sessions", h.RevokeAllSessions)
}

func (h *Handler) LegalDocuments(c *gin.Context) {
	versions := h.service.LegalDocumentVersions()
	response.OK(c, nethttp.StatusOK, gin.H{
		"documents": []gin.H{
			{
				"document_type": "terms",
				"version":       versions.Terms,
				"includes":      []string{"terms_of_use", "acceptable_use_policy"},
			},
			{
				"document_type": "privacy",
				"version":       versions.PrivacyPolicy,
				"includes":      []string{"privacy_policy"},
			},
		},
	})
}

func (h *Handler) GetLegalAcceptance(c *gin.Context) {
	workspaceID := legalAcceptanceWorkspaceID(c.Query("workspace_id"), middleware.CurrentWorkspaceID(c))
	status, err := h.service.GetCurrentLegalAcceptance(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		workspaceID,
		middleware.CurrentZoneID(c),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"legal_acceptance": status})
}

func (h *Handler) AcceptLegalDocuments(c *gin.Context) {
	var req legalAcceptanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Request body must be valid JSON.", nil)
		return
	}
	workspaceID := legalAcceptanceWorkspaceID(req.WorkspaceID, middleware.CurrentWorkspaceID(c))
	status, err := h.service.AcceptCurrentLegalDocuments(c.Request.Context(), authapp.AcceptLegalDocumentsInput{
		UserID: middleware.CurrentUserID(c), WorkspaceID: workspaceID, ZoneID: middleware.CurrentZoneID(c),
		TermsAccepted: req.TermsAccepted, TermsVersion: req.TermsVersion,
		PrivacyAccepted: req.PrivacyAccepted, PrivacyVersion: req.PrivacyVersion,
		IPAddress: clientIP(c), UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"legal_acceptance": status})
}

func legalAcceptanceWorkspaceID(explicit string, tokenWorkspace string) string {
	if workspaceID := strings.TrimSpace(explicit); workspaceID != "" {
		return workspaceID
	}
	return strings.TrimSpace(tokenWorkspace)
}

func (h *Handler) ListOIDCProviders(c *gin.Context) {
	if h.oidcService == nil {
		response.OK(c, nethttp.StatusOK, gin.H{"oidc_providers": []authapp.PublicOIDCProviderDTO{}})
		return
	}
	providers, err := h.oidcService.ListProviders(
		c.Request.Context(),
		h.authRequestDomain(c, c.Query("domain")),
	)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"oidc_providers": providers})
}

func (h *Handler) StartOIDC(c *gin.Context) {
	if h.oidcService == nil {
		response.Fail(c, nethttp.StatusServiceUnavailable, "OIDC_NOT_CONFIGURED", "Runtime OIDC SSO chưa được cấu hình.", nil)
		return
	}
	var req oidcStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	result, err := h.oidcService.Start(c.Request.Context(), authapp.OIDCStartInput{
		Domain:      h.authRequestDomain(c, req.Domain),
		ProviderID:  req.ProviderID,
		RedirectURI: requestOrigin(c) + "/api/v1/auth/oidc/callback",
		ReturnTo:    req.ReturnTo,
		DeviceName:  req.DeviceName,
		IPAddress:   clientIP(c),
		UserAgent:   c.Request.UserAgent(),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, result)
}

func (h *Handler) OIDCCallback(c *gin.Context) {
	if h.oidcService == nil {
		response.Fail(c, nethttp.StatusServiceUnavailable, "OIDC_NOT_CONFIGURED", "Runtime OIDC SSO chưa được cấu hình.", nil)
		return
	}
	result, err := h.oidcService.Callback(c.Request.Context(), authapp.OIDCCallbackInput{
		State:         c.Query("state"),
		Code:          c.Query("code"),
		ProviderError: c.Query("error"),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	c.Redirect(nethttp.StatusSeeOther, result.RedirectURL)
}

func (h *Handler) CompleteOIDC(c *gin.Context) {
	if h.oidcService == nil {
		response.Fail(c, nethttp.StatusServiceUnavailable, "OIDC_NOT_CONFIGURED", "Runtime OIDC SSO chưa được cấu hình.", nil)
		return
	}
	var req oidcCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Nội dung JSON không hợp lệ.", nil)
		return
	}
	result, err := h.oidcService.Complete(c.Request.Context(), authapp.OIDCCompleteInput{
		Code:       req.Code,
		Domain:     h.authRequestDomain(c, req.Domain),
		DeviceName: req.DeviceName,
		IPAddress:  clientIP(c),
		UserAgent:  c.Request.UserAgent(),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, result)
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}

	result, err := h.service.Register(c.Request.Context(), authapp.RegisterInput{
		Email:           req.Email,
		Username:        req.Username,
		DisplayName:     req.DisplayName,
		Domain:          h.authRequestDomain(c, req.Domain),
		InviteToken:     req.InviteToken,
		Password:        req.Password,
		DeviceName:      req.DeviceName,
		IPAddress:       clientIP(c),
		UserAgent:       c.Request.UserAgent(),
		TermsAccepted:   req.TermsAccepted,
		TermsVersion:    req.TermsVersion,
		PrivacyAccepted: req.PrivacyAccepted,
		PrivacyVersion:  req.PrivacyVersion,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, result)
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}

	result, err := h.service.Login(c.Request.Context(), authapp.LoginInput{
		Identifier: req.Identifier,
		Password:   req.Password,
		Domain:     h.authRequestDomain(c, req.Domain),
		DeviceName: req.DeviceName,
		IPAddress:  clientIP(c),
		UserAgent:  c.Request.UserAgent(),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, result)
}

func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}

	result, err := h.service.Refresh(c.Request.Context(), authapp.RefreshInput{
		RefreshToken: req.RefreshToken,
		Domain:       h.authRequestDomain(c, req.Domain),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, result)
}

func (h *Handler) Logout(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}

	if err := h.service.Logout(c.Request.Context(), authapp.LogoutInput{RefreshToken: req.RefreshToken}); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"status": "logged_out"})
}

func (h *Handler) Me(c *gin.Context) {
	user, err := h.service.Me(c.Request.Context(), middleware.CurrentUserID(c), middleware.CurrentZoneDomain(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, user)
}

func (h *Handler) ListSessions(c *gin.Context) {
	sessions, err := h.service.ListSessions(c.Request.Context(), middleware.CurrentUserID(c), middleware.CurrentZoneID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, gin.H{"sessions": sessions})
}

func (h *Handler) RevokeSession(c *gin.Context) {
	if err := h.service.RevokeSession(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
		c.Param("session_id"),
	); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) RevokeAllSessions(c *gin.Context) {
	if err := h.service.RevokeAllSessions(
		c.Request.Context(),
		middleware.CurrentUserID(c),
		middleware.CurrentZoneID(c),
	); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) authRequestDomain(c *gin.Context, bodyDomain string) string {
	if h.instanceDomain != "" {
		return h.instanceDomain
	}
	if value := strings.TrimSpace(bodyDomain); value != "" {
		return value
	}
	if host := strings.TrimSpace(c.Request.Host); host != "" {
		return host
	}
	return strings.TrimSpace(bodyDomain)
}

func requestOrigin(c *gin.Context) string {
	scheme := "https"
	host := strings.TrimSpace(c.Request.Host)
	hostname := host
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		hostname = parsedHost
	}
	hostname = strings.ToLower(strings.Trim(strings.TrimSpace(hostname), "[]"))
	if c.Request.TLS == nil && (hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") ||
		hostname == "127.0.0.1" || hostname == "::1") {
		scheme = "http"
	}
	if forwarded := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}
	return scheme + "://" + host
}
