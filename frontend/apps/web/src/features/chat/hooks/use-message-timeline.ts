"use client";

import { useEffect, useMemo, useState } from "react";
import {
  useInfiniteQuery,
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
  type InfiniteData
} from "@tanstack/react-query";
import { queryKeys, type MessagePage } from "@webtui/api-client";
import type {
  AuthUser,
  FileAttachment,
  Message as ApiMessage,
  MessageAttachment,
  MessageAuthor
} from "@webtui/types";
import { api } from "@/lib/api";
import type { ChatMessage, ChatUser, MessageAttachmentItem, MessageCallEvent, MessageReplyPreview } from "../model/types";
import {
  mergeMessageIntoTimeline,
  messageTimelineKey,
  removeMessageFromTimeline,
  sortMessagesAscending,
  uniqueMessages,
  updateMessageInPages
} from "../model/message-cache";
import { isLikelyOfflineError, readTimelineCache, writeTimelineCache } from "../model/offline-cache";

export {
  mergeMessageIntoTimeline,
  messageRoomName,
  messageTimelineKey,
  removeMessageFromTimeline
} from "../model/message-cache";

const timelineLimit = 50;
type MessageTimelineQueryKey = ReturnType<typeof messageTimelineKey>;

const uuidLikePattern = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/;

export type MessageTimelineOptions = {
  canManageMessages: boolean;
  channelId: string;
  currentUser: ChatUser;
  enabled?: boolean;
  searchQuery?: string;
  searchFilters?: MessageSearchFilters;
  threadMessageId?: string;
  workspaceId: string;
};

export type MessageSearchFilters = {
  channelId?: string;
  dateFrom?: string;
  dateTo?: string;
  kind?: string;
  senderId?: string;
};

export type EditMessagePayload = {
  body: string;
  messageId: string;
};

export type DeleteMessagePayload = {
  messageId: string;
};

export type ForwardMessagePayload = {
  messageId: string;
  targetChannelId: string;
};

export type ToggleReactionPayload = {
  emoji: string;
  messageId: string;
  reactedByMe?: boolean;
};

