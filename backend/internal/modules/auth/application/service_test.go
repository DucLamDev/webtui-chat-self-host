package application

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	authdomain "github.com/duclamdev/application-chat/backend/internal/modules/auth/domain"
	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
	apperrors "github.com/duclamdev/application-chat/backend/internal/shared/errors"
)

type workspaceProvisioningRepo struct {
	user                   authdomain.User
	provisionedUserID      string
	provisioningErr        error
	createdSessions        int
	createdUsers           int
	createdUserParams      CreateUserParams
	findUserErr            error
	target                 ZoneAccess
	legalRecord            LegalAcceptanceRecord
	legalParams            LegalAcceptanceParams
	legalRecordCount       int
	legalAccessAllowed     bool
	legalAccessErr         error
	legalAccessUserID      string
	legalAccessWorkspaceID string
	legalAccessZoneID      string
	legalReadWorkspaceID   string
}

func (r *workspaceProvisioningRepo) CreateUser(_ context.Context, params CreateUserParams) (authdomain.User, error) {
	r.createdUsers++
	r.createdUserParams = params
	return r.user, nil
}

func (r *workspaceProvisioningRepo) ResolveZoneAccess(context.Context, string) (ZoneAccess, error) {
	if r.target.ZoneID == "" {
		r.target = testZoneAccess()
	}
	return r.target, nil
}

func testZoneAccess() ZoneAccess {
	return ZoneAccess{
		ZoneID:           "11111111-1111-4111-8111-111111111111",
		ZoneSlug:         "vpsttt",
		ZoneName:         "VPSTTT",
		ZoneKind:         "vpsttt_internal",
		ZoneStatus:       "active",
		RegistrationMode: "open",
		WorkspaceID:      "22222222-2222-4222-8222-222222222222",
		WorkspaceSlug:    "vpsttt",
		Domain:           "chat.vpsttt.com",
	}
}

func (r *workspaceProvisioningRepo) EnsureZoneWorkspaceAccess(_ context.Context, userID string, _ ZoneAccess) error {
	r.provisionedUserID = userID
	return r.provisioningErr
}

func (r *workspaceProvisioningRepo) FindUserByID(context.Context, string) (authdomain.User, error) {
	return r.user, nil
}

func (r *workspaceProvisioningRepo) FindUserByIdentifier(context.Context, string) (authdomain.User, error) {
	if r.findUserErr != nil {
		return authdomain.User{}, r.findUserErr
	}
	return r.user, nil
}

func (r *workspaceProvisioningRepo) UpdateLastLoginInfo(context.Context, UpdateLastLoginInfoParams) error {
	return nil
}

func (r *workspaceProvisioningRepo) CreateSession(_ context.Context, params CreateSessionParams) (authdomain.Session, error) {
	r.createdSessions++
	return authdomain.Session{
		ID:          "session-1",
		UserID:      params.UserID,
		ZoneID:      params.ZoneID,
		WorkspaceID: params.WorkspaceID,
		Domain:      params.Domain,
		ExpiresAt:   params.ExpiresAt,
	}, nil
}

func (r *workspaceProvisioningRepo) FindSessionByRefreshTokenHash(context.Context, string) (authdomain.Session, error) {
	return authdomain.Session{}, authdomain.ErrSessionNotFound
}

func (r *workspaceProvisioningRepo) RotateSessionRefreshToken(context.Context, RotateSessionParams) (authdomain.Session, error) {
	return authdomain.Session{}, nil
}

func (r *workspaceProvisioningRepo) RevokeSessionByRefreshTokenHash(context.Context, string, time.Time) error {
	return nil
}

func (r *workspaceProvisioningRepo) RevokeSessionByID(context.Context, string, string, string, time.Time) error {
	return nil
}

func (r *workspaceProvisioningRepo) RevokeAllSessions(context.Context, string, string, time.Time) error {
	return nil
}

func (r *workspaceProvisioningRepo) ListSessions(context.Context, string, string) ([]authdomain.Session, error) {
	return nil, nil
}

func (r *workspaceProvisioningRepo) RecordAudit(context.Context, AuditEvent) error {
	return nil
}

func (r *workspaceProvisioningRepo) ReadCurrentLegalAcceptances(
	_ context.Context,
	_ string,
	workspaceID string,
	_ string,
	_ string,
) (LegalAcceptanceRecord, error) {
	r.legalReadWorkspaceID = workspaceID
	return r.legalRecord, nil
}

