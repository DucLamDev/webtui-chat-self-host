"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@webtui/api-client";
import type {
  Channel as ApiChannel,
  ContactUser,
  CreateDepartmentInput,
  CreateChannelInput,
  DirectConversation as ApiDirectConversation,
  FileAttachment,
  FileObject,
  Message as ApiMessage,
  Notification as ApiNotification,
  WorkspaceMember
} from "@webtui/types";
import { api } from "@/lib/api";
import { useWorkspaceContext } from "@/features/workspace/hooks/use-workspace-context";
import { useChannelRealtime } from "./use-channel-realtime";
import {
  createOptimisticMessage,
  mergeMessageIntoTimeline,
  messageTimelineKey,
  useMessageTimeline,
  type MessageSearchFilters
} from "./use-message-timeline";
import { useNotificationPresence } from "./use-notification-presence";
import {
  createClientMessageId,
  enqueueOutbox,
  isLikelyOfflineError,
  readOutbox,
  readWorkspaceChatCache,
  removeOutboxItem,
  updateOutboxItem,
  writeWorkspaceChatCache,
  type MessageOutboxEntry,
  type WorkspaceChatCache
} from "../model/offline-cache";
import type {
  ChannelTone,
  ChatChannel,
  ChatUser,
  DirectConversation,
  FileItem,
  MediaItem,
  MessageAttachmentItem,
  MessageReplyPreview,
  NotificationItem,
  PresenceStatus
} from "../model/types";
import { useUploadStore, type UploadQueueItem } from "../stores/upload-store";
import { buildChatRoute, buildWorkspaceSectionRoute, directIdPrefix, directRouteRef, parseChatRoute } from "@/lib/chat-route";

const channelTones: ChannelTone[] = ["purple", "green", "orange", "red", "violet", "slate"];
// WebSocket events invalidate these queries immediately. This interval is only
// a fallback for clients that temporarily lose realtime connectivity.
const contactRefetchMs = 30_000;

export type SendMessagePayload = {
  body: string;
  clientMessageId?: string;
  mentionedUserIds?: string[];
  parentId?: string;
  replyTo?: MessageReplyPreview;
  uploads: UploadQueueItem[];
};

export type SendCallEventPayload = {
  callId: string;
  durationSeconds?: number;
  endedAt: string;
  initiatorUserId: string;
  mode: "audio" | "video";
  reason?: string;
  startedAt?: string;
  status: "completed" | "missed";
};

export type SendMessageResult = {
  failedUploadNames: string[];
  message: Awaited<ReturnType<typeof api.messages.send>>;
  queued?: boolean;
};

export type CreateChannelPayload = Pick<CreateChannelInput, "description" | "name" | "slug" | "type">;
export type CreateDepartmentPayload = Pick<CreateDepartmentInput, "description" | "name" | "parent_id" | "slug">;
type CreateDirectConversationMutationInput =
  | string
  | {
      participantId: string;
      workspaceId?: string;
    };

export type ChatWorkspaceDataOptions = {
  friendSearchQuery?: string;
  messageSearchQuery?: string;
  messageSearchFilters?: MessageSearchFilters;
  threadMessageId?: string;
};

export { mapAuthUser } from "./use-message-timeline";

