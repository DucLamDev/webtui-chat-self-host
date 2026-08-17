import type {
  ApiScope,
  ApiToken,
  BackupJob,
  BackupRun,
  Bot,
  BotAIConfig,
  BotFlow,
  BotFlowRun,
  BotInstallation,
  BotMessage,
  CreateBackupJobInput,
  CreateApiTokenInput,
  CreateBotInput,
  CreateIncomingWebhookInput,
  CreateOutgoingWebhookInput,
  CreatedApiToken,
  CreatedIncomingWebhook,
  CreatedOutgoingWebhook,
  CronJob,
  CronJobRun,
  IncomingWebhook,
  InstallBotInput,
  Notification,
  ChannelNotificationPreference,
  ChannelNotificationPreferenceInput,
  NotificationPreference,
  NotificationPreferenceInput,
  OrderBotStatus,
  OrderPaymentQRInput,
  OrderPaymentQRResult,
  OrderRenewServiceInput,
  OrderRenewServiceResult,
  OrderServicesExpiringInput,
  OrderServicesExpiringResult,
  OrderWalletBalanceInput,
  OrderWalletBalanceResult,
  OrderWalletDepositQRInput,
  OrderWalletDepositQRResult,
  OutgoingWebhook,
  Presence,
  PresenceHeartbeatInput,
  SaveCronJobInput,
  SaveBotAIConfigInput,
  SaveBotFlowInput,
  SendBotMessageInput,
  TestOutgoingWebhookInput,
  UpdateIncomingWebhookInput,
  UpdateOutgoingWebhookInput,
  WebhookDelivery
} from "@webtui/types";
import type { HttpClient, QueryParams } from "./http-client";
import { collectionFrom } from "./response-utils";

export type ModuleRecord = Record<string, unknown> & {
  id?: string;
  name?: string;
};

export type WebPushConfig = {
  enabled: boolean;
  vapid_public_key?: string;
};

export type WebPushSubscriptionInput = {
  endpoint: string;
  expiration_time?: string;
  keys: {
    auth: string;
    p256dh: string;
  };
  workspace_id: string;
};

export type WebPushSubscriptionRecord = {
  created_at: string;
  id: string;
  updated_at: string;
};

export function createNotificationsClient(http: HttpClient) {
  return {
    async listMine(params: QueryParams = {}) {
      const data = await http.get<unknown>("/api/v1/notifications", { query: params });
      return collectionFrom<Notification>(data, "notifications");
    },
    markRead(notificationId: string) {
      return http.put<Notification>(`/api/v1/notifications/${encodeURIComponent(notificationId)}/read`);
    },
    markAllRead(params: QueryParams = {}) {
      return http.put<void>("/api/v1/notifications/read-all", undefined, { query: params });
    },
    getPreferences(workspaceId: string) {
      return http.get<NotificationPreference>("/api/v1/notifications/preferences", {
        query: { workspace_id: workspaceId }
      });
    },
    updatePreferences(input: NotificationPreferenceInput) {
      return http.put<NotificationPreference>("/api/v1/notifications/preferences", input);
    },
    getChannelPreference(workspaceId: string, channelId: string) {
      return http.get<ChannelNotificationPreference>(
        `/api/v1/notifications/preferences/channels/${encodeURIComponent(channelId)}`,
        { query: { workspace_id: workspaceId } }
      );
    },
    updateChannelPreference(channelId: string, input: ChannelNotificationPreferenceInput) {
      return http.put<ChannelNotificationPreference>(
        `/api/v1/notifications/preferences/channels/${encodeURIComponent(channelId)}`,
        input
      );
    },
    getWebPushConfig() {
      return http.get<WebPushConfig>("/api/v1/notifications/web-push/config", { auth: false });
    },
    registerWebPushSubscription(input: WebPushSubscriptionInput) {
      return http.post<WebPushSubscriptionRecord>("/api/v1/notifications/web-push/subscriptions", input);
    },
    revokeWebPushSubscription(subscriptionId: string) {
      return http.delete<void>(
        `/api/v1/notifications/web-push/subscriptions/${encodeURIComponent(subscriptionId)}`
      );
    }
  };
}

