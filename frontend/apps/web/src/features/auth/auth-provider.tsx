"use client";

import { createContext, type FormEvent, type ReactNode, useContext, useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AuthScreen, Button, Input, Skeleton } from "@webtui/ui";
import type {
  AuthUser,
  CurrentLegalDocuments,
  GoogleLoginInput,
  LoginInput,
  RegisterInput,
  ZoneRuntime
} from "@webtui/types";
import {
  ApiClientError,
  createWebTuiApiClient,
  isLocalHostname,
  legalDocumentsCompatibilityError,
  localizeZoneRuntime,
  queryKeys,
  resolveCurrentLegalDocuments,
  serverDiscoveryBaseUrl,
  zoneWebNavigationTarget
} from "@webtui/api-client";
import { getPlatformServices } from "@webtui/chat-core";
import { api, refreshApiSession, runtimeEnvironment } from "@/lib/api";
import { useAuthStore } from "./auth-store";
import type { AuthAccount } from "./auth-store";
import { DesktopAppLockProvider } from "./desktop-app-lock";
import { LegalAcceptanceProvider } from "./legal-acceptance-provider";
import { legalPolicyConfig } from "./legal-policy-config";
import { clearMediaObjectUrlCache } from "@/features/chat/model/media-cache";
import {
  accountKey as offlineAccountKey,
  clearOfflineAccount,
  type OfflineAccount
} from "@/features/chat/model/message-outbox";
import { cleanupWebPushForAccount } from "@/features/notifications/web-push";

