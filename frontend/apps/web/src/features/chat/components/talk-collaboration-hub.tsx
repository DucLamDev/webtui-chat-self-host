"use client";

import {
  type PointerEvent as ReactPointerEvent,
  useEffect,
  useMemo,
  useRef,
  useState
} from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { queryKeys } from "@webtui/api-client";
import {
  type BreakoutRoom,
  type ChannelMeeting,
  type ChannelMember,
  type ChannelTaskStatus,
  type CollaborationParticipantRole,
  type CollaborationRoomMode,
  type JsonObject,
  type TalkHome,
  type TalkIntegration,
  type TalkSummary
} from "@webtui/types";
import { Avatar, Badge, Button, EmptyState } from "@webtui/ui";
import {
  CheckCircle2,
  CalendarClock,
  ClipboardCheck,
  Clock3,
  ExternalLink,
  FileText,
  Globe2,
  Link,
  LockKeyhole,
  MicOff,
  Mic,
  Plus,
  ShieldCheck,
  Trash2,
  Users,
  Video,
  VideoOff,
  X
} from "@webtui/icons";
import { api } from "@/lib/api";

type HubChannel = {
  canManage?: boolean;
  id: string;
  memberCount: number;
  name: string;
  type?: string;
};

type WhiteboardPoint = { x: number; y: number };
type WhiteboardStroke = { color: string; points: WhiteboardPoint[] };

const collaborationTabs = [
  { id: "home", label: "Tổng quan" },
  { id: "shared", label: "Đã chia sẻ" },
  { id: "meeting", label: "Phòng họp" },
  { id: "notes", label: "Biên bản" },
  { id: "whiteboard", label: "Bảng trắng" },
  { id: "tasks", label: "Công việc" }
] as const;

type CollaborationTab = (typeof collaborationTabs)[number]["id"];

