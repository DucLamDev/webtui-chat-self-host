import type { Id, ISODateTime, JsonObject } from "./api";

export type FileObject = {
  id: Id;
  workspace_id?: Id;
  channel_id?: Id | null;
  owner_id?: Id | null;
  uploader_id?: Id;
  name?: string;
  file_name?: string;
  original_name?: string;
  mime_type?: string;
  size?: number;
  byte_size?: number;
  size_bytes?: number;
  checksum_sha256?: string | null;
  status?: string;
  url?: string;
  download_url?: string;
  metadata?: JsonObject | null;
  created_at?: ISODateTime;
  updated_at?: ISODateTime;
};

export type FileVersion = {
  id: Id;
  file_id: Id;
  version?: number;
  version_number?: number;
  mime_type?: string;
  byte_size?: number;
  size?: number;
  checksum_sha256?: string | null;
  created_by?: Id | null;
  created_at?: ISODateTime;
};

export type UploadFileInput = {
  file: File;
  channel_id?: Id;
  message_id?: Id;
  sort_order?: number;
  metadata?: JsonObject;
};

export type AttachFileInput = {
  file_id: Id;
  sort_order?: number;
};

export type FileAttachment = {
  workspace_id: Id;
  message_id: Id;
  file_id: Id;
  sort_order?: number;
  created_at?: ISODateTime;
  file: FileObject;
};

export type UploadPart = {
  part_number: number;
  byte_size: number;
  checksum_sha256: string;
  created_at: ISODateTime;
};

export type ResumableUploadSession = {
  id: Id;
  workspace_id: Id;
  owner_id: Id;
  channel_id?: Id | null;
  message_id?: Id | null;
  original_name: string;
  mime_type: string;
  total_size: number;
  chunk_size: number;
  total_chunks: number;
  received_bytes: number;
  uploaded_parts: UploadPart[];
  status: "uploading" | "completing" | "completed" | "cancelled" | "expired" | "failed";
  file_id?: Id | null;
  checksum_sha256?: string | null;
  expires_at: ISODateTime;
  completed_at?: ISODateTime | null;
  created_at: ISODateTime;
  updated_at: ISODateTime;
};

export type OnlyOfficeEditorSession = {
  enabled: boolean;
  session_id: string;
  document_server_url: string;
  script_url: string;
  config: JsonObject;
  expires_at: ISODateTime;
};

export type CreateResumableUploadInput = {
  channel_id?: Id;
  message_id?: Id;
  original_name: string;
  mime_type: string;
  total_size: number;
  chunk_size?: number;
  metadata?: JsonObject;
};
