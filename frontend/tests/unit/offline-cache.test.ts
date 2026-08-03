import { describe, expect, it } from "vitest";
import type { PlatformStorage } from "@webtui/chat-core";
import type { Message } from "@webtui/types";
import {
  discardLegacyOutbox,
  readDraft,
  readTimelineCache,
  writeDraft,
  writeTimelineCache
} from "../../apps/web/src/features/chat/model/offline-cache";

function createMemoryStorage(): PlatformStorage & { values: Map<string, string> } {
  const values = new Map<string, string>();
  return {
    values,
    getItem: (key) => values.get(key) ?? null,
    removeItem: (key) => {
      values.delete(key);
    },
    setItem: (key, value) => {
      values.set(key, value);
    }
  };
}

function message(index: number): Message {
  return {
    body: `message ${index}`,
    channel_id: "channel-1",
    created_at: new Date(Date.UTC(2026, 6, 14, 8, index, 0)).toISOString(),
    id: `msg-${index}`,
    workspace_id: "workspace-1"
  };
}

describe("offline cache", () => {
  it("stores a compact timeline without local optimistic messages", async () => {
    const storage = createMemoryStorage();
    const messages = Array.from({ length: 205 }, (_, index) => message(index));
    await writeTimelineCache("workspace-1", "channel-1", [{ ...message(999), id: "local-temp" }, ...messages], storage);

    const cached = await readTimelineCache("workspace-1", "channel-1", storage);

    expect(cached).toHaveLength(200);
    expect(cached[0].id).toBe("msg-5");
    expect(cached.some((item) => item.id.startsWith("local-"))).toBe(false);
  });

  it("persists and clears per-conversation drafts", async () => {
    const storage = createMemoryStorage();
    await writeDraft("workspace-1", "channel-1", "dang soan", storage);
    expect(await readDraft("workspace-1", "channel-1", storage)).toBe("dang soan");

    await writeDraft("workspace-1", "channel-1", "", storage);
    expect(await readDraft("workspace-1", "channel-1", storage)).toBe("");
  });

  it("discards the unscoped legacy outbox instead of migrating it", async () => {
    const storage = createMemoryStorage();
    await storage.setItem("webtui:offline:v1:outbox", JSON.stringify({ value: [{ body: "private" }] }));

    await discardLegacyOutbox(storage);

    expect(await storage.getItem("webtui:offline:v1:outbox")).toBeNull();
  });
});