export function TalkCollaborationHub({
  channel,
  currentUser,
  members,
  onToast,
  workspaceId
}: {
  channel: HubChannel;
  currentUser: { id: string; name: string };
  members: ChannelMember[];
  onToast: (message: string) => void;
  workspaceId: string;
}) {
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState<CollaborationTab>(
    channel.type === "direct" ? "home" : "meeting"
  );
  const [meetingRoomKey, setMeetingRoomKey] = useState("");
  const [publicPassword, setPublicPassword] = useState("");
  const [publicRoomMode, setPublicRoomMode] = useState<"public" | "webinar">("public");
  const [publicUrl, setPublicUrl] = useState("");
  const [promoteName, setPromoteName] = useState(channel.name);
  const [departmentId, setDepartmentId] = useState("");

  useEffect(() => {
    setActiveTab(channel.type === "direct" ? "home" : "meeting");
  }, [channel.id, channel.type]);

  const settingsQuery = useQuery({
    enabled: Boolean(workspaceId && channel.id),
    queryFn: () => api.channels.collaborationSettings(workspaceId, channel.id),
    queryKey: queryKeys.channels.collaboration(workspaceId, channel.id)
  });
  const settings = settingsQuery.data;

  const rolesQuery = useQuery({
    enabled: Boolean(settings && channel.type !== "direct"),
    queryFn: () => api.channels.collaborationRoles(workspaceId, channel.id),
    queryKey: queryKeys.channels.collaborationRoles(workspaceId, channel.id)
  });
  const guestsQuery = useQuery({
    enabled: Boolean(settings?.public_access_enabled && channel.canManage),
    queryFn: () => api.channels.guestRequests(workspaceId, channel.id),
    queryKey: queryKeys.channels.guestRequests(workspaceId, channel.id),
    refetchInterval: 5_000
  });
  const breakoutsQuery = useQuery({
    enabled: Boolean(settings && channel.type !== "direct"),
    queryFn: () => api.channels.breakoutRooms(workspaceId, channel.id),
    queryKey: queryKeys.channels.breakoutRooms(workspaceId, channel.id)
  });
  const departmentsQuery = useQuery({
    enabled: Boolean(channel.canManage && channel.type !== "direct"),
    queryFn: () => api.departments.list(workspaceId),
    queryKey: queryKeys.departments.all(workspaceId)
  });

  const invalidateCollaboration = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.channels.collaboration(workspaceId, channel.id) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.channels.all(workspaceId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.channels.directConversations(workspaceId) })
    ]);
  };

  const promoteMutation = useMutation({
    mutationFn: () => api.channels.promoteConversation(workspaceId, channel.id, promoteName),
    onError: (error) => onToast(error instanceof Error ? error.message : "Không chuyển được thành phòng nhóm."),
    onSuccess: async () => {
      await invalidateCollaboration();
      onToast("Đã chuyển cuộc trò chuyện thành phòng nhóm nội bộ.");
    }
  });
  const settingsMutation = useMutation({
    mutationFn: (roomMode: CollaborationRoomMode) => {
      if (!settings) throw new Error("Chưa tải được cấu hình phòng.");
      return api.channels.updateCollaborationSettings(workspaceId, channel.id, {
        chat_locked: settings.chat_locked,
        default_participant_role: roomMode === "webinar" ? "listener" : settings.default_participant_role,
        guest_camera_enabled: settings.guest_camera_enabled,
        guest_microphone_enabled: settings.guest_microphone_enabled,
        lobby_enabled: settings.lobby_enabled,
        meeting_provider: settings.meeting_provider,
        room_mode: roomMode
      });
    },
    onError: (error) => onToast(error instanceof Error ? error.message : "Không lưu được chế độ phòng."),
    onSuccess: invalidateCollaboration
  });
  const publicLinkMutation = useMutation({
    mutationFn: () =>
      api.channels.createPublicLink(workspaceId, channel.id, {
        chat_locked: settings?.chat_locked ?? false,
        guest_camera_enabled: settings?.guest_camera_enabled ?? false,
        guest_microphone_enabled: settings?.guest_microphone_enabled ?? false,
        lobby_enabled: settings?.lobby_enabled ?? true,
        password: publicPassword || undefined,
        room_mode: publicRoomMode
      }),
    onError: (error) => onToast(error instanceof Error ? error.message : "Không tạo được link công khai."),
    onSuccess: async (link) => {
      const url = `${window.location.origin}/join/${link.token}`;
      setPublicUrl(url);
      await navigator.clipboard?.writeText(url).catch(() => undefined);
      await invalidateCollaboration();
      onToast("Đã tạo và sao chép link khách. Token chỉ hiển thị trong lần này.");
    }
  });
  const disablePublicLinkMutation = useMutation({
    mutationFn: () => api.channels.disablePublicLink(workspaceId, channel.id),
    onSuccess: async () => {
      setPublicUrl("");
      await invalidateCollaboration();
      onToast("Đã thu hồi link công khai.");
    }
  });
  const moderationMutation = useMutation({
    mutationFn: ({ action, id }: { action: "approve" | "reject"; id: string }) =>
      api.channels.moderateGuest(workspaceId, channel.id, id, action),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.channels.guestRequests(workspaceId, channel.id) })
  });
  const roleMutation = useMutation({
    mutationFn: ({ role, userId }: { role: CollaborationParticipantRole; userId: string }) =>
      api.channels.updateCollaborationRole(workspaceId, channel.id, userId, role),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.channels.collaborationRoles(workspaceId, channel.id) })
  });
  const importDepartmentMutation = useMutation({
    mutationFn: async () => {
      if (!departmentId) throw new Error("Hãy chọn một phòng ban.");
      const departmentMembers = await api.departments.members(workspaceId, departmentId);
      const existing = new Set(members.map((member) => member.user_id));
      const missing = departmentMembers.filter((member) => !existing.has(member.user_id));
      await Promise.all(
        missing.map((member) => api.channels.addMember(workspaceId, channel.id, { user_id: member.user_id }))
      );
      return missing.length;
    },
    onError: (error) => onToast(error instanceof Error ? error.message : "Không thêm được nhóm người dùng."),
    onSuccess: async (count) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.members(workspaceId, channel.id) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.collaborationRoles(workspaceId, channel.id) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.detail(workspaceId, channel.id) })
      ]);
      onToast(count ? `Đã thêm ${count} thành viên từ phòng ban.` : "Tất cả thành viên phòng ban đã có trong phòng.");
    }
  });

  if (settingsQuery.isLoading) {
    return <div className="talk-hub talk-hub--loading">Đang tải không gian cộng tác…</div>;
  }
  if (!settings) {
    return (
      <EmptyState
        description="Hãy chạy migration 000029 và tải lại trang."
        title="Chưa khởi tạo được phòng cộng tác"
      />
    );
  }

  return (
    <section className="talk-hub">
      <header className="talk-hub__hero">
        <span className="talk-hub__hero-icon"><Video size={20} /></span>
        <span>
          <strong>{channel.name}</strong>
          <small>
            {settings.room_mode === "webinar"
              ? "Hội thảo có diễn giả và khán giả"
              : settings.room_mode === "public"
                ? "Phòng có thể mời khách ngoài"
                : channel.type === "direct"
                  ? "Cuộc trò chuyện 1-1 riêng tư"
                  : "Phòng nhóm chỉ dành cho thành viên nội bộ"}
          </small>
        </span>
        <Badge tone={settings.room_mode === "internal" ? "green" : "blue"}>
          {settings.room_mode === "webinar" ? "WEBINAR" : settings.room_mode === "public" ? "PUBLIC" : "INTERNAL"}
        </Badge>
      </header>

      <nav className="talk-hub__tabs" aria-label="Công cụ cộng tác">
        {collaborationTabs.map((tab) => (
          <button
            className={activeTab === tab.id ? "is-active" : ""}
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            type="button"
          >
            {tab.label}
          </button>
        ))}
      </nav>

      {activeTab === "home" ? (
        <TalkHomePanel
          channelId={channel.id}
          onOpenMeeting={() => setMeetingRoomKey(settings.meeting_room_key ?? "")}
          workspaceId={workspaceId}
        />
      ) : null}

      {activeTab === "meeting" ? (
        <div className="talk-hub__panel">
          {channel.type === "direct" ? (
            <section className="talk-card talk-promote-card">
              <header><Users size={17} /><strong>Cuộc trò chuyện riêng tư 1-1</strong></header>
              <p>Chat, voice message, file, gọi thoại/video và screen share được giữ riêng tư. Muốn tạo link cho khách, hãy chuyển thành phòng nhóm.</p>
              {channel.canManage ? (
                <div className="talk-inline-form">
                  <input
                    aria-label="Tên phòng nhóm"
                    maxLength={120}
                    onChange={(event) => setPromoteName(event.target.value)}
                    value={promoteName}
                  />
                  <Button
                    disabled={promoteMutation.isPending || promoteName.trim().length < 2}
                    onClick={() => promoteMutation.mutate()}
                    size="sm"
                  >
                    Promote
                  </Button>
                </div>
              ) : null}
            </section>
          ) : (
            <>
              <section className="talk-card talk-meeting-card">
                <header>
                  <span><Video size={18} /></span>
                  <span><strong>Phòng họp nhóm</strong><small>{channel.memberCount} thành viên</small></span>
                </header>
                <div className="talk-capability-chips">
                  <span><Users size={13} /> Grid view</span>
                  <span><MicOff size={13} /> Giơ tay</span>
                  <span><VideoOff size={13} /> Blur nền</span>
                  <span><ExternalLink size={13} /> Screen share</span>
                </div>
                <Button onClick={() => setMeetingRoomKey(settings.meeting_room_key ?? "")}>
                  Vào phòng họp
                </Button>
                {!settings.meeting_base_url && !jitsiBaseUrl() ? (
                  <small className="talk-warning">Cần đặt NEXT_PUBLIC_JITSI_BASE_URL tới máy chủ Jitsi self-host.</small>
                ) : null}
              </section>

              {channel.canManage ? (
                <section className="talk-card talk-room-mode-card">
                  <header><ShieldCheck size={17} /><strong>Loại phòng và quyền truy cập</strong></header>
                  <div className="talk-mode-options">
                    {(["internal", "public", "webinar"] as const).map((mode) => (
                      <button
                        className={settings.room_mode === mode ? "is-active" : ""}
                        key={mode}
                        onClick={() => settingsMutation.mutate(mode)}
                        type="button"
                      >
                        {mode === "internal" ? "Nội bộ" : mode === "public" ? "Khách ngoài" : "Webinar"}
                      </button>
                    ))}
                  </div>
                  <MeetingPolicyEditor
                    onChange={(next) =>
                      api.channels
                        .updateCollaborationSettings(workspaceId, channel.id, next)
                        .then(invalidateCollaboration)
                        .catch((error) => onToast(error instanceof Error ? error.message : "Không lưu được chính sách."))
                    }
                    settings={settings}
                  />
                </section>
              ) : null}

              {channel.canManage && departmentsQuery.data?.length ? (
                <section className="talk-card talk-group-import-card">
                  <header><Users size={17} /><strong>Thêm theo nhóm người dùng</strong></header>
                  <p>Chọn phòng ban nội bộ để thêm nhanh toàn bộ tài khoản đang thuộc nhóm đó.</p>
                  <div className="talk-inline-form">
                    <select
                      aria-label="Phòng ban cần thêm"
                      onChange={(event) => setDepartmentId(event.target.value)}
                      value={departmentId}
                    >
                      <option value="">Chọn phòng ban</option>
                      {departmentsQuery.data.map((department) => (
                        <option key={department.id} value={department.id}>
                          {department.name} ({department.member_count ?? 0})
                        </option>
                      ))}
                    </select>
                    <Button
                      disabled={!departmentId || importDepartmentMutation.isPending}
                      onClick={() => importDepartmentMutation.mutate()}
                      size="sm"
                    >
                      Thêm thành viên
                    </Button>
                  </div>
                </section>
              ) : null}

              {channel.canManage ? (
                <section className="talk-card talk-public-link-card">
                  <header><Globe2 size={17} /><strong>Link khách ngoài</strong></header>
                  <div className="talk-grid-form">
                    <label>
                      Loại phiên
                      <select value={publicRoomMode} onChange={(event) => setPublicRoomMode(event.target.value as "public" | "webinar")}>
                        <option value="public">Phòng họp công khai</option>
                        <option value="webinar">Hội thảo</option>
                      </select>
                    </label>
                    <label>
                      Mật khẩu tùy chọn
                      <input
                        autoComplete="new-password"
                        minLength={8}
                        onChange={(event) => setPublicPassword(event.target.value)}
                        placeholder="Tối thiểu 8 ký tự"
                        type="password"
                        value={publicPassword}
                      />
                    </label>
                  </div>
                  <div className="talk-card__actions">
                    <Button
                      disabled={publicLinkMutation.isPending || Boolean(publicPassword && publicPassword.length < 8)}
                      onClick={() => publicLinkMutation.mutate()}
                      size="sm"
                    >
                      <Link size={14} /> {settings.public_access_enabled ? "Tạo link mới" : "Tạo link"}
                    </Button>
                    {settings.public_access_enabled ? (
                      <Button
                        disabled={disablePublicLinkMutation.isPending}
                        onClick={() => disablePublicLinkMutation.mutate()}
                        size="sm"
                        variant="ghost"
                      >
                        Thu hồi
                      </Button>
                    ) : null}
                  </div>
                  {publicUrl ? (
                    <button
                      className="talk-public-url"
                      onClick={() => navigator.clipboard?.writeText(publicUrl).then(() => onToast("Đã sao chép link."))}
                      type="button"
                    >
                      <LockKeyhole size={14} /><span>{publicUrl}</span>
                    </button>
                  ) : settings.public_access_enabled ? (
                    <small>Link đang hoạt động · mã {settings.public_token_prefix}… Muốn lấy URL mới hãy xoay token.</small>
                  ) : null}
                </section>
              ) : null}

              {channel.canManage && settings.lobby_enabled ? (
                <GuestLobby
                  guests={guestsQuery.data ?? []}
                  isPending={moderationMutation.isPending}
                  onModerate={(id, action) => moderationMutation.mutate({ action, id })}
                />
              ) : null}

              {settings.room_mode === "webinar" ? (
                <WebinarRoles
                  canManage={Boolean(channel.canManage)}
                  isPending={roleMutation.isPending}
                  onChange={(userId, role) => roleMutation.mutate({ role, userId })}
                  roles={rolesQuery.data ?? []}
                />
              ) : null}

              <BreakoutRoomsPanel
                canManage={Boolean(channel.canManage)}
                channelId={channel.id}
                members={members}
                onOpenRoom={setMeetingRoomKey}
                onToast={onToast}
                rooms={breakoutsQuery.data ?? []}
                workspaceId={workspaceId}
              />
              <MeetingLifecyclePanel
                canManage={Boolean(channel.canManage)}
                channelId={channel.id}
                currentUserId={currentUser.id}
                members={members}
                onOpenMeeting={() => setMeetingRoomKey(settings.meeting_room_key ?? "")}
                onToast={onToast}
                workspaceId={workspaceId}
              />
            </>
          )}
        </div>
      ) : null}

      {activeTab === "notes" ? (
        <CollaborativeNotes channelId={channel.id} onToast={onToast} workspaceId={workspaceId} />
      ) : null}
      {activeTab === "whiteboard" ? (
        <CollaborativeWhiteboard channelId={channel.id} onToast={onToast} workspaceId={workspaceId} />
      ) : null}
      {activeTab === "tasks" ? (
        <ChannelTasks channelId={channel.id} members={members} onToast={onToast} workspaceId={workspaceId} />
      ) : null}
      {activeTab === "shared" ? (
        <SharedItemsPanel channelId={channel.id} workspaceId={workspaceId} />
      ) : null}

      {meetingRoomKey ? (
        <JitsiMeetingOverlay
          chatLocked={settings.chat_locked && settings.room_mode !== "internal"}
          displayName={currentUser.name}
          meetingBaseUrl={settings.meeting_base_url}
          microphoneEnabled={settings.guest_microphone_enabled || settings.room_mode === "internal"}
          onClose={() => setMeetingRoomKey("")}
          participantRole={
            rolesQuery.data?.find((role) => role.user_id === currentUser.id)?.role
              ?? settings.default_participant_role
          }
          roomKey={meetingRoomKey}
          videoEnabled={settings.guest_camera_enabled || settings.room_mode === "internal"}
        />
      ) : null}
    </section>
  );
}

