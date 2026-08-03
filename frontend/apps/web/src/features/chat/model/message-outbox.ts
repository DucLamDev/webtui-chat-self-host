"use client";

import { ApiClientError } from "@webtui/api-client";
import type { MessageReplyPreview } from "./types";

const databaseName = "webtui-chat-offline";
const databaseVersion = 1;
const outboxStoreName = "message-outbox";
const syncStoreName = "sync-checkpoints";
const accountIndexName = "account-key";
const scopeIndexName = "scope-key";
const maxMessagesPerScope = 100;
const defaultLeaseMs = 30_000;
const maxBackoffMs = 60_000;
const changeEventName = "webtui:offline-outbox-changed";
const broadcastChannelName = "webtui-offline-v1";

export type OfflineAccount = {
  serverId: string;
  userId: string;
};

export type MessageOutboxScope = OfflineAccount & {
  workspaceId: string;
};

export type MessageOutboxStatus = "pending" | "sending" | "failed";

export type MessageOutboxEntry = {
  accountKey: string;
  attempts: number;
  body: string;
  channelId: string;
  clientMessageId: string;
  createdAt: string;
  id: string;
  lastError?: string;
  leaseExpiresAt?: number;
  leaseOwner?: string;
  mentionedUserIds?: string[];
  nextAttemptAt?: number;
  operation: "message.send";
  parentId?: string;
  replyTo?: MessageReplyPreview;
  retryable: boolean;
  scopeKey: string;
  serverId: string;
  status: MessageOutboxStatus;
  updatedAt: string;
  userId: string;
  workspaceId: string;
};

export type SyncCheckpoint = {
  accountKey: string;
  cursor?: string;
  id: string;
  recentEventIds: string[];
  scopeKey: string;
  serverId: string;
  updatedAt: string;
  userId: string;
  workspaceId: string;
};

export type OutboxErrorPolicy = {
  message: string;
  retryable: boolean;
  status?: number;
};

export type EnqueueMessageInput = {
  body: string;
  channelId: string;
  clientMessageId: string;
  mentionedUserIds?: string[];
  parentId?: string;
  replyTo?: MessageReplyPreview;
};

export type FlushMessageOutboxOptions = {
  force?: boolean;
  leaseOwner?: string;
  now?: () => number;
  onSent?: (entry: MessageOutboxEntry, result: unknown) => void | Promise<void>;
  onUpdated?: (entries: MessageOutboxEntry[]) => void | Promise<void>;
  random?: () => number;
  send: (entry: MessageOutboxEntry) => Promise<unknown>;
};

export interface OfflineMessageRepository {
  claimNextMessage(
    scopeKey: string,
    owner: string,
    now: number,
    leaseMs: number,
    force: boolean
  ): Promise<MessageOutboxEntry | null>;
  deleteAccount(accountKey: string): Promise<void>;
  deleteMessage(id: string): Promise<void>;
  getMessage(id: string): Promise<MessageOutboxEntry | null>;
  getSyncCheckpoint(id: string): Promise<SyncCheckpoint | null>;
  listMessages(scopeKey: string): Promise<MessageOutboxEntry[]>;
  putMessage(entry: MessageOutboxEntry): Promise<void>;
  putSyncCheckpoint(checkpoint: SyncCheckpoint): Promise<void>;
}

let repository: OfflineMessageRepository | undefined;
let broadcastChannel: BroadcastChannel | undefined;

export function messageOutboxScope(input: MessageOutboxScope): MessageOutboxScope {
  const serverId = normalizeServerId(input.serverId);
  const userId = input.userId.trim();
  const workspaceId = input.workspaceId.trim();
  if (!serverId || !userId || !workspaceId) {
    throw new Error("Không thể tạo vùng lưu offline khi thiếu server, user hoặc workspace.");
  }
  return { serverId, userId, workspaceId };
}

