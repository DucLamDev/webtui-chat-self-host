import { expect, test } from "@playwright/test";

test("hiển thị màn hình đăng nhập admin sau khi hydrate", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByLabel(/Email|tên đăng nhập/i)).toBeVisible();
  await expect(page.getByLabel(/Mật khẩu/i)).toBeVisible();
  await expect(page.getByRole("button", { name: /Đăng nhập quản trị/i })).toBeVisible();
  await expect(page.getByRole("button", { name: "Đăng nhập bằng SSO" })).toBeVisible();
  await expect(page.locator(".auth-loading")).toHaveCount(0);
});

test("màn hình đăng nhập admin không tràn ở mobile", async ({ page }) => {
  await page.setViewportSize({ height: 844, width: 390 });
  await page.goto("/");

  const loginForm = page.locator(".admin-login-card");
  await expect(loginForm).toBeVisible();

  const box = await loginForm.boundingBox();
  expect(box).not.toBeNull();
  expect(box?.x ?? -1).toBeGreaterThanOrEqual(0);
  expect((box?.x ?? 0) + (box?.width ?? 0)).toBeLessThanOrEqual(390);
});

test("admin SSO chuyển tới authorization URL của provider", async ({ page }) => {
  let providerRequested = false;
  let startRequest: Record<string, unknown> | null = null;
  await page.route("**/api/v1/auth/oidc/providers**", async (route) => {
    providerRequested = true;
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: { oidc_providers: [{ id: "provider-1", name: "Company SSO" }] }
      })
    });
  });
  await page.route("**/api/v1/auth/oidc/start", async (route) => {
    startRequest = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          authorization_url: "http://127.0.0.1:3001/oidc-provider-sentinel",
          expires_at: "2026-07-23T12:10:00Z"
        }
      })
    });
  });

  await page.goto("/");
  await page.getByRole("button", { name: "Đăng nhập bằng SSO" }).click();
  await expect.poll(() => providerRequested).toBe(true);
  await expect.poll(() => startRequest).not.toBeNull();
  await expect(page).toHaveURL(/oidc-provider-sentinel/);
  expect(startRequest).toMatchObject({
    provider_id: "provider-1",
    return_to: "http://127.0.0.1:3001/"
  });
});
