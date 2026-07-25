"use client";

import { type FormEvent, useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@webtui/api-client";
import { Badge, Button, EmptyState, ErrorState, SegmentedControl, Skeleton } from "@webtui/ui";
import { CheckCircle2, Clock3, Cloud, FileText, Info, Plus, Send, Share2, Trash2, Workflow, X, Zap } from "@webtui/icons";
import type { CronJob, IncomingWebhook, JsonObject, OutgoingWebhook, WebhookDelivery } from "@webtui/types";
import { api } from "@/lib/api";
import type { ChatChannel } from "../model/types";

type AutomationTab = "jobs" | "webhooks";
type AutomationFeedback = { message: string; tone: "error" | "success" };

const automationTabs = [
  { label: "Quy trình", value: "jobs" as const },
  { label: "Webhook", value: "webhooks" as const }
];

export function AutomationPage({
  canManageCronjobs,
  canManageWebhooks,
  channels,
  workspaceId
}: {
  canManageCronjobs: boolean;
  canManageWebhooks: boolean;
  channels: ChatChannel[];
  workspaceId?: string;
}) {
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState<AutomationTab>(canManageCronjobs ? "jobs" : "webhooks");
  const [isCreateJobOpen, setIsCreateJobOpen] = useState(false);
  const [isCreateWebhookOpen, setIsCreateWebhookOpen] = useState(false);
  const [selectedJobId, setSelectedJobId] = useState("");
  const [selectedIncomingId, setSelectedIncomingId] = useState("");
  const [selectedOutgoingId, setSelectedOutgoingId] = useState("");
  const [feedback, setFeedback] = useState<AutomationFeedback | null>(null);
  const [createdSecret, setCreatedSecret] = useState<{ label: string; secret: string; url?: string } | null>(null);

  useEffect(() => {
    if (activeTab === "jobs" && !canManageCronjobs && canManageWebhooks) setActiveTab("webhooks");
    if (activeTab === "webhooks" && !canManageWebhooks && canManageCronjobs) setActiveTab("jobs");
  }, [activeTab, canManageCronjobs, canManageWebhooks]);

  const jobsQuery = useQuery({
    enabled: Boolean(workspaceId && canManageCronjobs),
    queryFn: () => api.cronjobs.list(workspaceId as string),
    queryKey: queryKeys.operations.cronjobs(workspaceId ?? "")
  });
  const incomingQuery = useQuery({
    enabled: Boolean(workspaceId && canManageWebhooks),
    queryFn: () => api.webhooks.incoming(workspaceId as string),
    queryKey: queryKeys.integrations.incomingWebhooks(workspaceId ?? "")
  });
  const outgoingQuery = useQuery({
    enabled: Boolean(workspaceId && canManageWebhooks),
    queryFn: () => api.webhooks.outgoing(workspaceId as string),
    queryKey: queryKeys.integrations.outgoingWebhooks(workspaceId ?? "")
  });

  const jobs: CronJob[] = jobsQuery.data ?? [];
  const incomingWebhooks: IncomingWebhook[] = incomingQuery.data ?? [];
  const outgoingWebhooks: OutgoingWebhook[] = outgoingQuery.data ?? [];
  const selectedJob = jobs.find((job) => job.id === selectedJobId) ?? jobs[0];
  const selectedIncoming = incomingWebhooks.find((webhook) => webhook.id === selectedIncomingId) ?? incomingWebhooks[0];
  const selectedOutgoing = outgoingWebhooks.find((webhook) => webhook.id === selectedOutgoingId) ?? outgoingWebhooks[0];

  useEffect(() => {
    if (jobs.length && !jobs.some((job) => job.id === selectedJobId)) setSelectedJobId(jobs[0].id);
  }, [jobs, selectedJobId]);
  useEffect(() => {
    if (incomingWebhooks.length && !incomingWebhooks.some((webhook) => webhook.id === selectedIncomingId)) {
      setSelectedIncomingId(incomingWebhooks[0].id);
    }
  }, [incomingWebhooks, selectedIncomingId]);
  useEffect(() => {
    if (outgoingWebhooks.length && !outgoingWebhooks.some((webhook) => webhook.id === selectedOutgoingId)) {
      setSelectedOutgoingId(outgoingWebhooks[0].id);
    }
  }, [outgoingWebhooks, selectedOutgoingId]);

  const runsQuery = useQuery({
    enabled: Boolean(workspaceId && selectedJob?.id && canManageCronjobs),
    queryFn: () => api.cronjobs.runs(workspaceId as string, selectedJob?.id ?? "", { limit: 8 }),
    queryKey: queryKeys.operations.cronJobRuns(workspaceId ?? "", selectedJob?.id ?? "")
  });
  const deliveriesQuery = useQuery({
    enabled: Boolean(workspaceId && selectedOutgoing?.id && canManageWebhooks),
    queryFn: () => api.webhooks.deliveries(workspaceId as string, selectedOutgoing?.id ?? ""),
    queryKey: queryKeys.integrations.webhookDeliveries(workspaceId ?? "", selectedOutgoing?.id ?? "")
  });

  const createJobMutation = useMutation({
    mutationFn: (input: { description?: string; name: string; payload: JsonObject; runner: string; schedule: string; status: string }) =>
      api.cronjobs.create(workspaceId as string, input),
    onError: (error) => setFeedback({ message: automationError(error, "Không tạo được quy trình."), tone: "error" }),
    onSuccess: async (job) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.operations.cronjobs(workspaceId ?? "") });
      setSelectedJobId(job.id);
      setIsCreateJobOpen(false);
      setFeedback({ message: `Đã tạo quy trình ${job.name}.`, tone: "success" });
    }
  });
  const runJobMutation = useMutation({
    mutationFn: (jobId: string) => api.cronjobs.runNow(workspaceId as string, jobId),
    onError: (error) => setFeedback({ message: automationError(error, "Không chạy được quy trình."), tone: "error" }),
    onSuccess: async (_, jobId) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.operations.cronjobs(workspaceId ?? "") }),
        queryClient.invalidateQueries({ queryKey: queryKeys.operations.cronJobRuns(workspaceId ?? "", jobId) })
      ]);
      setFeedback({ message: "Quy trình đã chạy xong.", tone: "success" });
    }
  });
  const updateJobMutation = useMutation({
    mutationFn: ({ job, status, values }: { job: CronJob; status?: string; values?: { description?: string; name: string; payload: JsonObject; runner: string; schedule: string; status: string } }) => api.cronjobs.update(workspaceId as string, job.id, {
      description: values?.description ?? job.description ?? undefined,
      name: values?.name ?? job.name,
      payload: values?.payload ?? job.payload,
      runner: values?.runner ?? job.runner,
      schedule: values?.schedule ?? job.schedule,
      status: status ?? values?.status ?? job.status
    }),
    onError: (error) => setFeedback({ message: automationError(error, "Không cập nhật được quy trình."), tone: "error" }),
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: queryKeys.operations.cronjobs(workspaceId ?? "") })
  });
  const deleteJobMutation = useMutation({
    mutationFn: (jobId: string) => api.cronjobs.delete(workspaceId as string, jobId),
    onError: (error) => setFeedback({ message: automationError(error, "Không xóa được quy trình."), tone: "error" }),
    onSuccess: async () => {
      setSelectedJobId("");
      await queryClient.invalidateQueries({ queryKey: queryKeys.operations.cronjobs(workspaceId ?? "") });
      setFeedback({ message: "Đã xóa quy trình.", tone: "success" });
    }
  });
  const createIncomingMutation = useMutation({
    mutationFn: (input: { channel_id?: string; name: string }) => api.webhooks.createIncoming(workspaceId as string, input),
    onError: (error) => setFeedback({ message: automationError(error, "Không tạo được incoming webhook."), tone: "error" }),
    onSuccess: async (webhook) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.integrations.incomingWebhooks(workspaceId ?? "") });
      setCreatedSecret({ label: webhook.name, secret: webhook.secret, url: webhook.url });
      setIsCreateWebhookOpen(false);
    }
  });
  const createOutgoingMutation = useMutation({
    mutationFn: (input: { event_types?: string[]; name: string; target_url: string }) => api.webhooks.createOutgoing(workspaceId as string, input),
    onError: (error) => setFeedback({ message: automationError(error, "Không tạo được outgoing webhook."), tone: "error" }),
    onSuccess: async (webhook) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.integrations.outgoingWebhooks(workspaceId ?? "") });
      setSelectedOutgoingId(webhook.id);
      setCreatedSecret({ label: webhook.name, secret: webhook.secret });
      setIsCreateWebhookOpen(false);
    }
  });

  const updateIncomingMutation = useMutation({
    mutationFn: (input: { channel_id?: string | null; name?: string; status?: string; webhookId: string }) =>
      api.webhooks.updateIncoming(workspaceId as string, input.webhookId, {
        channel_id: input.channel_id,
        name: input.name,
        status: input.status
      }),
    onError: (error) => setFeedback({ message: automationError(error, "Không cập nhật được incoming webhook."), tone: "error" }),
    onSuccess: async (webhook) => {
      setSelectedIncomingId(webhook.id);
      await queryClient.invalidateQueries({ queryKey: queryKeys.integrations.incomingWebhooks(workspaceId ?? "") });
      setFeedback({ message: "Đã cập nhật incoming webhook.", tone: "success" });
    }
  });
  const deleteIncomingMutation = useMutation({
    mutationFn: (webhookId: string) => api.webhooks.deleteIncoming(workspaceId as string, webhookId),
    onError: (error) => setFeedback({ message: automationError(error, "Không xóa được incoming webhook."), tone: "error" }),
    onSuccess: async () => {
      setSelectedIncomingId("");
      await queryClient.invalidateQueries({ queryKey: queryKeys.integrations.incomingWebhooks(workspaceId ?? "") });
      setFeedback({ message: "Đã xóa incoming webhook.", tone: "success" });
    }
  });
  const updateOutgoingMutation = useMutation({
    mutationFn: (input: { event_types?: string[]; name?: string; status?: string; target_url?: string; webhookId: string }) =>
      api.webhooks.updateOutgoing(workspaceId as string, input.webhookId, {
        event_types: input.event_types,
        name: input.name,
        status: input.status,
        target_url: input.target_url
      }),
    onError: (error) => setFeedback({ message: automationError(error, "Không cập nhật được outgoing webhook."), tone: "error" }),
    onSuccess: async (webhook) => {
      setSelectedOutgoingId(webhook.id);
      await queryClient.invalidateQueries({ queryKey: queryKeys.integrations.outgoingWebhooks(workspaceId ?? "") });
      setFeedback({ message: "Đã cập nhật outgoing webhook.", tone: "success" });
    }
  });
  const deleteOutgoingMutation = useMutation({
    mutationFn: (webhookId: string) => api.webhooks.deleteOutgoing(workspaceId as string, webhookId),
    onError: (error) => setFeedback({ message: automationError(error, "Không xóa được outgoing webhook."), tone: "error" }),
    onSuccess: async (_, webhookId) => {
      setSelectedOutgoingId("");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.integrations.outgoingWebhooks(workspaceId ?? "") }),
        queryClient.invalidateQueries({ queryKey: queryKeys.integrations.webhookDeliveries(workspaceId ?? "", webhookId) })
      ]);
      setFeedback({ message: "Đã xóa outgoing webhook.", tone: "success" });
    }
  });
  const testOutgoingMutation = useMutation({
    mutationFn: (input: { event_type?: string; payload: JsonObject; webhookId: string }) =>
      api.webhooks.testOutgoing(workspaceId as string, input.webhookId, {
        event_type: input.event_type,
        payload: input.payload
      }),
    onError: (error) => setFeedback({ message: automationError(error, "Không gửi thử được outgoing webhook."), tone: "error" }),
    onSuccess: async (delivery) => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.integrations.webhookDeliveries(workspaceId ?? "", delivery.outgoing_webhook_id) });
      setFeedback({
        message: delivery.status === "success" ? "Webhook test đã gửi thành công." : "Webhook test đã gửi nhưng endpoint trả lỗi. Xem delivery để kiểm tra.",
        tone: delivery.status === "success" ? "success" : "error"
      });
    }
  });

  const activeJobs = jobs.filter((job) => job.status === "active").length;
  const successfulRuns = (runsQuery.data ?? []).filter((run) => run.status === "success").length;
  const channelOptions = useMemo(() => channels.filter((channel) => channel.isMember), [channels]);

  return (
    <div className="workspace-page automation-page">
      <header className="workspace-page__header automation-page__header">
        <div>
          <h1>Automation</h1>
        </div>
        <SegmentedControl aria-label="Chọn phần automation" onValueChange={setActiveTab} options={automationTabs.filter((tab) => tab.value === "jobs" ? canManageCronjobs : canManageWebhooks)} value={activeTab} />
      </header>

      <section className="automation-hero">
        <div>
          <Badge tone="blue"><Workflow size={13} /> Luồng trực quan</Badge>
          <h2>Mọi tác vụ vận hành trong một trung tâm</h2>
          <div className="automation-hero__stats">
            <span><strong>{activeJobs}</strong> quy trình hoạt động</span>
            <span><strong>{incomingWebhooks.length + outgoingWebhooks.length}</strong> webhook</span>
            <span><strong>{successfulRuns}</strong> lượt chạy thành công</span>
          </div>
        </div>
        <AutomationFlowVisual />
      </section>

      {feedback ? <AutomationFeedbackView feedback={feedback} onClose={() => setFeedback(null)} /> : null}
      {createdSecret ? <SecretNotice notice={createdSecret} onClose={() => setCreatedSecret(null)} /> : null}

      {activeTab === "jobs" ? (
        canManageCronjobs ? <JobsView
          createMutation={createJobMutation}
          deleteMutation={deleteJobMutation}
          isCreateOpen={isCreateJobOpen}
          jobs={jobs}
          jobsQuery={jobsQuery}
          onCreateOpen={() => setIsCreateJobOpen((current) => !current)}
          onSelect={setSelectedJobId}
          runMutation={runJobMutation}
          runs={runsQuery.data ?? []}
          selectedJob={selectedJob}
          updateMutation={updateJobMutation}
        /> : <AutomationPermission permission="cronjob.manage" />
      ) : canManageWebhooks ? (
        <WebhooksView
          channelOptions={channelOptions}
          createIncomingMutation={createIncomingMutation}
          createOutgoingMutation={createOutgoingMutation}
          deleteIncomingMutation={deleteIncomingMutation}
          deleteOutgoingMutation={deleteOutgoingMutation}
          deliveries={deliveriesQuery.data ?? []}
          incoming={incomingWebhooks}
          incomingQuery={incomingQuery}
          isCreateOpen={isCreateWebhookOpen}
          onCreateOpen={() => setIsCreateWebhookOpen((current) => !current)}
          onSelectIncoming={setSelectedIncomingId}
          onSelectOutgoing={setSelectedOutgoingId}
          outgoing={outgoingWebhooks}
          outgoingQuery={outgoingQuery}
          selectedIncoming={selectedIncoming}
          selectedOutgoing={selectedOutgoing}
          testOutgoingMutation={testOutgoingMutation}
          updateIncomingMutation={updateIncomingMutation}
          updateOutgoingMutation={updateOutgoingMutation}
        />
      ) : <AutomationPermission permission="webhook.manage" />}
    </div>
  );
}

