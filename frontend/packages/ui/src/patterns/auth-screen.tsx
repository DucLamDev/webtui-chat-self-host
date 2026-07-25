"use client";

import { type FormEvent, useEffect, useRef, useState } from "react";
import { Button } from "../components/button";
import { Input } from "../components/input";

export type LoginFormValues = {
  domain: string;
  identifier: string;
  password: string;
  remember: boolean;
};
export type RegisterFormValues = {
  displayName: string;
  domain: string;
  email: string;
  inviteToken?: string;
  password: string;
  username: string;
};
export type AuthMode = "login" | "register";
export type AuthOIDCProvider = {
  id: string;
  name: string;
};

export type AuthScreenProps = {
  brandLogoAlt?: string;
  brandLogoSrc?: string;
  error?: string | null;
  isPending?: boolean;
  googleClientId?: string;
  initialDomain?: string;
  mode: AuthMode;
  onGoogleCredential?: (credential: string, domain: string) => void;
  onOIDCDiscover?: (domain: string) => Promise<AuthOIDCProvider[]>;
  onOIDCStart?: (domain: string, providerId: string) => Promise<void> | void;
  onLogin: (values: LoginFormValues) => void;
  onModeChange: (mode: AuthMode) => void;
  onRegister: (values: RegisterFormValues) => void;
  panelLogoAlt?: string;
  panelLogoSrc?: string;
  showServerField?: boolean;
  subtitle?: string;
  title?: string;
};

