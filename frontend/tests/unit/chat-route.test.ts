import { afterEach, describe, expect, it } from "vitest";
import { createBrowserPlatformServices, setPlatformServices } from "@webtui/chat-core";
import { buildChatRoute, buildWorkspaceSectionRoute, directIdPrefix, directRouteRef, parseChatRoute } from "../../apps/web/src/lib/chat-route";

describe("chat route", () => {
  afterEach(() => {
    setPlatformServices(createBrowserPlatformServices());
  });

  it("builds and parses readable channel routes", () => {
    const path = buildChatRoute("vpsttt", "channel", "ky-thuat");
    expect(path).toBe("/chat/vpsttt/channel/ky-thuat");
    expect(parseChatRoute(path)).toEqual({ kind: "channel", targetRef: "ky-thuat", workspaceRef: "vpsttt" });
  });

  it("builds readable module routes", () => {
    const path = buildWorkspaceSectionRoute("vpsttt", "automation");
    expect(path).toBe("/chat/vpsttt/automation");
    expect(parseChatRoute(path)?.sectionRef).toBe("automation");
  });

  it("keeps a short stable id in direct-message routes", () => {
    const reference = directRouteRef("Hồ Đức Lâm", "ff81f5d2-915c-43a5-9063-2ee7f178ee5c");
    expect(reference).toBe("ho-duc-lam--ff81f5d2");
    expect(directIdPrefix(reference)).toBe("ff81f5d2");
  });

  it("keeps the selected message list across navigation and reloads", () => {
    const path = buildChatRoute("vpsttt", undefined, undefined, "channels");
    expect(path).toBe("/chat/vpsttt?view=channels");

    const [pathname, query] = path.split("?");
    expect(parseChatRoute(pathname, new URLSearchParams(query))).toEqual({
      messageView: "channels",
      workspaceRef: "vpsttt"
    });
  });

  it("uses query routes for desktop static export", () => {
    setPlatformServices({
      ...createBrowserPlatformServices(),
      lifecycle: { isDesktop: true, platform: "tauri" }
    });

    const path = buildChatRoute("vpsttt", "channel", "ky-thuat");
    expect(path).toBe("/chat/desktop?workspace=vpsttt&kind=channel&target=ky-thuat");

    const params = new URLSearchParams(path.split("?")[1]);
    expect(parseChatRoute("/chat/desktop", params)).toEqual({
      kind: "channel",
      targetRef: "ky-thuat",
      workspaceRef: "vpsttt"
    });

    const messagePath = buildChatRoute("vpsttt", undefined, undefined, "channels");
    expect(messagePath).toBe("/chat/desktop?workspace=vpsttt&view=channels");
    const messageParams = new URLSearchParams(messagePath.split("?")[1]);
    expect(parseChatRoute("/chat/desktop", messageParams)).toEqual({
      messageView: "channels",
      workspaceRef: "vpsttt"
    });
  });
});