function TalkHomePanel({
  channelId,
  onOpenMeeting,
  workspaceId
}: {
  channelId: string;
  onOpenMeeting: () => void;
  workspaceId: string;
}) {
  const query = useQuery<TalkHome>({
    queryFn: () => api.channels.talkHome(workspaceId),
    queryKey: queryKeys.channels.talkHome(workspaceId),
    refetchInterval: 30_000
  });
  const integrationQuery = useQuery<TalkIntegration>({
    queryFn: () => api.channels.talkIntegration(workspaceId),
    queryKey: queryKeys.channels.talkIntegration(workspaceId)
  });
  const [summary, setSummary] = useState<TalkSummary | null>(null);
  const summaryMutation = useMutation({
    mutationFn: () => api.channels.summarizeChannel(workspaceId, channelId, {
      language: "vi",
      since: new Date(Date.now() - 7 * 24 * 60 * 60_000).toISOString()
    }),
    onSuccess: setSummary
  });
  const home = query.data;
  const channelMeetings = home?.upcoming_meetings.filter((meeting) => meeting.channel_id === channelId) ?? [];
  return (
    <div className="talk-hub__panel talk-home">
      <section className="talk-home__metrics" aria-label="Tổng quan công việc">
        <article><MessageMetric value={home?.unread_mentions ?? 0} /><span>Lượt nhắc chưa đọc</span></article>
        <article><Clock3 size={18} /><strong>{home?.pending_reminders ?? 0}</strong><span>Lời nhắc đang chờ</span></article>
        <article><VideoOff size={18} /><strong>{home?.missed_calls ?? 0}</strong><span>Cuộc gọi nhỡ</span></article>
        <article><ClipboardCheck size={18} /><strong>{home?.open_tasks.length ?? 0}</strong><span>Task đang mở</span></article>
      </section>
      <section className="talk-card talk-home__upcoming">
        <header><CalendarClock size={17} /><strong>Lịch sắp tới</strong><Badge tone="blue">{channelMeetings.length}</Badge></header>
        {channelMeetings.map((meeting) => (
          <button className="talk-home__meeting" key={meeting.id} onClick={onOpenMeeting} type="button">
            <span><strong>{meeting.title}</strong><small>{formatTalkDate(meeting.starts_at)}</small></span>
            <Badge tone={meeting.status === "active" ? "green" : "slate"}>
              {meeting.status === "active" ? "Đang diễn ra" : "Đã lên lịch"}
            </Badge>
          </button>
        ))}
        {!query.isLoading && !channelMeetings.length ? <p>Chưa có cuộc họp nào được lên lịch cho phòng này.</p> : null}
      </section>
      {home?.active_voice_rooms.some((room) => room.channel_id === channelId) ? (
        <section className="talk-card talk-home__voice">
          <header><Mic size={17} /><strong>Voice room đang hoạt động</strong><Badge tone="green">LIVE</Badge></header>
          <p>Thành viên trong phòng đang trò chuyện nhanh. Mở tab Phòng họp để tham gia.</p>
        </section>
      ) : null}
      {integrationQuery.data?.ai_enabled ? (
        <section className="talk-card talk-home__ai">
          <header>
            <ShieldCheck size={17} />
            <strong>AI nội bộ</strong>
            <Badge tone="green">{integrationQuery.data.ai_provider}</Badge>
          </header>
          {summary ? (
            <div className="talk-home__summary">
              <p>{summary.summary}</p>
              {summary.decisions.length ? (
                <div><strong>Quyết định</strong><ul>{summary.decisions.map((item) => <li key={item}>{item}</li>)}</ul></div>
              ) : null}
              {summary.action_items.length ? (
                <div><strong>Việc cần làm</strong><ul>{summary.action_items.map((item) => <li key={item}>{item}</li>)}</ul></div>
              ) : null}
              <small>{summary.message_count} tin nhắn · {summary.model}</small>
            </div>
          ) : (
            <p>Tóm tắt 7 ngày gần nhất bằng Ollama/LocalAI trên hạ tầng của tổ chức.</p>
          )}
          <Button
            disabled={summaryMutation.isPending}
            onClick={() => summaryMutation.mutate()}
            size="sm"
            type="button"
            variant="secondary"
          >
            {summaryMutation.isPending ? "Đang tóm tắt…" : summary ? "Tạo lại tóm tắt" : "Tóm tắt hội thoại"}
          </Button>
          {summaryMutation.isError ? <small>Không thể kết nối AI nội bộ. Kiểm tra cấu hình Talk Integration.</small> : null}
        </section>
      ) : null}
    </div>
  );
}