function JobsView({ createMutation, deleteMutation, isCreateOpen, jobs, jobsQuery, onCreateOpen, onSelect, runMutation, runs, selectedJob, updateMutation }: {
  createMutation: ReturnType<typeof useMutation<unknown, Error, { description?: string; name: string; payload: JsonObject; runner: string; schedule: string; status: string }>>;
  deleteMutation: ReturnType<typeof useMutation<unknown, Error, string>>;
  isCreateOpen: boolean;
  jobs: CronJob[];
  jobsQuery: { isError: boolean; isLoading: boolean; refetch: () => Promise<unknown> };
  onCreateOpen: () => void;
  onSelect: (id: string) => void;
  runMutation: ReturnType<typeof useMutation<unknown, Error, string>>;
  runs: Array<{ id: string; status: string; started_at: string; duration_ms?: number | null; error?: string | null }>;
  selectedJob?: CronJob;
  updateMutation: ReturnType<typeof useMutation<unknown, Error, { job: CronJob; status?: string; values?: { description?: string; name: string; payload: JsonObject; runner: string; schedule: string; status: string } }>>;
}) {
  return (
    <>
      <div className="automation-toolbar"><div><h2>Quy trình đã lập lịch</h2><p>Quản lý lịch chạy và kiểm tra lịch sử thực thi.</p></div><Button onClick={onCreateOpen} size="sm">{isCreateOpen ? <X size={15} /> : <Plus size={15} />}{isCreateOpen ? "Đóng" : "Tạo quy trình"}</Button></div>
      {isCreateOpen ? <JobCreateForm isPending={createMutation.isPending} onSubmit={(input) => createMutation.mutate(input)} /> : null}
      <div className="automation-grid">
        <section className="automation-list-panel">
          {jobsQuery.isLoading ? <><Skeleton style={{ height: 112 }} /><Skeleton style={{ height: 112 }} /></> : jobsQuery.isError ? <ErrorState action={<Button onClick={() => void jobsQuery.refetch()} size="sm" variant="secondary">Thử lại</Button>} description="Không tải được quy trình." title="Lỗi automation" /> : jobs.length ? jobs.map((job) => <button className={job.id === selectedJob?.id ? "automation-job-card automation-job-card--active" : "automation-job-card"} key={job.id} onClick={() => onSelect(job.id)} type="button"><span className="automation-job-card__icon"><Clock3 size={19} /></span><span><strong>{job.name}</strong><small>{job.schedule} · {runnerLabel(job.runner)}</small><p>{job.description || "Chưa có mô tả."}</p></span><AutomationStatus status={job.status} /></button>) : <EmptyState description="Tạo quy trình đầu tiên để bắt đầu tự động hóa." title="Chưa có quy trình" />}
        </section>
        <aside className="automation-detail-panel">
          {selectedJob ? <><header><span><Workflow size={21} /></span><div><h2>{selectedJob.name}</h2><p>Lần tới: {formatAutomationDate(selectedJob.next_run_at)}</p></div></header><div className="automation-detail-actions"><Button disabled={runMutation.isPending} onClick={() => runMutation.mutate(selectedJob.id)} size="sm"><Zap size={15} />{runMutation.isPending ? "Đang chạy..." : "Chạy ngay"}</Button><Button disabled={updateMutation.isPending} onClick={() => updateMutation.mutate({ job: selectedJob, status: selectedJob.status === "active" ? "paused" : "active" })} size="sm" variant="secondary">{selectedJob.status === "active" ? "Tạm dừng" : "Kích hoạt"}</Button><Button aria-label="Xóa quy trình" disabled={deleteMutation.isPending} onClick={() => window.confirm(`Xóa quy trình ${selectedJob.name}?`) && deleteMutation.mutate(selectedJob.id)} size="sm" variant="icon"><Trash2 size={16} /></Button></div><section><h3>Lịch sử gần đây</h3>{runs.length ? <div className="automation-run-list">{runs.map((run) => <article key={run.id}><AutomationStatus status={run.status} /><span><strong>{formatAutomationDate(run.started_at)}</strong><small>{run.duration_ms ? `${run.duration_ms} ms` : run.error || "Đang xử lý"}</small></span></article>)}</div> : <p className="automation-muted">Chưa có lượt chạy nào.</p>}</section></> : <EmptyState description="Chọn một quy trình để xem chi tiết." title="Chưa chọn quy trình" />}
        </aside>
      </div>
      {selectedJob ? <JobEditPanel isPending={updateMutation.isPending} job={selectedJob} onSubmit={(values) => updateMutation.mutate({ job: selectedJob, values })} /> : null}
    </>
  );
}

