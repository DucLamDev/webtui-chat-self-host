import type { HttpClient } from "./http-client";

export type HealthStatus = {
  status: "ok" | "ready" | "not_ready" | string;
  app?: string;
  env?: string;
  uptime?: string;
  checks?: Record<string, string>;
};

export type VersionInfo = {
  app: string;
  clients?: {
    desktop?: {
      minimum_version?: string;
      recommended_version?: string;
      update_url?: string;
    };
  };
  env: string;
  version: string;
};

export function createHealthClient(http: HttpClient) {
  return {
    health: () => http.get<HealthStatus>("/health", { auth: false }),
    ready: () => http.get<HealthStatus>("/ready", { auth: false }),
    version: () => http.get<VersionInfo>("/version", { auth: false }),
    apiV1: () =>
      http.get<{ name: string; version: string; status: string }>("/api/v1", {
        auth: false
      })
  };
}