export function useMessageTimeline({
  canManageMessages,
  channelId,
  currentUser,
  enabled = true,
  searchQuery = "",
  searchFilters = {},
  threadMessageId,
  workspaceId
}: MessageTimelineOptions) {
  const queryClient = useQueryClient();
  const timelineKey = messageTimelineKey(workspaceId, channelId);
  const timelineCacheKey = `${workspaceId}:${channelId}`;
  const cleanSearchQuery = searchQuery.trim();
  const searchFilterKey = JSON.stringify(searchFilters);
  const [cachedTimeline, setCachedTimeline] = useState<{ key: string; messages: ApiMessage[] }>({ key: "", messages: [] });
  const cachedApiMessages = cachedTimeline.key === timelineCacheKey ? cachedTimeline.messages : [];

  const messagesQuery = useInfiniteQuery<
    MessagePage,
    Error,
    InfiniteData<MessagePage>,
    MessageTimelineQueryKey,
    string | undefined
  >({
    enabled: Boolean(enabled && workspaceId && channelId),
    getNextPageParam: (lastPage) =>
      lastPage.meta.has_more ? lastPage.meta.next_cursor ?? lastPage.messages.at(-1)?.id : undefined,
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api.messages.listPage(workspaceId, channelId, {
        before: typeof pageParam === "string" ? pageParam : undefined,
        limit: timelineLimit
      }),
    queryKey: timelineKey
  });

  const remoteApiMessages = useMemo(
    () => sortMessagesAscending(uniqueMessages(messagesQuery.data?.pages.flatMap((page) => page.messages) ?? [])),
    [messagesQuery.data?.pages]
  );
  const isUsingCachedMessages = !messagesQuery.isSuccess && remoteApiMessages.length === 0 && cachedApiMessages.length > 0;
  const apiMessages = useMemo(
    () => sortMessagesAscending(uniqueMessages(isUsingCachedMessages ? cachedApiMessages : remoteApiMessages)),
    [cachedApiMessages, isUsingCachedMessages, remoteApiMessages]
  );
  const offlineReadMode = Boolean(
    isUsingCachedMessages &&
      (messagesQuery.isError || (typeof navigator !== "undefined" && navigator.onLine === false))
  );

  useEffect(() => {
    let disposed = false;
    if (!workspaceId || !channelId) {
      setCachedTimeline({ key: "", messages: [] });
      return undefined;
    }
    setCachedTimeline({ key: timelineCacheKey, messages: [] });
    void readTimelineCache(workspaceId, channelId)
      .then((messages) => {
        if (!disposed) {
          setCachedTimeline({ key: timelineCacheKey, messages });
        }
      })
      .catch(() => undefined);
    return () => {
      disposed = true;
    };
  }, [channelId, timelineCacheKey, workspaceId]);

  useEffect(() => {
    if (!workspaceId || !channelId || !messagesQuery.isSuccess) {
      return;
    }
    void writeTimelineCache(workspaceId, channelId, remoteApiMessages).catch(() => undefined);
  }, [channelId, messagesQuery.isSuccess, remoteApiMessages, workspaceId]);
  const attachmentMessageIds = useMemo(
    () => apiMessages.filter((message) => !message.id.startsWith("local-") && !message.deleted_at).map((message) => message.id),
    [apiMessages]
  );
  const attachmentQueries = useQueries({
    queries: attachmentMessageIds.map((messageId) => {
      const message = apiMessages.find((item) => item.id === messageId);
      const attachmentQueryKey = queryKeys.files.attachments(workspaceId, channelId, messageId);
      const cachedAttachments = queryClient.getQueryData<MessageAttachment[]>(attachmentQueryKey);
      const expectsAttachment =
        message?.kind === "file" ||
        message?.metadata?.has_attachments === true ||
        Boolean(cachedAttachments?.length) ||
        /đã gửi(?: \d+)? (?:ảnh|file|tin nhắn thoại)/i.test(message?.body ?? "");
      const attachmentWaitDeadline = Date.parse(message?.created_at ?? "") + 30_000;
      return {
        enabled: Boolean(enabled && workspaceId && channelId && expectsAttachment),
        gcTime: 30 * 60_000,
        queryFn: async () => {
          try {
            const attachments = await api.files.attachments(workspaceId, channelId, messageId);
            const mappedAttachments = attachments.map(mapFileAttachmentToMessageAttachment);
            return mappedAttachments.length ? mappedAttachments : cachedAttachments ?? [];
          } catch {
            return cachedAttachments ?? [];
          }
        },
        queryKey: attachmentQueryKey,
        refetchInterval: (query: { state: { data?: MessageAttachment[] } }) =>
          expectsAttachment && !query.state.data?.length && Date.now() < attachmentWaitDeadline ? 2_000 : false,
        staleTime: expectsAttachment ? 2_000 : Infinity
      };
    })
  });
  const attachmentsByMessageId = useMemo(
    () => new Map(attachmentMessageIds.map((messageId, index) => [messageId, attachmentQueries[index]?.data ?? []])),
    [attachmentMessageIds, attachmentQueries]
  );
  const messages = useMemo(
    () =>
      apiMessages.map((message) =>
        mapMessage(withLoadedAttachments(message, attachmentsByMessageId.get(message.id)), currentUser, canManageMessages)
      ),
    [apiMessages, attachmentsByMessageId, canManageMessages, currentUser]
  );
  const pinnedMessagesQuery = useQuery({
    enabled: Boolean(enabled && workspaceId && channelId),
    queryFn: () => api.messages.pins(workspaceId, channelId),
    queryKey: queryKeys.messages.pins(workspaceId, channelId),
    staleTime: 10_000
  });
  const pinnedMessages = useMemo(
    () =>
      sortMessagesAscending(uniqueMessages(pinnedMessagesQuery.data ?? [])).map((message) =>
        mapMessage(message, currentUser, canManageMessages)
      ),
    [canManageMessages, currentUser, pinnedMessagesQuery.data]
  );

  const threadQuery = useQuery({
    enabled: Boolean(enabled && workspaceId && channelId && threadMessageId),
    queryFn: () => api.messages.threadPage(workspaceId, channelId, threadMessageId ?? "", { limit: 50 }),
    queryKey: threadMessageId
      ? queryKeys.messages.thread(workspaceId, channelId, threadMessageId)
      : ["messages", workspaceId, channelId, "thread", "none"]
  });
  const threadMessages = useMemo(
    () =>
      sortMessagesAscending(uniqueMessages(threadQuery.data?.messages ?? [])).map((message) =>
        mapMessage(message, currentUser, canManageMessages)
      ),
    [canManageMessages, currentUser, threadQuery.data?.messages]
  );
  const sendThreadMessageMutation = useMutation({
    mutationFn: (body: string) => {
      if (!threadMessageId) {
        throw new Error("Chưa chọn luồng trả lời.");
      }
      return api.messages.send(workspaceId, channelId, {
        body: body.trim(),
        kind: "text",
        parent_id: threadMessageId
      });
    },
    onSuccess: (message) => {
      mergeMessageIntoTimeline(queryClient, workspaceId, channelId, message);
      if (threadMessageId) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.messages.thread(workspaceId, channelId, threadMessageId) });
      }
    }
  });

  const searchQueryResult = useQuery({
    enabled: Boolean(enabled && workspaceId && cleanSearchQuery.length >= 2),
    queryFn: () => api.messages.searchPage(workspaceId, {
      channel_id: searchFilters.channelId || undefined,
      date_from: searchFilters.dateFrom || undefined,
      date_to: searchFilters.dateTo || undefined,
      kind: searchFilters.kind || undefined,
      limit: 20,
      q: cleanSearchQuery,
      sender_id: searchFilters.senderId || undefined
    }),
    queryKey: queryKeys.messages.search(workspaceId, cleanSearchQuery, searchFilterKey)
  });
  const searchResults = useMemo(
    () =>
      sortMessagesAscending(uniqueMessages(searchQueryResult.data?.messages ?? [])).map((message) =>
        mapMessage(message, currentUser, canManageMessages)
      ),
    [canManageMessages, currentUser, searchQueryResult.data?.messages]
  );

  const editMessageMutation = useMutation({
    mutationFn: (input: EditMessagePayload) => api.messages.update(workspaceId, channelId, input.messageId, { body: input.body }),
    onMutate: async (input) => {
      await queryClient.cancelQueries({ queryKey: timelineKey });
      const previous = queryClient.getQueryData<InfiniteData<MessagePage>>(timelineKey);

      queryClient.setQueryData<InfiniteData<MessagePage>>(timelineKey, (current) =>
        updateMessageInPages(current, input.messageId, (message) => ({
          ...message,
          body: input.body,
          edited_at: new Date().toISOString(),
          updated_at: new Date().toISOString()
        }))
      );

      return { previous };
    },
    onError: (_error, _input, context) => {
      if (context?.previous) {
        queryClient.setQueryData(timelineKey, context.previous);
      }
    },
    onSuccess: (message) => {
      mergeMessageIntoTimeline(queryClient, workspaceId, channelId, message);
    }
  });

  const forwardMessageMutation = useMutation({
    mutationFn: (input: ForwardMessagePayload) =>
      api.messages.forward(workspaceId, channelId, input.messageId, { target_channel_id: input.targetChannelId }),
    onSuccess: (message, input) => {
      mergeMessageIntoTimeline(queryClient, workspaceId, input.targetChannelId, message);
      void queryClient.invalidateQueries({ queryKey: queryKeys.files.messageAttachments(workspaceId, input.targetChannelId) });
    }
  });

  const deleteMessageMutation = useMutation({
    mutationFn: (input: DeleteMessagePayload) => api.messages.delete(workspaceId, channelId, input.messageId),
    onMutate: async (input) => {
      await queryClient.cancelQueries({ queryKey: timelineKey });
      const previous = queryClient.getQueryData<InfiniteData<MessagePage>>(timelineKey);
      removeMessageFromTimeline(queryClient, workspaceId, channelId, input.messageId);
      return { previous };
    },
    onError: (_error, _input, context) => {
      if (context?.previous) {
        queryClient.setQueryData(timelineKey, context.previous);
      }
    }
  });

  const toggleReactionMutation = useMutation({
    mutationFn: async (input: ToggleReactionPayload) => {
      if (input.reactedByMe) {
        await api.messages.removeReaction(workspaceId, channelId, input.messageId, input.emoji);
        return null;
      }

      return api.messages.addReaction(workspaceId, channelId, input.messageId, { emoji: input.emoji });
    },
    onSuccess: (message) => {
      if (message) {
        mergeMessageIntoTimeline(queryClient, workspaceId, channelId, message);
      }
    }
  });

  const pinMessageMutation = useMutation({
    mutationFn: (messageId: string) => api.messages.pin(workspaceId, channelId, messageId),
    onSuccess: (message) => {
      mergeMessageIntoTimeline(queryClient, workspaceId, channelId, message);
      void queryClient.invalidateQueries({ queryKey: queryKeys.messages.pins(workspaceId, channelId) });
    }
  });

  const unpinMessageMutation = useMutation({
    mutationFn: (messageId: string) => api.messages.unpin(workspaceId, channelId, messageId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.messages.pins(workspaceId, channelId) });
    }
  });

  return {
    deleteMessageMutation,
    editMessageMutation,
    forwardMessageMutation,
    hasOlderMessages: Boolean(messagesQuery.hasNextPage),
    isLoadingOlderMessages: messagesQuery.isFetchingNextPage,
    isOfflineReadMode: offlineReadMode || (messagesQuery.isError && isLikelyOfflineError(messagesQuery.error)),
    loadOlderMessages: () => messagesQuery.fetchNextPage(),
    messages,
    messagesQuery,
    pinnedMessages,
    pinnedMessagesQuery,
    pinMessageMutation,
    searchQuery: searchQueryResult,
    searchResults,
    sendThreadMessageMutation,
    threadMessages,
    threadQuery,
    toggleReactionMutation,
    unpinMessageMutation
  };
}