func (r *workspaceProvisioningRepo) CanAccessLegalAcceptance(_ context.Context, userID string, workspaceID string, zoneID string) (bool, error) {
	r.legalAccessUserID = userID
	r.legalAccessWorkspaceID = workspaceID
	r.legalAccessZoneID = zoneID
	return r.legalAccessAllowed, r.legalAccessErr
}

func (r *workspaceProvisioningRepo) RecordLegalAcceptances(_ context.Context, params LegalAcceptanceParams) error {
	r.legalParams = params
	r.legalRecordCount++
	now := time.Now().UTC()
	if params.TermsAccepted {
		r.legalRecord.TermsAcceptedAt = &now
	}
	if params.PrivacyAccepted {
		r.legalRecord.PrivacyAcceptedAt = &now
	}
	return nil
}

func TestExistingUserLegalAcceptanceStatusAndRecording(t *testing.T) {
	repo := &workspaceProvisioningRepo{legalAccessAllowed: true}
	service := NewService(repo, nil)
	service.SetLegalDocumentVersions("terms-v2", "privacy-v3")

	status, err := service.GetCurrentLegalAcceptance(context.Background(), "user-1", "workspace-2", "zone-1")
	if err != nil {
		t.Fatalf("GetCurrentLegalAcceptance() error = %v", err)
	}
	if status.Complete || status.Terms.Accepted || status.Privacy.Accepted {
		t.Fatalf("initial status = %#v, want incomplete", status)
	}
	if repo.legalRecordCount != 0 {
		t.Fatal("status lookup must never backfill consent")
	}

	status, err = service.AcceptCurrentLegalDocuments(context.Background(), AcceptLegalDocumentsInput{
		UserID: "user-1", WorkspaceID: "workspace-2", ZoneID: "zone-1",
		TermsAccepted: true, TermsVersion: "terms-v2",
		PrivacyAccepted: true, PrivacyVersion: "privacy-v3",
		IPAddress: "127.0.0.1", UserAgent: "mobile-test",
	})
	if err != nil {
		t.Fatalf("AcceptCurrentLegalDocuments() error = %v", err)
	}
	if !status.Complete || !status.Terms.Accepted || !status.Privacy.Accepted {
		t.Fatalf("accepted status = %#v, want complete", status)
	}
	if repo.legalRecordCount != 1 || repo.legalParams.Source != "authenticated_acceptance" {
		t.Fatalf("recorded legal params = %#v, count=%d", repo.legalParams, repo.legalRecordCount)
	}
	if status.WorkspaceID != "workspace-2" || repo.legalReadWorkspaceID != "workspace-2" ||
		repo.legalParams.WorkspaceID != "workspace-2" || repo.legalParams.ZoneID != "zone-1" ||
		repo.legalAccessUserID != "user-1" || repo.legalAccessWorkspaceID != "workspace-2" ||
		repo.legalAccessZoneID != "zone-1" {
		t.Fatalf("cross-workspace acceptance scope was not preserved: status=%#v params=%#v", status, repo.legalParams)
	}
}

func TestExistingUserLegalAcceptanceRejectsMissingAndStaleConsent(t *testing.T) {
	repo := &workspaceProvisioningRepo{legalAccessAllowed: true}
	service := NewService(repo, nil)
	service.SetLegalDocumentVersions("terms-v2", "privacy-v3")

	tests := []struct {
		name  string
		input AcceptLegalDocumentsInput
		code  string
	}{
		{
			name:  "missing acceptance",
			input: AcceptLegalDocumentsInput{UserID: "user-1", WorkspaceID: "workspace-1", ZoneID: "zone-1"},
			code:  "LEGAL_ACCEPTANCE_REQUIRED",
		},
		{
			name:  "stale terms",
			input: AcceptLegalDocumentsInput{UserID: "user-1", WorkspaceID: "workspace-1", ZoneID: "zone-1", TermsAccepted: true, TermsVersion: "terms-v1", PrivacyAccepted: true, PrivacyVersion: "privacy-v3"},
			code:  "TERMS_VERSION_INVALID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.AcceptCurrentLegalDocuments(context.Background(), test.input)
			var appErr *apperrors.AppError
			if !errors.As(err, &appErr) || appErr.Code != test.code {
				t.Fatalf("error = %#v, want %s", err, test.code)
			}
		})
	}
	if repo.legalRecordCount != 0 {
		t.Fatalf("invalid consent wrote %d records", repo.legalRecordCount)
	}
}