export function createPresenceClient(http: HttpClient) {
  return {
    async list(workspaceId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/presence`, {
        query: params
      });
      return collectionFrom<Presence>(data, "presence");
    },
    heartbeat(workspaceId: string, input: PresenceHeartbeatInput) {
      return http.put<Presence>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/presence/heartbeat`, input);
    }
  };
}

export function createApiTokensClient(http: HttpClient) {
  return {
    async scopes() {
      const data = await http.get<unknown>("/api/v1/api-scopes");
      return collectionFrom<ApiScope>(data, "scopes");
    },
    async list(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/api-tokens`);
      return collectionFrom<ApiToken>(data, "api_tokens");
    },
    create(workspaceId: string, input: CreateApiTokenInput) {
      return http.post<CreatedApiToken>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/api-tokens`, input);
    },
    revoke(workspaceId: string, tokenId: string) {
      return http.delete<void>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/api-tokens/${encodeURIComponent(tokenId)}`
      );
    }
  };
}

export function createWebhooksClient(http: HttpClient) {
  return {
    async incoming(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/incoming-webhooks`);
      return collectionFrom<IncomingWebhook>(data, "incoming_webhooks");
    },
    createIncoming(workspaceId: string, input: CreateIncomingWebhookInput) {
      return http.post<CreatedIncomingWebhook>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/incoming-webhooks`,
        input
      );
    },
    updateIncoming(workspaceId: string, webhookId: string, input: UpdateIncomingWebhookInput) {
      return http.patch<IncomingWebhook>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/incoming-webhooks/${encodeURIComponent(webhookId)}`,
        input
      );
    },
    deleteIncoming(workspaceId: string, webhookId: string) {
      return http.delete<void>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/incoming-webhooks/${encodeURIComponent(webhookId)}`
      );
    },
    async outgoing(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/outgoing-webhooks`);
      return collectionFrom<OutgoingWebhook>(data, "outgoing_webhooks");
    },
    createOutgoing(workspaceId: string, input: CreateOutgoingWebhookInput) {
      return http.post<CreatedOutgoingWebhook>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/outgoing-webhooks`,
        input
      );
    },
    updateOutgoing(workspaceId: string, webhookId: string, input: UpdateOutgoingWebhookInput) {
      return http.patch<OutgoingWebhook>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/outgoing-webhooks/${encodeURIComponent(webhookId)}`,
        input
      );
    },
    deleteOutgoing(workspaceId: string, webhookId: string) {
      return http.delete<void>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/outgoing-webhooks/${encodeURIComponent(webhookId)}`
      );
    },
    async deliveries(workspaceId: string, webhookId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/outgoing-webhooks/${encodeURIComponent(webhookId)}/deliveries`,
        { query: params }
      );
      return collectionFrom<WebhookDelivery>(data, "deliveries");
    },
    testOutgoing(workspaceId: string, webhookId: string, input: TestOutgoingWebhookInput) {
      return http.post<WebhookDelivery>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/outgoing-webhooks/${encodeURIComponent(webhookId)}/test`,
        input
      );
    }
  };
}

export function createBotsClient(http: HttpClient) {
  return {
    async list(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/bots`);
      return collectionFrom<Bot>(data, "bots");
    },
    create(workspaceId: string, input: CreateBotInput) {
      return http.post<Bot>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/bots`, input);
    },
    async installations(workspaceId: string, botId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/bots/${encodeURIComponent(botId)}/installations`
      );
      return collectionFrom<BotInstallation>(data, "installations");
    },
    install(workspaceId: string, botId: string, input: InstallBotInput) {
      return http.post<BotInstallation>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/bots/${encodeURIComponent(botId)}/installations`,
        input
      );
    },
    sendMessage(workspaceId: string, botId: string, input: SendBotMessageInput) {
      return http.post<BotMessage>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/bots/${encodeURIComponent(botId)}/messages`,
        input
      );
    },
    aiConfig(workspaceId: string, botId: string) {
      return http.get<BotAIConfig>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/bots/${encodeURIComponent(botId)}/ai-config`
      );
    },
    saveAIConfig(workspaceId: string, botId: string, input: SaveBotAIConfigInput) {
      return http.put<BotAIConfig>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/bots/${encodeURIComponent(botId)}/ai-config`,
        input
      );
    },
    async flows(workspaceId: string, botId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/bots/${encodeURIComponent(botId)}/flows`
      );
      return collectionFrom<BotFlow>(data, "flows");
    },
    createFlow(workspaceId: string, botId: string, input: SaveBotFlowInput) {
      return http.post<BotFlow>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/bots/${encodeURIComponent(botId)}/flows`,
        input
      );
    },
    updateFlow(workspaceId: string, botId: string, flowId: string, input: SaveBotFlowInput) {
      return http.patch<BotFlow>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/bots/${encodeURIComponent(botId)}/flows/${encodeURIComponent(flowId)}`,
        input
      );
    },
    publishFlow(workspaceId: string, botId: string, flowId: string) {
      return http.post<BotFlow>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/bots/${encodeURIComponent(botId)}/flows/${encodeURIComponent(flowId)}/publish`
      );
    },
    testFlow(workspaceId: string, botId: string, flowId: string, input: unknown) {
      return http.post<BotFlowRun>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/bots/${encodeURIComponent(botId)}/flows/${encodeURIComponent(flowId)}/test`,
        { input }
      );
    }
  };
}

export function createOrderBotClient(http: HttpClient) {
  return {
    status(workspaceId: string) {
      return http.get<OrderBotStatus>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/order-bot/status`);
    },
    walletBalance(workspaceId: string, input: OrderWalletBalanceInput) {
      return http.post<OrderWalletBalanceResult>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/order-bot/wallet/balance`,
        input
      );
    },
    depositQr(workspaceId: string, input: OrderWalletDepositQRInput) {
      return http.post<OrderWalletDepositQRResult>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/order-bot/wallet/deposit-qr`,
        input
      );
    },
    orderPaymentQr(workspaceId: string, input: OrderPaymentQRInput) {
      return http.post<OrderPaymentQRResult>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/order-bot/payment/order-qr`,
        input
      );
    },
    expiringServices(workspaceId: string, input: OrderServicesExpiringInput) {
      return http.post<OrderServicesExpiringResult>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/order-bot/services/expiring`,
        input
      );
    },
    renewService(workspaceId: string, input: OrderRenewServiceInput) {
      return http.post<OrderRenewServiceResult>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/order-bot/services/renew`,
        input
      );
    }
  };
}

export function createCronjobsClient(http: HttpClient) {
  return {
    async list(workspaceId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/cronjobs`, {
        query: params
      });
      return collectionFrom<CronJob>(data, "cronjobs");
    },
    create(workspaceId: string, input: SaveCronJobInput) {
      return http.post<CronJob>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/cronjobs`, input);
    },
    update(workspaceId: string, cronjobId: string, input: SaveCronJobInput) {
      return http.patch<CronJob>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/cronjobs/${encodeURIComponent(cronjobId)}`,
        input
      );
    },
    delete(workspaceId: string, cronjobId: string) {
      return http.delete<void>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/cronjobs/${encodeURIComponent(cronjobId)}`);
    },
    async runs(workspaceId: string, cronjobId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/cronjobs/${encodeURIComponent(cronjobId)}/runs`,
        { query: params }
      );
      return collectionFrom<CronJobRun>(data, "runs");
    },
    runNow(workspaceId: string, cronjobId: string) {
      return http.post<CronJobRun>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/cronjobs/${encodeURIComponent(cronjobId)}/run`
      );
    }
  };
}

export function createBackupsClient(http: HttpClient) {
  return {
    async jobs(workspaceId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/backup-jobs`, {
        query: params
      });
      return collectionFrom<BackupJob>(data, "backup_jobs");
    },
    createJob(workspaceId: string, input: CreateBackupJobInput) {
      return http.post<BackupJob>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/backup-jobs`, input);
    },
    async runs(workspaceId: string, jobId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/backup-jobs/${encodeURIComponent(jobId)}/runs`,
        { query: params }
      );
      return collectionFrom<BackupRun>(data, "backup_runs");
    },
    runNow(workspaceId: string, jobId: string) {
      return http.post<BackupRun>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/backup-jobs/${encodeURIComponent(jobId)}/run`
      );
    }
  };
}
