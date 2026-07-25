export type ZoneKind = "vpsttt_internal" | "customer_saas" | "customer_dedicated";
export type ZoneStatus = "provisioning" | "active" | "suspended" | "archived";
export type ZoneDeploymentMode = "shared" | "dedicated_compose" | "dedicated_k8s";
export type ZoneDeploymentStatus = "provisioning" | "ready" | "failed" | "suspended";
export type ZoneRegistrationMode = "open" | "invite_only" | "closed";

export type ZoneSummary = {
  id: string;
  slug: string;
  name: string;
  kind: ZoneKind | string;
  status: ZoneStatus | string;
  registration_mode: ZoneRegistrationMode | string;
};

export type ZoneWorkspaceRef = {
  id: string;
  slug: string;
  name: string;
};

export type ZoneRuntime = {
  app_name: string;
  app_version: string;
  release_channel: string;
  locale: string;
  web_base_url: string;
  api_base_url: string;
  ws_base_url: string;
  admin_base_url?: string;
  rtc_ice_servers?: RuntimeIceServer[];
};

export type ZoneCapabilities = {
  chat: boolean;
  files: boolean;
  calls: boolean;
  bots: boolean;
  automation: boolean;
  webhooks: boolean;
  federation: boolean;
  sso: boolean;
  oidc_configuration: boolean;
  custom_domain: boolean;
  dedicated: boolean;
  self_hosted: boolean;
};

export type ZoneDeployment = {
  mode: ZoneDeploymentMode | string;
  database_mode: "shared_schema" | "dedicated_schema" | "dedicated_database" | string;
  status: ZoneDeploymentStatus | string;
};

export type ZoneDiscovery = {
  version: string;
  domain: string;
  zone: ZoneSummary;
  workspace?: ZoneWorkspaceRef | null;
  runtime: ZoneRuntime;
  capabilities: ZoneCapabilities;
  deployment: ZoneDeployment;
};

export type DomainClaim = {
  id: string;
  domain: string;
  status: "pending" | "verified" | "active" | "suspended" | string;
  routing_dns_type?: "A" | "AAAA" | string;
  routing_dns_name?: string;
  routing_dns_value?: string;
  verification_method: "dns_txt" | "http_well_known" | "manual" | string;
  verification_dns_name: string;
  verification_dns_value: string;
  verification_expires_at?: string | null;
  verification_attempts: number;
  last_verification_error?: string | null;
  last_checked_at?: string | null;
  zone: ZoneSummary;
  workspace?: ZoneWorkspaceRef | null;
};

export type CreateDomainClaimInput = {
  domain: string;
  zone_name: string;
  registration_mode?: ZoneRegistrationMode;
};

export type AutomationTemplate = {
  id: string;
  key: string;
  name: string;
  description?: string | null;
  zone_kind: ZoneKind | "any" | string;
  template_type: "bot" | "workflow" | "connector" | string;
  runtime_kind: "none" | "outgoing_webhook" | string;
  config_schema: Record<string, unknown>;
  default_config: Record<string, unknown>;
  required_scopes: string[];
  status: string;
};

export type AutomationInstallation = {
  id: string;
  zone_id: string;
  workspace_id?: string | null;
  template_id?: string | null;
  template_key?: string | null;
  name: string;
  status: "enabled" | "disabled" | "failed" | string;
  config: Record<string, unknown>;
  has_secret_ref: boolean;
  runtime_webhook_id?: string | null;
  runtime_ready: boolean;
  created_at: string;
  updated_at: string;
};

export type CreatedAutomationInstallation = AutomationInstallation & {
  runtime_secret?: string;
};

export type CreateAutomationInstallationInput = {
  workspace_id?: string;
  template_key: string;
  name: string;
  status?: "enabled" | "disabled";
  config?: Record<string, unknown>;
  secret_ref?: string;
};

export type UpdateAutomationInstallationInput = {
  name?: string;
  status?: "enabled" | "disabled";
  config?: Record<string, unknown>;
  secret_ref?: string;
};

export type ZoneAdminDomain = {
  id: string;
  domain: string;
  kind: "primary" | "alias" | "api" | "web" | string;
  status: string;
  tls_status: string;
  verification_method: string;
  verification_dns_name: string;
  verification_dns_value?: string;
  verification_expires_at?: string | null;
  verified_at?: string | null;
  created_at: string;
  updated_at: string;
};

export type ZoneAdminOverview = {
  zone: ZoneSummary;
  domains: ZoneAdminDomain[];
  deployments: ZoneDeployment[];
};

export type ZoneDeploymentRequest = {
  id: string;
  zone_id: string;
  requested_mode: ZoneDeploymentMode | string;
  requested_database_mode: string;
  status: string;
  idempotency_key: string;
  failure_reason?: string | null;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  completed_at?: string | null;
};

export type ZoneQuota = {
  max_workspaces: number;
  max_members: number;
  max_storage_bytes: number;
  max_automation_installations: number;
  max_webhooks: number;
  enforcement_mode: "monitor" | "hard";
  updated_at: string;
};

export type ZoneUsage = {
  workspaces: number;
  members: number;
  storage_bytes: number;
  automation_installations: number;
  webhooks: number;
};

export type ZoneQuotaOverview = {
  quota: ZoneQuota;
  usage: ZoneUsage;
};

export type UpdateZoneQuotaInput = Omit<ZoneQuota, "updated_at">;

export type ZoneOIDCProvider = {
  id: string;
  zone_id: string;
  name: string;
  issuer_url: string;
  client_id: string;
  has_client_secret_ref: boolean;
  scopes: string[];
  claim_mapping: Record<string, unknown>;
  jit_provisioning: boolean;
  require_verified_email: boolean;
  status: "configured" | "disabled";
  created_at: string;
  updated_at: string;
};

export type CreateZoneOIDCProviderInput = {
  name: string;
  issuer_url: string;
  client_id: string;
  client_secret_ref?: string;
  scopes?: string[];
  claim_mapping?: Record<string, unknown>;
  jit_provisioning?: boolean;
  require_verified_email?: boolean;
  status?: "configured" | "disabled";
};

export type UpdateZoneOIDCProviderInput = Partial<CreateZoneOIDCProviderInput>;
import type { RuntimeIceServer } from "./domain";
