"use client";

import { getPlatformServices, type PlatformStorage } from "@webtui/chat-core";
import type {
  Channel as ApiChannel,
  DirectConversation as ApiDirectConversation,
  Message as ApiMessage,
  Permission,
  Workspace,
  WorkspaceMember,
  WorkspaceSetting
} from "@webtui/types";

const schemaVersion = 1;
const maxCachedTimelines = 24;
const maxMessagesPerTimeline = 200;

type CacheEnvelope<T> = {
  savedAt: string;
  schemaVersion: number;
  value: T;
};

export type WorkspaceShellCache = {
  members: WorkspaceMember[];
  permissions: Permission[];
  selectedWorkspace?: Workspace | null;
  settings: WorkspaceSetting[];
  workspaces: Workspace[];
};

export type WorkspaceChatCache = {
  channels: ApiChannel[];
  directConversations: ApiDirectConversation[];
};

type TimelineIndexEntry = {
  channelId: string;
  savedAt: string;
  workspaceId: string;
};

function storage() {
  return getPlatformServices().storage;
}

export async function readWorkspaceShellCache(workspaceRef = "", store: PlatformStorage = storage()) {
  return readEnvelope<WorkspaceShellCache>(workspaceShellKey(workspaceRef), store);
}

export async function writeWorkspaceShellCache(
  workspaceRef: string,
  value: WorkspaceShellCache,
  store: PlatformStorage = storage()
) {
  await writeEnvelope(workspaceShellKey(workspaceRef), value, store);
}

export async function readWorkspaceChatCache(workspaceId: string, store: PlatformStorage = storage()) {
  return readEnvelope<WorkspaceChatCache>(workspaceChatKey(workspaceId), store);
}

export async function writeWorkspaceChatCache(
  workspaceId: string,
  value: WorkspaceChatCache,
  store: PlatformStorage = storage()
) {
  await writeEnvelope(workspaceChatKey(workspaceId), value, store);
}

export async function readTimelineCache(workspaceId: string, channelId: string, store: PlatformStorage = storage()) {
  return (await readEnvelope<ApiMessage[]>(timelineKey(workspaceId, channelId), store)) ?? [];
}

export async function writeTimelineCache(
  workspaceId: string,
  channelId: string,
  messages: ApiMessage[],
  store: PlatformStorage = storage()
) {
  const compactMessages = compactTimeline(messages);
  await writeEnvelope(timelineKey(workspaceId, channelId), compactMessages, store);
  await updateTimelineIndex(workspaceId, channelId, store);
}

export async function readDraft(workspaceId: string, channelId: string, store: PlatformStorage = storage()) {
  return (await readEnvelope<string>(draftKey(workspaceId, channelId), store)) ?? "";
}

export async function writeDraft(workspaceId: string, channelId: string, value: string, store: PlatformStorage = storage()) {
  const key = draftKey(workspaceId, channelId);
  if (!value.trim()) {
    await store.removeItem(key);
    return;
  }
  await writeEnvelope(key, value, store);
}

export function isLikelyOfflineError(error: unknown): boolean {
  if (typeof navigator !== "undefined" && navigator.onLine === false) {
    return true;
  }
  return error instanceof TypeError || (error instanceof Error && /fetch|network|offline|failed to fetch/i.test(error.message));
}

/**
 * Discard the pre-IndexedDB outbox. It had no server/user ownership metadata,
 * so migrating it could expose one account's drafts after an account switch.
 */
export async function discardLegacyOutbox(store: PlatformStorage = storage()) {
  await store.removeItem(legacyOutboxKey);
}

function compactTimeline(messages: ApiMessage[]): ApiMessage[] {
  const seen = new Set<string>();
  return [...messages]
    .sort((left, right) => new Date(left.created_at ?? left.sent_at ?? 0).getTime() - new Date(right.created_at ?? right.sent_at ?? 0).getTime())
    .filter((message) => {
      if (seen.has(message.id)) {
        return false;
      }
      seen.add(message.id);
      return !message.id.startsWith("local-");
    })
    .slice(-maxMessagesPerTimeline);
}

async function updateTimelineIndex(workspaceId: string, channelId: string, store: PlatformStorage) {
  const now = new Date().toISOString();
  const current = (await readEnvelope<TimelineIndexEntry[]>(timelineIndexKey, store)) ?? [];
  const next = [
    ...current.filter((item) => !(item.workspaceId === workspaceId && item.channelId === channelId)),
    { channelId, savedAt: now, workspaceId }
  ].sort((left, right) => new Date(right.savedAt).getTime() - new Date(left.savedAt).getTime());

  const retained = next.slice(0, maxCachedTimelines);
  const evicted = next.slice(maxCachedTimelines);
  await writeEnvelope(timelineIndexKey, retained, store);
  await Promise.all(evicted.map((item) => store.removeItem(timelineKey(item.workspaceId, item.channelId))));
}

async function readEnvelope<T>(key: string, store: PlatformStorage): Promise<T | null> {
  return parseEnvelope<T>(await store.getItem(key));
}

async function writeEnvelope<T>(key: string, value: T, store: PlatformStorage) {
  await store.setItem(key, JSON.stringify({ savedAt: new Date().toISOString(), schemaVersion, value }), "persistent");
}

function parseEnvelope<T>(raw: string | null | undefined): T | null {
  if (!raw) {
    return null;
  }
  try {
    const parsed = JSON.parse(raw) as Partial<CacheEnvelope<T>>;
    return parsed.schemaVersion === schemaVersion && parsed.value !== undefined ? parsed.value : null;
  } catch {
    return null;
  }
}

function workspaceShellKey(workspaceRef: string) {
  return `webtui:offline:v${schemaVersion}:workspace-shell:${workspaceRef || "last"}`;
}

function workspaceChatKey(workspaceId: string) {
  return `webtui:offline:v${schemaVersion}:workspace-chat:${workspaceId}`;
}

function timelineKey(workspaceId: string, channelId: string) {
  return `webtui:offline:v${schemaVersion}:timeline:${workspaceId}:${channelId}`;
}

function draftKey(workspaceId: string, channelId: string) {
  return `webtui:offline:v${schemaVersion}:draft:${workspaceId}:${channelId}`;
}

const legacyOutboxKey = `webtui:offline:v${schemaVersion}:outbox`;
const timelineIndexKey = `webtui:offline:v${schemaVersion}:timeline-index`;
