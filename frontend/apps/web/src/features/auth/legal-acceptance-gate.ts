import {
  ApiClientError,
  isUGCMutationRequest,
  type HttpRequestContext
} from "@webtui/api-client";

export type LegalGateKind = "checking" | "complete" | "required" | "unavailable" | "mismatch";

export type LegalGateSnapshot = {
  detail?: string;
  kind: LegalGateKind;
  reason?: "server" | "status";
  revision: number;
  scope: string | null;
};

const listeners = new Set<() => void>();
let snapshot: LegalGateSnapshot = { kind: "checking", revision: 0, scope: null };

export const legalAcceptanceGate = {
  assertRequest(request: HttpRequestContext, scope: string | null) {
    if (!request.auth || !isUGCMutationRequest(request.method, request.path, request.body)) {
      return;
    }
    if (scope && snapshot.scope === scope && snapshot.kind === "complete") {
      return;
    }
    if (snapshot.scope !== scope) {
      update({ kind: "checking", scope });
    }
    throw new ApiClientError({
      code: "LEGAL_ACCEPTANCE_REQUIRED",
      message: "Vui lòng chấp nhận Điều khoản và Chính sách quyền riêng tư hiện hành trước khi tạo nội dung.",
      status: 409
    });
  },
  getSnapshot: () => snapshot,
  handleApiError(error: ApiClientError, scope: string | null) {
    if (!scope) return;
    if (error.code === "LEGAL_ACCEPTANCE_REQUIRED") {
      update({ kind: "required", reason: "server", scope });
    } else if (error.code === "LEGAL_ACCEPTANCE_UNAVAILABLE") {
      update({ detail: error.message, kind: "unavailable", scope });
    }
  },
  markChecking(scope: string) {
    update({ kind: "checking", scope });
  },
  markComplete(scope: string) {
    update({ kind: "complete", scope });
  },
  markMismatch(scope: string, detail: string) {
    update({ detail, kind: "mismatch", scope });
  },
  markRequired(scope: string) {
    update({ kind: "required", reason: "status", scope });
  },
  markUnavailable(scope: string, detail: string) {
    update({ detail, kind: "unavailable", scope });
  },
  reset() {
    update({ kind: "checking", scope: null });
  },
  subscribe(listener: () => void) {
    listeners.add(listener);
    return () => listeners.delete(listener);
  }
};

export function createLegalAcceptanceScope(
  server: string | null | undefined,
  userId: string | null | undefined,
  workspaceId: string | null | undefined
): string | null {
  const normalizedServer = server?.trim().replace(/\/$/, "");
  const normalizedUserId = userId?.trim();
  const normalizedWorkspaceId = workspaceId?.trim();
  return normalizedServer && normalizedUserId && normalizedWorkspaceId
    ? `${normalizedServer}::${normalizedUserId}::${normalizedWorkspaceId}`
    : null;
}

export function workspaceIdFromApiPath(rawPath: string | null | undefined): string | null {
  const path = rawPath?.split(/[?#]/, 1)[0] ?? "";
  const match = path.match(/^\/api\/v1\/workspaces\/([^/]+)/);
  if (!match?.[1]) return null;
  try {
    return decodeURIComponent(match[1]);
  } catch {
    return null;
  }
}

function update(next: Omit<LegalGateSnapshot, "revision">) {
  if (
    snapshot.scope === next.scope
    && snapshot.kind === next.kind
    && snapshot.detail === next.detail
    && snapshot.reason === next.reason
  ) {
    return;
  }
  snapshot = { ...next, revision: snapshot.revision + 1 };
  for (const listener of listeners) listener();
}