function JobEditPanel({
  isPending,
  job,
  onSubmit
}: {
  isPending: boolean;
  job: CronJob;
  onSubmit: (values: { description?: string; name: string; payload: JsonObject; runner: string; schedule: string; status: string }) => void;
}) {
  const [name, setName] = useState(job.name);
  const [description, setDescription] = useState(job.description ?? "");
  const [schedule, setSchedule] = useState(job.schedule);
  const [runner, setRunner] = useState(job.runner);
  const [status, setStatus] = useState(job.status);
  const [payload, setPayload] = useState(JSON.stringify(job.payload, null, 2));
  const [error, setError] = useState("");

  useEffect(() => {
    setName(job.name);
    setDescription(job.description ?? "");
    setSchedule(job.schedule);
    setRunner(job.runner);
    setStatus(job.status);
    setPayload(JSON.stringify(job.payload, null, 2));
    setError("");
  }, [job]);

  return (
    <form
      className="automation-create-form automation-edit-form"
      onSubmit={(event) => {
        event.preventDefault();
        try {
          const parsed = JSON.parse(payload) as JsonObject;
          if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") throw new Error();
          setError("");
          onSubmit({
            description: description.trim() || undefined,
            name: name.trim(),
            payload: parsed,
            runner,
            schedule: schedule.trim(),
            status
          });
        } catch {
          setError("Payload phải là JSON object hợp lệ.");
        }
      }}
    >
      <label>Tên quy trình<input onChange={(event) => setName(event.target.value)} required value={name} /></label>
      <label>Lịch cron<input onChange={(event) => setSchedule(event.target.value)} required value={schedule} /></label>
      <label>
        Runner
        <select onChange={(event) => setRunner(event.target.value)} value={runner}>
          <option value="builtin_cleanup">Dọn dữ liệu</option>
          <option value="http">HTTP request</option>
          <option value="worker">Worker cleanup</option>
          <option value="script">Script allowlist</option>
        </select>
      </label>
      <label>
        Trạng thái
        <select onChange={(event) => setStatus(event.target.value)} value={status}>
          <option value="active">Hoạt động</option>
          <option value="paused">Tạm dừng</option>
          <option value="disabled">Đã tắt</option>
        </select>
      </label>
      <label className="automation-create-form__payload">Mô tả<input onChange={(event) => setDescription(event.target.value)} value={description} /></label>
      <label className="automation-create-form__payload">Payload JSON<textarea onChange={(event) => setPayload(event.target.value)} value={payload} /></label>
      {error ? <p>{error}</p> : null}
      <Button disabled={isPending || !name.trim() || !schedule.trim()} size="sm" type="submit">{isPending ? "Đang lưu..." : "Lưu thay đổi quy trình"}</Button>
    </form>
  );
}

