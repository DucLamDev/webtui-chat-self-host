"use client";

import {
  createContext,
  type FormEvent,
  type ReactNode,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState
} from "react";
import { Button, Input } from "@webtui/ui";
import { LockKeyhole, ShieldCheck } from "@webtui/icons";
import { getPlatformServices } from "@webtui/chat-core";

const desktopAppLockKey = "desktop_app_lock_v1";
const pinIterations = 120_000;
const resumeLockAfterMs = 60_000;
const maxUnlockAttempts = 5;
const unlockCooldownMs = 30_000;

type DesktopAppLockRecord = {
  hash: string;
  iterations: number;
  salt: string;
};

type DesktopAppLockContextValue = {
  configure: (pin: string) => Promise<void>;
  disable: (pin: string) => Promise<void>;
  enabled: boolean;
  isDesktop: boolean;
  lock: () => void;
};

const DesktopAppLockContext = createContext<DesktopAppLockContextValue>({
  configure: async () => undefined,
  disable: async () => undefined,
  enabled: false,
  isDesktop: false,
  lock: () => undefined
});

export function DesktopAppLockProvider({
  active,
  children,
  onLogout
}: {
  active: boolean;
  children: ReactNode;
  onLogout: () => void;
}) {
  const services = getPlatformServices();
  const isDesktop = services.lifecycle.isDesktop;
  const [enabled, setEnabled] = useState(false);
  const [locked, setLocked] = useState(false);
  const [ready, setReady] = useState(!isDesktop);
  const [record, setRecord] = useState<DesktopAppLockRecord | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [unlockError, setUnlockError] = useState<string | null>(null);
  const [failedAttempts, setFailedAttempts] = useState(0);
  const [retryAt, setRetryAt] = useState(0);
  const [clock, setClock] = useState(Date.now());
  const hiddenAtRef = useRef<number | null>(null);

  useEffect(() => {
    let mounted = true;
    if (!isDesktop || !active) {
      setEnabled(false);
      setLocked(false);
      setRecord(null);
      setLoadError(null);
      setReady(true);
      return () => {
        mounted = false;
      };
    }
    setReady(false);
    void Promise.resolve(services.storage.getItem(desktopAppLockKey))
      .then((raw) => {
        const stored = parseLockRecord(raw);
        if (raw && !stored) {
          throw new Error("invalid desktop app-lock record");
        }
        return stored;
      })
      .then((stored) => {
        if (!mounted) return;
        setLoadError(null);
        setRecord(stored);
        setEnabled(Boolean(stored));
        setLocked(Boolean(stored));
      })
      .catch(() => {
        if (!mounted) return;
        setRecord(null);
        setLoadError(
          "Không đọc được khóa ứng dụng từ kho bảo mật. Phiên được giữ khóa để bảo vệ dữ liệu."
        );
        setEnabled(true);
        setLocked(true);
      })
      .finally(() => {
        if (mounted) setReady(true);
      });
    return () => {
      mounted = false;
    };
  }, [active, isDesktop, services.storage]);

  useEffect(() => {
    if (!isDesktop || !active || !enabled) return;
    let disposed = false;
    let stopFocusListener: (() => void) | undefined;
    const markInactive = () => {
      if (hiddenAtRef.current === null) {
        hiddenAtRef.current = Date.now();
      }
    };
    const lockAfterResume = () => {
      const hiddenAt = hiddenAtRef.current;
      hiddenAtRef.current = null;
      if (hiddenAt && Date.now() - hiddenAt >= resumeLockAfterMs) {
        setLocked(true);
        setUnlockError(null);
      }
    };
    const onVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        markInactive();
      } else {
        lockAfterResume();
      }
    };
    document.addEventListener("visibilitychange", onVisibilityChange);
    void import("@tauri-apps/api/window")
      .then(({ getCurrentWindow }) =>
        getCurrentWindow().onFocusChanged(({ payload: focused }) => {
          if (focused) {
            lockAfterResume();
          } else {
            markInactive();
          }
        })
      )
      .then((stop) => {
        if (disposed) {
          stop();
        } else {
          stopFocusListener = stop;
        }
      })
      .catch(() => undefined);
    return () => {
      disposed = true;
      document.removeEventListener("visibilitychange", onVisibilityChange);
      stopFocusListener?.();
    };
  }, [active, enabled, isDesktop]);

  useEffect(() => {
    if (retryAt <= Date.now()) return;
    const timer = window.setInterval(() => setClock(Date.now()), 1_000);
    return () => window.clearInterval(timer);
  }, [retryAt]);

  const value = useMemo<DesktopAppLockContextValue>(() => ({
    async configure(pin: string) {
      validatePin(pin);
      const nextRecord = await createDesktopPinRecord(pin);
      await Promise.resolve(
        services.storage.setItem(desktopAppLockKey, JSON.stringify(nextRecord), "persistent")
      );
      setRecord(nextRecord);
      setLoadError(null);
      setEnabled(true);
      setLocked(false);
      setFailedAttempts(0);
      setRetryAt(0);
    },
    async disable(pin: string) {
      if (!record || !(await verifyDesktopPinRecord(pin, record))) {
        throw new Error("Mã PIN hiện tại không đúng.");
      }
      await Promise.resolve(services.storage.removeItem(desktopAppLockKey));
      setRecord(null);
      setLoadError(null);
      setEnabled(false);
      setLocked(false);
      setFailedAttempts(0);
      setRetryAt(0);
    },
    enabled,
    isDesktop,
    lock() {
      if (enabled) {
        setLocked(true);
        setUnlockError(null);
      }
    }
  }), [enabled, isDesktop, record, services.storage]);

  if (!ready) {
    return (
      <main className="desktop-app-lock desktop-app-lock--loading" aria-label="Đang tải khóa ứng dụng">
        <LockKeyhole size={28} />
        <p>Đang kiểm tra bảo mật cục bộ...</p>
      </main>
    );
  }

  if (isDesktop && active && loadError) {
    return (
      <main className="desktop-app-lock" aria-label="Không đọc được khóa ứng dụng">
        <section className="desktop-app-lock__card">
          <span className="desktop-app-lock__icon"><ShieldCheck size={30} /></span>
          <h1>Phiên đang được bảo vệ</h1>
          <p>{loadError}</p>
          <Button onClick={onLogout} type="button">Đăng xuất an toàn</Button>
        </section>
      </main>
    );
  }

  if (isDesktop && active && enabled && locked && record) {
    const remainingSeconds = Math.max(0, Math.ceil((retryAt - clock) / 1_000));
    return (
      <DesktopUnlockScreen
        error={unlockError}
        onLogout={onLogout}
        remainingSeconds={remainingSeconds}
        onUnlock={async (pin) => {
          if (retryAt > Date.now()) return;
          if (await verifyDesktopPinRecord(pin, record)) {
            setLocked(false);
            setUnlockError(null);
            setFailedAttempts(0);
            setRetryAt(0);
            return;
          }
          const attempts = failedAttempts + 1;
          setFailedAttempts(attempts);
          if (attempts >= maxUnlockAttempts) {
            const nextRetryAt = Date.now() + unlockCooldownMs;
            setRetryAt(nextRetryAt);
            setClock(Date.now());
            setFailedAttempts(0);
            setUnlockError("Quá nhiều lần thử. Vui lòng chờ 30 giây.");
          } else {
            setUnlockError("Mã PIN không đúng.");
          }
        }}
      />
    );
  }

  return (
    <DesktopAppLockContext.Provider value={value}>
      {children}
    </DesktopAppLockContext.Provider>
  );
}

