export const JITSI_LEAVE_EVENTS = ["videoConferenceLeft", "readyToClose"] as const;

type JitsiEventListener = (...args: unknown[]) => void;

export type JitsiExternalAPIInstance = {
  addListener: (event: string, listener: JitsiEventListener) => void;
  dispose: () => void;
  executeCommand: (command: string, ...args: unknown[]) => void;
  getIFrame?: () => HTMLIFrameElement | undefined;
  removeListener: (event: string, listener: JitsiEventListener) => void;
};

export type JitsiMeetingController = {
  dispose: () => void;
  leave: () => void;
};

export function createJitsiMeetingController(
  api: JitsiExternalAPIInstance,
  onClose: () => void
): JitsiMeetingController {
  let closed = false;

  const closeOnce = () => {
    if (closed) return;
    closed = true;
    onClose();
  };

  for (const event of JITSI_LEAVE_EVENTS) {
    api.addListener(event, closeOnce);
  }

  return {
    dispose: () => {
      for (const event of JITSI_LEAVE_EVENTS) {
        api.removeListener(event, closeOnce);
      }
      api.dispose();
    },
    leave: () => {
      try {
        api.executeCommand("hangup");
      } finally {
        closeOnce();
      }
    }
  };
}

export function ensureJitsiIframePermissions(
  parentNode: HTMLElement,
  api?: Pick<JitsiExternalAPIInstance, "getIFrame">
) {
  const iframe = api?.getIFrame?.() ?? parentNode.querySelector("iframe");
  if (!iframe) return;

  iframe.setAttribute("allow", JITSI_IFRAME_ALLOW);
  iframe.allowFullscreen = true;
}

export function buildJitsiToolbarButtons({
  canPresent,
  canRecord = false,
  chatEnabled
}: {
  canPresent: boolean;
  canRecord?: boolean;
  chatEnabled: boolean;
}) {
  return [
    ...(canPresent ? ["microphone", "camera", "desktop", "select-background"] : []),
    ...(chatEnabled ? ["chat", "participants-pane"] : []),
    "raisehand",
    "tileview",
    "fullscreen",
    ...(canRecord ? ["recording"] : []),
    "settings",
    "security",
    "hangup"
  ];
}

export function buildJitsiConfigOverwrite({
  meetingSubject,
  microphoneEnabled,
  toolbarButtons,
  videoEnabled
}: {
  meetingSubject: string;
  microphoneEnabled: boolean;
  toolbarButtons: string[];
  videoEnabled: boolean;
}): Record<string, unknown> {
  return {
    disableDeepLinking: true,
    disableInviteFunctions: true,
    hideConferenceSubject: true,
    prejoinConfig: { enabled: false },
    startWithAudioMuted: !microphoneEnabled,
    startWithVideoMuted: !videoEnabled,
    subject: normalizeJitsiSubject(meetingSubject),
    toolbarButtons
  };
}

export const JITSI_INTERFACE_CONFIG_OVERWRITE: Record<string, unknown> = {
  APP_NAME: "WebTUI Chat",
  BRAND_WATERMARK_LINK: "",
  DEFAULT_REMOTE_DISPLAY_NAME: "Thanh vien",
  DISABLE_JOIN_LEAVE_NOTIFICATIONS: true,
  DISPLAY_WELCOME_PAGE_CONTENT: false,
  DISPLAY_WELCOME_PAGE_TOOLBAR_ADDITIONAL_CONTENT: false,
  HIDE_INVITE_MORE_HEADER: true,
  JITSI_WATERMARK_LINK: "",
  MOBILE_APP_PROMO: false,
  NATIVE_APP_NAME: "WebTUI Chat",
  PROVIDER_NAME: "WebTUI Chat",
  SHOW_BRAND_WATERMARK: false,
  SHOW_JITSI_WATERMARK: false,
  SHOW_POWERED_BY: false,
  SHOW_WATERMARK_FOR_GUESTS: false,
  TILE_VIEW_MAX_COLUMNS: 5
};

export function applyJitsiMeetingSubject(api: JitsiExternalAPIInstance, subject: string) {
  try {
    api.executeCommand("subject", normalizeJitsiSubject(subject));
  } catch {
    // Older/self-hosted Jitsi builds may not expose this command.
  }
}

export function readJitsiScreenSharingState(payload: unknown) {
  if (typeof payload === "boolean") {
    return payload;
  }
  if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
    return false;
  }
  const candidate = payload as { on?: unknown };
  return Boolean(candidate.on);
}

function normalizeJitsiSubject(subject: string) {
  return subject.trim() || "Meeting";
}

export const JITSI_IFRAME_ALLOW = [
  "camera",
  "microphone",
  "display-capture",
  "fullscreen",
  "autoplay",
  "clipboard-write",
  "encrypted-media"
].join("; ");
