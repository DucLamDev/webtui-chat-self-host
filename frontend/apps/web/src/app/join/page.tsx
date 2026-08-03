"use client";

import { type FormEvent, useEffect, useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Badge, Button, ErrorState } from "@webtui/ui";
import {
  CheckCircle2,
  Clock3,
  LockKeyhole,
  MicOff,
  ShieldCheck,
  Users,
  Video,
  VideoOff
} from "@webtui/icons";
import { api } from "@/lib/api";

type StoredGuestSession = {
  accessToken: string;
  displayName: string;
  requestId: string;
};

export default function PublicConversationJoinPage() {
  const [token, setToken] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [guestSession, setGuestSession] = useState<StoredGuestSession | null>(null);

  const roomQuery = useQuery({
    enabled: Boolean(token),
    queryFn: () => api.channels.publicRoom(token),
    queryKey: ["public-conversation", token],
    retry: false
  });

  useEffect(() => {
    const value = new URL(window.location.href).searchParams.get("token")?.trim() ?? "";
    setToken(value);
  }, []);

  useEffect(() => {
    if (!token) return;
    const raw = window.sessionStorage.getItem(`webtui:guest:${token}`);
    if (!raw) return;
    try {
      const parsed = JSON.parse(raw) as StoredGuestSession;
      if (parsed.accessToken && parsed.requestId) {
        setGuestSession(parsed);
        setDisplayName(parsed.displayName);
      }
    } catch {
      window.sessionStorage.removeItem(`webtui:guest:${token}`);
    }
  }, [token]);

  const joinMutation = useMutation({
    mutationFn: () => api.channels.joinPublicRoom(token, {
      display_name: displayName,
      password: password || undefined
    }),
    onSuccess: (guest) => {
      if (!guest.guest_access_token) return;
      const next = {
        accessToken: guest.guest_access_token,
        displayName,
        requestId: guest.id
      };
      setGuestSession(next);
      window.sessionStorage.setItem(`webtui:guest:${token}`, JSON.stringify(next));
    }
  });

  const statusQuery = useQuery({
    enabled: Boolean(guestSession),
    queryFn: () =>
      api.channels.publicJoinStatus(
        token,
        guestSession?.requestId ?? "",
        guestSession?.accessToken ?? ""
      ),
    queryKey: ["public-conversation", token, "join", guestSession?.requestId],
    refetchInterval: (query) => query.state.data?.status === "waiting" ? 3_000 : false,
    retry: false
  });

  const joinedRoom = joinMutation.data?.room ?? statusQuery.data?.room;
  const joinStatus = joinMutation.data?.status ?? statusQuery.data?.status;

  if (roomQuery.isError) {
    return (
      <main className="guest-join-page">
        <ErrorState
          description="Link có thể đã bị thu hồi, hết hạn hoặc không thuộc server này."
          title="Không tìm thấy phòng họp"
        />
      </main>
    );
  }

  if (joinedRoom?.meeting_room_key) {
    return (
      <PublicMeetingRoom
        chatLocked={joinedRoom.chat_locked}
        displayName={guestSession?.displayName || displayName}
        meetingBaseUrl={joinedRoom.meeting_base_url}
        microphoneEnabled={joinedRoom.guest_microphone_enabled}
        roomMode={joinedRoom.room_mode}
        roomKey={joinedRoom.meeting_room_key}
        videoEnabled={joinedRoom.guest_camera_enabled}
      />
    );
  }

  const room = roomQuery.data;
  return (
    <main className="guest-join-page">
      <section className="guest-join-shell">
        <header>
          <span className="guest-join-logo"><Video size={27} /></span>
          <span>
            <small>PHÒNG HỌP BẢO MẬT</small>
            <h1>{room?.channel_name ?? "Đang tải phòng…"}</h1>
          </span>
          {room ? <Badge tone={room.room_mode === "webinar" ? "orange" : "blue"}>{room.room_mode === "webinar" ? "WEBINAR" : "GUEST"}</Badge> : null}
        </header>

        <div className="guest-room-features">
          <span><ShieldCheck size={16} /><strong>Dữ liệu self-host</strong><small>Không cần tạo tài khoản</small></span>
          <span><MicOff size={16} /><strong>Mic mặc định {room?.guest_microphone_enabled ? "bật" : "tắt"}</strong><small>Host kiểm soát quyền</small></span>
          <span><VideoOff size={16} /><strong>Camera mặc định {room?.guest_camera_enabled ? "bật" : "tắt"}</strong><small>Có thể đổi trước khi vào</small></span>
        </div>

        {joinStatus === "waiting" ? (
          <div className="guest-lobby-waiting">
            <span><Clock3 size={24} /></span>
            <strong>Đang chờ chủ trì duyệt</strong>
            <p>Giữ trang này mở. Bạn sẽ tự động được đưa vào phòng sau khi được chấp nhận.</p>
          </div>
        ) : joinStatus === "rejected" || joinStatus === "expired" ? (
          <div className="guest-lobby-waiting guest-lobby-waiting--rejected">
            <strong>{joinStatus === "expired" ? "Yêu cầu đã hết hạn" : "Chủ trì chưa chấp nhận yêu cầu"}</strong>
            <Button
              onClick={() => {
                setGuestSession(null);
                window.sessionStorage.removeItem(`webtui:guest:${token}`);
              }}
            >
              Thử lại
            </Button>
          </div>
        ) : (
          <form
            className="guest-join-form"
            onSubmit={(event: FormEvent<HTMLFormElement>) => {
              event.preventDefault();
              joinMutation.mutate();
            }}
          >
            <label>
              Tên hiển thị
              <input
                autoComplete="name"
                maxLength={80}
                minLength={2}
                onChange={(event) => setDisplayName(event.target.value)}
                placeholder="Nhập tên của bạn"
                value={displayName}
              />
            </label>
            {room?.has_password ? (
              <label>
                Mật khẩu phòng
                <span><LockKeyhole size={16} /><input autoComplete="current-password" onChange={(event) => setPassword(event.target.value)} type="password" value={password} /></span>
              </label>
            ) : null}
            {joinMutation.isError ? (
              <p className="guest-join-error">{joinMutation.error instanceof Error ? joinMutation.error.message : "Không gửi được yêu cầu tham gia."}</p>
            ) : null}
            <Button disabled={displayName.trim().length < 2 || joinMutation.isPending} type="submit">
              <Users size={17} /> {room?.lobby_enabled ? "Gửi yêu cầu tham gia" : "Vào phòng họp"}
            </Button>
          </form>
        )}

        <footer>
          <CheckCircle2 size={14} />
          Khách chỉ nhận tên phòng media sau khi qua mật khẩu và lobby.
        </footer>
      </section>
    </main>
  );
}

