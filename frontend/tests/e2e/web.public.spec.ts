import { expect, test } from "@playwright/test";

test("web instance khóa auth theo hostname hiện tại sau khi hydrate", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByLabel("Server domain")).toHaveCount(0);
  await expect(page.getByLabel(/Email|tên đăng nhập/i)).toBeVisible();
  await expect(page.getByRole("button", { name: "Đăng nhập", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Đăng nhập bằng SSO" })).toBeVisible();
  await expect(page.locator(".auth-loading")).toHaveCount(0);
});

test("form domain-first nằm gọn trong viewport mobile", async ({ page }) => {
  await page.setViewportSize({ height: 844, width: 390 });
  await page.goto("/");

  const authPanel = page.locator(".auth-panel");
  await expect(authPanel).toBeVisible();

  const box = await authPanel.boundingBox();
  expect(box).not.toBeNull();
  expect(box?.x ?? -1).toBeGreaterThanOrEqual(0);
  expect((box?.x ?? 0) + (box?.width ?? 0)).toBeLessThanOrEqual(390);
});

test("discovery local giữ đúng API port và không gửi domain trong body auth", async ({ page }) => {
  let loginURL = "";
  let loginBody: Record<string, unknown> | null = null;
  await page.route("**/api/v1/discovery**", async (route) => {
    await route.fulfill({
      contentType: "application/json",
      headers: { "Access-Control-Allow-Origin": "*" },
      body: JSON.stringify({
        success: true,
        data: {
          domain: "127.0.0.1",
          runtime: {
            admin_base_url: "http://127.0.0.1:3001",
            api_base_url: "http://127.0.0.1:8080",
            app_name: "Local Chat",
            web_base_url: "http://127.0.0.1:3000",
            ws_base_url: "ws://127.0.0.1:8080/ws"
          }
        }
      })
    });
  });
  await page.route("**/api/v1/auth/login", async (route) => {
    loginURL = route.request().url();
    loginBody = route.request().postDataJSON() as Record<string, unknown>;
    await route.fulfill({
      contentType: "application/json",
      status: 401,
      body: JSON.stringify({
        success: false,
        error: { code: "UNAUTHORIZED", message: "Thông tin đăng nhập thử không hợp lệ." }
      })
    });
  });

  await page.goto("/");
  await page.getByLabel(/Email|tên đăng nhập/i).fill("local-smoke@example.com");
  await page.getByLabel("Mật khẩu", { exact: true }).fill("not-a-real-password");
  await page.getByRole("button", { name: "Đăng nhập", exact: true }).click();

  await expect.poll(() => loginURL).toContain("127.0.0.1:8080");
  expect(loginBody).not.toHaveProperty("domain");
  await expect(page.getByText("Thông tin đăng nhập thử không hợp lệ.")).toBeVisible();
});

test("SSO discovery chuyển tới authorization URL của provider", async ({ page }) => {
  await page.route("**/api/v1/discovery**", async (route) => {
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          domain: "127.0.0.1",
          runtime: {
            admin_base_url: "http://127.0.0.1:3001",
            api_base_url: "http://127.0.0.1:3000",
            web_base_url: "http://127.0.0.1:3000",
            ws_base_url: "ws://127.0.0.1:3000/ws"
          }
        }
      })
    });
  });
  await page.route("**/api/v1/auth/oidc/providers**", async (route) => {
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: { oidc_providers: [{ id: "provider-1", name: "Company SSO" }] }
      })
    });
  });
  await page.route("**/api/v1/auth/oidc/start", async (route) => {
    expect(route.request().postDataJSON()).toMatchObject({
      domain: "127.0.0.1",
      provider_id: "provider-1",
      return_to: "http://127.0.0.1:3000/"
    });
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          authorization_url: "http://127.0.0.1:3000/oidc-provider-sentinel",
          expires_at: "2026-07-23T12:10:00Z"
        }
      })
    });
  });

  await page.goto("/");
  await page.getByRole("button", { name: "Đăng nhập bằng SSO" }).click();
  await expect(page).toHaveURL(/oidc-provider-sentinel/);
});
