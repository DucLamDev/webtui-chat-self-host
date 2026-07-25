package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"

	authdomain "github.com/duclamdev/application-chat/backend/internal/modules/auth/domain"
	tenancyapp "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/application"
	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{2,39}$`)

type Repository interface {
	CreateUser(ctx context.Context, params CreateUserParams) (authdomain.User, error)
	ResolveZoneAccess(ctx context.Context, domain string) (ZoneAccess, error)
	EnsureZoneWorkspaceAccess(ctx context.Context, userID string, target ZoneAccess) error
	FindUserByID(ctx context.Context, id string) (authdomain.User, error)
	FindUserByIdentifier(ctx context.Context, identifier string) (authdomain.User, error)
	UpdateLastLoginInfo(ctx context.Context, params UpdateLastLoginInfoParams) error
	CreateSession(ctx context.Context, params CreateSessionParams) (authdomain.Session, error)
	FindSessionByRefreshTokenHash(ctx context.Context, hash string) (authdomain.Session, error)
	RotateSessionRefreshToken(ctx context.Context, params RotateSessionParams) (authdomain.Session, error)
	RevokeSessionByRefreshTokenHash(ctx context.Context, hash string, revokedAt time.Time) error
	RevokeSessionByID(ctx context.Context, userID string, zoneID string, sessionID string, revokedAt time.Time) error
	RevokeAllSessions(ctx context.Context, userID string, zoneID string, revokedAt time.Time) error
	ListSessions(ctx context.Context, userID string, zoneID string) ([]authdomain.Session, error)
	RecordAudit(ctx context.Context, event AuditEvent) error
}

type Service struct {
	repo   Repository
	tokens *sharedauth.Manager
	now    func() time.Time
}

type RegisterInput struct {
	Email       string
	Username    string
	DisplayName string
	Domain      string
	InviteToken string
	Password    string
	DeviceName  string
	IPAddress   string
	UserAgent   string
}

type LoginInput struct {
	Identifier string
	Password   string
	Domain     string
	DeviceName string
	IPAddress  string
	UserAgent  string
}

type GoogleLoginInput struct {
	Subject       string
	Email         string
	EmailVerified bool
	DisplayName   string
	AvatarURL     string
	Domain        string
	DeviceName    string
	IPAddress     string
	UserAgent     string
}

type RefreshInput struct {
	RefreshToken string
	Domain       string
}

type LogoutInput struct {
	RefreshToken string
}

type CreateUserParams struct {
	Email         string
	Username      string
	DisplayName   string
	PasswordHash  string
	DeviceName    string
	IPAddress     string
	UserAgent     string
	AvatarURL     string
	EmailVerified bool
	Zone          ZoneAccess
	InviteToken   string
}

type ZoneAccess struct {
	ZoneID           string
	ZoneSlug         string
	ZoneName         string
	ZoneKind         string
	ZoneStatus       string
	RegistrationMode string
	WorkspaceID      string
	WorkspaceSlug    string
	Domain           string
}

type UpdateLastLoginInfoParams struct {
	UserID     string
	SeenAt     time.Time
	DeviceName string
	IPAddress  string
	UserAgent  string
}

type CreateSessionParams struct {
	UserID           string
	ZoneID           string
	WorkspaceID      string
	Domain           string
	RefreshTokenHash string
	DeviceName       string
	IPAddress        string
	UserAgent        string
	ExpiresAt        time.Time
}

type RotateSessionParams struct {
	SessionID        string
	UserID           string
	RefreshTokenHash string
	ExpiresAt        time.Time
}

type AuditEvent struct {
	ActorUserID string
	ZoneID      string
	WorkspaceID string
	Action      string
	EntityType  string
	EntityID    string
	IPAddress   string
	UserAgent   string
	Metadata    map[string]any
}

type AuthResult struct {
	User         UserDTO     `json:"user"`
	Tokens       TokenDTO    `json:"tokens"`
	Zone         AuthZoneDTO `json:"zone"`
	SessionID    string      `json:"session_id"`
	RefreshUntil string      `json:"refresh_until"`
}

type RefreshResult struct {
	Tokens       TokenDTO    `json:"tokens"`
	Zone         AuthZoneDTO `json:"zone"`
	SessionID    string      `json:"session_id"`
	RefreshUntil string      `json:"refresh_until"`
}

type TokenDTO struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token,omitempty"`
	TokenType             string `json:"token_type"`
	AccessTokenExpiresAt  string `json:"access_token_expires_at"`
	RefreshTokenExpiresAt string `json:"refresh_token_expires_at,omitempty"`
}

