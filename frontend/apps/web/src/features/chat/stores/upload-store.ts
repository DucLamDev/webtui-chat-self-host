"use client";

import { create } from "zustand";

export type UploadQueueStatus = "queued" | "uploading" | "attached" | "failed";

export type UploadQueueItem = {
  durationSeconds?: number;
  error?: string;
  file: File;
  fileId?: string;
  id: string;
  isAudio?: boolean;
  isImage?: boolean;
  messageId?: string;
  name: string;
  previewUrl?: string;
  size: number;
  status: UploadQueueStatus;
};

type UploadQueueState = {
  items: UploadQueueItem[];
  addFiles: (files: File[]) => void;
  addVoice: (file: File, durationSeconds: number) => void;
  clearAttached: () => void;
  markAttached: (id: string, messageId: string, fileId: string) => void;
  markFailed: (id: string, error: string) => void;
  markUploading: (id: string) => void;
  remove: (id: string) => void;
  retry: (id: string) => void;
};

export const useUploadStore = create<UploadQueueState>((set) => ({
  addFiles: (files) =>
    set((state) => ({
      items: [
        ...state.items,
        ...files.map(createUploadItem)
      ]
    })),
  addVoice: (file, durationSeconds) =>
    set((state) => ({
      items: [
        ...state.items,
        createUploadItem(file, durationSeconds)
      ]
    })),
  clearAttached: () =>
    set((state) => {
      state.items.filter((item) => item.status === "attached").forEach(revokeUploadPreview);
      return {
        items: state.items.filter((item) => item.status !== "attached")
      };
    }),
  items: [],
  markAttached: (id, messageId, fileId) =>
    set((state) => ({
      items: state.items.map((item) =>
        item.id === id
          ? {
              ...item,
              error: undefined,
              fileId,
              messageId,
              status: "attached"
            }
          : item
      )
    })),
  markFailed: (id, error) =>
    set((state) => ({
      items: state.items.map((item) =>
        item.id === id
          ? {
              ...item,
              error,
              status: "failed"
            }
          : item
      )
    })),
  markUploading: (id) =>
    set((state) => ({
      items: state.items.map((item) =>
        item.id === id
          ? {
              ...item,
              error: undefined,
              status: "uploading"
            }
          : item
      )
    })),
  remove: (id) =>
    set((state) => {
      state.items.filter((item) => item.id === id).forEach(revokeUploadPreview);
      return {
        items: state.items.filter((item) => item.id !== id)
      };
    }),
  retry: (id) =>
    set((state) => ({
      items: state.items.map((item) =>
        item.id === id
          ? {
              ...item,
              error: undefined,
              status: "queued"
            }
          : item
      )
    }))
}));

function createUploadId() {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }

  return `upload-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function createUploadItem(file: File, durationSeconds?: number): UploadQueueItem {
  const extension = file.name.split(".").at(-1)?.toLowerCase() ?? "";
  const isImage = file.type.startsWith("image/") || ["gif", "jpeg", "jpg", "png", "webp"].includes(extension);
  const isAudio = Boolean(durationSeconds) || file.type.startsWith("audio/") || file.type === "application/ogg";

  return {
    durationSeconds,
    file,
    id: createUploadId(),
    isAudio,
    isImage,
    name: file.name || (isImage ? "ảnh-dán-từ-clipboard.png" : "file-đính-kèm"),
    previewUrl: (isImage || isAudio) && typeof URL !== "undefined" ? URL.createObjectURL(file) : undefined,
    size: file.size,
    status: "queued"
  };
}

function revokeUploadPreview(item: UploadQueueItem) {
  if (item.previewUrl && typeof URL !== "undefined") {
    URL.revokeObjectURL(item.previewUrl);
  }
}
