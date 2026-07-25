"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type {
  CallMode,
  RealtimeCallSignal
} from "./use-channel-realtime";
import { api, runtimeEnvironment } from "@/lib/api";
import { useAuthStore } from "@/features/auth/auth-store";

export type WebRtcCallStatus =
  | "idle"
  | "incoming"
  | "outgoing"
  | "connecting"
  | "active"
  | "ended"
  | "error";

export type WebRtcCallState = {
  callId?: string;
  channelId?: string;
  error?: string;
  initiatorUserId?: string;
  mode: CallMode;
  peerName?: string;
  peerUserId?: string;
  startedAt?: number;
  status: WebRtcCallStatus;
};

export type WebRtcCallOutcome = {
  callId: string;
  direction: "incoming" | "outgoing";
  durationSeconds?: number;
  endedAt: number;
  initiatorUserId: string;
  mode: CallMode;
  reason?: string;
  startedAt?: number;
  status: "completed" | "missed";
};

type UseWebRtcCallOptions = {
  channelId?: string;
  channelName?: string;
  currentUserId: string;
  enabled?: boolean;
  lastSignal: RealtimeCallSignal | null;
  onCallOutcome?: (outcome: WebRtcCallOutcome) => void;
  peerName?: string;
  peerUserId?: string;
  resolvePeerName?: (userId?: string, channelId?: string) => string | undefined;
  workspaceId?: string;
};

const outgoingRingTimeoutMs = 30_000;
const mediaContainerWaitTimeoutMs = 4_000;

