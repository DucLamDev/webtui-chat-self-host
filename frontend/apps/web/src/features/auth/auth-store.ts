"use client";

import type {
  AuthResult,
  AuthUser,
  ZoneRuntime
} from "@webtui/types";
import { getPlatformServices } from "@webtui/chat-core";
import { create } from "zustand";
import { createJSONStorage, persist, type StateStorage } from "zustand/middleware";

export type AuthAccount = {
  accessToken: string | null;
  domain: string;
  refreshToken: string | null;
  runtime: ZoneRuntime;
  sessionId: string | null;
  updatedAt: string;
  user: AuthUser | null;
};

type AuthState = {
  accounts: AuthAccount[];
  accessToken: string | null;
  hydrated: boolean;
  refreshToken: string | null;
  rememberLogin: boolean;
  sessionId: string | null;
  user: AuthUser | null;
  zoneDomain: string | null;
  zoneRuntime: ZoneRuntime | null;
  clearZoneRuntime: () => void;
  clearSession: () => void;
  setHydrated: (hydrated: boolean) => void;
  setRememberLogin: (remember: boolean) => void;
  setSession: (result: AuthResult) => void;
  setUser: (user: AuthUser | null) => void;
  setZoneRuntime: (domain: string, runtime: ZoneRuntime) => void;
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accounts: [],
      accessToken: null,
      hydrated: false,
      refreshToken: null,
      rememberLogin: true,
      sessionId: null,
      user: null,
      zoneDomain: null,
      zoneRuntime: null,
      clearZoneRuntime: () =>
        set((state) => ({
          accessToken: null,
          accounts: rememberCurrentAccount(state),
          refreshToken: null,
          sessionId: null,
          user: null,
          zoneDomain: null,
          zoneRuntime: null
        })),
      clearSession: () =>
        set((state) => ({
          accessToken: null,
          accounts: state.accounts.map((account) =>
            account.domain === state.zoneDomain
              ? {
                  ...account,
                  accessToken: null,
                  refreshToken: null,
                  sessionId: null,
                  user: null,
                  updatedAt: new Date().toISOString()
                }
              : account
          ),
          refreshToken: null,
          sessionId: null,
          user: null
        })),
      setHydrated: (hydrated) => set({ hydrated }),
      setRememberLogin: (rememberLogin) => set({ rememberLogin }),
      setSession: (result) =>
        set((state) => {
          const accessToken = result.tokens?.access_token ?? result.access_token ?? state.accessToken;
          const refreshToken = result.tokens?.refresh_token ?? result.refresh_token ?? state.refreshToken;

          const next = {
            accessToken,
            refreshToken,
            sessionId: result.session_id ?? state.sessionId,
            user: result.user ?? state.user,
            zoneDomain: result.zone?.domain ?? state.zoneDomain
          };
          return {
            ...next,
            accounts:
              next.zoneDomain && state.zoneRuntime
                ? upsertAccount(state.accounts, {
                    accessToken: next.accessToken,
                    domain: next.zoneDomain,
                    refreshToken: next.refreshToken,
                    runtime: state.zoneRuntime,
                    sessionId: next.sessionId,
                    updatedAt: new Date().toISOString(),
                    user: next.user
                  })
                : state.accounts
          };
        }),
      setUser: (user) =>
        set((state) => ({
          accounts:
            state.zoneDomain && state.zoneRuntime
              ? upsertAccount(state.accounts, {
                  accessToken: state.accessToken,
                  domain: state.zoneDomain,
                  refreshToken: state.refreshToken,
                  runtime: state.zoneRuntime,
                  sessionId: state.sessionId,
                  updatedAt: new Date().toISOString(),
                  user
                })
              : state.accounts,
          user
        })),
      setZoneRuntime: (zoneDomain, zoneRuntime) =>
        set((state) => {
          const accounts = rememberCurrentAccount(state);
          const existing = accounts.find((account) => account.domain === zoneDomain);
          return {
            accessToken: existing?.accessToken ?? null,
            accounts: upsertAccount(accounts, {
              accessToken: existing?.accessToken ?? null,
              domain: zoneDomain,
              refreshToken: existing?.refreshToken ?? null,
              runtime: zoneRuntime,
              sessionId: existing?.sessionId ?? null,
              updatedAt: new Date().toISOString(),
              user: existing?.user ?? null
            }),
            refreshToken: existing?.refreshToken ?? null,
            sessionId: existing?.sessionId ?? null,
            user: existing?.user ?? null,
            zoneDomain,
            zoneRuntime
          };
        })
    }),
    {
      name: "webtui-web-auth",
      onRehydrateStorage: () => (state) => {
        if (!state) {
          return;
        }
        if (getPlatformServices().lifecycle.isDesktop) {
          useAuthStore.setState({
            accessToken: null,
            accounts: persistentDesktopAccounts(state.accounts)
          });
        }
        state.setHydrated(true);
      },
      partialize: (state) => ({
        accounts: getPlatformServices().lifecycle.isDesktop
          ? persistentDesktopAccounts(state.accounts)
          : [],
        accessToken: getPlatformServices().lifecycle.isDesktop ? null : state.accessToken,
        refreshToken: state.refreshToken,
        rememberLogin: state.rememberLogin,
        sessionId: state.sessionId,
        user: state.user,
        zoneDomain: state.zoneDomain,
        zoneRuntime: state.zoneRuntime
      }),
      skipHydration: true,
      storage: createJSONStorage(createAuthStorage)
    }
  )
);

export function persistentDesktopAccounts(accounts: AuthAccount[]): AuthAccount[] {
  return accounts.map((account) => ({
    ...account,
    accessToken: null
  }));
}

function rememberCurrentAccount(state: AuthState): AuthAccount[] {
  if (!state.zoneDomain || !state.zoneRuntime) {
    return state.accounts;
  }
  const existing = state.accounts.find((account) => account.domain === state.zoneDomain);
  return upsertAccount(state.accounts, {
    accessToken: state.accessToken ?? existing?.accessToken ?? null,
    domain: state.zoneDomain,
    refreshToken: state.refreshToken ?? existing?.refreshToken ?? null,
    runtime: state.zoneRuntime,
    sessionId: state.sessionId ?? existing?.sessionId ?? null,
    updatedAt: new Date().toISOString(),
    user: state.user ?? existing?.user ?? null
  });
}

function upsertAccount(accounts: AuthAccount[], account: AuthAccount): AuthAccount[] {
  return [
    account,
    ...accounts.filter((candidate) => candidate.domain !== account.domain)
  ]
    .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))
    .slice(0, 10);
}

function createAuthStorage(): StateStorage {
  return {
    getItem(name) {
      return getPlatformServices().storage.getItem(name);
    },
    removeItem(name) {
      return getPlatformServices().storage.removeItem(name);
    },
    setItem(name, value) {
      let remember = true;
      try {
        const parsed = JSON.parse(value) as { state?: { rememberLogin?: boolean } };
        remember = parsed.state?.rememberLogin !== false;
      } catch {
        // Keep the safer default for data written by older app versions.
      }

      return getPlatformServices().storage.setItem(name, value, remember ? "persistent" : "session");
    }
  };
}
