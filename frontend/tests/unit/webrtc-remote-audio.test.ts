import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const source = readFileSync(
  resolve(
    process.cwd(),
    "apps/web/src/features/chat/hooks/use-webrtc-call.ts"
  ),
  "utf8"
);

describe("web WebRTC remote audio playback", () => {
  it("renders a dedicated remote audio element for video calls", () => {
    expect(source).toContain('document.createElement("audio")');
    expect(source).toContain("remoteVideo.muted = true");
    expect(source).toContain(
      'remoteAudio.className = "webtui-webrtc-call__remote-audio"'
    );
    expect(source).toContain(
      "container.append(remoteVideo, remoteAudio, localVideo)"
    );
  });

  it("reattaches remote video-call streams to both video and audio elements", () => {
    expect(source).toContain('".webtui-webrtc-call__remote-video"');
    expect(source).toContain('".webtui-webrtc-call__remote-audio"');
    expect(source).toContain("selectors.forEach((selector) => {");
    expect(source).toContain("media.srcObject = stream");
  });
});
