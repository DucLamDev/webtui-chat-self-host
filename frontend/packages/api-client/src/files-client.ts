import type {
  AttachFileInput,
  CreateResumableUploadInput,
  FileAttachment,
  FileObject,
  FileVersion,
  ResumableUploadSession,
  UploadFileInput
} from "@webtui/types";
import type { HttpClient, QueryParams } from "./http-client";
import { collectionFrom, itemFrom } from "./response-utils";

const resumableUploadThreshold = 8 * 1024 * 1024;

export type ResumableBrowserUploadInput = UploadFileInput & {
  onProgress?: (receivedBytes: number, totalBytes: number) => void;
  onSession?: (sessionId: string) => void;
  session_id?: string;
};

export function createFilesClient(http: HttpClient) {
  return {
    async list(workspaceId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files`, {
        query: params
      });
      return collectionFrom<FileObject>(data, "files");
    },
    async upload(workspaceId: string, input: UploadFileInput) {
      const form = new FormData();
      form.append("file", input.file);

      if (input.channel_id) {
        form.append("channel_id", input.channel_id);
      }

      if (input.message_id) {
        form.append("message_id", input.message_id);
      }

      if (typeof input.sort_order === "number" && Number.isFinite(input.sort_order)) {
        form.append("sort_order", String(input.sort_order));
      }

      if (input.metadata) {
        form.append("metadata", JSON.stringify(input.metadata));
      }

      const data = await http.post<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files`, form);
      const file = itemFrom<FileObject>(data, "file");
      if (!file) {
        throw new Error("Không nhận được dữ liệu tệp sau khi tải lên.");
      }
      return file;
    },
    async createUploadSession(workspaceId: string, input: CreateResumableUploadInput) {
      const data = await http.post<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/uploads`,
        input
      );
      return itemFrom<ResumableUploadSession>(data, "upload") ?? (data as ResumableUploadSession);
    },
    async uploadSession(workspaceId: string, uploadId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/uploads/${encodeURIComponent(uploadId)}`
      );
      return itemFrom<ResumableUploadSession>(data, "upload") ?? (data as ResumableUploadSession);
    },
    async uploadPart(
      workspaceId: string,
      uploadId: string,
      partNumber: number,
      body: Blob,
      checksumSha256?: string
    ) {
      const data = await http.put<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/uploads/${encodeURIComponent(uploadId)}/parts/${partNumber}`,
        body,
        {
          headers: {
            "Content-Type": "application/octet-stream",
            ...(checksumSha256 ? { "X-Chunk-SHA256": checksumSha256 } : {})
          }
        }
      );
      return itemFrom<ResumableUploadSession>(data, "upload") ?? (data as ResumableUploadSession);
    },
    async completeUpload(workspaceId: string, uploadId: string, checksumSha256?: string) {
      const data = await http.post<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/uploads/${encodeURIComponent(uploadId)}/complete`,
        checksumSha256 ? { checksum_sha256: checksumSha256 } : {}
      );
      return itemFrom<FileObject>(data, "file") ?? (data as FileObject);
    },
    async uploadResumable(workspaceId: string, input: ResumableBrowserUploadInput) {
      const uploadBasePath = `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/uploads`;
      const initialData = input.session_id
        ? await http.get<unknown>(`${uploadBasePath}/${encodeURIComponent(input.session_id)}`)
        : await http.post<unknown>(uploadBasePath, {
            channel_id: input.channel_id,
            message_id: input.message_id,
            original_name: input.file.name,
            mime_type: input.file.type || "application/octet-stream",
            total_size: input.file.size,
            metadata: input.metadata
          });
      let session = itemFrom<ResumableUploadSession>(initialData, "upload")
        ?? (initialData as ResumableUploadSession);
      input.onSession?.(session.id);
      if (session.status === "completed" && session.file_id) {
        const data = await http.get<unknown>(
          `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/${encodeURIComponent(session.file_id)}`
        );
        return itemFrom<FileObject>(data, "file") ?? (data as FileObject);
      }
      const uploaded = new Set(session.uploaded_parts.map((part) => part.part_number));
      let receivedBytes = session.received_bytes;
      input.onProgress?.(receivedBytes, input.file.size);
      const missing = Array.from({ length: session.total_chunks }, (_, index) => index)
        .filter((partNumber) => !uploaded.has(partNumber));
      let cursor = 0;
      const uploadWorker = async () => {
        while (cursor < missing.length) {
          const partNumber = missing[cursor++];
          const start = partNumber * session.chunk_size;
          const end = Math.min(start + session.chunk_size, input.file.size);
          const partData = await http.put<unknown>(
            `${uploadBasePath}/${encodeURIComponent(session.id)}/parts/${partNumber}`,
            input.file.slice(start, end),
            { headers: { "Content-Type": "application/octet-stream" } }
          );
          session = itemFrom<ResumableUploadSession>(partData, "upload")
            ?? (partData as ResumableUploadSession);
          receivedBytes = Math.max(receivedBytes, session.received_bytes);
          input.onProgress?.(receivedBytes, input.file.size);
        }
      };
      await Promise.all(Array.from({ length: Math.min(3, missing.length) }, () => uploadWorker()));
      const completed = await http.post<unknown>(
        `${uploadBasePath}/${encodeURIComponent(session.id)}/complete`,
        {}
      );
      return itemFrom<FileObject>(completed, "file") ?? (completed as FileObject);
    },
    shouldUseResumableUpload(file: File) {
      return file.size >= resumableUploadThreshold;
    },
    cancelUpload(workspaceId: string, uploadId: string) {
      return http.delete<void>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/uploads/${encodeURIComponent(uploadId)}`
      );
    },
    async get(workspaceId: string, fileId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/${encodeURIComponent(fileId)}`
      );
      return itemFrom<FileObject>(data, "file");
    },
    download(workspaceId: string, fileId: string) {
      return http.blob(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/${encodeURIComponent(fileId)}/download`,
        {
          cache: "no-store",
          query: { _fresh: Date.now() }
        }
      );
    },
    async versions(workspaceId: string, fileId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/${encodeURIComponent(fileId)}/versions`
      );
      return collectionFrom<FileVersion>(data, "versions");
    },
    async createVersion(workspaceId: string, fileId: string, input: UploadFileInput) {
      const form = new FormData();
      form.append("file", input.file);

      if (input.metadata) {
        form.append("metadata", JSON.stringify(input.metadata));
      }

      const data = await http.post<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/${encodeURIComponent(fileId)}/versions`,
        form
      );
      return itemFrom<FileVersion>(data, "version") ?? (data as FileVersion);
    },
    async attachments(workspaceId: string, channelId: string, messageId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/attachments`
      );
      return collectionFrom<FileAttachment>(data, "attachments");
    },
    async channelMedia(workspaceId: string, channelId: string, params: QueryParams = {}) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/media`,
        { query: params }
      );
      return collectionFrom<FileAttachment>(data, "attachments");
    },
    async attach(workspaceId: string, channelId: string, messageId: string, input: AttachFileInput) {
      const data = await http.post<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/channels/${encodeURIComponent(channelId)}/messages/${encodeURIComponent(messageId)}/attachments`,
        input
      );
      const attachment = itemFrom<FileAttachment>(data, "attachment");
      if (!attachment) {
        throw new Error("Không nhận được dữ liệu đính kèm sau khi gắn tệp.");
      }
      return attachment;
    }
  };
}
