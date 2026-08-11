package config

import (
	"crypto/elliptic"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

const testCanonicalInstanceID = "3f1e32b9-0a2f-4ca1-b0dc-04221a551c1c"

func TestLoadIncludesLocalFrontendCORSOrigins(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	assertContains(t, cfg.HTTP.CORSAllowedOrigins, "http://localhost:3000")
	assertContains(t, cfg.HTTP.CORSAllowedOrigins, "http://localhost:3001")
	assertContains(t, cfg.HTTP.CORSAllowedOrigins, "http://127.0.0.1:3000")
	assertContains(t, cfg.HTTP.CORSAllowedOrigins, "http://127.0.0.1:3001")
	assertContains(t, cfg.HTTP.CORSAllowedOrigins, "http://tauri.localhost")
}

func TestLoadMergesConfiguredAndLocalFrontendCORSOrigins(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://chat.vpsttt.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	assertContains(t, cfg.HTTP.CORSAllowedOrigins, "https://chat.vpsttt.com")
	assertContains(t, cfg.HTTP.CORSAllowedOrigins, "http://localhost:3000")
	assertContains(t, cfg.HTTP.CORSAllowedOrigins, "http://localhost:3001")
	assertContains(t, cfg.HTTP.CORSAllowedOrigins, "http://tauri.localhost")
}

func TestValidateRejectsWeakProductionSecrets(t *testing.T) {
	cfg := &Config{
		App: AppConfig{
			Name: "webtui-chat",
			Env:  "production",
		},
		HTTP: HTTPConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Worker: WorkerConfig{
			Concurrency: 1,
		},
		Database: DatabaseConfig{
			URL: "postgres://user:pass@localhost:5432/app?sslmode=disable",
		},
		Security: SecurityConfig{
			JWTAccessSecret:      "change_me_access_secret",
			JWTRefreshSecret:     "change_me_refresh_secret",
			WebhookSigningSecret: "change_me_webhook_secret",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected weak secret error")
	}
}

func TestValidateRejectsUppercasePlaceholderOIDCSecret(t *testing.T) {
	cfg := &Config{
		App:    AppConfig{Name: "webtui-chat", Env: "dev"},
		HTTP:   HTTPConfig{Host: "0.0.0.0", Port: 8080},
		Worker: WorkerConfig{Concurrency: 1},
		Security: SecurityConfig{
			OIDCStateSecret: "CHANGE_ME_RANDOM_OIDC_STATE_SECRET",
		},
	}

	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "OIDC_STATE_SECRET") {
		t.Fatalf("Validate() error = %v, want OIDC_STATE_SECRET placeholder rejection", err)
	}
}

func TestValidateRejectsOIDCClientSecretsWithoutStateSecret(t *testing.T) {
	cfg := &Config{
		App:    AppConfig{Name: "webtui-chat", Env: "dev"},
		HTTP:   HTTPConfig{Host: "0.0.0.0", Port: 8080},
		Worker: WorkerConfig{Concurrency: 1},
		Security: SecurityConfig{
			OIDCClientSecrets: map[string]string{"company-sso": "client-secret"},
		},
	}

	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "OIDC_STATE_SECRET") {
		t.Fatalf("Validate() error = %v, want missing OIDC_STATE_SECRET rejection", err)
	}
}

func TestValidateRejectsInvalidOIDCClientSecretAlias(t *testing.T) {
	cfg := &Config{
		App:    AppConfig{Name: "webtui-chat", Env: "dev"},
		HTTP:   HTTPConfig{Host: "0.0.0.0", Port: 8080},
		Worker: WorkerConfig{Concurrency: 1},
		Security: SecurityConfig{
			OIDCStateSecret:   "valid-oidc-state-secret-at-least-32-bytes",
			OIDCClientSecrets: map[string]string{"../../system": "client-secret"},
		},
	}

	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "OIDC_CLIENT_SECRETS") {
		t.Fatalf("Validate() error = %v, want invalid alias rejection", err)
	}
}