export function mapAuthUser(user: AuthUser | null): ChatUser {
  return {
    avatarUrl: user?.avatar_url ?? undefined,
    email: user?.email,
    id: user?.id ?? "current-user",
    name: displayName(user),
    phoneNumber: user?.phone_number ?? undefined,
    username: user?.username,
    status: "online"
  };
}

function withLoadedAttachments(message: ApiMessage, attachments?: MessageAttachment[]): ApiMessage {
  if (!attachments?.length) {
    return message;
  }

  return {
    ...message,
    attachments: message.attachments?.length ? message.attachments : attachments
  };
}

function mapFileAttachmentToMessageAttachment(attachment: FileAttachment): MessageAttachment {
  return {
    byte_size: attachment.file.byte_size,
    file: attachment.file,
    file_id: attachment.file_id,
    id: `${attachment.message_id}-${attachment.file_id}`,
    mime_type: attachment.file.mime_type,
    name: attachment.file.name ?? attachment.file.file_name ?? attachment.file.original_name,
    original_name: attachment.file.original_name,
    size: attachment.file.size,
    size_bytes: attachment.file.size_bytes,
    url: attachment.file.url ?? attachment.file.download_url
  };
}

export function mapMessage(
  message: ApiMessage,
  fallbackAuthor: ChatUser,
  canManageMessages = false
): ChatMessage {
  const botAuthor = mapBotMessageAuthor(message);
  const systemTone = resolveSystemMessageTone(message);
  const systemAuthor = systemTone
    ? {
        id: systemTone === "announcement" ? "system:announcement" : "system",
        name: systemTone === "announcement" ? "Thong bao" : "He thong",
        status: "offline" as const
      }
    : null;
  const author =
    systemAuthor ?? botAuthor ?? mapMessageAuthor(message.author ?? message.user, fallbackAuthor, message.sender_id ?? message.author_id);
  const senderId = systemAuthor?.id ?? botAuthor?.id ?? message.sender_id ?? message.author_id ?? author.id;
  const isOwner = !systemAuthor && !botAuthor && (senderId === fallbackAuthor.id || author.id === fallbackAuthor.id);
  const attachments = mapMessageAttachments(message.attachments);
  const qrImageUrl = botAuthor ? messageQRImageURL(message) : undefined;
  const replyTo = mapReplyPreview(message.metadata, message.parent_id ?? undefined);
  const isLocal = message.id.startsWith("local-");
  const canTargetMessageAPI = uuidLikePattern.test(message.id);
  const callEvent = resolveCallEvent(message, fallbackAuthor.id);
  const isVoice = message.metadata?.message_type === "voice"
    || (message.kind === "file" && /^Đã gửi(?: \d+)? tin nhắn thoại$/i.test(message.body));

  return {
    attachmentName: attachments[0]?.name,
    attachments,
    author,
    body: message.deleted_at ? "Tin nhắn đã bị xóa." : message.body,
    canDelete: !callEvent && !systemAuthor && !message.deleted_at && !isLocal && canTargetMessageAPI && (isOwner || canManageMessages),
    canEdit: !callEvent && !systemAuthor && !message.deleted_at && isOwner,
    callEvent,
    editedAt: message.edited_at ? formatTime(message.edited_at) : undefined,
    id: message.id,
    isDeleted: Boolean(message.deleted_at),
    isForwarded: Boolean(message.metadata && typeof message.metadata === "object" && message.metadata.forwarded_from),
    isBot: Boolean(botAuthor),
    isMine: isOwner,
    isLocal,
    isPending: isLocal,
    isSystem: Boolean(systemTone),
    isVoice,
    rawChannelId: message.channel_id,
    rawCreatedAt: message.created_at ?? message.sent_at,
    rawSenderId: senderId,
    parentId: message.parent_id ?? undefined,
    threadRootId: message.thread_root_id ?? undefined,
    replyTo,
    qrImageUrl,
    qrReference: botAuthor && typeof message.metadata?.reference === "string" ? message.metadata.reference.trim() : undefined,
    reactions: systemTone || callEvent ? undefined : message.reactions?.map((reaction) => ({
      count: reaction.count ?? reaction.user_ids?.length ?? 0,
      emoji: reaction.emoji,
      reactedByMe: reaction.reacted_by_me
    })),
    sentAt: formatTime(message.created_at ?? message.sent_at),
    systemTone
  };
}

