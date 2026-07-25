package bootstrap

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/duclamdev/application-chat/backend/internal/config"
	aptokensapp "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/application"
	aptokenspostgres "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/infrastructure/postgres"
	backupsapp "github.com/duclamdev/application-chat/backend/internal/modules/backups/application"
	backupspostgres "github.com/duclamdev/application-chat/backend/internal/modules/backups/infrastructure/postgres"
	callsapp "github.com/duclamdev/application-chat/backend/internal/modules/calls/application"
	callspostgres "github.com/duclamdev/application-chat/backend/internal/modules/calls/infrastructure/postgres"
	cronjobsapp "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/application"
	cronjobspostgres "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/infrastructure/postgres"
	notificationsapp "github.com/duclamdev/application-chat/backend/internal/modules/notifications/application"
	notificationsfcm "github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/fcm"
	notificationspostgres "github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/postgres"
	outboxapp "github.com/duclamdev/application-chat/backend/internal/modules/outbox/application"
	outboxpostgres "github.com/duclamdev/application-chat/backend/internal/modules/outbox/infrastructure/postgres"
	outboxrabbitmq "github.com/duclamdev/application-chat/backend/internal/modules/outbox/infrastructure/rabbitmq"
	presenceapp "github.com/duclamdev/application-chat/backend/internal/modules/presence/application"
	presencepostgres "github.com/duclamdev/application-chat/backend/internal/modules/presence/infrastructure/postgres"
	webhooksapp "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/application"
	webhooksender "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/infrastructure/httpclient"
	webhookspostgres "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/infrastructure/postgres"
)

type Worker struct {
	cfg       *config.Config
	resources *Resources
}

type workerTask struct {
	name     string
	interval time.Duration
	run      func(context.Context) error
}

func NewWorker(cfg *config.Config) (*Worker, error) {
	cfg.App.ServiceName = "worker"
	configureLogger(cfg)

	resources, err := NewResources(context.Background(), cfg)
	if err != nil {
		return nil, err
	}

	return &Worker{
		cfg:       cfg,
		resources: resources,
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	defer w.resources.Close()

	slog.Info("Worker đã khởi động", "concurrency", w.cfg.Worker.Concurrency)

	tasks := w.tasks()
	if len(tasks) == 0 {
		slog.Warn("Worker không có tác vụ nền vì database đang tắt")
		<-ctx.Done()
		slog.Info("Worker đang tắt")
		return nil
	}

	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for _, task := range tasks {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.runTaskLoop(taskCtx, task)
		}()
	}

	<-ctx.Done()
	slog.Info("Worker đang tắt")
	cancel()
	wg.Wait()
	return nil
}