export function useDesktopAppLock() {
  return useContext(DesktopAppLockContext);
}

function DesktopUnlockScreen({
  error,
  onLogout,
  onUnlock,
  remainingSeconds
}: {
  error: string | null;
  onLogout: () => void;
  onUnlock: (pin: string) => Promise<void>;
  remainingSeconds: number;
}) {
  const [pin, setPin] = useState("");
  const [pending, setPending] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (remainingSeconds > 0 || pending) return;
    setPending(true);
    try {
      await onUnlock(pin);
      setPin("");
    } finally {
      setPending(false);
    }
  }

  return (
    <main className="desktop-app-lock" aria-label="WebTui Chat đang khóa">
      <form className="desktop-app-lock__card" onSubmit={submit}>
        <span className="desktop-app-lock__icon"><ShieldCheck size={30} /></span>
        <h1>WebTui Chat đang khóa</h1>
        <p>Nhập mã PIN cục bộ để tiếp tục phiên làm việc.</p>
        <Input
          autoComplete="current-password"
          autoFocus
          inputMode="numeric"
          maxLength={32}
          onChange={(event) => setPin(event.target.value)}
          placeholder="Mã PIN"
          type="password"
          value={pin}
        />
        {error ? <small className="desktop-app-lock__error" role="alert">{error}</small> : null}
        <Button disabled={!pin || pending || remainingSeconds > 0} type="submit">
          {remainingSeconds > 0 ? "Thử lại sau " + remainingSeconds + " giây" : pending ? "Đang xác minh..." : "Mở khóa"}
        </Button>
        <Button onClick={onLogout} type="button" variant="ghost">Đăng xuất</Button>
      </form>
    </main>
  );
}