export function AuthScreen({
  brandLogoAlt = "",
  brandLogoSrc,
  error,
  googleClientId,
  initialDomain = "",
  isPending = false,
  mode,
  onGoogleCredential,
  onOIDCDiscover,
  onOIDCStart,
  onLogin,
  onModeChange,
  onRegister,
  panelLogoAlt = "",
  panelLogoSrc,
  showServerField = true,
  subtitle = "Kết nối – Trò chuyện – Hiệu quả",
  title = "WEBTUI CHAT"
}: AuthScreenProps) {
  const [domain, setDomain] = useState(initialDomain);
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(true);
  const [confirmPassword, setConfirmPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [inviteToken, setInviteToken] = useState("");
  const [username, setUsername] = useState("");
  const [localError, setLocalError] = useState<string | null>(null);
  const [isGoogleReady, setIsGoogleReady] = useState(false);
  const [isOIDCPending, setIsOIDCPending] = useState(false);
  const [oidcProviders, setOIDCProviders] = useState<AuthOIDCProvider[]>([]);
  const [selectedOIDCProvider, setSelectedOIDCProvider] = useState("");
  const googleButtonRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!googleClientId || !onGoogleCredential) {
      setIsGoogleReady(false);
      return;
    }
    let cancelled = false;
    const renderGoogleButton = () => {
      if (cancelled || !window.google?.accounts?.id || !googleButtonRef.current) {
        return;
      }
      googleButtonRef.current.replaceChildren();
      window.google.accounts.id.initialize({
        callback: (response) => {
          if (response.credential) {
            setLocalError(null);
            onGoogleCredential(response.credential, domain);
          }
        },
        client_id: googleClientId
      });
      window.google.accounts.id.renderButton(googleButtonRef.current, {
        logo_alignment: "left",
        shape: "rectangular",
        size: "large",
        text: mode === "login" ? "signin_with" : "signup_with",
        theme: "outline",
        width: Math.min(420, Math.max(260, googleButtonRef.current.clientWidth || 360))
      });
      setIsGoogleReady(true);
    };
    const existing = document.querySelector<HTMLScriptElement>('script[data-webtui-google="true"]');
    if (window.google?.accounts?.id) {
      renderGoogleButton();
    } else if (existing) {
      existing.addEventListener("load", renderGoogleButton, { once: true });
    } else {
      const script = document.createElement("script");
      script.async = true;
      script.defer = true;
      script.dataset.webtuiGoogle = "true";
      script.src = "https://accounts.google.com/gsi/client";
      script.addEventListener("load", renderGoogleButton, { once: true });
      script.addEventListener("error", () => !cancelled && setLocalError("Không tải được dịch vụ đăng nhập Google."), { once: true });
      document.head.appendChild(script);
    }
    return () => {
      cancelled = true;
      existing?.removeEventListener("load", renderGoogleButton);
    };
  }, [domain, googleClientId, mode, onGoogleCredential]);

  useEffect(() => {
    setDomain(initialDomain);
    setOIDCProviders([]);
    setSelectedOIDCProvider("");
  }, [initialDomain]);

  async function handleOIDC() {
    const normalizedDomain = domain.trim();
    if (!normalizedDomain || !onOIDCDiscover || !onOIDCStart) {
      setLocalError("Vui lòng nhập server domain trước khi đăng nhập SSO.");
      return;
    }
    setLocalError(null);
    setIsOIDCPending(true);
    try {
      if (oidcProviders.length > 1) {
        if (!selectedOIDCProvider) {
          setLocalError("Vui lòng chọn nhà cung cấp SSO.");
          return;
        }
        await onOIDCStart(normalizedDomain, selectedOIDCProvider);
        return;
      }
      const providers = await onOIDCDiscover(normalizedDomain);
      if (providers.length === 0) {
        setLocalError("Domain này chưa cấu hình nhà cung cấp SSO.");
        return;
      }
      if (providers.length === 1) {
        await onOIDCStart(normalizedDomain, providers[0].id);
        return;
      }
      setOIDCProviders(providers);
      setSelectedOIDCProvider(providers[0].id);
    } catch (error) {
      setLocalError(error instanceof Error ? error.message : "Không thể bắt đầu đăng nhập SSO cho domain này.");
    } finally {
      setIsOIDCPending(false);
    }
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setLocalError(null);
    if (mode === "login") {
      onLogin({ domain, identifier, password, remember });
      return;
    }
    if (password !== confirmPassword) {
      setLocalError("Mật khẩu xác nhận không khớp.");
      return;
    }
    onRegister({
      displayName,
      domain,
      email,
      inviteToken: inviteToken.trim() || undefined,
      password,
      username
    });
  }

  return (
    <main className={`auth-screen auth-screen--${mode}`} aria-label="Xác thực WebTui Chat">
      <section className="auth-hero">
        <div className="auth-header-brand">
          <span className={brandLogoSrc ? "auth-header-brand__logo auth-header-brand__logo--image" : "auth-header-brand__logo"}>
            {brandLogoSrc ? <img alt={brandLogoAlt} src={brandLogoSrc} /> : "W"}
          </span>
          <span><strong>{title}</strong><small>{subtitle}</small></span>
        </div>
        <div className="auth-hero__copy">
          <h1>Giao tiếp thông minh,<span>kết nối không giới hạn</span></h1>
        </div>
        <div className="auth-showcase" aria-hidden="true">
          <div className="auth-visual">
            <div className="auth-product-preview">
              <div className="auth-product-preview__rail"><i /><i /><i /><i /></div>
              <div className="auth-product-preview__contacts">
                <span />
                <p><i /><b /></p>
                <p><i /><b /></p>
                <p><i /><b /></p>
              </div>
              <div className="auth-product-preview__chat">
                <header><i /><span /></header>
                <div className="auth-preview-message auth-preview-message--incoming"><i /><i /></div>
                <div className="auth-preview-message auth-preview-message--outgoing"><i /><i /></div>
                <footer><span /><b><i /><i /><i /></b></footer>
              </div>
            </div>
          </div>
          <div className="auth-bot-decoration">
            <span className="auth-bot-decoration__orbit auth-bot-decoration__orbit--one"><i /></span>
            <span className="auth-bot-decoration__orbit auth-bot-decoration__orbit--two"><i /></span>
            <span className="auth-bot-decoration__signal" />
            <span className="auth-bot-decoration__core"><i /><i /><b /></span>
            <span className="auth-bot-decoration__spark">✦</span>
          </div>
        </div>
      </section>

      <section className="auth-panel" aria-label={mode === "login" ? "Đăng nhập" : "Đăng ký"}>
        {panelLogoSrc ? (
          <div className="auth-panel__product-logo">
            <img alt={panelLogoAlt} src={panelLogoSrc} />
          </div>
        ) : (
          <span className="auth-panel__icon" aria-hidden="true">{mode === "login" ? "⌑" : "+"}</span>
        )}
        <div className="auth-panel__header">
          <h2>{mode === "login" ? "Đăng nhập" : "Tạo tài khoản mới"}</h2>
        </div>
        <form className="auth-form" onSubmit={handleSubmit}>
          {showServerField ? (
            <label>Server domain<Input autoCapitalize="none" autoComplete="url" onChange={(event) => {
              setDomain(event.target.value);
              setOIDCProviders([]);
              setSelectedOIDCProvider("");
            }} placeholder="chat.example.com" required spellCheck={false} value={domain} /></label>
          ) : null}
          {mode === "register" ? <>
            <label>Họ và tên<Input autoComplete="name" onChange={(event) => setDisplayName(event.target.value)} placeholder="Nhập họ và tên của bạn" required value={displayName} /></label>
            <label>Email công việc<Input autoComplete="email" onChange={(event) => setEmail(event.target.value)} placeholder="Nhập email công việc" required type="email" value={email} /></label>
            <label>Tên đăng nhập<Input autoComplete="username" onChange={(event) => setUsername(event.target.value)} placeholder="Nhập tên đăng nhập" required value={username} /></label>
            <label>Mã mời (nếu cần)<Input autoComplete="one-time-code" onChange={(event) => setInviteToken(event.target.value)} placeholder="Nhập mã mời của workspace" value={inviteToken} /></label>
          </> : <label>Email hoặc tên đăng nhập<Input autoComplete="username" onChange={(event) => setIdentifier(event.target.value)} placeholder="Nhập email hoặc tên đăng nhập" required value={identifier} /></label>}
          <label>Mật khẩu<Input autoComplete={mode === "login" ? "current-password" : "new-password"} minLength={8} onChange={(event) => setPassword(event.target.value)} placeholder={mode === "login" ? "Nhập mật khẩu của bạn" : "Tạo mật khẩu ít nhất 8 ký tự"} required type="password" value={password} /></label>
          {mode === "register" ? <label>Xác nhận mật khẩu<Input autoComplete="new-password" minLength={8} onChange={(event) => setConfirmPassword(event.target.value)} placeholder="Nhập lại mật khẩu" required type="password" value={confirmPassword} /></label> : <div className="auth-helper-row"><label className="auth-check"><input checked={remember} onChange={(event) => setRemember(event.target.checked)} type="checkbox" />Ghi nhớ đăng nhập</label><span>Quên mật khẩu?</span></div>}
          {localError || error ? <p className="auth-error">{localError || error}</p> : null}
          <Button className="auth-submit" disabled={isPending} type="submit">
            {isPending ? "Đang xử lý..." : mode === "login" ? "Đăng nhập" : "Đăng ký tài khoản"}
            <span className="auth-submit__arrow" aria-hidden="true">→</span>
          </Button>
        </form>
        {mode === "login" && onOIDCDiscover && onOIDCStart ? (
          <>
            <div className="auth-divider"><span>hoặc đăng nhập nội bộ</span></div>
            <div className="auth-oidc-area">
              {oidcProviders.length > 1 ? (
                <select
                  aria-label="Nhà cung cấp SSO"
                  disabled={isOIDCPending || isPending}
                  onChange={(event) => setSelectedOIDCProvider(event.target.value)}
                  value={selectedOIDCProvider}
                >
                  {oidcProviders.map((provider) => (
                    <option key={provider.id} value={provider.id}>{provider.name}</option>
                  ))}
                </select>
              ) : null}
              <Button
                className="auth-oidc-button"
                disabled={isOIDCPending || isPending}
                onClick={() => void handleOIDC()}
                variant="secondary"
              >
                {isOIDCPending ? "Đang kết nối..." : oidcProviders.length > 1 ? "Tiếp tục với SSO" : "Đăng nhập bằng SSO"}
              </Button>
            </div>
          </>
        ) : null}
        {onGoogleCredential ? (
          <>
            <div className="auth-divider"><span>hoặc tiếp tục với Google</span></div>
            <div className="auth-google-area">
              <div className="auth-google-render" ref={googleButtonRef} />
              {!isGoogleReady ? (
                <button
                  className="auth-google-fallback"
                  disabled={Boolean(googleClientId)}
                  onClick={() => setLocalError("Đăng nhập Google cần cấu hình NEXT_PUBLIC_GOOGLE_CLIENT_ID.")}
                  type="button"
                >
                  <GoogleMark />
                  {mode === "login" ? "Đăng nhập với Google" : "Đăng ký với Google"}
                </button>
              ) : null}
            </div>
          </>
        ) : null}
        <p className="auth-mode-link">
          {mode === "login" ? "Chưa có tài khoản?" : "Đã có tài khoản?"}{" "}
          <button onClick={() => onModeChange(mode === "login" ? "register" : "login")} type="button">{mode === "login" ? "Đăng ký ngay" : "Đăng nhập ngay"}</button>
        </p>
      </section>
    </main>
  );
}

