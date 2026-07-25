import type { Id, ISODateTime, JsonValue } from "./api";

export type AdminStats = {
  active_members?: number;
  audit_logs?: number;
  backup_jobs?: number;
  bots?: number;
  channels?: number;
  files?: number;
  generated_at?: ISODateTime;
  incoming_webhooks?: number;
  messages?: number;
  outgoing_webhooks?: number;
  workspace_id?: string;
  users_count?: number;
  user_count?: number;
  active_users_count?: number;
  channels_count?: number;
  channel_count?: number;
  messages_count?: number;
  message_count?: number;
  files_count?: number;
  file_count?: number;
  storage_bytes?: number;
  storage_size_bytes?: number;
  activity?: Array<{
    date?: string;
    messages?: number;
    users?: number;
  }>;
  channel_ranks?: Array<{
    id?: string;
    channel_id?: string;
    name?: string;
    messages_count?: number;
    count?: number;
  }>;
  updated_at?: ISODateTime;
  [key: string]: unknown;
};

export type AdminHealth = {
  status?: string;
  database?: string;
  redis?: string;
  storage?: string;
  queue?: string;
  checks?: Record<string, unknown>;
  updated_at?: ISODateTime;
  [key: string]: unknown;
};

export type AdminChannelOverview = {
  id: Id;
  name: string;
  slug?: string;
  type: string;
  status: string;
  member_count: number;
  message_count: number;
  private_session_mode: boolean;
  updated_at: ISODateTime;
};

export type AdminMessageOverview = {
  id: Id;
  channel_id: Id;
  channel_name: string;
  sender_name: string;
  kind: string;
  body: string;
  created_at: ISODateTime;
};

export type AuditLog = {
  id: Id;
  workspace_id?: Id;
  actor_user_id?: Id;
  action: string;
  entity_type: string;
  entity_id?: Id;
  ip_address?: string;
  user_agent?: string;
  before_data?: JsonValue;
  after_data?: JsonValue;
  metadata?: JsonValue;
  created_at: ISODateTime;
};