type AuthZoneDTO struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Domain      string `json:"domain"`
	WorkspaceID string `json:"workspace_id"`
}

type UserDTO struct {
	ID              string  `json:"id"`
	Email           string  `json:"email"`
	Username        string  `json:"username"`
	DisplayName     string  `json:"display_name"`
	AvatarURL       *string `json:"avatar_url,omitempty"`
	Status          string  `json:"status"`
	Locale          string  `json:"locale"`
	Timezone        string  `json:"timezone"`
	EmailVerifiedAt *string `json:"email_verified_at,omitempty"`
	LastSeenAt      *string `json:"last_seen_at,omitempty"`
	RegistrationIP  *string `json:"registration_ip_address,omitempty"`
	RegistrationDev *string `json:"registration_device_name,omitempty"`
	LastIPAddress   *string `json:"last_ip_address,omitempty"`
	DeviceName      *string `json:"device_name,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type SessionDTO struct {
	ID          string  `json:"id"`
	ZoneID      string  `json:"zone_id"`
	WorkspaceID string  `json:"workspace_id"`
	Domain      string  `json:"domain"`
	DeviceName  *string `json:"device_name,omitempty"`
	IPAddress   *string `json:"ip_address,omitempty"`
	UserAgent   *string `json:"user_agent,omitempty"`
	ExpiresAt   string  `json:"expires_at"`
	RevokedAt   *string `json:"revoked_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

func NewService(repo Repository, tokens *sharedauth.Manager) *Service {
	return &Service{
		repo:   repo,
		tokens: tokens,
		now:    time.Now,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	normalized, err := normalizeRegister(input)
	if err != nil {
		return AuthResult{}, err
	}
	target, err := s.resolveZoneAccess(ctx, normalized.Domain)
	if err != nil {
		return AuthResult{}, err
	}
	if target.ZoneStatus != "active" {
		return AuthResult{}, apperrors.Forbidden("Zone dang tam dung va khong nhan dang ky moi.")
	}

	passwordHash, err := sharedauth.HashPassword(normalized.Password)
	if err != nil {
		return AuthResult{}, apperrors.Internal("Không hash được mật khẩu.")
	}

	user, err := s.repo.CreateUser(ctx, CreateUserParams{
		Email:        normalized.Email,
		Username:     normalized.Username,
		DisplayName:  normalized.DisplayName,
		PasswordHash: passwordHash,
		DeviceName:   normalized.DeviceName,
		IPAddress:    normalized.IPAddress,
		UserAgent:    normalized.UserAgent,
		Zone:         target,
		InviteToken:  normalized.InviteToken,
	})
	if err != nil {
		if errors.Is(err, authdomain.ErrUserAlreadyExists) {
			return AuthResult{}, apperrors.Conflict("USER_ALREADY_EXISTS", "Email hoặc username đã tồn tại.")
		}
		if errors.Is(err, authdomain.ErrInviteRequired) {
			return AuthResult{}, apperrors.Forbidden("Zone nay yeu cau invite token hop le de dang ky.")
		}
		if errors.Is(err, authdomain.ErrRegistrationClosed) {
			return AuthResult{}, apperrors.Forbidden("Zone nay dang dong dang ky tai khoan moi.")
		}
		return AuthResult{}, err
	}

	result, err := s.issueTokens(ctx, user, target, normalized.DeviceName, normalized.IPAddress, normalized.UserAgent)
	if err != nil {
		return AuthResult{}, err
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: user.ID,
		Action:      "auth.register",
		EntityType:  "user",
		EntityID:    user.ID,
		IPAddress:   normalized.IPAddress,
		UserAgent:   normalized.UserAgent,
		ZoneID:      target.ZoneID,
		WorkspaceID: target.WorkspaceID,
		Metadata:    map[string]any{"domain": target.Domain},
	})
	return result, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	input = normalizeLogin(input)
	identifier := input.Identifier
	if identifier == "" || strings.TrimSpace(input.Password) == "" {
		return AuthResult{}, apperrors.BadRequest("VALIDATION_ERROR", "Thông tin đăng nhập không được để trống.")
	}
	target, err := s.resolveZoneAccess(ctx, input.Domain)
	if err != nil {
		return AuthResult{}, err
	}

	user, err := s.repo.FindUserByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, authdomain.ErrUserNotFound) {
			s.recordFailedLogin(ctx, "", input, target, "invalid_credentials")
			return AuthResult{}, apperrors.Unauthorized("Email, username hoặc mật khẩu không đúng.")
		}
		return AuthResult{}, err
	}
	if user.Status != "active" {
		s.recordFailedLogin(ctx, user.ID, input, target, "account_"+user.Status)
		return AuthResult{}, apperrors.Forbidden("Tài khoản chưa sẵn sàng hoặc đã bị khóa.")
	}
	if !sharedauth.VerifyPassword(user.PasswordHash, input.Password) {
		s.recordFailedLogin(ctx, user.ID, input, target, "invalid_credentials")
		return AuthResult{}, apperrors.Unauthorized("Email, username hoặc mật khẩu không đúng.")
	}

	if err := s.ensureZoneWorkspaceAccess(ctx, user.ID, target); err != nil {
		return AuthResult{}, err
	}

	result, err := s.issueTokens(ctx, user, target, input.DeviceName, input.IPAddress, input.UserAgent)
	if err != nil {
		return AuthResult{}, err
	}
	_ = s.repo.UpdateLastLoginInfo(ctx, UpdateLastLoginInfoParams{
		UserID:     user.ID,
		SeenAt:     s.now().UTC(),
		DeviceName: input.DeviceName,
		IPAddress:  input.IPAddress,
		UserAgent:  input.UserAgent,
	})
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: user.ID,
		Action:      "auth.login",
		EntityType:  "user",
		EntityID:    user.ID,
		IPAddress:   input.IPAddress,
		UserAgent:   input.UserAgent,
		ZoneID:      target.ZoneID,
		WorkspaceID: target.WorkspaceID,
		Metadata:    map[string]any{"domain": target.Domain},
	})
	return result, nil
}