function MessageMetric({ value }: { value: number }) {
  return <><BellMetricIcon /><strong>{value}</strong></>;
}

function BellMetricIcon() {
  return <span aria-hidden="true" className="talk-home__metric-dot" />;
}

function MeetingLifecyclePanel({
  canManage,
  channelId,
  currentUserId,
  members,
  onOpenMeeting,
  onToast,
  workspaceId
}: {
  canManage: boolean;
  channelId: string;
  currentUserId: string;
  members: ChannelMember[];
  onOpenMeeting: () => void;
  onToast: (message: string) => void;
  workspaceId: string;
}) {
  const queryClient = useQueryClient();
  const [title, setTitle] = useState("");
  const [startsAt, setStartsAt] = useState(() => localDateTimeValue(new Date(Date.now() + 30 * 60_000)));
  const meetingsQuery = useQuery({
    queryFn: () => api.channels.meetings(workspaceId, channelId),
    queryKey: queryKeys.channels.meetings(workspaceId, channelId)
  });
  const voiceQuery = useQuery({
    queryFn: () => api.channels.voiceRoom(workspaceId, channelId),
    queryKey: queryKeys.channels.voiceRoom(workspaceId, channelId),
    refetchInterval: 10_000
  });
  const recordingPolicyQuery = useQuery({
    queryFn: () => api.channels.recordingPolicy(workspaceId, channelId),
    queryKey: queryKeys.channels.recordingPolicy(workspaceId, channelId)
  });
  const recordingsQuery = useQuery({
    queryFn: () => api.channels.recordings(workspaceId, channelId),
    queryKey: queryKeys.channels.recordings(workspaceId, channelId),
    refetchInterval: 10_000
  });
  const invalidateMeetings = () => Promise.all([
    queryClient.invalidateQueries({ queryKey: queryKeys.channels.meetings(workspaceId, channelId) }),
    queryClient.invalidateQueries({ queryKey: queryKeys.channels.talkHome(workspaceId) })
  ]);
  const createMeetingMutation = useMutation({
    mutationFn: () => api.channels.createMeeting(workspaceId, channelId, {
      starts_at: new Date(startsAt).toISOString(),
      title,
      room_policy: "keep"
    }),
    onError: (error) => onToast(error instanceof Error ? error.message : "Không tạo được lịch họp."),
    onSuccess: async () => {
      setTitle("");
      await invalidateMeetings();
      onToast("Đã lên lịch cuộc họp.");
    }
  });
  const transitionMeetingMutation = useMutation({
    mutationFn: ({ action, id }: { action: "start" | "end" | "cancel"; id: string }) =>
      api.channels.transitionMeeting(workspaceId, channelId, id, action),
    onSuccess: async (meeting, input) => {
      await invalidateMeetings();
      if (input.action === "start") onOpenMeeting();
      onToast(meeting.status === "active" ? "Cuộc họp đã bắt đầu." : "Đã cập nhật cuộc họp.");
    }
  });
  const voiceMutation = useMutation({
    mutationFn: (active: boolean) =>
      active
        ? api.channels.startVoiceRoom(workspaceId, channelId)
        : api.channels.stopVoiceRoom(workspaceId, channelId),
    onSuccess: async (room) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.voiceRoom(workspaceId, channelId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.channels.talkHome(workspaceId) })
      ]);
      if (room.status === "active") onOpenMeeting();
    }
  });
  const policyMutation = useMutation({
    mutationFn: (enabled: boolean) => {
      const current = recordingPolicyQuery.data;
      return api.channels.updateRecordingPolicy(workspaceId, channelId, {
        consent_required: current?.consent_required ?? true,
        enabled,
        provider: current?.provider ?? "jibri",
        retention_days: current?.retention_days ?? 30,
        summary_enabled: current?.summary_enabled ?? false,
        transcription_enabled: current?.transcription_enabled ?? false
      });
    },
    onSuccess: () => queryClient.invalidateQueries({
      queryKey: queryKeys.channels.recordingPolicy(workspaceId, channelId)
    })
  });
  const recordingMutation = useMutation({
    mutationFn: () => api.channels.startRecording(workspaceId, channelId, {
      participant_user_ids: members.map((member) => member.user_id)
    }),
    onError: (error) => onToast(error instanceof Error ? error.message : "Không bắt đầu được ghi hình."),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.channels.recordings(workspaceId, channelId) })
  });
  const consentMutation = useMutation({
    mutationFn: ({ consented, id }: { consented: boolean; id: string }) =>
      api.channels.setRecordingConsent(workspaceId, channelId, id, consented),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.channels.recordings(workspaceId, channelId) })
  });
  const stopRecordingMutation = useMutation({
    mutationFn: (id: string) => api.channels.stopRecording(workspaceId, channelId, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.channels.recordings(workspaceId, channelId) })
  });
  const activeRecording = recordingsQuery.data?.find((recording) =>
    recording.status === "pending" || recording.status === "recording"
  );
  const scheduled = meetingsQuery.data?.filter((meeting) =>
    meeting.status === "scheduled" || meeting.status === "active"
  ) ?? [];
  const voiceActive = voiceQuery.data?.status === "active";
  return (
    <>
      <section className="talk-card talk-schedule-card">
        <header><CalendarClock size={17} /><strong>Lịch và vòng đời phòng họp</strong></header>
        {canManage ? (
          <div className="talk-grid-form">
            <label>
              Tiêu đề
              <input maxLength={160} onChange={(event) => setTitle(event.target.value)} placeholder="Họp sprint tuần" value={title} />
            </label>
            <label>
              Bắt đầu
              <input min={localDateTimeValue(new Date())} onChange={(event) => setStartsAt(event.target.value)} type="datetime-local" value={startsAt} />
            </label>
            <Button disabled={!title.trim() || !startsAt || createMeetingMutation.isPending} onClick={() => createMeetingMutation.mutate()} size="sm">
              Lên lịch
            </Button>
          </div>
        ) : null}
        <div className="talk-meeting-list">
          {scheduled.map((meeting: ChannelMeeting) => (
            <article key={meeting.id}>
              <span><strong>{meeting.title}</strong><small>{formatTalkDate(meeting.starts_at)}</small></span>
              <div>
                {meeting.status === "active" ? <Button onClick={onOpenMeeting} size="sm">Vào họp</Button> : null}
                {canManage && meeting.status === "scheduled" ? (
                  <Button onClick={() => transitionMeetingMutation.mutate({ action: "start", id: meeting.id })} size="sm">Bắt đầu</Button>
                ) : null}
                {canManage && meeting.status === "active" ? (
                  <Button onClick={() => transitionMeetingMutation.mutate({ action: "end", id: meeting.id })} size="sm" variant="ghost">Kết thúc</Button>
                ) : null}
              </div>
            </article>
          ))}
        </div>
      </section>
      <section className="talk-card talk-voice-card">
        <header><Mic size={17} /><strong>Voice room</strong><Badge tone={voiceActive ? "green" : "slate"}>{voiceActive ? "LIVE" : "OFF"}</Badge></header>
        <p>Mở một phòng thoại nhanh cho team mà không cần lên lịch cuộc họp.</p>
        <Button disabled={voiceMutation.isPending} onClick={() => voiceMutation.mutate(!voiceActive)} size="sm">
          {voiceActive ? "Rời / đóng voice room" : "Bắt đầu voice room"}
        </Button>
      </section>
      <section className="talk-card talk-recording-card">
        <header><Video size={17} /><strong>Ghi hình và đồng thuận</strong></header>
        {!recordingPolicyQuery.data?.enabled ? (
          <>
            <p>Ghi hình mặc định tắt. Khi bật, hệ thống yêu cầu sự đồng thuận của người tham gia trước khi provider bắt đầu.</p>
            {canManage ? <Button onClick={() => policyMutation.mutate(true)} size="sm">Bật chính sách ghi hình</Button> : null}
          </>
        ) : activeRecording ? (
          <div className="talk-recording-status">
            <Badge tone={activeRecording.status === "recording" ? "green" : "blue"}>
              {activeRecording.status === "recording" ? "ĐANG GHI" : "CHỜ ĐỒNG THUẬN"}
            </Badge>
            <span>{activeRecording.consent_count}/{activeRecording.participant_count} đã đồng ý</span>
            {activeRecording.status === "pending" && activeRecording.participant_user_ids.includes(currentUserId) ? (
              <div>
                <Button onClick={() => consentMutation.mutate({ consented: true, id: activeRecording.id })} size="sm">Tôi đồng ý</Button>
                <Button onClick={() => consentMutation.mutate({ consented: false, id: activeRecording.id })} size="sm" variant="ghost">Từ chối</Button>
              </div>
            ) : null}
            {canManage && activeRecording.status === "recording" ? (
              <Button onClick={() => stopRecordingMutation.mutate(activeRecording.id)} size="sm" variant="ghost">Dừng ghi hình</Button>
            ) : null}
          </div>
        ) : canManage ? (
          <Button disabled={recordingMutation.isPending} onClick={() => recordingMutation.mutate()} size="sm">Yêu cầu ghi hình</Button>
        ) : <p>Chưa có phiên ghi hình đang hoạt động.</p>}
      </section>
    </>
  );
}