func TestLoadReadsRegistrationDefaultWorkspaceID(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("REGISTRATION_DEFAULT_WORKSPACE_ID", "3f1e32b9-0a2f-4ca1-b0dc-04221a551c1c")
	t.Setenv("CUSTOM_DOMAIN_DNS_TYPE", "a")
	t.Setenv("CUSTOM_DOMAIN_DNS_TARGET", "160.191.55.144")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Registration.DefaultWorkspaceID != "3f1e32b9-0a2f-4ca1-b0dc-04221a551c1c" {
		t.Fatalf("Registration.DefaultWorkspaceID = %q", cfg.Registration.DefaultWorkspaceID)
	}
	if cfg.Registration.CustomDomainDNSType != "A" {
		t.Fatalf("Registration.CustomDomainDNSType = %q", cfg.Registration.CustomDomainDNSType)
	}
	if cfg.Registration.CustomDomainDNSTarget != "160.191.55.144" {
		t.Fatalf("Registration.CustomDomainDNSTarget = %q", cfg.Registration.CustomDomainDNSTarget)
	}
}

func TestLoadReadsLegalDocumentVersions(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("TERMS_VERSION", "terms-2026-08")
	t.Setenv("PRIVACY_POLICY_VERSION", "privacy-2026-08")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Legal.TermsVersion != "terms-2026-08" || cfg.Legal.PrivacyPolicyVersion != "privacy-2026-08" {
		t.Fatalf("Legal = %+v", cfg.Legal)
	}
}

func TestLoadReadsModerationEvidenceRetention(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("MODERATION_EVIDENCE_RETENTION_DAYS", "180")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Moderation.EvidenceRetentionDays != 180 {
		t.Fatalf("Moderation.EvidenceRetentionDays = %d, want 180", cfg.Moderation.EvidenceRetentionDays)
	}
}

func TestValidateRejectsUnsafeModerationEvidenceRetention(t *testing.T) {
	cfg := validConfigForTest()
	cfg.Moderation.EvidenceRetentionDays = 0
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "MODERATION_EVIDENCE_RETENTION_DAYS") {
		t.Fatalf("Validate() error = %v, want moderation retention error", err)
	}

	cfg.Moderation.EvidenceRetentionDays = 3651
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "MODERATION_EVIDENCE_RETENTION_DAYS") {
		t.Fatalf("Validate() error = %v, want moderation retention upper-bound error", err)
	}
}

func TestValidateProductionRequiresExplicitLegalDocumentVersions(t *testing.T) {
	cfg := validConfigForTest()
	cfg.App.Env = "production"
	cfg.Legal = LegalConfig{}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "TERMS_VERSION") || !strings.Contains(err.Error(), "PRIVACY_POLICY_VERSION") {
		t.Fatalf("Validate() error = %v, want both legal version errors", err)
	}

	cfg.Legal = LegalConfig{TermsVersion: "latest", PrivacyPolicyVersion: "1.0"}
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "TERMS_VERSION") {
		t.Fatalf("Validate() error = %v, want placeholder terms version error", err)
	}
}

func TestLoadReadsSelfHostedDeployment(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("DEPLOYMENT_MODE", "self_hosted")
	t.Setenv("INSTANCE_DOMAIN", "Chat.Company.Example")
	t.Setenv("INSTANCE_NAME", "Company Chat")
	t.Setenv("INSTANCE_LOGO_URL", "https://chat.company.example/logo.png")
	t.Setenv("INSTANCE_REGISTRATION_MODE", "invite_only")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Deployment.IsSelfHosted() {
		t.Fatalf("Deployment.Mode = %q, want self_hosted", cfg.Deployment.Mode)
	}
	if cfg.Deployment.InstanceDomain != "chat.company.example" {
		t.Fatalf("Deployment.InstanceDomain = %q", cfg.Deployment.InstanceDomain)
	}
	if cfg.Deployment.InstanceName != "Company Chat" {
		t.Fatalf("Deployment.InstanceName = %q", cfg.Deployment.InstanceName)
	}
	if cfg.Deployment.InstanceLogoURL != "https://chat.company.example/logo.png" {
		t.Fatalf("Deployment.InstanceLogoURL = %q", cfg.Deployment.InstanceLogoURL)
	}
	if cfg.Deployment.InstanceRegistrationMode != "invite_only" {
		t.Fatalf("Deployment.InstanceRegistrationMode = %q", cfg.Deployment.InstanceRegistrationMode)
	}
}

