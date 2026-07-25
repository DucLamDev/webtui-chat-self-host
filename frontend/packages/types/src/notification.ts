import type { Id, ISODateTime, JsonObject } from "./api";

export type Notification = {
  id: Id;
  user_id: Id;
  workspace_id?: Id | null;
  channel_id?: Id | null;
  message_id?: Id | null;
  type: string;
  title: string;
  body: string;
  data?: JsonObject | null;
  read_at?: ISODateTime | null;
  delivered_at?: ISODateTime | null;
  created_at: ISODateTime;
};

export type NotificationPreferenceMode = "all" | "mentions" | "muted";

export type NotificationPreference = {
  user_id: Id;
  workspace_id: Id;
  mode: NotificationPreferenceMode;
  preview: boolean;
  quiet_hours: boolean;
  quiet_start: string;
  quiet_end: string;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type NotificationPreferenceInput = {
  workspace_id: Id;
  mode: NotificationPreferenceMode;
  preview: boolean;
  quiet_hours: boolean;
  quiet_start: string;
  quiet_end: string;
};
