import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ApiClientError,
  createAuthClient,
  createCallsClient,
  createFilesClient,
  HttpClient,
  isLocalHostname,
  localizeZoneRuntime,
  serverDiscoveryBaseUrl,
  zoneWebNavigationTarget,
} from "@webtui/api-client";

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    headers: {
      "content-type": "application/json",
    },
    status,
  });
}

describe("HttpClient", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("unwraps API envelopes and attaches auth/query headers", async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({
        data: { name: "Kỹ thuật" },
        success: true,
        timestamp: "2026-07-09T00:00:00Z",
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new HttpClient({
      baseUrl: "https://chat.vpsttt.com/",
      getAccessToken: () => "access-token",
    });

    const result = await client.get<{ name: string }>("/api/v1/channels", {
      query: {
        empty: "",
        q: "kỹ thuật",
        tag: ["chat", "ops"],
      },
    });

    expect(result).toEqual({ name: "Kỹ thuật" });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(
      "https://chat.vpsttt.com/api/v1/channels?q=k%E1%BB%B9+thu%E1%BA%ADt&tag=chat&tag=ops",
    );
    expect((init.headers as Headers).get("Authorization")).toBe(
      "Bearer access-token",
    );
    expect((init.headers as Headers).get("Accept")).toBe("application/json");
  });

  it("serializes JSON bodies without overriding FormData content type", async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({
        data: { id: "msg-1" },
        success: true,
        timestamp: "2026-07-09T00:00:00Z",
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = new HttpClient({ baseUrl: "https://chat.vpsttt.com" });

    await client.post("/api/v1/messages", { body: "Xin chào" });

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.body).toBe(JSON.stringify({ body: "Xin chào" }));
    expect((init.headers as Headers).get("Content-Type")).toBe(
      "application/json",
    );
  });

  it("binds the native fetch receiver when no fetch adapter is provided", async () => {
    const fetchMock = vi.fn(function (this: unknown) {
      if (this !== globalThis) {
        throw new TypeError("Illegal invocation");
      }
      return Promise.resolve(
        jsonResponse({ data: { ok: true }, success: true }),
      );
    });
    vi.stubGlobal("fetch", fetchMock);
    const client = new HttpClient({ baseUrl: "https://chat.vpsttt.com" });

    await expect(client.get<{ ok: boolean }>("/health")).resolves.toEqual({
      ok: true,
    });
  });

  it("refreshes once after a 401 response and retries the request", async () => {
    let token = "expired-token";
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse(
          {
            error: { code: "UNAUTHORIZED", message: "Hết phiên đăng nhập." },
            success: false,
            timestamp: "2026-07-09T00:00:00Z",
          },
          401,
        ),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: { ok: true },
          success: true,
          timestamp: "2026-07-09T00:00:00Z",
        }),
      );
    const refreshAccessToken = vi.fn(async () => {
      token = "fresh-token";
      return token;
    });
    vi.stubGlobal("fetch", fetchMock);

    const client = new HttpClient({
      baseUrl: "https://chat.vpsttt.com",
      getAccessToken: () => token,
      refreshAccessToken,
    });

    await expect(client.get<{ ok: boolean }>("/api/v1/me")).resolves.toEqual({
      ok: true,
    });
    expect(refreshAccessToken).toHaveBeenCalledTimes(1);
    expect(
      (fetchMock.mock.calls[1][1].headers as Headers).get("Authorization"),
    ).toBe("Bearer fresh-token");
  });

  it("throws ApiClientError with backend request id", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        jsonResponse(
          {
            error: {
              code: "VALIDATION_ERROR",
              details: { field: "name" },
              message: "Tên không hợp lệ.",
            },
            request_id: "req-123",
            success: false,
            timestamp: "2026-07-09T00:00:00Z",
          },
          422,
        ),
      ),
    );

    const client = new HttpClient({ baseUrl: "https://chat.vpsttt.com" });

    await expect(
      client.get("/api/v1/workspaces"),
    ).rejects.toMatchObject<ApiClientError>({
      code: "VALIDATION_ERROR",
      details: { field: "name" },
      message: "Tên không hợp lệ.",
      requestId: "req-123",
      status: 422,
    });
  });

  it("returns binary blobs without envelope parsing", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("file-content", { status: 200 })),
    );

    const client = new HttpClient({ baseUrl: "https://chat.vpsttt.com" });
    const blob = await client.blob("/api/v1/files/file-1/download");

    await expect(blob.text()).resolves.toBe("file-content");
  });

  it("loads conversation media from the channel media endpoint", async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({
        data: {
          attachments: [
            {
              file: {
                id: "file-1",
                mime_type: "image/png",
                original_name: "anh.png",
              },
              file_id: "file-1",
              message_id: "message-1",
              workspace_id: "workspace-1",
            },
          ],
        },
        success: true,
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const files = createFilesClient(
      new HttpClient({ baseUrl: "https://chat.vpsttt.com" }),
    );

    const media = await files.channelMedia("workspace-1", "channel-1", {
      limit: 500,
    });

    expect(media).toHaveLength(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "https://chat.vpsttt.com/api/v1/workspaces/workspace-1/channels/channel-1/media?limit=500",
    );
  });

  it("uses the public OIDC discovery, start, and completion contract", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          data: { oidc_providers: [{ id: "provider-1", name: "Company SSO" }] },
          success: true,
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            authorization_url: "https://identity.example/authorize",
            expires_at: "2026-07-23T12:10:00Z",
          },
          success: true,
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            session_id: "session-1",
            tokens: { access_token: "access-token", token_type: "Bearer" },
          },
          success: true,
        }),
      );
    vi.stubGlobal("fetch", fetchMock);
    const auth = createAuthClient(
      new HttpClient({ baseUrl: "https://chat.company.example" }),
    );

    await expect(auth.oidcProviders("chat.company.example")).resolves.toEqual([
      { id: "provider-1", name: "Company SSO" },
    ]);
    await auth.oidcStart({
      domain: "chat.company.example",
      provider_id: "provider-1",
      return_to: "/",
    });
    await auth.oidcComplete({
      code: "one-time-code",
      domain: "chat.company.example",
    });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "https://chat.company.example/api/v1/auth/oidc/providers?domain=chat.company.example",
    );
    expect(JSON.parse(String(fetchMock.mock.calls[1]?.[1].body))).toMatchObject(
      {
        provider_id: "provider-1",
        return_to: "/",
      },
    );
    expect(JSON.parse(String(fetchMock.mock.calls[2]?.[1].body))).toEqual({
      code: "one-time-code",
      domain: "chat.company.example",
    });
  });

  it("sends authenticated WebRTC signals through the call API", async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({
        data: { delivered: true },
        success: true,
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const calls = createCallsClient(
      new HttpClient({ baseUrl: "https://chat.company.example" }),
    );

    await calls.signal("workspace-1", "call-1", "ready", {});

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "https://chat.company.example/api/v1/workspaces/workspace-1/calls/call-1/signals",
    );
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1].body))).toEqual({
      payload: {},
      signal_type: "ready",
    });
  });
});