func TestValidateRejectsUnsafeSelfHostedBranding(t *testing.T) {
	cfg := validConfigForTest()
	cfg.Deployment = DeploymentConfig{
		Mode:                     "self_hosted",
		InstanceDomain:           "localhost",
		InstanceName:             "Local Chat",
		InstanceLogoURL:          "http://localhost/logo.png",
		InstanceRegistrationMode: "unknown",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "INSTANCE_LOGO_URL") ||
		!strings.Contains(err.Error(), "INSTANCE_REGISTRATION_MODE") {
		t.Fatalf("Validate() error = %v, muốn lỗi logo và chế độ đăng ký", err)
	}
}

func TestValidateRequiresPublicDomainForProductionSelfHosted(t *testing.T) {
	cfg := validConfigForTest()
	cfg.App.Env = "production"
	cfg.Deployment = DeploymentConfig{
		Mode:           "self_hosted",
		InstanceDomain: "localhost",
		InstanceName:   "Local Chat",
	}

	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "INSTANCE_DOMAIN") {
		t.Fatalf("Validate() error = %v, want invalid production instance domain", err)
	}
}

func TestValidateAllowsLocalSelfHostedDevelopment(t *testing.T) {
	cfg := validConfigForTest()
	cfg.App.Env = "dev"
	cfg.Deployment = DeploymentConfig{
		Mode:           "self_hosted",
		InstanceDomain: "localhost",
		InstanceName:   "Local Chat",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidMobileMinimumVersion(t *testing.T) {
	cfg := validConfigForTest()
	cfg.Client.MobileMinimumVersion = "latest"

	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "MOBILE_MIN_VERSION") {
		t.Fatalf("Validate() error = %v, want semantic mobile minimum version rejection", err)
	}
}

func TestValidateRequiresCompletePushRelayConfiguration(t *testing.T) {
	cfg := validConfigForTest()
	cfg.PushRelay = PushRelayConfig{URL: "https://push.example.com/v1/deliver"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PUSH_RELAY_TOKEN") ||
		!strings.Contains(err.Error(), "PUSH_RELAY_INSTANCE_ID") {
		t.Fatalf("Validate() error = %v, want incomplete push relay rejection", err)
	}
}

func TestValidateRequiresCanonicalDiscoveryUUIDForPushRelay(t *testing.T) {
	cfg := validConfigForTest()
	cfg.PushRelay = PushRelayConfig{
		URL:        "https://push.example.com/v1/deliveries",
		Token:      strings.Repeat("a", 32),
		InstanceID: strings.ToUpper(testCanonicalInstanceID),
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "PUSH_RELAY_INSTANCE_ID") {
		t.Fatalf("Validate() error = %v, want non-canonical UUID rejection", err)
	}

	cfg.PushRelay.InstanceID = testCanonicalInstanceID
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() rejected canonical discovery UUID: %v", err)
	}
}

func TestValidateRejectsInsecureProductionPushRelay(t *testing.T) {
	cfg := validConfigForTest()
	cfg.App.Env = "production"
	cfg.PushRelay = PushRelayConfig{
		URL:        "http://push.example.com/v1/deliver",
		Token:      "CHANGE_ME_ISSUED_BY_PUBLISHER",
		InstanceID: "instance/unsafe",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PUSH_RELAY_URL") ||
		!strings.Contains(err.Error(), "PUSH_RELAY_TOKEN") ||
		!strings.Contains(err.Error(), "PUSH_RELAY_INSTANCE_ID") {
		t.Fatalf("Validate() error = %v, want insecure push relay rejection", err)
	}
}

func TestValidateRejectsPartialDirectPushConfiguration(t *testing.T) {
	cfg := validConfigForTest()
	cfg.Firebase.ProjectID = "mobile-project"
	cfg.APNS.KeyID = "key-id"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "FIREBASE_SERVICE_ACCOUNT") ||
		!strings.Contains(err.Error(), "APNS_TEAM_ID") ||
		!strings.Contains(err.Error(), "APNS_PRIVATE_KEY") {
		t.Fatalf("Validate() error = %v, want incomplete direct push rejection", err)
	}
}

func TestValidateRejectsRelayAndDirectPushTogether(t *testing.T) {
	cfg := validConfigForTest()
	cfg.PushRelay = PushRelayConfig{
		URL:        "https://push.example.com/v1/deliver",
		Token:      "relay-token-for-development",
		InstanceID: testCanonicalInstanceID,
	}
	cfg.Firebase.ServiceAccountFile = "/run/secrets/firebase.json"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "cannot be configured together") {
		t.Fatalf("Validate() error = %v, want ambiguous push mode rejection", err)
	}
}

