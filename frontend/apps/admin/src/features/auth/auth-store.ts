"use client";

import type { AuthResult, AuthUser } from "@webtui/types";
import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";

type AuthState = {
  accessToken: string | null;
  hydrated: boolean;
  refreshToken: string | null;
  user: AuthUser | null;
  clearSession: () => void;
  setHydrated: (hydrated: boolean) => void;
  setSession: (result: AuthResult) => void;
  setUser: (user: AuthUser | null) => void;
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      hydrated: false,
      refreshToken: null,
      user: null,
      clearSession: () =>
        set({
          accessToken: null,
          refreshToken: null,
          user: null
        }),
      setHydrated: (hydrated) => set({ hydrated }),
      setSession: (result) =>
        set((state) => {
          const accessToken = result.tokens?.access_token ?? result.access_token ?? state.accessToken;
          const refreshToken = result.tokens?.refresh_token ?? result.refresh_token ?? state.refreshToken;

          return {
            accessToken,
            refreshToken,
            user: result.user ?? state.user
          };
        }),
      setUser: (user) => set({ user })
    }),
    {
      name: "webtui-admin-auth",
      onRehydrateStorage: () => (state) => {
        state?.setHydrated(true);
      },
      partialize: (state) => ({
        accessToken: state.accessToken,
        refreshToken: state.refreshToken,
        user: state.user
      }),
      skipHydration: true,
      storage: createJSONStorage(() => localStorage)
    }
  )
);
