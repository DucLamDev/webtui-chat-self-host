import { createBrowserPlatformServices } from "./browser";
import type {
  FilePickerOptions,
  NotificationPermissionState,
  PlatformServices,
  PlatformStorage,
  PlatformStorageArea
} from "./contracts";

const sessionStorage = new Map<string, string>();

export function isTauriRuntime(): boolean {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

export function createTauriPlatformServices(): PlatformServices {
  const browser = createBrowserPlatformServices();

  return {
    ...browser,
    autostart: {
      async disable() {
        const { disable } = await import("@tauri-apps/plugin-autostart");
        await disable();
      },
      async enable() {
        const { enable } = await import("@tauri-apps/plugin-autostart");
        await enable();
      },
      async isEnabled() {
        const { isEnabled } = await import("@tauri-apps/plugin-autostart");
        return isEnabled();
      }
    },
    deepLinks: {
      async getInitialUrls() {
        const { getCurrent } = await import("@tauri-apps/plugin-deep-link");
        return (await getCurrent()) ?? [];
      },
      async onOpenUrl(handler) {
        const { onOpenUrl } = await import("@tauri-apps/plugin-deep-link");
        return onOpenUrl(handler);
      },
      async registerProtocol(protocol) {
        const { isRegistered, register } = await import("@tauri-apps/plugin-deep-link");
        if (!(await isRegistered(protocol))) {
          await register(protocol);
        }
      }
    },
    lifecycle: {
      isDesktop: true,
      platform: "tauri"
    },
    links: {
      async openExternal(url) {
        const { openUrl } = await import("@tauri-apps/plugin-opener");
        await openUrl(url);
      }
    },
    notifications: createTauriNotificationService(),
    files: {
      async pickFiles(options) {
        const { open } = await import("@tauri-apps/plugin-dialog");
        const { readFile } = await import("@tauri-apps/plugin-fs");
        const selected = await open({
          filters: dialogFiltersFromAccept(options?.accept),
          multiple: options?.multiple ?? true,
          title: options?.title
        });
        const paths = Array.isArray(selected) ? selected : selected ? [selected] : [];
        const files = await Promise.all(
          paths.map(async (path) => {
            const bytes = await readFile(path);
            const name = fileNameFromPath(path);
            return new File([new Blob([bytes])], name, {
              type: mimeTypeFromFileName(name, options?.accept)
            });
          })
        );
        return files;
      },
      async saveBlob(blob, suggestedName) {
        const { save } = await import("@tauri-apps/plugin-dialog");
        const { writeFile } = await import("@tauri-apps/plugin-fs");
        const path = await save({
          defaultPath: suggestedName,
          filters: dialogFiltersFromAccept([extensionFromFileName(suggestedName)].filter(Boolean) as string[]),
          title: "Lưu file"
        });
        if (!path) {
          return;
        }
        await writeFile(path, new Uint8Array(await blob.arrayBuffer()));
      }
    },
    storage: createTauriStorage(),
    tray: {
      async setUnreadCount(count) {
        await invokeDesktopCommand<void>("tray_set_unread_count", {
          count: Math.max(0, Math.floor(count))
        });
      }
    },
    updates: {
      async checkAndInstall() {
        const { check } = await import("@tauri-apps/plugin-updater");
        const update = await check();
        if (!update) {
          return { available: false };
        }
        try {
          await update.downloadAndInstall();
          return {
            available: true,
            currentVersion: update.currentVersion,
            version: update.version
          };
        } finally {
          await update.close();
        }
      }
    }
  };
}

function createTauriNotificationService() {
  let permissionCache: NotificationPermissionState = "default";

  return {
    getPermission(): NotificationPermissionState {
      return permissionCache;
    },
    async requestPermission(): Promise<NotificationPermissionState> {
      const { isPermissionGranted, requestPermission } = await import("@tauri-apps/plugin-notification");
      if (await isPermissionGranted()) {
        permissionCache = "granted";
        return permissionCache;
      }
      const permission = await requestPermission();
      permissionCache = permission === "granted" ? "granted" : permission === "denied" ? "denied" : "default";
      return permissionCache;
    },
    async onClick(handler: (payload: { body?: string; data?: Record<string, unknown>; icon?: string; tag?: string; title: string }) => void) {
      const { onAction } = await import("@tauri-apps/plugin-notification");
      const listener = await onAction((notification) => {
        const extra = typeof notification.extra === "object" && notification.extra ? notification.extra : {};
        const { tag, ...data } = extra as Record<string, unknown>;
        handler({
          body: notification.body,
          data,
          icon: notification.icon,
          tag: typeof tag === "string" ? tag : undefined,
          title: notification.title
        });
      });
      return () => {
        void listener.unregister();
      };
    },
    async show({ body, data, icon, tag, title }: { body?: string; data?: Record<string, unknown>; icon?: string; tag?: string; title: string }) {
      const { isPermissionGranted, sendNotification } = await import("@tauri-apps/plugin-notification");
      if (!(await isPermissionGranted())) {
        return;
      }
      permissionCache = "granted";
      sendNotification({
        autoCancel: true,
        body,
        extra: {
          ...(data ?? {}),
          tag
        },
        icon,
        title
      });
    }
  };
}

function createTauriStorage(): PlatformStorage {
  return {
    async getItem(key) {
      if (sessionStorage.has(key)) {
        return sessionStorage.get(key) ?? null;
      }
      return invokeDesktopCommand<string | null>("secure_store_get", { key });
    },
    async removeItem(key) {
      sessionStorage.delete(key);
      await invokeDesktopCommand<void>("secure_store_remove", { key });
    },
    async setItem(key, value, area: PlatformStorageArea = "persistent") {
      if (area === "session") {
        sessionStorage.set(key, value);
        await invokeDesktopCommand<void>("secure_store_remove", { key });
        return;
      }
      sessionStorage.delete(key);
      await invokeDesktopCommand<void>("secure_store_set", { key, value });
    }
  };
}

async function invokeDesktopCommand<T>(command: string, args: Record<string, unknown>): Promise<T> {
  const { invoke } = await import("@tauri-apps/api/core");
  return invoke<T>(command, args);
}

function dialogFiltersFromAccept(accept?: string[]) {
  const extensions = new Set<string>();

  for (const rawToken of accept ?? []) {
    for (const rawPart of rawToken.split(",")) {
      const token = rawPart.trim().toLowerCase();
      if (!token) {
        continue;
      }
      if (token.startsWith(".")) {
        extensions.add(token.slice(1));
        continue;
      }
      for (const extension of extensionsForMimeToken(token)) {
        extensions.add(extension);
      }
    }
  }

  return extensions.size
    ? [
        {
          extensions: [...extensions],
          name: "Tệp phù hợp"
        }
      ]
    : undefined;
}

function extensionsForMimeToken(token: string): string[] {
  if (token === "image/*") {
    return ["avif", "gif", "jpeg", "jpg", "png", "webp"];
  }
  if (token === "audio/*") {
    return ["m4a", "mp3", "ogg", "wav", "webm"];
  }
  if (token === "text/*") {
    return ["csv", "log", "md", "txt"];
  }
  if (token === "video/*") {
    return ["mp4", "mov", "webm"];
  }

  const map: Record<string, string[]> = {
    "application/json": ["json"],
    "application/msword": ["doc"],
    "application/ogg": ["ogg"],
    "application/pdf": ["pdf"],
    "application/vnd.ms-excel": ["xls"],
    "application/vnd.ms-powerpoint": ["ppt"],
    "application/vnd.openxmlformats-officedocument.presentationml.presentation": ["pptx"],
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": ["xlsx"],
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document": ["docx"],
    "application/zip": ["zip"],
    "audio/mpeg": ["mp3"],
    "audio/mp4": ["m4a", "mp4"],
    "audio/ogg": ["ogg"],
    "audio/wav": ["wav"],
    "audio/webm": ["webm"],
    "image/jpeg": ["jpeg", "jpg"],
    "image/png": ["png"],
    "image/webp": ["webp"],
    "text/plain": ["txt"]
  };

  return map[token] ?? [];
}

function fileNameFromPath(path: string): string {
  return path.split(/[\\/]/).filter(Boolean).at(-1) ?? "file-dinh-kem";
}

function extensionFromFileName(name: string): string {
  const extension = name.split(".").at(-1)?.trim().toLowerCase() ?? "";
  return extension && extension !== name.toLowerCase() ? `.${extension}` : "";
}

function mimeTypeFromFileName(name: string, accept?: string[]): string {
  const extension = extensionFromFileName(name).slice(1);
  if (!extension) {
    return accept?.find((item) => !item.includes("*") && item.includes("/")) ?? "application/octet-stream";
  }

  const map: Record<string, string> = {
    avif: "image/avif",
    doc: "application/msword",
    docx: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    gif: "image/gif",
    jpeg: "image/jpeg",
    jpg: "image/jpeg",
    json: "application/json",
    m4a: "audio/mp4",
    mov: "video/quicktime",
    mp3: "audio/mpeg",
    mp4: "video/mp4",
    ogg: "audio/ogg",
    pdf: "application/pdf",
    png: "image/png",
    ppt: "application/vnd.ms-powerpoint",
    pptx: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
    txt: "text/plain",
    wav: "audio/wav",
    webm: "audio/webm",
    webp: "image/webp",
    xls: "application/vnd.ms-excel",
    xlsx: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    zip: "application/zip"
  };

  return map[extension] ?? "application/octet-stream";
}