function JobCreateForm({ isPending, onSubmit }: { isPending: boolean; onSubmit: (input: { description?: string; name: string; payload: JsonObject; runner: string; schedule: string; status: string }) => void }) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [schedule, setSchedule] = useState("*/15 * * * *");
  const [runner, setRunner] = useState("builtin_cleanup");
  const [payload, setPayload] = useState('{"task":"cleanup_expired_sessions","older_than":"168h"}');
  const [error, setError] = useState("");
  function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); try { const parsed = JSON.parse(payload) as JsonObject; if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") throw new Error(); setError(""); onSubmit({ description: description.trim() || undefined, name: name.trim(), payload: parsed, runner, schedule: schedule.trim(), status: "active" }); } catch { setError("Payload phải là một JSON object hợp lệ."); } }
  return <form className="automation-create-form" onSubmit={submit}><label>Tên quy trình<input autoFocus onChange={(event) => setName(event.target.value)} placeholder="Dọn phiên đăng nhập cũ" required value={name} /></label><label>Lịch cron<input onChange={(event) => setSchedule(event.target.value)} required value={schedule} /></label><label>Runner<select onChange={(event) => { const value = event.target.value; setRunner(value); setPayload(value === "http" ? '{"method":"POST","url":"https://example.com/hook","body":{}}' : '{"task":"cleanup_expired_sessions","older_than":"168h"}'); }} value={runner}><option value="builtin_cleanup">Dọn dữ liệu</option><option value="http">HTTP request</option><option value="worker">Worker cleanup</option><option value="script">Script allowlist</option></select></label><label>Mô tả<input onChange={(event) => setDescription(event.target.value)} placeholder="Mục đích của quy trình" value={description} /></label><label className="automation-create-form__payload">Payload JSON<textarea onChange={(event) => setPayload(event.target.value)} value={payload} /></label>{error ? <p>{error}</p> : null}<Button disabled={isPending || !name.trim()} size="sm" type="submit">{isPending ? "Đang tạo..." : "Lưu quy trình"}</Button></form>;
}

