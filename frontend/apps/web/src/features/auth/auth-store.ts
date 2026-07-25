"use client";

import type {
  AuthResult,
  AuthUser,
  ZoneRuntime
} from "@webtui/types";
import { getPlatformServices } from "@webtui/chat-core";
import { create } from "zustand";
import { createJSONStorage, persist, type StateStorage } from "zustand/middleware";

type AuthState = {
  accessToken: string | null;
  hydrated: boolean;
  refreshToken: string | null;
  rememberLogin: boolean;
  sessionId: string | null;
  user: AuthUser | null;
  zoneDomain: string | null;
  zoneRuntime: ZoneRuntime | null;
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
      accessToken: null,
      hydrated: false,
      refreshToken: null,
      rememberLogin: true,
      sessionId: null,
      user: null,
      zoneDomain: null,
      zoneRuntime: null,
      clearSession: () =>
        set({
          accessToken: null,
          refreshToken: null,
          sessionId: null,
          user: null,
          zoneDomain: null,
          zoneRuntime: null
        }),
      setHydrated: (hydrated) => set({ hydrated }),
      setRememberLogin: (rememberLogin) => set({ rememberLogin }),
      setSession: (result) =>
        set((state) => {
          const accessToken = result.tokens?.access_token ?? result.access_token ?? state.accessToken;
          const refreshToken = result.tokens?.refresh_token ?? result.refresh_token ?? state.refreshToken;

          return {
            accessToken,
            refreshToken,
            sessionId: result.session_id ?? state.sessionId,
            user: result.user ?? state.user,
            zoneDomain: result.zone?.domain ?? state.zoneDomain
          };
        }),
      setUser: (user) => set({ user }),
      setZoneRuntime: (zoneDomain, zoneRuntime) => set({ zoneDomain, zoneRuntime })
    }),
    {
      name: "webtui-web-auth",
      onRehydrateStorage: () => (state) => {
        state?.setHydrated(true);
      },
      partialize: (state) => ({
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
