"use client";

import {
  forwardRef,
  type PointerEvent as ReactPointerEvent,
  useEffect,
  useImperativeHandle,
  useRef,
  useState
} from "react";
import { Edit3, Monitor, RefreshCw, Trash2 } from "@webtui/icons";
import {
  applyJitsiMeetingSubject,
  buildJitsiConfigOverwrite,
  createJitsiMeetingController,
  ensureJitsiIframePermissions,
  JITSI_INTERFACE_CONFIG_OVERWRITE,
  readJitsiScreenSharingState,
  type JitsiExternalAPIInstance,
  type JitsiMeetingController
} from "./jitsi-meeting-controller";

type JitsiExternalAPIOptions = {
  configOverwrite: Record<string, unknown>;
  height: string;
  interfaceConfigOverwrite: Record<string, unknown>;
  onload: () => void;
  parentNode: HTMLElement;
  roomName: string;
  userInfo: { displayName: string };
  width: string;
};

type JitsiExternalAPIConstructor = new (
  domain: string,
  options: JitsiExternalAPIOptions
) => JitsiExternalAPIInstance;

declare global {
  interface Window {
    JitsiMeetExternalAPI?: JitsiExternalAPIConstructor;
  }
}

export type JitsiMeetingHandle = {
  leave: () => void;
  startRecording: () => void;
  stopRecording: () => void;
  toggleScreenShare: () => void;
};

type AnnotationPoint = { x: number; y: number };
type AnnotationStroke = { color: string; points: AnnotationPoint[] };

const ANNOTATION_COLORS = ["#2563eb", "#dc2626", "#16a34a", "#f59e0b", "#ffffff"];

