import { describe, expect, it } from "vitest";
import { QueryClient, type InfiniteData } from "@tanstack/react-query";
import type { MessagePage } from "@webtui/api-client";
import type { Message } from "@webtui/types";
import {
  chooseAuthoritativeMessage,
  mergeMessageIntoTimeline,
  messageTimelineKey,
  removeMessageFromTimeline,
  upsertMessageInPages
} from "../../apps/web/src/features/chat/model/message-cache";

describe("message cache conflict policy", () => {
  it("replaces an optimistic message with the canonical server id using client_message_id", () => {
    const optimistic = apiMessage("local-client-1", "2026-08-03T10:00:00Z", "client-1");
    const canonical = apiMessage("server-1", "2026-08-03T10:00:01Z", "client-1");
    const current: InfiniteData<MessagePage> = {
      pageParams: [undefined],
      pages: [{ messages: [optimistic], meta: {} }]
    };

    const result = upsertMessageInPages(current, canonical);

    expect(result.pages[0].messages).toEqual([canonical]);
  });

  it("does not let a late, older REST/WS result overwrite a newer server revision", () => {
    const newer = apiMessage("server-1", "2026-08-03T10:00:05Z", "client-1");
    const older = apiMessage("server-1", "2026-08-03T10:00:01Z", "client-1");
    const current: InfiniteData<MessagePage> = {
      pageParams: [undefined],
      pages: [{ messages: [newer], meta: {} }]
    };

    expect(upsertMessageInPages(current, older).pages[0].messages[0]).toEqual(newer);
    expect(chooseAuthoritativeMessage(newer, older)).toEqual(newer);
  });

  it("keeps a late pre-delete response from resurrecting a removed message", () => {
    const queryClient = new QueryClient();
    const message = apiMessage("server-delete-test", "2026-08-03T10:00:01Z", "client-delete-test");
    mergeMessageIntoTimeline(queryClient, "workspace-1", "channel-1", message);
    removeMessageFromTimeline(
      queryClient,
      "workspace-1",
      "channel-1",
      message.id,
      "2026-08-03T10:00:05Z"
    );

    mergeMessageIntoTimeline(queryClient, "workspace-1", "channel-1", message);

    const timeline = queryClient.getQueryData<InfiniteData<MessagePage>>(
      messageTimelineKey("workspace-1", "channel-1")
    );
    expect(timeline?.pages[0].messages).toEqual([]);
  });
});

function apiMessage(id: string, updatedAt: string, clientMessageId: string): Message {
  return {
    body: id,
    channel_id: "channel-1",
    created_at: "2026-08-03T10:00:00Z",
    id,
    kind: "text",
    metadata: { client_message_id: clientMessageId },
    updated_at: updatedAt,
    workspace_id: "workspace-1"
  } as Message;
}
