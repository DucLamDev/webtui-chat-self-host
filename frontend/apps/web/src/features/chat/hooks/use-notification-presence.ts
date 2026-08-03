"use client";

import { useEffect, useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@webtui/api-client";
import { getPlatformServices } from "@webtui/chat-core";
import type { Notification, Presence, PresenceHeartbeatInput } from "@webtui/types";
import { api } from "@/lib/api";
import { useRealtimeStore } from "../stores/realtime-store";

const heartbeatMs = 25_000;
const presenceRefetchMs = 10_000;
const notificationFallbackMs = 10_000;
const notificationConnectedSafetyMs = 2 * 60_000;
const deviceIdStorageKey = "webtui-device-id";
let cachedDeviceId: string | null = null;
let cachedDeviceIdPromise: Promise<string> | null = null;
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
  const realtimeStatus = useRealtimeStore((state) => state.status);

  const notificationsQuery = useQuery({
    enabled: isEnabled,
    queryFn: () => api.notifications.listMine({ limit: 30, workspace_id: workspaceId }),
    queryKey: queryKeys.notifications.list(workspaceId),
    refetchInterval:
      realtimeStatus === "connected" ? notificationConnectedSafetyMs : notificationFallbackMs,
    refetchIntervalInBackground: false,
    retry: 2
  });

  const presenceQuery = useQuery({
    enabled: isEnabled,
    queryFn: () => api.presence.list(workspaceId, { limit: 100 }),
    queryKey: queryKeys.presence.list(workspaceId),
    refetchInterval: presenceRefetchMs,
    refetchIntervalInBackground: false,
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
    const deviceIdPromise = getPlatformDeviceId();

    async function sendHeartbeat(status: "online" | "away" | "offline") {
      const deviceId = await deviceIdPromise;
      if (cancelled && status !== "offline") {
        return;
      }
      const socketId = getPresenceSocketId(deviceId, currentUserId);
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
      if (document.visibilityState === "visible") {
        void sendHeartbeat("online");
      }
    }

    let intervalId: number | undefined;

    function stopHeartbeatLoop() {
      if (intervalId !== undefined) {
        window.clearInterval(intervalId);
        intervalId = undefined;
      }
    }

    function startHeartbeatLoop() {
      stopHeartbeatLoop();
      if (document.visibilityState === "visible") {
        heartbeat();
        intervalId = window.setInterval(heartbeat, heartbeatMs);
      }
    }

    function handleVisibilityChange() {
      if (document.visibilityState === "hidden") {
        stopHeartbeatLoop();
        void sendHeartbeat("away");
        return;
      }
      startHeartbeatLoop();
    }

    startHeartbeatLoop();
    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("online", startHeartbeatLoop);

    return () => {
      cancelled = true;
      stopHeartbeatLoop();
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("online", startHeartbeatLoop);
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

function getPlatformDeviceId(): Promise<string> {
  if (typeof window === "undefined") {
    return Promise.resolve("web-server");
  }
  if (cachedDeviceId) {
    return Promise.resolve(cachedDeviceId);
  }
  if (cachedDeviceIdPromise) {
    return cachedDeviceIdPromise;
  }

  cachedDeviceIdPromise = (async () => {
    const storage = getPlatformServices().storage;
    try {
      const existing = (await storage.getItem(deviceIdStorageKey))?.trim();
      if (existing) {
        cachedDeviceId = existing;
        return existing;
      }
    } catch {
      // Presence can continue with an in-memory identifier if secure storage is unavailable.
    }

    const next =
      typeof crypto !== "undefined" && "randomUUID" in crypto
        ? crypto.randomUUID()
        : `web-${Date.now()}-${Math.random().toString(16).slice(2)}`;
    try {
      await storage.setItem(deviceIdStorageKey, next, "persistent");
    } catch {
      // Keep the identifier stable for the current runtime even if persistence fails.
    }
    cachedDeviceId = next;
    return next;
  })();

  return cachedDeviceIdPromise;
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
