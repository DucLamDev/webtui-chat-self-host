import { describe, expect, it } from "vitest";
import {
  persistentDesktopAccounts,
  type AuthAccount
} from "../../apps/web/src/features/auth/auth-store";

describe("desktop auth persistence", () => {
  it("never persists access tokens in the multi-server account registry", () => {
    const account: AuthAccount = {
      accessToken: "short-lived-access-token",
      domain: "chat.example.com",
      refreshToken: "refresh-token",
      runtime: {
        api_base_url: "https://chat.example.com",
        app_name: "Chat",
        app_version: "1.0.0",
        locale: "vi",
        release_channel: "stable",
        web_base_url: "https://chat.example.com",
        ws_base_url: "wss://chat.example.com/ws"
      },
      sessionId: "session",
      updatedAt: "2026-08-01T00:00:00Z",
      user: null
    };

    expect(persistentDesktopAccounts([account])).toEqual([
      { ...account, accessToken: null }
    ]);
    expect(account.accessToken).toBe("short-lived-access-token");
  });
});
