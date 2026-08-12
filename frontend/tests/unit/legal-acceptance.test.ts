import { describe, expect, it, vi } from "vitest";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import {
  ApiClientError,
  createAuthClient,
  createLegalPolicyConfig,
  HttpClient,
  isCompleteLegalAcceptance,
  isUGCMutationRequest,
  legalAcceptanceCompatibilityError,
  legalDocumentsCompatibilityError,
  resolveCurrentLegalDocuments,
} from "@webtui/api-client";
import type {
  CurrentLegalAcceptance,
  LegalDocumentVersion,
} from "@webtui/types";
import {
  createLegalAcceptanceScope,
  legalAcceptanceGate,
  workspaceIdFromApiPath,
} from "../../apps/web/src/features/auth/legal-acceptance-gate";

const policyConfig = createLegalPolicyConfig({
  NEXT_PUBLIC_PRIVACY_URL: "https://download.webtui.vn/privacy",
  NEXT_PUBLIC_PRIVACY_VERSION: "2026-08-07",
  NEXT_PUBLIC_TERMS_URL: "https://download.webtui.vn/terms",
  NEXT_PUBLIC_TERMS_VERSION: "2026-08-07",
});
const documentList: LegalDocumentVersion[] = [
  {
    document_type: "terms",
    includes: ["terms_of_use", "acceptable_use_policy"],
    version: "2026-08-07",
  },
  {
    document_type: "privacy",
    includes: ["privacy_policy"],
    version: "2026-08-07",
  },
];
const completeAcceptance: CurrentLegalAcceptance = {
  complete: true,
  privacy: {
    accepted: true,
    accepted_at: "2026-08-07T08:00:00Z",
    version: "2026-08-07",
  },
  terms: {
    accepted: true,
    accepted_at: "2026-08-07T08:00:00Z",
    version: "2026-08-07",
  },
  workspace_id: "workspace-1",
};