export function useChatWorkspaceData(currentUser: ChatUser, options: ChatWorkspaceDataOptions = {}) {
  const queryClient = useQueryClient();
  const lastMarkedReadRef = useRef("");
  const isFlushingOutboxRef = useRef(false);
  const [isViewportActive, setIsViewportActive] = useState(() =>
    typeof document === "undefined" ? true : document.visibilityState === "visible" && document.hasFocus()
  );
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const workspaceContext = useWorkspaceContext();
  const { workspaceId } = workspaceContext;
  const [cachedChatState, setCachedChatState] = useState<{ value: WorkspaceChatCache | null; workspaceId: string }>({
    value: null,
    workspaceId: ""
  });
  const cachedChat = cachedChatState.workspaceId === workspaceId ? cachedChatState.value : null;
  const [outboxItems, setOutboxItems] = useState<MessageOutboxEntry[]>([]);
  const parsedRoute = parseChatRoute(pathname, searchParams);
  const legacyChannelId = searchParams.get("channel") ?? "";
  const friendSearchQuery = options.friendSearchQuery?.trim() ?? "";

  useEffect(() => {
    let disposed = false;
    void readOutbox()
      .then((items) => {
        if (!disposed) {
          setOutboxItems(items);
        }
      })
      .catch(() => undefined);
    return () => {
      disposed = true;
    };
  }, []);

  const channelsQuery = useQuery({
    enabled: Boolean(workspaceId),
    queryFn: () => api.channels.list(workspaceId),
    queryKey: queryKeys.channels.all(workspaceId),
    refetchInterval: contactRefetchMs
  });
  useEffect(() => {
    let disposed = false;
    if (!workspaceId) {
      setCachedChatState({ value: null, workspaceId: "" });
      return undefined;
    }
    setCachedChatState({ value: null, workspaceId });
    void readWorkspaceChatCache(workspaceId)
      .then((cache) => {
        if (!disposed) {
          setCachedChatState({ value: cache, workspaceId });
        }
      })
      .catch(() => undefined);
    return () => {
      disposed = true;
    };
  }, [workspaceId]);
  const channelSource = channelsQuery.data ?? cachedChat?.channels ?? [];
  const channels = useMemo(
    () => channelSource.map(mapChannel).filter((channel) => channel.type !== "direct"),
    [channelSource]
  );

  const notificationPresence = useNotificationPresence({
    currentUserId: currentUser.id,
    enabled: Boolean(workspaceId),
    workspaceId
  });

  // Channel/workspace member payloads from older servers do not include the
  // profile avatar. Contacts do, so keep this fallback while the member API
  // catches up and prefer the accepted-contact record when both are present.
  const contactsQuery = useQuery({
    queryFn: () => api.contacts.list(),
    queryKey: queryKeys.contacts.all,
    refetchInterval: contactRefetchMs
  });
  const contactRequestsQuery = useQuery({
    queryFn: () => api.contacts.requests({ status: "all" }),
    queryKey: queryKeys.contacts.requests("all"),
    refetchInterval: contactRefetchMs
  });
  const contacts = contactsQuery.data ?? [];
  const contactRequests = contactRequestsQuery.data ?? [];
  const contactProfileByUserId = useMemo(
    () => new Map<string, ContactUser>(
      [...contactRequests, ...contacts].map((contact) => [contact.user.id, contact.user])
    ),
    [contactRequests, contacts]
  );

  const directConversationsQuery = useQuery({
    enabled: Boolean(workspaceId),
    queryFn: () => api.channels.directConversations(workspaceId),
    queryKey: queryKeys.channels.directConversations(workspaceId),
    refetchInterval: contactRefetchMs
  });
  const directConversationSource = directConversationsQuery.data ?? cachedChat?.directConversations ?? [];
  useEffect(() => {
    if (!workspaceId || channelsQuery.data === undefined) {
      return;
    }
    void writeWorkspaceChatCache(workspaceId, {
      channels: channelsQuery.data,
      directConversations: directConversationsQuery.data ?? cachedChat?.directConversations ?? []
    }).catch(() => undefined);
  }, [cachedChat?.directConversations, channelsQuery.data, directConversationsQuery.data, workspaceId]);
  const directConversationSummaries = useQueries({
    queries: directConversationSource.map((conversation) => {
      const channelId = conversation.channel_id ?? conversation.id;
      return {
        enabled: Boolean(workspaceId && channelId && !conversation.last_message),
        queryFn: () => api.messages.list(workspaceId, channelId, { limit: 1 }),
        queryKey: ["direct-conversation-summary", workspaceId, channelId] as const,
        refetchInterval: contactRefetchMs,
        retry: 2,
        staleTime: 3_000
      };
    })
  });
  const directConversations = useMemo(
    () =>
      directConversationSource
        .map((item, index) =>
          mapDirectConversation(
            item,
            notificationPresence.presenceByUserId,
            currentUser.id,
            directConversationSummaries[index]?.data?.[0],
            workspaceContext.members,
            contactProfileByUserId
          )
        )
        .filter(Boolean) as DirectConversation[],
    [contactProfileByUserId, currentUser.id, directConversationSource, directConversationSummaries, notificationPresence.presenceByUserId, workspaceContext.members]
  );
  const requestedChannelId = useMemo(() => {
    if (legacyChannelId) {
      return legacyChannelId;
    }
    const reference = parsedRoute?.targetRef;
    if (!reference) {
      return "";
    }
    if (parsedRoute?.kind === "channel") {
      return channels.find((channel) => channel.slug === reference || channel.id === reference)?.id ?? "";
    }
    if (parsedRoute?.kind === "dm") {
      const prefix = directIdPrefix(reference);
      return directConversations.find((conversation) => conversation.id === reference || conversation.id.startsWith(prefix))?.id ?? "";
    }
    return "";
  }, [channels, directConversations, legacyChannelId, parsedRoute?.kind, parsedRoute?.targetRef]);
  const membersWithPresence = useMemo(
    () => mapMembersWithPresence(workspaceContext.members, notificationPresence.presenceByUserId),
    [notificationPresence.presenceByUserId, workspaceContext.members]
  );

  const selectedDirectConversation = directConversations.find((conversation) => conversation.id === requestedChannelId);
  const selectedChannel =
    selectedDirectConversation
      ? directConversationToChannel(selectedDirectConversation)
      : channels.find((channel) => channel.id === requestedChannelId) ?? null;
  const selectedChannelId = requestedChannelId || "";
  const canAccessSelectedChannel = Boolean(
    selectedChannel && (selectedChannel.type === "direct" || selectedChannel.isMember)
  );

  const setSelectedChannelId = useCallback(
    (nextChannelId: string, nextWorkspaceId = workspaceId, requestedType?: "channel" | "direct") => {
      const workspace = workspaceContext.workspaces.find((item) => item.id === nextWorkspaceId);
      const workspaceRef = workspace?.slug || nextWorkspaceId;
      if (!workspaceRef) {
        return;
      }
      if (!nextChannelId) {
        router.replace(buildChatRoute(workspaceRef), { scroll: false });
        return;
      }
      const direct = directConversations.find((item) => item.id === nextChannelId);
      const channel = channels.find((item) => item.id === nextChannelId);
      const isDirect = requestedType === "direct" || Boolean(direct);
      const targetRef = isDirect
        ? directRouteRef(direct?.user.name ?? "hoi-thoai", nextChannelId)
        : channel?.slug || nextChannelId;
      router.replace(buildChatRoute(workspaceRef, isDirect ? "dm" : "channel", targetRef), { scroll: false });
    },
    [channels, directConversations, router, workspaceContext.workspaces, workspaceId]
  );
  const setWorkspaceSection = useCallback(
    (section?: string, messageView?: "channels" | "conversations") => {
      const workspace = workspaceContext.workspaces.find((item) => item.id === workspaceId);
      const workspaceRef = workspace?.slug || workspaceId;
      if (!workspaceRef) return;
      router.replace(
        section ? buildWorkspaceSectionRoute(workspaceRef, section) : buildChatRoute(workspaceRef, undefined, undefined, messageView),
        { scroll: false }
      );
    },
    [router, workspaceContext.workspaces, workspaceId]
  );

  useEffect(() => {
    if (legacyChannelId && requestedChannelId && workspaceId) {
      setSelectedChannelId(requestedChannelId, workspaceId);
    }
  }, [legacyChannelId, requestedChannelId, setSelectedChannelId, workspaceId]);

  const messageTimeline = useMessageTimeline({
    canManageMessages: workspaceContext.can("message.manage"),
    channelId: selectedChannelId,
    currentUser,
    enabled: Boolean(workspaceId && selectedChannelId && canAccessSelectedChannel),
    searchQuery: options.messageSearchQuery,
    searchFilters: options.messageSearchFilters,
    threadMessageId: options.threadMessageId,
    workspaceId
  });
  const realtime = useChannelRealtime({
    channelId: selectedChannelId,
    channelIds: [
      selectedChannelId,
      ...channels.filter((channel) => channel.isMember).map((channel) => channel.id),
      ...directConversations.map((conversation) => conversation.id)
    ],
    currentUserId: currentUser.id,
    enabled: Boolean(workspaceId),
    workspaceId
  });
  const selectedOutboxMessages = useMemo(() => {
    if (!workspaceId || !selectedChannelId) {
      return [];
    }
    const existingIds = new Set(messageTimeline.messages.map((message) => message.id));
    return outboxItems
      .filter((item) => item.workspaceId === workspaceId && item.channelId === selectedChannelId)
      .filter((item) => !existingIds.has(`local-${item.clientMessageId}`))
      .map((item) => outboxEntryToChatMessage(item, currentUser));
  }, [currentUser, messageTimeline.messages, outboxItems, selectedChannelId, workspaceId]);
  const selectedChannelWithMessages = selectedChannelId
    ? { ...(selectedChannel ?? placeholderChannel(selectedChannelId)), messages: [...messageTimeline.messages, ...selectedOutboxMessages] }
    : null;
  const managedChannelIds = channels.filter((channel) => channel.canManage).map((channel) => channel.id);
  const joinRequestQueries = useQueries({
    queries: managedChannelIds.map((channelId) => ({
      enabled: Boolean(workspaceId),
      queryFn: () => api.channels.joinRequests(workspaceId, channelId),
      queryKey: queryKeys.channels.joinRequests(workspaceId, channelId),
      refetchInterval: contactRefetchMs
    }))
  });
  const joinRequestsByChannelId = useMemo(
    () => new Map(managedChannelIds.map((channelId, index) => [channelId, joinRequestQueries[index]?.data ?? []])),
    [joinRequestQueries, managedChannelIds]
  );

  const searchUsersQuery = useQuery({
    enabled: friendSearchQuery.length >= 2,
    queryFn: () => api.users.list({ limit: 25, q: friendSearchQuery, status: "active" }),
    queryKey: queryKeys.users.all(friendSearchQuery, "active")
  });
  const searchUsers = useMemo(
    () => (searchUsersQuery.data ?? []).filter((item) => item.id !== currentUser.id),
    [currentUser.id, searchUsersQuery.data]
  );
  const filesQuery = useQuery({
    enabled: Boolean(workspaceId),
    queryFn: () => api.files.list(workspaceId),
    queryKey: queryKeys.files.all(workspaceId)
  });
  const files = useMemo(() => (filesQuery.data ?? []).map(mapFile), [filesQuery.data]);
  const channelMediaQuery = useQuery({
    enabled: Boolean(workspaceId && selectedChannelId && canAccessSelectedChannel),
    queryFn: () => api.files.channelMedia(workspaceId, selectedChannelId, { limit: 500 }),
    queryKey: queryKeys.files.channelMedia(workspaceId, selectedChannelId),
    staleTime: 10_000
  });
  const canManageDepartments = workspaceContext.can("workspace.manage");
  const departmentsQuery = useQuery({
    enabled: Boolean(workspaceId && canManageDepartments),
    queryFn: () => api.departments.list(workspaceId),
    queryKey: queryKeys.departments.all(workspaceId),
    retry: false
  });
  const departments = departmentsQuery.data ?? [];
  const mediaItems = useMemo(
    () => (channelMediaQuery.data ?? []).map(mapChannelMediaAttachment),
    [channelMediaQuery.data]
  );
  const offlineReadMode = Boolean(
    workspaceContext.offlineReadMode ||
      messageTimeline.isOfflineReadMode ||
      (cachedChat && (
        (channelsQuery.isError && isLikelyOfflineError(channelsQuery.error)) ||
        (directConversationsQuery.isError && isLikelyOfflineError(directConversationsQuery.error))
      ))
  );

  const createChannelMutation = useMutation({
    mutationFn: (input: CreateChannelPayload) => {
      if (!workspaceId) {
        throw new Error("Chưa có workspace để tạo kênh.");
      }

      return api.channels.create(workspaceId, input);
    },
    onSuccess: async (channel) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.channels.all(workspaceId) });
      setSelectedChannelId(channel.id);
    }
  });

  const openPrivateSessionMutation = useMutation({
    mutationFn: (sourceChannelId: string) => {
      if (!workspaceId) {
        throw new Error("Chưa có workspace để mở phiên riêng tư.");
      }
      return api.channels.openPrivateSession(workspaceId, sourceChannelId);
    },
    onSuccess: async (channel) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.channels.all(workspaceId) });
      setSelectedChannelId(channel.id, workspaceId, "channel");
    }
  });

  const requestChannelJoinMutation = useMutation({
    mutationFn: (channelId: string) => {
      if (!workspaceId) {
        throw new Error("Chưa có workspace để tham gia kênh.");
      }
      return api.channels.requestJoin(workspaceId, channelId);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.channels.all(workspaceId) });
    }
  });

  const inviteChannelMemberMutation = useMutation({
    mutationFn: ({ channelId, userId }: { channelId: string; userId: string }) =>
      api.channels.addMember(workspaceId, channelId, { user_id: userId }),
    onSuccess: async (_member, input) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.all(workspaceId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.members(workspaceId, input.channelId) })
      ]);
    }
  });

  const approveChannelJoinMutation = useMutation({
    mutationFn: ({ channelId, userId }: { channelId: string; userId: string }) =>
      api.channels.approveJoinRequest(workspaceId, channelId, userId),
    onSuccess: async (_member, input) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.joinRequests(workspaceId, input.channelId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.all(workspaceId) })
      ]);
    }
  });

  const rejectChannelJoinMutation = useMutation({
    mutationFn: ({ channelId, userId }: { channelId: string; userId: string }) =>
      api.channels.rejectJoinRequest(workspaceId, channelId, userId),
    onSuccess: async (_result, input) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.channels.joinRequests(workspaceId, input.channelId) });
    }
  });

  const createDepartmentMutation = useMutation({
    mutationFn: (input: CreateDepartmentPayload) => {
      if (!workspaceId) {
        throw new Error("Chưa có workspace để tạo phòng ban.");
      }
      return api.departments.create(workspaceId, input);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.departments.all(workspaceId) });
    }
  });

  const sendMessageMutation = useMutation({
    mutationFn: async (input: SendMessagePayload): Promise<SendMessageResult> => {
      if (!workspaceId || !selectedChannelId) {
        throw new Error("Hãy chọn kênh trước khi gửi.");
      }

      const clientMessageId = input.clientMessageId || createClientMessageId();
      input.clientMessageId = clientMessageId;
      const uploads = input.uploads.filter((item) => item.status === "queued" || item.status === "failed");
      const messageBody = input.body || uploadMessageFallback(uploads);
      const isVoiceMessage = Boolean(uploads.length && !input.body && uploads.every((upload) => upload.isAudio));
      let sentMessage: ApiMessage;
      try {
        sentMessage = await api.messages.send(workspaceId, selectedChannelId, {
          body: messageBody,
          client_message_id: clientMessageId,
          kind: isVoiceMessage ? "file" : "text",
          ...(input.mentionedUserIds?.length ? { mentioned_user_ids: input.mentionedUserIds } : {}),
          ...(input.parentId ? { parent_id: input.parentId } : {}),
          metadata: {
            client_message_id: clientMessageId,
            ...(input.replyTo ? { reply_to: input.replyTo } : {}),
            ...(uploads.length
              ? {
                  has_attachments: true,
                  ...(isVoiceMessage ? { message_type: "voice" } : {})
                }
              : {})
          }
        });
        await removeOutboxItem(clientMessageId).catch(() => undefined);
      } catch (error) {
        if (!uploads.length && messageBody && isLikelyOfflineError(error)) {
          const entry = await enqueueOutbox({
            body: messageBody,
            channelId: selectedChannelId,
            clientMessageId,
            mentionedUserIds: input.mentionedUserIds,
            parentId: input.parentId,
            replyTo: input.replyTo,
            workspaceId
          });
          setOutboxItems(await readOutbox().catch(() => [entry]));
          return {
            failedUploadNames: [],
            message: createOptimisticMessage({
              body: messageBody,
              channelId: selectedChannelId,
              clientMessageId,
              currentUser,
              parentId: input.parentId,
              replyTo: input.replyTo,
              workspaceId
            }),
            queued: true
          };
        }
        throw error;
      }

      const attachedFiles: NonNullable<ApiMessage["attachments"]> = [];
      const failedUploadNames: string[] = [];

      for (const [index, upload] of uploads.entries()) {
        try {
          useUploadStore.getState().markUploading(upload.id);
          const uploadedFile = await api.files.upload(workspaceId, {
            channel_id: selectedChannelId,
            file: upload.file,
            message_id: sentMessage.id,
            sort_order: index,
            ...(upload.isAudio
              ? {
                  metadata: {
                    duration_seconds: upload.durationSeconds ?? 0,
                    media_type: "voice"
                  }
                }
              : {})
          });
          // Uploading with channel_id/message_id attaches the file atomically on
          // the backend. Calling /attachments again here duplicated the attach
          // operation and could incorrectly mark a successful image as failed.
          const attachmentFile = uploadedFile;
          const attachmentFileId = uploadedFile.id;

          attachedFiles.push({
            byte_size: attachmentFile.byte_size,
            file: attachmentFile,
            file_id: attachmentFileId,
            id: `${sentMessage.id}-${attachmentFileId}`,
            mime_type: attachmentFile.mime_type,
            name: attachmentFile.name ?? attachmentFile.file_name ?? attachmentFile.original_name,
            original_name: attachmentFile.original_name,
            size: attachmentFile.size,
            size_bytes: attachmentFile.size_bytes,
            url: attachmentFile.url ?? attachmentFile.download_url
          });

          useUploadStore.getState().markAttached(upload.id, sentMessage.id, attachmentFileId);
        } catch (error) {
          failedUploadNames.push(upload.name);
          useUploadStore
            .getState()
            .markFailed(upload.id, error instanceof Error ? error.message : "Không upload được file.");
        }
      }

      if (uploads.length && !attachedFiles.length && !input.body) {
        await api.messages.delete(workspaceId, selectedChannelId, sentMessage.id).catch(() => undefined);
        throw new Error(
          isVoiceMessage
            ? "Không tải được tin nhắn thoại. Bản ghi tạm đã được thu hồi; hãy thử lại."
            : "Không tải được tệp đính kèm. Tin nhắn tạm đã được thu hồi; hãy thử lại."
        );
      }
      if (attachedFiles.length) {
        queryClient.setQueryData(queryKeys.files.attachments(workspaceId, selectedChannelId, sentMessage.id), attachedFiles);
      }

      return {
        failedUploadNames,
        message: attachedFiles.length
          ? {
              ...sentMessage,
              attachments: attachedFiles,
              metadata: {
                ...(sentMessage.metadata ?? {}),
                has_attachments: true
              }
            }
          : sentMessage
      };
    },
    onMutate: async (input) => {
      if (!workspaceId || !selectedChannelId) {
        return undefined;
      }

      const uploads = input.uploads.filter((item) => item.status === "queued" || item.status === "failed");
      input.clientMessageId = input.clientMessageId || createClientMessageId();
      const body = input.body || uploadMessageFallback(uploads);
      if (!body) {
        return undefined;
      }

      const queryKey = messageTimelineKey(workspaceId, selectedChannelId);
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData(queryKey);
      const optimisticMessage = createOptimisticMessage({
        attachments: uploads.map((upload) => uploadToOptimisticAttachment(upload)),
        body,
        channelId: selectedChannelId,
        clientMessageId: input.clientMessageId,
        currentUser,
        parentId: input.parentId,
        replyTo: input.replyTo,
        workspaceId
      });

      mergeMessageIntoTimeline(queryClient, workspaceId, selectedChannelId, optimisticMessage);

      return {
        optimisticId: optimisticMessage.id,
        previous
      };
    },
    onError: (error, input, context) => {
      if (input.uploads.length === 0 && isLikelyOfflineError(error)) {
        return;
      }
      if (context?.previous) {
        queryClient.setQueryData(messageTimelineKey(workspaceId, selectedChannelId), context.previous);
      }
    },
    onSuccess: async (result, input, context) => {
      mergeMessageIntoTimeline(queryClient, workspaceId, selectedChannelId, result.message, context?.optimisticId);
      if (!result.queued && input.clientMessageId) {
        setOutboxItems(await readOutbox().catch(() => []));
      }
      queryClient.setQueryData<ApiDirectConversation[]>(
        queryKeys.channels.directConversations(workspaceId),
        (current) => updateDirectConversationLastMessage(current, selectedChannelId, result.message)
      );
      queryClient.setQueryData<ApiChannel[]>(queryKeys.channels.all(workspaceId), (current) =>
        updateChannelAfterOwnMessage(current, selectedChannelId, result.message)
      );
      if (input.uploads.length) {
        await Promise.all([
          queryClient.invalidateQueries({
            queryKey: queryKeys.files.all(workspaceId),
            refetchType: "inactive"
          }),
          queryClient.invalidateQueries({
            queryKey: queryKeys.files.channelMedia(workspaceId, selectedChannelId)
          })
        ]);
      }
    }
  });

  const sendCallEventMutation = useMutation({
    mutationFn: (input: SendCallEventPayload) => {
      if (!workspaceId || !selectedChannelId) {
        throw new Error("Hãy chọn hội thoại trước khi lưu cuộc gọi.");
      }

      return api.messages.send(workspaceId, selectedChannelId, {
        body: callEventBody(input),
        client_message_id: `call-${input.callId}-${input.status}`,
        kind: "event",
        metadata: {
          call_id: input.callId,
          call_mode: input.mode,
          call_status: input.status,
          duration_seconds: input.durationSeconds ?? 0,
          ended_at: input.endedAt,
          initiator_user_id: input.initiatorUserId,
          message_type: "call",
          reason: input.reason ?? "",
          started_at: input.startedAt ?? ""
        }
      });
    },
    onSuccess: (message) => {
      mergeMessageIntoTimeline(queryClient, workspaceId, selectedChannelId, message);
      queryClient.setQueryData<ApiDirectConversation[]>(
        queryKeys.channels.directConversations(workspaceId),
        (current) => updateDirectConversationLastMessage(current, selectedChannelId, message)
      );
      queryClient.setQueryData<ApiChannel[]>(queryKeys.channels.all(workspaceId), (current) =>
        updateChannelAfterOwnMessage(current, selectedChannelId, message)
      );
    }
  });

  const flushOutbox = useCallback(async () => {
    if (isFlushingOutboxRef.current || (typeof navigator !== "undefined" && navigator.onLine === false)) {
      return;
    }
    isFlushingOutboxRef.current = true;
    try {
      const items = await readOutbox();
      for (const item of items) {
        try {
          await updateOutboxItem(item.clientMessageId, { attempts: item.attempts + 1, lastError: undefined });
          const message = await api.messages.send(item.workspaceId, item.channelId, {
            body: item.body,
            client_message_id: item.clientMessageId,
            kind: "text",
            ...(item.mentionedUserIds?.length ? { mentioned_user_ids: item.mentionedUserIds } : {}),
            ...(item.parentId ? { parent_id: item.parentId } : {}),
            metadata: {
              client_message_id: item.clientMessageId,
              ...(item.replyTo ? { reply_to: item.replyTo } : {})
            }
          });
          await removeOutboxItem(item.clientMessageId);
          mergeMessageIntoTimeline(queryClient, item.workspaceId, item.channelId, message, `local-${item.clientMessageId}`);
          queryClient.setQueryData<ApiDirectConversation[]>(
            queryKeys.channels.directConversations(item.workspaceId),
            (current) => updateDirectConversationLastMessage(current, item.channelId, message)
          );
          queryClient.setQueryData<ApiChannel[]>(queryKeys.channels.all(item.workspaceId), (current) =>
            updateChannelAfterOwnMessage(current, item.channelId, message)
          );
        } catch (error) {
          await updateOutboxItem(item.clientMessageId, {
            attempts: item.attempts + 1,
            lastError: error instanceof Error ? error.message : "Send failed."
          }).catch(() => undefined);
          if (isLikelyOfflineError(error)) {
            break;
          }
        }
      }
      setOutboxItems(await readOutbox().catch(() => []));
    } finally {
      isFlushingOutboxRef.current = false;
    }
  }, [queryClient]);

  useEffect(() => {
    const handleOnline = () => {
      void flushOutbox();
      if (workspaceId && selectedChannelId) {
        void queryClient.invalidateQueries({ queryKey: messageTimelineKey(workspaceId, selectedChannelId) });
      }
    };
    window.addEventListener("online", handleOnline);
    return () => window.removeEventListener("online", handleOnline);
  }, [flushOutbox, queryClient, selectedChannelId, workspaceId]);

  useEffect(() => {
    if (realtime.status !== "connected") {
      return;
    }
    void flushOutbox();
    if (workspaceId && selectedChannelId) {
      void Promise.all([
        queryClient.invalidateQueries({ queryKey: messageTimelineKey(workspaceId, selectedChannelId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.all(workspaceId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.directConversations(workspaceId) })
      ]);
    }
  }, [flushOutbox, queryClient, realtime.status, selectedChannelId, workspaceId]);

  const downloadMutation = useMutation({
    mutationFn: (file: FileItem) => api.files.download(workspaceId, file.id)
  });
  const downloadAttachment = useCallback(
    (fileId: string) => api.files.download(workspaceId, fileId),
    [workspaceId]
  );

  const createDirectConversationMutation = useMutation({
    mutationFn: (input: CreateDirectConversationMutationInput) => {
      const participantId = typeof input === "string" ? input : input.participantId;
      const targetWorkspaceId = typeof input === "string" ? workspaceId : input.workspaceId || workspaceId;

      if (!targetWorkspaceId) {
        throw new Error("Chưa có workspace để tạo hội thoại riêng.");
      }

      return api.channels.createDirectConversation(targetWorkspaceId, {
        participant_ids: [participantId]
      });
    },
    onSuccess: async (conversation, input) => {
      const targetWorkspaceId = typeof input === "string" ? workspaceId : input.workspaceId || workspaceId;
      if (!targetWorkspaceId) {
        return;
      }
      queryClient.setQueryData<ApiDirectConversation[]>(
        queryKeys.channels.directConversations(targetWorkspaceId),
        (current) => upsertDirectConversation(current, conversation)
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.directConversations(targetWorkspaceId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.members(targetWorkspaceId) })
      ]);
    }
  });

  const invalidateContacts = useCallback(async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.contacts.all }),
      queryClient.invalidateQueries({ queryKey: queryKeys.contacts.requests("all") })
    ]);
  }, [queryClient]);

  const sendContactRequestMutation = useMutation({
    mutationFn: (userId: string) => api.contacts.sendRequest({ user_id: userId }),
    onSuccess: invalidateContacts
  });

  const acceptContactRequestMutation = useMutation({
    mutationFn: (requestId: string) => api.contacts.acceptRequest(requestId),
    onSuccess: invalidateContacts
  });

  const rejectContactRequestMutation = useMutation({
    mutationFn: (requestId: string) => api.contacts.rejectRequest(requestId),
    onSuccess: invalidateContacts
  });

  const cancelContactRequestMutation = useMutation({
    mutationFn: (requestId: string) => api.contacts.cancelRequest(requestId),
    onSuccess: invalidateContacts
  });

  const updateProfileMutation = useMutation({
    mutationFn: (input: {
      avatar_url?: string | null;
      display_name?: string;
      locale?: string;
      phone_number?: string | null;
      timezone?: string;
    }) => api.users.updateMe(input),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.auth.me }),
        queryClient.invalidateQueries({ queryKey: queryKeys.users.me })
      ]);
    }
  });

  useEffect(() => {
    const updateViewportState = () => {
      setIsViewportActive(document.visibilityState === "visible" && document.hasFocus());
    };

    updateViewportState();
    window.addEventListener("focus", updateViewportState);
    window.addEventListener("blur", updateViewportState);
    document.addEventListener("visibilitychange", updateViewportState);

    return () => {
      window.removeEventListener("focus", updateViewportState);
      window.removeEventListener("blur", updateViewportState);
      document.removeEventListener("visibilitychange", updateViewportState);
    };
  }, []);

  const markChannelRead = useCallback(
    (channelId: string, messageId?: string) => {
      if (!workspaceId) {
        return;
      }

      void api.channels
        .updateReadState(workspaceId, channelId, messageId ? { last_read_message_id: messageId } : {})
        .then(() => {
          queryClient.setQueryData<ApiChannel[]>(queryKeys.channels.all(workspaceId), (current) =>
            markChannelCacheRead(current, channelId)
          );
          queryClient.setQueryData<ApiDirectConversation[]>(
            queryKeys.channels.directConversations(workspaceId),
            (current) => markDirectConversationCacheRead(current, channelId)
          );
        })
        .catch(() => undefined);
    },
    [queryClient, workspaceId]
  );

  const lastTimelineMessage = messageTimeline.messages.at(-1);
  const lastTimelineMessageId = lastTimelineMessage?.id ?? "";
  const lastTimelineMessagePending = Boolean(lastTimelineMessage?.isPending);

  useEffect(() => {
    if (!workspaceId || !selectedChannelId || !lastTimelineMessageId || lastTimelineMessagePending) {
      return;
    }
    if (!isViewportActive) {
      return;
    }

    const readKey = `${workspaceId}:${selectedChannelId}:${lastTimelineMessageId}`;
    if (lastMarkedReadRef.current === readKey) {
      return;
    }

    lastMarkedReadRef.current = readKey;
    markChannelRead(selectedChannelId, lastTimelineMessageId);
  }, [isViewportActive, lastTimelineMessageId, lastTimelineMessagePending, markChannelRead, selectedChannelId, workspaceId]);

  return {
    ...workspaceContext,
    approveChannelJoinMutation,
    canAccessSelectedChannel,
    channels,
    channelsQuery,
    acceptContactRequestMutation,
    cancelContactRequestMutation,
    createChannelMutation,
    createDirectConversationMutation,
    createDepartmentMutation,
    directConversations,
    directConversationsQuery,
    downloadAttachment,
    downloadMutation,
    departments,
    departmentsQuery,
    contacts,
    contactsQuery,
    contactRequests,
    contactRequestsQuery,
    files,
    channelMediaQuery,
    filesQuery,
    inviteChannelMemberMutation,
    joinRequestsByChannelId,
    hasOlderMessages: messageTimeline.hasOlderMessages,
    isLoadingOlderMessages: messageTimeline.isLoadingOlderMessages,
    loadOlderMessages: messageTimeline.loadOlderMessages,
    markChannelRead,
    mediaItems,
    members: membersWithPresence,
    messages: messageTimeline.messages,
    messagesQuery: messageTimeline.messagesQuery,
    messageSearchQuery: messageTimeline.searchQuery,
    messageSearchResults: messageTimeline.searchResults,
    markAllNotificationsReadMutation: notificationPresence.markAllNotificationsReadMutation,
    markNotificationReadMutation: notificationPresence.markNotificationReadMutation,
    notifications: notificationPresence.notifications.map(mapNotification),
    notificationsQuery: notificationPresence.notificationsQuery,
    openPrivateSessionMutation,
    offlineReadMode,
    outboxItems,
    queuedOutboxCount: outboxItems.length,
    flushOutbox,
    presenceByUserId: notificationPresence.presenceByUserId,
    presenceQuery: notificationPresence.presenceQuery,
    pinnedMessages: messageTimeline.pinnedMessages,
    pinnedMessagesQuery: messageTimeline.pinnedMessagesQuery,
    pinMessageMutation: messageTimeline.pinMessageMutation,
    realtime,
    selectedChannel,
    selectedChannelId,
    selectedChannelWithMessages,
    setSelectedChannelId,
    setWorkspaceSection,
    searchUsers,
    searchUsersQuery,
    sendCallEventMutation,
    sendThreadMessageMutation: messageTimeline.sendThreadMessageMutation,
    rejectContactRequestMutation,
    rejectChannelJoinMutation,
    requestChannelJoinMutation,
    sendContactRequestMutation,
    threadMessages: messageTimeline.threadMessages,
    threadQuery: messageTimeline.threadQuery,
    editMessageMutation: messageTimeline.editMessageMutation,
    forwardMessageMutation: messageTimeline.forwardMessageMutation,
    deleteMessageMutation: messageTimeline.deleteMessageMutation,
    toggleReactionMutation: messageTimeline.toggleReactionMutation,
    unpinMessageMutation: messageTimeline.unpinMessageMutation,
    unreadNotificationsCount: notificationPresence.unreadNotificationsCount,
    updateProfileMutation,
    sendMessageMutation
  };
}

