import type {
  AddWorkspaceMemberInput,
  CreateWorkspaceInput,
  CreateWorkspaceInviteInput,
  UpdateMemberStatusInput,
  UpdateWorkspaceInput,
  UpsertWorkspaceSettingInput,
  Workspace,
  WorkspaceInvite,
  WorkspaceMember,
  WorkspaceSetting
} from "@webtui/types";
import type { HttpClient } from "./http-client";
import { collectionFrom, itemFrom } from "./response-utils";

export function createWorkspacesClient(http: HttpClient) {
  return {
    async listMine() {
      const data = await http.get<unknown>("/api/v1/workspaces");
      return collectionFrom<Workspace>(data, "workspaces");
    },
    create(input: CreateWorkspaceInput) {
      return http.post<Workspace>("/api/v1/workspaces", input);
    },
    async get(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}`);
      return itemFrom<Workspace>(data, "workspace");
    },
    update(workspaceId: string, input: UpdateWorkspaceInput) {
      return http.patch<Workspace>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}`, input);
    },
    archive(workspaceId: string) {
      return http.delete<void>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}`);
    },
    async members(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/members`);
      return collectionFrom<WorkspaceMember>(data, "members");
    },
    addMember(workspaceId: string, input: AddWorkspaceMemberInput) {
      return http.post<WorkspaceMember>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/members`, input);
    },
    updateMemberStatus(workspaceId: string, userId: string, input: UpdateMemberStatusInput) {
      return http.patch<WorkspaceMember>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/members/${encodeURIComponent(userId)}`,
        input
      );
    },
    async settings(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/settings`);
      return collectionFrom<WorkspaceSetting>(data, "settings");
    },
    upsertSetting(workspaceId: string, key: string, input: UpsertWorkspaceSettingInput) {
      return http.put<WorkspaceSetting>(
        `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/settings/${encodeURIComponent(key)}`,
        input
      );
    },
    async invites(workspaceId: string) {
      const data = await http.get<unknown>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/invites`);
      return collectionFrom<WorkspaceInvite>(data, "invites");
    },
    createInvite(workspaceId: string, input: CreateWorkspaceInviteInput) {
      return http.post<WorkspaceInvite>(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/invites`, input);
    }
  };
}
