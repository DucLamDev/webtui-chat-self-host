import { expect, test } from "@playwright/test";

test.skip(process.env.E2E_RUN !== "true", "Đặt E2E_RUN=true và cung cấp tài khoản staging/production để chạy file upload smoke test.");

test("đăng nhập và kiểm tra luồng chọn file trong composer", async ({ page }) => {
  const identifier = process.env.E2E_IDENTIFIER;
  const password = process.env.E2E_PASSWORD;

  test.skip(!identifier || !password, "Thiếu E2E_IDENTIFIER hoặc E2E_PASSWORD.");

  await page.goto("/");
  await page.getByLabel(/Email|tên đăng nhập/i).fill(identifier);
  await page.getByLabel(/Mật khẩu/i).fill(password);
  await page.getByRole("button", { name: /Đăng nhập/i }).click();

  await expect(page.getByText(/Kênh|Tin nhắn|workspace/i).first()).toBeVisible();
  await expect(page.locator('input[type="file"]').first()).toBeAttached();
});