export const JitsiMeeting = forwardRef<JitsiMeetingHandle, {
  baseUrl: string;
  canAnnotate?: boolean;
  displayName: string;
  meetingTitle?: string;
  microphoneEnabled: boolean;
  onClose: () => void;
  onScreenSharingChange?: (isSharing: boolean) => void;
  roomKey: string;
  toolbarButtons: string[];
  videoEnabled: boolean;
}>(function JitsiMeeting({
  baseUrl,
  canAnnotate = false,
  displayName,
  meetingTitle,
  microphoneEnabled,
  onClose,
  onScreenSharingChange,
  roomKey,
  toolbarButtons,
  videoEnabled
}, ref) {
  const containerRef = useRef<HTMLDivElement>(null);
  const apiRef = useRef<JitsiExternalAPIInstance | null>(null);
  const controllerRef = useRef<JitsiMeetingController | null>(null);
  const drawingRef = useRef<AnnotationStroke | null>(null);
  const onCloseRef = useRef(onClose);
  const onScreenSharingChangeRef = useRef(onScreenSharingChange);
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const [screenSharing, setScreenSharing] = useState(false);
  const [annotationEnabled, setAnnotationEnabled] = useState(false);
  const [annotationColor, setAnnotationColor] = useState(ANNOTATION_COLORS[0]);
  const [annotationStrokes, setAnnotationStrokes] = useState<AnnotationStroke[]>([]);
  const [draftAnnotation, setDraftAnnotation] = useState<AnnotationStroke | null>(null);
  const normalizedMeetingTitle = meetingTitle?.trim() || "Meeting";

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    onScreenSharingChangeRef.current = onScreenSharingChange;
  }, [onScreenSharingChange]);

  useEffect(() => {
    if (!canAnnotate) {
      setAnnotationEnabled(false);
      drawingRef.current = null;
      setDraftAnnotation(null);
    }
  }, [canAnnotate]);

  useImperativeHandle(ref, () => ({
    leave: () => {
      if (controllerRef.current) {
        controllerRef.current.leave();
        return;
      }
      onCloseRef.current();
    },
    startRecording: () => {
      try {
        apiRef.current?.executeCommand("startRecording", { mode: "file" });
      } catch {
        // The self-hosted meeting server may not have a native recorder enabled.
      }
    },
    stopRecording: () => {
      try {
        apiRef.current?.executeCommand("stopRecording", "file");
      } catch {
        // The app-level recording lifecycle is still handled by our backend.
      }
    },
    toggleScreenShare: () => {
      try {
        apiRef.current?.executeCommand("toggleShareScreen");
      } catch {
        // Browser permissions or Jitsi policy can block screen capture.
      }
    }
  }), []);

  useEffect(() => {
    const parentNode = containerRef.current;
    if (!parentNode || !baseUrl || !roomKey) return;

    let cancelled = false;
    let controller: JitsiMeetingController | null = null;
    let removeScreenSharingListener: (() => void) | null = null;
    let subjectTimer: number | null = null;
    setStatus("loading");
    setScreenSharing(false);
    setAnnotationEnabled(false);
    setDraftAnnotation(null);

    void loadJitsiExternalAPI(baseUrl)
      .then((ExternalAPI) => {
        if (cancelled) return;
        const meetingURL = new URL(baseUrl);
        const handleScreenSharingStatus = (payload?: unknown) => {
          const nextScreenSharing = readJitsiScreenSharingState(payload);
          setScreenSharing(nextScreenSharing);
          onScreenSharingChangeRef.current?.(nextScreenSharing);
        };
        const api = new ExternalAPI(meetingURL.host, {
          configOverwrite: buildJitsiConfigOverwrite({
            meetingSubject: normalizedMeetingTitle,
            microphoneEnabled,
            toolbarButtons,
            videoEnabled
          }),
          height: "100%",
          interfaceConfigOverwrite: JITSI_INTERFACE_CONFIG_OVERWRITE,
          onload: () => {
            if (!cancelled) {
              ensureJitsiIframePermissions(parentNode);
              applyJitsiMeetingSubject(api, normalizedMeetingTitle);
              setStatus("ready");
            }
          },
          parentNode,
          roomName: roomKey,
          userInfo: { displayName },
          width: "100%"
        });
        apiRef.current = api;
        api.addListener("screenSharingStatusChanged", handleScreenSharingStatus);
        removeScreenSharingListener = () => api.removeListener("screenSharingStatusChanged", handleScreenSharingStatus);
        applyJitsiMeetingSubject(api, normalizedMeetingTitle);
        subjectTimer = window.setTimeout(() => applyJitsiMeetingSubject(api, normalizedMeetingTitle), 900);
        ensureJitsiIframePermissions(parentNode, api);
        controller = createJitsiMeetingController(api, () => onCloseRef.current());
        controllerRef.current = controller;
      })
      .catch(() => {
        if (!cancelled) setStatus("error");
      });

    return () => {
      cancelled = true;
      if (subjectTimer) window.clearTimeout(subjectTimer);
      removeScreenSharingListener?.();
      if (controllerRef.current === controller) {
        controllerRef.current = null;
      }
      if (apiRef.current) {
        apiRef.current = null;
      }
      controller?.dispose();
    };
  }, [baseUrl, displayName, microphoneEnabled, normalizedMeetingTitle, roomKey, toolbarButtons, videoEnabled]);

  const annotationPoint = (event: ReactPointerEvent<SVGSVGElement>): AnnotationPoint => {
    const box = event.currentTarget.getBoundingClientRect();
    return {
      x: Math.max(0, Math.min(1, (event.clientX - box.left) / box.width)),
      y: Math.max(0, Math.min(1, (event.clientY - box.top) / box.height))
    };
  };

  const startAnnotation = (event: ReactPointerEvent<SVGSVGElement>) => {
    if (!annotationEnabled || !canAnnotate) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    drawingRef.current = { color: annotationColor, points: [annotationPoint(event)] };
    setDraftAnnotation(drawingRef.current);
  };

  const moveAnnotation = (event: ReactPointerEvent<SVGSVGElement>) => {
    if (!annotationEnabled || !drawingRef.current) return;
    event.preventDefault();
    drawingRef.current = {
      ...drawingRef.current,
      points: [...drawingRef.current.points, annotationPoint(event)]
    };
    setDraftAnnotation(drawingRef.current);
  };

  const endAnnotation = () => {
    const stroke = drawingRef.current;
    if (!stroke) return;
    drawingRef.current = null;
    setDraftAnnotation(null);
    if (stroke.points.length > 1) {
      setAnnotationStrokes((current) => [...current, stroke]);
    }
  };

  const renderedAnnotations = draftAnnotation
    ? [...annotationStrokes, draftAnnotation]
    : annotationStrokes;

  return (
    <div className="jitsi-meeting-stage">
      <div className="jitsi-meeting-stage__frame" ref={containerRef} />
      {canAnnotate ? (
        <div className="jitsi-meeting-stage__tools" aria-label="Công cụ trình bày">
          <button
            aria-label={screenSharing ? "Dừng chia sẻ màn hình" : "Chia sẻ màn hình"}
            className={screenSharing ? "is-active" : ""}
            disabled={status !== "ready"}
            onClick={() => apiRef.current?.executeCommand("toggleShareScreen")}
            title={screenSharing ? "Dừng chia sẻ màn hình" : "Chia sẻ màn hình"}
            type="button"
          >
            <Monitor size={16} />
          </button>
          <button
            aria-label={annotationEnabled ? "Tắt bút vẽ" : "Bật bút vẽ"}
            className={annotationEnabled ? "is-active" : ""}
            disabled={status !== "ready"}
            onClick={() => setAnnotationEnabled((current) => !current)}
            title={annotationEnabled ? "Tắt bút vẽ" : "Vẽ trên màn hình"}
            type="button"
          >
            <Edit3 size={16} />
          </button>
          {annotationEnabled ? (
            <div className="jitsi-annotation-palette" aria-label="Màu vẽ">
              {ANNOTATION_COLORS.map((color) => (
                <button
                  aria-label={`Màu ${color}`}
                  className={annotationColor === color ? "is-active" : ""}
                  key={color}
                  onClick={() => setAnnotationColor(color)}
                  style={{ background: color }}
                  type="button"
                />
              ))}
              <button
                aria-label="Hoàn tác nét vẽ"
                disabled={!annotationStrokes.length}
                onClick={() => setAnnotationStrokes((current) => current.slice(0, -1))}
                title="Hoàn tác"
                type="button"
              >
                <RefreshCw size={15} />
              </button>
              <button
                aria-label="Xóa nét vẽ"
                disabled={!annotationStrokes.length}
                onClick={() => setAnnotationStrokes([])}
                title="Xóa nét vẽ"
                type="button"
              >
                <Trash2 size={15} />
              </button>
            </div>
          ) : null}
        </div>
      ) : null}
      {renderedAnnotations.length || annotationEnabled ? (
        <svg
          aria-hidden="true"
          className={`jitsi-annotation-layer${annotationEnabled ? " is-drawing" : ""}`}
          onPointerCancel={endAnnotation}
          onPointerDown={startAnnotation}
          onPointerLeave={endAnnotation}
          onPointerMove={moveAnnotation}
          onPointerUp={endAnnotation}
          viewBox="0 0 1000 1000"
        >
          {renderedAnnotations.map((stroke, index) => (
            <polyline
              fill="none"
              key={`${index}-${stroke.points.length}`}
              points={stroke.points.map((point) => `${point.x * 1000},${point.y * 1000}`).join(" ")}
              stroke={stroke.color}
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="7"
            />
          ))}
        </svg>
      ) : null}
      {status !== "ready" ? (
        <div className={`jitsi-meeting-stage__status${status === "error" ? " is-error" : ""}`}>
          <strong>{status === "error" ? "Chưa thể mở phòng họp" : "Đang mở phòng họp..."}</strong>
          <span>{status === "error" ? "Vui lòng đóng cửa sổ này và thử lại sau ít phút." : "Kết nối an toàn đang được chuẩn bị."}</span>
        </div>
      ) : null}
    </div>
  );
});