function GoogleMark() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M21.8 12.2c0-.7-.1-1.4-.2-2H12v3.8h5.5a4.7 4.7 0 0 1-2 3.1v2.5h3.2c1.9-1.8 3.1-4.3 3.1-7.4Z" fill="#4285F4" />
      <path d="M12 22c2.7 0 5-.9 6.7-2.4l-3.2-2.5c-.9.6-2 1-3.5 1a5.9 5.9 0 0 1-5.5-4.1H3.2v2.6A10 10 0 0 0 12 22Z" fill="#34A853" />
      <path d="M6.5 14a6 6 0 0 1 0-3.9V7.5H3.2a10 10 0 0 0 0 9.1L6.5 14Z" fill="#FBBC05" />
      <path d="M12 5.9c1.6 0 3 .5 4.1 1.6l3-3A10 10 0 0 0 3.2 7.5l3.3 2.6A5.9 5.9 0 0 1 12 5.9Z" fill="#EA4335" />
    </svg>
  );
}

declare global {
  interface Window {
    google?: {
      accounts?: {
        id?: {
          initialize: (options: { callback: (response: { credential?: string }) => void; client_id: string }) => void;
          renderButton: (element: HTMLElement, options: Record<string, string | number>) => void;
        };
      };
    };
  }
}
