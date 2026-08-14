"use client";

import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  useState
} from "react";
import {
  createJitsiMeetingController,
  ensureJitsiIframePermissions,
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
};

export const JitsiMeeting = forwardRef<JitsiMeetingHandle, {
  baseUrl: string;
  displayName: string;
  microphoneEnabled: boolean;
  onClose: () => void;
  roomKey: string;
  toolbarButtons: string[];
  videoEnabled: boolean;
}>(function JitsiMeeting({
  baseUrl,
  displayName,
  microphoneEnabled,
  onClose,
  roomKey,
  toolbarButtons,
  videoEnabled
}, ref) {
  const containerRef = useRef<HTMLDivElement>(null);
  const controllerRef = useRef<JitsiMeetingController | null>(null);
  const onCloseRef = useRef(onClose);
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useImperativeHandle(ref, () => ({
    leave: () => {
      if (controllerRef.current) {
        controllerRef.current.leave();
        return;
      }
      onCloseRef.current();
    }
  }), []);

  useEffect(() => {
    const parentNode = containerRef.current;
    if (!parentNode || !baseUrl || !roomKey) return;

    let cancelled = false;
    let controller: JitsiMeetingController | null = null;
    setStatus("loading");

    void loadJitsiExternalAPI(baseUrl)
      .then((ExternalAPI) => {
        if (cancelled) return;
        const meetingURL = new URL(baseUrl);
        const api = new ExternalAPI(meetingURL.host, {
          configOverwrite: {
            prejoinConfig: { enabled: false },
            startWithAudioMuted: !microphoneEnabled,
            startWithVideoMuted: !videoEnabled,
            toolbarButtons
          },
          height: "100%",
          interfaceConfigOverwrite: { TILE_VIEW_MAX_COLUMNS: 5 },
          onload: () => {
            if (!cancelled) {
              ensureJitsiIframePermissions(parentNode);
              setStatus("ready");
            }
          },
          parentNode,
          roomName: roomKey,
          userInfo: { displayName },
          width: "100%"
        });
        ensureJitsiIframePermissions(parentNode, api);
        controller = createJitsiMeetingController(api, () => onCloseRef.current());
        controllerRef.current = controller;
      })
      .catch(() => {
        if (!cancelled) setStatus("error");
      });

    return () => {
      cancelled = true;
      if (controllerRef.current === controller) {
        controllerRef.current = null;
      }
      controller?.dispose();
    };
  }, [baseUrl, displayName, microphoneEnabled, roomKey, toolbarButtons, videoEnabled]);

  return (
    <div className="jitsi-meeting-stage">
      <div className="jitsi-meeting-stage__frame" ref={containerRef} />
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