function SharedItemsPanel({ channelId, workspaceId }: { channelId: string; workspaceId: string }) {
  const [kind, setKind] = useState("all");
  const query = useQuery({
    queryFn: () => api.channels.sharedItems(workspaceId, channelId, { kind, limit: 100 }),
    queryKey: queryKeys.channels.sharedItems(workspaceId, channelId, kind)
  });
  return (
    <div className="talk-hub__panel talk-shared">
      <section className="talk-card">
        <header><FileText size={17} /><strong>Shared Items</strong><Badge tone="slate">{query.data?.length ?? 0}</Badge></header>
        <nav className="talk-shared__filters" aria-label="Lọc nội dung đã chia sẻ">
          {["all", "file", "pin", "poll", "task", "recording"].map((value) => (
            <button className={kind === value ? "is-active" : ""} key={value} onClick={() => setKind(value)} type="button">
              {value === "all" ? "Tất cả" : value}
            </button>
          ))}
        </nav>
        <div className="talk-shared__list">
          {query.data?.map((item) => (
            <a href={item.url || "#"} key={`${item.kind}-${item.id}`} onClick={(event) => !item.url && event.preventDefault()}>
              <span className="talk-shared__icon"><FileText size={16} /></span>
              <span><strong>{item.title}</strong><small>{item.subtitle || item.kind} · {formatTalkDate(item.created_at)}</small></span>
              <Badge tone="slate">{item.kind}</Badge>
            </a>
          ))}
          {!query.isLoading && !query.data?.length ? <p>Chưa có nội dung nào trong mục này.</p> : null}
        </div>
      </section>
    </div>
  );
}

function formatTalkDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : new Intl.DateTimeFormat("vi-VN", { dateStyle: "short", timeStyle: "short" }).format(date);
}

