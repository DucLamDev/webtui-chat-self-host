import type { InfiniteData } from "@tanstack/react-query";
import type { MessagePage } from "@webtui/api-client";
import type { Message } from "@webtui/types";
import { describe, expect, it } from "vitest";
import {
  messageRoomName,
  messageTimelineKey,
  removeMessageFromPages,
  sortMessagesAscending,
  uniqueMessages,
  updateMessageInPages,
  upsertMessageInPages
} from "../../apps/web/src/features/chat/model/message-cache";

function message(id: string, createdAt: string, body = id): Message {
  return {
    body,
    channel_id: "channel-1",
    created_at: createdAt,
    id,
    workspace_id: "workspace-1"
  };
}

function timeline(messages: Message[]): InfiniteData<MessagePage> {
  return timelinePages([messages]);
}

function timelinePages(pages: Message[][]): InfiniteData<MessagePage> {
  return {
    pageParams: pages.map((_, index) => (index === 0 ? undefined : `cursor-${index}`)),
    pages: pages.map((messages) => ({
      messages,
      meta: {}
    }))
  };
}

describe("message cache helpers", () => {
  it("builds stable room and query keys", () => {
    expect(messageRoomName("workspace-1", "channel-1")).toBe("workspace:workspace-1:channel:channel-1");
    expect(messageTimelineKey("workspace-1", "channel-1")).toEqual(["messages", "workspace-1", "channel-1"]);
    expect(messageTimelineKey("workspace-1", "")).toEqual(["messages", "workspace-1", "none"]);
  });

  it("deduplicates messages by id and keeps the first item", () => {
    const first = message("msg-1", "2026-07-09T01:00:00Z", "Bản đầu");
    const duplicate = message("msg-1", "2026-07-09T02:00:00Z", "Bản trùng");

    expect(uniqueMessages([first, duplicate, message("msg-2", "2026-07-09T03:00:00Z")])).toEqual([
      first,
      message("msg-2", "2026-07-09T03:00:00Z")
    ]);
  });

  it("sorts messages ascending by created or sent time", () => {
    const result = sortMessagesAscending([
      message("msg-3", "2026-07-09T03:00:00Z"),
      message("msg-1", "2026-07-09T01:00:00Z"),
      { ...message("msg-2", ""), sent_at: "2026-07-09T02:00:00Z" }
    ]);

    expect(result.map((item) => item.id)).toEqual(["msg-1", "msg-2", "msg-3"]);
  });

  it("keeps a bot response after its trigger when timestamps are equal", () => {
    const trigger = message("msg-user", "2026-07-12T12:09:52Z");
    const botResponse = {
      ...message("msg-bot", "2026-07-12T12:09:52Z"),
      kind: "bot",
      metadata: { trigger_message_id: trigger.id }
    };

    const result = sortMessagesAscending([botResponse, trigger]);

    expect(result.map((item) => item.id)).toEqual(["msg-user", "msg-bot"]);
  });

  it("puts messages with valid timestamps before malformed timestamps", () => {
    const result = sortMessagesAscending([
      message("msg-invalid", "not-a-date"),
      message("msg-valid", "2026-07-12T12:09:52.493177Z")
    ]);

    expect(result.map((item) => item.id)).toEqual(["msg-valid", "msg-invalid"]);
  });

  it("inserts new messages into the first page", () => {
    const current = timeline([message("msg-1", "2026-07-09T01:00:00Z")]);
    const next = upsertMessageInPages(current, message("msg-2", "2026-07-09T02:00:00Z"));

    expect(next.pages[0].messages.map((item) => item.id)).toEqual(["msg-2", "msg-1"]);
  });

  it("replaces optimistic messages with the server message", () => {
    const current = timeline([message("local-1", "2026-07-09T01:00:00Z", "Đang gửi")]);
    const next = upsertMessageInPages(current, message("msg-1", "2026-07-09T01:00:01Z", "Đã gửi"), "local-1");

    expect(next.pages[0].messages).toEqual([message("msg-1", "2026-07-09T01:00:01Z", "Đã gửi")]);
  });

  it("updates realtime messages in older pages without duplicating them", () => {
    const current = timelinePages([
      [message("msg-3", "2026-07-09T03:00:00Z")],
      [message("msg-2", "2026-07-09T02:00:00Z")]
    ]);
    const pinned = { ...message("msg-2", "2026-07-09T02:00:00Z", "Đã ghim"), pinned_at: "2026-07-09T04:00:00Z" };

    const next = upsertMessageInPages(current, pinned);

    expect(next.pages[0].messages.map((item) => item.id)).toEqual(["msg-3"]);
    expect(next.pages[1].messages).toEqual([pinned]);
  });

  it("creates an initial page when realtime arrives before history", () => {
    const next = upsertMessageInPages(undefined, message("msg-1", "2026-07-09T01:00:00Z"));

    expect(next.pageParams).toEqual([undefined]);
    expect(next.pages[0].messages.map((item) => item.id)).toEqual(["msg-1"]);
  });

  it("updates and removes messages without changing other pages", () => {
    const current = timeline([
      message("msg-1", "2026-07-09T01:00:00Z"),
      message("msg-2", "2026-07-09T02:00:00Z")
    ]);

    const updated = updateMessageInPages(current, "msg-1", (item) => ({ ...item, body: "Đã sửa" }));
    expect(updated?.pages[0].messages[0].body).toBe("Đã sửa");

    const removed = removeMessageFromPages(updated, "msg-2");
    expect(removed?.pages[0].messages.map((item) => item.id)).toEqual(["msg-1"]);
  });
});
