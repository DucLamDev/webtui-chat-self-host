import { describe, expect, it } from "vitest";
import {
  adminNavigationGroups,
  adminPageMeta,
  adminSectionGroup,
  type AdminNavId,
  resolveAdminSection
} from "../../apps/admin/src/features/dashboard/model/navigation";

describe("admin navigation", () => {
  it("lists every admin page exactly once", () => {
    const groupedItems = adminNavigationGroups.flatMap((group) => group.items);
    const pageIds = Object.keys(adminPageMeta);

    expect(new Set(groupedItems).size).toBe(groupedItems.length);
    expect([...groupedItems].sort()).toEqual([...pageIds].sort());
  });

  it("uses unique group identifiers", () => {
    const groupIds = adminNavigationGroups.map((group) => group.id);

    expect(new Set(groupIds).size).toBe(groupIds.length);
  });

  it.each(Object.keys(adminPageMeta) as AdminNavId[])(
    "resolves the valid %s section",
    (section) => {
      expect(resolveAdminSection(section)).toBe(section);
    }
  );

  it.each([undefined, null, "", "unknown", "__proto__", "toString"])(
    "falls back to overview for invalid section %s",
    (section) => {
      expect(resolveAdminSection(section)).toBe("overview");
    }
  );

  it.each([
    ["overview", "Giám sát"],
    ["push", "Giám sát"],
    ["messages", "Workspace"],
    ["channels", "Workspace"],
    ["users", "Workspace"],
    ["roles", "Workspace"],
    ["integrations", "Mở rộng"],
    ["automations", "Mở rộng"],
    ["bots", "Mở rộng"],
    ["cronjobs", "Vận hành"],
    ["backups", "Vận hành"],
    ["settings", "Vận hành"]
  ] as const)("maps %s to the %s group", (section, group) => {
    expect(adminSectionGroup(section)).toBe(group);
  });

  it("uses the management fallback for an unknown runtime section", () => {
    expect(adminSectionGroup("unknown" as AdminNavId)).toBe("Quản trị");
  });
});
