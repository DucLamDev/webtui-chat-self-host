import type {
  AssignRoleInput,
  CreateRoleInput,
  Permission,
  PermissionCheckResult,
  Role
} from "@webtui/types";
import type { HttpClient, QueryParams } from "./http-client";
import { collectionFrom } from "./response-utils";

export function createRbacClient(http: HttpClient) {
  return {
    async permissions() {
      const data = await http.get<unknown>("/api/v1/rbac/permissions");
      return collectionFrom<Permission>(data, "permissions");
    },
    async roles(params: QueryParams = {}) {
      const data = await http.get<unknown>("/api/v1/rbac/roles", { query: params });
      return collectionFrom<Role>(data, "roles");
    },
    createRole(input: CreateRoleInput) {
      return http.post<Role>("/api/v1/rbac/roles", input);
    },
    async myPermissions(workspaceId?: string) {
      const data = await http.get<unknown>("/api/v1/rbac/me", {
        query: workspaceId ? { workspace_id: workspaceId } : undefined
      });
      return collectionFrom<Permission>(data, "permissions");
    },
    check(params: QueryParams) {
      return http.get<PermissionCheckResult>("/api/v1/rbac/check", { query: params });
    },
    async memberRoles(workspaceId: string, userId: string) {
      const data = await http.get<unknown>(
        `/api/v1/rbac/workspaces/${encodeURIComponent(workspaceId)}/members/${encodeURIComponent(userId)}/roles`
      );
      return collectionFrom<Role>(data, "roles");
    },
    assignMemberRole(workspaceId: string, userId: string, input: AssignRoleInput) {
      return http.post<Role>(
        `/api/v1/rbac/workspaces/${encodeURIComponent(workspaceId)}/members/${encodeURIComponent(userId)}/roles`,
        input
      );
    },
    revokeMemberRole(workspaceId: string, userId: string, roleId: string) {
      return http.delete<void>(
        `/api/v1/rbac/workspaces/${encodeURIComponent(workspaceId)}/members/${encodeURIComponent(userId)}/roles/${encodeURIComponent(roleId)}`
      );
    }
  };
}
