package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/duclamdev/application-chat/backend/internal/config"
	tenancypostgres "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/infrastructure/postgres"
	"github.com/duclamdev/application-chat/backend/internal/platform/observability"
	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type API struct {
	cfg       *config.Config
	resources *Resources
	engine    *gin.Engine
	server    *http.Server
	tokens    *sharedauth.Manager
	telemetry observability.Shutdown
}

func NewAPI(cfg *config.Config) (*API, error) {
	configureLogger(cfg)
	configureGin(cfg)

	resources, err := NewResources(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	if cfg.Deployment.IsSelfHosted() {
		if resources.Database == nil {
			resources.Close()
			return nil, fmt.Errorf("self-hosted mode requires DATABASE_ENABLED=true")
		}
		workspaceID, bootstrapErr := tenancypostgres.EnsureSelfHostedInstance(
			context.Background(),
			resources.Database.Pool(),
			tenancypostgres.SelfHostedInstanceParams{
				Domain:           cfg.Deployment.InstanceDomain,
				Name:             cfg.Deployment.InstanceName,
				LogoURL:          cfg.Deployment.InstanceLogoURL,
				RegistrationMode: cfg.Deployment.InstanceRegistrationMode,
			},
		)
		if bootstrapErr != nil {
			resources.Close()
			return nil, fmt.Errorf("bootstrap self-hosted instance: %w", bootstrapErr)
		}
		cfg.Registration.DefaultWorkspaceID = workspaceID
	}
	tokenManager := sharedauth.NewManager(
		cfg.Security.JWTAccessSecret,
		cfg.Security.JWTRefreshSecret,
		15*time.Minute,
		30*24*time.Hour,
	)

	engine := gin.New()
	if err := engine.SetTrustedProxies(cfg.HTTP.TrustedProxies); err != nil {
		resources.Close()
		return nil, err
	}
	telemetryShutdown, err := observability.Setup(context.Background(), observability.Config{
		Enabled: cfg.Telemetry.Enabled, Endpoint: cfg.Telemetry.OTLPEndpoint,
		ServiceName: cfg.App.ServiceName, Version: cfg.App.Version,
		Environment: cfg.App.Env, SampleRatio: cfg.Telemetry.SampleRatio,
	})
	if err != nil {
		resources.Close()
		return nil, fmt.Errorf("initialize OpenTelemetry: %w", err)
	}
	engine.Use(
		middleware.RequestID(),
		middleware.Recovery(),
		middleware.AccessLog(),
	)
	if cfg.Telemetry.Enabled {
		engine.Use(otelgin.Middleware(cfg.App.ServiceName))
	}
	if cfg.HTTP.SecureHeadersEnabled {
		engine.Use(middleware.SecurityHeaders(cfg.App.Env == "production"))
	}
	engine.Use(middleware.CORS(cfg.HTTP.CORSAllowedOrigins))
	if cfg.Metrics.Enabled {
		httpMetrics := middleware.NewHTTPMetrics(cfg.Metrics.Path)
		httpMetrics.RegisterGaugeFunc(func() []middleware.Gauge {
			return resourceGauges(resources)
		})
		engine.Use(httpMetrics.Middleware())
		engine.GET(cfg.Metrics.Path, httpMetrics.Handler())
	}
	if cfg.HTTP.RateLimitEnabled {
		engine.Use(middleware.RateLimit(cfg.HTTP.RateLimitPerMinute, cfg.HTTP.RateLimitBurst, tokenManager))
	}

	api := &API{
		cfg:       cfg,
		resources: resources,
		engine:    engine,
		tokens:    tokenManager,
		telemetry: telemetryShutdown,
	}
	api.registerRoutes()

	api.server = &http.Server{
		Addr:         cfg.HTTP.Addr(),
		Handler:      engine,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	return api, nil
}

func resourceGauges(resources *Resources) []middleware.Gauge {
	gauges := []middleware.Gauge{
		{
			Name:   "webtui_dependency_up",
			Help:   "Trạng thái dependency kỹ thuật của API.",
			Labels: map[string]string{"dependency": "postgres"},
			Value:  boolToGauge(resources != nil && resources.Database != nil),
		},
		{
			Name:   "webtui_dependency_up",
			Help:   "Trạng thái dependency kỹ thuật của API.",
			Labels: map[string]string{"dependency": "redis"},
			Value:  boolToGauge(resources != nil && resources.Redis != nil),
		},
		{
			Name:   "webtui_dependency_up",
			Help:   "Trạng thái dependency kỹ thuật của API.",
			Labels: map[string]string{"dependency": "rabbitmq"},
			Value:  boolToGauge(resources != nil && resources.RabbitMQ != nil),
		},
	}

	if resources != nil && resources.WebSocket != nil {
		stats := resources.WebSocket.Stats()
		gauges = append(gauges,
			middleware.Gauge{
				Name:  "webtui_websocket_clients",
				Help:  "Số client WebSocket đang kết nối trên node hiện tại.",
				Value: float64(stats["clients"]),
			},
			middleware.Gauge{
				Name:  "webtui_websocket_rooms",
				Help:  "Số room WebSocket đang có thành viên trên node hiện tại.",
				Value: float64(stats["rooms"]),
			},
		)
	}

	return append(gauges, operationalGauges(resources)...)
}

func operationalGauges(resources *Resources) []middleware.Gauge {
	if resources == nil || resources.Database == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var pending, processing, failed, dead, sent24h, skipped24h, dead24h int64
	var oldestQueueAge float64
	err := resources.Database.Pool().QueryRow(ctx, `
SELECT
    count(*) FILTER (WHERE status = 'pending'),
    count(*) FILTER (WHERE status = 'processing'),
    count(*) FILTER (WHERE status = 'failed'),
    count(*) FILTER (WHERE status = 'dead'),
    count(*) FILTER (WHERE status = 'sent' AND sent_at >= now() - interval '24 hours'),
    count(*) FILTER (WHERE status = 'skipped' AND updated_at >= now() - interval '24 hours'),
    count(*) FILTER (WHERE status = 'dead' AND updated_at >= now() - interval '24 hours'),
    COALESCE(extract(epoch FROM (now() - min(created_at) FILTER (
        WHERE status IN ('pending', 'processing', 'failed')
    ))), 0)
FROM notification_jobs
WHERE channel = 'push'
`).Scan(&pending, &processing, &failed, &dead, &sent24h, &skipped24h, &dead24h, &oldestQueueAge)
	if err != nil {
		return []middleware.Gauge{{
			Name:  "webtui_operational_metrics_scrape_success",
			Help:  "Whether database-backed operational metrics were collected successfully.",
			Value: 0,
		}}
	}

	gauges := []middleware.Gauge{
		{Name: "webtui_operational_metrics_scrape_success", Help: "Whether database-backed operational metrics were collected successfully.", Value: 1},
		{Name: "webtui_push_jobs", Help: "Current push notification jobs by status.", Labels: map[string]string{"status": "pending"}, Value: float64(pending)},
		{Name: "webtui_push_jobs", Help: "Current push notification jobs by status.", Labels: map[string]string{"status": "processing"}, Value: float64(processing)},
		{Name: "webtui_push_jobs", Help: "Current push notification jobs by status.", Labels: map[string]string{"status": "failed"}, Value: float64(failed)},
		{Name: "webtui_push_jobs", Help: "Current push notification jobs by status.", Labels: map[string]string{"status": "dead"}, Value: float64(dead)},
		{Name: "webtui_push_sent_jobs_24h", Help: "Push jobs delivered during the last 24 hours.", Value: float64(sent24h)},
		{Name: "webtui_push_skipped_jobs_24h", Help: "Push jobs skipped because no eligible destination existed during the last 24 hours.", Value: float64(skipped24h)},
		{Name: "webtui_push_dead_jobs_24h", Help: "Push jobs moved to dead-letter during the last 24 hours.", Value: float64(dead24h)},
		{Name: "webtui_push_oldest_queued_age_seconds", Help: "Age of the oldest pending, processing, or failed push job.", Value: oldestQueueAge},
	}
	completed := sent24h + dead24h
	if completed > 0 {
		gauges = append(gauges, middleware.Gauge{
			Name:  "webtui_push_delivery_rate_ratio_24h",
			Help:  "Ratio of sent push jobs among terminal push jobs during the last 24 hours.",
			Value: float64(sent24h) / float64(completed),
		})
	}

	var backupFailures24h int64
	var lastBackupSuccess float64
	if err := resources.Database.Pool().QueryRow(ctx, `
SELECT
    count(*) FILTER (WHERE status = 'failed' AND finished_at >= now() - interval '24 hours'),
    COALESCE(extract(epoch FROM max(finished_at) FILTER (WHERE status = 'success')), 0)
FROM backup_runs
WHERE backup_job_id IS NULL
`).Scan(&backupFailures24h, &lastBackupSuccess); err == nil {
		gauges = append(gauges,
			middleware.Gauge{Name: "webtui_backup_failed_runs_24h", Help: "Backup runs that failed during the last 24 hours.", Value: float64(backupFailures24h)},
			middleware.Gauge{Name: "webtui_backup_last_success_timestamp_seconds", Help: "Unix timestamp of the last successful backup run.", Value: lastBackupSuccess},
		)
	} else {
		// The success family covers the complete operational snapshot. Returning
		// a partial push-only snapshot as successful would hide a broken backup
		// metrics query after a migration or database incident.
		gauges[0].Value = 0
	}

	var pendingReports, reviewingReports, urgentOpenReports int64
	var urgentTriageOverdue, normalTriageOverdue, closureOverdue int64
	var oldestOpenReportAge float64
	if err := resources.Database.Pool().QueryRow(ctx, `
SELECT
    count(*) FILTER (WHERE status = 'pending'),
    count(*) FILTER (WHERE status = 'reviewing'),
    count(*) FILTER (
        WHERE status IN ('pending', 'reviewing')
          AND reason IN ('violence', 'illegal_content', 'sexual_content')
    ),
    count(*) FILTER (
        WHERE status = 'pending'
          AND reason IN ('violence', 'illegal_content', 'sexual_content')
          AND created_at < now() - interval '4 hours'
    ),
    count(*) FILTER (
        WHERE status = 'pending'
          AND created_at < now() - interval '24 hours'
    ),
    count(*) FILTER (
        WHERE status IN ('pending', 'reviewing')
          AND created_at < now() - interval '72 hours'
    ),
    COALESCE(extract(epoch FROM (now() - min(created_at) FILTER (
        WHERE status IN ('pending', 'reviewing')
    ))), 0)
FROM moderation_reports
`).Scan(
		&pendingReports,
		&reviewingReports,
		&urgentOpenReports,
		&urgentTriageOverdue,
		&normalTriageOverdue,
		&closureOverdue,
		&oldestOpenReportAge,
	); err == nil {
		gauges = append(gauges,
			middleware.Gauge{Name: "webtui_moderation_reports", Help: "Current moderation reports by open status.", Labels: map[string]string{"status": "pending"}, Value: float64(pendingReports)},
			middleware.Gauge{Name: "webtui_moderation_reports", Help: "Current moderation reports by open status.", Labels: map[string]string{"status": "reviewing"}, Value: float64(reviewingReports)},
			middleware.Gauge{Name: "webtui_moderation_urgent_open_reports", Help: "Open reports in urgent safety reason categories.", Value: float64(urgentOpenReports)},
			middleware.Gauge{Name: "webtui_moderation_urgent_triage_overdue", Help: "Urgent pending reports older than the four-hour triage SLO.", Value: float64(urgentTriageOverdue)},
			middleware.Gauge{Name: "webtui_moderation_normal_triage_overdue", Help: "Pending reports older than the 24-hour triage SLO.", Value: float64(normalTriageOverdue)},
			middleware.Gauge{Name: "webtui_moderation_closure_overdue", Help: "Open reports older than the 72-hour closure SLO.", Value: float64(closureOverdue)},
			middleware.Gauge{Name: "webtui_moderation_oldest_open_age_seconds", Help: "Age of the oldest pending or reviewing moderation report.", Value: oldestOpenReportAge},
		)
	} else {
		gauges[0].Value = 0
	}
	return gauges
}

func boolToGauge(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func (a *API) Run(ctx context.Context) error {
	defer a.resources.Close()
	defer func() {
		if a.telemetry == nil {
			return
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.telemetry(shutdownCtx); err != nil {
			slog.Warn("OpenTelemetry shutdown failed", "error", err)
		}
	}()

	errCh := make(chan error, 1)

	go func() {
		slog.Info("API đang lắng nghe", "addr", a.server.Addr, "env", a.cfg.App.Env)
		errCh <- a.server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.HTTP.ShutdownTimeout)
		defer cancel()

		slog.Info("API đang tắt")
		return a.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (a *API) Engine() *gin.Engine {
	return a.engine
}

func configureGin(cfg *config.Config) {
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
		return
	}
	gin.SetMode(gin.DebugMode)
}