function PublicMeetingRoom({
  chatLocked,
  displayName,
  meetingBaseUrl,
  microphoneEnabled,
  roomMode,
  roomKey,
  videoEnabled
}: {
  chatLocked: boolean;
  displayName: string;
  meetingBaseUrl?: string;
  microphoneEnabled: boolean;
  roomMode: "internal" | "public" | "webinar";
  roomKey: string;
  videoEnabled: boolean;
}) {
  const baseUrl = meetingBaseUrl?.trim() || process.env.NEXT_PUBLIC_JITSI_BASE_URL?.trim() || "";
  const source = useMemo(() => {
    if (!baseUrl) return "";
    const url = new URL(encodeURIComponent(roomKey), `${baseUrl.replace(/\/+$/, "")}/`);
    url.searchParams.set("userInfo.displayName", displayName);
    url.hash = new URLSearchParams({
      "config.prejoinConfig.enabled": "false",
      "config.startWithAudioMuted": String(!microphoneEnabled),
      "config.startWithVideoMuted": String(!videoEnabled),
      "config.toolbarButtons": JSON.stringify([
        ...(roomMode === "webinar" ? [] : ["microphone", "camera", "select-background"]),
        ...(!chatLocked ? ["chat"] : []),
        "raisehand",
        "tileview",
        "fullscreen",
        "settings",
        "hangup"
      ]),
      "interfaceConfig.TILE_VIEW_MAX_COLUMNS": "5"
    }).toString();
    return url.toString();
  }, [baseUrl, chatLocked, displayName, microphoneEnabled, roomKey, roomMode, videoEnabled]);

  if (!source) {
    return (
      <main className="guest-join-page">
        <ErrorState
          description="Server cần đặt NEXT_PUBLIC_JITSI_BASE_URL tới instance Jitsi self-host."
          title="Media server chưa được cấu hình"
        />
      </main>
    );
  }

  return (
    <main className="public-meeting-room">
      <iframe
        allow="camera; microphone; display-capture; fullscreen; clipboard-write; autoplay"
        allowFullScreen
        referrerPolicy="no-referrer"
        src={source}
        title="Phòng họp khách"
      />
    </main>
  );
}
