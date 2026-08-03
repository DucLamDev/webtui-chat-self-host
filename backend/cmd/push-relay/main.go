package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/duclamdev/application-chat/backend/internal/config"
	notificationsapns "github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/apns"
	notificationsfcm "github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/fcm"
	"github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/relayserver"
	"github.com/duclamdev/application-chat/backend/internal/platform/database"
	"github.com/duclamdev/application-chat/backend/internal/platform/observability"
)

func main() {
	if strings.TrimSpace(os.Getenv("SERVICE_NAME")) == "" {
		_ = os.Setenv("SERVICE_NAME", "push-relay")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error("push relay stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load push relay configuration: %w", err)
	}
	configureLogger(cfg)
	telemetryShutdown, err := observability.Setup(ctx, observability.Config{
		Enabled: cfg.Telemetry.Enabled, Endpoint: cfg.Telemetry.OTLPEndpoint,
		ServiceName: cfg.App.ServiceName, Version: cfg.App.Version,
		Environment: cfg.App.Env, SampleRatio: cfg.Telemetry.SampleRatio,
	})
	if err != nil {
		return fmt.Errorf("initialize OpenTelemetry: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := telemetryShutdown(shutdownCtx); err != nil {
			slog.Warn("push relay telemetry shutdown failed")
		}
	}()
	if !cfg.PushRelayServer.Enabled {
		return errors.New("push relay server is disabled; set PUSH_RELAY_SERVER_ENABLED=true explicitly")
	}

	db, err := database.NewPostgres(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connect push relay database: %w", err)
	}
	if db == nil {
		return errors.New("push relay database is disabled")
	}
	defer db.Close()

	senders, err := configuredSenders(cfg)
	if err != nil {
		return fmt.Errorf("initialize push relay provider: %w", err)
	}
	relay, err := relayserver.New(relayserver.NewPostgresStore(db.Pool()), relayserver.Config{
		Publishers: cfg.PushRelayServer.Publishers, MaxBodyBytes: cfg.PushRelayServer.MaxBodyBytes,
		RateLimitPerMinute: cfg.PushRelayServer.RateLimitPerMinute,
		RateLimitBurst:     cfg.PushRelayServer.RateLimitBurst,
		WorkerConcurrency:  cfg.PushRelayServer.WorkerConcurrency,
		PollInterval:       cfg.PushRelayServer.PollInterval,
	}, senders...)
	if err != nil {
		return fmt.Errorf("initialize push relay: %w", err)
	}
	addr := net.JoinHostPort(cfg.PushRelayServer.Host, strconv.Itoa(cfg.PushRelayServer.Port))
	if err := relay.Run(ctx, addr); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func configuredSenders(cfg *config.Config) ([]relayserver.PushSender, error) {
	senders := make([]relayserver.PushSender, 0, 2)
	if anyConfigured(cfg.Firebase.ProjectID, cfg.Firebase.ServiceAccountFile, cfg.Firebase.ServiceAccountJSONBase64) {
		sender := notificationsfcm.NewSender(notificationsfcm.Config{
			ProjectID: cfg.Firebase.ProjectID, ServiceAccountFile: cfg.Firebase.ServiceAccountFile,
			ServiceAccountJSONBase64: cfg.Firebase.ServiceAccountJSONBase64,
		})
		if err := sender.InitializationError(); err != nil || !sender.Enabled() {
			if err == nil {
				err = errors.New("FCM sender is disabled")
			}
			return nil, err
		}
		senders = append(senders, sender)
	}
	if anyConfigured(cfg.APNS.KeyID, cfg.APNS.TeamID, cfg.APNS.PrivateKeyFile, cfg.APNS.PrivateKeyBase64) {
		sender := notificationsapns.NewSender(notificationsapns.Config{
			KeyID: cfg.APNS.KeyID, TeamID: cfg.APNS.TeamID, BundleID: cfg.APNS.BundleID,
			PrivateKeyFile: cfg.APNS.PrivateKeyFile, PrivateKeyBase64: cfg.APNS.PrivateKeyBase64,
			Sandbox: cfg.APNS.Sandbox,
		})
		if err := sender.InitializationError(); err != nil || !sender.Enabled() {
			if err == nil {
				err = errors.New("APNs sender is disabled")
			}
			return nil, err
		}
		senders = append(senders, sender)
	}
	return senders, nil
}

func anyConfigured(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func configureLogger(cfg *config.Config) {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(cfg.App.LogLevel)) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	options := &slog.HandlerOptions{Level: level}
	if strings.EqualFold(cfg.App.LogFormat, "text") {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, options)))
		return
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, options)))
}
