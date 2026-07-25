package config

import (
	"encoding/json"
	"errors"
	"fmt"
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

type Config struct {
	App          AppConfig
	Client       ClientConfig
	HTTP         HTTPConfig
	Metrics      MetricsConfig
	Worker       WorkerConfig
	ModuleRunner ModuleRunnerConfig
	Backup       BackupConfig
	Database     DatabaseConfig
	Redis        RedisConfig
	RabbitMQ     RabbitMQConfig
	Storage      StorageConfig
	Security     SecurityConfig
	Calls        CallsConfig
	Firebase     FirebaseConfig
	Deployment   DeploymentConfig
	Registration RegistrationConfig
	Order        OrderConfig
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
	JWTAccessSecret      string
	JWTRefreshSecret     string
	WebhookSigningSecret string
	GoogleClientID       string
	CaddyAskSecret       string
	OIDCStateSecret      string
	OIDCClientSecrets    map[string]string
}

type CallsConfig struct {
	RingTimeout time.Duration
	ICEServers  []map[string]any
}

type FirebaseConfig struct {
	ProjectID                string
	ServiceAccountFile       string
	ServiceAccountJSONBase64 string
}

type DeploymentConfig struct {
	Mode           string
	InstanceDomain string
	InstanceName   string
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
	deploymentMode := strings.ToLower(getEnv("DEPLOYMENT_MODE", "self_hosted"))
	instanceDomainFallback := ""
	orderAPIBaseURLFallback := ""
	if deploymentMode == "self_hosted" && appEnv != "production" {
		instanceDomainFallback = "localhost"
	}
	if deploymentMode == "saas" {
		orderAPIBaseURLFallback = "https://order.vpsttt.com/api"
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
			ServiceName: getEnv("SERVICE_NAME", "api"),
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
			CORSAllowedOrigins:   getEnvCSVWithDefaults("CORS_ALLOWED_ORIGINS", defaultCORSAllowedOrigins),
			SecureHeadersEnabled: getEnvBool("SECURE_HEADERS_ENABLED", true),
			RateLimitEnabled:     getEnvBool("RATE_LIMIT_ENABLED", true),
			RateLimitPerMinute:   getEnvInt("RATE_LIMIT_PER_MINUTE", 120),
			RateLimitBurst:       getEnvInt("RATE_LIMIT_BURST", 60),
		},
		Metrics: MetricsConfig{
			Enabled: getEnvBool("METRICS_ENABLED", true),
			Path:    getEnv("METRICS_PATH", "/metrics"),
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
			URL:            getEnv("DATABASE_URL", "postgres://postgres:123456@localhost:5432/vpstttdb_chat?sslmode=disable"),
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
			JWTAccessSecret:      getEnv("JWT_ACCESS_SECRET", "dev_access_secret_local_only_do_not_use_in_production"),
			JWTRefreshSecret:     getEnv("JWT_REFRESH_SECRET", "dev_refresh_secret_local_only_do_not_use_in_production"),
			WebhookSigningSecret: getEnv("WEBHOOK_SIGNING_SECRET", ""),
			GoogleClientID:       getEnv("GOOGLE_CLIENT_ID", ""),
			CaddyAskSecret:       getEnv("CADDY_ASK_SECRET", ""),
			OIDCStateSecret:      getEnv("OIDC_STATE_SECRET", ""),
			OIDCClientSecrets:    getEnvMap("OIDC_CLIENT_SECRETS"),
		},
		Calls: CallsConfig{
			RingTimeout: getEnvDuration("CALL_RING_TIMEOUT", 30*time.Second),
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
		Deployment: DeploymentConfig{
			Mode:           deploymentMode,
			InstanceDomain: strings.ToLower(strings.TrimSpace(getEnv("INSTANCE_DOMAIN", instanceDomainFallback))),
			InstanceName:   strings.TrimSpace(getEnv("INSTANCE_NAME", "VPSTTT Chat")),
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
	}
	if c.Security.OIDCStateSecret != "" && isWeakSecret(c.Security.OIDCStateSecret) {
		problems = append(problems, "OIDC_STATE_SECRET chua an toan")
	}
	if len(c.Security.OIDCClientSecrets) > 0 && strings.TrimSpace(c.Security.OIDCStateSecret) == "" {
		problems = append(problems, "OIDC_STATE_SECRET bat buoc khi OIDC_CLIENT_SECRETS duoc cau hinh")
	}
	for alias := range c.Security.OIDCClientSecrets {
		if !oidcSecretAliasPattern.MatchString(strings.ToLower(strings.TrimSpace(alias))) {
			problems = append(problems, "OIDC_CLIENT_SECRETS chua alias khong hop le")
			break
		}
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
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