function resolveCallEvent(message: ApiMessage, currentUserId: string): MessageCallEvent | undefined {
  const metadata = message.metadata;
  const messageType = typeof metadata?.message_type === "string" ? metadata.message_type.trim().toLowerCase() : "";
  if (message.kind !== "event" || messageType !== "call") {
    return undefined;
  }

  const statusValue = typeof metadata?.call_status === "string" ? metadata.call_status.trim().toLowerCase() : "";
  const modeValue = typeof metadata?.call_mode === "string" ? metadata.call_mode.trim().toLowerCase() : "";
  const initiatorUserId = typeof metadata?.initiator_user_id === "string" ? metadata.initiator_user_id.trim() : "";
  const durationValue = typeof metadata?.duration_seconds === "number" ? metadata.duration_seconds : undefined;

  return {
    direction: initiatorUserId && initiatorUserId === currentUserId ? "outgoing" : "incoming",
    durationSeconds: typeof durationValue === "number" && Number.isFinite(durationValue) ? Math.max(0, Math.round(durationValue)) : undefined,
    initiatorUserId: initiatorUserId || undefined,
    mode: modeValue === "video" ? "video" : "audio",
    status: statusValue === "completed" ? "completed" : "missed",
    targetUserId: typeof metadata?.target_user_id === "string" ? metadata.target_user_id.trim() || undefined : undefined
  };
}

