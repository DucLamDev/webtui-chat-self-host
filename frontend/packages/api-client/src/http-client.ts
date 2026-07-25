import type { ApiEnvelope } from "@webtui/types";

export type QueryValue = string | number | boolean | null | undefined;

export type QueryParams = Record<string, QueryValue | QueryValue[]>;

export type JsonRequestBody =
  | Record<string, unknown>
  | Array<unknown>
  | string
  | number
  | boolean
  | null;

export type HttpClientOptions = {
  baseUrl: string | (() => string);
  fetcher?: typeof fetch;
  getAccessToken?: () => string | null | undefined;
  onUnauthorized?: () => void;
  refreshAccessToken?: () => Promise<string | null | undefined>;
};

export type RequestOptions = Omit<RequestInit, "body"> & {
  auth?: boolean;
  body?: BodyInit | JsonRequestBody;
  query?: QueryParams;
  unwrap?: boolean;
};

type InternalRequestOptions = RequestOptions & {
  didRefresh?: boolean;
};

export class ApiClientError extends Error {
  readonly code: string;
  readonly details?: Record<string, unknown>;
  readonly requestId?: string;
  readonly status: number;

  constructor(params: {
    code: string;
    details?: Record<string, unknown>;
    message: string;
    requestId?: string;
    status: number;
  }) {
    super(params.message);
    this.name = "ApiClientError";
    this.code = params.code;
    this.details = params.details;
    this.requestId = params.requestId;
    this.status = params.status;
  }
}

export class HttpClient {
  private baseUrl: string | (() => string);
  private readonly fetcher: typeof fetch;
  private readonly getAccessToken?: () => string | null | undefined;
  private readonly onUnauthorized?: () => void;
  private readonly refreshAccessToken?: () => Promise<string | null | undefined>;
  private refreshPromise?: Promise<string | null | undefined>;

  constructor(options: HttpClientOptions) {
    this.baseUrl = options.baseUrl;
    this.fetcher = options.fetcher ?? globalThis.fetch.bind(globalThis);
    this.getAccessToken = options.getAccessToken;
    this.onUnauthorized = options.onUnauthorized;
    this.refreshAccessToken = options.refreshAccessToken;
  }

