import { describe, expect, it, vi } from "vitest";
import {
  clearOfflineAccount,
  enqueueMessageOutbox,
  flushMessageOutbox,
  listMessageOutbox,
  MemoryOfflineRepository,
  outboxBackoffMs,
  readSyncCheckpoint,
  retryMessageOutbox,
  writeSyncCheckpoint
} from "../../apps/web/src/features/chat/model/message-outbox";

const primaryScope = {
  serverId: "https://chat.example.test/",
  userId: "user-1",
  workspaceId: "workspace-1"
};

describe("durable message outbox policy", () => {
  it("isolates records by server, user and workspace", async () => {
    const store = new MemoryOfflineRepository();
    const otherWorkspace = { ...primaryScope, workspaceId: "workspace-2" };
    const otherUser = { ...primaryScope, userId: "user-2" };

    await enqueueMessageOutbox(primaryScope, message("client-a"), undefined, store);
    await enqueueMessageOutbox(otherWorkspace, message("client-b"), undefined, store);
    await enqueueMessageOutbox(otherUser, message("client-c"), undefined, store);

    expect((await listMessageOutbox(primaryScope, store)).map((entry) => entry.clientMessageId)).toEqual(["client-a"]);
    expect((await listMessageOutbox(otherWorkspace, store)).map((entry) => entry.clientMessageId)).toEqual(["client-b"]);
    expect((await listMessageOutbox(otherUser, store)).map((entry) => entry.clientMessageId)).toEqual(["client-c"]);
  });

  it("upserts the same client id and flushes messages sequentially", async () => {
    const store = new MemoryOfflineRepository();
    await enqueueMessageOutbox(primaryScope, message("client-a", "old"), undefined, store);
    await enqueueMessageOutbox(primaryScope, message("client-a", "updated"), undefined, store);
    await enqueueMessageOutbox(primaryScope, message("client-b", "second"), undefined, store);
    const order: string[] = [];

    const remaining = await flushMessageOutbox(
      primaryScope,
      {
        force: true,
        send: async (entry) => {
          order.push(entry.body);
          return { id: `server-${entry.clientMessageId}` };
        }
      },
      store
    );

    expect(order).toEqual(["updated", "second"]);
    expect(remaining).toEqual([]);
  });

  it("backs off after a transient failure and preserves later message order", async () => {
    const store = new MemoryOfflineRepository();
    await enqueueMessageOutbox(primaryScope, message("client-a"), undefined, store);
    await enqueueMessageOutbox(primaryScope, message("client-b"), undefined, store);
    const send = vi.fn(async () => {
      throw new TypeError("Failed to fetch");
    });

    const remaining = await flushMessageOutbox(
      primaryScope,
      { force: true, now: () => 10_000, random: () => 0, send },
      store
    );

    expect(send).toHaveBeenCalledTimes(1);
    expect(remaining).toHaveLength(2);
    expect(remaining[0]).toMatchObject({ attempts: 1, retryable: true, status: "failed" });
    expect(remaining[0].nextAttemptAt).toBe(10_750);
    expect(remaining[1]).toMatchObject({ attempts: 0, status: "pending" });
  });

  it("uses an atomic lease so another tab cannot claim an in-flight record", async () => {
    const store = new MemoryOfflineRepository();
    await enqueueMessageOutbox(primaryScope, message("client-a"), undefined, store);
    const scopeKey = (await listMessageOutbox(primaryScope, store))[0].scopeKey;

    const firstClaim = await store.claimNextMessage(scopeKey, "tab-a", 1_000, 30_000, true);
    const competingClaim = await store.claimNextMessage(scopeKey, "tab-b", 1_001, 30_000, true);
    const recoveredClaim = await store.claimNextMessage(scopeKey, "tab-b", 31_001, 30_000, true);

    expect(firstClaim).toMatchObject({ leaseOwner: "tab-a", status: "sending" });
    expect(competingClaim).toBeNull();
    expect(recoveredClaim).toMatchObject({ attempts: 2, leaseOwner: "tab-b" });
  });

  it("does not automatically retry permanent errors but allows an explicit retry", async () => {
    const store = new MemoryOfflineRepository();
    await enqueueMessageOutbox(primaryScope, message("client-a"), undefined, store);
    await enqueueMessageOutbox(primaryScope, message("client-b"), undefined, store);
    const firstPass = vi.fn(async (entry: { clientMessageId: string }) => {
      if (entry.clientMessageId === "client-a") {
        throw { message: "Invalid body", status: 422 };
      }
      return { id: "server-b" };
    });

    const remaining = await flushMessageOutbox(primaryScope, { force: true, send: firstPass }, store);
    expect(firstPass).toHaveBeenCalledTimes(2);
    expect(remaining).toHaveLength(1);
    expect(remaining[0]).toMatchObject({ retryable: false, status: "failed" });

    await flushMessageOutbox(primaryScope, { force: true, send: vi.fn(async () => ({ id: "ignored" })) }, store);
    expect(await listMessageOutbox(primaryScope, store)).toHaveLength(1);

    await retryMessageOutbox(primaryScope, remaining[0].id, store);
    const manualSend = vi.fn(async () => ({ id: "server-a" }));
    await flushMessageOutbox(primaryScope, { force: true, send: manualSend }, store);
    expect(manualSend).toHaveBeenCalledTimes(1);
    expect(await listMessageOutbox(primaryScope, store)).toEqual([]);
  });

  it("clears all outbox/checkpoint data for one account without touching another", async () => {
    const store = new MemoryOfflineRepository();
    const secondWorkspace = { ...primaryScope, workspaceId: "workspace-2" };
    const otherAccount = { ...primaryScope, userId: "user-2" };
    await enqueueMessageOutbox(primaryScope, message("client-a"), undefined, store);
    await enqueueMessageOutbox(secondWorkspace, message("client-b"), undefined, store);
    await enqueueMessageOutbox(otherAccount, message("client-c"), undefined, store);
    await writeSyncCheckpoint(primaryScope, "cursor-1", ["event-1"], store);

    await clearOfflineAccount(primaryScope, store);

    expect(await listMessageOutbox(primaryScope, store)).toEqual([]);
    expect(await listMessageOutbox(secondWorkspace, store)).toEqual([]);
    expect(await readSyncCheckpoint(primaryScope, store)).toBeNull();
    expect((await listMessageOutbox(otherAccount, store)).map((entry) => entry.clientMessageId)).toEqual(["client-c"]);
  });

  it("uses capped exponential backoff with bounded jitter", () => {
    expect(outboxBackoffMs(1, () => 0)).toBe(750);
    expect(outboxBackoffMs(1, () => 1)).toBe(1_250);
    expect(outboxBackoffMs(20, () => 0.5)).toBe(60_000);
    expect(outboxBackoffMs(20, () => 1)).toBe(60_000);
  });
});

function message(clientMessageId: string, body = clientMessageId) {
  return {
    body,
    channelId: "channel-1",
    clientMessageId
  };
}