function localDateTimeValue(date: Date) {
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

function MeetingPolicyEditor({
  onChange,
  settings
}: {
  onChange: (value: {
    chat_locked: boolean;
    default_participant_role: CollaborationParticipantRole;
    guest_camera_enabled: boolean;
    guest_microphone_enabled: boolean;
    lobby_enabled: boolean;
    meeting_provider: "jitsi" | "webrtc";
    room_mode: CollaborationRoomMode;
  }) => void;
  settings: {
    chat_locked: boolean;
    default_participant_role: CollaborationParticipantRole;
    guest_camera_enabled: boolean;
    guest_microphone_enabled: boolean;
    lobby_enabled: boolean;
    meeting_provider: "jitsi" | "webrtc";
    room_mode: CollaborationRoomMode;
  };
}) {
  const update = (patch: Partial<typeof settings>) => onChange({ ...settings, ...patch });
  return (
    <div className="talk-policy-list">
      <label><span>Phòng chờ cho khách<small>Host phải duyệt trước khi lộ tên phòng media.</small></span><input checked={settings.lobby_enabled} onChange={(event) => update({ lobby_enabled: event.target.checked })} type="checkbox" /></label>
      <label><span>Khóa chat khách<small>Khách chỉ xem và đặt câu hỏi theo quyền webinar.</small></span><input checked={settings.chat_locked} onChange={(event) => update({ chat_locked: event.target.checked })} type="checkbox" /></label>
      <label><span>Bật mic khách khi vào<small>Nên tắt cho webinar và phòng đông.</small></span><input checked={settings.guest_microphone_enabled} onChange={(event) => update({ guest_microphone_enabled: event.target.checked })} type="checkbox" /></label>
      <label><span>Bật camera khách khi vào<small>Khách vẫn có thể bật lại nếu host cho phép.</small></span><input checked={settings.guest_camera_enabled} onChange={(event) => update({ guest_camera_enabled: event.target.checked })} type="checkbox" /></label>
    </div>
  );
}

function GuestLobby({
  guests,
  isPending,
  onModerate
}: {
  guests: Array<{ display_name: string; id: string; status: string }>;
  isPending: boolean;
  onModerate: (id: string, action: "approve" | "reject") => void;
}) {
  const waiting = guests.filter((guest) => guest.status === "waiting");
  return (
    <section className="talk-card talk-lobby-card">
      <header><Users size={17} /><strong>Phòng chờ</strong><Badge tone={waiting.length ? "orange" : "slate"}>{waiting.length}</Badge></header>
      {waiting.length ? waiting.map((guest) => (
        <div className="talk-lobby-row" key={guest.id}>
          <Avatar name={guest.display_name} size="sm" />
          <strong>{guest.display_name}</strong>
          <button disabled={isPending} onClick={() => onModerate(guest.id, "approve")} type="button"><CheckCircle2 size={15} /> Duyệt</button>
          <button disabled={isPending} onClick={() => onModerate(guest.id, "reject")} type="button"><X size={15} /></button>
        </div>
      )) : <p>Chưa có khách nào đang chờ.</p>}
    </section>
  );
}

function WebinarRoles({
  canManage,
  isPending,
  onChange,
  roles
}: {
  canManage: boolean;
  isPending: boolean;
  onChange: (userId: string, role: CollaborationParticipantRole) => void;
  roles: Array<{
    avatar_url?: string | null;
    display_name: string;
    role: CollaborationParticipantRole;
    user_id: string;
    username: string;
  }>;
}) {
  return (
    <section className="talk-card talk-role-card">
      <header><Users size={17} /><strong>Diễn giả và khán giả</strong></header>
      <div>
        {roles.map((member) => (
          <label key={member.user_id}>
            <Avatar name={member.display_name} size="sm" src={member.avatar_url ?? undefined} />
            <span><strong>{member.display_name}</strong><small>@{member.username}</small></span>
            <select
              disabled={!canManage || isPending}
              onChange={(event) => onChange(member.user_id, event.target.value as CollaborationParticipantRole)}
              value={member.role}
            >
              <option value="moderator">Chủ trì</option>
              <option value="presenter">Diễn giả</option>
              <option value="member">Thành viên</option>
              <option value="listener">Khán giả</option>
            </select>
          </label>
        ))}
      </div>
    </section>
  );
}

function CollaborativeNotes({
  channelId,
  onToast,
  workspaceId
}: {
  channelId: string;
  onToast: (message: string) => void;
  workspaceId: string;
}) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryFn: () => api.channels.collaborationDocument(workspaceId, channelId, "notes"),
    queryKey: queryKeys.channels.collaborationDocument(workspaceId, channelId, "notes"),
    refetchInterval: 8_000
  });
  const [text, setText] = useState("");
  const loadedVersion = useRef(0);
  useEffect(() => {
    if (!query.data || query.data.version === loadedVersion.current) return;
    loadedVersion.current = query.data.version;
    setText(typeof query.data.content.text === "string" ? query.data.content.text : "");
  }, [query.data]);
  const mutation = useMutation({
    mutationFn: () =>
      api.channels.updateCollaborationDocument(
        workspaceId,
        channelId,
        "notes",
        { text },
        query.data?.version ?? 1
      ),
    onError: (error) => {
      void query.refetch();
      onToast(error instanceof Error ? error.message : "Không lưu được biên bản.");
    },
    onSuccess: (document) => {
      queryClient.setQueryData(queryKeys.channels.collaborationDocument(workspaceId, channelId, "notes"), document);
      onToast("Đã lưu biên bản họp.");
    }
  });
  return (
    <section className="talk-document">
      <header><FileText size={18} /><span><strong>Collaborative Notes</strong><small>Version {query.data?.version ?? "…"}</small></span><Button disabled={mutation.isPending} onClick={() => mutation.mutate()} size="sm">Lưu</Button></header>
      <textarea
        aria-label="Biên bản họp"
        onChange={(event) => setText(event.target.value)}
        placeholder={"# Nội dung cuộc họp\n\n- Quyết định\n- Việc cần làm\n- Người phụ trách"}
        value={text}
      />
      <small>Dữ liệu được lưu trên server của tổ chức và dùng version để tránh ghi đè thay đổi mới hơn.</small>
    </section>
  );
}

function CollaborativeWhiteboard({
  channelId,
  onToast,
  workspaceId
}: {
  channelId: string;
  onToast: (message: string) => void;
  workspaceId: string;
}) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryFn: () => api.channels.collaborationDocument(workspaceId, channelId, "whiteboard"),
    queryKey: queryKeys.channels.collaborationDocument(workspaceId, channelId, "whiteboard"),
    refetchInterval: 5_000
  });
  const [strokes, setStrokes] = useState<WhiteboardStroke[]>([]);
  const [draftStroke, setDraftStroke] = useState<WhiteboardStroke | null>(null);
  const [color, setColor] = useState("#2563eb");
  const drawingRef = useRef<WhiteboardStroke | null>(null);
  const loadedVersion = useRef(0);
  useEffect(() => {
    if (!query.data || query.data.version === loadedVersion.current) return;
    loadedVersion.current = query.data.version;
    setStrokes(parseWhiteboardStrokes(query.data.content));
  }, [query.data]);
  const saveMutation = useMutation({
    mutationFn: (nextStrokes: WhiteboardStroke[]) =>
      api.channels.updateCollaborationDocument(
        workspaceId,
        channelId,
        "whiteboard",
        { strokes: nextStrokes } as unknown as JsonObject,
        query.data?.version ?? 1
      ),
    onError: (error) => {
      void query.refetch();
      onToast(error instanceof Error ? error.message : "Bảng trắng vừa thay đổi, đã tải lại bản mới.");
    },
    onSuccess: (document) =>
      queryClient.setQueryData(queryKeys.channels.collaborationDocument(workspaceId, channelId, "whiteboard"), document)
  });
  const pointerPoint = (event: ReactPointerEvent<SVGSVGElement>): WhiteboardPoint => {
    const box = event.currentTarget.getBoundingClientRect();
    return {
      x: Math.max(0, Math.min(800, ((event.clientX - box.left) / box.width) * 800)),
      y: Math.max(0, Math.min(420, ((event.clientY - box.top) / box.height) * 420))
    };
  };
  const start = (event: ReactPointerEvent<SVGSVGElement>) => {
    event.currentTarget.setPointerCapture(event.pointerId);
    drawingRef.current = { color, points: [pointerPoint(event)] };
    setDraftStroke(drawingRef.current);
  };
  const move = (event: ReactPointerEvent<SVGSVGElement>) => {
    if (!drawingRef.current) return;
    drawingRef.current = {
      ...drawingRef.current,
      points: [...drawingRef.current.points, pointerPoint(event)]
    };
    setDraftStroke(drawingRef.current);
  };
  const end = () => {
    const stroke = drawingRef.current;
    if (!stroke) return;
    drawingRef.current = null;
    setDraftStroke(null);
    setStrokes((current) => {
      const next = [...current, stroke];
      saveMutation.mutate(next);
      return next;
    });
  };
  return (
    <section className="talk-whiteboard">
      <header>
        <span><strong>Interactive Whiteboard</strong><small>{saveMutation.isPending ? "Đang đồng bộ…" : `Version ${query.data?.version ?? "…"}`}</small></span>
        <div>
          {["#2563eb", "#16a34a", "#ea580c", "#dc2626", "#111827"].map((value) => (
            <button
              aria-label={`Màu ${value}`}
              className={color === value ? "talk-color-swatch is-active" : "talk-color-swatch"}
              key={value}
              onClick={() => setColor(value)}
              style={{ background: value }}
              type="button"
            />
          ))}
          <Button
            onClick={() => {
              setStrokes([]);
              saveMutation.mutate([]);
            }}
            size="sm"
            variant="ghost"
          >
            <Trash2 size={14} /> Xóa
          </Button>
        </div>
      </header>
      <svg
        aria-label="Bảng trắng cộng tác"
        className="talk-whiteboard__canvas"
        onPointerDown={start}
        onPointerMove={move}
        onPointerUp={end}
        onPointerCancel={end}
        role="img"
        viewBox="0 0 800 420"
      >
        <defs><pattern height="24" id="whiteboard-grid" patternUnits="userSpaceOnUse" width="24"><circle cx="1" cy="1" fill="#dbe3ef" r="1" /></pattern></defs>
        <rect fill="url(#whiteboard-grid)" height="420" width="800" />
        {strokes.map((stroke, index) => (
          <polyline
            fill="none"
            key={`${index}-${stroke.points.length}`}
            points={stroke.points.map((point) => `${point.x},${point.y}`).join(" ")}
            stroke={stroke.color}
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth="4"
          />
        ))}
        {draftStroke ? (
          <polyline
            fill="none"
            points={draftStroke.points.map((point) => `${point.x},${point.y}`).join(" ")}
            stroke={draftStroke.color}
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth="4"
          />
        ) : null}
      </svg>
    </section>
  );
}

