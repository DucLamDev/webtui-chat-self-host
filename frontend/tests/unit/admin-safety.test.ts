import { describe, expect, it } from "vitest";
import {
  escapeCsvCell,
  isAdminUserBlockedStatus
} from "../../apps/admin/src/features/dashboard/model/admin-safety";

describe("admin safety", () => {
  it.each(["locked", "blocked", "disabled", "LOCKED", "Blocked", "Disabled"])(
    "treats %s as a blocked admin user status",
    (status) => {
      expect(isAdminUserBlockedStatus(status)).toBe(true);
    }
  );

  it.each(["active", "invited", "", null, undefined, 0])(
    "does not treat %s as a blocked admin user status",
    (status) => {
      expect(isAdminUserBlockedStatus(status)).toBe(false);
    }
  );

  it.each([
    "=HYPERLINK(\"https://example.test\")",
    "+SUM(1,2)",
    "-1+2",
    "@SUM(1,2)"
  ])("neutralizes a CSV formula beginning with %s", (value) => {
    expect(escapeCsvCell(value)).toBe(`"'${value.replaceAll('"', '""')}"`);
  });

  it.each([
    " =SUM(1,2)",
    "\t=SUM(1,2)",
    "\r\n+SUM(1,2)",
    "\u0000@SUM(1,2)",
    "\u001f-1+2"
  ])("neutralizes a CSV formula after whitespace or control characters", (value) => {
    expect(escapeCsvCell(value)).toBe(`"'${value}"`);
  });

  it("preserves non-formula values while escaping CSV quotes", () => {
    expect(escapeCsvCell(" normal text")).toBe('" normal text"');
    expect(escapeCsvCell('a"b')).toBe('"a""b"');
    expect(escapeCsvCell(42)).toBe('"42"');
  });
});