describe("legal acceptance policy", () => {
  it("keeps public guest UGC fail-closed and bound to the serving origin", () => {
    const joinPage = readFileSync(
      resolve(process.cwd(), "apps/web/src/app/join/page.tsx"),
      "utf8",
    );
    const apiSource = readFileSync(
      resolve(process.cwd(), "apps/web/src/lib/api.ts"),
      "utf8",
    );

    expect(joinPage).toContain("publicApi.auth.legalDocuments()");
    expect(joinPage).toContain("legalDocumentsCompatibilityError");
    expect(joinPage).toContain("terms_accepted: true");
    expect(joinPage).toContain("privacy_accepted: true");
    expect(joinPage).toContain("!termsAccepted");
    expect(joinPage).toContain("!privacyAccepted");
    expect(apiSource).toContain("export const publicApi");
    expect(apiSource).toContain("baseUrl: () => runtimeEnvironment.apiBaseUrl");
  });

  it("stays aligned with every backend route mounted on a UGC middleware group", () => {
    const handlers = [
      ["users", "/api/v1/users"],
      ["channels", "/api/v1/workspaces/workspace-1"],
      ["files", "/api/v1/workspaces/workspace-1"],
      ["messages", "/api/v1/workspaces/workspace-1"],
      ["calls", "/api/v1/workspaces/workspace-1/calls"],
      ["bots", "/api/v1/workspaces/workspace-1"],
    ] as const;

    for (const [moduleName, prefix] of handlers) {
      const source = readFileSync(
        resolve(
          process.cwd(),
          `../backend/internal/modules/${moduleName}/delivery/http/handler.go`,
        ),
        "utf8",
      );
      const routes = [
        ...source.matchAll(/ugc\.(POST|PUT|PATCH|DELETE)\("([^"]*)"/g),
      ];
      expect(
        routes.length,
        `${moduleName} should expose UGC routes`,
      ).toBeGreaterThan(0);
      for (const route of routes) {
        expect(
          isUGCMutationRequest(route[1], `${prefix}${route[2]}`),
          `${moduleName}: ${route[1]} ${prefix}${route[2]}`,
        ).toBe(true);
      }
    }
  });

  it.each([
    ["PATCH", "/api/v1/users/me"],
    ["POST", "/api/v1/users/me/avatar"],
    ["POST", "/api/v1/workspaces/w1/channels"],
    ["PATCH", "/api/v1/workspaces/w1/channels/c1"],
    ["POST", "/api/v1/workspaces/w1/direct-conversations"],
    ["POST", "/api/v1/workspaces/w1/channels/c1/messages"],
    ["PATCH", "/api/v1/workspaces/w1/channels/c1/messages/m1"],
    ["POST", "/api/v1/workspaces/w1/channels/c1/messages/m1/reactions"],
    ["PUT", "/api/v1/workspaces/w1/channels/c1/messages/m1/thread/details"],
    ["POST", "/api/v1/workspaces/w1/files/uploads"],
    ["PUT", "/api/v1/workspaces/w1/files/uploads/u1/parts/1"],
    ["POST", "/api/v1/workspaces/w1/calls"],
    ["POST", "/api/v1/workspaces/w1/calls/call1/accept"],
    ["POST", "/api/v1/workspaces/w1/calls/call1/signals?transport=ws"],
  ])("classifies protected UGC mutation %s %s", (method, path) => {
    expect(isUGCMutationRequest(method, path)).toBe(true);
  });

  it.each([
    [
      "POST",
      "/api/v1/workspaces/w1/channels/c1/collaboration/meetings/m1/start",
      undefined,
    ],
    [
      "POST",
      "/api/v1/workspaces/w1/channels/c1/collaboration/federation-invites/i1/accepted",
      undefined,
    ],
    [
      "PUT",
      "/api/v1/workspaces/w1/channels/c1/collaboration/recording-policy",
      { enabled: true },
    ],
    [
      "PUT",
      "/api/v1/workspaces/w1/channels/c1/collaboration/recordings/r1/consent",
      { consented: true },
    ],
    [
      "PATCH",
      "/api/v1/workspaces/w1/channels/c1/collaboration/roles/u1",
      { role: "moderator" },
    ],
    [
      "PUT",
      "/api/v1/workspaces/w1/channels/c1/collaboration/breakouts/b1/assignments",
      { assigned_user_ids: ["u1"] },
    ],
    [
      "PUT",
      "/api/v1/workspaces/w1/channels/c1/collaboration/breakouts/b1/assignments",
      {},
    ],
    [
      "PATCH",
      "/api/v1/workspaces/w1/channels/c1/collaboration/roles/u1",
      { role: false },
    ],
    [
      "PUT",
      "/api/v1/workspaces/w1/channels/c1/collaboration/recording-policy",
      {},
    ],
    [
      "PUT",
      "/api/v1/workspaces/w1/channels/c1/collaboration",
      {
        chat_locked: false,
        default_participant_role: "presenter",
        guest_camera_enabled: true,
        guest_microphone_enabled: true,
        lobby_enabled: false,
        room_mode: "public",
      },
    ],
    [
      "PUT",
      "/api/v1/workspaces/w1/channels/c1/collaboration",
      { default_participant_role: "presenter", room_mode: "public" },
    ],
  ])(
    "classifies conditional positive UGC mutation %s %s",
    (method, path, body) => {
      expect(isUGCMutationRequest(method, path, body)).toBe(true);
    },
  );

  it.each([
    [
      "POST",
      "/api/v1/workspaces/w1/channels/c1/collaboration/meetings/m1/end",
      undefined,
    ],
    [
      "POST",
      "/api/v1/workspaces/w1/channels/c1/collaboration/federation-invites/i1/revoked",
      undefined,
    ],
    [
      "PUT",
      "/api/v1/workspaces/w1/channels/c1/collaboration/recording-policy",
      { enabled: false },
    ],
    [
      "PUT",
      "/api/v1/workspaces/w1/channels/c1/collaboration/recordings/r1/consent",
      { consented: false },
    ],
    [
      "PATCH",
      "/api/v1/workspaces/w1/channels/c1/collaboration/roles/u1",
      { role: "listener" },
    ],
    [
      "PUT",
      "/api/v1/workspaces/w1/channels/c1/collaboration/breakouts/b1/assignments",
      { assigned_user_ids: [] },
    ],
    [
      "PUT",
      "/api/v1/workspaces/w1/channels/c1/collaboration",
      {
        chat_locked: true,
        default_participant_role: "listener",
        guest_camera_enabled: false,
        guest_microphone_enabled: false,
        lobby_enabled: true,
        room_mode: "internal",
      },
    ],
  ])("keeps conditional safety cleanup exempt %s %s", (method, path, body) => {
    expect(isUGCMutationRequest(method, path, body)).toBe(false);
  });

  it.each([
    ["GET", "/api/v1/workspaces/w1/channels"],
    ["POST", "/api/v1/workspaces/w1/moderation/reports"],
    ["POST", "/api/v1/workspaces/w1/users/u1/block"],
    ["PATCH", "/api/v1/users/me/settings"],
    ["DELETE", "/api/v1/users/me"],
    ["POST", "/api/v1/auth/logout"],
    ["DELETE", "/api/v1/workspaces/w1/channels/c1/messages/m1"],
    ["DELETE", "/api/v1/workspaces/w1/channels/c1/messages/m1/reactions/like"],
  ])("keeps recovery/safety request exempt %s %s", (method, path) => {
    expect(isUGCMutationRequest(method, path)).toBe(false);
  });

  it("keys completion by server, user, and selected workspace", () => {
    legalAcceptanceGate.reset();
    const workspaceA = createLegalAcceptanceScope(
      "https://chat.example.com/",
      "user-1",
      "workspace-a",
    );
    const workspaceB = createLegalAcceptanceScope(
      "https://chat.example.com",
      "user-1",
      "workspace-b",
    );
    expect(workspaceA).toBe("https://chat.example.com::user-1::workspace-a");
    expect(workspaceB).not.toBe(workspaceA);
    expect(
      workspaceIdFromApiPath(
        "/api/v1/workspaces/workspace-b/channels?limit=10",
      ),
    ).toBe("workspace-b");

    legalAcceptanceGate.markComplete(workspaceA as string);
    expect(() =>
      legalAcceptanceGate.assertRequest(
        {
          auth: true,
          method: "POST",
          path: "/api/v1/workspaces/workspace-a/channels",
        },
        workspaceA,
      ),
    ).not.toThrow();
    expect(() =>
      legalAcceptanceGate.assertRequest(
        {
          auth: true,
          method: "POST",
          path: "/api/v1/workspaces/workspace-b/channels",
        },
        workspaceB,
      ),
    ).toThrowError(ApiClientError);
    expect(legalAcceptanceGate.getSnapshot()).toMatchObject({
      kind: "checking",
      scope: workspaceB,
    });
  });

  it("clears same-workspace unavailable state before a successful retry", () => {
    legalAcceptanceGate.reset();
    const scope = createLegalAcceptanceScope(
      "https://chat.example.com",
      "user-1",
      "workspace-1",
    ) as string;
    legalAcceptanceGate.markUnavailable(scope, "temporary database error");
    expect(legalAcceptanceGate.getSnapshot()).toMatchObject({
      detail: "temporary database error",
      kind: "unavailable",
      scope,
    });

    legalAcceptanceGate.markChecking(scope);
    expect(legalAcceptanceGate.getSnapshot()).toMatchObject({
      kind: "checking",
      scope,
    });
    expect(legalAcceptanceGate.getSnapshot().detail).toBeUndefined();
    legalAcceptanceGate.markComplete(scope);
    expect(legalAcceptanceGate.getSnapshot()).toMatchObject({
      kind: "complete",
      scope,
    });
  });

  it("clears a same-workspace server-required reason before refreshed acceptance completes", () => {
    legalAcceptanceGate.reset();
    const scope = createLegalAcceptanceScope(
      "https://chat.example.com",
      "user-1",
      "workspace-1",
    ) as string;
    legalAcceptanceGate.handleApiError(
      new ApiClientError({
        code: "LEGAL_ACCEPTANCE_REQUIRED",
        message: "Consent required",
        status: 409,
      }),
      scope,
    );
    expect(legalAcceptanceGate.getSnapshot()).toMatchObject({
      kind: "required",
      reason: "server",
      scope,
    });

    legalAcceptanceGate.markChecking(scope);
    expect(legalAcceptanceGate.getSnapshot()).toMatchObject({
      kind: "checking",
      scope,
    });
    expect(legalAcceptanceGate.getSnapshot().reason).toBeUndefined();
    legalAcceptanceGate.markComplete(scope);
    expect(() =>
      legalAcceptanceGate.assertRequest(
        {
          auth: true,
          method: "POST",
          path: "/api/v1/workspaces/workspace-1/channels",
        },
        scope,
      ),
    ).not.toThrow();
  });

  it("fails closed for missing config, version mismatch, and malformed timestamp evidence", () => {
    expect(createLegalPolicyConfig({}).configurationError).toContain(
      "NEXT_PUBLIC_TERMS_URL",
    );
    const resolution = resolveCurrentLegalDocuments(documentList);
    expect(resolution.error).toBeNull();
    expect(
      legalDocumentsCompatibilityError(resolution.documents!, policyConfig),
    ).toBeNull();
    expect(
      legalDocumentsCompatibilityError(
        {
          ...resolution.documents!,
          terms: { ...resolution.documents!.terms, version: "2026-09-01" },
        },
        policyConfig,
      ),
    ).toContain("không khớp");

    expect(isCompleteLegalAcceptance(completeAcceptance)).toBe(true);
    const malformed = {
      ...completeAcceptance,
      terms: { ...completeAcceptance.terms, accepted_at: null },
    };
    expect(isCompleteLegalAcceptance(malformed)).toBe(false);
    expect(
      legalAcceptanceCompatibilityError(
        malformed,
        resolution.documents!,
        policyConfig,
        "workspace-1",
      ),
    ).toContain("thời điểm");
    expect(
      legalAcceptanceCompatibilityError(
        completeAcceptance,
        resolution.documents!,
        policyConfig,
        "workspace-2",
      ),
    ).toContain("workspace không mong đợi");
    const missingWorkspace = {
      ...completeAcceptance,
      workspace_id: undefined,
    } as unknown as CurrentLegalAcceptance;
    expect(
      legalAcceptanceCompatibilityError(
        missingWorkspace,
        resolution.documents!,
        policyConfig,
        "workspace-1",
      ),
    ).toContain("không xác định");
  });
});

