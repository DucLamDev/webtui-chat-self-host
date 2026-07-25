package bootstrap

import (
	"context"
	nethttp "net/http"
	"time"

	adminapp "github.com/duclamdev/application-chat/backend/internal/modules/admin/application"
	adminhttp "github.com/duclamdev/application-chat/backend/internal/modules/admin/delivery/http"
	adminpostgres "github.com/duclamdev/application-chat/backend/internal/modules/admin/infrastructure/postgres"
	aptokensapp "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/application"
	aptokenshttp "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/delivery/http"
	aptokenspostgres "github.com/duclamdev/application-chat/backend/internal/modules/api_tokens/infrastructure/postgres"
	auditapp "github.com/duclamdev/application-chat/backend/internal/modules/audit/application"
	audithttp "github.com/duclamdev/application-chat/backend/internal/modules/audit/delivery/http"
	auditpostgres "github.com/duclamdev/application-chat/backend/internal/modules/audit/infrastructure/postgres"
	authapp "github.com/duclamdev/application-chat/backend/internal/modules/auth/application"
	authhttp "github.com/duclamdev/application-chat/backend/internal/modules/auth/delivery/http"
	authoidc "github.com/duclamdev/application-chat/backend/internal/modules/auth/infrastructure/oidc"
	authpostgres "github.com/duclamdev/application-chat/backend/internal/modules/auth/infrastructure/postgres"
	backupsapp "github.com/duclamdev/application-chat/backend/internal/modules/backups/application"
	backupshttp "github.com/duclamdev/application-chat/backend/internal/modules/backups/delivery/http"
	backupspostgres "github.com/duclamdev/application-chat/backend/internal/modules/backups/infrastructure/postgres"
	botsapp "github.com/duclamdev/application-chat/backend/internal/modules/bots/application"
	botshttp "github.com/duclamdev/application-chat/backend/internal/modules/bots/delivery/http"
	botspostgres "github.com/duclamdev/application-chat/backend/internal/modules/bots/infrastructure/postgres"
	callsapp "github.com/duclamdev/application-chat/backend/internal/modules/calls/application"
	callshttp "github.com/duclamdev/application-chat/backend/internal/modules/calls/delivery/http"
	callspostgres "github.com/duclamdev/application-chat/backend/internal/modules/calls/infrastructure/postgres"
	callsws "github.com/duclamdev/application-chat/backend/internal/modules/calls/infrastructure/websocket"
	channelsapp "github.com/duclamdev/application-chat/backend/internal/modules/channels/application"
	channelshttp "github.com/duclamdev/application-chat/backend/internal/modules/channels/delivery/http"
	channelspostgres "github.com/duclamdev/application-chat/backend/internal/modules/channels/infrastructure/postgres"
	contactsapp "github.com/duclamdev/application-chat/backend/internal/modules/contacts/application"
	contactshttp "github.com/duclamdev/application-chat/backend/internal/modules/contacts/delivery/http"
	contactspostgres "github.com/duclamdev/application-chat/backend/internal/modules/contacts/infrastructure/postgres"
	contactsws "github.com/duclamdev/application-chat/backend/internal/modules/contacts/infrastructure/websocket"
	cronjobsapp "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/application"
	cronjobshttp "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/delivery/http"
	cronjobspostgres "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/infrastructure/postgres"
	departmentsapp "github.com/duclamdev/application-chat/backend/internal/modules/departments/application"
	departmentshttp "github.com/duclamdev/application-chat/backend/internal/modules/departments/delivery/http"
	departmentspostgres "github.com/duclamdev/application-chat/backend/internal/modules/departments/infrastructure/postgres"
	filesapp "github.com/duclamdev/application-chat/backend/internal/modules/files/application"
	fileshttp "github.com/duclamdev/application-chat/backend/internal/modules/files/delivery/http"
	filespostgres "github.com/duclamdev/application-chat/backend/internal/modules/files/infrastructure/postgres"
	filesstorage "github.com/duclamdev/application-chat/backend/internal/modules/files/infrastructure/storage"
	filesws "github.com/duclamdev/application-chat/backend/internal/modules/files/infrastructure/websocket"
	healthhttp "github.com/duclamdev/application-chat/backend/internal/modules/health/delivery/http"
	messagesapp "github.com/duclamdev/application-chat/backend/internal/modules/messages/application"
	messageshttp "github.com/duclamdev/application-chat/backend/internal/modules/messages/delivery/http"
	messagespostgres "github.com/duclamdev/application-chat/backend/internal/modules/messages/infrastructure/postgres"
	messagesws "github.com/duclamdev/application-chat/backend/internal/modules/messages/infrastructure/websocket"
	notificationsapp "github.com/duclamdev/application-chat/backend/internal/modules/notifications/application"
	notificationshttp "github.com/duclamdev/application-chat/backend/internal/modules/notifications/delivery/http"
	notificationspostgres "github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/postgres"
	orderapp "github.com/duclamdev/application-chat/backend/internal/modules/order/application"
	orderhttp "github.com/duclamdev/application-chat/backend/internal/modules/order/delivery/http"
	orderclient "github.com/duclamdev/application-chat/backend/internal/modules/order/infrastructure/httpclient"
	orderpostgres "github.com/duclamdev/application-chat/backend/internal/modules/order/infrastructure/postgres"
	presenceapp "github.com/duclamdev/application-chat/backend/internal/modules/presence/application"
	presencehttp "github.com/duclamdev/application-chat/backend/internal/modules/presence/delivery/http"
	presencepostgres "github.com/duclamdev/application-chat/backend/internal/modules/presence/infrastructure/postgres"
	pushdevicesapp "github.com/duclamdev/application-chat/backend/internal/modules/push_devices/application"
	pushdeviceshttp "github.com/duclamdev/application-chat/backend/internal/modules/push_devices/delivery/http"
	pushdevicespostgres "github.com/duclamdev/application-chat/backend/internal/modules/push_devices/infrastructure/postgres"
	rbacapp "github.com/duclamdev/application-chat/backend/internal/modules/rbac/application"
	rbachttp "github.com/duclamdev/application-chat/backend/internal/modules/rbac/delivery/http"
	rbacpostgres "github.com/duclamdev/application-chat/backend/internal/modules/rbac/infrastructure/postgres"
	syncapp "github.com/duclamdev/application-chat/backend/internal/modules/sync/application"
	synchttp "github.com/duclamdev/application-chat/backend/internal/modules/sync/delivery/http"
	syncpostgres "github.com/duclamdev/application-chat/backend/internal/modules/sync/infrastructure/postgres"
	tenancyapp "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/application"
	tenancyhttp "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/delivery/http"
	tenancypostgres "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/infrastructure/postgres"
	ticketsapp "github.com/duclamdev/application-chat/backend/internal/modules/tickets/application"
	ticketshttp "github.com/duclamdev/application-chat/backend/internal/modules/tickets/delivery/http"
	ticketspostgres "github.com/duclamdev/application-chat/backend/internal/modules/tickets/infrastructure/postgres"
	usersapp "github.com/duclamdev/application-chat/backend/internal/modules/users/application"
	usershttp "github.com/duclamdev/application-chat/backend/internal/modules/users/delivery/http"
	userspostgres "github.com/duclamdev/application-chat/backend/internal/modules/users/infrastructure/postgres"
	webhooksapp "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/application"
	webhookshttp "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/delivery/http"
	webhooksender "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/infrastructure/httpclient"
	webhookspostgres "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/infrastructure/postgres"
	workspacesapp "github.com/duclamdev/application-chat/backend/internal/modules/workspaces/application"
	workspaceshttp "github.com/duclamdev/application-chat/backend/internal/modules/workspaces/delivery/http"
	workspacespostgres "github.com/duclamdev/application-chat/backend/internal/modules/workspaces/infrastructure/postgres"
	wshttp "github.com/duclamdev/application-chat/backend/internal/platform/websocket/delivery/http"
	"github.com/duclamdev/application-chat/backend/internal/shared/middleware"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func (a *API) registerRoutes() {
	healthHandler := healthhttp.NewHandler(healthhttp.BuildInfo{
		Name:                      a.cfg.App.Name,
		Env:                       a.cfg.App.Env,
		Version:                   a.cfg.App.Version,
		DesktopMinimumVersion:     a.cfg.Client.DesktopMinimumVersion,
		DesktopRecommendedVersion: a.cfg.Client.DesktopRecommendedVersion,
		DesktopReleaseManifestDir: a.cfg.Client.DesktopReleaseManifestDir,
		DesktopUpdateURL:          a.cfg.Client.DesktopUpdateURL,
		MobileMinimumVersion:      a.cfg.Client.MobileMinimumVersion,
		MobileRecommendedVersion:  a.cfg.Client.MobileRecommendedVersion,
		MobileReleaseManifestDir:  a.cfg.Client.MobileReleaseManifestDir,
		MobileDownloadURL:         a.cfg.Client.MobileDownloadURL,
		MobileStoreURL:            a.cfg.Client.MobileStoreURL,
		DownloadManifestDir:       a.cfg.Client.DownloadManifestDir,
		StartedAt:                 a.cfg.App.StartedAt,
		Now:                       time.Now,
		Checks:                    a.healthChecks(),
	})

	healthHandler.Register(a.engine)

	a.registerAPIV1()
}

func (a *API) registerAPIV1() {
	apiInfo := func(c *gin.Context) {
		response.OK(c, nethttp.StatusOK, gin.H{
			"name":            a.cfg.App.Name,
			"version":         a.cfg.App.Version,
			"status":          "ok",
			"api_version":     "v1",
			"deployment_mode": a.cfg.Deployment.Mode,
		})
	}
	a.engine.GET("/api", apiInfo)
	v1 := a.engine.Group("/api/v1")
	v1.GET("", apiInfo)

	if a.resources.Database == nil {
		v1.Any("/*path", func(c *gin.Context) {
			response.Fail(c, nethttp.StatusServiceUnavailable, "DATABASE_DISABLED", "Database đang tắt nên API nghiệp vụ chưa sẵn sàng.", nil)
		})
		return
	}

	tokenManager := a.tokens
	pool := a.resources.Database.Pool()

	tenancyRepo := tenancypostgres.NewRepository(pool, a.cfg.Registration.DefaultWorkspaceID)
	tenancyService := tenancyapp.NewService(tenancyRepo, tenancyapp.Options{
		AppName:              a.cfg.App.Name,
		AppVersion:           a.cfg.App.Version,
		DefaultLocale:        "vi-VN",
		ReleaseChannel:       a.cfg.App.Env,
		DeploymentMode:       a.cfg.Deployment.Mode,
		RTCICEServers:        a.cfg.Calls.ICEServers,
		WebhookSigningSecret: a.cfg.Security.WebhookSigningSecret,
		OIDCEnabled:          len(a.cfg.Security.OIDCStateSecret) >= 32,
		OIDCClientSecrets:    a.cfg.Security.OIDCClientSecrets,
		RoutingDNSType:       a.cfg.Registration.CustomDomainDNSType,
		RoutingDNSTarget:     a.cfg.Registration.CustomDomainDNSTarget,
	})
	v1.Use(tenancyhttp.OptionalZoneContext(tenancyService))
	authMiddleware := middleware.Auth(tokenManager, tenancyService)
	zoneRecoveryAuthMiddleware := middleware.AuthForZoneRecovery(tokenManager, tenancyService)
	tenancyHandler := tenancyhttp.NewHandler(tenancyService, a.cfg.Security.CaddyAskSecret)
	tenancyHandler.SetSaaSProvisioningEnabled(!a.cfg.Deployment.IsSelfHosted())
	tenancyHandler.RegisterRoutes(a.engine, v1, authMiddleware, zoneRecoveryAuthMiddleware)

	authRepo := authpostgres.NewRepository(pool, a.cfg.Registration.DefaultWorkspaceID)
	authService := authapp.NewService(authRepo, tokenManager)
	authHandler := authhttp.NewHandler(authService, a.cfg.Security.GoogleClientID)
	if a.cfg.Deployment.IsSelfHosted() {
		authHandler.SetInstanceDomain(a.cfg.Deployment.InstanceDomain)
	}
	authHandler.SetOIDCService(authapp.NewOIDCService(
		authService,
		authRepo,
		authoidc.NewClient(),
		a.cfg.Security.OIDCStateSecret,
		a.cfg.Security.OIDCClientSecrets,
	))
	authHandler.RegisterRoutes(v1.Group("/auth"), authMiddleware)

	rbacRepo := rbacpostgres.NewRepository(pool)
	rbacService := rbacapp.NewService(rbacRepo)
	rbacHandler := rbachttp.NewHandler(rbacService)
	rbacHandler.RegisterRoutes(v1.Group("/rbac"), authMiddleware)

	usersRepo := userspostgres.NewRepository(pool)
	usersService := usersapp.NewService(usersRepo, rbacService)
	usersHandler := usershttp.NewHandler(usersService)
	usersHandler.RegisterRoutes(v1.Group("/users"), authMiddleware)

	pushDevicesRepo := pushdevicespostgres.NewRepository(pool)
	pushDevicesService := pushdevicesapp.NewService(pushDevicesRepo, rbacService)
	pushDevicesHandler := pushdeviceshttp.NewHandler(pushDevicesService)
	pushDevicesHandler.RegisterRoutes(v1, authMiddleware)

	contactsRepo := contactspostgres.NewRepository(pool)
	var contactsRealtime contactsapp.RealtimePublisher
	if a.resources.WebSocket != nil {
		contactsRealtime = contactsws.NewPublisher(a.resources.WebSocket)
	}
	contactsService := contactsapp.NewService(contactsRepo, contactsRealtime)
	contactsHandler := contactshttp.NewHandler(contactsService)
	contactsHandler.RegisterRoutes(v1, authMiddleware)

	adminRepo := adminpostgres.NewRepository(pool)
	adminService := adminapp.NewService(adminRepo, rbacService)
	adminHandler := adminhttp.NewHandler(adminService, a.healthChecks())
	adminHandler.RegisterRoutes(v1, authMiddleware)

	auditRepo := auditpostgres.NewRepository(pool)
	auditService := auditapp.NewService(auditRepo, rbacService)
	auditHandler := audithttp.NewHandler(auditService)
	auditHandler.RegisterRoutes(v1, authMiddleware)

	cronjobsRepo := cronjobspostgres.NewRepository(pool)
	cronjobsService := cronjobsapp.NewService(cronjobsRepo, rbacService, cronjobsapp.WithScriptAllowlist(a.cfg.ModuleRunner.ScriptAllowlist))
	cronjobsHandler := cronjobshttp.NewHandler(cronjobsService, a.cfg.App.ServiceName)
	cronjobsHandler.RegisterRoutes(v1, authMiddleware)

	backupsRepo := backupspostgres.NewRepository(pool)
	backupsService := backupsapp.NewService(backupsRepo, a.resources.Storage, rbacService, backupsapp.Options{
		DatabaseURL:     a.cfg.Database.URL,
		PGDumpPath:      a.cfg.Backup.PGDumpPath,
		Timeout:         a.cfg.Backup.Timeout,
		StorageProvider: a.cfg.Storage.Provider,
	})
	backupsHandler := backupshttp.NewHandler(backupsService)
	backupsHandler.RegisterRoutes(v1, authMiddleware)

	apiTokensRepo := aptokenspostgres.NewRepository(pool)
	apiTokensService := aptokensapp.NewService(apiTokensRepo, rbacService)
	apiTokensHandler := aptokenshttp.NewHandler(apiTokensService)
	apiTokensHandler.RegisterRoutes(v1, authMiddleware)

	workspacesRepo := workspacespostgres.NewRepository(pool)
	workspacesService := workspacesapp.NewService(workspacesRepo, rbacService)
	workspacesHandler := workspaceshttp.NewHandler(workspacesService)
	workspacesHandler.RegisterRoutes(v1.Group("/workspaces"), authMiddleware)

	departmentsRepo := departmentspostgres.NewRepository(pool)
	departmentsService := departmentsapp.NewService(departmentsRepo, rbacService)
	departmentsHandler := departmentshttp.NewHandler(departmentsService)
	departmentsHandler.RegisterRoutes(v1, authMiddleware)

	channelsRepo := channelspostgres.NewRepository(pool)
	channelsService := channelsapp.NewService(channelsRepo, rbacService)
	channelsHandler := channelshttp.NewHandler(channelsService)
	channelsHandler.RegisterRoutes(v1, authMiddleware)

	if a.resources.WebSocket != nil {
		wsHandler := wshttp.NewHandler(a.resources.WebSocket, tokenManager, channelsService)
		wsHandler.SetWorkspaceZoneChecker(tenancyService)
		wsHandler.RegisterRoutes(v1)
		// Keep the discovery URL usable without requiring an ingress rewrite.
		wsHandler.RegisterPublicRoute(a.engine)
	}

	notificationsRepo := notificationspostgres.NewRepository(pool)
	notificationsService := notificationsapp.NewService(notificationsRepo)
	notificationsHandler := notificationshttp.NewHandler(notificationsService)
	notificationsHandler.RegisterRoutes(v1, authMiddleware)

	presenceRepo := presencepostgres.NewRepository(pool)
	presenceService := presenceapp.NewService(presenceRepo, rbacService)
	presenceHandler := presencehttp.NewHandler(presenceService)
	presenceHandler.RegisterRoutes(v1, authMiddleware)

	syncRepo := syncpostgres.NewRepository(pool)
	syncService := syncapp.NewService(syncRepo, rbacService)
	syncHandler := synchttp.NewHandler(syncService)
	syncHandler.RegisterRoutes(v1, authMiddleware)

	botsRepo := botspostgres.NewRepository(pool)
	botsService := botsapp.NewService(botsRepo, rbacService)
	botsHandler := botshttp.NewHandler(botsService)
	botsHandler.RegisterRoutes(v1, authMiddleware)

	callsRepo := callspostgres.NewRepository(pool)
	var callsRealtime callsapp.RealtimePublisher
	if a.resources.WebSocket != nil {
		callsRealtime = callsws.NewPublisher(a.resources.WebSocket)
	}
	callsService := callsapp.NewService(callsRepo, rbacService, callsRealtime, notificationsService)
	callsService.SetRingTimeout(a.cfg.Calls.RingTimeout)
	callsHandler := callshttp.NewHandler(callsService)
	callsHandler.RegisterRoutes(v1, authMiddleware)

	orderRepo := orderpostgres.NewRepository(pool)
	orderAPIClient := orderclient.New(orderclient.Config{
		BaseURL:        a.cfg.Order.BaseURL,
		InternalAPIKey: a.cfg.Order.InternalAPIKey,
		QuickOrderKey:  a.cfg.Order.QuickOrderKey,
		Timeout:        a.cfg.Order.Timeout,
	})
	orderService := orderapp.NewService(orderAPIClient, orderRepo, rbacService)
	orderHandler := orderhttp.NewHandler(orderService)
	orderHandler.RegisterRoutes(v1, authMiddleware)

	ticketsRepo := ticketspostgres.NewRepository(pool)
	ticketsService := ticketsapp.NewService(ticketsRepo, rbacService)
	ticketsHandler := ticketshttp.NewHandler(ticketsService)
	ticketsHandler.RegisterRoutes(v1, authMiddleware)

	webhooksRepo := webhookspostgres.NewRepository(pool)
	webhooksService := webhooksapp.NewService(
		webhooksRepo,
		rbacService,
		apiTokensService,
		webhooksender.NewSender(a.cfg.Security.WebhookSigningSecret),
		a.cfg.Security.WebhookSigningSecret,
	)
	webhooksHandler := webhookshttp.NewHandler(webhooksService, a.cfg.App.URL)
	webhooksHandler.RegisterRoutes(v1, authMiddleware)

	if a.resources.Storage != nil {
		filesRepo := filespostgres.NewRepository(pool)
		filesStore := filesstorage.NewStore(a.resources.Storage)
		filesService := filesapp.NewService(filesRepo, filesStore, rbacService, a.cfg.Storage.Provider, a.cfg.Storage.Bucket)
		if a.resources.WebSocket != nil {
			filesService.SetRealtimePublisher(filesws.NewPublisher(a.resources.WebSocket))
		}
		filesHandler := fileshttp.NewHandler(filesService)
		filesHandler.RegisterRoutes(v1, authMiddleware)
	}

	messagesRepo := messagespostgres.NewRepository(pool)
	var realtimePublisher messagesapp.RealtimePublisher
	if a.resources.WebSocket != nil {
		realtimePublisher = messagesws.NewPublisher(a.resources.WebSocket)
	}
	messagesService := messagesapp.NewService(messagesRepo, rbacService, realtimePublisher)
	messagesService.SetAutoResponders(orderService)
	messagesHandler := messageshttp.NewHandler(messagesService)
	messagesHandler.RegisterRoutes(v1, authMiddleware)
}

func (a *API) healthChecks() map[string]healthhttp.CheckFunc {
	checks := map[string]healthhttp.CheckFunc{}

	if a.resources.Database != nil {
		checks["database"] = func(ctx context.Context) error {
			return a.resources.Database.Ping(ctx)
		}
	}
	if a.resources.Redis != nil {
		checks["redis"] = func(ctx context.Context) error {
			return a.resources.Redis.Ping(ctx)
		}
	}
	if a.resources.RabbitMQ != nil {
		checks["rabbitmq"] = func(ctx context.Context) error {
			return a.resources.RabbitMQ.Ping(ctx)
		}
	}
	if a.resources.Storage != nil {
		checks["storage"] = func(ctx context.Context) error {
			return a.resources.Storage.Health(ctx)
		}
	}
	if a.resources.WebSocket != nil {
		checks["websocket"] = func(ctx context.Context) error {
			return a.resources.WebSocket.Health(ctx)
		}
	}

	return checks
}
