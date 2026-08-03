import type { HttpClient } from "./http-client";

export type WorkspaceSyncEvent = {
  aggregate_id: string;
  aggregate_type: string;
  event_id: string;
  event_version: number;
  occurred_at: string;
  payload?: Record<string, unknown>;
  type: string;
  workspace_id: string;
};

export type WorkspaceSyncPage = {
  events: WorkspaceSyncEvent[];
  has_more: boolean;
  next_cursor?: string;
  server_time: string;
};

export function createSyncClient(http: HttpClient) {
  const basePath = (workspaceId: string) =>
    `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/sync`;
  return {
    catchUp(workspaceId: string, deviceId: string, cursor?: string) {
      return http.get<WorkspaceSyncPage>(basePath(workspaceId), {
        query: { cursor, device_id: deviceId, limit: 200 }
      });
    },
    ack(workspaceId: string, deviceId: string, cursor: string) {
      return http.post(`${basePath(workspaceId)}/ack`, {
        cursor,
        device_id: deviceId
      });
    }
  };
}
