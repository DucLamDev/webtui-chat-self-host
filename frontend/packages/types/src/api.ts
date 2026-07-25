export type ApiErrorBody = {
  code: string;
  message: string;
  details?: Record<string, unknown>;
};

export type ApiEnvelope<TData, TMeta = unknown> = {
  success: boolean;
  data?: TData;
  error?: ApiErrorBody;
  meta?: TMeta;
  request_id?: string;
  timestamp: string;
};

export type JsonPrimitive = string | number | boolean | null;

export type JsonValue = JsonPrimitive | JsonObject | JsonValue[];

export type JsonObject = {
  [key: string]: JsonValue;
};

export type ISODateTime = string;

export type CursorMeta = {
  has_more?: boolean;
  next_cursor?: string | null;
  prev_cursor?: string | null;
};

export type Id = string;
