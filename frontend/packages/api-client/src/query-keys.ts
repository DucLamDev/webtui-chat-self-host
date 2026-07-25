export const queryKeys = {
  admin: {
    auditLogs: (workspaceId: string, filter = "") => ["admin", "audit-logs", workspaceId, filter] as const,
    health: (workspaceId: string) => ["admin", "health", workspaceId] as const,
    stats: (workspaceId: string) => ["admin", "stats", workspaceId] as const
  },
  auth: {
    me: ["auth", "me"] as const,
    sessions: ["auth", "sessions"] as const
  },
  channels: {
    all: (workspaceId: string) => ["channels", workspaceId] as const,
    detail: (workspaceId: string, channelId: string) => ["channels", workspaceId, channelId] as const,
    directConversations: (workspaceId: string) => ["direct-conversations", workspaceId] as const,
    joinRequests: (workspaceId: string, channelId: string) => ["channels", workspaceId, channelId, "join-requests"] as const,
    members: (workspaceId: string, channelId: string) => ["channels", workspaceId, channelId, "members"] as const
  },
  contacts: {
    all: ["contacts"] as const,
    requests: (status = "pending") => ["contact-requests", status] as const
  },
  departments: {
    all: (workspaceId: string) => ["departments", workspaceId] as const,
    detail: (workspaceId: string, departmentId: string) => ["departments", workspaceId, departmentId] as const,
    members: (workspaceId: string, departmentId: string) => ["departments", workspaceId, departmentId, "members"] as const
  },
  files: {
    all: (workspaceId: string) => ["files", workspaceId] as const,
    channelMedia: (workspaceId: string, channelId: string) =>
      ["files", workspaceId, channelId, "media"] as const,
    attachments: (workspaceId: string, channelId: string, messageId: string) =>
      ["files", workspaceId, channelId, messageId, "attachments"] as const,
    detail: (workspaceId: string, fileId: string) => ["files", workspaceId, fileId] as const,
    messageAttachments: (workspaceId: string, channelId: string) =>
      ["files", workspaceId, channelId, "message-attachments"] as const
  },
  messages: {
    channel: (workspaceId: string, channelId: string) => ["messages", workspaceId, channelId] as const,
    pins: (workspaceId: string, channelId: string) => ["messages", workspaceId, channelId, "pins"] as const,
    search: (workspaceId: string, query: string, filters = "") => ["messages", workspaceId, "search", query, filters] as const,
    thread: (workspaceId: string, channelId: string, messageId: string) =>
      ["messages", workspaceId, channelId, messageId, "thread"] as const
  },
  notifications: {
    list: (workspaceId?: string) => ["notifications", workspaceId ?? "all"] as const,
    preferences: (workspaceId: string) => ["notifications", workspaceId, "preferences"] as const
  },
  orderBot: {
    status: (workspaceId: string) => ["order-bot", workspaceId, "status"] as const
  },
  presence: {
    list: (workspaceId: string) => ["presence", workspaceId] as const
  },
  integrations: {
    apiScopes: ["integrations", "api-scopes"] as const,
    apiTokens: (workspaceId: string) => ["integrations", "api-tokens", workspaceId] as const,
    botInstallations: (workspaceId: string, botId: string) =>
      ["integrations", "bots", workspaceId, botId, "installations"] as const,
    bots: (workspaceId: string) => ["integrations", "bots", workspaceId] as const,
    incomingWebhooks: (workspaceId: string) => ["integrations", "incoming-webhooks", workspaceId] as const,
    outgoingWebhooks: (workspaceId: string) => ["integrations", "outgoing-webhooks", workspaceId] as const,
    webhookDeliveries: (workspaceId: string, webhookId: string) =>
      ["integrations", "outgoing-webhooks", workspaceId, webhookId, "deliveries"] as const
  },
  operations: {
    backupJobs: (workspaceId: string) => ["operations", "backup-jobs", workspaceId] as const,
    backupRuns: (workspaceId: string, backupJobId: string) =>
      ["operations", "backup-jobs", workspaceId, backupJobId, "runs"] as const,
    cronJobRuns: (workspaceId: string, cronJobId: string) =>
      ["operations", "cronjobs", workspaceId, cronJobId, "runs"] as const,
    cronjobs: (workspaceId: string, status = "") => ["operations", "cronjobs", workspaceId, status] as const
  },
  rbac: {
    memberRoles: (workspaceId: string, userId: string) => ["rbac", "member-roles", workspaceId, userId] as const,
    me: (workspaceId?: string) => ["rbac", "me", workspaceId] as const,
    permissions: ["rbac", "permissions"] as const,
    roles: (workspaceId?: string) => ["rbac", "roles", workspaceId ?? "global"] as const
  },
  tickets: {
    all: (workspaceId: string, status = "") => ["tickets", workspaceId, status] as const,
    detail: (workspaceId: string, ticketId: string) => ["tickets", workspaceId, ticketId] as const
  },
  tenancy: {
    automationInstallations: ["tenancy", "automation-installations"] as const,
    automationTemplates: ["tenancy", "automation-templates"] as const,
    capabilities: (domain: string) => ["tenancy", "capabilities", domain] as const,
    claim: (domainId: string) => ["tenancy", "claim", domainId] as const,
    currentZone: ["tenancy", "current-zone"] as const,
    deploymentRequests: ["tenancy", "deployment-requests"] as const,
    discovery: (domain: string) => ["tenancy", "discovery", domain] as const,
    oidcProviders: ["tenancy", "oidc-providers"] as const,
    quota: ["tenancy", "quota"] as const
  },
  users: {
    all: (query?: string, status?: string) => ["users", query ?? "", status ?? ""] as const,
    detail: (userId: string) => ["users", userId] as const,
    me: ["users", "me"] as const
  },
  workspaces: {
    all: ["workspaces"] as const,
    detail: (workspaceId: string) => ["workspaces", workspaceId] as const,
    members: (workspaceId: string) => ["workspaces", workspaceId, "members"] as const,
    settings: (workspaceId: string) => ["workspaces", workspaceId, "settings"] as const
  }
};
