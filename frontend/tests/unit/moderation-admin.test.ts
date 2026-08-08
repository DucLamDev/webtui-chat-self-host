import { afterEach, describe, expect, it, vi } from "vitest";
import { createModerationClient, HttpClient } from "@webtui/api-client";
import type { ModerationReport } from "@webtui/types";
import {
  buildModerationEvidence,
  filterModerationReports,
  validateModerationResolution,
} from "../../apps/admin/src/features/dashboard/model/moderation";

const baseReport: ModerationReport = {
  created_at: "2026-08-07T10:00:00Z",
  id: "11111111-1111-4111-8111-111111111111",
  reason: "harassment",
  status: "pending",
  target_id: "22222222-2222-4222-8222-222222222222",
  target_type: "message",
  updated_at: "2026-08-07T10:00:00Z",
  workspace_id: "33333333-3333-4333-8333-333333333333",
};

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), {
    headers: { "content-type": "application/json" },
    status: 200,
  });
}

describe("moderation API client", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("loads the permissioned report envelope with filters", async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({ data: { reports: [baseReport] }, success: true }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const moderation = createModerationClient(
      new HttpClient({ baseUrl: "https://chat.example.test" }),
    );

    await expect(
      moderation.listReports("workspace/one", {
        limit: 50,
        offset: 10,
        status: "reviewing",
        target_type: "message",
      }),
    ).resolves.toEqual([baseReport]);

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "https://chat.example.test/api/v1/workspaces/workspace%2Fone/moderation/reports?limit=50&offset=10&status=reviewing&target_type=message",
    );
    expect(fetchMock.mock.calls[0]?.[1].method).toBe("GET");
  });

  it("patches the exact report route and resolution contract", async () => {
    const resolved = {
      ...baseReport,
      resolution_note: "Đã xóa nội dung vi phạm.",
      status: "resolved" as const,
    };
    const fetchMock = vi.fn(async () =>
      jsonResponse({ data: resolved, success: true }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const moderation = createModerationClient(
      new HttpClient({ baseUrl: "https://chat.example.test" }),
    );

    await expect(
      moderation.updateReport("workspace-one", "report/one", {
        resolution_note: "Đã xóa nội dung vi phạm.",
        status: "resolved",
      }),
    ).resolves.toEqual(resolved);

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "https://chat.example.test/api/v1/workspaces/workspace-one/moderation/reports/report%2Fone",
    );
    expect(fetchMock.mock.calls[0]?.[1].method).toBe("PATCH");
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1].body))).toEqual({
      resolution_note: "Đã xóa nội dung vi phạm.",
      status: "resolved",
    });
  });
});

describe("moderation admin safety", () => {
  it("shows both open statuses by default and sorts newest first", () => {
    const reports: ModerationReport[] = [
      baseReport,
      {
        ...baseReport,
        created_at: "2026-08-07T12:00:00Z",
        id: "reviewing",
        status: "reviewing",
      },
      {
        ...baseReport,
        created_at: "2026-08-07T14:00:00Z",
        id: "closed",
        status: "resolved",
      },
    ];

    expect(
      filterModerationReports(reports, "open", "all").map(
        (report) => report.id,
      ),
    ).toEqual(["reviewing", baseReport.id]);
    expect(filterModerationReports(reports, "all", "user")).toEqual([]);
  });

  it("requires a bounded resolution note only when closing", () => {
    expect(validateModerationResolution("resolved", "   ")).toContain(
      "Cần ghi chú",
    );
    expect(
      validateModerationResolution("dismissed", "x".repeat(2001)),
    ).toContain("2.000");
    expect(
      validateModerationResolution("resolved", "Đã kiểm tra và xử lý."),
    ).toBeNull();
    expect(validateModerationResolution("reviewing", "")).toBeNull();
  });

  it("allowlists immutable message evidence and never exposes unknown snapshot fields", () => {
    const evidence = buildModerationEvidence("message", {
      attachment_payload: "private-binary-data",
      body_excerpt: "<img src=x onerror=alert(1)>",
      body_sha256: "a".repeat(64),
      channel_id: "44444444-4444-4444-8444-444444444444",
      created_at: "2026-08-07T09:00:00Z",
      producer_kind: "integration",
      sender_display_name: "Người gửi",
      server_secret: "must-not-render",
    });

    expect(evidence.excerpt).toBe("<img src=x onerror=alert(1)>");
    expect(evidence.digest).toBe("a".repeat(64));
    expect(JSON.stringify(evidence)).not.toContain("private-binary-data");
    expect(JSON.stringify(evidence)).not.toContain("must-not-render");
    expect(evidence.fields.map((field) => field.label)).toEqual([
      "Người gửi",
      "Nguồn nội dung",
      "Thời điểm gửi",
      "Mã kênh",
    ]);
  });

  it("treats an empty or expired snapshot as unavailable evidence", () => {
    expect(buildModerationEvidence("user", {})).toEqual({
      fields: [],
      retained: false,
    });
    expect(buildModerationEvidence("message", undefined)).toEqual({
      fields: [],
      retained: false,
    });
  });
});