function callEventBody(input: SendCallEventPayload): string {
  const modeLabel = input.mode === "video" ? "video" : "thoại";
  if (input.status === "missed") {
    return `Cuộc gọi ${modeLabel} bị nhỡ`;
  }
  return `Cuộc gọi ${modeLabel} ${formatCallDuration(input.durationSeconds ?? 0)}`;
}

function formatCallDuration(durationSeconds: number): string {
  const safeSeconds = Math.max(0, Math.round(durationSeconds));
  const minutes = Math.floor(safeSeconds / 60);
  const seconds = safeSeconds % 60;
  return `${minutes} phút ${seconds} giây`;
}

function uploadMessageFallback(uploads: UploadQueueItem[]): string {
  if (!uploads.length) {
    return "";
  }

  const imageCount = uploads.filter((upload) => upload.isImage).length;
  const audioCount = uploads.filter((upload) => upload.isAudio).length;

  if (audioCount === uploads.length) {
    return audioCount === 1 ? "Đã gửi tin nhắn thoại" : `Đã gửi ${audioCount} tin nhắn thoại`;
  }

  if (imageCount === uploads.length) {
    return imageCount === 1 ? "Đã gửi ảnh" : `Đã gửi ${imageCount} ảnh`;
  }

  if (uploads.length === 1) {
    return `Đã gửi file ${uploads[0].name}`;
  }

  return `Đã gửi ${uploads.length} file`;
}

