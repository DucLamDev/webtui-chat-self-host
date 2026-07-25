import type { CreateDepartmentInput, Department, DepartmentMember, UpdateDepartmentInput } from "@webtui/types";
import type { HttpClient } from "./http-client";
import { collectionFrom, itemFrom } from "./response-utils";

export function createDepartmentsClient(http: HttpClient) {
  const base = (workspaceId: string) => `/api/v1/workspaces/${encodeURIComponent(workspaceId)}/departments`;

  return {
    async list(workspaceId: string) {
      const data = await http.get<unknown>(base(workspaceId));
      return collectionFrom<Department>(data, "departments");
    },
    create(workspaceId: string, input: CreateDepartmentInput) {
      return http.post<Department>(base(workspaceId), input);
    },
    async get(workspaceId: string, departmentId: string) {
      const data = await http.get<unknown>(`${base(workspaceId)}/${encodeURIComponent(departmentId)}`);
      return itemFrom<Department>(data, "department");
    },
    update(workspaceId: string, departmentId: string, input: UpdateDepartmentInput) {
      return http.patch<Department>(`${base(workspaceId)}/${encodeURIComponent(departmentId)}`, input);
    },
    delete(workspaceId: string, departmentId: string) {
      return http.delete<void>(`${base(workspaceId)}/${encodeURIComponent(departmentId)}`);
    },
    async members(workspaceId: string, departmentId: string) {
      const data = await http.get<unknown>(`${base(workspaceId)}/${encodeURIComponent(departmentId)}/members`);
      return collectionFrom<DepartmentMember>(data, "members");
    },
    addMember(workspaceId: string, departmentId: string, userId: string, role: "lead" | "member" = "member") {
      return http.post<DepartmentMember>(`${base(workspaceId)}/${encodeURIComponent(departmentId)}/members`, {
        role,
        user_id: userId
      });
    },
    removeMember(workspaceId: string, departmentId: string, userId: string) {
      return http.delete<void>(`${base(workspaceId)}/${encodeURIComponent(departmentId)}/members/${encodeURIComponent(userId)}`);
    }
  };
}