func TestExistingUserLegalAcceptanceRejectsUnauthorizedWorkspace(t *testing.T) {
	repo := &workspaceProvisioningRepo{legalAccessAllowed: false}
	service := NewService(repo, nil)
	service.SetLegalDocumentVersions("terms-v2", "privacy-v3")

	_, err := service.GetCurrentLegalAcceptance(context.Background(), "user-1", "workspace-other", "zone-1")
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Status != http.StatusForbidden || appErr.Code != "LEGAL_ACCEPTANCE_WORKSPACE_FORBIDDEN" {
		t.Fatalf("GetCurrentLegalAcceptance() error = %#v, want workspace forbidden", err)
	}
	_, err = service.AcceptCurrentLegalDocuments(context.Background(), AcceptLegalDocumentsInput{
		UserID: "user-1", WorkspaceID: "workspace-other", ZoneID: "zone-1",
		TermsAccepted: true, TermsVersion: "terms-v2",
		PrivacyAccepted: true, PrivacyVersion: "privacy-v3",
	})
	if !errors.As(err, &appErr) || appErr.Code != "LEGAL_ACCEPTANCE_WORKSPACE_FORBIDDEN" {
		t.Fatalf("AcceptCurrentLegalDocuments() error = %#v, want workspace forbidden", err)
	}
	if repo.legalRecordCount != 0 {
		t.Fatalf("unauthorized acceptance wrote %d records", repo.legalRecordCount)
	}
}