function outboxEntryToChatMessage(entry: MessageOutboxEntry, currentUser: ChatUser): ChatChannel["messages"][number] {
  return {
    author: currentUser,
    body: entry.body,
    id: `local-${entry.clientMessageId}`,
    isLocal: true,
    isMine: true,
    isPending: true,
    parentId: entry.parentId,
    rawChannelId: entry.channelId,
    rawCreatedAt: entry.createdAt,
    rawSenderId: currentUser.id,
    replyTo: entry.replyTo,
    sentAt: formatOutboxTime(entry.createdAt)
  };
}

function formatOutboxTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toLocaleTimeString("vi-VN", { hour: "2-digit", minute: "2-digit" });
}

function uploadToOptimisticAttachment(upload: UploadQueueItem): NonNullable<ApiMessage["attachments"]>[number] {
  return {
    byte_size: upload.size,
    file_id: upload.id,
    id: upload.id,
    mime_type: upload.file.type,
    name: upload.name,
    original_name: upload.name,
    size: upload.size,
    size_bytes: upload.size,
    url: upload.previewUrl
  };
}

function mapChannel(channel: ApiChannel, index: number): ChatChannel {
  return {
    canManage: Boolean(channel.can_manage),
    createdBy: channel.created_by ?? undefined,
    departmentId: channel.department_id ?? undefined,
    description: channel.description || "Chưa có mô tả",
    id: channel.id,
    isFavorite: Boolean(channel.is_favorite),
    isMember: Boolean(channel.is_member),
    membershipStatus: channel.membership_status ?? "none",
    privateSessionMode: Boolean(channel.private_session_mode),
    memberCount: channel.member_count ?? 0,
    messages: [],
    name: channel.name,
    relativeTime: formatConversationTime(channel.updated_at ?? channel.created_at),
    slug: channel.slug,
    tone: channelTones[index % channelTones.length],
    type: channel.type ?? channel.kind,
    unreadCount: channel.unread_count ?? 0
  };
}