  setBaseUrl(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  getBaseUrl() {
    return this.resolveBaseUrl();
  }

  async get<TData>(path: string, options: RequestOptions = {}): Promise<TData> {
    return this.request<TData>(path, {
      ...options,
      method: "GET"
    });
  }

  async post<TData>(
    path: string,
    body?: RequestOptions["body"],
    options: RequestOptions = {}
  ): Promise<TData> {
    return this.request<TData>(path, {
      ...options,
      body,
      method: "POST"
    });
  }

  async put<TData>(
    path: string,
    body?: RequestOptions["body"],
    options: RequestOptions = {}
  ): Promise<TData> {
    return this.request<TData>(path, {
      ...options,
      body,
      method: "PUT"
    });
  }

  async patch<TData>(
    path: string,
    body?: RequestOptions["body"],
    options: RequestOptions = {}
  ): Promise<TData> {
    return this.request<TData>(path, {
      ...options,
      body,
      method: "PATCH"
    });
  }

  async delete<TData>(path: string, options: RequestOptions = {}): Promise<TData> {
    return this.request<TData>(path, {
      ...options,
      method: "DELETE"
    });
  }

  async blob(path: string, options: RequestOptions = {}): Promise<Blob> {
    const response = await this.fetch(path, {
      ...options,
      method: options.method ?? "GET",
      unwrap: false
    });

    if (!response.ok) {
      await this.throwResponseError(response);
    }

    return response.blob();
  }

  async request<TData>(
    path: string,
    options: RequestOptions = {}
  ): Promise<TData> {
    const response = await this.fetch(path, options);

    if (
      response.status === 401 &&
      options.auth !== false &&
      !("didRefresh" in options) &&
      this.refreshAccessToken
    ) {
      const nextToken = await this.refreshOnce();
      if (nextToken) {
        return this.request<TData>(path, {
          ...options,
          didRefresh: true
        } as InternalRequestOptions);
      }
    }

    if (response.status === 204) {
      return undefined as TData;
    }

    return this.readJsonResponse<TData>(response, options.unwrap !== false);
  }

  private async fetch(path: string, options: InternalRequestOptions): Promise<Response> {
    const { body, query, didRefresh: _didRefresh, unwrap: _unwrap, ...requestInit } = options;

    return this.fetcher(this.createUrl(path, query), {
      ...requestInit,
      body: this.createBody(body),
      headers: this.createHeaders(options)
    });
  }

  private createUrl(path: string, query?: QueryParams): string {
    const url = new URL(`${this.resolveBaseUrl()}${path.startsWith("/") ? path : `/${path}`}`);

    if (query) {
      for (const [key, value] of Object.entries(query)) {
        const values = Array.isArray(value) ? value : [value];
        for (const item of values) {
          if (item !== undefined && item !== null && item !== "") {
            url.searchParams.append(key, String(item));
          }
        }
      }
    }

    return url.toString();
  }

  private resolveBaseUrl(): string {
    const selected = typeof this.baseUrl === "function" ? this.baseUrl() : this.baseUrl;
    return selected.replace(/\/$/, "");
  }

  private createBody(body: RequestOptions["body"]): BodyInit | null | undefined {
    if (body === undefined || body === null) {
      return body as null | undefined;
    }

    if (this.isNativeBody(body)) {
      return body;
    }

    return JSON.stringify(body);
  }

  private createHeaders(options: RequestOptions): Headers {
    const headers = new Headers(options.headers);
    const hasBody = options.body !== undefined && options.body !== null;

    if (!headers.has("Accept")) {
      headers.set("Accept", "application/json");
    }

    if (hasBody && !headers.has("Content-Type") && !this.isFormData(options.body)) {
      headers.set("Content-Type", "application/json");
    }

    if (options.auth !== false) {
      const token = this.getAccessToken?.();
      if (token) {
        headers.set("Authorization", `Bearer ${token}`);
      }
    }

    return headers;
  }

  private async readJsonResponse<TData>(response: Response, unwrap: boolean): Promise<TData> {
    const payload = await this.parseJson(response);

    if (!response.ok) {
      throw this.toError(response, payload);
    }

    if (!unwrap) {
      return payload as TData;
    }

    if (this.isEnvelope<TData>(payload)) {
      if (!payload.success) {
        throw this.toError(response, payload);
      }

      return payload.data as TData;
    }

    return payload as TData;
  }

  private async throwResponseError(response: Response): Promise<never> {
    const payload = await this.parseJson(response);
    throw this.toError(response, payload);
  }

  private async parseJson(response: Response): Promise<unknown> {
    const text = await response.text();

    if (!text) {
      return undefined;
    }

    try {
      return JSON.parse(text) as unknown;
    } catch {
      return { message: text };
    }
  }

  private toError(response: Response, payload: unknown): ApiClientError {
    const envelope = this.isEnvelope<unknown>(payload) ? payload : undefined;
    const fallbackMessage =
      typeof payload === "object" && payload && "message" in payload
        ? String((payload as { message?: unknown }).message)
        : "Yêu cầu không thành công.";
    const requestId = envelope?.request_id;
    const responseMessage = envelope?.error?.message ?? fallbackMessage;
    const message = response.status >= 500 && requestId
      ? `${responseMessage} (Mã yêu cầu: ${requestId})`
      : responseMessage;

    if (response.status === 401) {
      this.onUnauthorized?.();
    }

    return new ApiClientError({
      code: envelope?.error?.code ?? `HTTP_${response.status}`,
      details: envelope?.error?.details,
      message,
      requestId,
      status: response.status
    });
  }

  private async refreshOnce(): Promise<string | null | undefined> {
    if (!this.refreshPromise) {
      this.refreshPromise = this.refreshAccessToken?.().finally(() => {
        this.refreshPromise = undefined;
      });
    }

    return this.refreshPromise;
  }

  private isEnvelope<TData>(payload: unknown): payload is ApiEnvelope<TData> {
    return Boolean(
      payload &&
        typeof payload === "object" &&
        "success" in payload &&
        typeof (payload as { success?: unknown }).success === "boolean"
    );
  }

  private isNativeBody(body: RequestOptions["body"]): body is BodyInit {
    return (
      this.isFormData(body) ||
      body instanceof Blob ||
      body instanceof ArrayBuffer ||
      body instanceof URLSearchParams ||
      typeof body === "string"
    );
  }

  private isFormData(body: unknown): body is FormData {
    return typeof FormData !== "undefined" && body instanceof FormData;
  }
}
