"use client";

import { createContext, type FormEvent, type ReactNode, useContext, useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Skeleton } from "@webtui/ui";
import { LockKeyhole, Send, ShieldCheck } from "@webtui/icons";
import type { AuthUser, LoginInput, OIDCProviderSummary } from "@webtui/types";
import { queryKeys } from "@webtui/api-client";
import { api } from "@/lib/api";
import { useAuthStore } from "./auth-store";

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
  const clearSession = useAuthStore((state) => state.clearSession);
  const setSession = useAuthStore((state) => state.setSession);
  const setUser = useAuthStore((state) => state.setUser);
  const [formError, setFormError] = useState<string | null>(null);
  const [isCompletingOIDC, setIsCompletingOIDC] = useState(false);
  const oidcCompletionStarted = useRef(false);

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
      device_name: "VPSTTT Admin Panel",
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
  }, [accessToken, hydrated, queryClient, setSession]);

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
    if (meQuery.isError) {
      clearSession();
    }
  }, [clearSession, meQuery.isError]);

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
      user
    }),
    [accessToken, logoutMutation, user]
  );

  if (!hydrated) {
    return <AuthLoadingState label="Đang khởi tạo phiên quản trị..." />;
  }

  if (!accessToken) {
    return (
      <AdminLoginScreen
        error={formError}
        isPending={loginMutation.isPending || isCompletingOIDC}
        onOIDCDiscover={() => api.auth.oidcProviders(browserDomain())}
        onOIDCStart={async (providerId) => {
          setFormError(null);
          const result = await api.auth.oidcStart({
            device_name: "VPSTTT Admin Panel",
            domain: browserDomain(),
            provider_id: providerId,
            return_to: browserOIDCReturnTo()
          });
          window.location.assign(result.authorization_url);
        }}
        onSubmit={(identifier, password) => loginMutation.mutate({
          device_name: "VPSTTT Admin Panel",
          domain: browserDomain(),
          identifier,
          password
        })}
      />
    );
  }

  if ((meQuery.isLoading && !user) || adminAccessQuery.isLoading) {
    return <AuthLoadingState label="Đang tải hồ sơ quản trị..." />;
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

function AdminLoginScreen({
  error,
  isPending,
  onOIDCDiscover,
  onOIDCStart,
  onSubmit
}: {
  error: string | null;
  isPending: boolean;
  onOIDCDiscover: () => Promise<OIDCProviderSummary[]>;
  onOIDCStart: (providerId: string) => Promise<void>;
  onSubmit: (identifier: string, password: string) => void;
}) {
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [isOIDCPending, setIsOIDCPending] = useState(false);
  const [oidcProviders, setOIDCProviders] = useState<OIDCProviderSummary[]>([]);
  const [selectedOIDCProvider, setSelectedOIDCProvider] = useState("");
  const [oidcError, setOIDCError] = useState<string | null>(null);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit(identifier.trim(), password);
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
        <div className="admin-login-brand__mark"><ShieldCheck size={34} /></div>
        <span className="admin-login-brand__eyebrow">VPSTTT · CONTROL CENTER</span>
        <h1>Quản trị vận hành<br />tập trung và an toàn.</h1>
        <p>Không gian riêng dành cho quản trị viên theo dõi người dùng, phân quyền, bot và tự động hóa.</p>
        <div className="admin-login-brand__features">
          <span><ShieldCheck size={18} /><b>RBAC bắt buộc</b><small>Mọi tác vụ đều được kiểm tra quyền tại API.</small></span>
          <span><LockKeyhole size={18} /><b>Không cho đăng ký</b><small>Tài khoản quản trị chỉ được cấp bởi hệ thống.</small></span>
        </div>
      </section>
      <section className="admin-login-panel">
        <form className="admin-login-card" onSubmit={handleSubmit}>
          <div className="admin-login-card__icon"><LockKeyhole size={24} /></div>
          <div>
            <span className="admin-login-card__eyebrow">XÁC THỰC QUẢN TRỊ</span>
            <h2>Đăng nhập Admin Panel</h2>
          </div>
          {error ? <div className="admin-login-error" role="alert">{error}</div> : null}
          <label>
            Email hoặc tên đăng nhập
            <input
              autoComplete="username"
              autoFocus
              onChange={(event) => setIdentifier(event.target.value)}
              placeholder="admin@vpsttt.com"
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
              placeholder="Nhập mật khẩu quản trị"
              required
              type="password"
              value={password}
            />
          </label>
          <Button disabled={isPending || !identifier.trim() || !password} type="submit">
            {isPending ? "Đang xác thực..." : "Đăng nhập quản trị"}<Send size={17} />
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
          <small className="admin-login-card__notice">Không có chức năng đăng ký tại Admin Panel.</small>
        </form>
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
