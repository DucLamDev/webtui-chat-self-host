"use client";

import { useMemo, useState } from "react";
import {
  type InfiniteData,
  useInfiniteQuery,
  useMutation,
  useQueryClient
} from "@tanstack/react-query";
import { queryKeys } from "@webtui/api-client";
import { CheckCircle2, Clock3, RefreshCw, ShieldCheck, X } from "@webtui/icons";
import { Badge, Button, EmptyState, ErrorState, Skeleton } from "@webtui/ui";
import type {
  ModerationReport,
  ModerationReportStatus
} from "@webtui/types";
import { api } from "@/lib/api";
import {
  buildModerationEvidence,
  filterModerationReports,
  moderationReasonLabels,
  moderationStatusLabels,
  type ModerationStatusFilter,
  type ModerationTargetFilter,
  validateModerationResolution
} from "../model/moderation";

type ModerationMutationInput = {
  note: string;
  reportId: string;
  status: ModerationReportStatus;
};

type ModerationReportPage = {
  hasMoreStatuses: ModerationReportStatus[];
  reports: ModerationReport[];
};

const moderationStatuses: ModerationReportStatus[] = ["pending", "reviewing", "resolved", "dismissed"];

export function ModerationSection({
  canManage,
  showToast,
  workspaceId
}: {
  canManage: boolean;
  showToast: (message: string, tone?: "danger" | "info" | "success") => void;
  workspaceId: string;
}) {
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState<ModerationStatusFilter>("open");
  const [targetFilter, setTargetFilter] = useState<ModerationTargetFilter>("all");
  const [notes, setNotes] = useState<Record<string, string>>({});
  const [validationErrors, setValidationErrors] = useState<Record<string, string>>({});
  const reportsQuery = useInfiniteQuery<
    ModerationReportPage,
    Error,
    InfiniteData<ModerationReportPage>,
    ReturnType<typeof queryKeys.admin.moderationReports>,
    number
  >({
    enabled: Boolean(workspaceId && canManage),
    getNextPageParam: (lastPage, pages) => lastPage.hasMoreStatuses.length ? pages.length : undefined,
    initialPageParam: 0,
    queryFn: async ({ pageParam }) => {
      const queues = await Promise.all(
        moderationStatuses.map((status) => api.moderation.listReports(workspaceId, {
          limit: 100,
          offset: pageParam * 100,
          status,
          target_type: targetFilter === "all" ? undefined : targetFilter
        }))
      );
      return {
        hasMoreStatuses: moderationStatuses.filter((_, index) => queues[index]?.length === 100),
        reports: queues.flat()
      };
    },
    queryKey: queryKeys.admin.moderationReports(workspaceId, "", targetFilter),
    retry: false
  });
  const reports = useMemo(() => {
    const reportsById = new Map<string, ModerationReport>();
    for (const page of reportsQuery.data?.pages ?? []) {
      for (const report of page.reports) {
        if (!reportsById.has(report.id)) reportsById.set(report.id, report);
      }
    }
    return [...reportsById.values()];
  }, [reportsQuery.data]);
  const visibleReports = useMemo(
    () => filterModerationReports(reports, statusFilter, targetFilter),
    [reports, statusFilter, targetFilter]
  );
  const counts = useMemo(() => ({
    pending: reports.filter((report) => report.status === "pending").length,
    reviewing: reports.filter((report) => report.status === "reviewing").length,
    closed: reports.filter((report) => report.status === "resolved" || report.status === "dismissed").length
  }), [reports]);
  const nextStatuses = reportsQuery.data?.pages.at(-1)?.hasMoreStatuses ?? [];
  const hasMoreVisibleReports = nextStatuses.some((status) =>
    statusFilter === "all"
      || (statusFilter === "open" ? status === "pending" || status === "reviewing" : status === statusFilter)
  );

  const updateMutation = useMutation({
    mutationFn: ({ note, reportId, status }: ModerationMutationInput) =>
      api.moderation.updateReport(workspaceId, reportId, {
        resolution_note: note.trim() || undefined,
        status
      }),
    onError: (error, variables) => {
      setValidationErrors((current) => ({ ...current, [variables.reportId]: errorMessage(error) }));
      showToast(errorMessage(error), "danger");
    },
    onSuccess: async (_, variables) => {
      setNotes((current) => omitKey(current, variables.reportId));
      setValidationErrors((current) => omitKey(current, variables.reportId));
      await queryClient.invalidateQueries({
        queryKey: queryKeys.admin.moderationReportsRoot(workspaceId)
      });
      showToast(`Đã chuyển báo cáo sang “${moderationStatusLabels[variables.status]}”.`);
    }
  });

  function updateReport(report: ModerationReport, status: ModerationReportStatus) {
    const note = notes[report.id] ?? "";
    const validationError = validateModerationResolution(status, note);
    if (validationError) {
      setValidationErrors((current) => ({ ...current, [report.id]: validationError }));
      return;
    }
    setValidationErrors((current) => omitKey(current, report.id));
    updateMutation.mutate({ note, reportId: report.id, status });
  }

  if (!canManage) {
    return (
      <ErrorState
        description="Tài khoản hiện tại chưa có quyền `moderation.manage` trong workspace này."
        title="Chưa đủ quyền kiểm duyệt"
      />
    );
  }

  if (reportsQuery.isLoading) {
    return (
      <section aria-label="Đang tải hàng đợi kiểm duyệt" className="admin-content-stack moderation-queue">
        <Skeleton style={{ height: 96 }} />
        <Skeleton style={{ height: 240 }} />
        <Skeleton style={{ height: 240 }} />
      </section>
    );
  }

  if (reportsQuery.isError) {
    return (
      <ErrorState
        action={<Button onClick={() => void reportsQuery.refetch()} size="sm" variant="secondary">Thử lại</Button>}
        description={errorMessage(reportsQuery.error)}
        title="Không tải được hàng đợi kiểm duyệt"
      />
    );
  }

  return (
    <section className="admin-content-stack moderation-queue">
      <div className="admin-summary-grid moderation-summary" aria-label="Tóm tắt hàng đợi">
        <Summary label="Chờ xử lý" tone="orange" value={counts.pending} />
        <Summary label="Đang xem xét" tone="blue" value={counts.reviewing} />
        <Summary label="Đã đóng" tone="green" value={counts.closed} />
      </div>

      <div className="admin-filter-bar moderation-filters">
        <label className="admin-filter-control">
          <span>Trạng thái</span>
          <select
            aria-label="Lọc báo cáo theo trạng thái"
            onChange={(event) => setStatusFilter(event.target.value as ModerationStatusFilter)}
            value={statusFilter}
          >
            <option value="open">Đang mở</option>
            <option value="pending">Chờ xử lý</option>
            <option value="reviewing">Đang xem xét</option>
            <option value="resolved">Đã xử lý</option>
            <option value="dismissed">Đã bác bỏ</option>
            <option value="all">Tất cả</option>
          </select>
        </label>
        <label className="admin-filter-control">
          <span>Đối tượng</span>
          <select
            aria-label="Lọc báo cáo theo đối tượng"
            onChange={(event) => setTargetFilter(event.target.value as ModerationTargetFilter)}
            value={targetFilter}
          >
            <option value="all">Tin nhắn và người dùng</option>
            <option value="message">Tin nhắn</option>
            <option value="user">Người dùng</option>
          </select>
        </label>
        <span aria-live="polite" className="moderation-filter-count">
          {visibleReports.length} / {reports.length} báo cáo
        </span>
        <Button
          disabled={reportsQuery.isFetching}
          onClick={() => void reportsQuery.refetch()}
          size="sm"
          variant="secondary"
        >
          <RefreshCw aria-hidden className={reportsQuery.isFetching ? "admin-spin" : undefined} size={15} />
          Làm mới
        </Button>
      </div>

      {!visibleReports.length ? (
        <EmptyState
          description={reports.length ? "Không có báo cáo khớp bộ lọc hiện tại." : "Workspace chưa có báo cáo kiểm duyệt nào."}
          title={reports.length ? "Không có kết quả" : "Hàng đợi đang trống"}
        />
      ) : (
        <div className="moderation-report-list">
          {visibleReports.map((report) => (
            <ModerationReportCard
              actionError={validationErrors[report.id]}
              isUpdating={updateMutation.isPending && updateMutation.variables?.reportId === report.id}
              key={report.id}
              note={notes[report.id] ?? ""}
              onNoteChange={(value) => {
                setNotes((current) => ({ ...current, [report.id]: value }));
                if (validationErrors[report.id]) {
                  setValidationErrors((current) => omitKey(current, report.id));
                }
              }}
              onUpdate={(status) => updateReport(report, status)}
              report={report}
            />
          ))}
        </div>
      )}
      {reportsQuery.hasNextPage && hasMoreVisibleReports ? (
        <div className="moderation-load-more">
          <Button
            disabled={reportsQuery.isFetchingNextPage}
            onClick={() => void reportsQuery.fetchNextPage()}
            size="sm"
            variant="secondary"
          >
            {reportsQuery.isFetchingNextPage ? "Đang tải…" : "Tải thêm báo cáo"}
          </Button>
        </div>
      ) : null}
    </section>
  );
}