type AuthContextValue = {
  isAuthenticated: boolean;
  logout: () => void;
  user: AuthUser | null;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const accessToken = useAuthStore((state) => state.accessToken);
  const accounts = useAuthStore((state) => state.accounts);
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
  const [pendingGoogleInput, setPendingGoogleInput] = useState<GoogleLoginInput | null>(null);
  const [initialInviteToken, setInitialInviteToken] = useState("");
  const [isCompletingOIDC, setIsCompletingOIDC] = useState(false);
  const oidcCompletionStarted = useRef(false);
  const offlineAccountRef = useRef<OfflineAccount | null>(null);
  const supportsBrowserOIDC = true;
  const isDesktop = getPlatformServices().lifecycle.isDesktop;
  const legalDocumentsServer = zoneRuntime?.api_base_url ?? runtimeEnvironment.apiBaseUrl;
  const legalDocumentsQuery = useQuery({
    enabled: hydrated && !accessToken,
    queryFn: () => api.auth.legalDocuments(),
    queryKey: queryKeys.auth.legalDocuments(legalDocumentsServer),
    retry: false
  });
  const legalDocumentsResolution = useMemo(
    () => resolveCurrentLegalDocuments(legalDocumentsQuery.data),
    [legalDocumentsQuery.data]
  );
  const currentLegalDocuments: CurrentLegalDocuments | null = legalDocumentsResolution.documents;
  const registrationLegalError = legalPolicyConfig.configurationError
    ?? (legalDocumentsQuery.isError
      ? legalDocumentsQuery.error instanceof Error
        ? legalDocumentsQuery.error.message
        : "Không tải được tài liệu pháp lý từ máy chủ."
      : legalDocumentsQuery.isLoading ? null : legalDocumentsResolution.error)
    ?? (currentLegalDocuments
      ? legalDocumentsCompatibilityError(currentLegalDocuments, legalPolicyConfig)
      : null);

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

  useEffect(() => {
    if (!hydrated || accessToken || !isDesktop) {
      return;
    }
    const services = getPlatformServices();
    let disposed = false;
    let stopListening: (() => void) | undefined;

    const handleURLs = (urls: string[]) => {
      if (disposed || oidcCompletionStarted.current) {
        return;
      }
      const callback = urls.map(parseNativeOIDCCallback).find(Boolean);
      if (!callback) {
        return;
      }
      oidcCompletionStarted.current = true;
      if (callback.error || !callback.code) {
        setFormError("Đăng nhập SSO đã bị hủy hoặc không được xác minh.");
        return;
      }
      setIsCompletingOIDC(true);
      selectZone(callback.server, setZoneRuntime)
        .then((domain) => api.auth.oidcComplete({
          code: callback.code as string,
          device_name: browserDeviceName(),
          domain
        }))
        .then((result) => {
          if (!disposed) {
            setRememberLogin(true);
            setSession(result);
            void queryClient.invalidateQueries({ queryKey: queryKeys.auth.me });
          }
        })
        .catch((error) => {
          if (!disposed) {
            setFormError(error instanceof Error ? error.message : "Đăng nhập SSO không thành công.");
          }
        })
        .finally(() => {
          if (!disposed) {
            setIsCompletingOIDC(false);
          }
        });
    };

    void services.deepLinks.getInitialUrls().then(handleURLs);
    void services.deepLinks.onOpenUrl(handleURLs).then((stop) => {
      if (disposed) {
        stop();
      } else {
        stopListening = stop;
      }
    });
    return () => {
      disposed = true;
      stopListening?.();
    };
  }, [accessToken, hydrated, isDesktop, queryClient, setRememberLogin, setSession, setZoneRuntime]);
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
    if (!hydrated || !accessToken || !refreshToken) {
      return undefined;
    }

    const expiresAt = accessTokenExpiry(accessToken);
    if (!expiresAt) {
      return undefined;
    }

    let disposed = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const currentToken = accessToken;

    const schedule = (delay: number) => {
      timer = setTimeout(() => {
        if (disposed || useAuthStore.getState().accessToken !== currentToken) {
          return;
        }
        void refreshApiSession().catch(() => {
          // The session owner intentionally keeps transient refresh failures.
          // Retry while the same access token is active instead of waiting for
          // a user action to discover that the short-lived token has expired.
          if (!disposed && useAuthStore.getState().accessToken === currentToken) {
            schedule(10_000);
          }
        });
      }, Math.max(0, delay));
    };

    schedule(expiresAt - Date.now() - 60_000);
    return () => {
      disposed = true;
      if (timer) {
        clearTimeout(timer);
      }
    };
  }, [accessToken, hydrated, refreshToken]);

  useEffect(() => {
    if (!hydrated) {
      return;
    }
    const nextAccount = user?.id
      ? {
          serverId: zoneRuntime?.api_base_url ?? runtimeEnvironment.apiBaseUrl,
          userId: user.id
        }
      : null;
    const previousAccount = offlineAccountRef.current;
    if (
      previousAccount &&
      (!nextAccount || offlineAccountKey(previousAccount) !== offlineAccountKey(nextAccount))
    ) {
      void clearOfflineAccount(previousAccount).catch(() => undefined);
      void cleanupWebPushForAccount(previousAccount).catch(() => undefined);
    }
    offlineAccountRef.current = nextAccount;
  }, [hydrated, user?.id, zoneRuntime?.api_base_url]);

  useEffect(() => {
    // A temporary API/transport failure must not destroy a valid remembered
    // login. HttpClient already tries token rotation; only an explicit auth
    // rejection proves that this session is no longer usable.
    if (
      meQuery.isError &&
      meQuery.error instanceof ApiClientError &&
      (meQuery.error.status === 401 || meQuery.error.status === 403)
    ) {
      clearSession();
    }
  }, [clearSession, meQuery.error, meQuery.isError]);

  useEffect(() => {
    if (!hydrated || accessToken || !refreshToken || restoreAttemptedToken === refreshToken) {
      return;
    }

    let active = true;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let retryScheduled = false;
    setIsRestoringSession(true);
    setRestoreAttemptedToken(refreshToken);

    refreshApiSession()
      .then(() => {
        if (!active) {
          return;
        }
        void queryClient.invalidateQueries({ queryKey: queryKeys.auth.me });
      })
      .catch((error: unknown) => {
        if (!active) {
          return;
        }
        if (error instanceof ApiClientError && (error.status === 401 || error.status === 403)) {
          clearSession();
          return;
        }
        retryScheduled = true;
        retryTimer = setTimeout(() => {
          if (active) {
            setRestoreAttemptedToken(null);
          }
        }, 10_000);
      })
      .finally(() => {
        if (active && !retryScheduled) {
          setIsRestoringSession(false);
        }
      });

    return () => {
      active = false;
      if (retryTimer) {
        clearTimeout(retryTimer);
      }
    };
  }, [
    accessToken,
    clearSession,
    hydrated,
    queryClient,
    refreshToken,
    restoreAttemptedToken,
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
    onError: (error, input) => {
      if (error instanceof ApiClientError && error.code === "LEGAL_ACCEPTANCE_REQUIRED") {
        setPendingGoogleInput(input);
        setMode("register");
        setFormError("Tài khoản Google này chưa tồn tại. Hãy đọc và chấp nhận tài liệu pháp lý để tiếp tục đăng ký.");
        return;
      }
      if (!isZoneNavigationStarted(error)) {
        setFormError(error instanceof Error ? error.message : "Đăng nhập Google không thành công.");
      }
    },
    onMutate: () => {
      setFormError(null);
      setRememberLogin(true);
    },
    onSuccess: (result) => {
      setPendingGoogleInput(null);
      setSession(result);
      void queryClient.invalidateQueries({ queryKey: queryKeys.auth.me });
    }
  });

  const logoutMutation = useMutation({
    mutationFn: () =>
      refreshToken ? api.auth.logout({ refresh_token: refreshToken }) : Promise.resolve({ status: "ok" }),
    onSettled: async () => {
      const offlineAccount = user?.id
        ? {
            serverId: zoneRuntime?.api_base_url ?? runtimeEnvironment.apiBaseUrl,
            userId: user.id
          }
        : null;
      try {
        if (offlineAccount) {
          await Promise.allSettled([
            clearOfflineAccount(offlineAccount),
            cleanupWebPushForAccount(
              offlineAccount,
              (subscriptionId) => api.notifications.revokeWebPushSubscription(subscriptionId)
            )
          ]);
        }
      } finally {
        clearMediaObjectUrlCache();
        clearSession();
        queryClient.clear();
      }
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
          recentAccounts={accounts}
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
        hasPendingGoogleRegistration={Boolean(pendingGoogleInput)}
        initialDomain={zoneDomain ?? browserDomain()}
        initialInviteToken={initialInviteToken}
        isPending={loginMutation.isPending || registerMutation.isPending || googleMutation.isPending || isCompletingOIDC}
        mode={mode}
        onGoogleCredential={(credential, domain, consent) => {
          if (mode === "register" && (
            !currentLegalDocuments
            || registrationLegalError
            || !consent?.privacyAccepted
            || !consent.termsAccepted
          )) {
            setFormError("Chưa thể đăng ký vì tài liệu pháp lý chưa sẵn sàng hoặc chưa được chấp nhận đầy đủ.");
            return;
          }
          googleMutation.mutate({
            credential,
            device_name: browserDeviceName(),
            domain,
            ...(mode === "register" && currentLegalDocuments ? {
              privacy_accepted: true,
              privacy_version: currentLegalDocuments.privacy.version,
              terms_accepted: true,
              terms_version: currentLegalDocuments.terms.version
            } : {})
          });
        }}
        onGoogleRegistrationContinue={(consent) => {
          if (
            !pendingGoogleInput
            || !currentLegalDocuments
            || registrationLegalError
            || !consent.privacyAccepted
            || !consent.termsAccepted
          ) {
            setFormError("Chưa thể tiếp tục đăng ký Google vì tài liệu pháp lý chưa sẵn sàng.");
            return;
          }
          googleMutation.mutate({
            ...pendingGoogleInput,
            privacy_accepted: true,
            privacy_version: currentLegalDocuments.privacy.version,
            terms_accepted: true,
            terms_version: currentLegalDocuments.terms.version
          });
        }}
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
        onModeChange={(nextMode) => {
          setMode(nextMode);
          if (nextMode === "login") setPendingGoogleInput(null);
        }}
        onOpenLegalDocument={isDesktop
          ? (url) => getPlatformServices().links.openExternal(url)
          : undefined}
        onOIDCDiscover={supportsBrowserOIDC ? async (domain) => {
          const selectedDomain = await selectZone(domain, setZoneRuntime);
          if (useAuthStore.getState().zoneRuntime?.capabilities?.sso === false) {
            return [];
          }
          return api.auth.oidcProviders(selectedDomain);
        } : undefined}
        onOIDCStart={supportsBrowserOIDC ? async (domain, providerId) => {
          const selectedDomain = await selectZone(domain, setZoneRuntime);
          if (useAuthStore.getState().zoneRuntime?.capabilities?.sso === false) {
            throw new Error("Máy chủ này không bật đăng nhập SSO.");
          }
          const result = await api.auth.oidcStart({
            device_name: browserDeviceName(),
            domain: selectedDomain,
            provider_id: providerId,
            return_to: oidcReturnTo(selectedDomain, isDesktop)
          });
          if (isDesktop) {
            await getPlatformServices().links.openExternal(result.authorization_url);
          } else {
            window.location.assign(result.authorization_url);
          }
        } : undefined}
        onRegister={(values) => {
          if (
            !currentLegalDocuments
            || registrationLegalError
            || !values.privacyAccepted
            || !values.termsAccepted
          ) {
            setFormError("Chưa thể đăng ký vì tài liệu pháp lý chưa sẵn sàng hoặc chưa được chấp nhận đầy đủ.");
            return;
          }
          registerMutation.mutate({
            device_name: browserDeviceName(),
            display_name: values.displayName,
            domain: values.domain,
            email: values.email,
            invite_token: values.inviteToken,
            password: values.password,
            privacy_accepted: true,
            privacy_version: currentLegalDocuments.privacy.version,
            terms_accepted: true,
            terms_version: currentLegalDocuments.terms.version,
            username: values.username
          });
        }}
        panelLogoAlt={organizationName}
        panelLogoSrc={organizationLogo}
        registrationLegal={{
          error: registrationLegalError,
          isLoading: legalDocumentsQuery.isLoading || legalDocumentsQuery.isFetching,
          onRetry: () => void legalDocumentsQuery.refetch(),
          privacyUrl: legalPolicyConfig.privacyUrl,
          privacyVersion: currentLegalDocuments?.privacy.version,
          termsUrl: legalPolicyConfig.termsUrl,
          termsVersion: currentLegalDocuments?.terms.version
        }}
        showServerField={false}
        title={organizationName}
      />
    );
  }

  if (meQuery.isLoading && !user) {
    return <AuthLoadingState label="Đang tải hồ sơ người dùng..." />;
  }

  return (
    <AuthContext.Provider value={value}>
      <DesktopAppLockProvider
        active={Boolean(accessToken)}
        onLogout={() => logoutMutation.mutate()}
      >
        <LegalAcceptanceProvider>{children}</LegalAcceptanceProvider>
      </DesktopAppLockProvider>
    </AuthContext.Provider>
  );
}

