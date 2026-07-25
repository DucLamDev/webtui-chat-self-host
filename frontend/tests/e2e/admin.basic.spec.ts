import { expect, test } from "@playwright/test";

test.skip(process.env.E2E_RUN !== "true", "Đặt E2E_RUN=true và cung cấp tài khoản admin để chạy smoke test.");

test("đăng nhập và mở dashboard admin", async ({ page }) => {
  const identifier = process.env.E2E_ADMIN_IDENTIFIER ?? process.env.E2E_IDENTIFIER;
  const password = process.env.E2E_ADMIN_PASSWORD ?? process.env.E2E_PASSWORD;

  test.skip(!identifier || !password, "Thiếu E2E_ADMIN_IDENTIFIER/E2E_ADMIN_PASSWORD hoặc E2E_IDENTIFIER/E2E_PASSWORD.");

  await page.goto("/");
  await page.getByLabel(/Email|tên đăng nhập/i).fill(identifier);
  await page.getByLabel(/Mật khẩu/i).fill(password);
  await page.getByRole("button", { name: /Đăng nhập/i }).click();

  await expect(page.getByRole("main", { name: /Bảng quản trị|WebTui/i })).toBeVisible();
  await expect(page.getByText(/Tổng quan|Quản trị|Dashboard/i).first()).toBeVisible();
});
