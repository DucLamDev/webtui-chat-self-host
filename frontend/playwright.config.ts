import { defineConfig, devices } from "@playwright/test";

const webBaseUrl = process.env.E2E_WEB_BASE_URL ?? "http://127.0.0.1:3000";
const adminBaseUrl = process.env.E2E_ADMIN_BASE_URL ?? "http://127.0.0.1:3001";

export default defineConfig({
  expect: {
    timeout: 10_000
  },
  fullyParallel: true,
  projects: [
    {
      name: "web-chromium",
      testMatch: /web\..*\.spec\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: webBaseUrl
      }
    },
    {
      name: "admin-chromium",
      testMatch: /admin\..*\.spec\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: adminBaseUrl
      }
    }
  ],
  reporter: [["list"], ["html", { open: "never" }]],
  retries: process.env.CI ? 1 : 0,
  testDir: "./tests/e2e",
  timeout: 30_000,
  use: {
    trace: "retain-on-failure"
  }
});
