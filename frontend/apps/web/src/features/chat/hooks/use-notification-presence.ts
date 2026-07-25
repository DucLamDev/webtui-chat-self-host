"use client";

import { useEffect, useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@webtui/api-client";
import { getPlatformServices } from "@webtui/chat-core";
import type { Notification, Presence, PresenceHeartbeatInput } from "@webtui/types";
import { api } from "@/lib/api";

const heartbeatMs = 25_000;
const presenceRefetchMs = 10_000;
const notificationFallbackMs = 10_000;
const deviceIdStorageKey = "webtui-device-id";
let cachedDeviceId: string | null = null;
let cachedSocketId: string | null = null;

export type NotificationPresenceOptions = {
  currentUserId: string;
  enabled?: boolean;
  workspaceId: string;
};

export function useNotificationPresence({
  currentUserId,
  enabled = true,
  workspaceId
}: NotificationPresenceOptions) {
  const queryClient = useQueryClient();
  const isEnabled = Boolean(enabled && workspaceId);

  const notificationsQuery = useQuery({
    enabled: isEnabled,
    queryFn: () => api.notifications.listMine({ limit: 30, workspace_id: workspaceId }),
    queryKey: queryKeys.notifications.list(workspaceId),
    refetchInterval: notificationFallbackMs,
    refetchIntervalInBackground: true,
    retry: 2
  });

  const presenceQuery = useQuery({
    enabled: isEnabled,
    queryFn: () => api.presence.list(workspaceId, { limit: 100 }),
    queryKey: queryKeys.presence.list(workspaceId),
    refetchInterval: presenceRefetchMs,
    refetchIntervalInBackground: true,
    retry: false
  });

  const markNotificationReadMutation = useMutation({
    mutationFn: (notificationId: string) => api.notifications.markRead(notificationId),
    onSuccess: (notification) => {
      queryClient.setQueryData<Notification[]>(queryKeys.notifications.list(workspaceId), (current) =>
        current?.map((item) => (item.id === notification.id ? notification : item)) ?? current
      );
    }
  });

  const markAllNotificationsReadMutation = useMutation({
    mutationFn: () => api.notifications.markAllRead({ workspace_id: workspaceId }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.notifications.list(workspaceId) });
    }
  });

  useEffect(() => {
    if (!isEnabled || !currentUserId) {
      return undefined;
    }

    let cancelled = false;
    const deviceId = getPlatformDeviceId();
    const socketId = getPresenceSocketId(deviceId, currentUserId);

    async function sendHeartbeat(status: "online" | "away" | "offline") {
      const input: PresenceHeartbeatInput = {
        device_id: deviceId,
        metadata: {
          source: "web",
          visibility: typeof document === "undefined" ? "visible" : document.visibilityState
        },
        node_id: "web",
        socket_id: socketId,
        status
      };

      try {
        const presence = await api.presence.heartbeat(workspaceId, input);

        if (!cancelled) {
          upsertPresence(queryClient, workspaceId, presence);
        }
      } catch {
        // Presence is helpful context, not a hard blocker for chat.
      }
    }

    function heartbeat() {
      void sendHeartbeat("online");
    }

    heartbeat();
    const intervalId = window.setInterval(heartbeat, heartbeatMs);
    document.addEventListener("visibilitychange", heartbeat);
    window.addEventListener("online", heartbeat);

    return () => {
      cancelled = true;
      window.clearInterval(intervalId);
      document.removeEventListener("visibilitychange", heartbeat);
      window.removeEventListener("online", heartbeat);
      void sendHeartbeat("offline");
    };
  }, [currentUserId, isEnabled, queryClient, workspaceId]);

  const presenceByUserId = useMemo(() => {
    const map = new Map<string, Presence>();

    for (const presence of presenceQuery.data ?? []) {
      const current = map.get(presence.user_id);
      if (!current || comparePresence(presence, current) > 0) {
        map.set(presence.user_id, presence);
      }
    }

    return map;
  }, [presenceQuery.data]);

  const unreadNotificationsCount = useMemo(
    () => (notificationsQuery.data ?? []).filter((notification) => !notification.read_at).length,
    [notificationsQuery.data]
  );

  return {
    markAllNotificationsReadMutation,
    markNotificationReadMutation,
    notifications: notificationsQuery.data ?? [],
    notificationsQuery,
    presenceByUserId,
    presenceQuery,
    unreadNotificationsCount
  };
}

function comparePresence(next: Presence, current: Presence): number {
  const rankDiff = presenceRank(next.status) - presenceRank(current.status);
  if (rankDiff !== 0) {
    return rankDiff;
  }
  return Date.parse(next.last_heartbeat_at) - Date.parse(current.last_heartbeat_at);
}

function presenceRank(status: string | undefined): number {
  if (status === "online" || status === "active") {
    return 3;
  }
  if (status === "away") {
    return 2;
  }
  if (status === "busy") {
    return 1;
  }
  return 0;
}

function upsertPresence(queryClient: ReturnType<typeof useQueryClient>, workspaceId: string, presence: Presence) {
  queryClient.setQueryData<Presence[]>(queryKeys.presence.list(workspaceId), (current) => {
    const list = current ?? [];
    const exists = list.some(
      (item) => item.user_id === presence.user_id && item.device_id === presence.device_id
    );

    if (!exists) {
      return [presence, ...list];
    }

    return list.map((item) =>
      item.user_id === presence.user_id && item.device_id === presence.device_id ? presence : item
    );
  });
}

function getPlatformDeviceId() {
  if (typeof window === "undefined") {
    return "web-server";
  }
  if (cachedDeviceId) {
    return cachedDeviceId;
  }

  const existing = getPlatformServices().storage.getItem(deviceIdStorageKey);
  if (existing) {
    if (!(existing instanceof Promise)) {
      cachedDeviceId = existing;
      return existing;
    }
    void existing.then((value) => {
      if (value) {
        cachedDeviceId = value;
      }
    });
  }

  const next =
    typeof crypto !== "undefined" && "randomUUID" in crypto
      ? crypto.randomUUID()
      : `web-${Date.now()}-${Math.random().toString(16).slice(2)}`;

  void getPlatformServices().storage.setItem(deviceIdStorageKey, next, "persistent");
  cachedDeviceId = next;
  return next;
}

function getPresenceSocketId(deviceId: string, userId: string) {
  const prefix = `${deviceId}:${userId}:`;
  if (cachedSocketId?.startsWith(prefix)) {
    return cachedSocketId;
  }

  const next =
    typeof crypto !== "undefined" && "randomUUID" in crypto
      ? `${prefix}${crypto.randomUUID()}`
      : `${prefix}${Date.now()}-${Math.random().toString(16).slice(2)}`;
  cachedSocketId = next;
  return next;
}
