package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadAllowsProductionPushRelayAllowlist(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_NAME", "webtui-push-relay")
	t.Setenv("SERVICE_NAME", "push-relay")
	t.Setenv("DATABASE_ENABLED", "true")
	t.Setenv("DATABASE_URL", "postgres://relay:secret@postgres:5432/push_relay?sslmode=require")
	t.Setenv("PUSH_RELAY_SERVER_ENABLED", "true")
	t.Setenv("PUSH_RELAY_HTTP_HOST", "0.0.0.0")
	t.Setenv("PUSH_RELAY_HTTP_PORT", "8090")
	t.Setenv("PUSH_RELAY_PUBLISHERS", testCanonicalInstanceID+"=publisher-token-with-at-least-thirty-two-random-characters")
	t.Setenv("PUSH_RELAY_MAX_BODY_BYTES", "32768")
	t.Setenv("PUSH_RELAY_RATE_LIMIT_PER_MINUTE", "240")
	t.Setenv("PUSH_RELAY_RATE_LIMIT_BURST", "60")
	t.Setenv("PUSH_RELAY_WORKER_CONCURRENCY", "4")
	t.Setenv("PUSH_RELAY_POLL_INTERVAL", "1s")
	t.Setenv("FIREBASE_PROJECT_ID", "official-mobile")
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_FILE", "/run/secrets/firebase.json")
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON_BASE64", "")
	t.Setenv("APNS_KEY_ID", "")
	t.Setenv("APNS_TEAM_ID", "")
	t.Setenv("APNS_PRIVATE_KEY_FILE", "")
	t.Setenv("APNS_PRIVATE_KEY_BASE64", "")
	t.Setenv("OTEL_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4318")
	t.Setenv("OTEL_TRACE_SAMPLE_RATIO", "0.25")

	// JWT, webhook, bot, OIDC, TURN, storage and instance-domain variables are
	// deliberately not part of this relay environment.

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want production relay allowlist to validate", err)
	}
	if cfg.App.ServiceName != "push-relay" || !cfg.PushRelayServer.Enabled {
		t.Fatalf("loaded wrong role: service=%q enabled=%v", cfg.App.ServiceName, cfg.PushRelayServer.Enabled)
	}
}

func TestValidatePushRelayIgnoresApplicationOnlyConfiguration(t *testing.T) {
	cfg := validPushRelayRoleConfig()
	cfg.Client.DesktopUpdateURL = "not-a-url"
	cfg.HTTP.Port = -1
	cfg.Worker.Concurrency = 0
	cfg.Backup = BackupConfig{}
	cfg.Storage = StorageConfig{Provider: "s3"}
	cfg.Security = SecurityConfig{
		JWTAccessSecret:      "change_me",
		JWTRefreshSecret:     "change_me",
		WebhookSigningSecret: "change_me",
		BotAISecretKey:       "change_me",
		OIDCClientSecrets:    map[string]string{"../../invalid": "secret"},
	}
	cfg.Calls = CallsConfig{TURNURLs: []string{"turn:unused.example"}}
	cfg.Deployment = DeploymentConfig{Mode: "self_hosted"}
	cfg.WebPush = WebPushConfig{Enabled: true}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want application-only settings ignored by push-relay", err)
	}
}

