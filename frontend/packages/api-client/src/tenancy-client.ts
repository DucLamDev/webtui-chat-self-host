import type {
  AutomationInstallation,
  AutomationTemplate,
  CreatedAutomationInstallation,
  CreateAutomationInstallationInput,
  CreateDomainClaimInput,
  CreateZoneOIDCProviderInput,
  DomainClaim,
  UpdateZoneOIDCProviderInput,
  UpdateZoneQuotaInput,
  UpdateAutomationInstallationInput,
  ZoneAdminOverview,
  ZoneCapabilities,
  ZoneDeploymentRequest,
  ZoneDiscovery,
  ZoneOIDCProvider,
  ZoneQuotaOverview
} from "@webtui/types";
import type { HttpClient } from "./http-client";
import { collectionFrom, itemFrom } from "./response-utils";

export function createTenancyClient(http: HttpClient) {
  return {
    async discover(domain: string) {
      const data = await http.get<unknown>("/api/v1/discovery", {
        auth: false,
        query: { domain }
      });
      return requiredItem<ZoneDiscovery>(data, "discovery");
    },
    async capabilities(domain: string) {
      const data = await http.get<unknown>("/api/v1/capabilities", {
        auth: false,
        query: { domain }
      });
      return requiredItem<ZoneCapabilities>(data, "capabilities");
    },
    async createClaim(input: CreateDomainClaimInput) {
      const data = await http.post<unknown>("/api/v1/zones/claims", input);
      return requiredItem<DomainClaim>(data, "claim");
    },
    async claim(domainId: string) {
      const data = await http.get<unknown>(`/api/v1/zones/claims/${encodeURIComponent(domainId)}`);
      return requiredItem<DomainClaim>(data, "claim");
    },
    async verifyClaim(domainId: string) {
      const data = await http.post<unknown>(
        `/api/v1/zones/claims/${encodeURIComponent(domainId)}/verify`
      );
      return requiredItem<ZoneDiscovery>(data, "discovery");
    },
    async automationTemplates() {
      const data = await http.get<unknown>("/api/v1/zones/current/automation-templates");
      return collectionFrom<AutomationTemplate>(data, "templates");
    },
    async automationInstallations() {
      const data = await http.get<unknown>("/api/v1/zones/current/automation-installations");
      return collectionFrom<AutomationInstallation>(data, "installations");
    },
    async createAutomationInstallation(input: CreateAutomationInstallationInput) {
      const data = await http.post<unknown>(
        "/api/v1/zones/current/automation-installations",
        input
      );
      return requiredItem<CreatedAutomationInstallation>(data, "installation");
    },
    async updateAutomationInstallation(
      installationId: string,
      input: UpdateAutomationInstallationInput
    ) {
      const data = await http.patch<unknown>(
        `/api/v1/zones/current/automation-installations/${encodeURIComponent(installationId)}`,
        input
      );
      return requiredItem<AutomationInstallation>(data, "installation");
    },
    async deleteAutomationInstallation(installationId: string) {
      await http.delete<void>(
        `/api/v1/zones/current/automation-installations/${encodeURIComponent(installationId)}`
      );
    },
    async currentZone() {
      const data = await http.get<unknown>("/api/v1/zones/current");
      return requiredItem<ZoneAdminOverview>(data, "zone");
    },
    async updateCurrentZone(input: {
      name?: string;
      registration_mode?: "open" | "invite_only" | "closed";
    }) {
      const data = await http.patch<unknown>("/api/v1/zones/current", input);
      return requiredItem<ZoneAdminOverview>(data, "zone");
    },
    async setZoneLifecycle(
      action: "suspend" | "resume" | "archive",
      reason?: string,
      accessToken?: string
    ) {
      await http.post<void>(
        "/api/v1/zones/current/lifecycle",
        { action, reason },
        accessToken
          ? { auth: false, headers: { Authorization: `Bearer ${accessToken}` } }
          : undefined
      );
    },
    async createAdditionalDomain(input: { domain: string; kind?: "alias" | "api" | "web" }) {
      const data = await http.post<unknown>("/api/v1/zones/current/domains", input);
      return requiredItem<DomainClaim>(data, "claim");
    },
    async setPrimaryDomain(domainId: string) {
      await http.post<void>(
        `/api/v1/zones/current/domains/${encodeURIComponent(domainId)}/primary`
      );
    },
    async deleteDomain(domainId: string) {
      await http.delete<void>(
        `/api/v1/zones/current/domains/${encodeURIComponent(domainId)}`
      );
    },
    async deploymentRequests() {
      const data = await http.get<unknown>("/api/v1/zones/current/deployment-requests");
      return collectionFrom<ZoneDeploymentRequest>(data, "deployment_requests");
    },
    async createDeploymentRequest(
      input: { requested_mode: string; requested_database_mode: string },
      idempotencyKey: string
    ) {
      const data = await http.post<unknown>(
        "/api/v1/zones/current/deployment-requests",
        input,
        { headers: { "Idempotency-Key": idempotencyKey } }
      );
      return requiredItem<ZoneDeploymentRequest>(data, "deployment_request");
    },
    async zoneQuota() {
      const data = await http.get<unknown>("/api/v1/zones/current/quota");
      return requiredItem<ZoneQuotaOverview>(data, "quota_overview");
    },
    async updateZoneQuota(input: UpdateZoneQuotaInput) {
      const data = await http.put<unknown>("/api/v1/zones/current/quota", input);
      return requiredItem<ZoneQuotaOverview>(data, "quota_overview");
    },
    async oidcProviders() {
      const data = await http.get<unknown>("/api/v1/zones/current/oidc-providers");
      return collectionFrom<ZoneOIDCProvider>(data, "oidc_providers");
    },
    async createOIDCProvider(input: CreateZoneOIDCProviderInput) {
      const data = await http.post<unknown>("/api/v1/zones/current/oidc-providers", input);
      return requiredItem<ZoneOIDCProvider>(data, "oidc_provider");
    },
    async updateOIDCProvider(providerId: string, input: UpdateZoneOIDCProviderInput) {
      const data = await http.patch<unknown>(
        `/api/v1/zones/current/oidc-providers/${encodeURIComponent(providerId)}`,
        input
      );
      return requiredItem<ZoneOIDCProvider>(data, "oidc_provider");
    },
    async deleteOIDCProvider(providerId: string) {
      await http.delete<void>(
        `/api/v1/zones/current/oidc-providers/${encodeURIComponent(providerId)}`
      );
    }
  };
}

function requiredItem<T>(data: unknown, key: string): T {
  const item = itemFrom<T>(data, key);
  if (!item) {
    throw new Error(`API response is missing ${key}.`);
  }
  return item;
}
