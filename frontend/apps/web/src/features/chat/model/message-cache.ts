import type { InfiniteData, QueryClient } from "@tanstack/react-query";
import { queryKeys, type MessagePage } from "@webtui/api-client";
import type { Message as ApiMessage } from "@webtui/types";

const deletionTombstones = new Map<string, number>();
const maxDeletionTombstones = 1_000;

export function mergeMessageIntoTimeline(
  queryClient: QueryClient,
  workspaceId: string,
  channelId: string,
  message: ApiMessage,
  replaceId?: string
) {
  const tombstoneKey = messageTombstoneKey(workspaceId, channelId, message.id);
  const deletedAt = deletionTombstones.get(tombstoneKey);
  if (deletedAt !== undefined && messageRevision(message) <= deletedAt) {
    return;
  }
  if (deletedAt !== undefined) {
    deletionTombstones.delete(tombstoneKey);
  }
  queryClient.setQueryData<InfiniteData<MessagePage>>(messageTimelineKey(workspaceId, channelId), (current) =>
    upsertMessageInPages(current, message, replaceId)
  );
}

export function removeMessageFromTimeline(
  queryClient: QueryClient,
  workspaceId: string,
  channelId: string,
  messageId: string,
  deletedAt?: string
) {
  if (deletedAt) {
    rememberMessageDeletion(workspaceId, channelId, messageId, deletedAt);
  }
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
  const clientMessageId = messageClientId(message);
  const isMatch = (item: ApiMessage) =>
    item.id === message.id ||
    Boolean(replaceId && item.id === replaceId) ||
    Boolean(clientMessageId && messageClientId(item) === clientMessageId);
  const matches = next.pages.flatMap((page) => page.messages.filter(isMatch));
  const authoritativeMessage = matches.reduce(
    (candidate, existing) => chooseAuthoritativeMessage(existing, candidate),
    message
  );
  let inserted = false;

  const pages = next.pages.map((page, pageIndex) => {
    const messages = page.messages.flatMap((item) => {
      if (isMatch(item)) {
        if (inserted) {
          return [];
        }
        inserted = true;
        return [authoritativeMessage];
      }
      return [item];
    });

    if (!matches.length && pageIndex === 0) {
      inserted = true;
      return {
        ...page,
        messages: [authoritativeMessage, ...messages]
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
  const result: ApiMessage[] = [];
  const indexById = new Map<string, number>();
  const indexByClientId = new Map<string, number>();

  for (const message of messages) {
    const clientMessageId = messageClientId(message);
    const existingIndex = indexById.get(message.id) ?? (clientMessageId ? indexByClientId.get(clientMessageId) : undefined);
    if (existingIndex !== undefined) {
      const selected = chooseAuthoritativeMessage(result[existingIndex], message);
      result[existingIndex] = selected;
      indexById.set(selected.id, existingIndex);
      const selectedClientId = messageClientId(selected);
      if (selectedClientId) {
        indexByClientId.set(selectedClientId, existingIndex);
      }
      continue;
    }
    indexById.set(message.id, result.length);
    if (clientMessageId) {
      indexByClientId.set(clientMessageId, result.length);
    }
    result.push(message);
  }

  return result;
}

export function messageClientId(message: ApiMessage): string {
  const value = message.metadata?.client_message_id;
  return typeof value === "string" ? value.trim() : "";
}

/**
 * REST and realtime can complete in either order. A canonical server id always
 * replaces a local optimistic id; between server records, the newest server
 * timestamp wins so a late response cannot roll back a newer edit.
 */
export function chooseAuthoritativeMessage(existing: ApiMessage, incoming: ApiMessage): ApiMessage {
  const existingLocal = existing.id.startsWith("local-");
  const incomingLocal = incoming.id.startsWith("local-");
  if (existingLocal !== incomingLocal) {
    return existingLocal ? incoming : existing;
  }
  const existingRevision = messageRevision(existing);
  const incomingRevision = messageRevision(incoming);
  return existingRevision > incomingRevision ? existing : incoming;
}

function messageRevision(message: ApiMessage): number {
  return [message.deleted_at, message.updated_at, message.edited_at, message.created_at, message.sent_at]
    .map((value) => Date.parse(value || ""))
    .filter(Number.isFinite)
    .reduce((latest, value) => Math.max(latest, value), 0);
}

function rememberMessageDeletion(workspaceId: string, channelId: string, messageId: string, deletedAt: string) {
  const timestamp = Date.parse(deletedAt);
  deletionTombstones.set(
    messageTombstoneKey(workspaceId, channelId, messageId),
    Number.isFinite(timestamp) ? timestamp : Date.now()
  );
  while (deletionTombstones.size > maxDeletionTombstones) {
    const oldestKey = deletionTombstones.keys().next().value as string | undefined;
    if (!oldestKey) {
      break;
    }
    deletionTombstones.delete(oldestKey);
  }
}

function messageTombstoneKey(workspaceId: string, channelId: string, messageId: string) {
  return JSON.stringify([workspaceId, channelId, messageId]);
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
