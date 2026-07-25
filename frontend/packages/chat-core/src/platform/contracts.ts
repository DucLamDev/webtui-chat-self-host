export type PlatformKind = "browser" | "tauri";

export type PlatformFetcher = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;

export type PlatformStorageArea = "persistent" | "session";

export type PlatformStorage = {
  getItem: (key: string) => string | null | Promise<string | null>;
  removeItem: (key: string) => void | Promise<void>;
  setItem: (key: string, value: string, area?: PlatformStorageArea) => void | Promise<void>;
};

export type NotificationPermissionState = "default" | "denied" | "granted" | "unsupported";

export type NotificationPayload = {
  body?: string;
  data?: Record<string, unknown>;
  icon?: string;
  tag?: string;
  title: string;
};

export type NotificationService = {
  getPermission: () => NotificationPermissionState;
  onClick: (handler: (payload: NotificationPayload) => void) => Promise<() => void>;
  requestPermission: () => Promise<NotificationPermissionState>;
  show: (payload: NotificationPayload) => Promise<void> | void;
};

export type MediaRecorderHandle = {
  mimeType: string;
  readonly state: "inactive" | "paused" | "recording";
  pause?: () => void;
  resume?: () => void;
  start: (timesliceMs?: number) => void;
  stop: () => void;
};

export type MediaRecorderFactoryInput = {
  audioBitsPerSecond?: number;
  mimeType?: string;
  onDataAvailable: (blob: Blob) => void;
  onError?: (error: Error) => void;
  onStop: () => void;
};

export type MediaService = {
  createAudioRecorder: (input: MediaRecorderFactoryInput) => Promise<MediaRecorderHandle>;
  getSupportedAudioMimeType: (candidates: string[]) => string;
};

export type ClipboardService = {
  writeText: (value: string) => Promise<void>;
};

export type FilePickerOptions = {
  accept?: string[];
  multiple?: boolean;
  title?: string;
};

export type FileService = {
  pickFiles: (options?: FilePickerOptions) => Promise<File[]>;
  saveBlob: (blob: Blob, suggestedName: string) => Promise<void>;
};

export type LinkService = {
  openExternal: (url: string) => Promise<void>;
};

export type DeepLinkService = {
  getInitialUrls: () => Promise<string[]>;
  onOpenUrl: (handler: (urls: string[]) => void) => Promise<() => void>;
  registerProtocol: (protocol: string) => Promise<void>;
};

export type AutoStartService = {
  disable: () => Promise<void>;
  enable: () => Promise<void>;
  isEnabled: () => Promise<boolean>;
};

export type TrayService = {
  setUnreadCount: (count: number) => Promise<void> | void;
};

export type UpdateInstallResult =
  | {
      available: false;
    }
  | {
      available: true;
      currentVersion: string;
      version: string;
    };

export type UpdateService = {
  checkAndInstall: () => Promise<UpdateInstallResult>;
};

export type LifecycleService = {
  platform: PlatformKind;
  isDesktop: boolean;
};

export type PlatformServices = {
  autostart: AutoStartService;
  clipboard: ClipboardService;
  deepLinks: DeepLinkService;
  fetcher: PlatformFetcher;
  files: FileService;
  links: LinkService;
  lifecycle: LifecycleService;
  media: MediaService;
  notifications: NotificationService;
  storage: PlatformStorage;
  tray: TrayService;
  updates: UpdateService;
};
