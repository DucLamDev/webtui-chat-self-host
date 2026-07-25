import { describe, expect, it } from "vitest";
import { createPermissionSet, hasPermission, normalizePermissionCode } from "@webtui/types";

describe("permission utils", () => {
  it("normalizes permission values from strings and backend objects", () => {
    expect(normalizePermissionCode("message.send")).toBe("message.send");
    expect(normalizePermissionCode({ code: "workspace.manage", name: "Quản lý workspace" })).toBe("workspace.manage");
  });

  it("creates a compact permission set and filters empty codes", () => {
    const permissions = createPermissionSet([
      "message.send",
      { code: "file.upload" },
      { code: "" },
      "message.send"
    ]);

    expect([...permissions].sort()).toEqual(["file.upload", "message.send"]);
  });

  it("allows exact permission matches", () => {
    const permissions = createPermissionSet(["message.send", "file.upload"]);

    expect(hasPermission(permissions, "message.send")).toBe(true);
    expect(hasPermission(permissions, "admin.view")).toBe(false);
  });

  it("allows wildcard permissions", () => {
    const permissions = createPermissionSet(["*"]);

    expect(hasPermission(permissions, "admin.view")).toBe(true);
    expect(hasPermission(permissions, "cronjob.manage")).toBe(true);
  });
});
