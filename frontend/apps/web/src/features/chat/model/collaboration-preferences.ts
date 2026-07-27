export type ConversationPreference = {
  important: boolean;
  sensitive: boolean;
  tags: string[];
};

export type ConversationPreferenceMap = Record<string, ConversationPreference>;

export type CallJoinPreferences = {
  cameraEnabled: boolean;
  microphoneEnabled: boolean;
};

export const defaultConversationPreference: ConversationPreference = {
  important: false,
  sensitive: false,
  tags: []
};

export const defaultCallJoinPreferences: CallJoinPreferences = {
  cameraEnabled: true,
  microphoneEnabled: true
};

export function collaborationPreferencesStorageKey(workspaceId: string): string {
  return `vpsttt:collaboration-preferences:${workspaceId}`;
}

export function collaborationDisplayStorageKey(workspaceId: string): string {
  return `vpsttt:collaboration-display:${workspaceId}`;
}

export function parseConversationPreferences(value: string | null): ConversationPreferenceMap {
  if (!value) {
    return {};
  }
  try {
    const decoded = JSON.parse(value) as unknown;
    if (!decoded || typeof decoded !== "object" || Array.isArray(decoded)) {
      return {};
    }
    const result: ConversationPreferenceMap = {};
    for (const [conversationId, rawPreference] of Object.entries(decoded)) {
      if (!rawPreference || typeof rawPreference !== "object" || Array.isArray(rawPreference)) {
        continue;
      }
      const preference = rawPreference as Record<string, unknown>;
      result[conversationId] = {
        important: preference.important === true,
        sensitive: preference.sensitive === true,
        tags: normalizeConversationTags(preference.tags)
      };
    }
    return result;
  } catch {
    return {};
  }
}

export function normalizeConversationPreference(
  value?: Partial<ConversationPreference> | null
): ConversationPreference {
  return {
    important: value?.important === true,
    sensitive: value?.sensitive === true,
    tags: normalizeConversationTags(value?.tags)
  };
}

export function normalizeConversationTags(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  const seen = new Set<string>();
  const result: string[] = [];
  for (const item of value) {
    const tag = typeof item === "string"
      ? item.trim().replace(/^#+/, "").replace(/\s+/g, " ")
      : "";
    const key = tag.toLocaleLowerCase("vi");
    if (!tag || tag.length > 32 || seen.has(key)) {
      continue;
    }
    seen.add(key);
    result.push(tag);
    if (result.length === 6) {
      break;
    }
  }
  return result;
}

export function parseCollaborationDisplay(value: string | null): {
  callJoin: CallJoinPreferences;
  compactMode: boolean;
} {
  if (!value) {
    return {
      callJoin: defaultCallJoinPreferences,
      compactMode: false
    };
  }
  try {
    const decoded = JSON.parse(value) as Record<string, unknown>;
    const callJoin =
      decoded.callJoin && typeof decoded.callJoin === "object" && !Array.isArray(decoded.callJoin)
        ? decoded.callJoin as Record<string, unknown>
        : {};
    return {
      callJoin: {
        cameraEnabled: callJoin.cameraEnabled !== false,
        microphoneEnabled: callJoin.microphoneEnabled !== false
      },
      compactMode: decoded.compactMode === true
    };
  } catch {
    return {
      callJoin: defaultCallJoinPreferences,
      compactMode: false
    };
  }
}
