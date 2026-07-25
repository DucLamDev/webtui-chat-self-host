import { describe, expect, it } from "vitest";
import type { Department } from "@webtui/types";
import {
  buildDepartmentRows,
  departmentDescendantIds
} from "../../apps/web/src/features/chat/model/department-tree";

const departments: Department[] = [
  { id: "child", workspace_id: "workspace", parent_id: "root", name: "Kỹ thuật", slug: "ky-thuat" },
  { id: "root", workspace_id: "workspace", name: "Vận hành", slug: "van-hanh" },
  { id: "grandchild", workspace_id: "workspace", parent_id: "child", name: "Hạ tầng", slug: "ha-tang" }
];

describe("department tree helpers", () => {
  it("orders parent departments before their children", () => {
    expect(buildDepartmentRows(departments).map(({ department, depth }) => [department.id, depth])).toEqual([
      ["root", 0],
      ["child", 1],
      ["grandchild", 2]
    ]);
  });

  it("includes the selected department and all descendants in the invalid parent set", () => {
    expect([...departmentDescendantIds(departments, "child")].sort()).toEqual(["child", "grandchild"]);
  });
});
