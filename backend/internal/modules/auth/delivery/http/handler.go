package http

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	nethttp "net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	authapp "github.com/duclamdev/application-chat/backend/internal/modules/auth/application"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service        *authapp.Service
	oidcService    *authapp.OIDCService
	googleClientID string
	httpClient     *nethttp.Client
	instanceDomain string
}

type registerRequest struct {
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Domain      string `json:"domain"`
	InviteToken string `json:"invite_token"`
	Password    string `json:"password"`
	DeviceName  string `json:"device_name"`
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

type googleLoginRequest struct {
	Credential string `json:"credential"`
	DeviceName string `json:"device_name"`
	Domain     string `json:"domain"`
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

type googleTokenInfo struct {
	Audience      string `json:"aud"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	ExpiresAt     string `json:"exp"`
	Issuer        string `json:"iss"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Subject       string `json:"sub"`
}

func NewHandler(service *authapp.Service, googleClientIDs ...string) *Handler {
	clientID := ""
	if len(googleClientIDs) > 0 {
		clientID = strings.TrimSpace(googleClientIDs[0])
	}
	return &Handler{
		service:        service,
		googleClientID: clientID,
		httpClient:     &nethttp.Client{Timeout: 10 * time.Second},
	}
}

func (h *Handler) SetOIDCService(service *authapp.OIDCService) {
	h.oidcService = service
}

func (h *Handler) SetInstanceDomain(domain string) {
	h.instanceDomain = strings.TrimSpace(domain)
}

func (h *Handler) RegisterRoutes(router gin.IRouter, authMiddleware gin.HandlerFunc) {
	router.POST("/register", h.Register)
	router.POST("/login", h.Login)
	router.POST("/google", h.GoogleLogin)
	router.GET("/oidc/providers", h.ListOIDCProviders)
	router.POST("/oidc/start", h.StartOIDC)
	router.GET("/oidc/callback", h.OIDCCallback)
	router.POST("/oidc/complete", h.CompleteOIDC)
	router.POST("/refresh", h.Refresh)
	router.POST("/logout", h.Logout)

	private := router.Group("")
	private.Use(authMiddleware)
	private.GET("/me", h.Me)
	private.GET("/sessions", h.ListSessions)
	private.DELETE("/sessions/:session_id", h.RevokeSession)
	private.DELETE("/sessions", h.RevokeAllSessions)
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
		response.Fail(c, nethttp.StatusServiceUnavailable, "OIDC_NOT_CONFIGURED", "OIDC SSO runtime chua duoc cau hinh.", nil)
		return
	}
	var req oidcStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON khong hop le.", nil)
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
		response.Fail(c, nethttp.StatusServiceUnavailable, "OIDC_NOT_CONFIGURED", "OIDC SSO runtime chua duoc cau hinh.", nil)
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
		response.Fail(c, nethttp.StatusServiceUnavailable, "OIDC_NOT_CONFIGURED", "OIDC SSO runtime chua duoc cau hinh.", nil)
		return
	}
	var req oidcCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON khong hop le.", nil)
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

func (h *Handler) GoogleLogin(c *gin.Context) {
	if h.googleClientID == "" {
		response.Fail(c, nethttp.StatusServiceUnavailable, "GOOGLE_AUTH_NOT_CONFIGURED", "Đăng nhập Google chưa được cấu hình.", nil)
		return
	}
	var req googleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Credential) == "" {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_GOOGLE_CREDENTIAL", "Thiếu thông tin xác thực Google.", nil)
		return
	}
	profile, err := h.verifyGoogleCredential(c.Request.Context(), req.Credential)
	if err != nil {
		response.Fail(c, nethttp.StatusUnauthorized, "INVALID_GOOGLE_CREDENTIAL", "Phiên xác thực Google không hợp lệ hoặc đã hết hạn.", nil)
		return
	}
	result, err := h.service.LoginWithGoogle(c.Request.Context(), authapp.GoogleLoginInput{
		Subject:       profile.Subject,
		Email:         profile.Email,
		EmailVerified: profile.EmailVerified == "true",
		DisplayName:   profile.Name,
		AvatarURL:     profile.Picture,
		Domain:        h.authRequestDomain(c, req.Domain),
		DeviceName:    req.DeviceName,
		IPAddress:     clientIP(c),
		UserAgent:     c.Request.UserAgent(),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nethttp.StatusOK, result)
}

func (h *Handler) verifyGoogleCredential(ctx context.Context, credential string) (googleTokenInfo, error) {
	var profile googleTokenInfo
	endpoint := "https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(strings.TrimSpace(credential))
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, endpoint, nil)
	if err != nil {
		return profile, err
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return profile, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != nethttp.StatusOK {
		return profile, errors.New("google tokeninfo rejected credential")
	}
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return profile, err
	}
	expiresAt, err := strconv.ParseInt(profile.ExpiresAt, 10, 64)
	if err != nil || expiresAt <= time.Now().Unix() {
		return profile, errors.New("google credential expired")
	}
	if profile.Audience != h.googleClientID || (profile.Issuer != "accounts.google.com" && profile.Issuer != "https://accounts.google.com") || profile.Subject == "" || profile.Email == "" || profile.EmailVerified != "true" {
		return profile, errors.New("google credential claims are invalid")
	}
	return profile, nil
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, nethttp.StatusBadRequest, "INVALID_JSON", "Body JSON không hợp lệ.", nil)
		return
	}

	result, err := h.service.Register(c.Request.Context(), authapp.RegisterInput{
		Email:       req.Email,
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Domain:      h.authRequestDomain(c, req.Domain),
		InviteToken: req.InviteToken,
		Password:    req.Password,
		DeviceName:  req.DeviceName,
		IPAddress:   clientIP(c),
		UserAgent:   c.Request.UserAgent(),
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
