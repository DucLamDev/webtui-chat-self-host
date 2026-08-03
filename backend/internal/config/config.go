package config

import (
	"bytes"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var defaultCORSAllowedOrigins = []string{
	"http://localhost:3000",
	"http://localhost:3001",
	"http://localhost:5173",
	"http://127.0.0.1:3000",
	"http://127.0.0.1:3001",
	"http://127.0.0.1:5173",
	"http://tauri.localhost",
	"https://tauri.localhost",
	"tauri://localhost",
	"https://chat.vpsttt.com",
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var oidcSecretAliasPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
var instanceDomainPattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
var pushRelayInstancePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type Config struct {
	App             AppConfig
	Client          ClientConfig
	HTTP            HTTPConfig
	Metrics         MetricsConfig
	Telemetry       TelemetryConfig
	Worker          WorkerConfig
	ModuleRunner    ModuleRunnerConfig
	Backup          BackupConfig
	Database        DatabaseConfig
	Redis           RedisConfig
	RabbitMQ        RabbitMQConfig
	Storage         StorageConfig
	Security        SecurityConfig
	Calls           CallsConfig
	Firebase        FirebaseConfig
	APNS            APNSConfig
	PushRelay       PushRelayConfig
	PushRelayServer PushRelayServerConfig
	WebPush         WebPushConfig
	Deployment      DeploymentConfig
	Registration    RegistrationConfig
	Order           OrderConfig
}

type AppConfig struct {
	Name        string
	Env         string
	URL         string
	Version     string
	LogLevel    string
	LogFormat   string
	StartedAt   time.Time
	ServiceName string
}

type ClientConfig struct {
	DesktopMinimumVersion     string
	DesktopRecommendedVersion string
	DesktopReleaseManifestDir string
	DesktopUpdateURL          string
	MobileMinimumVersion      string
	MobileRecommendedVersion  string
	MobileReleaseManifestDir  string
	MobileDownloadURL         string
	MobileStoreURL            string
	DownloadManifestDir       string
}

type HTTPConfig struct {
	Host                 string
	Port                 int
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	IdleTimeout          time.Duration
	ShutdownTimeout      time.Duration
	TrustedProxies       []string
	CORSAllowedOrigins   []string
	SecureHeadersEnabled bool
	RateLimitEnabled     bool
	RateLimitPerMinute   int
	RateLimitBurst       int
}

type MetricsConfig struct {
	Enabled bool
	Path    string
}

type TelemetryConfig struct {
	Enabled      bool
	OTLPEndpoint string
	SampleRatio  float64
}

type WorkerConfig struct {
	Concurrency int
}

type ModuleRunnerConfig struct {
	ScriptAllowlist map[string]string
}

type BackupConfig struct {
	PGDumpPath string
	Timeout    time.Duration
}

type DatabaseConfig struct {
	Enabled        bool
	URL            string
	MigrationsPath string
}

type RedisConfig struct {
	Enabled bool
	URL     string
}

type RabbitMQConfig struct {
	Enabled bool
	URL     string
}

type StorageConfig struct {
	Provider        string
	LocalPath       string
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

type SecurityConfig struct {
	JWTAccessSecret       string
	JWTRefreshSecret      string
	WebhookSigningSecret  string
	StorageCredentialsKey string
	BotAISecretKey        string
	GoogleClientID        string
	CaddyAskSecret        string
	OIDCStateSecret       string
	OIDCClientSecrets     map[string]string
}

type CallsConfig struct {
	RingTimeout       time.Duration
	ICEServers        []map[string]any
	JitsiBaseURL      string
	TURNURLs          []string
	TURNSharedSecret  string
	TURNCredentialTTL time.Duration
}

type FirebaseConfig struct {
	ProjectID                string
	ServiceAccountFile       string
	ServiceAccountJSONBase64 string
}

type APNSConfig struct {
	KeyID            string
	TeamID           string
	BundleID         string
	PrivateKeyFile   string
	PrivateKeyBase64 string
	Sandbox          bool
}

// PushRelayConfig lets a self-hosted instance deliver notifications through
// the publisher-operated FCM/APNs relay. This keeps vendor signing keys out of
// customer installations while the instance remains authoritative for access.
type PushRelayConfig struct {
	URL        string
	Token      string
	InstanceID string
}

// PushRelayServerConfig configures the optional, publisher-operated relay
// service. It is disabled by default and never starts as part of the API or
// worker processes.
type PushRelayServerConfig struct {
	Enabled            bool
	Host               string
	Port               int
	Publishers         map[string]string
	MaxBodyBytes       int64
	RateLimitPerMinute int
	RateLimitBurst     int
	WorkerConcurrency  int
	PollInterval       time.Duration
}

// WebPushConfig keeps VAPID ownership with each self-hosted instance. The
// private key is consumed only by the worker; the API exposes the public key.
type WebPushConfig struct {
	Enabled                 bool
	VAPIDPublicKey          string
	VAPIDPrivateKey         string
	VAPIDSubject            string
	TTL                     int
	MaxSubscriptionsPerUser int
}

type DeploymentConfig struct {
	Mode                     string
	InstanceDomain           string
	InstanceName             string
	InstanceLogoURL          string
	InstanceRegistrationMode string
}

func (c DeploymentConfig) IsSelfHosted() bool {
	return strings.EqualFold(strings.TrimSpace(c.Mode), "self_hosted")
}

type RegistrationConfig struct {
	DefaultWorkspaceID    string
	CustomDomainDNSType   string
	CustomDomainDNSTarget string
}

type OrderConfig struct {
	BaseURL        string
	InternalAPIKey string
	QuickOrderKey  string
	Timeout        time.Duration
}

func Load() (*Config, error) {
	appEnv := getEnv("APP_ENV", "dev")
	serviceName := getEnv("SERVICE_NAME", "api")
	deploymentMode := strings.ToLower(getEnv("DEPLOYMENT_MODE", "self_hosted"))
	instanceDomainFallback := ""
	orderAPIBaseURLFallback := ""
	databaseURLFallback := "postgres://postgres:123456@localhost:5432/vpstttdb_chat?sslmode=disable"
	if isOperationalService(serviceName) {
		// Operational binaries must never silently connect to the development
		// database when their deliberately small environment omits DATABASE_URL.
		databaseURLFallback = ""
	}
	if deploymentMode == "self_hosted" && appEnv != "production" {
		instanceDomainFallback = "localhost"
	}
	if deploymentMode == "saas" {
		orderAPIBaseURLFallback = "https://order.vpsttt.com/api"
	}
	corsAllowedOrigins := getEnvCSV("CORS_ALLOWED_ORIGINS", []string{})
	if appEnv != "production" {
		corsAllowedOrigins = getEnvCSVWithDefaults("CORS_ALLOWED_ORIGINS", defaultCORSAllowedOrigins)
	}

	cfg := &Config{
		App: AppConfig{
			Name:        getEnv("APP_NAME", "webtui-chat"),
			Env:         appEnv,
			URL:         getEnv("APP_URL", "http://localhost:8080"),
			Version:     getEnv("APP_VERSION", "dev"),
			LogLevel:    getEnv("LOG_LEVEL", "info"),
			LogFormat:   getEnv("LOG_FORMAT", "text"),
			StartedAt:   time.Now().UTC(),
			ServiceName: serviceName,
		},
		Client: ClientConfig{
			DesktopMinimumVersion:     getEnv("DESKTOP_MIN_VERSION", "0.1.0"),
			DesktopRecommendedVersion: getEnv("DESKTOP_RECOMMENDED_VERSION", getEnv("APP_VERSION", "dev")),
			DesktopReleaseManifestDir: getEnv("DESKTOP_RELEASE_MANIFEST_DIR", ""),
			DesktopUpdateURL:          getEnv("DESKTOP_UPDATE_URL", "https://download.vpsttt.com/desktop"),
			MobileMinimumVersion:      getEnv("MOBILE_MIN_VERSION", "0.1.0"),
			MobileRecommendedVersion:  getEnv("MOBILE_RECOMMENDED_VERSION", getEnv("APP_VERSION", "dev")),
			MobileReleaseManifestDir:  getEnv("MOBILE_RELEASE_MANIFEST_DIR", ""),
			MobileDownloadURL:         getEnv("MOBILE_DOWNLOAD_URL", "https://download.vpsttt.com/mobile"),
			MobileStoreURL:            getEnv("MOBILE_STORE_URL", ""),
			DownloadManifestDir:       getEnv("DOWNLOAD_MANIFEST_DIR", ""),
		},
		HTTP: HTTPConfig{
			Host:                 getEnv("API_HTTP_HOST", "0.0.0.0"),
			Port:                 getEnvInt("API_HTTP_PORT", 8080),
			ReadTimeout:          getEnvDuration("API_READ_TIMEOUT", 10*time.Second),
			WriteTimeout:         getEnvDuration("API_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:          getEnvDuration("API_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:      getEnvDuration("API_SHUTDOWN_TIMEOUT", 10*time.Second),
			TrustedProxies:       getEnvCSV("TRUSTED_PROXIES", []string{}),
			CORSAllowedOrigins:   corsAllowedOrigins,
			SecureHeadersEnabled: getEnvBool("SECURE_HEADERS_ENABLED", true),
			RateLimitEnabled:     getEnvBool("RATE_LIMIT_ENABLED", appEnv == "production"),
			RateLimitPerMinute:   getEnvInt("RATE_LIMIT_PER_MINUTE", 120),
			RateLimitBurst:       getEnvInt("RATE_LIMIT_BURST", 60),
		},
		Metrics: MetricsConfig{
			Enabled: getEnvBool("METRICS_ENABLED", true),
			Path:    getEnv("METRICS_PATH", "/metrics"),
		},
		Telemetry: TelemetryConfig{
			Enabled:      getEnvBool("OTEL_ENABLED", false),
			OTLPEndpoint: strings.TrimRight(getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""), "/"),
			SampleRatio:  getEnvFloat("OTEL_TRACE_SAMPLE_RATIO", 0.1),
		},
		Worker: WorkerConfig{
			Concurrency: getEnvInt("WORKER_CONCURRENCY", 4),
		},
		ModuleRunner: ModuleRunnerConfig{
			ScriptAllowlist: getEnvMap("MODULE_RUNNER_SCRIPT_ALLOWLIST"),
		},
		Backup: BackupConfig{
			PGDumpPath: getEnv("BACKUP_PG_DUMP_PATH", "pg_dump"),
			Timeout:    getEnvDuration("BACKUP_TIMEOUT", 10*time.Minute),
		},
		Database: DatabaseConfig{
			Enabled:        getEnvBool("DATABASE_ENABLED", true),
			URL:            getEnv("DATABASE_URL", databaseURLFallback),
			MigrationsPath: getEnv("DATABASE_MIGRATIONS_PATH", "db/migrations"),
		},
		Redis: RedisConfig{
			Enabled: getEnvBool("REDIS_ENABLED", false),
			URL:     getEnv("REDIS_URL", "redis://localhost:6379/0"),
		},
		RabbitMQ: RabbitMQConfig{
			Enabled: getEnvBool("RABBITMQ_ENABLED", false),
			URL:     getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		},
		Storage: StorageConfig{
			Provider:        getEnv("STORAGE_PROVIDER", "local"),
			LocalPath:       getEnv("LOCAL_STORAGE_PATH", "data/storage"),
			Endpoint:        getEnv("S3_ENDPOINT", ""),
			Region:          getEnv("S3_REGION", "us-east-1"),
			Bucket:          getEnv("MINIO_BUCKET", "webtui-chat"),
			AccessKeyID:     getEnv("S3_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY", ""),
		},
		Security: SecurityConfig{
			JWTAccessSecret:       getEnv("JWT_ACCESS_SECRET", "dev_access_secret_local_only_do_not_use_in_production"),
			JWTRefreshSecret:      getEnv("JWT_REFRESH_SECRET", "dev_refresh_secret_local_only_do_not_use_in_production"),
			WebhookSigningSecret:  getEnv("WEBHOOK_SIGNING_SECRET", ""),
			StorageCredentialsKey: getEnv("STORAGE_CREDENTIALS_KEY", getEnv("WEBHOOK_SIGNING_SECRET", "")),
			BotAISecretKey:        getEnv("BOT_AI_SECRET_KEY", ""),
			GoogleClientID:        getEnv("GOOGLE_CLIENT_ID", ""),
			CaddyAskSecret:        getEnv("CADDY_ASK_SECRET", ""),
			OIDCStateSecret:       getEnv("OIDC_STATE_SECRET", ""),
			OIDCClientSecrets:     getEnvMap("OIDC_CLIENT_SECRETS"),
		},
		Calls: CallsConfig{
			RingTimeout:       getEnvDuration("CALL_RING_TIMEOUT", 30*time.Second),
			JitsiBaseURL:      strings.TrimRight(getEnv("JITSI_BASE_URL", getEnv("NEXT_PUBLIC_JITSI_BASE_URL", "")), "/"),
			TURNURLs:          getEnvCSV("TURN_URLS", []string{}),
			TURNSharedSecret:  getEnv("TURN_SHARED_SECRET", ""),
			TURNCredentialTTL: getEnvDuration("TURN_CREDENTIAL_TTL", 10*time.Minute),
			ICEServers: getEnvJSONObjectList(
				"RTC_ICE_SERVERS",
				getEnvJSONObjectList(
					"NEXT_PUBLIC_RTC_ICE_SERVERS",
					[]map[string]any{{"urls": "stun:stun.l.google.com:19302"}},
				),
			),
		},
		Firebase: FirebaseConfig{
			ProjectID:                getEnv("FIREBASE_PROJECT_ID", ""),
			ServiceAccountFile:       getEnv("FIREBASE_SERVICE_ACCOUNT_FILE", ""),
			ServiceAccountJSONBase64: getEnv("FIREBASE_SERVICE_ACCOUNT_JSON_BASE64", ""),
		},
		APNS: APNSConfig{
			KeyID:            getEnv("APNS_KEY_ID", ""),
			TeamID:           getEnv("APNS_TEAM_ID", ""),
			BundleID:         getEnv("APNS_BUNDLE_ID", ""),
			PrivateKeyFile:   getEnv("APNS_PRIVATE_KEY_FILE", ""),
			PrivateKeyBase64: getEnv("APNS_PRIVATE_KEY_BASE64", ""),
			Sandbox:          getEnvBool("APNS_SANDBOX", false),
		},
		PushRelay: PushRelayConfig{
			URL:        strings.TrimRight(getEnv("PUSH_RELAY_URL", ""), "/"),
			Token:      getEnv("PUSH_RELAY_TOKEN", ""),
			InstanceID: getEnv("PUSH_RELAY_INSTANCE_ID", ""),
		},
		PushRelayServer: PushRelayServerConfig{
			Enabled:            getEnvBool("PUSH_RELAY_SERVER_ENABLED", false),
			Host:               getEnv("PUSH_RELAY_HTTP_HOST", "0.0.0.0"),
			Port:               getEnvInt("PUSH_RELAY_HTTP_PORT", 8090),
			Publishers:         getEnvMap("PUSH_RELAY_PUBLISHERS"),
			MaxBodyBytes:       int64(getEnvInt("PUSH_RELAY_MAX_BODY_BYTES", 32768)),
			RateLimitPerMinute: getEnvInt("PUSH_RELAY_RATE_LIMIT_PER_MINUTE", 240),
			RateLimitBurst:     getEnvInt("PUSH_RELAY_RATE_LIMIT_BURST", 60),
			WorkerConcurrency:  getEnvInt("PUSH_RELAY_WORKER_CONCURRENCY", 4),
			PollInterval:       getEnvDuration("PUSH_RELAY_POLL_INTERVAL", time.Second),
		},
		WebPush: WebPushConfig{
			Enabled:                 getEnvBool("WEB_PUSH_ENABLED", false),
			VAPIDPublicKey:          getEnv("WEB_PUSH_VAPID_PUBLIC_KEY", ""),
			VAPIDPrivateKey:         getEnv("WEB_PUSH_VAPID_PRIVATE_KEY", ""),
			VAPIDSubject:            getEnv("WEB_PUSH_VAPID_SUBJECT", ""),
			TTL:                     getEnvInt("WEB_PUSH_TTL_SECONDS", 300),
			MaxSubscriptionsPerUser: getEnvInt("WEB_PUSH_MAX_SUBSCRIPTIONS_PER_USER", 10),
		},
		Deployment: DeploymentConfig{
			Mode:                     deploymentMode,
			InstanceDomain:           strings.ToLower(strings.TrimSpace(getEnv("INSTANCE_DOMAIN", instanceDomainFallback))),
			InstanceName:             strings.TrimSpace(getEnv("INSTANCE_NAME", "WebTui Chat")),
			InstanceLogoURL:          strings.TrimSpace(getEnv("INSTANCE_LOGO_URL", "")),
			InstanceRegistrationMode: strings.ToLower(strings.TrimSpace(getEnv("INSTANCE_REGISTRATION_MODE", "open"))),
		},
		Registration: RegistrationConfig{
			DefaultWorkspaceID:    getEnv("REGISTRATION_DEFAULT_WORKSPACE_ID", ""),
			CustomDomainDNSType:   strings.ToUpper(getEnv("CUSTOM_DOMAIN_DNS_TYPE", "")),
			CustomDomainDNSTarget: getEnv("CUSTOM_DOMAIN_DNS_TARGET", ""),
		},
		Order: OrderConfig{
			BaseURL:        getEnv("ORDER_API_BASE_URL", orderAPIBaseURLFallback),
			InternalAPIKey: getEnv("ORDER_INTERNAL_API_KEY", ""),
			QuickOrderKey:  getEnv("ORDER_QUICK_ORDER_KEY", ""),
			Timeout:        getEnvDuration("ORDER_API_TIMEOUT", 10*time.Second),
		},
	}

	return cfg, cfg.Validate()
}

func (c *Config) Validate() error {
	switch strings.ToLower(strings.TrimSpace(c.App.ServiceName)) {
	case "push-relay":
		return c.validatePushRelayService()
	case "migrate":
		return c.validateMigrateService()
	default:
		return c.validateApplicationService()
	}
}

func (c *Config) validateApplicationService() error {
	var problems []string

	if c.App.Name == "" {
		problems = append(problems, "APP_NAME không được để trống")
	}
	if strings.TrimSpace(c.Client.DesktopUpdateURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(c.Client.DesktopUpdateURL))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			problems = append(problems, "DESKTOP_UPDATE_URL must be a valid http/https URL")
		}
	}
	if strings.TrimSpace(c.Client.MobileDownloadURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(c.Client.MobileDownloadURL))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			problems = append(problems, "MOBILE_DOWNLOAD_URL must be a valid http/https URL")
		}
	}
	if strings.TrimSpace(c.Client.MobileStoreURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(c.Client.MobileStoreURL))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			problems = append(problems, "MOBILE_STORE_URL must be a valid http/https URL")
		}
	}
	relayURL := strings.TrimSpace(c.PushRelay.URL)
	relayToken := strings.TrimSpace(c.PushRelay.Token)
	relayInstanceID := strings.TrimSpace(c.PushRelay.InstanceID)
	relayConfigured := relayURL != "" || relayToken != "" || relayInstanceID != ""
	if relayConfigured {
		parsed, err := url.Parse(relayURL)
		validScheme := parsed != nil && (parsed.Scheme == "https" || (c.App.Env != "production" && parsed.Scheme == "http"))
		if err != nil || !validScheme || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
			problems = append(problems, "PUSH_RELAY_URL must be an HTTPS URL without embedded credentials")
		}
		if relayToken == "" {
			problems = append(problems, "PUSH_RELAY_TOKEN is required when push relay is configured")
		} else if c.App.Env == "production" && isWeakSecret(relayToken) {
			problems = append(problems, "PUSH_RELAY_TOKEN is not safe for production")
		}
		if !pushRelayInstancePattern.MatchString(relayInstanceID) {
			problems = append(problems, "PUSH_RELAY_INSTANCE_ID is required and must contain only letters, digits, dot, underscore, colon or dash")
		}
	}
	firebaseFile := strings.TrimSpace(c.Firebase.ServiceAccountFile)
	firebaseBase64 := strings.TrimSpace(c.Firebase.ServiceAccountJSONBase64)
	firebaseConfigured := strings.TrimSpace(c.Firebase.ProjectID) != "" || firebaseFile != "" || firebaseBase64 != ""
	if firebaseConfigured {
		if firebaseFile == "" && firebaseBase64 == "" {
			problems = append(problems, "FIREBASE_SERVICE_ACCOUNT_FILE or FIREBASE_SERVICE_ACCOUNT_JSON_BASE64 is required for direct FCM delivery")
		}
		if firebaseFile != "" && firebaseBase64 != "" {
			problems = append(problems, "configure only one Firebase service account source")
		}
	}
	apnsFile := strings.TrimSpace(c.APNS.PrivateKeyFile)
	apnsBase64 := strings.TrimSpace(c.APNS.PrivateKeyBase64)
	apnsConfigured := strings.TrimSpace(c.APNS.KeyID) != "" || strings.TrimSpace(c.APNS.TeamID) != "" ||
		apnsFile != "" || apnsBase64 != ""
	if apnsConfigured {
		if strings.TrimSpace(c.APNS.KeyID) == "" || strings.TrimSpace(c.APNS.TeamID) == "" ||
			strings.TrimSpace(c.APNS.BundleID) == "" {
			problems = append(problems, "APNS_KEY_ID, APNS_TEAM_ID and APNS_BUNDLE_ID are required for direct APNs delivery")
		}
		if apnsFile == "" && apnsBase64 == "" {
			problems = append(problems, "APNS_PRIVATE_KEY_FILE or APNS_PRIVATE_KEY_BASE64 is required for direct APNs delivery")
		}
		if apnsFile != "" && apnsBase64 != "" {
			problems = append(problems, "configure only one APNs private key source")
		}
	}
	if relayConfigured && (firebaseConfigured || apnsConfigured) {
		problems = append(problems, "push relay and direct FCM/APNs credentials cannot be configured together")
	}
	if c.PushRelayServer.Enabled {
		if relayConfigured {
			problems = append(problems, "PUSH_RELAY_SERVER_ENABLED cannot be combined with PUSH_RELAY_URL client mode")
		}
		if !c.Database.Enabled || strings.TrimSpace(c.Database.URL) == "" {
			problems = append(problems, "push relay server requires DATABASE_ENABLED=true and DATABASE_URL")
		}
		if net.ParseIP(c.PushRelayServer.Host) == nil && c.PushRelayServer.Host != "localhost" && c.PushRelayServer.Host != "" {
			problems = append(problems, "PUSH_RELAY_HTTP_HOST is invalid")
		}
		if c.PushRelayServer.Port <= 0 || c.PushRelayServer.Port > 65535 {
			problems = append(problems, "PUSH_RELAY_HTTP_PORT is invalid")
		}
		if c.PushRelayServer.MaxBodyBytes < 1024 || c.PushRelayServer.MaxBodyBytes > 1048576 {
			problems = append(problems, "PUSH_RELAY_MAX_BODY_BYTES must be between 1024 and 1048576")
		}
		if c.PushRelayServer.RateLimitPerMinute <= 0 || c.PushRelayServer.RateLimitBurst < 0 {
			problems = append(problems, "push relay rate limits must be positive")
		}
		if c.PushRelayServer.WorkerConcurrency <= 0 || c.PushRelayServer.WorkerConcurrency > 128 {
			problems = append(problems, "PUSH_RELAY_WORKER_CONCURRENCY must be between 1 and 128")
		}
		if c.PushRelayServer.PollInterval < 100*time.Millisecond || c.PushRelayServer.PollInterval > time.Minute {
			problems = append(problems, "PUSH_RELAY_POLL_INTERVAL must be between 100ms and 1m")
		}
		if strings.EqualFold(strings.TrimSpace(c.App.ServiceName), "push-relay") {
			if len(c.PushRelayServer.Publishers) == 0 {
				problems = append(problems, "PUSH_RELAY_PUBLISHERS is required when the relay server is enabled")
			}
			for publisherID, token := range c.PushRelayServer.Publishers {
				if !pushRelayInstancePattern.MatchString(publisherID) {
					problems = append(problems, "PUSH_RELAY_PUBLISHERS contains an invalid publisher ID")
					break
				}
				if len(strings.TrimSpace(token)) < 32 || (c.App.Env == "production" && isWeakSecret(token)) {
					problems = append(problems, "PUSH_RELAY_PUBLISHERS contains a weak token")
					break
				}
			}
			if !firebaseConfigured && !apnsConfigured {
				problems = append(problems, "push relay server requires at least one direct FCM or APNs provider")
			}
		}
	}
	if c.WebPush.Enabled {
		publicKeyValid := validVAPIDKey(c.WebPush.VAPIDPublicKey, 65)
		if !publicKeyValid {
			problems = append(problems, "WEB_PUSH_VAPID_PUBLIC_KEY must be a valid 65-byte URL-safe base64 key")
		}
		privateKey := strings.TrimSpace(c.WebPush.VAPIDPrivateKey)
		privateKeyValid := privateKey == "" || validVAPIDKey(privateKey, 32)
		if !privateKeyValid {
			problems = append(problems, "WEB_PUSH_VAPID_PRIVATE_KEY must be a valid 32-byte URL-safe base64 key")
		}
		if publicKeyValid && privateKey != "" && privateKeyValid &&
			!matchingVAPIDKeyPair(c.WebPush.VAPIDPublicKey, privateKey) {
			problems = append(problems, "WEB_PUSH_VAPID_PUBLIC_KEY and WEB_PUSH_VAPID_PRIVATE_KEY must form a matching key pair")
		}
		subjectValue := strings.TrimSpace(c.WebPush.VAPIDSubject)
		if subjectValue != "" {
			subject, err := url.Parse(subjectValue)
			validSubject := err == nil && subject != nil && subject.User == nil && subject.Fragment == ""
			if validSubject {
				switch subject.Scheme {
				case "mailto":
					validSubject = strings.Contains(subject.Opaque, "@") && !strings.ContainsAny(subject.Opaque, "\r\n")
				case "https":
					validSubject = subject.Host != ""
				default:
					validSubject = false
				}
			}
			if !validSubject {
				problems = append(problems, "WEB_PUSH_VAPID_SUBJECT must use mailto: or https:")
			}
		}
		if strings.EqualFold(strings.TrimSpace(c.App.ServiceName), "worker") && (privateKey == "" || subjectValue == "") {
			problems = append(problems, "WEB_PUSH_VAPID_PRIVATE_KEY and WEB_PUSH_VAPID_SUBJECT are required by the worker")
		}
		if c.WebPush.TTL < 0 || c.WebPush.TTL > 2419200 {
			problems = append(problems, "WEB_PUSH_TTL_SECONDS must be between 0 and 2419200")
		}
		if c.WebPush.MaxSubscriptionsPerUser <= 0 || c.WebPush.MaxSubscriptionsPerUser > 100 {
			problems = append(problems, "WEB_PUSH_MAX_SUBSCRIPTIONS_PER_USER must be between 1 and 100")
		}
	}
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		problems = append(problems, "API_HTTP_PORT không hợp lệ")
	}
	if c.HTTP.RateLimitEnabled && c.HTTP.RateLimitPerMinute <= 0 {
		problems = append(problems, "RATE_LIMIT_PER_MINUTE phải lớn hơn 0 khi RATE_LIMIT_ENABLED=true")
	}
	if c.HTTP.RateLimitBurst < 0 {
		problems = append(problems, "RATE_LIMIT_BURST không được nhỏ hơn 0")
	}
	if c.Metrics.Enabled && !strings.HasPrefix(c.Metrics.Path, "/") {
		problems = append(problems, "METRICS_PATH phải bắt đầu bằng /")
	}
	if c.Telemetry.Enabled {
		endpoint, err := url.Parse(strings.TrimSpace(c.Telemetry.OTLPEndpoint))
		if err != nil || endpoint == nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") ||
			endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
			problems = append(problems, "OTEL_EXPORTER_OTLP_ENDPOINT must be a valid HTTP(S) base URL without credentials, query or fragment")
		}
		if c.Telemetry.SampleRatio <= 0 || c.Telemetry.SampleRatio > 1 {
			problems = append(problems, "OTEL_TRACE_SAMPLE_RATIO must be greater than 0 and at most 1")
		}
	}
	if net.ParseIP(c.HTTP.Host) == nil && c.HTTP.Host != "localhost" && c.HTTP.Host != "" {
		problems = append(problems, "API_HTTP_HOST không hợp lệ")
	}
	if c.Worker.Concurrency <= 0 {
		problems = append(problems, "WORKER_CONCURRENCY phải lớn hơn 0")
	}
	if strings.TrimSpace(c.Backup.PGDumpPath) == "" {
		problems = append(problems, "BACKUP_PG_DUMP_PATH không được để trống")
	}
	if c.Backup.Timeout <= 0 {
		problems = append(problems, "BACKUP_TIMEOUT phải lớn hơn 0")
	}
	if c.Database.Enabled && c.Database.URL == "" {
		problems = append(problems, "DATABASE_URL bắt buộc khi DATABASE_ENABLED=true")
	}
	if c.Redis.Enabled && c.Redis.URL == "" {
		problems = append(problems, "REDIS_URL bắt buộc khi REDIS_ENABLED=true")
	}
	if c.RabbitMQ.Enabled && c.RabbitMQ.URL == "" {
		problems = append(problems, "RABBITMQ_URL bắt buộc khi RABBITMQ_ENABLED=true")
	}
	if c.Order.Timeout <= 0 {
		problems = append(problems, "ORDER_API_TIMEOUT must be greater than 0")
	}
	if c.Calls.RingTimeout <= 0 {
		problems = append(problems, "CALL_RING_TIMEOUT must be greater than 0")
	}
	if c.Calls.TURNCredentialTTL != 0 && (c.Calls.TURNCredentialTTL < time.Second || c.Calls.TURNCredentialTTL > 24*time.Hour) {
		problems = append(problems, "TURN_CREDENTIAL_TTL must be between 1s and 24h")
	}
	if len(c.Calls.TURNURLs) > 0 && len(strings.TrimSpace(c.Calls.TURNSharedSecret)) < 32 {
		problems = append(problems, "TURN_SHARED_SECRET must contain at least 32 characters when TURN_URLS is configured")
	}
	switch strings.ToLower(strings.TrimSpace(c.Deployment.Mode)) {
	case "saas":
	case "self_hosted":
		domain := strings.ToLower(strings.TrimSpace(c.Deployment.InstanceDomain))
		validLocalDomain := c.App.Env != "production" &&
			(domain == "localhost" || net.ParseIP(domain) != nil || strings.HasSuffix(domain, ".localhost"))
		if domain == "" || (!instanceDomainPattern.MatchString(domain) && !validLocalDomain) {
			problems = append(problems, "INSTANCE_DOMAIN must be a valid public domain when DEPLOYMENT_MODE=self_hosted")
		}
		if strings.TrimSpace(c.Deployment.InstanceName) == "" {
			problems = append(problems, "INSTANCE_NAME must not be empty when DEPLOYMENT_MODE=self_hosted")
		}
		if logoURL := strings.TrimSpace(c.Deployment.InstanceLogoURL); logoURL != "" {
			parsed, err := url.Parse(logoURL)
			if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
				problems = append(problems, "INSTANCE_LOGO_URL must be a public HTTPS URL without embedded credentials")
			}
		}
		switch strings.ToLower(strings.TrimSpace(c.Deployment.InstanceRegistrationMode)) {
		case "", "open", "invite_only", "closed":
		default:
			problems = append(problems, "INSTANCE_REGISTRATION_MODE only supports open, invite_only or closed")
		}
	default:
		problems = append(problems, "DEPLOYMENT_MODE only supports saas or self_hosted")
	}
	if c.Registration.DefaultWorkspaceID != "" && !uuidPattern.MatchString(c.Registration.DefaultWorkspaceID) {
		problems = append(problems, "REGISTRATION_DEFAULT_WORKSPACE_ID must be a valid UUID")
	}
	customDomainDNSType := strings.ToUpper(strings.TrimSpace(c.Registration.CustomDomainDNSType))
	customDomainDNSTarget := strings.TrimSpace(c.Registration.CustomDomainDNSTarget)
	if (customDomainDNSType == "") != (customDomainDNSTarget == "") {
		problems = append(problems, "CUSTOM_DOMAIN_DNS_TYPE and CUSTOM_DOMAIN_DNS_TARGET must be configured together")
	}
	if customDomainDNSType != "" {
		ip := net.ParseIP(customDomainDNSTarget)
		switch customDomainDNSType {
		case "A":
			if ip == nil || ip.To4() == nil {
				problems = append(problems, "CUSTOM_DOMAIN_DNS_TARGET must be a valid IPv4 address when CUSTOM_DOMAIN_DNS_TYPE=A")
			}
		case "AAAA":
			if ip == nil || ip.To4() != nil {
				problems = append(problems, "CUSTOM_DOMAIN_DNS_TARGET must be a valid IPv6 address when CUSTOM_DOMAIN_DNS_TYPE=AAAA")
			}
		default:
			problems = append(problems, "CUSTOM_DOMAIN_DNS_TYPE only supports A or AAAA")
		}
	}
	if strings.TrimSpace(c.Order.BaseURL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(c.Order.BaseURL))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			problems = append(problems, "ORDER_API_BASE_URL must be a valid http/https URL")
		}
	}
	switch strings.ToLower(strings.TrimSpace(c.Storage.Provider)) {
	case "", "local":
	case "minio", "s3":
		if strings.TrimSpace(c.Storage.Endpoint) == "" {
			problems = append(problems, "S3_ENDPOINT bắt buộc khi STORAGE_PROVIDER=minio hoặc s3")
		}
		if strings.TrimSpace(c.Storage.Bucket) == "" {
			problems = append(problems, "MINIO_BUCKET bắt buộc khi STORAGE_PROVIDER=minio hoặc s3")
		}
		if strings.TrimSpace(c.Storage.AccessKeyID) == "" {
			problems = append(problems, "S3_ACCESS_KEY_ID bắt buộc khi STORAGE_PROVIDER=minio hoặc s3")
		}
		if strings.TrimSpace(c.Storage.SecretAccessKey) == "" {
			problems = append(problems, "S3_SECRET_ACCESS_KEY bắt buộc khi STORAGE_PROVIDER=minio hoặc s3")
		}
	default:
		problems = append(problems, "STORAGE_PROVIDER chỉ hỗ trợ local, minio hoặc s3")
	}
	if c.App.Env == "production" {
		if !c.Database.Enabled || c.Database.URL == "" {
			problems = append(problems, "DATABASE_URL bắt buộc trong production")
		}
		if isWeakSecret(c.Security.JWTAccessSecret) {
			problems = append(problems, "JWT_ACCESS_SECRET chưa an toàn")
		}
		if isWeakSecret(c.Security.JWTRefreshSecret) {
			problems = append(problems, "JWT_REFRESH_SECRET chưa an toàn")
		}
		if isWeakSecret(c.Security.WebhookSigningSecret) {
			problems = append(problems, "WEBHOOK_SIGNING_SECRET chưa an toàn")
		}
		if isWeakSecret(c.Security.StorageCredentialsKey) {
			problems = append(problems, "STORAGE_CREDENTIALS_KEY chưa an toàn")
		}
		if isWeakSecret(c.Security.BotAISecretKey) {
			problems = append(problems, "BOT_AI_SECRET_KEY chưa an toàn")
		}
	}
	if c.Security.OIDCStateSecret != "" && isWeakSecret(c.Security.OIDCStateSecret) {
		problems = append(problems, "OIDC_STATE_SECRET chưa an toàn")
	}
	if len(c.Security.OIDCClientSecrets) > 0 && strings.TrimSpace(c.Security.OIDCStateSecret) == "" {
		problems = append(problems, "OIDC_STATE_SECRET bắt buộc khi OIDC_CLIENT_SECRETS được cấu hình")
	}
	for alias := range c.Security.OIDCClientSecrets {
		if !oidcSecretAliasPattern.MatchString(strings.ToLower(strings.TrimSpace(alias))) {
			problems = append(problems, "OIDC_CLIENT_SECRETS chứa alias không hợp lệ")
			break
		}
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (c *Config) validateMigrateService() error {
	problems := c.validateOperationalServiceBase()
	if strings.TrimSpace(c.Database.MigrationsPath) == "" {
		problems = append(problems, "DATABASE_MIGRATIONS_PATH is required for SERVICE_NAME=migrate")
	}
	return validationProblems(problems)
}

func (c *Config) validatePushRelayService() error {
	problems := c.validateOperationalServiceBase()
	if c.Telemetry.Enabled {
		endpoint, err := url.Parse(strings.TrimSpace(c.Telemetry.OTLPEndpoint))
		if err != nil || endpoint == nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") ||
			endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
			problems = append(problems, "OTEL_EXPORTER_OTLP_ENDPOINT must be a valid HTTP(S) base URL without credentials, query or fragment")
		}
		if c.Telemetry.SampleRatio <= 0 || c.Telemetry.SampleRatio > 1 {
			problems = append(problems, "OTEL_TRACE_SAMPLE_RATIO must be greater than 0 and at most 1")
		}
	}

	relayClientConfigured := strings.TrimSpace(c.PushRelay.URL) != "" ||
		strings.TrimSpace(c.PushRelay.Token) != "" || strings.TrimSpace(c.PushRelay.InstanceID) != ""
	if relayClientConfigured {
		problems = append(problems, "SERVICE_NAME=push-relay cannot be combined with PUSH_RELAY_URL client mode")
	}
	if !c.PushRelayServer.Enabled {
		problems = append(problems, "PUSH_RELAY_SERVER_ENABLED=true is required for SERVICE_NAME=push-relay")
	}
	if net.ParseIP(c.PushRelayServer.Host) == nil && c.PushRelayServer.Host != "localhost" && c.PushRelayServer.Host != "" {
		problems = append(problems, "PUSH_RELAY_HTTP_HOST is invalid")
	}
	if c.PushRelayServer.Port <= 0 || c.PushRelayServer.Port > 65535 {
		problems = append(problems, "PUSH_RELAY_HTTP_PORT is invalid")
	}
	if c.PushRelayServer.MaxBodyBytes < 1024 || c.PushRelayServer.MaxBodyBytes > 1048576 {
		problems = append(problems, "PUSH_RELAY_MAX_BODY_BYTES must be between 1024 and 1048576")
	}
	if c.PushRelayServer.RateLimitPerMinute <= 0 || c.PushRelayServer.RateLimitBurst < 0 {
		problems = append(problems, "push relay rate limits must be positive")
	}
	if c.PushRelayServer.WorkerConcurrency <= 0 || c.PushRelayServer.WorkerConcurrency > 128 {
		problems = append(problems, "PUSH_RELAY_WORKER_CONCURRENCY must be between 1 and 128")
	}
	if c.PushRelayServer.PollInterval < 100*time.Millisecond || c.PushRelayServer.PollInterval > time.Minute {
		problems = append(problems, "PUSH_RELAY_POLL_INTERVAL must be between 100ms and 1m")
	}
	if len(c.PushRelayServer.Publishers) == 0 {
		problems = append(problems, "PUSH_RELAY_PUBLISHERS is required for SERVICE_NAME=push-relay")
	}
	production := strings.EqualFold(strings.TrimSpace(c.App.Env), "production")
	for publisherID, token := range c.PushRelayServer.Publishers {
		if !pushRelayInstancePattern.MatchString(publisherID) {
			problems = append(problems, "PUSH_RELAY_PUBLISHERS contains an invalid publisher ID")
			break
		}
		if len(strings.TrimSpace(token)) < 32 || (production && isWeakSecret(token)) {
			problems = append(problems, "PUSH_RELAY_PUBLISHERS contains a weak token")
			break
		}
	}

	firebaseFile := strings.TrimSpace(c.Firebase.ServiceAccountFile)
	firebaseBase64 := strings.TrimSpace(c.Firebase.ServiceAccountJSONBase64)
	firebaseConfigured := strings.TrimSpace(c.Firebase.ProjectID) != "" || firebaseFile != "" || firebaseBase64 != ""
	if firebaseConfigured {
		if firebaseFile == "" && firebaseBase64 == "" {
			problems = append(problems, "FIREBASE_SERVICE_ACCOUNT_FILE or FIREBASE_SERVICE_ACCOUNT_JSON_BASE64 is required for relay FCM delivery")
		}
		if firebaseFile != "" && firebaseBase64 != "" {
			problems = append(problems, "configure only one Firebase service account source")
		}
		if production && firebaseBase64 != "" && isWeakSecret(firebaseBase64) {
			problems = append(problems, "FIREBASE_SERVICE_ACCOUNT_JSON_BASE64 is not safe for production")
		}
	}
	apnsFile := strings.TrimSpace(c.APNS.PrivateKeyFile)
	apnsBase64 := strings.TrimSpace(c.APNS.PrivateKeyBase64)
	apnsConfigured := strings.TrimSpace(c.APNS.KeyID) != "" || strings.TrimSpace(c.APNS.TeamID) != "" ||
		apnsFile != "" || apnsBase64 != ""
	if apnsConfigured {
		if strings.TrimSpace(c.APNS.KeyID) == "" || strings.TrimSpace(c.APNS.TeamID) == "" ||
			strings.TrimSpace(c.APNS.BundleID) == "" {
			problems = append(problems, "APNS_KEY_ID, APNS_TEAM_ID and APNS_BUNDLE_ID are required for relay APNs delivery")
		}
		if apnsFile == "" && apnsBase64 == "" {
			problems = append(problems, "APNS_PRIVATE_KEY_FILE or APNS_PRIVATE_KEY_BASE64 is required for relay APNs delivery")
		}
		if apnsFile != "" && apnsBase64 != "" {
			problems = append(problems, "configure only one APNs private key source")
		}
		if production && apnsBase64 != "" && isWeakSecret(apnsBase64) {
			problems = append(problems, "APNS_PRIVATE_KEY_BASE64 is not safe for production")
		}
	}
	if !firebaseConfigured && !apnsConfigured {
		problems = append(problems, "push relay server requires at least one direct FCM or APNs provider")
	}

	return validationProblems(problems)
}