const jitsiScriptLoads = new Map<string, Promise<JitsiExternalAPIConstructor>>();

function loadJitsiExternalAPI(baseUrl: string) {
  const scriptURL = new URL("external_api.js", `${baseUrl.replace(/\/+$/, "")}/`).toString();
  const cached = jitsiScriptLoads.get(scriptURL);
  if (cached) return cached;

  const load = new Promise<JitsiExternalAPIConstructor>((resolve, reject) => {
    if (window.JitsiMeetExternalAPI) {
      resolve(window.JitsiMeetExternalAPI);
      return;
    }

    const existing = Array.from(document.scripts).find((script) => script.src === scriptURL);
    const script = existing ?? document.createElement("script");
    const handleLoad = () => {
      if (window.JitsiMeetExternalAPI) {
        resolve(window.JitsiMeetExternalAPI);
      } else {
        reject(new Error("Jitsi External API is unavailable."));
      }
    };
    const handleError = () => reject(new Error("Could not load Jitsi External API."));

    script.addEventListener("load", handleLoad, { once: true });
    script.addEventListener("error", handleError, { once: true });
    if (!existing) {
      script.async = true;
      script.src = scriptURL;
      script.dataset.webtuiJitsiApi = "true";
      document.head.appendChild(script);
    }
  });

  jitsiScriptLoads.set(scriptURL, load);
  void load.catch(() => {
    if (jitsiScriptLoads.get(scriptURL) === load) {
      jitsiScriptLoads.delete(scriptURL);
    }
  });
  return load;
}
