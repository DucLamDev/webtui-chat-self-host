"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { createRealtimeGateway, queryKeys, type RealtimeServerEvent } from "@webtui/api-client";
import type { Message as ApiMessage } from "@webtui/types";
import { runtimeEnvironment } from "@/lib/api";
import { useAuthStore } from "@/features/auth/auth-store";
import { useRealtimeStore } from "../stores/realtime-store";
import {
  mergeMessageIntoTimeline,
  messageRoomName,
  removeMessageFromTimeline
} from "./use-message-timeline";

type RealtimeMessagePayload = {
  call?: CallSignalPayload;
  signal?: Pick<CallSignalPayload, "candidate" | "sdp">;
  actor_user_id?: string;
  call_id?: string;
  candidate?: RTCIceCandidateInit;
  channel_id?: string;
  initiator_user_id?: string;
  mode?: CallMode;
  reason?: string;
  sdp?: RTCSessionDescriptionInit;
  status?: string;
  target_user_id?: string;
  contact_request?: unknown;
  message_id?: string;
  message?: ApiMessage;
  workspace_id?: string;
  user_id?: string;
};

export type CallMode = "audio" | "video";

export type CallSignalType =
  | "CallInvited"
  | "CallRinging"
  | "CallAccepted"
  | "CallReady"
  | "CallOffer"
  | "CallAnswer"
  | "CallIceCandidate"
  | "CallRejected"
  | "CallCancelled"
  | "CallEnded"
  | "CallMissed";

export type CallSignalPayload = {
  actor_user_id?: string;
  call_id: string;
  candidate?: RTCIceCandidateInit;
  channel_id?: string;
  initiator_user_id?: string;
  mode?: CallMode;
  reason?: string;
  sdp?: RTCSessionDescriptionInit;
  status?: string;
  target_user_id?: string;
  workspace_id?: string;
};

export type RealtimeCallSignal = {
  payload: CallSignalPayload;
  room: string;
  sequence: number;
  timestamp?: string;
  type: CallSignalType;
  userId: string;
};

export type ChannelRealtimeOptions = {
  channelId: string;
  channelIds?: string[];
  currentUserId?: string;
  enabled?: boolean;
  workspaceId: string;
};