export async function createDesktopPinRecord(pin: string): Promise<DesktopAppLockRecord> {
  validatePin(pin);
  const salt = crypto.getRandomValues(new Uint8Array(16));
  return {
    hash: bytesToBase64(await derivePin(pin, salt, pinIterations)),
    iterations: pinIterations,
    salt: bytesToBase64(salt)
  };
}

export async function verifyDesktopPinRecord(
  pin: string,
  record: DesktopAppLockRecord
): Promise<boolean> {
  if (!Number.isInteger(record.iterations) || record.iterations < pinIterations) return false;
  try {
    const actual = await derivePin(pin, base64ToBytes(record.salt), record.iterations);
    const expected = base64ToBytes(record.hash);
    if (actual.length !== expected.length) return false;
    let difference = 0;
    for (let index = 0; index < actual.length; index += 1) {
      difference |= actual[index] ^ expected[index];
    }
    return difference === 0;
  } catch {
    return false;
  }
}

async function derivePin(pin: string, salt: Uint8Array, iterations: number): Promise<Uint8Array> {
  const saltBuffer = new ArrayBuffer(salt.byteLength);
  new Uint8Array(saltBuffer).set(salt);
  const key = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(pin),
    "PBKDF2",
    false,
    ["deriveBits"]
  );
  const bits = await crypto.subtle.deriveBits(
    { hash: "SHA-256", iterations, name: "PBKDF2", salt: saltBuffer },
    key,
    256
  );
  return new Uint8Array(bits);
}

function validatePin(pin: string) {
  if (!/^\d{6,32}$/.test(pin)) {
    throw new Error("Mã PIN phải gồm ít nhất 6 chữ số.");
  }
}

function parseLockRecord(raw: string | null): DesktopAppLockRecord | null {
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as Partial<DesktopAppLockRecord>;
    if (
      typeof parsed.hash !== "string" ||
      typeof parsed.salt !== "string" ||
      !Number.isInteger(parsed.iterations) ||
      Number(parsed.iterations) < pinIterations
    ) {
      return null;
    }
    return {
      hash: parsed.hash,
      iterations: Number(parsed.iterations),
      salt: parsed.salt
    };
  } catch {
    return null;
  }
}

function bytesToBase64(value: Uint8Array): string {
  let binary = "";
  value.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary);
}

function base64ToBytes(value: string): Uint8Array {
  const binary = atob(value);
  return Uint8Array.from(binary, (character) => character.charCodeAt(0));
}
