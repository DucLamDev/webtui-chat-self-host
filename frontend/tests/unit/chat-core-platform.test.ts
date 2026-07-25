import { describe, expect, it } from "vitest";
import {
  getPlatformServices,
  setPlatformServices,
  type PlatformServices
} from "@webtui/chat-core";

describe("chat-core platform runtime", () => {
  it("allows desktop host to inject platform services", async () => {
    const original = getPlatformServices();
    const injected = createPlatformServices("tauri");

    try {
      setPlatformServices(injected);

      expect(getPlatformServices().lifecycle).toEqual({ isDesktop: true, platform: "tauri" });
      await expect(getPlatformServices().storage.getItem("refresh-token")).resolves.toBe("stored:refresh-token");
    } finally {
      setPlatformServices(original);
    }
  });
});

function createPlatformServices(platform: "browser" | "tauri"): PlatformServices {
  return {
    autostart: {
      disable: async () => undefined,
      enable: async () => undefined,
      isEnabled: async () => false
    },
    clipboard: {
      writeText: async () => undefined
    },
    deepLinks: {
      getInitialUrls: async () => [],
      onOpenUrl: async () => () => undefined,
      registerProtocol: async () => undefined
    },
    fetcher: fetch,
    files: {
      pickFiles: async () => [],
      saveBlob: async () => undefined
    },
    lifecycle: {
      isDesktop: platform === "tauri",
      platform
    },
    links: {
      openExternal: async () => undefined
    },
    media: {
      createAudioRecorder: async () => ({
        mimeType: "audio/webm",
        start: () => undefined,
        state: "inactive",
        stop: () => undefined
      }),
      getSupportedAudioMimeType: (candidates) => candidates[0] ?? ""
    },
    notifications: {
      getPermission: () => "unsupported",
      onClick: async () => () => undefined,
      requestPermission: async () => "unsupported",
      show: async () => undefined
    },
    storage: {
      getItem: async (key) => `stored:${key}`,
      removeItem: async () => undefined,
      setItem: async () => undefined
    },
    tray: {
      setUnreadCount: async () => undefined
    }
  };
}
