"use client";

import { createContext, type FormEvent, type ReactNode, useContext, useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AuthScreen, Button, Input, Skeleton } from "@webtui/ui";
import type {
  AuthUser,
  GoogleLoginInput,
  LoginInput,
  RegisterInput,
  ZoneRuntime
} from "@webtui/types";
import {
  createWebTuiApiClient,
  isLocalHostname,
  localizeZoneRuntime,
  queryKeys,
  serverDiscoveryBaseUrl,
  zoneWebNavigationTarget
} from "@webtui/api-client";
import { getPlatformServices } from "@webtui/chat-core";
import { api, runtimeEnvironment } from "@/lib/api";
import { useAuthStore } from "./auth-store";
import { clearMediaObjectUrlCache } from "@/features/chat/model/media-cache";
import { isLikelyOfflineError } from "@/features/chat/model/offline-cache";

type AuthContextValue = {
  isAuthenticated: boolean;
  logout: () => void;
  user: AuthUser | null;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const accessToken = useAuthStore((state) => state.accessToken);
  const hydrated = useAuthStore((state) => state.hydrated);
  const refreshToken = useAuthStore((state) => state.refreshToken);
  const user = useAuthStore((state) => state.user);
  const zoneDomain = useAuthStore((state) => state.zoneDomain);
  const zoneRuntime = useAuthStore((state) => state.zoneRuntime);
  const clearZoneRuntime = useAuthStore((state) => state.clearZoneRuntime);
  const clearSession = useAuthStore((state) => state.clearSession);
  const setSession = useAuthStore((state) => state.setSession);
  const setRememberLogin = useAuthStore((state) => state.setRememberLogin);
  const setUser = useAuthStore((state) => state.setUser);
  const setZoneRuntime = useAuthStore((state) => state.setZoneRuntime);
  const [mode, setMode] = useState<"login" | "register">("login");
  const [formError, setFormError] = useState<string | null>(null);
  const [initialInviteToken, setInitialInviteToken] = useState("");
  const [isCompletingOIDC, setIsCompletingOIDC] = useState(false);
  const oidcCompletionStarted = useRef(false);
  const supportsBrowserOIDC = !getPlatformServices().lifecycle.isDesktop;
  const isDesktop = getPlatformServices().lifecycle.isDesktop;

  useEffect(() => {
    let mounted = true;

    void Promise.resolve(useAuthStore.persist.rehydrate()).finally(() => {
      if (mounted) {
        useAuthStore.getState().setHydrated(true);
      }
    });

    return () => {
      mounted = false;
    };
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const params = new URL(window.location.href).searchParams;
    const requestedMode = params.get("auth");
    const inviteToken = params.get("invite_token") || params.get("inviteToken") || "";
    if (inviteToken) {
      setInitialInviteToken(inviteToken);
      setMode("register");
      return;
    }
    if (requestedMode === "register" || requestedMode === "login") {
      setMode(requestedMode);
    }
  }, []);

  useEffect(() => {
    const name = zoneRuntime?.app_name?.trim();
    if (typeof document === "undefined" || !name) {
      return;
    }
    document.title = name;
    const logo = resolveBrandLogoURL(
      zoneRuntime?.logo_url,
      zoneRuntime?.api_base_url
    ) ?? organizationInitialFavicon(name);
    let icon = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
    if (!icon) {
      icon = document.createElement("link");
      icon.rel = "icon";
      document.head.append(icon);
    }
    icon.href = logo;
  }, [zoneRuntime?.api_base_url, zoneRuntime?.app_name, zoneRuntime?.logo_url]);

  useEffect(() => {
    if (!hydrated || accessToken || oidcCompletionStarted.current || typeof window === "undefined") {
      return;
    }
    const callbackURL = new URL(window.location.href);
    const code = callbackURL.searchParams.get("oidc_code");
    const providerError = callbackURL.searchParams.get("oidc_error");
    if (!code && !providerError) {
      return;
    }
    oidcCompletionStarted.current = true;
    callbackURL.searchParams.delete("oidc_code");
    callbackURL.searchParams.delete("oidc_error");
    window.history.replaceState({}, "", `${callbackURL.pathname}${callbackURL.search}${callbackURL.hash}`);
    if (!code) {
      setFormError("Đăng nhập SSO đã bị hủy hoặc không được xác minh.");
      return;
    }

    setIsCompletingOIDC(true);
    selectZone(browserDomain(), setZoneRuntime)
      .then((domain) => api.auth.oidcComplete({
        code,
        device_name: browserDeviceName(),
        domain
      }))
      .then((result) => {
        setRememberLogin(true);
        setSession(result);
        void queryClient.invalidateQueries({ queryKey: queryKeys.auth.me });
      })
      .catch((error) => {
        setFormError(error instanceof Error ? error.message : "Đăng nhập SSO không thành công.");
      })
      .finally(() => {
        setIsCompletingOIDC(false);
      });
  }, [
    accessToken,
    hydrated,
    queryClient,
    setRememberLogin,
    setSession,
    setZoneRuntime
  ]);
  const [isRestoringSession, setIsRestoringSession] = useState(false);
  const [restoreAttemptedToken, setRestoreAttemptedToken] = useState<string | null>(null);

  const meQuery = useQuery({
    enabled: hydrated && Boolean(accessToken),
    queryFn: async () => {
      const currentUser = await api.auth.me();
      return currentUser ?? api.users.me();
    },
    queryKey: queryKeys.auth.me,
    retry: false
  });

  useEffect(() => {
    if (meQuery.data) {
      setUser(meQuery.data);
    }
  }, [meQuery.data, setUser]);

  useEffect(() => {
    if (meQuery.isError && !isLikelyOfflineError(meQuery.error)) {
      clearSession();
    }
  }, [clearSession, meQuery.error, meQuery.isError]);

  useEffect(() => {
    if (!hydrated || accessToken || !refreshToken || restoreAttemptedToken === refreshToken) {
      return;
    }

    let active = true;
    setIsRestoringSession(true);
    setRestoreAttemptedToken(refreshToken);

    api.auth
      .refresh({
        refresh_token: refreshToken
      })
      .then((result) => {
        if (!active) {
          return;
        }
        setSession(result);
        void queryClient.invalidateQueries({ queryKey: queryKeys.auth.me });
      })
      .catch(() => {
        if (active) {
          clearSession();
        }
      })
      .finally(() => {
        if (active) {
          setIsRestoringSession(false);
        }
      });

    return () => {
      active = false;
    };
  }, [
    accessToken,
    clearSession,
    hydrated,
    queryClient,
    refreshToken,
    restoreAttemptedToken,
    setSession,
    zoneDomain
  ]);

  const loginMutation = useMutation({
    mutationFn: async (input: LoginInput) => {
      const { domain: requestedDomain, ...credentials } = input;
      await selectZone(requestedDomain ?? browserDomain(), setZoneRuntime, true);
      return api.auth.login(credentials);
    },
    onError: (error) => {
      if (!isZoneNavigationStarted(error)) {
        setFormError(error instanceof Error ? error.message : "Đăng nhập không thành công.");
      }
    },
    onMutate: () => setFormError(null),
    onSuccess: (result) => {
      setSession(result);
      void queryClient.invalidateQueries({ queryKey: queryKeys.auth.me });
    }
  });

  const registerMutation = useMutation({
    mutationFn: async (input: RegisterInput) => {
      const { domain: requestedDomain, ...profile } = input;
      await selectZone(requestedDomain ?? browserDomain(), setZoneRuntime, true);
      return api.auth.register(profile);
    },
    onError: (error) => {
      if (!isZoneNavigationStarted(error)) {
        setFormError(error instanceof Error ? error.message : "Đăng ký không thành công.");
      }
    },
    onMutate: () => setFormError(null),
    onSuccess: (result) => {
      setSession(result);
      void queryClient.invalidateQueries({ queryKey: queryKeys.auth.me });
    }
  });

  const googleMutation = useMutation({
    mutationFn: async (input: GoogleLoginInput) => {
      const { domain: requestedDomain, ...credential } = input;
      await selectZone(requestedDomain ?? browserDomain(), setZoneRuntime, true);
      return api.auth.google(credential);
    },
    onError: (error) => {
      if (!isZoneNavigationStarted(error)) {
        setFormError(error instanceof Error ? error.message : "Đăng nhập Google không thành công.");
      }
    },
    onMutate: () => {
      setFormError(null);
      setRememberLogin(true);
    },
    onSuccess: (result) => {
      setSession(result);
      void queryClient.invalidateQueries({ queryKey: queryKeys.auth.me });
    }
  });

  const logoutMutation = useMutation({
    mutationFn: () =>
      refreshToken ? api.auth.logout({ refresh_token: refreshToken }) : Promise.resolve({ status: "ok" }),
    onSettled: () => {
      clearMediaObjectUrlCache();
      clearSession();
      queryClient.clear();
    }
  });

  const connectServerMutation = useMutation({
    mutationFn: (domain: string) => selectZone(domain, setZoneRuntime),
    onError: (error) => {
      setFormError(error instanceof Error ? error.message : "Không thể kết nối tới máy chủ.");
    },
    onMutate: () => setFormError(null)
  });

  const value = useMemo<AuthContextValue>(
    () => ({
      isAuthenticated: Boolean(accessToken),
      logout: () => logoutMutation.mutate(),
      user
    }),
    [accessToken, logoutMutation, user]
  );

  if (!hydrated || isRestoringSession) {
    return <AuthLoadingState label="Đang khởi tạo phiên làm việc..." />;
  }

  if (!accessToken) {
    if (isDesktop && (!zoneDomain || !zoneRuntime)) {
      return (
        <ServerConnectScreen
          error={formError}
          initialDomain={zoneDomain ?? ""}
          isPending={connectServerMutation.isPending}
          onConnect={(domain) => connectServerMutation.mutate(domain)}
        />
      );
    }
    const organizationName = zoneRuntime?.app_name ?? runtimeEnvironment.appName;
    const organizationLogo = resolveBrandLogoURL(
      zoneRuntime?.logo_url,
      zoneRuntime?.api_base_url
    );
    return (
      <AuthScreen
        brandLogoAlt={organizationName}
        brandLogoSrc={organizationLogo}
        error={formError}
        googleClientId={process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID}
        initialDomain={zoneDomain ?? browserDomain()}
        initialInviteToken={initialInviteToken}
        isPending={loginMutation.isPending || registerMutation.isPending || googleMutation.isPending || isCompletingOIDC}
        mode={mode}
        onGoogleCredential={(credential, domain) =>
          googleMutation.mutate({
            credential,
            device_name: browserDeviceName(),
            domain
          })
        }
        onLogin={(values) =>
          {
            setRememberLogin(values.remember);
            loginMutation.mutate({
              device_name: browserDeviceName(),
              domain: values.domain,
              identifier: values.identifier,
              password: values.password
            });
          }
        }
        onChangeServer={isDesktop ? () => {
          setFormError(null);
          clearZoneRuntime();
        } : undefined}
        onModeChange={setMode}
        onOIDCDiscover={supportsBrowserOIDC ? async (domain) => {
          const selectedDomain = await selectZone(domain, setZoneRuntime);
          return api.auth.oidcProviders(selectedDomain);
        } : undefined}
        onOIDCStart={supportsBrowserOIDC ? async (domain, providerId) => {
          const selectedDomain = await selectZone(domain, setZoneRuntime);
          const result = await api.auth.oidcStart({
            device_name: browserDeviceName(),
            domain: selectedDomain,
            provider_id: providerId,
            return_to: browserOIDCReturnTo()
          });
          window.location.assign(result.authorization_url);
        } : undefined}
        onRegister={(values) =>
          registerMutation.mutate({
            device_name: browserDeviceName(),
            display_name: values.displayName,
            domain: values.domain,
            email: values.email,
            invite_token: values.inviteToken,
            password: values.password,
            username: values.username
          })
        }
        panelLogoAlt={organizationName}
        panelLogoSrc={organizationLogo}
        showServerField={false}
        title={organizationName}
      />
    );
  }

  if (meQuery.isLoading && !user) {
    return <AuthLoadingState label="Đang tải hồ sơ người dùng..." />;
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

function ServerConnectScreen({
  error,
  initialDomain,
  isPending,
  onConnect
}: {
  error?: string | null;
  initialDomain: string;
  isPending: boolean;
  onConnect: (domain: string) => void;
}) {
  const [domain, setDomain] = useState(initialDomain);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (domain.trim()) {
      onConnect(domain.trim());
    }
  }

  return (
    <main className="server-connect-screen" aria-label="Chọn máy chủ">
      <section className="server-connect-card">
        <div className="server-connect-card__mark" aria-hidden="true">W</div>
        <header>
          <h1>Kết nối tới máy chủ</h1>
          <p>Nhập domain của tổ chức. Ứng dụng sẽ xác minh máy chủ trước khi hiển thị trang đăng nhập.</p>
        </header>
        <form onSubmit={submit}>
          <label>
            Địa chỉ máy chủ
            <Input
              autoCapitalize="none"
              autoComplete="url"
              autoFocus
              onChange={(event) => setDomain(event.target.value)}
              placeholder="chat.example.com"
              required
              spellCheck={false}
              value={domain}
            />
          </label>
          {error ? <p className="auth-error">{error}</p> : null}
          <Button disabled={isPending || !domain.trim()} type="submit">
            {isPending ? "Đang kiểm tra..." : "Kết nối"}
          </Button>
        </form>
        <small>Có thể nhập domain hoặc URL HTTPS đầy đủ.</small>
      </section>
    </main>
  );
}

function browserDeviceName() {
  if (typeof navigator === "undefined") {
    return "Web App";
  }
  const platform = navigator.platform || "Web";
  return `Web · ${platform}`.slice(0, 120);
}

function browserDomain() {
  if (typeof window !== "undefined" && window.location.hostname) {
    return window.location.hostname;
  }

  try {
    return new URL(runtimeEnvironment.apiBaseUrl).hostname;
  } catch {
    return "chat.vpsttt.com";
  }
}

function browserOIDCReturnTo() {
  if (typeof window === "undefined") {
    return "/";
  }
  const hostname = window.location.hostname;
  const isLocal =
    hostname === "localhost" ||
    hostname === "127.0.0.1" ||
    hostname === "::1" ||
    hostname.endsWith(".localhost");
  return isLocal ? `${window.location.origin}${window.location.pathname}` : window.location.pathname;
}

async function selectZone(
  rawDomain: string,
  setZoneRuntime: ReturnType<typeof useAuthStore.getState>["setZoneRuntime"],
  navigateToWeb = false
) {
  const serverBaseUrl = serverDiscoveryBaseUrl(rawDomain, runtimeEnvironment.apiBaseUrl);
  if (
    typeof window !== "undefined" &&
    !getPlatformServices().lifecycle.isDesktop &&
    (navigateToWeb || !isLocalHostname(window.location.hostname)) &&
    navigateToZoneWeb(serverBaseUrl)
  ) {
    throw new ZoneNavigationStartedError();
  }
  const discoveryApi = createWebTuiApiClient({
    baseUrl: serverBaseUrl,
    fetcher: getPlatformServices().fetcher
  });
  const discoveryDomain = new URL(serverBaseUrl).hostname;
  const discovery = await discoveryApi.tenancy.discover(discoveryDomain);
  const discoveredRuntime = runtimeForCurrentBrowser(discovery.runtime);
  const runtime = {
    ...discoveredRuntime,
    logo_url: resolveBrandLogoURL(
      discoveredRuntime.logo_url ?? discovery.zone.logo_url,
      discoveredRuntime.api_base_url
    )
  };
  setZoneRuntime(discovery.domain, runtime);
  if (navigateToWeb && navigateToZoneWeb(runtime.web_base_url)) {
    throw new ZoneNavigationStartedError();
  }
  return discovery.domain;
}

function resolveBrandLogoURL(value?: string, apiBaseURL?: string): string | undefined {
  const logo = value?.trim();
  if (!logo) {
    return undefined;
  }
  try {
    const resolved = apiBaseURL ? new URL(logo, apiBaseURL) : new URL(logo);
    if (resolved.protocol !== "https:" && !isLocalHostname(resolved.hostname)) {
      return undefined;
    }
    if (resolved.username || resolved.password) {
      return undefined;
    }
    return resolved.toString();
  } catch {
    return undefined;
  }
}

function organizationInitialFavicon(name: string): string {
  const initial = name.trim().slice(0, 1).toUpperCase() || "O";
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="16" fill="#2563eb"/><text x="32" y="43" text-anchor="middle" font-family="Arial,sans-serif" font-size="34" font-weight="700" fill="white">${initial}</text></svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

function runtimeForCurrentBrowser(runtime: ZoneRuntime): ZoneRuntime {
  if (
    typeof window === "undefined" ||
    getPlatformServices().lifecycle.isDesktop ||
    !isLocalHostname(window.location.hostname)
  ) {
    return runtime;
  }
  return localizeZoneRuntime(
    runtime,
    window.location.origin,
    runtimeEnvironment.apiBaseUrl,
    runtimeEnvironment.wsBaseUrl
  );
}

function navigateToZoneWeb(webBaseUrl: string): boolean {
  if (
    typeof window === "undefined" ||
    getPlatformServices().lifecycle.isDesktop
  ) {
    return false;
  }
  const target = zoneWebNavigationTarget(webBaseUrl, window.location.href);
  if (!target) {
    return false;
  }
  window.location.assign(target);
  return true;
}

class ZoneNavigationStartedError extends Error {
  constructor() {
    super("Đang chuyển sang domain của zone.");
    this.name = "ZoneNavigationStartedError";
  }
}

function isZoneNavigationStarted(error: unknown): boolean {
  return error instanceof ZoneNavigationStartedError;
}

export function useAuth() {
  const value = useContext(AuthContext);

  if (!value) {
    throw new Error("useAuth phải được dùng bên trong AuthProvider.");
  }

  return value;
}

function AuthLoadingState({ label }: { label: string }) {
  return (
    <main className="auth-loading" aria-label={label}>
      <Skeleton style={{ height: 64, width: 64 }} />
      <Skeleton style={{ height: 24, width: 260 }} />
      <Skeleton style={{ height: 18, width: 360 }} />
    </main>
  );
}
