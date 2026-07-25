"use client";

import { useEffect, useState } from "react";
import {
  createHealthClient,
  createRuntimeEnvironment,
  HttpClient,
  type HealthStatus,
  type VersionInfo
} from "@webtui/api-client";
import { getPlatformServices } from "@webtui/chat-core";

type ApiStatusState =
  | { status: "checking"; label: string; detail?: string }
  | { status: "online"; label: string; detail?: string; payload: HealthStatus }
  | { status: "offline"; label: string; detail?: string };

export type DesktopVersionStatus =
  | { status: "unsupported"; label: string; detail?: string; updateUrl?: string; version?: VersionInfo }
  | { status: "update_available"; label: string; detail?: string; updateUrl?: string; version?: VersionInfo }
  | { status: "current"; label: string; detail?: string; updateUrl?: string; version?: VersionInfo }
  | { status: "checking"; label: string; detail?: string; updateUrl?: string; version?: VersionInfo }
  | { status: "offline"; label: string; detail?: string; updateUrl?: string; version?: VersionInfo };

export function useApiStatus(): ApiStatusState {
  const [state, setState] = useState<ApiStatusState>({
    status: "checking",
    label: "Đang kiểm tra API"
  });

  useEffect(() => {
    let active = true;
    const env = createRuntimeEnvironment({
      NEXT_PUBLIC_API_BASE_URL: process.env.NEXT_PUBLIC_API_BASE_URL,
      NEXT_PUBLIC_APP_VERSION: process.env.NEXT_PUBLIC_APP_VERSION,
      NEXT_PUBLIC_WS_BASE_URL: process.env.NEXT_PUBLIC_WS_BASE_URL,
      NEXT_PUBLIC_APP_NAME: process.env.NEXT_PUBLIC_APP_NAME,
      NEXT_PUBLIC_DEFAULT_LOCALE: process.env.NEXT_PUBLIC_DEFAULT_LOCALE,
      NEXT_PUBLIC_RELEASE_CHANNEL: process.env.NEXT_PUBLIC_RELEASE_CHANNEL
    });
    const client = createHealthClient(
      new HttpClient({
        baseUrl: env.apiBaseUrl,
        fetcher: getPlatformServices().fetcher
      })
    );

    client
      .ready()
      .then((payload) => {
        if (!active) {
          return;
        }

        const isOnline = payload.status === "ready" || payload.status === "ok";

        setState({
          detail: env.apiBaseUrl,
          label: isOnline ? "API sẵn sàng" : "API chưa sẵn sàng",
          payload,
          status: isOnline ? "online" : "offline"
        });
      })
      .catch(() => {
        if (!active) {
          return;
        }

        setState({
          detail: env.apiBaseUrl,
          label: "Không kết nối được API",
          status: "offline"
        });
      });

    return () => {
      active = false;
    };
  }, []);

  return state;
}

export function useDesktopVersionStatus(): DesktopVersionStatus {
  const [state, setState] = useState<DesktopVersionStatus>({
    label: "Dang kiem tra phien ban",
    status: "checking"
  });

  useEffect(() => {
    let active = true;
    const services = getPlatformServices();
    const env = createRuntimeEnvironment({
      NEXT_PUBLIC_API_BASE_URL: process.env.NEXT_PUBLIC_API_BASE_URL,
      NEXT_PUBLIC_APP_VERSION: process.env.NEXT_PUBLIC_APP_VERSION,
      NEXT_PUBLIC_WS_BASE_URL: process.env.NEXT_PUBLIC_WS_BASE_URL,
      NEXT_PUBLIC_APP_NAME: process.env.NEXT_PUBLIC_APP_NAME,
      NEXT_PUBLIC_DEFAULT_LOCALE: process.env.NEXT_PUBLIC_DEFAULT_LOCALE,
      NEXT_PUBLIC_RELEASE_CHANNEL: process.env.NEXT_PUBLIC_RELEASE_CHANNEL
    });

    if (!services.lifecycle.isDesktop) {
      setState({
        detail: `Web ${env.appVersion}`,
        label: "Ban web khong can updater desktop",
        status: "current"
      });
      return;
    }

    const client = createHealthClient(
      new HttpClient({
        baseUrl: env.apiBaseUrl,
        fetcher: services.fetcher
      })
    );

    const checkVersion = () => {
      void client.version()
        .then((version) => {
        if (!active) {
          return;
        }
        const desktop = version.clients?.desktop;
        const minimum = desktop?.minimum_version;
        const recommended = desktop?.recommended_version;
        const updateUrl = desktop?.update_url;
        const clientLabel = `${env.appVersion} (${env.releaseChannel})`;

        if (minimum && compareVersions(env.appVersion, minimum) < 0) {
          setState({
            detail: `Dang dung ${clientLabel}, toi thieu can ${minimum}.`,
            label: "Can cap nhat de tiep tuc tuong thich",
            status: "unsupported",
            updateUrl,
            version
          });
          return;
        }

        if (recommended && compareVersions(env.appVersion, recommended) < 0) {
          setState({
            detail: `Dang dung ${clientLabel}, khuyen nghi ${recommended}.`,
            label: "Co ban cap nhat desktop",
            status: "update_available",
            updateUrl,
            version
          });
          return;
        }

        setState({
          detail: `Dang dung ${clientLabel}. Backend ${version.version}.`,
          label: "Phien ban desktop dang tuong thich",
          status: "current",
          updateUrl,
          version
        });
      })
      .catch(() => {
        if (!active) {
          return;
        }
        setState({
          detail: env.apiBaseUrl,
          label: "Khong kiem tra duoc phien ban backend",
          status: "offline"
        });
      });
    };

    checkVersion();
    const interval = window.setInterval(checkVersion, 5 * 60 * 1000);

    return () => {
      active = false;
      window.clearInterval(interval);
    };
  }, []);

  return state;
}

function compareVersions(left: string, right: string): number {
  const leftParts = parseVersion(left);
  const rightParts = parseVersion(right);

  for (let index = 0; index < Math.max(leftParts.length, rightParts.length); index += 1) {
    const diff = (leftParts[index] ?? 0) - (rightParts[index] ?? 0);
    if (diff !== 0) {
      return diff > 0 ? 1 : -1;
    }
  }

  return 0;
}

function parseVersion(value: string): number[] {
  const match = value.trim().match(/\d+(?:\.\d+)*/);
  if (!match) {
    return [0];
  }
  return match[0].split(".").map((part) => Number.parseInt(part, 10) || 0);
}
