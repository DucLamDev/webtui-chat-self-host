import type { HttpClient } from "./http-client";
import { itemFrom } from "./response-utils";

export type CallMode = "audio" | "video";

export type CallStatus = "ringing" | "accepted" | "rejected" | "cancelled" | "ended" | "missed";

export type CallSession = {
  id: string;
  workspace_id: string;
  channel_id: string;
  initiator_user_id: string;
  target_user_id: string;
  client_call_id?: string | null;
  mode: CallMode;
  status: CallStatus;
  metadata?: Record<string, unknown>;
  started_at?: string | null;
  ended_at?: string | null;
  created_at?: string;
  updated_at?: string;
};

export type CreateCallInput = {
  channel_id: string;
  target_user_id: string;
  client_call_id?: string;
  mode: CallMode;
  metadata?: Record<string, unknown>;
};

export type CallSignalType = "offer" | "answer" | "ice_candidate" | "ready";

export function createCallsClient(http: HttpClient) {
  const basePath = (workspaceId: string) => `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/calls`;
  const callPath = (workspaceId: string, callId: string) => `${basePath(workspaceId)}/${encodeURIComponent(callId)}`;

  async function unwrapCall(value: Promise<unknown>): Promise<CallSession> {
    const data = await value;
    const call = itemFrom<CallSession>(data, "call");
    if (!call) {
      throw new Error("Máy chủ không trả về thông tin cuộc gọi.");
    }
    return call;
  }

  return {
    create(workspaceId: string, input: CreateCallInput) {
      return unwrapCall(http.post<unknown>(basePath(workspaceId), input));
    },
    get(workspaceId: string, callId: string) {
      return unwrapCall(http.get<unknown>(callPath(workspaceId, callId)));
    },
    accept(workspaceId: string, callId: string) {
      return unwrapCall(http.post<unknown>(`${callPath(workspaceId, callId)}/accept`, {}));
    },
    reject(workspaceId: string, callId: string, reason?: string) {
      return unwrapCall(http.post<unknown>(`${callPath(workspaceId, callId)}/reject`, { reason }));
    },
    cancel(workspaceId: string, callId: string, reason?: string) {
      return unwrapCall(http.post<unknown>(`${callPath(workspaceId, callId)}/cancel`, { reason }));
    },
    hangup(workspaceId: string, callId: string, reason?: string) {
      return unwrapCall(http.post<unknown>(`${callPath(workspaceId, callId)}/hangup`, { reason }));
    },
    miss(workspaceId: string, callId: string, reason?: string) {
      return unwrapCall(http.post<unknown>(`${callPath(workspaceId, callId)}/miss`, { reason }));
    },
    signal(
      workspaceId: string,
      callId: string,
      signalType: CallSignalType,
      payload: Record<string, unknown>
    ) {
      return http.post<unknown>(`${callPath(workspaceId, callId)}/signals`, {
        payload,
        signal_type: signalType
      });
    }
  };
}
