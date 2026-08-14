"use client";

import { useEffect, useState } from "react";
import {
  createHealthClient,
  HttpClient,
  type HealthStatus
} from "@webtui/api-client";
import { resolveAdminApiBaseUrl } from "@/lib/api";

type ApiStatusState =
  | { status: "checking"; label: string; detail?: string }
  | { status: "online"; label: string; detail?: string; payload: HealthStatus }
  | { status: "offline"; label: string; detail?: string };

export function useApiStatus(): ApiStatusState {
  const [state, setState] = useState<ApiStatusState>({
    status: "checking",
    label: "Đang kiểm tra API"
  });

  useEffect(() => {
    let active = true;
    const client = createHealthClient(
      new HttpClient({
        baseUrl: resolveAdminApiBaseUrl
      })
    );

    const checkApi = async () => {
      const baseUrl = resolveAdminApiBaseUrl();
      try {
        const payload = await client.ready();
        if (!active) {
          return;
        }

        const isOnline = payload.status === "ready" || payload.status === "ok";

        setState({
          detail: baseUrl,
          label: isOnline ? "API sẵn sàng" : "API chưa sẵn sàng",
          payload,
          status: isOnline ? "online" : "offline"
        });
      } catch {
        if (!active) {
          return;
        }

        setState({
          detail: baseUrl,
          label: "Không kết nối được API",
          status: "offline"
        });
      }
    };

    const checkWhenVisible = () => {
      if (document.visibilityState === "visible") {
        void checkApi();
      }
    };

    void checkApi();
    const pollTimer = window.setInterval(() => void checkApi(), 30_000);
    window.addEventListener("online", checkApi);
    document.addEventListener("visibilitychange", checkWhenVisible);

    return () => {
      active = false;
      window.clearInterval(pollTimer);
      window.removeEventListener("online", checkApi);
      document.removeEventListener("visibilitychange", checkWhenVisible);
    };
  }, []);

  return state;
}
