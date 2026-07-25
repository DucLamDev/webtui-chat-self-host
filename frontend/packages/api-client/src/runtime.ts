import type { RuntimeEnvironment, RuntimeIceServer } from "@webtui/types";

export const DEFAULT_API_BASE_URL = "https://chat.vpsttt.com";
export const DEFAULT_WS_BASE_URL = "wss://chat.vpsttt.com/ws";
export const DEFAULT_APP_NAME = "WebTui Chat";
export const DEFAULT_APP_VERSION = "0.1.0";
export const DEFAULT_LOCALE = "vi-VN";
export const DEFAULT_RELEASE_CHANNEL = "stable";
export const DEFAULT_RTC_ICE_SERVERS = "stun:stun.l.google.com:19302";

type RuntimeSource = {
  NEXT_PUBLIC_API_BASE_URL?: string;
  NEXT_PUBLIC_APP_VERSION?: string;
  NEXT_PUBLIC_WS_BASE_URL?: string;
  NEXT_PUBLIC_APP_NAME?: string;
  NEXT_PUBLIC_DEFAULT_LOCALE?: string;
  NEXT_PUBLIC_RELEASE_CHANNEL?: string;
  NEXT_PUBLIC_RTC_ICE_SERVERS?: string;
};

export function createRuntimeEnvironment(
  source: RuntimeSource = {}
): RuntimeEnvironment {
  return {
    apiBaseUrl: normalizeBaseUrl(
      source.NEXT_PUBLIC_API_BASE_URL,
      DEFAULT_API_BASE_URL
    ),
    wsBaseUrl: normalizeBaseUrl(
      source.NEXT_PUBLIC_WS_BASE_URL,
      DEFAULT_WS_BASE_URL
    ),
    appName: source.NEXT_PUBLIC_APP_NAME ?? DEFAULT_APP_NAME,
    appVersion: source.NEXT_PUBLIC_APP_VERSION?.trim() || DEFAULT_APP_VERSION,
    releaseChannel: source.NEXT_PUBLIC_RELEASE_CHANNEL?.trim() || DEFAULT_RELEASE_CHANNEL,
    rtcIceServers: parseRtcIceServers(source.NEXT_PUBLIC_RTC_ICE_SERVERS),
    locale: source.NEXT_PUBLIC_DEFAULT_LOCALE ?? DEFAULT_LOCALE
  };
}

function normalizeBaseUrl(value: string | undefined, fallback: string): string {
  const selected = value?.trim() || fallback;
  return selected.endsWith("/") ? selected.slice(0, -1) : selected;
}

function parseRtcIceServers(value: string | undefined): RuntimeIceServer[] {
  const selected = value?.trim();
  if (!selected) {
    return [{ urls: DEFAULT_RTC_ICE_SERVERS }];
  }

  if (selected.startsWith("[") || selected.startsWith("{")) {
    try {
      const parsed = JSON.parse(selected) as unknown;
      const servers = Array.isArray(parsed) ? parsed : [parsed];
      const normalized = servers.filter(isIceServer);
      return normalized.length ? normalized : [{ urls: DEFAULT_RTC_ICE_SERVERS }];
    } catch {
      return [{ urls: DEFAULT_RTC_ICE_SERVERS }];
    }
  }

  const servers = selected
    .split(",")
    .map((url) => url.trim())
    .filter(Boolean)
    .map((url) => ({ urls: url }));

  return servers.length ? servers : [{ urls: DEFAULT_RTC_ICE_SERVERS }];
}

function isIceServer(value: unknown): value is RuntimeIceServer {
  if (!value || typeof value !== "object") {
    return false;
  }
  const urls = (value as { urls?: unknown }).urls;
  return typeof urls === "string" || (Array.isArray(urls) && urls.every((url) => typeof url === "string"));
}
