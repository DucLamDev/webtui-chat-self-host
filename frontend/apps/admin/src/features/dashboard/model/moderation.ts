import type {
  ModerationReport,
  ModerationReportReason,
  ModerationReportStatus,
  ModerationReportTargetType,
  ModerationTargetSnapshot,
} from "@webtui/types";

export type ModerationStatusFilter = "open" | "all" | ModerationReportStatus;
export type ModerationTargetFilter = "all" | ModerationReportTargetType;

export type ModerationEvidence = {
  digest?: string;
  excerpt?: string;
  fields: Array<{ label: string; value: string }>;
  retained: boolean;
};

export const moderationReasonLabels: Record<ModerationReportReason, string> = {
  spam: "Spam",
  harassment: "Quấy rối",
  hate_speech: "Ngôn từ thù ghét",
  sexual_content: "Nội dung tình dục",
  violence: "Bạo lực",
  illegal_content: "Nội dung bất hợp pháp",
  privacy: "Xâm phạm quyền riêng tư",
  impersonation: "Mạo danh",
  other: "Khác",
};

export const moderationStatusLabels: Record<ModerationReportStatus, string> = {
  pending: "Chờ xử lý",
  reviewing: "Đang xem xét",
  resolved: "Đã xử lý",
  dismissed: "Đã bác bỏ",
};

export function filterModerationReports(
  reports: ModerationReport[],
  status: ModerationStatusFilter,
  target: ModerationTargetFilter,
): ModerationReport[] {
  return reports
    .filter((report) => {
      const statusMatches =
        status === "all" ||
        (status === "open"
          ? report.status === "pending" || report.status === "reviewing"
          : report.status === status);
      return (
        statusMatches && (target === "all" || report.target_type === target)
      );
    })
    .sort(
      (left, right) =>
        Date.parse(right.created_at) - Date.parse(left.created_at),
    );
}

export function validateModerationResolution(
  status: ModerationReportStatus,
  note: string,
): string | null {
  const normalized = note.trim();
  if ((status === "resolved" || status === "dismissed") && !normalized) {
    return "Cần ghi chú kết quả trước khi đóng báo cáo.";
  }
  if (Array.from(normalized).length > 2000) {
    return "Ghi chú kết quả không được vượt quá 2.000 ký tự.";
  }
  return null;
}

export function buildModerationEvidence(
  targetType: ModerationReportTargetType,
  snapshot: ModerationTargetSnapshot | null | undefined,
): ModerationEvidence {
  if (
    !snapshot ||
    typeof snapshot !== "object" ||
    !Object.keys(snapshot).length
  ) {
    return { fields: [], retained: false };
  }

  if (targetType === "message") {
    const fields = compactEvidenceFields([
      ["Người gửi", safeText(snapshot.sender_display_name, 160)],
      ["Nguồn nội dung", safeText(snapshot.producer_kind, 40)],
      ["Loại tin", safeText(snapshot.kind, 40)],
      ["Thời điểm gửi", safeDateTime(snapshot.created_at)],
      ["Mã kênh", safeIdentifier(snapshot.channel_id)],
      ["Mã tin nhắn", safeIdentifier(snapshot.message_id)],
    ]);
    const excerpt = safeText(snapshot.body_excerpt, 2000, true);
    const digest = safeDigest(snapshot.body_sha256);
    return {
      digest,
      excerpt,
      fields,
      retained: Boolean(fields.length || excerpt || digest),
    };
  }

  const fields = compactEvidenceFields([
    ["Tên hiển thị", safeText(snapshot.display_name, 160)],
    ["Tên người dùng", safeText(snapshot.username, 160)],
    ["Ngày tạo tài khoản", safeDateTime(snapshot.created_at)],
    ["Mã người dùng", safeIdentifier(snapshot.user_id)],
  ]);
  return { fields, retained: Boolean(fields.length) };
}

function compactEvidenceFields(
  fields: Array<readonly [string, string | undefined]>,
): Array<{ label: string; value: string }> {
  return fields.flatMap(([label, value]) => (value ? [{ label, value }] : []));
}

function safeText(
  value: unknown,
  maxLength: number,
  preserveWhitespace = false,
): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }
  const normalized = preserveWhitespace ? value : value.trim();
  if (!normalized) {
    return undefined;
  }
  return Array.from(normalized).slice(0, maxLength).join("");
}

function safeIdentifier(value: unknown): string | undefined {
  const identifier = safeText(value, 80);
  return identifier && /^[a-zA-Z0-9-]+$/.test(identifier)
    ? identifier
    : undefined;
}

function safeDigest(value: unknown): string | undefined {
  const digest = safeText(value, 64);
  return digest && /^[a-fA-F0-9]{64}$/.test(digest)
    ? digest.toLowerCase()
    : undefined;
}

function safeDateTime(value: unknown): string | undefined {
  const dateTime = safeText(value, 64);
  return dateTime && !Number.isNaN(Date.parse(dateTime)) ? dateTime : undefined;
}
