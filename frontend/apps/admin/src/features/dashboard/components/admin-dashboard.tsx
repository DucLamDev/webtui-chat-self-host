"use client";

import { type FormEvent, type ReactNode, useEffect, useMemo, useState } from "react";
import {
  Avatar,
  Badge,
  type BadgeProps,
  Button,
  EmptyState,
  ErrorState,
  Input,
  MetricCard,
  NavigationRail,
  SegmentedControl,
  Skeleton,
  Toast,
  Tooltip,
  useTheme
} from "@webtui/ui";
import {
  Activity,
  Bell,
  Bot,
  CalendarClock,
  Database,
  Hash,
  KeyRound,
  LogOut,
  MessageCircle,
  Plus,
  Moon,
  Search,
  Send,
  Settings,
  ShieldCheck,
  Sun,
  Trash2,
  Users,
  Workflow,
  Zap
} from "@webtui/icons";
import type {
  AdminHealth,
  AdminStats,
  ApiScope,
  ApiToken,
  AutomationInstallation,
  AuthUser,
  BackupJob,
  BackupRun,
  Bot as BotRecord,
  BotInstallation,
  Channel,
  CreateBackupJobInput,
  CronJob,
  CronJobRun,
  IncomingWebhook,
  JsonObject,
  OutgoingWebhook,
  SaveCronJobInput,
  WebhookDelivery,
  WorkspaceMember,
  WorkspaceSetting,
  ZoneOIDCProvider
} from "@webtui/types";
import { useAuth } from "@/features/auth/auth-provider";
import { useApiStatus } from "../../platform/hooks/use-api-status";
import { useAdminDashboardData } from "../hooks/use-admin-dashboard-data";
import type { AdminUser, AdminUserFilter, ChannelRank, DashboardMetric } from "../model/types";

type DashboardData = ReturnType<typeof useAdminDashboardData>;
type ToastTone = "danger" | "info" | "success";

const navItems = [
  { id: "overview", label: "Tổng quan", icon: Activity },
  { id: "messages", label: "Tin nhắn", icon: MessageCircle },
  { id: "channels", label: "Kênh", icon: Hash },
  { id: "users", label: "Người dùng", icon: Users },
  { id: "roles", label: "Vai trò", icon: ShieldCheck },
  { id: "integrations", label: "Tích hợp", icon: Workflow },
  { id: "automations", label: "Automation", icon: Zap },
  { id: "bots", label: "Bot", icon: Bot },
  { id: "cronjobs", label: "Cronjob", icon: CalendarClock },
  { id: "backups", label: "Backup", icon: Database },
  { id: "settings", label: "Cài đặt", icon: Settings }
] as const;

type AdminNavId = (typeof navItems)[number]["id"];

const pageMeta: Record<AdminNavId, { description: string; title: string }> = {
  overview: {
    description: "Theo dõi sức khỏe, hoạt động và các chỉ số quan trọng của workspace.",
    title: "Tổng quan hệ thống"
  },
  messages: {
    description: "Quản lý và giám sát tất cả tin nhắn trong hệ thống.",
    title: "Quản trị tin nhắn"
  },
  channels: {
    description: "Quản lý kênh nhóm, kênh riêng và các phiên bot riêng tư.",
    title: "Quản trị kênh"
  },
  users: {
    description: "Quản lý tài khoản, thành viên và trạng thái truy cập workspace.",
    title: "Quản trị người dùng"
  },
  roles: {
    description: "Thiết lập vai trò và quyền hạn theo nguyên tắc tối thiểu.",
    title: "Vai trò và phân quyền"
  },
  integrations: {
    description: "Quản lý API token, webhook và các kết nối dịch vụ bên ngoài.",
    title: "Tích hợp hệ thống"
  },
  automations: {
    description: "Cài workflow, connector và bot theo cấu hình riêng của zone hiện tại.",
    title: "Automation theo zone"
  },
  bots: {
    description: "Quản lý bot, cài đặt vào workspace và theo dõi hoạt động.",
    title: "Quản trị bot"
  },
  cronjobs: {
    description: "Lập lịch, theo dõi và vận hành các tác vụ tự động.",
    title: "Tác vụ định kỳ"
  },
  backups: {
    description: "Quản lý lịch sao lưu và lịch sử khôi phục dữ liệu.",
    title: "Sao lưu dữ liệu"
  },
  settings: {
    description: "Cấu hình workspace và kiểm tra trạng thái các dịch vụ nền.",
    title: "Cài đặt hệ thống"
  }
};

const userFilters: Array<{ label: string; value: AdminUserFilter }> = [
  { label: "Tất cả", value: "all" },
  { label: "Đang hoạt động", value: "active" },
  { label: "Bị khóa", value: "blocked" }
];

const cronJobStatuses = [
  { label: "Đang hoạt động", value: "active" },
  { label: "Tạm dừng", value: "paused" },
  { label: "Vô hiệu hóa", value: "disabled" }
] as const;

const cronJobRunners = [
  { label: "Dọn dẹp hệ thống", value: "builtin_cleanup" },
  { label: "HTTP", value: "http" },
  { label: "Worker", value: "worker" },
  { label: "Script", value: "script" }
] as const;

const backupTargets = [
  { label: "Local storage", value: "local" },
  { label: "MinIO", value: "minio" },
  { label: "S3", value: "s3" }
] as const;

const backupStatuses = [
  { label: "Đang hoạt động", value: "active" },
  { label: "Tạm dừng", value: "paused" },
  { label: "Vô hiệu hóa", value: "disabled" }
] as const;

export function AdminDashboard() {
  const { logout, user } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const [activeNavItem, setActiveNavItem] = useState<AdminNavId>("overview");
  const [userFilter, setUserFilter] = useState<AdminUserFilter>("all");
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedBackupJobId, setSelectedBackupJobId] = useState("");
  const [selectedBotId, setSelectedBotId] = useState("");
  const [selectedCronJobId, setSelectedCronJobId] = useState("");
  const [selectedMemberId, setSelectedMemberId] = useState("");
  const [selectedOutgoingWebhookId, setSelectedOutgoingWebhookId] = useState("");
  const [toast, setToastState] = useState<{ message: string; tone: ToastTone } | null>(null);
  const apiStatus = useApiStatus();
  const data = useAdminDashboardData({
    selectedBackupJobId,
    selectedBotId,
    selectedCronJobId,
    selectedMemberId,
    selectedOutgoingWebhookId
  });

  const users = useMemo(() => data.users.map(mapAdminUser), [data.users]);
  const metrics = useMemo(() => mapMetrics(data.statsQuery.data), [data.statsQuery.data]);
  const activityBars = useMemo(() => mapActivityBars(data.statsQuery.data), [data.statsQuery.data]);
  const channelRanks = useMemo(() => mapChannelRanks(data.statsQuery.data), [data.statsQuery.data]);
  const healthChecks = useMemo(() => mapHealthChecks(data.healthQuery.data), [data.healthQuery.data]);
  const activePage = pageMeta[activeNavItem];
  const showSystemPanel = activeNavItem === "overview";
  const profile = useMemo(() => mapProfile(user), [user]);

  const filteredUsers = useMemo(() => {
    const normalizedQuery = searchQuery.trim().toLowerCase();

    return users.filter((item) => {
      const matchesFilter = userFilter === "all" || item.status === userFilter;
      const matchesQuery =
        !normalizedQuery ||
        item.name.toLowerCase().includes(normalizedQuery) ||
        item.email.toLowerCase().includes(normalizedQuery) ||
        item.department.toLowerCase().includes(normalizedQuery);

      return matchesFilter && matchesQuery;
    });
  }, [searchQuery, userFilter, users]);

  useEffect(() => {
    setSelectedMemberId((current) => pickKnownId(current, data.members.map((member) => member.user_id)));
  }, [data.members]);

  useEffect(() => {
    setSelectedBotId((current) => pickKnownId(current, data.bots.map((bot) => bot.id)));
  }, [data.bots]);

  useEffect(() => {
    setSelectedCronJobId((current) => pickKnownId(current, data.cronjobs.map((job) => job.id)));
  }, [data.cronjobs]);

  useEffect(() => {
    setSelectedBackupJobId((current) => pickKnownId(current, data.backupJobs.map((job) => job.id)));
  }, [data.backupJobs]);

  useEffect(() => {
    setSelectedOutgoingWebhookId((current) =>
      pickKnownId(current, data.outgoingWebhooks.map((webhook) => webhook.id))
    );
  }, [data.outgoingWebhooks]);

  const adminDenied = Boolean(data.workspaceId && !data.permissionsQuery.isLoading && !data.canViewAdmin);
  const showToast = (message: string, tone: ToastTone = "success") => setToastState({ message, tone });

  return (
    <main className="admin-shell admin-shell--wide" aria-label="Bảng quản trị WebTui">
      <NavigationRail
        activeId={activeNavItem}
        ariaLabel="Điều hướng quản trị"
        brandTitle="Quản trị hệ thống"
        items={[...navItems]}
        onSelect={(id) => setActiveNavItem(id as AdminNavId)}
        profile={{ ...profile, description: "Quản trị viên", label: profile.name }}
      />

      <section className="admin-main">
        <header className="admin-header">
          <div className="admin-header__context">
            <strong>Admin workspace</strong>
            <span>Trung tâm điều hành VPSTTT</span>
          </div>
          <div className="admin-actions">
            <select
              aria-label="Chọn workspace"
              className="workspace-select"
              disabled={!data.workspaces.length}
              onChange={(event) => data.setWorkspaceId(event.target.value)}
              value={data.workspaceId}
            >
              {data.workspaces.map((workspace) => (
                <option key={workspace.id} value={workspace.id}>
                  {workspace.name}
                </option>
              ))}
            </select>
            <Input
              aria-label="Tìm kiếm trong trang quản trị"
              className="admin-search-control"
              leftAddon={<Search size={18} />}
              onChange={(event) => setSearchQuery(event.target.value)}
              placeholder="Tìm kiếm dữ liệu..."
              value={searchQuery}
            />
            <Tooltip label="Thông báo hệ thống">
              <Button
                aria-label="Thông báo hệ thống"
                className="admin-notification-button"
                onClick={() => {
                  setActiveNavItem("overview");
                  showToast(apiStatus.status === "online" ? "Hệ thống đang hoạt động ổn định." : apiStatus.label, "info");
                }}
                variant="icon"
              >
                <Bell size={19} />
                {apiStatus.status === "offline" ? <i aria-hidden="true" /> : null}
              </Button>
            </Tooltip>
            <Tooltip label={theme === "dark" ? "Chuyển sang chế độ sáng" : "Chuyển sang chế độ tối"}>
              <Button
                aria-label={theme === "dark" ? "Chuyển sang chế độ sáng" : "Chuyển sang chế độ tối"}
                onClick={toggleTheme}
                variant="icon"
              >
                {theme === "dark" ? <Sun size={19} /> : <Moon size={19} />}
              </Button>
            </Tooltip>
            <Tooltip label="Đăng xuất">
              <Button aria-label="Đăng xuất" onClick={logout} variant="icon">
                <LogOut size={19} />
              </Button>
            </Tooltip>
            <Avatar name={profile.name} src={profile.src} status={profile.status} />
          </div>
        </header>

        <div className="admin-page-heading">
          <div>
            <span className="eyebrow">Quản trị hệ thống</span>
            <h1>{activePage.title}</h1>
            <p>{activePage.description}</p>
          </div>
          <div className={`api-status-pill api-status-pill--${apiStatus.status}`}>
            <span />
            <strong>{apiStatus.label}</strong>
          </div>
        </div>

        {!data.workspaceId && !data.workspacesQuery.isLoading ? (
          <ErrorState description="Tài khoản hiện tại chưa có workspace để quản trị." title="Chưa có workspace" />
        ) : null}

        {adminDenied ? (
          <ErrorState
            description="Tài khoản hiện tại chưa có quyền `admin.view` trong workspace này."
            title="Chưa đủ quyền quản trị"
          />
        ) : (
          <DashboardSection
            activeNavItem={activeNavItem}
            activityBars={activityBars}
            channelRanks={channelRanks}
            data={data}
            filteredUsers={filteredUsers}
            healthChecks={healthChecks}
            metrics={metrics}
            searchQuery={searchQuery}
            selectedBackupJobId={selectedBackupJobId}
            selectedBotId={selectedBotId}
            selectedCronJobId={selectedCronJobId}
            selectedMemberId={selectedMemberId}
            selectedOutgoingWebhookId={selectedOutgoingWebhookId}
            setSearchQuery={setSearchQuery}
            setSelectedBackupJobId={setSelectedBackupJobId}
            setSelectedBotId={setSelectedBotId}
            setSelectedCronJobId={setSelectedCronJobId}
            setSelectedMemberId={setSelectedMemberId}
            setSelectedOutgoingWebhookId={setSelectedOutgoingWebhookId}
            setUserFilter={setUserFilter}
            showToast={showToast}
            userFilter={userFilter}
          />
        )}
        {showSystemPanel ? (
          <SettingsPanel
            data={data}
            healthChecks={healthChecks}
            onOpenSettings={() => setActiveNavItem("settings")}
          />
        ) : null}
      </section>

      {toast ? (
        <div className="toast-stack">
          <Toast tone={toast.tone}>
            {toast.message}
            <Button onClick={() => setToastState(null)} size="sm" variant="ghost">
              Đóng
            </Button>
          </Toast>
        </div>
      ) : null}
    </main>
  );
}