export function useWebRtcCall({
  channelId,
  channelName,
  currentUserId,
  enabled = true,
  lastSignal,
  onCallOutcome,
  peerName,
  peerUserId,
  resolvePeerName,
  workspaceId
}: UseWebRtcCallOptions) {
  const [callState, setCallState] = useState<WebRtcCallState>({
    mode: "audio",
    status: "idle"
  });
  const [hasMediaSession, setHasMediaSession] = useState(false);
  const mediaContainerRef = useRef<HTMLDivElement | null>(null);
  const peerConnectionRef = useRef<RTCPeerConnection | null>(null);
  const localStreamRef = useRef<MediaStream | null>(null);
  const remoteStreamRef = useRef<MediaStream | null>(null);
  const activeCallIdRef = useRef("");
  const pendingCandidatesRef = useRef<RTCIceCandidateInit[]>([]);
  const pendingOfferRef = useRef<RTCSessionDescriptionInit | null>(null);
  const callStateRef = useRef<WebRtcCallState>({
    mode: "audio",
    status: "idle"
  });
  const lastSignalSequenceRef = useRef(0);
  const previousChannelIdRef = useRef(channelId);
  const operationTokenRef = useRef(0);
  const outgoingRingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const disconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reportedOutcomeCallIdsRef = useRef(new Set<string>());

  useEffect(() => {
    callStateRef.current = callState;
  }, [callState]);

  const clearOutgoingTimer = useCallback(() => {
    if (outgoingRingTimerRef.current) {
      clearTimeout(outgoingRingTimerRef.current);
      outgoingRingTimerRef.current = null;
    }
  }, []);

  const clearDisconnectTimer = useCallback(() => {
    if (disconnectTimerRef.current) {
      clearTimeout(disconnectTimerRef.current);
      disconnectTimerRef.current = null;
    }
  }, []);

  const reportOutcome = useCallback(
    (state: WebRtcCallState, reason?: string) => {
      if (
        !onCallOutcome ||
        !state.callId ||
        !state.initiatorUserId ||
        reportedOutcomeCallIdsRef.current.has(state.callId)
      ) {
        return;
      }
      reportedOutcomeCallIdsRef.current.add(state.callId);
      const endedAt = Date.now();
      onCallOutcome({
        callId: state.callId,
        direction:
          state.initiatorUserId === currentUserId ? "outgoing" : "incoming",
        durationSeconds: state.startedAt
          ? Math.max(0, Math.round((endedAt - state.startedAt) / 1000))
          : undefined,
        endedAt,
        initiatorUserId: state.initiatorUserId,
        mode: state.mode,
        reason,
        startedAt: state.startedAt,
        status: state.startedAt ? "completed" : "missed"
      });
    },
    [currentUserId, onCallOutcome]
  );

  const stopMediaSession = useCallback(() => {
    clearDisconnectTimer();
    const connection = peerConnectionRef.current;
    peerConnectionRef.current = null;
    connection?.close();
    localStreamRef.current?.getTracks().forEach((track) => track.stop());
    remoteStreamRef.current?.getTracks().forEach((track) => track.stop());
    localStreamRef.current = null;
    remoteStreamRef.current = null;
    activeCallIdRef.current = "";
    pendingCandidatesRef.current = [];
    pendingOfferRef.current = null;
    mediaContainerRef.current?.replaceChildren();
    setHasMediaSession(false);
  }, [clearDisconnectTimer]);

  const resetCallUi = useCallback(
    (nextState?: WebRtcCallState) => {
      operationTokenRef.current += 1;
      clearOutgoingTimer();
      stopMediaSession();
      setCallState(nextState ?? { mode: "audio", status: "idle" });
    },
    [clearOutgoingTimer, stopMediaSession]
  );

  const finishCall = useCallback(
    (state: WebRtcCallState, reason?: string, error?: string) => {
      reportOutcome(state, reason);
      resetCallUi({
        callId: state.callId,
        channelId: state.channelId,
        error,
        initiatorUserId: state.initiatorUserId,
        mode: state.mode,
        peerName: state.peerName,
        peerUserId: state.peerUserId,
        startedAt: state.startedAt,
        status: error ? "error" : "ended"
      });
    },
    [reportOutcome, resetCallUi]
  );

  const waitForMediaContainer = useCallback(async () => {
    const startedAt = Date.now();
    while (!mediaContainerRef.current) {
      if (Date.now() - startedAt > mediaContainerWaitTimeoutMs) {
        throw new Error("Không tìm thấy khung hiển thị cuộc gọi.");
      }
      await delay(50);
    }
    return mediaContainerRef.current;
  }, []);

  const signal = useCallback(
    async (
      type: "CallOffer" | "CallAnswer" | "CallIceCandidate",
      callId: string,
      payload: {
        candidate?: RTCIceCandidateInit;
        sdp?: RTCSessionDescriptionInit;
      } = {}
    ) => {
      if (!workspaceId) {
        throw new Error("Workspace chưa sẵn sàng cho cuộc gọi.");
      }
      const signalType =
        type === "CallOffer"
          ? "offer"
          : type === "CallAnswer"
            ? "answer"
            : "ice_candidate";
      await api.calls.signal(workspaceId, callId, signalType, payload);
    },
    [workspaceId]
  );

  const beginMediaSession = useCallback(
    async (callId: string, mode: CallMode) => {
      if (
        activeCallIdRef.current === callId &&
        peerConnectionRef.current
      ) {
        return peerConnectionRef.current;
      }
      if (
        typeof RTCPeerConnection === "undefined" ||
        !navigator.mediaDevices?.getUserMedia
      ) {
        throw new Error("Trình duyệt này không hỗ trợ cuộc gọi WebRTC.");
      }

      stopMediaSession();
      const operationToken = operationTokenRef.current;
      setHasMediaSession(true);
      const container = await waitForMediaContainer();
      const localStream = await navigator.mediaDevices.getUserMedia({
        audio: true,
        video:
          mode === "video"
            ? { facingMode: "user", height: { ideal: 720 }, width: { ideal: 1280 } }
            : false
      });
      if (operationToken !== operationTokenRef.current) {
        localStream.getTracks().forEach((track) => track.stop());
        throw new Error("Cuộc gọi đã kết thúc.");
      }

      const remoteStream = new MediaStream();
      const discoveredIceServers =
        useAuthStore.getState().zoneRuntime?.rtc_ice_servers;
      const connection = new RTCPeerConnection({
        iceServers: (
          discoveredIceServers?.length
            ? discoveredIceServers
            : runtimeEnvironment.rtcIceServers
        ) as RTCIceServer[]
      });
      activeCallIdRef.current = callId;
      localStreamRef.current = localStream;
      remoteStreamRef.current = remoteStream;
      peerConnectionRef.current = connection;
      renderMedia(container, mode, localStream, remoteStream);

      localStream.getTracks().forEach((track) => {
        connection.addTrack(track, localStream);
      });
      connection.ontrack = (event) => {
        const stream = event.streams[0];
        if (stream) {
          remoteStreamRef.current = stream;
          attachRemoteStream(container, mode, stream);
          return;
        }
        remoteStream.addTrack(event.track);
      };
      connection.onicecandidate = (event) => {
        if (!event.candidate || activeCallIdRef.current !== callId) {
          return;
        }
        try {
          void signal("CallIceCandidate", callId, {
            candidate: event.candidate.toJSON()
          }).catch(() => undefined);
        } catch {
          // A later ICE candidate can still establish the connection.
        }
      };
      connection.onconnectionstatechange = () => {
        if (activeCallIdRef.current !== callId) {
          return;
        }
        if (connection.connectionState === "connected") {
          clearDisconnectTimer();
          clearOutgoingTimer();
          setCallState((current) =>
            current.callId === callId
              ? {
                  ...current,
                  startedAt: current.startedAt ?? Date.now(),
                  status: "active"
                }
              : current
          );
          return;
        }
        if (connection.connectionState === "failed") {
          const current = callStateRef.current;
          if (current.callId === callId) {
            if (workspaceId) {
              void api.calls
                .hangup(workspaceId, callId, "webrtc_failed")
                .catch(() => undefined);
            }
            finishCall(
              current,
              "webrtc_failed",
              "Không thể thiết lập đường truyền WebRTC. Hãy kiểm tra TURN và tường lửa."
            );
          }
          return;
        }
        if (connection.connectionState === "disconnected") {
          clearDisconnectTimer();
          disconnectTimerRef.current = setTimeout(() => {
            if (
              connection.connectionState === "disconnected" &&
              callStateRef.current.callId === callId
            ) {
              if (workspaceId) {
                void api.calls
                  .hangup(workspaceId, callId, "webrtc_disconnected")
                  .catch(() => undefined);
              }
              finishCall(
                callStateRef.current,
                "webrtc_disconnected",
                "Kết nối cuộc gọi đã bị gián đoạn."
              );
            }
          }, 5_000);
        }
      };

      return connection;
    },
    [
      clearDisconnectTimer,
      clearOutgoingTimer,
      finishCall,
      signal,
      stopMediaSession,
      waitForMediaContainer,
      workspaceId
    ]
  );

  const flushPendingCandidates = useCallback(
    async (connection: RTCPeerConnection) => {
      const candidates = pendingCandidatesRef.current;
      pendingCandidatesRef.current = [];
      for (const candidate of candidates) {
        await connection.addIceCandidate(candidate);
      }
    },
    []
  );

  const sendOffer = useCallback(
    async (callId: string, mode: CallMode) => {
      const connection = await beginMediaSession(callId, mode);
      const offer = await connection.createOffer();
      await connection.setLocalDescription(offer);
      await signal("CallOffer", callId, { sdp: offer });
    },
    [beginMediaSession, signal]
  );

  const receiveOffer = useCallback(
    async (
      callId: string,
      mode: CallMode,
      offer: RTCSessionDescriptionInit
    ) => {
      if (callStateRef.current.status === "incoming") {
        pendingOfferRef.current = offer;
        return;
      }
      const connection = await beginMediaSession(callId, mode);
      await connection.setRemoteDescription(offer);
      await flushPendingCandidates(connection);
      const answer = await connection.createAnswer();
      await connection.setLocalDescription(answer);
      await signal("CallAnswer", callId, { sdp: answer });
    },
    [beginMediaSession, flushPendingCandidates, signal]
  );

  const receiveAnswer = useCallback(
    async (callId: string, answer: RTCSessionDescriptionInit) => {
      const connection = peerConnectionRef.current;
      if (!connection || activeCallIdRef.current !== callId) {
        return;
      }
      await connection.setRemoteDescription(answer);
      await flushPendingCandidates(connection);
    },
    [flushPendingCandidates]
  );

  const receiveCandidate = useCallback(
    async (callId: string, candidate: RTCIceCandidateInit) => {
      const connection = peerConnectionRef.current;
      if (
        !connection ||
        activeCallIdRef.current !== callId ||
        !connection.remoteDescription
      ) {
        pendingCandidatesRef.current.push(candidate);
        return;
      }
      await connection.addIceCandidate(candidate);
    },
    []
  );

  useEffect(() => {
    const channelChanged = previousChannelIdRef.current !== channelId;
    previousChannelIdRef.current = channelId;
    if (channelChanged) {
      lastSignalSequenceRef.current = 0;
    }
  }, [channelId]);

  useEffect(() => {
    if (callState.status !== "ended" && callState.status !== "error") {
      return undefined;
    }
    const timeout = window.setTimeout(() => {
      setCallState({ mode: "audio", status: "idle" });
    }, 4_000);
    return () => window.clearTimeout(timeout);
  }, [callState.status]);

  useEffect(
    () => () => {
      clearOutgoingTimer();
      stopMediaSession();
    },
    [clearOutgoingTimer, stopMediaSession]
  );

  const startCall = useCallback(
    async (mode: CallMode) => {
      if (!enabled || !workspaceId || !channelId) {
        setCallState({
          error: "Realtime chưa sẵn sàng để bắt đầu cuộc gọi.",
          mode,
          status: "error"
        });
        return;
      }
      if (!peerUserId) {
        setCallState({
          error: "Không tìm thấy người nhận cuộc gọi trong hội thoại này.",
          mode,
          status: "error"
        });
        return;
      }
      if (!isCallFinished(callStateRef.current.status)) {
        return;
      }

      const operationToken = ++operationTokenRef.current;
      let backendCallId = "";
      try {
        setCallState({
          channelId,
          initiatorUserId: currentUserId,
          mode,
          peerName: peerName || channelName,
          peerUserId,
          status: "outgoing"
        });
        const backendCall = await api.calls.create(workspaceId, {
          channel_id: channelId,
          client_call_id: newCallId(),
          metadata: {
            client: "web",
            provider: "self_hosted_webrtc"
          },
          mode,
          target_user_id: peerUserId
        });
        backendCallId = backendCall.id;
        if (operationToken !== operationTokenRef.current) {
          return;
        }

        const nextState: WebRtcCallState = {
          callId: backendCall.id,
          channelId: backendCall.channel_id || channelId,
          initiatorUserId:
            backendCall.initiator_user_id || currentUserId,
          mode,
          peerName: peerName || channelName,
          peerUserId,
          status: "outgoing"
        };
        setCallState(nextState);
        await beginMediaSession(backendCall.id, mode);

        outgoingRingTimerRef.current = setTimeout(() => {
          const current = callStateRef.current;
          if (
            current.callId !== backendCall.id ||
            current.status !== "outgoing"
          ) {
            return;
          }
          void api.calls
            .cancel(workspaceId, backendCall.id, "no_answer")
            .catch(() => undefined);
          finishCall(current, "no_answer", "Không có phản hồi.");
        }, outgoingRingTimeoutMs);
      } catch (error) {
        if (backendCallId) {
          void api.calls
            .cancel(workspaceId, backendCallId, "webrtc_setup_failed")
            .catch(() => undefined);
        }
        const current = callStateRef.current;
        finishCall(
          {
            ...current,
            callId: backendCallId || current.callId,
            channelId,
            initiatorUserId: currentUserId,
            mode,
            peerName: peerName || channelName,
            peerUserId
          },
          "webrtc_setup_failed",
          callErrorMessage(error)
        );
      }
    },
    [
      beginMediaSession,
      channelId,
      channelName,
      currentUserId,
      enabled,
      finishCall,
      peerName,
      peerUserId,
      workspaceId
    ]
  );

  const acceptCall = useCallback(async () => {
    const current = callStateRef.current;
    if (!workspaceId || !current.callId || current.status !== "incoming") {
      return;
    }
    const operationToken = ++operationTokenRef.current;
    try {
      setCallState({ ...current, status: "connecting" });
      await beginMediaSession(current.callId, current.mode);
      if (operationToken !== operationTokenRef.current) {
        return;
      }
      await api.calls.accept(workspaceId, current.callId);
      await api.calls.signal(workspaceId, current.callId, "ready", {});
      const pendingOffer = pendingOfferRef.current;
      pendingOfferRef.current = null;
      if (pendingOffer) {
        await receiveOffer(current.callId, current.mode, pendingOffer);
      }
    } catch (error) {
      await api.calls
        .reject(workspaceId, current.callId, "webrtc_setup_failed")
        .catch(() => undefined);
      finishCall(
        current,
        "webrtc_setup_failed",
        callErrorMessage(error)
      );
    }
  }, [beginMediaSession, finishCall, receiveOffer, workspaceId]);

  const rejectCall = useCallback(() => {
    const current = callStateRef.current;
    if (workspaceId && current.callId) {
      void api.calls
        .reject(workspaceId, current.callId, "declined")
        .catch(() => undefined);
    }
    finishCall(current, "declined");
  }, [finishCall, workspaceId]);

  const endCall = useCallback(() => {
    const current = callStateRef.current;
    if (workspaceId && current.callId) {
      if (current.status === "outgoing") {
        void api.calls
          .cancel(workspaceId, current.callId, "cancelled")
          .catch(() => undefined);
      } else if (current.status === "incoming") {
        void api.calls
          .reject(workspaceId, current.callId, "declined")
          .catch(() => undefined);
      } else {
        void api.calls
          .hangup(workspaceId, current.callId, "ended")
          .catch(() => undefined);
      }
    }
    finishCall(current, "ended");
  }, [finishCall, workspaceId]);

  useEffect(() => {
    if (!lastSignal) {
      return;
    }
    if (!enabled || !workspaceId) {
      lastSignalSequenceRef.current = Math.max(
        lastSignalSequenceRef.current,
        lastSignal.sequence
      );
      return;
    }
    if (lastSignal.sequence <= lastSignalSequenceRef.current) {
      return;
    }
    lastSignalSequenceRef.current = lastSignal.sequence;

    const payload = lastSignal.payload;
    const callId = payload.call_id;
    const mode = payload.mode ?? callStateRef.current.mode ?? "audio";
    const signalWorkspaceId = payload.workspace_id || workspaceId;
    const signalChannelId =
      payload.channel_id || callStateRef.current.channelId || channelId;
    const initiatorUserId =
      payload.initiator_user_id || lastSignal.userId;

    if (lastSignal.type === "CallInvited") {
      if (
        payload.target_user_id !== currentUserId ||
        initiatorUserId === currentUserId
      ) {
        return;
      }
      if (!isCallFinished(callStateRef.current.status)) {
        void api.calls
          .reject(signalWorkspaceId, callId, "busy")
          .catch(() => undefined);
        return;
      }
      setCallState({
        callId,
        channelId: signalChannelId,
        initiatorUserId,
        mode,
        peerName:
          resolvePeerName?.(initiatorUserId, signalChannelId) ||
          peerName ||
          channelName,
        peerUserId: initiatorUserId,
        status: "incoming"
      });
      return;
    }

    if (callStateRef.current.callId !== callId) {
      return;
    }

    if (
      lastSignal.userId !== currentUserId &&
      lastSignal.type === "CallOffer" &&
      payload.sdp
    ) {
      void receiveOffer(callId, mode, payload.sdp).catch((error) => {
        finishCall(
          callStateRef.current,
          "webrtc_negotiation_failed",
          callErrorMessage(error)
        );
      });
      return;
    }
    if (
      lastSignal.userId !== currentUserId &&
      lastSignal.type === "CallAnswer" &&
      payload.sdp
    ) {
      void receiveAnswer(callId, payload.sdp).catch((error) => {
        finishCall(
          callStateRef.current,
          "webrtc_negotiation_failed",
          callErrorMessage(error)
        );
      });
      return;
    }
    if (
      lastSignal.userId !== currentUserId &&
      lastSignal.type === "CallIceCandidate" &&
      payload.candidate
    ) {
      void receiveCandidate(callId, payload.candidate).catch(() => undefined);
      return;
    }

    if (lastSignal.type === "CallAccepted") {
      clearOutgoingTimer();
      setCallState((current) =>
        current.callId === callId
          ? { ...current, status: "connecting" }
          : current
      );
      return;
    }

    if (lastSignal.type === "CallReady") {
      if (callStateRef.current.initiatorUserId === currentUserId) {
        void sendOffer(callId, mode).catch((error) => {
          finishCall(
            callStateRef.current,
            "webrtc_negotiation_failed",
            callErrorMessage(error)
          );
        });
      }
      return;
    }

    if (lastSignal.type === "CallRejected") {
      finishCall(
        callStateRef.current,
        payload.reason,
        rejectionMessage(payload.reason)
      );
      return;
    }

    if (
      lastSignal.type === "CallCancelled" ||
      lastSignal.type === "CallMissed"
    ) {
      finishCall(
        callStateRef.current,
        lastSignal.type === "CallMissed" ? "missed" : "cancelled",
        lastSignal.type === "CallMissed"
          ? "Cuộc gọi bị nhỡ."
          : "Cuộc gọi đã bị hủy."
      );
      return;
    }

    if (lastSignal.type === "CallEnded") {
      finishCall(callStateRef.current, payload.reason || "remote_ended");
    }
  }, [
    channelId,
    channelName,
    clearOutgoingTimer,
    currentUserId,
    enabled,
    finishCall,
    lastSignal,
    peerName,
    receiveAnswer,
    receiveCandidate,
    receiveOffer,
    resolvePeerName,
    sendOffer,
    workspaceId
  ]);

  const openIncomingCall = useCallback(
    async (callId: string) => {
      if (!enabled || !workspaceId || !callId) {
        return false;
      }
      if (!isCallFinished(callStateRef.current.status)) {
        return callStateRef.current.callId === callId;
      }
      try {
        const call = await api.calls.get(workspaceId, callId);
        if (
          call.status !== "ringing" ||
          call.target_user_id !== currentUserId
        ) {
          return false;
        }
        const mode = call.mode === "video" ? "video" : "audio";
        setCallState({
          callId: call.id,
          channelId: call.channel_id,
          initiatorUserId: call.initiator_user_id,
          mode,
          peerName:
            resolvePeerName?.(call.initiator_user_id, call.channel_id) ||
            peerName ||
            channelName,
          peerUserId: call.initiator_user_id,
          status: "incoming"
        });
        return true;
      } catch {
        return false;
      }
    },
    [
      channelName,
      currentUserId,
      enabled,
      peerName,
      resolvePeerName,
      workspaceId
    ]
  );

  return {
    acceptCall,
    callState,
    endCall,
    hasMediaSession,
    mediaContainerRef,
    openIncomingCall,
    rejectCall,
    startCall
  };
}