func (s *Service) recordFailedLogin(
	ctx context.Context,
	userID string,
	input LoginInput,
	target ZoneAccess,
	reason string,
) {
	// Chỉ lưu dấu vân tay một chiều của định danh; tuyệt đối không ghi password,
	// token hoặc email/username dạng rõ vào audit log.
	fingerprint := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(input.Identifier))))
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: userID,
		Action:      "auth.login_failed",
		EntityType:  "user",
		EntityID:    userID,
		IPAddress:   input.IPAddress,
		UserAgent:   input.UserAgent,
		ZoneID:      target.ZoneID,
		WorkspaceID: target.WorkspaceID,
		Metadata: map[string]any{
			"reason":            reason,
			"domain":            target.Domain,
			"identifier_sha256": hex.EncodeToString(fingerprint[:]),
		},
	})
}

func (s *Service) LoginWithGoogle(ctx context.Context, input GoogleLoginInput) (AuthResult, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Subject = strings.TrimSpace(input.Subject)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.AvatarURL = strings.TrimSpace(input.AvatarURL)
	input.DeviceName, input.IPAddress, input.UserAgent = normalizeClientInfo(input.DeviceName, input.IPAddress, input.UserAgent)
	if input.Subject == "" || !input.EmailVerified {
		return AuthResult{}, apperrors.Unauthorized("Tài khoản Google chưa xác minh email.")
	}
	if _, err := mail.ParseAddress(input.Email); err != nil {
		return AuthResult{}, apperrors.Unauthorized("Google không trả về email hợp lệ.")
	}

	target, err := s.resolveZoneAccess(ctx, input.Domain)
	if err != nil {
		return AuthResult{}, err
	}

	user, err := s.repo.FindUserByIdentifier(ctx, input.Email)
	if errors.Is(err, authdomain.ErrUserNotFound) {
		if target.ZoneStatus != "active" {
			return AuthResult{}, apperrors.Forbidden("Zone dang tam dung va khong nhan tai khoan Google moi.")
		}
		passwordBytes := make([]byte, 32)
		if _, randomErr := rand.Read(passwordBytes); randomErr != nil {
			return AuthResult{}, apperrors.Internal("Không tạo được thông tin tài khoản Google.")
		}
		passwordHash, hashErr := sharedauth.HashPassword(base64.RawURLEncoding.EncodeToString(passwordBytes))
		if hashErr != nil {
			return AuthResult{}, apperrors.Internal("Không tạo được thông tin tài khoản Google.")
		}
		displayName := input.DisplayName
		if displayName == "" {
			displayName = strings.Split(input.Email, "@")[0]
		}
		user, err = s.repo.CreateUser(ctx, CreateUserParams{
			Email:         input.Email,
			Username:      googleUsername(input.Email, input.Subject),
			DisplayName:   displayName,
			PasswordHash:  passwordHash,
			DeviceName:    input.DeviceName,
			IPAddress:     input.IPAddress,
			UserAgent:     input.UserAgent,
			AvatarURL:     input.AvatarURL,
			EmailVerified: true,
			Zone:          target,
		})
		if errors.Is(err, authdomain.ErrUserAlreadyExists) {
			user, err = s.repo.FindUserByIdentifier(ctx, input.Email)
		}
	}
	if err != nil {
		return AuthResult{}, err
	}
	if user.Status != "active" {
		return AuthResult{}, apperrors.Forbidden("Tài khoản chưa sẵn sàng hoặc đã bị khóa.")
	}
	if err := s.ensureZoneWorkspaceAccess(ctx, user.ID, target); err != nil {
		return AuthResult{}, err
	}

	result, err := s.issueTokens(ctx, user, target, input.DeviceName, input.IPAddress, input.UserAgent)
	if err != nil {
		return AuthResult{}, err
	}
	_ = s.repo.UpdateLastLoginInfo(ctx, UpdateLastLoginInfoParams{
		UserID:     user.ID,
		SeenAt:     s.now().UTC(),
		DeviceName: input.DeviceName,
		IPAddress:  input.IPAddress,
		UserAgent:  input.UserAgent,
	})
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: user.ID,
		Action:      "auth.google_login",
		EntityType:  "user",
		EntityID:    user.ID,
		IPAddress:   input.IPAddress,
		UserAgent:   input.UserAgent,
		ZoneID:      target.ZoneID,
		WorkspaceID: target.WorkspaceID,
		Metadata:    map[string]any{"provider": "google", "domain": target.Domain},
	})
	return result, nil
}

