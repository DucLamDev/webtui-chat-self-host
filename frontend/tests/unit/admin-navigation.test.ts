import { describe, expect, it } from "vitest";
import {
  adminNavigationGroups,
  adminPageMeta,
  adminSectionGroup,
  canAccessAdminSection,
  customAdminModulesEnabled,
  customAdminSections,
  enabledAdminSections,
  type AdminNavId,
  resolveAdminSection
} from "../../apps/admin/src/features/dashboard/model/navigation";

describe("admin navigation", () => {
  it("lists every default admin section exactly once", () => {
    const groupedItems = adminNavigationGroups.flatMap((group) => group.items);

    expect(new Set(groupedItems).size).toBe(groupedItems.length);
    expect([...groupedItems].sort()).toEqual([...enabledAdminSections].sort());
    expect(customAdminSections.every((section) => groupedItems.includes(section))).toBe(customAdminModulesEnabled);
  });

  it("keeps custom-only admin pages out of the default navigation until enabled", () => {
    const pageIds = Object.keys(adminPageMeta) as AdminNavId[];
    const disabledPageIds = pageIds.filter((section) => !enabledAdminSections.includes(section));
    const expectedDisabledPageIds = customAdminModulesEnabled ? [] : [...customAdminSections];

    expect(disabledPageIds.sort()).toEqual(expectedDisabledPageIds.sort());
  });

  it("uses unique group identifiers", () => {
    const groupIds = adminNavigationGroups.map((group) => group.id);

    expect(new Set(groupIds).size).toBe(groupIds.length);
  });

  it.each(enabledAdminSections)(
    "resolves the enabled %s section",
    (section) => {
      expect(resolveAdminSection(section)).toBe(section);
    }
  );

  it.each(customAdminSections)(
    "handles the custom-only %s section according to the feature flag",
    (section) => {
      if (customAdminModulesEnabled) {
        expect(resolveAdminSection(section)).toBe(section);
        expect(adminSectionGroup(section)).not.toBe("Qu\u1ea3n tr\u1ecb");
      } else {
        expect(resolveAdminSection(section)).toBe("overview");
        expect(adminSectionGroup(section)).toBe("Qu\u1ea3n tr\u1ecb");
      }
    }
  );

  it.each([undefined, null, "", "unknown", "__proto__", "toString"])(
    "falls back to overview for invalid section %s",
    (section) => {
      expect(resolveAdminSection(section)).toBe("overview");
    }
  );

  it.each([
    ["overview", "monitoring"],
    ["push", "monitoring"],
    ["moderation", "workspace"],
    ["messages", "workspace"],
    ["channels", "workspace"],
    ["users", "workspace"],
    ["roles", "workspace"],
    ["cronjobs", "operations"],
    ["backups", "operations"],
    ["settings", "operations"]
  ] as const)("maps %s to the %s group", (section, groupId) => {
    const group = adminNavigationGroups.find((item) => item.id === groupId);

    expect(group).toBeDefined();
    expect(adminSectionGroup(section)).toBe(group?.label);
  });

  it("uses the management fallback for an unknown runtime section", () => {
    expect(adminSectionGroup("unknown" as AdminNavId)).toBe("Qu\u1ea3n tr\u1ecb");
  });

  it("gates only the moderation section with moderation.manage", () => {
    const denied = () => false;
    const moderationManager = (permission: string) => permission === "moderation.manage";

    expect(canAccessAdminSection("moderation", denied)).toBe(false);
    expect(canAccessAdminSection("moderation", moderationManager)).toBe(true);
    expect(canAccessAdminSection("messages", denied)).toBe(true);
  });
});
