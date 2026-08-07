import { describe, expect, it, vi } from "vitest";
import {
  createJitsiMeetingController,
  JITSI_LEAVE_EVENTS,
  type JitsiExternalAPIInstance
} from "../../apps/web/src/features/chat/components/jitsi-meeting-controller";

function fakeAPI() {
  const listeners = new Map<string, () => void>();
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
});