export function LegacyWebhooksView({ channelOptions, createIncomingMutation, createOutgoingMutation, deliveries, incoming, incomingQuery, isCreateOpen, onCreateOpen, onSelectOutgoing, outgoing, outgoingQuery, selectedOutgoing }: {
  channelOptions: ChatChannel[];
  createIncomingMutation: ReturnType<typeof useMutation<unknown, Error, { channel_id?: string; name: string }>>;
  createOutgoingMutation: ReturnType<typeof useMutation<unknown, Error, { event_types?: string[]; name: string; target_url: string }>>;
  deliveries: Array<{ id: string; status: string; event_type: string; response_status?: number | null; created_at: string }>;
  incoming: IncomingWebhook[];
  incomingQuery: { isError: boolean; isLoading: boolean; refetch: () => Promise<unknown> };
  isCreateOpen: boolean;
  onCreateOpen: () => void;
  onSelectOutgoing: (id: string) => void;
  outgoing: OutgoingWebhook[];
  outgoingQuery: { isError: boolean; isLoading: boolean; refetch: () => Promise<unknown> };
  selectedOutgoing?: OutgoingWebhook;
}) {
  return <><div className="automation-toolbar"><div><h2>Kết nối webhook</h2><p>Nhận sự kiện vào kênh hoặc đẩy sự kiện ra hệ thống khác.</p></div><Button onClick={onCreateOpen} size="sm">{isCreateOpen ? <X size={15} /> : <Plus size={15} />}{isCreateOpen ? "Đóng" : "Tạo webhook"}</Button></div>{isCreateOpen ? <WebhookCreateForm channels={channelOptions} incomingPending={createIncomingMutation.isPending} onIncoming={(input) => createIncomingMutation.mutate(input)} onOutgoing={(input) => createOutgoingMutation.mutate(input)} outgoingPending={createOutgoingMutation.isPending} /> : null}<div className="webhook-columns"><section className="webhook-panel"><header><span><Cloud size={19} /></span><div><h2>Incoming</h2><p>Đưa cảnh báo và dữ liệu vào kênh.</p></div><Badge tone="blue">{incoming.length}</Badge></header>{incomingQuery.isLoading ? <Skeleton style={{ height: 100 }} /> : incomingQuery.isError ? <ErrorState action={<Button onClick={() => void incomingQuery.refetch()} size="sm" variant="secondary">Thử lại</Button>} description="Không tải được incoming webhook." title="Lỗi webhook" /> : incoming.length ? <div className="webhook-list">{incoming.map((webhook) => <article key={webhook.id}><span><strong>{webhook.name}</strong><small>{channelName(channelOptions, webhook.channel_id)}</small></span><AutomationStatus status={webhook.status} /></article>)}</div> : <EmptyState description="Tạo endpoint để nhận dữ liệu bên ngoài." title="Chưa có incoming webhook" />}</section><section className="webhook-panel"><header><span><Share2 size={19} /></span><div><h2>Outgoing</h2><p>Đẩy sự kiện chat tới dịch vụ khác.</p></div><Badge tone="blue">{outgoing.length}</Badge></header>{outgoingQuery.isLoading ? <Skeleton style={{ height: 100 }} /> : outgoingQuery.isError ? <ErrorState action={<Button onClick={() => void outgoingQuery.refetch()} size="sm" variant="secondary">Thử lại</Button>} description="Không tải được outgoing webhook." title="Lỗi webhook" /> : outgoing.length ? <div className="webhook-list">{outgoing.map((webhook) => <button className={webhook.id === selectedOutgoing?.id ? "webhook-row webhook-row--active" : "webhook-row"} key={webhook.id} onClick={() => onSelectOutgoing(webhook.id)} type="button"><span><strong>{webhook.name}</strong><small>{webhook.target_url}</small></span><AutomationStatus status={webhook.status} /></button>)}</div> : <EmptyState description="Tạo webhook để phát sự kiện ra ngoài." title="Chưa có outgoing webhook" />}</section></div>{selectedOutgoing ? <section className="delivery-panel"><header><div><h2>Delivery gần đây · {selectedOutgoing.name}</h2><p>Theo dõi phản hồi từ endpoint đích.</p></div><Badge tone="slate">{deliveries.length} lần</Badge></header>{deliveries.length ? <div className="delivery-list">{deliveries.map((delivery) => <article key={delivery.id}><AutomationStatus status={delivery.status} /><span><strong>{delivery.event_type}</strong><small>{formatAutomationDate(delivery.created_at)}</small></span><em>{delivery.response_status || "—"}</em></article>)}</div> : <p className="automation-muted">Chưa có delivery nào.</p>}</section> : null}</>;
}