function renderMedia(
  container: HTMLDivElement,
  mode: CallMode,
  localStream: MediaStream,
  remoteStream: MediaStream
) {
  container.replaceChildren();
  if (mode === "video") {
    const remoteVideo = document.createElement("video");
    remoteVideo.autoplay = true;
    remoteVideo.className = "webtui-webrtc-call__remote-video";
    remoteVideo.playsInline = true;
    remoteVideo.srcObject = remoteStream;

    const localVideo = document.createElement("video");
    localVideo.autoplay = true;
    localVideo.className = "webtui-webrtc-call__local-video";
    localVideo.muted = true;
    localVideo.playsInline = true;
    localVideo.srcObject = localStream;
    container.append(remoteVideo, localVideo);
    void remoteVideo.play().catch(() => undefined);
    void localVideo.play().catch(() => undefined);
    return;
  }

  const audioState = document.createElement("div");
  audioState.className = "webtui-webrtc-call__audio-state";
  audioState.textContent = "Cuộc gọi thoại";
  const remoteAudio = document.createElement("audio");
  remoteAudio.autoplay = true;
  remoteAudio.className = "webtui-webrtc-call__remote-audio";
  remoteAudio.srcObject = remoteStream;
  container.append(audioState, remoteAudio);
  void remoteAudio.play().catch(() => undefined);
}

