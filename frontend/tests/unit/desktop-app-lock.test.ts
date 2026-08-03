import { describe, expect, it } from "vitest";
import {
  createDesktopPinRecord,
  verifyDesktopPinRecord
} from "../../apps/web/src/features/auth/desktop-app-lock";

describe("desktop app lock", () => {
  it("derives a salted PBKDF2 record and verifies without storing the PIN", async () => {
    const first = await createDesktopPinRecord("123456");
    const second = await createDesktopPinRecord("123456");

    expect(first.iterations).toBe(120_000);
    expect(first.hash).not.toBe("123456");
    expect(first.salt).not.toBe(second.salt);
    await expect(verifyDesktopPinRecord("123456", first)).resolves.toBe(true);
    await expect(verifyDesktopPinRecord("654321", first)).resolves.toBe(false);
  });

  it("rejects weak PINs and downgraded records", async () => {
    await expect(createDesktopPinRecord("12345")).rejects.toThrow(/6/);
    const record = await createDesktopPinRecord("123456");
    await expect(
      verifyDesktopPinRecord("123456", { ...record, iterations: 1 })
    ).resolves.toBe(false);
  });
});