export function normalizeServerId(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }
  try {
    const url = new URL(trimmed.includes("://") ? trimmed : `https://${trimmed}`);
    const path = url.pathname.replace(/\/+$/, "");
    return `${url.protocol.toLowerCase()}//${url.host.toLowerCase()}${path}`;
  } catch {
    return trimmed.toLowerCase().replace(/\/+$/, "");
  }
}

export function accountKey(account: OfflineAccount): string {
  return JSON.stringify([normalizeServerId(account.serverId), account.userId.trim()]);
}

export function outboxScopeKey(scope: MessageOutboxScope): string {
  const normalized = messageOutboxScope(scope);
  return JSON.stringify([normalized.serverId, normalized.userId, normalized.workspaceId]);
}

export function createClientMessageId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `client-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

export async function enqueueMessageOutbox(
  scopeInput: MessageOutboxScope,
  input: EnqueueMessageInput,
  initialError?: unknown,
  store: OfflineMessageRepository = offlineRepository()
): Promise<MessageOutboxEntry> {
  const scope = messageOutboxScope(scopeInput);
  const scopeKey = outboxScopeKey(scope);
  const id = messageEntryId(scopeKey, input.clientMessageId);
  const existing = await store.getMessage(id);
  const current = existing ? [] : await store.listMessages(scopeKey);
  if (!existing && current.length >= maxMessagesPerScope) {
    throw new Error("Hàng đợi offline đã đầy. Hãy kết nối mạng và gửi lại trước khi soạn thêm tin.");
  }

  const now = Date.now();
  const timestamp = new Date(now).toISOString();
  const policy = initialError === undefined ? undefined : classifyOutboxError(initialError);
  const attempts = existing?.attempts ?? (policy ? 1 : 0);
  const entry: MessageOutboxEntry = {
    accountKey: accountKey(scope),
    attempts,
    body: input.body,
    channelId: input.channelId,
    clientMessageId: input.clientMessageId,
    createdAt: existing?.createdAt ?? timestamp,
    id,
    ...(policy?.message ? { lastError: policy.message } : {}),
    mentionedUserIds: input.mentionedUserIds,
    nextAttemptAt: policy?.retryable ? now + outboxBackoffMs(Math.max(attempts, 1)) : undefined,
    operation: "message.send",
    parentId: input.parentId,
    replyTo: input.replyTo,
    retryable: policy?.retryable ?? true,
    scopeKey,
    serverId: scope.serverId,
    status: policy && !policy.retryable ? "failed" : "pending",
    updatedAt: timestamp,
    userId: scope.userId,
    workspaceId: scope.workspaceId
  };
  await store.putMessage(entry);
  notifyOutboxChanged(scopeKey);
  return entry;
}

export async function listMessageOutbox(
  scope: MessageOutboxScope,
  store: OfflineMessageRepository = offlineRepository()
): Promise<MessageOutboxEntry[]> {
  return store.listMessages(outboxScopeKey(scope));
}

export async function retryMessageOutbox(
  scope: MessageOutboxScope,
  entryId: string,
  store: OfflineMessageRepository = offlineRepository()
): Promise<MessageOutboxEntry | null> {
  const entry = await store.getMessage(entryId);
  const expectedScopeKey = outboxScopeKey(scope);
  if (!entry || entry.scopeKey !== expectedScopeKey) {
    return null;
  }
  const next: MessageOutboxEntry = {
    ...entry,
    lastError: undefined,
    leaseExpiresAt: undefined,
    leaseOwner: undefined,
    nextAttemptAt: undefined,
    retryable: true,
    status: "pending",
    updatedAt: new Date().toISOString()
  };
  await store.putMessage(next);
  notifyOutboxChanged(expectedScopeKey);
  return next;
}

export async function removeMessageOutbox(
  scope: MessageOutboxScope,
  entryId: string,
  store: OfflineMessageRepository = offlineRepository()
): Promise<boolean> {
  const entry = await store.getMessage(entryId);
  const expectedScopeKey = outboxScopeKey(scope);
  if (!entry || entry.scopeKey !== expectedScopeKey) {
    return false;
  }
  await store.deleteMessage(entry.id);
  notifyOutboxChanged(expectedScopeKey);
  return true;
}

export async function confirmMessageOutbox(
  scope: MessageOutboxScope,
  clientMessageId: string,
  store: OfflineMessageRepository = offlineRepository()
): Promise<boolean> {
  const scopeKey = outboxScopeKey(scope);
  const entry = await store.getMessage(messageEntryId(scopeKey, clientMessageId));
  if (!entry || entry.scopeKey !== scopeKey) {
    return false;
  }
  await store.deleteMessage(entry.id);
  notifyOutboxChanged(scopeKey);
  return true;
}

export async function flushMessageOutbox(
  scope: MessageOutboxScope,
  options: FlushMessageOutboxOptions,
  store: OfflineMessageRepository = offlineRepository()
): Promise<MessageOutboxEntry[]> {
  const scopeKey = outboxScopeKey(scope);
  const now = options.now ?? Date.now;
  const owner = options.leaseOwner ?? createClientMessageId();
  const random = options.random ?? Math.random;

  while (true) {
    const entry = await store.claimNextMessage(scopeKey, owner, now(), defaultLeaseMs, options.force === true);
    if (!entry) {
      break;
    }
    notifyOutboxChanged(scopeKey);
    try {
      const result = await options.send(entry);
      await options.onSent?.(entry, result);
      await store.deleteMessage(entry.id);
      notifyOutboxChanged(scopeKey);
    } catch (error) {
      const policy = classifyOutboxError(error);
      const failedAt = now();
      const failed: MessageOutboxEntry = {
        ...entry,
        lastError: policy.message,
        leaseExpiresAt: undefined,
        leaseOwner: undefined,
        nextAttemptAt: policy.retryable
          ? failedAt + outboxBackoffMs(entry.attempts, random)
          : undefined,
        retryable: policy.retryable,
        status: "failed",
        updatedAt: new Date(failedAt).toISOString()
      };
      await store.putMessage(failed);
      notifyOutboxChanged(scopeKey);
      if (policy.retryable) {
        break;
      }
    }
  }

  const entries = await store.listMessages(scopeKey);
  await options.onUpdated?.(entries);
  return entries;
}

export function classifyOutboxError(error: unknown): OutboxErrorPolicy {
  const message = error instanceof Error && error.message.trim() ? error.message : "Không gửi được tin nhắn.";
  if (typeof navigator !== "undefined" && navigator.onLine === false) {
    return { message, retryable: true };
  }
  if (error instanceof ApiClientError) {
    return {
      message,
      retryable: error.status === 408 || error.status === 425 || error.status === 429 || error.status >= 500,
      status: error.status
    };
  }
  const status = statusFromUnknownError(error);
  if (status !== undefined) {
    return {
      message,
      retryable: status === 408 || status === 425 || status === 429 || status >= 500,
      status
    };
  }
  return {
    message,
    retryable:
      error instanceof TypeError ||
      (error instanceof Error && /fetch|network|offline|failed to fetch|connection/i.test(error.message))
  };
}

export function isLikelyOfflineError(error: unknown): boolean {
  return classifyOutboxError(error).retryable;
}

export function outboxBackoffMs(attempt: number, random: () => number = Math.random): number {
  const exponential = Math.min(maxBackoffMs, 1_000 * 2 ** Math.max(0, Math.min(attempt - 1, 6)));
  const jitter = 0.75 + Math.max(0, Math.min(1, random())) * 0.5;
  return Math.min(maxBackoffMs, Math.round(exponential * jitter));
}

export async function readSyncCheckpoint(
  scope: MessageOutboxScope,
  store: OfflineMessageRepository = offlineRepository()
): Promise<SyncCheckpoint | null> {
  return store.getSyncCheckpoint(outboxScopeKey(scope));
}

export async function writeSyncCheckpoint(
  scopeInput: MessageOutboxScope,
  cursor: string | undefined,
  eventIds: string[],
  store: OfflineMessageRepository = offlineRepository()
): Promise<SyncCheckpoint> {
  const scope = messageOutboxScope(scopeInput);
  const scopeKey = outboxScopeKey(scope);
  const current = await store.getSyncCheckpoint(scopeKey);
  const recentEventIds = [...new Set([...(current?.recentEventIds ?? []), ...eventIds.filter(Boolean)])].slice(-500);
  const checkpoint: SyncCheckpoint = {
    accountKey: accountKey(scope),
    cursor: cursor || current?.cursor,
    id: scopeKey,
    recentEventIds,
    scopeKey,
    serverId: scope.serverId,
    updatedAt: new Date().toISOString(),
    userId: scope.userId,
    workspaceId: scope.workspaceId
  };
  await store.putSyncCheckpoint(checkpoint);
  return checkpoint;
}

export async function clearOfflineAccount(
  input: OfflineAccount,
  store: OfflineMessageRepository = offlineRepository()
): Promise<void> {
  const key = accountKey(input);
  if (!normalizeServerId(input.serverId) || !input.userId.trim()) {
    return;
  }
  await store.deleteAccount(key);
  notifyOutboxChanged();
}

export function subscribeOutboxChanges(listener: (scopeKey?: string) => void): () => void {
  if (typeof window === "undefined") {
    return () => undefined;
  }
  const handleLocal = (event: Event) => {
    const custom = event as CustomEvent<{ scopeKey?: string }>;
    listener(custom.detail?.scopeKey);
  };
  const channel = offlineBroadcastChannel();
  const handleBroadcast = (event: MessageEvent<{ scopeKey?: string }>) => listener(event.data?.scopeKey);
  window.addEventListener(changeEventName, handleLocal);
  channel?.addEventListener("message", handleBroadcast);
  return () => {
    window.removeEventListener(changeEventName, handleLocal);
    channel?.removeEventListener("message", handleBroadcast);
  };
}

export function offlineRepository(): OfflineMessageRepository {
  repository ??= new IndexedDbOfflineRepository();
  return repository;
}

export class MemoryOfflineRepository implements OfflineMessageRepository {
  private readonly messages = new Map<string, MessageOutboxEntry>();
  private readonly checkpoints = new Map<string, SyncCheckpoint>();

  async claimNextMessage(scopeKey: string, owner: string, now: number, leaseMs: number, force: boolean) {
    const candidates = await this.listMessages(scopeKey);
    const entry = candidates.find((item) => isEntryDue(item, now, force));
    if (!entry) {
      return null;
    }
    const claimed: MessageOutboxEntry = {
      ...entry,
      attempts: entry.attempts + 1,
      leaseExpiresAt: now + leaseMs,
      leaseOwner: owner,
      status: "sending",
      updatedAt: new Date(now).toISOString()
    };
    this.messages.set(claimed.id, clone(claimed));
    return clone(claimed);
  }

  async deleteAccount(key: string) {
    for (const [id, entry] of this.messages) {
      if (entry.accountKey === key) {
        this.messages.delete(id);
      }
    }
    for (const [id, checkpoint] of this.checkpoints) {
      if (checkpoint.accountKey === key) {
        this.checkpoints.delete(id);
      }
    }
  }

  async deleteMessage(id: string) {
    this.messages.delete(id);
  }

  async getMessage(id: string) {
    const entry = this.messages.get(id);
    return entry ? clone(entry) : null;
  }

  async getSyncCheckpoint(id: string) {
    const checkpoint = this.checkpoints.get(id);
    return checkpoint ? clone(checkpoint) : null;
  }

  async listMessages(scopeKey: string) {
    return [...this.messages.values()]
      .filter((entry) => entry.scopeKey === scopeKey)
      .sort(compareOutboxEntries)
      .map(clone);
  }

  async putMessage(entry: MessageOutboxEntry) {
    this.messages.set(entry.id, clone(entry));
  }

  async putSyncCheckpoint(checkpoint: SyncCheckpoint) {
    this.checkpoints.set(checkpoint.id, clone(checkpoint));
  }
}

class IndexedDbOfflineRepository implements OfflineMessageRepository {
  private databasePromise?: Promise<IDBDatabase>;

  async claimNextMessage(scopeKey: string, owner: string, now: number, leaseMs: number, force: boolean) {
    const database = await this.database();
    return new Promise<MessageOutboxEntry | null>((resolve, reject) => {
      const transaction = database.transaction(outboxStoreName, "readwrite");
      const store = transaction.objectStore(outboxStoreName);
      const request = store.index(scopeIndexName).getAll(scopeKey);
      let claimed: MessageOutboxEntry | null = null;
      request.onerror = () => reject(request.error ?? new Error("Không đọc được hàng đợi offline."));
      request.onsuccess = () => {
        const entry = (request.result as MessageOutboxEntry[]).sort(compareOutboxEntries).find((item) =>
          isEntryDue(item, now, force)
        );
        if (!entry) {
          return;
        }
        claimed = {
          ...entry,
          attempts: entry.attempts + 1,
          leaseExpiresAt: now + leaseMs,
          leaseOwner: owner,
          status: "sending",
          updatedAt: new Date(now).toISOString()
        };
        store.put(claimed);
      };
      transaction.onabort = () => reject(transaction.error ?? new Error("Không thể khóa tin nhắn offline."));
      transaction.onerror = () => reject(transaction.error ?? new Error("Không thể khóa tin nhắn offline."));
      transaction.oncomplete = () => resolve(claimed);
    });
  }

  async deleteAccount(key: string) {
    const database = await this.database();
    await Promise.all([
      deleteByIndex(database, outboxStoreName, accountIndexName, key),
      deleteByIndex(database, syncStoreName, accountIndexName, key)
    ]);
  }

  async deleteMessage(id: string) {
    const database = await this.database();
    await requestInTransaction(database, outboxStoreName, "readwrite", (store) => store.delete(id));
  }

  async getMessage(id: string) {
    const database = await this.database();
    return requestInTransaction<MessageOutboxEntry | undefined>(database, outboxStoreName, "readonly", (store) =>
      store.get(id)
    ).then((entry) => entry ?? null);
  }

  async getSyncCheckpoint(id: string) {
    const database = await this.database();
    return requestInTransaction<SyncCheckpoint | undefined>(database, syncStoreName, "readonly", (store) =>
      store.get(id)
    ).then((entry) => entry ?? null);
  }

  async listMessages(scopeKey: string) {
    const database = await this.database();
    return requestInTransaction<MessageOutboxEntry[]>(database, outboxStoreName, "readonly", (store) =>
      store.index(scopeIndexName).getAll(scopeKey)
    ).then((entries) => entries.sort(compareOutboxEntries));
  }

  async putMessage(entry: MessageOutboxEntry) {
    const database = await this.database();
    await requestInTransaction(database, outboxStoreName, "readwrite", (store) => store.put(entry));
  }

  async putSyncCheckpoint(checkpoint: SyncCheckpoint) {
    const database = await this.database();
    await requestInTransaction(database, syncStoreName, "readwrite", (store) => store.put(checkpoint));
  }

  private database(): Promise<IDBDatabase> {
    if (typeof indexedDB === "undefined") {
      return Promise.reject(new Error("Trình duyệt này không hỗ trợ IndexedDB để lưu hàng đợi offline."));
    }
    this.databasePromise ??= new Promise<IDBDatabase>((resolve, reject) => {
      const request = indexedDB.open(databaseName, databaseVersion);
      request.onerror = () => reject(request.error ?? new Error("Không mở được kho dữ liệu offline."));
      request.onblocked = () => reject(new Error("Kho dữ liệu offline đang bị khóa bởi một phiên bản ứng dụng khác."));
      request.onupgradeneeded = () => {
        const database = request.result;
        if (!database.objectStoreNames.contains(outboxStoreName)) {
          const store = database.createObjectStore(outboxStoreName, { keyPath: "id" });
          store.createIndex(scopeIndexName, "scopeKey", { unique: false });
          store.createIndex(accountIndexName, "accountKey", { unique: false });
        }
        if (!database.objectStoreNames.contains(syncStoreName)) {
          const store = database.createObjectStore(syncStoreName, { keyPath: "id" });
          store.createIndex(accountIndexName, "accountKey", { unique: false });
        }
      };
      request.onsuccess = () => {
        request.result.onversionchange = () => request.result.close();
        resolve(request.result);
      };
    });
    return this.databasePromise;
  }
}

function messageEntryId(scopeKey: string, clientMessageId: string): string {
  return JSON.stringify([scopeKey, clientMessageId.trim()]);
}

function compareOutboxEntries(left: MessageOutboxEntry, right: MessageOutboxEntry): number {
  return left.createdAt.localeCompare(right.createdAt) || left.id.localeCompare(right.id);
}

function isEntryDue(entry: MessageOutboxEntry, now: number, force: boolean): boolean {
  if (!entry.retryable) {
    return false;
  }
  if (entry.status === "sending" && (entry.leaseExpiresAt ?? Number.POSITIVE_INFINITY) > now) {
    return false;
  }
  return force || !entry.nextAttemptAt || entry.nextAttemptAt <= now;
}

function statusFromUnknownError(error: unknown): number | undefined {
  if (!error || typeof error !== "object" || !("status" in error)) {
    return undefined;
  }
  const status = Number((error as { status?: unknown }).status);
  return Number.isFinite(status) ? status : undefined;
}

function notifyOutboxChanged(scopeKey?: string) {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent(changeEventName, { detail: { scopeKey } }));
    offlineBroadcastChannel()?.postMessage({ scopeKey });
  }
}

function offlineBroadcastChannel(): BroadcastChannel | undefined {
  if (typeof BroadcastChannel === "undefined") {
    return undefined;
  }
  broadcastChannel ??= new BroadcastChannel(broadcastChannelName);
  return broadcastChannel;
}

function requestInTransaction<T = undefined>(
  database: IDBDatabase,
  storeName: string,
  mode: IDBTransactionMode,
  createRequest: (store: IDBObjectStore) => IDBRequest<T>
): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const transaction = database.transaction(storeName, mode);
    const request = createRequest(transaction.objectStore(storeName));
    let result: T;
    request.onsuccess = () => {
      result = request.result;
    };
    request.onerror = () => reject(request.error ?? new Error("Không thể truy cập kho dữ liệu offline."));
    transaction.onabort = () => reject(transaction.error ?? new Error("Giao dịch offline bị hủy."));
    transaction.onerror = () => reject(transaction.error ?? new Error("Giao dịch offline thất bại."));
    transaction.oncomplete = () => resolve(result);
  });
}

function deleteByIndex(
  database: IDBDatabase,
  storeName: string,
  indexName: string,
  key: string
): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    const transaction = database.transaction(storeName, "readwrite");
    const store = transaction.objectStore(storeName);
    const request = store.index(indexName).openKeyCursor(IDBKeyRange.only(key));
    request.onsuccess = () => {
      const cursor = request.result;
      if (!cursor) {
        return;
      }
      store.delete(cursor.primaryKey);
      cursor.continue();
    };
    request.onerror = () => reject(request.error ?? new Error("Không thể xóa dữ liệu offline của tài khoản."));
    transaction.onabort = () => reject(transaction.error ?? new Error("Không thể xóa dữ liệu offline của tài khoản."));
    transaction.onerror = () => reject(transaction.error ?? new Error("Không thể xóa dữ liệu offline của tài khoản."));
    transaction.oncomplete = () => resolve();
  });
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}