export function useChannelRealtime({
  channelId,
  channelIds = [],
  currentUserId,
  enabled = true,
  workspaceId
}: ChannelRealtimeOptions) {
  const accessToken = useAuthStore((state) => state.accessToken);
  const zoneWebSocketBaseUrl = useAuthStore(
    (state) => state.zoneRuntime?.ws_base_url
  );
  const queryClient = useQueryClient();
  const setConnection = useRealtimeStore((state) => state.setConnection);
  const status = useRealtimeStore((state) => state.status);
  const retryAttempt = useRealtimeStore((state) => state.retryAttempt);
  const lastEventAt = useRealtimeStore((state) => state.lastEventAt);
  const gateway = useMemo(
    () =>
      createRealtimeGateway(
        zoneWebSocketBaseUrl ?? runtimeEnvironment.wsBaseUrl
      ),
    [zoneWebSocketBaseUrl]
  );
  const socketRef = useRef<WebSocket | null>(null);
  const typingTimersRef = useRef(new Map<string, ReturnType<typeof setTimeout>>());
  const [typingEntries, setTypingEntries] = useState<Array<{ room: string; userId: string }>>([]);
  const [lastCallSignal, setLastCallSignal] = useState<RealtimeCallSignal | null>(null);
  const callSignalSequenceRef = useRef(0);
  const [lifecycleVersion, setLifecycleVersion] = useState(0);
  const room = workspaceId && channelId ? messageRoomName(workspaceId, channelId) : "";
  const channelIdsKey = [...new Set(channelIds.filter(Boolean))].sort().join("|");
  const rooms = useMemo(
    () => channelIdsKey.split("|").filter(Boolean).map((id) => messageRoomName(workspaceId, id)),
    [channelIdsKey, workspaceId]
  );
  const typingUserIds = useMemo(
    () => typingEntries.filter((entry) => entry.room === room).map((entry) => entry.userId),
    [room, typingEntries]
  );

  useEffect(() => {
    if (!enabled || !workspaceId || typeof window === "undefined") {
      return undefined;
    }

    const invalidateRealtimeQueries = () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.channels.all(workspaceId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.channels.directConversations(workspaceId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.notifications.list(workspaceId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.presence.list(workspaceId) });
      for (const id of channelIdsKey.split("|")) {
        if (id) {
          void queryClient.invalidateQueries({ queryKey: queryKeys.messages.channel(workspaceId, id) });
        }
      }
    };

    const handleResume = () => {
      if (typeof document !== "undefined" && document.visibilityState === "hidden") {
        return;
      }
      if (typeof navigator !== "undefined" && navigator.onLine === false) {
        return;
      }
      invalidateRealtimeQueries();
      setLifecycleVersion((version) => version + 1);
    };

    const handleOffline = () => {
      setConnection({
        retryAttempt,
        room: room || null,
        status: "offline"
      });
    };

    window.addEventListener("online", handleResume);
    window.addEventListener("focus", handleResume);
    window.addEventListener("offline", handleOffline);
    document.addEventListener("visibilitychange", handleResume);

    return () => {
      window.removeEventListener("online", handleResume);
      window.removeEventListener("focus", handleResume);
      window.removeEventListener("offline", handleOffline);
      document.removeEventListener("visibilitychange", handleResume);
    };
  }, [channelIdsKey, enabled, queryClient, retryAttempt, room, setConnection, workspaceId]);

  useEffect(() => {
    if (!enabled || !workspaceId || !accessToken || typeof WebSocket === "undefined") {
      setConnection({
        retryAttempt: 0,
        room: null,
        status: "idle"
      });
      return undefined;
    }

    const token = accessToken;
    let disposed = false;
    let socket: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let attempt = 0;

    function clearReconnectTimer() {
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
    }

    function connect() {
      if (disposed) {
        return;
      }

      socket = gateway.connect({
        accessToken: token,
        workspaceId
      });
      socketRef.current = socket;

      setConnection({
        retryAttempt: attempt,
        room: room || null,
        status: attempt > 0 ? "reconnecting" : "connecting"
      });

      socket.addEventListener("open", () => {
        if (!socket || disposed) {
          return;
        }

        attempt = 0;
        rooms.forEach((nextRoom) => gateway.join(socket as WebSocket, nextRoom));
        setConnection({
          retryAttempt: 0,
          room: room || null,
          status: "connected"
        });
      });

      socket.addEventListener("message", (event) => {
        if (disposed) {
          return;
        }

        handleRealtimeMessage(event.data);
      });

      socket.addEventListener("error", () => {
        socket?.close();
      });

      socket.addEventListener("close", () => {
        if (disposed) {
          return;
        }

        attempt += 1;
        const delay = Math.min(15000, 1000 * 2 ** Math.min(attempt, 4));
        setConnection({
          retryAttempt: attempt,
          room: room || null,
          status: "reconnecting"
        });
        reconnectTimer = setTimeout(connect, delay);
      });
    }

    function handleRealtimeMessage(raw: string) {
      const event = parseRealtimeEvent(raw);
      if (!event) {
        return;
      }
      const callPayload = isCallSignalType(event.type) ? normalizeCallSignalPayload(event.payload) : null;
      const signalChannelId = callPayload?.channel_id || channelIdFromRoom(event.room);
      const isKnownCallEvent = Boolean(
        callPayload && (!signalChannelId || signalChannelId === channelId || channelIdsKey.split("|").includes(signalChannelId))
      );
      const isChannelEvent = !event.room || rooms.includes(event.room);
      const isUserEvent = event?.room?.startsWith("user:");

      if (!isChannelEvent && !isUserEvent && !isKnownCallEvent) {
        return;
      }

      if (event.type === "TypingStarted" || event.type === "TypingStopped") {
        const typingUserId = event.user_id || event.payload?.user_id;
        if (!typingUserId || typingUserId === currentUserId) {
          return;
        }
        const typingRoom = event.room || room;
        if (!typingRoom || !rooms.includes(typingRoom)) {
          return;
        }
        const typingKey = `${typingRoom}:${typingUserId}`;
        const currentTimer = typingTimersRef.current.get(typingKey);
        if (currentTimer) {
          clearTimeout(currentTimer);
          typingTimersRef.current.delete(typingKey);
        }
        setTypingEntries((current) =>
          event.type === "TypingStarted"
            ? current.some((entry) => entry.room === typingRoom && entry.userId === typingUserId)
              ? current
              : [...current, { room: typingRoom, userId: typingUserId }]
            : current.filter((entry) => entry.room !== typingRoom || entry.userId !== typingUserId)
        );
        if (event.type === "TypingStarted") {
          typingTimersRef.current.set(
            typingKey,
            setTimeout(() => {
              setTypingEntries((current) => current.filter((entry) => entry.room !== typingRoom || entry.userId !== typingUserId));
              typingTimersRef.current.delete(typingKey);
            }, 4_000)
          );
        }
        return;
      }

      if (isCallSignalType(event.type)) {
        const payload = callPayload;
        if (!payload?.call_id) {
          return;
        }
        if (
          event.type === "CallRejected" ||
          event.type === "CallCancelled" ||
          event.type === "CallEnded" ||
          event.type === "CallMissed"
        ) {
          const terminalChannelId = payload.channel_id || signalChannelId || channelId;
          if (terminalChannelId) {
            window.setTimeout(() => {
              void queryClient.invalidateQueries({
                queryKey: queryKeys.messages.channel(workspaceId, terminalChannelId)
              });
              void queryClient.invalidateQueries({
                queryKey: queryKeys.channels.directConversations(workspaceId)
              });
              void queryClient.invalidateQueries({ queryKey: queryKeys.channels.all(workspaceId) });
            }, 250);
          }
        }
        const actorUserId = payload.actor_user_id || event.user_id || event.payload?.user_id || payload.initiator_user_id || "";
        if (!actorUserId || actorUserId === currentUserId || !event.room) {
          return;
        }
        callSignalSequenceRef.current += 1;
        setLastCallSignal({
          payload,
          room: event.room,
          sequence: callSignalSequenceRef.current,
          timestamp: event.timestamp,
          type: event.type,
          userId: actorUserId
        });
        setConnection({
          lastEventAt: new Date().toISOString(),
          room: room || null,
          status: "connected"
        });
        return;
      }

      if (
        event.type === "ContactRequestCreated" ||
        event.type === "ContactRequestUpdated" ||
        event.type === "ContactRequestCancelled"
      ) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.contacts.all });
        void queryClient.invalidateQueries({ queryKey: queryKeys.contacts.requests("all") });
        void queryClient.invalidateQueries({ queryKey: queryKeys.contacts.requests("pending") });
        void queryClient.invalidateQueries({ queryKey: queryKeys.notifications.list(workspaceId) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.notifications.list() });
        setConnection({
          lastEventAt: new Date().toISOString(),
          room: room || null,
          status: "connected"
        });
        return;
      }

      if (isUserEvent || event.type === "NotificationCreated" || event.type === "NotificationUpdated") {
        void queryClient.invalidateQueries({ queryKey: queryKeys.notifications.list(workspaceId) });
      }

      if (event.type === "AttachmentCreated") {
        const eventChannelId = event.payload?.channel_id || channelIdFromRoom(event.room) || channelId;
        const messageId = event.payload?.message_id;
        if (eventChannelId && messageId) {
          void queryClient.invalidateQueries({
            queryKey: queryKeys.files.attachments(workspaceId, eventChannelId, messageId)
          });
          void queryClient.invalidateQueries({
            queryKey: queryKeys.files.channelMedia(workspaceId, eventChannelId)
          });
        }
        setConnection({
          lastEventAt: new Date().toISOString(),
          room: room || null,
          status: "connected"
        });
        return;
      }

      const message = event.payload?.message;
      if (!message?.id) {
        return;
      }

      const eventChannelId = message.channel_id || channelId;
      if (!eventChannelId) {
        return;
      }

      if (event.type === "MessageDeleted") {
        removeMessageFromTimeline(queryClient, workspaceId, eventChannelId, message.id);
      } else if (event.type === "MessageCreated" || event.type === "MessageUpdated" || event.type === "ReactionChanged") {
        mergeMessageIntoTimeline(queryClient, workspaceId, eventChannelId, message);
      } else if (event.type === "MessagePinned" || event.type === "MessageUnpinned") {
        mergeMessageIntoTimeline(queryClient, workspaceId, eventChannelId, message);
        void queryClient.invalidateQueries({ queryKey: queryKeys.messages.pins(workspaceId, eventChannelId) });
      }

      void queryClient.invalidateQueries({ queryKey: ["direct-conversation-summary", workspaceId, eventChannelId] });
      void queryClient.invalidateQueries({ queryKey: queryKeys.channels.directConversations(workspaceId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.channels.all(workspaceId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.notifications.list(workspaceId) });
      window.setTimeout(() => {
        void queryClient.invalidateQueries({ queryKey: queryKeys.notifications.list(workspaceId) });
      }, 1_500);

      setConnection({
        lastEventAt: new Date().toISOString(),
        room: room || null,
        status: "connected"
      });
    }

    connect();

    return () => {
      disposed = true;
      clearReconnectTimer();
      if (socket && socket.readyState === WebSocket.OPEN) {
        rooms.forEach((nextRoom) => gateway.leave(socket as WebSocket, nextRoom));
      }
      socket?.close();
      if (socketRef.current === socket) {
        socketRef.current = null;
      }
      typingTimersRef.current.forEach((timer) => clearTimeout(timer));
      typingTimersRef.current.clear();
      setTypingEntries([]);
      setConnection({
        retryAttempt: 0,
        room: null,
        status: "offline"
      });
    };
  }, [accessToken, channelId, currentUserId, enabled, gateway, lifecycleVersion, queryClient, room, rooms, setConnection, workspaceId]);

  const publishTyping = useCallback(
    (active: boolean) => {
      const socket = socketRef.current;
      if (!socket || socket.readyState !== WebSocket.OPEN || !room) {
        return false;
      }
      return gateway.send(socket, { room, type: active ? "TypingStarted" : "TypingStopped" });
    },
    [gateway, room]
  );

  return {
    lastEventAt,
    lastCallSignal,
    publishTyping,
    retryAttempt,
    room,
    status,
    typingUserIds
  };
}

