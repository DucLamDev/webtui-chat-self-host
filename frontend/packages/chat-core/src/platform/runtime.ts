import { createBrowserPlatformServices } from "./browser";
import type { PlatformServices } from "./contracts";
import { createTauriPlatformServices, isTauriRuntime } from "./tauri";

let currentPlatformServices: PlatformServices = isTauriRuntime()
  ? createTauriPlatformServices()
  : createBrowserPlatformServices();

export function getPlatformServices(): PlatformServices {
  return currentPlatformServices;
}

export function setPlatformServices(services: PlatformServices): void {
  currentPlatformServices = services;
}