func (s *Service) Refresh(ctx context.Context, input RefreshInput) (RefreshResult, error) {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken == "" {
		return RefreshResult{}, apperrors.BadRequest("VALIDATION_ERROR", "Refresh token không được để trống.")
	}

	session, err := s.repo.FindSessionByRefreshTokenHash(ctx, s.tokens.HashRefreshToken(refreshToken))
	if err != nil {
		if errors.Is(err, authdomain.ErrSessionNotFound) {
			return RefreshResult{}, apperrors.Unauthorized("Refresh token không hợp lệ.")
		}
		return RefreshResult{}, err
	}
	if err := session.Active(s.now().UTC()); err != nil {
		return RefreshResult{}, apperrors.Unauthorized("Phiên đăng nhập không còn hiệu lực.")
	}

	target, err := s.resolveZoneAccess(ctx, session.Domain)
	if err != nil {
		return RefreshResult{}, apperrors.Unauthorized("Zone cua phien dang nhap khong con kha dung.")
	}
	if target.ZoneID != session.ZoneID || target.WorkspaceID != session.WorkspaceID {
		return RefreshResult{}, apperrors.Unauthorized("Phien dang nhap khong khop zone hien tai.")
	}
	if strings.TrimSpace(input.Domain) != "" {
		requestedTarget, resolveErr := s.resolveZoneAccess(ctx, input.Domain)
		if resolveErr != nil || requestedTarget.ZoneID != target.ZoneID {
			return RefreshResult{}, apperrors.Forbidden("Refresh token khong thuoc domain hien tai.")
		}
		target = requestedTarget
	}

	user, err := s.repo.FindUserByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, authdomain.ErrUserNotFound) {
			return RefreshResult{}, apperrors.Unauthorized("Người dùng không còn tồn tại.")
		}
		return RefreshResult{}, err
	}
	if user.Status != "active" {
		return RefreshResult{}, apperrors.Forbidden("Tài khoản chưa sẵn sàng hoặc đã bị khóa.")
	}
	if err := s.ensureZoneWorkspaceAccess(ctx, user.ID, target); err != nil {
		return RefreshResult{}, err
	}

	accessToken, accessExpiresAt, err := s.tokens.CreateZoneAccessToken(
		user.ID,
		user.Email,
		user.Username,
		target.ZoneID,
		target.WorkspaceID,
		target.Domain,
	)
	if err != nil {
		return RefreshResult{}, apperrors.Internal("Không tạo được access token.")
	}
	newRefreshToken, refreshExpiresAt, err := s.tokens.NewRefreshToken()
	if err != nil {
		return RefreshResult{}, apperrors.Internal("Không tạo được refresh token.")
	}
	session, err = s.repo.RotateSessionRefreshToken(ctx, RotateSessionParams{
		SessionID:        session.ID,
		UserID:           user.ID,
		RefreshTokenHash: s.tokens.HashRefreshToken(newRefreshToken),
		ExpiresAt:        refreshExpiresAt,
	})
	if err != nil {
		if errors.Is(err, authdomain.ErrSessionNotFound) {
			return RefreshResult{}, apperrors.Unauthorized("Phiên đăng nhập không còn hiệu lực.")
		}
		return RefreshResult{}, err
	}

	return RefreshResult{
		Tokens: TokenDTO{
			AccessToken:           accessToken,
			RefreshToken:          newRefreshToken,
			TokenType:             "Bearer",
			AccessTokenExpiresAt:  formatTime(accessExpiresAt),
			RefreshTokenExpiresAt: formatTime(session.ExpiresAt),
		},
		Zone:         toAuthZoneDTO(target),
		SessionID:    session.ID,
		RefreshUntil: formatTime(session.ExpiresAt),
	}, nil
}

