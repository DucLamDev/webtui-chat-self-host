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

export type DeleteMeInput = {
  confirmation: "DELETE";
  ownership_successor_email?: string;
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
    async uploadMyAvatar(file: File) {
      const form = new FormData();
      form.append("avatar", file, file.name);
      const data = await http.post<unknown>("/api/v1/users/me/avatar", form);
      return requiredUserAvatar(data);
    },
    deleteMe(input: DeleteMeInput) {
      return http.delete<void>("/api/v1/users/me", {
        body: input
      });
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

function requiredUserAvatar(data: unknown) {
  const avatar = itemFrom<{ avatar_path: string; content_type: string; size: number }>(data, "avatar");
  if (!avatar?.avatar_path) {
    throw new Error("Không nhận được đường dẫn ảnh đại diện sau khi tải lên.");
  }
  return avatar;
}
