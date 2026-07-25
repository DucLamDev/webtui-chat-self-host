"use client";

import { useCallback, useEffect, useMemo } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiClientError, queryKeys } from "@webtui/api-client";
import { createPermissionSet, hasPermission } from "@webtui/types";
import type {
  AddWorkspaceMemberInput,
  AdminChannelOverview,
  AdminMessageOverview,
  AuthUser,
  CreateBackupJobInput,
  CreateApiTokenInput,
  CreateAutomationInstallationInput,
  CreateBotInput,
  CreateIncomingWebhookInput,
  CreateOutgoingWebhookInput,
  CreateRoleInput,
  CreateZoneOIDCProviderInput,
  InstallBotInput,
  PermissionCode,
  SaveCronJobInput,
  SendBotMessageInput,
  UpdateAutomationInstallationInput,
  UpdateMemberStatusInput,
  UpdateZoneOIDCProviderInput,
  UpdateZoneQuotaInput,
  UpsertWorkspaceSettingInput
} from "@webtui/types";
import { api } from "@/lib/api";

export type AdminPermissionValue = PermissionCode | string;

export type AdminDashboardDataOptions = {
  selectedBackupJobId?: string;
  selectedBotId?: string;
  selectedCronJobId?: string;
  selectedMemberId?: string;
  selectedOutgoingWebhookId?: string;
};

type CreateRoleMutationInput = Omit<CreateRoleInput, "workspace_id">;