function attachRemoteStream(
  container: HTMLDivElement,
  mode: CallMode,
  stream: MediaStream
) {
  const selector =
    mode === "video"
      ? ".webtui-webrtc-call__remote-video"
      : ".webtui-webrtc-call__remote-audio";
  const media = container.querySelector<HTMLMediaElement>(selector);
  if (!media) {
    return;
  }
  media.srcObject = stream;
  void media.play().catch(() => undefined);
}

function isCallFinished(status: WebRtcCallStatus): boolean {
  return status === "idle" || status === "ended" || status === "error";
}

function delay(milliseconds: number): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, milliseconds);
  });
}

function newCallId(): string {
  return `call-${Date.now().toString(36)}-${Math.random()
    .toString(36)
    .slice(2, 10)}`;
}

function rejectionMessage(reason: string | undefined): string {
  if (reason === "busy") {
    return "Người nhận đang bận trong cuộc gọi khác.";
  }
  if (reason === "declined") {
    return "Người nhận đã từ chối cuộc gọi.";
  }
  return "Cuộc gọi đã kết thúc.";
}

function callErrorMessage(error: unknown): string {
  if (error instanceof DOMException) {
    if (
      error.name === "NotAllowedError" ||
      error.name === "PermissionDeniedError"
    ) {
      return "Cần cấp quyền camera và microphone để thực hiện cuộc gọi.";
    }
    if (error.name === "NotFoundError") {
      return "Không tìm thấy camera hoặc microphone phù hợp.";
    }
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return "Không thể bắt đầu cuộc gọi.";
}
