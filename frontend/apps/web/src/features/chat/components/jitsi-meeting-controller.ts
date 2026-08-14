export const JITSI_LEAVE_EVENTS = ["videoConferenceLeft", "readyToClose"] as const;

export type JitsiExternalAPIInstance = {
  addListener: (event: string, listener: () => void) => void;
  dispose: () => void;
  executeCommand: (command: string, ...args: unknown[]) => void;
  getIFrame?: () => HTMLIFrameElement | undefined;
  removeListener: (event: string, listener: () => void) => void;
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

export const JITSI_IFRAME_ALLOW = [
  "camera",
  "microphone",
  "display-capture",
  "fullscreen",
  "autoplay",
  "clipboard-write",
  "encrypted-media"
].join("; ");
