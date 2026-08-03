"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiClientError, type WebPushConfig } from "@webtui/api-client";
import { getPlatformServices } from "@webtui/chat-core";
import { Bell, Cloud, ShieldCheck } from "@webtui/icons";
import { Badge, Button } from "@webtui/ui";
import { api, runtimeEnvironment } from "@/lib/api";
import { useAuthStore } from "@/features/auth/auth-store";
import {
  cleanupWebPushForAccount,
  clearLocalWebPushRecord,
  currentWebPushSubscription,
  detectWebPushCapability,
  ensureWebPushServiceWorker,
  readLocalWebPushRecord,
  sameWebPushAccount,
  subscriptionInput,
  subscriptionUsesVapidKey,
  vapidPublicKeyBytes,
  writeLocalWebPushRecord,
  type WebPushAccount
} from "./web-push";

type WebPushState = "checking" | "disabled" | "enabled" | "server-disabled" | "unsupported";

export function WebPushSettings({ userId, workspaceId }: { userId: string; workspaceId?: string }) {
  const zoneApiBaseUrl = useAuthStore((state) => state.zoneRuntime?.api_base_url);
  const account = useMemo<WebPushAccount>(
    () => ({
      serverId: zoneApiBaseUrl ?? runtimeEnvironment.apiBaseUrl,
      userId
    }),
    [userId, zoneApiBaseUrl]
  );
  const [config, setConfig] = useState<WebPushConfig | null>(null);
  const [state, setState] = useState<WebPushState>("checking");
  const [feedback, setFeedback] = useState("");
  const [busy, setBusy] = useState(false);
  const isDesktop = getPlatformServices().lifecycle.isDesktop;

  const refresh = useCallback(async () => {
    if (!workspaceId || isDesktop) {
      return;
    }
    const capability = detectWebPushCapability();
    if (!capability.supported) {
      setFeedback(capability.reason ?? "Trình duyệt chưa hỗ trợ Web Push.");
      setState("unsupported");
      return;
    }
    setState("checking");
    setFeedback("");
    try {
      const nextConfig = await api.notifications.getWebPushConfig();
      setConfig(nextConfig);
      const record = await readLocalWebPushRecord();

      if (!nextConfig.enabled || !nextConfig.vapid_public_key?.trim()) {
        if (record && sameWebPushAccount(record, account)) {
          await cleanupWebPushForAccount(account, (id) => api.notifications.revokeWebPushSubscription(id)).catch(
            () => undefined
          );
        }
        setState("server-disabled");
        setFeedback("Instance này chưa bật VAPID Web Push.");
        return;
      }

      let subscription = await currentWebPushSubscription();
      if (record && !sameWebPushAccount(record, account)) {
        await subscription?.unsubscribe();
        await clearLocalWebPushRecord();
        subscription = null;
      }
      if (!record || !subscription) {
        if (record && sameWebPushAccount(record, account)) {
          await api.notifications.revokeWebPushSubscription(record.subscriptionId).catch(() => undefined);
          await clearLocalWebPushRecord();
        } else if (subscription) {
          await subscription.unsubscribe();
        }
        setState("disabled");
        return;
      }
      const applicationServerKey = subscription.options.applicationServerKey;
      if (
        applicationServerKey &&
        !subscriptionUsesVapidKey(subscription, vapidPublicKeyBytes(nextConfig.vapid_public_key))
      ) {
        await cleanupWebPushForAccount(account, (id) => api.notifications.revokeWebPushSubscription(id)).catch(
          () => undefined
        );
        setState("disabled");
        setFeedback("Instance đã đổi VAPID key. Hãy bật lại Web Push trên trình duyệt này.");
        return;
      }
      if (Notification.permission !== "granted") {
        await cleanupWebPushForAccount(account, (id) => api.notifications.revokeWebPushSubscription(id)).catch(
          () => undefined
        );
        setState("disabled");
        setFeedback(
          Notification.permission === "denied"
            ? "Quyền thông báo đang bị chặn trong cài đặt trình duyệt."
            : "Bật Web Push để nhận thông báo khi đóng tab."
        );
        return;
      }

      if (record.endpoint !== subscription.endpoint) {
        await api.notifications.revokeWebPushSubscription(record.subscriptionId).catch(() => undefined);
      }
      const stored = await api.notifications.registerWebPushSubscription(subscriptionInput(subscription, workspaceId));
      await writeLocalWebPushRecord(account, {
        endpoint: subscription.endpoint,
        subscriptionId: stored.id
      });
      setState("enabled");
      setFeedback("Trình duyệt này có thể nhận thông báo nền cho tài khoản trên instance hiện tại.");
    } catch (error) {
      setState("disabled");
      setFeedback(error instanceof Error ? error.message : "Không kiểm tra được trạng thái Web Push.");
    }
  }, [account, isDesktop, workspaceId]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  if (isDesktop || !workspaceId) {
    return null;
  }

  async function enable() {
    if (!workspaceId || busy) {
      return;
    }
    setBusy(true);
    setFeedback("");
    let createdSubscription: PushSubscription | null = null;
    try {
      const capability = detectWebPushCapability();
      if (!capability.supported) {
        throw new Error(capability.reason);
      }
      const nextConfig = config?.enabled ? config : await api.notifications.getWebPushConfig();
      setConfig(nextConfig);
      if (!nextConfig.enabled || !nextConfig.vapid_public_key?.trim()) {
        setState("server-disabled");
        throw new Error("Instance này chưa bật VAPID Web Push.");
      }
      const permission = await Notification.requestPermission();
      if (permission !== "granted") {
        setState("disabled");
        throw new Error("Bạn cần cho phép thông báo trong trình duyệt để bật Web Push.");
      }

      const registration = await ensureWebPushServiceWorker();
      const publicKey = vapidPublicKeyBytes(nextConfig.vapid_public_key);
      const localRecord = await readLocalWebPushRecord();
      let subscription = await registration.pushManager.getSubscription();
      if (subscription && !subscriptionUsesVapidKey(subscription, publicKey)) {
        if (localRecord && sameWebPushAccount(localRecord, account)) {
          await api.notifications.revokeWebPushSubscription(localRecord.subscriptionId).catch(() => undefined);
        }
        await subscription.unsubscribe();
        await clearLocalWebPushRecord();
        subscription = null;
      }
      if (localRecord && !sameWebPushAccount(localRecord, account)) {
        await subscription?.unsubscribe();
        await clearLocalWebPushRecord();
        subscription = null;
      }
      if (!subscription) {
        subscription = await registration.pushManager.subscribe({
          applicationServerKey: publicKey,
          userVisibleOnly: true
        });
        createdSubscription = subscription;
      }

      const stored = await api.notifications.registerWebPushSubscription(subscriptionInput(subscription, workspaceId));
      await writeLocalWebPushRecord(account, {
        endpoint: subscription.endpoint,
        subscriptionId: stored.id
      });
      setState("enabled");
      setFeedback("Đã bật Web Push cho tài khoản trên instance hiện tại.");
    } catch (error) {
      if (createdSubscription) {
        await createdSubscription.unsubscribe().catch(() => false);
      }
      setFeedback(error instanceof Error ? error.message : "Không bật được Web Push.");
    } finally {
      setBusy(false);
    }
  }

  async function disable() {
    if (!workspaceId || busy) {
      return;
    }
    setBusy(true);
    setFeedback("");
    let serverCleanupFailed = false;
    try {
      const record = await readLocalWebPushRecord();
      const subscription = await currentWebPushSubscription();
      if (record && sameWebPushAccount(record, account)) {
        try {
          await api.notifications.revokeWebPushSubscription(record.subscriptionId);
        } catch (error) {
          if (!(error instanceof ApiClientError && error.status === 404)) {
            serverCleanupFailed = true;
          }
        }
      } else if (subscription && config?.enabled) {
        try {
          const stored = await api.notifications.registerWebPushSubscription(
            subscriptionInput(subscription, workspaceId)
          );
          await api.notifications.revokeWebPushSubscription(stored.id);
        } catch {
          serverCleanupFailed = true;
        }
      }
      await subscription?.unsubscribe();
      await clearLocalWebPushRecord();
      setState("disabled");
      setFeedback(
        serverCleanupFailed
          ? "Đã tắt trên trình duyệt; server sẽ tự thu hồi endpoint không còn hợp lệ."
          : "Đã tắt Web Push trên trình duyệt này."
      );
    } catch (error) {
      setFeedback(error instanceof Error ? error.message : "Không tắt được Web Push.");
    } finally {
      setBusy(false);
    }
  }

  const enabled = state === "enabled";
  const canEnable = state === "disabled";
  const badge = webPushBadge(state);

  return (
    <div className="web-push-settings" aria-busy={busy}>
      <div className="web-push-settings__heading">
        <span aria-hidden="true"><Cloud size={18} /></span>
        <div>
          <strong>Thông báo nền trên web</strong>
          <small>Nhận thông báo ngay cả khi tab chat đã đóng.</small>
        </div>
        <Badge tone={badge.tone}>{badge.label}</Badge>
      </div>
      {feedback ? <p aria-live="polite" role="status">{feedback}</p> : null}
      <div className="web-push-settings__actions">
        {enabled ? (
          <Button disabled={busy} onClick={() => void disable()} size="sm" type="button" variant="secondary">
            <Bell size={15} /> {busy ? "Đang tắt..." : "Tắt Web Push"}
          </Button>
        ) : canEnable ? (
          <Button disabled={busy || !workspaceId} onClick={() => void enable()} size="sm" type="button" variant="secondary">
            <ShieldCheck size={15} />
            {busy ? "Đang bật..." : "Bật Web Push"}
          </Button>
        ) : null}
      </div>
    </div>
  );
}

function webPushBadge(state: WebPushState): { label: string; tone: "blue" | "green" | "red" | "slate" } {
  switch (state) {
    case "enabled":
      return { label: "Đã bật", tone: "green" };
    case "checking":
      return { label: "Đang kiểm tra", tone: "blue" };
    case "server-disabled":
      return { label: "Server chưa bật", tone: "slate" };
    case "unsupported":
      return { label: "Không hỗ trợ", tone: "red" };
    default:
      return { label: "Đang tắt", tone: "slate" };
  }
}
