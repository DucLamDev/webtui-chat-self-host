import type { InfiniteData, QueryClient } from "@tanstack/react-query";
import { queryKeys, type MessagePage } from "@webtui/api-client";
import type { Message as ApiMessage } from "@webtui/types";

export function mergeMessageIntoTimeline(
  queryClient: QueryClient,
  workspaceId: string,
  channelId: string,
  message: ApiMessage,
  replaceId?: string
) {
  queryClient.setQueryData<InfiniteData<MessagePage>>(messageTimelineKey(workspaceId, channelId), (current) =>
    upsertMessageInPages(current, message, replaceId)
  );
}

export function removeMessageFromTimeline(
  queryClient: QueryClient,
  workspaceId: string,
  channelId: string,
  messageId: string
) {
  queryClient.setQueryData<InfiniteData<MessagePage>>(messageTimelineKey(workspaceId, channelId), (current) =>
    removeMessageFromPages(current, messageId)
  );
}

export function messageRoomName(workspaceId: string, channelId: string) {
  return `workspace:${workspaceId}:channel:${channelId}`;
}

export function messageTimelineKey(workspaceId: string, channelId: string) {
  return channelId ? queryKeys.messages.channel(workspaceId, channelId) : (["messages", workspaceId, "none"] as const);
}

export function upsertMessageInPages(
  current: InfiniteData<MessagePage> | undefined,
  message: ApiMessage,
  replaceId?: string
): InfiniteData<MessagePage> {
  const emptyPage: MessagePage = { messages: [], meta: {} };
  const next = current ?? { pageParams: [undefined], pages: [emptyPage] };
  const shouldReplace = next.pages.some((page) =>
    page.messages.some((item) => item.id === message.id || (replaceId && item.id === replaceId))
  );

  const pages = next.pages.map((page, pageIndex) => {
    const messages = page.messages.map((item) => {
      if (item.id === message.id || (replaceId && item.id === replaceId)) {
        return message;
      }
      return item;
    });

    if (!shouldReplace && pageIndex === 0) {
      return {
        ...page,
        messages: [message, ...messages]
      };
    }

    return {
      ...page,
      messages
    };
  });

  return {
    ...next,
    pages: pages.map((page) => ({
      ...page,
      messages: uniqueMessages(page.messages)
    }))
  };
}

export function updateMessageInPages(
  current: InfiniteData<MessagePage> | undefined,
  messageId: string,
  update: (message: ApiMessage) => ApiMessage
): InfiniteData<MessagePage> | undefined {
  if (!current) {
    return current;
  }

  return {
    ...current,
    pages: current.pages.map((page) => ({
      ...page,
      messages: page.messages.map((message) => (message.id === messageId ? update(message) : message))
    }))
  };
}

export function removeMessageFromPages(
  current: InfiniteData<MessagePage> | undefined,
  messageId: string
): InfiniteData<MessagePage> | undefined {
  if (!current) {
    return current;
  }

  return {
    ...current,
    pages: current.pages.map((page) => ({
      ...page,
      messages: page.messages.filter((message) => message.id !== messageId)
    }))
  };
}

export function uniqueMessages(messages: ApiMessage[]): ApiMessage[] {
  const seen = new Set<string>();
  const result: ApiMessage[] = [];

  for (const message of messages) {
    if (seen.has(message.id)) {
      continue;
    }
    seen.add(message.id);
    result.push(message);
  }

  return result;
}

export function sortMessagesAscending(messages: ApiMessage[]): ApiMessage[] {
  return [...messages].sort((left, right) => {
    if (triggerMessageId(left) === right.id) {
      return 1;
    }
    if (triggerMessageId(right) === left.id) {
      return -1;
    }

    const leftTime = Date.parse(left.created_at || left.sent_at || "");
    const rightTime = Date.parse(right.created_at || right.sent_at || "");
    if (Number.isFinite(leftTime) && Number.isFinite(rightTime)) {
      return leftTime - rightTime;
    }
    if (Number.isFinite(leftTime)) {
      return -1;
    }
    if (Number.isFinite(rightTime)) {
      return 1;
    }
    return 0;
  });
}

function triggerMessageId(message: ApiMessage): string {
  const value = message.metadata?.trigger_message_id;
  return typeof value === "string" ? value.trim() : "";
}
