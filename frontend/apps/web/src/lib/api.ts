import {
  createRuntimeEnvironment,
  createWebTuiApiClient
} from "@webtui/api-client";
import { getPlatformServices } from "@webtui/chat-core";
import { useAuthStore } from "@/features/auth/auth-store";
import { clearMediaObjectUrlCache } from "@/features/chat/model/media-cache";

// Keep the API client and the health indicator on the same runtime target.
// Calling createRuntimeEnvironment() without this source silently falls back to
// the production API, even when NEXT_PUBLIC_API_BASE_URL is configured for a
// local/staging frontend build.
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
  baseUrl: () =>
    useAuthStore.getState().zoneRuntime?.api_base_url ??
    runtimeEnvironment.apiBaseUrl,
  fetcher: getPlatformServices().fetcher,
  getAccessToken: () => useAuthStore.getState().accessToken,
  onUnauthorized: () => {
    clearMediaObjectUrlCache();
    useAuthStore.getState().clearSession();
  },
  refreshAccessToken: () => {
    const state = useAuthStore.getState();
    const refreshToken = state.refreshToken;

    if (!refreshToken) {
      return Promise.resolve(null);
    }

    if (!refreshRequest) {
      refreshRequest = api.auth
        .refresh({
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