function ModerationReportCard({
  actionError,
  isUpdating,
  note,
  onNoteChange,
  onUpdate,
  report
}: {
  actionError?: string;
  isUpdating: boolean;
  note: string;
  onNoteChange: (value: string) => void;
  onUpdate: (status: ModerationReportStatus) => void;
  report: ModerationReport;
}) {
  const evidence = buildModerationEvidence(report.target_type, report.target_snapshot);
  const targetLabel = report.target_type === "message" ? "Tin nhắn" : "Người dùng";
  const targetName = report.target_user_display_name || shortId(report.target_id);
  const noteId = `moderation-resolution-${report.id}`;
  const errorId = `moderation-error-${report.id}`;

  return (
    <article className="admin-panel moderation-report-card">
      <header className="moderation-report-card__header">
        <div>
          <span className="moderation-report-card__eyebrow">{targetLabel} · {shortId(report.id)}</span>
          <h2>{moderationReasonLabels[report.reason]}</h2>
        </div>
        <Badge tone={statusTone(report.status)}>{moderationStatusLabels[report.status]}</Badge>
      </header>

      <dl className="moderation-report-meta">
        <div><dt>Đối tượng</dt><dd>{targetName}</dd></div>
        <div><dt>Người báo cáo</dt><dd>{report.reporter_display_name || "Tài khoản đã ẩn/xóa"}</dd></div>
        <div><dt>Tiếp nhận</dt><dd><time dateTime={report.created_at}>{formatDateTime(report.created_at)}</time></dd></div>
        <div><dt>Cập nhật</dt><dd><time dateTime={report.updated_at}>{formatDateTime(report.updated_at)}</time></dd></div>
      </dl>

      {report.details ? (
        <div className="moderation-report-details">
          <strong>Mô tả của người báo cáo</strong>
          <p>{report.details}</p>
        </div>
      ) : null}

      <section aria-label="Bản chụp bằng chứng bất biến" className="moderation-evidence">
        <header>
          <ShieldCheck aria-hidden size={17} />
          <div>
            <strong>Bản chụp bằng chứng bất biến</strong>
            <small>Chỉ hiển thị các trường tối thiểu đã được backend cho phép.</small>
          </div>
        </header>
        {!evidence.retained ? (
          <p className="moderation-evidence__expired">Bằng chứng đã hết thời hạn lưu giữ hoặc không còn khả dụng.</p>
        ) : (
          <>
            {evidence.fields.length ? (
              <dl>
                {evidence.fields.map((field) => (
                  <div key={field.label}><dt>{field.label}</dt><dd>{formatEvidenceValue(field.label, field.value)}</dd></div>
                ))}
              </dl>
            ) : null}
            {evidence.excerpt ? <blockquote>{evidence.excerpt}</blockquote> : null}
            {evidence.digest ? <p className="moderation-evidence__digest"><span>SHA-256</span><code>{evidence.digest}</code></p> : null}
          </>
        )}
      </section>

      {report.status === "reviewing" ? (
        <div className="moderation-resolution">
          <label htmlFor={noteId}>Ghi chú kết quả <span aria-hidden="true">*</span></label>
          <textarea
            aria-describedby={actionError ? errorId : undefined}
            id={noteId}
            maxLength={2000}
            onChange={(event) => onNoteChange(event.target.value)}
            placeholder="Nêu nội dung đã kiểm tra, quyết định và hành động đã thực hiện..."
            rows={3}
            value={note}
          />
          <div><small>{Array.from(note).length}/2.000 ký tự</small></div>
        </div>
      ) : report.resolution_note ? (
        <div className="moderation-report-details">
          <strong>Kết quả đã ghi nhận</strong>
          <p>{report.resolution_note}</p>
        </div>
      ) : null}

      {actionError ? <p className="moderation-action-error" id={errorId} role="alert">{actionError}</p> : null}

      <footer className="moderation-actions">
        {report.status === "pending" ? (
          <Button disabled={isUpdating} onClick={() => onUpdate("reviewing")} size="sm">
            <Clock3 aria-hidden size={15} />
            Bắt đầu xem xét
          </Button>
        ) : null}
        {report.status === "reviewing" ? (
          <>
            <Button disabled={isUpdating} onClick={() => onUpdate("resolved")} size="sm">
              <CheckCircle2 aria-hidden size={15} />
              Đã xử lý vi phạm
            </Button>
            <Button disabled={isUpdating} onClick={() => onUpdate("dismissed")} size="sm" variant="secondary">
              <X aria-hidden size={15} />
              Bác bỏ báo cáo
            </Button>
          </>
        ) : null}
        {isUpdating ? <span aria-live="polite">Đang cập nhật…</span> : null}
      </footer>
    </article>
  );
}

function Summary({ label, tone, value }: { label: string; tone: "blue" | "green" | "orange"; value: number }) {
  return (
    <div className={`admin-summary-card admin-summary-card--${tone}`}>
      <span aria-hidden="true"><ShieldCheck size={19} /></span>
      <div><small>{label}</small><strong>{value}</strong><em>Báo cáo</em></div>
    </div>
  );
}

function statusTone(status: ModerationReportStatus): "green" | "orange" | "blue" | "slate" {
  if (status === "pending") return "orange";
  if (status === "reviewing") return "blue";
  if (status === "resolved") return "green";
  return "slate";
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Không xác định";
  return new Intl.DateTimeFormat("vi-VN", {
    dateStyle: "short",
    timeStyle: "short"
  }).format(date);
}

function formatEvidenceValue(label: string, value: string): string {
  return label.includes("Thời điểm") || label.includes("Ngày tạo") ? formatDateTime(value) : value;
}

function shortId(value: string): string {
  return value.length > 12 ? `${value.slice(0, 8)}…` : value;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Yêu cầu kiểm duyệt không thành công.";
}

function omitKey<T>(record: Record<string, T>, key: string): Record<string, T> {
  const remaining = { ...record };
  delete remaining[key];
  return remaining;
}
