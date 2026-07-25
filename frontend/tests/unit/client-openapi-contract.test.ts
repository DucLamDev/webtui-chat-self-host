import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../../..");
const openApiSource = readFileSync(resolve(repoRoot, "backend/api/openapi/openapi.yaml"), "utf8");
const openApiPaths = new Set(
  [...openApiSource.matchAll(/^ {2}(\/(?:api\/v1\/[^:\n]+|version|desktop\/releases\/[^:\n]+)):/gm)].map((match) => match[1])
);

const desktopMvpPaths = [
  "/version",
  "/desktop/releases/{channel}/{target}/{arch}/{current_version}",
  "/api/v1/ws",
  "/api/v1/auth/login",
  "/api/v1/auth/refresh",
  "/api/v1/auth/logout",
  "/api/v1/auth/me",
  "/api/v1/auth/sessions",
  "/api/v1/workspaces",
  "/api/v1/workspaces/{workspace_id}/members",
  "/api/v1/workspaces/{workspace_id}/channels",
  "/api/v1/workspaces/{workspace_id}/channels/{channel_id}/read-state",
  "/api/v1/workspaces/{workspace_id}/direct-conversations",
  "/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages",
  "/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages/{message_id}",
  "/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages/{message_id}/thread",
  "/api/v1/workspaces/{workspace_id}/channels/{channel_id}/messages/{message_id}/attachments",
  "/api/v1/workspaces/{workspace_id}/messages/search",
  "/api/v1/workspaces/{workspace_id}/files",
  "/api/v1/notifications",
  "/api/v1/notifications/preferences",
  "/api/v1/notifications/read-all",
  "/api/v1/workspaces/{workspace_id}/presence",
  "/api/v1/workspaces/{workspace_id}/presence/heartbeat",
  "/api/v1/workspaces/{workspace_id}/departments",
  "/api/v1/workspaces/{workspace_id}/bots",
  "/api/v1/workspaces/{workspace_id}/tickets",
  "/api/v1/workspaces/{workspace_id}/tickets/{ticket_id}",
  "/api/v1/workspaces/{workspace_id}/cronjobs"
] as const;

const orderBotPaths = [
  "/api/v1/workspaces/{workspace_id}/order-bot/wallet/balance",
  "/api/v1/workspaces/{workspace_id}/order-bot/wallet/deposit-qr",
  "/api/v1/workspaces/{workspace_id}/order-bot/payment/order-qr",
  "/api/v1/workspaces/{workspace_id}/order-bot/services/renew"
] as const;

describe("desktop OpenAPI contract", () => {
  it("documents every route required by the desktop MVP chat shell", () => {
    const missingPaths = desktopMvpPaths.filter((path) => !openApiPaths.has(path));

    expect(missingPaths).toEqual([]);
  });

  it("does not allow a partially documented order bot contract", () => {
    const declaredOrderBotPaths = orderBotPaths.filter((path) => openApiPaths.has(path));

    expect([0, orderBotPaths.length]).toContain(declaredOrderBotPaths.length);
  });

  it("documents desktop version compatibility policy", () => {
    expect(openApiSource).toContain("DesktopVersionPolicy");
    expect(openApiSource).toContain("DesktopUpdaterManifest");
    expect(openApiSource).toContain("minimum_version");
    expect(openApiSource).toContain("recommended_version");
    expect(openApiSource).toContain("update_url");
  });
});