function mapDirectConversation(
  item: ApiDirectConversation,
  presenceByUserId: Map<string, { status?: string }>,
  currentUserId: string,
  latestMessage?: ApiMessage,
  workspaceMembers: WorkspaceMember[] = [],
  contactProfileByUserId: ReadonlyMap<string, ContactUser> = new Map()
): DirectConversation | null {
  const participant = item.user ?? item.participants?.find((member) => member.user_id !== currentUserId) ?? item.participants?.[0];

  if (!participant) {
    return null;
  }

  const lastMessage = item.last_message ?? latestMessage;
  const workspaceMember = workspaceMembers.find((member) => member.user_id === participant.user_id);
  const contactProfile = contactProfileByUserId.get(participant.user_id);
  const participantName =
    participant.display_name ||
    contactProfile?.display_name ||
    workspaceMember?.display_name ||
    participant.username ||
    contactProfile?.username ||
    workspaceMember?.username ||
    participant.email ||
    contactProfile?.email ||
    workspaceMember?.email ||
    "Người dùng";

  return {
    id: item.channel_id ?? item.id,
    lastMessage: lastMessage?.body ?? "Chưa có tin nhắn",
    relativeTime: formatConversationTime(lastMessage?.created_at ?? lastMessage?.updated_at ?? item.updated_at),
    unreadCount: item.unread_count,
    user: {
      avatarUrl: participant.avatar_url || contactProfile?.avatar_url || workspaceMember?.avatar_url || undefined,
      email: participant.email || contactProfile?.email || workspaceMember?.email,
      id: participant.user_id,
      name: participantName,
      username: participant.username || contactProfile?.username || workspaceMember?.username,
      status: mapPresenceStatus(presenceByUserId.get(participant.user_id)?.status)
    }
  };
}