function mapReplyPreview(metadata: ApiMessage["metadata"], parentId?: string): MessageReplyPreview | undefined {
  const raw = metadata?.reply_to ?? metadata?.replyTo;
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    return undefined;
  }

  const record = raw as Record<string, unknown>;
  const body = stringValue(record.body) || stringValue(record.text);
  if (!body) {
    return undefined;
  }

  return {
    authorName:
      stringValue(record.authorName) ||
      stringValue(record.author_name) ||
      stringValue(metadata?.reply_to_author_name) ||
      "Tin nhắn",
    body,
    messageId: stringValue(record.messageId) || stringValue(record.message_id) || parentId || ""
  };
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function resolveSystemMessageTone(message: ApiMessage): "announcement" | "system" | undefined {
  const kind = (message.kind ?? "").trim().toLowerCase();
  const metadataType = typeof message.metadata?.type === "string" ? message.metadata.type.trim().toLowerCase() : "";
  const metadataMessageType =
    typeof message.metadata?.message_type === "string" ? message.metadata.message_type.trim().toLowerCase() : "";
  const variants = new Set([kind, metadataType, metadataMessageType]);

  if (variants.has("announcement") || message.metadata?.announcement === true) {
    return "announcement";
  }
  if (variants.has("system") || variants.has("system_message")) {
    return "system";
  }
  return undefined;
}