function ChannelTasks({
  channelId,
  members,
  onToast,
  workspaceId
}: {
  channelId: string;
  members: ChannelMember[];
  onToast: (message: string) => void;
  workspaceId: string;
}) {
  const queryClient = useQueryClient();
  const [title, setTitle] = useState("");
  const [assigneeUserId, setAssigneeUserId] = useState("");
  const query = useQuery({
    queryFn: () => api.channels.channelTasks(workspaceId, channelId),
    queryKey: queryKeys.channels.tasks(workspaceId, channelId)
  });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: queryKeys.channels.tasks(workspaceId, channelId) });
  const createMutation = useMutation({
    mutationFn: () => api.channels.createChannelTask(workspaceId, channelId, {
      assignee_user_id: assigneeUserId || undefined,
      title
    }),
    onSuccess: async () => {
      setTitle("");
      await invalidate();
      onToast("Đã tạo task trong phòng.");
    }
  });
  const updateMutation = useMutation({
    mutationFn: ({ id, status }: { id: string; status: ChannelTaskStatus }) =>
      api.channels.updateChannelTask(workspaceId, channelId, id, { status }),
    onSuccess: invalidate
  });
  return (
    <section className="talk-task-panel">
      <header><ClipboardCheck size={18} /><span><strong>Task công việc</strong><small>Tương thích mô hình Deck card</small></span></header>
      <form onSubmit={(event) => { event.preventDefault(); createMutation.mutate(); }}>
        <input maxLength={240} onChange={(event) => setTitle(event.target.value)} placeholder="Việc cần làm…" value={title} />
        <select onChange={(event) => setAssigneeUserId(event.target.value)} value={assigneeUserId}>
          <option value="">Chưa giao</option>
          {members.map((member) => <option key={member.user_id} value={member.user_id}>{member.display_name || member.username}</option>)}
        </select>
        <Button disabled={!title.trim() || createMutation.isPending} size="sm" type="submit"><Plus size={14} /> Thêm</Button>
      </form>
      <div className="talk-task-list">
        {(query.data ?? []).map((task) => (
          <label className={task.status === "done" ? "is-done" : ""} key={task.id}>
            <input
              checked={task.status === "done"}
              onChange={(event) => updateMutation.mutate({ id: task.id, status: event.target.checked ? "done" : "open" })}
              type="checkbox"
            />
            <span><strong>{task.title}</strong><small>{task.assignee_user_id ? memberName(members, task.assignee_user_id) : "Chưa giao"}</small></span>
          </label>
        ))}
        {!query.isLoading && !query.data?.length ? <p>Chưa có task. Bạn cũng có thể chuyển trực tiếp một tin nhắn thành task.</p> : null}
      </div>
    </section>
  );
}