func TestDetectDeviceName(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		want      string
	}{
		{
			name:      "windows chrome",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36",
			want:      "Windows - Chrome",
		},
		{
			name:      "iphone safari",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1",
			want:      "iPhone - Safari",
		},
		{
			name:      "unknown",
			userAgent: "",
			want:      "Thiết bị không xác định",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectDeviceName(tt.userAgent); got != tt.want {
				t.Fatalf("detectDeviceName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeClientInfoKeepsExplicitDeviceName(t *testing.T) {
	deviceName, ipAddress, userAgent := normalizeClientInfo("Laptop kế toán", "127.0.0.1", "Mozilla/5.0")

	if deviceName != "Laptop kế toán" {
		t.Fatalf("deviceName = %q", deviceName)
	}
	if ipAddress != "127.0.0.1" {
		t.Fatalf("ipAddress = %q", ipAddress)
	}
	if userAgent != "Mozilla/5.0" {
		t.Fatalf("userAgent = %q", userAgent)
	}
}

func TestNormalizeRegisterRequiresValidServerDomain(t *testing.T) {
	normalized, err := normalizeRegister(RegisterInput{
		Email:       "member@example.com",
		Username:    "member",
		DisplayName: "Member",
		Domain:      " https://Chat.Company.com/path ",
		Password:    "password-123",
	})
	if err != nil {
		t.Fatalf("normalizeRegister() error = %v", err)
	}
	if normalized.Domain != "chat.company.com" {
		t.Fatalf("domain = %q, want chat.company.com", normalized.Domain)
	}

	_, err = normalizeRegister(RegisterInput{
		Email:       "member@example.com",
		Username:    "member",
		DisplayName: "Member",
		Domain:      "not a domain",
		Password:    "password-123",
	})
	if err == nil {
		t.Fatal("normalizeRegister() expected invalid domain error")
	}
}

func TestNormalizeRegisterAcceptsUsernameWithoutSpecialCharacters(t *testing.T) {
	normalized, err := normalizeRegister(RegisterInput{
		Email:       "member@example.com",
		Username:    "  DucLam24  ",
		DisplayName: "Đức Lâm",
		Domain:      "chat.company.com",
		Password:    "password",
	})
	if err != nil {
		t.Fatalf("normalizeRegister() rejected a plain username: %v", err)
	}
	if normalized.Username != "duclam24" {
		t.Fatalf("username = %q, want duclam24", normalized.Username)
	}
}

func TestNormalizeRegisterAllowsOptionalUsernameSeparators(t *testing.T) {
	for _, username := range []string{"duc.lam", "duc_lam", "duc-lam"} {
		t.Run(username, func(t *testing.T) {
			_, err := normalizeRegister(RegisterInput{
				Email:       "member@example.com",
				Username:    username,
				DisplayName: "Đức Lâm",
				Domain:      "chat.company.com",
				Password:    "password",
			})
			if err != nil {
				t.Fatalf("normalizeRegister() rejected %q: %v", username, err)
			}
		})
	}
}

func TestValidateLegalAcceptancesRequiresCurrentVersionsForNewAccounts(t *testing.T) {
	service := NewService(nil, nil)
	service.SetLegalDocumentVersions("terms-v2", "privacy-v3")

	for _, test := range []struct {
		name            string
		termsAccepted   bool
		termsVersion    string
		privacyAccepted bool
		privacyVersion  string
		wantCode        string
		wantStatus      int
	}{
		{name: "missing both", wantCode: "LEGAL_ACCEPTANCE_REQUIRED", wantStatus: http.StatusConflict},
		{name: "terms false", privacyAccepted: true, privacyVersion: "privacy-v3", wantCode: "LEGAL_ACCEPTANCE_REQUIRED", wantStatus: http.StatusConflict},
		{name: "privacy false", termsAccepted: true, termsVersion: "terms-v2", wantCode: "LEGAL_ACCEPTANCE_REQUIRED", wantStatus: http.StatusConflict},
		{name: "stale terms", termsAccepted: true, termsVersion: "terms-v1", privacyAccepted: true, privacyVersion: "privacy-v3", wantCode: "TERMS_VERSION_INVALID", wantStatus: http.StatusBadRequest},
		{name: "stale privacy", termsAccepted: true, termsVersion: "terms-v2", privacyAccepted: true, privacyVersion: "privacy-v2", wantCode: "PRIVACY_VERSION_INVALID", wantStatus: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := service.validateLegalAcceptances(
				test.termsAccepted,
				test.termsVersion,
				test.privacyAccepted,
				test.privacyVersion,
				true,
			)
			var appErr *apperrors.AppError
			if !errors.As(err, &appErr) || appErr.Code != test.wantCode || appErr.Status != test.wantStatus {
				t.Fatalf("error = %#v, want %s/%d", err, test.wantCode, test.wantStatus)
			}
		})
	}

	if err := service.validateLegalAcceptances(true, "terms-v2", true, "privacy-v3", true); err != nil {
		t.Fatalf("current legal versions rejected: %v", err)
	}
	if err := service.validateLegalAcceptances(false, "", false, "", false); err != nil {
		t.Fatalf("existing login without acceptance rejected: %v", err)
	}
}

func TestGoogleLoginRequiresConsentOnlyWhenCreatingAccount(t *testing.T) {
	user := authdomain.User{
		ID:          "6c8dd8dd-e7c3-4fa9-9f95-1cdf312587ba",
		Email:       "member@example.com",
		Username:    "member",
		DisplayName: "Member",
		Status:      "active",
	}
	newUserRepo := &workspaceProvisioningRepo{user: user, findUserErr: authdomain.ErrUserNotFound}
	newUserService := NewService(newUserRepo, sharedauth.NewManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour))
	newUserService.SetLegalDocumentVersions("terms-v2", "privacy-v3")
	input := GoogleLoginInput{
		Subject:       "google-subject-123",
		Email:         user.Email,
		EmailVerified: true,
		DisplayName:   user.DisplayName,
		Domain:        "chat.vpsttt.com",
	}

	_, err := newUserService.LoginWithGoogle(context.Background(), input)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "LEGAL_ACCEPTANCE_REQUIRED" || appErr.Status != http.StatusConflict {
		t.Fatalf("new Google account error = %#v, want LEGAL_ACCEPTANCE_REQUIRED/409", err)
	}
	if newUserRepo.createdUsers != 0 || newUserRepo.createdSessions != 0 {
		t.Fatalf("missing consent created account/session: users=%d sessions=%d", newUserRepo.createdUsers, newUserRepo.createdSessions)
	}

	input.TermsAccepted = true
	input.TermsVersion = "terms-v2"
	input.PrivacyAccepted = true
	input.PrivacyVersion = "privacy-v3"
	if _, err := newUserService.LoginWithGoogle(context.Background(), input); err != nil {
		t.Fatalf("Google signup retry with consent error = %v", err)
	}
	if newUserRepo.createdUsers != 1 || newUserRepo.createdSessions != 1 {
		t.Fatalf("consented signup users=%d sessions=%d, want 1/1", newUserRepo.createdUsers, newUserRepo.createdSessions)
	}
	if !newUserRepo.createdUserParams.TermsAccepted || !newUserRepo.createdUserParams.PrivacyAccepted {
		t.Fatalf("legal acceptance not passed transactionally: %+v", newUserRepo.createdUserParams)
	}

	existingRepo := &workspaceProvisioningRepo{user: user}
	existingService := NewService(existingRepo, sharedauth.NewManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour))
	existingService.SetLegalDocumentVersions("terms-v2", "privacy-v3")
	if _, err := existingService.LoginWithGoogle(context.Background(), GoogleLoginInput{
		Subject:       input.Subject,
		Email:         input.Email,
		EmailVerified: true,
		Domain:        input.Domain,
	}); err != nil {
		t.Fatalf("existing Google login without consent error = %v", err)
	}
	if existingRepo.createdUsers != 0 || existingRepo.createdSessions != 1 {
		t.Fatalf("existing login users=%d sessions=%d, want 0/1", existingRepo.createdUsers, existingRepo.createdSessions)
	}
}