function messageQRImageURL(message: ApiMessage): string | undefined {
  const metadataURL = typeof message.metadata?.qr_url === "string" ? message.metadata.qr_url.trim() : "";
  const bodyURL = message.body.match(/(?:QR|Mã QR|Ma QR)\s*:\s*(https?:\/\/[^\s]+)/i)?.[1] ?? "";
  const candidate = metadataURL || bodyURL;
  if (!candidate) {
    return undefined;
  }

  try {
    const parsed = new URL(candidate);
    return parsed.protocol === "https:" || parsed.protocol === "http:" ? parsed.toString() : undefined;
  } catch {
    return undefined;
  }
}

function mapBotMessageAuthor(message: ApiMessage): ChatUser | null {
  const metadata = message.metadata;
  const botId = typeof metadata?.bot_id === "string" ? metadata.bot_id.trim() : "";
  const botSlug = typeof metadata?.bot_slug === "string" ? metadata.bot_slug.trim().toLowerCase() : "";

  if (message.kind !== "bot" && !botId && !botSlug) {
    return null;
  }

  const names: Record<string, string> = {
    "cskh-bot": "CSKH Bot",
    "gia-han-bot": "Gia Hạn Bot",
    "server-alert-bot": "Server Alert Bot",
    "thanh-toan-bot": "Thanh Toán Bot",
    "ticket-bot": "Ticket Bot"
  };

  return {
    id: botId || (botSlug ? `bot:${botSlug}` : "bot:system"),
    name: names[botSlug] ?? "Bot hệ thống",
    status: "online"
  };
}