function BreakoutRoomsPanel({
  canManage,
  channelId,
  members,
  onOpenRoom,
  onToast,
  rooms,
  workspaceId
}: {
  canManage: boolean;
  channelId: string;
  members: ChannelMember[];
  onOpenRoom: (roomKey: string) => void;
  onToast: (message: string) => void;
  rooms: BreakoutRoom[];
  workspaceId: string;
}) {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [assignedUserIds, setAssignedUserIds] = useState<string[]>([]);
  const [assignmentMode, setAssignmentMode] = useState<"automatic" | "manual" | "self_select">("automatic");
  const [roomCount, setRoomCount] = useState(2);
  const [broadcast, setBroadcast] = useState("");
  const invalidate = () => queryClient.invalidateQueries({ queryKey: queryKeys.channels.breakoutRooms(workspaceId, channelId) });
  const createMutation = useMutation({
    mutationFn: () => api.channels.createBreakoutRoom(workspaceId, channelId, { assigned_user_ids: assignedUserIds, name }),
    onSuccess: async () => {
      setName("");
      setAssignedUserIds([]);
      await invalidate();
      onToast("Đã tạo phòng thảo luận nhỏ.");
    }
  });
  const closeMutation = useMutation({
    mutationFn: (roomId?: string) =>
      roomId
        ? api.channels.closeBreakoutRoom(workspaceId, channelId, roomId)
        : api.channels.returnBreakoutRooms(workspaceId, channelId),
    onSuccess: invalidate
  });
  const setupMutation = useMutation({
    mutationFn: () => api.channels.setupBreakoutRooms(workspaceId, channelId, {
      allow_self_select: assignmentMode === "self_select",
      assignment_mode: assignmentMode,
      room_count: roomCount
    }),
    onError: (error) => onToast(error instanceof Error ? error.message : "Không chuẩn bị được breakout rooms."),
    onSuccess: async () => {
      await invalidate();
      onToast("Đã chuẩn bị breakout rooms.");
    }
  });
  const startMutation = useMutation({
    mutationFn: () => api.channels.startBreakoutRooms(workspaceId, channelId),
    onSuccess: invalidate
  });
  const joinMutation = useMutation({
    mutationFn: (room: BreakoutRoom) =>
      room.allow_self_select
        ? api.channels.joinBreakoutRoom(workspaceId, channelId, room.id)
        : Promise.resolve(rooms),
    onSuccess: (_result, room) => {
      void invalidate();
      onOpenRoom(room.room_key);
    }
  });
  const broadcastMutation = useMutation({
    mutationFn: () => api.channels.broadcastToBreakouts(workspaceId, channelId, broadcast),
    onSuccess: () => {
      setBroadcast("");
      onToast("Đã gửi thông báo tới tất cả breakout rooms.");
    }
  });
  const openRooms = rooms.filter((room) => room.status === "prepared" || room.status === "active");
  const activeRooms = openRooms.filter((room) => room.status === "active");
  const preparedRooms = openRooms.filter((room) => room.status === "prepared");
  return (
    <section className="talk-card talk-breakout-card">
      <header><Users size={17} /><strong>Breakout rooms</strong><Badge tone="slate">{openRooms.length}</Badge></header>
      {canManage ? (
        <div className="talk-breakout-setup">
          <div className="talk-grid-form">
            <label>
              Chia thành viên
              <select onChange={(event) => setAssignmentMode(event.target.value as typeof assignmentMode)} value={assignmentMode}>
                <option value="automatic">Tự động, cân bằng</option>
                <option value="self_select">Tự chọn phòng</option>
                <option value="manual">Thủ công từng phòng</option>
              </select>
            </label>
            <label>
              Số phòng
              <input max={20} min={2} onChange={(event) => setRoomCount(Number(event.target.value))} type="number" value={roomCount} />
            </label>
            {assignmentMode !== "manual" ? (
              <Button disabled={setupMutation.isPending} onClick={() => setupMutation.mutate()} size="sm">Chuẩn bị phòng</Button>
            ) : null}
          </div>
        </div>
      ) : null}
      {canManage && assignmentMode === "manual" ? (
        <div className="talk-breakout-create">
          <input maxLength={80} onChange={(event) => setName(event.target.value)} placeholder="Tên phòng nhỏ" value={name} />
          <div>
            {members.map((member) => (
              <label key={member.user_id}>
                <input
                  checked={assignedUserIds.includes(member.user_id)}
                  onChange={(event) => setAssignedUserIds((current) => event.target.checked ? [...current, member.user_id] : current.filter((id) => id !== member.user_id))}
                  type="checkbox"
                />
                {member.display_name || member.username}
              </label>
            ))}
          </div>
          <Button disabled={name.trim().length < 2 || createMutation.isPending} onClick={() => createMutation.mutate()} size="sm">Tạo phòng</Button>
        </div>
      ) : null}
      {canManage && preparedRooms.length ? (
        <Button disabled={startMutation.isPending} onClick={() => startMutation.mutate()} size="sm">
          Bắt đầu tất cả phòng
        </Button>
      ) : null}
      {canManage && activeRooms.length ? (
        <div className="talk-inline-form">
          <input
            maxLength={2000}
            onChange={(event) => setBroadcast(event.target.value)}
            placeholder="Thông báo tới tất cả phòng nhỏ"
            value={broadcast}
          />
          <Button disabled={!broadcast.trim() || broadcastMutation.isPending} onClick={() => broadcastMutation.mutate()} size="sm">
            Gửi
          </Button>
        </div>
      ) : null}
      {openRooms.map((room) => (
        <div className="talk-breakout-row" key={room.id}>
          <span><strong>{room.name}</strong><small>{room.assigned_user_ids.length} thành viên</small></span>
          <button
            disabled={room.status !== "active" || joinMutation.isPending}
            onClick={() => joinMutation.mutate(room)}
            type="button"
          >
            Tham gia
          </button>
          {canManage ? <button onClick={() => closeMutation.mutate(room.id)} type="button"><X size={14} /></button> : null}
        </div>
      ))}
      {canManage && openRooms.length ? <Button onClick={() => closeMutation.mutate(undefined)} size="sm" variant="ghost">Đưa tất cả về phòng chính</Button> : null}
    </section>
  );
}

function JitsiMeetingOverlay({
  chatLocked,
  displayName,
  meetingBaseUrl,
  microphoneEnabled,
  onClose,
  participantRole,
  roomKey,
  videoEnabled
}: {
  chatLocked: boolean;
  displayName: string;
  meetingBaseUrl?: string;
  microphoneEnabled: boolean;
  onClose: () => void;
  participantRole: CollaborationParticipantRole;
  roomKey: string;
  videoEnabled: boolean;
}) {
  const baseUrl = meetingBaseUrl?.trim() || jitsiBaseUrl();
  const canPresent = participantRole !== "listener";
  const source = useMemo(() => {
    if (!baseUrl || !roomKey) return "";
    const url = new URL(encodeURIComponent(roomKey), `${baseUrl.replace(/\/+$/, "")}/`);
    url.searchParams.set("userInfo.displayName", displayName);
    const hash = new URLSearchParams({
      "config.prejoinConfig.enabled": "false",
      "config.startWithAudioMuted": String(!microphoneEnabled),
      "config.startWithVideoMuted": String(!videoEnabled),
      "config.toolbarButtons": JSON.stringify(jitsiToolbarButtons(canPresent, !chatLocked)),
      "interfaceConfig.TILE_VIEW_MAX_COLUMNS": "5"
    });
    url.hash = hash.toString();
    return url.toString();
  }, [baseUrl, canPresent, chatLocked, displayName, microphoneEnabled, roomKey, videoEnabled]);
  return (
    <div className="talk-meeting-overlay" role="dialog" aria-label="Phòng họp video">
      <header><span><Video size={18} /><strong>Phòng họp bảo mật</strong></span><button onClick={onClose} type="button"><X size={18} /></button></header>
      {source ? (
        <iframe
          allow="camera; microphone; display-capture; fullscreen; clipboard-write; autoplay"
          allowFullScreen
          referrerPolicy="no-referrer"
          src={source}
          title="Jitsi meeting"
        />
      ) : (
        <div className="talk-meeting-missing">
          <ShieldCheck size={28} />
          <strong>Chưa cấu hình Jitsi self-host</strong>
          <p>Đặt NEXT_PUBLIC_JITSI_BASE_URL, ví dụ https://meet.congty.vn, rồi build lại web/desktop.</p>
        </div>
      )}
    </div>
  );
}

function parseWhiteboardStrokes(content: JsonObject): WhiteboardStroke[] {
  if (!Array.isArray(content.strokes)) return [];
  return content.strokes.flatMap((value) => {
    if (!value || typeof value !== "object" || Array.isArray(value)) return [];
    const record = value as Record<string, unknown>;
    if (typeof record.color !== "string" || !Array.isArray(record.points)) return [];
    const points = record.points.flatMap((point) => {
      if (!point || typeof point !== "object" || Array.isArray(point)) return [];
      const candidate = point as Record<string, unknown>;
      return typeof candidate.x === "number" && typeof candidate.y === "number"
        ? [{ x: candidate.x, y: candidate.y }]
        : [];
    });
    return points.length ? [{ color: record.color, points }] : [];
  });
}

function memberName(members: ChannelMember[], userId: string) {
  const member = members.find((item) => item.user_id === userId);
  return member?.display_name || member?.username || "Thành viên";
}

function jitsiBaseUrl() {
  return process.env.NEXT_PUBLIC_JITSI_BASE_URL?.trim() ?? "";
}

function jitsiToolbarButtons(canPresent: boolean, chatEnabled: boolean) {
  return [
    ...(canPresent ? ["microphone", "camera", "desktop", "select-background"] : []),
    ...(chatEnabled ? ["chat"] : []),
    "raisehand",
    "tileview",
    "fullscreen",
    "settings",
    "security",
    "hangup"
  ];
}