function directConversationToChannel(conversation: DirectConversation): ChatChannel {
  return {
    avatarUrl: conversation.user.avatarUrl,
    canManage: false,
    departmentId: undefined,
    description: "Tin nhắn riêng",
    id: conversation.id,
    isFavorite: false,
    isMember: true,
    membershipStatus: "active",
    memberCount: 2,
    messages: [],
    name: conversation.user.name,
    peerUserId: conversation.user.id,
    relativeTime: conversation.relativeTime,
    slug: undefined,
    tone: "purple",
    type: "direct",
    unreadCount: conversation.unreadCount ?? 0,
    userStatus: conversation.user.status
  };
}

function placeholderChannel(channelId: string): ChatChannel {
  return {
    canManage: false,
    departmentId: undefined,
    description: "Tin nhắn riêng",
    id: channelId,
    isFavorite: false,
    isMember: true,
    membershipStatus: "active",
    memberCount: 2,
    messages: [],
    name: "Hội thoại",
    relativeTime: "",
    slug: undefined,
    tone: "purple",
    type: "direct",
    unreadCount: 0
  };
}

function upsertDirectConversation(
  current: ApiDirectConversation[] | undefined,
  conversation: ApiDirectConversation
): ApiDirectConversation[] {
  const nextId = conversation.channel_id ?? conversation.id;
  const list = current ?? [];

  if (!nextId) {
    return list;
  }

  const exists = list.some((item) => (item.channel_id ?? item.id) === nextId);
  if (!exists) {
    return [conversation, ...list];
  }

  return list.map((item) => ((item.channel_id ?? item.id) === nextId ? { ...item, ...conversation } : item));
}