func TestRegisterRejectsMissingOrStaleConsentBeforePersistence(t *testing.T) {
	user := authdomain.User{
		ID:          "6c8dd8dd-e7c3-4fa9-9f95-1cdf312587ba",
		Email:       "new@example.com",
		Username:    "new-user",
		DisplayName: "New User",
		Status:      "active",
	}
	repo := &workspaceProvisioningRepo{user: user}
	service := NewService(repo, sharedauth.NewManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour))
	service.SetLegalDocumentVersions("terms-v2", "privacy-v3")
	input := RegisterInput{
		Email:       user.Email,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Password:    "password-123",
		Domain:      "chat.vpsttt.com",
	}

	_, err := service.Register(context.Background(), input)
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "LEGAL_ACCEPTANCE_REQUIRED" {
		t.Fatalf("missing consent error = %#v", err)
	}
	input.TermsAccepted = true
	input.TermsVersion = "terms-v1"
	input.PrivacyAccepted = true
	input.PrivacyVersion = "privacy-v3"
	_, err = service.Register(context.Background(), input)
	if !errors.As(err, &appErr) || appErr.Code != "TERMS_VERSION_INVALID" {
		t.Fatalf("stale consent error = %#v", err)
	}
	if repo.createdUsers != 0 || repo.createdSessions != 0 {
		t.Fatalf("invalid consent reached persistence: users=%d sessions=%d", repo.createdUsers, repo.createdSessions)
	}
}

func TestGoogleUsernameIsStableAndValid(t *testing.T) {
	username := googleUsername("Ho.Duc.Lam@example.com", "google-subject-123")
	if !usernamePattern.MatchString(username) {
		t.Fatalf("googleUsername() = %q không hợp lệ", username)
	}
	if username != googleUsername("Ho.Duc.Lam@example.com", "google-subject-123") {
		t.Fatal("googleUsername() phải ổn định với cùng Google subject")
	}
}

