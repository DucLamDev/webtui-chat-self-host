import { describe, expect, it, vi } from "vitest";
import {
  buildJitsiConfigOverwrite,
  buildJitsiToolbarButtons,
  createJitsiMeetingController,
  ensureJitsiIframePermissions,
  JITSI_INTERFACE_CONFIG_OVERWRITE,
  JITSI_LEAVE_EVENTS,
  readJitsiScreenSharingState,
  type JitsiExternalAPIInstance
} from "../../apps/web/src/features/chat/components/jitsi-meeting-controller";

function fakeAPI() {
  const listeners = new Map<string, (...args: unknown[]) => void>();
  const api: JitsiExternalAPIInstance = {
    addListener: vi.fn((event, listener) => listeners.set(event, listener)),
    dispose: vi.fn(),
    executeCommand: vi.fn(),
    removeListener: vi.fn((event) => listeners.delete(event))
  };
  return { api, listeners };
}

describe("Jitsi meeting controller", () => {
  it.each(JITSI_LEAVE_EVENTS)("closes the host overlay after %s", (event) => {
    const { api, listeners } = fakeAPI();
    const onClose = vi.fn();

    createJitsiMeetingController(api, onClose);
    listeners.get(event)?.();

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("hangs up and closes immediately when the host close button is used", () => {
    const { api, listeners } = fakeAPI();
    const onClose = vi.fn();
    const controller = createJitsiMeetingController(api, onClose);

    controller.leave();
    listeners.get("readyToClose")?.();

    expect(api.executeCommand).toHaveBeenCalledWith("hangup");
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("removes event handlers and disposes the embedded meeting", () => {
    const { api } = fakeAPI();
    const controller = createJitsiMeetingController(api, vi.fn());

    controller.dispose();

    expect(api.removeListener).toHaveBeenCalledTimes(JITSI_LEAVE_EVENTS.length);
    expect(api.dispose).toHaveBeenCalledTimes(1);
  });

  it("grants iframe permissions needed by Jitsi media controls", () => {
    const iframe = {
      allowFullscreen: false,
      setAttribute: vi.fn()
    } as unknown as HTMLIFrameElement;
    const parentNode = {
      querySelector: vi.fn(() => iframe)
    } as unknown as HTMLElement;

    ensureJitsiIframePermissions(parentNode);

    expect(iframe.setAttribute).toHaveBeenCalledWith(
      "allow",
      expect.stringContaining("microphone")
    );
    expect(iframe.setAttribute).toHaveBeenCalledWith(
      "allow",
      expect.stringContaining("camera")
    );
    expect(iframe.setAttribute).toHaveBeenCalledWith(
      "allow",
      expect.stringContaining("display-capture")
    );
    expect(iframe.allowFullscreen).toBe(true);
  });

  it("builds a presenter toolbar with screen sharing and recording controls", () => {
    expect(buildJitsiToolbarButtons({
      canPresent: true,
      canRecord: true,
      chatEnabled: true
    })).toEqual(expect.arrayContaining(["desktop", "recording", "chat", "participants-pane"]));
  });

  it("keeps meeting branding and generated room keys out of the Jitsi chrome", () => {
    const config = buildJitsiConfigOverwrite({
      meetingSubject: "Sprint weekly",
      microphoneEnabled: true,
      toolbarButtons: ["desktop"],
      videoEnabled: false
    });

    expect(config.hideConferenceSubject).toBe(true);
    expect(config.subject).toBe("Sprint weekly");
    expect(config.startWithAudioMuted).toBe(false);
    expect(config.startWithVideoMuted).toBe(true);
    expect(JITSI_INTERFACE_CONFIG_OVERWRITE.SHOW_JITSI_WATERMARK).toBe(false);
    expect(JITSI_INTERFACE_CONFIG_OVERWRITE.SHOW_POWERED_BY).toBe(false);
  });

  it("reads screen sharing state from the external api event payload", () => {
    expect(readJitsiScreenSharingState({ on: true })).toBe(true);
    expect(readJitsiScreenSharingState(true)).toBe(true);
    expect(readJitsiScreenSharingState({ on: false })).toBe(false);
    expect(readJitsiScreenSharingState(null)).toBe(false);
  });
});