function DashboardSection({
  activeNavItem,
  activityBars,
  channelRanks,
  data,
  filteredUsers,
  healthChecks,
  metrics,
  searchQuery,
  selectedBackupJobId,
  selectedBotId,
  selectedCronJobId,
  selectedMemberId,
  selectedOutgoingWebhookId,
  setSearchQuery,
  setSelectedBackupJobId,
  setSelectedBotId,
  setSelectedCronJobId,
  setSelectedMemberId,
  setSelectedOutgoingWebhookId,
  setUserFilter,
  showToast,
  userFilter
}: {
  activeNavItem: AdminNavId;
  activityBars: number[];
  channelRanks: ChannelRank[];
  data: DashboardData;
  filteredUsers: AdminUser[];
  healthChecks: Array<{ name: string; value: string }>;
  metrics: DashboardMetric[];
  searchQuery: string;
  selectedBackupJobId: string;
  selectedBotId: string;
  selectedCronJobId: string;
  selectedMemberId: string;
  selectedOutgoingWebhookId: string;
  setSearchQuery: (value: string) => void;
  setSelectedBackupJobId: (value: string) => void;
  setSelectedBotId: (value: string) => void;
  setSelectedCronJobId: (value: string) => void;
  setSelectedMemberId: (value: string) => void;
  setSelectedOutgoingWebhookId: (value: string) => void;
  setUserFilter: (value: AdminUserFilter) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  userFilter: AdminUserFilter;
}) {
  if (activeNavItem === "overview") {
    return (
      <OverviewSection
        activityBars={activityBars}
        channelRanks={channelRanks}
        data={data}
        metrics={metrics}
      />
    );
  }

  if (activeNavItem === "messages") {
    return <AdminMessagesSection data={data} searchQuery={searchQuery} setSearchQuery={setSearchQuery} />;
  }

  if (activeNavItem === "channels") {
    return <AdminChannelsSection data={data} searchQuery={searchQuery} setSearchQuery={setSearchQuery} />;
  }

  if (activeNavItem === "users") {
    return (
      <UsersSection
        data={data}
        filteredUsers={filteredUsers}
        searchQuery={searchQuery}
        setSearchQuery={setSearchQuery}
        setUserFilter={setUserFilter}
        showToast={showToast}
        userFilter={userFilter}
      />
    );
  }

  if (activeNavItem === "roles") {
    return (
      <RolesSection
        data={data}
        selectedMemberId={selectedMemberId}
        setSelectedMemberId={setSelectedMemberId}
        showToast={showToast}
      />
    );
  }

  if (activeNavItem === "integrations") {
    return (
      <IntegrationsSection
        data={data}
        selectedOutgoingWebhookId={selectedOutgoingWebhookId}
        setSelectedOutgoingWebhookId={setSelectedOutgoingWebhookId}
        showToast={showToast}
      />
    );
  }

  if (activeNavItem === "automations") {
    return <AutomationsSection data={data} showToast={showToast} />;
  }

  if (activeNavItem === "bots") {
    return (
      <BotsSection
        data={data}
        selectedBotId={selectedBotId}
        setSelectedBotId={setSelectedBotId}
        showToast={showToast}
      />
    );
  }

  if (activeNavItem === "cronjobs") {
    return (
      <CronjobsSection
        data={data}
        selectedCronJobId={selectedCronJobId}
        setSelectedCronJobId={setSelectedCronJobId}
        showToast={showToast}
      />
    );
  }

  if (activeNavItem === "backups") {
    return (
      <BackupsSection
        data={data}
        selectedBackupJobId={selectedBackupJobId}
        setSelectedBackupJobId={setSelectedBackupJobId}
        showToast={showToast}
      />
    );
  }

  if (activeNavItem === "settings") {
    return <SystemSettingsSection data={data} healthChecks={healthChecks} showToast={showToast} />;
  }

  return (
    <section className="admin-panel">
      <EmptyState
        description="Màn này thuộc phase vận hành tiếp theo. Hiện dashboard chưa dựng dữ liệu giả cho khu vực này."
        title="Chưa triển khai trong phase hiện tại"
      />
    </section>
  );
}

function AdminMessagesSection({ data, searchQuery, setSearchQuery }: { data: DashboardData; searchQuery: string; setSearchQuery: (value: string) => void }) {
  const [kindFilter, setKindFilter] = useState("all");
  const [senderFilter, setSenderFilter] = useState("all");
  const [page, setPage] = useState(1);
  const query = searchQuery.trim().toLowerCase();
  const senders = useMemo(
    () => Array.from(new Set(data.adminMessages.map((message) => message.sender_name).filter(Boolean))).sort(),
    [data.adminMessages]
  );
  const kinds = useMemo(
    () => Array.from(new Set(data.adminMessages.map((message) => message.kind).filter(Boolean))).sort(),
    [data.adminMessages]
  );
  const messages = data.adminMessages.filter((message) => {
    const matchesQuery = !query
      || `${message.sender_name} ${message.channel_name} ${message.body}`.toLowerCase().includes(query);
    const matchesKind = kindFilter === "all" || message.kind === kindFilter;
    const matchesSender = senderFilter === "all" || message.sender_name === senderFilter;
    return matchesQuery && matchesKind && matchesSender;
  });
  const pageSize = 10;
  const pageCount = Math.max(1, Math.ceil(messages.length / pageSize));
  const safePage = Math.min(page, pageCount);
  const paginatedMessages = messages.slice((safePage - 1) * pageSize, safePage * pageSize);
  const todayKey = new Date().toDateString();
  const todayCount = data.adminMessages.filter((message) => {
    const date = new Date(message.created_at);
    return !Number.isNaN(date.getTime()) && date.toDateString() === todayKey;
  }).length;
  const botCount = data.adminMessages.filter((message) =>
    message.kind.toLowerCase() === "bot" || message.sender_name.toLowerCase().includes("bot")
  ).length;

  useEffect(() => setPage(1), [kindFilter, query, senderFilter]);

  return (
    <section className="admin-content-stack admin-resource-page">
      <div className="admin-summary-grid" aria-label="Thống kê tin nhắn">
        <AdminSummaryCard hint="Dữ liệu API" icon={<MessageCircle size={20} />} label="Tổng tin nhắn" tone="blue" value={data.adminMessages.length} />
        <AdminSummaryCard hint="Trong ngày" icon={<CalendarClock size={20} />} label="Tin nhắn hôm nay" tone="green" value={todayCount} />
        <AdminSummaryCard hint="Người dùng duy nhất" icon={<Users size={20} />} label="Người gửi" tone="purple" value={senders.length} />
        <AdminSummaryCard hint={`${data.adminMessages.length ? ((botCount / data.adminMessages.length) * 100).toFixed(1) : "0"}% tổng số`} icon={<Bot size={20} />} label="Bot messages" tone="orange" value={botCount} />
      </div>

      <div className="admin-filter-bar">
        <label className="admin-filter-control admin-filter-control--search">
          <span>Tìm nội dung</span>
          <span className="admin-inline-search"><Search size={16} /><input aria-label="Tìm nội dung tin nhắn" onChange={(event) => setSearchQuery(event.target.value)} placeholder="Nội dung, kênh hoặc người gửi" value={searchQuery} /></span>
        </label>
        <label className="admin-filter-control">
          <span>Loại tin nhắn</span>
          <select onChange={(event) => setKindFilter(event.target.value)} value={kindFilter}>
            <option value="all">Tất cả loại</option>
            {kinds.map((kind) => <option key={kind} value={kind}>{kind}</option>)}
          </select>
        </label>
        <label className="admin-filter-control">
          <span>Người gửi</span>
          <select onChange={(event) => setSenderFilter(event.target.value)} value={senderFilter}>
            <option value="all">Tất cả người gửi</option>
            {senders.map((sender) => <option key={sender} value={sender}>{sender}</option>)}
          </select>
        </label>
        <div className="admin-filter-bar__spacer" />
        <Button
          disabled={!messages.length}
          onClick={() => downloadCsv("tin-nhan", [
            ["Thời gian", "Kênh", "Người gửi", "Loại", "Nội dung"],
            ...messages.map((message) => [message.created_at, message.channel_name, message.sender_name, message.kind, message.body])
          ])}
          variant="secondary"
        >
          Xuất dữ liệu
        </Button>
      </div>

      <section className="admin-panel admin-table-panel">
        <header><div><h2>Danh sách tin nhắn <small>{messages.length} tin nhắn</small></h2><p>Dữ liệu trực tiếp từ API quản trị, tối đa 100 bản ghi mới nhất.</p></div></header>
      {data.adminMessagesQuery.isLoading ? <TableSkeleton /> : data.adminMessagesQuery.isError ? (
        <ErrorState
          action={<Button onClick={() => void data.adminMessagesQuery.refetch()} size="sm" variant="secondary">Tải lại</Button>}
          description={errorMessage(data.adminMessagesQuery.error)}
          title="Không tải được tin nhắn"
        />
      ) : paginatedMessages.length ? (
        <><div className="admin-table-wrap"><table className="admin-table admin-table--messages"><thead><tr><th>Thời gian</th><th>Kênh</th><th>Người gửi</th><th>Loại</th><th>Nội dung</th></tr></thead><tbody>
          {paginatedMessages.map((message) => <tr key={message.id}><td>{formatDateTime(message.created_at)}</td><td><span className="admin-channel-cell"><i />{message.channel_name}</span></td><td><strong>{message.sender_name}</strong></td><td><Badge tone={statusTone(message.kind)}>{message.kind}</Badge></td><td><span className="admin-message-preview" title={message.body}>{message.body}</span></td></tr>)}
        </tbody></table></div>
        <PaginationFooter count={messages.length} onPageChange={setPage} page={safePage} pageCount={pageCount} pageSize={pageSize} /></>
      ) : <EmptyState description="API không trả về tin nhắn phù hợp bộ lọc." title="Không có tin nhắn" />}
      </section>
    </section>
  );
}

