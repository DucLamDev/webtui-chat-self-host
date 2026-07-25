import type { Department } from "@webtui/types";

export type DepartmentTreeRow = { department: Department; depth: number };

export function buildDepartmentRows(departments: Department[]): DepartmentTreeRow[] {
  const ids = new Set(departments.map((department) => department.id));
  const children = new Map<string | null, Department[]>();

  for (const department of departments) {
    const parentKey = department.parent_id && ids.has(department.parent_id) ? department.parent_id : null;
    const siblings = children.get(parentKey) ?? [];
    siblings.push(department);
    children.set(parentKey, siblings);
  }
  for (const siblings of children.values()) {
    siblings.sort((left, right) => left.name.localeCompare(right.name, "vi"));
  }

  const rows: DepartmentTreeRow[] = [];
  const visited = new Set<string>();
  const visit = (department: Department, depth: number) => {
    if (visited.has(department.id)) return;
    visited.add(department.id);
    rows.push({ department, depth });
    for (const child of children.get(department.id) ?? []) visit(child, depth + 1);
  };

  for (const department of children.get(null) ?? []) visit(department, 0);
  for (const department of departments) visit(department, 0);
  return rows;
}

export function departmentDescendantIds(departments: Department[], departmentId: string): Set<string> {
  const result = new Set<string>([departmentId]);
  let changed = true;

  while (changed) {
    changed = false;
    for (const department of departments) {
      if (department.parent_id && result.has(department.parent_id) && !result.has(department.id)) {
        result.add(department.id);
        changed = true;
      }
    }
  }
  return result;
}