func TestLoginRepairsDefaultWorkspaceMembershipBeforeCreatingSession(t *testing.T) {
	passwordHash, err := sharedauth.HashPassword("password-123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	repo := &workspaceProvisioningRepo{user: authdomain.User{
		ID:           "6c8dd8dd-e7c3-4fa9-9f95-1cdf312587ba",
		Email:        "member@example.com",
		Username:     "member",
		DisplayName:  "Member",
		PasswordHash: passwordHash,
		Status:       "active",
	}}
	service := NewService(repo, sharedauth.NewManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour))

	result, err := service.Login(context.Background(), LoginInput{
		Identifier: "member@example.com",
		Password:   "password-123",
		Domain:     "chat.vpsttt.com",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if repo.provisionedUserID != repo.user.ID {
		t.Fatalf("provisioned user = %q, want %q", repo.provisionedUserID, repo.user.ID)
	}
	if repo.createdSessions != 1 || result.SessionID != "session-1" {
		t.Fatalf("session was not created after provisioning: count=%d id=%q", repo.createdSessions, result.SessionID)
	}
	claims, err := service.tokens.VerifyAccessToken(result.Tokens.AccessToken)
	if err != nil {
		t.Fatalf("VerifyAccessToken() error = %v", err)
	}
	target := testZoneAccess()
	if claims.ZoneID != target.ZoneID || claims.WorkspaceID != target.WorkspaceID || claims.Domain != target.Domain {
		t.Fatalf("token zone claims = %+v", claims)
	}
	if result.Zone.ID != target.ZoneID || result.Zone.Domain != target.Domain {
		t.Fatalf("auth result zone = %+v", result.Zone)
	}
}

func TestSuspendedZoneAllowsExistingLoginOnlyForRecovery(t *testing.T) {
	passwordHash, err := sharedauth.HashPassword("password-123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	target := testZoneAccess()
	target.ZoneStatus = "suspended"
	repo := &workspaceProvisioningRepo{
		target: target,
		user: authdomain.User{
			ID:           "6c8dd8dd-e7c3-4fa9-9f95-1cdf312587ba",
			Email:        "member@example.com",
			Username:     "member",
			DisplayName:  "Member",
			PasswordHash: passwordHash,
			Status:       "active",
		},
	}
	service := NewService(repo, sharedauth.NewManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour))

	if _, err := service.Login(context.Background(), LoginInput{
		Identifier: "member@example.com",
		Password:   "password-123",
		Domain:     "chat.vpsttt.com",
	}); err != nil {
		t.Fatalf("Login() suspended recovery error = %v", err)
	}
	if repo.createdSessions != 1 {
		t.Fatalf("recovery login sessions = %d, want 1", repo.createdSessions)
	}

	if _, err := service.Register(context.Background(), RegisterInput{
		Email:           "new@example.com",
		Username:        "new-user",
		DisplayName:     "New User",
		Password:        "password-123",
		Domain:          "chat.vpsttt.com",
		TermsAccepted:   true,
		TermsVersion:    defaultLegalDocumentVersion,
		PrivacyAccepted: true,
		PrivacyVersion:  defaultLegalDocumentVersion,
	}); err == nil {
		t.Fatal("Register() expected suspended zone error")
	}
}

func TestLoginDoesNotProvisionWorkspaceForInvalidPassword(t *testing.T) {
	passwordHash, err := sharedauth.HashPassword("password-123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	repo := &workspaceProvisioningRepo{user: authdomain.User{
		ID:           "6c8dd8dd-e7c3-4fa9-9f95-1cdf312587ba",
		Email:        "member@example.com",
		Username:     "member",
		DisplayName:  "Member",
		PasswordHash: passwordHash,
		Status:       "active",
	}}
	service := NewService(repo, sharedauth.NewManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour))

	if _, err := service.Login(context.Background(), LoginInput{
		Identifier: "member@example.com",
		Password:   "wrong-password",
		Domain:     "chat.vpsttt.com",
	}); err == nil {
		t.Fatal("Login() expected invalid credentials error")
	}
	if repo.provisionedUserID != "" || repo.createdSessions != 0 {
		t.Fatalf("invalid login changed access: provisioned=%q sessions=%d", repo.provisionedUserID, repo.createdSessions)
	}
}

func TestLoginDoesNotCreateSessionWhenWorkspaceProvisioningFails(t *testing.T) {
	passwordHash, err := sharedauth.HashPassword("password-123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	provisioningErr := errors.New("default workspace unavailable")
	repo := &workspaceProvisioningRepo{
		user: authdomain.User{
			ID:           "6c8dd8dd-e7c3-4fa9-9f95-1cdf312587ba",
			Email:        "member@example.com",
			Username:     "member",
			DisplayName:  "Member",
			PasswordHash: passwordHash,
			Status:       "active",
		},
		provisioningErr: provisioningErr,
	}
	service := NewService(repo, sharedauth.NewManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour))

	_, err = service.Login(context.Background(), LoginInput{
		Identifier: "member@example.com",
		Password:   "password-123",
		Domain:     "chat.vpsttt.com",
	})
	if !errors.Is(err, provisioningErr) {
		t.Fatalf("Login() error = %v, want provisioning error", err)
	}
	if repo.provisionedUserID != repo.user.ID || repo.createdSessions != 0 {
		t.Fatalf("failed provisioning created access: provisioned=%q sessions=%d", repo.provisionedUserID, repo.createdSessions)
	}
}

func TestMeRepairsDefaultWorkspaceMembershipForExistingSession(t *testing.T) {
	repo := &workspaceProvisioningRepo{user: authdomain.User{
		ID:          "6c8dd8dd-e7c3-4fa9-9f95-1cdf312587ba",
		Email:       "member@example.com",
		Username:    "member",
		DisplayName: "Member",
		Status:      "active",
	}}
	service := NewService(repo, sharedauth.NewManager("access-secret", "refresh-secret", time.Hour, 24*time.Hour))

	result, err := service.Me(context.Background(), repo.user.ID, "chat.vpsttt.com")
	if err != nil {
		t.Fatalf("Me() error = %v", err)
	}
	if repo.provisionedUserID != repo.user.ID {
		t.Fatalf("provisioned user = %q, want %q", repo.provisionedUserID, repo.user.ID)
	}
	if result.ID != repo.user.ID {
		t.Fatalf("Me() user ID = %q, want %q", result.ID, repo.user.ID)
	}
}