func (s *Service) Logout(ctx context.Context, input LogoutInput) error {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken == "" {
		return apperrors.BadRequest("VALIDATION_ERROR", "Refresh token không được để trống.")
	}

	hash := s.tokens.HashRefreshToken(refreshToken)
	session, err := s.repo.FindSessionByRefreshTokenHash(ctx, hash)
	if err != nil && !errors.Is(err, authdomain.ErrSessionNotFound) {
		return err
	}
	if err := s.repo.RevokeSessionByRefreshTokenHash(ctx, hash, s.now().UTC()); err != nil {
		return err
	}
	if session.ID != "" {
		_ = s.repo.RecordAudit(ctx, AuditEvent{
			ActorUserID: session.UserID,
			ZoneID:      session.ZoneID,
			WorkspaceID: session.WorkspaceID,
			Action:      "auth.logout",
			EntityType:  "user_session",
			EntityID:    session.ID,
		})
	}
	return nil
}

func (s *Service) Me(ctx context.Context, userID string, domain string) (UserDTO, error) {
	target, err := s.resolveZoneAccess(ctx, domain)
	if err != nil {
		return UserDTO{}, err
	}
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, authdomain.ErrUserNotFound) {
			return UserDTO{}, apperrors.NotFound("USER_NOT_FOUND", "Không tìm thấy người dùng.")
		}
		return UserDTO{}, err
	}
	if err := s.ensureZoneWorkspaceAccess(ctx, user.ID, target); err != nil {
		return UserDTO{}, err
	}
	return toUserDTO(user), nil
}

