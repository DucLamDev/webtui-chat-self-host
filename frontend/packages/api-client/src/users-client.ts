import type { AuthUser } from "@webtui/types";
import type { HttpClient, QueryParams } from "./http-client";
import { collectionFrom, itemFrom } from "./response-utils";

export type ListUsersParams = QueryParams & {
  q?: string;
  status?: string;
  limit?: number;
};

export type UpdateUserInput = Partial<AuthUser> & {
  workspace_id: string;
};

export function createUsersClient(http: HttpClient) {
  return {
    async me() {
      const data = await http.get<unknown>("/api/v1/users/me");
      return itemFrom<AuthUser>(data, "user");
    },
    updateMe(input: Partial<AuthUser>) {
      return http.patch<AuthUser>("/api/v1/users/me", input);
    },
    async list(params: ListUsersParams = {}) {
      const data = await http.get<unknown>("/api/v1/users", { query: params });
      return collectionFrom<AuthUser>(data, "users");
    },
    async get(userId: string) {
      const data = await http.get<unknown>(`/api/v1/users/${encodeURIComponent(userId)}`);
      return itemFrom<AuthUser>(data, "user");
    },
    update(userId: string, input: UpdateUserInput) {
      return http.patch<AuthUser>(`/api/v1/users/${encodeURIComponent(userId)}`, input);
    },
    delete(userId: string, workspaceId: string) {
      return http.delete<void>(`/api/v1/users/${encodeURIComponent(userId)}`, {
        query: { workspace_id: workspaceId }
      });
    }
  };
}
