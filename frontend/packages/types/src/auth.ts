import type { Id, ISODateTime } from "./api";

export type AuthUser = {
  id: Id;
  email: string;
  username: string;
  display_name?: string;
  avatar_url?: string | null;
  phone_number?: string | null;
  status?: string;
  created_at?: ISODateTime;
  updated_at?: ISODateTime;
};

export type AuthTokenSet = {
  access_token: string;
  access_token_expires_at?: string;
  refresh_token?: string;
  refresh_token_expires_at?: string;
  token_type?: string;
  expires_in?: number;
};

export type AuthResult = {
  access_token?: string;
  access_token_expires_at?: string;
  expires_in?: number;
  refresh_token?: string;
  refresh_token_expires_at?: string;
  refresh_until?: string;
  session_id?: string;
  token_type?: string;
  tokens?: AuthTokenSet;
  user?: AuthUser;
  zone?: AuthZone;
};

export type AuthZone = {
  id: Id;
  slug: string;
  name: string;
  kind: string;
  domain: string;
  workspace_id: Id;
};

export type LoginInput = {
  identifier: string;
  password: string;
  domain?: string;
  device_name?: string;
};

export type GoogleLoginInput = {
  credential: string;
  domain?: string;
  device_name?: string;
};

export type OIDCProviderSummary = {
  id: Id;
  name: string;
};

export type OIDCStartInput = {
  domain: string;
  provider_id: Id;
  return_to?: string;
  device_name?: string;
};

export type OIDCStartResult = {
  authorization_url: string;
  expires_at: ISODateTime;
};

export type OIDCCompleteInput = {
  code: string;
  domain: string;
  device_name?: string;
};

export type RegisterInput = {
  email: string;
  username: string;
  display_name: string;
  domain?: string;
  invite_token?: string;
  password: string;
  device_name?: string;
};

export type RefreshInput = {
  refresh_token: string;
  domain?: string;
};

export type LogoutInput = {
  refresh_token: string;
};

export type AuthSession = {
  id: Id;
  zone_id?: Id;
  workspace_id?: Id;
  domain?: string;
  device_name?: string | null;
  ip_address?: string | null;
  user_agent?: string | null;
  last_seen_at?: ISODateTime | null;
  expires_at?: ISODateTime;
  revoked_at?: ISODateTime | null;
  created_at?: ISODateTime;
};
