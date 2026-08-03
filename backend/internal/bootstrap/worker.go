package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/duclamdev/application-chat/backend/internal/config"
	aptokensapp "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/application"
	aptokenspostgres "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/infrastructure/postgres"
	backupsapp "github.com/duclamdev/application-chat/backend/internal/modules/backups/application"
	backupspostgres "github.com/duclamdev/application-chat/backend/internal/modules/backups/infrastructure/postgres"
	callsapp "github.com/duclamdev/application-chat/backend/internal/modules/calls/application"
	callspostgres "github.com/duclamdev/application-chat/backend/internal/modules/calls/infrastructure/postgres"
	channelspostgres "github.com/duclamdev/application-chat/backend/internal/modules/channels/infrastructure/postgres"
	cronjobsapp "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/application"
	cronjobspostgres "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/infrastructure/postgres"
	messagesapp "github.com/duclamdev/application-chat/backend/internal/modules/messages/application"
	messagespostgres "github.com/duclamdev/application-chat/backend/internal/modules/messages/infrastructure/postgres"
	notificationsapp "github.com/duclamdev/application-chat/backend/internal/modules/notifications/application"
	notificationsapns "github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/apns"
	notificationsfcm "github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/fcm"
	notificationspostgres "github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/postgres"
	notificationsrelay "github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/relay"
	notificationswebpush "github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/webpush"
	outboxapp "github.com/duclamdev/application-chat/backend/internal/modules/outbox/application"
	outboxpostgres "github.com/duclamdev/application-chat/backend/internal/modules/outbox/infrastructure/postgres"
	outboxrabbitmq "github.com/duclamdev/application-chat/backend/internal/modules/outbox/infrastructure/rabbitmq"
	presenceapp "github.com/duclamdev/application-chat/backend/internal/modules/presence/application"
	presencepostgres "github.com/duclamdev/application-chat/backend/internal/modules/presence/infrastructure/postgres"
	webhooksapp "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/application"
	webhooksender "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/infrastructure/httpclient"
	webhookspostgres "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/infrastructure/postgres"
	"github.com/duclamdev/application-chat/backend/internal/platform/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type Worker struct {
	cfg       *config.Config
	resources *Resources
	telemetry observability.Shutdown
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
	telemetryShutdown, err := observability.Setup(context.Background(), observability.Config{
		Enabled: cfg.Telemetry.Enabled, Endpoint: cfg.Telemetry.OTLPEndpoint,
		ServiceName: cfg.App.ServiceName, Version: cfg.App.Version,
		Environment: cfg.App.Env, SampleRatio: cfg.Telemetry.SampleRatio,
	})
	if err != nil {
		resources.Close()
		return nil, fmt.Errorf("initialize OpenTelemetry: %w", err)
	}

	return &Worker{
		cfg:       cfg,
		resources: resources,
		telemetry: telemetryShutdown,
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	defer w.resources.Close()
	defer func() {
		if w.telemetry == nil {
			return
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.telemetry(shutdownCtx); err != nil {
			slog.Warn("OpenTelemetry shutdown failed", "error", err)
		}
	}()

	slog.Info("Worker đã khởi động", "concurrency", w.cfg.Worker.Concurrency)

	tasks, err := w.tasks()
	if err != nil {
		return err
	}
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

func (w *Worker) tasks() ([]workerTask, error) {
	if w.resources.Database == nil {
		return nil, nil
	}

	pool := w.resources.Database.Pool()
	pushSenders := make([]notificationspostgres.PushSender, 0, 2)
	if w.cfg.PushRelay.URL != "" {
		pushSenders = []notificationspostgres.PushSender{
			notificationsrelay.NewSender(notificationsrelay.Config{
				URL: w.cfg.PushRelay.URL, Token: w.cfg.PushRelay.Token,
				InstanceID: w.cfg.PushRelay.InstanceID, Provider: "fcm",
			}),
			notificationsrelay.NewSender(notificationsrelay.Config{
				URL: w.cfg.PushRelay.URL, Token: w.cfg.PushRelay.Token,
				InstanceID: w.cfg.PushRelay.InstanceID, Provider: "apns",
			}),
		}
	} else {
		if anyConfigured(
			w.cfg.Firebase.ProjectID,
			w.cfg.Firebase.ServiceAccountFile,
			w.cfg.Firebase.ServiceAccountJSONBase64,
		) {
			pushSender := notificationsfcm.NewSender(notificationsfcm.Config{
				ProjectID:                w.cfg.Firebase.ProjectID,
				ServiceAccountFile:       w.cfg.Firebase.ServiceAccountFile,
				ServiceAccountJSONBase64: w.cfg.Firebase.ServiceAccountJSONBase64,
			})
			if err := pushSender.InitializationError(); err != nil || !pushSender.Enabled() {
				if err == nil {
					err = fmt.Errorf("sender is disabled")
				}
				return nil, fmt.Errorf("initialize FCM push sender: %w", err)
			}
			pushSenders = append(pushSenders, pushSender)
		}
		if anyConfigured(
			w.cfg.APNS.KeyID,
			w.cfg.APNS.TeamID,
			w.cfg.APNS.PrivateKeyFile,
			w.cfg.APNS.PrivateKeyBase64,
		) {
			voipPushSender := notificationsapns.NewSender(notificationsapns.Config{
				KeyID:            w.cfg.APNS.KeyID,
				TeamID:           w.cfg.APNS.TeamID,
				BundleID:         w.cfg.APNS.BundleID,
				PrivateKeyFile:   w.cfg.APNS.PrivateKeyFile,
				PrivateKeyBase64: w.cfg.APNS.PrivateKeyBase64,
				Sandbox:          w.cfg.APNS.Sandbox,
			})
			if err := voipPushSender.InitializationError(); err != nil || !voipPushSender.Enabled() {
				if err == nil {
					err = fmt.Errorf("sender is disabled")
				}
				return nil, fmt.Errorf("initialize APNs push sender: %w", err)
			}
			pushSenders = append(pushSenders, voipPushSender)
		}
	}
	notificationsRepo := notificationspostgres.NewRepository(pool, pushSenders...)
	if w.cfg.WebPush.Enabled {
		webPushSender := notificationswebpush.NewSender(notificationswebpush.Config{
			Enabled: true, PublicKey: w.cfg.WebPush.VAPIDPublicKey,
			PrivateKey: w.cfg.WebPush.VAPIDPrivateKey, Subject: w.cfg.WebPush.VAPIDSubject,
			TTL: w.cfg.WebPush.TTL,
		})
		if !webPushSender.Enabled() {
			return nil, fmt.Errorf("initialize Web Push sender: VAPID private key and subject are required")
		}
		notificationsRepo.SetWebPushSender(webPushSender)
	}
	notificationsService := notificationsapp.NewService(notificationsRepo)
	messagesRepo := messagespostgres.NewRepository(pool)
	messagesService := messagesapp.NewService(messagesRepo, nil)
	callsRepo := callspostgres.NewRepository(pool)
	callsService := callsapp.NewService(callsRepo, nil, nil, notificationsService)
	callsService.SetRingTimeout(w.cfg.Calls.RingTimeout)
	channelsRepo := channelspostgres.NewRepository(pool)
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
			name:     "talk_maintenance",
			interval: 60 * time.Second,
			run: func(ctx context.Context) error {
				result, err := channelsRepo.MaintainTalk(
					ctx,
					w.resources.Storage,
					limit,
				)
				total := result.ExpiredPins +
					result.EndedMeetings +
					result.ArchivedMeetings +
					result.DeletedMeetings +
					result.ClosedBreakouts +
					result.StoppedVoiceRooms +
					result.FailedRecordings +
					result.DeletedRecordings +
					result.ExpiredUploads
				if total > 0 {
					slog.Debug(
						"Đã bảo trì Talk",
						"expired_pins", result.ExpiredPins,
						"ended_meetings", result.EndedMeetings,
						"archived_meetings", result.ArchivedMeetings,
						"deleted_meetings", result.DeletedMeetings,
						"closed_breakouts", result.ClosedBreakouts,
						"stopped_voice_rooms", result.StoppedVoiceRooms,
						"failed_recordings", result.FailedRecordings,
						"deleted_recordings", result.DeletedRecordings,
						"expired_uploads", result.ExpiredUploads,
					)
				}
				return err
			},
		},
		{
			name:     "scheduled_messages",
			interval: time.Second,
			run: func(ctx context.Context) error {
				count, err := messagesService.ProcessDueScheduledMessages(ctx, limit)
				if count > 0 {
					slog.Debug("Đã gửi tin nhắn được hẹn giờ", "count", count)
				}
				return err
			},
		},
		{
			name:     "message_reminders",
			interval: time.Second,
			run: func(ctx context.Context) error {
				count, err := messagesService.ProcessDueReminders(ctx, limit)
				if count > 0 {
					slog.Debug("Đã phát lời nhắc tin nhắn", "count", count)
				}
				return err
			},
		},
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
			name:     "call_signal_cleanup",
			interval: 10 * time.Minute,
			run: func(ctx context.Context) error {
				command, err := pool.Exec(ctx, `
WITH expired AS (
    SELECT signal.id
    FROM call_signals signal
    JOIN call_sessions call ON call.id = signal.call_id
    WHERE signal.created_at < now() - interval '24 hours'
       OR call.status IN ('rejected', 'cancelled', 'ended', 'missed')
    ORDER BY signal.created_at
    LIMIT $1
    FOR UPDATE OF signal SKIP LOCKED
)
DELETE FROM call_signals signal
USING expired
WHERE signal.id = expired.id
`, limit)
				if command.RowsAffected() > 0 {
					slog.Debug("Purged expired WebRTC signaling records", "count", command.RowsAffected())
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
	}, nil
}

func anyConfigured(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
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
	ctx, span := otel.Tracer("webtui-chat/worker").Start(ctx, "worker."+task.name)
	span.SetAttributes(attribute.String("worker.task.name", task.name))
	defer span.End()
	startedAt := time.Now()
	if err := task.run(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "worker task failed")
		slog.Warn("Tác vụ worker chạy thất bại", "task", task.name, "duration_ms", time.Since(startedAt).Milliseconds(), "error", err)
		return
	}
	slog.Debug("Tác vụ worker chạy xong", "task", task.name, "duration_ms", time.Since(startedAt).Milliseconds())
}
