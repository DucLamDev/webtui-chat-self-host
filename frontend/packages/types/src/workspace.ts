import type { Id, ISODateTime } from "./api";

export type Workspace = {
  id: Id;
  zone_id?: Id;
  name: string;
  slug: string;
  description?: string | null;
  status?: string;
  created_at?: ISODateTime;
  updated_at?: ISODateTime;
};

export type WorkspaceMember = {
  id?: Id;
  user_id: Id;
  workspace_id?: Id;
  display_name?: string;
  username?: string;
  email?: string;
  avatar_url?: string | null;
  phone_number?: string | null;
  role?: string;
  status?: string;
  joined_at?: ISODateTime;
};

export type WorkspaceSetting = {
  key: string;
  value: unknown;
  description?: string | null;
  updated_at?: ISODateTime;
};

export type WorkspaceInvite = {
  id: Id;
  email?: string | null;
  role_id?: Id | null;
  token?: string;
  status?: string;
  expires_at?: ISODateTime | null;
  created_at?: ISODateTime;
};

export type CreateWorkspaceInput = {
  name: string;
  slug: string;
  description?: string;
};

export type UpdateWorkspaceInput = Partial<CreateWorkspaceInput>;

export type AddWorkspaceMemberInput = {
  role_code?: string;
  title?: string;
  user_id: Id;
};

export type UpdateMemberStatusInput = {
  status: string;
};

export type UpsertWorkspaceSettingInput = {
  description?: string;
  value: unknown;
  value_type?: string;
};

export type CreateWorkspaceInviteInput = {
  email?: string;
  expires_days?: number;
  role_code?: string;
};
