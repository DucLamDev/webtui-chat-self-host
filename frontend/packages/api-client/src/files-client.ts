import type { AttachFileInput, FileAttachment, FileObject, FileVersion, UploadFileInput } from "@webtui/types";
import type { HttpClient, QueryParams } from "./http-client";
import { collectionFrom, itemFrom } from "./response-utils";

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
    async get(workspaceId: string, fileId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/${encodeURIComponent(fileId)}`
      );
      return itemFrom<FileObject>(data, "file");
    },
    download(workspaceId: string, fileId: string) {
      return http.blob(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/${encodeURIComponent(fileId)}/download`);
    },
    async versions(workspaceId: string, fileId: string) {
      const data = await http.get<unknown>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/${encodeURIComponent(fileId)}/versions`
      );
      return collectionFrom<FileVersion>(data, "versions");
    },
    createVersion(workspaceId: string, fileId: string, input: UploadFileInput) {
      const form = new FormData();
      form.append("file", input.file);

      if (input.metadata) {
        form.append("metadata", JSON.stringify(input.metadata));
      }

      return http.post<FileVersion>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/files/${encodeURIComponent(fileId)}/versions`,
        form
      );
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
