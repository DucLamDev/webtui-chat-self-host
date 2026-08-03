"use client";

import { getPlatformServices, type PlatformStorage } from "@webtui/chat-core";
import type { WebPushSubscriptionInput } from "@webtui/api-client";
import { normalizeServerId } from "../chat/model/message-outbox";

const localRecordKey = "webtui:web-push:v1:active-subscription";

export type WebPushAccount = {
  serverId: string;
  userId: string;
};

export type LocalWebPushRecord = WebPushAccount & {
  endpoint: string;
  subscriptionId: string;
  updatedAt: string;
};

export type WebPushCapability = {
  reason?: string;
  supported: boolean;
};

function platformStorage() {
  return getPlatformServices().storage;
}

export function detectWebPushCapability(): WebPushCapability {
  if (typeof window === "undefined" || typeof navigator === "undefined") {
    return { reason: "Web Push chỉ khả dụng trong trình duyệt.", supported: false };
  }
  if (!window.isSecureContext) {
    return { reason: "Web Push yêu cầu HTTPS (hoặc localhost khi phát triển).", supported: false };
  }
  if (!("serviceWorker" in navigator) || !("PushManager" in window) || !("Notification" in window)) {
    return { reason: "Trình duyệt này chưa hỗ trợ Service Worker và Push API.", supported: false };
  }
  return { supported: true };
}

export function normalizeWebPushAccount(account: WebPushAccount): WebPushAccount {
  return {
    serverId: normalizeServerId(account.serverId),
    userId: account.userId.trim()
  };
}

export function sameWebPushAccount(record: LocalWebPushRecord, account: WebPushAccount) {
  return (
    normalizeServerId(record.serverId) === normalizeServerId(account.serverId) &&
    record.userId === account.userId.trim()
  );
}

export async function readLocalWebPushRecord(
  storage: PlatformStorage = platformStorage()
): Promise<LocalWebPushRecord | null> {
  const raw = await storage.getItem(localRecordKey);
  if (!raw) {
    return null;
  }
  try {
    const record = JSON.parse(raw) as Partial<LocalWebPushRecord>;
    if (
      typeof record.serverId !== "string" ||
      typeof record.userId !== "string" ||
      typeof record.endpoint !== "string" ||
      typeof record.subscriptionId !== "string" ||
      typeof record.updatedAt !== "string"
    ) {
      return null;
    }
    return {
      endpoint: record.endpoint,
      serverId: normalizeServerId(record.serverId),
      subscriptionId: record.subscriptionId,
      updatedAt: record.updatedAt,
      userId: record.userId.trim()
    };
  } catch {
    return null;
  }
}

export async function writeLocalWebPushRecord(
  account: WebPushAccount,
  input: { endpoint: string; subscriptionId: string },
  storage: PlatformStorage = platformStorage()
): Promise<LocalWebPushRecord> {
  const record: LocalWebPushRecord = {
    ...normalizeWebPushAccount(account),
    endpoint: input.endpoint,
    subscriptionId: input.subscriptionId,
    updatedAt: new Date().toISOString()
  };
  await storage.setItem(localRecordKey, JSON.stringify(record), "persistent");
  return record;
}

export async function clearLocalWebPushRecord(storage: PlatformStorage = platformStorage()) {
  await storage.removeItem(localRecordKey);
}

export async function ensureWebPushServiceWorker(): Promise<ServiceWorkerRegistration> {
  const capability = detectWebPushCapability();
  if (!capability.supported) {
    throw new Error(capability.reason);
  }
  await navigator.serviceWorker.register("/sw.js", {
    scope: "/",
    updateViaCache: "none"
  });
  return navigator.serviceWorker.ready;
}

export async function currentWebPushSubscription(): Promise<PushSubscription | null> {
  if (typeof navigator === "undefined" || !("serviceWorker" in navigator)) {
    return null;
  }
  const registration = await navigator.serviceWorker.getRegistration("/");
  return registration?.pushManager.getSubscription() ?? null;
}

export function subscriptionInput(subscription: PushSubscription, workspaceId: string): WebPushSubscriptionInput {
  const json = subscription.toJSON();
  const p256dh = json.keys?.p256dh?.trim();
  const auth = json.keys?.auth?.trim();
  if (!subscription.endpoint || !p256dh || !auth) {
    throw new Error("Trình duyệt không trả về đủ khóa Web Push.");
  }
  const expirationTime = subscription.expirationTime;
  return {
    endpoint: subscription.endpoint,
    ...(typeof expirationTime === "number" && expirationTime > Date.now()
      ? { expiration_time: new Date(expirationTime).toISOString() }
      : {}),
    keys: { auth, p256dh },
    workspace_id: workspaceId
  };
}

export function vapidPublicKeyBytes(value: string): Uint8Array<ArrayBuffer> {
  const normalized = value.trim().replace(/-/g, "+").replace(/_/g, "/");
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=");
  let decoded: string;
  try {
    decoded = atob(padded);
  } catch {
    throw new Error("VAPID public key của instance không hợp lệ.");
  }
  if (!decoded.length) {
    throw new Error("VAPID public key của instance đang trống.");
  }
  return Uint8Array.from(decoded, (character) => character.charCodeAt(0));
}

export function subscriptionUsesVapidKey(subscription: PushSubscription, publicKey: Uint8Array<ArrayBuffer>) {
  const existing = subscription.options.applicationServerKey;
  if (!existing) {
    return false;
  }
  const bytes = new Uint8Array(existing);
  return bytes.length === publicKey.length && bytes.every((value, index) => value === publicKey[index]);
}

export async function cleanupWebPushForAccount(
  account: WebPushAccount,
  revoke?: (subscriptionId: string) => Promise<unknown>,
  storage: PlatformStorage = platformStorage()
) {
  const record = await readLocalWebPushRecord(storage);
  if (!record || !sameWebPushAccount(record, account)) {
    return;
  }
  let revokeError: unknown;
  if (revoke) {
    try {
      await revoke(record.subscriptionId);
    } catch (error) {
      revokeError = error;
    }
  }
  try {
    const subscription = await currentWebPushSubscription();
    await subscription?.unsubscribe();
  } finally {
    await clearLocalWebPushRecord(storage);
  }
  if (revokeError) {
    throw revokeError;
  }
}
