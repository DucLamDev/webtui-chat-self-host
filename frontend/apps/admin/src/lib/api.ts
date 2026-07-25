import {
  createRuntimeEnvironment,
  createWebTuiApiClient
} from "@webtui/api-client";
import { useAuthStore } from "@/features/auth/auth-store";

export const runtimeEnvironment = createRuntimeEnvironment({
  NEXT_PUBLIC_API_BASE_URL: process.env.NEXT_PUBLIC_API_BASE_URL,
  NEXT_PUBLIC_APP_VERSION: process.env.NEXT_PUBLIC_APP_VERSION,
  NEXT_PUBLIC_WS_BASE_URL: process.env.NEXT_PUBLIC_WS_BASE_URL,
  NEXT_PUBLIC_APP_NAME: process.env.NEXT_PUBLIC_APP_NAME,
  NEXT_PUBLIC_DEFAULT_LOCALE: process.env.NEXT_PUBLIC_DEFAULT_LOCALE,
  NEXT_PUBLIC_RELEASE_CHANNEL: process.env.NEXT_PUBLIC_RELEASE_CHANNEL,
  NEXT_PUBLIC_RTC_ICE_SERVERS: process.env.NEXT_PUBLIC_RTC_ICE_SERVERS
});

let refreshRequest: Promise<string | null> | null = null;

export const api = createWebTuiApiClient({
  baseUrl: apiBaseUrl,
  getAccessToken: () => useAuthStore.getState().accessToken,
  onUnauthorized: () => useAuthStore.getState().clearSession(),
  refreshAccessToken: () => {
    const refreshToken = useAuthStore.getState().refreshToken;

    if (!refreshToken) {
      return Promise.resolve(null);
    }

    if (!refreshRequest) {
      refreshRequest = api.auth
        .refresh({
          domain:
            typeof window !== "undefined"
              ? window.location.hostname
              : undefined,
          refresh_token: refreshToken
        })
        .then((result) => {
          useAuthStore.getState().setSession(result);
          return result.tokens?.access_token ?? result.access_token ?? null;
        })
        .catch(() => {
          useAuthStore.getState().clearSession();
          return null;
        })
        .finally(() => {
          refreshRequest = null;
        });
    }

    return refreshRequest;
  }
});

function apiBaseUrl() {
  if (typeof window === "undefined") {
    return runtimeEnvironment.apiBaseUrl;
  }
  const { hostname, origin, protocol } = window.location;
  const isLocal =
    hostname === "localhost" ||
    hostname === "127.0.0.1" ||
    hostname === "::1" ||
    hostname.endsWith(".localhost") ||
    protocol === "tauri:";
  return isLocal ? runtimeEnvironment.apiBaseUrl : origin;
}