func (c *Config) validateOperationalServiceBase() []string {
	problems := make([]string, 0)
	if strings.TrimSpace(c.App.Name) == "" {
		problems = append(problems, "APP_NAME must not be empty")
	}
	switch strings.ToLower(strings.TrimSpace(c.App.Env)) {
	case "dev", "development", "test", "staging", "production":
	default:
		problems = append(problems, "APP_ENV must be dev, development, test, staging or production")
	}
	if !c.Database.Enabled || strings.TrimSpace(c.Database.URL) == "" {
		problems = append(problems, "DATABASE_ENABLED=true and DATABASE_URL are required for "+strings.TrimSpace(c.App.ServiceName))
	}
	return problems
}

func validationProblems(problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(problems, "; "))
}

func isOperationalService(serviceName string) bool {
	switch strings.ToLower(strings.TrimSpace(serviceName)) {
	case "push-relay", "migrate":
		return true
	default:
		return false
	}
}

func (c HTTPConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getEnvFloat(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func getEnvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getEnvJSONObjectList(key string, fallback []map[string]any) []map[string]any {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	var values []map[string]any
	if err := json.Unmarshal([]byte(raw), &values); err != nil || len(values) == 0 {
		return fallback
	}
	return values
}

func getEnvCSV(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	values := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return fallback
	}
	return values
}

func getEnvCSVWithDefaults(key string, defaults []string) []string {
	values := append([]string{}, defaults...)
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return uniqueCSVValues(values)
	}

	for _, part := range strings.Split(raw, ",") {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if value == "*" {
			return []string{"*"}
		}
		values = append(values, value)
	}

	return uniqueCSVValues(values)
}

