import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

const rootDir = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@webtui/api-client": resolve(rootDir, "packages/api-client/src/index.ts"),
      "@webtui/chat-core": resolve(rootDir, "packages/chat-core/src/index.ts"),
      "@webtui/icons": resolve(rootDir, "packages/icons/src/index.ts"),
      "@webtui/types": resolve(rootDir, "packages/types/src/index.ts"),
      "@webtui/ui": resolve(rootDir, "packages/ui/src/index.ts")
    }
  },
  test: {
    coverage: {
      reporter: ["text", "lcov"]
    },
    environment: "node",
    include: ["tests/unit/**/*.test.ts", "tests/component/**/*.test.tsx"],
    restoreMocks: true
  }
});