function AdminChannelsSection({ data, searchQuery, setSearchQuery }: { data: DashboardData; searchQuery: string; setSearchQuery: (value: string) => void }) {
  const [typeFilter, setTypeFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState("all");
  const [page, setPage] = useState(1);
  const query = searchQuery.trim().toLowerCase();
  const types = useMemo(
    () => Array.from(new Set(data.adminChannels.map((channel) => channel.type).filter(Boolean))).sort(),
    [data.adminChannels]
  );
  const statuses = useMemo(
    () => Array.from(new Set(data.adminChannels.map((channel) => channel.status).filter(Boolean))).sort(),
    [data.adminChannels]
  );
  const channels = data.adminChannels.filter((channel) => {
    const matchesQuery = !query
      || `${channel.name} ${channel.slug ?? ""} ${channel.type}`.toLowerCase().includes(query);
    const matchesType = typeFilter === "all" || channel.type === typeFilter;
    const matchesStatus = statusFilter === "all" || channel.status === statusFilter;
    return matchesQuery && matchesType && matchesStatus;
  });
  const pageSize = 10;
  const pageCount = Math.max(1, Math.ceil(channels.length / pageSize));
  const safePage = Math.min(page, pageCount);
  const paginatedChannels = channels.slice((safePage - 1) * pageSize, safePage * pageSize);
  const publicCount = data.adminChannels.filter((channel) => channel.type === "public").length;
  const privateCount = data.adminChannels.filter((channel) => channel.type === "private").length;
  const sessionCount = data.adminChannels.filter((channel) => channel.private_session_mode).length;

  useEffect(() => setPage(1), [query, statusFilter, typeFilter]);

  return (
    <section className="admin-content-stack admin-resource-page">
      <div className="admin-summary-grid" aria-label="Thống kê kênh">
        <AdminSummaryCard hint="Kênh" icon={<Hash size={20} />} label="Tổng kênh" tone="blue" value={data.adminChannels.length} />
        <AdminSummaryCard hint={`${percentage(publicCount, data.adminChannels.length)}% tổng số`} icon={<MessageCircle size={20} />} label="Kênh công khai" tone="green" value={publicCount} />
        <AdminSummaryCard hint={`${percentage(privateCount, data.adminChannels.length)}% tổng số`} icon={<ShieldCheck size={20} />} label="Kênh riêng tư" tone="purple" value={privateCount} />
        <AdminSummaryCard hint={`${percentage(sessionCount, data.adminChannels.length)}% tổng số`} icon={<Bot size={20} />} label="Phiên bot riêng tư" tone="orange" value={sessionCount} />
      </div>

      <div className="admin-filter-bar">
        <label className="admin-filter-control admin-filter-control--search">
          <span>Tìm kênh</span>
          <span className="admin-inline-search"><Search size={16} /><input aria-label="Tìm kiếm kênh" onChange={(event) => setSearchQuery(event.target.value)} placeholder="Tên hoặc slug của kênh" value={searchQuery} /></span>
        </label>
        <label className="admin-filter-control">
          <span>Loại kênh</span>
          <select onChange={(event) => setTypeFilter(event.target.value)} value={typeFilter}>
            <option value="all">Tất cả loại</option>
            {types.map((type) => <option key={type} value={type}>{type}</option>)}
          </select>
        </label>
        <label className="admin-filter-control">
          <span>Trạng thái</span>
          <select onChange={(event) => setStatusFilter(event.target.value)} value={statusFilter}>
            <option value="all">Tất cả trạng thái</option>
            {statuses.map((status) => <option key={status} value={status}>{status}</option>)}
          </select>
        </label>
      </div>

      <section className="admin-panel admin-table-panel">
        <header><div><h2>Danh sách kênh <small>{channels.length} kênh</small></h2><p>Bao gồm kênh nhóm, kênh riêng và các phiên bot riêng tư từ API.</p></div></header>
      {data.adminChannelsQuery.isLoading ? <TableSkeleton /> : data.adminChannelsQuery.isError ? (
        <ErrorState
          action={<Button onClick={() => void data.adminChannelsQuery.refetch()} size="sm" variant="secondary">Tải lại</Button>}
          description={errorMessage(data.adminChannelsQuery.error)}
          title="Không tải được kênh"
        />
      ) : paginatedChannels.length ? (
        <><div className="admin-table-wrap"><table className="admin-table admin-table--channels"><thead><tr><th>Kênh</th><th>Loại</th><th>Trạng thái</th><th>Thành viên</th><th>Tin nhắn</th><th>Cập nhật</th></tr></thead><tbody>
          {paginatedChannels.map((channel) => <tr key={channel.id}><td><span className={`admin-channel-identity admin-channel-identity--${channel.type}`}><i><Hash size={15} /></i><span><strong>{channel.name}</strong><small>{channel.slug ? `#${channel.slug}` : shortId(channel.id)}</small></span></span></td><td>{channel.private_session_mode ? "Cổng phiên riêng" : channel.type}</td><td><Badge tone={statusTone(channel.status)}>{channel.status}</Badge></td><td>{formatNumber(channel.member_count)}</td><td>{formatNumber(channel.message_count)}</td><td>{formatDateTime(channel.updated_at)}</td></tr>)}
        </tbody></table></div>
        <PaginationFooter count={channels.length} onPageChange={setPage} page={safePage} pageCount={pageCount} pageSize={pageSize} /></>
      ) : <EmptyState description="API không trả về kênh phù hợp bộ lọc." title="Không có kênh" />}
      </section>
    </section>
  );
}

function AdminSummaryCard({ hint, icon, label, tone, value }: { hint?: string; icon: ReactNode; label: string; tone: "blue" | "green" | "orange" | "purple"; value: number }) {
  return (
    <article className={`admin-summary-card admin-summary-card--${tone}`}>
      <span>{icon}</span>
      <div><small>{label}</small><strong>{formatNumber(value)}</strong>{hint ? <em>{hint}</em> : null}</div>
    </article>
  );
}

function PaginationFooter({ count, onPageChange, page, pageCount, pageSize }: { count: number; onPageChange: (page: number) => void; page: number; pageCount: number; pageSize: number }) {
  const start = count ? (page - 1) * pageSize + 1 : 0;
  const end = Math.min(page * pageSize, count);
  const visiblePages = Array.from({ length: Math.min(pageCount, 5) }, (_, index) => {
    const firstPage = Math.max(1, Math.min(page - 2, pageCount - 4));
    return firstPage + index;
  });

  return (
    <footer className="admin-pagination">
      <span>Hiển thị <strong>{start}-{end}</strong> trong tổng số <strong>{count}</strong></span>
      <nav aria-label="Phân trang">
        <Button disabled={page <= 1} onClick={() => onPageChange(page - 1)} size="sm" variant="secondary">‹</Button>
        {visiblePages.map((item) => (
          <Button key={item} onClick={() => onPageChange(item)} size="sm" variant={item === page ? "primary" : "secondary"}>{item}</Button>
        ))}
        <Button disabled={page >= pageCount} onClick={() => onPageChange(page + 1)} size="sm" variant="secondary">›</Button>
      </nav>
    </footer>
  );
}

function OverviewSection({
  activityBars,
  channelRanks,
  data,
  metrics
}: {
  activityBars: number[];
  channelRanks: ChannelRank[];
  data: DashboardData;
  metrics: DashboardMetric[];
}) {
  return (
    <>
      <section className="metric-grid" aria-label="Chỉ số tổng quan">
        {data.statsQuery.isLoading || data.permissionsQuery.isLoading ? (
          <MetricSkeleton />
        ) : (
          metrics.map((metric) => (
            <MetricCard
              delta={metric.delta}
              key={metric.label}
              label={metric.label}
              tone={metric.tone}
              value={metric.value}
            />
          ))
        )}
      </section>

      <section className="admin-grid">
        <article className="admin-panel activity-panel">
          <header>
            <div>
              <h2>Hoạt động hệ thống</h2>
              <p>Dữ liệu biểu đồ lấy từ API admin stats khi backend cung cấp.</p>
            </div>
            <Badge tone={data.healthQuery.data?.status === "ready" ? "green" : "orange"}>
              {data.healthQuery.data?.status === "ready" ? "Ổn định" : "Đang kiểm tra"}
            </Badge>
          </header>
          {activityBars.length ? (
            <div className="line-chart" aria-label="Biểu đồ hoạt động hệ thống">
              {activityBars.map((height, index) => (
                <i aria-hidden="true" key={`${height}-${index}`} style={{ height: `${height}%` }} />
              ))}
            </div>
          ) : (
            <EmptyState description="Backend hiện chưa trả về chuỗi hoạt động theo ngày." title="Chưa có dữ liệu biểu đồ" />
          )}
        </article>

        <article className="admin-panel channel-panel-admin">
          <header>
            <div>
              <h2>Kênh hoạt động nhiều</h2>
              <p>Xếp hạng theo dữ liệu API admin stats.</p>
            </div>
            <Hash size={20} />
          </header>
          {channelRanks.length ? (
            <div className="channel-rank-list">
              {channelRanks.map((channel) => (
                <div className="channel-rank" key={channel.id}>
                  <span className={`rank-dot rank-dot--${channel.tone}`} />
                  <strong>{channel.name}</strong>
                  <em>{channel.count}</em>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState description="Backend hiện chưa trả về ranking kênh." title="Chưa có ranking" />
          )}
        </article>
      </section>

      <AuditPanel data={data} compact />
    </>
  );
}

function UsersSection({
  data,
  filteredUsers,
  searchQuery,
  setSearchQuery,
  setUserFilter,
  showToast,
  userFilter
}: {
  data: DashboardData;
  filteredUsers: AdminUser[];
  searchQuery: string;
  setSearchQuery: (value: string) => void;
  setUserFilter: (value: AdminUserFilter) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  userFilter: AdminUserFilter;
}) {
  async function handleAddMember(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const userId = formValue(form, "user_id");
    const roleCode = formValue(form, "role_code");

    if (!userId) {
      showToast("Vui lòng chọn người dùng cần thêm vào workspace.", "danger");
      return;
    }

    try {
      await data.addMemberMutation.mutateAsync({
        role_code: roleCode || undefined,
        user_id: userId
      });
      formElement.reset();
      showToast("Đã thêm thành viên vào workspace.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function updateMemberStatus(userId: string, status: string) {
    try {
      await data.updateMemberStatusMutation.mutateAsync({ input: { status }, userId });
      showToast("Đã cập nhật trạng thái thành viên.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function updateUserStatus(userId: string, status: "active" | "locked") {
    try {
      await data.updateUserMutation.mutateAsync({ input: { status }, userId });
      showToast(status === "locked" ? "Đã khóa người dùng." : "Đã mở khóa người dùng.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  return (
    <section className="admin-content-stack">
      <article className="admin-panel users-panel">
        <header>
          <div>
            <h2>Quản lý người dùng</h2>
            <p>Danh sách người dùng lấy trực tiếp từ API `/api/v1/users`.</p>
          </div>
        </header>
        {!data.canManageUsers ? <PermissionNotice permission="user.manage" /> : null}
        <div className="table-toolbar">
          <Input
            aria-label="Tìm kiếm người dùng"
            className="admin-search-control admin-search-control--compact"
            leftAddon={<Search size={16} />}
            onChange={(event) => setSearchQuery(event.target.value)}
            placeholder="Tìm kiếm người dùng, email..."
            value={searchQuery}
          />
          <SegmentedControl
            aria-label="Lọc trạng thái người dùng"
            className="toolbar-tabs"
            onValueChange={setUserFilter}
            options={userFilters}
            value={userFilter}
          />
        </div>
        {data.usersQuery.isLoading ? (
          <TableSkeleton />
        ) : data.usersQuery.isError ? (
          <ErrorState description="Không thể tải danh sách người dùng từ backend." title="Lỗi tải người dùng" />
        ) : filteredUsers.length ? (
          <div className="data-table data-table--users" role="table">
            <div className="data-table__row data-table__row--head" role="row">
              <span>#</span>
              <span>Họ và tên</span>
              <span>Email</span>
              <span>Phòng ban</span>
              <span>Vai trò</span>
              <span>Trạng thái</span>
              <span>Thao tác</span>
            </div>
            {filteredUsers.map((item, index) => (
              <div className="data-table__row" key={item.id} role="row">
                <span>{index + 1}</span>
                <span className="user-cell">
                  <Avatar name={item.name} size="sm" />
                  {item.name}
                </span>
                <span>{item.email}</span>
                <span>{item.department}</span>
                <span>{item.role}</span>
                <span>
                  <Badge tone={item.status === "active" ? "green" : "red"}>
                    {item.status === "active" ? "Hoạt động" : "Bị khóa"}
                  </Badge>
                </span>
                <span className="row-actions">
                  {item.status === "active" ? (
                    <Button
                      disabled={!data.canManageUsers || data.updateUserMutation.isPending}
                      onClick={() => void updateUserStatus(item.id, "locked")}
                      size="sm"
                      variant="secondary"
                    >
                      Khóa
                    </Button>
                  ) : (
                    <Button
                      disabled={!data.canManageUsers || data.updateUserMutation.isPending}
                      onClick={() => void updateUserStatus(item.id, "active")}
                      size="sm"
                      variant="secondary"
                    >
                      Mở khóa
                    </Button>
                  )}
                </span>
              </div>
            ))}
          </div>
        ) : (
          <EmptyState description="Không có người dùng nào khớp bộ lọc hiện tại." title="Danh sách trống" />
        )}
      </article>

      <article className="admin-panel">
        <header>
          <div>
            <h2>Thành viên workspace</h2>
            <p>Thêm thành viên và cập nhật trạng thái bằng API workspace members.</p>
          </div>
          <Users size={20} />
        </header>
        <form className="admin-form admin-form--inline" onSubmit={(event) => void handleAddMember(event)}>
          <label>
            Người dùng
            <select name="user_id" required>
              <option value="">Chọn người dùng</option>
              {data.users.map((item) => (
                <option key={item.id} value={item.id}>
                  {displayName(item)} - {item.email}
                </option>
              ))}
            </select>
          </label>
          <label>
            Role ban đầu
            <select name="role_code">
              <option value="">Không gán role</option>
              {data.roles.map((role) => (
                <option key={role.id} value={role.code}>
                  {role.name}
                </option>
              ))}
            </select>
          </label>
          <Button disabled={data.addMemberMutation.isPending || !data.canManageWorkspace} type="submit">
            <Plus size={16} />
            Thêm thành viên
          </Button>
        </form>
        {!data.canManageWorkspace ? (
          <PermissionNotice permission="workspace.manage" />
        ) : null}
        {data.membersQuery.isLoading ? (
          <TableSkeleton />
        ) : data.members.length ? (
          <div className="data-table data-table--members" role="table">
            <div className="data-table__row data-table__row--head" role="row">
              <span>Thành viên</span>
              <span>Email</span>
              <span>Vai trò</span>
              <span>Trạng thái</span>
              <span>Thao tác</span>
            </div>
            {data.members.map((member) => (
              <div className="data-table__row" key={member.user_id} role="row">
                <span className="user-cell">
                  <Avatar name={memberDisplayName(member)} size="sm" src={member.avatar_url ?? undefined} />
                  {memberDisplayName(member)}
                </span>
                <span>{member.email ?? "Chưa có email"}</span>
                <span>{member.role ?? "Chưa gán"}</span>
                <span>
                  <Badge tone={member.status === "active" ? "green" : "slate"}>{member.status ?? "unknown"}</Badge>
                </span>
                <span className="row-actions">
                  <Button
                    disabled={data.updateMemberStatusMutation.isPending || !data.canManageWorkspace}
                    onClick={() => void updateMemberStatus(member.user_id, "active")}
                    size="sm"
                    variant="secondary"
                  >
                    Active
                  </Button>
                  <Button
                    disabled={data.updateMemberStatusMutation.isPending || !data.canManageWorkspace}
                    onClick={() => void updateMemberStatus(member.user_id, "inactive")}
                    size="sm"
                    variant="ghost"
                  >
                    Inactive
                  </Button>
                </span>
              </div>
            ))}
          </div>
        ) : (
          <EmptyState description="Backend chưa trả về thành viên nào cho workspace này." title="Chưa có thành viên" />
        )}
      </article>
    </section>
  );
}

function RolesSection({
  data,
  selectedMemberId,
  setSelectedMemberId,
  showToast
}: {
  data: DashboardData;
  selectedMemberId: string;
  setSelectedMemberId: (value: string) => void;
  showToast: (message: string, tone?: ToastTone) => void;
}) {
  async function handleCreateRole(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const code = formValue(form, "code");
    const name = formValue(form, "name");

    if (!code || !name) {
      showToast("Vui lòng nhập mã role và tên role.", "danger");
      return;
    }

    try {
      await data.createRoleMutation.mutateAsync({
        code,
        description: formValue(form, "description") || undefined,
        name,
        permission_codes: formValues(form, "permission_codes")
      });
      formElement.reset();
      showToast("Đã tạo role mới.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function handleAssignRole(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const userId = formValue(form, "user_id");
    const roleId = formValue(form, "role_id");

    if (!userId || !roleId) {
      showToast("Vui lòng chọn thành viên và role.", "danger");
      return;
    }

    try {
      await data.assignMemberRoleMutation.mutateAsync({ roleId, userId });
      setSelectedMemberId(userId);
      showToast("Đã gán role cho thành viên.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function handleRevokeRole(roleId: string) {
    if (!selectedMemberId || !roleId) {
      showToast("Vui lòng chọn thành viên và role cần gỡ.", "danger");
      return;
    }

    try {
      await data.revokeMemberRoleMutation.mutateAsync({ roleId, userId: selectedMemberId });
      showToast("Đã gỡ role khỏi thành viên.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  return (
    <section className="admin-content-stack">
      <InstanceAdministration data={data} showToast={showToast} />
      <article className="admin-panel">
        <header>
          <div>
            <h2>Danh sách vai trò</h2>
            <p>Role và permission lấy từ `/api/v1/rbac/roles` theo workspace.</p>
          </div>
          <ShieldCheck size={20} />
        </header>
        {data.rolesQuery.isLoading ? (
          <TableSkeleton />
        ) : data.roles.length ? (
          <div className="data-table data-table--roles" role="table">
            <div className="data-table__row data-table__row--head" role="row">
              <span>Role</span>
              <span>Mã</span>
              <span>Loại</span>
              <span>Permission</span>
            </div>
            {data.roles.map((role) => (
              <div className="data-table__row" key={role.id} role="row">
                <span>
                  <strong>{role.name}</strong>
                  <small>{role.description ?? "Không có mô tả"}</small>
                </span>
                <span>{role.code}</span>
                <span>
                  <Badge tone={role.is_system ? "blue" : "slate"}>
                    {role.is_system ? "Hệ thống" : "Tùy chỉnh"}
                  </Badge>
                </span>
                <span>{formatNumber(role.permissions?.length ?? 0)}</span>
              </div>
            ))}
          </div>
        ) : (
          <EmptyState description="Workspace chưa có role nào được backend trả về." title="Chưa có role" />
        )}
      </article>

      <section className="admin-split-grid">
        <article className="admin-panel">
          <header>
            <div>
              <h2>Tạo role</h2>
              <p>Chọn permission từ catalog thật của backend.</p>
            </div>
          </header>
          <form className="admin-form" onSubmit={(event) => void handleCreateRole(event)}>
            <label>
              Mã role
              <input name="code" placeholder="support-lead" required />
            </label>
            <label>
              Tên role
              <input name="name" placeholder="Support Lead" required />
            </label>
            <label>
              Mô tả
              <textarea name="description" placeholder="Quyền vận hành hỗ trợ nội bộ" rows={3} />
            </label>
            <label>
              Permission
              <select multiple name="permission_codes" size={8}>
                {data.permissionsCatalog.map((permission) => (
                  <option key={permission.code} value={permission.code}>
                    {permission.code} - {permission.name ?? permission.code}
                  </option>
                ))}
              </select>
            </label>
            <Button disabled={data.createRoleMutation.isPending || !data.canManageRoles} type="submit">
              <Plus size={16} />
              Tạo role
            </Button>
          </form>
          {!data.canManageRoles ? <PermissionNotice permission="role.manage" /> : null}
        </article>

        <article className="admin-panel">
          <header>
            <div>
              <h2>Gán role</h2>
              <p>Gán hoặc gỡ role cho thành viên workspace.</p>
            </div>
          </header>
          <form className="admin-form" onSubmit={(event) => void handleAssignRole(event)}>
            <label>
              Thành viên
              <select
                name="user_id"
                onChange={(event) => setSelectedMemberId(event.target.value)}
                value={selectedMemberId}
              >
                <option value="">Chọn thành viên</option>
                {data.members.map((member) => (
                  <option key={member.user_id} value={member.user_id}>
                    {memberDisplayName(member)}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Role
              <select name="role_id" required>
                <option value="">Chọn role</option>
                {data.roles.map((role) => (
                  <option key={role.id} value={role.id}>
                    {role.name}
                  </option>
                ))}
              </select>
            </label>
            <Button disabled={data.assignMemberRoleMutation.isPending || !data.canManageRoles} type="submit">
              Gán role
            </Button>
          </form>
          <div className="mini-list">
            <strong>Role hiện tại</strong>
            {data.selectedMemberRolesQuery.isLoading ? (
              <Skeleton />
            ) : data.selectedMemberRoles.length ? (
              data.selectedMemberRoles.map((role) => (
                <div key={role.id}>
                  <span>{role.name}</span>
                  <Button
                    disabled={data.revokeMemberRoleMutation.isPending || !data.canManageRoles}
                    onClick={() => void handleRevokeRole(role.id)}
                    size="sm"
                    variant="ghost"
                  >
                    Gỡ
                  </Button>
                </div>
              ))
            ) : (
              <small>Chọn thành viên để xem role hiện tại.</small>
            )}
          </div>
          {!data.canManageRoles ? <PermissionNotice permission="role.manage" /> : null}
        </article>
      </section>

      <AuditPanel data={data} />
    </section>
  );
}

function IntegrationsSection({
  data,
  selectedOutgoingWebhookId,
  setSelectedOutgoingWebhookId,
  showToast
}: {
  data: DashboardData;
  selectedOutgoingWebhookId: string;
  setSelectedOutgoingWebhookId: (value: string) => void;
  showToast: (message: string, tone?: ToastTone) => void;
}) {
  async function handleCreateToken(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const scopes = formValues(form, "scopes");

    if (!scopes.length) {
      showToast("Vui lòng chọn ít nhất một API scope.", "danger");
      return;
    }

    try {
      await data.createApiTokenMutation.mutateAsync({
        expires_days: Number(formValue(form, "expires_days")) || undefined,
        name: formValue(form, "name"),
        scopes
      });
      formElement.reset();
      showToast("Đã tạo API token. Secret chỉ hiển thị một lần.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function handleCreateIncoming(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);

    try {
      await data.createIncomingWebhookMutation.mutateAsync({
        channel_id: formValue(form, "channel_id") || undefined,
        name: formValue(form, "name")
      });
      formElement.reset();
      showToast("Đã tạo incoming webhook. Secret chỉ hiển thị một lần.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function handleCreateOutgoing(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);

    try {
      await data.createOutgoingWebhookMutation.mutateAsync({
        event_types: splitList(formValue(form, "event_types")),
        name: formValue(form, "name"),
        target_url: formValue(form, "target_url")
      });
      formElement.reset();
      showToast("Đã tạo outgoing webhook. Secret chỉ hiển thị một lần.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function revokeToken(tokenId: string) {
    try {
      await data.revokeApiTokenMutation.mutateAsync(tokenId);
      showToast("Đã thu hồi API token.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function deleteIncoming(webhookId: string) {
    try {
      await data.deleteIncomingWebhookMutation.mutateAsync(webhookId);
      showToast("Đã xóa incoming webhook.");
    } catch (error) { showToast(errorMessage(error), "danger"); }
  }

  async function toggleIncoming(webhook: IncomingWebhook) {
    try {
      await data.updateIncomingWebhookMutation.mutateAsync({ input: { status: webhook.status === "active" ? "disabled" : "active" }, webhookId: webhook.id });
      showToast("Đã cập nhật incoming webhook.");
    } catch (error) { showToast(errorMessage(error), "danger"); }
  }

  async function deleteOutgoing(webhookId: string) {
    try {
      await data.deleteOutgoingWebhookMutation.mutateAsync(webhookId);
      showToast("Đã xóa outgoing webhook.");
    } catch (error) { showToast(errorMessage(error), "danger"); }
  }

  async function toggleOutgoing(webhook: OutgoingWebhook) {
    try {
      await data.updateOutgoingWebhookMutation.mutateAsync({ input: { status: webhook.status === "active" ? "disabled" : "active" }, webhookId: webhook.id });
      showToast("Đã cập nhật outgoing webhook.");
    } catch (error) { showToast(errorMessage(error), "danger"); }
  }

  async function testOutgoing() {
    if (!selectedOutgoingWebhookId) return;
    try {
      await data.testOutgoingWebhookMutation.mutateAsync(selectedOutgoingWebhookId);
      showToast("Webhook test đã được gửi qua API.");
    } catch (error) { showToast(errorMessage(error), "danger"); }
  }

  return (
    <section className="admin-content-stack">
      <section className="admin-split-grid">
        <article className="admin-panel">
          <header>
            <div>
              <h2>API token</h2>
              <p>Tạo token theo scope để hệ thống ngoài gửi dữ liệu vào chat.</p>
            </div>
            <KeyRound size={20} />
          </header>
          {!data.canManageApiTokens ? <PermissionNotice permission="api_token.manage" /> : null}
          <form className="admin-form" onSubmit={(event) => void handleCreateToken(event)}>
            <label>
              Tên token
              <input name="name" placeholder="Monitor production" required />
            </label>
            <label>
              Hết hạn sau số ngày
              <input min={1} name="expires_days" placeholder="30" type="number" />
            </label>
            <ScopeSelector scopes={data.apiScopes} />
            <Button disabled={data.createApiTokenMutation.isPending || !data.canManageApiTokens} type="submit">
              <Plus size={16} />
              Tạo token
            </Button>
          </form>
          {data.createApiTokenMutation.data?.token ? (
            <SecretBox label="API token mới" value={data.createApiTokenMutation.data.token} />
          ) : null}
        </article>

        <article className="admin-panel">
          <header>
            <div>
              <h2>Token đang hoạt động</h2>
              <p>Danh sách token lấy từ `/api-tokens`.</p>
            </div>
          </header>
          <TokenTable onRevoke={(tokenId) => void revokeToken(tokenId)} tokens={data.apiTokens} />
        </article>
      </section>

      <section className="admin-split-grid">
        <article className="admin-panel">
          <header>
            <div>
              <h2>Incoming webhook</h2>
              <p>Tạo URL nhận message từ hệ thống ngoài.</p>
            </div>
            <Workflow size={20} />
          </header>
          {!data.canManageWebhooks ? <PermissionNotice permission="webhook.manage" /> : null}
          <form className="admin-form" onSubmit={(event) => void handleCreateIncoming(event)}>
            <label>
              Tên webhook
              <input name="name" placeholder="Alert server" required />
            </label>
            <label>
              Kênh mặc định
              <ChannelSelect channels={data.channels} name="channel_id" />
            </label>
            <Button disabled={data.createIncomingWebhookMutation.isPending || !data.canManageWebhooks} type="submit">
              <Plus size={16} />
              Tạo incoming
            </Button>
          </form>
          {data.createIncomingWebhookMutation.data?.secret ? (
            <SecretBox label="Secret incoming webhook" value={data.createIncomingWebhookMutation.data.secret} />
          ) : null}
          {data.createIncomingWebhookMutation.data?.url ? (
            <SecretBox label="URL incoming webhook" value={data.createIncomingWebhookMutation.data.url} />
          ) : null}
          <WebhookList incomingWebhooks={data.incomingWebhooks} onDelete={(id) => void deleteIncoming(id)} onToggle={(webhook) => void toggleIncoming(webhook)} />
        </article>

        <article className="admin-panel">
          <header>
            <div>
              <h2>Outgoing webhook</h2>
              <p>Đẩy event chat sang hệ thống ngoài và theo dõi delivery log.</p>
            </div>
            <Zap size={20} />
          </header>
          {!data.canManageWebhooks ? <PermissionNotice permission="webhook.manage" /> : null}
          <form className="admin-form" onSubmit={(event) => void handleCreateOutgoing(event)}>
            <label>
              Tên webhook
              <input name="name" placeholder="CRM sync" required />
            </label>
            <label>
              Target URL
              <input name="target_url" placeholder="https://example.com/webtui" required type="url" />
            </label>
            <label>
              Event types
              <input name="event_types" placeholder="message.created, message.updated" />
            </label>
            <Button disabled={data.createOutgoingWebhookMutation.isPending || !data.canManageWebhooks} type="submit">
              <Plus size={16} />
              Tạo outgoing
            </Button>
          </form>
          {data.createOutgoingWebhookMutation.data?.secret ? (
            <SecretBox label="Secret outgoing webhook" value={data.createOutgoingWebhookMutation.data.secret} />
          ) : null}
          <OutgoingWebhookList
            onDelete={(id) => void deleteOutgoing(id)}
            onSelect={setSelectedOutgoingWebhookId}
            onToggle={(webhook) => void toggleOutgoing(webhook)}
            selectedId={selectedOutgoingWebhookId}
            webhooks={data.outgoingWebhooks}
          />
        </article>
      </section>

      <DeliveryPanel deliveries={data.webhookDeliveries} isLoading={data.webhookDeliveriesQuery.isLoading} onTest={() => void testOutgoing()} testDisabled={!selectedOutgoingWebhookId || data.testOutgoingWebhookMutation.isPending} />
    </section>
  );
}

function AutomationsSection({
  data,
  showToast
}: {
  data: DashboardData;
  showToast: (message: string, tone?: ToastTone) => void;
}) {
  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const configResult = parseJsonObject(
      formValue(form, "config"),
      "Config automation phải là JSON object hợp lệ."
    );
    if (!configResult.ok) {
      showToast(configResult.message, "danger");
      return;
    }

    try {
      await data.createAutomationInstallationMutation.mutateAsync({
        config: configResult.value,
        name: formValue(form, "name"),
        secret_ref: formValue(form, "secret_ref") || undefined,
        status: formValue(form, "status") as "enabled" | "disabled",
        template_key: formValue(form, "template_key"),
        workspace_id: data.workspaceId
      });
      formElement.reset();
      showToast("Đã cài automation cho zone hiện tại.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function handleToggle(installation: AutomationInstallation) {
    try {
      await data.updateAutomationInstallationMutation.mutateAsync({
        input: {
          status: installation.status === "enabled" ? "disabled" : "enabled"
        },
        installationId: installation.id
      });
      showToast("Đã cập nhật trạng thái automation.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function handleDelete(installationId: string) {
    try {
      await data.deleteAutomationInstallationMutation.mutateAsync(installationId);
      showToast("Đã gỡ automation khỏi zone.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  const isLoading =
    data.automationTemplatesQuery.isLoading || data.automationInstallationsQuery.isLoading;
  const queryError =
    data.automationTemplatesQuery.error || data.automationInstallationsQuery.error;

  return (
    <section className="admin-content-stack">
      {!data.canManageAutomation ? <PermissionNotice permission="workspace.manage" /> : null}
      {queryError ? (
        <ErrorState
          description={errorMessage(queryError)}
          title="Không tải được automation của zone"
        />
      ) : null}
      <section className="admin-split-grid">
        <article className="admin-panel">
          <header>
            <div>
              <h2>Cài automation</h2>
              <p>Template được lọc theo loại zone và domain đang đăng nhập.</p>
            </div>
            <Zap size={20} />
          </header>
          <form className="admin-form" onSubmit={(event) => void handleCreate(event)}>
            <label>
              Template
              <select name="template_key" required>
                <option value="">Chọn template</option>
                {data.automationTemplates.map((template) => (
                  <option key={template.id} value={template.key}>
                    {template.name} ({template.template_type})
                  </option>
                ))}
              </select>
            </label>
            <label>
              Tên cài đặt
              <input name="name" placeholder="Cảnh báo vận hành" required />
            </label>
            <label>
              Trạng thái
              <select defaultValue="disabled" name="status">
                <option value="disabled">Tắt</option>
                <option value="enabled">Bật</option>
              </select>
            </label>
            <label>
              Config JSON
              <textarea defaultValue="{}" name="config" rows={6} spellCheck={false} />
            </label>
            <label>
              Secret reference
              <input
                name="secret_ref"
                placeholder="vault://zones/customer/automation"
              />
            </label>
            <Button
              disabled={
                data.createAutomationInstallationMutation.isPending ||
                !data.canManageAutomation ||
                !data.automationTemplates.length
              }
              type="submit"
            >
              <Plus size={16} />
              Cài automation
            </Button>
          </form>
          {data.createAutomationInstallationMutation.data?.runtime_secret ? (
            <SecretBox
              label="Signing secret automation (chi hien thi mot lan)"
              value={data.createAutomationInstallationMutation.data.runtime_secret}
            />
          ) : null}
        </article>

        <article className="admin-panel">
          <header>
            <div>
              <h2>Template khả dụng</h2>
              <p>{data.automationTemplates.length} template phù hợp với zone.</p>
            </div>
          </header>
          {data.automationTemplates.length ? (
            <div className="mini-list">
              {data.automationTemplates.map((template) => (
                <div key={template.id}>
                  <span>
                    <strong>{template.name}</strong>
                    <small>{template.key}</small>
                  </span>
                  <Badge tone="blue">{template.template_type}</Badge>
                </div>
              ))}
            </div>
          ) : isLoading ? (
            <TableSkeleton />
          ) : (
            <EmptyState
              description="Zone hiện tại chưa có template phù hợp."
              title="Chưa có automation template"
            />
          )}
        </article>
      </section>

      <article className="admin-panel">
        <header>
          <div>
            <h2>Automation đã cài</h2>
            <p>Mỗi cài đặt chỉ thuộc zone và workspace hiện tại.</p>
          </div>
        </header>
        {isLoading ? (
          <TableSkeleton />
        ) : data.automationInstallations.length ? (
          <AutomationInstallationTable
            installations={data.automationInstallations}
            onDelete={(id) => void handleDelete(id)}
            onToggle={(installation) => void handleToggle(installation)}
          />
        ) : (
          <EmptyState
            description="Chưa có workflow, connector hoặc bot template nào được cài."
            title="Chưa có automation"
          />
        )}
      </article>
    </section>
  );
}

function AutomationInstallationTable({
  installations,
  onDelete,
  onToggle
}: {
  installations: AutomationInstallation[];
  onDelete: (installationId: string) => void;
  onToggle: (installation: AutomationInstallation) => void;
}) {
  return (
    <div className="data-table data-table--automations" role="table">
      <div className="data-table__row data-table__row--head" role="row">
        <span>Automation</span>
        <span>Template</span>
        <span>Runtime</span>
        <span>Trạng thái</span>
        <span>Thao tác</span>
      </div>
      {installations.map((installation) => (
        <div className="data-table__row" key={installation.id} role="row">
          <span>
            <strong>{installation.name}</strong>
            <small>{formatDateTime(installation.updated_at)}</small>
          </span>
          <span>{installation.template_key || "Template đã gỡ"}</span>
          <span>
            <Badge tone={installation.runtime_ready ? "green" : "slate"}>
              {installation.runtime_ready ? "Sẵn sàng" : "Registry"}
            </Badge>
          </span>
          <span>
            <Badge tone={installation.status === "enabled" ? "green" : "slate"}>
              {installation.status}
            </Badge>
          </span>
          <span className="row-actions">
            <Button onClick={() => onToggle(installation)} size="sm" variant="ghost">
              {installation.status === "enabled" ? "Tắt" : "Bật"}
            </Button>
            <Tooltip label="Gỡ automation">
              <Button
                aria-label={`Gỡ ${installation.name}`}
                onClick={() => onDelete(installation.id)}
                size="sm"
                variant="icon"
              >
                <Trash2 size={15} />
              </Button>
            </Tooltip>
          </span>
        </div>
      ))}
    </div>
  );
}

function BotsSection({
  data,
  selectedBotId,
  setSelectedBotId,
  showToast
}: {
  data: DashboardData;
  selectedBotId: string;
  setSelectedBotId: (value: string) => void;
  showToast: (message: string, tone?: ToastTone) => void;
}) {
  async function handleCreateBot(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);

    try {
      await data.createBotMutation.mutateAsync({
        avatar_url: formValue(form, "avatar_url") || undefined,
        description: formValue(form, "description") || undefined,
        name: formValue(form, "name"),
        settings: {},
        slug: formValue(form, "slug")
      });
      formElement.reset();
      showToast("Đã tạo bot.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function handleInstallBot(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const botId = formValue(form, "bot_id");
    const channelId = formValue(form, "channel_id");

    if (!botId) {
      showToast("Vui lòng chọn bot cần cài đặt.", "danger");
      return;
    }

    try {
      await data.installBotMutation.mutateAsync({
        botId,
        input: { channel_id: channelId || undefined, config: {} }
      });
      setSelectedBotId(botId);
      showToast("Đã cài bot vào kênh.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function handleSendBotMessage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const botId = formValue(form, "bot_id");
    const channelId = formValue(form, "channel_id");
    const body = formValue(form, "body");

    if (!botId || !channelId || !body) {
      showToast("Vui lòng chọn bot, kênh và nhập nội dung test.", "danger");
      return;
    }

    try {
      await data.sendBotMessageMutation.mutateAsync({
        botId,
        input: {
          body,
          channel_id: channelId,
          metadata: { source: "admin-test" }
        }
      });
      formElement.reset();
      showToast("Bot đã gửi tin nhắn test.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  return (
    <section className="admin-content-stack">
      {!data.canManageBots ? <PermissionNotice permission="bot.manage" /> : null}
      <section className="admin-split-grid">
        <article className="admin-panel">
          <header>
            <div>
              <h2>Tạo bot</h2>
              <p>Bot được tạo qua API `/bots` và có thể cài vào kênh.</p>
            </div>
            <Bot size={20} />
          </header>
          <form className="admin-form" onSubmit={(event) => void handleCreateBot(event)}>
            <label>
              Slug
              <input name="slug" placeholder="server-alert" required />
            </label>
            <label>
              Tên bot
              <input name="name" placeholder="Server Alert" required />
            </label>
            <label>
              Avatar URL
              <input name="avatar_url" placeholder="https://..." />
            </label>
            <label>
              Mô tả
              <textarea name="description" placeholder="Bot gửi cảnh báo hệ thống" rows={3} />
            </label>
            <Button disabled={data.createBotMutation.isPending || !data.canManageBots} type="submit">
              <Plus size={16} />
              Tạo bot
            </Button>
          </form>
        </article>

        <article className="admin-panel">
          <header>
            <div>
              <h2>Bot hiện có</h2>
              <p>Danh sách bot lấy từ backend theo workspace.</p>
            </div>
          </header>
          <BotTable bots={data.bots} onSelect={setSelectedBotId} selectedId={selectedBotId} />
        </article>
      </section>

      <section className="admin-split-grid">
        <article className="admin-panel">
          <header>
            <div>
              <h2>Cài đặt bot</h2>
              <p>Cài bot vào workspace hoặc kênh cụ thể.</p>
            </div>
          </header>
          <form className="admin-form" onSubmit={(event) => void handleInstallBot(event)}>
            <label>
              Bot
              <BotSelect bots={data.bots} name="bot_id" selectedId={selectedBotId} />
            </label>
            <label>
              Kênh
              <ChannelSelect channels={data.channels} name="channel_id" />
            </label>
            <Button disabled={data.installBotMutation.isPending || !data.canManageBots} type="submit">
              Cài bot
            </Button>
          </form>
          <InstallationList installations={data.botInstallations} isLoading={data.botInstallationsQuery.isLoading} />
        </article>

        <article className="admin-panel">
          <header>
            <div>
              <h2>Gửi tin test</h2>
              <p>Gọi API bot message để kiểm tra bot đã hoạt động trong kênh.</p>
            </div>
            <Send size={20} />
          </header>
          <form className="admin-form" onSubmit={(event) => void handleSendBotMessage(event)}>
            <label>
              Bot
              <BotSelect bots={data.bots} name="bot_id" selectedId={selectedBotId} />
            </label>
            <label>
              Kênh
              <ChannelSelect channels={data.channels} name="channel_id" required />
            </label>
            <label>
              Nội dung
              <textarea name="body" placeholder="Kiểm tra cảnh báo từ bot" required rows={4} />
            </label>
            <Button disabled={data.sendBotMessageMutation.isPending || !data.canManageBots} type="submit">
              <Send size={16} />
              Gửi test
            </Button>
          </form>
        </article>
      </section>
    </section>
  );
}

function CronjobsSection({
  data,
  selectedCronJobId,
  setSelectedCronJobId,
  showToast
}: {
  data: DashboardData;
  selectedCronJobId: string;
  setSelectedCronJobId: (value: string) => void;
  showToast: (message: string, tone?: ToastTone) => void;
}) {
  async function handleCreateCronJob(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const payloadResult = parseJsonObject(
      formValue(form, "payload"),
      "Payload cronjob phải là JSON object hợp lệ."
    );

    if (!payloadResult.ok) {
      showToast(payloadResult.message, "danger");
      return;
    }

    try {
      const created = await data.createCronjobMutation.mutateAsync({
        description: formValue(form, "description") || undefined,
        name: formValue(form, "name"),
        payload: payloadResult.value,
        runner: formValue(form, "runner"),
        schedule: formValue(form, "schedule"),
        status: formValue(form, "status")
      });
      formElement.reset();
      setSelectedCronJobId(created.id);
      showToast("Đã tạo cronjob mới.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function handleRunCronJob(job: CronJob) {
    try {
      const run = await data.runCronjobMutation.mutateAsync(job.id);
      setSelectedCronJobId(job.id);
      showToast(`Đã tạo lượt chạy ${shortId(run.id)}.`);
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function handleUpdateCronJobStatus(job: CronJob, status: string) {
    try {
      await data.updateCronjobMutation.mutateAsync({
        cronjobId: job.id,
        input: cronJobToInput(job, status)
      });
      showToast("Đã cập nhật trạng thái cronjob.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function handleDeleteCronJob(job: CronJob) {
    try {
      await data.deleteCronjobMutation.mutateAsync(job.id);
      if (selectedCronJobId === job.id) {
        setSelectedCronJobId("");
      }
      showToast("Đã xóa cronjob.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  const selectedJob = data.cronjobs.find((job) => job.id === selectedCronJobId) ?? null;

  return (
    <section className="admin-content-stack">
      {!data.canManageCronjobs ? <PermissionNotice permission="cronjob.manage" /> : null}
      <section className="admin-split-grid">
        <article className="admin-panel">
          <header>
            <div>
              <h2>Tạo cronjob</h2>
              <p>Lịch chạy, runner và payload được gửi thẳng tới API cronjob của backend.</p>
            </div>
            <CalendarClock size={20} />
          </header>
          <form className="admin-form" onSubmit={(event) => void handleCreateCronJob(event)}>
            <label>
              Tên cronjob
              <input name="name" placeholder="Dọn dẹp file tạm" required />
            </label>
            <label>
              Mô tả
              <textarea name="description" placeholder="Tác vụ vận hành chạy định kỳ" rows={3} />
            </label>
            <label>
              Lịch chạy
              <input name="schedule" placeholder="@every 15m hoặc 0 3 * * *" required />
            </label>
            <label>
              Runner
              <select defaultValue="builtin_cleanup" name="runner" required>
                {cronJobRunners.map((runner) => (
                  <option key={runner.value} value={runner.value}>
                    {runner.label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Trạng thái
              <select defaultValue="active" name="status">
                {cronJobStatuses.map((status) => (
                  <option key={status.value} value={status.value}>
                    {status.label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Payload JSON
              <textarea defaultValue="{}" name="payload" required rows={6} />
            </label>
            <Button disabled={data.createCronjobMutation.isPending || !data.canManageCronjobs} type="submit">
              <Plus size={16} />
              Tạo cronjob
            </Button>
          </form>
        </article>

        <article className="admin-panel">
          <header>
            <div>
              <h2>Cronjob hiện có</h2>
              <p>Danh sách và thao tác vận hành lấy từ `/cronjobs`.</p>
            </div>
          </header>
          <CronJobTable
            canManage={data.canManageCronjobs}
            isError={data.cronjobsQuery.isError}
            isLoading={data.cronjobsQuery.isLoading}
            jobs={data.cronjobs}
            mutationPending={
              data.deleteCronjobMutation.isPending ||
              data.runCronjobMutation.isPending ||
              data.updateCronjobMutation.isPending
            }
            onDelete={(job) => void handleDeleteCronJob(job)}
            onRun={(job) => void handleRunCronJob(job)}
            onSelect={setSelectedCronJobId}
            onStatusChange={(job, status) => void handleUpdateCronJobStatus(job, status)}
            selectedId={selectedCronJobId}
          />
        </article>
      </section>

      <article className="admin-panel">
        <header>
          <div>
            <h2>Lịch sử chạy cronjob</h2>
            <p>{selectedJob ? `Đang xem ${selectedJob.name}.` : "Chọn một cronjob để xem run history từ backend."}</p>
          </div>
          <Badge tone={selectedJob ? statusTone(selectedJob.status) : "slate"}>
            {selectedJob?.status ?? "Chưa chọn"}
          </Badge>
        </header>
        <CronJobRunsTable
          isLoading={data.cronjobRunsQuery.isLoading}
          runs={data.cronjobRuns}
          selectedJob={selectedJob}
        />
      </article>
    </section>
  );
}

function BackupsSection({
  data,
  selectedBackupJobId,
  setSelectedBackupJobId,
  showToast
}: {
  data: DashboardData;
  selectedBackupJobId: string;
  setSelectedBackupJobId: (value: string) => void;
  showToast: (message: string, tone?: ToastTone) => void;
}) {
  async function handleCreateBackupJob(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const configResult = parseJsonObject(
      formValue(form, "config"),
      "Config backup phải là JSON object hợp lệ."
    );

    if (!configResult.ok) {
      showToast(configResult.message, "danger");
      return;
    }

    const input: CreateBackupJobInput = {
      backup_type: formValue(form, "backup_type") || "database",
      config: configResult.value,
      name: formValue(form, "name"),
      schedule: formValue(form, "schedule") || undefined,
      status: formValue(form, "status"),
      target: formValue(form, "target")
    };

    try {
      const created = await data.createBackupJobMutation.mutateAsync(input);
      formElement.reset();
      setSelectedBackupJobId(created.id);
      showToast("Đã tạo backup job mới.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function handleRunBackupJob(job: BackupJob) {
    try {
      const run = await data.runBackupJobMutation.mutateAsync(job.id);
      setSelectedBackupJobId(job.id);
      showToast(`Đã tạo lượt backup ${shortId(run.id)}.`);
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  const selectedJob = data.backupJobs.find((job) => job.id === selectedBackupJobId) ?? null;

  return (
    <section className="admin-content-stack">
      {!data.canManageBackups ? <PermissionNotice permission="backup.manage" /> : null}
      <section className="admin-split-grid">
        <article className="admin-panel">
          <header>
            <div>
              <h2>Tạo backup job</h2>
              <p>Backend MVP hỗ trợ tạo job database backup và chạy thủ công từ UI.</p>
            </div>
            <Database size={20} />
          </header>
          <form className="admin-form" onSubmit={(event) => void handleCreateBackupJob(event)}>
            <label>
              Tên backup
              <input name="name" placeholder="Backup PostgreSQL hằng ngày" required />
            </label>
            <label>
              Target
              <select defaultValue="local" name="target">
                {backupTargets.map((target) => (
                  <option key={target.value} value={target.value}>
                    {target.label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Loại backup
              <select defaultValue="database" name="backup_type">
                <option value="database">Database</option>
              </select>
            </label>
            <label>
              Lịch chạy
              <input name="schedule" placeholder="0 3 * * * hoặc bỏ trống" />
            </label>
            <label>
              Trạng thái
              <select defaultValue="active" name="status">
                {backupStatuses.map((status) => (
                  <option key={status.value} value={status.value}>
                    {status.label}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Config JSON
              <textarea defaultValue="{}" name="config" rows={6} />
            </label>
            <Button disabled={data.createBackupJobMutation.isPending || !data.canManageBackups} type="submit">
              <Plus size={16} />
              Tạo backup
            </Button>
          </form>
        </article>

        <article className="admin-panel">
          <header>
            <div>
              <h2>Backup job</h2>
              <p>Danh sách job từ `/backup-jobs`; API hiện hỗ trợ tạo và chạy thủ công.</p>
            </div>
          </header>
          <BackupJobTable
            canManage={data.canManageBackups}
            isError={data.backupJobsQuery.isError}
            isLoading={data.backupJobsQuery.isLoading}
            jobs={data.backupJobs}
            mutationPending={data.runBackupJobMutation.isPending}
            onRun={(job) => void handleRunBackupJob(job)}
            onSelect={setSelectedBackupJobId}
            selectedId={selectedBackupJobId}
          />
        </article>
      </section>

      <article className="admin-panel">
        <header>
          <div>
            <h2>Lịch sử backup</h2>
            <p>{selectedJob ? `Đang xem ${selectedJob.name}.` : "Chọn một backup job để xem lịch sử chạy."}</p>
          </div>
          <Badge tone={selectedJob ? statusTone(selectedJob.status) : "slate"}>
            {selectedJob?.status ?? "Chưa chọn"}
          </Badge>
        </header>
        <BackupRunsTable
          isLoading={data.backupRunsQuery.isLoading}
          runs={data.backupRuns}
          selectedJob={selectedJob}
        />
      </article>
    </section>
  );
}

function CronJobTable({
  canManage,
  isError,
  isLoading,
  jobs,
  mutationPending,
  onDelete,
  onRun,
  onSelect,
  onStatusChange,
  selectedId
}: {
  canManage: boolean;
  isError: boolean;
  isLoading: boolean;
  jobs: CronJob[];
  mutationPending: boolean;
  onDelete: (job: CronJob) => void;
  onRun: (job: CronJob) => void;
  onSelect: (value: string) => void;
  onStatusChange: (job: CronJob, status: string) => void;
  selectedId: string;
}) {
  if (isLoading) {
    return <TableSkeleton />;
  }

  if (isError) {
    return <ErrorState description="Không thể tải danh sách cronjob từ backend." title="Lỗi tải cronjob" />;
  }

  if (!jobs.length) {
    return <EmptyState description="Workspace chưa có cronjob nào." title="Chưa có cronjob" />;
  }

  return (
    <div className="data-table data-table--cronjobs" role="table">
      <div className="data-table__row data-table__row--head" role="row">
        <span>Cronjob</span>
        <span>Lịch</span>
        <span>Runner</span>
        <span>Trạng thái</span>
        <span>Thao tác</span>
      </div>
      {jobs.map((job) => {
        const nextStatus = job.status === "active" ? "paused" : "active";

        return (
          <div className="data-table__row" key={job.id} role="row">
            <span>
              <strong>{job.name}</strong>
              <small>{job.description ?? `Payload ${formatJsonPreview(job.payload)}`}</small>
            </span>
            <span>
              <strong>{job.schedule}</strong>
              <small>Tiếp theo {formatDateTime(job.next_run_at)}</small>
            </span>
            <span>{job.runner}</span>
            <span>
              <Badge tone={statusTone(job.status)}>{job.status}</Badge>
            </span>
            <span className="row-actions">
              <Button onClick={() => onSelect(job.id)} size="sm" variant={selectedId === job.id ? "secondary" : "ghost"}>
                Chọn
              </Button>
              <Button disabled={!canManage || mutationPending} onClick={() => onRun(job)} size="sm" variant="secondary">
                <Zap size={15} />
                Chạy
              </Button>
              <Button
                disabled={!canManage || mutationPending}
                onClick={() => onStatusChange(job, nextStatus)}
                size="sm"
                variant="ghost"
              >
                {nextStatus === "active" ? "Bật" : "Tạm dừng"}
              </Button>
              <Button disabled={!canManage || mutationPending} onClick={() => onDelete(job)} size="sm" variant="ghost">
                <Trash2 size={15} />
              </Button>
            </span>
          </div>
        );
      })}
    </div>
  );
}

function CronJobRunsTable({
  isLoading,
  runs,
  selectedJob
}: {
  isLoading: boolean;
  runs: CronJobRun[];
  selectedJob: CronJob | null;
}) {
  if (!selectedJob) {
    return <EmptyState description="Chọn cronjob ở bảng bên trên để xem lịch sử chạy." title="Chưa chọn cronjob" />;
  }

  if (isLoading) {
    return <TableSkeleton />;
  }

  if (!runs.length) {
    return <EmptyState description="Cronjob này chưa có lần chạy nào." title="Chưa có run history" />;
  }

  return (
    <div className="data-table data-table--runs" role="table">
      <div className="data-table__row data-table__row--head" role="row">
        <span>Trạng thái</span>
        <span>Bắt đầu</span>
        <span>Kết thúc</span>
        <span>Thời lượng</span>
        <span>Log</span>
      </div>
      {runs.map((run) => (
        <div className="data-table__row" key={run.id} role="row">
          <span>
            <Badge tone={statusTone(run.status)}>{run.status}</Badge>
          </span>
          <span>{formatDateTime(run.started_at)}</span>
          <span>{formatDateTime(run.finished_at)}</span>
          <span>{formatDuration(run.duration_ms)}</span>
          <span className="log-snippet">{run.error ?? run.log ?? "Chưa có log"}</span>
        </div>
      ))}
    </div>
  );
}

function BackupJobTable({
  canManage,
  isError,
  isLoading,
  jobs,
  mutationPending,
  onRun,
  onSelect,
  selectedId
}: {
  canManage: boolean;
  isError: boolean;
  isLoading: boolean;
  jobs: BackupJob[];
  mutationPending: boolean;
  onRun: (job: BackupJob) => void;
  onSelect: (value: string) => void;
  selectedId: string;
}) {
  if (isLoading) {
    return <TableSkeleton />;
  }

  if (isError) {
    return <ErrorState description="Không thể tải danh sách backup job từ backend." title="Lỗi tải backup" />;
  }

  if (!jobs.length) {
    return <EmptyState description="Workspace chưa có backup job nào." title="Chưa có backup" />;
  }

  return (
    <div className="data-table data-table--backups" role="table">
      <div className="data-table__row data-table__row--head" role="row">
        <span>Backup</span>
        <span>Target</span>
        <span>Loại</span>
        <span>Trạng thái</span>
        <span>Thao tác</span>
      </div>
      {jobs.map((job) => (
        <div className="data-table__row" key={job.id} role="row">
          <span>
            <strong>{job.name}</strong>
            <small>{job.schedule ? `Lịch ${job.schedule}` : "Chạy thủ công"}</small>
          </span>
          <span>{job.target}</span>
          <span>{job.backup_type}</span>
          <span>
            <Badge tone={statusTone(job.status)}>{job.status}</Badge>
          </span>
          <span className="row-actions">
            <Button onClick={() => onSelect(job.id)} size="sm" variant={selectedId === job.id ? "secondary" : "ghost"}>
              Chọn
            </Button>
            <Button disabled={!canManage || mutationPending} onClick={() => onRun(job)} size="sm" variant="secondary">
              <Zap size={15} />
              Chạy
            </Button>
          </span>
        </div>
      ))}
    </div>
  );
}

function BackupRunsTable({
  isLoading,
  runs,
  selectedJob
}: {
  isLoading: boolean;
  runs: BackupRun[];
  selectedJob: BackupJob | null;
}) {
  if (!selectedJob) {
    return <EmptyState description="Chọn backup job ở bảng bên trên để xem lịch sử chạy." title="Chưa chọn backup" />;
  }

  if (isLoading) {
    return <TableSkeleton />;
  }

  if (!runs.length) {
    return <EmptyState description="Backup job này chưa có lần chạy nào." title="Chưa có run history" />;
  }

  return (
    <div className="data-table data-table--runs" role="table">
      <div className="data-table__row data-table__row--head" role="row">
        <span>Trạng thái</span>
        <span>Bắt đầu</span>
        <span>Kết thúc</span>
        <span>Dung lượng</span>
        <span>Kết quả</span>
      </div>
      {runs.map((run) => (
        <div className="data-table__row" key={run.id} role="row">
          <span>
            <Badge tone={statusTone(run.status)}>{run.status}</Badge>
          </span>
          <span>{formatDateTime(run.started_at)}</span>
          <span>{formatDateTime(run.finished_at)}</span>
          <span>{formatBytes(run.byte_size)}</span>
          <span className="log-snippet">{run.error ?? run.object_key ?? run.checksum_sha256 ?? "Chưa có kết quả"}</span>
        </div>
      ))}
    </div>
  );
}

function AuditPanel({ compact = false, data }: { compact?: boolean; data: DashboardData }) {
  return (
    <article className="admin-panel">
      <header>
        <div>
          <h2>Audit log</h2>
          <p>Theo dõi hành động quản trị trong workspace.</p>
        </div>
        <Badge tone={data.canViewAudit ? "green" : "orange"}>{data.canViewAudit ? "audit.view" : "Thiếu quyền"}</Badge>
      </header>
      {!data.canViewAudit ? <PermissionNotice permission="audit.view" /> : null}
      {data.auditLogsQuery.isLoading ? (
        <TableSkeleton />
      ) : data.auditLogs.length ? (
        <div className={compact ? "data-table data-table--audit data-table--compact" : "data-table data-table--audit"} role="table">
          <div className="data-table__row data-table__row--head" role="row">
            <span>Thời gian</span>
            <span>Action</span>
            <span>Entity</span>
            <span>Actor</span>
          </div>
          {data.auditLogs.slice(0, compact ? 6 : 50).map((log) => (
            <div className="data-table__row" key={log.id} role="row">
              <span>{formatDateTime(log.created_at)}</span>
              <span>{log.action}</span>
              <span>
                {log.entity_type}
                {log.entity_id ? ` / ${shortId(log.entity_id)}` : ""}
              </span>
              <span>{log.actor_user_id ? shortId(log.actor_user_id) : "Hệ thống"}</span>
            </div>
          ))}
        </div>
      ) : (
        <EmptyState description="Backend chưa trả về audit log hoặc workspace chưa có hoạt động." title="Chưa có audit log" />
      )}
    </article>
  );
}

function SettingsPanel({
  data,
  healthChecks,
  onOpenSettings
}: {
  data: DashboardData;
  healthChecks: Array<{ name: string; value: string }>;
  onOpenSettings: () => void;
}) {
  return (
    <aside className="settings-panel" aria-label="Cấu hình nhanh">
      <header>
        <div>
          <h2>Trạng thái hệ thống</h2>
          <p>Tóm tắt workspace và dịch vụ.</p>
        </div>
        <Settings size={20} />
      </header>

      <section className="system-summary-card">
        <div>
          <span>Workspace</span>
          <strong>{data.selectedWorkspace?.name ?? "Chưa có workspace"}</strong>
          <small>{data.selectedWorkspace?.slug ?? "Không có slug"}</small>
        </div>
        <div>
          <span>Backend</span>
          {data.healthQuery.isLoading ? (
            <Skeleton />
          ) : data.healthQuery.isError ? (
            <strong className="system-status system-status--error">Lỗi</strong>
          ) : (
            <strong className="system-status system-status--ready">
              {data.healthQuery.data?.status ?? "Không rõ"}
            </strong>
          )}
        </div>
      </section>

      <HealthGrid healthChecks={healthChecks} />
      <Button onClick={onOpenSettings} size="sm" variant="secondary">Mở cấu hình đầy đủ</Button>
    </aside>
  );
}

function SystemSettingsSection({
  data,
  healthChecks,
  showToast
}: {
  data: DashboardData;
  healthChecks: Array<{ name: string; value: string }>;
  showToast: (message: string, tone?: ToastTone) => void;
}) {
  async function handleUpsertSetting(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const key = formValue(form, "key").trim();
    const rawValue = formValue(form, "value").trim();
    if (!key || !rawValue) {
      showToast("Key và JSON value là bắt buộc.", "danger");
      return;
    }
    try {
      const value = JSON.parse(rawValue) as unknown;
      await data.upsertWorkspaceSettingMutation.mutateAsync({
        input: {
          description: formValue(form, "description") || undefined,
          value,
          value_type: formValue(form, "value_type") || "json"
        },
        key
      });
      event.currentTarget.reset();
      showToast(`Đã lưu setting ${key}.`);
    } catch (error) {
      showToast(error instanceof SyntaxError ? "Value phải là JSON hợp lệ." : errorMessage(error), "danger");
    }
  }

  return (
    <section className="admin-content-stack">
      <article className="admin-panel">
        <header>
          <div>
            <h2>Health check</h2>
            <p>Trạng thái phụ thuộc hệ thống lấy từ admin health API.</p>
          </div>
          <Database size={20} />
        </header>
        <HealthGrid healthChecks={healthChecks} />
      </article>
      <article className="admin-panel">
        <header>
          <div>
            <h2>Thiết lập workspace</h2>
            <p>Danh sách setting hiện có từ backend.</p>
          </div>
        </header>
        <form className="admin-form admin-form--inline" onSubmit={(event) => void handleUpsertSetting(event)}>
          <label>Key<input name="key" placeholder="chat.retention_days" required /></label>
          <label>JSON value<input name="value" placeholder="30 hoặc true hoặc {&quot;enabled&quot;:true}" required /></label>
          <label>Kiểu<select defaultValue="json" name="value_type"><option value="json">JSON</option><option value="string">String</option><option value="number">Number</option><option value="boolean">Boolean</option></select></label>
          <label>Mô tả<input name="description" placeholder="Mô tả cấu hình" /></label>
          <Button disabled={!data.canManageWorkspace || data.upsertWorkspaceSettingMutation.isPending} type="submit">Lưu setting</Button>
        </form>
        <WorkspaceSettingsList settings={data.settings} isLoading={data.settingsQuery.isLoading} />
      </article>
    </section>
  );
}

function InstanceAdministration({
  data,
  showToast
}: {
  data: DashboardData;
  showToast: (message: string, tone?: ToastTone) => void;
}) {
  const zoneOverview = data.currentZone;
  const quotaOverview = data.zoneQuota;
  const [editingOIDCProviderID, setEditingOIDCProviderID] = useState<string | null>(null);

  async function updateZone(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    try {
      await data.updateCurrentZoneMutation.mutateAsync({
        name: formValue(form, "name"),
        registration_mode: formValue(form, "registration_mode") as
          | "open"
          | "invite_only"
          | "closed"
      });
      showToast("Đã cập nhật zone.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function addDomain(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    try {
      await data.createAdditionalDomainMutation.mutateAsync({
        domain: formValue(form, "domain"),
        kind: formValue(form, "kind") as "alias" | "api" | "web"
      });
      formElement.reset();
      showToast("Đã tạo domain claim.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function changeLifecycle(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    try {
      await data.setZoneLifecycleMutation.mutateAsync({
        action: formValue(form, "action") as "suspend" | "resume" | "archive",
        reason: formValue(form, "reason") || undefined
      });
      showToast("Đã cập nhật vòng đời zone.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function requestDeployment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    try {
      await data.createDeploymentRequestMutation.mutateAsync({
        idempotency_key: crypto.randomUUID(),
        requested_database_mode: formValue(form, "requested_database_mode"),
        requested_mode: formValue(form, "requested_mode")
      });
      showToast("Đã gửi yêu cầu deployment.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function updateQuota(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    try {
      await data.updateZoneQuotaMutation.mutateAsync({
        enforcement_mode: formValue(form, "enforcement_mode") as "monitor" | "hard",
        max_automation_installations: Number(formValue(form, "max_automation_installations")),
        max_members: Number(formValue(form, "max_members")),
        max_storage_bytes: Number(formValue(form, "max_storage_bytes")),
        max_webhooks: Number(formValue(form, "max_webhooks")),
        max_workspaces: Number(formValue(form, "max_workspaces"))
      });
      showToast("Đã cập nhật quota.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function createOIDCProvider(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    try {
      await data.createOIDCProviderMutation.mutateAsync({
        client_id: formValue(form, "client_id"),
        client_secret_ref: formValue(form, "client_secret_ref") || undefined,
        claim_mapping: oidcClaimMappingFromForm(form),
        issuer_url: formValue(form, "issuer_url"),
        jit_provisioning: form.has("jit_provisioning"),
        name: formValue(form, "name"),
        require_verified_email: form.has("require_verified_email"),
        scopes: splitList(formValue(form, "scopes")),
        status: "configured"
      });
      formElement.reset();
      showToast("Đã lưu OIDC provider.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  async function updateOIDCProvider(
    event: FormEvent<HTMLFormElement>,
    provider: ZoneOIDCProvider
  ) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const secretRef = formValue(form, "client_secret_ref");
    try {
      await data.updateOIDCProviderMutation.mutateAsync({
        input: {
          client_id: formValue(form, "client_id"),
          claim_mapping: oidcClaimMappingFromForm(form),
          issuer_url: formValue(form, "issuer_url"),
          jit_provisioning: form.has("jit_provisioning"),
          name: formValue(form, "name"),
          require_verified_email: form.has("require_verified_email"),
          scopes: splitList(formValue(form, "scopes")),
          status: formValue(form, "status") as "configured" | "disabled",
          ...(form.has("clear_client_secret_ref")
            ? { client_secret_ref: "" }
            : secretRef
              ? { client_secret_ref: secretRef }
              : {})
        },
        providerId: provider.id
      });
      setEditingOIDCProviderID(null);
      showToast("Đã cập nhật OIDC provider.");
    } catch (error) {
      showToast(errorMessage(error), "danger");
    }
  }

  if (!data.canManageWorkspace) {
    return <PermissionNotice permission="workspace.manage" />;
  }

  if (data.currentZoneQuery.isLoading) {
    return <TableSkeleton />;
  }

  if (!zoneOverview) {
    return (
      <article className="admin-panel">
        <ErrorState
          description={errorMessage(data.currentZoneQuery.error)}
          title="Zone hiện tại không hoạt động"
        />
        <Button
          disabled={data.setZoneLifecycleMutation.isPending}
          onClick={async () => {
            try {
              await data.setZoneLifecycleMutation.mutateAsync({ action: "resume" });
              await data.currentZoneQuery.refetch();
              showToast("Đã resume zone.");
            } catch (error) {
              showToast(errorMessage(error), "danger");
            }
          }}
          variant="secondary"
        >
          Resume zone
        </Button>
      </article>
    );
  }

  const isSelfHosted = zoneOverview.zone.kind === "customer_dedicated";

  return (
    <>
      <section className="admin-split-grid">
        <article className="admin-panel">
          <header>
            <div>
              <h2>Zone</h2>
              <p>{zoneOverview.zone.slug}</p>
            </div>
            <Badge tone={zoneOverview.zone.status === "active" ? "green" : "slate"}>
              {zoneOverview.zone.status}
            </Badge>
          </header>
          <form className="admin-form" onSubmit={(event) => void updateZone(event)}>
            <label>
              Tên zone
              <input defaultValue={zoneOverview.zone.name} name="name" required />
            </label>
            <label>
              Đăng ký
              <select
                defaultValue={zoneOverview.zone.registration_mode}
                name="registration_mode"
              >
                <option value="open">Open</option>
                <option value="invite_only">Invite only</option>
                <option value="closed">Closed</option>
              </select>
            </label>
            <Button disabled={data.updateCurrentZoneMutation.isPending} type="submit">
              Lưu zone
            </Button>
          </form>
          {!isSelfHosted ? (
            <form className="admin-form admin-form--inline" onSubmit={(event) => void changeLifecycle(event)}>
              <label>
                Vòng đời
                <select
                  defaultValue={zoneOverview.zone.status === "suspended" ? "resume" : "suspend"}
                  name="action"
                >
                  <option value="suspend">Suspend</option>
                  <option value="resume">Resume</option>
                  <option value="archive">Archive</option>
                </select>
              </label>
              <label>
                Lý do
                <input name="reason" placeholder="Thay đổi vận hành" />
              </label>
              <Button disabled={data.setZoneLifecycleMutation.isPending} type="submit" variant="secondary">
                Áp dụng
              </Button>
            </form>
          ) : null}
        </article>

        <article className="admin-panel">
          <header>
            <div>
              <h2>Domain</h2>
              <p>{zoneOverview.domains.length} domain</p>
            </div>
          </header>
          {!isSelfHosted ? (
            <form className="admin-form admin-form--inline" onSubmit={(event) => void addDomain(event)}>
              <label>
                Domain
                <input name="domain" placeholder="chat.example.com" required />
              </label>
              <label>
                Loại
                <select defaultValue="alias" name="kind">
                  <option value="alias">Alias</option>
                  <option value="web">Web</option>
                  <option value="api">API</option>
                </select>
              </label>
              <Button disabled={data.createAdditionalDomainMutation.isPending} type="submit">
                <Plus size={16} />
                Thêm
              </Button>
            </form>
          ) : null}
          <div className="mini-list">
            {zoneOverview.domains.map((domain) => (
              <div key={domain.id}>
                <span>
                  <strong>{domain.domain}</strong>
                  <small>
                    {domain.kind} · {domain.status} · TLS {domain.tls_status}
                  </small>
                  {domain.verification_dns_value ? (
                    <small>{domain.verification_dns_name}: {domain.verification_dns_value}</small>
                  ) : null}
                </span>
                <span className="row-actions">
                  {!isSelfHosted && domain.status === "pending" ? (
                    <Button
                      onClick={() => void data.verifyZoneDomainMutation.mutateAsync(domain.id)}
                      size="sm"
                      variant="secondary"
                    >
                      Xác minh
                    </Button>
                  ) : null}
                  {!isSelfHosted && domain.status === "active" && domain.kind !== "primary" ? (
                    <Button
                      onClick={() => void data.setPrimaryDomainMutation.mutateAsync(domain.id)}
                      size="sm"
                      variant="ghost"
                    >
                      Primary
                    </Button>
                  ) : null}
                  {!isSelfHosted && domain.kind !== "primary" ? (
                    <Button
                      aria-label={`Xóa ${domain.domain}`}
                      onClick={() => void data.deleteZoneDomainMutation.mutateAsync(domain.id)}
                      size="sm"
                      variant="icon"
                    >
                      <Trash2 size={15} />
                    </Button>
                  ) : null}
                </span>
              </div>
            ))}
          </div>
        </article>
      </section>

      <section className="admin-split-grid">
        {!isSelfHosted ? (
          <article className="admin-panel">
          <header>
            <div>
              <h2>Deployment</h2>
              <p>{data.deploymentRequests.length} yêu cầu</p>
            </div>
          </header>
          <form className="admin-form" onSubmit={(event) => void requestDeployment(event)}>
            <label>
              Runtime
              <select defaultValue="shared" name="requested_mode">
                <option value="shared">Shared</option>
                <option value="dedicated_compose">Dedicated Compose</option>
                <option value="dedicated_k8s">Dedicated Kubernetes</option>
              </select>
            </label>
            <label>
              Database
              <select defaultValue="shared_schema" name="requested_database_mode">
                <option value="shared_schema">Shared schema</option>
                <option value="dedicated_schema">Dedicated schema</option>
                <option value="dedicated_database">Dedicated database</option>
              </select>
            </label>
            <Button disabled={data.createDeploymentRequestMutation.isPending} type="submit">
              Gửi yêu cầu
            </Button>
          </form>
          <div className="mini-list">
            {data.deploymentRequests.map((request) => (
              <div key={request.id}>
                <span>
                  <strong>{request.requested_mode}</strong>
                  <small>{request.requested_database_mode}</small>
                </span>
                <Badge tone={request.status === "ready" ? "green" : "slate"}>
                  {request.status}
                </Badge>
              </div>
            ))}
          </div>
          </article>
        ) : null}

        <article className="admin-panel">
          <header>
            <div>
              <h2>Quota</h2>
              <p>{quotaOverview?.quota.enforcement_mode ?? "..."}</p>
            </div>
          </header>
          {quotaOverview ? (
            <form
              className="admin-form"
              key={quotaOverview.quota.updated_at}
              onSubmit={(event) => void updateQuota(event)}
            >
              <label>Workspace<input defaultValue={quotaOverview.quota.max_workspaces} min={1} name="max_workspaces" type="number" /></label>
              <label>Thành viên<input defaultValue={quotaOverview.quota.max_members} min={1} name="max_members" type="number" /></label>
              <label>Storage bytes<input defaultValue={quotaOverview.quota.max_storage_bytes} min={1} name="max_storage_bytes" type="number" /></label>
              <label>Automation<input defaultValue={quotaOverview.quota.max_automation_installations} min={1} name="max_automation_installations" type="number" /></label>
              <label>Webhook<input defaultValue={quotaOverview.quota.max_webhooks} min={1} name="max_webhooks" type="number" /></label>
              <label>
                Chế độ
                <select defaultValue={quotaOverview.quota.enforcement_mode} name="enforcement_mode">
                  <option value="hard">Hard</option>
                  <option value="monitor">Monitor</option>
                </select>
              </label>
              <Button disabled={data.updateZoneQuotaMutation.isPending} type="submit">
                Lưu quota
              </Button>
            </form>
          ) : (
            <TableSkeleton />
          )}
          {quotaOverview ? (
            <div className="mini-list">
              <div><span>Workspace</span><strong>{quotaOverview.usage.workspaces}</strong></div>
              <div><span>Thành viên</span><strong>{quotaOverview.usage.members}</strong></div>
              <div><span>Storage</span><strong>{formatBytes(quotaOverview.usage.storage_bytes)}</strong></div>
              <div><span>Automation</span><strong>{quotaOverview.usage.automation_installations}</strong></div>
              <div><span>Webhook</span><strong>{quotaOverview.usage.webhooks}</strong></div>
            </div>
          ) : null}
        </article>
      </section>

      <article className="admin-panel">
        <header>
          <div>
            <h2>OIDC providers</h2>
            <p>{data.oidcProviders.length} provider</p>
          </div>
          <KeyRound size={20} />
        </header>
        <form className="admin-form admin-form--inline" onSubmit={(event) => void createOIDCProvider(event)}>
          <label>Tên<input name="name" placeholder="Company SSO" required /></label>
          <label>Issuer URL<input name="issuer_url" placeholder="https://id.example.com" required type="url" /></label>
          <label>Client ID<input name="client_id" required /></label>
          <label>Secret ref<input name="client_secret_ref" placeholder="env://company-sso" /></label>
          <label>Scopes<input defaultValue="openid,profile,email" name="scopes" /></label>
          <details className="oidc-claim-editor">
            <summary>Claim mapping</summary>
            <div className="oidc-claim-grid">
              <label>Subject<input defaultValue="sub" name="claim_subject" required /></label>
              <label>Email<input defaultValue="email" name="claim_email" required /></label>
              <label>Email verified<input defaultValue="email_verified" name="claim_email_verified" required /></label>
              <label>Username<input defaultValue="preferred_username" name="claim_username" required /></label>
              <label>Display name<input defaultValue="name" name="claim_display_name" required /></label>
              <label>Groups<input defaultValue="groups" name="claim_groups" required /></label>
            </div>
          </details>
          <label className="admin-check"><input defaultChecked name="jit_provisioning" type="checkbox" />JIT provisioning</label>
          <label className="admin-check"><input defaultChecked name="require_verified_email" type="checkbox" />Email đã xác minh</label>
          <Button disabled={data.createOIDCProviderMutation.isPending} type="submit">
            <Plus size={16} />
            Thêm provider
          </Button>
        </form>
        <div className="mini-list">
          {data.oidcProviders.map((provider) => (
            <div className="oidc-provider-entry" key={provider.id}>
              <div className="oidc-provider-summary">
                <span>
                  <strong>{provider.name}</strong>
                  <small>{provider.issuer_url}</small>
                  <small>{provider.status} · JIT {provider.jit_provisioning ? "bật" : "tắt"} · Verified email {provider.require_verified_email ? "bắt buộc" : "không bắt buộc"} · Secret ref {provider.has_client_secret_ref ? "đã cấu hình" : "không dùng"}</small>
                </span>
                <span className="row-actions">
                  <Button
                    onClick={() => setEditingOIDCProviderID(
                      editingOIDCProviderID === provider.id ? null : provider.id
                    )}
                    size="sm"
                    variant="ghost"
                  >
                    {editingOIDCProviderID === provider.id ? "Đóng" : "Sửa"}
                  </Button>
                  <Button
                    onClick={() =>
                      void data.updateOIDCProviderMutation.mutateAsync({
                        input: { status: provider.status === "configured" ? "disabled" : "configured" },
                        providerId: provider.id
                      })
                    }
                    size="sm"
                    variant="ghost"
                  >
                    {provider.status === "configured" ? "Tắt" : "Bật"}
                  </Button>
                  <Button
                    aria-label={`Xóa ${provider.name}`}
                    onClick={() => void data.deleteOIDCProviderMutation.mutateAsync(provider.id)}
                    size="sm"
                    variant="icon"
                  >
                    <Trash2 size={15} />
                  </Button>
                </span>
              </div>
              {editingOIDCProviderID === provider.id ? (
                <form
                  className="admin-form oidc-provider-form"
                  key={provider.updated_at}
                  onSubmit={(event) => void updateOIDCProvider(event, provider)}
                >
                  <label>Tên<input defaultValue={provider.name} name="name" required /></label>
                  <label>Issuer URL<input defaultValue={provider.issuer_url} name="issuer_url" required type="url" /></label>
                  <label>Client ID<input defaultValue={provider.client_id} name="client_id" required /></label>
                  <label>Scopes<input defaultValue={provider.scopes.join(",")} name="scopes" required /></label>
                  <label>
                    Secret ref mới
                    <input name="client_secret_ref" placeholder={provider.has_client_secret_ref ? "Giữ nguyên nếu để trống" : "env://company-sso"} />
                  </label>
                  <label>
                    Trạng thái
                    <select defaultValue={provider.status} name="status">
                      <option value="configured">Configured</option>
                      <option value="disabled">Disabled</option>
                    </select>
                  </label>
                  <div className="oidc-claim-grid">
                    <label>Subject<input defaultValue={oidcClaimName(provider, "subject", "sub")} name="claim_subject" required /></label>
                    <label>Email<input defaultValue={oidcClaimName(provider, "email", "email")} name="claim_email" required /></label>
                    <label>Email verified<input defaultValue={oidcClaimName(provider, "email_verified", "email_verified")} name="claim_email_verified" required /></label>
                    <label>Username<input defaultValue={oidcClaimName(provider, "username", "preferred_username")} name="claim_username" required /></label>
                    <label>Display name<input defaultValue={oidcClaimName(provider, "display_name", "name")} name="claim_display_name" required /></label>
                    <label>Groups<input defaultValue={oidcClaimName(provider, "groups", "groups")} name="claim_groups" required /></label>
                  </div>
                  <div className="oidc-policy-row">
                    <label className="admin-check"><input defaultChecked={provider.jit_provisioning} name="jit_provisioning" type="checkbox" />JIT provisioning</label>
                    <label className="admin-check"><input defaultChecked={provider.require_verified_email} name="require_verified_email" type="checkbox" />Email đã xác minh</label>
                    {provider.has_client_secret_ref ? (
                      <label className="admin-check"><input name="clear_client_secret_ref" type="checkbox" />Xóa secret ref</label>
                    ) : null}
                  </div>
                  <div className="row-actions">
                    <Button disabled={data.updateOIDCProviderMutation.isPending} type="submit">Lưu provider</Button>
                    <Button onClick={() => setEditingOIDCProviderID(null)} type="button" variant="secondary">Hủy</Button>
                  </div>
                </form>
              ) : null}
            </div>
          ))}
        </div>
      </article>
    </>
  );
}

function ScopeSelector({ scopes }: { scopes: ApiScope[] }) {
  if (!scopes.length) {
    return <EmptyState description="Backend chưa trả về API scope." title="Chưa có scope" />;
  }

  return (
    <fieldset className="scope-grid">
      <legend>Scope</legend>
      {scopes.map((scope) => (
        <label key={scope.code}>
          <input name="scopes" type="checkbox" value={scope.code} />
          <span>
            <strong>{scope.code}</strong>
            <small>{scope.name}</small>
          </span>
        </label>
      ))}
    </fieldset>
  );
}

function TokenTable({ onRevoke, tokens }: { onRevoke: (tokenId: string) => void; tokens: ApiToken[] }) {
  if (!tokens.length) {
    return <EmptyState description="Workspace chưa có API token nào." title="Chưa có token" />;
  }

  return (
    <div className="data-table data-table--tokens" role="table">
      <div className="data-table__row data-table__row--head" role="row">
        <span>Tên</span>
        <span>Scope</span>
        <span>Trạng thái</span>
        <span>Thao tác</span>
      </div>
      {tokens.map((token) => (
        <div className="data-table__row" key={token.id} role="row">
          <span>
            <strong>{token.name}</strong>
            <small>Tạo {formatDateTime(token.created_at)}</small>
          </span>
          <span>{token.scopes.map((scope) => scope.code).join(", ") || "Không có scope"}</span>
          <span>
            <Badge tone={token.status === "active" ? "green" : "slate"}>{token.status}</Badge>
          </span>
          <span>
            <Button onClick={() => onRevoke(token.id)} size="sm" variant="ghost">
              <Trash2 size={15} />
              Thu hồi
            </Button>
          </span>
        </div>
      ))}
    </div>
  );
}

function WebhookList({ incomingWebhooks, onDelete, onToggle }: { incomingWebhooks: IncomingWebhook[]; onDelete: (id: string) => void; onToggle: (webhook: IncomingWebhook) => void }) {
  if (!incomingWebhooks.length) {
    return <EmptyState description="Chưa có incoming webhook nào." title="Danh sách trống" />;
  }

  return (
    <div className="mini-list">
      <strong>Incoming webhook</strong>
      {incomingWebhooks.map((webhook) => (
        <div key={webhook.id}>
          <span>{webhook.name}</span>
          <Badge tone={webhook.status === "active" ? "green" : "slate"}>{webhook.status}</Badge>
          <Button onClick={() => onToggle(webhook)} size="sm" variant="ghost">{webhook.status === "active" ? "Tắt" : "Bật"}</Button>
          <Button onClick={() => onDelete(webhook.id)} size="sm" variant="ghost"><Trash2 size={14} /> Xóa</Button>
        </div>
      ))}
    </div>
  );
}

function OutgoingWebhookList({
  onDelete,
  onSelect,
  onToggle,
  selectedId,
  webhooks
}: {
  onDelete: (id: string) => void;
  onSelect: (value: string) => void;
  onToggle: (webhook: OutgoingWebhook) => void;
  selectedId: string;
  webhooks: OutgoingWebhook[];
}) {
  if (!webhooks.length) {
    return <EmptyState description="Chưa có outgoing webhook nào." title="Danh sách trống" />;
  }

  return (
    <div className="mini-list">
      <strong>Outgoing webhook</strong>
      {webhooks.map((webhook) => (
        <div className={webhook.id === selectedId ? "mini-list__button mini-list__button--active" : "mini-list__button"} key={webhook.id}>
          <button onClick={() => onSelect(webhook.id)} type="button">
          <span>
            <strong>{webhook.name}</strong>
            <small>{webhook.target_url}</small>
          </span>
          <Badge tone={webhook.status === "active" ? "green" : "slate"}>{webhook.status}</Badge>
          </button>
          <Button onClick={() => onToggle(webhook)} size="sm" variant="ghost">{webhook.status === "active" ? "Tắt" : "Bật"}</Button>
          <Button onClick={() => onDelete(webhook.id)} size="sm" variant="ghost"><Trash2 size={14} /> Xóa</Button>
        </div>
      ))}
    </div>
  );
}

function DeliveryPanel({ deliveries, isLoading, onTest, testDisabled }: { deliveries: WebhookDelivery[]; isLoading: boolean; onTest: () => void; testDisabled: boolean }) {
  return (
    <article className="admin-panel">
      <header>
        <div>
          <h2>Delivery logs</h2>
          <p>Theo dõi trạng thái gửi event của outgoing webhook đã chọn.</p>
        </div>
        <Button disabled={testDisabled} onClick={onTest} size="sm" variant="secondary"><Zap size={15} /> Gửi test</Button>
      </header>
      {isLoading ? (
        <TableSkeleton />
      ) : deliveries.length ? (
        <div className="data-table data-table--deliveries" role="table">
          <div className="data-table__row data-table__row--head" role="row">
            <span>Event</span>
            <span>Status</span>
            <span>HTTP</span>
            <span>Lần thử</span>
            <span>Cập nhật</span>
          </div>
          {deliveries.map((delivery) => (
            <div className="data-table__row" key={delivery.id} role="row">
              <span>{delivery.event_type}</span>
              <span>
                <Badge tone={delivery.status === "delivered" ? "green" : "orange"}>{delivery.status}</Badge>
              </span>
              <span>{delivery.response_status ?? "Chưa có"}</span>
              <span>{delivery.attempt_count}</span>
              <span>{formatDateTime(delivery.updated_at)}</span>
            </div>
          ))}
        </div>
      ) : (
        <EmptyState description="Chọn outgoing webhook hoặc chờ backend tạo delivery." title="Chưa có delivery log" />
      )}
    </article>
  );
}

function BotTable({
  bots,
  onSelect,
  selectedId
}: {
  bots: BotRecord[];
  onSelect: (value: string) => void;
  selectedId: string;
}) {
  if (!bots.length) {
    return <EmptyState description="Workspace chưa có bot nào." title="Chưa có bot" />;
  }

  return (
    <div className="data-table data-table--bots" role="table">
      <div className="data-table__row data-table__row--head" role="row">
        <span>Bot</span>
        <span>Slug</span>
        <span>Trạng thái</span>
        <span></span>
      </div>
      {bots.map((bot) => (
        <div className="data-table__row" key={bot.id} role="row">
          <span className="user-cell">
            <Avatar name={bot.name} size="sm" src={bot.avatar_url ?? undefined} />
            {bot.name}
          </span>
          <span>{bot.slug}</span>
          <span>
            <Badge tone={bot.status === "active" ? "green" : "slate"}>{bot.status}</Badge>
          </span>
          <span>
            <Button onClick={() => onSelect(bot.id)} size="sm" variant={selectedId === bot.id ? "secondary" : "ghost"}>
              Chọn
            </Button>
          </span>
        </div>
      ))}
    </div>
  );
}

function InstallationList({
  installations,
  isLoading
}: {
  installations: BotInstallation[];
  isLoading: boolean;
}) {
  if (isLoading) {
    return <Skeleton style={{ height: 92 }} />;
  }

  if (!installations.length) {
    return <EmptyState description="Bot đang chọn chưa có cài đặt nào." title="Chưa cài bot" />;
  }

  return (
    <div className="mini-list">
      <strong>Cài đặt hiện tại</strong>
      {installations.map((installation) => (
        <div key={installation.id}>
          <span>{installation.channel_id ? shortId(installation.channel_id) : "Toàn workspace"}</span>
          <Badge tone={installation.status === "active" ? "green" : "slate"}>{installation.status}</Badge>
        </div>
      ))}
    </div>
  );
}

function BotSelect({
  bots,
  name,
  selectedId
}: {
  bots: BotRecord[];
  name: string;
  selectedId: string;
}) {
  return (
    <select defaultValue={selectedId} name={name} required>
      <option value="">Chọn bot</option>
      {bots.map((bot) => (
        <option key={bot.id} value={bot.id}>
          {bot.name}
        </option>
      ))}
    </select>
  );
}

function ChannelSelect({
  channels,
  name,
  required = false
}: {
  channels: Channel[];
  name: string;
  required?: boolean;
}) {
  return (
    <select name={name} required={required}>
      <option value="">Chọn kênh</option>
      {channels.map((channel) => (
        <option key={channel.id} value={channel.id}>
          {channel.name ?? channel.slug ?? shortId(channel.id)}
        </option>
      ))}
    </select>
  );
}

function SecretBox({ label, value }: { label: string; value: string }) {
  return (
    <div className="secret-box">
      <span>{label}</span>
      <code>{value}</code>
      <small>Giá trị này chỉ nên lưu ở nơi an toàn và không hiển thị lại sau khi rời màn hình.</small>
    </div>
  );
}

function HealthGrid({ healthChecks }: { healthChecks: Array<{ name: string; value: string }> }) {
  return (
    <section className="config-grid">
      {healthChecks.length ? (
        healthChecks.map((check) => (
          <div key={check.name}>
            <Database size={18} />
            <span>{check.name}</span>
            <strong>{check.value}</strong>
          </div>
        ))
      ) : (
        <div className="config-grid__empty">
          <span>Health checks</span>
          <strong>Chưa có dữ liệu</strong>
        </div>
      )}
    </section>
  );
}

function WorkspaceSettingsList({
  isLoading,
  settings
}: {
  isLoading: boolean;
  settings: WorkspaceSetting[];
}) {
  return (
    <section className="config-block">
      <span>Thiết lập workspace</span>
      {isLoading ? (
        <Skeleton />
      ) : settings.length ? (
        <div className="settings-list">
          {settings.map((setting) => (
            <div key={setting.key}>
              <strong>{setting.key}</strong>
              <small>{formatSettingValue(setting.value)}</small>
            </div>
          ))}
        </div>
      ) : (
        <small>Backend chưa trả về thiết lập nào cho workspace này.</small>
      )}
    </section>
  );
}

function PermissionNotice({ permission }: { permission: string }) {
  return (
    <div className="permission-notice">
      <ShieldCheck size={17} />
      Cần quyền <code>{permission}</code> để dùng thao tác này.
    </div>
  );
}

function MetricSkeleton() {
  return (
    <>
      <Skeleton style={{ height: 112 }} />
      <Skeleton style={{ height: 112 }} />
      <Skeleton style={{ height: 112 }} />
      <Skeleton style={{ height: 112 }} />
    </>
  );
}

function TableSkeleton() {
  return (
    <div className="table-skeleton">
      <Skeleton style={{ height: 42 }} />
      <Skeleton style={{ height: 58 }} />
      <Skeleton style={{ height: 58 }} />
      <Skeleton style={{ height: 58 }} />
    </div>
  );
}

function cronJobToInput(job: CronJob, status: string): SaveCronJobInput {
  return {
    description: job.description ?? undefined,
    name: job.name,
    payload: ensureJsonObject(job.payload),
    runner: job.runner,
    schedule: job.schedule,
    status
  };
}

function ensureJsonObject(value: unknown): JsonObject {
  return isJsonObject(value) ? value : {};
}

function parseJsonObject(
  rawValue: string,
  message: string
): { ok: true; value: JsonObject } | { message: string; ok: false } {
  const trimmed = rawValue.trim();

  if (!trimmed) {
    return { ok: true, value: {} };
  }

  try {
    const parsed = JSON.parse(trimmed) as unknown;

    if (!isJsonObject(parsed)) {
      return { message, ok: false };
    }

    return { ok: true, value: parsed };
  } catch {
    return { message, ok: false };
  }
}

function isJsonObject(value: unknown): value is JsonObject {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function statusTone(status?: string | null): BadgeProps["tone"] {
  if (status === "active" || status === "success" || status === "ready" || status === "delivered") {
    return "green";
  }

  if (status === "failed" || status === "disabled" || status === "cancelled") {
    return "red";
  }

  if (status === "paused" || status === "running" || status === "pending") {
    return "orange";
  }

  return "slate";
}

function mapProfile(user: AuthUser | null) {
  return {
    name: displayName(user),
    src: user?.avatar_url ?? undefined,
    status: "online" as const
  };
}

function mapAdminUser(user: AuthUser): AdminUser {
  const raw = user as AuthUser & Record<string, unknown>;
  const status = String(raw.status ?? "").toLowerCase();

  return {
    department: stringValue(raw.department_name ?? raw.department, "Chưa phân phòng"),
    email: user.email,
    id: user.id,
    name: displayName(user),
    role: stringValue(raw.role_name ?? raw.role, "Chưa gán"),
    status: status === "blocked" || status === "disabled" ? "blocked" : "active"
  };
}

function mapMetrics(stats?: AdminStats): DashboardMetric[] {
  const updatedAt = stats?.generated_at ? `Cập nhật ${formatDateTime(stats.generated_at)}` : "Từ API admin stats";

  return [
    {
      delta: updatedAt,
      label: "Thành viên hoạt động",
      tone: "blue",
      value: formatNumber(stats?.active_members ?? stats?.users_count ?? stats?.user_count ?? 0)
    },
    {
      delta: updatedAt,
      label: "Kênh",
      tone: "green",
      value: formatNumber(stats?.channels ?? stats?.channels_count ?? stats?.channel_count ?? 0)
    },
    {
      delta: updatedAt,
      label: "Tin nhắn",
      tone: "orange",
      value: formatNumber(stats?.messages ?? stats?.messages_count ?? stats?.message_count ?? 0)
    },
    {
      delta: updatedAt,
      label: "File",
      tone: "purple",
      value: formatNumber(stats?.files ?? stats?.files_count ?? stats?.file_count ?? 0)
    }
  ];
}

function mapActivityBars(stats?: AdminStats): number[] {
  const activity = stats?.activity ?? [];

  if (!activity.length) {
    return [];
  }

  const values = activity.map((item) => item.messages ?? item.users ?? 0);
  const max = Math.max(...values, 1);
  return values.map((value) => Math.max(12, Math.round((value / max) * 100)));
}

function mapChannelRanks(stats?: AdminStats): ChannelRank[] {
  const tones: ChannelRank["tone"][] = ["blue", "green", "red", "orange", "purple"];

  return (stats?.channel_ranks ?? []).map((channel, index) => ({
    count: formatNumber(channel.messages_count ?? channel.count ?? 0),
    id: channel.id ?? channel.channel_id ?? `${channel.name}-${index}`,
    name: channel.name ?? "Kênh chưa đặt tên",
    tone: tones[index % tones.length]
  }));
}

function mapHealthChecks(health?: AdminHealth): Array<{ name: string; value: string }> {
  if (!health?.checks) {
    return [];
  }

  return Object.entries(health.checks).map(([name, value]) => ({
    name,
    value: typeof value === "string" ? value : JSON.stringify(value)
  }));
}

function memberDisplayName(member: WorkspaceMember): string {
  return member.display_name || member.username || member.email || shortId(member.user_id);
}

function displayName(user: Pick<AuthUser, "display_name" | "email" | "username"> | null): string {
  return user?.display_name || user?.username || user?.email || "Người dùng";
}

function stringValue(value: unknown, fallback: string): string {
  return typeof value === "string" && value.trim() ? value : fallback;
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat("vi-VN").format(value);
}

function percentage(value: number, total: number): string {
  return total ? ((value / total) * 100).toFixed(1) : "0";
}

function downloadCsv(filename: string, rows: Array<Array<number | string>>): void {
  const escapeCell = (value: number | string) => `"${String(value).replaceAll('"', '""')}"`;
  const content = `\uFEFF${rows.map((row) => row.map(escapeCell).join(",")).join("\r\n")}`;
  const url = URL.createObjectURL(new Blob([content], { type: "text/csv;charset=utf-8" }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `${filename}-${new Date().toISOString().slice(0, 10)}.csv`;
  anchor.click();
  URL.revokeObjectURL(url);
}

function formatDateTime(value?: string | null): string {
  if (!value) {
    return "Chưa có";
  }

  return new Intl.DateTimeFormat("vi-VN", {
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    month: "2-digit"
  }).format(new Date(value));
}

function formatJsonPreview(value?: JsonObject | null): string {
  const text = JSON.stringify(value ?? {});
  return text.length > 72 ? `${text.slice(0, 72)}...` : text;
}

function formatDuration(value?: number | null): string {
  if (typeof value !== "number") {
    return "Chưa có";
  }

  if (value < 1000) {
    return `${value} ms`;
  }

  return `${(value / 1000).toFixed(1)} giây`;
}

function formatBytes(value?: number | null): string {
  if (typeof value !== "number") {
    return "Chưa có";
  }

  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unitIndex = 0;

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }

  const rounded = size >= 10 || unitIndex === 0 ? size.toFixed(0) : size.toFixed(1);
  return `${rounded} ${units[unitIndex]}`;
}

function formatSettingValue(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }

  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }

  return JSON.stringify(value);
}

function shortId(value: string): string {
  return value.length > 10 ? `${value.slice(0, 8)}...` : value;
}

function formValue(form: FormData, key: string): string {
  const value = form.get(key);
  return typeof value === "string" ? value.trim() : "";
}

function formValues(form: FormData, key: string): string[] {
  return form.getAll(key).map(String).map((value) => value.trim()).filter(Boolean);
}

function splitList(value: string): string[] {
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}

function oidcClaimMappingFromForm(form: FormData): Record<string, string> {
  return {
    subject: formValue(form, "claim_subject"),
    email: formValue(form, "claim_email"),
    email_verified: formValue(form, "claim_email_verified"),
    username: formValue(form, "claim_username"),
    display_name: formValue(form, "claim_display_name"),
    groups: formValue(form, "claim_groups")
  };
}

function oidcClaimName(
  provider: ZoneOIDCProvider,
  key: string,
  fallback: string
): string {
  const value = provider.claim_mapping[key];
  return typeof value === "string" && value.trim() ? value.trim() : fallback;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Yêu cầu không thành công.";
}

function pickKnownId(current: string, ids: string[]): string {
  if (!ids.length) {
    return "";
  }

  return current && ids.includes(current) ? current : ids[0] ?? "";
}