func (w *Worker) tasks() []workerTask {
	if w.resources.Database == nil {
		return nil
	}

	pool := w.resources.Database.Pool()
	pushSender := notificationsfcm.NewSender(notificationsfcm.Config{
		ProjectID:                w.cfg.Firebase.ProjectID,
		ServiceAccountFile:       w.cfg.Firebase.ServiceAccountFile,
		ServiceAccountJSONBase64: w.cfg.Firebase.ServiceAccountJSONBase64,
	})
	notificationsRepo := notificationspostgres.NewRepository(pool, pushSender)
	notificationsService := notificationsapp.NewService(notificationsRepo)
	callsRepo := callspostgres.NewRepository(pool)
	callsService := callsapp.NewService(callsRepo, nil, nil, notificationsService)
	callsService.SetRingTimeout(w.cfg.Calls.RingTimeout)
	apiTokensRepo := aptokenspostgres.NewRepository(pool)
	apiTokensService := aptokensapp.NewService(apiTokensRepo, nil)
	webhooksRepo := webhookspostgres.NewRepository(pool)
	webhooksService := webhooksapp.NewService(
		webhooksRepo,
		nil,
		apiTokensService,
		webhooksender.NewSender(w.cfg.Security.WebhookSigningSecret),
		w.cfg.Security.WebhookSigningSecret,
	)
	outboxRepo := outboxpostgres.NewRepository(pool)
	outboxPublisher := outboxrabbitmq.NewPublisher(w.resources.RabbitMQ)
	outboxService := outboxapp.NewService(outboxRepo, outboxPublisher, notificationsService, webhooksService)
	presenceRepo := presencepostgres.NewRepository(pool)
	presenceService := presenceapp.NewService(presenceRepo, nil)
	cronjobsRepo := cronjobspostgres.NewRepository(pool)
	cronjobsService := cronjobsapp.NewService(cronjobsRepo, nil, cronjobsapp.WithScriptAllowlist(w.cfg.ModuleRunner.ScriptAllowlist))
	backupsRepo := backupspostgres.NewRepository(pool)
	backupsService := backupsapp.NewService(backupsRepo, w.resources.Storage, nil, backupsapp.Options{
		DatabaseURL:     w.cfg.Database.URL,
		PGDumpPath:      w.cfg.Backup.PGDumpPath,
		Timeout:         w.cfg.Backup.Timeout,
		StorageProvider: w.cfg.Storage.Provider,
	})
	limit := w.cfg.Worker.Concurrency * 10
	nodeID := w.cfg.App.ServiceName
	if nodeID == "" {
		nodeID = "worker"
	}

	return []workerTask{
		{
			name:     "outbox",
			interval: 3 * time.Second,
			run: func(ctx context.Context) error {
				count, err := outboxService.Process(ctx, limit)
				if count > 0 {
					slog.Debug("Đã xử lý outbox event", "count", count)
				}
				return err
			},
		},
		{
			name:     "call_timeouts",
			interval: time.Second,
			run: func(ctx context.Context) error {
				count, err := callsService.ExpireUnanswered(ctx, limit)
				if count > 0 {
					slog.Debug("Đã kết thúc cuộc gọi không được trả lời", "count", count)
				}
				return err
			},
		},
		{
			name:     "notification_jobs",
			interval: time.Second,
			run: func(ctx context.Context) error {
				count, err := notificationsService.ProcessJobs(ctx, limit)
				if count > 0 {
					slog.Debug("Đã xử lý notification job", "count", count)
				}
				return err
			},
		},
		{
			name:     "webhook_deliveries",
			interval: 5 * time.Second,
			run: func(ctx context.Context) error {
				count, err := webhooksService.ProcessDeliveries(ctx, limit)
				if count > 0 {
					slog.Debug("Đã xử lý webhook delivery", "count", count)
				}
				return err
			},
		},
		{
			name:     "presence_cleanup",
			interval: 30 * time.Second,
			run: func(ctx context.Context) error {
				count, err := presenceService.CleanupStale(ctx, 90*time.Second)
				if count > 0 {
					slog.Debug("Đã chuyển presence cũ sang offline", "count", count)
				}
				return err
			},
		},
		{
			name:     "cronjobs",
			interval: 15 * time.Second,
			run: func(ctx context.Context) error {
				count, err := cronjobsService.ProcessDue(ctx, limit, nodeID)
				if count > 0 {
					slog.Debug("Đã xử lý cronjob đến hạn", "count", count)
				}
				return err
			},
		},
		{
			name:     "backup_jobs",
			interval: 60 * time.Second,
			run: func(ctx context.Context) error {
				count, err := backupsService.ProcessDue(ctx, limit, nodeID)
				if count > 0 {
					slog.Debug("Đã xử lý backup job đến hạn", "count", count)
				}
				return err
			},
		},
	}
}

func (w *Worker) runTaskLoop(ctx context.Context, task workerTask) {
	w.runTask(ctx, task)

	ticker := time.NewTicker(task.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runTask(ctx, task)
		}
	}
}

func (w *Worker) runTask(ctx context.Context, task workerTask) {
	startedAt := time.Now()
	if err := task.run(ctx); err != nil {
		slog.Warn("Tác vụ worker chạy thất bại", "task", task.name, "duration_ms", time.Since(startedAt).Milliseconds(), "error", err)
		return
	}
	slog.Debug("Tác vụ worker chạy xong", "task", task.name, "duration_ms", time.Since(startedAt).Milliseconds())
}
