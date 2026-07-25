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
  size?: number;
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
