export { createBrowserPlatformServices } from "./platform/browser";
export { getPlatformServices, setPlatformServices } from "./platform/runtime";
export { createTauriPlatformServices, isTauriRuntime } from "./platform/tauri";
export type {
  ClipboardService,
  AutoStartService,
  DeepLinkService,
  FilePickerOptions,
  FileService,
  LifecycleService,
  MediaRecorderFactoryInput,
  MediaRecorderHandle,
  MediaService,
  NotificationPayload,
  NotificationPermissionState,
  NotificationService,
  PlatformFetcher,
  PlatformKind,
  PlatformServices,
  PlatformStorage,
  PlatformStorageArea,
  TrayService
} from "./platform/contracts";
