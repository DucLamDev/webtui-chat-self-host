import type { Id, ISODateTime, JsonValue } from "./api";

export type ApiScope = {
  id: Id;
  code: string;
  name: string;
  description?: string | null;
  module: string;
  action: string;
};

export type ApiToken = {
  id: Id;
  workspace_id: Id;
  owner_id?: Id | null;
  name: string;
  status: string;
  last_used_at?: ISODateTime | null;
  expires_at?: ISODateTime | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
  revoked_at?: ISODateTime | null;
  scopes: ApiScope[];
};

export type CreatedApiToken = ApiToken & {
  token: string;
};

export type CreateApiTokenInput = {
  name: string;
  scopes: string[];
  expires_days?: number;
};

export type Bot = {
  id: Id;
  workspace_id: Id;
  slug: string;
  name: string;
  description?: string | null;
  avatar_url?: string | null;
  status: string;
  created_by?: Id | null;
  settings?: JsonValue;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type CreateBotInput = {
  slug: string;
  name: string;
  description?: string;
  avatar_url?: string;
  settings?: JsonValue;
};

export type BotInstallation = {
  id: Id;
  bot_id: Id;
  workspace_id: Id;
  channel_id?: Id | null;
  status: string;
  config?: JsonValue;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type InstallBotInput = {
  channel_id?: Id;
  config?: JsonValue;
};

export type BotMessage = {
  id: Id;
  workspace_id: Id;
  channel_id: Id;
  bot_id: Id;
  kind: string;
  body: string;
  metadata?: JsonValue;
  created_at: ISODateTime;
};

export type SendBotMessageInput = {
  channel_id: Id;
  body: string;
  metadata?: JsonValue;
};

export type OrderBotStatus = {
  configured: boolean;
  quick_order_configured?: boolean;
  base_url?: string;
};

export type OrderWalletBalanceData = {
  user_id?: number;
  email?: string;
  name?: string;
  balance?: number;
  balance_vnd?: number;
  money?: number;
  agency?: string;
  services?: Record<string, number>;
};

export type OrderWalletBalanceInput = {
  email?: string;
  user_id?: number;
  channel_id?: Id;
  post_to_channel?: boolean;
};

export type OrderWalletBalanceResult = {
  data: OrderWalletBalanceData;
  bot_message?: BotMessage;
};

export type OrderWalletDepositQRData = {
  request_id?: number;
  reference?: string;
  email?: string;
  user_id?: number;
  name?: string;
  amount?: number;
  currency?: string;
  status?: string;
  qr_url?: string;
  bank?: {
    bank_code?: string;
    bin?: string;
    account_number?: string;
    account_name?: string;
    transfer_content?: string;
    requested_amount?: number;
    auto_check?: boolean;
  };
  transfer_content?: string;
  user_balance_before?: number;
  expires_at?: string;
};

export type OrderWalletDepositQRInput = {
  email: string;
  amount: number;
  expires_minutes?: number;
  channel_id?: Id;
  post_to_channel?: boolean;
};

export type OrderWalletDepositQRResult = {
  data: OrderWalletDepositQRData;
  bot_message?: BotMessage;
};

export type OrderPaymentQRData = {
  payment_id?: number;
  intent_id?: number;
  external_order_id?: string;
  reference?: string;
  customer_email?: string;
  amount?: number;
  currency?: string;
  status?: string;
  qr_url?: string;
  bank?: Record<string, unknown>;
  expires_at?: string;
};

export type OrderPaymentQRInput = {
  intent_id?: number;
  intent_code?: string;
  reference?: string;
  channel_id?: Id;
  post_to_channel?: boolean;
};

export type OrderPaymentQRResult = {
  data: OrderPaymentQRData;
  bot_message?: BotMessage;
};

export type OrderServicesExpiringItem = {
  service_type_key?: string;
  service_type?: string;
  service_id?: number;
  name?: string;
  meta?: string;
  status?: string;
  status_label?: string;
  expires_at?: string;
  days_remaining?: number;
  autoextend?: number | null;
  route?: string;
  renewal_transfer_content?: string | null;
};

export type OrderServicesExpiringData = {
  user?: {
    user_id?: number;
    email?: string;
    name?: string;
    balance?: number;
  };
  days?: number;
  include_expired?: boolean;
  service_type?: string;
  summary?: {
    total?: number;
    expired?: number;
    expiring?: number;
    auto_renew_off?: number;
    by_type?: Record<string, number>;
  };
  items?: OrderServicesExpiringItem[];
};

export type OrderServicesExpiringInput = {
  email?: string;
  user_id?: number;
  days?: number;
  include_expired?: boolean;
  service_type?: "all" | "vps" | "proxy" | "hosting" | "s3" | "drive" | "waf" | "domain" | "separate";
  channel_id?: Id;
  post_to_channel?: boolean;
};

export type OrderServicesExpiringResult = {
  data: OrderServicesExpiringData;
  bot_message?: BotMessage;
};

export type OrderRenewServiceInput = {
  email?: string;
  user_id?: number;
  service_type?: "all" | "vps" | "proxy" | "hosting" | "s3" | "drive" | "waf" | "domain" | "separate";
  service_id?: number;
  service_name?: string;
  months: number;
  idempotency_key: string;
  channel_id?: Id;
  post_to_channel?: boolean;
};

export type OrderRenewServiceData = {
  outcome?: string;
  transaction_id?: string;
  user?: {
    user_id?: number;
    email?: string;
    name?: string;
    balance?: number;
  };
  service_type?: string;
  service_id?: number;
  service_name?: string;
  months?: number;
  amount?: number;
  balance_before?: number;
  balance_after?: number;
  shortage_amount?: number;
  expires_at_before?: string;
  expires_at_after?: string;
};

export type OrderRenewServiceResult = {
  data: OrderRenewServiceData;
  bot_message?: BotMessage;
};

export type IncomingWebhook = {
  id: Id;
  workspace_id: Id;
  channel_id?: Id | null;
  name: string;
  status: string;
  created_by?: Id | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
  last_used_at?: ISODateTime | null;
};

export type CreatedIncomingWebhook = IncomingWebhook & {
  secret: string;
  url: string;
};

export type CreateIncomingWebhookInput = {
  channel_id?: Id;
  name: string;
};

export type UpdateIncomingWebhookInput = {
  channel_id?: Id | null;
  name?: string;
  status?: string;
};

export type OutgoingWebhook = {
  id: Id;
  workspace_id: Id;
  name: string;
  target_url: string;
  event_types: string[];
  status: string;
  created_by?: Id | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type CreatedOutgoingWebhook = OutgoingWebhook & {
  secret: string;
};

export type CreateOutgoingWebhookInput = {
  name: string;
  target_url: string;
  event_types?: string[];
};

export type UpdateOutgoingWebhookInput = {
  name?: string;
  target_url?: string;
  event_types?: string[];
  status?: string;
};

export type TestOutgoingWebhookInput = {
  event_type?: string;
  payload?: JsonValue;
};

export type WebhookDelivery = {
  id: Id;
  outgoing_webhook_id: Id;
  event_id?: Id | null;
  event_type: string;
  request_body?: JsonValue;
  response_status?: number | null;
  response_body?: string | null;
  status: string;
  attempt_count: number;
  next_attempt_at?: ISODateTime | null;
  delivered_at?: ISODateTime | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};
