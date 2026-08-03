import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import { describe, expect, it, vi } from "vitest";

describe("Web Push service worker", () => {
  it("deduplicates event ids and rejects cross-origin notification targets", async () => {
    const listeners = new Map<string, (event: WorkerEvent) => void>();
    const cacheEntries = new Map<string, Response>();
    const showNotification = vi.fn(async (_title: string, _options: NotificationOptions) => undefined);
    const worker = {
      addEventListener: (type: string, listener: (event: WorkerEvent) => void) => listeners.set(type, listener),
      clients: {
        claim: vi.fn(async () => undefined),
        matchAll: vi.fn(async () => []),
        openWindow: vi.fn(async () => null)
      },
      location: { origin: "https://chat.example.test" },
      registration: { showNotification },
      skipWaiting: vi.fn()
    };
    const cache = {
      delete: async (request: Request) => cacheEntries.delete(request.url),
      keys: async () => [...cacheEntries.keys()].map((url) => new Request(url)),
      match: async (request: Request) => cacheEntries.get(request.url),
      put: async (request: Request, response: Response) => {
        cacheEntries.set(request.url, response);
      }
    };
    const source = readFileSync(new URL("../../apps/web/public/sw.js", import.meta.url), "utf8");
    runInNewContext(source, {
      Array,
      Promise,
      Request,
      Response,
      URL,
      caches: { open: async () => cache },
      encodeURIComponent,
      self: worker
    });

    await dispatchPush(listeners, {
      body: "Nội dung",
      data: { url: "https://evil.example/phishing" },
      id: "event-1",
      title: "Tin mới",
      type: "mention",
      version: 1
    });
    await dispatchPush(listeners, {
      body: "Nội dung lặp",
      data: { url: "/chat/workspace-1" },
      id: "event-1",
      title: "Tin lặp",
      type: "mention",
      version: 1
    });

    expect(showNotification).toHaveBeenCalledTimes(1);
    expect(showNotification.mock.calls[0]?.[1]).toMatchObject({
      data: { id: "event-1", url: "/" }
    });

    await dispatchPush(listeners, {
      body: "Hợp lệ",
      data: { url: "/chat/workspace-1/channel/channel-1?message=message-1#ignored" },
      id: "event-2",
      title: "Tin hợp lệ",
      type: "message",
      version: 1
    });
    expect(showNotification.mock.calls[1]?.[1]).toMatchObject({
      data: {
        id: "event-2",
        url: "/chat/workspace-1/channel/channel-1?message=message-1"
      }
    });
  });
});

type WorkerEvent = {
  data?: { json: () => unknown };
  waitUntil: (promise: Promise<unknown>) => void;
};

async function dispatchPush(listeners: Map<string, (event: WorkerEvent) => void>, payload: unknown) {
  let pending: Promise<unknown> | undefined;
  listeners.get("push")?.({
    data: { json: () => payload },
    waitUntil: (promise) => {
      pending = promise;
    }
  });
  await pending;
}
