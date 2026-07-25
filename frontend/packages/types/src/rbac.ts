import type { Id } from "./api";

export type Permission = {
  id?: Id;
  action?: string;
  code: string;
  name?: string;
  module?: string;
  description?: string | null;
};

export type Role = {
  id: Id;
  name: string;
  code: string;
  workspace_id?: Id | null;
  description?: string | null;
  is_system?: boolean;
  created_by?: Id | null;
  permissions?: Permission[];
};

export type CreateRoleInput = {
  code: string;
  name: string;
  description?: string;
  permission_codes?: string[];
  workspace_id: Id;
};

export type AssignRoleInput = {
  role_id: Id;
};

export type PermissionCheckResult = {
  allowed: boolean;
  permission?: string;
};
