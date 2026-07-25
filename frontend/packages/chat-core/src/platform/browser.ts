import type {
  FilePickerOptions,
  MediaRecorderFactoryInput,
  MediaRecorderHandle,
  NotificationPermissionState,
  PlatformServices,
  PlatformStorage,
  PlatformStorageArea
} from "./contracts";

export function createBrowserPlatformServices(): PlatformServices {
  return {
    autostart: {
      async disable() {
        throw new Error("Tự khởi động chỉ hỗ trợ trên ứng dụng desktop.");
      },
      async enable() {
        throw new Error("Tự khởi động chỉ hỗ trợ trên ứng dụng desktop.");
      },
      async isEnabled() {
        return false;
      }
    },
    clipboard: {
      async writeText(value) {
        if (typeof navigator === "undefined" || !navigator.clipboard?.writeText) {
          throw new Error("Clipboard không được hỗ trợ trên môi trường hiện tại.");
        }
        await navigator.clipboard.writeText(value);
      }
    },
    deepLinks: {
      async getInitialUrls() {
        return [];
      },
      async onOpenUrl() {
        return () => undefined;
      },
      async registerProtocol() {
        return undefined;
      }
    },
    fetcher: (input, init) => fetch(input, init),
    files: {
      pickFiles: pickBrowserFiles,
      async saveBlob(blob, suggestedName) {
        if (typeof document === "undefined" || typeof URL === "undefined") {
          throw new Error("Không thể lưu file trên môi trường hiện tại.");
        }
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement("a");
        anchor.href = url;
        anchor.download = suggestedName;
        anchor.rel = "noreferrer";
        anchor.click();
        URL.revokeObjectURL(url);
      }
    },
    lifecycle: {
      isDesktop: false,
      platform: "browser"
    },
    links: {
      async openExternal(url) {
        if (typeof window === "undefined") {
          throw new Error("Không thể mở liên kết trên môi trường hiện tại.");
        }
        window.open(url, "_blank", "noopener,noreferrer");
      }
    },
    media: {
      createAudioRecorder: createBrowserAudioRecorder,
      getSupportedAudioMimeType(candidates) {
        if (typeof MediaRecorder === "undefined" || typeof MediaRecorder.isTypeSupported !== "function") {
          return "";
        }
        return candidates.find((mimeType) => MediaRecorder.isTypeSupported(mimeType)) ?? "";
      }
    },
    notifications: createBrowserNotificationService(),
    storage: createBrowserStorage(),
    tray: {
      setUnreadCount() {
        return undefined;
      }
    },
    updates: {
      async checkAndInstall() {
        return { available: false };
      }
    }
  };
}

function createBrowserStorage(): PlatformStorage {
  return {
    getItem(key) {
      if (typeof window === "undefined") {
        return null;
      }
      return window.localStorage.getItem(key) ?? window.sessionStorage.getItem(key);
    },
    removeItem(key) {
      if (typeof window === "undefined") {
        return;
      }
      window.localStorage.removeItem(key);
      window.sessionStorage.removeItem(key);
    },
    setItem(key, value, area: PlatformStorageArea = "persistent") {
      if (typeof window === "undefined") {
        return;
      }
      const target = area === "session" ? window.sessionStorage : window.localStorage;
      const stale = area === "session" ? window.localStorage : window.sessionStorage;
      stale.removeItem(key);
      target.setItem(key, value);
    }
  };
}

function createBrowserNotificationService() {
  return {
    getPermission(): NotificationPermissionState {
      if (typeof Notification === "undefined") {
        return "unsupported";
      }
      return Notification.permission;
    },
    async onClick() {
      return () => undefined;
    },
    async requestPermission(): Promise<NotificationPermissionState> {
      if (typeof Notification === "undefined") {
        return "unsupported";
      }
      return Notification.requestPermission();
    },
    show({ body, icon, tag, title }: { body?: string; icon?: string; tag?: string; title: string }) {
      if (typeof Notification === "undefined" || Notification.permission !== "granted") {
        return;
      }
      new Notification(title, { body, icon, tag });
    }
  };
}

async function createBrowserAudioRecorder(input: MediaRecorderFactoryInput): Promise<MediaRecorderHandle> {
  if (
    typeof navigator === "undefined" ||
    !navigator.mediaDevices?.getUserMedia ||
    typeof MediaRecorder === "undefined"
  ) {
    throw new Error("Trình duyệt không hỗ trợ ghi âm.");
  }

  let stream: MediaStream;
  try {
    stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        echoCancellation: true,
        noiseSuppression: true
      }
    });
  } catch (error) {
    const name = error instanceof DOMException ? error.name : "";
    if (name === "NotAllowedError" || name === "PermissionDeniedError") {
      throw new Error("Micro đang bị chặn. Hãy cấp quyền micro trong trình duyệt hoặc cài đặt hệ điều hành rồi thử lại.");
    }
    if (name === "NotFoundError" || name === "DevicesNotFoundError") {
      throw new Error("Không tìm thấy micro khả dụng trên thiết bị này.");
    }
    throw new Error("Không bật được micro. Hãy kiểm tra thiết bị ghi âm và thử lại.");
  }
  const recorderOptions: MediaRecorderOptions = {
    ...(input.audioBitsPerSecond ? { audioBitsPerSecond: input.audioBitsPerSecond } : {}),
    ...(input.mimeType ? { mimeType: input.mimeType } : {})
  };
  const recorder = new MediaRecorder(stream, recorderOptions);

  recorder.ondataavailable = (event) => {
    if (event.data.size > 0) {
      input.onDataAvailable(event.data);
    }
  };
  recorder.onstop = () => {
    for (const track of stream.getTracks()) {
      track.stop();
    }
    input.onStop();
  };
  recorder.onerror = (event) => {
    input.onError?.(event.error ?? new Error("Ghi âm bị gián đoạn."));
  };

  return {
    mimeType: recorder.mimeType || "audio/webm",
    get state() {
      return recorder.state;
    },
    pause: () => recorder.pause(),
    resume: () => recorder.resume(),
    start: (timesliceMs?: number) => recorder.start(timesliceMs),
    stop: () => recorder.stop()
  };
}

function pickBrowserFiles(options: FilePickerOptions = {}): Promise<File[]> {
  if (typeof document === "undefined") {
    return Promise.reject(new Error("Không thể chọn file trên môi trường hiện tại."));
  }

  return new Promise((resolve) => {
    const input = document.createElement("input");
    input.type = "file";
    input.multiple = options.multiple ?? true;
    input.accept = options.accept?.join(",") ?? "";
    input.className = "visually-hidden";

    const cleanup = () => {
      input.remove();
    };

    const finish = (files: File[] = []) => {
      resolve(files);
      cleanup();
    };

    input.addEventListener("change", () => finish(Array.from(input.files ?? [])), { once: true });
    input.addEventListener("cancel", () => finish(), { once: true });

    document.body.append(input);
    input.click();
  });
}
