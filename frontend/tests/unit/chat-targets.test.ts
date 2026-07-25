import { describe, expect, it } from "vitest";
import { buildChatTargets } from "../../apps/web/src/features/chat/model/chat-targets";

describe("chat target helpers", () => {
  it("deduplicates direct channels returned by both channel APIs", () => {
    const targets = buildChatTargets(
      [
        { id: "channel-1", isMember: true, name: "Kỹ thuật", type: "public" },
        { id: "direct-1", isMember: true, name: "Direct", type: "direct" }
      ],
      [
        {
          id: "direct-1",
          user: { id: "user-2", name: "Nguyễn Văn B", status: "online" }
        }
      ]
    );

    expect(targets).toEqual([
      { id: "channel-1", name: "Kỹ thuật" },
      { id: "direct-1", name: "Chat: Nguyễn Văn B" }
    ]);
    expect(new Set(targets.map((target) => target.id)).size).toBe(targets.length);
  });
});