func (s *Service) ListSessions(ctx context.Context, userID string, zoneID string) ([]SessionDTO, error) {
	sessions, err := s.repo.ListSessions(ctx, userID, strings.TrimSpace(zoneID))
	if err != nil {
		return nil, err
	}
	dtos := make([]SessionDTO, 0, len(sessions))
	for _, session := range sessions {
		dtos = append(dtos, toSessionDTO(session))
	}
	return dtos, nil
}

func (s *Service) RevokeSession(ctx context.Context, userID string, zoneID string, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return apperrors.BadRequest("VALIDATION_ERROR", "session_id không được để trống.")
	}
	if err := s.repo.RevokeSessionByID(ctx, userID, zoneID, sessionID, s.now().UTC()); err != nil {
		if errors.Is(err, authdomain.ErrSessionNotFound) {
			return apperrors.NotFound("SESSION_NOT_FOUND", "Không tìm thấy phiên đăng nhập.")
		}
		return err
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: userID,
		ZoneID:      strings.TrimSpace(zoneID),
		Action:      "auth.revoke_session",
		EntityType:  "user_session",
		EntityID:    strings.TrimSpace(sessionID),
	})
	return nil
}

func (s *Service) RevokeAllSessions(ctx context.Context, userID string, zoneID string) error {
	if err := s.repo.RevokeAllSessions(ctx, userID, zoneID, s.now().UTC()); err != nil {
		return err
	}
	_ = s.repo.RecordAudit(ctx, AuditEvent{
		ActorUserID: userID,
		ZoneID:      strings.TrimSpace(zoneID),
		Action:      "auth.revoke_all_sessions",
		EntityType:  "user",
		EntityID:    userID,
	})
	return nil
}

func (s *Service) issueTokens(
	ctx context.Context,
	user authdomain.User,
	target ZoneAccess,
	deviceName string,
	ipAddress string,
	userAgent string,
) (AuthResult, error) {
	accessToken, accessExpiresAt, err := s.tokens.CreateZoneAccessToken(
		user.ID,
		user.Email,
		user.Username,
		target.ZoneID,
		target.WorkspaceID,
		target.Domain,
	)
	if err != nil {
		return AuthResult{}, apperrors.Internal("Không tạo được access token.")
	}
	refreshToken, refreshExpiresAt, err := s.tokens.NewRefreshToken()
	if err != nil {
		return AuthResult{}, apperrors.Internal("Không tạo được refresh token.")
	}

	session, err := s.repo.CreateSession(ctx, CreateSessionParams{
		UserID:           user.ID,
		ZoneID:           target.ZoneID,
		WorkspaceID:      target.WorkspaceID,
		Domain:           target.Domain,
		RefreshTokenHash: s.tokens.HashRefreshToken(refreshToken),
		DeviceName:       strings.TrimSpace(deviceName),
		IPAddress:        strings.TrimSpace(ipAddress),
		UserAgent:        strings.TrimSpace(userAgent),
		ExpiresAt:        refreshExpiresAt,
	})
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		User: toUserDTO(user),
		Tokens: TokenDTO{
			AccessToken:           accessToken,
			RefreshToken:          refreshToken,
			TokenType:             "Bearer",
			AccessTokenExpiresAt:  formatTime(accessExpiresAt),
			RefreshTokenExpiresAt: formatTime(session.ExpiresAt),
		},
		Zone:         toAuthZoneDTO(target),
		SessionID:    session.ID,
		RefreshUntil: formatTime(session.ExpiresAt),
	}, nil
}