function mapMessageAttachments(attachments?: MessageAttachment[]): MessageAttachmentItem[] {
  return (attachments ?? [])
    .map((attachment, index) => {
      const file = attachment.file;
      const fileId = attachment.file_id ?? file?.id ?? attachment.id;

      if (!fileId) {
        return null;
      }

      const name =
        attachment.file_name ??
        attachment.name ??
        attachment.original_name ??
        file?.name ??
        file?.file_name ??
        file?.original_name ??
        "File đính kèm";
      const mimeType = attachment.mime_type ?? file?.mime_type;
      const size = attachment.byte_size ?? attachment.size_bytes ?? attachment.size ?? file?.byte_size ?? file?.size_bytes ?? file?.size;
      const url = attachment.url ?? attachment.download_url ?? file?.url ?? file?.download_url;
      const extension = name.split(".").at(-1)?.toLowerCase() ?? "";
      const isAudio = Boolean(mimeType?.startsWith("audio/") || mimeType === "application/ogg");
      const isImage = Boolean(mimeType?.startsWith("image/") || ["gif", "jpeg", "jpg", "png", "webp"].includes(extension));
      const isVideo = Boolean(mimeType?.startsWith("video/"));

      return {
        checksumSha256: file?.checksum_sha256,
        fileId,
        id: attachment.id ?? `${fileId}-${index}`,
        isAudio,
        isImage,
        isVideo,
        mimeType,
        name,
        previewUrl: url,
        size: formatFileSize(size),
        status: file?.status,
        tone: fileTone(mimeType),
        url
      };
    })
    .filter(Boolean) as MessageAttachmentItem[];
}

export function createOptimisticMessage(params: {
  attachments?: ApiMessage["attachments"];
  body: string;
  channelId: string;
  clientMessageId?: string;
  currentUser: ChatUser;
  parentId?: string;
  replyTo?: MessageReplyPreview;
  workspaceId: string;
}): ApiMessage {
  const now = new Date().toISOString();
  const metadata =
    params.clientMessageId || params.replyTo
      ? {
          ...(params.clientMessageId ? { client_message_id: params.clientMessageId } : {}),
          ...(params.replyTo ? { reply_to: params.replyTo } : {})
        }
      : undefined;

  return {
    author: {
      avatar_url: params.currentUser.avatarUrl,
      display_name: params.currentUser.name,
      id: params.currentUser.id,
      status: params.currentUser.status
    },
    attachments: params.attachments,
    body: params.body,
    channel_id: params.channelId,
    created_at: now,
    id: params.clientMessageId ? `local-${params.clientMessageId}` : `local-${Date.now()}`,
    kind: "text",
    metadata,
    parent_id: params.parentId,
    thread_root_id: params.parentId,
    sender_id: params.currentUser.id,
    updated_at: now,
    workspace_id: params.workspaceId
  };
}

function mapMessageAuthor(
  author: MessageAuthor | null | undefined,
  fallbackAuthor: ChatUser,
  senderId?: string | null
): ChatUser {
  if (!author) {
    return senderId && senderId !== fallbackAuthor.id
      ? {
          id: senderId,
          name: "Người dùng",
          status: "offline"
        }
      : fallbackAuthor;
  }

  return {
    avatarUrl: author.avatar_url ?? undefined,
    id: author.id,
    name: displayName(author),
    status: author.status === "busy" ? "busy" : author.status === "offline" ? "offline" : "online"
  };
}

function displayName(
  user:
    | Pick<AuthUser, "display_name" | "email" | "username">
    | MessageAuthor
    | { display_name?: string; email?: string; username?: string }
    | null
    | undefined
): string {
  return user?.display_name || user?.username || user?.email || "Người dùng";
}

function formatTime(value?: string): string {
  if (!value) {
    return "";
  }

  return new Intl.DateTimeFormat("vi-VN", {
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value));
}

function formatFileSize(size?: number): string | undefined {
  if (!size) {
    return undefined;
  }

  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }

  if (size < 1024 * 1024 * 1024) {
    return `${(size / (1024 * 1024)).toFixed(1)} MB`;
  }

  return `${(size / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function fileTone(mimeType?: string): MessageAttachmentItem["tone"] {
  if (mimeType?.includes("pdf")) {
    return "red";
  }

  if (mimeType?.startsWith("image/")) {
    return "green";
  }

  return "slate";
}