func uniqueCSVValues(values []string) []string {
	seen := make(map[string]bool, len(values))
	unique := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, value)
	}

	return unique
}

func getEnvMap(key string) map[string]string {
	raw := strings.TrimSpace(os.Getenv(key))
	values := map[string]string{}
	if raw == "" {
		return values
	}
	for _, pair := range strings.Split(raw, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" || !strings.Contains(pair, "=") {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		name := strings.ToLower(strings.TrimSpace(parts[0]))
		path := strings.TrimSpace(parts[1])
		if name == "" || path == "" {
			continue
		}
		values[name] = path
	}
	return values
}

func isWeakSecret(secret string) bool {
	secret = strings.TrimSpace(secret)
	normalized := strings.ToLower(secret)
	return len(secret) < 32 ||
		strings.Contains(normalized, "change_me") ||
		strings.Contains(normalized, "local_only") ||
		strings.Contains(normalized, "do_not_use_in_production")
}

func validVAPIDKey(value string, expectedBytes int) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) != expectedBytes {
		return false
	}
	switch expectedBytes {
	case 65:
		x, y := elliptic.Unmarshal(elliptic.P256(), raw)
		return x != nil && y != nil
	case 32:
		scalar := new(big.Int).SetBytes(raw)
		return scalar.Sign() > 0 && scalar.Cmp(elliptic.P256().Params().N) < 0
	default:
		return true
	}
}

func matchingVAPIDKeyPair(publicValue string, privateValue string) bool {
	publicKey, publicErr := base64.RawURLEncoding.DecodeString(strings.TrimSpace(publicValue))
	privateKey, privateErr := base64.RawURLEncoding.DecodeString(strings.TrimSpace(privateValue))
	if publicErr != nil || privateErr != nil || len(publicKey) != 65 || len(privateKey) != 32 {
		return false
	}
	x, y := elliptic.P256().ScalarBaseMult(privateKey)
	return bytes.Equal(publicKey, elliptic.Marshal(elliptic.P256(), x, y))
}
