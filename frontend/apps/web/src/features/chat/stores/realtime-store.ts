"use client";

import { create } from "zustand";

export type RealtimeConnectionStatus = "idle" | "connecting" | "connected" | "reconnecting" | "offline";

type RealtimeState = {
  lastEventAt: string | null;
  retryAttempt: number;
  room: string | null;
  status: RealtimeConnectionStatus;
  setConnection: (next: Partial<RealtimeState>) => void;
};

export const useRealtimeStore = create<RealtimeState>((set) => ({
  lastEventAt: null,
  retryAttempt: 0,
  room: null,
  setConnection: (next) => set(next),
  status: "idle"
}));