func TestLoadReadsDesktopVersionPolicy(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("DESKTOP_MIN_VERSION", "1.4.0")
	t.Setenv("DESKTOP_RECOMMENDED_VERSION", "1.5.2")
	t.Setenv("DESKTOP_RELEASE_MANIFEST_DIR", "data/desktop-releases")
	t.Setenv("DESKTOP_UPDATE_URL", "https://chat.vpsttt.com/downloads/desktop/stable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Client.DesktopMinimumVersion != "1.4.0" {
		t.Fatalf("DesktopMinimumVersion = %q", cfg.Client.DesktopMinimumVersion)
	}
	if cfg.Client.DesktopRecommendedVersion != "1.5.2" {
		t.Fatalf("DesktopRecommendedVersion = %q", cfg.Client.DesktopRecommendedVersion)
	}
	if cfg.Client.DesktopReleaseManifestDir != "data/desktop-releases" {
		t.Fatalf("DesktopReleaseManifestDir = %q", cfg.Client.DesktopReleaseManifestDir)
	}
	if cfg.Client.DesktopUpdateURL != "https://chat.vpsttt.com/downloads/desktop/stable" {
		t.Fatalf("DesktopUpdateURL = %q", cfg.Client.DesktopUpdateURL)
	}
}

func TestValidateRejectsInvalidDesktopUpdateURL(t *testing.T) {
	cfg := &Config{
		App:      AppConfig{Name: "webtui-chat", Env: "dev"},
		Client:   ClientConfig{DesktopUpdateURL: "not-a-url"},
		HTTP:     HTTPConfig{Host: "0.0.0.0", Port: 8080},
		Worker:   WorkerConfig{Concurrency: 1},
		Backup:   BackupConfig{PGDumpPath: "pg_dump", Timeout: time.Minute},
		Database: DatabaseConfig{Enabled: true, URL: "postgres://user:pass@localhost:5432/app?sslmode=disable"},
		Order:    OrderConfig{Timeout: time.Second},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected invalid desktop update URL error")
	}
}

func TestValidateRejectsInvalidRegistrationDefaultWorkspaceID(t *testing.T) {
	cfg := &Config{
		App:          AppConfig{Name: "webtui-chat", Env: "dev"},
		HTTP:         HTTPConfig{Host: "0.0.0.0", Port: 8080},
		Worker:       WorkerConfig{Concurrency: 1},
		Backup:       BackupConfig{PGDumpPath: "pg_dump", Timeout: time.Minute},
		Database:     DatabaseConfig{Enabled: true, URL: "postgres://user:pass@localhost:5432/app?sslmode=disable"},
		Order:        OrderConfig{Timeout: time.Second},
		Registration: RegistrationConfig{DefaultWorkspaceID: "not-a-uuid"},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected invalid default workspace UUID error")
	}
}

func TestValidateRejectsInvalidCustomDomainDNSTarget(t *testing.T) {
	cfg := &Config{
		App:      AppConfig{Name: "webtui-chat", Env: "dev"},
		HTTP:     HTTPConfig{Host: "0.0.0.0", Port: 8080},
		Worker:   WorkerConfig{Concurrency: 1},
		Backup:   BackupConfig{PGDumpPath: "pg_dump", Timeout: time.Minute},
		Database: DatabaseConfig{Enabled: true, URL: "postgres://user:pass@localhost:5432/app?sslmode=disable"},
		Order:    OrderConfig{Timeout: time.Second},
		Registration: RegistrationConfig{
			CustomDomainDNSType:   "A",
			CustomDomainDNSTarget: "not-an-ip",
		},
	}

	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "CUSTOM_DOMAIN_DNS_TARGET") {
		t.Fatalf("Validate() error = %v, want invalid custom-domain DNS target rejection", err)
	}
}

func assertContains(t *testing.T, values []string, expected string) {
	t.Helper()

	for _, value := range values {
		if value == expected {
			return
		}
	}

	t.Fatalf("%q không có trong %v", expected, values)
}