export function useAdminDashboardData(options: AdminDashboardDataOptions = {}) {
  const pathname = usePathname();
  const queryClient = useQueryClient();
  const router = useRouter();
  const searchParams = useSearchParams();
  const requestedWorkspaceId = searchParams.get("workspace") ?? "";

  const workspacesQuery = useQuery({
    queryFn: () => api.workspaces.listMine(),
    queryKey: queryKeys.workspaces.all
  });

  const workspaces = workspacesQuery.data ?? [];
  const resolvedWorkspaceId = requestedWorkspaceId || workspaces[0]?.id || "";

  const workspaceQuery = useQuery({
    enabled: Boolean(resolvedWorkspaceId),
    queryFn: () => api.workspaces.get(resolvedWorkspaceId),
    queryKey: queryKeys.workspaces.detail(resolvedWorkspaceId),
    retry: false
  });

  const selectedWorkspace =
    workspaceQuery.data ?? workspaces.find((workspace) => workspace.id === resolvedWorkspaceId) ?? null;
  const workspaceId = selectedWorkspace?.id ?? resolvedWorkspaceId;

  const permissionsQuery = useQuery({
    enabled: Boolean(workspaceId),
    queryFn: () => api.rbac.myPermissions(workspaceId),
    queryKey: queryKeys.rbac.me(workspaceId),
    retry: false
  });

  const permissionCodes = useMemo(
    () => createPermissionSet(permissionsQuery.data ?? []),
    [permissionsQuery.data]
  );

  const can = useCallback(
    (permission: AdminPermissionValue) => hasPermission(permissionCodes, permission),
    [permissionCodes]
  );

  const canViewAdmin = can("admin.view");
  const canViewAudit = can("audit.view");
  const canManageRoles = can("role.manage");
  const canManageUsers = can("user.manage");
  const canManageApiTokens = can("api_token.manage");
  const canManageBots = can("bot.manage");
  const canManageBackups = can("backup.manage");
  const canManageCronjobs = can("cronjob.manage");
  const canManageWebhooks = can("webhook.manage");
  const canManageWorkspace = can("workspace.manage");
  const adminQueryEnabled = Boolean(workspaceId && canViewAdmin);
  const integrationQueryEnabled = Boolean(workspaceId && (canManageApiTokens || canManageBots || canManageWebhooks));
  const operationsQueryEnabled = Boolean(workspaceId && (canManageCronjobs || canManageBackups));

  const statsQuery = useQuery({
    enabled: adminQueryEnabled,
    queryFn: () => api.admin.stats(workspaceId),
    queryKey: queryKeys.admin.stats(workspaceId),
    retry: false
  });

  const healthQuery = useQuery({
    enabled: adminQueryEnabled,
    queryFn: () => api.admin.health(workspaceId),
    queryKey: queryKeys.admin.health(workspaceId),
    retry: false
  });

  const adminChannelsQuery = useQuery({
    enabled: adminQueryEnabled,
    queryFn: () => loadAdminChannels(workspaceId),
    queryKey: ["admin", workspaceId, "channels"],
    retry: false
  });

  const adminMessagesQuery = useQuery({
    enabled: adminQueryEnabled,
    queryFn: () => loadAdminMessages(workspaceId),
    queryKey: ["admin", workspaceId, "messages"],
    retry: false
  });

  const usersQuery = useQuery({
    enabled: adminQueryEnabled,
    queryFn: () => api.users.list({ limit: 100 }),
    queryKey: queryKeys.users.all()
  });

  const membersQuery = useQuery({
    enabled: adminQueryEnabled,
    queryFn: () => api.workspaces.members(workspaceId),
    queryKey: queryKeys.workspaces.members(workspaceId),
    retry: false
  });

  const settingsQuery = useQuery({
    enabled: Boolean(workspaceId),
    queryFn: () => api.workspaces.settings(workspaceId),
    queryKey: queryKeys.workspaces.settings(workspaceId)
  });

  const permissionsCatalogQuery = useQuery({
    enabled: adminQueryEnabled,
    queryFn: () => api.rbac.permissions(),
    queryKey: queryKeys.rbac.permissions,
    retry: false
  });

  const rolesQuery = useQuery({
    enabled: adminQueryEnabled,
    queryFn: () => api.rbac.roles({ workspace_id: workspaceId }),
    queryKey: queryKeys.rbac.roles(workspaceId),
    retry: false
  });

  const selectedMemberRolesQuery = useQuery({
    enabled: Boolean(workspaceId && options.selectedMemberId && adminQueryEnabled),
    queryFn: () => api.rbac.memberRoles(workspaceId, options.selectedMemberId ?? ""),
    queryKey: queryKeys.rbac.memberRoles(workspaceId, options.selectedMemberId ?? ""),
    retry: false
  });

  const auditLogsQuery = useQuery({
    enabled: Boolean(workspaceId && canViewAudit),
    queryFn: () => api.admin.auditLogs(workspaceId, { limit: 50 }),
    queryKey: queryKeys.admin.auditLogs(workspaceId),
    retry: false
  });

  const channelsQuery = useQuery({
    enabled: Boolean(workspaceId && (adminQueryEnabled || integrationQueryEnabled)),
    queryFn: () => api.channels.list(workspaceId),
    queryKey: queryKeys.channels.all(workspaceId),
    retry: false
  });

  async function loadAdminChannels(currentWorkspaceId: string): Promise<AdminChannelOverview[]> {
    try {
      return await api.admin.channels(currentWorkspaceId);
    } catch (error) {
      if (!isMissingAdminOverviewEndpoint(error)) {
        throw error;
      }
      const channels = await api.channels.list(currentWorkspaceId);
      return channels.map((channel) => ({
        id: channel.id,
        member_count: channel.member_count ?? 0,
        message_count: 0,
        name: channel.name,
        private_session_mode: channel.private_session_mode ?? false,
        slug: channel.slug,
        status: "active",
        type: channel.type ?? channel.kind ?? "public",
        updated_at: channel.updated_at ?? channel.created_at ?? ""
      }));
    }
  }

  async function loadAdminMessages(currentWorkspaceId: string): Promise<AdminMessageOverview[]> {
    try {
      return await api.admin.messages(currentWorkspaceId, { limit: 100 });
    } catch (error) {
      if (!isMissingAdminOverviewEndpoint(error)) {
        throw error;
      }

      const channels = await api.channels.list(currentWorkspaceId);
      const results = await Promise.allSettled(
        channels.map(async (channel) => ({
          channel,
          messages: await api.messages.list(currentWorkspaceId, channel.id, { limit: 25 })
        }))
      );

      return results
        .flatMap((result) => result.status === "fulfilled"
          ? result.value.messages.map<AdminMessageOverview>((message) => ({
              body: message.body,
              channel_id: result.value.channel.id,
              channel_name: result.value.channel.name,
              created_at: message.created_at ?? message.sent_at ?? message.updated_at ?? "",
              id: message.id,
              kind: message.kind ?? "text",
              sender_name: message.author?.display_name
                || message.author?.username
                || message.author?.email
                || message.user?.display_name
                || message.user?.username
                || message.user?.email
                || "Hệ thống"
            }))
          : [])
        .sort((left, right) => Date.parse(right.created_at) - Date.parse(left.created_at))
        .slice(0, 100);
    }
  }

  const apiScopesQuery = useQuery({
    enabled: canManageApiTokens,
    queryFn: () => api.apiTokens.scopes(),
    queryKey: queryKeys.integrations.apiScopes,
    retry: false
  });

  const apiTokensQuery = useQuery({
    enabled: Boolean(workspaceId && canManageApiTokens),
    queryFn: () => api.apiTokens.list(workspaceId),
    queryKey: queryKeys.integrations.apiTokens(workspaceId),
    retry: false
  });

  const botsQuery = useQuery({
    enabled: Boolean(workspaceId && canManageBots),
    queryFn: () => api.bots.list(workspaceId),
    queryKey: queryKeys.integrations.bots(workspaceId),
    retry: false
  });

  const canManageAutomation = canManageWorkspace || canManageBots || canManageWebhooks;

  const automationTemplatesQuery = useQuery({
    enabled: Boolean(workspaceId && canManageAutomation),
    queryFn: () => api.tenancy.automationTemplates(),
    queryKey: queryKeys.tenancy.automationTemplates,
    retry: false
  });

  const automationInstallationsQuery = useQuery({
    enabled: Boolean(workspaceId && canManageAutomation),
    queryFn: () => api.tenancy.automationInstallations(),
    queryKey: queryKeys.tenancy.automationInstallations,
    retry: false
  });

  const currentZoneQuery = useQuery({
    enabled: Boolean(workspaceId && canManageWorkspace),
    queryFn: () => api.tenancy.currentZone(),
    queryKey: queryKeys.tenancy.currentZone,
    retry: false
  });

  const deploymentRequestsQuery = useQuery({
    enabled: Boolean(
      workspaceId &&
      canManageWorkspace &&
      currentZoneQuery.data &&
      currentZoneQuery.data?.zone.kind !== "customer_dedicated"
    ),
    queryFn: () => api.tenancy.deploymentRequests(),
    queryKey: queryKeys.tenancy.deploymentRequests,
    retry: false
  });

  const zoneQuotaQuery = useQuery({
    enabled: Boolean(workspaceId && canManageWorkspace),
    queryFn: () => api.tenancy.zoneQuota(),
    queryKey: queryKeys.tenancy.quota,
    retry: false
  });

  const oidcProvidersQuery = useQuery({
    enabled: Boolean(workspaceId && canManageWorkspace),
    queryFn: () => api.tenancy.oidcProviders(),
    queryKey: queryKeys.tenancy.oidcProviders,
    retry: false
  });

  const botInstallationsQuery = useQuery({
    enabled: Boolean(workspaceId && options.selectedBotId && canManageBots),
    queryFn: () => api.bots.installations(workspaceId, options.selectedBotId ?? ""),
    queryKey: queryKeys.integrations.botInstallations(workspaceId, options.selectedBotId ?? ""),
    retry: false
  });

  const incomingWebhooksQuery = useQuery({
    enabled: Boolean(workspaceId && canManageWebhooks),
    queryFn: () => api.webhooks.incoming(workspaceId),
    queryKey: queryKeys.integrations.incomingWebhooks(workspaceId),
    retry: false
  });

  const outgoingWebhooksQuery = useQuery({
    enabled: Boolean(workspaceId && canManageWebhooks),
    queryFn: () => api.webhooks.outgoing(workspaceId),
    queryKey: queryKeys.integrations.outgoingWebhooks(workspaceId),
    retry: false
  });

  const webhookDeliveriesQuery = useQuery({
    enabled: Boolean(workspaceId && options.selectedOutgoingWebhookId && canManageWebhooks),
    queryFn: () => api.webhooks.deliveries(workspaceId, options.selectedOutgoingWebhookId ?? ""),
    queryKey: queryKeys.integrations.webhookDeliveries(workspaceId, options.selectedOutgoingWebhookId ?? ""),
    retry: false
  });

  const cronjobsQuery = useQuery({
    enabled: Boolean(workspaceId && canManageCronjobs),
    queryFn: () => api.cronjobs.list(workspaceId, { limit: 100 }),
    queryKey: queryKeys.operations.cronjobs(workspaceId),
    retry: false
  });

  const cronjobRunsQuery = useQuery({
    enabled: Boolean(workspaceId && options.selectedCronJobId && canManageCronjobs),
    queryFn: () => api.cronjobs.runs(workspaceId, options.selectedCronJobId ?? "", { limit: 50 }),
    queryKey: queryKeys.operations.cronJobRuns(workspaceId, options.selectedCronJobId ?? ""),
    retry: false
  });

  const backupJobsQuery = useQuery({
    enabled: Boolean(workspaceId && canManageBackups),
    queryFn: () => api.backups.jobs(workspaceId, { limit: 100 }),
    queryKey: queryKeys.operations.backupJobs(workspaceId),
    retry: false
  });

  const backupRunsQuery = useQuery({
    enabled: Boolean(workspaceId && options.selectedBackupJobId && canManageBackups),
    queryFn: () => api.backups.runs(workspaceId, options.selectedBackupJobId ?? "", { limit: 50 }),
    queryKey: queryKeys.operations.backupRuns(workspaceId, options.selectedBackupJobId ?? ""),
    retry: false
  });

  const invalidateWorkspaceMembers = useCallback(() => {
    if (workspaceId) {
      void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.members(workspaceId) });
    }
  }, [queryClient, workspaceId]);

  const invalidateRoles = useCallback(() => {
    if (workspaceId) {
      void queryClient.invalidateQueries({ queryKey: queryKeys.rbac.roles(workspaceId) });
      if (options.selectedMemberId) {
        void queryClient.invalidateQueries({
          queryKey: queryKeys.rbac.memberRoles(workspaceId, options.selectedMemberId)
        });
      }
    }
  }, [options.selectedMemberId, queryClient, workspaceId]);

  const invalidateCronjobs = useCallback(() => {
    if (workspaceId) {
      void queryClient.invalidateQueries({ queryKey: queryKeys.operations.cronjobs(workspaceId) });
      if (options.selectedCronJobId) {
        void queryClient.invalidateQueries({
          queryKey: queryKeys.operations.cronJobRuns(workspaceId, options.selectedCronJobId)
        });
      }
    }
  }, [options.selectedCronJobId, queryClient, workspaceId]);

  const invalidateBackupJobs = useCallback(() => {
    if (workspaceId) {
      void queryClient.invalidateQueries({ queryKey: queryKeys.operations.backupJobs(workspaceId) });
      if (options.selectedBackupJobId) {
        void queryClient.invalidateQueries({
          queryKey: queryKeys.operations.backupRuns(workspaceId, options.selectedBackupJobId)
        });
      }
    }
  }, [options.selectedBackupJobId, queryClient, workspaceId]);

  const addMemberMutation = useMutation({
    mutationFn: (input: AddWorkspaceMemberInput) =>
      api.workspaces.addMember(requireWorkspaceId(workspaceId), input),
    onSuccess: invalidateWorkspaceMembers
  });

  const updateMemberStatusMutation = useMutation({
    mutationFn: ({ input, userId }: { input: UpdateMemberStatusInput; userId: string }) =>
      api.workspaces.updateMemberStatus(requireWorkspaceId(workspaceId), userId, input),
    onSuccess: invalidateWorkspaceMembers
  });

  const updateUserMutation = useMutation({
    mutationFn: ({ input, userId }: { input: Partial<AuthUser>; userId: string }) =>
      api.users.update(userId, {
        ...input,
        workspace_id: requireWorkspaceId(workspaceId)
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.users.all() });
    }
  });

  const upsertWorkspaceSettingMutation = useMutation({
    mutationFn: ({ input, key }: { input: UpsertWorkspaceSettingInput; key: string }) =>
      api.workspaces.upsertSetting(requireWorkspaceId(workspaceId), key, input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.settings(workspaceId) });
    }
  });

  const createRoleMutation = useMutation({
    mutationFn: (input: CreateRoleMutationInput) =>
      api.rbac.createRole({
        ...input,
        workspace_id: requireWorkspaceId(workspaceId)
      }),
    onSuccess: invalidateRoles
  });

  const assignMemberRoleMutation = useMutation({
    mutationFn: ({ roleId, userId }: { roleId: string; userId: string }) =>
      api.rbac.assignMemberRole(requireWorkspaceId(workspaceId), userId, { role_id: roleId }),
    onSuccess: invalidateRoles
  });

  const revokeMemberRoleMutation = useMutation({
    mutationFn: ({ roleId, userId }: { roleId: string; userId: string }) =>
      api.rbac.revokeMemberRole(requireWorkspaceId(workspaceId), userId, roleId),
    onSuccess: invalidateRoles
  });

  const createApiTokenMutation = useMutation({
    mutationFn: (input: CreateApiTokenInput) =>
      api.apiTokens.create(requireWorkspaceId(workspaceId), input),
    onSuccess: () => {
      if (workspaceId) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.integrations.apiTokens(workspaceId) });
      }
    }
  });

  const revokeApiTokenMutation = useMutation({
    mutationFn: (tokenId: string) => api.apiTokens.revoke(requireWorkspaceId(workspaceId), tokenId),
    onSuccess: () => {
      if (workspaceId) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.integrations.apiTokens(workspaceId) });
      }
    }
  });

  const createBotMutation = useMutation({
    mutationFn: (input: CreateBotInput) => api.bots.create(requireWorkspaceId(workspaceId), input),
    onSuccess: () => {
      if (workspaceId) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.integrations.bots(workspaceId) });
      }
    }
  });

  const createAutomationInstallationMutation = useMutation({
    mutationFn: (input: CreateAutomationInstallationInput) =>
      api.tenancy.createAutomationInstallation({
        ...input,
        workspace_id: input.workspace_id || requireWorkspaceId(workspaceId)
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.tenancy.automationInstallations
      });
    }
  });

  const updateAutomationInstallationMutation = useMutation({
    mutationFn: ({
      input,
      installationId
    }: {
      input: UpdateAutomationInstallationInput;
      installationId: string;
    }) => api.tenancy.updateAutomationInstallation(installationId, input),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.tenancy.automationInstallations
      });
    }
  });

  const deleteAutomationInstallationMutation = useMutation({
    mutationFn: (installationId: string) =>
      api.tenancy.deleteAutomationInstallation(installationId),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.tenancy.automationInstallations
      });
    }
  });

  const updateCurrentZoneMutation = useMutation({
    mutationFn: (input: {
      name?: string;
      registration_mode?: "open" | "invite_only" | "closed";
    }) => api.tenancy.updateCurrentZone(input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tenancy.currentZone });
    }
  });

  const setZoneLifecycleMutation = useMutation({
    mutationFn: (input: { action: "suspend" | "resume" | "archive"; reason?: string }) =>
      api.tenancy.setZoneLifecycle(input.action, input.reason),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tenancy.currentZone });
    }
  });

  const createAdditionalDomainMutation = useMutation({
    mutationFn: (input: { domain: string; kind?: "alias" | "api" | "web" }) =>
      api.tenancy.createAdditionalDomain(input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tenancy.currentZone });
    }
  });

  const verifyZoneDomainMutation = useMutation({
    mutationFn: (domainId: string) => api.tenancy.verifyClaim(domainId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tenancy.currentZone });
    }
  });

  const setPrimaryDomainMutation = useMutation({
    mutationFn: (domainId: string) => api.tenancy.setPrimaryDomain(domainId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tenancy.currentZone });
    }
  });

  const deleteZoneDomainMutation = useMutation({
    mutationFn: (domainId: string) => api.tenancy.deleteDomain(domainId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tenancy.currentZone });
    }
  });

  const createDeploymentRequestMutation = useMutation({
    mutationFn: (input: {
      requested_mode: string;
      requested_database_mode: string;
      idempotency_key: string;
    }) =>
      api.tenancy.createDeploymentRequest(
        {
          requested_mode: input.requested_mode,
          requested_database_mode: input.requested_database_mode
        },
        input.idempotency_key
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tenancy.deploymentRequests });
    }
  });

  const updateZoneQuotaMutation = useMutation({
    mutationFn: (input: UpdateZoneQuotaInput) => api.tenancy.updateZoneQuota(input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tenancy.quota });
    }
  });

  const createOIDCProviderMutation = useMutation({
    mutationFn: (input: CreateZoneOIDCProviderInput) =>
      api.tenancy.createOIDCProvider(input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tenancy.oidcProviders });
    }
  });

  const updateOIDCProviderMutation = useMutation({
    mutationFn: ({
      input,
      providerId
    }: {
      input: UpdateZoneOIDCProviderInput;
      providerId: string;
    }) => api.tenancy.updateOIDCProvider(providerId, input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tenancy.oidcProviders });
    }
  });

  const deleteOIDCProviderMutation = useMutation({
    mutationFn: (providerId: string) => api.tenancy.deleteOIDCProvider(providerId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tenancy.oidcProviders });
    }
  });

  const installBotMutation = useMutation({
    mutationFn: ({ botId, input }: { botId: string; input: InstallBotInput }) =>
      api.bots.install(requireWorkspaceId(workspaceId), botId, input),
    onSuccess: (_, variables) => {
      if (workspaceId) {
        void queryClient.invalidateQueries({
          queryKey: queryKeys.integrations.botInstallations(workspaceId, variables.botId)
        });
      }
    }
  });

  const sendBotMessageMutation = useMutation({
    mutationFn: ({ botId, input }: { botId: string; input: SendBotMessageInput }) =>
      api.bots.sendMessage(requireWorkspaceId(workspaceId), botId, input)
  });

  const createIncomingWebhookMutation = useMutation({
    mutationFn: (input: CreateIncomingWebhookInput) =>
      api.webhooks.createIncoming(requireWorkspaceId(workspaceId), input),
    onSuccess: () => {
      if (workspaceId) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.integrations.incomingWebhooks(workspaceId) });
      }
    }
  });

  const createOutgoingWebhookMutation = useMutation({
    mutationFn: (input: CreateOutgoingWebhookInput) =>
      api.webhooks.createOutgoing(requireWorkspaceId(workspaceId), input),
    onSuccess: () => {
      if (workspaceId) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.integrations.outgoingWebhooks(workspaceId) });
      }
    }
  });

  const updateIncomingWebhookMutation = useMutation({
    mutationFn: ({ input, webhookId }: { input: { name?: string; status?: string }; webhookId: string }) =>
      api.webhooks.updateIncoming(requireWorkspaceId(workspaceId), webhookId, input),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: queryKeys.integrations.incomingWebhooks(workspaceId) })
  });

  const deleteIncomingWebhookMutation = useMutation({
    mutationFn: (webhookId: string) => api.webhooks.deleteIncoming(requireWorkspaceId(workspaceId), webhookId),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: queryKeys.integrations.incomingWebhooks(workspaceId) })
  });

  const updateOutgoingWebhookMutation = useMutation({
    mutationFn: ({ input, webhookId }: { input: { event_types?: string[]; name?: string; status?: string; target_url?: string }; webhookId: string }) =>
      api.webhooks.updateOutgoing(requireWorkspaceId(workspaceId), webhookId, input),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: queryKeys.integrations.outgoingWebhooks(workspaceId) })
  });

  const deleteOutgoingWebhookMutation = useMutation({
    mutationFn: (webhookId: string) => api.webhooks.deleteOutgoing(requireWorkspaceId(workspaceId), webhookId),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: queryKeys.integrations.outgoingWebhooks(workspaceId) })
  });

  const testOutgoingWebhookMutation = useMutation({
    mutationFn: (webhookId: string) => api.webhooks.testOutgoing(requireWorkspaceId(workspaceId), webhookId, {
      event_type: "admin.webhook.test",
      payload: { source: "admin-panel" }
    }),
    onSuccess: (_delivery, webhookId) => void queryClient.invalidateQueries({
      queryKey: queryKeys.integrations.webhookDeliveries(workspaceId, webhookId)
    })
  });

  const createCronjobMutation = useMutation({
    mutationFn: (input: SaveCronJobInput) => api.cronjobs.create(requireWorkspaceId(workspaceId), input),
    onSuccess: invalidateCronjobs
  });

  const updateCronjobMutation = useMutation({
    mutationFn: ({ cronjobId, input }: { cronjobId: string; input: SaveCronJobInput }) =>
      api.cronjobs.update(requireWorkspaceId(workspaceId), cronjobId, input),
    onSuccess: invalidateCronjobs
  });

  const deleteCronjobMutation = useMutation({
    mutationFn: (cronjobId: string) => api.cronjobs.delete(requireWorkspaceId(workspaceId), cronjobId),
    onSuccess: invalidateCronjobs
  });

  const runCronjobMutation = useMutation({
    mutationFn: (cronjobId: string) => api.cronjobs.runNow(requireWorkspaceId(workspaceId), cronjobId),
    onSuccess: invalidateCronjobs
  });

  const createBackupJobMutation = useMutation({
    mutationFn: (input: CreateBackupJobInput) => api.backups.createJob(requireWorkspaceId(workspaceId), input),
    onSuccess: invalidateBackupJobs
  });

  const runBackupJobMutation = useMutation({
    mutationFn: (backupJobId: string) => api.backups.runNow(requireWorkspaceId(workspaceId), backupJobId),
    onSuccess: invalidateBackupJobs
  });

  const setWorkspaceId = useCallback(
    (nextWorkspaceId: string) => {
      const nextParams = new URLSearchParams(searchParams.toString());

      if (nextWorkspaceId) {
        nextParams.set("workspace", nextWorkspaceId);
      } else {
        nextParams.delete("workspace");
      }

      const query = nextParams.toString();
      router.replace(query ? `${pathname}?${query}` : pathname, { scroll: false });
    },
    [pathname, router, searchParams]
  );

  useEffect(() => {
    if (!requestedWorkspaceId && workspaceId) {
      setWorkspaceId(workspaceId);
    }
  }, [requestedWorkspaceId, setWorkspaceId, workspaceId]);

  return {
    addMemberMutation,
    adminChannels: adminChannelsQuery.data ?? [],
    adminChannelsQuery,
    adminMessages: adminMessagesQuery.data ?? [],
    adminMessagesQuery,
    apiScopes: apiScopesQuery.data ?? [],
    apiScopesQuery,
    apiTokens: apiTokensQuery.data ?? [],
    apiTokensQuery,
    automationInstallations: automationInstallationsQuery.data ?? [],
    automationInstallationsQuery,
    automationTemplates: automationTemplatesQuery.data ?? [],
    automationTemplatesQuery,
    assignMemberRoleMutation,
    auditLogs: auditLogsQuery.data ?? [],
    auditLogsQuery,
    botInstallations: botInstallationsQuery.data ?? [],
    botInstallationsQuery,
    bots: botsQuery.data ?? [],
    botsQuery,
    backupJobs: backupJobsQuery.data ?? [],
    backupJobsQuery,
    backupRuns: backupRunsQuery.data ?? [],
    backupRunsQuery,
    can,
    canManageBackups,
    canManageApiTokens,
    canManageAutomation,
    canManageBots,
    canManageCronjobs,
    canManageRoles,
    canManageUsers,
    canManageWebhooks,
    canManageWorkspace,
    canViewAdmin,
    canViewAudit,
    channels: channelsQuery.data ?? [],
    channelsQuery,
    createApiTokenMutation,
    createAutomationInstallationMutation,
    createAdditionalDomainMutation,
    createBackupJobMutation,
    createBotMutation,
    createCronjobMutation,
    createIncomingWebhookMutation,
    createOutgoingWebhookMutation,
    createDeploymentRequestMutation,
    createOIDCProviderMutation,
    createRoleMutation,
    cronjobRuns: cronjobRunsQuery.data ?? [],
    cronjobRunsQuery,
    cronjobs: cronjobsQuery.data ?? [],
    cronjobsQuery,
    deleteCronjobMutation,
    deleteAutomationInstallationMutation,
    deleteOIDCProviderMutation,
    deleteZoneDomainMutation,
    deleteIncomingWebhookMutation,
    deleteOutgoingWebhookMutation,
    healthQuery,
    incomingWebhooks: incomingWebhooksQuery.data ?? [],
    incomingWebhooksQuery,
    installBotMutation,
    members: membersQuery.data ?? [],
    membersQuery,
    outgoingWebhooks: outgoingWebhooksQuery.data ?? [],
    outgoingWebhooksQuery,
    oidcProviders: oidcProvidersQuery.data ?? [],
    oidcProvidersQuery,
    permissionCodes,
    permissions: permissionsQuery.data ?? [],
    permissionsCatalog: permissionsCatalogQuery.data ?? [],
    permissionsCatalogQuery,
    permissionsQuery,
    revokeApiTokenMutation,
    revokeMemberRoleMutation,
    roles: rolesQuery.data ?? [],
    rolesQuery,
    runBackupJobMutation,
    runCronjobMutation,
    selectedMemberRoles: selectedMemberRolesQuery.data ?? [],
    selectedMemberRolesQuery,
    selectedWorkspace,
    sendBotMessageMutation,
    setWorkspaceId,
    setPrimaryDomainMutation,
    setZoneLifecycleMutation,
    settings: settingsQuery.data ?? [],
    settingsQuery,
    statsQuery,
    operationsQueryEnabled,
    updateMemberStatusMutation,
    updateAutomationInstallationMutation,
    updateCurrentZoneMutation,
    updateIncomingWebhookMutation,
    updateOutgoingWebhookMutation,
    updateOIDCProviderMutation,
    updateZoneQuotaMutation,
    updateCronjobMutation,
    updateUserMutation,
    upsertWorkspaceSettingMutation,
    users: usersQuery.data ?? [],
    usersQuery,
    verifyZoneDomainMutation,
    webhookDeliveries: webhookDeliveriesQuery.data ?? [],
    webhookDeliveriesQuery,
    currentZone: currentZoneQuery.data ?? null,
    currentZoneQuery,
    deploymentRequests: deploymentRequestsQuery.data ?? [],
    deploymentRequestsQuery,
    testOutgoingWebhookMutation,
    workspaceQuery,
    workspaceId,
    workspaces,
    workspacesQuery,
    zoneQuota: zoneQuotaQuery.data ?? null,
    zoneQuotaQuery
  };
}

function requireWorkspaceId(workspaceId: string): string {
  if (!workspaceId) {
    throw new Error("Chưa chọn workspace.");
  }

  return workspaceId;
}

function isMissingAdminOverviewEndpoint(error: unknown) {
  return error instanceof ApiClientError && error.status === 404;
}
