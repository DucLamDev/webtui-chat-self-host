const workspacePrefix = String.raw`/api/v1/workspaces/[^/]+`;
const channelPrefix = `${workspacePrefix}/channels/[^/]+`;
const messagePrefix = `${channelPrefix}/messages/[^/]+`;

const ugcMutationRoutes: ReadonlyArray<readonly [string, RegExp]> = [
  ["PATCH", /^\/api\/v1\/users\/me$/],
  ["POST", /^\/api\/v1\/users\/me\/avatar$/],
  ["POST", new RegExp(`^${workspacePrefix}/channels$`)],
  ["PATCH", new RegExp(`^${workspacePrefix}/channels/[^/]+$`)],
  ["POST", new RegExp(`^${workspacePrefix}/direct-conversations$`)],
  ["POST", new RegExp(`^${channelPrefix}/private-session$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/promote$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/public-link$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/guests/[^/]+/approve$`)],
  ["PUT", new RegExp(`^${channelPrefix}/collaboration/documents/[^/]+$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/tasks$`)],
  ["PATCH", new RegExp(`^${channelPrefix}/collaboration/tasks/[^/]+$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/breakouts$`)],
  ["PUT", new RegExp(`^${channelPrefix}/collaboration/breakouts/setup$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/breakouts/start$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/breakouts/[^/]+/join$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/breakouts/broadcast$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/meetings$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/voice-room/start$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/ai/summary$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/recordings$`)],
  ["POST", new RegExp(`^${channelPrefix}/collaboration/federation-invites$`)],
  ["POST", new RegExp(`^${workspacePrefix}/bots/[^/]+/messages$`)],
  ["POST", new RegExp(`^${workspacePrefix}/messages/scheduled$`)],
  ["POST", new RegExp(`^${channelPrefix}/messages$`)],
  ["PATCH", new RegExp(`^${messagePrefix}$`)],
  ["POST", new RegExp(`^${messagePrefix}/forward$`)],
  ["POST", new RegExp(`^${messagePrefix}/pin$`)],
  ["PUT", new RegExp(`^${messagePrefix}/thread/details$`)],
  ["POST", new RegExp(`^${messagePrefix}/reactions$`)],
  ["POST", new RegExp(`^${workspacePrefix}/files$`)],
  ["POST", new RegExp(`^${workspacePrefix}/files/uploads$`)],
  ["PUT", new RegExp(`^${workspacePrefix}/files/uploads/[^/]+/parts/[^/]+$`)],
  ["POST", new RegExp(`^${workspacePrefix}/files/uploads/[^/]+/complete$`)],
  ["POST", new RegExp(`^${workspacePrefix}/files/[^/]+/versions$`)],
  ["POST", new RegExp(`^${workspacePrefix}/files/[^/]+/office/session$`)],
  ["POST", new RegExp(`^${messagePrefix}/attachments$`)],
  ["POST", new RegExp(`^${workspacePrefix}/calls$`)],
  ["POST", new RegExp(`^${workspacePrefix}/calls/[^/]+/accept$`)],
  ["POST", new RegExp(`^${workspacePrefix}/calls/[^/]+/signals$`)],
];

export function isUGCMutationRequest(
  method: string,
  rawPath: string,
  body?:
    | BodyInit
    | Record<string, unknown>
    | Array<unknown>
    | string
    | number
    | boolean
    | null,
): boolean {
  const normalizedMethod = method.trim().toUpperCase();
  const path = rawPath.split(/[?#]/, 1)[0]?.replace(/\/$/, "") || "/";
  if (
    ugcMutationRoutes.some(
      ([candidateMethod, pattern]) =>
        candidateMethod === normalizedMethod && pattern.test(path),
    )
  ) {
    return true;
  }
  if (
    normalizedMethod === "POST" &&
    new RegExp(`^${channelPrefix}/collaboration/meetings/[^/]+/start$`).test(
      path,
    )
  ) {
    return true;
  }
  if (
    normalizedMethod === "POST" &&
    new RegExp(
      `^${channelPrefix}/collaboration/federation-invites/[^/]+/accepted$`,
    ).test(path)
  ) {
    return true;
  }

  const payload = jsonRecord(body);
  if (
    normalizedMethod === "PUT" &&
    new RegExp(`^${channelPrefix}/collaboration$`).test(path)
  ) {
    return !isFullCollaborationSafetyLockdown(payload);
  }
  if (
    normalizedMethod === "PATCH" &&
    new RegExp(`^${channelPrefix}/collaboration/roles/[^/]+$`).test(path)
  ) {
    return normalizedString(payload?.role) !== "listener";
  }
  if (
    normalizedMethod === "PUT" &&
    new RegExp(
      `^${channelPrefix}/collaboration/breakouts/[^/]+/assignments$`,
    ).test(path)
  ) {
    return (
      !Array.isArray(payload?.assigned_user_ids) ||
      payload.assigned_user_ids.length > 0
    );
  }
  if (
    normalizedMethod === "PUT" &&
    new RegExp(`^${channelPrefix}/collaboration/recording-policy$`).test(path)
  ) {
    return payload?.enabled !== false;
  }
  if (
    normalizedMethod === "PUT" &&
    new RegExp(
      `^${channelPrefix}/collaboration/recordings/[^/]+/consent$`,
    ).test(path)
  ) {
    return payload?.consented !== false;
  }
  return false;
}

function jsonRecord(body: unknown): Record<string, unknown> | null {
  if (!body || typeof body !== "object" || Array.isArray(body)) {
    return null;
  }
  if (typeof FormData !== "undefined" && body instanceof FormData) return null;
  if (typeof Blob !== "undefined" && body instanceof Blob) return null;
  if (typeof URLSearchParams !== "undefined" && body instanceof URLSearchParams)
    return null;
  return body as Record<string, unknown>;
}

function isFullCollaborationSafetyLockdown(
  payload: Record<string, unknown> | null,
): boolean {
  return (
    normalizedString(payload?.room_mode) === "internal" &&
    payload?.lobby_enabled === true &&
    payload?.chat_locked === true &&
    payload?.guest_microphone_enabled === false &&
    payload?.guest_camera_enabled === false &&
    normalizedString(payload?.default_participant_role) === "listener"
  );
}

function normalizedString(value: unknown): string {
  return typeof value === "string" ? value.trim().toLowerCase() : "";
}
