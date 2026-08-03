import type {
  AdminChannelOverview,
  AdminHealth,
  AdminMessageOverview,
  AdminPushQueue,
  AdminPushReplay,
  AdminStats,
  AuditLog
} from "@webtui/types";
import type { HttpClient, QueryParams } from "./http-client";
import { collectionFrom } from "./response-utils";

export function createAdminClient(http: HttpClient) {
  return {
    stats(workspaceId: string) {
      return http.get<AdminStats>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/admin/stats`);
    },
    health(workspaceId: string) {
      return http.get<AdminHealth>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/admin/health`);
    },
    pushQueue(workspaceId: string, deadLetterLimit = 50) {
      return http.get<AdminPushQueue>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/admin/push`, {
        query: { dead_letter_limit: deadLetterLimit }
      });
    },
    replayPushDeadLetter(workspaceId: string, jobId: string) {
      return http.post<AdminPushReplay>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/admin/push/dead-letters/${encodeURIComponent(jobId)}/replay`,
        {}
      );
    },
    async channels(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/admin/channels`);
      return collectionFrom<AdminChannelOverview>(data, "channels");
    },
    async messages(workspaceId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/admin/messages`, { query: params });
      return collectionFrom<AdminMessageOverview>(data, "messages");
    },
    async auditLogs(workspaceId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/audit-logs`, {
        query: params
      });
      return collectionFrom<AuditLog>(data, "audit_logs");
    }
  };
}
