import type { Id } from "./api";

export type UserStatus = "active" | "blocked" | "disabled" | "locked" | "pending";

export type RuntimeIceServer = {
  credential?: string;
  credentialType?: "oauth" | "password";
  urls: string | string[];
  username?: string;
};

export type UserSummary = {
  id: Id;
  email: string;
  username: string;
  display_name: string;
  avatar_url?: string | null;
  status: UserStatus;
};

export type WorkspaceSummary = {
  id: Id;
  name: string;
  slug: string;
  description?: string | null;
};

export type ChannelKind = "public" | "private" | "direct";

export type ChannelSummary = {
  id: Id;
  workspace_id: Id;
  name: string;
  kind: ChannelKind;
  description?: string | null;
  unread_count?: number;
  is_favorite?: boolean;
};

export type PermissionCode =
  | "workspace.manage"
  | "workspace.invite_user"
  | "workspace.view_members"
  | "user.manage"
  | "role.manage"
  | "channel.create"
  | "channel.manage"
  | "channel.delete"
  | "message.send"
  | "message.manage"
  | "file.upload"
  | "api_token.manage"
  | "bot.manage"
  | "order.view"
  | "order.billing"
  | "webhook.manage"
  | "cronjob.manage"
  | "backup.manage"
  | "audit.view"
  | "admin.view";

export type RuntimeEnvironment = {
  apiBaseUrl: string;
  appVersion: string;
  releaseChannel: "beta" | "stable" | string;
  rtcIceServers: RuntimeIceServer[];
  wsBaseUrl: string;
  appName: string;
  locale: string;
};