func TestValidateAllowsAPIToKeepOnlyVAPIDPublicKey(t *testing.T) {
	cfg := validConfigForTest()
	cfg.App.ServiceName = "api"
	cfg.WebPush = WebPushConfig{
		Enabled: true, VAPIDPublicKey: testVAPIDKey(65),
		TTL: 300, MaxSubscriptionsPerUser: 10,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRequiresVAPIDSigningMaterialForWorker(t *testing.T) {
	cfg := validConfigForTest()
	cfg.App.ServiceName = "worker"
	cfg.WebPush = WebPushConfig{
		Enabled: true, VAPIDPublicKey: testVAPIDKey(65),
		TTL: 300, MaxSubscriptionsPerUser: 10,
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "WEB_PUSH_VAPID_PRIVATE_KEY") {
		t.Fatalf("Validate() error = %v, want missing worker VAPID signing material", err)
	}
}

func TestValidateAcceptsMatchingVAPIDKeyPairForWorker(t *testing.T) {
	cfg := validConfigForTest()
	cfg.App.ServiceName = "worker"
	cfg.WebPush = WebPushConfig{
		Enabled:                 true,
		VAPIDPublicKey:          testVAPIDKey(65),
		VAPIDPrivateKey:         testVAPIDKey(32),
		VAPIDSubject:            "mailto:admin@example.com",
		TTL:                     300,
		MaxSubscriptionsPerUser: 10,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want matching VAPID key pair to pass", err)
	}
}

func TestValidateRejectsOffCurveVAPIDPublicKey(t *testing.T) {
	cfg := validConfigForTest()
	invalidPoint := append([]byte{4}, make([]byte, 64)...)
	cfg.WebPush = WebPushConfig{
		Enabled: true, VAPIDPublicKey: base64.RawURLEncoding.EncodeToString(invalidPoint),
		TTL: 300, MaxSubscriptionsPerUser: 10,
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "WEB_PUSH_VAPID_PUBLIC_KEY") {
		t.Fatalf("Validate() error = %v, want off-curve VAPID rejection", err)
	}
}

func TestValidateRejectsMismatchedVAPIDKeyPair(t *testing.T) {
	cfg := validConfigForTest()
	cfg.App.ServiceName = "worker"
	otherPrivateKey := make([]byte, 32)
	otherPrivateKey[len(otherPrivateKey)-1] = 2
	cfg.WebPush = WebPushConfig{
		Enabled:                 true,
		VAPIDPublicKey:          testVAPIDKey(65),
		VAPIDPrivateKey:         base64.RawURLEncoding.EncodeToString(otherPrivateKey),
		VAPIDSubject:            "mailto:admin@example.com",
		TTL:                     300,
		MaxSubscriptionsPerUser: 10,
	}

	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "matching key pair") {
		t.Fatalf("Validate() error = %v, want mismatched VAPID key pair rejection", err)
	}
}

func TestValidateRejectsPaddedVAPIDKeys(t *testing.T) {
	cfg := validConfigForTest()
	cfg.App.ServiceName = "worker"
	cfg.WebPush = WebPushConfig{
		Enabled:                 true,
		VAPIDPublicKey:          testVAPIDKey(65) + "=",
		VAPIDPrivateKey:         testVAPIDKey(32) + "=",
		VAPIDSubject:            "mailto:admin@example.com",
		TTL:                     300,
		MaxSubscriptionsPerUser: 10,
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "WEB_PUSH_VAPID_PUBLIC_KEY") ||
		!strings.Contains(err.Error(), "WEB_PUSH_VAPID_PRIVATE_KEY") {
		t.Fatalf("Validate() error = %v, want non-canonical padded VAPID keys rejected", err)
	}
}

func testVAPIDKey(size int) string {
	privateKey := make([]byte, 32)
	privateKey[len(privateKey)-1] = 1
	if size == 32 {
		return base64.RawURLEncoding.EncodeToString(privateKey)
	}
	x, y := elliptic.P256().ScalarBaseMult(privateKey)
	return base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), x, y))
}

func validConfigForTest() *Config {
	return &Config{
		App:        AppConfig{Name: "webtui-chat", Env: "dev"},
		HTTP:       HTTPConfig{Host: "0.0.0.0", Port: 8080},
		Worker:     WorkerConfig{Concurrency: 1},
		Backup:     BackupConfig{PGDumpPath: "pg_dump", Timeout: time.Minute},
		Deployment: DeploymentConfig{Mode: "saas"},
		Legal:      LegalConfig{TermsVersion: "2026-08-07", PrivacyPolicyVersion: "2026-08-07"},
		Moderation: ModerationConfig{EvidenceRetentionDays: 365},
		Order:      OrderConfig{Timeout: time.Second},
		Calls:      CallsConfig{RingTimeout: 30 * time.Second},
	}
}