func TestValidatePushRelayRejectsRequiredRoleGaps(t *testing.T) {
	tests := []struct {
		name       string
		change     func(*Config)
		wantErrors []string
	}{
		{
			name: "database disabled",
			change: func(cfg *Config) {
				cfg.Database.Enabled = false
			},
			wantErrors: []string{"DATABASE_ENABLED=true", "DATABASE_URL"},
		},
		{
			name: "server not explicitly enabled",
			change: func(cfg *Config) {
				cfg.PushRelayServer.Enabled = false
			},
			wantErrors: []string{"PUSH_RELAY_SERVER_ENABLED=true"},
		},
		{
			name: "publisher authentication missing",
			change: func(cfg *Config) {
				cfg.PushRelayServer.Publishers = nil
			},
			wantErrors: []string{"PUSH_RELAY_PUBLISHERS"},
		},
		{
			name: "publisher token weak in production",
			change: func(cfg *Config) {
				cfg.PushRelayServer.Publishers = map[string]string{testCanonicalInstanceID: "CHANGE_ME_RELAY_TOKEN"}
			},
			wantErrors: []string{"PUSH_RELAY_PUBLISHERS contains a weak token"},
		},
		{
			name: "publisher identity is not a discovery UUID",
			change: func(cfg *Config) {
				cfg.PushRelayServer.Publishers = map[string]string{"instance-a": "publisher-token-with-at-least-thirty-two-random-characters"}
			},
			wantErrors: []string{"canonical lowercase discovery instance UUID"},
		},
		{
			name: "listen and worker settings invalid",
			change: func(cfg *Config) {
				cfg.PushRelayServer.Host = "relay host"
				cfg.PushRelayServer.Port = 0
				cfg.PushRelayServer.WorkerConcurrency = 0
				cfg.PushRelayServer.PollInterval = time.Millisecond
			},
			wantErrors: []string{"PUSH_RELAY_HTTP_HOST", "PUSH_RELAY_HTTP_PORT", "PUSH_RELAY_WORKER_CONCURRENCY", "PUSH_RELAY_POLL_INTERVAL"},
		},
		{
			name: "provider missing",
			change: func(cfg *Config) {
				cfg.Firebase = FirebaseConfig{}
			},
			wantErrors: []string{"at least one direct FCM or APNs provider"},
		},
		{
			name: "FCM provider incomplete",
			change: func(cfg *Config) {
				cfg.Firebase = FirebaseConfig{ProjectID: "official-mobile"}
			},
			wantErrors: []string{"FIREBASE_SERVICE_ACCOUNT_FILE", "FIREBASE_SERVICE_ACCOUNT_JSON_BASE64"},
		},
		{
			name: "APNs provider incomplete",
			change: func(cfg *Config) {
				cfg.Firebase = FirebaseConfig{}
				cfg.APNS = APNSConfig{KeyID: "key-id"}
			},
			wantErrors: []string{"APNS_TEAM_ID", "APNS_PRIVATE_KEY_FILE"},
		},
		{
			name: "client relay mode conflicts",
			change: func(cfg *Config) {
				cfg.PushRelay = PushRelayConfig{URL: "https://customer-relay.example/v1/deliveries"}
			},
			wantErrors: []string{"cannot be combined with PUSH_RELAY_URL client mode"},
		},
		{
			name: "enabled telemetry invalid",
			change: func(cfg *Config) {
				cfg.Telemetry = TelemetryConfig{Enabled: true, OTLPEndpoint: "collector:4318", SampleRatio: 0}
			},
			wantErrors: []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_TRACE_SAMPLE_RATIO"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validPushRelayRoleConfig()
			tt.change(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want role validation failure")
			}
			for _, expected := range tt.wantErrors {
				if !strings.Contains(err.Error(), expected) {
					t.Fatalf("Validate() error = %v, want %q", err, expected)
				}
			}
		})
	}
}

func TestLoadAllowsProductionMigrateAllowlist(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_NAME", "webtui-migrate")
	t.Setenv("SERVICE_NAME", "migrate")
	t.Setenv("DATABASE_ENABLED", "true")
	t.Setenv("DATABASE_URL", "postgres://migrate:secret@postgres:5432/app?sslmode=require")
	t.Setenv("DATABASE_MIGRATIONS_PATH", "db/migrations")

	if _, err := Load(); err != nil {
		t.Fatalf("Load() error = %v, want production migrate allowlist to validate", err)
	}
}

func TestValidateMigrateRejectsOnlyRequiredRoleGaps(t *testing.T) {
	cfg := &Config{
		App: AppConfig{Name: "webtui-migrate", Env: "production", ServiceName: "migrate"},
		Database: DatabaseConfig{
			Enabled:        true,
			URL:            "postgres://migrate:secret@postgres:5432/app?sslmode=require",
			MigrationsPath: "db/migrations",
		},
		Security:   SecurityConfig{JWTAccessSecret: "change_me"},
		Storage:    StorageConfig{Provider: "s3"},
		Deployment: DeploymentConfig{Mode: "self_hosted"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want application-only settings ignored by migrate", err)
	}

	cfg.Database.URL = ""
	cfg.Database.MigrationsPath = ""
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || !strings.Contains(err.Error(), "DATABASE_MIGRATIONS_PATH") {
		t.Fatalf("Validate() error = %v, want database and migration path rejection", err)
	}
}

func validPushRelayRoleConfig() *Config {
	return &Config{
		App: AppConfig{Name: "webtui-push-relay", Env: "production", ServiceName: "push-relay"},
		Database: DatabaseConfig{
			Enabled: true,
			URL:     "postgres://relay:secret@postgres:5432/push_relay?sslmode=require",
		},
		PushRelayServer: PushRelayServerConfig{
			Enabled:            true,
			Host:               "0.0.0.0",
			Port:               8090,
			Publishers:         map[string]string{testCanonicalInstanceID: "publisher-token-with-at-least-thirty-two-random-characters"},
			MaxBodyBytes:       32768,
			RateLimitPerMinute: 240,
			RateLimitBurst:     60,
			WorkerConcurrency:  4,
			PollInterval:       time.Second,
		},
		Firebase: FirebaseConfig{
			ProjectID:          "official-mobile",
			ServiceAccountFile: "/run/secrets/firebase.json",
		},
	}
}