describe("legal acceptance API contract", () => {
  it("uses public discovery and authenticated acceptance envelopes", async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ data: { documents: documentList }, success: true }),
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            data: { legal_acceptance: completeAcceptance },
            success: true,
          }),
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            data: { legal_acceptance: completeAcceptance },
            success: true,
          }),
        ),
      );
    const auth = createAuthClient(
      new HttpClient({
        baseUrl: "https://chat.example.com",
        fetcher,
        getAccessToken: () => "token",
      }),
    );

    await expect(auth.legalDocuments()).resolves.toEqual(documentList);
    await expect(auth.legalAcceptance("workspace-1")).resolves.toEqual(
      completeAcceptance,
    );
    await expect(
      auth.acceptLegalDocuments({
        privacy_accepted: true,
        privacy_version: "2026-08-07",
        terms_accepted: true,
        terms_version: "2026-08-07",
        workspace_id: "workspace-1",
      }),
    ).resolves.toEqual(completeAcceptance);

    expect(
      (fetcher.mock.calls[0][1].headers as Headers).has("Authorization"),
    ).toBe(false);
    expect(
      (fetcher.mock.calls[1][1].headers as Headers).get("Authorization"),
    ).toBe("Bearer token");
    expect(fetcher.mock.calls[2][1].body).toBe(
      JSON.stringify({
        privacy_accepted: true,
        privacy_version: "2026-08-07",
        terms_accepted: true,
        terms_version: "2026-08-07",
        workspace_id: "workspace-1",
      }),
    );
    expect(fetcher.mock.calls[1][0]).toBe(
      "https://chat.example.com/api/v1/auth/legal-acceptance?workspace_id=workspace-1",
    );
  });

  it("runs request guards before network and reports backend legal errors", async () => {
    const fetcher = vi.fn(
      async () =>
        new Response(
          JSON.stringify({
            error: {
              code: "LEGAL_ACCEPTANCE_REQUIRED",
              message: "Consent required",
            },
            success: false,
          }),
          { status: 409 },
        ),
    );
    const beforeRequest = vi.fn(() => {
      throw new ApiClientError({
        code: "LEGAL_ACCEPTANCE_REQUIRED",
        message: "blocked",
        status: 409,
      });
    });
    const guarded = new HttpClient({
      baseUrl: "https://chat.example.com",
      beforeRequest,
      fetcher,
    });
    await expect(
      guarded.post("/api/v1/workspaces/w1/channels", {}),
    ).rejects.toMatchObject({ code: "LEGAL_ACCEPTANCE_REQUIRED" });
    expect(fetcher).not.toHaveBeenCalled();

    const onRequestError = vi.fn();
    const serverGuarded = new HttpClient({
      baseUrl: "https://chat.example.com",
      fetcher,
      onRequestError,
    });
    await expect(
      serverGuarded.post("/api/v1/workspaces/w1/channels", {}),
    ).rejects.toMatchObject({ status: 409 });
    expect(onRequestError).toHaveBeenCalledWith(
      expect.objectContaining({ code: "LEGAL_ACCEPTANCE_REQUIRED" }),
      expect.objectContaining({
        method: "POST",
        path: "/api/v1/workspaces/w1/channels",
      }),
    );
  });
});