function WebhooksView({
  channelOptions,
  createIncomingMutation,
  createOutgoingMutation,
  deleteIncomingMutation,
  deleteOutgoingMutation,
  deliveries,
  incoming,
  incomingQuery,
  isCreateOpen,
  onCreateOpen,
  onSelectIncoming,
  onSelectOutgoing,
  outgoing,
  outgoingQuery,
  selectedIncoming,
  selectedOutgoing,
  testOutgoingMutation,
  updateIncomingMutation,
  updateOutgoingMutation
}: {
  channelOptions: ChatChannel[];
  createIncomingMutation: ReturnType<typeof useMutation<unknown, Error, { channel_id?: string; name: string }>>;
  createOutgoingMutation: ReturnType<typeof useMutation<unknown, Error, { event_types?: string[]; name: string; target_url: string }>>;
  deleteIncomingMutation: ReturnType<typeof useMutation<unknown, Error, string>>;
  deleteOutgoingMutation: ReturnType<typeof useMutation<unknown, Error, string>>;
  deliveries: WebhookDelivery[];
  incoming: IncomingWebhook[];
  incomingQuery: { isError: boolean; isLoading: boolean; refetch: () => Promise<unknown> };
  isCreateOpen: boolean;
  onCreateOpen: () => void;
  onSelectIncoming: (id: string) => void;
  onSelectOutgoing: (id: string) => void;
  outgoing: OutgoingWebhook[];
  outgoingQuery: { isError: boolean; isLoading: boolean; refetch: () => Promise<unknown> };
  selectedIncoming?: IncomingWebhook;
  selectedOutgoing?: OutgoingWebhook;
  testOutgoingMutation: ReturnType<typeof useMutation<unknown, Error, { event_type?: string; payload: JsonObject; webhookId: string }>>;
  updateIncomingMutation: ReturnType<typeof useMutation<unknown, Error, { channel_id?: string | null; name?: string; status?: string; webhookId: string }>>;
  updateOutgoingMutation: ReturnType<typeof useMutation<unknown, Error, { event_types?: string[]; name?: string; status?: string; target_url?: string; webhookId: string }>>;
}) {
  return (
    <>
      <div className="automation-toolbar">
        <div>
          <h2>Kết nối webhook</h2>
          <p>Nhận sự kiện vào kênh, đẩy sự kiện ra hệ thống khác và kiểm tra delivery ngay trên web.</p>
        </div>
        <Button onClick={onCreateOpen} size="sm">
          {isCreateOpen ? <X size={15} /> : <Plus size={15} />}
          {isCreateOpen ? "Đóng" : "Tạo webhook"}
        </Button>
      </div>

      {isCreateOpen ? (
        <WebhookCreateForm
          channels={channelOptions}
          incomingPending={createIncomingMutation.isPending}
          onIncoming={(input) => createIncomingMutation.mutate(input)}
          onOutgoing={(input) => createOutgoingMutation.mutate(input)}
          outgoingPending={createOutgoingMutation.isPending}
        />
      ) : null}

      <div className="webhook-columns">
        <section className="webhook-panel">
          <header>
            <span><Cloud size={19} /></span>
            <div>
              <h2>Incoming</h2>
              <p>Endpoint nhận cảnh báo, ticket, billing và đẩy vào kênh nội bộ.</p>
            </div>
            <Badge tone="blue">{incoming.length}</Badge>
          </header>
          {incomingQuery.isLoading ? (
            <Skeleton style={{ height: 100 }} />
          ) : incomingQuery.isError ? (
            <ErrorState
              action={<Button onClick={() => void incomingQuery.refetch()} size="sm" variant="secondary">Thử lại</Button>}
              description="Không tải được incoming webhook."
              title="Lỗi webhook"
            />
          ) : incoming.length ? (
            <div className="webhook-list">
              {incoming.map((webhook) => (
                <button
                  className={webhook.id === selectedIncoming?.id ? "webhook-row webhook-row--active" : "webhook-row"}
                  key={webhook.id}
                  onClick={() => onSelectIncoming(webhook.id)}
                  type="button"
                >
                  <span>
                    <strong>{webhook.name}</strong>
                    <small>{channelName(channelOptions, webhook.channel_id)} · dùng gần nhất {formatAutomationDate(webhook.last_used_at)}</small>
                  </span>
                  <AutomationStatus status={webhook.status} />
                </button>
              ))}
            </div>
          ) : (
            <EmptyState description="Tạo endpoint để nhận dữ liệu bên ngoài." title="Chưa có incoming webhook" />
          )}
        </section>

        <section className="webhook-panel">
          <header>
            <span><Share2 size={19} /></span>
            <div>
              <h2>Outgoing</h2>
              <p>Gửi sự kiện chat sang CRM, billing, monitoring hoặc hệ thống automation khác.</p>
            </div>
            <Badge tone="blue">{outgoing.length}</Badge>
          </header>
          {outgoingQuery.isLoading ? (
            <Skeleton style={{ height: 100 }} />
          ) : outgoingQuery.isError ? (
            <ErrorState
              action={<Button onClick={() => void outgoingQuery.refetch()} size="sm" variant="secondary">Thử lại</Button>}
              description="Không tải được outgoing webhook."
              title="Lỗi webhook"
            />
          ) : outgoing.length ? (
            <div className="webhook-list">
              {outgoing.map((webhook) => (
                <button
                  className={webhook.id === selectedOutgoing?.id ? "webhook-row webhook-row--active" : "webhook-row"}
                  key={webhook.id}
                  onClick={() => onSelectOutgoing(webhook.id)}
                  type="button"
                >
                  <span>
                    <strong>{webhook.name}</strong>
                    <small>{webhook.target_url}</small>
                  </span>
                  <AutomationStatus status={webhook.status} />
                </button>
              ))}
            </div>
          ) : (
            <EmptyState description="Tạo webhook để phát sự kiện ra ngoài." title="Chưa có outgoing webhook" />
          )}
        </section>
      </div>

      <div className="webhook-detail-grid">
        {selectedIncoming ? (
          <IncomingWebhookEditor
            channels={channelOptions}
            deleteMutation={deleteIncomingMutation}
            updateMutation={updateIncomingMutation}
            webhook={selectedIncoming}
          />
        ) : (
          <section className="webhook-detail-card">
            <EmptyState description="Chọn incoming webhook để sửa kênh mặc định, bật/tắt hoặc xóa." title="Chưa chọn incoming" />
          </section>
        )}

        {selectedOutgoing ? (
          <OutgoingWebhookEditor
            deleteMutation={deleteOutgoingMutation}
            deliveries={deliveries}
            testMutation={testOutgoingMutation}
            updateMutation={updateOutgoingMutation}
            webhook={selectedOutgoing}
          />
        ) : (
          <section className="webhook-detail-card">
            <EmptyState description="Chọn outgoing webhook để sửa endpoint, bật/tắt, gửi thử và xem delivery." title="Chưa chọn outgoing" />
          </section>
        )}
      </div>
    </>
  );
}

