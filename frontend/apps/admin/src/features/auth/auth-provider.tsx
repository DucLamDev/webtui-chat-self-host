"use client";

import { createContext, type FormEvent, type ReactNode, useContext, useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Skeleton } from "@webtui/ui";
import { ArrowRight, CheckCircle2, LockKeyhole, ShieldCheck, Sparkles } from "@webtui/icons";
import type { AuthUser, LoginInput, OIDCProviderSummary, RegisterInput } from "@webtui/types";
import { ApiClientError, queryKeys } from "@webtui/api-client";
import { api, runtimeEnvironment } from "@/lib/api";
import { useAuthStore } from "./auth-store";

type AuthContextValue = {
  isAuthenticated: boolean;
  logout: () => void;
  organizationLogo?: string;
  organizationName: string;
  user: AuthUser | null;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const accessToken = useAuthStore((state) => state.accessToken);
  const hydrated = useAuthStore((state) => state.hydrated);
  const refreshToken = useAuthStore((state) => state.refreshToken);
  const user = useAuthStore((state) => state.user);
  const clearSession = useAuthStore((state) => state.clearSession);
  const setSession = useAuthStore((state) => state.setSession);
  const setUser = useAuthStore((state) => state.setUser);
  const [formError, setFormError] = useState<string | null>(null);
  const [isCompletingOIDC, setIsCompletingOIDC] = useState(false);
  const oidcCompletionStarted = useRef(false);
  const previousAccessToken = useRef<string | null>(null);
  const discoveryQuery = useQuery({
    enabled: hydrated,
    queryFn: () => api.tenancy.discover(browserDomain()),
    queryKey: queryKeys.tenancy.discovery(browserDomain()),
    refetchOnWindowFocus: "always",
    retry: false
  });
  const organizationName = discoveryQuery.data?.runtime.app_name || runtimeEnvironment.appName || "Tổ chức";
  const organizationLogo = resolveOrganizationLogo(
    discoveryQuery.data?.runtime.logo_url || discoveryQuery.data?.zone.logo_url,
    discoveryQuery.data?.runtime.api_base_url
  );

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
    api.auth.oidcComplete({
      code,
      device_name: `${organizationName} Admin Panel`,
      domain: browserDomain()
    })
      .then((result) => {
        setSession(result);
        void queryClient.invalidateQueries({ queryKey: queryKeys.auth.me });
      })
      .catch((error) => {
        setFormError(error instanceof Error ? error.message : "Đăng nhập SSO không thành công.");
      })
      .finally(() => setIsCompletingOIDC(false));
  }, [accessToken, hydrated, organizationName, queryClient, setSession]);

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
    if (
      meQuery.error instanceof ApiClientError &&
      (meQuery.error.status === 401 || meQuery.error.status === 403)
    ) {
      clearSession();
    }
  }, [clearSession, meQuery.error]);

  useEffect(() => {
    if (!hydrated) {
      return;
    }

    if (previousAccessToken.current && !accessToken) {
      queryClient.clear();
    }
    previousAccessToken.current = accessToken;
  }, [accessToken, hydrated, queryClient]);

  const loginMutation = useMutation({
    mutationFn: async (input: LoginInput) => {
      const discovery = await api.tenancy.discover(input.domain || browserDomain());
      const result = await api.auth.login(input);
      if (discovery.zone.status === "suspended") {
        const recoveryAccessToken = result.tokens?.access_token;
        if (!recoveryAccessToken) {
          throw new Error("Phiên đăng nhập không có access token để resume zone.");
        }
        await api.tenancy.setZoneLifecycle(
          "resume",
          "zone owner recovery login",
          recoveryAccessToken
        );
      }
      return result;
    },
    onError: (error) => setFormError(error instanceof Error ? error.message : "Đăng nhập không thành công."),
    onMutate: () => setFormError(null),
    onSuccess: (result) => {
      setSession(result);
      void queryClient.invalidateQueries({ queryKey: queryKeys.auth.me });
    }
  });

  const setupMutation = useMutation({
    mutationFn: (input: RegisterInput) => api.auth.register(input),
    onError: (error) => setFormError(error instanceof Error ? error.message : "Không thể tạo tài khoản quản trị."),
    onMutate: () => setFormError(null),
    onSuccess: (result) => {
      setSession(result);
      void Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.auth.me }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tenancy.discovery(browserDomain()) })
      ]);
    }
  });

  const adminAccessQuery = useQuery({
    enabled: hydrated && Boolean(accessToken) && Boolean(meQuery.data || user),
    queryFn: async () => {
      const workspaces = await api.workspaces.listMine();
      for (const workspace of workspaces) {
        const permissions = await api.rbac.myPermissions(workspace.id);
        if (permissions.some((permission) => permission.code === "admin.view")) {
          return true;
        }
      }
      return false;
    },
    queryKey: ["admin-access", meQuery.data?.id || user?.id || "anonymous"],
    retry: false
  });

  const logoutMutation = useMutation({
    mutationFn: () =>
      refreshToken ? api.auth.logout({ refresh_token: refreshToken }) : Promise.resolve({ status: "ok" }),
    onSettled: () => {
      clearSession();
      queryClient.clear();
    }
  });

  const value = useMemo<AuthContextValue>(
    () => ({
      isAuthenticated: Boolean(accessToken),
      logout: () => logoutMutation.mutate(),
      organizationLogo,
      organizationName,
      user
    }),
    [accessToken, logoutMutation, organizationLogo, organizationName, user]
  );

  if (!hydrated) {
    return <AuthLoadingState label="Đang khởi tạo phiên quản trị..." />;
  }

  if (!accessToken && discoveryQuery.isLoading) {
    return <AuthLoadingState label="Đang chuẩn bị trang quản trị..." />;
  }

  if (!accessToken) {
    return (
      <AdminLoginScreen
        error={formError}
        isPending={loginMutation.isPending || setupMutation.isPending || isCompletingOIDC}
        organizationLogo={organizationLogo}
        organizationName={organizationName}
        setupRequired={discoveryQuery.data?.setup_required === true}
        onOIDCDiscover={() => api.auth.oidcProviders(browserDomain())}
        onOIDCStart={async (providerId) => {
          setFormError(null);
          const result = await api.auth.oidcStart({
            device_name: `${organizationName} Admin Panel`,
            domain: browserDomain(),
            provider_id: providerId,
            return_to: browserOIDCReturnTo()
          });
          window.location.assign(result.authorization_url);
        }}
        onSubmit={(identifier, password) => loginMutation.mutate({
          device_name: `${organizationName} Admin Panel`,
          domain: browserDomain(),
          identifier,
          password
        })}
        onSetup={(profile) => setupMutation.mutate({
          ...profile,
          device_name: `${organizationName} - Trang quản trị`,
          domain: browserDomain()
        })}
      />
    );
  }

  if ((meQuery.isLoading && !user) || adminAccessQuery.isLoading) {
    return <AuthLoadingState label="Đang tải hồ sơ quản trị..." />;
  }

  const accessCheckError = meQuery.error || adminAccessQuery.error;
  if (accessCheckError) {
    return (
      <main className="admin-access-denied" role="alert">
        <span><ShieldCheck size={32} /></span>
        <h1>Chưa thể kiểm tra quyền quản trị</h1>
        <p>{accessCheckError instanceof Error ? accessCheckError.message : "Kết nối API đang gián đoạn. Phiên đăng nhập của bạn vẫn được giữ an toàn."}</p>
        <div className="admin-actions">
          <Button onClick={() => void Promise.all([meQuery.refetch(), adminAccessQuery.refetch()])}>Thử lại</Button>
          <Button onClick={() => logoutMutation.mutate()} variant="secondary">Đăng xuất</Button>
        </div>
      </main>
    );
  }

  if (adminAccessQuery.data !== true) {
    return (
      <main className="admin-access-denied">
        <span><ShieldCheck size={32} /></span>
        <h1>Không có quyền quản trị</h1>
        <p>Tài khoản này không được cấp quyền <code>admin.view</code> trong bất kỳ workspace nào.</p>
        <Button onClick={() => logoutMutation.mutate()} variant="secondary">Quay lại đăng nhập</Button>
      </main>
    );
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

function browserDomain() {
  if (typeof window !== "undefined" && window.location.hostname) {
    return window.location.hostname;
  }
  return "chat.vpsttt.com";
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

function resolveOrganizationLogo(rawLogoURL?: string, apiBaseURL?: string) {
  const value = rawLogoURL?.trim();
  if (!value) {
    return undefined;
  }
  try {
    const baseURL = apiBaseURL?.trim() || (typeof window !== "undefined" ? window.location.origin : runtimeEnvironment.apiBaseUrl);
    return new URL(value, baseURL).toString();
  } catch {
    return value;
  }
}

function AdminLoginScreen({
  error,
  isPending,
  onOIDCDiscover,
  onOIDCStart,
  onSetup,
  onSubmit,
  organizationLogo,
  organizationName,
  setupRequired
}: {
  error: string | null;
  isPending: boolean;
  onOIDCDiscover: () => Promise<OIDCProviderSummary[]>;
  onOIDCStart: (providerId: string) => Promise<void>;
  onSetup: (profile: Pick<RegisterInput, "display_name" | "email" | "password" | "username">) => void;
  onSubmit: (identifier: string, password: string) => void;
  organizationLogo?: string;
  organizationName: string;
  setupRequired: boolean;
}) {
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [setupDisplayName, setSetupDisplayName] = useState("");
  const [setupEmail, setSetupEmail] = useState("");
  const [setupUsername, setSetupUsername] = useState("");
  const [setupPassword, setSetupPassword] = useState("");
  const [setupPasswordConfirmation, setSetupPasswordConfirmation] = useState("");
  const [setupError, setSetupError] = useState<string | null>(null);
  const [isOIDCPending, setIsOIDCPending] = useState(false);
  const [oidcProviders, setOIDCProviders] = useState<OIDCProviderSummary[]>([]);
  const [selectedOIDCProvider, setSelectedOIDCProvider] = useState("");
  const [oidcError, setOIDCError] = useState<string | null>(null);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit(identifier.trim(), password);
  }

  function handleSetup(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSetupError(null);
    if (setupPassword !== setupPasswordConfirmation) {
      setSetupError("Hai mật khẩu chưa giống nhau. Vui lòng kiểm tra lại.");
      return;
    }
    onSetup({
      display_name: setupDisplayName.trim(),
      email: setupEmail.trim(),
      password: setupPassword,
      username: setupUsername.trim().toLowerCase()
    });
  }

  async function handleOIDC() {
    setOIDCError(null);
    setIsOIDCPending(true);
    try {
      if (oidcProviders.length > 1) {
        await onOIDCStart(selectedOIDCProvider);
        return;
      }
      const providers = await onOIDCDiscover();
      if (providers.length === 0) {
        setOIDCError("Zone này chưa cấu hình nhà cung cấp SSO.");
        return;
      }
      if (providers.length === 1) {
        await onOIDCStart(providers[0].id);
        return;
      }
      setOIDCProviders(providers);
      setSelectedOIDCProvider(providers[0].id);
    } catch (error) {
      setOIDCError(error instanceof Error ? error.message : "Không thể bắt đầu đăng nhập SSO.");
    } finally {
      setIsOIDCPending(false);
    }
  }

  return (
    <main className="admin-login-shell">
      <section className="admin-login-brand">
        <div className="admin-login-brand__mark">
          {organizationLogo ? <img alt="" src={organizationLogo} /> : <span>{organizationName.slice(0, 1).toUpperCase()}</span>}
        </div>
        <span className="admin-login-brand__eyebrow">TRANG QUẢN LÝ · {organizationName}</span>
        <h1>Mọi hoạt động của doanh nghiệp trong một nơi.</h1>
        <p>Theo dõi thành viên, nội dung và thiết lập chung bằng một giao diện rõ ràng, dễ sử dụng.</p>
        <div className="admin-login-brand__features">
          <span><CheckCircle2 size={19} /><b>Dễ theo dõi</b><small>Thông tin quan trọng được sắp xếp gọn gàng.</small></span>
          <span><ShieldCheck size={19} /><b>Truy cập an toàn</b><small>Chỉ người được cấp quyền mới có thể quản lý.</small></span>
        </div>
      </section>
      <section className="admin-login-panel">
        {setupRequired ? (
          <form className="admin-login-card admin-login-card--setup" onSubmit={handleSetup}>
            <div className="admin-login-card__icon"><Sparkles size={24} /></div>
            <div>
              <span className="admin-login-card__eyebrow">THIẾT LẬP LẦN ĐẦU</span>
              <h2>Tạo tài khoản quản trị</h2>
              <p>Đây sẽ là tài khoản chủ sở hữu đầu tiên của {organizationName}.</p>
            </div>
            {error || setupError ? <div className="admin-login-error" role="alert">{setupError || error}</div> : null}
            <div className="admin-login-card__fields admin-login-card__fields--two-columns">
              <label>
                Họ và tên
                <input
                  autoComplete="name"
                  autoFocus
                  maxLength={120}
                  onChange={(event) => setSetupDisplayName(event.target.value)}
                  placeholder="Nguyễn Minh Anh"
                  required
                  value={setupDisplayName}
                />
              </label>
              <label>
                Tên đăng nhập
                <input
                  autoCapitalize="none"
                  autoComplete="username"
                  minLength={3}
                  onChange={(event) => setSetupUsername(event.target.value)}
                  pattern="[a-zA-Z0-9][a-zA-Z0-9_.-]{2,39}"
                  placeholder="minhanh"
                  required
                  value={setupUsername}
                />
              </label>
            </div>
            <label>
              Email công việc
              <input
                autoCapitalize="none"
                autoComplete="email"
                onChange={(event) => setSetupEmail(event.target.value)}
                placeholder="minhanh@congty.vn"
                required
                type="email"
                value={setupEmail}
              />
            </label>
            <div className="admin-login-card__fields admin-login-card__fields--two-columns">
              <label>
                Mật khẩu
                <input
                  autoComplete="new-password"
                  minLength={8}
                  onChange={(event) => setSetupPassword(event.target.value)}
                  placeholder="Tối thiểu 8 ký tự"
                  required
                  type="password"
                  value={setupPassword}
                />
              </label>
              <label>
                Nhập lại mật khẩu
                <input
                  autoComplete="new-password"
                  minLength={8}
                  onChange={(event) => setSetupPasswordConfirmation(event.target.value)}
                  placeholder="Nhập lại mật khẩu"
                  required
                  type="password"
                  value={setupPasswordConfirmation}
                />
              </label>
            </div>
            <Button disabled={isPending} type="submit">
              {isPending ? "Đang tạo tài khoản..." : "Bắt đầu quản lý"}<ArrowRight size={17} />
            </Button>
            <small className="admin-login-card__notice">Vì lý do an toàn, biểu mẫu này sẽ tự ẩn sau khi tài khoản đầu tiên được tạo.</small>
          </form>
        ) : (
          <form className="admin-login-card" onSubmit={handleSubmit}>
            <div className="admin-login-card__icon"><LockKeyhole size={24} /></div>
            <div>
              <span className="admin-login-card__eyebrow">DÀNH CHO NGƯỜI QUẢN LÝ</span>
              <h2>Chào mừng bạn quay lại</h2>
              <p>Đăng nhập để tiếp tục quản lý {organizationName}.</p>
            </div>
            {error ? <div className="admin-login-error" role="alert">{error}</div> : null}
            <label>
              Email hoặc tên đăng nhập
              <input
                autoComplete="username"
                autoFocus
                onChange={(event) => setIdentifier(event.target.value)}
                placeholder="tenban@congty.vn"
                required
                value={identifier}
              />
            </label>
            <label>
              Mật khẩu
              <input
                autoComplete="current-password"
                minLength={8}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="Nhập mật khẩu"
                required
                type="password"
                value={password}
              />
            </label>
            <Button disabled={isPending || !identifier.trim() || !password} type="submit">
              {isPending ? "Đang kiểm tra..." : "Đăng nhập"}<ArrowRight size={17} />
            </Button>
            <div className="admin-login-card__divider"><span>hoặc</span></div>
            <div className="admin-login-card__sso">
              {oidcProviders.length > 1 ? (
                <select
                  aria-label="Nhà cung cấp SSO"
                  disabled={isPending || isOIDCPending}
                  onChange={(event) => setSelectedOIDCProvider(event.target.value)}
                  value={selectedOIDCProvider}
                >
                  {oidcProviders.map((provider) => (
                    <option key={provider.id} value={provider.id}>{provider.name}</option>
                  ))}
                </select>
              ) : null}
              <Button
                disabled={isPending || isOIDCPending}
                onClick={() => void handleOIDC()}
                type="button"
                variant="secondary"
              >
                <LockKeyhole size={17} />
                {isOIDCPending ? "Đang kết nối..." : oidcProviders.length > 1 ? "Tiếp tục với SSO" : "Đăng nhập bằng SSO"}
              </Button>
            </div>
            {oidcError ? <div className="admin-login-error" role="alert">{oidcError}</div> : null}
            <small className="admin-login-card__notice">Nếu chưa có tài khoản, vui lòng liên hệ người quản lý của doanh nghiệp.</small>
          </form>
        )}
      </section>
    </main>
  );
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