func normalizeRegister(input RegisterInput) (RegisterInput, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Domain = strings.TrimSpace(input.Domain)
	input.InviteToken = strings.TrimSpace(input.InviteToken)
	input.Password = strings.TrimSpace(input.Password)
	input.DeviceName, input.IPAddress, input.UserAgent = normalizeClientInfo(input.DeviceName, input.IPAddress, input.UserAgent)

	if _, err := mail.ParseAddress(input.Email); err != nil {
		return input, apperrors.BadRequest("VALIDATION_ERROR", "Email không đúng định dạng.")
	}
	if !usernamePattern.MatchString(input.Username) {
		return input, apperrors.BadRequest("VALIDATION_ERROR", "Username phải dài 3-40 ký tự và chỉ gồm chữ thường, số, dấu gạch dưới, gạch ngang hoặc dấu chấm.")
	}
	if input.DisplayName == "" || len([]rune(input.DisplayName)) > 120 {
		return input, apperrors.BadRequest("VALIDATION_ERROR", "Tên hiển thị phải dài từ 1 đến 120 ký tự.")
	}
	domain, err := tenancyapp.NormalizeDomain(input.Domain)
	if err != nil {
		return input, apperrors.BadRequest("INVALID_DOMAIN", "Domain server khong dung dinh dang.")
	}
	input.Domain = domain
	if len([]rune(input.Password)) < 8 {
		return input, apperrors.BadRequest("VALIDATION_ERROR", "Mật khẩu phải có ít nhất 8 ký tự.")
	}
	return input, nil
}

func normalizeLogin(input LoginInput) LoginInput {
	input.Identifier = strings.ToLower(strings.TrimSpace(input.Identifier))
	input.Password = strings.TrimSpace(input.Password)
	input.Domain = strings.TrimSpace(input.Domain)
	input.DeviceName, input.IPAddress, input.UserAgent = normalizeClientInfo(input.DeviceName, input.IPAddress, input.UserAgent)
	return input
}

func googleUsername(email string, subject string) string {
	local := strings.ToLower(strings.Split(email, "@")[0])
	local = regexp.MustCompile(`[^a-z0-9_.-]+`).ReplaceAllString(local, "-")
	local = strings.Trim(local, "-._")
	if local == "" {
		local = "google-user"
	}
	if len(local) > 27 {
		local = local[:27]
	}
	digest := sha256.Sum256([]byte(subject))
	return local + "-" + hex.EncodeToString(digest[:4])
}

func normalizeClientInfo(deviceName string, ipAddress string, userAgent string) (string, string, string) {
	userAgent = strings.TrimSpace(userAgent)
	deviceName = strings.TrimSpace(deviceName)
	ipAddress = strings.TrimSpace(ipAddress)
	if deviceName == "" {
		deviceName = detectDeviceName(userAgent)
	}
	return deviceName, ipAddress, userAgent
}

func detectDeviceName(userAgent string) string {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return "Thiết bị không xác định"
	}

	device := "Thiết bị không xác định"
	switch {
	case strings.Contains(ua, "iphone"):
		device = "iPhone"
	case strings.Contains(ua, "ipad"):
		device = "iPad"
	case strings.Contains(ua, "android"):
		device = "Android"
	case strings.Contains(ua, "windows"):
		device = "Windows"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os"):
		device = "macOS"
	case strings.Contains(ua, "linux"):
		device = "Linux"
	}

	browser := ""
	switch {
	case strings.Contains(ua, "edg/"):
		browser = "Edge"
	case strings.Contains(ua, "opr/") || strings.Contains(ua, "opera"):
		browser = "Opera"
	case strings.Contains(ua, "firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "chrome/") || strings.Contains(ua, "crios/"):
		browser = "Chrome"
	case strings.Contains(ua, "safari/"):
		browser = "Safari"
	}

	if browser == "" {
		return device
	}
	return device + " - " + browser
}

