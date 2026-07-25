import type { Id, ISODateTime, JsonObject } from "./api";

export type PresenceStatusValue = "online" | "away" | "offline" | string;

export type Presence = {
  user_id: Id;
  workspace_id?: Id | null;
  device_id: string;
  socket_id: string;
  node_id: string;
  status: PresenceStatusValue;
  last_heartbeat_at: ISODateTime;
  connected_at: ISODateTime;
  metadata?: JsonObject | null;
};

export type PresenceHeartbeatInput = {
  device_id: string;
  socket_id?: string;
  node_id?: string;
  status?: PresenceStatusValue;
  metadata?: JsonObject;
};