function updateDirectConversationLastMessage(
  current: ApiDirectConversation[] | undefined,
  channelId: string,
  message: ApiMessage
): ApiDirectConversation[] | undefined {
  if (!current?.length || !channelId) {
    return current;
  }

  return current.map((conversation) => {
    const conversationChannelId = conversation.channel_id ?? conversation.id;
    if (conversationChannelId !== channelId) {
      return conversation;
    }

    return {
      ...conversation,
      last_message: message,
      updated_at: message.updated_at ?? message.created_at ?? conversation.updated_at
    };
  });
}

function updateChannelAfterOwnMessage(
  current: ApiChannel[] | undefined,
  channelId: string,
  message: ApiMessage
): ApiChannel[] | undefined {
  if (!current?.length || !channelId) {
    return current;
  }

  return current.map((channel) => {
    if (channel.id !== channelId) {
      return channel;
    }

    return {
      ...channel,
      unread_count: 0,
      updated_at: message.updated_at ?? message.created_at ?? channel.updated_at
    };
  });
}

function markChannelCacheRead(current: ApiChannel[] | undefined, channelId: string): ApiChannel[] | undefined {
  if (!current?.length || !channelId) {
    return current;
  }

  return current.map((channel) => (channel.id === channelId ? { ...channel, unread_count: 0 } : channel));
}