function parseRealtimeEvent(raw: string): RealtimeServerEvent<RealtimeMessagePayload> | null {
  try {
    const parsed = JSON.parse(raw) as RealtimeServerEvent<RealtimeMessagePayload>;

    if (!parsed || typeof parsed.type !== "string") {
      return null;
    }

    return parsed;
  } catch {
    return null;
  }
}

function isCallSignalType(type: string): type is CallSignalType {
  return (
    type === "CallInvited" ||
    type === "CallRinging" ||
    type === "CallAccepted" ||
    type === "CallReady" ||
    type === "CallOffer" ||
    type === "CallAnswer" ||
    type === "CallIceCandidate" ||
    type === "CallRejected" ||
    type === "CallCancelled" ||
    type === "CallEnded" ||
    type === "CallMissed"
  );
}

function normalizeCallSignalPayload(payload: RealtimeMessagePayload | undefined): CallSignalPayload | null {
  if (!payload) {
    return null;
  }
  const nested = payload.call;
  const source = nested?.call_id ? nested : payload;
  const signal = payload.signal;
  if (!source.call_id) {
    return null;
  }
  return {
    actor_user_id: source.actor_user_id,
    call_id: source.call_id,
    candidate: signal?.candidate ?? source.candidate,
    channel_id: source.channel_id,
    initiator_user_id: source.initiator_user_id,
    mode: source.mode,
    reason: source.reason,
    sdp: signal?.sdp ?? source.sdp,
    status: source.status,
    target_user_id: source.target_user_id,
    workspace_id: source.workspace_id
  };
}

function channelIdFromRoom(room?: string): string {
  if (!room) {
    return "";
  }
  const parts = room.split(":");
  return parts.length === 4 && parts[0] === "workspace" && parts[2] === "channel" ? parts[3] : "";
}