function ServerConnectScreen({
  error,
  initialDomain,
  isPending,
  recentAccounts,
  onConnect
}: {
  error?: string | null;
  initialDomain: string;
  isPending: boolean;
  recentAccounts: AuthAccount[];
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
        {recentAccounts.length ? (
          <div className="server-connect-card__recent" aria-label="Máy chủ gần đây">
            <strong>Máy chủ gần đây</strong>
            {recentAccounts.map((account) => (
              <Button
                key={account.domain}
                disabled={isPending}
                onClick={() => onConnect(account.domain)}
                type="button"
                variant="secondary"
              >
                {account.runtime.app_name || account.domain}
              </Button>
            ))}
          </div>
        ) : null}
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

function oidcReturnTo(domain: string, isDesktop: boolean) {
  if (isDesktop) {
    const callback = new URL("webtui://oidc/callback");
    callback.searchParams.set("server", domain);
    return callback.toString();
  }
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
  if (!discovery.capabilities.chat) {
    throw new Error("Máy chủ này không hỗ trợ WebTui Chat.");
  }
  const discoveredRuntime = runtimeForCurrentBrowser(discovery.runtime);
  const runtime = {
    ...discoveredRuntime,
    capabilities: discovery.capabilities,
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

function parseNativeOIDCCallback(raw: string): { code?: string; error?: string; server: string } | null {
  try {
    const url = new URL(raw);
    if (url.protocol !== "webtui:" || url.hostname !== "oidc" || url.pathname !== "/callback") {
      return null;
    }
    const server = url.searchParams.get("server")?.trim() ?? "";
    if (!server) {
      return null;
    }
    return {
      code: url.searchParams.get("oidc_code") ?? undefined,
      error: url.searchParams.get("oidc_error") ?? undefined,
      server
    };
  } catch {
    return null;
  }
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

function accessTokenExpiry(token: string): number | null {
  const payload = token.split(".")[1];
  if (!payload || typeof globalThis.atob !== "function") {
    return null;
  }
  try {
    const normalized = payload.replace(/-/g, "+").replace(/_/g, "/");
    const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=");
    const parsed = JSON.parse(globalThis.atob(padded)) as { exp?: unknown };
    return typeof parsed.exp === "number" && Number.isFinite(parsed.exp)
      ? parsed.exp * 1_000
      : null;
  } catch {
    return null;
  }
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