func toUserDTO(user authdomain.User) UserDTO {
	return UserDTO{
		ID:              user.ID,
		Email:           user.Email,
		Username:        user.Username,
		DisplayName:     user.DisplayName,
		AvatarURL:       user.AvatarURL,
		Status:          user.Status,
		Locale:          user.Locale,
		Timezone:        user.Timezone,
		EmailVerifiedAt: formatOptionalTime(user.EmailVerifiedAt),
		LastSeenAt:      formatOptionalTime(user.LastSeenAt),
		RegistrationIP:  user.RegistrationIP,
		RegistrationDev: user.RegistrationDev,
		LastIPAddress:   user.LastIPAddress,
		DeviceName:      user.DeviceName,
		CreatedAt:       formatTime(user.CreatedAt),
		UpdatedAt:       formatTime(user.UpdatedAt),
	}
}

func toSessionDTO(session authdomain.Session) SessionDTO {
	return SessionDTO{
		ID:          session.ID,
		ZoneID:      session.ZoneID,
		WorkspaceID: session.WorkspaceID,
		Domain:      session.Domain,
		DeviceName:  session.DeviceName,
		IPAddress:   session.IPAddress,
		UserAgent:   session.UserAgent,
		ExpiresAt:   formatTime(session.ExpiresAt),
		RevokedAt:   formatOptionalTime(session.RevokedAt),
		CreatedAt:   formatTime(session.CreatedAt),
	}
}

func (s *Service) resolveZoneAccess(ctx context.Context, rawDomain string) (ZoneAccess, error) {
	domain, err := tenancyapp.NormalizeDomain(rawDomain)
	if err != nil {
		return ZoneAccess{}, apperrors.BadRequest("INVALID_DOMAIN", "Domain server khong dung dinh dang.")
	}
	target, err := s.repo.ResolveZoneAccess(ctx, domain)
	if err != nil {
		switch {
		case errors.Is(err, authdomain.ErrZoneNotFound):
			return ZoneAccess{}, apperrors.NotFound("ZONE_NOT_FOUND", "Domain chua san sang cho dang nhap.")
		case errors.Is(err, authdomain.ErrZoneAccessDenied):
			return ZoneAccess{}, apperrors.Forbidden("Tai khoan khong co quyen truy cap zone nay.")
		case errors.Is(err, authdomain.ErrRegistrationClosed):
			return ZoneAccess{}, apperrors.Forbidden("Zone nay khong cho phep dang ky tai khoan moi.")
		default:
			return ZoneAccess{}, err
		}
	}
	return target, nil
}

func (s *Service) ensureZoneWorkspaceAccess(ctx context.Context, userID string, target ZoneAccess) error {
	err := s.repo.EnsureZoneWorkspaceAccess(ctx, userID, target)
	switch {
	case errors.Is(err, authdomain.ErrZoneAccessDenied):
		return apperrors.Forbidden("Tai khoan khong phai thanh vien cua zone nay.")
	case errors.Is(err, authdomain.ErrZoneNotFound):
		return apperrors.NotFound("ZONE_NOT_FOUND", "Zone hoac workspace khong con kha dung.")
	default:
		return err
	}
}

func toAuthZoneDTO(target ZoneAccess) AuthZoneDTO {
	return AuthZoneDTO{
		ID:          target.ZoneID,
		Slug:        target.ZoneSlug,
		Name:        target.ZoneName,
		Kind:        target.ZoneKind,
		Domain:      target.Domain,
		WorkspaceID: target.WorkspaceID,
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTime(*value)
	return &formatted
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}