function markDirectConversationCacheRead(
  current: ApiDirectConversation[] | undefined,
  channelId: string
): ApiDirectConversation[] | undefined {
  if (!current?.length || !channelId) {
    return current;
  }

  return current.map((conversation) =>
    (conversation.channel_id ?? conversation.id) === channelId ? { ...conversation, unread_count: 0 } : conversation
  );
}

function mapFile(file: FileObject): FileItem {
  const name = file.name ?? file.file_name ?? file.original_name ?? "File chưa đặt tên";
  const mimeType = file.mime_type;

  return {
    checksumSha256: file.checksum_sha256,
    downloadUrl: file.download_url ?? file.url,
    id: file.id,
    mimeType,
    name,
    size: formatFileSize(file.byte_size ?? file.size_bytes ?? file.size),
    status: file.status,
    tone: mimeType?.includes("pdf") ? "red" : mimeType?.startsWith("image/") ? "green" : "slate",
    updatedAt: formatRelative(file.updated_at ?? file.created_at)
  };
}

function mapChannelMediaAttachment(item: FileAttachment): MediaItem {
  const file = item.file;
  const name = file.name ?? file.file_name ?? file.original_name ?? "Ảnh";
  const attachment: MessageAttachmentItem = {
    checksumSha256: file.checksum_sha256,
    fileId: item.file_id || file.id,
    id: `${item.message_id}-${item.file_id || file.id}`,
    isImage: true,
    mimeType: file.mime_type,
    name,
    size: formatFileSize(file.byte_size ?? file.size_bytes ?? file.size),
    status: file.status,
    tone: "green",
    url: file.url ?? file.download_url
  };

  return {
    attachment,
    id: attachment.fileId,
    label: name,
    name,
    url: attachment.url
  };
}

function mapMembersWithPresence(
  members: WorkspaceMember[],
  presenceByUserId: Map<string, { status?: string }>
): WorkspaceMember[] {
  return members.map((member) => ({
    ...member,
    status: mapPresenceStatus(presenceByUserId.get(member.user_id)?.status)
  }));
}

function mapNotification(notification: ApiNotification): NotificationItem {
  const data = notification.data && typeof notification.data === "object" && !Array.isArray(notification.data)
    ? notification.data
    : undefined;
  const callMode = stringValue(data?.mode);
  const channelId = notification.channel_id ?? stringValue(data?.channel_id) ?? undefined;
  return {
    body: notification.body,
    callId: stringValue(data?.call_id) ?? undefined,
    callMode: callMode === "video" ? "video" : callMode === "audio" ? "audio" : undefined,
    callStatus: stringValue(data?.status) ?? undefined,
    channelId,
    createdAt: formatRelative(notification.created_at),
    data,
    id: notification.id,
    initiatorUserId: stringValue(data?.initiator_user_id) ?? undefined,
    isRead: Boolean(notification.read_at),
    messageId: notification.message_id ?? undefined,
    targetUserId: stringValue(data?.target_user_id) ?? undefined,
    title: notification.title,
    type: notification.type
  };
}

function stringValue(value: unknown): string | null {
  return typeof value === "string" && value.trim() ? value.trim() : null;
}

function mapPresenceStatus(status?: string): PresenceStatus {
  if (status === "online" || status === "active" || status === "away") {
    return "online";
  }

  return "offline";
}

function formatRelative(value?: string): string {
  if (!value) {
    return "Không rõ";
  }

  return new Intl.DateTimeFormat("vi-VN", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric"
  }).format(new Date(value));
}

function formatConversationTime(value?: string): string {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const startOfMessageDay = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const dayDifference = Math.round((startOfToday.getTime() - startOfMessageDay.getTime()) / 86_400_000);

  if (dayDifference === 0) {
    return new Intl.DateTimeFormat("vi-VN", { hour: "2-digit", minute: "2-digit", hour12: false }).format(date);
  }
  if (dayDifference === 1) {
    return "Hôm qua";
  }
  if (dayDifference > 1 && dayDifference < 7) {
    return `${dayDifference} ngày`;
  }
  return new Intl.DateTimeFormat("vi-VN", { day: "2-digit", month: "2-digit" }).format(date);
}

function formatFileSize(size?: number): string {
  if (!size) {
    return "0 B";
  }

  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }

  if (size < 1024 * 1024 * 1024) {
    return `${(size / (1024 * 1024)).toFixed(1)} MB`;
  }

  return `${(size / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}
