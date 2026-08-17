"use client";

import { type FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  legalDocumentsCompatibilityError,
  queryKeys,
  resolveCurrentLegalDocuments
} from "@webtui/api-client";
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
import { legalPolicyConfig } from "@/features/auth/legal-policy-config";
import { publicApi, runtimeEnvironment } from "@/lib/api";
import { JitsiMeeting, type JitsiMeetingHandle } from "@/features/chat/components/jitsi-meeting";
import { buildJitsiToolbarButtons } from "@/features/chat/components/jitsi-meeting-controller";

type StoredGuestSession = {
  accessToken: string;
  displayName: string;
  requestId: string;
};

export default function PublicConversationJoinPage() {
  const [token, setToken] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [termsAccepted, setTermsAccepted] = useState(false);
  const [privacyAccepted, setPrivacyAccepted] = useState(false);
  const [guestSession, setGuestSession] = useState<StoredGuestSession | null>(null);

  const legalDocumentsQuery = useQuery({
    queryFn: () => publicApi.auth.legalDocuments(),
    queryKey: queryKeys.auth.legalDocuments(runtimeEnvironment.apiBaseUrl),
    retry: false
  });
  const legalDocumentsResolution = useMemo(
    () => resolveCurrentLegalDocuments(legalDocumentsQuery.data),
    [legalDocumentsQuery.data]
  );
  const legalDocuments = legalDocumentsResolution.documents;
  const legalError = legalPolicyConfig.configurationError
    ?? (legalDocumentsQuery.isError
      ? legalDocumentsQuery.error instanceof Error
        ? legalDocumentsQuery.error.message
        : "Không tải được tài liệu pháp lý từ máy chủ."
      : legalDocumentsQuery.isLoading ? null : legalDocumentsResolution.error)
    ?? (legalDocuments
      ? legalDocumentsCompatibilityError(legalDocuments, legalPolicyConfig)
      : null);
  const legalReady = Boolean(legalDocuments && !legalError && !legalDocumentsQuery.isFetching);

  const roomQuery = useQuery({
    enabled: Boolean(token),
    queryFn: () => publicApi.channels.publicRoom(token),
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
    mutationFn: () => {
      if (!legalDocuments || legalError || !termsAccepted || !privacyAccepted) {
        throw new Error("Hãy chấp nhận đầy đủ tài liệu pháp lý hiện hành trước khi tham gia.");
      }
      return publicApi.channels.joinPublicRoom(token, {
        display_name: displayName,
        password: password || undefined,
        privacy_accepted: true,
        privacy_version: legalDocuments.privacy.version,
        terms_accepted: true,
        terms_version: legalDocuments.terms.version
      });
    },
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
      publicApi.channels.publicJoinStatus(
        token,
        guestSession?.requestId ?? "",
        guestSession?.accessToken ?? ""
      ),
    queryKey: ["public-conversation", token, "join", guestSession?.requestId],
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return !status || status === "waiting" ? 2_000 : false;
    },
    refetchIntervalInBackground: true,
    retry: 3
  });

  // Polling data is newer than the original join response. Preferring it is
  // important because the first response remains "waiting" after a host has
  // approved the guest.
  const latestGuest = statusQuery.data ?? joinMutation.data;
  const joinedRoom = latestGuest?.room;
  const joinStatus = latestGuest?.status;

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
        roomTitle={joinedRoom.channel_name}
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
            {statusQuery.isError ? (
              <Button onClick={() => void statusQuery.refetch()} variant="ghost">
                Kết nối lại
              </Button>
            ) : null}
          </div>
        ) : joinStatus === "approved" ? (
          <div className="guest-lobby-waiting">
            <strong>Đã được chấp nhận</strong>
            <p>Phòng họp đang được chuẩn bị. Bạn sẽ được đưa vào tự động.</p>
            <Button onClick={() => void statusQuery.refetch()} variant="ghost">Thử lại</Button>
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
            {legalError ? (
              <div className="guest-legal-error" role="alert">
                <p>{legalError}</p>
                <Button
                  disabled={legalDocumentsQuery.isFetching}
                  onClick={() => void legalDocumentsQuery.refetch()}
                  type="button"
                  variant="ghost"
                >
                  Thử tải lại tài liệu
                </Button>
              </div>
            ) : legalReady && legalDocuments ? (
              <div className="guest-legal-consents">
                <label className="guest-legal-consent">
                  <input
                    checked={termsAccepted}
                    onChange={(event) => setTermsAccepted(event.target.checked)}
                    type="checkbox"
                  />
                  <span>
                    Tôi chấp thuận <a href={legalPolicyConfig.termsUrl} rel="noreferrer" target="_blank">Điều khoản và Quy tắc sử dụng</a> (bản {legalDocuments.terms.version}).
                  </span>
                </label>
                <label className="guest-legal-consent">
                  <input
                    checked={privacyAccepted}
                    onChange={(event) => setPrivacyAccepted(event.target.checked)}
                    type="checkbox"
                  />
                  <span>
                    Tôi xác nhận đã đọc <a href={legalPolicyConfig.privacyUrl} rel="noreferrer" target="_blank">Chính sách quyền riêng tư</a> (bản {legalDocuments.privacy.version}).
                  </span>
                </label>
              </div>
            ) : (
              <p className="guest-legal-loading">Đang xác minh tài liệu pháp lý hiện hành…</p>
            )}
            {joinMutation.isError ? (
              <p className="guest-join-error">{joinMutation.error instanceof Error ? joinMutation.error.message : "Không gửi được yêu cầu tham gia."}</p>
            ) : null}
            <Button
              disabled={
                displayName.trim().length < 2
                || !legalReady
                || !termsAccepted
                || !privacyAccepted
                || joinMutation.isPending
              }
              type="submit"
            >
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
  roomTitle,
  roomMode,
  roomKey,
  videoEnabled
}: {
  chatLocked: boolean;
  displayName: string;
  meetingBaseUrl?: string;
  microphoneEnabled: boolean;
  roomTitle: string;
  roomMode: "internal" | "public" | "webinar";
  roomKey: string;
  videoEnabled: boolean;
}) {
  const meetingRef = useRef<JitsiMeetingHandle>(null);
  const [hasLeft, setHasLeft] = useState(false);
  const baseUrl = meetingBaseUrl?.trim()
    || process.env.NEXT_PUBLIC_JITSI_BASE_URL?.trim()
    || defaultMeetingBaseUrl();
  const canPresent = roomMode !== "webinar";
  const toolbarButtons = useMemo(
    () => buildJitsiToolbarButtons({ canPresent, chatEnabled: !chatLocked }),
    [canPresent, chatLocked]
  );

  if (!baseUrl) {
    return (
      <main className="guest-join-page">
        <ErrorState
          description="Dịch vụ họp chưa sẵn sàng. Vui lòng thử lại sau ít phút."
          title="Chưa thể mở phòng họp"
        />
      </main>
    );
  }

  if (hasLeft) {
    return (
      <main className="guest-join-page">
        <ErrorState
          description="Cửa sổ cuộc họp đã được đóng an toàn. Bạn có thể đóng trang này hoặc quay lại đường dẫn mời để tham gia lại."
          title="Bạn đã rời cuộc họp"
        />
      </main>
    );
  }

  return (
    <main className="public-meeting-room">
      <JitsiMeeting
        baseUrl={baseUrl}
        canAnnotate={canPresent}
        displayName={displayName}
        meetingTitle={roomTitle}
        microphoneEnabled={microphoneEnabled}
        onClose={() => setHasLeft(true)}
        ref={meetingRef}
        roomKey={roomKey}
        toolbarButtons={toolbarButtons}
        videoEnabled={videoEnabled}
      />
    </main>
  );
}

function defaultMeetingBaseUrl() {
  if (typeof window === "undefined") return "";
  const url = new URL(window.location.origin);
  url.port = "8443";
  return url.origin;
}