function IncomingWebhookEditor({
  channels,
  deleteMutation,
  updateMutation,
  webhook
}: {
  channels: ChatChannel[];
  deleteMutation: ReturnType<typeof useMutation<unknown, Error, string>>;
  updateMutation: ReturnType<typeof useMutation<unknown, Error, { channel_id?: string | null; name?: string; status?: string; webhookId: string }>>;
  webhook: IncomingWebhook;
}) {
  const [name, setName] = useState(webhook.name);
  const [channelId, setChannelId] = useState(webhook.channel_id ?? "");
  const [status, setStatus] = useState(webhook.status);
  const isPending = updateMutation.isPending || deleteMutation.isPending;

  useEffect(() => {
    setName(webhook.name);
    setChannelId(webhook.channel_id ?? "");
    setStatus(webhook.status);
  }, [webhook]);

  return (
    <section className="webhook-detail-card">
      <header>
        <span><Cloud size={18} /></span>
        <div>
          <h2>Quản lý incoming</h2>
          <p>Đổi kênh mặc định, bật/tắt endpoint hoặc xóa webhook không dùng nữa.</p>
        </div>
        <AutomationStatus status={webhook.status} />
      </header>
      <form
        className="webhook-editor-form"
        onSubmit={(event) => {
          event.preventDefault();
          updateMutation.mutate({ channel_id: channelId, name: name.trim(), status, webhookId: webhook.id });
        }}
      >
        <label>Tên webhook<input onChange={(event) => setName(event.target.value)} required value={name} /></label>
        <label>
          Kênh mặc định
          <select onChange={(event) => setChannelId(event.target.value)} value={channelId}>
            <option value="">Không gán mặc định</option>
            {channels.map((channel) => <option key={channel.id} value={channel.id}>#{channel.name}</option>)}
          </select>
        </label>
        <label>
          Trạng thái
          <select onChange={(event) => setStatus(event.target.value)} value={status}>
            <option value="active">Hoạt động</option>
            <option value="disabled">Đã tắt</option>
          </select>
        </label>
        <div className="webhook-detail-actions">
          <Button disabled={isPending || !name.trim()} size="sm" type="submit">Lưu incoming</Button>
          <Button
            disabled={isPending}
            onClick={() => updateMutation.mutate({ status: webhook.status === "active" ? "disabled" : "active", webhookId: webhook.id })}
            size="sm"
            type="button"
            variant="secondary"
          >
            {webhook.status === "active" ? "Tắt" : "Bật lại"}
          </Button>
          <Button
            disabled={isPending}
            onClick={() => window.confirm(`Xóa incoming webhook ${webhook.name}?`) && deleteMutation.mutate(webhook.id)}
            size="sm"
            type="button"
            variant="ghost"
          >
            Xóa
          </Button>
        </div>
      </form>
    </section>
  );
}

function OutgoingWebhookEditor({
  deleteMutation,
  deliveries,
  testMutation,
  updateMutation,
  webhook
}: {
  deleteMutation: ReturnType<typeof useMutation<unknown, Error, string>>;
  deliveries: WebhookDelivery[];
  testMutation: ReturnType<typeof useMutation<unknown, Error, { event_type?: string; payload: JsonObject; webhookId: string }>>;
  updateMutation: ReturnType<typeof useMutation<unknown, Error, { event_types?: string[]; name?: string; status?: string; target_url?: string; webhookId: string }>>;
  webhook: OutgoingWebhook;
}) {
  const [name, setName] = useState(webhook.name);
  const [targetUrl, setTargetUrl] = useState(webhook.target_url);
  const [events, setEvents] = useState(webhook.event_types.join(", "));
  const [status, setStatus] = useState(webhook.status);
  const [testEventType, setTestEventType] = useState("webhook.test");
  const [testPayload, setTestPayload] = useState('{"hello":"vpsttt","source":"automation-test"}');
  const [testError, setTestError] = useState("");
  const isPending = updateMutation.isPending || deleteMutation.isPending || testMutation.isPending;

  useEffect(() => {
    setName(webhook.name);
    setTargetUrl(webhook.target_url);
    setEvents(webhook.event_types.join(", "));
    setStatus(webhook.status);
    setTestError("");
  }, [webhook]);

  function submitTest() {
    try {
      const parsed = JSON.parse(testPayload) as JsonObject;
      if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") throw new Error();
      setTestError("");
      testMutation.mutate({ event_type: testEventType.trim() || undefined, payload: parsed, webhookId: webhook.id });
    } catch {
      setTestError("Payload test phải là JSON object hợp lệ.");
    }
  }

  return (
    <section className="webhook-detail-card">
      <header>
        <span><Share2 size={18} /></span>
        <div>
          <h2>Quản lý outgoing</h2>
          <p>Sửa endpoint, chọn event, bật/tắt và gửi thử đến hệ thống ngoài.</p>
        </div>
        <AutomationStatus status={webhook.status} />
      </header>
      <form
        className="webhook-editor-form"
        onSubmit={(event) => {
          event.preventDefault();
          updateMutation.mutate({
            event_types: events.split(",").map((item) => item.trim()).filter(Boolean),
            name: name.trim(),
            status,
            target_url: targetUrl.trim(),
            webhookId: webhook.id
          });
        }}
      >
        <label>Tên webhook<input onChange={(event) => setName(event.target.value)} required value={name} /></label>
        <label>Target URL<input onChange={(event) => setTargetUrl(event.target.value)} required type="url" value={targetUrl} /></label>
        <label>Sự kiện<input onChange={(event) => setEvents(event.target.value)} placeholder="message.created, ticket.created" value={events} /></label>
        <label>
          Trạng thái
          <select onChange={(event) => setStatus(event.target.value)} value={status}>
            <option value="active">Hoạt động</option>
            <option value="disabled">Đã tắt</option>
          </select>
        </label>
        <div className="webhook-detail-actions">
          <Button disabled={isPending || !name.trim() || !targetUrl.trim()} size="sm" type="submit">Lưu outgoing</Button>
          <Button
            disabled={isPending}
            onClick={() => updateMutation.mutate({ status: webhook.status === "active" ? "disabled" : "active", webhookId: webhook.id })}
            size="sm"
            type="button"
            variant="secondary"
          >
            {webhook.status === "active" ? "Tắt" : "Bật lại"}
          </Button>
          <Button
            disabled={isPending}
            onClick={() => window.confirm(`Xóa outgoing webhook ${webhook.name}?`) && deleteMutation.mutate(webhook.id)}
            size="sm"
            type="button"
            variant="ghost"
          >
            Xóa
          </Button>
        </div>
      </form>

      <div className="webhook-test-form">
        <h3>Gửi thử</h3>
        <label>Event type<input onChange={(event) => setTestEventType(event.target.value)} value={testEventType} /></label>
        <label>Payload JSON<textarea onChange={(event) => setTestPayload(event.target.value)} value={testPayload} /></label>
        {testError ? <p>{testError}</p> : null}
        <Button disabled={isPending} onClick={submitTest} size="sm" type="button"><Zap size={15} />{testMutation.isPending ? "Đang gửi..." : "Gửi thử webhook"}</Button>
      </div>

      <section className="delivery-panel delivery-panel--embedded">
        <header>
          <div>
            <h2>Delivery gần đây · {webhook.name}</h2>
            <p>Theo dõi phản hồi từ endpoint đích.</p>
          </div>
          <Badge tone="slate">{deliveries.length} lần</Badge>
        </header>
        {deliveries.length ? (
          <div className="delivery-list">
            {deliveries.map((delivery) => (
              <article key={delivery.id}>
                <AutomationStatus status={delivery.status} />
                <span>
                  <strong>{delivery.event_type}</strong>
                  <small>{formatAutomationDate(delivery.created_at)} · thử {delivery.attempt_count} lần</small>
                  {delivery.response_body ? <small>{delivery.response_body.slice(0, 120)}</small> : null}
                </span>
                <em>{delivery.response_status || "—"}</em>
              </article>
            ))}
          </div>
        ) : <p className="automation-muted">Chưa có delivery nào.</p>}
      </section>
    </section>
  );
}

function WebhookCreateForm({ channels, incomingPending, onIncoming, onOutgoing, outgoingPending }: { channels: ChatChannel[]; incomingPending: boolean; onIncoming: (input: { channel_id?: string; name: string }) => void; onOutgoing: (input: { event_types?: string[]; name: string; target_url: string }) => void; outgoingPending: boolean }) {
  const [kind, setKind] = useState<"incoming" | "outgoing">("incoming"); const [name, setName] = useState(""); const [channelId, setChannelId] = useState(channels[0]?.id ?? ""); const [targetUrl, setTargetUrl] = useState(""); const [events, setEvents] = useState("message.created,message.updated");
  return <form className="webhook-create-form" onSubmit={(event) => { event.preventDefault(); if (kind === "incoming") onIncoming({ channel_id: channelId || undefined, name: name.trim() }); else onOutgoing({ event_types: events.split(",").map((item) => item.trim()).filter(Boolean), name: name.trim(), target_url: targetUrl.trim() }); }}><label>Loại<select onChange={(event) => setKind(event.target.value as "incoming" | "outgoing")} value={kind}><option value="incoming">Incoming</option><option value="outgoing">Outgoing</option></select></label><label>Tên webhook<input autoFocus onChange={(event) => setName(event.target.value)} placeholder="Billing webhook" required value={name} /></label>{kind === "incoming" ? <label>Kênh mặc định<select onChange={(event) => setChannelId(event.target.value)} value={channelId}>{channels.map((channel) => <option key={channel.id} value={channel.id}>#{channel.name}</option>)}</select></label> : <><label>Target URL<input onChange={(event) => setTargetUrl(event.target.value)} placeholder="https://example.com/webhook" required type="url" value={targetUrl} /></label><label>Sự kiện<input onChange={(event) => setEvents(event.target.value)} value={events} /></label></>}<Button disabled={!name.trim() || incomingPending || outgoingPending} size="sm" type="submit">{incomingPending || outgoingPending ? "Đang tạo..." : "Tạo webhook"}</Button></form>;
}

function AutomationFlowVisual() { return <div className="automation-flow" aria-hidden="true"><span className="automation-flow__line automation-flow__line--one" /><span className="automation-flow__line automation-flow__line--two" /><span className="automation-flow__node automation-flow__node--start"><Clock3 size={20} /></span><span className="automation-flow__node automation-flow__node--middle"><Workflow size={24} /></span><span className="automation-flow__node automation-flow__node--end"><Send size={20} /></span><i className="automation-flow__packet automation-flow__packet--one" /><i className="automation-flow__packet automation-flow__packet--two" /></div>; }
function AutomationStatus({ status }: { status: string }) { const tone = status === "active" || status === "success" || status === "delivered" ? "green" : status === "failed" ? "red" : status === "running" ? "blue" : "slate"; return <Badge className="automation-status" tone={tone}>{statusLabel(status)}</Badge>; }
function AutomationPermission({ permission }: { permission: string }) { return <section className="automation-permission"><Info size={28} /><div><h2>Cần thêm quyền truy cập</h2><p>Liên hệ quản trị viên để được cấp quyền <code>{permission}</code>.</p></div></section>; }
function AutomationFeedbackView({ feedback, onClose }: { feedback: AutomationFeedback; onClose: () => void }) { return <div className={`automation-feedback automation-feedback--${feedback.tone}`}><span>{feedback.tone === "success" ? <CheckCircle2 size={16} /> : <Info size={16} />}{feedback.message}</span><button aria-label="Đóng" onClick={onClose} type="button"><X size={15} /></button></div>; }
function SecretNotice({ notice, onClose }: { notice: { label: string; secret: string; url?: string }; onClose: () => void }) { return <section className="automation-secret"><header><span><FileText size={18} /></span><div><strong>Lưu thông tin {notice.label}</strong><small>Secret chỉ hiển thị một lần. Hãy lưu ở nơi an toàn.</small></div><button aria-label="Đóng" onClick={onClose} type="button"><X size={15} /></button></header>{notice.url ? <label>URL<code>{notice.url}</code></label> : null}<label>Secret<code>{notice.secret}</code></label></section>; }
function runnerLabel(runner: string) { return runner === "builtin_cleanup" ? "Dọn dữ liệu" : runner === "http" ? "HTTP" : runner === "worker" ? "Worker" : "Script"; }
function statusLabel(status: string) { const labels: Record<string, string> = { active: "Hoạt động", cancelled: "Đã hủy", delivered: "Đã gửi", disabled: "Đã tắt", failed: "Thất bại", paused: "Tạm dừng", pending: "Đang chờ", running: "Đang chạy", success: "Thành công" }; return labels[status] ?? status; }
function channelName(channels: ChatChannel[], id?: string | null) { return channels.find((channel) => channel.id === id)?.name ? `#${channels.find((channel) => channel.id === id)?.name}` : "Mọi kênh"; }
function formatAutomationDate(value?: string | null) { if (!value) return "Chưa có"; const date = new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleString("vi-VN", { dateStyle: "short", timeStyle: "short" }); }
function automationError(error: unknown, fallback: string) { return error instanceof Error ? error.message : fallback; }
