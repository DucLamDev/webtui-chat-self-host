import { describe, expect, it, vi } from "vitest";
import type { PlatformStorage } from "@webtui/chat-core";
import {
  cleanupWebPushForAccount,
  detectWebPushCapability,
  readLocalWebPushRecord,
  sameWebPushAccount,
  subscriptionInput,
  vapidPublicKeyBytes,
  writeLocalWebPushRecord
} from "../../apps/web/src/features/notifications/web-push";

function memoryStorage(): PlatformStorage {
  const values = new Map<string, string>();
  return {
    getItem: (key) => values.get(key) ?? null,
    removeItem: (key) => {
      values.delete(key);
    },
    setItem: (key, value) => {
      values.set(key, value);
    }
  };
}

const account = {
  serverId: "https://CHAT.example.test/",
  userId: "user-1"
};

describe("Web Push browser model", () => {
  it("persists normalized account ownership independent of the workspace proof", async () => {
    const storage = memoryStorage();
    const record = await writeLocalWebPushRecord(
      account,
      { endpoint: "https://push.example.test/1", subscriptionId: "subscription-1" },
      storage
    );

    expect((await readLocalWebPushRecord(storage))?.serverId).toBe("https://chat.example.test");
    expect(record).not.toHaveProperty("workspaceId");
    expect(sameWebPushAccount(record, { ...account, serverId: "https://chat.example.test" })).toBe(true);
    expect(sameWebPushAccount(record, { ...account, userId: "user-2" })).toBe(false);
  });

  it("serializes browser subscription keys and expiration for the API contract", () => {
    const expirationTime = Date.now() + 60_000;
    const subscription = {
      endpoint: "https://push.example.test/1",
      expirationTime,
      toJSON: () => ({
        endpoint: "https://push.example.test/1",
        expirationTime,
        keys: { auth: "auth-key", p256dh: "p256dh-key" }
      })
    } as unknown as PushSubscription;

    expect(subscriptionInput(subscription, "workspace-1")).toEqual({
      endpoint: "https://push.example.test/1",
      expiration_time: new Date(expirationTime).toISOString(),
      keys: { auth: "auth-key", p256dh: "p256dh-key" },
      workspace_id: "workspace-1"
    });
  });

  it("decodes URL-safe base64 VAPID public keys", () => {
    expect([...vapidPublicKeyBytes("AQIDBA")]).toEqual([1, 2, 3, 4]);
    expect(() => vapidPublicKeyBytes("***")).toThrow("VAPID public key");
  });

  it("revokes and clears only the matching account during logout", async () => {
    const storage = memoryStorage();
    await writeLocalWebPushRecord(
      account,
      { endpoint: "https://push.example.test/1", subscriptionId: "subscription-1" },
      storage
    );
    const revoke = vi.fn(async () => undefined);

    await cleanupWebPushForAccount(account, revoke, storage);

    expect(revoke).toHaveBeenCalledWith("subscription-1");
    expect(await readLocalWebPushRecord(storage)).toBeNull();
  });

  it("reports unsupported outside a secure browser without requesting permission", () => {
    expect(detectWebPushCapability()).toMatchObject({ supported: false });
  });
});