describe("zone runtime helpers", () => {
  it("redirects a browser to the selected self-hosted server", () => {
    expect(
      zoneWebNavigationTarget(
        "https://chat.customer.example/",
        "https://chat.vpsttt.com/login?source=control",
      ),
    ).toBe("https://chat.customer.example/");
  });

  it("builds secure discovery origins for self-hosted domains", () => {
    expect(
      serverDiscoveryBaseUrl(
        "chat.company.example",
        "https://fallback.example",
      ),
    ).toBe("https://chat.company.example");
    expect(
      serverDiscoveryBaseUrl(
        "http://chat.company.example",
        "https://fallback.example",
      ),
    ).toBe("https://chat.company.example");
  });

  it("reuses the configured API port for local self-hosted development", () => {
    expect(serverDiscoveryBaseUrl("127.0.0.1", "http://127.0.0.1:8080")).toBe(
      "http://127.0.0.1:8080",
    );
  });

  it("rejects server URLs with credentials or paths", () => {
    expect(() =>
      serverDiscoveryBaseUrl(
        "https://user:pass@chat.company.example",
        "https://fallback.example",
      ),
    ).toThrow();
    expect(() =>
      serverDiscoveryBaseUrl(
        "https://chat.company.example/chat",
        "https://fallback.example",
      ),
    ).toThrow();
  });

  it("does not redirect local development, same-origin, or insecure public targets", () => {
    expect(
      zoneWebNavigationTarget(
        "https://chat.customer.example",
        "http://127.0.0.1:3000/",
      ),
    ).toBeNull();
    expect(
      zoneWebNavigationTarget(
        "https://chat.customer.example/",
        "https://chat.customer.example/login",
      ),
    ).toBeNull();
    expect(
      zoneWebNavigationTarget(
        "http://chat.customer.example",
        "https://chat.vpsttt.com/",
      ),
    ).toBeNull();
  });

  it("keeps local API and WebSocket ports from the configured runtime", () => {
    expect(isLocalHostname("app.localhost")).toBe(true);
    expect(
      localizeZoneRuntime(
        {
          app_name: "VPSTTT Chat",
          app_version: "1.0.0",
          release_channel: "development",
          locale: "vi-VN",
          web_base_url: "https://chat.customer.example",
          api_base_url: "https://chat.customer.example",
          ws_base_url: "wss://chat.customer.example/ws",
        },
        "http://127.0.0.1:3000",
        "http://127.0.0.1:8080/",
        "ws://127.0.0.1:8080/ws/",
      ),
    ).toMatchObject({
      web_base_url: "http://127.0.0.1:3000",
      api_base_url: "http://127.0.0.1:8080",
      ws_base_url: "ws://127.0.0.1:8080/ws",
    });
  });

  it("keeps discovered endpoints when build defaults are public", () => {
    expect(
      localizeZoneRuntime(
        {
          app_name: "Company Chat",
          app_version: "1.0.0",
          release_channel: "stable",
          locale: "vi-VN",
          web_base_url: "https://chat.company.example",
          api_base_url: "https://chat.company.example",
          ws_base_url: "wss://chat.company.example/ws",
        },
        "http://127.0.0.1:3000",
        "https://chat.vpsttt.com",
        "wss://chat.vpsttt.com/ws",
      ),
    ).toMatchObject({
      api_base_url: "https://chat.company.example",
      web_base_url: "http://127.0.0.1:3000",
      ws_base_url: "wss://chat.company.example/ws",
    });
  });
});
