import type { ZoneRuntime } from "@webtui/types";

export function isLocalHostname(hostname: string): boolean {
  const value = hostname.trim().toLowerCase().replace(/^\[|\]$/g, "");
  return (
    value === "localhost" ||
    value === "127.0.0.1" ||
    value === "::1" ||
    value.endsWith(".localhost")
  );
}

export function localizeZoneRuntime(
  discovered: ZoneRuntime,
  currentOrigin: string,
  apiBaseUrl: string,
  wsBaseUrl: string
): ZoneRuntime {
  return {
    ...discovered,
    web_base_url: trimTrailingSlash(currentOrigin),
    api_base_url: trimTrailingSlash(
      isLocalRuntimeUrl(apiBaseUrl) ? apiBaseUrl : discovered.api_base_url
    ),
    ws_base_url: trimTrailingSlash(
      isLocalRuntimeUrl(wsBaseUrl) ? wsBaseUrl : discovered.ws_base_url
    )
  };
}

export function zoneWebNavigationTarget(
  webBaseUrl: string,
  currentUrl: string
): string | null {
  let current: URL;
  let target: URL;
  try {
    current = new URL(currentUrl);
    target = new URL(webBaseUrl);
  } catch {
    return null;
  }

  if (
    !["http:", "https:"].includes(current.protocol) ||
    !["http:", "https:"].includes(target.protocol) ||
    isLocalHostname(current.hostname) ||
    target.username ||
    target.password
  ) {
    return null;
  }
  if (target.protocol !== "https:" && !isLocalHostname(target.hostname)) {
    return null;
  }
  if (current.origin === target.origin) {
    return null;
  }

  target.search = "";
  target.hash = "";
  return target.toString();
}

export function serverDiscoveryBaseUrl(
  rawServer: string,
  fallbackApiBaseUrl: string
): string {
  const input = rawServer.trim();
  if (!input) {
    throw new Error("Server domain không được để trống.");
  }

  const hasScheme = /^[a-z][a-z0-9+.-]*:\/\//i.test(input);
  let target: URL;
  try {
    target = new URL(hasScheme ? input : `https://${input}`);
  } catch {
    throw new Error("Server domain không đúng định dạng.");
  }
  if (
    !["http:", "https:"].includes(target.protocol) ||
    target.username ||
    target.password ||
    (target.pathname !== "/" && target.pathname !== "")
  ) {
    throw new Error("Chỉ nhập domain hoặc URL gốc của server.");
  }

  if (isLocalHostname(target.hostname)) {
    if (hasScheme) {
      return target.origin;
    }
    try {
      const fallback = new URL(fallbackApiBaseUrl);
      if (fallback.hostname === target.hostname) {
        return fallback.origin;
      }
    } catch {
      // Fall through to the conventional local HTTP origin.
    }
    target.protocol = "http:";
    return target.origin;
  }

  target.protocol = "https:";
  return target.origin;
}

function trimTrailingSlash(value: string): string {
  const normalized = value.trim();
  return normalized.endsWith("/") ? normalized.slice(0, -1) : normalized;
}

function isLocalRuntimeUrl(value: string): boolean {
  try {
    return isLocalHostname(new URL(value).hostname);
  } catch {
    return false;
  }
}
